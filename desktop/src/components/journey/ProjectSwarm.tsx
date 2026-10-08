import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { errorText } from '../../api/product';
import { projectRunsApi, runOperationNeedsRecovery, runOperationNeedsReview, runSessionRequest, type ProjectRun, type RunOperation, type SwarmTask } from '../../api/projectRuns';
import { swarmTaskCapability } from '../../api/swarmTaskCapability';
import { projectSwarmApi, type SwarmReview } from '../../api/projectSwarm';
import { useCockpit } from '../../store/store';
import { Button } from '../fluid/components/button';
import { WorkflowDialog } from '../common/WorkflowDialog';
import { SwarmWorkspaceReview } from './SwarmWorkspaceReview';
import { SwarmRunQueue } from './SwarmRunQueue';
import type { ProjectSwarmQueue } from '../../api/swarmQueue';
import './project-swarm.css';

const inFlight = (operation?: RunOperation) => !!operation && (runOperationNeedsRecovery(operation) || ['reserved', 'dispatching', 'running', 'unknown'].includes(operation.status) || operation.allocation === 'allocating' || operation.allocation === 'unknown');
const integrated = (task: SwarmTask) => task.integration?.state === 'integrated';
const operationFor = (run: ProjectRun, task: SwarmTask) => run.operations.find(item => item.id === task.operationId);
const pathOverlaps = (a: string, b: string) => {
  const x = a.toLowerCase().replace(/\/$/, ''), y = b.toLowerCase().replace(/\/$/, '');
  return x === y || x.startsWith(y + '/') || y.startsWith(x + '/');
};
function waitingReason(run: ProjectRun, task: SwarmTask) {
  if (integrated(task)) return 'Changes integrated. Whole-build verification is still separate.';
  if (task.integration) return 'Integration outcome is unconfirmed. Check team status and inspect the retained changes before continuing.';
  if (run.swarm?.state !== 'ready' || run.state === 'cancelled') return 'New starts are paused or cancelled.';
  if (run.codingBlocker || run.contextFreshness && run.contextFreshness.status !== 'current') return 'Review the current project decisions and context before starting more work.';
  const waiting = task.dependsOn.filter(id => run.swarm?.tasks.find(item => item.id === id)?.integration?.state !== 'integrated');
  if (waiting.length) return `Waiting for reviewed integration: ${waiting.map(id => run.swarm?.tasks.find(item => item.id === id)?.title || id).join('; ')}.`;
  const active = run.swarm?.tasks.filter(item => item.id !== task.id && inFlight(operationFor(run, item))) || [];
  if (active.length >= (run.swarm?.maxParallel || 1)) return 'All agent slots are occupied. Check team status when work finishes.';
  const overlap = active.find(item => !item.writePaths.length || !task.writePaths.length || item.writePaths.some(a => task.writePaths.some(b => pathOverlaps(a, b))));
  if (overlap) return `Wait for ${overlap.title}: intended files overlap or have not been specified.`;
  return '';
}
function taskStatus(run: ProjectRun, task: SwarmTask, taskAllowed: boolean) {
  if (integrated(task)) return 'Integrated';
  if (task.integration) return 'Integration outcome unconfirmed';
  const operation = operationFor(run, task);
  if (runOperationNeedsRecovery(operation)) return 'Recovery pending · finish recovery to release capacity';
  if (operation?.allocation === 'unknown') return 'Workspace outcome unconfirmed';
  if (operation?.allocation === 'allocating') return 'Preparing workspace';
  if (operation?.status === 'reserved') return 'Reserved · not started';
  if (operation?.status === 'running') return 'Recorded as running';
  if (operation?.status === 'dispatching' || operation?.status === 'unknown') return 'Start outcome unconfirmed';
  if (runOperationNeedsReview(operation)) return 'Awaiting review';
  if (operation?.status === 'failed' || operation?.status === 'cancelled') return 'Stopped · inspect before retry';
  if (!taskAllowed) return 'Needs task permission review';
  return waitingReason(run, task) ? 'Waiting' : 'Ready to start';
}

interface Props {
  run: ProjectRun; canEdit: boolean; canExecute: boolean; busy: boolean;
  onBusy(label: string): void; onUpdate(run: ProjectRun): void;
  onOpenSession(operation: RunOperation): Promise<void>;
  onReviewPlan?(): void;
}

/** One reviewed run owns the task team; rendering and polling never dispatch it. */
export function ProjectSwarm({run, canEdit, canExecute, busy, onBusy, onUpdate, onOpenSession, onReviewPlan}: Props) {
  const harnesses = useCockpit(state => state.harnesses);
  const capability = swarmTaskCapability(run, harnesses);
  const [dialog, setDialog] = useState<'prepare' | 'cancel' | null>(null);
  const [limit, setLimit] = useState(2), [review, setReview] = useState<SwarmReview | null>(null), [approved, setApproved] = useState(false);
  const [retire, setRetire] = useState<string | null>(null);
  const [workspace, setWorkspace] = useState<RunOperation | null>(null);
  const [queuedTasks, setQueuedTasks] = useState<string[]>([]);
  const [queuedOperations, setQueuedOperations] = useState<string[]>([]);
  const queueSignature = useRef('');
  const [error, setError] = useState(''), [notice, setNotice] = useState('');
  const live = useRef(true), working = useRef(false), generation = useRef(0);
  const current = useRef({run, canEdit, canExecute, taskAllowed: capability.allowed}); current.current = {run, canEdit, canExecute, taskAllowed: capability.allowed};
  const authorityKey = `${canEdit}:${canExecute}:${capability.signature}`;
  const authority = useRef(authorityKey);
  if (authority.current !== authorityKey) { authority.current = authorityKey; generation.current++; }
  const keys = useRef(new Map<string, string>()), uncertain = useRef(new Set<string>());
  const [, setUncertainVersion] = useState(0);
  useEffect(() => { live.current = true; return () => { live.current = false; generation.current++; }; }, []);
  useEffect(() => { setReview(null); setApproved(false); setRetire(null); setDialog(null); }, [run.revision, canEdit, canExecute, capability.signature]);
  const usable = (epoch: number) => live.current && generation.current === epoch;
  const blocked = !!run.codingBlocker || !!run.contextFreshness && run.contextFreshness.status !== 'current' || ['cancelled', 'complete'].includes(run.state);
  const subscription = run.input.route.connection.kind === 'codex-subscription' || run.input.route.connection.authMode === 'chatgpt';
  const selectedLimit = subscription ? 1 : limit;
  const tasks = run.swarm?.tasks || [], active = tasks.filter(task => inFlight(operationFor(run, task))).length;
  const retiringTask = tasks.find(task => task.id === retire);
  const finishingRecovery = !!retiringTask && runOperationNeedsRecovery(operationFor(run, retiringTask));
  const ready = tasks.filter(task => !queuedTasks.includes(task.id) && !task.integration && !task.operationId && !waitingReason(run, task));
  const batchReady: SwarmTask[] = [];
  for (const task of ready) {
    if (batchReady.length >= (run.swarm?.maxParallel || 1) - active) break;
    if (batchReady.some(other => !task.writePaths.length || !other.writePaths.length || task.writePaths.some(a => other.writePaths.some(b => pathOverlaps(a, b))))) continue;
    batchReady.push(task);
  }
  function remember(value: ProjectRun, epoch: number) {
    if (!usable(epoch)) return false;
    current.current = {...current.current, run: value}; onUpdate(value); return true;
  }
  async function operate(label: string, action: (epoch: number) => Promise<void>, requires: 'read' | 'edit' | 'execute' = 'execute') {
    if (working.current || busy || requires !== 'read' && !canEdit || requires === 'execute' && !canExecute) return;
    const epoch = generation.current;
    working.current = true; onBusy(label); setError(''); setNotice('');
    try { await action(epoch); }
    catch (cause) { if (usable(epoch)) setError(errorText(cause)); }
    finally { working.current = false; if (live.current) onBusy(''); }
  }
  async function control(action: 'pause' | 'resume' | 'cancel' | 'reconcile', epoch: number) {
    if (action === 'resume' && !current.current.taskAllowed) return;
    const value = await projectSwarmApi.control(run.projectId, run.id, action, run.revision);
    if (!remember(value, epoch)) return;
    uncertain.current.clear(); setUncertainVersion(value => value + 1); setDialog(null);
    setNotice(action === 'reconcile' ? 'Existing task sessions inspected. No new agent was started.' : action === 'resume' ? 'New starts enabled. Previously authorized queued tasks can continue when their dependencies and capacity allow.' : 'Future task starts stopped. Existing sessions keep their own stop controls.');
  }
  async function startTask(saved: ProjectRun, task: SwarmTask, epoch: number): Promise<ProjectRun | undefined> {
    if (!usable(epoch) || !current.current.canEdit || !current.current.canExecute || !current.current.taskAllowed) return;
    let operation = operationFor(saved, task);
    if (!operation || !inFlight(operation)) {
      const keyScope = `${saved.id}:${task.id}:${saved.revision}`;
      let key = keys.current.get(keyScope);
      if (!key) { key = crypto.randomUUID(); keys.current.set(keyScope, key); }
      const reservation = await projectSwarmApi.reserve(saved.projectId, saved.id, task.id, saved.revision, key);
      if (!remember(reservation.run, epoch)) return;
      saved = reservation.run; operation = reservation.operation;
    }
    if (!usable(epoch) || !current.current.canExecute || !current.current.canEdit || !current.current.taskAllowed) return;
    if (operation.status !== 'reserved' || operation.allocation !== 'ready') throw new Error('Check team status before starting this reserved task. An uncertain start is never replayed.');
    uncertain.current.add(task.id); setUncertainVersion(value => value + 1);
    const session = await api.createSession(runSessionRequest(saved, operation));
    if (!usable(epoch)) return;
    if (session.id !== operation.childId || session.worktreeId !== operation.worktreeId || session.projectId !== saved.projectId || session.projectRunId !== saved.id || session.projectRunOperationId !== operation.id) throw new Error('The session did not match its task reservation. Check team status before continuing.');
    useCockpit.getState().upsertSession(session);
    const value = await projectRunsApi.get(saved.projectId, saved.id);
    if (!remember(value, epoch)) return;
    uncertain.current.delete(task.id); setUncertainVersion(value => value + 1);
    setNotice('Task session started in its isolated workspace. Review its changes before integration, then verify the complete build.');
    return value;
  }
  async function startReady(epoch: number) {
    let saved = run;
    for (const task of batchReady) {
      const nextTask = saved.swarm?.tasks.find(item => item.id === task.id);
      if (!nextTask || waitingReason(saved, nextTask)) continue;
      const value = await startTask(saved, nextTask, epoch);
      if (!value) break;
      saved = value;
    }
  }
  function queueChanged(queue: ProjectSwarmQueue) {
    const items = queue.items.filter(item => item.runId === run.id);
    const pending = items.filter(item => !['cancelled', 'completed'].includes(item.state)).map(item => item.taskId);
    setQueuedTasks(previous => previous.length === pending.length && previous.every((id, i) => id === pending[i]) ? previous : pending);
    const owned = items.flatMap(item => item.operationId ? [item.operationId] : []);
    setQueuedOperations(previous => previous.length === owned.length && previous.every((id, i) => id === owned[i]) ? previous : owned);
    const signature = JSON.stringify(items.map(item => [item.id, item.operationId, item.childId, item.state]));
    if (signature === queueSignature.current) return;
    if (busy || working.current || !items.some(item => item.operationId)) return;
    queueSignature.current = signature;
    const epoch = generation.current;
    void projectRunsApi.get(run.projectId, run.id).then(value => {
      if (usable(epoch) && !working.current && value.revision >= current.current.run.revision) remember(value, epoch);
      else if (queueSignature.current === signature) queueSignature.current = '';
    }).catch(() => { if (queueSignature.current === signature) queueSignature.current = ''; });
  }
  async function readReview(task: SwarmTask, epoch: number) {
    const value = await projectSwarmApi.review(run.projectId, run, task.id);
    if (!usable(epoch) || current.current.run.revision !== run.revision) return;
    setReview(value); setApproved(false);
  }
  async function retireAttempt(taskId: string, epoch: number) {
    const task = tasks.find(item => item.id === taskId), operation = task && operationFor(run, task);
    if (!operation) throw new Error('This task attempt changed. Refresh and inspect it before continuing.');
    const recoverMissing = operation.allocation === 'ready' && (['dispatching', 'unknown'].includes(operation.status) || !!operation.absenceRetirement);
    try {
      const value = recoverMissing
        ? await projectSwarmApi.abandon(run.projectId, run.id, taskId, run.revision, {operationId: operation.id, childId: operation.childId, recoverMissingChild: true})
        : await projectSwarmApi.abandon(run.projectId, run.id, taskId, run.revision);
      if (remember(value, epoch)) { setRetire(null); setNotice('Attempt retired. Output is retained. No new agent was started.'); }
    } catch (cause) {
      // The tombstone may have persisted before finalization or its response
      // failed. Read it once; never replay a state-changing recovery request.
      if (usable(epoch)) {
        try {
          const value = await projectRunsApi.get(run.projectId, run.id);
          if (value.revision >= current.current.run.revision) remember(value, epoch);
        } catch {
          throw new Error(`${errorText(cause)} The recovery result could not be refreshed. Check team status before continuing.`);
        }
      }
      throw cause;
    }
  }
  return <section className="project-swarm" aria-label="Agent team">
    <div className="project-swarm-heading"><h3>Agent team</h3>{run.swarm && <span>{tasks.filter(integrated).length} of {tasks.length} integrated</span>}</div>
    {error && <p role="alert" className="journey-error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    <p>Autonomous tasks run commands without interactive approvals inside isolated Docker workspaces, then exit after their response. Outputs still need review, integration and verification; Luci does not accept them automatically.</p>
    <p className="journey-muted">Saved agent: {run.input.route.harness} · Permission: {capability.mode}. Keep Ask or Plan in an interactive agent if you want approvals during execution.</p>
    {!capability.allowed && <p role="status" className="journey-warning">{capability.reason}</p>}
    {capability.notice && <details><summary>Agent task permissions</summary><p>{capability.notice}</p></details>}
    {!capability.allowed && <div className="journey-actions">{onReviewPlan && <Button variant="secondary" disabled={busy || !canEdit} onClick={onReviewPlan}>Review plan and create a new build</Button>}<Button variant="tertiary" disabled={busy} onClick={() => void operate('Refreshing task capability', async epoch => { const values = await api.listHarnesses(); if (usable(epoch)) useCockpit.getState().setHarnesses(values); }, 'read')}>Refresh task capability</Button></div>}
    {!run.swarm ? <>
      <p>Split the reviewed tasks between agents. Each gets its own workspace and focused context; dependent work waits for reviewed changes to be integrated.</p>
      {run.operations.length ? <p className="journey-muted">This build already has coding attempts. Prepare a new reviewed build to use an agent team.</p> : <Button variant="secondary" disabled={!canEdit || !canExecute || busy || blocked || !capability.allowed} onClick={() => setDialog('prepare')}>Prepare agent team</Button>}
    </> : <>
      <p>{run.swarm.state === 'ready' ? capability.allowed ? 'New starts enabled' : 'New starts unavailable for this saved agent configuration' : run.swarm.state === 'paused' ? 'New starts paused' : 'Future starts cancelled'} · {active} of {run.swarm.maxParallel} agent slots occupied</p>
      <p className="journey-muted">Status is recorded, not a live process guarantee. Check team status after an interruption. Pausing future starts leaves existing agents running.</p>
      <div className="journey-actions">
        <Button disabled={!canEdit || !canExecute || busy || blocked || !capability.allowed || !batchReady.length} onClick={() => void operate('Starting ready tasks', startReady)}>Start ready tasks{batchReady.length ? ` (${batchReady.length})` : ''}</Button>
        <Button variant="secondary" disabled={!canEdit || busy} onClick={() => void operate('Checking team status', epoch => control('reconcile', epoch), 'edit')}>Check team status</Button>
        {run.swarm.state === 'ready' ? <Button variant="tertiary" disabled={!canEdit || busy} onClick={() => void operate('Pausing new starts', epoch => control('pause', epoch), 'edit')}>Pause new starts</Button> : run.swarm.state === 'paused' ? <Button variant="secondary" disabled={!canEdit || !canExecute || busy || blocked || !capability.allowed} onClick={() => void operate('Enabling new starts', epoch => control('resume', epoch))}>Resume new starts</Button> : null}
        {run.swarm.state !== 'cancelled' && <Button variant="tertiary" disabled={!canEdit || busy} onClick={() => setDialog('cancel')}>Cancel remaining tasks</Button>}
      </div>
      <SwarmRunQueue key={`${run.projectId}:${run.id}`} run={run} canEdit={canEdit} canExecute={canExecute} busy={busy} onBusy={onBusy} onQueueChange={queueChanged} />
      <ol className="project-swarm-tasks">{tasks.map(task => {
        const operation = operationFor(run, task), reason = waitingReason(run, task);
        const uncertainStart = uncertain.current.has(task.id);
        const resumable = operation?.status === 'reserved' && operation.allocation === 'ready';
        const queuedReservation = !!operation && resumable && queuedOperations.includes(operation.id);
        const retryable = operation && !runOperationNeedsRecovery(operation) && ['failed', 'cancelled'].includes(operation.status);
        return <li key={task.id}>
          <div className="project-swarm-heading"><h4>{task.title}</h4><span>{taskStatus(run, task, capability.allowed)}</span></div>
          {reason && <p>{reason}</p>}
          {queuedTasks.includes(task.id) && !operation && <p>Authorized in the background queue above. Luci will start it when its dependencies and capacity allow.</p>}
          {queuedReservation && <p>This reservation belongs to the background queue. Use its queue controls; if it cannot continue, retain its output and authorize a fresh attempt.</p>}
          {operation?.message && <p className="journey-muted">{operation.message}</p>}
          {runOperationNeedsReview(operation) && !integrated(task) && <p>The task worker exited. Inspect its retained output and review the changes; its exit does not establish correctness.</p>}
          {uncertainStart && <p role="status">The start result is unconfirmed. Check team status before continuing this task.</p>}
          <details><summary>Task scope and handoff</summary>
            <p>{task.acceptance || 'No task-specific acceptance recorded. The build’s linked requirements still apply.'}</p>
            <p>Intended files: {task.writePaths.length ? task.writePaths.join(', ') : 'Not specified; this task runs without other active tasks.'}</p>
            <p>Dependencies: {task.dependsOn.length ? task.dependsOn.map(id => tasks.find(item => item.id === id)?.title || id).join('; ') : 'None'}</p>
            {operation && <p>Attempt {operation.attempt} · workspace {operation.worktreeId || 'being prepared'} · session {operation.childId}</p>}
            {integrated(task) && <p>Integrated commit <code>{task.integration!.commit}</code>. Integration records the reviewed change; it does not verify the application.</p>}
            {task.integration && !integrated(task) && <p>Retained integration attempt: {task.integration.state}. Its outcome has not been confirmed.</p>}
            {run.operations.filter(item => item.taskId === task.id && item.id !== task.operationId).map(item => <p key={item.id}>Retained attempt {item.attempt} · {item.status} · {item.worktreeId}. <Button variant="tertiary" disabled={busy} onClick={() => setWorkspace(item)}>Inspect retained attempt {item.attempt}</Button></p>)}
          </details>
          <div className="journey-actions">
            {!task.integration && (!operation || resumable || retryable) && <Button variant="secondary" disabled={!canEdit || !canExecute || busy || blocked || !capability.allowed || !!reason || uncertainStart || queuedTasks.includes(task.id) || queuedReservation} onClick={() => void operate('Starting task', epoch => startTask(run, task, epoch).then(() => undefined))}>{resumable ? 'Continue reserved task' : retryable ? 'Retry in a new workspace' : 'Start task'}</Button>}
            {operation && !operation.absenceRetirement && operation.allocation === 'ready' && operation.status !== 'reserved' && <Button variant="secondary" disabled={busy} onClick={() => void operate('Opening task session', () => onOpenSession(operation), 'read')}>Open task session</Button>}
            {operation && <Button variant="tertiary" disabled={busy} onClick={() => setWorkspace(operation)}>Inspect workspace output</Button>}
            {operation && !inFlight(operation) && !task.integration && <Button variant="secondary" disabled={busy} onClick={() => void operate('Reading task changes', epoch => readReview(task, epoch), 'read')}>Review task changes</Button>}
            {operation && !operation.absenceRetirement?.finalizedAt && !integrated(task) && (task.integration || operation.allocation === 'unknown' || ['reserved', 'dispatching', 'unknown', 'failed', 'cancelled'].includes(operation.status)) && <Button variant="tertiary" disabled={busy || !canEdit} onClick={() => setRetire(task.id)}>{runOperationNeedsRecovery(operation) ? 'Finish recovery' : 'Retain changes and retry'}</Button>}
          </div>
        </li>;
      })}</ol>
      {tasks.length > 0 && tasks.every(integrated) && <p role="status">All task changes are integrated. Verify this build against its reviewed requirements before accepting the result.</p>}
    </>}
    {dialog === 'prepare' && <WorkflowDialog title="Prepare agent team" onClose={() => setDialog(null)} busy={busy} description="Use this build’s saved tasks and model connection. Preparation records the team; it does not start agents.">
      <p>Saved permission: {capability.mode}. Task commands run without interactive approvals inside isolated Docker workspaces. Each task exits after its response; review and integration remain your choice.</p>
      {capability.notice && <p>{capability.notice}</p>}
      <label className="luci-field">Maximum parallel agents<select value={selectedLimit} disabled={busy || subscription} onChange={event => setLimit(Number(event.target.value))}>{(subscription ? [1] : [1, 2, 3, 4]).map(value => <option key={value} value={value}>{value}</option>)}</select></label>
      {subscription && <p>This saved Codex subscription connection supports one active agent at a time. To run independent tasks in parallel, prepare a new build with a compatible API or local-model connection.</p>}
      <p>Each active agent uses memory, disk and provider usage. Workspace and provider limits may keep additional tasks queued. This limit controls concurrency, not provider billing.</p>
      <p>Commit the starting project first. Include each selected task’s dependencies in this build. Conflicting or unspecified file scopes are scheduled separately.</p>
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={() => setDialog(null)}>Keep reviewing</Button><Button disabled={busy || !canEdit || !canExecute || blocked || !capability.allowed} onClick={() => void operate('Preparing agent team', async epoch => { if (!current.current.taskAllowed) return; const value = await projectSwarmApi.prepare(run.projectId, run.id, run.revision, selectedLimit); if (remember(value, epoch)) { setDialog(null); setNotice('Agent team prepared. Review the task scopes, then start the ready tasks.'); } })}>Prepare team</Button></div>
    </WorkflowDialog>}
    {dialog === 'cancel' && <WorkflowDialog title="Cancel remaining tasks" onClose={() => setDialog(null)} busy={busy} description="Prevent further task starts for this team. Existing agents keep running until you stop them in their sessions. Saved changes and integration history remain available.">
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={() => setDialog(null)}>Keep team</Button><Button disabled={busy || !canEdit} onClick={() => void operate('Cancelling new starts', epoch => control('cancel', epoch), 'edit')}>Cancel future starts</Button></div>
    </WorkflowDialog>}
    {review && <WorkflowDialog title="Review task changes" onClose={() => setReview(null)} busy={busy} description="Review the exact committed task changes before integrating them into the main project. The server checks the task and target again before changing files.">
      <p>{tasks.find(task => task.id === review.taskId)?.title}</p>
      <p className="project-swarm-commits">Task commit <code>{review.sourceCommit || 'unavailable'}</code><br />Target commit <code>{review.targetCommit || 'unavailable'}</code></p>
      <p className={review.canIntegrate ? 'journey-muted' : 'journey-warning'}>{review.reason}</p>
      <ul>{review.paths.map(path => <li key={path}><code>{path}</code></li>)}</ul>
      {review.patch && <pre className="project-swarm-patch" tabIndex={0} aria-label="Reviewed task diff">{review.patch}</pre>}
      {review.canIntegrate && <label className="setup-check"><input type="checkbox" checked={approved} disabled={busy || !canEdit} onChange={event => setApproved(event.target.checked)} />I reviewed these changes for integration. I will verify the combined build afterwards.</label>}
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={() => setReview(null)}>Keep reviewing</Button><Button disabled={busy || !canEdit || !canExecute || blocked || !review.canIntegrate || !approved} onClick={() => void operate('Integrating reviewed changes', async epoch => { const selected = review; setReview(null); const value = await projectSwarmApi.integrate(run.projectId, run.id, selected.taskId, run.revision, selected.reviewDigest); if (remember(value, epoch)) setNotice('Reviewed changes integrated. Dependent tasks can now start; the combined build still needs verification.'); })}>Integrate reviewed changes</Button></div>
    </WorkflowDialog>}
    {retire && <WorkflowDialog title={finishingRecovery ? 'Finish recovery' : 'Retain changes and retry'} onClose={() => setRetire(null)} busy={busy} description="Keep this attempt’s workspace and output. Luci will allow retirement only when it can prove the child is stopped or was never allocated. A running process, an in-flight allocation, or an uncertain observation prevents recovery. This does not undo integrated changes or start another agent; a fresh attempt requires execution permission and a separate start.">
      <p>{tasks.find(task => task.id === retire)?.title}</p>
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={() => setRetire(null)}>Keep inspecting</Button><Button disabled={busy || !canEdit} onClick={() => void operate('Verifying and retiring attempt', epoch => retireAttempt(retire, epoch), 'edit')}>{finishingRecovery ? 'Finish recovery' : 'Retain output and allow retry'}</Button></div>
    </WorkflowDialog>}
    {workspace && <SwarmWorkspaceReview key={workspace.id} projectId={run.projectId} runId={run.id} operation={workspace} canRead onClose={() => setWorkspace(null)} />}
  </section>;
}

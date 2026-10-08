import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { errorText } from '../../api/product';
import { projectRunsApi, runSessionRequest, type ProjectRun, type RunOperation, type SwarmTask } from '../../api/projectRuns';
import { projectSwarmApi, type SwarmReview } from '../../api/projectSwarm';
import { useCockpit } from '../../store/store';
import { Button } from '../fluid/components/button';
import { WorkflowDialog } from '../common/WorkflowDialog';
import { SwarmWorkspaceReview } from './SwarmWorkspaceReview';
import './project-swarm.css';

const inFlight = (operation?: RunOperation) => !!operation && (['reserved', 'dispatching', 'running', 'unknown'].includes(operation.status) || operation.allocation === 'allocating' || operation.allocation === 'unknown');
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
function taskStatus(run: ProjectRun, task: SwarmTask) {
  if (integrated(task)) return 'Integrated';
  if (task.integration) return 'Integration outcome unconfirmed';
  const operation = operationFor(run, task);
  if (operation?.allocation === 'unknown') return 'Workspace outcome unconfirmed';
  if (operation?.allocation === 'allocating') return 'Preparing workspace';
  if (operation?.status === 'reserved') return 'Reserved · not started';
  if (operation?.status === 'running') return 'Recorded as running';
  if (operation?.status === 'dispatching' || operation?.status === 'unknown') return 'Start outcome unconfirmed';
  if (operation?.status === 'succeeded') return 'Ready for change review';
  if (operation?.status === 'failed' || operation?.status === 'cancelled') return 'Stopped · inspect before retry';
  return waitingReason(run, task) ? 'Waiting' : 'Ready to start';
}

interface Props {
  run: ProjectRun; canEdit: boolean; canExecute: boolean; busy: boolean;
  onBusy(label: string): void; onUpdate(run: ProjectRun): void;
  onOpenSession(operation: RunOperation): Promise<void>;
}

/** One reviewed run owns the task team; rendering and polling never dispatch it. */
export function ProjectSwarm({run, canEdit, canExecute, busy, onBusy, onUpdate, onOpenSession}: Props) {
  const [dialog, setDialog] = useState<'prepare' | 'cancel' | null>(null);
  const [limit, setLimit] = useState(2), [review, setReview] = useState<SwarmReview | null>(null), [approved, setApproved] = useState(false);
  const [retire, setRetire] = useState<string | null>(null);
  const [workspace, setWorkspace] = useState<RunOperation | null>(null);
  const [error, setError] = useState(''), [notice, setNotice] = useState('');
  const live = useRef(true), working = useRef(false), generation = useRef(0);
  const current = useRef({run, canEdit, canExecute}); current.current = {run, canEdit, canExecute};
  const authority = useRef(`${canEdit}:${canExecute}`);
  if (authority.current !== `${canEdit}:${canExecute}`) { authority.current = `${canEdit}:${canExecute}`; generation.current++; }
  const keys = useRef(new Map<string, string>()), uncertain = useRef(new Set<string>());
  const [, setUncertainVersion] = useState(0);
  useEffect(() => { live.current = true; return () => { live.current = false; generation.current++; }; }, []);
  useEffect(() => { setReview(null); setApproved(false); }, [run.revision, canEdit]);
  const usable = (epoch: number) => live.current && generation.current === epoch;
  const blocked = !!run.codingBlocker || !!run.contextFreshness && run.contextFreshness.status !== 'current' || ['cancelled', 'complete'].includes(run.state);
  const subscription = run.input.route.connection.kind === 'codex-subscription' || run.input.route.connection.authMode === 'chatgpt';
  const selectedLimit = subscription ? 1 : limit;
  const tasks = run.swarm?.tasks || [], active = tasks.filter(task => inFlight(operationFor(run, task))).length;
  const ready = tasks.filter(task => !task.integration && !task.operationId && !waitingReason(run, task));
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
    const value = await projectSwarmApi.control(run.projectId, run.id, action, run.revision);
    if (!remember(value, epoch)) return;
    uncertain.current.clear(); setUncertainVersion(value => value + 1); setDialog(null);
    setNotice(action === 'reconcile' ? 'Existing task sessions inspected. No new agent was started.' : action === 'resume' ? 'New starts enabled. Choose the ready tasks to continue.' : 'Future task starts stopped. Existing sessions keep their own stop controls.');
  }
  async function startTask(saved: ProjectRun, task: SwarmTask, epoch: number): Promise<ProjectRun | undefined> {
    if (!usable(epoch) || !current.current.canEdit || !current.current.canExecute) return;
    let operation = operationFor(saved, task);
    if (!operation || !inFlight(operation)) {
      const keyScope = `${saved.id}:${task.id}:${saved.revision}`;
      let key = keys.current.get(keyScope);
      if (!key) { key = crypto.randomUUID(); keys.current.set(keyScope, key); }
      const reservation = await projectSwarmApi.reserve(saved.projectId, saved.id, task.id, saved.revision, key);
      if (!remember(reservation.run, epoch)) return;
      saved = reservation.run; operation = reservation.operation;
    }
    if (!usable(epoch) || !current.current.canExecute || !current.current.canEdit) return;
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
  async function readReview(task: SwarmTask, epoch: number) {
    const value = await projectSwarmApi.review(run.projectId, run, task.id);
    if (!usable(epoch) || current.current.run.revision !== run.revision) return;
    setReview(value); setApproved(false);
  }
  return <section className="project-swarm" aria-label="Agent team">
    <div className="project-swarm-heading"><h3>Agent team</h3>{run.swarm && <span>{tasks.filter(integrated).length} of {tasks.length} integrated</span>}</div>
    {error && <p role="alert" className="journey-error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    {!run.swarm ? <>
      <p>Split the reviewed tasks between agents. Each gets its own workspace and focused context; dependent work waits for reviewed changes to be integrated.</p>
      {run.operations.length ? <p className="journey-muted">This build already has coding attempts. Prepare a new reviewed build to use an agent team.</p> : <Button variant="secondary" disabled={!canEdit || !canExecute || busy || blocked} onClick={() => setDialog('prepare')}>Prepare agent team</Button>}
    </> : <>
      <p>{run.swarm.state === 'ready' ? 'New starts enabled' : run.swarm.state === 'paused' ? 'New starts paused' : 'Future starts cancelled'} · {active} of {run.swarm.maxParallel} agent slots occupied</p>
      <p className="journey-muted">Status is recorded, not a live process guarantee. Check team status after an interruption. Pausing future starts leaves existing agents running.</p>
      <div className="journey-actions">
        <Button disabled={!canEdit || !canExecute || busy || blocked || !batchReady.length} onClick={() => void operate('Starting ready tasks', startReady)}>Start ready tasks{batchReady.length ? ` (${batchReady.length})` : ''}</Button>
        <Button variant="secondary" disabled={!canEdit || busy} onClick={() => void operate('Checking team status', epoch => control('reconcile', epoch), 'edit')}>Check team status</Button>
        {run.swarm.state === 'ready' ? <Button variant="tertiary" disabled={!canEdit || busy} onClick={() => void operate('Pausing new starts', epoch => control('pause', epoch), 'edit')}>Pause new starts</Button> : run.swarm.state === 'paused' ? <Button variant="secondary" disabled={!canEdit || !canExecute || busy || blocked} onClick={() => void operate('Enabling new starts', epoch => control('resume', epoch))}>Resume new starts</Button> : null}
        {run.swarm.state !== 'cancelled' && <Button variant="tertiary" disabled={!canEdit || busy} onClick={() => setDialog('cancel')}>Cancel remaining tasks</Button>}
      </div>
      <ol className="project-swarm-tasks">{tasks.map(task => {
        const operation = operationFor(run, task), reason = waitingReason(run, task);
        const uncertainStart = uncertain.current.has(task.id);
        const resumable = operation?.status === 'reserved' && operation.allocation === 'ready';
        const retryable = operation && ['failed', 'cancelled'].includes(operation.status);
        return <li key={task.id}>
          <div className="project-swarm-heading"><h4>{task.title}</h4><span>{taskStatus(run, task)}</span></div>
          {reason && <p>{reason}</p>}
          {operation?.message && <p className="journey-muted">{operation.message}</p>}
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
            {!task.integration && (!operation || resumable || retryable) && <Button variant="secondary" disabled={!canEdit || !canExecute || busy || blocked || !!reason || uncertainStart} onClick={() => void operate('Starting task', epoch => startTask(run, task, epoch).then(() => undefined))}>{resumable ? 'Continue reserved task' : retryable ? 'Retry in a new workspace' : 'Start task'}</Button>}
            {operation && operation.allocation === 'ready' && operation.status !== 'reserved' && <Button variant="secondary" disabled={busy} onClick={() => void operate('Opening task session', () => onOpenSession(operation), 'read')}>Open task session</Button>}
            {operation && <Button variant="tertiary" disabled={busy} onClick={() => setWorkspace(operation)}>Inspect workspace output</Button>}
            {operation && !inFlight(operation) && !task.integration && <Button variant="secondary" disabled={busy} onClick={() => void operate('Reading task changes', epoch => readReview(task, epoch), 'read')}>Review task changes</Button>}
            {operation && !integrated(task) && (task.integration || operation.allocation === 'unknown' || ['failed', 'cancelled'].includes(operation.status)) && <Button variant="tertiary" disabled={busy || !canEdit || !canExecute} onClick={() => setRetire(task.id)}>Retain changes and retry</Button>}
          </div>
        </li>;
      })}</ol>
      {tasks.length > 0 && tasks.every(integrated) && <p role="status">All task changes are integrated. Verify this build against its reviewed requirements before accepting the result.</p>}
    </>}
    {dialog === 'prepare' && <WorkflowDialog title="Prepare agent team" onClose={() => setDialog(null)} busy={busy} description="Use this build’s saved tasks and model connection. Preparation records the team; it does not start agents.">
      <label className="luci-field">Maximum parallel agents<select value={selectedLimit} disabled={busy || subscription} onChange={event => setLimit(Number(event.target.value))}>{(subscription ? [1] : [1, 2, 3, 4]).map(value => <option key={value} value={value}>{value}</option>)}</select></label>
      {subscription && <p>This saved Codex subscription connection supports one active agent at a time. To run independent tasks in parallel, prepare a new build with a compatible API or local-model connection.</p>}
      <p>Each active agent uses local memory, disk and provider usage. Start with two; the limit controls concurrency, not provider billing.</p>
      <p>Commit the starting project first. Include each selected task’s dependencies in this build. Conflicting or unspecified file scopes are scheduled separately.</p>
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={() => setDialog(null)}>Keep reviewing</Button><Button disabled={busy || !canEdit || !canExecute || blocked} onClick={() => void operate('Preparing agent team', async epoch => { const value = await projectSwarmApi.prepare(run.projectId, run.id, run.revision, selectedLimit); if (remember(value, epoch)) { setDialog(null); setNotice('Agent team prepared. Review the task scopes, then start the ready tasks.'); } })}>Prepare team</Button></div>
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
    {retire && <WorkflowDialog title="Retain changes and retry" onClose={() => setRetire(null)} busy={busy} description="Keep the previous workspace and its output, and retire this stopped attempt. This does not undo changes already in the main project or start another agent. A new attempt gets a fresh workspace after you explicitly start it. Active or unconfirmed agent processes must be stopped and checked first.">
      <p>{tasks.find(task => task.id === retire)?.title}</p>
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={() => setRetire(null)}>Keep inspecting</Button><Button disabled={busy || !canEdit || !canExecute} onClick={() => void operate('Retiring stopped attempt', async epoch => { const value = await projectSwarmApi.abandon(run.projectId, run.id, retire, run.revision); if (remember(value, epoch)) { setRetire(null); setNotice('Previous changes retained. Start the task explicitly when ready to retry.'); } })}>Retain output and allow retry</Button></div>
    </WorkflowDialog>}
    {workspace && <SwarmWorkspaceReview key={workspace.id} projectId={run.projectId} runId={run.id} operation={workspace} canRead onClose={() => setWorkspace(null)} />}
  </section>;
}

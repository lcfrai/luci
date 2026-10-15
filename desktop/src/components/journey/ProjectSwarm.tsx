import { useEffect, useRef, useState } from 'react';
import { api } from '../../api';
import { ApiError } from '../../api/client';
import { errorText } from '../../api/product';
import { projectRunsApi, runOperationNeedsRecovery, runOperationNeedsReview, runSessionRequest, type ProjectRun, type RunOperation, type SwarmTask } from '../../api/projectRuns';
import { swarmTaskCapability } from '../../api/swarmTaskCapability';
import { MAX_SWARM_PARALLEL_TASKS } from '../../api/swarmLimits';
import { projectSwarmApi, type SwarmReview, type SwarmRecoveryAvailability } from '../../api/projectSwarm';
import { useCockpit } from '../../store/store';
import { Button } from '../fluid/components/button';
import { Input } from '../ui/input';
import { WorkflowDialog } from '../common/WorkflowDialog';
import { SwarmWorkspaceReview } from './SwarmWorkspaceReview';
import { SwarmCheckpointDialog } from './SwarmCheckpointDialog';
import { SwarmTaskHandoff } from './SwarmTaskHandoff';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { swarmCheckpointApi, type SwarmCheckpointReview } from '../../api/swarmCheckpoint';
import { SwarmRunQueue } from './SwarmRunQueue';
import { SwarmDependencyNavigator } from '../develop/SwarmDependencyNavigator';
import { SwarmSupervisionView } from '../develop/SwarmSupervisionView';
import { SwarmLaunchRecoveryDialog, launchRecoveryStatus } from './SwarmLaunchRecoveryDialog';
import type { ProjectSwarmQueue } from '../../api/swarmQueue';
import './project-swarm.css';

type IntegrationReview = SwarmReview & { focus: {opener: HTMLButtonElement; epoch: number; projectId: string; runId: string; revision: number} };

const inFlight = (operation?: RunOperation) => !!operation && (runOperationNeedsRecovery(operation) || ['reserved', 'dispatching', 'running', 'unknown'].includes(operation.status) || operation.allocation === 'allocating' || operation.allocation === 'unknown');
const integrated = (task: SwarmTask) => task.integration?.state === 'integrated';
const operationFor = (run: ProjectRun, task: SwarmTask) => run.operations.find(item => item.id === task.operationId);
const startIdentity = (run: ProjectRun, operation: RunOperation) => JSON.stringify([run.projectId, run.id, operation.id, operation.childId]);
const awaitsObservation = (operation: RunOperation) => {
  if (operation.startFailure || !operation.taskId || !operation.childId || !operation.worktreeId || operation.allocation !== 'ready') return false;
  if (['dispatching', 'running', 'unknown'].includes(operation.status)) return true;
  const modern = /^[a-f0-9]{64}$/.test(operation.dependencyContext?.digest ?? operation.implementation?.dependencyContextDigest ?? '');
  // Cancellation can retire an unstarted reservation with no registry child;
  // there will be no automatic completion report to wait for in that case.
  return modern && !runOperationNeedsRecovery(operation) && ['review-required', 'succeeded', 'failed'].includes(operation.status) && !operation.handoff;
};
const unansweredQuestions = (operation?: RunOperation) => {
  const value = operation?.handoff;
  return value?.status === 'captured' && Number.isSafeInteger(value.questions) && value.questions >= 0 && value.questions <= 8 &&
    Number.isSafeInteger(value.unanswered) && value.unanswered > 0 && value.unanswered <= value.questions ? value.unanswered : 0;
};
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
function taskStatus(run: ProjectRun, task: SwarmTask, taskAllowed: boolean, startAcknowledged = false) {
  if (integrated(task)) return 'Integrated';
  if (task.integration) return 'Integration outcome unconfirmed';
  const operation = operationFor(run, task);
  if (operation?.startFailure && operation.status === 'failed') return 'Start blocked';
  if (operation?.recovery?.state === 'pending') return 'Launch recovery pending · capacity held';
  if (operation?.recovery?.state === 'fenced') return 'Old launch blocked · capacity release pending';
  if (operation?.recovery?.state === 'finalized') return 'Attempt retired · output retained';
  if (runOperationNeedsRecovery(operation)) return 'Recovery pending · finish recovery to release capacity';
  if (operation?.startFailure && operation.status === 'cancelled') return 'Attempt retained';
  if (operation?.allocation === 'unknown') return 'Workspace outcome unconfirmed';
  if (operation?.allocation === 'allocating') return 'Preparing workspace';
  if (startAcknowledged && operation && ['reserved', 'dispatching', 'unknown'].includes(operation.status)) return 'Started · current status unconfirmed';
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
  const projectAccess = useProjectAccess(run.projectId);
  const harnesses = useCockpit(state => state.harnesses);
  const projectName = useCockpit(state => state.projects.find(project => project.id === run.projectId)?.name) || run.projectId;
  const capability = swarmTaskCapability(run, harnesses);
  const [dialog, setDialog] = useState<'prepare' | 'cancel' | null>(null);
  const [limit, setLimit] = useState(2), [review, setReview] = useState<IntegrationReview | null>(null), [approved, setApproved] = useState(false);
  const [retire, setRetire] = useState<string | null>(null);
  const [launchRecovery, setLaunchRecovery] = useState<{projectId: string; runId: string; taskId: string; operationId: string; childId: string; revision: number; epoch: number} | null>(null);
  const [checkpoint, setCheckpoint] = useState<{projectId: string; runId: string; taskId: string; operationId: string; childId: string; revision: number; epoch: number} | null>(null);
  const taskRows = useRef(new Map<string, HTMLLIElement>());
  const [workspace, setWorkspace] = useState<RunOperation | null>(null);
  const [handoff, setHandoff] = useState<{projectId: string; runId: string; operationId: string; childId: string} | null>(null);
  const [queuedTasks, setQueuedTasks] = useState<string[]>([]);
  const [queuedOperations, setQueuedOperations] = useState<string[]>([]);
  const [queueConsent, setQueueConsent] = useState<string | null>(null);
  const queueSignature = useRef('');
  const queueRegion = useRef<HTMLDivElement>(null);
  const queueReviewButton = useRef<HTMLButtonElement>(null);
  const [queueAttention, setQueueAttention] = useState(0);
  const [navigatorQueue, setNavigatorQueue] = useState<{projectId: string; runId: string; value: ProjectSwarmQueue} | null>(null);
  const [supervisionOpen, setSupervisionOpen] = useState(false), [supervisionMounted, setSupervisionMounted] = useState(false);
  const [error, setError] = useState(''), [notice, setNotice] = useState('');
  const [readNotice, setReadNotice] = useState(''), [readCycle, setReadCycle] = useState(0);
  useEffect(() => { setReadNotice(''); }, [run.projectId, run.id]);
  const live = useRef(true), working = useRef(false), generation = useRef(0);
  const backgroundRead = useRef<{controller: AbortController; epoch: number} | null>(null);
  const readEpoch = useRef(0);
  useEffect(() => { live.current = true; return () => { live.current = false; generation.current++; backgroundRead.current?.controller.abort(); }; }, []);
  const current = useRef({run, canEdit, canExecute, taskAllowed: capability.allowed}); current.current = {run, canEdit, canExecute, taskAllowed: capability.allowed};
  const authorityKey = `${run.projectId}:${run.id}:${canEdit}:${canExecute}:${capability.signature}`;
  const authority = useRef(authorityKey);
  if (authority.current !== authorityKey) { authority.current = authorityKey; generation.current++; }
  const queueScope = JSON.stringify([run.projectId, run.id]);
  const keys = useRef(new Map<string, string>()), uncertain = useRef(new Set<string>());
  const acknowledgedStarts = useRef(new Set<string>());
  const [, setUncertainVersion] = useState(0);
  const readReady = projectAccess.canRead && !projectAccess.loading && !projectAccess.error;
  const pauseReads = busy || !!dialog || !!review || !!retire || !!launchRecovery || !!checkpoint || !!workspace || !!handoff || queueConsent === queueScope;
  const readBoundary = JSON.stringify([run.projectId, run.id, readReady, pauseReads, canEdit, canExecute, capability.signature]);
  const previousReadBoundary = useRef(readBoundary);
  if (previousReadBoundary.current !== readBoundary) { previousReadBoundary.current = readBoundary; readEpoch.current++; }
  const readState = useRef({readReady, pauseReads}); readState.current = {readReady, pauseReads};
  const observing = !!run.swarm && run.operations.some(awaitsObservation);
  useEffect(() => {
    if (!readReady || pauseReads || !observing) return;
    let stopped = false, denied = false;
    let timer: ReturnType<typeof setTimeout> | undefined;
    const epoch = generation.current, boundary = readEpoch.current;
    const usableRead = () => !stopped && live.current && generation.current === epoch && readEpoch.current === boundary &&
      readState.current.readReady && !readState.current.pauseReads && !working.current;
    const schedule = () => { clearTimeout(timer); if (usableRead() && !denied && document.visibilityState !== 'hidden') timer = setTimeout(() => void refresh(), 5000); };
    async function refresh() {
      if (!usableRead() || document.visibilityState === 'hidden') return;
      if (backgroundRead.current) { schedule(); return; }
      const controller = new AbortController(); backgroundRead.current = {controller, epoch: boundary};
      try {
        const value = await projectRunsApi.get(run.projectId, run.id, controller.signal);
        if (!usableRead() || controller.signal.aborted) return;
        if (value.revision > current.current.run.revision) remember(value, epoch);
        setReadNotice('');
      } catch (cause) {
        if (!usableRead() || controller.signal.aborted) return;
        denied = cause instanceof ApiError && (cause.status === 401 || cause.status === 403);
        setReadNotice(denied ? 'Automatic status refresh was refused. Refresh project access before checking task status.'
          : 'Automatic status refresh is unavailable. Showing the last recorded status; use Check team status to inspect it explicitly.');
      } finally {
        if (backgroundRead.current?.controller === controller) backgroundRead.current = null;
        schedule();
      }
    }
    const visibility = () => {
      clearTimeout(timer);
      if (document.visibilityState === 'hidden') backgroundRead.current?.controller.abort();
      else schedule();
    };
    schedule(); document.addEventListener('visibilitychange', visibility);
    return () => { stopped = true; clearTimeout(timer); backgroundRead.current?.controller.abort(); document.removeEventListener('visibilitychange', visibility); };
  }, [run.projectId, run.id, readReady, pauseReads, observing, canEdit, canExecute, capability.signature, readCycle]);
  useEffect(() => { setReview(null); setApproved(false); setRetire(null); setLaunchRecovery(null); setCheckpoint(null); setDialog(null); }, [run.projectId, run.id, run.revision, canEdit, canExecute, capability.signature]);
  const usable = (epoch: number) => live.current && generation.current === epoch;
  const blocked = !!run.codingBlocker || !!run.contextFreshness && run.contextFreshness.status !== 'current' || ['cancelled', 'complete'].includes(run.state);
  const subscription = run.input.route.connection.kind === 'codex-subscription' || run.input.route.connection.authMode === 'chatgpt';
  const selectedLimit = subscription ? 1 : limit;
  const validLimit = Number.isInteger(selectedLimit) && selectedLimit >= 1 && selectedLimit <= MAX_SWARM_PARALLEL_TASKS;
  const tasks = run.swarm?.tasks || [], active = tasks.filter(task => inFlight(operationFor(run, task))).length;
  const retiringTask = tasks.find(task => task.id === retire);
  const finishingRecovery = !!retiringTask && runOperationNeedsRecovery(operationFor(run, retiringTask));
  // Render guards discard old consent before effects run, including revoke/regrant.
  const recoveryTask = launchRecovery && canEdit && launchRecovery.epoch === generation.current &&
    launchRecovery.projectId === run.projectId && launchRecovery.runId === run.id && launchRecovery.revision === run.revision
    ? tasks.find(task => task.id === launchRecovery.taskId && task.operationId === launchRecovery.operationId) : undefined;
  const recoveryOperation = recoveryTask && operationFor(run, recoveryTask);
  const checkpointTask = checkpoint && canEdit && canExecute && checkpoint.epoch === generation.current &&
    checkpoint.projectId === run.projectId && checkpoint.runId === run.id && checkpoint.revision === run.revision
    ? tasks.find(task => task.id === checkpoint.taskId && task.operationId === checkpoint.operationId) : undefined;
  const checkpointOperation = checkpointTask && operationFor(run, checkpointTask);
  const handoffOperation = handoff && projectAccess.canRead && handoff.projectId === run.projectId && handoff.runId === run.id
    ? run.operations.find(item => item.id === handoff.operationId && item.childId === handoff.childId && item.taskId) : undefined;
  const handoffTask = handoffOperation && tasks.find(task => task.id === handoffOperation.taskId);
  const reviewTasks = tasks.filter(task => !integrated(task) && runOperationNeedsReview(operationFor(run, task)));
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
    readEpoch.current++; backgroundRead.current?.controller.abort();
    working.current = true; onBusy(label); setError(''); setNotice('');
    try { await action(epoch); }
    catch (cause) { if (usable(epoch)) setError(errorText(cause)); }
    finally { working.current = false; if (live.current) { onBusy(''); setReadCycle(value => value + 1); } }
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
    if (operation.status !== 'reserved' || operation.allocation !== 'ready' || acknowledgedStarts.current.has(startIdentity(saved, operation))) throw new Error('Check team status before starting this reserved task. An uncertain start is never replayed.');
    uncertain.current.add(task.id); setUncertainVersion(value => value + 1);
    const session = await api.createSession(runSessionRequest(saved, operation));
    if (!usable(epoch)) return;
    if (session.id !== operation.childId || session.worktreeId !== operation.worktreeId || session.projectId !== saved.projectId || session.projectRunId !== saved.id || session.projectRunOperationId !== operation.id) throw new Error('The session did not match its task reservation. Check team status before continuing.');
    useCockpit.getState().upsertSession(session);
    acknowledgedStarts.current.add(startIdentity(saved, operation));
    uncertain.current.delete(task.id); setUncertainVersion(value => value + 1);
    try {
      // Session creation records dispatch intent. A read alone cannot join its
      // registry observation into the run; reconcile once using a fresh CAS.
      const latest = await projectRunsApi.get(saved.projectId, saved.id);
      if (!usable(epoch)) return;
      if (latest.revision < current.current.run.revision) throw new Error('A newer build revision is already visible.');
      if (!remember(latest, epoch)) return;
      const value = await projectSwarmApi.control(saved.projectId, saved.id, 'reconcile', latest.revision);
      if (!usable(epoch)) return;
      if (value.revision < current.current.run.revision) throw new Error('A newer build revision is already visible.');
      if (!remember(value, epoch)) return;
      const observedTask = value.swarm?.tasks.find(item => item.id === task.id);
      const observed = observedTask && operationFor(value, observedTask);
      if (!observed || observed.id !== operation.id || observed.childId !== operation.childId || ['reserved', 'dispatching', 'unknown'].includes(observed.status)) {
        setNotice('Task started; its current status is still unconfirmed. Check team status before continuing. No start was repeated.');
        return;
      }
      setNotice('Task session started in its isolated workspace. Its recorded status is shown below. Review its changes before integration, then verify the complete build.');
      return value;
    } catch (cause) {
      if (usable(epoch)) setError(`Task started; current status could not be checked. Check team status before continuing. No start was repeated. ${errorText(cause)}`);
      // The validated child remains in the session store. Stop this batch;
      // neither a lost read nor a stale reconciliation authorizes a replay.
      return;
    }
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
    setNavigatorQueue({projectId: run.projectId, runId: run.id, value: queue});
    const items = queue.items.filter(item => item.runId === run.id);
    setQueueAttention(items.filter(item => ['blocked', 'unknown', 'failed'].includes(item.state)).length);
    const pending = items.filter(item => !['cancelled', 'completed', 'failed'].includes(item.state)).map(item => item.taskId);
    setQueuedTasks(previous => previous.length === pending.length && previous.every((id, i) => id === pending[i]) ? previous : pending);
    const owned = items.flatMap(item => item.operationId ? [item.operationId] : []);
    setQueuedOperations(previous => previous.length === owned.length && previous.every((id, i) => id === owned[i]) ? previous : owned);
    const signature = JSON.stringify(items.map(item => [item.id, item.operationId, item.childId, item.state]));
    if (signature === queueSignature.current) return;
    if (busy || working.current || readState.current.pauseReads || !readState.current.readReady || backgroundRead.current || !items.some(item => item.operationId)) return;
    queueSignature.current = signature;
    const epoch = generation.current, boundary = readEpoch.current, controller = new AbortController();
    backgroundRead.current = {controller, epoch: boundary};
    void projectRunsApi.get(run.projectId, run.id, controller.signal).then(value => {
      if (usable(epoch) && boundary === readEpoch.current && !controller.signal.aborted && !working.current && !readState.current.pauseReads && value.revision >= current.current.run.revision) remember(value, epoch);
      else if (queueSignature.current === signature) queueSignature.current = '';
    }).catch(() => { if (queueSignature.current === signature) queueSignature.current = ''; })
      .finally(() => { if (backgroundRead.current?.controller === controller) backgroundRead.current = null; });
  }
  async function readReview(task: SwarmTask, epoch: number, opener: HTMLButtonElement) {
    const value = await projectSwarmApi.review(run.projectId, run, task.id);
    if (!usable(epoch) || current.current.run.revision !== run.revision) return;
    setReview({...value, focus: {opener, epoch, projectId: run.projectId, runId: run.id, revision: run.revision}}); setApproved(false);
  }
  function reviewCloseTarget(selected: IntegrationReview): HTMLElement | null {
    const saved = current.current.run, focus = selected.focus;
    if (!usable(focus.epoch) || !readState.current.readReady || saved.projectId !== focus.projectId || saved.id !== focus.runId || saved.revision !== focus.revision) return null;
    const task = saved.swarm?.tasks.find(item => item.id === selected.taskId);
    const row = taskRows.current.get(selected.taskId);
    if (!row?.isConnected || task?.operationId !== selected.operationId) return null;
    if (focus.opener.isConnected && row.contains(focus.opener) && !focus.opener.disabled) return focus.opener;
    return row.querySelector<HTMLElement>('[data-task-heading]');
  }
  function checkpointCloseTarget(selected: NonNullable<typeof checkpoint>, openingControl: HTMLElement | null): HTMLElement | null {
    const saved = current.current.run;
    if (!usable(selected.epoch) || saved.projectId !== selected.projectId || saved.id !== selected.runId) return null;
    const row = taskRows.current.get(selected.taskId);
    if (!row?.isConnected) return null;
    if (openingControl?.isConnected && row.contains(openingControl)) return openingControl;
    const task = saved.swarm?.tasks.find(item => item.id === selected.taskId);
    const successor = task && operationFor(saved, task);
    if (successor?.status === 'reserved' && successor.allocation === 'ready' &&
      successor.checkpoint?.sourceOperationId === selected.operationId && successor.checkpoint.sourceChildId === selected.childId) {
      const start = row.querySelector<HTMLButtonElement>('[data-task-start]');
      if (start && !start.disabled) return start;
    }
    return row.querySelector<HTMLElement>('[data-task-heading]');
  }
  async function continueCheckpoint(selected: SwarmCheckpointReview, idempotencyKey: string, epoch: number) {
    const saved = current.current.run;
    const source = saved.operations.find(item => item.id === selected.operationId);
    if (!usable(epoch) || !current.current.canEdit || !current.current.canExecute || !source ||
      saved.id !== run.id || saved.projectId !== run.projectId || saved.revision !== selected.expectedRevision ||
      saved.swarm?.tasks.find(task => task.id === selected.taskId)?.operationId !== source.id || source.childId !== selected.childId) {
      throw new Error('The retained task review changed. Refresh team status and review it again.');
    }
    try {
      const value = await swarmCheckpointApi.reserve(saved, source, selected, idempotencyKey);
      if (remember(value.run, epoch)) {
        setCheckpoint(null);
        setNotice(value.operation.allocation === 'ready'
          ? 'A new attempt is prepared with the reviewed retained changes. This action did not start an agent. Start the reserved task when ready; integration and verification remain separate.'
          : 'Continuation preparation was recorded. Check team status before starting its agent; an uncertain preparation is never replayed automatically.');
      }
    } catch (cause) {
      // A response can be lost after the durable reservation. Inspect once;
      // never turn a transport failure into another allocation or worker start.
      if (usable(epoch)) {
        try {
          const value = await projectRunsApi.get(saved.projectId, saved.id);
          if (value.revision >= current.current.run.revision) remember(value, epoch);
        } catch { throw new Error(`${errorText(cause)} The reservation could not be refreshed. Check team status before another action.`); }
      }
      throw cause;
    }
  }
  async function retireAttempt(taskId: string, epoch: number, stop?: SwarmRecoveryAvailability) {
    const task = tasks.find(item => item.id === taskId), operation = task && operationFor(run, task);
    if (!operation) throw new Error('This task attempt changed. Refresh and inspect it before continuing.');
    if (stop && (!stop.availability.supported || stop.expectedRevision !== run.revision || stop.operationId !== operation.id || stop.childId !== operation.childId ||
      current.current.run.projectId !== run.projectId || current.current.run.id !== run.id || current.current.run.revision !== run.revision || !usable(epoch) || !current.current.canEdit)) {
      throw new Error('This recovery review changed. Refresh and inspect the exact attempt before continuing.');
    }
    const recoverMissing = operation.allocation === 'ready' && (['dispatching', 'unknown'].includes(operation.status) || !!operation.absenceRetirement);
    try {
      const value = stop
        ? await projectSwarmApi.abandon(run.projectId, run.id, taskId, run.revision, {operationId: operation.id, childId: operation.childId, stopUncertainLaunch: true})
        : recoverMissing
        ? await projectSwarmApi.abandon(run.projectId, run.id, taskId, run.revision, {operationId: operation.id, childId: operation.childId, recoverMissingChild: true})
        : await projectSwarmApi.abandon(run.projectId, run.id, taskId, run.revision);
      if (remember(value, epoch)) {
        setRetire(null); setLaunchRecovery(null);
        const updated = value.operations.find(item => item.id === operation.id);
        setNotice(stop ? updated?.recovery ? launchRecoveryStatus(updated) : 'Recovery response received. Check the recorded attempt before continuing; no replacement was requested.' : 'Attempt retired. Output is retained. No new agent was started.');
      }
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
  function goToQueueReview() {
    const control = queueReviewButton.current;
    const target = control && !control.disabled ? control : queueRegion.current;
    target?.scrollIntoView?.({block:target === control ? 'nearest' : 'start'});
    target?.focus({preventScroll:true});
  }
  function goToTask(id: string) {
    const row = taskRows.current.get(id);
    if (!row?.isConnected) return;
    const scope = row.querySelector<HTMLDetailsElement>('[data-task-scope]');
    if (scope) scope.open = true;
    const heading = row.querySelector<HTMLElement>('[data-task-heading]');
    heading?.scrollIntoView?.({block: 'nearest'});
    heading?.focus({preventScroll: true});
  }
  return <section className="project-swarm" aria-label="Agent team">
    <div className="project-swarm-heading"><h3>Agent team</h3>{run.swarm && <span>{tasks.filter(integrated).length} of {tasks.length} integrated</span>}</div>
    {error && <p role="alert" className="journey-error">{error}</p>}
    {notice && <p role="status">{notice}</p>}
    <p className="project-swarm-permission">{run.input.route.harness} · Permission: {capability.mode}. Commands run without interactive approvals inside Docker; results need review.</p>
    {!capability.allowed && <p role="status" className="journey-warning">{capability.reason}</p>}
    {!capability.allowed && <div className="journey-actions">{onReviewPlan && <Button variant="secondary" disabled={busy || !canEdit} onClick={onReviewPlan}>Review plan and create a new build</Button>}<Button variant="tertiary" disabled={busy} onClick={() => void operate('Refreshing task capability', async epoch => { const values = await api.listHarnesses(); if (usable(epoch)) useCockpit.getState().setHarnesses(values); }, 'read')}>Refresh task capability</Button></div>}
    {!run.swarm ? <>
      <p>Split the reviewed tasks between agents. Each gets its own workspace and focused context; dependent work waits for reviewed changes to be integrated.</p>
      {run.operations.length ? <p className="journey-muted">This build already has coding attempts. Prepare a new reviewed build to use an agent team.</p> : <Button variant="secondary" disabled={!canEdit || !canExecute || busy || blocked || !capability.allowed} onClick={() => setDialog('prepare')}>Prepare agent team</Button>}
    </> : <>
      <p>{run.swarm.state === 'ready' ? capability.allowed ? 'New starts enabled' : 'New starts unavailable for this saved agent configuration' : run.swarm.state === 'paused' ? 'New starts paused' : 'Future starts cancelled'} · {active} of {run.swarm.maxParallel} agent slots occupied</p>

      <div className="project-swarm-summary" aria-label="Task progress">
        <span>{reviewTasks.length} awaiting review · {ready.length} ready to start</span>
        <Button variant="tertiary" onClick={goToQueueReview}>Background queue{queueAttention ? ` · ${queueAttention} need attention` : ''}</Button>
      </div>
      <SwarmDependencyNavigator run={run} queue={navigatorQueue?.projectId === run.projectId && navigatorQueue.runId === run.id ? navigatorQueue.value : undefined} onOpenTask={goToTask} />
      <details className="project-swarm-guidance" onToggle={event => { const open = event.currentTarget.open; setSupervisionOpen(open); if (open) setSupervisionMounted(true); }}>
      <summary>Team messages and supervision</summary>
      {supervisionMounted && <SwarmSupervisionView run={run} canRead={readReady} canEdit={canEdit && readReady} paused={pauseReads || !supervisionOpen} onOpenTask={goToTask}
        onWrite={action => operate('Recording team message', async epoch => {
          if (!readState.current.readReady || !current.current.canEdit) return;
          await action();
          if (usable(epoch) && readState.current.readReady) remember(await projectRunsApi.get(run.projectId, run.id), epoch);
        }, 'edit')} />}
      </details>
      {readNotice && <p role="status" className="journey-warning">{readNotice}</p>}
      <div className="journey-actions">
        <Button disabled={!canEdit || !canExecute || busy || blocked || !capability.allowed || !batchReady.length} onClick={() => void operate('Starting ready tasks', startReady)}>Start ready tasks{batchReady.length ? ` (${batchReady.length})` : ''}</Button>
        <Button variant="secondary" disabled={!canEdit || busy} onClick={() => void operate('Checking team status', epoch => control('reconcile', epoch), 'edit')}>Check team status</Button>
        {run.swarm.state === 'ready' ? <Button variant="tertiary" disabled={!canEdit || busy} onClick={() => void operate('Pausing new starts', epoch => control('pause', epoch), 'edit')}>Pause new starts</Button> : run.swarm.state === 'paused' ? <Button variant="secondary" disabled={!canEdit || !canExecute || busy || blocked || !capability.allowed} onClick={() => void operate('Enabling new starts', epoch => control('resume', epoch))}>Resume new starts</Button> : null}
        {run.swarm.state !== 'cancelled' && <Button variant="tertiary" disabled={!canEdit || busy} onClick={() => setDialog('cancel')}>Cancel remaining tasks</Button>}
      </div>
      <ol className="project-swarm-tasks">{tasks.map(task => {
        const operation = operationFor(run, task), reason = waitingReason(run, task);
        const uncertainStart = uncertain.current.has(task.id);
        const startAcknowledged = !!operation && acknowledgedStarts.current.has(startIdentity(run, operation));
        const acknowledgedUnconfirmed = startAcknowledged && !!operation && ['reserved', 'dispatching', 'unknown'].includes(operation.status);
        const resumable = operation?.status === 'reserved' && operation.allocation === 'ready' && !startAcknowledged;
        const queuedReservation = !!operation && resumable && queuedOperations.includes(operation.id);
        const retryable = operation && !operation.startFailure && !runOperationNeedsRecovery(operation) && ['failed', 'cancelled'].includes(operation.status);
        const retiredStartFailure = operation?.status === 'cancelled' && !runOperationNeedsRecovery(operation) && !task.integration ? operation.startFailure : undefined;
        const retained = run.operations.filter(item => item.taskId === task.id && item.id !== task.operationId);
        const earlierQuestions = retained.reduce((count, item) => count + unansweredQuestions(item), 0);
        return <li key={task.id} data-review-task={reviewTasks.some(item => item.id === task.id) || undefined} ref={element => { if (element) taskRows.current.set(task.id, element); else taskRows.current.delete(task.id); }}>
          <div className="project-swarm-heading"><h4 tabIndex={-1} data-task-heading>{task.title}</h4><span>{taskStatus(run, task, capability.allowed, startAcknowledged)}</span></div>
          {reason && <p>{reason}</p>}
          {queuedTasks.includes(task.id) && !operation && <p>Authorized in the background queue. Luci will start it when its dependencies and capacity allow.</p>}
          {queuedReservation && <p>This reservation belongs to the background queue. Use its queue controls; if it cannot continue, retain its output and authorize a fresh attempt.</p>}
          {operation?.message && !operation.startFailure && !integrated(task) && <p className="journey-muted">{operation.message}</p>}
          {operation?.startFailure && operation.status === 'failed' && <p className="journey-warning">{operation.startFailure.message} Resolve the cause, then retain this attempt and review a new start. No replacement was started.</p>}
          {retiredStartFailure && <><p className="journey-muted">Previous start: {retiredStartFailure.message}</p><p role="status">This attempt is retained. In Background queue, inspect existing work or choose Queue reviewed tasks to review a new start. Retiring this attempt did not start another agent.</p></>}
          {operation?.recovery && <p>{launchRecoveryStatus(operation)}</p>}
          {runOperationNeedsReview(operation) && !integrated(task) && !operation?.message && <p>Review the retained changes before integration, then verify the combined build.</p>}
          {unansweredQuestions(operation) > 0 && <p className="journey-warning">{unansweredQuestions(operation)} agent-reported {unansweredQuestions(operation) === 1 ? 'question needs' : 'questions need'} an answer. Open Task result and questions to review their impact; answering does not start an agent.</p>}
          {earlierQuestions > 0 && <p className="journey-warning">{earlierQuestions} agent-reported {earlierQuestions === 1 ? 'question remains' : 'questions remain'} unanswered in earlier attempts. <Button variant="tertiary" disabled={busy || !projectAccess.canRead} onClick={() => {
            const details = taskRows.current.get(task.id)?.querySelector<HTMLDetailsElement>('[data-task-scope]');
            if (details) { details.open = true; details.querySelector('summary')?.focus(); }
          }}>View earlier attempts</Button></p>}
          {uncertainStart && <p role="status">The start result is unconfirmed. Check team status before continuing this task.</p>}
          <details data-task-scope><summary>Task scope and handoff</summary>
            {integrated(task) && operation?.message && <p className="journey-muted">Worker observation before integration: {operation.message}</p>}
            <p>{task.acceptance || 'No task-specific acceptance recorded. The build’s linked requirements still apply.'}</p>
            <p>Intended files: {task.writePaths.length ? task.writePaths.join(', ') : 'Not specified; this task runs without other active tasks.'}</p>
            <p>Dependencies: {task.dependsOn.length ? task.dependsOn.map(id => tasks.find(item => item.id === id)?.title || id).join('; ') : 'None'}</p>
            {operation && <p>Attempt {operation.attempt} · workspace {operation.worktreeId || 'being prepared'} · session {operation.childId}</p>}
            {operation?.checkpoint && <p>Continues retained operation <code>{operation.checkpoint.sourceOperationId}</code> in a new workspace. Starting checkpoint <code>{operation.checkpoint.seedCommit || 'being prepared'}</code>. This does not mark the task integrated.</p>}
            {integrated(task) && <p>Integrated commit <code>{task.integration!.commit}</code>. Integration records the reviewed change; it does not verify the application.</p>}
            {task.integration && !integrated(task) && <p>Retained integration attempt: {task.integration.state}. Its outcome has not been confirmed.</p>}
            {retained.map(item => <p key={item.id}>Retained attempt {item.attempt} · {item.status} · {item.worktreeId}. {unansweredQuestions(item) > 0 && <strong>{unansweredQuestions(item)} agent-reported {unansweredQuestions(item) === 1 ? 'question remains' : 'questions remain'} unanswered. </strong>}<Button variant="tertiary" disabled={busy} onClick={() => setWorkspace(item)}>Inspect retained attempt {item.attempt}</Button> {!item.startFailure && <Button variant="tertiary" disabled={busy || !projectAccess.canRead} onClick={() => setHandoff({projectId: run.projectId, runId: run.id, operationId: item.id, childId: item.childId})}>Result and questions for attempt {item.attempt}</Button>}</p>)}
          </details>
          <div className="journey-actions">
            {retiredStartFailure && <Button variant="secondary" disabled={busy || !projectAccess.canRead} onClick={goToQueueReview}>Go to queue review</Button>}
            {!task.integration && (!operation || resumable || retryable) && <Button data-task-start variant="secondary" disabled={!canEdit || !canExecute || busy || blocked || !capability.allowed || !!reason || uncertainStart || queuedTasks.includes(task.id) || queuedReservation} onClick={() => void operate('Starting task', epoch => startTask(run, task, epoch).then(() => undefined))}>{resumable ? 'Continue reserved task' : retryable ? 'Retry in a new workspace' : 'Start task'}</Button>}
            {operation && !operation.startFailure && !inFlight(operation) && !task.integration && <Button variant="secondary" disabled={busy} onClick={event => { const opener = event.currentTarget; void operate('Reading task changes', epoch => readReview(task, epoch, opener), 'read'); }}>Review task changes</Button>}
            {operation && !operation.startFailure && <Button variant="tertiary" disabled={busy || !projectAccess.canRead} onClick={() => setHandoff({projectId: run.projectId, runId: run.id, operationId: operation.id, childId: operation.childId})}>Task result and questions</Button>}
            {operation && (operation.status === 'running' || acknowledgedUnconfirmed) && !operation.absenceRetirement && !operation.recovery && operation.allocation === 'ready' && <Button variant="secondary" disabled={busy} onClick={() => void operate('Opening task session', () => onOpenSession(operation), 'read')}>Open task session</Button>}
          </div>
          {operation && <details className="project-swarm-more-actions">
            <summary>More task actions</summary>
            <div className="journey-actions">
            {operation && operation.status !== 'running' && !acknowledgedUnconfirmed && !operation.absenceRetirement && !operation.recovery && operation.allocation === 'ready' && operation.status !== 'reserved' && <Button variant="secondary" disabled={busy} onClick={() => void operate('Opening task session', () => onOpenSession(operation), 'read')}>Open task session</Button>}
            {operation && !operation.startFailure && !task.integration && !inFlight(operation) && operation.allocation === 'ready' && ['failed', 'cancelled', 'review-required', 'succeeded'].includes(operation.status) && <Button variant="secondary" disabled={busy || !canEdit || !canExecute || blocked || !capability.allowed || !!reason || queuedTasks.includes(task.id)} onClick={() => {
              setError(''); setCheckpoint({projectId: run.projectId, runId: run.id, taskId: task.id, operationId: operation.id, childId: operation.childId, revision: run.revision, epoch: generation.current});
            }}>Continue from retained changes</Button>}
            {operation && <Button variant="tertiary" disabled={busy} onClick={() => setWorkspace(operation)}>Inspect workspace output</Button>}
            {operation && !operation.recovery && !operation.absenceRetirement?.finalizedAt && !integrated(task) && (task.integration || operation.allocation === 'unknown' || ['reserved', 'dispatching', 'unknown', 'failed', 'cancelled'].includes(operation.status)) && <Button variant="tertiary" disabled={busy || !canEdit} onClick={() => setRetire(task.id)}>{runOperationNeedsRecovery(operation) ? 'Finish recovery' : 'Retain changes and retry'}</Button>}
            {operation && !task.integration && operation.recovery?.state !== 'finalized' && (operation.recovery || operation.allocation === 'ready' && ['dispatching', 'unknown', 'running', 'failed', 'review-required'].includes(operation.status)) && <Button variant="tertiary" disabled={busy || !canEdit} onClick={() => {
              setError(''); setLaunchRecovery({projectId: run.projectId, runId: run.id, taskId: task.id, operationId: operation.id, childId: operation.childId, revision: run.revision, epoch: generation.current});
            }}>{operation.recovery ? 'Continue launch recovery' : 'Stop uncertain launch and retain output'}</Button>}
            </div>
          </details>}
        </li>;
      })}</ol>
      <div ref={queueRegion} tabIndex={-1} role="region" aria-label="Background queue review" className="project-swarm-queue-region">
      <SwarmRunQueue readAccess={projectAccess} key={`${run.projectId}:${run.id}`} run={run} canEdit={canEdit} canExecute={canExecute} busy={busy} onBusy={onBusy} onQueueChange={queueChanged}
        reviewButtonRef={queueReviewButton}
        onConsentChange={open => setQueueConsent(previous => open ? queueScope : previous === queueScope ? null : previous)} />
      </div>


      {tasks.length > 0 && tasks.every(integrated) && <p role="status">All task changes are integrated. Verify this build against its reviewed requirements before accepting the result.</p>}
    </>}
    <details className="project-swarm-guidance"><summary>Agent task permissions</summary><p>Keep Ask or Plan in an interactive agent if you want approvals during execution.</p><p>Autonomous tasks run commands without interactive approvals inside {run.input.route.executionLocation === 'cloud-microvm' ? 'separate owned cloud microVMs' : 'isolated Docker workspaces'}, then exit after their response. Outputs still need review, integration and verification; Luci does not accept them automatically.</p>{capability.notice && <p>{capability.notice}</p>}<p>Status is recorded, not a live process guarantee. Check team status after an interruption. Pausing future starts leaves existing agents running.</p></details>
    {dialog === 'prepare' && <WorkflowDialog readAccess={projectAccess} title="Prepare agent team" onClose={() => setDialog(null)} busy={busy} description="Use this build’s saved tasks and model connection. Preparation records the team; it does not start agents.">
      <p>Saved permission: {capability.mode}. Task commands run without interactive approvals inside {run.input.route.executionLocation === 'cloud-microvm' ? 'separate owned cloud microVMs' : 'isolated Docker workspaces'}. Each task exits after its response; review and integration remain your choice.</p>
      {capability.notice && <p>{capability.notice}</p>}
      <label className="luci-field">Maximum parallel agents<Input type="number" min={1} max={subscription ? 1 : MAX_SWARM_PARALLEL_TASKS} step={1} value={Number.isNaN(selectedLimit) ? '' : selectedLimit} disabled={busy || subscription} onChange={event => setLimit(event.target.value === '' ? NaN : Number(event.target.value))} /></label>
      {!validLimit && <p role="alert">Choose a whole number from 1 to {MAX_SWARM_PARALLEL_TASKS}.</p>}
      <p>Actual starts also depend on ready tasks and the current workspace, account, provider and resource limits.</p>
      {subscription && <p>This saved Codex subscription connection supports one active agent at a time. To run independent tasks in parallel, prepare a new build with a compatible API or local-model connection.</p>}
      <p>Each active agent uses memory, disk and provider usage. Workspace and provider limits may keep additional tasks queued. This limit controls concurrency, not provider billing.</p>
      <p>Commit the starting project first. Include each selected task’s dependencies in this build. Conflicting or unspecified file scopes are scheduled separately.</p>
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={() => setDialog(null)}>Keep reviewing</Button><Button disabled={busy || !canEdit || !canExecute || blocked || !capability.allowed || !validLimit} onClick={() => void operate('Preparing agent team', async epoch => { if (!current.current.taskAllowed || !validLimit) return; const value = await projectSwarmApi.prepare(run.projectId, run.id, run.revision, selectedLimit); if (remember(value, epoch)) { setDialog(null); setNotice('Agent team prepared. Review the task scopes, then start the ready tasks.'); } })}>Prepare team</Button></div>
    </WorkflowDialog>}
    {dialog === 'cancel' && <WorkflowDialog readAccess={projectAccess} title="Cancel remaining tasks" onClose={() => setDialog(null)} busy={busy} description="Prevent further task starts for this team. Existing agents keep running until you stop them in their sessions. Saved changes and integration history remain available.">
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={() => setDialog(null)}>Keep team</Button><Button disabled={busy || !canEdit} onClick={() => void operate('Cancelling new starts', epoch => control('cancel', epoch), 'edit')}>Cancel future starts</Button></div>
    </WorkflowDialog>}
    {review && <WorkflowDialog readAccess={projectAccess} title="Review task changes" getCloseFocusTarget={() => reviewCloseTarget(review)} onClose={() => setReview(null)} busy={busy} description="Review the exact committed task changes before integrating them into the main project. The server checks the task and target again before changing files.">
      <p>{tasks.find(task => task.id === review.taskId)?.title}</p>
      <p className="project-swarm-commits">Task commit <code>{review.sourceCommit || 'unavailable'}</code><br />Target commit <code>{review.targetCommit || 'unavailable'}</code></p>
      <p className={review.canIntegrate ? 'journey-muted' : 'journey-warning'}>{review.reason}</p>
      <ul>{review.paths.map(path => <li key={path}><code>{path}</code></li>)}</ul>
      {review.patch && <pre className="project-swarm-patch" tabIndex={0} aria-label="Reviewed task diff">{review.patch}</pre>}
      {review.canIntegrate && <label className="setup-check"><input type="checkbox" checked={approved} disabled={busy || !canEdit} onChange={event => setApproved(event.target.checked)} />I reviewed these changes for integration. I will verify the combined build afterwards.</label>}
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={() => setReview(null)}>Keep reviewing</Button><Button disabled={busy || !canEdit || !canExecute || blocked || !review.canIntegrate || !approved} onClick={() => void operate('Integrating reviewed changes', async epoch => { const selected = review; setReview(null); const value = await projectSwarmApi.integrate(run.projectId, run.id, selected.taskId, run.revision, selected.reviewDigest); if (remember(value, epoch)) setNotice(value.swarm?.tasks.find(task => task.id === selected.taskId)?.integration?.state === 'integrated' ? 'Reviewed changes integrated. Dependent tasks can now start; the combined build still needs verification.' : 'Integration did not complete. The task changes are retained; inspect the recorded outcome before retrying. Dependent tasks still wait for reviewed integration.'); })}>Integrate reviewed changes</Button></div>
    </WorkflowDialog>}
    {retire && <WorkflowDialog readAccess={projectAccess} title={finishingRecovery ? 'Finish recovery' : 'Retain changes and retry'} onClose={() => setRetire(null)} busy={busy} description="Keep this attempt’s workspace and output. Luci will allow retirement only when it can prove the child is stopped or was never allocated. A running process, an in-flight allocation, or an uncertain observation prevents recovery. This does not undo integrated changes or start another agent; a fresh attempt requires execution permission and a separate start.">
      <p>{tasks.find(task => task.id === retire)?.title}</p>
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={() => setRetire(null)}>Keep inspecting</Button><Button disabled={busy || !canEdit} onClick={() => void operate('Verifying and retiring attempt', epoch => retireAttempt(retire, epoch), 'edit')}>{finishingRecovery ? 'Finish recovery' : 'Retain output and allow retry'}</Button></div>
    </WorkflowDialog>}
    {recoveryTask && recoveryOperation && recoveryOperation.childId === launchRecovery?.childId && <SwarmLaunchRecoveryDialog readAccess={projectAccess}
      key={`${run.projectId}:${run.id}:${run.revision}:${recoveryOperation.id}:${launchRecovery?.epoch}`}
      run={run} operation={recoveryOperation} taskTitle={recoveryTask.title} projectName={projectName} canEdit={canEdit} busy={busy} error={error}
      onClose={() => setLaunchRecovery(null)} onConfirm={availability => void operate('Stopping the exact uncertain launch', epoch => retireAttempt(recoveryTask.id, epoch, availability), 'edit')} />}
    {checkpointTask && checkpointOperation && checkpointOperation.childId === checkpoint?.childId && <SwarmCheckpointDialog readAccess={projectAccess}
      key={`${run.projectId}:${run.id}:${run.revision}:${checkpointOperation.id}:${checkpoint?.epoch}`}
      run={run} operation={checkpointOperation} taskTitle={checkpointTask.title} projectName={projectName}
      canContinue={canEdit && canExecute && !blocked && capability.allowed} busy={busy} error={error}
      getCloseFocusTarget={openingControl => checkpoint ? checkpointCloseTarget(checkpoint, openingControl) : null}
      onClose={() => setCheckpoint(null)} onConfirm={(selected, key) => void operate('Preparing a continued task attempt', epoch => continueCheckpoint(selected, key, epoch))} />}
    {workspace && <SwarmWorkspaceReview key={workspace.id} projectId={run.projectId} runId={run.id} operation={workspace} canRead onClose={() => setWorkspace(null)} />}
    {handoffOperation && handoffTask && <SwarmTaskHandoff projectId={run.projectId} projectName={projectName} run={run} operation={handoffOperation}
      taskTitle={handoffTask.title} canRead={projectAccess.canRead} canEdit={canEdit} onClose={() => setHandoff(null)}
      onRun={value => { if (value.revision >= current.current.run.revision) remember(value, generation.current); }} />}
  </section>;
}

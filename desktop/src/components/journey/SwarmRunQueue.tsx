import { useEffect, useRef, useState, type Ref } from 'react';
import { errorText } from '../../api/product';
import { runOperationNeedsRecovery, type ProjectRun } from '../../api/projectRuns';
import { swarmQueueApi, type ProjectSwarmQueue, type SwarmQueueItem, type QueuePriority } from '../../api/swarmQueue';
import { swarmTaskCapability } from '../../api/swarmTaskCapability';
import { useCockpit } from '../../store/store';
import { Button } from '../fluid/components/button';
import { WorkflowDialog, type WorkflowDialogReadAccess } from '../common/WorkflowDialog';

const unfinished = (item: SwarmQueueItem) => !['cancelled', 'completed', 'failed'].includes(item.state);
type QueueControlAction = 'pause' | 'resume' | 'cancel';
type UncertainQueueControl = { item: SwarmQueueItem; action: QueueControlAction; readBackEpoch?: number };
const sameControlScope = (a: SwarmQueueItem, b: SwarmQueueItem) =>
  a.projectId === b.projectId && a.runId === b.runId && a.id === b.id && a.taskId === b.taskId;
const controlItemMatches = (items: SwarmQueueItem[], target: SwarmQueueItem) =>
  items.filter(item => item.projectId === target.projectId && item.id === target.id);
const sameControlItem = (a: SwarmQueueItem, b: SwarmQueueItem) =>
  sameControlScope(a, b) && a.revision === b.revision && a.state === b.state && a.reasonCode === b.reasonCode &&
  a.reason === b.reason && a.priority === b.priority && a.queuedAt === b.queuedAt && a.updatedAt === b.updatedAt &&
  a.operationId === b.operationId && a.childId === b.childId && a.memoryMiB === b.memoryMiB && a.cpuMillis === b.cpuMillis;
interface Props {
  readAccess: WorkflowDialogReadAccess; run: ProjectRun; canEdit: boolean; canExecute: boolean; busy: boolean;
  onBusy(label: string): void;
  onQueueChange(queue: ProjectSwarmQueue): void;
  onConsentChange?(open: boolean): void;
  reviewButtonRef?: Ref<HTMLButtonElement>;
}
/** Queue consent names saved task inputs. A read, tab switch or retrying a read never enqueues work. */
export function SwarmRunQueue(props: Props) {
  const { run, readAccess, canEdit, canExecute, busy } = props;
  const cloudRun = run.input.route.executionLocation === 'cloud-microvm';
  const harnesses = useCockpit(state => state.harnesses);
  const capability = swarmTaskCapability(run, harnesses);
  const [queue, setQueue] = useState<ProjectSwarmQueue | null>(null), [error, setError] = useState('');
  const [notice, setNotice] = useState(''), [confirm, setConfirm] = useState(false);
  const [priority, setPriority] = useState<QueuePriority>('normal');
  const [unconfirmed, setUnconfirmed] = useState(false);
  const [uncertainControls, setUncertainControls] = useState<UncertainQueueControl[]>([]);
  const uncertainRef = useRef<UncertainQueueControl[]>([]);
  const current = useRef(props); current.current = props;
  useEffect(() => { current.current.onConsentChange?.(confirm); }, [confirm, run.projectId, run.id]);
  useEffect(() => {
    const notify = props.onConsentChange;
    return () => notify?.(false);
  }, [run.projectId, run.id]);
  const epoch = useRef(0), live = useRef(true), working = useRef(false), reading = useRef(false);
  const runScope = useRef({projectId:run.projectId, runId:run.id, generation:0});
  if (runScope.current.projectId !== run.projectId || runScope.current.runId !== run.id) {
    runScope.current = {projectId:run.projectId, runId:run.id, generation:runScope.current.generation + 1};
  }
  const identity = `${run.projectId}:${run.id}:${readAccess.canRead}:${canEdit}:${canExecute}:${capability.signature}`;
  const previousIdentity = useRef(identity);
  if (identity !== previousIdentity.current) { previousIdentity.current = identity; epoch.current++; }
  const keys = useRef(new Map<string, string>());
  const readAttempt = useRef(0);
  function receive(value: ProjectSwarmQueue) {
    setQueue(value); current.current.onQueueChange(value);
  }
  function updateUncertain(change: (entries: UncertainQueueControl[]) => UncertainQueueControl[]) {
    const next = change(uncertainRef.current);
    uncertainRef.current = next;
    setUncertainControls(next);
  }
  function reconcileControls(value: ProjectSwarmQueue, generation: number, fromRead: boolean) {
    updateUncertain(previous => previous.flatMap(entry => {
      if (entry.item.projectId !== run.projectId || entry.item.runId !== run.id) return [entry];
      if (value.hasMore) return [{...entry, readBackEpoch:undefined}];
      const matches = controlItemMatches(value.items, entry.item);
      if (matches.length !== 1 || !sameControlScope(matches[0], entry.item)) return [{...entry, readBackEpoch:undefined}]; // An absent or incomplete read is not a result.
      if (matches[0].revision !== entry.item.revision) return []; // Current state changed; origin is unknown.
      return [{...entry, readBackEpoch: fromRead && sameControlItem(matches[0], entry.item) ? generation : undefined}];
    }));
  }
  async function refresh(signal?: AbortSignal) {
    if (reading.current || working.current || signal?.aborted || !current.current.readAccess.canRead) return;
    reading.current = true;
    const generation = epoch.current, attempt = ++readAttempt.current;
    try {
      const value = await swarmQueueApi.get(run.projectId, signal);
      if (live.current && generation === epoch.current && attempt === readAttempt.current && current.current.readAccess.canRead) {
        receive(value); reconcileControls(value, generation, true); setError(''); setUnconfirmed(false);
      }
    } catch (cause) {
      if (live.current && generation === epoch.current && !signal?.aborted && attempt === readAttempt.current) {
        updateUncertain(previous => previous.map(entry => ({...entry, readBackEpoch:undefined})));
        setError(errorText(cause));
      }
    } finally { reading.current = false; }
  }
  useEffect(() => {
    live.current = true;
    const controller = new AbortController();
    const tick = () => { if (document.visibilityState !== 'hidden') void refresh(controller.signal); };
    tick();
    const timer = window.setInterval(tick, 5000);
    document.addEventListener('visibilitychange', tick);
    return () => { live.current = false; epoch.current++; runScope.current.generation++; controller.abort(); window.clearInterval(timer); document.removeEventListener('visibilitychange', tick); };
  }, [run.projectId, run.id]);
  useEffect(() => { updateUncertain(() => []); }, [run.projectId, run.id]);
  useEffect(() => { setConfirm(false); }, [run.revision, canEdit, canExecute, capability.signature]);
  const items = queue?.items.filter(item => item.runId === run.id) ?? [];
  const scopedUncertainty = uncertainControls.filter(entry => entry.item.projectId === run.projectId && entry.item.runId === run.id);
  const uncertainFor = (item: SwarmQueueItem) => scopedUncertainty.find(entry => sameControlScope(entry.item, item));
  const pending = items.filter(unfinished);
  const failures = items.filter(item => item.state === 'failed');
  const candidates = run.swarm?.tasks.filter(task => !task.integration &&
    (!task.operationId || run.operations.some(operation => operation.id === task.operationId && operation.status === 'cancelled' && !runOperationNeedsRecovery(operation))) &&
    !pending.some(item => item.taskId === task.id)) ?? [];
  const sourceBlocked = !!run.codingBlocker || !!run.contextFreshness && run.contextFreshness.status !== 'current' || run.swarm?.state !== 'ready' || ['cancelled', 'complete'].includes(run.state);
  const retryReady = (entry: UncertainQueueControl) => {
    if (!readAccess.canRead || !canEdit || busy || entry.readBackEpoch !== epoch.current || !queue || queue.hasMore ||
        entry.action === 'resume' && (!canExecute || sourceBlocked || !capability.allowed)) return false;
    const matches = controlItemMatches(queue.items, entry.item);
    return matches.length === 1 && sameControlItem(matches[0], entry.item);
  };
  async function operate(label: string, action: (generation: number) => Promise<void>, execute = false) {
    if (working.current || busy || !readAccess.canRead || !canEdit || execute && (!canExecute || !capability.allowed)) return;
    working.current = true; readAttempt.current++; props.onBusy(label); setError(''); setNotice('');
    const generation = epoch.current;
    try { await action(generation); }
    catch (cause) { if (live.current && generation === epoch.current) setError(errorText(cause)); }
    finally { working.current = false; if (live.current) current.current.onBusy(''); }
  }
  const usable = (generation: number) => live.current && generation === epoch.current && current.current.readAccess.canRead && current.current.canEdit;
  async function enqueue(generation: number) {
    const selected = candidates.map(task => task.id);
    if (!usable(generation) || !current.current.canExecute) return;
    const keyScope = JSON.stringify([run.id, run.revision, selected, priority]);
    let key = keys.current.get(keyScope);
    if (!key) {
      key = crypto.randomUUID(); keys.current.set(keyScope, key);
      if (keys.current.size > 64) keys.current.delete(keys.current.keys().next().value!);
    }
    try {
      const result = await swarmQueueApi.enqueueMany(run.projectId, run.id, selected, run.revision, key, priority);
      if (usable(generation)) { receive(result.queue); setConfirm(false); setNotice(`${result.items.length} tasks queued. They can start automatically while Luci’s service is running, including when you work in another project.`); }
    } catch (cause) {
      if (usable(generation)) { setUnconfirmed(true); setConfirm(false); setNotice('The queue result is unconfirmed. Refresh the queue before trying again; an interrupted response may already have been accepted.'); }
      throw cause;
    }
  }

  async function control(item: SwarmQueueItem, action: 'pause' | 'resume' | 'cancel' | 'priority' | 'stop', generation: number, value?: QueuePriority, retry = false) {
    const unresolved = uncertainRef.current.find(entry => sameControlScope(entry.item, item));
    if (unresolved && (!retry || unresolved.action !== action || !retryReady(unresolved))) return;
    const originalItem = {...item}, originalRunScope = runScope.current.generation;
    try {
      const result = await swarmQueueApi.control(run.projectId, item, action, value);
      if (usable(generation)) {
        receive(result);
        reconcileControls(result, generation, false);
        if (action === 'pause' || action === 'resume' || action === 'cancel') {
          updateUncertain(previous => previous.filter(entry => !sameControlScope(entry.item, item)));
        }
        setNotice(action === 'stop' ? 'Exact task stop requested. Refresh status until its child and reservation are confirmed settled; saved output remains available.' : action === 'cancel' ? 'Future starts cancelled for this queue entry. Existing sessions and saved output are retained.' : 'Queue updated. Active work keeps its own session controls.');
      }
    } catch (cause) {
      if (live.current && runScope.current.generation === originalRunScope &&
          current.current.run.projectId === originalItem.projectId && current.current.run.id === originalItem.runId &&
          (action === 'pause' || action === 'resume' || action === 'cancel')) {
        const originalAction: QueueControlAction = action;
        updateUncertain(previous => [...previous.filter(entry => !sameControlScope(entry.item, originalItem)), {item:originalItem, action:originalAction}]);
      }
      throw cause;
    }
  }
  return <section className="swarm-run-queue" aria-label="Background task queue">
    <div className="project-swarm-heading"><h4>Background queue</h4>
      <Button ref={props.reviewButtonRef} disabled={!queue || !canEdit || !canExecute || busy || unconfirmed || sourceBlocked || !capability.allowed || !candidates.length} onClick={() => { setConfirm(true); setNotice(''); }}>Queue reviewed tasks{candidates.length ? ` (${candidates.length})` : ''}</Button>
    </div>
    {!capability.allowed && <p className="journey-warning">{capability.reason}</p>}
    {error && <p role="status" className="journey-warning">Queue update unavailable: {error}</p>}
    {notice && <p role="status">{notice}</p>}
    {readAccess.canRead && scopedUncertainty.map(entry => <p key={entry.item.id} role="status" className="journey-warning">
      The {entry.action} result for {entry.item.taskTitle} is unconfirmed. The original request may still finish. Refresh the queue to inspect its saved status.
      {entry.readBackEpoch === epoch.current && ' The latest read still matches the original revision; this does not prove the request has finished. Only an explicit retry can send the same action again.'}
    </p>)}
    {queue?.dispatchPaused && <p role="status">Background starts are paused for this workspace. Manage this in the projects overview.</p>}
    {(failures.length > 0 || pending.some(item => ['blocked', 'unknown'].includes(item.state))) && <p className="journey-warning">Queued work needs attention. Open Queue status and controls to inspect it before continuing.</p>}
    <details className="swarm-queue-controls"><summary>Queue status and controls{pending.length ? ` · ${pending.length} active or waiting` : ''}</summary>
      <Button variant="tertiary" disabled={busy} onClick={() => void refresh()}>Refresh queue</Button>
      <p>Queue reviewed tasks once. Luci waits for dependencies and capacity across projects. Review each result before integrating it.</p>
    {failures.length > 0 && <ul className="swarm-run-queue-items">{failures.map(item => <li key={item.id}><strong>{item.taskTitle}</strong> · Failed<p>{item.reason || 'The attempt failed. Inspect its recorded outcome before retrying.'}</p><p>Review this attempt in the task list. A new start requires explicit recovery and fresh authorization.</p></li>)}</ul>}
    {pending.length > 0 && <ul className="swarm-run-queue-items">{pending.map(item => { const uncertain = uncertainFor(item); return <li key={item.id}>
      <div><strong>{item.taskTitle}</strong> <span>{item.state}</span></div>
      {item.reason && <p>{item.reason}</p>}
      {item.state === 'paused' && item.reasonCode === 'repair_checkpoint_pending' && <p role="status">Repair continuation is waiting for a protected checkpoint of the stopped attempt. No repair agent has started for this continuation. You can pause this continuation while the checkpoint is pending.</p>}
      {item.reasonCode === 'cloud_restore_pending' && <p role="status">Luci is checking the exact restored microVM identities before another agent starts. Existing VM ownership and capacity remain reserved. Refresh status to see the outcome; you can request an exact stop for an owned active attempt.</p>}
      {item.state === 'unknown' && ((item.memoryMiB ?? 0) > 0 || (item.cpuMillis ?? 0) > 0) &&
        <p role="status">Capacity still reserved: {item.memoryMiB ?? 0} MiB memory and {item.cpuMillis ?? 0}m CPU. {item.childId ? `Exact child ${item.childId}.` : 'Child identity is not yet confirmed.'} Luci will not replace this attempt until ownership and release are proven.</p>}
      <div className="journey-actions">
        <label>Priority for {item.taskTitle}<select value={item.priority} disabled={!canEdit || busy || !!uncertain || !['queued', 'paused', 'blocked'].includes(item.state)} onChange={event => void operate('Changing queue priority', generation => control(item, 'priority', generation, event.target.value as QueuePriority))}><option value="high">High</option><option value="normal">Normal</option><option value="low">Low</option></select></label>
        {item.state === 'queued' && <Button variant="secondary" disabled={!canEdit || busy || !!uncertain} onClick={() => void operate('Pausing queued task', generation => control(item, 'pause', generation))}>Pause queued task</Button>}
        {item.state === 'paused' && item.reasonCode === 'repair_checkpoint_pending' && <Button variant="secondary" disabled={!canEdit || busy || !!uncertain} onClick={() => void operate('Pausing repair continuation', generation => control(item, 'pause', generation))}>Pause repair continuation</Button>}
        {item.state === 'paused' && item.reasonCode === 'operator_paused' && <Button variant="secondary" disabled={!canEdit || !canExecute || busy || !!uncertain || sourceBlocked || !capability.allowed} onClick={() => void operate('Resuming queued task', generation => control(item, 'resume', generation), true)}>Resume queued task</Button>}
        {['queued', 'paused', 'blocked'].includes(item.state) && <Button variant="tertiary" disabled={!canEdit || busy || !!uncertain} onClick={() => void operate('Cancelling queued task', generation => control(item, 'cancel', generation))}>Cancel queued task</Button>}
        {uncertain && <Button variant="secondary" disabled={!retryReady(uncertain)} onClick={() => void operate(`Retrying exact ${uncertain.action}`, generation => control(uncertain.item, uncertain.action, generation, undefined, true), uncertain.action === 'resume')}>Retry exact {uncertain.action}</Button>}
        {['reserved', 'running', 'unknown'].includes(item.state) && item.operationId && item.childId && <Button variant="tertiary" disabled={!canEdit || busy} onClick={() => void operate('Stopping exact active task', generation => control(item, 'stop', generation))}>{item.reasonCode?.endsWith('stop_pending') ? 'Retry exact stop' : 'Stop active task'}</Button>}
      </div>
      {item.state === 'blocked' && <p>Resolve the reason above, cancel this queue entry, then review the build and queue a new attempt.</p>}
      {item.state === 'unknown' && (item.reasonCode?.startsWith('context_superseded_')
        ? <p role="status">Luci is reconciling this old-context attempt and stopping its exact child when identified. Its reservation remains held until absence and release are confirmed; revised work waits. Saved output stays available for review.</p>
        : <p>Inspect the task session and retained output, then check team status. An uncertain start is never replayed automatically.</p>)}
    </li>; })}</ul>}
    {queue?.hasMore && <p>Only part of the project queue is shown. Refresh as earlier entries complete to inspect remaining work.</p>}
    </details>
    {confirm && <WorkflowDialog readAccess={readAccess} title="Queue reviewed work" description="Authorize these exact saved tasks to start automatically as capacity becomes available. Dependent tasks wait for reviewed integration. Your configured model connection may incur usage charges." busy={busy} onClose={() => setConfirm(false)}>
      <p>{run.input.route.harness} · {run.input.route.model || 'Configured model'} · up to {run.swarm?.maxParallel} agents in this build, within workspace and provider limits.</p>
      <p>Saved permission: {capability.mode}. Tasks run commands without interactive approvals inside {cloudRun ? 'separate owned cloud microVMs' : 'isolated Docker workspaces'} and exit after their response. Their output still needs review; queue completion does not accept or integrate changes.</p>
      {capability.notice && <p>{capability.notice}</p>}
      <ul>{candidates.map(task => <li key={task.id}>{task.title}</li>)}</ul>
      <label className="luci-field">Initial task priority<select value={priority} disabled={busy} onChange={event => setPriority(event.target.value as QueuePriority)}><option value="high">High</option><option value="normal">Normal</option><option value="low">Low</option></select></label>
      <p>Priorities affect ordering; other ready projects continue to receive capacity. Pause or cancel queued work from any project overview. Existing agents have separate stop controls.</p>
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={() => setConfirm(false)}>Keep reviewing</Button><Button disabled={busy || !canEdit || !canExecute || sourceBlocked || !capability.allowed || !candidates.length} onClick={() => void operate('Queuing reviewed tasks', enqueue, true)}>Authorize queued tasks</Button></div>
    </WorkflowDialog>}
  </section>;
}

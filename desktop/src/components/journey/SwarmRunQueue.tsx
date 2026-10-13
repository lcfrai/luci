import { useEffect, useRef, useState, type Ref } from 'react';
import { errorText } from '../../api/product';
import { runOperationNeedsRecovery, type ProjectRun } from '../../api/projectRuns';
import { swarmQueueApi, type ProjectSwarmQueue, type SwarmQueueItem, type QueuePriority } from '../../api/swarmQueue';
import { swarmTaskCapability } from '../../api/swarmTaskCapability';
import { useCockpit } from '../../store/store';
import { Button } from '../fluid/components/button';
import { WorkflowDialog, type WorkflowDialogReadAccess } from '../common/WorkflowDialog';

const unfinished = (item: SwarmQueueItem) => !['cancelled', 'completed', 'failed'].includes(item.state);
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
  const current = useRef(props); current.current = props;
  useEffect(() => { current.current.onConsentChange?.(confirm); }, [confirm, run.projectId, run.id]);
  useEffect(() => {
    const notify = props.onConsentChange;
    return () => notify?.(false);
  }, [run.projectId, run.id]);
  const epoch = useRef(0), live = useRef(true), working = useRef(false), reading = useRef(false);
  const identity = `${run.projectId}:${run.id}:${canEdit}:${canExecute}:${capability.signature}`;
  const previousIdentity = useRef(identity);
  if (identity !== previousIdentity.current) { previousIdentity.current = identity; epoch.current++; }
  const keys = useRef(new Map<string, string>());
  const readAttempt = useRef(0);
  function receive(value: ProjectSwarmQueue) {
    setQueue(value); current.current.onQueueChange(value);
  }
  async function refresh(signal?: AbortSignal) {
    if (reading.current || working.current || signal?.aborted) return;
    reading.current = true;
    const generation = epoch.current, attempt = ++readAttempt.current;
    try {
      const value = await swarmQueueApi.get(run.projectId, signal);
      if (live.current && generation === epoch.current && attempt === readAttempt.current) { receive(value); setError(''); setUnconfirmed(false); }
    } catch (cause) {
      if (live.current && generation === epoch.current && !signal?.aborted && attempt === readAttempt.current) setError(errorText(cause));
    } finally { reading.current = false; }
  }
  useEffect(() => {
    live.current = true;
    const controller = new AbortController();
    const tick = () => { if (document.visibilityState !== 'hidden') void refresh(controller.signal); };
    tick();
    const timer = window.setInterval(tick, 5000);
    document.addEventListener('visibilitychange', tick);
    return () => { live.current = false; epoch.current++; controller.abort(); window.clearInterval(timer); document.removeEventListener('visibilitychange', tick); };
  }, [run.projectId, run.id]);
  useEffect(() => { setConfirm(false); }, [run.revision, canEdit, canExecute, capability.signature]);
  const items = queue?.items.filter(item => item.runId === run.id) ?? [];
  const pending = items.filter(unfinished);
  const failures = items.filter(item => item.state === 'failed');
  const candidates = run.swarm?.tasks.filter(task => !task.integration &&
    (!task.operationId || run.operations.some(operation => operation.id === task.operationId && operation.status === 'cancelled' && !runOperationNeedsRecovery(operation))) &&
    !pending.some(item => item.taskId === task.id)) ?? [];
  const sourceBlocked = !!run.codingBlocker || !!run.contextFreshness && run.contextFreshness.status !== 'current' || run.swarm?.state !== 'ready' || ['cancelled', 'complete'].includes(run.state);
  async function operate(label: string, action: (generation: number) => Promise<void>, execute = false) {
    if (working.current || busy || !canEdit || execute && (!canExecute || !capability.allowed)) return;
    working.current = true; readAttempt.current++; props.onBusy(label); setError(''); setNotice('');
    const generation = epoch.current;
    try { await action(generation); }
    catch (cause) { if (live.current && generation === epoch.current) setError(errorText(cause)); }
    finally { working.current = false; if (live.current) current.current.onBusy(''); }
  }
  const usable = (generation: number) => live.current && generation === epoch.current && current.current.canEdit;
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

  async function control(item: SwarmQueueItem, action: 'pause' | 'resume' | 'cancel' | 'priority' | 'stop', generation: number, value?: QueuePriority) {
    const result = await swarmQueueApi.control(run.projectId, item, action, value);
    if (usable(generation)) { receive(result); setNotice(action === 'stop' ? 'Exact task stop requested. Refresh status until its child and reservation are confirmed settled; saved output remains available.' : action === 'cancel' ? 'Future starts cancelled for this queue entry. Existing sessions and saved output are retained.' : 'Queue updated. Active work keeps its own session controls.'); }
  }
  return <section className="swarm-run-queue" aria-label="Background task queue">
    <div className="project-swarm-heading"><h4>Background queue</h4>
      <Button ref={props.reviewButtonRef} disabled={!queue || !canEdit || !canExecute || busy || unconfirmed || sourceBlocked || !capability.allowed || !candidates.length} onClick={() => { setConfirm(true); setNotice(''); }}>Queue reviewed tasks{candidates.length ? ` (${candidates.length})` : ''}</Button>
    </div>
    {!capability.allowed && <p className="journey-warning">{capability.reason}</p>}
    {error && <p role="status" className="journey-warning">Queue update unavailable: {error}</p>}
    {notice && <p role="status">{notice}</p>}
    {queue?.dispatchPaused && <p role="status">Background starts are paused for this workspace. Manage this in the projects overview.</p>}
    {(failures.length > 0 || pending.some(item => ['blocked', 'unknown'].includes(item.state))) && <p className="journey-warning">Queued work needs attention. Open Queue status and controls to inspect it before continuing.</p>}
    <details className="swarm-queue-controls"><summary>Queue status and controls{pending.length ? ` · ${pending.length} active or waiting` : ''}</summary>
      <Button variant="tertiary" disabled={busy} onClick={() => void refresh()}>Refresh queue</Button>
      <p>Queue reviewed tasks once. Luci waits for dependencies and capacity across projects. Review each result before integrating it.</p>
    {failures.length > 0 && <ul className="swarm-run-queue-items">{failures.map(item => <li key={item.id}><strong>{item.taskTitle}</strong> · Failed<p>{item.reason || 'The attempt failed. Inspect its recorded outcome before retrying.'}</p><p>Review this attempt in the task list. A new start requires explicit recovery and fresh authorization.</p></li>)}</ul>}
    {pending.length > 0 && <ul className="swarm-run-queue-items">{pending.map(item => <li key={item.id}>
      <div><strong>{item.taskTitle}</strong> <span>{item.state}</span></div>
      {item.reason && <p>{item.reason}</p>}
      {item.state === 'unknown' && ((item.memoryMiB ?? 0) > 0 || (item.cpuMillis ?? 0) > 0) &&
        <p role="status">Capacity still reserved: {item.memoryMiB ?? 0} MiB memory and {item.cpuMillis ?? 0}m CPU. {item.childId ? `Exact child ${item.childId}.` : 'Child identity is not yet confirmed.'} Luci will not replace this attempt until ownership and release are proven.</p>}
      <div className="journey-actions">
        <label>Priority for {item.taskTitle}<select value={item.priority} disabled={!canEdit || busy || !['queued', 'paused', 'blocked'].includes(item.state)} onChange={event => void operate('Changing queue priority', generation => control(item, 'priority', generation, event.target.value as QueuePriority))}><option value="high">High</option><option value="normal">Normal</option><option value="low">Low</option></select></label>
        {item.state === 'queued' && <Button variant="secondary" disabled={!canEdit || busy} onClick={() => void operate('Pausing queued task', generation => control(item, 'pause', generation))}>Pause queued task</Button>}
        {item.state === 'paused' && <Button variant="secondary" disabled={!canEdit || !canExecute || busy || sourceBlocked || !capability.allowed} onClick={() => void operate('Resuming queued task', generation => control(item, 'resume', generation), true)}>Resume queued task</Button>}
        {['queued', 'paused', 'blocked'].includes(item.state) && <Button variant="tertiary" disabled={!canEdit || busy} onClick={() => void operate('Cancelling queued task', generation => control(item, 'cancel', generation))}>Cancel queued task</Button>}
        {['reserved', 'running', 'unknown'].includes(item.state) && item.operationId && item.childId && <Button variant="tertiary" disabled={!canEdit || busy} onClick={() => void operate('Stopping exact active task', generation => control(item, 'stop', generation))}>{item.reasonCode?.endsWith('stop_pending') ? 'Retry exact stop' : 'Stop active task'}</Button>}
      </div>
      {item.state === 'blocked' && <p>Resolve the reason above, cancel this queue entry, then review the build and queue a new attempt.</p>}
      {item.state === 'unknown' && (item.reasonCode?.startsWith('context_superseded_')
        ? <p role="status">Luci is reconciling this old-context attempt and stopping its exact child when identified. Its reservation remains held until absence and release are confirmed; revised work waits. Saved output stays available for review.</p>
        : <p>Inspect the task session and retained output, then check team status. An uncertain start is never replayed automatically.</p>)}
    </li>)}</ul>}
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

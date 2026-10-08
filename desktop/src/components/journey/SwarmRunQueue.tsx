import { useEffect, useRef, useState } from 'react';
import { errorText } from '../../api/product';
import { runOperationNeedsRecovery, type ProjectRun } from '../../api/projectRuns';
import { swarmQueueApi, type ProjectSwarmQueue, type SwarmQueueItem, type QueuePriority } from '../../api/swarmQueue';
import { Button } from '../fluid/components/button';
import { WorkflowDialog } from '../common/WorkflowDialog';

const unfinished = (item: SwarmQueueItem) => !['cancelled', 'completed'].includes(item.state);
interface Props {
  run: ProjectRun; canEdit: boolean; canExecute: boolean; busy: boolean;
  onBusy(label: string): void;
  onQueueChange(queue: ProjectSwarmQueue): void;
}
/** Queue consent names saved task inputs. A read, tab switch or retrying a read never enqueues work. */
export function SwarmRunQueue(props: Props) {
  const { run, canEdit, canExecute, busy } = props;
  const [queue, setQueue] = useState<ProjectSwarmQueue | null>(null), [error, setError] = useState('');
  const [notice, setNotice] = useState(''), [confirm, setConfirm] = useState(false);
  const [priority, setPriority] = useState<QueuePriority>('normal');
  const [unconfirmed, setUnconfirmed] = useState(false);
  const current = useRef(props); current.current = props;
  const epoch = useRef(0), live = useRef(true), working = useRef(false), reading = useRef(false);
  const identity = `${run.projectId}:${run.id}:${canEdit}:${canExecute}`;
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
  useEffect(() => { setConfirm(false); }, [run.revision, canEdit, canExecute]);
  const items = queue?.items.filter(item => item.runId === run.id) ?? [];
  const pending = items.filter(unfinished);
  const candidates = run.swarm?.tasks.filter(task => !task.integration &&
    (!task.operationId || run.operations.some(operation => operation.id === task.operationId && operation.status === 'cancelled' && !runOperationNeedsRecovery(operation))) &&
    !pending.some(item => item.taskId === task.id)) ?? [];
  const sourceBlocked = !!run.codingBlocker || !!run.contextFreshness && run.contextFreshness.status !== 'current' || run.swarm?.state !== 'ready' || ['cancelled', 'complete'].includes(run.state);
  async function operate(label: string, action: (generation: number) => Promise<void>, execute = false) {
    if (working.current || busy || !canEdit || execute && !canExecute) return;
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

  async function control(item: SwarmQueueItem, action: 'pause' | 'resume' | 'cancel' | 'priority', generation: number, value?: QueuePriority) {
    const result = await swarmQueueApi.control(run.projectId, item, action, value);
    if (usable(generation)) { receive(result); setNotice(action === 'cancel' ? 'Future starts cancelled for this queue entry. Existing sessions and saved output are retained.' : 'Queue updated. Active work keeps its own session controls.'); }
  }
  return <section className="swarm-run-queue" aria-label="Background task queue">
    <div className="project-swarm-heading"><h4>Keep work moving across projects</h4><Button variant="tertiary" disabled={busy} onClick={() => void refresh()}>Refresh queue</Button></div>
    <p>Queue reviewed tasks once. Luci waits for dependencies, shares capacity across projects, and keeps each task’s context separate. Review each result before integrating it.</p>
    {error && <p role="status" className="journey-warning">Queue update unavailable: {error}</p>}
    {notice && <p role="status">{notice}</p>}
    {queue?.dispatchPaused && <p role="status">Background starts are paused for this workspace. Manage this in the projects overview.</p>}
    <Button disabled={!queue || !canEdit || !canExecute || busy || unconfirmed || sourceBlocked || !candidates.length} onClick={() => { setConfirm(true); setNotice(''); }}>Queue reviewed tasks{candidates.length ? ` (${candidates.length})` : ''}</Button>
    {pending.length > 0 && <ul className="swarm-run-queue-items">{pending.map(item => <li key={item.id}>
      <div><strong>{item.taskTitle}</strong> <span>{item.state}</span></div>
      {item.reason && <p>{item.reason}</p>}
      <div className="journey-actions">
        <label>Priority for {item.taskTitle}<select value={item.priority} disabled={!canEdit || busy || !['queued', 'paused', 'blocked'].includes(item.state)} onChange={event => void operate('Changing queue priority', generation => control(item, 'priority', generation, event.target.value as QueuePriority))}><option value="high">High</option><option value="normal">Normal</option><option value="low">Low</option></select></label>
        {item.state === 'queued' && <Button variant="secondary" disabled={!canEdit || busy} onClick={() => void operate('Pausing queued task', generation => control(item, 'pause', generation))}>Pause queued task</Button>}
        {item.state === 'paused' && <Button variant="secondary" disabled={!canEdit || !canExecute || busy || sourceBlocked} onClick={() => void operate('Resuming queued task', generation => control(item, 'resume', generation), true)}>Resume queued task</Button>}
        {['queued', 'paused', 'blocked'].includes(item.state) && <Button variant="tertiary" disabled={!canEdit || busy} onClick={() => void operate('Cancelling queued task', generation => control(item, 'cancel', generation))}>Cancel queued task</Button>}
      </div>
      {item.state === 'blocked' && <p>Resolve the reason above, cancel this queue entry, then review the build and queue a new attempt.</p>}
      {item.state === 'unknown' && <p>Inspect the task session and retained output, then check team status. An uncertain start is never replayed automatically.</p>}
    </li>)}</ul>}
    {queue?.hasMore && <p>Only part of the project queue is shown. Refresh as earlier entries complete to inspect remaining work.</p>}
    {confirm && <WorkflowDialog title="Queue reviewed work" description="Authorize these exact saved tasks to start automatically as capacity becomes available. Dependent tasks wait for reviewed integration. Your configured model connection may incur usage charges." busy={busy} onClose={() => setConfirm(false)}>
      <p>{run.input.route.harness} · {run.input.route.model || 'Configured model'} · up to {run.swarm?.maxParallel} agents in this build, within workspace and provider limits.</p>
      <ul>{candidates.map(task => <li key={task.id}>{task.title}</li>)}</ul>
      <label className="luci-field">Initial task priority<select value={priority} disabled={busy} onChange={event => setPriority(event.target.value as QueuePriority)}><option value="high">High</option><option value="normal">Normal</option><option value="low">Low</option></select></label>
      <p>Priorities affect ordering; other ready projects continue to receive capacity. Pause or cancel queued work from any project overview. Existing agents have separate stop controls.</p>
      {error && <p role="alert">{error}</p>}
      <div className="workflow-actions"><Button variant="secondary" disabled={busy} onClick={() => setConfirm(false)}>Keep reviewing</Button><Button disabled={busy || !canEdit || !canExecute || sourceBlocked || !candidates.length} onClick={() => void operate('Queuing reviewed tasks', enqueue, true)}>Authorize queued tasks</Button></div>
    </WorkflowDialog>}
  </section>;
}

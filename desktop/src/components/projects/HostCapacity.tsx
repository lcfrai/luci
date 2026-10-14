import { useEffect, useId, useRef, useState } from 'react';
import { swarmSchedulerApi, type SchedulerPolicy, type SwarmScheduler } from '../../api/swarmScheduler';
import { MAX_SWARM_PARALLEL_TASKS, MAX_SWARM_SCHEDULED_TASKS } from '../../api/swarmLimits';
import { useAuth } from '../auth/AuthProvider';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';
import { Input } from '../ui/input';
import { WorkflowDialog } from '../common/WorkflowDialog';

const memory = (bytes: number | null | undefined) => typeof bytes === 'number' && Number.isFinite(bytes) && bytes >= 0 ? `${(bytes / 1024 ** 3).toFixed(1)} GiB` : 'Unavailable';
const fresh = (observedAt?: string) => { const age = observedAt ? Date.now() - Date.parse(observedAt) : Infinity; return Number.isFinite(age) && age >= 0 && age < 30_000; };
const fields: { key: Exclude<keyof SchedulerPolicy, 'maxHeadroomMiB'>; label: string; min: number; max: number }[] = [
  { key: 'maxActive', label: 'Workspace agent ceiling', min: 1, max: MAX_SWARM_SCHEDULED_TASKS },
  { key: 'maxPerProject', label: 'Agents per project', min: 1, max: MAX_SWARM_PARALLEL_TASKS },
  { key: 'headroomPercent', label: 'RAM and CPU headroom (%)', min: 10, max: 75 },
  { key: 'minHeadroomMiB', label: 'Minimum RAM reserve (MiB)', min: 512, max: 65536 },
  { key: 'diskReserveMiB', label: 'Minimum disk reserve (MiB)', min: 512, max: 1048576 },
];
export function HostCapacity() {
  const auth = useAuth(), team = useTeamWorkspace();
  const permitted = !team?.teamId && !!auth && ['admin', 'super-admin'].includes(auth.profile.user.role);
  const [snapshot, setSnapshot] = useState<SwarmScheduler | null>(null);
  const reservations = snapshot?.resources?.reservations;
  const [, refreshReservationAge] = useState(0);
  useEffect(() => {
    if (reservations?.status !== 'ready') return;
    const remaining = Date.parse(reservations.observedAt) + 30_000 - Date.now();
    if (!Number.isFinite(remaining) || remaining <= 0 || remaining > 30_000) return;
    const timer = setTimeout(() => refreshReservationAge(value => value + 1), remaining);
    return () => clearTimeout(timer);
  }, [reservations]);
  const [expanded, setExpanded] = useState(false);
  const detailsId = useId();
  const [error, setError] = useState(false), [retry, setRetry] = useState(0);
  const [changing, setChanging] = useState(false), [changeError, setChangeError] = useState('');
  const [unconfirmed, setUnconfirmed] = useState(false);
  const [review, setReview] = useState<{ snapshot: SwarmScheduler; mode: 'dispatch' | 'settings'; dispatchPaused: boolean } | null>(null);
  const [draft, setDraft] = useState<SchedulerPolicy | null>(null);
  const live = useRef(true), generation = useRef(0), writing = useRef(false);
  const permittedNow = useRef(permitted); permittedNow.current = permitted;
  const authority = useRef(permitted), authorityEpoch = useRef(0), uncertainAuthority = useRef(false);
  if (authority.current !== permitted) {
    authority.current = permitted; authorityEpoch.current++; generation.current++;
    if (writing.current) uncertainAuthority.current = true;
    writing.current = false;
  }
  useEffect(() => { live.current = true; return () => { live.current = false; generation.current++; }; }, []);
  useEffect(() => {
    setSnapshot(null); setError(false); setReview(null); setChanging(false); setExpanded(false);
    if (uncertainAuthority.current) {
      setUnconfirmed(true);
      setChangeError('Access changed while a scheduling operation was pending. Explicitly refresh the current settings before making another change.');
    }
    if (!permitted) return;
    const controller = new AbortController();
    let timer: ReturnType<typeof setTimeout>;
    let active = false;
    async function load() {
      if (active || controller.signal.aborted) return;
      if (writing.current) { timer = setTimeout(() => void load(), 5_000); return; }
      active = true;
      const readGeneration = generation.current;
      try { const value = await swarmSchedulerApi.get(controller.signal); if (!controller.signal.aborted && readGeneration === generation.current) { setSnapshot(value); setError(false); } }
      catch { if (!controller.signal.aborted && readGeneration === generation.current) { setSnapshot(null); setError(true); } }
      active = false;
      if (!controller.signal.aborted) timer = setTimeout(() => { if (document.visibilityState === 'visible') void load(); }, 15_000);
    }
    const focus = () => { if (document.visibilityState === 'visible') { clearTimeout(timer); void load(); } };
    void load(); document.addEventListener('visibilitychange', focus);
    return () => { controller.abort(); clearTimeout(timer); document.removeEventListener('visibilitychange', focus); };
  }, [permitted, retry]);
  function open(mode: 'dispatch' | 'settings') {
    if (!snapshot || changing || unconfirmed) return;
    setChangeError(''); setDraft({ ...snapshot.policy });
    setReview({ snapshot, mode, dispatchPaused: mode === 'dispatch' ? !snapshot.dispatchPaused : snapshot.dispatchPaused });
  }
  async function save() {
    if (!review || !draft || writing.current || unconfirmed || !permittedNow.current) return;
    writing.current = true; generation.current++; setChanging(true); setChangeError('');
    const epoch = authorityEpoch.current;
    try {
      const value = await swarmSchedulerApi.configure(review.snapshot, review.mode === 'settings' ? draft : review.snapshot.policy, review.dispatchPaused);
      if (live.current && permittedNow.current && epoch === authorityEpoch.current) { setSnapshot(value); setReview(null); }
    } catch {
      if (live.current && permittedNow.current && epoch === authorityEpoch.current) { setUnconfirmed(true); setChangeError('This change could not be confirmed. Refresh and review the current settings before trying again; the service may already have accepted it. Your proposed values are retained.'); }
    } finally { if (epoch === authorityEpoch.current) { writing.current = false; if (live.current) setChanging(false); } }
  }
  async function refreshReview() {
    if (writing.current || !permittedNow.current) return;
    writing.current = true; generation.current++; setChanging(true);
    const epoch = authorityEpoch.current;
    try {
      const value = await swarmSchedulerApi.get();
      if (live.current && permittedNow.current && epoch === authorityEpoch.current) {
        setSnapshot(value); setUnconfirmed(false); setChangeError('');
        uncertainAuthority.current = false;
        // An explicit refresh updates the compare-and-swap basis. Keep the
        // proposed policy and dispatch choice for another deliberate review.
        setReview(previous => previous ? { ...previous, snapshot: value } : null);
      }
    } catch { if (live.current && permittedNow.current && epoch === authorityEpoch.current) setChangeError('Current settings could not be read. Your proposed values remain here; refresh before saving.'); }
    finally { if (epoch === authorityEpoch.current) { writing.current = false; if (live.current) setChanging(false); } }
  }
  if (!permitted) return null;
  const host = snapshot?.resources?.host, execution = snapshot?.resources?.execution;
  const hostFresh = fresh(host?.observedAt), executionFresh = fresh(execution?.observedAt);
  const cap = draft?.maxHeadroomMiB ?? 0;
  const draftValid = !!draft && draft.maxPerProject <= draft.maxActive && Number.isInteger(cap) && (cap === 0 || cap >= draft.minHeadroomMiB && cap <= 65536) && fields.every(field => Number.isInteger(draft[field.key]) && draft[field.key] >= field.min && draft[field.key] <= field.max);
  return <section className="overview-capacity" aria-label="Workspace capacity">
    <h2 className="overview-capacity-title">Workspace capacity</h2>
    {error ? <p role="status">Capacity unavailable. <Button variant="ghost" onClick={() => setRetry(value => value + 1)}>Retry capacity</Button></p> : !snapshot ? <p role="status">Reading capacity…</p> : <>
      <div className="overview-capacity-brief"><dl><div><dt>Host RAM free</dt><dd>{hostFresh ? memory(host?.memory?.effectiveAvailableBytes) : 'Unavailable'}</dd></div><div><dt>Host CPU</dt><dd>{hostFresh && typeof host?.cpu?.utilizationPercent === 'number' ? `${host.cpu.utilizationPercent.toFixed(0)}%` : 'Unavailable'}</dd></div><div><dt>New starts</dt><dd>{snapshot.dispatchPaused ? 'Paused' : 'Enabled'}</dd></div></dl><Button variant="ghost" aria-expanded={expanded} aria-controls={detailsId} onClick={() => setExpanded(value => !value)}>{expanded ? 'Hide capacity and settings' : 'Expand capacity and settings'}</Button></div>
      {(!executionFresh || execution?.status === 'unavailable') && <p className="overview-capacity-warning">Runtime capacity unknown. Expand for details.</p>}
      <div className="overview-capacity-detail" id={detailsId} hidden={!expanded}>{expanded && <>
        <p><strong>Workspace ceiling: {snapshot.policy.maxActive} agents · up to {snapshot.policy.maxPerProject} per project</strong></p>
        <p>Actual concurrency also depends on account and provider limits, available execution resources, and task dependencies.</p>
        <p>Execution memory capacity: {executionFresh && execution?.status !== 'unavailable' ? memory(execution?.totalMemoryBytes) : 'Unavailable'}. Host free memory alone does not authorize a start.</p>
        {reservations?.status === 'ready' && fresh(reservations.observedAt)
          ? <p>Reserved execution demand: {reservations.count} allocations · {memory(reservations.memoryMiB * 1024 ** 2)} · {(reservations.cpuMillis / 1000).toFixed(1)} CPU cores. Observed {new Date(reservations.observedAt).toLocaleTimeString()}; includes workers and checks across this machine, including allocations already running. This is not additional usage.</p>
          : <p>Reserved execution demand: Unavailable. Current worker and check reservations could not be confirmed.</p>}
        <p>Memory profile: {(snapshot.policy.maxHeadroomMiB ?? 0) > 0 ? `Responsive local work · percentage RAM target capped at ${snapshot.policy.maxHeadroomMiB} MiB` : 'Percentage reserve · saved settings'}. The RAM floor is {snapshot.policy.minHeadroomMiB} MiB; CPU headroom remains {snapshot.policy.headroomPercent}% and disk reserve {snapshot.policy.diskReserveMiB} MiB.</p>
        <p>Effective RAM reserve: {hostFresh ? memory(snapshot.resources?.memoryReserve?.hostBytes) : 'Unavailable'} on this computer · {executionFresh ? memory(snapshot.resources?.memoryReserve?.executionBytes) : 'Unavailable'} inside Docker. Pending allocations are added separately before each start.</p>
        {snapshot.resources?.limits && <p>Actual new agent allocation: {snapshot.resources.limits.worker.memoryMiB} MiB · {snapshot.resources.limits.worker.cpuMillis / 1000} CPU cores. One-time runtime build: {snapshot.resources.limits.runtimeBuild.memoryMiB} MiB · {snapshot.resources.limits.runtimeBuild.cpuMillis / 1000} CPU cores. Account limits can reduce these allocations; existing agents keep their limits. Larger tasks may need higher operator limits in Admin and the service configuration.</p>}
        <p>Waiting work stays queued while capacity recovers. Keep writing messages and editing project context; agent responses still need execution capacity. Cancel or pause queued work from Manage queue. Low limits may cause an agent to run out of memory: review retained output before an explicit restart.</p>
        <p>{host ? `${host.scope} · ${host.platform}. ${hostFresh ? 'Observed' : 'Stale observation from'} ${new Date(host.observedAt).toLocaleTimeString()}.` : 'Host observations have not arrived; capacity is unknown.'}</p>
        {host?.disks?.map(disk => <p key={disk.role}>{disk.role}: {hostFresh ? memory(disk.availableBytes) : 'Unavailable'} available</p>)}
        {host?.unavailable?.map((issue, index) => <p key={index}>{issue.metric}: {issue.reason}</p>)}
        <p>{execution ? `${execution.scope}: ${execution.status}. ${executionFresh && typeof execution.cpuMillis === 'number' ? (execution.cpuMillis / 1000).toFixed(1) + ' CPU cores available to the execution engine.' : 'Execution CPU capacity unavailable.'}` : 'Execution engine observations have not arrived.'}</p>
        {execution?.unavailable?.map((issue, index) => <p key={index}>{issue.metric}: {issue.reason}</p>)}
        <div className="overview-actions"><Button variant="secondary" disabled={changing || unconfirmed} onClick={() => open('dispatch')}>{snapshot.dispatchPaused ? 'Resume new starts' : 'Pause new starts'}</Button><Button variant="ghost" disabled={changing || unconfirmed} onClick={() => open('settings')}>Scheduling settings</Button><span className="overview-meta">Affects future starts across this workspace. Existing agents keep running.</span></div>
      </>}</div>
    </>}
    {changeError && !review && <p role="status">{changeError} <Button variant="ghost" disabled={changing} onClick={() => void refreshReview()}>Refresh workspace capacity</Button></p>}
    {review && draft && <WorkflowDialog title={review.mode === 'settings' ? 'Workspace scheduling ceilings' : review.dispatchPaused ? 'Pause new queued starts?' : 'Resume new queued starts?'} description={review.mode === 'settings' ? 'Review the limits used to admit future agents across every project in this workspace. Existing workers keep running; this does not raise account or provider limits.' : review.dispatchPaused ? 'Waiting tasks across every project stay queued until starts are resumed. Agents that have already started keep running and have separate stop controls.' : 'Previously authorized tasks across every project can start automatically when dependencies and capacity allow. Your model connections may incur usage charges.'} busy={changing} onClose={() => setReview(null)}>
      <form className="workflow-form" onSubmit={event => { event.preventDefault(); if (draftValid) void save(); }}>
        {review.mode === 'settings' ? <><label>Memory reserve profile<select value={cap > 0 ? 'responsive' : 'percentage'} disabled={changing} onChange={event => setDraft(previous => previous ? { ...previous, maxHeadroomMiB: event.target.value === 'responsive' ? 1024 : 0, ...(event.target.value === 'responsive' ? { minHeadroomMiB: 512 } : {}) } : previous)}><option value="responsive">Responsive local work</option><option value="percentage">Percentage reserve (saved settings)</option></select></label><p>Choosing responsive starts with a 1 GiB percentage RAM cap and a 512 MiB floor; you can review both values below. It keeps the CPU margin, exact workload demand and all capacity checks. Existing saved policies change only when you save this review.</p>{fields.map(field => <label key={field.key}>{field.label}<Input type="number" required min={field.min} max={field.max} step="1" value={Number.isNaN(draft[field.key]) ? '' : draft[field.key]} disabled={changing} onChange={event => setDraft(previous => previous ? { ...previous, [field.key]: event.target.value === '' ? NaN : Number(event.target.value) } : previous)} /></label>)}{cap !== 0 && <label>Maximum percentage RAM reserve (MiB)<Input type="number" required min={draft.minHeadroomMiB} max={65536} step="1" value={Number.isNaN(cap) ? '' : cap} disabled={changing} onChange={event => setDraft(previous => previous ? { ...previous, maxHeadroomMiB: event.target.value === '' ? NaN : Number(event.target.value) } : previous)} /></label>}{draft.maxPerProject > draft.maxActive && <p className="overview-warning">The per-project ceiling must not exceed the workspace ceiling.</p>}{cap > 0 && cap < draft.minHeadroomMiB && <p className="overview-warning">The RAM target cap must not be below the RAM floor.</p>}</> : <p>Current workspace ceiling: {review.snapshot.policy.maxActive} agents; {review.snapshot.policy.maxPerProject} per project. Actual starts remain subject to all resource, account and provider limits.</p>}
        <p className="overview-meta">Current saved settings: {review.snapshot.policy.maxActive} workspace / {review.snapshot.policy.maxPerProject} per project, {review.snapshot.policy.headroomPercent}% headroom. New starts {review.snapshot.dispatchPaused ? 'paused' : 'enabled'}.</p>
        {changeError && <p role="alert">{changeError} <Button variant="secondary" disabled={changing} onClick={() => void refreshReview()}>Refresh and review current settings</Button></p>}
        <div className="workflow-actions"><Button variant="secondary" disabled={changing} onClick={() => setReview(null)}>Keep current settings</Button><Button type="submit" loading={changing} disabled={!draftValid || unconfirmed}>{review.mode === 'settings' ? 'Save scheduling ceilings' : review.dispatchPaused ? 'Pause future starts' : 'Resume authorized starts'}</Button></div>
      </form>
    </WorkflowDialog>}
  </section>;
}

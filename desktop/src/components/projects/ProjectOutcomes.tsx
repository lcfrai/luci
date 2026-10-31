import { useEffect, useState } from 'react';
import { projectOverviewApi, type ProjectOutcomes as Outcomes } from '../../api/projectOverview';
import { errorText } from '../../api/product';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';
import { OutcomeCostReconciliation } from './OutcomeCostReconciliation';

export function ProjectOutcomes({ projectId, projectPath, refresh, disabled, onInspect }: {
  projectId: string; projectPath: string; refresh: number; disabled?: boolean; onInspect(runId: string): void;
}) {
  const scope = useProjectDraftScope(projectId), access = useProjectAccess(projectId), team = useTeamWorkspace();
  // Remove retained data immediately on account, workspace, grant or source-path
  // changes. The gateway independently rechecks fresh project reading authority.
  const key = JSON.stringify([scope, projectId, projectPath, team?.teamId, team?.revision, access.canRead, access.loading, access.error]);
  if (!scope || !access.canRead || access.loading || access.error || team?.accessBlocked || disabled) return null;
  return <ScopedOutcomes key={key} projectId={projectId} refresh={refresh} onInspect={onInspect} />;
}

function ScopedOutcomes({ projectId, refresh, onInspect }: { projectId: string; refresh: number; onInspect(runId: string): void }) {
  const [open, setOpen] = useState(false), [retry, setRetry] = useState(0);
  const [value, setValue] = useState<Outcomes | null>(null), [error, setError] = useState('');
  useEffect(() => {
    setValue(null); setError('');
    if (!open) return;
    const controller = new AbortController();
    void projectOverviewApi.get(projectId, controller.signal, { includeOutcomes: true }).then(result => {
      if (controller.signal.aborted) return;
      if (!result.outcomes) { setError('This service does not provide retained outcomes yet. Open project history for its recorded evidence.'); return; }
      setValue(result.outcomes);
    }).catch(cause => { if (!controller.signal.aborted) setError(errorText(cause)); });
    return () => controller.abort();
  }, [open, projectId, refresh, retry]);
  return <details className="overview-details overview-recorded-outcomes" open={open} onToggle={event => setOpen(event.currentTarget.open)}>
    <summary>Recorded outcomes and review work</summary>
    {open && <>
      {!value && !error && <p role="status">Reading retained run evidence…</p>}
      {error && <p role="status">Outcomes unavailable. {error}</p>}
      {value?.status === 'unavailable' && <p role="status">Retained run history could not be read. Acceptance, review and repair counts are unknown.</p>}
      {value?.status === 'recorded' && <>
        <dl className="overview-counts"><div><dt>Recorded acceptances</dt><dd>{value.recordedAcceptances}</dd></div><div><dt>Runs needing review</dt><dd>{value.reviewRequiredRuns}</dd></div><div><dt>Unresolved check-repair tasks</dt><dd>{value.unresolvedCheckRepairTasks}</dd></div></dl>
        {value.latestAccepted && <p>Latest recorded acceptance: <time dateTime={value.latestAccepted.acceptedAt}>{new Date(value.latestAccepted.acceptedAt).toLocaleString()}</time> · {value.latestAccepted.target}. <Button variant="ghost" onClick={() => onInspect(value.latestAccepted!.runId)}>Inspect accepted run</Button></p>}
        {value.measurements ? <section aria-label="Measured retained work"><h4>Measured retained work</h4><dl className="overview-counts"><div><dt>Summed measured task runtime</dt><dd>{duration(value.measurements.taskRuntimeMilliseconds)}</dd></div><div><dt>Run-to-accept elapsed</dt><dd>{duration(value.measurements.runToAcceptanceMilliseconds)}</dd></div><div><dt>Recorded control actions</dt><dd>{value.measurements.recordedHumanActions ?? 'Unknown'}</dd></div></dl></section> : <p>This retained history has no task-timing or control-action instrumentation.</p>}
        <p className="journey-muted">Coverage: {value.retainedRuns} retained runs; {value.omittedRuns} omitted. Timing and control actions cover recorded paths; missing measurements remain unknown.</p>
        <details className="overview-details"><summary>How these numbers are counted</summary>
          <p>{value.retainedRuns} retained runs inspected (maximum 200); {value.omittedRuns} omitted from this retained set. These are not lifetime totals.</p>
          {!!value.unconfirmedAcceptedObservations && <p>{value.unconfirmedAcceptedObservations} accepted records have an unconfirmed retained observation. Their historical acceptance is preserved.</p>}
          <p>Acceptance joins saved check and release evidence. It does not confirm current production health. Review counts include recorded questions and blocked steps; repair counts deduplicate unresolved tasks within each run.</p>
          {value.measurements && <>
            <p>{value.measurements.measuredTaskIntervals} original container intervals measured; {value.measurements.unknownTaskIntervals} unknown; {value.measurements.recordedNoWorkerAttempts} recorded pre-allocation failures. Parallel task durations overlap and are not project elapsed time.</p>
            <p>Run-to-accept time includes waiting. Control counts cover recorded start, cancel, check/release joins, acceptance, reconciliation and swarm pause/resume/cancel changes; external edits and actions are not measured. {value.measurements.runsWithEarlierUninstrumentedActions} runs have earlier uninstrumented history; {value.measurements.omittedHumanActions} older recorded actions are outside the retained window.</p>
            <p>These measurements have no comparison baseline and do not establish a speed multiplier, task correctness or actual spend.</p>
          </>}
          <p>Waiting and running work appears in the recorded queue above.</p>
        </details>
      </>}
      <OutcomeCostReconciliation projectId={projectId} active={open} readOnly onEdit={()=>onInspect('')}/>
      {(value || error) && <Button variant="ghost" onClick={() => setRetry(previous => previous + 1)}>Refresh outcomes</Button>}
    </>}
  </details>;
}

function duration(milliseconds:number|null):string {
  if(milliseconds===null)return 'Unknown';
  if(milliseconds<1000)return `${milliseconds} ms`;
  if(milliseconds<60000)return `${(milliseconds/1000).toLocaleString(undefined,{maximumFractionDigits:1})} s`;
  if(milliseconds<3600000)return `${(milliseconds/60000).toLocaleString(undefined,{maximumFractionDigits:1})} min`;
  return `${(milliseconds/3600000).toLocaleString(undefined,{maximumFractionDigits:1})} h`;
}

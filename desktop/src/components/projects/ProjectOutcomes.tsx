import { useEffect, useState } from 'react';
import { projectOverviewApi, type ProjectOutcomes as Outcomes } from '../../api/projectOverview';
import { errorText } from '../../api/product';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useProjectAccess } from '../../hooks/useProjectAccess';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';

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
        <p>{value.retainedRuns} retained runs inspected (maximum 200); {value.omittedRuns} omitted from this retained set. These are not lifetime totals.</p>
        {!!value.unconfirmedAcceptedObservations && <p>{value.unconfirmedAcceptedObservations} accepted records have an unconfirmed retained observation. Their historical acceptance is preserved.</p>}
        {value.latestAccepted && <p>Latest recorded acceptance: <time dateTime={value.latestAccepted.acceptedAt}>{new Date(value.latestAccepted.acceptedAt).toLocaleString()}</time> · {value.latestAccepted.target}. <Button variant="ghost" onClick={() => onInspect(value.latestAccepted!.runId)}>Inspect accepted run</Button></p>}
        <p>Acceptance joins saved check and release evidence. It does not confirm current production health. Review counts include recorded questions and blocked steps; repair counts deduplicate unresolved tasks within each run.</p>
        <p>Waiting and running work appears in the recorded queue above. Costs and elapsed effort are not summarized here.</p>
      </>}
      {(value || error) && <Button variant="ghost" onClick={() => setRetry(previous => previous + 1)}>Refresh outcomes</Button>}
    </>}
  </details>;
}

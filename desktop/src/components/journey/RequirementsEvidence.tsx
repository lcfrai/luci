import type { CheckRun, RequirementCoverage, RequirementEvidenceStatus } from "../../api/product";
import { Button } from "../fluid/components/button";
import "./requirements-evidence.css";

const changed = "luci-requirement-evidence-changed";
export function notifyRequirementEvidenceChanged(projectId: string) {
  window.dispatchEvent(new CustomEvent(changed, { detail: projectId }));
}
export function watchRequirementEvidence(projectId: string, refresh: () => void) {
  const listener = (event: Event) => {
    if ((event as CustomEvent<string>).detail === projectId) refresh();
  };
  window.addEventListener(changed, listener);
  return () => window.removeEventListener(changed, listener);
}
export function evidenceLabel(status?: RequirementEvidenceStatus) {
  return status === "linked-test-evidence" ? "Current linked test evidence" : status === "stale" ? "Stale evidence" : "Unverified";
}
export function commandStatus(status: string) {
  return status === "passed" ? "Commands passed" : status === "failed" ? "Commands failed" : ["cancelled", "canceled"].includes(status) ? "Commands canceled" : status === "running" ? "Commands running" : status === "queued" ? "Commands queued" : status;
}
export function RequirementsEvidence({ coverage, runs, refreshing, onRefresh }: {
  coverage: RequirementCoverage | null;
  runs: CheckRun[];
  refreshing: boolean;
  onRefresh: () => void;
}) {
  const current = coverage?.rows.filter(row => row.evidenceStatus === "linked-test-evidence").length ?? 0;
  return <details className="requirements-evidence">
    <summary>Requirement evidence{coverage ? ` · ${current} of ${coverage.rows.length} with current linked tests` : " · unavailable"}</summary>
    <p className="journey-muted">Linked test evidence records the selected commands and their source. Review what those checks cover; passing commands and tasks marked Done do not establish requirement satisfaction.</p>
    <Button type="button" variant="secondary" disabled={refreshing} onClick={onRefresh}>{refreshing ? "Refreshing evidence…" : "Refresh requirement evidence"}</Button>
    {!coverage ? <p className="journey-muted">Requirement evidence could not be loaded. You can still run generic project checks.</p> : <>
      <p className="requirements-context">Requirements revision {coverage.specification.revision} · {coverage.specification.status}; design revision {coverage.design.revision} · {coverage.design.status}. {coverage.source.available ? `Source ${coverage.source.commit ? coverage.source.commit.slice(0, 12) : "without a commit"}${coverage.source.dirty ? " · includes uncommitted changes" : " · clean working tree"}.` : "Current source could not be read."}</p>
      {!coverage.rows.length && <p className="journey-muted">Add and approve requirements in Define to record linked evidence.</p>}
      <ul className="requirements-evidence-list">{coverage.rows.map(row => {
        const latest = runs.filter(run => row.evidenceRunIds.includes(run.id)).sort((a, b) => b.createdAt.localeCompare(a.createdAt))[0];
        return <li key={row.requirementId}>
          <div className="requirements-evidence-heading"><strong>{row.requirementId} · {row.title}</strong><span>{evidenceLabel(row.evidenceStatus)}</span></div>
          <p>{row.acceptance}</p>
          <small>{row.taskIds.length ? `Linked tasks: ${row.taskIds.join(", ")}. ${row.doneTaskIds.length} marked Done by you.` : "No linked tasks."}</small>
          {latest ? <p className="requirements-latest">Latest linked run {latest.id} · {commandStatus(latest.status)} · {evidenceLabel(latest.evidenceStatus)}<br /><small>{latest.commit ? latest.commit.slice(0, 12) : "No commit"}{latest.dirty ? " · includes uncommitted changes" : " · clean working tree"} · requirements revision {latest.specificationRevision ?? 0} · design revision {latest.designPlanRevision ?? 0}</small></p> : <p className="journey-muted">No linked check run recorded.</p>}
          {latest?.staleReasons?.length ? <ul>{latest.staleReasons.map((reason, index) => <li key={index}>{reason}</li>)}</ul> : null}
        </li>;
      })}</ul>
    </>}
  </details>;
}

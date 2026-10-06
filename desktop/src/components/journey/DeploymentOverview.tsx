import { useEffect, useRef, useState } from "react";
import { productApi, errorText } from "../../api/product";
import { platformApi } from "../../api/platform";
import { PROJECT_CONTEXT_CHANGED, type ProjectContextIdentity } from "../../api/projectContextEvents";
import { Button } from "../fluid/components/button";

export type DeploymentInspection = { kind: "release" | "platform"; id: string };
type Summary = DeploymentInspection & { title: string; status: string; message: string; commit: string; at: string };
export function DeploymentOverview({ projectId, scope, disabled, onInspect }: {
  projectId: string; scope: string | null; disabled: boolean; onInspect(target: DeploymentInspection): void;
}) {
  const [rows, setRows] = useState<Summary[]>([]), [issues, setIssues] = useState<string[]>([]), [loading, setLoading] = useState(true);
  const generation = useRef(0), alive = useRef(true);
  async function reload() {
    const ticket = ++generation.current;
    setLoading(true);
    const results = await Promise.allSettled([productApi.releases(projectId), platformApi.project(projectId)]);
    if (!alive.current || ticket !== generation.current) return;
    const next: Summary[] = [], errors: string[] = [];
    const releases = results[0], platform = results[1];
    if (releases.status === "fulfilled") {
      if (releases.value.releases.some(row => row.projectId !== projectId)) errors.push("Release history did not match this project. Refresh before continuing.");
      else next.push(...releases.value.releases.map(row => ({ kind: "release" as const, id: row.id, title: row.target === "local" ? "Workspace preview" : row.target === "github-pages" ? "Published static site" : "Cloud application", status: row.status, message: row.message, commit: row.commit, at: row.createdAt })));
    } else errors.push("Preview and publishing history: " + errorText(releases.reason));
    if (platform.status === "fulfilled") {
      if (platform.value.project.projectId !== projectId || platform.value.project.deployments.some(row => row.projectId !== projectId)) errors.push("Application services did not match this project. Refresh before continuing.");
      else next.push(...platform.value.project.deployments.map(row => ({ kind: "platform" as const, id: row.id, title: row.provider === "docker" ? "Persistent application" : "Hosted application", status: row.status, message: row.message, commit: row.commit, at: row.updatedAt || row.createdAt })));
    } else errors.push("Persistent application history: " + errorText(platform.reason));
    setRows(next.sort((a, b) => b.at.localeCompare(a.at))); setIssues(errors); setLoading(false);
  }
  useEffect(() => {
    alive.current = true; void reload();
    const changed = (event: Event) => { const id = (event as CustomEvent<ProjectContextIdentity>).detail; if (scope && id?.scope === scope && id.projectId === projectId) void reload(); };
    window.addEventListener(PROJECT_CONTEXT_CHANGED, changed);
    return () => { alive.current = false; ++generation.current; window.removeEventListener(PROJECT_CONTEXT_CHANGED, changed); };
  }, [projectId, scope]);
  return <section className="journey-section" aria-label="Application status and recovery">
    <div className="journey-header"><div><h2>Your applications</h2><p>Find a retained release before starting another build. Saved health is a previous observation; inspect the selected app to check its current behavior.</p></div>
      <Button variant="secondary" disabled={loading || disabled} onClick={() => void reload()}>Refresh application history</Button></div>
    {loading && <p role="status">Reading this project's releases and application services…</p>}
    {issues.map(issue => <p role="alert" key={issue}>{issue}</p>)}
    {!loading && !rows.length && !issues.length && <p>No application has been released from this project yet. Start with a checked preview or prepare a persistent deployment below.</p>}
    <ul className="deploy-releases">{rows.slice(0, 4).map(row => <li key={row.kind + row.id}>
      <div><strong>{row.title}</strong><span>{row.status === "ready" ? "Last verified healthy" : row.status === "unknown" || row.status === "attention" ? "Needs verification" : row.status}</span></div>
      <p>{row.message}</p><small>Source {row.commit ? row.commit.slice(0, 12) : "not recorded"} · {row.at ? new Date(row.at).toLocaleString() : "Time unavailable"}</small>
      <Button variant="secondary" disabled={disabled} onClick={() => onInspect({ kind: row.kind, id: row.id })}>Inspect {row.title.toLowerCase()}</Button>
    </li>)}</ul>
    {rows.length > 4 && <p>{rows.length - 4} earlier releases remain in the histories below.</p>}
  </section>;
}

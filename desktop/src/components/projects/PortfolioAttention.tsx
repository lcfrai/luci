import { useEffect, useId, useState } from 'react';
import { portfolioApi, type Portfolio, type PortfolioIntervention } from '../../api/portfolio';
import { teamScopeGeneration } from '../../api/teamScope';
import { errorText } from '../../api/product';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useCockpit } from '../../store/store';
import type { Project } from '../../store/types';
import { useTeamWorkspace } from '../teams/TeamWorkspaceProvider';
import { Button } from '../fluid/components/button';
import { useProjectHome } from './ProjectHomeProvider';

export function PortfolioAttention({ projects }: { projects: Project[] }) {
  const scope = useProjectDraftScope('portfolio-attention');
  const team = useTeamWorkspace();
  const key = JSON.stringify([scope, teamScopeGeneration(), team?.teamId, team?.revision, team?.selected?.permissions,
    projects.map(p => [p.id, p.path])]);
  if (!scope || team?.accessBlocked) return null;
  // Remount before rendering a changed actor/workspace/membership projection.
  return <ScopedAttention key={key} projects={projects} />;
}

function ScopedAttention({ projects }: { projects: Project[] }) {
  const heading = useId();
  const [snapshot, setSnapshot] = useState<Portfolio | null>(null);
  const [error, setError] = useState('');
  const [loading, setLoading] = useState(true);
  const [refresh, setRefresh] = useState(0);
  const [limit, setLimit] = useState(5);
  const generation = teamScopeGeneration();
  useEffect(() => {
    const controller = new AbortController();
    setLoading(true); setSnapshot(null); setError('');
    void portfolioApi.get(controller.signal).then(value => {
      if (!controller.signal.aborted && generation === teamScopeGeneration()) setSnapshot(value);
    }).catch(cause => {
      if (!controller.signal.aborted && generation === teamScopeGeneration()) setError(errorText(cause));
    }).finally(() => {
      if (!controller.signal.aborted && generation === teamScopeGeneration()) setLoading(false);
    });
    return () => controller.abort();
  }, [refresh, generation]);
  const allowed = new Map(projects.map(project => [project.id, project]));
  const items = snapshot?.interventions.filter(item => allowed.has(item.projectId)) ?? [];
  return <section className="project-overview-board" aria-labelledby={heading}>
    <header className="overview-heading"><div><h2 id={heading}>Needs your attention</h2>
      <p>Resolve decisions and blockers across your projects.</p></div>
      <Button variant="secondary" disabled={loading} onClick={() => setRefresh(value => value + 1)}>Refresh attention</Button>
    </header>
    {loading && <p role="status">Reading saved project evidence…</p>}
    {error && <p className="journey-error" role="alert">{error}</p>}
    {snapshot && <>
      <p className="overview-meta">Saved project reviews, work queues and your Luci follow-ups. Open a project to check its current state.</p>
      {snapshot.status === 'partial' && <p className="overview-warning" role="status">Some evidence is unavailable or omitted. An empty list does not establish that every project is clear.</p>}
      {snapshot.omittedProjects > 0 && <p>{snapshot.omittedProjects} additional projects are outside this bounded view. Open their detailed overview.</p>}
      {items.length === 0 && <p>{snapshot.status === 'ready' ? 'No recorded interventions for the projects shown.' : 'No confirmed interventions to show. Refresh or open a project to inspect its evidence.'}</p>}
      <ol className="home-activity-list">{items.slice(0, limit).map(item => {
        const project = snapshot.projects.find(p => p.id === item.projectId);
        return <li key={item.id}><div><p className="overview-meta">{allowed.get(item.projectId)?.name} · {item.kind === 'unconfirmed' ? 'Outcome unconfirmed' : item.kind}</p>
          <h3>{item.title}</h3><p>{item.reason}</p><p><strong>What it holds up:</strong> {item.blocks}</p>
          {project?.coordinator.status === 'recorded' && <p className="overview-meta">Saved Luci continuation: {project.coordinator.state}{project.coordinator.unresolved > 0 ? ` · ${project.coordinator.unresolved} unresolved` : ''}.</p>}
          {project?.coordinator.status === 'unavailable' && <p className="overview-meta">Luci continuation unavailable.</p>}
          <details><summary>Recorded evidence</summary><ul>{item.evidence.map(e => <li key={`${e.source}:${e.id}`}>{e.source}: <code>{e.id}</code>{e.revision ? ` · revision ${e.revision}` : ''}{e.observedAt ? ` · ${e.observedAt}` : ''}</li>)}</ul></details>
        </div><InterventionDestination item={item} project={allowed.get(item.projectId)!} generation={generation} /></li>;
      })}</ol>
      {items.length > limit && <Button variant="secondary" onClick={() => setLimit(value => value + 5)}>Show more decisions and blockers</Button>}
      {snapshot.omittedInterventions > 0 && <p>{snapshot.omittedInterventions} additional interventions are omitted. Inspect project queues for the full retained records.</p>}
    </>}
  </section>;
}

function InterventionDestination({ item, project, generation }: { item: PortfolioIntervention; project: Project; generation: number }) {
  const scope = useProjectDraftScope(project.id);
  const home = useProjectHome();
  function open() {
    if (!scope || generation !== teamScopeGeneration()) return;
    const lens = item.nextAction.lens;
    if (home) home.open(project.id, lens); else useCockpit.getState().selectProject(project.id, lens);
    const state = useCockpit.getState(), pending = state.pendingNavigation;
    const accepted = pending ? pending.kind === 'project' && pending.projectId === project.id && pending.lens === lens
      : state.selection.projectId === project.id && state.lens === lens;
    if (!accepted || !state.projects.some(p => p.id === project.id && p.path === project.path)) return;
    // Navigation intent only. The destination reloads current evidence and
    // owns approvals; the ordinary unsaved-draft guard remains in control.
    useCockpit.setState({ projectRunInspection: lens === 'develop' && item.nextAction.runId
      ? { scope, projectId: project.id, runId: item.nextAction.runId, refresh: true } : null });
    if (item.nextAction.id === 'open-conversation' && !state.pendingNavigation && state.selection.projectId === project.id) state.showLuci();
  }
  return <Button variant="secondary" disabled={!scope} onClick={open}>{item.nextAction.label}</Button>;
}

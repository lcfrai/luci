import type { GoalsView } from '../../api/portfolioGoals';
import { teamScopeGeneration } from '../../api/teamScope';
import type { Project } from '../../store/types';
import { PortfolioActionDestination } from './PortfolioActionDestination';

export function PortfolioOutcomeRoutes({ view, projects }: { view: GoalsView; projects: Project[] }) {
  const routing = view.outcomeRouting, generation = teamScopeGeneration();
  if (!routing) return <p className="journey-muted">Goal-linked work is unavailable in this workspace version. Saved goals and explicit priority review remain available.</p>;
  const allowed = new Map(projects.map(project => [project.id, project]));
  return <section aria-label="Goals linked to current work">
    <h3>What your goals are linked to</h3>
    <p>See where your saved goals meet current work. Open the next decision, your project Luci conversation or its retained run.</p>
    {routing.status === 'partial' && <p role="status">Some work evidence is unavailable or outside this bounded view. No entry here is proof that a goal is finished.</p>}
    {(routing.omittedProjects > 0 || routing.omittedRoutes > 0) && <p>{routing.omittedProjects} projects and {routing.omittedRoutes} goal-to-project links are outside this view. At most 16 distinct projects and 32 links are shown.</p>}
    {routing.unknownProjects > 0 && <p>{routing.unknownProjects} inspected projects have unconfirmed or incomplete evidence.</p>}
    {view.goals.map(goal => {
      const routes = routing.routes.filter(route => route.goalId === goal.id && route.goalRevision === goal.revision);
      return <section key={goal.id}><h4>{goal.title} · saved revision {goal.revision}</h4><p>{goal.outcome}</p>
        {routes.length === 0 && <p>No confirmed link is shown in this bounded read. Inspect the selected projects individually.</p>}
        <ul className="home-activity-list">{routes.map(route => {
          const project = allowed.get(route.projectId);
          if (!project) return null;
          return <li key={route.id}><div><strong>{project.name}</strong><p>{route.status === 'recorded' ? 'Recorded continuation' : route.status === 'attention' ? 'Needs attention' : route.status === 'not-configured' ? 'No saved continuation' : 'Evidence unconfirmed'}: {route.reason}</p>
            {route.coordinator && <p className="overview-meta">Your continuation {route.coordinator.id} · revision {route.coordinator.revision} · {route.coordinator.state}{route.coordinator.runId ? ` · run ${route.coordinator.runId}` : ''}. This is retained evidence, not a completion claim.</p>}
            <details><summary>Linked evidence</summary><ul>{route.evidence.map((row, index) => <li key={`${row.source}:${row.id}:${index}`}>{row.source}: <code>{row.id}</code>{row.revision !== undefined ? ` · revision ${row.revision}` : ''}{row.observedAt ? ` · ${row.observedAt}` : ''}</li>)}</ul></details>
          </div><PortfolioActionDestination action={route.nextAction} project={project} generation={generation} /></li>;
        })}</ul>
      </section>;
    })}
    {view.goals.length === 0 && <p>Save a goal and select its projects to link it to existing work.</p>}
    <p className="journey-muted">Observed {routing.observedAt}. Sources are read separately; current project permissions, approvals, capacity and execution controls remain authoritative.</p>
    <details><summary>About these links</summary><p>Links are read-only observations, not instructions to start or advance work. Your private goal text stays out of shared project conversations. Review any continuation through its existing project controls.</p></details>
  </section>;
}

import { api } from '../../api';
import { teamScopeGeneration } from '../../api/teamScope';
import { useCockpit } from '../../store/store';
import type { CatalogueProblem } from '../../store/slices/dataSlice';
import { useSetup } from '../setup/SetupProvider';
import { Button } from '../fluid/components/button';
import './api-problem-notice.css';

export function ApiProblemNotice({ problem, id }: { problem: CatalogueProblem; id?: string }) {
  const setup = useSetup();
  const epoch = useCockpit(state => state.projectCatalogueEpoch);
  const origin = problem.branchLoad ?? problem.sessionCreate;
  const project = useCockpit(state => origin ? state.projects.find(item => item.id === origin.projectId) : undefined);
  const loading = useCockpit(state => origin ? !!state.loading[`branches:${origin.projectId}`] : false);
  const branchTimeout = problem.code === 'timeout' && !!problem.branchLoad;
  const timeout = problem.code === 'timeout' && !problem.sessionCreate;
  const current = !origin || (origin.catalogueEpoch === epoch && origin.workspaceGeneration === teamScopeGeneration() && origin.service === api.baseUrl);
  // Never expose a previous workspace's route or retry it through the new one.
  if (!current) return null;
  return <div id={id} className="api-problem-notice" role="alert" tabIndex={-1}>
    <p><span className="api-problem-mark" aria-hidden="true" />{branchTimeout
      ? project ? `Branches for ${project.name} took too long to load.` : 'The project’s branches took too long to load.'
      : timeout ? 'The workspace took too long to respond. Try refreshing.'
      : problem.message}</p>
    {problem.sessionCreate ? <Button type="button" variant="ghost" onClick={() => {
      // Dismissing this operation must not clear a newer, unrelated failure.
      const state = useCockpit.getState();
      if (state.lastError === problem) state.setError(null);
    }}>Dismiss</Button> : branchTimeout ? <>
      <Button type="button" variant="ghost" loading={loading} disabled={!project || !setup?.retryBranches}
        onClick={() => setup?.retryBranches?.(problem)}>Retry branches</Button>
      <details><summary>Technical details</summary><p>{problem.message}</p><code>{problem.code}</code></details>
    </> : <>
      <Button type="button" variant="ghost" onClick={() => setup?.refreshProjects()}>Refresh projects</Button>
      {timeout && <details><summary>Technical details</summary><p>{problem.message}</p><code>{problem.code}</code></details>}
    </>}
  </div>;
}

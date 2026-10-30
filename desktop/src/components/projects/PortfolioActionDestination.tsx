import type { PortfolioIntervention } from '../../api/portfolio';
import { teamScopeGeneration } from '../../api/teamScope';
import { useProjectDraftScope } from '../../hooks/useProjectDraft';
import { useCockpit } from '../../store/store';
import type { Project } from '../../store/types';
import { Button } from '../fluid/components/button';
import { useProjectHome } from './ProjectHomeProvider';

export function PortfolioActionDestination({ action, project, generation }: { action: PortfolioIntervention['nextAction']; project: Project; generation: number }) {
  const scope = useProjectDraftScope(project.id);
  const home = useProjectHome();
  function open() {
    if (!scope || generation !== teamScopeGeneration()) return;
    const lens = action.lens;
    if (action.id === 'open-conversation') {
      if (home) home.open(project.id, lens, 'luci'); else useCockpit.getState().selectProject(project.id, lens, 'luci');
    } else {
      if (home) home.open(project.id, lens); else useCockpit.getState().selectProject(project.id, lens);
    }
    const state = useCockpit.getState(), pending = state.pendingNavigation;
    const accepted = pending ? pending.kind === 'project' && pending.projectId === project.id && pending.lens === lens
      : state.selection.projectId === project.id && state.lens === lens;
    if (!accepted || !state.projects.some(p => p.id === project.id && p.path === project.path)) return;
    // Navigation intent only. The destination reloads current evidence and
    // owns approvals; the ordinary unsaved-draft guard remains in control.
    useCockpit.setState({ projectRunInspection: lens === 'develop' && action.runId
      ? { scope, projectId: project.id, runId: action.runId, refresh: true } : null });
  }
  return <Button variant="secondary" disabled={!scope} onClick={open}>{action.label}</Button>;
}

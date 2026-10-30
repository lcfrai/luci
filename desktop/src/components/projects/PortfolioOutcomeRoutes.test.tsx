import { cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { PortfolioOutcomeRoutes } from './PortfolioOutcomeRoutes';
import { portfolioGoalsApi, type GoalsView } from '../../api/portfolioGoals';
import { selectTeamScope } from '../../api/teamScope';
import { resetCockpit, useCockpit } from '../../store/store';

vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: (id: string) => `alice:workspace:${id}` }));
vi.mock('./ProjectHomeProvider', () => ({ useProjectHome: () => null }));
const projects = [{ id: 'p-one', name: 'Appointments', path: '/appointments' }, { id: 'p-two', name: 'Shop', path: '/shop' }];
function view(): GoalsView {
  return { revision: 2, partial: false, applications: [], goals: [{ id: 'goal-one', revision: 3, title: 'Book appointments', outcome: 'Customers select an available slot', updatedAt: '2026-10-30T00:00:00Z', projects: [{ projectId: 'p-one', priority: 'high', requestedSlots: 2, dependsOn: [] }] }],
    outcomeRouting: { version: 1, status: 'ready', observedAt: '2026-10-30T00:00:00Z', readProjects: 1, omittedProjects: 0, omittedRoutes: 0, unknownProjects: 0,
      routes: [{ id: 'a'.repeat(64), goalId: 'goal-one', goalRevision: 3, projectId: 'p-one', status: 'recorded', reason: 'Inspect retained work.', coordinator: { status: 'recorded', id: 'continue-alice', revision: 7, state: 'working', runId: 'run-one', updatedAt: '2026-10-30T00:00:00Z', unresolved: 0 }, evidence: [{ source: 'coordinator', id: 'continue-alice', revision: 7 }], nextAction: { id: 'inspect-run', label: 'Inspect recorded work', lens: 'develop', runId: 'run-one' } }] } };
}
beforeEach(() => { vi.restoreAllMocks(); selectTeamScope(''); resetCockpit(); useCockpit.getState().setProjects(projects); vi.spyOn(portfolioGoalsApi, 'apply'); vi.spyOn(portfolioGoalsApi, 'review'); });
afterEach(() => { cleanup(); selectTeamScope(''); });

it('shows the saved goal and exact owned continuation, then opens its existing run without applying', async () => {
  render(<PortfolioOutcomeRoutes view={view()} projects={projects} />);
  expect(screen.getByRole('heading', { name: 'Book appointments · saved revision 3' })).not.toBeNull();
  expect(screen.getByText(/Your continuation continue-alice · revision 7/)).not.toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Inspect recorded work' }));
  expect(useCockpit.getState().selection.projectId).toBe('p-one');
  expect(useCockpit.getState().workspaceSurface).toBe('tools');
  expect(useCockpit.getState().projectRunInspection).toEqual({ scope: 'alice:workspace:p-one', projectId: 'p-one', runId: 'run-one', refresh: true });
  expect(portfolioGoalsApi.apply).not.toHaveBeenCalled(); expect(portfolioGoalsApi.review).not.toHaveBeenCalled();
});

it('keeps the ordinary unsaved-draft guard when routing to another project', async () => {
  useCockpit.getState().selectProject('p-two');
  useCockpit.getState().setUnsavedProjectDraft('existing', { projectId: 'p-two', label: 'Shop definition', lens: 'discover' });
  render(<PortfolioOutcomeRoutes view={view()} projects={projects} />);
  await userEvent.click(screen.getByRole('button', { name: 'Inspect recorded work' }));
  await waitFor(() => expect(useCockpit.getState().pendingNavigation).toMatchObject({ kind: 'project', projectId: 'p-one', lens: 'develop' }));
  expect(useCockpit.getState().selection.projectId).toBe('p-two');
  expect(portfolioGoalsApi.apply).not.toHaveBeenCalled();
});

it('keeps unknown and omitted work explicit and hides unavailable project destinations', () => {
  const value = view(); value.outcomeRouting!.status = 'partial'; value.outcomeRouting!.unknownProjects = 1; value.outcomeRouting!.routes[0].status = 'unknown';
  render(<PortfolioOutcomeRoutes view={value} projects={[]} />);
  expect(screen.getByText(/Some work evidence is unavailable/)).not.toBeNull();
  expect(screen.getByText(/1 inspected projects have unconfirmed/)).not.toBeNull();
  expect(screen.queryByRole('button', { name: 'Inspect recorded work' })).toBeNull();
});

it('does not route a captured action after the workspace generation changes', async () => {
  render(<PortfolioOutcomeRoutes view={view()} projects={projects} />);
  selectTeamScope('');
  await userEvent.click(screen.getByRole('button', { name: 'Inspect recorded work' }));
  expect(useCockpit.getState().selection.projectId).toBeNull();
  expect(useCockpit.getState().projectRunInspection).toBeNull();
});

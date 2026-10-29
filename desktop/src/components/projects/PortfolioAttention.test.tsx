import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { PortfolioAttention } from './PortfolioAttention';
import { portfolioApi, type Portfolio } from '../../api/portfolio';
import { resetCockpit, useCockpit } from '../../store/store';

const environment = vi.hoisted(() => ({ scope: 'actor-one', revision: 1, blocked: false }));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: (id: string) => `${environment.scope}:${id}` }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => ({ teamId: '', revision: environment.revision, accessBlocked: environment.blocked }) }));
vi.mock('./ProjectHomeProvider', () => ({ useProjectHome: () => null }));
const projects = [{ id: 'p-one', name: 'Notes', path: '/notes' }, { id: 'p-two', name: 'Shop', path: '/shop' }];
function snapshot(): Portfolio {
  return { version: 1, observedAt: '2026-10-29T00:00:00Z', status: 'ready', totalProjects: 1, omittedProjects: 0, omittedInterventions: 0,
    projects: [{ id: 'p-one', name: 'Notes', status: 'ready', coordinator: { status: 'recorded', id: 'continue-one', revision: 3, state: 'blocked', updatedAt: '2026-10-29T00:00:00Z', unresolved: 1 } }],
    interventions: [{ id: 'a'.repeat(64), projectId: 'p-one', kind: 'blocked', title: 'Review blocked work', reason: 'This attempt needs review.', blocks: 'Affected task: Build notes.',
      nextAction: { id: 'inspect-run', label: 'Review project work', lens: 'develop', runId: 'run-one' }, evidence: [{ source: 'queue-item', id: 'queue-one', revision: 8 }] }] };
}
beforeEach(() => {
  vi.restoreAllMocks(); resetCockpit(); useCockpit.getState().setProjects(projects);
  environment.scope = 'actor-one'; environment.revision = 1; environment.blocked = false;
  vi.spyOn(portfolioApi, 'get').mockResolvedValue(snapshot());
});
afterEach(cleanup);

it('shows causal task and coordinator evidence and opens the exact existing run', async () => {
  const user = userEvent.setup(); render(<PortfolioAttention projects={projects} />);
  expect(await screen.findByText('Review blocked work')).toBeInTheDocument();
  expect(screen.getByText(/Affected task: Build notes/)).toBeInTheDocument();
  expect(screen.getByText(/Saved Luci continuation: blocked/)).toBeInTheDocument();
  await user.click(screen.getByRole('button', { name: 'Review project work' }));
  expect(useCockpit.getState().selection.projectId).toBe('p-one');
  expect(useCockpit.getState().lens).toBe('develop');
  expect(useCockpit.getState().projectRunInspection).toEqual({ projectId: 'p-one', scope: 'actor-one:p-one', runId: 'run-one', refresh: true });
});

it('withdraws old actor responses before a new scope can render them', async () => {
  let resolveOld!: (value: Portfolio) => void;
  vi.mocked(portfolioApi.get).mockImplementationOnce(() => new Promise(resolve => { resolveOld = resolve; }));
  const view = render(<PortfolioAttention projects={projects} />);
  environment.scope = 'actor-two';
  const empty = snapshot(); empty.interventions = []; empty.projects[0].coordinator = { status: 'none', unresolved: 0 };
  vi.mocked(portfolioApi.get).mockResolvedValue(empty);
  view.rerender(<PortfolioAttention projects={projects} />);
  await screen.findByText('No recorded interventions for the projects shown.');
  await act(async () => { resolveOld(snapshot()); });
  expect(screen.queryByText('Review blocked work')).not.toBeInTheDocument();
});

it('removes revoked membership immediately and keeps partial empty results uncertain', async () => {
  const partial = snapshot(); partial.status = 'partial'; partial.interventions = [];
  vi.mocked(portfolioApi.get).mockResolvedValue(partial);
  const view = render(<PortfolioAttention projects={projects} />);
  await screen.findByText(/No confirmed interventions to show/);
  expect(screen.queryByText('No recorded interventions for the projects shown.')).not.toBeInTheDocument();
  environment.blocked = true;
  view.rerender(<PortfolioAttention projects={projects} />);
  expect(screen.queryByRole('heading', { name: 'Needs your attention' })).not.toBeInTheDocument();
});

it('retains the normal unsaved-project navigation guard when opening an intervention', async () => {
  const user = userEvent.setup();
  useCockpit.getState().selectProject('p-two');
  useCockpit.getState().setUnsavedProjectDraft('one', { projectId: 'p-two', label: 'Unsaved definition', lens: 'discover' });
  render(<PortfolioAttention projects={projects} />);
  await user.click(await screen.findByRole('button', { name: 'Review project work' }));
  await waitFor(() => expect(useCockpit.getState().pendingNavigation).toMatchObject({ kind: 'project', projectId: 'p-one', lens: 'develop' }));
  expect(useCockpit.getState().selection.projectId).toBe('p-two');
});

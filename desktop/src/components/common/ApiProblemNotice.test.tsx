import { fireEvent, render, screen, within } from '@testing-library/react';
import { beforeEach, expect, it, vi } from 'vitest';
import { api } from '../../api';
import { selectTeamScope, teamScopeGeneration } from '../../api/teamScope';
import { resetCockpit, useCockpit } from '../../store/store';
import type { CatalogueProblem } from '../../store/slices/dataSlice';
import { aProject } from '../../test/fixtures';
import { ApiProblemNotice } from './ApiProblemNotice';
import { RailFooter } from '../rail/RailFooter';

const setup = vi.hoisted(() => ({ retryBranches: vi.fn(), refreshProjects: vi.fn(), openSettings: vi.fn() }));
vi.mock('../setup/SetupProvider', () => ({ useSetup: () => setup }));
vi.mock('../auth/AuthProvider', () => ({ useAuth: () => null }));
vi.mock('../teams/TeamWorkspaceControl', () => ({ TeamWorkspaceControl: () => null }));
beforeEach(() => {
  vi.clearAllMocks(); resetCockpit(); selectTeamScope('');
  useCockpit.setState({ projects: [aProject({ id: 'p', name: 'Counter' })] });
});
const timeout = (): CatalogueProblem => ({ code: 'timeout', message: 'The control plane did not answer GET http://127.0.0.1:62000/v1/projects/p/branches within 10000ms', branchLoad: {
  projectId: 'p', service: api.baseUrl, catalogueEpoch: useCockpit.getState().projectCatalogueEpoch, workspaceGeneration: teamScopeGeneration(),
} });

it('announces one plain Home notice, retains exact collapsed details, and retries only the original problem', () => {
  const problem = timeout(); useCockpit.getState().setError(problem);
  render(<><ApiProblemNotice id="home-api-notice" problem={problem} /><RailFooter errorPresentedInMain /></>);
  expect(screen.getAllByRole('alert')).toHaveLength(1);
  const notice = screen.getByRole('alert');
  expect(within(notice).getByText('Branches for Counter took too long to load.')).toBeTruthy();
  const details = within(notice).getByText('Technical details').parentElement as HTMLDetailsElement;
  expect(details.open).toBe(false); expect(details.textContent).toContain(problem.message);
  fireEvent.click(screen.getByRole('button', { name: 'Retry branches' }));
  expect(setup.retryBranches).toHaveBeenCalledTimes(1);
  expect(setup.retryBranches).toHaveBeenCalledWith(problem);
  fireEvent.click(screen.getByRole('button', { name: 'View project notice' }));
  expect(document.activeElement).toBe(notice);
  expect(setup.refreshProjects).not.toHaveBeenCalled();
});

it('presents the full branch notice in the footer outside Home and keeps loading retries disabled', () => {
  const problem = timeout(); useCockpit.setState({ lastError: problem, loading: { 'branches:p': true } });
  render(<RailFooter />);
  expect(screen.getAllByRole('alert')).toHaveLength(1);
  expect(screen.getByRole('button', { name: 'Retry branches' })).toHaveProperty('disabled', true);
});

it('keeps unrelated refusal text verbatim rather than classifying a URL or the word timeout', () => {
  const message = 'A timeout was mentioned, but this sandbox is still running; public publishing was refused.';
  render(<ApiProblemNotice problem={{ code: 'non_loopback_publish', message }} />);
  expect(screen.getByRole('alert').textContent).toContain(message);
  expect(screen.queryByText('Technical details')).toBeNull();
  expect(screen.queryByRole('button', { name: 'Retry branches' })).toBeNull();
});

it.each(['catalogue', 'workspace', 'service'] as const)('does not expose a stale branch retry after its %s changes', change => {
  const problem = timeout();
  if (change === 'catalogue') useCockpit.setState(state => ({ projectCatalogueEpoch: state.projectCatalogueEpoch + 1 }));
  if (change === 'workspace') selectTeamScope('');
  if (change === 'service') problem.branchLoad!.service = 'http://127.0.0.1:other';
  render(<ApiProblemNotice problem={problem} />);
  expect(screen.queryByRole('alert')).toBeNull();
  expect(setup.retryBranches).not.toHaveBeenCalled();
});

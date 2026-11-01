import { beforeEach, expect, it, vi } from 'vitest';
import { act, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { ApiError } from '../../api/client';
import { projectOverviewApi, type ProjectOverview } from '../../api/projectOverview';
import { projectHomeApi, type ProjectHome } from '../../api/projectHome';
import { aProject } from '../../test/fixtures';
import { resetCockpit, useCockpit } from '../../store/store';
import { ProjectHomeProvider, useProjectHome } from './ProjectHomeProvider';
import { ProjectsHome } from './ProjectsHome';
import { Rail } from '../rail/Rail';

vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => 'current:actor:workspace' }));
vi.mock('../setup/SetupProvider', () => ({ useSetup: () => ({ openProject: vi.fn(), openSettings: vi.fn() }) }));
vi.mock('./ProjectOutcomes', () => ({ ProjectOutcomes: () => null }));
vi.mock('../teams/TeamWorkspaceProvider', async importOriginal => ({
  ...await importOriginal<typeof import('../teams/TeamWorkspaceProvider')>(),
  useTeamWorkspace: () => null,
}));

const projects = [aProject({ id: 'p-a', name: 'Alpha' }), aProject({ id: 'p-b', name: 'Beta' })];
const stop = { id: `stop-${'a'.repeat(64)}`, at: '2026-10-15T10:00:00Z', kind: 'stopped' as const };
const home: ProjectHome = { version: 1, revision: 3, pinnedProjectIds: ['p-a'], projects: [
  { id: 'p-a', name: 'Alpha', working: true, needsInput: false, queued: false, unknown: false, stops: [] },
  { id: 'p-b', name: 'Beta', working: false, needsInput: false, queued: false, unknown: false, stops: [stop] },
] };
let context: ReturnType<typeof useProjectHome> = null;
function Navigation() {
  context = useProjectHome();
  return <><ProjectsHome /><Rail /></>;
}
const homePage = () => within(screen.getByRole('heading', { name: 'Your projects' }).closest('section')!);
beforeEach(() => {
  resetCockpit(); useCockpit.getState().setProjects(projects); vi.restoreAllMocks(); context = null;
  vi.spyOn(projectOverviewApi, 'get').mockImplementation(async (projectId): Promise<ProjectOverview> => ({ projectId, observedAt: '', status: 'ready', goal: { text: 'Build a useful product', revision: 1, updatedAt: '', truncated: false }, phase: { id: 'develop', label: 'Develop' }, context: { briefRevision: 1, specificationRevision: 1, definitionStatus: 'approved', designRevision: 1, designStatus: 'approved', planRevision: 2, tasks: { total: 3, recordedDone: 1 } }, questions: { status: 'recorded', count: 0, items: [], omitted: 0 }, execution: { scope: 'swarm-queue', status: 'recorded', observedAt: '', running: 1, queued: 0, paused: 0, reserved: 0, blocked: 0, unknown: 0, failed: 0, dispatchPaused: false }, nextAction: { id: 'review-plan', label: 'Review the plan', lens: 'develop' }, limitations: [] }));
  vi.spyOn(projectHomeApi, 'pin').mockResolvedValue({ revision: 4, pinnedProjectIds: [] });
  vi.spyOn(projectHomeApi, 'acknowledge').mockResolvedValue({ revision: 4, acknowledgedStopIds: [stop.id] });
});

it.each([
  ['network', new ApiError('network_unavailable', 'Connection interrupted.', 0)],
  ['timeout', new ApiError('timeout', 'Activity timed out.', 0)],
  ['server', new ApiError('backend_unavailable', 'Activity temporarily unavailable.', 503)],
] as const)('keeps verified Home and rail navigation through a %s outage without using stale activity for mutations', async (_kind, failure) => {
  vi.spyOn(projectHomeApi, 'getAll').mockResolvedValueOnce(home)
    .mockRejectedValueOnce(failure).mockResolvedValue(home);
  render(<ProjectHomeProvider><Navigation /></ProjectHomeProvider>);
  await waitFor(() => expect(homePage().getAllByRole('button', { name: 'Open project' })).toHaveLength(2));
  expect(homePage().getByText('Working')).toBeTruthy();
  fireEvent.click(homePage().getAllByRole('button', { name: 'Project details' })[0]);
  expect(homePage().getByRole('button', { name: 'Manage queue' })).toBeTruthy();
  act(() => context!.refresh());
  await waitFor(() => expect(homePage().getByText(/Showing last verified projects/)).toBeTruthy());
  expect(homePage().getAllByRole('button', { name: 'Open project' })).toHaveLength(2);
  expect(homePage().queryByText('Working')).toBeNull();
  expect(homePage().queryByText(/Stopped ·/)).toBeNull();
  expect(homePage().getAllByText('Activity unavailable')).toHaveLength(2);
  expect(homePage().getByRole('button', { name: 'Unpin' })).toHaveProperty('disabled', true);
  expect(homePage().getByRole('button', { name: 'Pin' })).toHaveProperty('disabled', true);
  expect(homePage().queryByRole('button', { name: 'Manage queue' })).toBeNull();
  expect(homePage().queryByRole('button', { name: 'Project details' })).toBeNull();
  expect(screen.getAllByRole('treeitem')).toHaveLength(2);
  expect(screen.getByRole('group', { name: 'Last verified projects' })).toBeTruthy();
  fireEvent.click(screen.getByRole('treeitem', { name: 'Beta' }));
  expect(useCockpit.getState().selection.projectId).toBe('p-b');
  expect(projectHomeApi.acknowledge).not.toHaveBeenCalled();
  await act(async () => context!.pin('p-a', false));
  expect(projectHomeApi.pin).not.toHaveBeenCalled();
  fireEvent.click(homePage().getByRole('button', { name: 'Refresh activity' }));
  await waitFor(() => expect(context?.activityAvailable).toBe(true));
  expect(homePage().getByText('Working')).toBeTruthy();
  expect(homePage().getByRole('button', { name: 'Unpin' })).toHaveProperty('disabled', false);
  expect(homePage().getAllByRole('button', { name: 'Project details' })).toHaveLength(2);
  fireEvent.click(homePage().getAllByRole('button', { name: 'Open project' })[1]);
  await waitFor(() => expect(projectHomeApi.acknowledge).toHaveBeenCalledWith('p-b', [stop.id], 3));
});

it('does not expose unverified catalogue rows or claim no projects when the first activity read fails', async () => {
  vi.spyOn(projectHomeApi, 'getAll').mockRejectedValue(new ApiError('timeout', 'Activity timed out.', 0));
  render(<ProjectHomeProvider><Navigation /></ProjectHomeProvider>);
  await screen.findByText('Activity timed out.');
  expect(homePage().queryByRole('button', { name: 'Open project' })).toBeNull();
  expect(screen.queryByRole('treeitem')).toBeNull();
  expect(screen.queryByText(/No projects yet/)).toBeNull();
  expect(homePage().getByText(/Refresh activity to verify your accessible projects/)).toBeTruthy();
});

it.each([
  ['401', new ApiError('conflict', 'Access must be verified again.', 401)],
  ['403', new ApiError('conflict', 'Access must be verified again.', 403)],
  ['409', new ApiError('conflict', 'Access must be verified again.', 409)],
  ['unverified response', new Error('Access must be verified again.')],
] as const)('removes retained navigation after %s', async (_kind, failure) => {
  vi.spyOn(projectHomeApi, 'getAll').mockResolvedValueOnce(home)
    .mockRejectedValue(failure);
  render(<ProjectHomeProvider><Navigation /></ProjectHomeProvider>);
  await waitFor(() => expect(context?.activityAvailable).toBe(true));
  act(() => context!.refresh());
  await screen.findByText('Access must be verified again.');
  expect(context?.home).toBeNull();
  expect(homePage().queryByRole('button', { name: 'Open project' })).toBeNull();
  expect(screen.queryByRole('treeitem')).toBeNull();
  expect(projectHomeApi.acknowledge).not.toHaveBeenCalled();
});

it('removes projects omitted by a fresh verified activity read and by the current catalogue', async () => {
  vi.spyOn(projectHomeApi, 'getAll').mockResolvedValueOnce(home).mockResolvedValue({ ...home, revision: 4, pinnedProjectIds: [], projects: [home.projects[1]] });
  render(<ProjectHomeProvider><Navigation /></ProjectHomeProvider>);
  await waitFor(() => expect(context?.activityAvailable).toBe(true));
  act(() => context!.refresh());
  await waitFor(() => expect(context?.home?.revision).toBe(4));
  expect(homePage().getAllByRole('button', { name: 'Open project' })).toHaveLength(1);
  expect(screen.queryByRole('treeitem', { name: 'Alpha' })).toBeNull();
  act(() => useCockpit.getState().setProjects([]));
  expect(homePage().queryByRole('button', { name: 'Open project' })).toBeNull();
  expect(screen.queryByRole('treeitem')).toBeNull();
});

it('introduces all five activities when creating the first project', async () => {
  useCockpit.getState().setProjects([]);
  vi.spyOn(projectHomeApi, 'getAll').mockResolvedValue({ ...home, pinnedProjectIds: [], projects: [] });
  render(<ProjectHomeProvider><Navigation /></ProjectHomeProvider>);
  await waitFor(() => expect(context?.activityAvailable).toBe(true));
  expect(homePage().getAllByRole('listitem').map(item => item.querySelector('strong')?.textContent))
    .toEqual(['Discover', 'Design', 'Develop', 'Deploy', 'Distribute']);
});

it('puts project goals and next actions on Home without opening detailed controls', async () => {
  vi.spyOn(projectHomeApi, 'getAll').mockResolvedValue(home);
  render(<ProjectHomeProvider><Navigation /></ProjectHomeProvider>);
  await waitFor(() => expect(homePage().getAllByText('Build a useful product')).toHaveLength(2));
  expect(homePage().getAllByRole('button', { name: 'Review the plan' })).toHaveLength(2);
  expect(homePage().getAllByRole('list', { name: 'Your project cards' })).toHaveLength(1);
  expect(homePage().queryByRole('button', { name: 'Manage queue' })).toBeNull();
  fireEvent.click(homePage().getAllByRole('button', { name: 'Review the plan' })[1]);
  expect(useCockpit.getState().selection.projectId).toBe('p-b');
  expect(useCockpit.getState().lens).toBe('develop');
});

it('uses the stable Home destination strip to acknowledge an observed stopped project', async () => {
  vi.spyOn(projectHomeApi, 'getAll').mockResolvedValue(home);
  render(<ProjectHomeProvider><Navigation /></ProjectHomeProvider>);
  const destinations = await screen.findByRole('navigation', { name: 'Open a project' });
  fireEvent.click(within(destinations).getByRole('button', { name: 'Open project Beta' }));
  expect(useCockpit.getState().selection.projectId).toBe('p-b');
  await waitFor(() => expect(projectHomeApi.acknowledge).toHaveBeenCalledWith('p-b', [stop.id], 3));
});

import { act, cleanup, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { PortfolioActionDestination } from './PortfolioActionDestination';
import { ProjectHomeProvider, useProjectHome } from './ProjectHomeProvider';
import { projectHomeApi } from '../../api/projectHome';
import { teamScopeGeneration, selectTeamScope } from '../../api/teamScope';
import { resetCockpit, useCockpit } from '../../store/store';

vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: (id: string) => `alice:workspace:${id}` }));
const projects = [{ id: 'p-one', name: 'Appointments', path: '/appointments' }, { id: 'p-two', name: 'Shop', path: '/shop' }];
const stop = { id: `stop-${'a'.repeat(64)}`, at: '2026-10-30T00:00:00Z', kind: 'stopped' as const };
function home(withStop = false) {
  return { version: 1 as const, revision: 1, pinnedProjectIds: [], projects: projects.map(project => ({ id: project.id, name: project.name, working: false, needsInput: false, queued: false, unknown: false, stops: withStop && project.id === 'p-one' ? [stop] : [] })) };
}
function ActivityReady() {
  const value = useProjectHome();
  return <span>{value?.home ? `${value.home.projects.reduce((count, project) => count + project.stops.length, 0)} saved stops` : 'Reading activity'}</span>;
}
beforeEach(() => {
  vi.restoreAllMocks(); selectTeamScope(''); resetCockpit(); useCockpit.getState().setProjects(projects);
  useCockpit.getState().selectProject('p-one', 'design');
  useCockpit.getState().setUnsavedProjectDraft('edited', { projectId: 'p-one', label: 'Appointment design', lens: 'design', retained: true });
  vi.spyOn(projectHomeApi, 'getAll').mockResolvedValue(home());
  vi.spyOn(projectHomeApi, 'acknowledge').mockResolvedValue({ revision: 2, acknowledgedStopIds: [stop.id] });
});
afterEach(() => { cleanup(); selectTeamScope(''); });

it.each(['confirm', 'cancel'] as const)('preserves the Luci destination through a deferred project switch and %s', async decision => {
  render(<ProjectHomeProvider><PortfolioActionDestination action={{ id: 'open-conversation', label: 'Open project Luci', lens: 'discover' }} project={projects[1]} generation={teamScopeGeneration()} /></ProjectHomeProvider>);
  await waitFor(() => expect(projectHomeApi.getAll).toHaveBeenCalledTimes(1));
  await userEvent.click(screen.getByRole('button', { name: 'Open project Luci' }));
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: 'project', projectId: 'p-two', lens: 'discover', surface: 'luci' });
  expect(useCockpit.getState().selection.projectId).toBe('p-one');
  expect(useCockpit.getState().workspaceSurface).toBe('tools');
  if (decision === 'confirm') {
    act(() => useCockpit.getState().discardDraftsAndNavigate());
    expect(useCockpit.getState().selection.projectId).toBe('p-two');
    expect(useCockpit.getState().lens).toBe('discover');
    expect(useCockpit.getState().workspaceSurface).toBe('luci');
    expect(useCockpit.getState().unsavedProjectDrafts.edited).toBeUndefined();
  } else {
    act(() => useCockpit.getState().cancelNavigation());
    expect(useCockpit.getState().selection.projectId).toBe('p-one');
    expect(useCockpit.getState().lens).toBe('design');
    expect(useCockpit.getState().workspaceSurface).toBe('tools');
    expect(useCockpit.getState().unsavedProjectDrafts.edited).toBeDefined();
  }
  expect(projectHomeApi.acknowledge).not.toHaveBeenCalled();
});

it('does not acknowledge current-project stops when a different-lens Luci destination is cancelled', async () => {
  vi.mocked(projectHomeApi.getAll).mockResolvedValue(home(true));
  render(<ProjectHomeProvider><ActivityReady /><PortfolioActionDestination action={{ id: 'open-conversation', label: 'Open project Luci', lens: 'discover' }} project={projects[0]} generation={teamScopeGeneration()} /></ProjectHomeProvider>);
  await screen.findByText('1 saved stops');
  await userEvent.click(screen.getByRole('button', { name: 'Open project Luci' }));
  expect(useCockpit.getState().pendingNavigation).toMatchObject({ projectId: 'p-one', lens: 'discover', surface: 'luci' });
  await act(async () => { useCockpit.getState().cancelNavigation(); });
  expect(useCockpit.getState().selection.projectId).toBe('p-one');
  expect(useCockpit.getState().lens).toBe('design');
  expect(useCockpit.getState().workspaceSurface).toBe('tools');
  expect(useCockpit.getState().unsavedProjectDrafts.edited).toBeDefined();
  expect(projectHomeApi.acknowledge).not.toHaveBeenCalled();
});

it.each(['confirm', 'cancel'] as const)('guards a same-project same-lens tools-to-Luci transition before %s and acknowledges only commitment', async decision => {
  useCockpit.setState({ lens: 'discover', workspaceSurface: 'tools' });
  useCockpit.getState().setUnsavedProjectDraft('edited', { projectId: 'p-one', label: 'Appointment brief', lens: 'discover', retained: true });
  vi.mocked(projectHomeApi.getAll).mockResolvedValue(home(true));
  render(<ProjectHomeProvider><ActivityReady /><PortfolioActionDestination action={{ id: 'open-conversation', label: 'Open project Luci', lens: 'discover' }} project={projects[0]} generation={teamScopeGeneration()} /></ProjectHomeProvider>);
  await screen.findByText('1 saved stops');
  await userEvent.click(screen.getByRole('button', { name: 'Open project Luci' }));
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: 'project', projectId: 'p-one', lens: 'discover', surface: 'luci' });
  expect(useCockpit.getState().workspaceSurface).toBe('tools');
  expect(useCockpit.getState().unsavedProjectDrafts.edited).toBeDefined();
  expect(projectHomeApi.acknowledge).not.toHaveBeenCalled();
  if (decision === 'confirm') {
    act(() => useCockpit.getState().discardDraftsAndNavigate());
    expect(useCockpit.getState().workspaceSurface).toBe('luci');
    await waitFor(() => expect(projectHomeApi.acknowledge).toHaveBeenCalledWith('p-one', [stop.id], 1));
  } else {
    await act(async () => { useCockpit.getState().cancelNavigation(); });
    expect(useCockpit.getState().workspaceSurface).toBe('tools');
    expect(useCockpit.getState().unsavedProjectDrafts.edited).toBeDefined();
    expect(projectHomeApi.acknowledge).not.toHaveBeenCalled();
  }
});

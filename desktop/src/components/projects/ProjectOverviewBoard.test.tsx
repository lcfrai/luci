import { act, cleanup, render, screen, waitFor, within } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { ProjectOverviewBoard } from './ProjectOverviewBoard';
import { projectOverviewApi, type ProjectOverview } from '../../api/projectOverview';
import { swarmQueueApi, type ProjectSwarmQueue, type SwarmQueueItem } from '../../api/swarmQueue';
import { ApiError, type ProblemCode } from '../../api/client';
import { personalPermissions, teamsApi, type ProjectAccess } from '../../api/teams';
import { resetCockpit, useCockpit } from '../../store/store';

const environment = vi.hoisted(() => ({ scope: 'account-one', teamId: '', edit: true, execute: true }));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: (projectId: string) => `${environment.scope}:${projectId}` }));
vi.mock('../teams/TeamWorkspaceProvider', () => ({ useTeamWorkspace: () => environment.teamId ? { teamId: environment.teamId, selected: null } : null, useTeamProjectPermissions: () => ({ permissions: { read: true, edit: environment.edit, execute: environment.execute }, loading: false }) }));
vi.mock('../auth/AuthProvider', () => ({ useAuth: () => ({ profile: { user: { role: 'member' } } }) }));
const projects = [{ id: 'p1', name: 'Notes', path: '/private/notes' }, { id: 'p2', name: 'Shop', path: '/private/shop' }];
function overview(projectId: string): ProjectOverview {
  return { projectId, observedAt: new Date().toISOString(), status: 'ready', goal: { text: `Build ${projectId}`, revision: 1, updatedAt: '', truncated: false }, phase: { id: 'develop', label: 'Develop' }, context: { briefRevision: 1, specificationRevision: 1, definitionStatus: 'approved', designRevision: 1, designStatus: 'approved', planRevision: 2, tasks: { total: 3, recordedDone: 1 } }, questions: { status: 'recorded', count: 0, items: [], omitted: 0 }, execution: { scope: 'swarm-queue', status: 'recorded', observedAt: '', running: 1, queued: 2, paused: 0, reserved: 0, blocked: 0, unknown: 0, dispatchPaused: false }, nextAction: { id: 'review-plan', label: 'Review the plan', lens: 'develop' }, limitations: [] };
}
function item(projectId: string, state: SwarmQueueItem['state'] = 'queued'): SwarmQueueItem { return { id: `q-${projectId}`, revision: 2, projectId, runId: 'r1', taskId: 't1', taskTitle: 'Build comments', state, priority: 'normal', queuedAt: '', updatedAt: '' }; }
function queue(projectId: string): ProjectSwarmQueue { return { revision: 7, observedAt: '', dispatchPaused: false, hasMore: false, counts: { queued: 1, paused: 0, reserved: 0, running: 0, blocked: 0, unknown: 0, completed: 0, cancelled: 0 }, items: [item(projectId)] }; }
beforeEach(() => {
  vi.restoreAllMocks(); resetCockpit(); useCockpit.getState().setProjects(projects);
  environment.scope = 'account-one'; environment.teamId = ''; environment.edit = true; environment.execute = true;
  vi.spyOn(projectOverviewApi, 'get').mockImplementation(async id => overview(id)); vi.spyOn(swarmQueueApi, 'get').mockImplementation(async id => queue(id)); vi.spyOn(swarmQueueApi, 'control').mockImplementation(async id => queue(id));
  vi.spyOn(teamsApi, 'projectAccess').mockImplementation(async (_teamId, projectId) => ({ projectId, permissions: { ...personalPermissions, edit: environment.edit, execute: environment.execute } } as ProjectAccess));
});
afterEach(cleanup);

it('reads compact summaries only until the operator opens a queue, and navigates atomically', async () => {
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={projects} />);
  await screen.findByText('Build p1'); expect(swarmQueueApi.get).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Open project Notes' })); expect(useCockpit.getState().selection.projectId).toBe('p1');
  await user.click(screen.getAllByRole('button', { name: 'Review the plan' })[0]); expect(useCockpit.getState().lens).toBe('develop');
  await user.click(screen.getAllByRole('button', { name: 'Manage queue' })[0]); await screen.findByRole('checkbox', { name: 'Select Build comments in Notes' });
  expect(swarmQueueApi.get).toHaveBeenCalledTimes(1);
  expect(screen.queryByText('/private/notes')).toBeNull();
});
it('opens the exact overview run in its project scope instead of a previous inspection', async () => {
  const value = overview('p1'); value.nextAction = { id: 'inspect-run', label: 'Inspect blocked run', lens: 'develop', runId: 'run-latest' };
  vi.mocked(projectOverviewApi.get).mockResolvedValue(value);
  useCockpit.setState({ projectRunInspection: { scope: 'account-one:p1', projectId: 'p1', runId: 'run-old' } });
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await user.click(await screen.findByRole('button', { name: 'Inspect blocked run' }));
  expect(useCockpit.getState().selection.projectId).toBe('p1');
  expect(useCockpit.getState().lens).toBe('develop');
  expect(useCockpit.getState().projectRunInspection).toEqual({ scope: 'account-one:p1', projectId: 'p1', runId: 'run-latest', refresh: true });
  expect(swarmQueueApi.control).not.toHaveBeenCalled();
});
it('keeps exact-run navigation behind the draft guard and discards it on cancellation', async () => {
  const value = overview('p1'); value.nextAction = { id: 'inspect-run', label: 'Inspect blocked run', lens: 'develop', runId: 'run-latest' };
  vi.mocked(projectOverviewApi.get).mockResolvedValue(value);
  const store = useCockpit.getState(), discard = vi.fn();
  store.selectProject('p2', 'design'); store.setUnsavedProjectDraft('draft', { projectId: 'p2', label: 'Design', lens: 'design', retained: true, discard });
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await user.click(await screen.findByRole('button', { name: 'Inspect blocked run' }));
  expect(useCockpit.getState().selection.projectId).toBe('p2');
  expect(useCockpit.getState().pendingNavigation).toEqual({ kind: 'project', projectId: 'p1', lens: 'develop' });
  expect(useCockpit.getState().projectRunInspection?.runId).toBe('run-latest');
  act(() => store.cancelNavigation());
  expect(useCockpit.getState().projectRunInspection).toBeNull();
  expect(useCockpit.getState().selection.projectId).toBe('p2');
  await user.click(screen.getByRole('button', { name: 'Inspect blocked run' }));
  act(() => store.discardDraftsAndNavigate(true));
  expect(useCockpit.getState().selection.projectId).toBe('p1');
  expect(useCockpit.getState().lens).toBe('develop');
  expect(useCockpit.getState().projectRunInspection?.runId).toBe('run-latest');
  expect(discard).not.toHaveBeenCalled();
});
it('does not create a run intent after a removed project rejects navigation', async () => {
  const value = overview('p1'); value.nextAction = { id: 'inspect-run', label: 'Inspect blocked run', lens: 'develop', runId: 'run-latest' };
  vi.mocked(projectOverviewApi.get).mockResolvedValue(value);
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await screen.findByRole('button', { name: 'Inspect blocked run' });
  act(() => useCockpit.getState().setProjects([projects[1]]));
  await user.click(screen.getByRole('button', { name: 'Inspect blocked run' }));
  expect(useCockpit.getState().projectRunInspection).toBeNull();
  expect(useCockpit.getState().selection.projectId).toBeNull();
});
it('preserves an in-progress queue click while its project summary arrives', async () => {
  let resolve!: (value: ProjectOverview) => void;
  vi.mocked(projectOverviewApi.get).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  const button = screen.getByRole('button', { name: 'Manage queue' });
  await user.pointer({ target: button, keys: '[MouseLeft>]' });
  await act(async () => resolve(overview('p1')));
  expect(screen.getByRole('button', { name: 'Manage queue' })).toBe(button);
  await user.pointer({ target: button, keys: '[/MouseLeft]' });
  await screen.findByRole('checkbox', { name: 'Select Build comments in Notes' });
  expect(swarmQueueApi.control).not.toHaveBeenCalled();
});
it('keeps unavailable counters and incomplete question history distinct from zero', async () => {
  const value = overview('p1'); value.execution = { ...value.execution, status: 'unavailable', running: null, queued: null, paused: null, reserved: null, blocked: null, unknown: null }; value.questions.status = 'partial'; value.questions.count = null;
  vi.mocked(projectOverviewApi.get).mockResolvedValue(value); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await screen.findByText('Queue activity unavailable; no zero counts inferred.');
  expect(within(screen.getByLabelText('Recorded swarm queue activity')).getAllByText('—')).toHaveLength(3);
  expect(screen.getByText(/Question history is incomplete/)).toBeTruthy();
});
it('retains explicitly recorded context during source preparation while keeping queue intervention available', async () => {
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await screen.findByText('Build p1');
  vi.mocked(projectOverviewApi.get).mockRejectedValueOnce(new ApiError('branch_busy' as ProblemCode, 'Project source is locked.', 409));
  await user.click(screen.getByRole('button', { name: 'Refresh overview' }));
  await screen.findByText(/Source is being prepared; showing last recorded context/);
  expect(screen.getByText('Build p1')).toBeTruthy();
  expect(screen.getByText('Last recorded: Develop')).toBeTruthy();
  expect(screen.queryByRole('button', { name: 'Review the plan' })).toBeNull();
  expect(screen.getByText(/Activity counts are recorded, not live/)).toBeTruthy();
  await user.click(screen.getByRole('button', { name: 'Manage queue' }));
  await user.click(await screen.findByRole('checkbox', { name: 'Select Build comments in Notes' }));
  await user.click(screen.getByRole('button', { name: 'Pause waiting' }));
  await screen.findByText('1 changed · 0 failed · 0 unconfirmed · 0 skipped');
  await screen.findByRole('button', { name: 'Review the plan' });
  expect(screen.queryByText(/showing last recorded context/)).toBeNull();
  expect(swarmQueueApi.control).toHaveBeenCalledExactlyOnceWith('p1', expect.objectContaining({ revision: 2 }), 'pause', 'normal');
});
it.each([403, 500])('clears retained private context when a later read fails with status %i', async status => {
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await screen.findByText('Build p1');
  vi.mocked(projectOverviewApi.get).mockRejectedValueOnce(new ApiError('branch_busy' as ProblemCode, 'Project source is locked.', 409));
  await user.click(screen.getByRole('button', { name: 'Refresh overview' }));
  await screen.findByText('Last recorded: Develop');
  vi.mocked(projectOverviewApi.get).mockRejectedValueOnce(new ApiError('internal', 'Access or context unavailable.', status));
  await user.click(screen.getByRole('button', { name: 'Refresh overview' }));
  await screen.findByText('Summary unavailable. Access or context unavailable.');
  expect(screen.queryByText('Build p1')).toBeNull();
  expect(screen.queryByText('Last recorded: Develop')).toBeNull();
  expect(screen.queryByRole('button', { name: 'Review the plan' })).toBeNull();
  expect(screen.queryByLabelText('Recorded swarm queue activity')).toBeNull();
});
it('bounds concurrent summaries to four and never starts the fifth while four are pending', async () => {
  const resolves: (() => void)[] = [];
  vi.mocked(projectOverviewApi.get).mockImplementation(id => new Promise(resolve => resolves.push(() => resolve(overview(id)))));
  render(<ProjectOverviewBoard projects={Array.from({ length: 6 }, (_, n) => ({ id: `p${n}`, name: `Project ${n}`, path: '/private' }))} />);
  expect(projectOverviewApi.get).toHaveBeenCalledTimes(4);
  await act(async () => { resolves[0](); });
  expect(projectOverviewApi.get).toHaveBeenCalledTimes(5);
});
it('drops a late summary when the account scope changes', async () => {
  let resolve!: (value: ProjectOverview) => void;
  vi.mocked(projectOverviewApi.get).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const view = render(<ProjectOverviewBoard projects={[projects[0]]} />);
  environment.scope = 'account-two'; view.rerender(<ProjectOverviewBoard projects={[projects[1]]} />);
  await screen.findByText('Build p2'); await act(async () => { resolve(overview('p1')); });
  expect(screen.queryByText('Build p1')).toBeNull();
});
it('forgets removed project summaries before that project can be shown again', async () => {
  const view = render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await screen.findByText('Build p1'); view.rerender(<ProjectOverviewBoard projects={[]} />);
  vi.mocked(projectOverviewApi.get).mockImplementation(() => new Promise(() => {}));
  view.rerender(<ProjectOverviewBoard projects={[projects[0]]} />);
  expect(screen.queryByText('Build p1')).toBeNull();
  expect(screen.getByText('Loading project context…')).toBeTruthy();
});
it('reports a cross-project partial failure using individual item revisions', async () => {
  vi.mocked(swarmQueueApi.control).mockResolvedValueOnce(queue('p1')).mockRejectedValueOnce(new ApiError('conflict', 'This task changed; refresh its queue.', 409));
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={projects} />);
  await screen.findByText('Build p1');
  for (const button of screen.getAllByRole('button', { name: 'Manage queue' })) await user.click(button);
  await user.click(await screen.findByRole('checkbox', { name: 'Select Build comments in Notes' }));
  await user.click(await screen.findByRole('checkbox', { name: 'Select Build comments in Shop' }));
  await user.click(screen.getByRole('button', { name: 'Pause waiting' }));
  await screen.findByText('1 changed · 1 failed · 0 unconfirmed · 0 skipped');
  expect(swarmQueueApi.control).toHaveBeenNthCalledWith(1, 'p1', expect.objectContaining({ revision: 2 }), 'pause', 'normal');
  expect(screen.getByText(/Failed: This task changed/)).toBeTruthy();
});
it('keeps an interrupted mutation unconfirmed and never retries it during a refresh', async () => {
  vi.mocked(swarmQueueApi.control).mockRejectedValueOnce(new Error('Connection interrupted'));
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await user.click(screen.getByRole('button', { name: 'Manage queue' }));
  await user.click(await screen.findByRole('checkbox', { name: 'Select Build comments in Notes' }));
  await user.click(screen.getByRole('button', { name: 'Pause waiting' }));
  await screen.findByText(/Unconfirmed: Connection interrupted/);
  await user.click(screen.getByRole('button', { name: 'Refresh overview' }));
  await waitFor(() => expect(swarmQueueApi.get).toHaveBeenCalledTimes(3));
  expect(swarmQueueApi.control).toHaveBeenCalledTimes(1);
  expect(screen.getByText('0 changed · 0 failed · 1 unconfirmed · 0 skipped')).toBeTruthy();
});
it('requires an explicit list review before cancelling and does not mutate on dialog dismissal', async () => {
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await user.click(screen.getByRole('button', { name: 'Manage queue' }));
  await user.click(await screen.findByRole('checkbox', { name: 'Select Build comments in Notes' }));
  await user.click(screen.getByRole('button', { name: 'Cancel waiting…' }));
  const dialog = screen.getByRole('dialog', { name: 'Cancel selected waiting tasks?' });
  expect(within(dialog).getByText('Notes · Build comments')).toBeTruthy();
  expect(swarmQueueApi.control).not.toHaveBeenCalled();
  await user.click(screen.getByRole('button', { name: 'Keep queued' }));
  expect(swarmQueueApi.control).not.toHaveBeenCalled();
});
it('stops the remaining bulk dispatches on a scope change during an in-flight control', async () => {
  let resolve!: (value: ProjectSwarmQueue) => void;
  vi.mocked(swarmQueueApi.control).mockImplementationOnce(() => new Promise(done => { resolve = done; }));
  const user = userEvent.setup(), view = render(<ProjectOverviewBoard projects={projects} />);
  for (const button of screen.getAllByRole('button', { name: 'Manage queue' })) await user.click(button);
  await user.click(await screen.findByRole('checkbox', { name: 'Select Build comments in Notes' }));
  await user.click(await screen.findByRole('checkbox', { name: 'Select Build comments in Shop' }));
  await user.click(screen.getByRole('button', { name: 'Pause waiting' }));
  await waitFor(() => expect(swarmQueueApi.control).toHaveBeenCalledTimes(1));
  environment.scope = 'another-workspace'; view.rerender(<ProjectOverviewBoard projects={[]} />);
  await act(async () => { resolve(queue('p1')); });
  expect(swarmQueueApi.control).toHaveBeenCalledTimes(1);
  expect(screen.queryByText('Notes · Build comments')).toBeNull();
});
it('keeps queue selection disabled for a viewer even with an execution grant', async () => {
  environment.edit = false; const user = userEvent.setup(); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await user.click(screen.getByRole('button', { name: 'Manage queue' }));
  const checkbox = await screen.findByRole('checkbox', { name: 'Select Build comments in Notes' });
  expect((checkbox as HTMLInputElement).disabled).toBe(true);
  expect(swarmQueueApi.control).not.toHaveBeenCalled();
});
it.each(['pause', 'cancel', 'priority'] as const)('allows an editor without execution access to %s waiting work', async action => {
  environment.teamId = 'team-one'; environment.execute = false;
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await user.click(screen.getByRole('button', { name: 'Manage queue' }));
  const checkbox = await screen.findByRole('checkbox', { name: 'Select Build comments in Notes' });
  expect((checkbox as HTMLInputElement).disabled).toBe(false);
  await user.click(checkbox);
  if (action === 'pause') await user.click(screen.getByRole('button', { name: 'Pause waiting' }));
  if (action === 'priority') {
    await user.selectOptions(screen.getByRole('combobox', { name: 'Selected task priority' }), 'high');
    await user.click(screen.getByRole('button', { name: 'Set priority' }));
  }
  if (action === 'cancel') {
    await user.click(screen.getByRole('button', { name: 'Cancel waiting…' }));
    expect(swarmQueueApi.control).not.toHaveBeenCalled();
    await user.click(screen.getByRole('button', { name: 'Cancel 1 waiting task' }));
  }
  await screen.findByText('1 changed · 0 failed · 0 unconfirmed · 0 skipped');
  expect(teamsApi.projectAccess).toHaveBeenCalledWith('team-one', 'p1');
  expect(swarmQueueApi.control).toHaveBeenCalledExactlyOnceWith('p1', expect.objectContaining({ revision: 2 }), action, action === 'priority' ? 'high' : 'normal');
});
it('allows paused-task selection for an editor but does not offer resume without execution access', async () => {
  environment.teamId = 'team-one'; environment.execute = false;
  vi.mocked(swarmQueueApi.get).mockResolvedValue({ ...queue('p1'), items: [item('p1', 'paused')] });
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await user.click(screen.getByRole('button', { name: 'Manage queue' }));
  await user.click(await screen.findByRole('checkbox', { name: 'Select Build comments in Notes' }));
  expect((screen.getByRole('button', { name: 'Resume paused' }) as HTMLButtonElement).disabled).toBe(true);
  expect((screen.getByRole('button', { name: 'Cancel waiting…' }) as HTMLButtonElement).disabled).toBe(false);
  expect(screen.getByText(/Editing access allows pause, cancellation and priority changes/)).toBeTruthy();
  expect(swarmQueueApi.control).not.toHaveBeenCalled();
});
it('checks fresh editing access before intervening in a previously selected task', async () => {
  environment.teamId = 'team-one';
  vi.mocked(teamsApi.projectAccess).mockResolvedValue({ projectId: 'p1', permissions: { ...personalPermissions, edit: false } } as ProjectAccess);
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={[projects[0]]} />);
  await user.click(screen.getByRole('button', { name: 'Manage queue' }));
  await user.click(await screen.findByRole('checkbox', { name: 'Select Build comments in Notes' }));
  await user.click(screen.getByRole('button', { name: 'Pause waiting' }));
  await screen.findByText('Failed: Editing access is unavailable for this project.');
  expect(swarmQueueApi.control).not.toHaveBeenCalled();
});
it('resumes only projects with fresh execution access and preserves partial outcomes', async () => {
  environment.teamId = 'team-one';
  vi.mocked(swarmQueueApi.get).mockImplementation(async id => ({ ...queue(id), items: [item(id, 'paused')] }));
  vi.mocked(teamsApi.projectAccess).mockImplementation(async (_team, id) => ({ projectId: id, permissions: { ...personalPermissions, execute: id === 'p1' } } as ProjectAccess));
  const user = userEvent.setup(); render(<ProjectOverviewBoard projects={projects} />);
  for (const button of screen.getAllByRole('button', { name: 'Manage queue' })) await user.click(button);
  await user.click(await screen.findByRole('checkbox', { name: 'Select Build comments in Notes' }));
  await user.click(await screen.findByRole('checkbox', { name: 'Select Build comments in Shop' }));
  await user.click(screen.getByRole('button', { name: 'Resume paused' }));
  await screen.findByText('1 changed · 1 failed · 0 unconfirmed · 0 skipped');
  expect(swarmQueueApi.control).toHaveBeenCalledExactlyOnceWith('p1', expect.objectContaining({ revision: 2 }), 'resume', 'normal');
  expect(screen.getByText('Failed: Execution access is required to resume this project’s queued work.')).toBeTruthy();
  await user.click(screen.getByRole('button', { name: 'Refresh overview' }));
  expect(swarmQueueApi.control).toHaveBeenCalledTimes(1);
});

import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { StrictMode } from 'react';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TeamWorkspaceProvider, useTeamProjectPermissions, useTeamWorkspace } from './TeamWorkspaceProvider';
import { teamsApi, noPermissions, personalPermissions, type ProjectAccess, type TeamSummary } from '../../api/teams';
import { productApi } from '../../api/product';
import { reportTeamAccessDenied, selectTeamScope, teamScopeGeneration } from '../../api/teamScope';
import { teamWorkspaceStorageKey } from '../../api/teamWorkspacePreference';
import { resetCockpit } from '../../store/store';
import { api } from '../../api';

const auth = vi.hoisted(() => ({ userId: 'alice' }));
vi.mock('../auth/AuthProvider', () => ({ useAuth: () => ({ profile: { user: { id: auth.userId } }, logout: vi.fn() }) }));
vi.mock('../../api/attachRegistry', () => ({ detachAllTerminals: vi.fn() }));
const teamId = 'team-abcdefghijklmnopqrstuv';
const summary: TeamSummary = { team: { id: teamId, name: 'Atlas', revision: 1, createdAt: '', updatedAt: '' }, membership: { userId: 'alice', role: 'owner', grants: personalPermissions, revision: 1 }, permissions: personalPermissions };
const access = (projectId = 'project-a', permissions = personalPermissions): ProjectAccess => ({ team: summary.team, projectId, permissions, grants: [] });
function deferred<T>() { let resolve!: (value: T) => void; let reject!: (error: Error) => void; const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; }); return { promise, resolve, reject }; }
function remember(userId = 'alice') { const key = teamWorkspaceStorageKey(productApi.baseUrl, userId); localStorage.setItem(key, teamId); localStorage.setItem(key + '.cockpit', teamId); }
function Probe({ name, projectId = 'project-a' }: { name: string; projectId?: string }) {
  const current = useTeamProjectPermissions(projectId);
  return <section aria-label={name}><output data-testid={name}>{JSON.stringify(current)}</output>{current.permissions.read && <label>{name} draft<textarea defaultValue="Saved idea" /></label>}<button disabled={current.loading || !current.permissions.edit}>Save {name}</button></section>;
}
function Controls() { const workspace = useTeamWorkspace()!; return <button onClick={() => void workspace.refresh().catch(() => {})}>Refresh access</button>; }
function Workspace({ names = ['one', 'two'], secondProject = 'project-a' }: { names?: string[]; secondProject?: string }) { return <TeamWorkspaceProvider><Controls />{names.map(name => <Probe key={name} name={name} projectId={name === 'two' ? secondProject : 'project-a'} />)}</TeamWorkspaceProvider>; }
const permission = (name: string) => JSON.parse(screen.getByTestId(name).textContent!);
const settle = async () => { await act(async () => {}); };
beforeEach(() => {
  auth.userId = 'alice'; selectTeamScope(''); localStorage.clear(); resetCockpit(); remember();
  vi.spyOn(teamsApi, 'list').mockResolvedValue({ teams: [summary] });
  vi.spyOn(teamsApi, 'projectAccess').mockImplementation(async (_team, projectId) => access(projectId));
  vi.spyOn(api, 'listProjects').mockResolvedValue([]); vi.spyOn(api, 'listSessions').mockResolvedValue([]);
});
afterEach(() => { vi.restoreAllMocks(); selectTeamScope(''); localStorage.clear(); resetCockpit(); });

it('shares one overlapping project grant read but a later subscriber must read fresh authority', async () => {
  const first = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(first.promise);
  const view = render(<Workspace />);
  await screen.findByTestId('two'); await settle();
  expect(teamsApi.projectAccess).toHaveBeenCalledTimes(1);
  expect(permission('one').loading).toBe(true); expect(permission('two').loading).toBe(true);
  await act(async () => first.resolve(access()));
  expect(permission('one').permissions).toEqual(personalPermissions); expect(permission('two').permissions).toEqual(personalPermissions);
  const fresh = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(fresh.promise);
  view.rerender(<Workspace names={['one', 'two', 'three']} />); await settle();
  expect(teamsApi.projectAccess).toHaveBeenCalledTimes(2);
  expect(permission('three')).toEqual({ permissions: noPermissions, loading: true, error: '' });
  await act(async () => fresh.resolve(access('project-a', { ...noPermissions, read: true })));
  expect(permission('three').permissions.edit).toBe(false);
});

it('unsubscribing one reader preserves the other and leaving the last reader aborts its request', async () => {
  const pending = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(pending.promise);
  const view = render(<Workspace />); await screen.findByTestId('two'); await settle();
  expect(teamsApi.projectAccess).toHaveBeenCalledTimes(1);
  const signal = vi.mocked(teamsApi.projectAccess).mock.calls[0][2]!;
  expect(signal?.aborted).toBe(false);
  view.rerender(<Workspace names={['two']} />); await settle(); expect(signal.aborted).toBe(false);
  await act(async () => pending.resolve(access())); expect(permission('two').permissions.edit).toBe(true);
  const next = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(next.promise);
  view.rerender(<Workspace names={['two', 'three']} />); await settle();
  const nextSignal = vi.mocked(teamsApi.projectAccess).mock.calls[1][2]!;
  view.rerender(<Workspace names={['two']} />); await settle(); expect(nextSignal.aborted).toBe(true);
  await act(async () => next.resolve(access())); expect(screen.queryByTestId('three')).toBeNull(); expect(permission('two').permissions.edit).toBe(true);
});

it('different projects and refreshed membership revisions have independent reads and late results cannot restore grants', async () => {
  const oldA = deferred<ProjectAccess>(), oldB = deferred<ProjectAccess>();
  vi.mocked(teamsApi.projectAccess).mockImplementation((_team, projectId) => projectId === 'project-a' ? oldA.promise : oldB.promise);
  render(<Workspace secondProject="project-b" />); await screen.findByTestId('two'); await settle();
  expect(teamsApi.projectAccess).toHaveBeenCalledTimes(2);
  const oldSignals = vi.mocked(teamsApi.projectAccess).mock.calls.map(call => call[2]);
  const fresh = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockImplementation(async (_team, projectId) => projectId === 'project-a' ? fresh.promise : access(projectId, noPermissions));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  await waitFor(() => expect(teamsApi.projectAccess).toHaveBeenCalledTimes(4));
  expect(oldSignals.every(signal => signal?.aborted)).toBe(true);
  await act(async () => fresh.resolve(access('project-a', { ...noPermissions, read: true })));
  await act(async () => { oldA.resolve(access()); oldB.resolve(access('project-b')); });
  expect(permission('one').permissions).toEqual({ ...noPermissions, read: true }); expect(permission('two').permissions).toEqual(noPermissions);
});

it('a denial aborts shared reads and its delayed success cannot authorize restored consumers', async () => {
  const old = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(old.promise);
  render(<Workspace />); await screen.findByTestId('two'); await settle();
  const signal = vi.mocked(teamsApi.projectAccess).mock.calls[0][2]!;
  const membership = deferred<{ teams: TeamSummary[] }>(); vi.mocked(teamsApi.list).mockReturnValueOnce(membership.promise);
  act(() => reportTeamAccessDenied(teamScopeGeneration()));
  expect(signal?.aborted).toBe(true); expect(screen.getByRole('heading', { name: 'Workspace unavailable' })).toBeTruthy();
  const restored = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(restored.promise);
  await act(async () => membership.resolve({ teams: [summary] }));
  await waitFor(() => expect(screen.queryByRole('heading', { name: 'Workspace unavailable' })).toBeNull());
  expect(teamsApi.projectAccess).toHaveBeenCalledTimes(2);
  await act(async () => old.resolve(access()));
  expect(permission('one').permissions).toEqual(noPermissions); expect(permission('two').permissions).toEqual(noPermissions);
  await act(async () => restored.resolve(access('project-a', { ...noPermissions, read: true })));
  expect(permission('one').permissions.edit).toBe(false); expect(permission('two').permissions.edit).toBe(false);
});

it('account replacement disposes shared reads and requires a new authority observation', async () => {
  const old = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(old.promise);
  const view = render(<Workspace />); await screen.findByTestId('two'); await settle();
  const signal = vi.mocked(teamsApi.projectAccess).mock.calls[0][2]!;
  const replacement = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(replacement.promise);
  auth.userId = 'bob'; remember('bob'); view.rerender(<Workspace />); await screen.findByTestId('two'); await settle();
  expect(signal?.aborted).toBe(true); expect(teamsApi.projectAccess).toHaveBeenCalledTimes(2);
  await act(async () => old.resolve(access())); expect(permission('one').permissions).toEqual(noPermissions);
  await act(async () => replacement.resolve(access('project-a', { ...noPermissions, read: true })));
  expect(permission('one').permissions.edit).toBe(false); expect(permission('two').permissions.edit).toBe(false);
});

it('shared refresh keeps each dirty editor mounted and pauses writes through an offline failure', async () => {
  render(<Workspace />); const first = await screen.findByRole('textbox', { name: 'one draft' }); const second = screen.getByRole('textbox', { name: 'two draft' });
  await userEvent.type(first, ' first edits'); await userEvent.type(second, ' second edits');
  const pending = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(pending.promise);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' })); await waitFor(() => expect(permission('one').loading).toBe(true));
  expect(screen.getByRole('textbox', { name: 'one draft' })).toBe(first); expect(screen.getByRole('textbox', { name: 'two draft' })).toBe(second);
  expect(screen.getByRole('button', { name: 'Save one' })).toHaveProperty('disabled', true); expect(screen.getByRole('button', { name: 'Save two' })).toHaveProperty('disabled', true);
  await act(async () => pending.reject(new Error('Offline')));
  expect(permission('one').permissions).toEqual({ ...noPermissions, read: true }); expect(permission('two').permissions).toEqual({ ...noPermissions, read: true });
  expect(first).toHaveProperty('value', 'Saved idea first edits'); expect(second).toHaveProperty('value', 'Saved idea second edits');
});

it('StrictMode remounts never reuse an aborted request', async () => {
  const pending = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(pending.promise);
  render(<StrictMode><Workspace /></StrictMode>); await screen.findByTestId('two'); await settle();
  const calls = vi.mocked(teamsApi.projectAccess).mock.calls;
  expect(calls.filter(call => call[2] && !call[2].aborted)).toHaveLength(1);
  await act(async () => pending.resolve(access()));
  expect(permission('one').permissions.edit).toBe(true); expect(permission('two').permissions.edit).toBe(true);
});

it('a grant resolved immediately before denial cannot become a prior readable projection after recovery', async () => {
  const old = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(old.promise);
  render(<Workspace />); await screen.findByTestId('two'); await settle();
  const membership = deferred<{ teams: TeamSummary[] }>(); vi.mocked(teamsApi.list).mockReturnValueOnce(membership.promise);
  act(() => { old.resolve(access()); reportTeamAccessDenied(teamScopeGeneration()); }); await settle();
  const restored = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(restored.promise);
  await act(async () => membership.resolve({ teams: [summary] }));
  await waitFor(() => expect(screen.queryByRole('heading', { name: 'Workspace unavailable' })).toBeNull());
  expect(permission('one').permissions).toEqual(noPermissions); expect(permission('two').permissions).toEqual(noPermissions);
  expect(screen.queryByRole('textbox', { name: 'one draft' })).toBeNull();
  await act(async () => restored.resolve(access('project-a', noPermissions)));
});

it('late completion from an abandoned request cannot evict a replacement shared by new readers', async () => {
  const abandoned = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(abandoned.promise);
  const view = render(<Workspace names={['one']} />); await screen.findByTestId('one'); await settle();
  const signal = vi.mocked(teamsApi.projectAccess).mock.calls[0][2]!;
  view.rerender(<Workspace names={[]} />); await settle();
  const replacement = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(replacement.promise);
  view.rerender(<Workspace names={['two']} />); await settle();
  expect(signal?.aborted).toBe(true); expect(teamsApi.projectAccess).toHaveBeenCalledTimes(2);
  await act(async () => abandoned.resolve(access()));
  view.rerender(<Workspace names={['two', 'three']} />); await settle(); expect(teamsApi.projectAccess).toHaveBeenCalledTimes(2);
  await act(async () => replacement.resolve(access('project-a', { ...noPermissions, read: true })));
  expect(permission('two').permissions.edit).toBe(false); expect(permission('three').permissions.edit).toBe(false);
});

it('separate provider instances never share even simultaneous reads for the same team and project', async () => {
  const pending = deferred<ProjectAccess>(); vi.mocked(teamsApi.projectAccess).mockReturnValue(pending.promise);
  render(<><TeamWorkspaceProvider><Probe name="one" /></TeamWorkspaceProvider><TeamWorkspaceProvider><Probe name="two" /></TeamWorkspaceProvider></>);
  await screen.findByTestId('two'); await settle(); expect(teamsApi.projectAccess).toHaveBeenCalledTimes(2);
  await act(async () => pending.resolve(access()));
  expect(permission('one').permissions.edit).toBe(true); expect(permission('two').permissions.edit).toBe(true);
});

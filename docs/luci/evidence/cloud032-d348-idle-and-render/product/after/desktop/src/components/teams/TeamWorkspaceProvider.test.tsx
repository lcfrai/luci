import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { memo } from 'react';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { TeamWorkspaceProvider, useTeamProjectPermissions, useTeamWorkspace } from './TeamWorkspaceProvider';
import { teamsApi, noPermissions, personalPermissions, type ProjectAccess, type TeamDetail, type TeamSummary } from '../../api/teams';
import { teamInvitationsApi } from '../../api/teamInvitations';
import { productApi } from '../../api/product';
import { ApiError } from '../../api/client';
import { selectedTeamId, selectTeamScope } from '../../api/teamScope';
import { detachAllTerminals } from '../../api/attachRegistry';
import { resetCockpit, useCockpit } from '../../store/store';

const auth = vi.hoisted(() => ({ userId: 'alice', logout: vi.fn() }));
vi.mock('../auth/AuthProvider', () => ({ useAuth: () => ({ profile: { user: { id: auth.userId } }, logout: auth.logout }) }));
vi.mock('../../api/attachRegistry', () => ({ detachAllTerminals: vi.fn() }));
const teamA = 'team-abcdefghijklmnopqrstuv';
const teamB = 'team-bcdefghijklmnopqrstuvw';
const summary = (id = teamA): TeamSummary => ({
  team: { id, name: id === teamA ? 'Atlas' : 'Birch', revision: 1, createdAt: '', updatedAt: '' },
  membership: { userId: 'alice', role: 'owner', grants: personalPermissions, revision: 1 },
  permissions: personalPermissions,
});
const access = (projectId = 'project-a', permissions = personalPermissions): ProjectAccess => ({
  team: summary().team, projectId, permissions, grants: [],
});
const storageKey = () => JSON.stringify(['luci.workspace', productApi.baseUrl.replace(/\/v1\/teams\/[^/]+\/workspace$/, ''), 'alice']);
function remembered(id = teamA, cockpit = id) {
  localStorage.setItem(storageKey(), id);
  localStorage.setItem(storageKey() + '.cockpit', cockpit);
}
function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => { resolve = yes; reject = no; });
  return { promise, resolve, reject };
}
function Probe({ projectId = 'project-a' }: { projectId?: string }) {
  const workspace = useTeamWorkspace()!;
  const current = useTeamProjectPermissions(projectId);
  return <section aria-label="Workspace content">
    <span data-testid="scope">{workspace.teamId || 'personal'}</span>
    <span data-testid="access">{JSON.stringify(current)}</span>
    {current.permissions.read && <label>Project draft<textarea defaultValue="My saved idea" /></label>}
    <button disabled={current.loading || !current.permissions.edit}>Save project</button>
    <button onClick={() => void workspace.refresh().catch(() => {})}>Refresh access</button>
    <button onClick={() => workspace.select(teamB)}>Select Birch</button>
    <button onClick={() => workspace.select('')}>Select personal</button>
  </section>;
}
const permissions = () => JSON.parse(screen.getByTestId('access').textContent!);
const renderWorkspace = (projectId = 'project-a') => render(<TeamWorkspaceProvider><Probe projectId={projectId} /></TeamWorkspaceProvider>);

beforeEach(() => {
  auth.userId = 'alice'; auth.logout.mockReset().mockResolvedValue(undefined);
  selectTeamScope(''); localStorage.clear(); resetCockpit();
  vi.mocked(detachAllTerminals).mockClear();
  vi.spyOn(teamsApi, 'list').mockResolvedValue({ teams: [summary(), summary(teamB)] });
  vi.spyOn(teamsApi, 'projectAccess').mockImplementation(async (_teamId, projectId) => access(projectId));
});
afterEach(() => { vi.restoreAllMocks(); selectTeamScope(''); localStorage.clear(); resetCockpit(); });

it('resolves the remembered team before mounting project content or issuing project requests', async () => {
  remembered();
  const pending = deferred<{ teams: TeamSummary[] }>();
  vi.mocked(teamsApi.list).mockReturnValueOnce(pending.promise);
  renderWorkspace();
  expect(screen.queryByRole('region', { name: 'Workspace content' })).toBeNull();
  expect(teamsApi.projectAccess).not.toHaveBeenCalled();
  await act(async () => pending.resolve({ teams: [summary()] }));
  expect(await screen.findByTestId('scope')).toHaveProperty('textContent', teamA);
  await waitFor(() => expect(permissions().loading).toBe(false));
  expect(selectedTeamId()).toBe(teamA);
  expect(teamsApi.projectAccess).toHaveBeenCalledWith(teamA, 'project-a');
});

it('opens the managed team regardless of a personal preference and refuses workspace switching', async () => {
  remembered('');
  vi.mocked(teamsApi.list).mockResolvedValue({ teams: [summary()], managedTeamId: teamA });
  renderWorkspace();
  await screen.findByRole('textbox', { name: 'Project draft' });
  expect(selectedTeamId()).toBe(teamA);
  await userEvent.click(screen.getByRole('button', { name: 'Select personal' }));
  await userEvent.click(screen.getByRole('button', { name: 'Select Birch' }));
  expect(selectedTeamId()).toBe(teamA);
  expect(screen.queryByRole('dialog')).toBeNull();
});

it('blocks an account outside the managed team without offering personal fallback', async () => {
  vi.mocked(teamsApi.list).mockResolvedValue({ teams: [], managedTeamId: teamA });
  renderWorkspace();
  expect(await screen.findByRole('alert')).toHaveProperty('textContent', expect.stringContaining('Ask a team owner for access'));
  expect(screen.queryByTestId('access')).toBeNull();
  expect(selectedTeamId()).toBe(teamA);
  expect(screen.queryByRole('region', { name: 'Workspace content' })).toBeNull();
  expect(screen.queryByRole('combobox', { name: 'Workspace' })).toBeNull();
  expect(teamsApi.projectAccess).not.toHaveBeenCalled();
});

it('lets an existing nonmember join the bound managed team only after membership is refreshed', async () => {
  const accepted = deferred<TeamDetail>();
  const membership = deferred<{ teams: TeamSummary[]; managedTeamId: string }>();
  vi.mocked(teamsApi.list).mockResolvedValueOnce({ teams: [], managedTeamId: teamA }).mockReturnValueOnce(membership.promise);
  const accept = vi.spyOn(teamInvitationsApi, 'accept').mockReturnValue(accepted.promise);
  renderWorkspace();
  const code = await screen.findByLabelText('Team invitation code');
  expect(screen.queryByRole('region', { name: 'Workspace content' })).toBeNull();
  await userEvent.type(code, 'private-managed-invitation');
  await userEvent.dblClick(screen.getByRole('button', { name: 'Join team' }));
  expect(accept).toHaveBeenCalledExactlyOnceWith('private-managed-invitation');
  expect(screen.queryByRole('region', { name: 'Workspace content' })).toBeNull();
  await act(async () => accepted.resolve({ ...summary(), members: [], executionBoundary: 'dedicated-team' }));
  expect(screen.queryByRole('region', { name: 'Workspace content' })).toBeNull();
  expect(selectedTeamId()).toBe(teamA);
  await act(async () => membership.resolve({ teams: [summary()], managedTeamId: teamA }));
  expect(await screen.findByRole('region', { name: 'Workspace content' })).toBeTruthy();
  expect(screen.queryByLabelText('Team invitation code')).toBeNull();
  expect(selectedTeamId()).toBe(teamA);
});

it('defers private child effects until the first confirmed membership and mounts a clean workspace after joining', async () => {
  const membership = deferred<{ teams: TeamSummary[]; managedTeamId: string }>();
  vi.mocked(teamsApi.list).mockResolvedValueOnce({ teams: [], managedTeamId: teamA }).mockReturnValueOnce(membership.promise);
  vi.spyOn(teamInvitationsApi, 'accept').mockResolvedValue({ ...summary(), members: [], executionBoundary: 'dedicated-team' });
  renderWorkspace();
  await screen.findByLabelText('Team invitation code');
  expect(teamsApi.projectAccess).not.toHaveBeenCalled();
  expect(screen.queryByTestId('access')).toBeNull();
  await userEvent.type(screen.getByLabelText('Team invitation code'), 'confirmed-first-membership');
  await userEvent.click(screen.getByRole('button', { name: 'Join team' }));
  await screen.findByText(/You joined Atlas/);
  expect(teamsApi.projectAccess).not.toHaveBeenCalled();
  expect(screen.queryByTestId('access')).toBeNull();
  await act(async () => membership.resolve({ teams: [summary()], managedTeamId: teamA }));
  await screen.findByRole('textbox', { name: 'Project draft' });
  expect(teamsApi.projectAccess).toHaveBeenCalledExactlyOnceWith(teamA, 'project-a');
  expect(permissions()).toEqual({ permissions: personalPermissions, loading: false, error: '' });
});

it('keeps a previously admitted managed editor mounted but hidden after team access is revoked', async () => {
  vi.mocked(teamsApi.list).mockResolvedValue({ teams: [summary()], managedTeamId: teamA });
  renderWorkspace();
  const draft = await screen.findByRole('textbox', { name: 'Project draft' });
  await userEvent.type(draft, ' retained managed edits');
  vi.mocked(teamsApi.list).mockResolvedValueOnce({ teams: [], managedTeamId: teamA });
  const pending = deferred<ProjectAccess>();
  vi.mocked(teamsApi.projectAccess).mockReturnValueOnce(pending.promise);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  await screen.findByRole('heading', { name: 'Workspace unavailable' });
  expect(screen.queryByRole('textbox', { name: 'Project draft' })).toBeNull();
  expect(draft.isConnected).toBe(true);
  expect(draft).toHaveProperty('value', 'My saved idea retained managed edits');
  expect(selectedTeamId()).toBe(teamA);
  await userEvent.click(screen.getByRole('button', { name: 'Retry connection' }));
  await screen.findByRole('textbox', { name: 'Project draft' });
  expect(screen.getByRole('textbox', { name: 'Project draft' })).toBe(draft);
  await act(async () => pending.resolve(access()));
});

it('keeps an accepted invitation blocked when fresh membership is absent and retries reads without resubmitting its code', async () => {
  vi.mocked(teamsApi.list).mockResolvedValue({ teams: [], managedTeamId: teamA });
  const accept = vi.spyOn(teamInvitationsApi, 'accept').mockResolvedValue({ ...summary(), members: [], executionBoundary: 'dedicated-team' });
  renderWorkspace();
  await userEvent.type(await screen.findByLabelText('Team invitation code'), 'accepted-before-revocation');
  await userEvent.click(screen.getByRole('button', { name: 'Join team' }));
  await screen.findByText(/You joined Atlas/);
  expect(screen.getByRole('alert').textContent).toContain('You no longer have access');
  expect(screen.queryByRole('region', { name: 'Workspace content' })).toBeNull();
  expect(selectedTeamId()).toBe(teamA);
  vi.mocked(teamsApi.list).mockResolvedValueOnce({ teams: [summary()], managedTeamId: teamA });
  await userEvent.click(screen.getByRole('button', { name: 'Retry connection' }));
  expect(await screen.findByRole('region', { name: 'Workspace content' })).toBeTruthy();
  expect(accept).toHaveBeenCalledTimes(1);
});

it('rejects a wrong-team invitation response without switching, refreshing membership or consuming the entry form', async () => {
  vi.mocked(teamsApi.list).mockResolvedValue({ teams: [], managedTeamId: teamA });
  const accept = vi.spyOn(teamInvitationsApi, 'accept').mockResolvedValue({ ...summary(teamB), members: [], executionBoundary: 'dedicated-team' });
  renderWorkspace();
  await userEvent.type(await screen.findByLabelText('Team invitation code'), 'wrong-team-code');
  await userEvent.click(screen.getByRole('button', { name: 'Join team' }));
  await screen.findByText('This invitation belongs to another team. Ask an owner of this managed workspace for its invitation.');
  expect(teamsApi.list).toHaveBeenCalledTimes(1);
  expect(screen.queryByText(/You joined/)).toBeNull();
  expect(screen.queryByRole('region', { name: 'Workspace content' })).toBeNull();
  expect(screen.getByLabelText('Team invitation code')).toHaveProperty('value', 'wrong-team-code');
  expect(screen.getByRole('button', { name: 'Join team' })).toHaveProperty('disabled', false);
  expect(selectedTeamId()).toBe(teamA);
  expect(accept).toHaveBeenCalledTimes(1);
});

it('does not expose an invitation for an unconfirmed managed binding after the initial lookup fails', async () => {
  remembered(teamA);
  vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Workspace connection interrupted'));
  renderWorkspace();
  await screen.findByRole('alert');
  expect(screen.queryByLabelText('Team invitation code')).toBeNull();
  expect(screen.queryByRole('region', { name: 'Workspace content' })).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Sign out to use another account' }));
  expect(auth.logout).toHaveBeenCalledTimes(1);
});

it.each(['account', 'installation'])('clears an entered invitation and fences late acceptance when the %s changes', async change => {
  const accepted = deferred<TeamDetail>();
  vi.mocked(teamsApi.list).mockResolvedValue({ teams: [], managedTeamId: teamA });
  vi.spyOn(teamInvitationsApi, 'accept').mockReturnValue(accepted.promise);
  const view = renderWorkspace();
  await userEvent.type(await screen.findByLabelText('Team invitation code'), 'private-alice-code');
  await userEvent.click(screen.getByRole('button', { name: 'Join team' }));
  if (change === 'account') auth.userId = 'bob';
  else vi.spyOn(productApi, 'baseUrl', 'get').mockReturnValue('https://another.example');
  view.rerender(<TeamWorkspaceProvider><Probe /></TeamWorkspaceProvider>);
  expect(await screen.findByLabelText('Team invitation code')).toHaveProperty('value', '');
  const reads = vi.mocked(teamsApi.list).mock.calls.length;
  await act(async () => accepted.resolve({ ...summary(), members: [], executionBoundary: 'dedicated-team' }));
  expect(teamsApi.list).toHaveBeenCalledTimes(reads);
  expect(screen.queryByText(/You joined/)).toBeNull();
  expect(screen.queryByRole('region', { name: 'Workspace content' })).toBeNull();
});

it('ignores the old account membership refresh after an accepted invitation when the account changes', async () => {
  const membership = deferred<{ teams: TeamSummary[]; managedTeamId: string }>();
  vi.mocked(teamsApi.list).mockResolvedValue({ teams: [], managedTeamId: teamA }).mockResolvedValueOnce({ teams: [], managedTeamId: teamA }).mockReturnValueOnce(membership.promise);
  vi.spyOn(teamInvitationsApi, 'accept').mockResolvedValue({ ...summary(), members: [], executionBoundary: 'dedicated-team' });
  const view = renderWorkspace();
  await userEvent.type(await screen.findByLabelText('Team invitation code'), 'accepted-alice-code');
  await userEvent.click(screen.getByRole('button', { name: 'Join team' }));
  await screen.findByText(/You joined Atlas/);
  auth.userId = 'bob';
  view.rerender(<TeamWorkspaceProvider><Probe /></TeamWorkspaceProvider>);
  await screen.findByLabelText('Team invitation code');
  await act(async () => membership.resolve({ teams: [summary()], managedTeamId: teamA }));
  expect(screen.queryByRole('region', { name: 'Workspace content' })).toBeNull();
  expect(screen.getByLabelText('Team invitation code')).toHaveProperty('value', '');
});

it('fences an accepted invitation refresh after a newer read confirms membership removal', async () => {
  const membership = deferred<{ teams: TeamSummary[]; managedTeamId: string }>();
  vi.mocked(teamsApi.list).mockResolvedValue({ teams: [], managedTeamId: teamA }).mockResolvedValueOnce({ teams: [], managedTeamId: teamA }).mockReturnValueOnce(membership.promise);
  vi.spyOn(teamInvitationsApi, 'accept').mockResolvedValue({ ...summary(), members: [], executionBoundary: 'dedicated-team' });
  renderWorkspace();
  await userEvent.type(await screen.findByLabelText('Team invitation code'), 'accepted-before-newer-read');
  await userEvent.click(screen.getByRole('button', { name: 'Join team' }));
  await screen.findByText(/You joined Atlas/);
  await userEvent.click(screen.getByRole('button', { name: 'Retry connection' }));
  await screen.findByText(/You no longer have access to this team/);
  await act(async () => membership.resolve({ teams: [summary()], managedTeamId: teamA }));
  expect(screen.queryByRole('region', { name: 'Workspace content' })).toBeNull();
  expect(selectedTeamId()).toBe(teamA);
});

it('clears invitation input and fences a late acceptance while signing out from blocked entry', async () => {
  const accepted = deferred<TeamDetail>();
  const logout = deferred<void>();
  auth.logout.mockReturnValue(logout.promise);
  vi.mocked(teamsApi.list).mockResolvedValue({ teams: [], managedTeamId: teamA });
  vi.spyOn(teamInvitationsApi, 'accept').mockReturnValue(accepted.promise);
  renderWorkspace();
  await userEvent.type(await screen.findByLabelText('Team invitation code'), 'private-code-before-sign-out');
  await userEvent.click(screen.getByRole('button', { name: 'Join team' }));
  await userEvent.click(screen.getByRole('button', { name: 'Sign out to use another account' }));
  expect(screen.queryByLabelText('Team invitation code')).toBeNull();
  expect(auth.logout).toHaveBeenCalledTimes(1);
  await act(async () => accepted.resolve({ ...summary(), members: [], executionBoundary: 'dedicated-team' }));
  expect(teamsApi.list).toHaveBeenCalledTimes(1);
  expect(screen.queryByRole('region', { name: 'Workspace content' })).toBeNull();
  await act(async () => logout.resolve());
});

it('retries the complete initial load and restores the remembered team after a connection failure', async () => {
  remembered();
  vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Network unavailable'));
  renderWorkspace();
  expect(await screen.findByRole('alert')).toHaveProperty('textContent', 'Network unavailable');
  expect(screen.queryByLabelText('Workspace content')).toBeNull();
  await userEvent.click(screen.getByRole('button', { name: 'Retry connection' }));
  expect(await screen.findByTestId('scope')).toHaveProperty('textContent', teamA);
  await waitFor(() => expect(permissions().permissions.edit).toBe(true));
  expect(teamsApi.list).toHaveBeenCalledTimes(2);
});

it.each([true, false])('restores navigation only when its stored workspace marker matches (match=%s)', async matching => {
  remembered(teamA, matching ? teamA : teamB);
  useCockpit.setState({ selection: { projectId: 'saved-navigation', branchId: null, worktreeId: null, sessionId: null } });
  renderWorkspace();
  await screen.findByTestId('scope');
  expect(useCockpit.getState().selection.projectId).toBe(matching ? 'saved-navigation' : null);
});

it('does not broadcast draft registry changes while workspace selection still guards the current drafts', async () => {
  const observed = vi.fn();
  const ContextProbe = memo(function ContextProbe() {
    const workspace = useTeamWorkspace()!;
    observed(workspace);
    return <><span data-testid="context-scope">{workspace.teamId}</span><button onClick={() => workspace.select(teamB)}>Select Birch</button></>;
  });
  remembered();
  render(<TeamWorkspaceProvider><ContextProbe /></TeamWorkspaceProvider>);
  expect(await screen.findByTestId('context-scope')).toHaveProperty('textContent', teamA);
  expect(observed).toHaveBeenCalled();
  observed.mockClear();
  const discard = vi.fn();
  act(() => useCockpit.getState().setUnsavedProjectDraft('definition', { projectId: 'project-a', label: 'Definition', lens: 'discover', retained: false, discard }));
  expect(observed).not.toHaveBeenCalled();
  act(() => useCockpit.getState().setUnsavedProjectDraft('definition', { projectId: 'project-a', label: 'Definition', lens: 'discover', retained: true, discard }));
  expect(observed).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button', { name: 'Select Birch' }));
  await screen.findByRole('dialog', { name: 'Switch workspace?' });
  expect(screen.getByRole('button', { name: 'Keep drafts and switch' })).toBeTruthy();
  expect(selectedTeamId()).toBe(teamA);
  expect(observed).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button', { name: 'Stay here' }));
  act(() => useCockpit.getState().setUnsavedProjectDraft('definition', null));
  expect(observed).not.toHaveBeenCalled();
  await userEvent.click(screen.getByRole('button', { name: 'Select Birch' }));
  expect(screen.queryByRole('dialog')).toBeNull();
  expect(screen.getByTestId('context-scope')).toHaveProperty('textContent', teamB);
  expect(observed).toHaveBeenCalled();
  expect(discard).not.toHaveBeenCalled();
});

it('broadcasts revision-only refreshes and fresh membership permissions after context memoization', async () => {
  const teams = [summary(), summary(teamB)];
  vi.mocked(teamsApi.list).mockResolvedValue({ teams });
  const observed = vi.fn();
  const ContextProbe = memo(function ContextProbe() {
    const workspace = useTeamWorkspace()!;
    observed(workspace);
    return <><span data-testid="context-revision">{workspace.revision}</span><button disabled={!workspace.selected?.permissions.edit}>Edit team context</button></>;
  });
  remembered();
  render(<TeamWorkspaceProvider><ContextProbe /><Probe /></TeamWorkspaceProvider>);
  await screen.findByRole('textbox', { name: 'Project draft' });
  expect(screen.getByTestId('context-revision')).toHaveProperty('textContent', '0');
  observed.mockClear();
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  await waitFor(() => expect(screen.getByTestId('context-revision')).toHaveProperty('textContent', '1'));
  await waitFor(() => expect(permissions().loading).toBe(false));
  expect(observed).toHaveBeenCalled();
  expect(teamsApi.projectAccess).toHaveBeenCalledTimes(2);
  expect(screen.getByRole('button', { name: 'Edit team context' })).toHaveProperty('disabled', false);
  const readOnly = { ...noPermissions, read: true };
  vi.mocked(teamsApi.list).mockResolvedValueOnce({ teams: [{ ...summary(), permissions: readOnly }, summary(teamB)] });
  vi.mocked(teamsApi.projectAccess).mockResolvedValueOnce(access('project-a', readOnly));
  observed.mockClear();
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  await waitFor(() => expect(screen.getByTestId('context-revision')).toHaveProperty('textContent', '2'));
  await waitFor(() => expect(permissions()).toEqual({ permissions: readOnly, loading: false, error: '' }));
  expect(observed).toHaveBeenCalled();
  expect(teamsApi.projectAccess).toHaveBeenCalledTimes(3);
  expect(screen.getByRole('button', { name: 'Edit team context' })).toHaveProperty('disabled', true);
  expect(screen.getByRole('button', { name: 'Save project' })).toHaveProperty('disabled', true);
});

it('keeps an unsaved editor mounted while refreshing access and disables every stale write grant', async () => {
  remembered(); renderWorkspace();
  const draft = await screen.findByRole('textbox', { name: 'Project draft' });
  await userEvent.type(draft, ' with unsaved changes');
  const pending = deferred<ProjectAccess>();
  vi.mocked(teamsApi.projectAccess).mockReturnValueOnce(pending.promise);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  await waitFor(() => expect(permissions().loading).toBe(true));
  expect(permissions().permissions).toEqual({ ...noPermissions, read: true });
  expect(screen.getByRole('button', { name: 'Save project' })).toHaveProperty('disabled', true);
  expect(screen.getByRole('textbox', { name: 'Project draft' })).toBe(draft);
  await act(async () => pending.resolve(access()));
  expect(permissions().loading).toBe(false);
  expect(screen.getByRole('textbox', { name: 'Project draft' })).toBe(draft);
  expect(draft).toHaveProperty('value', 'My saved idea with unsaved changes');
  expect(screen.getByRole('button', { name: 'Save project' })).toHaveProperty('disabled', false);
});

it('keeps drafts mounted after a transient team refresh failure and retries without resetting the workspace', async () => {
  remembered(); renderWorkspace();
  const draft = await screen.findByRole('textbox', { name: 'Project draft' });
  await userEvent.type(draft, ' offline draft');
  vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Connection interrupted'));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  expect(await screen.findByRole('status')).toHaveProperty('textContent', expect.stringContaining('Connection interrupted'));
  expect(screen.getByRole('textbox', { name: 'Project draft' })).toBe(draft);
  await userEvent.click(screen.getByRole('button', { name: 'Retry' }));
  await waitFor(() => expect(screen.queryByRole('status')).toBeNull());
  expect(draft).toHaveProperty('value', 'My saved idea offline draft');
  expect(selectedTeamId()).toBe(teamA);
});

it('retains only prior read access after a temporary project lookup failure, then recovers on refresh', async () => {
  remembered(); renderWorkspace();
  const draft = await screen.findByRole('textbox', { name: 'Project draft' });
  await userEvent.type(draft, ' offline edits');
  vi.mocked(teamsApi.projectAccess).mockRejectedValueOnce(new ApiError('network_unavailable', 'Connection interrupted', 0));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  await waitFor(() => expect(permissions().error).toBe('Connection interrupted'));
  expect(permissions().permissions).toEqual({ ...noPermissions, read: true });
  expect(screen.getByRole('button', { name: 'Save project' })).toHaveProperty('disabled', true);
  expect(screen.getByRole('textbox', { name: 'Project draft' })).toBe(draft);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  await waitFor(() => expect(permissions().error).toBe(''));
  expect(draft).toHaveProperty('value', 'My saved idea offline edits');
  vi.mocked(teamsApi.projectAccess).mockRejectedValueOnce(new ApiError('forbidden_origin', 'Project access was removed', 403));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  await waitFor(() => expect(permissions().error).toBe('Project access was removed'));
  expect(permissions().permissions).toEqual(noPermissions);
  expect(screen.queryByRole('textbox', { name: 'Project draft' })).toBeNull();
});

it('hides a revoked team without switching to personal authority or discarding its mounted drafts', async () => {
  remembered(); renderWorkspace();
  const draft = await screen.findByRole('textbox', { name: 'Project draft' });
  await userEvent.type(draft, ' private changes');
  vi.mocked(teamsApi.list).mockResolvedValueOnce({ teams: [] });
  const denied = deferred<ProjectAccess>();
  vi.mocked(teamsApi.projectAccess).mockReturnValueOnce(denied.promise);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  await screen.findByRole('heading', { name: 'Workspace unavailable' });
  expect(screen.queryByRole('textbox', { name: 'Project draft' })).toBeNull();
  expect(draft.isConnected).toBe(true);
  expect(draft).toHaveProperty('value', 'My saved idea private changes');
  expect(selectedTeamId()).toBe(teamA);
  expect(permissions().permissions).toEqual({ ...noPermissions, read: true });
  expect(vi.mocked(detachAllTerminals)).toHaveBeenCalledTimes(2);
  await userEvent.selectOptions(screen.getByRole('combobox', { name: 'Workspace' }), '');
  expect(screen.getByTestId('scope')).toHaveProperty('textContent', 'personal');
  expect(selectedTeamId()).toBe('');
  await act(async () => denied.reject(new Error('Revoked')));
  expect(permissions().permissions).toEqual(personalPermissions);
});

it('clears confirmed denied project permissions and rejects late results from a different project', async () => {
  remembered(); const view = renderWorkspace();
  await screen.findByRole('textbox', { name: 'Project draft' });
  const old = deferred<ProjectAccess>();
  vi.mocked(teamsApi.projectAccess).mockReturnValueOnce(old.promise);
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  const next = deferred<ProjectAccess>();
  vi.mocked(teamsApi.projectAccess).mockReturnValueOnce(next.promise);
  view.rerender(<TeamWorkspaceProvider><Probe projectId="project-b" /></TeamWorkspaceProvider>);
  expect(permissions()).toEqual({ permissions: noPermissions, loading: true, error: '' });
  await act(async () => next.reject(new Error('Project access denied')));
  expect(permissions()).toEqual({ permissions: noPermissions, loading: false, error: 'Project access denied' });
  await act(async () => old.resolve(access()));
  expect(permissions().permissions).toEqual(noPermissions);
  expect(screen.queryByRole('textbox', { name: 'Project draft' })).toBeNull();
});

it('ignores an older successful refresh after the latest refresh revokes membership', async () => {
  remembered(); renderWorkspace();
  await screen.findByRole('textbox', { name: 'Project draft' });
  const stale = deferred<{ teams: TeamSummary[] }>();
  vi.mocked(teamsApi.list).mockReturnValueOnce(stale.promise).mockResolvedValueOnce({ teams: [] });
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  await userEvent.click(screen.getByRole('button', { name: 'Refresh access' }));
  await screen.findByRole('heading', { name: 'Workspace unavailable' });
  await act(async () => stale.resolve({ teams: [summary()] }));
  expect(screen.getByRole('heading', { name: 'Workspace unavailable' })).toBeTruthy();
  expect(selectedTeamId()).toBe(teamA);
});

it.each([true, false])('asks before switching a workspace with unsaved drafts (retained=%s)', async retained => {
  remembered(); renderWorkspace();
  await screen.findByRole('textbox', { name: 'Project draft' });
  const discard = vi.fn();
  act(() => useCockpit.getState().setUnsavedProjectDraft('definition', { projectId: 'project-a', label: 'Definition', lens: 'discover', retained, discard }));
  await userEvent.click(screen.getByRole('button', { name: 'Select Birch' }));
  await screen.findByRole('dialog', { name: 'Switch workspace?' });
  expect(selectedTeamId()).toBe(teamA);
  expect(!!screen.queryByRole('button', { name: 'Keep drafts and switch' })).toBe(retained);
  await userEvent.click(screen.getByRole('button', { name: 'Stay here' }));
  expect(discard).not.toHaveBeenCalled();
  expect(selectedTeamId()).toBe(teamA);
  await userEvent.click(screen.getByRole('button', { name: 'Select Birch' }));
  await userEvent.click(screen.getByRole('button', { name: retained ? 'Keep drafts and switch' : 'Discard edits and switch' }));
  await waitFor(() => expect(screen.getByTestId('scope')).toHaveProperty('textContent', teamB));
  expect(selectedTeamId()).toBe(teamB);
  expect(discard).toHaveBeenCalledTimes(retained ? 0 : 1);
  expect(localStorage.getItem(storageKey())).toBe(teamB);
  expect(useCockpit.getState().unsavedProjectDrafts).toEqual({});
});
it('keeps local check drafts protected on workspace change without suggesting a nonexistent shared save', async () => {
  remembered(); renderWorkspace();
  await screen.findByRole('textbox', { name: 'Project draft' });
  const discard = vi.fn();
  act(() => useCockpit.getState().setUnsavedProjectDraft('checks', { projectId: 'project-a', label: 'Verification setup', lens: 'develop', purpose: 'verification-setup', retained: true, discard }));
  await userEvent.click(screen.getByRole('button', { name: 'Select Birch' }));
  const dialog = await screen.findByRole('dialog', { name: 'Switch workspace?' });
  expect(dialog.textContent).toContain('Continue editing here, keep any retained browser drafts');
  expect(dialog.textContent).not.toContain('Save them before leaving');
  expect(selectedTeamId()).toBe(teamA);
  await userEvent.click(screen.getByRole('button', { name: 'Keep drafts and switch' }));
  await waitFor(() => expect(selectedTeamId()).toBe(teamB));
  expect(discard).not.toHaveBeenCalled();
});

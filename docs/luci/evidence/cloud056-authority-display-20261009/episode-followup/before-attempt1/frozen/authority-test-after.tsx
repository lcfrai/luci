// UNEXECUTED at preparation. Controlled React/transport reproduction; no real gateway,
// browser, xterm renderer, worker, Docker or provider is started by this test.
import { useEffect } from 'react';
import { afterEach, beforeEach, expect, it, vi, type Mock } from 'vitest';
import { act, cleanup, render, screen } from '@testing-library/react';
import { TeamWorkspaceProvider, useTeamWorkspace } from '../../components/teams/TeamWorkspaceProvider';
import { teamsApi, personalPermissions, type TeamSummary } from '../../api/teams';
import { productApi } from '../../api/product';
import { api } from '../../api';
import { createClient } from '../../api/client';
import { startPolling, POLL_INTERVAL_MS } from '../../api/poller';
import { selectTeamScope, selectedTeamId } from '../../api/teamScope';
import { __resetAttachRegistry, configureAttachRegistry, acquireAttach, releaseAttach } from '../../api/attachRegistry';
import { SessionPane } from '../../components/develop/SessionPane';
import { resetCockpit, useCockpit } from '../../store/store';
import { aSession } from '../../test/fixtures';
import { FakeTerm, FakeWebSocket } from '../../test/fakes';
import type { Session } from '../../store/types';

const auth = vi.hoisted(() => ({ userId: 'authority-alice' }));
vi.mock('../../components/auth/AuthProvider', () => ({
  useAuth: () => ({ profile: { user: { id: auth.userId } }, logout: vi.fn() }),
}));
vi.mock('@xterm/xterm', async () => {
  const { FakeTerm } = await import('../../test/fakes');
  return { Terminal: class extends FakeTerm {
    options: Record<string, unknown>;
    constructor(options: Record<string, unknown>) { super(); this.options = options; }
    // A visible text projection of the fake's retained buffer makes the display
    // check explicit. This does NOT test real xterm painting/rendering.
    override write(data: string | Uint8Array, callback?: () => void) {
      super.write(data, callback);
      if (this.openedIn) this.openedIn.textContent = this.written.join('');
    }
  } };
});
vi.mock('@xterm/addon-fit', () => ({ FitAddon: class { fit() {} activate() {} dispose() {} } }));
vi.mock('@xterm/xterm/css/xterm.css', () => ({}));

const teamId = 'team-abcdefghijklmnopqrstuv';
const title = 'Private already-delivered task title';
const output = 'Private already-delivered stdout';
const summary: TeamSummary = {
  team: { id: teamId, name: 'Authority fixture', revision: 1, createdAt: '', updatedAt: '' },
  membership: { userId: 'authority-alice', role: 'editor', grants: personalPermissions, revision: 1 },
  permissions: personalPermissions,
};
const session = aSession({ id: 's-authority', projectId: 'p-authority', branchId: 'b-authority',
  worktreeId: 'w-authority', harness: 'shell', title, status: 'running',
  attachPath: `/v1/teams/${teamId}/workspace/v1/sessions/s-authority/attach` });
let transport: Mock<typeof fetch>;
let client: ReturnType<typeof createClient>;
let mounted: ReturnType<typeof render>;
const response = (value: unknown, status = 200) => new Response(JSON.stringify(value), { status, headers: { 'Content-Type': 'application/json' } });
const success = () => response({ sessions: [session] });
// Structural visibility only: actual renderer/layout is intentionally outside this fixture.
const shown = (node: HTMLElement | null) => !!node && node.isConnected && !node.closest('[hidden]');
const denied = (error = 'team_access_denied') => response({ error, message: 'Team workspace access is unavailable.' }, 403);
async function settle() { await act(async () => { await vi.advanceTimersByTimeAsync(0); }); }

function Surface() {
  const workspace = useTeamWorkspace()!;
  useEffect(() => startPolling(client, useCockpit), []);
  return <section aria-label="Admitted workspace">
    <button onClick={() => workspace.select('')}>Select personal fixture scope</button>
    <SessionPane sessionId={session.id} variant="column" />
  </section>;
}
async function open() {
  const key = JSON.stringify(['luci.workspace', productApi.baseUrl.replace(/\/v1\/teams\/[^/]+\/workspace$/, ''), auth.userId]);
  localStorage.setItem(key, teamId); localStorage.setItem(key + '.cockpit', teamId);
  mounted = render(<TeamWorkspaceProvider><Surface /></TeamWorkspaceProvider>);
  await settle();
  expect(shown(screen.getByText(title))).toBe(true);
  expect(FakeWebSocket.instances.length).toBeGreaterThan(0);
  FakeWebSocket.last()!.acceptOpen();
  const held = acquireAttach(session, { createTerm: () => { throw new Error('The actual Terminal component must own its registry entry'); }, readOnly: false });
  releaseAttach(session.id);
  (held.term as FakeTerm).write(output);
  expect(shown(screen.getByText(output))).toBe(true);
  return held;
}
async function tick() { await act(async () => { await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS); }); }

beforeEach(() => {
  vi.useFakeTimers(); auth.userId = 'authority-alice'; localStorage.clear();
  selectTeamScope(''); resetCockpit(); __resetAttachRegistry(); FakeWebSocket.reset();
  Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
  transport = vi.fn<typeof fetch>(async (url) => String(url).includes(teamId) ? success() : response({ sessions: [] })); client = createClient({ fetch: transport });
  vi.spyOn(teamsApi, 'list').mockResolvedValue({ teams: [summary] });
  vi.spyOn(teamsApi, 'projectAccess').mockResolvedValue({ team: summary.team, projectId: session.projectId, permissions: personalPermissions, grants: [] });
  vi.spyOn(api, 'listProjects').mockResolvedValue([{ id: session.projectId, name: 'Private project', path: '/controlled/project', machineId: 'local' } as never]);
  vi.spyOn(api, 'listSessions').mockResolvedValue([session]);
  configureAttachRegistry({ client: {
    openAttach: (row: Session) => new FakeWebSocket(`ws://127.0.0.1:8730${row.attachPath}`) as unknown as WebSocket,
    getSession: (id: string) => client.getSession(id),
  } });
});
afterEach(() => {
  cleanup(); __resetAttachRegistry(); resetCockpit(); selectTeamScope('');
  vi.restoreAllMocks(); vi.useRealTimers();
});

it('CLOUD-056 stable regression: a fresh authoritative list denial hides already-delivered session content before an unrelated refresh', async () => {
  await open();
  transport.mockImplementation(async () => denied());
  vi.mocked(teamsApi.list).mockReturnValue(new Promise(() => {}));
  // Server revocation closes its socket. No post-revocation stdout is supplied.
  act(() => FakeWebSocket.last()!.emitClose(1006));
  await tick();
  expect(useCockpit.getState().lastError?.code).toBe('team_access_denied');
  // Expected to FAIL on pinned edd0: cached title and output remain visible.
  expect(shown(screen.queryByText(title))).toBe(false);
  expect(shown(screen.queryByText(output))).toBe(false);
});

it('CLOUD-056 control: the independent fresh membership refresh really hides the app and disposes its terminal', async () => {
  const held = await open();
  transport.mockImplementation(async () => denied());
  act(() => FakeWebSocket.last()!.emitClose(1006));
  await tick();
  expect(shown(screen.getByText(title))).toBe(true);
  expect(shown(screen.getByText(output))).toBe(true);
  expect((held.term as FakeTerm).disposed).toBe(false);
  vi.mocked(teamsApi.list).mockResolvedValue({ teams: [] });
  await act(async () => { window.dispatchEvent(new Event('focus')); });
  await settle();
  expect(shown(screen.getByRole('heading', { name: 'Workspace unavailable' }))).toBe(true);
  expect(shown(screen.queryByText(title))).toBe(false);
  expect((held.term as FakeTerm).disposed).toBe(true);
  expect(selectedTeamId()).toBe(teamId);
});

it('CLOUD-056 control: an authoritative filtered 200 removes revoked project sessions', async () => {
  await open();
  transport.mockImplementation(async () => response({ sessions: [] }));
  await tick();
  expect(useCockpit.getState().sessions[session.id]).toBeUndefined();
  expect(screen.queryByText(title)).toBeNull();
  expect(screen.queryByText(output)).toBeNull();
  expect(shown(screen.getByText('This session is no longer listed.'))).toBe(true);
});

it.each(['forbidden_origin', 'forbidden_host'])('CLOUD-056 non-authority control: %s does not imply revoked membership', async code => {
  const held = await open(); transport.mockImplementation(async () => denied(code));
  await tick();
  expect(useCockpit.getState().lastError?.code).toBe(code);
  expect(shown(screen.getByText(title))).toBe(true); expect(shown(screen.getByText(output))).toBe(true);
  expect((held.term as FakeTerm).disposed).toBe(false);
  expect(teamsApi.list).toHaveBeenCalledTimes(1);
});

it('CLOUD-056 non-authority control: a transport failure preserves cached content and clears after success', async () => {
  await open(); transport.mockRejectedValue(new TypeError('Synthetic transport offline'));
  await tick();
  expect(useCockpit.getState().lastError?.code).toBe('network_unavailable');
  expect(shown(screen.getByText(title))).toBe(true); expect(shown(screen.getByText(output))).toBe(true);
  transport.mockImplementation(async () => success()); await tick();
  expect(useCockpit.getState().lastError).toBeNull();
});

it.each(['denial', 'success'])('CLOUD-056 scope control: an old workspace %s cannot apply to a newly selected workspace', async outcome => {
  await open();
  let resolve!: (response: Response) => void;
  transport.mockImplementationOnce(() => new Promise<Response>(yes => { resolve = yes; }));
  await tick();
  act(() => screen.getByRole('button', { name: 'Select personal fixture scope' }).click());
  await settle();
  expect(selectedTeamId()).toBe('');
  await act(async () => resolve(outcome === 'denial' ? denied() : success())); await settle();
  expect(useCockpit.getState().lastError?.code).not.toBe('team_access_denied');
  expect(useCockpit.getState().sessions[session.id]).toBeUndefined();
  expect(screen.queryByRole('heading', { name: 'Workspace unavailable' })).toBeNull();
});

it.each(['denial', 'success'])('CLOUD-056 actor control: an old actor %s cannot apply after the provider changes account scope', async outcome => {
  await open();
  let resolve!: (response: Response) => void;
  transport.mockImplementationOnce(() => new Promise<Response>(yes => { resolve = yes; }));
  await tick();
  act(() => { auth.userId = 'authority-bob'; mounted.rerender(<TeamWorkspaceProvider><Surface /></TeamWorkspaceProvider>); });
  await settle();
  expect(selectedTeamId()).toBe('');
  await act(async () => resolve(outcome === 'denial' ? denied() : success())); await settle();
  expect(useCockpit.getState().lastError?.code).not.toBe('team_access_denied');
  expect(useCockpit.getState().sessions[session.id]).toBeUndefined();
  expect(screen.queryByRole('heading', { name: 'Workspace unavailable' })).toBeNull();
});


it('CLOUD-056 recovery: membership restored without old project grants never exposes the old session again', async () => {
  const held = await open();
  let membership!: (result: { teams: TeamSummary[] }) => void;
  vi.mocked(teamsApi.list).mockReturnValueOnce(new Promise(resolve => { membership=resolve; }));
  transport.mockImplementation(async () => denied()); await tick();
  expect(shown(screen.queryByText(title))).toBe(false);
  expect((held.term as FakeTerm).disposed).toBe(false);
  vi.mocked(api.listProjects).mockResolvedValueOnce([]);
  vi.mocked(api.listSessions).mockResolvedValueOnce([]);
  await act(async () => membership({ teams: [summary] })); await settle();
  expect(screen.queryByText(title)).toBeNull(); expect(screen.queryByText(output)).toBeNull();
  expect(useCockpit.getState().sessions[session.id]).toBeUndefined();
  expect(screen.queryByRole('heading', { name: 'Workspace unavailable' })).toBeNull();
});

it('CLOUD-056 recovery: transient revalidation failure keeps private views hidden and drafts/terminal buffer intact until explicit successful retry', async () => {
  const held = await open();
  act(() => useCockpit.getState().setUnsavedProjectDraft('definition', { projectId: session.projectId, label: 'Definition', lens: 'discover', retained: true }));
  const draft=useCockpit.getState().unsavedProjectDrafts;
  vi.mocked(teamsApi.list).mockRejectedValueOnce(new Error('Synthetic membership network outage'));
  transport.mockImplementation(async () => denied()); await tick();
  expect(shown(screen.queryByText(title))).toBe(false);
  expect(useCockpit.getState().unsavedProjectDrafts).toBe(draft);
  expect((held.term as FakeTerm).disposed).toBe(false);
  expect((held.term as FakeTerm).written).toContain(output);
  expect(screen.getByRole('alert').textContent).toContain('Synthetic membership network outage');
  transport.mockImplementation(async () => success());
  act(() => screen.getByRole('button', { name: 'Retry connection' }).click()); await settle();
  expect(shown(screen.getByText(title))).toBe(true); expect(shown(screen.getByText(output))).toBe(true);
  expect((held.term as FakeTerm).disposed).toBe(false);
  expect(useCockpit.getState().unsavedProjectDrafts).toBe(draft);
});

it('CLOUD-056 coalesces repeated denials without accepting an older membership success', async () => {
  await open();
  let membership!: (result: { teams: TeamSummary[] }) => void;
  vi.mocked(teamsApi.list).mockReturnValueOnce(new Promise(resolve => { membership=resolve; }));
  transport.mockImplementation(async () => denied()); await tick(); await tick();
  expect(teamsApi.list).toHaveBeenCalledTimes(2);
  expect(shown(screen.queryByText(title))).toBe(false);
  await act(async () => membership({ teams: [summary] })); await settle();
  expect(shown(screen.queryByText(title))).toBe(false);
  expect(api.listSessions).not.toHaveBeenCalled();
});

it('CLOUD-056 fresh catalogue failure leaves recovery blocked without deleting already-delivered content', async () => {
  const held=await open();
  vi.mocked(api.listProjects).mockRejectedValueOnce(new Error('Synthetic catalogue failure'));
  transport.mockImplementation(async () => denied()); await tick();
  expect(shown(screen.queryByText(title))).toBe(false);
  expect((held.term as FakeTerm).disposed).toBe(false);
  expect((held.term as FakeTerm).written).toContain(output);
  expect(screen.getByRole('alert').textContent).toContain('Synthetic catalogue failure');
});

// UNEXECUTED at preparation. Controlled React/transport reproduction; no real gateway,
// browser, xterm renderer, worker, Docker or provider is started by this test.
import React, { useEffect } from 'react';
import { afterEach, beforeEach, expect, it, vi, type Mock } from 'vitest';
import { act, cleanup, render, screen } from '@testing-library/react';
import { TeamWorkspaceProvider, useTeamWorkspace } from '/workspace/luci/desktop/src/components/teams/TeamWorkspaceProvider';
import { teamsApi, personalPermissions, type TeamSummary } from '/workspace/luci/desktop/src/api/teams';
import { productApi } from '/workspace/luci/desktop/src/api/product';
import { api } from '/workspace/luci/desktop/src/api';
import { createClient } from '/workspace/luci/desktop/src/api/client';
import { startPolling, POLL_INTERVAL_MS } from '/workspace/luci/desktop/src/api/poller';
import { selectTeamScope, selectedTeamId } from '/workspace/luci/desktop/src/api/teamScope';
import { __resetAttachRegistry, configureAttachRegistry, acquireAttach, releaseAttach } from '/workspace/luci/desktop/src/api/attachRegistry';
import { SessionPane } from '/workspace/luci/desktop/src/components/develop/SessionPane';
import { resetCockpit, useCockpit } from '/workspace/luci/desktop/src/store/store';
import { aSession } from '/workspace/luci/desktop/src/test/fixtures';
import { FakeTerm, FakeWebSocket } from '/workspace/luci/desktop/src/test/fakes';
import type { Session } from '/workspace/luci/desktop/src/store/types';

const auth = vi.hoisted(() => ({ userId: 'authority-alice' }));
vi.mock('/workspace/luci/desktop/src/components/auth/AuthProvider', () => ({
  useAuth: () => ({ profile: { user: { id: auth.userId } }, logout: vi.fn() }),
}));
vi.mock('@xterm/xterm', async () => {
  const { FakeTerm } = await import('/workspace/luci/desktop/src/test/fakes');
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


it('CLOUD-056 episode: settled absent membership does not trigger a new read on each subsequent403 or overwrite its explanation', async () => {
  await open();
  vi.mocked(teamsApi.list).mockResolvedValue({ teams: [] });
  transport.mockImplementation(async () => denied()); await tick();
  expect(teamsApi.list).toHaveBeenCalledTimes(2);
  const explanation=screen.getByRole('alert').textContent;
  expect(explanation).toContain('You no longer have access');
  for(let index=0;index<5;index++)await tick();
  expect(teamsApi.list).toHaveBeenCalledTimes(2);
  expect(screen.getByRole('alert').textContent).toBe(explanation);
  expect(shown(screen.queryByText(title))).toBe(false);
  expect(api.listSessions).not.toHaveBeenCalled();
});

it('CLOUD-056 episode: transient revalidation keeps the buffer and explicit Retry starts one fresh recovery; a later denial starts a new episode', async () => {
  const held=await open();
  vi.mocked(teamsApi.list).mockRejectedValue(new Error('Synthetic membership read unavailable'));
  transport.mockImplementation(async () => denied()); await tick();
  for(let index=0;index<3;index++)await tick();
  expect(teamsApi.list).toHaveBeenCalledTimes(2);
  expect((held.term as FakeTerm).disposed).toBe(false);
  expect((held.term as FakeTerm).written).toContain(output);
  vi.mocked(teamsApi.list).mockResolvedValue({ teams: [summary] });
  transport.mockImplementation(async () => success());
  act(()=>screen.getByRole('button',{name:'Retry connection'}).click());await settle();
  expect(teamsApi.list).toHaveBeenCalledTimes(3);
  expect(shown(screen.getByText(title))).toBe(true);
  expect(shown(screen.getByText(output))).toBe(true);
  vi.mocked(teamsApi.list).mockResolvedValue({teams:[]});
  transport.mockImplementation(async()=>denied());await tick();
  expect(teamsApi.list).toHaveBeenCalledTimes(4);
  expect(shown(screen.queryByText(title))).toBe(false);
});

it('CLOUD-056 episode: the existing focus and60second refresh remain available while automatic denial reads coalesce', async()=>{
  await open();
  vi.mocked(teamsApi.list).mockResolvedValue({teams:[]});
  transport.mockImplementation(async()=>denied());await tick();
  await act(async()=>{window.dispatchEvent(new Event('focus'));});await settle();
  expect(teamsApi.list).toHaveBeenCalledTimes(3);
  await act(async()=>{await vi.advanceTimersByTimeAsync(58000);});
  expect(teamsApi.list).toHaveBeenCalledTimes(4);
  expect(shown(screen.queryByText(title))).toBe(false);
});

it('CLOUD-056 episode: a repeated denial still fences a pending explicit membership recovery', async()=>{
  await open();vi.mocked(teamsApi.list).mockResolvedValueOnce({teams:[]});
  transport.mockImplementation(async()=>denied());await tick();
  let resolve!:(value:{teams:TeamSummary[]})=>void;
  vi.mocked(teamsApi.list).mockReturnValueOnce(new Promise(yes=>{resolve=yes;}));
  act(()=>screen.getByRole('button',{name:'Retry connection'}).click());await settle();
  await tick();
  await act(async()=>resolve({teams:[summary]}));await settle();
  expect(shown(screen.queryByText(title))).toBe(false);
  expect(api.listSessions).not.toHaveBeenCalled();
});

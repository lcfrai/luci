import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SessionPane } from './SessionPane';
import { FakeWebSocket } from '../../test/fakes';
import { aSession } from '../../test/fixtures';
import { resetCockpit, useCockpit } from '../../store/store';
import { __resetAttachRegistry, configureAttachRegistry, getAttachState } from '../../api/attachRegistry';
import { startPolling } from '../../api/poller';
import { ApiError, type CockpitClient } from '../../api/client';
import { __resetSessionVerbs, sessionVerbs } from './sessionVerbs';
import type { Session } from '../../store/types';

const mocks = vi.hoisted(() => ({ restart: vi.fn(), create: vi.fn() }));
vi.mock('../../api', () => ({ api: { restartSession: mocks.restart, createSession: mocks.create } }));
const access = vi.hoisted(() => ({ canExecute: true, canAssist: true, canEdit: true, loading: false, permissions: { secrets: true, admin: true }, message: '' }));
vi.mock('../../hooks/useProjectAccess', () => ({ useProjectAccess: () => access }));
vi.mock('../../hooks/useProjectDraft', () => ({ useProjectDraftScope: () => 'account:workspace' }));
vi.mock('@xterm/xterm', async () => {
  const { FakeTerm } = await import('../../test/fakes');
  return { Terminal: class extends FakeTerm { options: Record<string, unknown>; constructor(options: Record<string, unknown>) { super(); this.options = options; } } };
});
vi.mock('@xterm/addon-fit', () => ({ FitAddon: class { fit() {} activate() {} dispose() {} } }));
vi.mock('@xterm/xterm/css/xterm.css', () => ({}));

beforeEach(() => {
  resetCockpit(); __resetAttachRegistry(); __resetSessionVerbs(); FakeWebSocket.reset();
  mocks.restart.mockReset(); mocks.create.mockReset();
  access.canExecute = true; access.canAssist = true; access.loading = false;
  configureFakeTransport();
});
function configureFakeTransport() {
  configureAttachRegistry({ client: {
    openAttach: (session: Session) => new FakeWebSocket(`ws://localhost${session.attachPath}`) as unknown as WebSocket,
    getSession: async (id: string) => useCockpit.getState().sessions[id],
  } });
}
let stopPoll: (() => void) | undefined;
afterEach(() => { stopPoll?.(); stopPoll = undefined; __resetSessionVerbs(); __resetAttachRegistry(); });

it('attaches once after actual Restart from a cold already-cleaned exited pane without an allocation baseline', async () => {
  const exited = aSession({ id: 'cold-restart', harness: 'shell', status: 'exited', executionGeneration: undefined, exitReason: 'exit status 0', attachPath: '/v1/sessions/cold-restart/attach' });
  const restarted: Session = { ...exited, status: 'running', exitReason: '', executionGeneration: 'b'.repeat(64) };
  let finish!: (value: Session) => void;
  mocks.restart.mockReturnValue(new Promise<Session>(resolve => { finish = resolve; }));
  useCockpit.setState({ sessions: { [exited.id]: exited } });
  render(<SessionPane sessionId={exited.id} variant="column" />);
  expect(getAttachState(exited.id).phase).toBe('ended');
  expect(FakeWebSocket.instances).toHaveLength(0);
  let finishOldPoll!: (value: Session[]) => void;
  stopPoll = startPolling({ listSessions: () => new Promise<Session[]>(resolve => { finishOldPoll = resolve; }) } as unknown as CockpitClient, useCockpit);
  await userEvent.setup().click(screen.getByRole('button', { name: 'Restart', exact: true }));
  expect(mocks.restart).toHaveBeenCalledExactlyOnceWith(exited.id);
  expect(FakeWebSocket.instances).toHaveLength(0);
  await act(async () => finish(restarted));
  expect(useCockpit.getState().sessions[exited.id]).toEqual(restarted);
  expect(screen.queryByRole('button', { name: 'Restart', exact: true })).toBeNull();
  expect(mocks.create).not.toHaveBeenCalled();
  await waitFor(() => expect(FakeWebSocket.instances).toHaveLength(1));
  act(() => FakeWebSocket.last()!.acceptOpen());
  expect(getAttachState(exited.id).phase).toBe('attached');
  await act(async () => finishOldPoll([exited]));
  expect(useCockpit.getState().sessions[exited.id]).toEqual(restarted);
  expect(getAttachState(exited.id).phase).toBe('attached');
  act(() => useCockpit.getState().upsertSession({ ...restarted }));
  expect(FakeWebSocket.instances).toHaveLength(1);
});


function coldPane() {
  const exited = aSession({ id: 'cold-restart', harness: 'shell', status: 'exited', executionGeneration: undefined, exitReason: 'exit status 0', attachPath: '/v1/sessions/cold-restart/attach' });
  const restarted: Session = { ...exited, status: 'running', exitReason: '', executionGeneration: 'b'.repeat(64) };
  let finish!: (value: Session) => void;
  let reject!: (error: unknown) => void;
  mocks.restart.mockReturnValue(new Promise<Session>((resolve, fail) => { finish = resolve; reject = fail; }));
  useCockpit.setState({ sessions: { [exited.id]: exited } });
  const view = render(<SessionPane sessionId={exited.id} variant="column" />);
  return { exited, restarted, finish, reject, view };
}

it('waits for the actual menu Restart response rather than attaching its optimistic starting record', async () => {
  const { exited, restarted, finish } = coldPane();
  const menu = sessionVerbs(exited.id, { close: vi.fn(), beginRename: vi.fn(), createSession: mocks.create, access });
  act(() => menu.find(item => item.id === 'restart')!.run!());
  expect(useCockpit.getState().sessions[exited.id].status).toBe('starting');
  expect(FakeWebSocket.instances).toHaveLength(0);
  expect(mocks.restart).toHaveBeenCalledExactlyOnceWith(exited.id);
  await act(async () => finish(restarted));
  expect(FakeWebSocket.instances).toHaveLength(1);
  act(() => FakeWebSocket.last()!.acceptOpen());
  expect(getAttachState(exited.id).phase).toBe('attached');
  expect(mocks.create).not.toHaveBeenCalled();
});

it('never attaches on an uncertain restart rejection or invents a successful outcome', async () => {
  const { exited, reject } = coldPane();
  await userEvent.setup().click(screen.getByRole('button', { name: 'Restart', exact: true }));
  await act(async () => reject(new ApiError('internal', 'Restart response unavailable', 0)));
  expect(FakeWebSocket.instances).toHaveLength(0);
  expect(getAttachState(exited.id).phase).toBe('ended');
  expect(useCockpit.getState().sessions[exited.id].status).toBe('exited');
  expect(screen.getByRole('button', { name: 'Restart', exact: true })).not.toBeNull();
  expect(mocks.restart).toHaveBeenCalledTimes(1);
  expect(mocks.create).not.toHaveBeenCalled();
});

it('retains current attach authority while an acknowledged restart is received', async () => {
  const { exited, restarted, finish, view } = coldPane();
  await userEvent.setup().click(screen.getByRole('button', { name: 'Restart', exact: true }));
  access.canExecute = false;
  view.rerender(<SessionPane sessionId={exited.id} variant="tile" />);
  await act(async () => finish(restarted));
  expect(FakeWebSocket.instances).toHaveLength(0);
  access.canExecute = true;
  view.rerender(<SessionPane sessionId={exited.id} variant="column" />);
  expect(FakeWebSocket.instances).toHaveLength(1);
  act(() => FakeWebSocket.last()!.acceptOpen());
  access.canExecute = false;
  view.rerender(<SessionPane sessionId={exited.id} variant="tile" />);
  expect(FakeWebSocket.last()!.closedWith).not.toBeNull();
  expect(FakeWebSocket.instances).toHaveLength(1);
});

it.each(['unmount', 'shutdown', 'switch-project'] as const)('does not attach a pending restart response after %s; returning to its pane attaches exactly once', async (departure) => {
  const { exited, restarted, finish, view } = coldPane();
  await userEvent.setup().click(screen.getByRole('button', { name: 'Restart', exact: true }));
  if (departure === 'switch-project') {
    const other = aSession({ id: 'other-session', projectId: 'other-project', status: 'exited' });
    act(() => useCockpit.getState().upsertSession(other));
    view.rerender(<SessionPane sessionId={other.id} variant="column" />);
  } else {
    view.unmount();
    if (departure === 'shutdown') { __resetAttachRegistry(); configureFakeTransport(); }
  }
  await act(async () => finish(restarted));
  expect(FakeWebSocket.instances).toHaveLength(0);
  if (departure === 'switch-project') view.rerender(<SessionPane sessionId={exited.id} variant="column" />);
  else render(<SessionPane sessionId={exited.id} variant="column" />);
  expect(FakeWebSocket.instances).toHaveLength(1);
  act(() => FakeWebSocket.last()!.acceptOpen());
  expect(getAttachState(exited.id).phase).toBe('attached');
  expect(Object.keys(useCockpit.getState().sessions)).toContain(exited.id);
  expect(mocks.restart).toHaveBeenCalledTimes(1);
  expect(mocks.create).not.toHaveBeenCalled();
});

it.each([undefined, '', 'invalid-generation'])('does not infer allocation replacement from a missing/malformed restart fingerprint %j', async (executionGeneration) => {
  const { exited, restarted, finish } = coldPane();
  await userEvent.setup().click(screen.getByRole('button', { name: 'Restart', exact: true }));
  await act(async () => finish({ ...restarted, executionGeneration }));
  expect(FakeWebSocket.instances).toHaveLength(0);
  expect(getAttachState(exited.id).phase).toBe('ended');
  expect(mocks.restart).toHaveBeenCalledTimes(1);
});


it('a late old live list response before any Restart cannot attach a cold ended pane', async () => {
  const { exited } = coldPane();
  let finishList!: (value: Session[]) => void;
  stopPoll = startPolling({ listSessions: () => new Promise<Session[]>(resolve => { finishList = resolve; }) } as unknown as CockpitClient, useCockpit);
  await act(async () => finishList([{ ...exited, status: 'running', executionGeneration: 'a'.repeat(64), exitReason: '' }]));
  expect(FakeWebSocket.instances).toHaveLength(0);
  expect(getAttachState(exited.id).phase).toBe('ended');
  expect(mocks.restart).not.toHaveBeenCalled();
});

it.each([['old', 'success'], ['old', 'uncertain'], ['new', 'success'], ['new', 'uncertain']] as const)('pending Restart plus %s live list fingerprint never attaches before its %s outcome', async (fingerprint, outcome) => {
  const { exited, restarted, finish, reject } = coldPane();
  await userEvent.setup().click(screen.getByRole('button', { name: 'Restart', exact: true }));
  let finishList!: (value: Session[]) => void;
  stopPoll = startPolling({ listSessions: () => new Promise<Session[]>(resolve => { finishList = resolve; }) } as unknown as CockpitClient, useCockpit);
  await act(async () => finishList([{ ...exited, status: 'running', executionGeneration: (fingerprint === 'old' ? 'a' : 'b').repeat(64), exitReason: '' }]));
  expect(FakeWebSocket.instances).toHaveLength(0);
  expect(getAttachState(exited.id).phase).toBe('ended');
  if (outcome === 'uncertain') {
    await act(async () => reject(new ApiError('internal', 'Restart response unavailable', 0)));
    expect(FakeWebSocket.instances).toHaveLength(0);
    expect(getAttachState(exited.id).phase).toBe('ended');
  } else {
    await act(async () => finish(restarted));
    expect(FakeWebSocket.instances).toHaveLength(1);
    act(() => FakeWebSocket.last()!.acceptOpen());
    expect(getAttachState(exited.id).phase).toBe('attached');
  }
  expect(mocks.restart).toHaveBeenCalledTimes(1);
  expect(mocks.create).not.toHaveBeenCalled();
});

it('an acknowledgement belonging to a different project cannot reopen the original entry', async () => {
  const { exited, restarted, finish } = coldPane();
  await userEvent.setup().click(screen.getByRole('button', { name: 'Restart', exact: true }));
  await act(async () => finish({ ...restarted, projectId: 'unrelated-project' }));
  expect(FakeWebSocket.instances).toHaveLength(0);
  expect(getAttachState(exited.id).phase).toBe('ended');
});

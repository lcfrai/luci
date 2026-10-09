import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { act, render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { SessionPane } from './SessionPane';
import { FakeWebSocket } from '../../test/fakes';
import { aSession } from '../../test/fixtures';
import { resetCockpit, useCockpit } from '../../store/store';
import { __resetAttachRegistry, configureAttachRegistry, getAttachState } from '../../api/attachRegistry';
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
  resetCockpit(); __resetAttachRegistry(); FakeWebSocket.reset();
  mocks.restart.mockReset(); mocks.create.mockReset();
  access.canExecute = true; access.canAssist = true; access.loading = false;
  configureAttachRegistry({ client: {
    openAttach: (session: Session) => new FakeWebSocket(`ws://localhost${session.attachPath}`) as unknown as WebSocket,
    getSession: async (id: string) => useCockpit.getState().sessions[id],
  } });
});
afterEach(() => __resetAttachRegistry());

it('attaches once after actual Restart from a cold already-cleaned exited pane without an allocation baseline', async () => {
  const exited = aSession({ id: 'cold-restart', harness: 'shell', status: 'exited', executionGeneration: undefined, exitReason: 'exit status 0', attachPath: '/v1/sessions/cold-restart/attach' });
  const restarted: Session = { ...exited, status: 'running', exitReason: '', executionGeneration: 'b'.repeat(64) };
  let finish!: (value: Session) => void;
  mocks.restart.mockReturnValue(new Promise<Session>(resolve => { finish = resolve; }));
  useCockpit.setState({ sessions: { [exited.id]: exited } });
  render(<SessionPane sessionId={exited.id} variant="column" />);
  expect(getAttachState(exited.id).phase).toBe('ended');
  expect(FakeWebSocket.instances).toHaveLength(0);
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
  act(() => useCockpit.getState().upsertSession({ ...restarted }));
  expect(FakeWebSocket.instances).toHaveLength(1);
});

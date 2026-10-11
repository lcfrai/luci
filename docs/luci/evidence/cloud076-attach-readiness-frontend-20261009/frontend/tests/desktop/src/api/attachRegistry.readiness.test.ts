import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { __resetAttachRegistry, acquireAttach, configureAttachRegistry, getAttachState, GRACE_MS,
  releaseAttach, RETRY_BUDGET_MS, retryAttach, setAttachPermission, prepareAttachRestart,
  updateAttachSession, MAX_OUTPUT_BYTES } from './attachRegistry';
import { attachLabel } from '../components/develop/SessionPaneFooter';
import { aSession } from '../test/fixtures';
import { FakeTerm, FakeWebSocket } from '../test/fakes';
import { encode, Kind, type KindValue } from '../lib/wire';
import { resetCockpit, useCockpit } from '../store/store';
import { startPolling } from './poller';
import type { CockpitClient } from './client';
import type { Session } from '../store/types';

// The identical test body can execute against the original optional-field-free type.
type ReadinessSession = Session & { attachReady?: boolean };
const session = (over: Partial<ReadinessSession> = {}): ReadinessSession => aSession({
  id: 'readiness-session', status: 'starting', executionGeneration: 'a'.repeat(64),
  ...over,
});
const ready = (over: Partial<ReadinessSession> = {}) => session({ attachReady: true, ...over });
const preparing = (over: Partial<ReadinessSession> = {}) => session({ attachReady: false, ...over });
const frame = (kind: KindValue, value: string) => encode(kind, new TextEncoder().encode(value));
let term: FakeTerm;
let open: ReturnType<typeof vi.fn<(value: Session) => WebSocket | Promise<WebSocket>>>;
const acquire = (value: ReadinessSession) => acquireAttach(value, {
  createTerm: () => (term = new FakeTerm()), readOnly: value.status === 'exited',
});
beforeEach(() => {
  vi.useFakeTimers(); FakeWebSocket.reset(); __resetAttachRegistry(); resetCockpit();
  open = vi.fn((value: Session) => new FakeWebSocket('ws://localhost' + value.attachPath) as unknown as WebSocket);
  configureAttachRegistry({ client: { openAttach: open, getSession: async () => ready() } });
});
afterEach(() => { __resetAttachRegistry(); vi.useRealTimers(); });

describe('recorded endpoint preparation is distinct from a retrying transport', () => {
  it('starts no ticket/socket or retry clock until a same-generation endpoint is published', () => {
    acquire(preparing());
    for (let i = 0; i < 3; i++) {
      updateAttachSession(preparing());
      vi.advanceTimersByTime(RETRY_BUDGET_MS);
    }
    expect(open).not.toHaveBeenCalled();
    expect(retryAttach('readiness-session')).toBe(false);
    expect(getAttachState('readiness-session')).toMatchObject({ phase: 'preparing', attempts: 0, ladderStartedAt: 0 });
    expect(attachLabel(getAttachState('readiness-session'), Date.now())).toBe('preparing');
    expect(getAttachState('readiness-session').reason).not.toMatch(/reconnect|ready to|authoriz/i);
    updateAttachSession(ready());
    expect(open).toHaveBeenCalledTimes(1);
    FakeWebSocket.last()!.acceptOpen();
    updateAttachSession(ready());
    expect(open).toHaveBeenCalledTimes(1);
    expect(getAttachState('readiness-session').phase).toBe('attached');
  });

  it.each([undefined, true])('preserves immediate starting attach when readiness is %j', (attachReady) => {
    acquire(session({ attachReady }));
    expect(open).toHaveBeenCalledTimes(1);
    FakeWebSocket.last()!.acceptOpen();
    expect(getAttachState('readiness-session').phase).toBe('attached');
  });

  it('keeps older-server behavior when a readiness wait is followed by omission', () => {
    acquire(preparing());
    updateAttachSession(session());
    expect(open).toHaveBeenCalledTimes(1);
  });

  it('preserves a warm transcript while a replacement allocation is preparing', () => {
    acquire(ready({ status: 'running' }));
    const old = FakeWebSocket.last()!; old.acceptOpen();
    old.emitBinary(frame(Kind.Stdout, 'retained transcript'));
    old.emitBinary(frame(Kind.Exit, 'exit status 0')); old.emitClose(1000);
    const originalTerm = term;
    updateAttachSession(preparing({ executionGeneration: 'b'.repeat(64) }));
    vi.advanceTimersByTime(RETRY_BUDGET_MS + 1);
    expect(open).toHaveBeenCalledTimes(1);
    expect(getAttachState('readiness-session').phase).toBe('preparing');
    expect(originalTerm.written.join('')).toContain('retained transcript');
    expect(originalTerm.disposed).toBe(false);
    updateAttachSession(ready({ executionGeneration: 'b'.repeat(64) }));
    expect(open).toHaveBeenCalledTimes(2);
    expect(term).toBe(originalTerm);
  });

  it('retains typed terminal outcome when readiness changes on the same allocation', () => {
    acquire(ready()); const socket = FakeWebSocket.last()!; socket.acceptOpen();
    socket.emitBinary(frame(Kind.Exit, 'exit status 0')); socket.emitClose(1000);
    updateAttachSession(preparing()); updateAttachSession(ready());
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(open).toHaveBeenCalledTimes(1);
    expect(getAttachState('readiness-session').phase).toBe('ended');
  });

  it('terminal publication cancels a readiness wait without inventing successful attachment', () => {
    acquire(preparing());
    updateAttachSession(preparing({ status: 'exited', exitReason: 'the sandbox could not be recreated' }));
    updateAttachSession(ready({ status: 'exited' }));
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(open).not.toHaveBeenCalled();
    expect(getAttachState('readiness-session').phase).toBe('ended');
  });
});

describe('readiness never grants restart, scope or execution authority', () => {
  it('cold live polls cannot use false-to-true readiness as a missing Restart acknowledgement', () => {
    acquire(session({ status: 'exited', executionGeneration: undefined }));
    updateAttachSession(preparing({ executionGeneration: 'b'.repeat(64) }));
    updateAttachSession(ready({ executionGeneration: 'b'.repeat(64) }));
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(open).not.toHaveBeenCalled();
    expect(getAttachState('readiness-session').phase).toBe('ended');
  });

  it.each(['ack-before-poll', 'ack-after-poll'])('an exact cold Restart acknowledgement enables only preparation: %s', (order) => {
    const ended = session({ status: 'exited', executionGeneration: undefined }); acquire(ended);
    const acknowledge = prepareAttachRestart(ended.id), pending = preparing({ executionGeneration: 'b'.repeat(64) });
    if (order === 'ack-before-poll') acknowledge(pending);
    updateAttachSession(pending);
    if (order === 'ack-after-poll') acknowledge(pending);
    expect(open).not.toHaveBeenCalled();
    expect(getAttachState(ended.id).phase).toBe('preparing');
    updateAttachSession(ready({ executionGeneration: 'b'.repeat(64) }));
    expect(open).toHaveBeenCalledTimes(1);
  });

  it('a rejected or wrong-scope Restart cannot arm a readiness wake', () => {
    const ended = session({ status: 'exited', executionGeneration: undefined }); acquire(ended);
    const acknowledge = prepareAttachRestart(ended.id);
    acknowledge(preparing({ projectId: 'foreign', executionGeneration: 'b'.repeat(64) }));
    updateAttachSession(preparing({ executionGeneration: 'b'.repeat(64) }));
    updateAttachSession(ready({ executionGeneration: 'b'.repeat(64) }));
    expect(open).not.toHaveBeenCalled();
  });

  it.each(['projectId', 'machineId', 'worktreeId', 'branchId', 'attachPath'] as const)('ignores a ready update for a different %s', (field) => {
    acquire(preparing()); updateAttachSession(ready({ [field]: 'foreign' }));
    expect(open).not.toHaveBeenCalled();
    expect(getAttachState('readiness-session').phase).toBe('preparing');
  });

  it('permission denial wins over readiness; restoration starts exactly one eligible attach', () => {
    acquire(preparing()); setAttachPermission('readiness-session', false);
    updateAttachSession(ready());
    expect(open).not.toHaveBeenCalled();
    setAttachPermission('readiness-session', true);
    expect(open).toHaveBeenCalledTimes(1);
    FakeWebSocket.last()!.acceptOpen();
    setAttachPermission('readiness-session', false);
    updateAttachSession(ready());
    expect(open).toHaveBeenCalledTimes(1);
    expect(FakeWebSocket.last()!.closedWith).not.toBeNull();
  });

  it('background permission refresh pauses new attachment even after endpoint publication', () => {
    acquire(preparing()); setAttachPermission('readiness-session', true, false);
    updateAttachSession(ready()); expect(open).not.toHaveBeenCalled();
    setAttachPermission('readiness-session', true, true); expect(open).toHaveBeenCalledTimes(1);
  });

  it('an unmounted cached wait resumes only on reacquire, preserving its terminal', () => {
    const handle = acquire(preparing()); releaseAttach('readiness-session'); updateAttachSession(ready());
    expect(open).not.toHaveBeenCalled();
    const again = acquireAttach(ready(), { createTerm: () => { throw Error('must retain terminal'); }, readOnly: false });
    expect(again.term).toBe(handle.term); expect(open).toHaveBeenCalledTimes(1);
  });

  it('expired or explicitly reset registry entries cannot be revived by ready updates', () => {
    acquire(preparing()); releaseAttach('readiness-session'); vi.advanceTimersByTime(GRACE_MS + 1);
    updateAttachSession(ready()); expect(open).not.toHaveBeenCalled(); expect(term.disposed).toBe(true);
    acquire(preparing()); __resetAttachRegistry(); updateAttachSession(ready()); expect(open).not.toHaveBeenCalled();
  });
});

describe('readiness fences retries and delayed sockets without stopping an established stream', () => {
  it('cancels an armed retry before it creates another ticket or socket', () => {
    acquire(ready()); FakeWebSocket.last()!.emitClose(1006);
    updateAttachSession(preparing()); vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(open).toHaveBeenCalledTimes(1);
    expect(getAttachState('readiness-session')).toMatchObject({ phase: 'preparing', attempts: 0, ladderStartedAt: 0 });
    updateAttachSession(ready()); expect(open).toHaveBeenCalledTimes(2);
  });

  it('a false hint preserves an already open stream and defers its next reconnect', () => {
    acquire(ready()); const socket = FakeWebSocket.last()!; socket.acceptOpen();
    updateAttachSession(preparing());
    expect(socket.closedWith).toBeNull(); expect(getAttachState('readiness-session').phase).toBe('attached');
    socket.emitClose(1006); vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(open).toHaveBeenCalledTimes(1); expect(getAttachState('readiness-session').phase).toBe('preparing');
    updateAttachSession(ready()); expect(open).toHaveBeenCalledTimes(2);
  });

  it('rejects a late async ticket/socket from before readiness was withdrawn', async () => {
    let resolve!: (socket: WebSocket) => void;
    open.mockImplementationOnce(() => new Promise<WebSocket>(done => { resolve = done; }));
    acquire(ready()); updateAttachSession(preparing());
    const late = new FakeWebSocket('ws://localhost/old'); resolve(late as unknown as WebSocket);
    await Promise.resolve();
    expect(late.closedWith).not.toBeNull(); expect(getAttachState('readiness-session').phase).toBe('preparing');
    updateAttachSession(ready()); expect(open).toHaveBeenCalledTimes(2);
  });

  it('readiness cannot clear output-overload protection', () => {
    acquire(ready()); const socket = FakeWebSocket.last()!; socket.acceptOpen();
    socket.emitBinary(encode(Kind.Stdout, new Uint8Array(MAX_OUTPUT_BYTES + 1)));
    updateAttachSession(preparing()); updateAttachSession(ready()); vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(open).toHaveBeenCalledTimes(1); expect(getAttachState('readiness-session').phase).toBe('failed');
  });

  it('a readiness-only store change reaches the terminal, while an older in-flight poll cannot undo it', async () => {
    const pending = preparing(); useCockpit.getState().upsertSession(pending); acquire(pending);
    let resolve!: (rows: Session[]) => void;
    const unsubscribe = useCockpit.subscribe(state => {
      const value = state.sessions[pending.id]; if (value) updateAttachSession(value);
    });
    const stop = startPolling({ listSessions: () => new Promise<Session[]>(done => { resolve = done; }) } as unknown as CockpitClient, useCockpit);
    try {
      const published = ready(); useCockpit.getState().upsertSession(published);
      expect(open).toHaveBeenCalledTimes(1);
      resolve([pending]); await Promise.resolve(); await Promise.resolve();
      expect(useCockpit.getState().sessions[pending.id]).toEqual(published);
      expect(open).toHaveBeenCalledTimes(1);
    } finally { stop(); unsubscribe(); }
  });
});

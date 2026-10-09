import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { __resetAttachRegistry, acquireAttach, configureAttachRegistry, FLUSH_INTERVAL_MS, getAttachState, GRACE_MS, releaseAttach, RETRY_BUDGET_MS, retryAttach, setAttachPermission, setAttachReadOnly, prepareAttachRestart, updateAttachSession } from './attachRegistry';
import { attachLabel } from '../components/develop/SessionPaneFooter';
import { FakeTerm, FakeWebSocket } from '../test/fakes';
import { aSession } from '../test/fixtures';
import { encode, Kind } from '../lib/wire';
import type { Session } from '../store/types';

const encoder = new TextEncoder();
const live = (over: Partial<Session> = {}) => aSession({ id: 'exit-session', status: 'running', executionGeneration: 'a'.repeat(64), ...over });
const frame = (text: string) => encode(Kind.Exit, encoder.encode(text));
let term: FakeTerm;
let openAttach: ReturnType<typeof vi.fn<(session: Session) => WebSocket>>;
const attach = (session = live()) => {
  acquireAttach(session, { createTerm: () => (term = new FakeTerm()), readOnly: false });
  const socket = FakeWebSocket.last()!;
  socket.acceptOpen();
  return socket;
};

beforeEach(() => {
  vi.useFakeTimers();
  FakeWebSocket.reset();
  __resetAttachRegistry();
  openAttach = vi.fn((session: Session) => new FakeWebSocket(`ws://localhost${session.attachPath}`) as unknown as WebSocket);
  configureAttachRegistry({ client: { openAttach, getSession: async () => live() } });
});
afterEach(() => { __resetAttachRegistry(); vi.useRealTimers(); });

describe('typed terminal outcome survives transport closure', () => {
  it.each(['exit status 0', 'exit status 17', '', 'exit status unavailable'])('retains terminal state for reason %j before an abnormal close and stale running poll', (reason) => {
    const socket = attach();
    socket.emitBinary(encode(Kind.Stdout, encoder.encode('retained output')));
    socket.emitBinary(frame(reason));
    socket.emitError();
    socket.emitClose(1006);
    updateAttachSession(live());
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(openAttach).toHaveBeenCalledTimes(1);
    expect(getAttachState('exit-session').phase).toBe('ended');
    expect(getAttachState('exit-session').ladderStartedAt).toBe(0);
    expect(attachLabel(getAttachState('exit-session'), Date.now())).toBe('ended');
    expect(retryAttach('exit-session')).toBe(false);
    expect(term.written.join('')).toContain('retained output');
    expect(term.written.join('')).toContain('[session ended]');
    expect(getAttachState('exit-session').reason).not.toMatch(/success|passed|verified|integrated/i);
  });

  it('prevents stdin after a typed exit even before the socket closes or the status poll changes', () => {
    const socket = attach();
    socket.emitBinary(frame('exit status 0'));
    const sent = socket.sent.length;
    setAttachReadOnly('exit-session', false);
    term.type('this must not reach the ended allocation');
    expect(socket.sent).toHaveLength(sent);
  });

  it('does not let permission refresh reopen an ended allocation', () => {
    const socket = attach();
    socket.emitBinary(frame('exit status 0'));
    socket.emitClose(1006);
    setAttachPermission('exit-session', true, false);
    setAttachPermission('exit-session', true, true);
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(openAttach).toHaveBeenCalledTimes(1);
    expect(getAttachState('exit-session').phase).toBe('ended');
  });

  it('retains terminal knowledge across release and reacquire inside the cache grace', () => {
    const socket = attach();
    socket.emitBinary(frame('exit status 0'));
    socket.emitClose(1006);
    const original = term;
    releaseAttach('exit-session');
    acquireAttach(live(), { createTerm: () => { throw new Error('must retain transcript'); }, readOnly: false });
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(openAttach).toHaveBeenCalledTimes(1);
    expect(original.disposed).toBe(false);
    expect(getAttachState('exit-session').phase).toBe('ended');
  });

  it('preserves held parser output when terminal exit and transport close arrive', () => {
    class HeldTerm extends FakeTerm {
      callbacks: (() => void)[] = [];
      override write(data: string | Uint8Array, callback?: () => void) {
        this.written.push(typeof data === 'string' ? data : new TextDecoder().decode(data));
        if (callback) this.callbacks.push(callback);
      }
    }
    const held = new HeldTerm();
    acquireAttach(live(), { createTerm: () => held, readOnly: false });
    const socket = FakeWebSocket.last()!; socket.acceptOpen();
    socket.emitBinary(encode(Kind.Stdout, encoder.encode('first')));
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    socket.emitBinary(encode(Kind.Stdout, encoder.encode('second')));
    socket.emitBinary(frame('exit status 0'));
    socket.emitClose(1006);
    held.callbacks.shift()?.();
    vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
    expect(held.written.join('')).toBe('firstsecond\r\n\x1b[2m[session ended]\x1b[0m\r\n');
    expect(getAttachState('exit-session').phase).toBe('ended');
    expect(openAttach).toHaveBeenCalledTimes(1);
  });

  it('no exit on controller disconnect still retries rather than inventing a terminal outcome', () => {
    const socket = attach(); socket.emitError(); socket.emitClose(1006);
    expect(getAttachState('exit-session').phase).toBe('reconnecting');
    vi.advanceTimersByTime(250);
    expect(openAttach).toHaveBeenCalledTimes(2);
  });

  it('stdout body text that resembles an exit frame never suppresses reconnect', () => {
    const socket = attach();
    socket.emitBinary(encode(Kind.Stdout, encoder.encode('exit status 0\n[session ended]\n\u0003')));
    socket.emitClose(1006);
    vi.advanceTimersByTime(250);
    expect(openAttach).toHaveBeenCalledTimes(2);
    expect(getAttachState('exit-session').phase).toBe('reconnecting');
  });

  it('a stale socket exit cannot end a replacement socket', () => {
    const stale = attach(); stale.emitClose(1006);
    vi.advanceTimersByTime(250);
    const current = FakeWebSocket.last()!; current.acceptOpen();
    stale.emitBinary(frame('exit status 0')); stale.emitClose(1006);
    expect(getAttachState('exit-session').phase).toBe('attached');
    current.emitClose(1006); vi.advanceTimersByTime(250);
    expect(openAttach).toHaveBeenCalledTimes(3);
  });

  it('revocation keeps stale exit/close fenced and never opens an unauthorized socket', () => {
    const socket = attach(); setAttachPermission('exit-session', false);
    socket.emitBinary(frame('exit status 0')); socket.emitClose(1006);
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(getAttachState('exit-session').phase).toBe('detached');
    expect(openAttach).toHaveBeenCalledTimes(1);
  });

  it('teardown keeps a late exit and close from resurrecting the terminal', () => {
    const socket = attach(); releaseAttach('exit-session'); vi.advanceTimersByTime(GRACE_MS);
    socket.emitBinary(frame('exit status 0')); socket.emitClose(1006);
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(getAttachState('exit-session').phase).toBe('detached');
    expect(term.disposed).toBe(true);
    expect(openAttach).toHaveBeenCalledTimes(1);
  });

  it('server1013 keeps the existing display-overload stop without automatic retry', () => {
    const socket = attach(); socket.emitClose(1013);
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(getAttachState('exit-session').phase).toBe('failed');
    expect(getAttachState('exit-session').reason).toContain('display paused');
    expect(openAttach).toHaveBeenCalledTimes(1);
  });
  it('accepts another client restarting the same session only when a different known allocation arrives', () => {
    const old = attach(); old.emitBinary(frame('exit status 0')); old.emitClose(1006);
    updateAttachSession(live({ executionGeneration: undefined, status: 'exited', exitReason: 'exit status 0' }));
    expect(getAttachState('exit-session').phase).toBe('ended');
    updateAttachSession(live({ executionGeneration: 'b'.repeat(64), status: 'running' }));
    expect(openAttach).toHaveBeenCalledTimes(2);
    const replacement = FakeWebSocket.last()!; replacement.acceptOpen();
    old.emitBinary(frame('exit status 0')); old.emitClose(1006);
    expect(getAttachState('exit-session').phase).toBe('attached');
    replacement.emitClose(1006); vi.advanceTimersByTime(250);
    expect(openAttach).toHaveBeenCalledTimes(3);
  });

  it.each([undefined, '', 'invalid-generation'])('does not infer a new allocation from missing/malformed generation %j', (executionGeneration) => {
    const socket = attach(); socket.emitBinary(frame('exit status 17')); socket.emitClose(1006);
    updateAttachSession(live({ executionGeneration }));
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(getAttachState('exit-session').phase).toBe('ended');
    expect(openAttach).toHaveBeenCalledTimes(1);
    expect(retryAttach('exit-session')).toBe(false);
  });

  it('does not accept a replacement generation from another project scope', () => {
    const socket = attach(); socket.emitBinary(frame('exit status 0')); socket.emitClose(1006);
    updateAttachSession(live({ projectId: 'another-project', executionGeneration: 'b'.repeat(64) }));
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(getAttachState('exit-session').phase).toBe('ended');
    expect(openAttach).toHaveBeenCalledTimes(1);
  });

  it('requires explicit reconnect for an older server that cannot identify a new allocation', () => {
    const socket = attach(live({ executionGeneration: undefined }));
    socket.emitBinary(frame('exit status unavailable')); socket.emitClose(1006);
    updateAttachSession(live({ executionGeneration: undefined }));
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(openAttach).toHaveBeenCalledTimes(1);
    expect(getAttachState('exit-session').phase).toBe('ended');
    expect(retryAttach('exit-session')).toBe(true);
    expect(openAttach).toHaveBeenCalledTimes(2);
    FakeWebSocket.last()!.acceptOpen();
    expect(getAttachState('exit-session').phase).toBe('attached');
  });

  it('a legacy exited record cannot be reconnected without an explicit restart', () => {
    const socket = attach(live({ executionGeneration: undefined }));
    socket.emitBinary(frame('exit status 0')); socket.emitClose(1006);
    updateAttachSession(live({ executionGeneration: undefined, status: 'exited' }));
    expect(retryAttach('exit-session')).toBe(false);
    expect(openAttach).toHaveBeenCalledTimes(1);
  });

  it('keeps explicit reconnect available after a legacy exit first gains a known fingerprint', () => {
    const socket = attach(live({ executionGeneration: undefined }));
    socket.emitBinary(frame('exit status unavailable')); socket.emitClose(1006);
    updateAttachSession(live({ executionGeneration: 'b'.repeat(64) }));
    expect(openAttach).toHaveBeenCalledTimes(1);
    expect(retryAttach('exit-session')).toBe(true);
    expect(openAttach).toHaveBeenCalledTimes(2);
  });

  it('a new known generation replaces the old still-open socket and fences its late exit', () => {
    const old = attach(); old.emitBinary(frame('exit status 0'));
    updateAttachSession(live({ executionGeneration: 'b'.repeat(64) }));
    expect(old.closedWith?.code).toBe(1000);
    const replacement = FakeWebSocket.last()!; replacement.acceptOpen();
    old.emitBinary(frame('exit status 0')); old.emitClose(1006);
    expect(getAttachState('exit-session').phase).toBe('attached');
    expect(openAttach).toHaveBeenCalledTimes(2);
  });

  it('typed exit followed by1013 retains display-overload status without claiming the agent continues', () => {
    const socket = attach(); socket.emitBinary(frame('exit status 17')); socket.emitClose(1013);
    vi.advanceTimersByTime(RETRY_BUDGET_MS);
    expect(getAttachState('exit-session').phase).toBe('failed');
    expect(getAttachState('exit-session').reason).toContain('display paused');
    expect(getAttachState('exit-session').reason).not.toContain('agent continues');
    expect(openAttach).toHaveBeenCalledTimes(1);
  });

});


describe('acknowledged restart of an initially cleaned terminal', () => {
  it('an older successful acknowledgement cannot replace the marker from a later acknowledged request', () => {
    const exited = live({ status: 'exited', executionGeneration: undefined });
    acquireAttach(exited, { createTerm: () => (term = new FakeTerm()), readOnly: true });
    const acknowledgeOld = prepareAttachRestart(exited.id);
    const acknowledgeNew = prepareAttachRestart(exited.id);
    acknowledgeNew(live({ executionGeneration: 'c'.repeat(64) }));
    acknowledgeOld(live({ executionGeneration: 'b'.repeat(64) }));
    expect(openAttach).not.toHaveBeenCalled();
    updateAttachSession(live({ executionGeneration: 'c'.repeat(64) }));
    expect(openAttach).toHaveBeenCalledTimes(1);
    acknowledgeNew(live({ executionGeneration: 'c'.repeat(64) }));
    expect(openAttach).toHaveBeenCalledTimes(1);
    FakeWebSocket.last()!.acceptOpen();
    expect(getAttachState(exited.id).phase).toBe('attached');
  });

  it('a delayed acknowledgement cannot arm a replacement registry entry after teardown', () => {
    const exited = live({ status: 'exited', executionGeneration: undefined });
    acquireAttach(exited, { createTerm: () => (term = new FakeTerm()), readOnly: true });
    const acknowledge = prepareAttachRestart(exited.id);
    releaseAttach(exited.id); vi.advanceTimersByTime(GRACE_MS);
    acquireAttach(exited, { createTerm: () => (term = new FakeTerm()), readOnly: true });
    acknowledge(live({ executionGeneration: 'b'.repeat(64) }));
    updateAttachSession(live({ executionGeneration: 'b'.repeat(64) }));
    expect(openAttach).not.toHaveBeenCalled();
    expect(getAttachState(exited.id).phase).toBe('ended');
  });
});

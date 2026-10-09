import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { __resetAttachRegistry, acquireAttach, configureAttachRegistry, getAttachState, FLUSH_INTERVAL_MS, MAX_OUTPUT_BYTES, RETRY_BUDGET_MS, retryAttach, updateAttachSession } from './attachRegistry';
import { FakeTerm, FakeWebSocket } from '../test/fakes';
import { aSession } from '../test/fixtures';
import type { Session } from '../store/types';
import { encode, Kind } from '../lib/wire';

beforeEach(() => { vi.useFakeTimers(); FakeWebSocket.reset(); __resetAttachRegistry(); });
afterEach(() => { __resetAttachRegistry(); vi.useRealTimers(); });

it('keeps a display-overflow pause through allocation replacement until the user explicitly reconnects', () => {
  const generationA = aSession({ id: 'overload-generation', executionGeneration: 'a'.repeat(64), status: 'running' });
  const openAttach = vi.fn((session: Session) => new FakeWebSocket(`ws://localhost${session.attachPath}`) as unknown as WebSocket);
  configureAttachRegistry({ client: { openAttach, getSession: async () => generationA } });
  const term = new FakeTerm();
  acquireAttach(generationA, { createTerm: () => term, readOnly: false });
  const oldSocket = FakeWebSocket.last()!;
  oldSocket.acceptOpen();
  oldSocket.emitBinary(encode(Kind.Stdout, new Uint8Array(MAX_OUTPUT_BYTES + 1)));
  const paused = getAttachState(generationA.id);
  expect(paused.phase).toBe('failed');
  expect(paused.reason).toContain('earlier output may be missing');
  expect(oldSocket.closedWith?.code).toBe(4008);
  const writtenBeforeReplacement = [...term.written];

  const generationB = { ...generationA, executionGeneration: 'b'.repeat(64) };
  updateAttachSession(generationB);
  oldSocket.emitBinary(encode(Kind.Exit, new TextEncoder().encode('exit status 0')));
  oldSocket.emitClose(1006);
  vi.advanceTimersByTime(RETRY_BUDGET_MS);
  expect(openAttach).toHaveBeenCalledTimes(1);
  expect(getAttachState(generationA.id)).toEqual(paused);
  expect(term.written).toEqual(writtenBeforeReplacement);
  expect(term.disposed).toBe(false);

  expect(retryAttach(generationA.id)).toBe(true);
  expect(openAttach).toHaveBeenCalledTimes(2);
  expect(openAttach.mock.calls[1][0]).toBe(generationB);
  vi.advanceTimersByTime(FLUSH_INTERVAL_MS);
  expect(term.written.join('')).toContain('\x18\x1bc');
  const replacement = FakeWebSocket.last()!;
  replacement.acceptOpen();
  expect(getAttachState(generationA.id).phase).toBe('attached');
  oldSocket.emitBinary(encode(Kind.Exit, new TextEncoder().encode('exit status 0')));
  expect(getAttachState(generationA.id).phase).toBe('attached');
  expect(openAttach).toHaveBeenCalledTimes(2);
});

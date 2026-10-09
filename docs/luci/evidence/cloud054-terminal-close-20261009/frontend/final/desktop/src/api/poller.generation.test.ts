import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { startPolling, POLL_INTERVAL_MS } from './poller';
import type { CockpitClient, ProblemCode } from './client';
import { ApiError } from './client';
import { aSession } from '../test/fixtures';
import { resetCockpit, useCockpit } from '../store/store';
import type { Session } from '../store/types';
import { acquireAttach, configureAttachRegistry, __resetAttachRegistry, updateAttachSession, getAttachState } from './attachRegistry';
import { FakeTerm, FakeWebSocket } from '../test/fakes';
import { Kind, encode } from '../lib/wire';

let stop: () => void;
const fixture = (over: Partial<Session> = {}) => aSession({ id: 'generation-session', executionGeneration: 'a'.repeat(64), ...over });
beforeEach(() => {
 vi.useFakeTimers(); resetCockpit(); FakeWebSocket.reset(); __resetAttachRegistry();
 Object.defineProperty(document, 'visibilityState', { configurable: true, value: 'visible' });
});
afterEach(() => { stop?.(); __resetAttachRegistry(); vi.useRealTimers(); });
function deferredPoll(next: Session[] = []) {
 let release!: (sessions: Session[]) => void;
 const listSessions = vi.fn().mockImplementationOnce(() => new Promise<Session[]>(resolve => { release=resolve; })).mockResolvedValue(next);
 stop=startPolling({ listSessions } as unknown as CockpitClient,useCockpit);
 return { release, listSessions };
}

describe('session polls preserve changes newer than the request', () => {
 it('does not treat a late old-generation poll as a replacement after a completed same-ID restart',async()=>{
  const old=fixture(); useCockpit.getState().upsertSession(old);
  const openAttach=vi.fn((s:Session)=>new FakeWebSocket(`ws://localhost${s.attachPath}`) as unknown as WebSocket);
  configureAttachRegistry({client:{openAttach,getSession:async()=>old}});
  acquireAttach(old,{createTerm:()=>new FakeTerm(),readOnly:false});const stale=FakeWebSocket.last()!;stale.acceptOpen();
  stale.emitBinary(encode(Kind.Exit,new TextEncoder().encode('exit status 0')));stale.emitClose(1006);
  const fresh=fixture({executionGeneration:'b'.repeat(64)});
  const poll=deferredPoll([fresh]);
  useCockpit.getState().upsertSession(fresh);updateAttachSession(fresh);FakeWebSocket.last()!.acceptOpen();
  poll.release([old]);await vi.advanceTimersByTimeAsync(0);
  expect(useCockpit.getState().sessions[old.id]).toBe(fresh);
  updateAttachSession(useCockpit.getState().sessions[old.id]);
  stale.emitBinary(encode(Kind.Exit,new TextEncoder().encode('exit status 0')));
  expect(openAttach).toHaveBeenCalledTimes(2);expect(getAttachState(old.id).phase).toBe('attached');
  await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);expect(poll.listSessions).toHaveBeenCalledTimes(2);
  expect(useCockpit.getState().sessions[old.id]).toBe(fresh);
 });
 it('preserves added and removed rows without resurrecting a deleted session',async()=>{
  const removed=fixture();useCockpit.getState().upsertSession(removed);
  const poll=deferredPoll();const added=fixture({id:'new-session'});
  useCockpit.getState().removeSession(removed.id);useCockpit.getState().upsertSession(added);
  poll.release([removed]);await vi.advanceTimersByTimeAsync(0);
  expect(useCockpit.getState().sessions[removed.id]).toBeUndefined();expect(useCockpit.getState().sessions[added.id]).toBe(added);
  await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);
  expect(Object.keys(useCockpit.getState().sessions)).toEqual([]);
 });
 it('the next fresh serial response can revoke read visibility after a protected changed row',async()=>{
  const old=fixture();useCockpit.getState().upsertSession(old);const poll=deferredPoll([]);
  const changed={...old,executionGeneration:'b'.repeat(64)};useCockpit.getState().upsertSession(changed);
  poll.release([old]);await vi.advanceTimersByTimeAsync(0);expect(useCockpit.getState().sessions[old.id]).toBe(changed);
  await vi.advanceTimersByTimeAsync(POLL_INTERVAL_MS);expect(useCockpit.getState().sessions[old.id]).toBeUndefined();
 });
 it('scope reset and cleared private rows win over an old request',async()=>{
  useCockpit.getState().bindProjectNavigation('owner:workspace-a');const old=fixture();useCockpit.getState().upsertSession(old);
  const poll=deferredPoll();useCockpit.getState().bindProjectNavigation('member:workspace-b');
  poll.release([old]);await vi.advanceTimersByTimeAsync(0);expect(useCockpit.getState().sessions[old.id]).toBeUndefined();
 });
 it('a fresh read denial does not restore a record already removed by authority change',async()=>{
  const old=fixture();useCockpit.getState().upsertSession(old);
  let reject!: (error:ApiError)=>void;
  stop=startPolling({listSessions:()=>new Promise<Session[]>((_,r)=>{reject=r;})} as unknown as CockpitClient,useCockpit);
  useCockpit.getState().removeSession(old.id);reject(new ApiError('forbidden' as ProblemCode,'Read access was removed',403));await vi.advanceTimersByTimeAsync(0);
  expect(useCockpit.getState().sessions[old.id]).toBeUndefined();expect(useCockpit.getState().lastError?.code).toBe('forbidden');
 });
 it('unchanged request-baseline records accept a fresh external restart normally',async()=>{
  const old=fixture();useCockpit.getState().upsertSession(old);const poll=deferredPoll();const fresh={...old,executionGeneration:'b'.repeat(64)};
  poll.release([fresh]);await vi.advanceTimersByTimeAsync(0);expect(useCockpit.getState().sessions[old.id]).toBe(fresh);
 });
});

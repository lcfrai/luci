import { beforeEach, expect, it } from 'vitest';
import { archiveRejectedMessageRecovery, clearMessageRecovery, messageRecoveryKey, readMessageRecovery, readRejectedMessageArchive, retainMessageRecovery, type PendingSwarmMessage } from './swarmMessageRecovery';
beforeEach(() => localStorage.clear());
const pending = (): PendingSwarmMessage => ({type: 'post', operationId: 'op-a', request: {expectedRevision: 1, idempotencyKey: 'stable-request-key-123456', recipientTaskId: 'b', kind: 'question', causalMessageId: '', evidence: {kind: 'none', id: '', digest: ''}, summary: 'Question'}});
it('LOCAL151 preserves exact legacy omission and empty-field bytes without normalization', () => {
  const value = pending(); if (value.type !== 'post') throw new Error('post fixture');
  for (const [scope, retained] of [['omitted', value], ['empty', {...value, request: {...value.request, resolvesQuestionId: ''}}]] as const) {
    retainMessageRecovery(scope, retained);
    const raw = localStorage.getItem(messageRecoveryKey(scope));
    expect(JSON.stringify(readMessageRecovery(scope))).toBe(raw);
    expect(localStorage.getItem(messageRecoveryKey(scope))).toBe(raw);
    expect(Object.prototype.hasOwnProperty.call(readMessageRecovery(scope)!.request, 'resolvesQuestionId')).toBe(scope === 'empty');
  }
});
it('LOCAL151 retains explicit resolution through remount/readback and archives the exact rejected request', () => {
  const value = pending(); if (value.type !== 'post') throw new Error('post fixture');
  const retained: PendingSwarmMessage = {...value, request: {...value.request, kind: 'decision', causalMessageId: 'msg-q', resolvesQuestionId: 'msg-q'}};
  retainMessageRecovery('scope', retained);
  const raw = localStorage.getItem(messageRecoveryKey('scope'));
  expect(readMessageRecovery('scope')).toEqual(retained);
  expect(localStorage.getItem(messageRecoveryKey('scope'))).toBe(raw);
  archiveRejectedMessageRecovery('scope', retained, 'rejected', 2);
  expect(readMessageRecovery('scope')).toBeNull();
  expect(readRejectedMessageArchive('scope')).toEqual([{request: retained, state: 'rejected', runRevision: 2}]);
});
it.each([null, false, {}, 'msg-other', 'x'.repeat(257)])('LOCAL151 preserves invalid pending resolution bytes and refuses recovery %#', resolvesQuestionId => {
  const value = pending();
  const raw = JSON.stringify({...value, request: {...value.request, kind: 'decision', causalMessageId: 'msg-q', resolvesQuestionId}});
  localStorage.setItem(messageRecoveryKey('scope'), raw);
  expect(() => readMessageRecovery('scope')).toThrow();
  expect(localStorage.getItem(messageRecoveryKey('scope'))).toBe(raw);
});
it('LOCAL151 refuses a retained non-decision resolution without deleting the original', () => {
  const value = pending(), raw = JSON.stringify({...value, request: {...value.request, causalMessageId: 'msg-q', resolvesQuestionId: 'msg-q'}});
  localStorage.setItem(messageRecoveryKey('scope'), raw);
  expect(() => readMessageRecovery('scope')).toThrow();
  expect(localStorage.getItem(messageRecoveryKey('scope'))).toBe(raw);
});
it('retains exact payload/key across remounts and separates actor/workspace/run scopes', () => {
  const value = pending(); retainMessageRecovery('actor/team/run1', value);
  if (value.type !== 'post') throw new Error('expected a message post fixture');
  expect(readMessageRecovery('actor/team/run1')).toEqual(value); expect(readMessageRecovery('other/team/run1')).toBeNull();
  retainMessageRecovery('actor/team/run1', value);
  expect(() => retainMessageRecovery('actor/team/run1', {...value, operationId: 'different'})).toThrow();
});
it('never deletes another tabs request and rejects injected authority fields', () => {
  const value = pending(); retainMessageRecovery('scope', value);
  localStorage.setItem(messageRecoveryKey('scope'), JSON.stringify({...value, operationId: 'another'}));
  expect(() => clearMessageRecovery('scope', value)).toThrow();
  localStorage.setItem(messageRecoveryKey('scope'), JSON.stringify({...value, request: {...value.request, command: 'execute'}}));
  expect(() => readMessageRecovery('scope')).toThrow();
});

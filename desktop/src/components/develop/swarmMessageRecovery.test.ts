import { beforeEach, expect, it } from 'vitest';
import { clearMessageRecovery, messageRecoveryKey, readMessageRecovery, retainMessageRecovery, type PendingSwarmMessage } from './swarmMessageRecovery';
beforeEach(() => localStorage.clear());
const pending = (): PendingSwarmMessage => ({type: 'post', operationId: 'op-a', request: {expectedRevision: 1, idempotencyKey: 'stable-request-key-123456', recipientTaskId: 'b', kind: 'question', causalMessageId: '', evidence: {kind: 'none', id: '', digest: ''}, summary: 'Question'}});
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

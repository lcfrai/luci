import { SWARM_MESSAGE_KINDS, type AckSwarmMessage, type PostSwarmMessage } from '../../api/swarmSupervision';

export type PendingSwarmMessage = {type: 'post'; operationId: string; request: PostSwarmMessage} |
  {type: 'ack'; messageId: string; request: AckSwarmMessage};
const prefix = 'luci.swarm-message-pending.v1:';
export const messageRecoveryKey = (scope: string) => prefix + encodeURIComponent(scope);
export function readMessageRecovery(scope: string): PendingSwarmMessage | null {
  const raw = localStorage.getItem(messageRecoveryKey(scope));
  if (!raw) return null;
  if (raw.length > 16000) throw new Error('The retained message request exceeds its recovery bound.');
  const value = JSON.parse(raw) as PendingSwarmMessage, request = value?.request;
  const identity = (value: unknown) => typeof value === 'string' && value.length > 0 && value.length <= 256;
  if (!request || !Number.isSafeInteger(request.expectedRevision) || request.expectedRevision < 1 ||
      value.type === 'ack' && (!identity(value.messageId) || !identity(value.request.recipientOperationId)) ||
      value.type === 'post' && (!identity(value.operationId) ||
        typeof value.request.idempotencyKey !== 'string' || !/^[!-~]{16,200}$/.test(value.request.idempotencyKey) ||
        !SWARM_MESSAGE_KINDS.includes(value.request.kind) || !identity(value.request.recipientTaskId) ||
        typeof value.request.summary !== 'string' || !value.request.summary.trim() || value.request.summary.length > 4000 ||
        typeof value.request.causalMessageId !== 'string' || value.request.causalMessageId.length > 256 || value.request.evidence?.kind !== 'none' || value.request.evidence.id !== '' || value.request.evidence.digest !== '') ||
      !['post', 'ack'].includes(value.type)) throw new Error('The retained message request could not be verified. No write was replayed.');
  const allowed = value.type === 'post' ? ['expectedRevision', 'idempotencyKey', 'recipientTaskId', 'kind', 'causalMessageId', 'evidence', 'summary'] : ['expectedRevision', 'recipientOperationId'];
  if (Object.keys(value).some(key => !(value.type === 'post' ? ['type', 'operationId', 'request'] : ['type', 'messageId', 'request']).includes(key)) ||
      Object.keys(request).some(key => !allowed.includes(key)) || value.type === 'post' && Object.keys(value.request.evidence).some(key => !['kind', 'id', 'digest'].includes(key))) {
    throw new Error('The retained message request has unsupported fields. No write was replayed.');
  }
  return value;
}
export function retainMessageRecovery(scope: string, value: PendingSwarmMessage) {
  const raw = JSON.stringify(value), key = messageRecoveryKey(scope), previous = localStorage.getItem(key);
  if (raw.length > 16000 || previous && previous !== raw) throw new Error('Another message request is retained. Inspect its outcome before sending another.');
  localStorage.setItem(key, raw);
  if (localStorage.getItem(key) !== raw) throw new Error('The exact message request could not be retained. No write was sent.');
}
export function clearMessageRecovery(scope: string, value: PendingSwarmMessage) {
  const key = messageRecoveryKey(scope);
  if (localStorage.getItem(key) !== JSON.stringify(value)) throw new Error('Another tab changed message recovery. Refresh before continuing.');
  localStorage.removeItem(key);
}

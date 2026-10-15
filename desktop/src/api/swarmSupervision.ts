import { productApi } from './product';

/** Additive interface pinned at d2ba56f7. Unknown phases/fields grant no authority. */
export const SWARM_MESSAGE_KINDS = ['discovery', 'question', 'dependency-result', 'review-finding', 'decision', 'lifecycle-request'] as const;
export type SwarmMessageKind = typeof SWARM_MESSAGE_KINDS[number];
export interface SwarmMessage {
  id: string; sequence: number; kind: string;
  sender: {taskId: string; operationId: string; childId: string; worktreeId: string; contextDigest: string};
  recipientTaskId: string; causalMessageId: string;
  evidence: {kind: string; id: string; digest: string}; summary: string; createdAt: string;
  ack: {operationId: string; childId: string; actorId: string; at: string} | null;
  [futureField: string]: unknown;
}
export interface SwarmSupervision {
  version: 1; projectId?: string; observedAt?: string; runId: string; runRevision: number; queueRevision: number;
  context: {briefRevision: number; planRevision: number; contractDigest: string; freshness: string};
  activityPhase: string; qualityPhase: string; settlement: string;
  tasks: {taskId: string; operationId: string; state: string; integrationState: string}[];
  messages: SwarmMessage[];
  page: {after: number; nextCursor: number; hasMore: boolean; omitted: number};
  attention: {id: string; kind: string; taskId: string; action: string}[];
  metrics: {routineDecisions: unknown; latency: unknown; quality: unknown; throughput: unknown; cost: unknown};
  [futureField: string]: unknown;
}
export interface PostSwarmMessage {
  expectedRevision: number; idempotencyKey: string; recipientTaskId: string; kind: SwarmMessageKind;
  causalMessageId: string; evidence: {kind: 'none'; id: ''; digest: ''}; summary: string;
}
export interface AckSwarmMessage { expectedRevision: number; recipientOperationId: string }
export interface SwarmMessageResult {message: SwarmMessage; runRevision: number}
const text = (value: unknown, max = 256): value is string => typeof value === 'string' && value.length <= max;
const id = (value: unknown): value is string => text(value) && value.length > 0;
const integer = (value: unknown): value is number => Number.isSafeInteger(value) && (value as number) >= 0;
const digest = (value: unknown) => value === '' || typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
export function checkedSwarmMessage(value: SwarmMessage): SwarmMessage {
  if (!value || !id(value.id) || !integer(value.sequence) || value.sequence < 1 || !id(value.kind) || !id(value.sender?.taskId) ||
      !id(value.sender?.operationId) || !id(value.sender?.childId) || !id(value.sender?.worktreeId) || !digest(value.sender?.contextDigest) || !value.sender.contextDigest ||
      !id(value.recipientTaskId) || !text(value.causalMessageId) || !id(value.evidence?.kind) || !text(value.evidence?.id) || !digest(value.evidence?.digest) ||
      !text(value.summary, 4000) || !id(value.createdAt) || value.ack !== null && (!value.ack || !id(value.ack.operationId) || !id(value.ack.childId) || !id(value.ack.actorId) || !id(value.ack.at))) {
    throw new Error('The attributed message receipt could not be verified. Its result remains unconfirmed.');
  }
  return value;
}
export function checkedSupervision(projectId: string, runId: string, after: number, value: SwarmSupervision): SwarmSupervision {
  if (!value || value.version !== 1 || value.projectId !== undefined && value.projectId !== projectId || value.runId !== runId ||
      value.observedAt !== undefined && !id(value.observedAt) || !integer(value.runRevision) || !integer(value.queueRevision) ||
      !integer(value.context?.briefRevision) || !integer(value.context?.planRevision) || !digest(value.context?.contractDigest) || !id(value.context?.freshness) ||
      !id(value.activityPhase) || !id(value.qualityPhase) || !id(value.settlement) ||
      !Array.isArray(value.tasks) || value.tasks.length > 10000 || value.tasks.some(task => !id(task.taskId) || !text(task.operationId) || !id(task.state) || !id(task.integrationState)) ||
      new Set(value.tasks.map(task => task.taskId)).size !== value.tasks.length ||
      !Array.isArray(value.messages) || value.messages.length > 50 || !value.page || value.page.after !== after || !integer(value.page.nextCursor) ||
      value.page.nextCursor < after || typeof value.page.hasMore !== 'boolean' || value.page.hasMore && value.page.nextCursor <= after || !integer(value.page.omitted) ||
      !Array.isArray(value.attention) || value.attention.length > 1000 || value.attention.some(item => !id(item.id) || !id(item.kind) || !text(item.taskId) || !text(item.action, 4000)) || !value.metrics) {
    throw new Error('The supervision page could not be verified for this run. Refresh its saved history.');
  }
  value.messages.forEach(checkedSwarmMessage);
  if (value.messages.some(message => message.sequence <= after || message.sequence > value.page.nextCursor) ||
      new Set(value.messages.map(message => message.id)).size !== value.messages.length || new Set(value.messages.map(message => message.sequence)).size !== value.messages.length) {
    throw new Error('The supervision cursor or message identities are inconsistent. Refresh its saved history.');
  }
  return value;
}
const path = (projectId: string, runId: string) => `/v1/product/projects/${encodeURIComponent(projectId)}/runs/${encodeURIComponent(runId)}/swarm`;
export const swarmSupervisionApi = {
  async get(projectId: string, runId: string, after = 0, signal?: AbortSignal) {
    if (!integer(after)) throw new Error('Invalid supervision cursor.');
    return checkedSupervision(projectId, runId, after, await productApi.request<SwarmSupervision>(`${path(projectId, runId)}/supervision?after=${after}&limit=50`, undefined, 'GET', 30_000, signal));
  },
  async post(projectId: string, runId: string, operationId: string, request: PostSwarmMessage, signal?: AbortSignal) {
    const result = await productApi.request<SwarmMessageResult>(`${path(projectId, runId)}/operations/${encodeURIComponent(operationId)}/messages`, request, 'POST', 30_000, signal);
    const message = checkedSwarmMessage(result?.message);
    if (!integer(result.runRevision) || result.runRevision < request.expectedRevision || message.sender.operationId !== operationId ||
        message.recipientTaskId !== request.recipientTaskId || message.kind !== request.kind || message.causalMessageId !== request.causalMessageId ||
        message.summary !== request.summary || message.evidence.kind !== request.evidence.kind || message.evidence.id !== request.evidence.id || message.evidence.digest !== request.evidence.digest) {
      throw new Error('The message response does not match the retained request. Its result remains unconfirmed.');
    }
    return result;
  },
  async ack(projectId: string, runId: string, messageId: string, request: AckSwarmMessage, signal?: AbortSignal) {
    const result = await productApi.request<SwarmMessageResult>(`${path(projectId, runId)}/messages/${encodeURIComponent(messageId)}/ack`, request, 'POST', 30_000, signal);
    const message = checkedSwarmMessage(result?.message);
    if (!integer(result.runRevision) || result.runRevision < request.expectedRevision || message.id !== messageId || message.ack?.operationId !== request.recipientOperationId) {
      throw new Error('The delivery response does not match the retained request. Its result remains unconfirmed.');
    }
    return result;
  },
};

/** Bounded accumulated window. Refresh starts a new window; no hidden page draining. */
export function mergeSwarmMessages(current: SwarmMessage[], incoming: SwarmMessage[]): SwarmMessage[] {
  const identity = (message: SwarmMessage) => JSON.stringify([message.id, message.sequence, message.kind, message.sender.taskId, message.sender.operationId,
    message.sender.childId, message.sender.worktreeId, message.sender.contextDigest, message.recipientTaskId, message.causalMessageId,
    message.evidence.kind, message.evidence.id, message.evidence.digest, message.summary, message.createdAt]);
  const byId = new Map(current.map(message => [message.id, message]));
  for (const message of incoming) {
    const previous = byId.get(message.id);
    if (previous && (identity(previous) !== identity(message) || previous.ack && (!message.ack ||
        previous.ack.operationId !== message.ack.operationId || previous.ack.childId !== message.ack.childId || previous.ack.actorId !== message.ack.actorId || previous.ack.at !== message.ack.at))) {
      throw new Error('A retained message changed identity. Refresh its saved history.');
    }
    byId.set(message.id, message);
  }
  const values = [...byId.values()].sort((a, b) => a.sequence - b.sequence);
  if (new Set(values.map(message => message.sequence)).size !== values.length || values.length > 250) throw new Error('The message window is full or inconsistent. Refresh to start a new bounded window.');
  return values;
}

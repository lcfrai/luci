import { productApi } from './product';

/** Additive interface pinned at d2ba56f7. Unknown phases/fields grant no authority. */
export const SWARM_MESSAGE_KINDS = ['discovery', 'question', 'dependency-result', 'review-finding', 'decision', 'lifecycle-request'] as const;
export type SwarmMessageKind = typeof SWARM_MESSAGE_KINDS[number];
export interface SwarmMessage {
  id: string; sequence: number; kind: string;
  sender: {taskId: string; operationId: string; childId: string; worktreeId: string; contextDigest: string};
  recipientTaskId: string; causalMessageId?: string;
  evidence: {kind: string; id: string; digest: string}; summary: string; attribution: 'coordinator-recorded' | 'owned-worker-report'; createdAt: string;
  ack: {operationId: string; childId: string; actorId: string; at: string} | null;
  [futureField: string]: unknown;
}
export interface SwarmReviewObligation {
  id: string; taskId: string; candidateOperationId: string; candidateChildId: string; candidateCommit: string;
  contextDigest: string; reviewDigest: string; reviewerOperationId: string; reviewerChildId: string;
  reviewerHandoffDigest?: string; findingMessageId?: string; repairOperationId?: string;
  state: 'pending' | 'passed' | 'repair-required' | 'unknown'; attempt: number; createdAt: string; resolvedAt?: string;
  keyDigest: string; requestDigest: string; repairLimit: 2; repairsConsumed: number;
}
export type SwarmAttentionAction = {kind: 'open-message' | 'open-review-obligation' | 'inspect-recovery'; targetId: string};
export interface SwarmSupervision {
  version: 1; projectId: string; observedAt: string; runId: string; runRevision: number; queueRevision: number;
  context: {briefRevision: number; planRevision: number; contractDigest: string; freshness: string};
  activityPhase: string; qualityPhase: string; settlement: string;
  tasks: {taskId: string; operationId: string; role: 'coding' | 'review-only'; state: string; integrationState: string}[];
  messages: SwarmMessage[];
  reviewObligations: SwarmReviewObligation[];
  page: {after: number; nextCursor: number; hasMore: boolean; omitted: number};
  attention: {id: string; kind: string; taskId: string; action: SwarmAttentionAction}[];
  metrics: {routineDecisions: unknown; latency: unknown; quality: unknown; throughput: unknown; cost: unknown};
  [futureField: string]: unknown;
}
export interface PostSwarmMessage {
  expectedRevision: number; idempotencyKey: string; recipientTaskId: string; kind: SwarmMessageKind;
  causalMessageId: string; evidence: {kind: 'none'; id: ''; digest: ''} | {kind: 'handoff' | 'source-commit'; id: string; digest: string}; summary: string;
}
export interface AckSwarmMessage { expectedRevision: number; recipientOperationId: string }
export interface SwarmMessageResult {message: SwarmMessage; runRevision: number}
export interface SwarmMessageReadback extends SwarmMessageResult {projectId: string; runId: string}
export interface SwarmMessageDisposition {projectId: string; runId: string; operationId: string; state: 'recorded' | 'rejected' | 'unconfirmed' | 'key-conflict'; runRevision: number; message: SwarmMessage | null}
export interface SwarmAckRetirement {projectId: string; runId: string; messageId: string; state: 'rejected'; runRevision: number}
const text = (value: unknown, max = 256): value is string => typeof value === 'string' && new TextEncoder().encode(value).length <= max;
const id = (value: unknown): value is string => text(value) && value.length > 0;
const integer = (value: unknown): value is number => Number.isSafeInteger(value) && (value as number) >= 0;
const digest = (value: unknown) => value === '' || typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
export function checkedSwarmMessage(value: SwarmMessage): SwarmMessage {
  if (!value || !id(value.id) || !integer(value.sequence) || value.sequence < 1 || !id(value.kind) || !id(value.sender?.taskId) ||
      !id(value.sender?.operationId) || !id(value.sender?.childId) || !id(value.sender?.worktreeId) || !digest(value.sender?.contextDigest) || !value.sender.contextDigest ||
      !id(value.recipientTaskId) || !text(value.causalMessageId ?? '') || !id(value.evidence?.kind) || !text(value.evidence?.id) || !digest(value.evidence?.digest) ||
      !text(value.summary, 1000) || !value.summary.trim() || !['coordinator-recorded', 'owned-worker-report'].includes(value.attribution) || !id(value.createdAt) || value.ack !== null && (!value.ack || !id(value.ack.operationId) || !id(value.ack.childId) || !id(value.ack.actorId) || !id(value.ack.at))) {
    throw new Error('The attributed message receipt could not be verified. Its result remains unconfirmed.');
  }
  return value;
}
export function checkedReviewObligation(value: SwarmReviewObligation): SwarmReviewObligation {
  if (!value || !id(value.id) || !id(value.taskId) || !id(value.candidateOperationId) || !id(value.candidateChildId) ||
      !/^[a-f0-9]{40}$/.test(value.candidateCommit) || !digest(value.contextDigest) || !value.contextDigest ||
      !digest(value.reviewDigest) || !value.reviewDigest || !id(value.reviewerOperationId) || !id(value.reviewerChildId) ||
      !digest(value.keyDigest) || !value.keyDigest || !digest(value.requestDigest) || !value.requestDigest ||
      !['pending', 'passed', 'repair-required', 'unknown'].includes(value.state) || !integer(value.attempt) || value.attempt < 1 ||
      !id(value.createdAt) || value.repairLimit !== 2 || !integer(value.repairsConsumed) || value.repairsConsumed > 2 ||
      value.reviewerHandoffDigest !== undefined && (!digest(value.reviewerHandoffDigest) || !value.reviewerHandoffDigest) ||
      value.findingMessageId !== undefined && !id(value.findingMessageId) || value.repairOperationId !== undefined && !id(value.repairOperationId) ||
      value.resolvedAt !== undefined && !id(value.resolvedAt)) {
    throw new Error('The independent review receipt could not be verified. Its outcome remains unconfirmed.');
  }
  return value;
}
export function checkedSupervision(projectId: string, runId: string, after: number, value: SwarmSupervision): SwarmSupervision {
  if (!value || value.version !== 1 || value.projectId !== projectId || value.runId !== runId ||
      !id(value.observedAt) || !integer(value.runRevision) || !integer(value.queueRevision) ||
      !integer(value.context?.briefRevision) || !integer(value.context?.planRevision) || !digest(value.context?.contractDigest) || !id(value.context?.freshness) ||
      !id(value.activityPhase) || !id(value.qualityPhase) || !id(value.settlement) ||
      !Array.isArray(value.tasks) || value.tasks.length > 10000 || value.tasks.some(task => !id(task.taskId) || !text(task.operationId) || !['coding', 'review-only'].includes(task.role) || !id(task.state) || !id(task.integrationState)) ||
      new Set(value.tasks.map(task => task.taskId)).size !== value.tasks.length ||
      !Array.isArray(value.messages) || value.messages.length > 50 || !Array.isArray(value.reviewObligations) || value.reviewObligations.length > 200 ||
      !value.page || value.page.after !== after || !integer(value.page.nextCursor) || value.page.nextCursor > 512 ||
      value.page.nextCursor < after || typeof value.page.hasMore !== 'boolean' || value.page.hasMore && value.page.nextCursor <= after || !integer(value.page.omitted) || value.page.omitted > 512 ||
      !Array.isArray(value.attention) || value.attention.length > 1000 || value.attention.some(item => !id(item.id) || !['question', 'review', 'repair', 'unknown'].includes(item.kind) || !text(item.taskId) ||
        !['open-message', 'open-review-obligation', 'inspect-recovery'].includes(item.action?.kind) || item.action.targetId !== item.id) || !value.metrics) {
    throw new Error('The supervision page could not be verified for this run. Refresh its saved history.');
  }
  value.messages.forEach(checkedSwarmMessage);
  value.reviewObligations.forEach(checkedReviewObligation);
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
        message.recipientTaskId !== request.recipientTaskId || message.kind !== request.kind || (message.causalMessageId ?? '') !== request.causalMessageId ||
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
  async message(projectId: string, runId: string, messageId: string, signal?: AbortSignal) {
    const value = await productApi.request<SwarmMessageReadback>(`${path(projectId, runId)}/messages/${encodeURIComponent(messageId)}`, undefined, 'GET', 30_000, signal);
    if (value?.projectId !== projectId || value.runId !== runId || value.message?.id !== messageId || !integer(value.runRevision)) throw new Error('The saved message readback does not match this project and run.');
    checkedSwarmMessage(value.message);
    return value;
  },
  async disposition(projectId: string, runId: string, operationId: string, request: PostSwarmMessage, signal?: AbortSignal) {
    const value = await productApi.request<SwarmMessageDisposition>(`${path(projectId, runId)}/operations/${encodeURIComponent(operationId)}/messages/disposition`, request, 'POST', 30_000, signal);
    if (value?.projectId !== projectId || value.runId !== runId || value.operationId !== operationId || !integer(value.runRevision) || value.runRevision < request.expectedRevision ||
        !['recorded', 'rejected', 'unconfirmed', 'key-conflict'].includes(value.state) || value.state === 'recorded' !== !!value.message) throw new Error('The exact message disposition could not be verified. Keep the retained request.');
    if (value.message) {
      checkedSwarmMessage(value.message);
      if (value.message.sender.operationId !== operationId || value.message.recipientTaskId !== request.recipientTaskId || value.message.kind !== request.kind ||
          (value.message.causalMessageId ?? '') !== request.causalMessageId || value.message.summary !== request.summary || value.message.evidence.kind !== request.evidence.kind ||
          value.message.evidence.id !== request.evidence.id || value.message.evidence.digest !== request.evidence.digest) throw new Error('The recorded message differs from the retained request. Keep the exact request for inspection.');
    }
    return value;
  },
  async retirePost(projectId: string, runId: string, operationId: string, request: PostSwarmMessage, signal?: AbortSignal) {
    const value = await productApi.request<SwarmMessageDisposition>(`${path(projectId, runId)}/operations/${encodeURIComponent(operationId)}/messages/retire`, request, 'POST', 30_000, signal);
    if (value?.projectId !== projectId || value.runId !== runId || value.operationId !== operationId || value.state !== 'rejected' || value.message !== null || !integer(value.runRevision) || value.runRevision <= request.expectedRevision) throw new Error('The exact message retirement was not confirmed. Keep its retained request.');
    return value;
  },
  async retireAck(projectId: string, runId: string, messageId: string, request: AckSwarmMessage, signal?: AbortSignal) {
    const value = await productApi.request<SwarmAckRetirement>(`${path(projectId, runId)}/messages/${encodeURIComponent(messageId)}/ack/retire`, request, 'POST', 30_000, signal);
    if (value?.projectId !== projectId || value.runId !== runId || value.messageId !== messageId || value.state !== 'rejected' || !integer(value.runRevision) || value.runRevision <= request.expectedRevision) throw new Error('The exact delivery retirement was not confirmed. Keep its retained request.');
    return value;
  },
};

/** Bounded accumulated window. Refresh starts a new window; no hidden page draining. */
export function mergeSwarmMessages(current: SwarmMessage[], incoming: SwarmMessage[]): SwarmMessage[] {
  const identity = (message: SwarmMessage) => JSON.stringify([message.id, message.sequence, message.kind, message.sender.taskId, message.sender.operationId,
    message.sender.childId, message.sender.worktreeId, message.sender.contextDigest, message.recipientTaskId, message.causalMessageId,
    message.evidence.kind, message.evidence.id, message.evidence.digest, message.summary, message.attribution, message.createdAt]);
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

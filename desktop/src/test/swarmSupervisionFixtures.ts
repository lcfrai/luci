import type { SwarmMessage, SwarmSupervision } from '../api/swarmSupervision';
export const swarmMessageFixture = (patch: Partial<SwarmMessage> = {}): SwarmMessage => ({id: 'msg-1', sequence: 1, kind: 'question',
  sender: {taskId: 'a', operationId: 'op-a', childId: 'session-a', worktreeId: 'isolated-a', contextDigest: 'a'.repeat(64)},
  recipientTaskId: 'b', causalMessageId: '', evidence: {kind: 'none', id: '', digest: ''}, summary: 'Should the empty state include archived items?', createdAt: '2026-10-16T00:00:00Z', ack: null, ...patch});
export const swarmSupervisionFixture = (patch: Partial<SwarmSupervision> = {}): SwarmSupervision => ({version: 1, runId: 'run-1', runRevision: 1, queueRevision: 2,
  context: {briefRevision: 1, planRevision: 1, contractDigest: '', freshness: 'current'}, activityPhase: 'working', qualityPhase: 'needs-review', settlement: 'uncertain',
  tasks: [{taskId: 'a', operationId: 'op-a', state: 'working', integrationState: 'none'}], messages: [swarmMessageFixture()],
  page: {after: 0, nextCursor: 1, hasMore: false, omitted: 0}, attention: [{id: 'msg-1', kind: 'question', taskId: 'b', action: 'https://arbitrary.invalid/execute'}],
  metrics: {routineDecisions: null, latency: null, quality: null, throughput: null, cost: null}, ...patch});

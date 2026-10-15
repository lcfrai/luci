import { beforeEach, expect, it } from 'vitest';
import { archiveSwarmReviewRecovery, clearSwarmReviewRecovery, readSwarmReviewRecovery, retainSwarmReviewRecovery, type PendingSwarmReview } from './swarmReviewRecovery';
beforeEach(() => localStorage.clear());
const scope = 'actor/workspace/project/run';
const request: PendingSwarmReview = {type: 'create', taskId: 'task-a', request: {expectedRevision: 4, idempotencyKey: 'stable-review-request-123456', candidateOperationId: 'op-candidate', reviewerOperationId: 'op-reviewer', reviewDigest: 'a'.repeat(64)}};
it('retains exact review requests without replay and archives only a definitive advanced CAS refusal', () => {
  retainSwarmReviewRecovery(scope, request);
  expect(readSwarmReviewRecovery(scope)).toEqual(request);
  expect(() => archiveSwarmReviewRecovery(scope, request, 'rejected', 4)).toThrow();
  expect(readSwarmReviewRecovery(scope)).toEqual(request);
  archiveSwarmReviewRecovery(scope, request, 'rejected', 5);
  expect(readSwarmReviewRecovery(scope)).toBeNull();
  expect(Array.from({length: localStorage.length}, (_, index) => localStorage.key(index)).some(key => key?.startsWith('luci.swarm-review-rejected.v1:'))).toBe(true);
});
it('refuses mismatched tabs, unsupported fields and oversized retained requests', () => {
  retainSwarmReviewRecovery(scope, request);
  expect(() => retainSwarmReviewRecovery(scope, {...request, taskId: 'task-b'})).toThrow();
  expect(() => clearSwarmReviewRecovery(scope, {...request, taskId: 'task-b'})).toThrow();
  clearSwarmReviewRecovery(scope, request);
  expect(readSwarmReviewRecovery(scope)).toBeNull();
  const bad = {...request, request: {...request.request, secret: 'must-not-persist'}};
  expect(() => retainSwarmReviewRecovery(scope, bad)).toThrow();
});

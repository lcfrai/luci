import type { CreateReviewObligation, ResolveReviewObligation } from '../../api/swarmReviewObligations';

export type PendingSwarmReview =
  | {type: 'create'; taskId: string; request: CreateReviewObligation}
  | {type: 'resolve'; obligationId: string; request: ResolveReviewObligation};
const key = (scope: string) => `luci.swarm-review-pending.v1:${encodeURIComponent(scope)}`;
const archiveKey = (scope: string) => `luci.swarm-review-rejected.v1:${encodeURIComponent(scope)}`;
const digest = (value: unknown) => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
const id = (value: unknown) => typeof value === 'string' && /^[a-z0-9-]{2,256}$/.test(value);
const bytes = (value: string) => new TextEncoder().encode(value).length;
function valid(value: PendingSwarmReview): boolean {
  if (!value || !Number.isSafeInteger(value.request?.expectedRevision) || value.request.expectedRevision < 1) return false;
  if (value.type === 'create') return id(value.taskId) && id(value.request.idempotencyKey) && id(value.request.candidateOperationId) &&
    id(value.request.reviewerOperationId) && digest(value.request.reviewDigest) &&
    Object.keys(value).sort().join() === 'request,taskId,type' &&
    Object.keys(value.request).sort().join() === 'candidateOperationId,expectedRevision,idempotencyKey,reviewDigest,reviewerOperationId';
  return value.type === 'resolve' && id(value.obligationId) && ['passed', 'repair-required'].includes(value.request.outcome) &&
    digest(value.request.reviewerHandoffDigest) && id(value.request.findingMessageId) &&
    Object.keys(value).sort().join() === 'obligationId,request,type' &&
    Object.keys(value.request).sort().join() === 'expectedRevision,findingMessageId,outcome,reviewerHandoffDigest';
}
export function readSwarmReviewRecovery(scope: string): PendingSwarmReview | null {
  const raw = localStorage.getItem(key(scope));
  if (!raw) return null;
  if (bytes(raw) > 4096) throw new Error('The retained review request exceeds its recovery bound. No request was replayed.');
  const value = JSON.parse(raw) as PendingSwarmReview;
  if (!valid(value)) throw new Error('The retained review request is invalid. No request was replayed.');
  return value;
}
export function retainSwarmReviewRecovery(scope: string, value: PendingSwarmReview) {
  if (!valid(value)) throw new Error('The exact review request is invalid. No write was sent.');
  const raw = JSON.stringify(value), prior = localStorage.getItem(key(scope));
  if (bytes(raw) > 4096 || prior && prior !== raw) throw new Error('Another review request is retained. Inspect its exact outcome before another write.');
  localStorage.setItem(key(scope), raw);
  if (localStorage.getItem(key(scope)) !== raw) throw new Error('The exact review request was not retained. No write was sent.');
}
export function clearSwarmReviewRecovery(scope: string, value: PendingSwarmReview) {
  if (localStorage.getItem(key(scope)) !== JSON.stringify(value)) throw new Error('Another tab changed the retained review request. Refresh before continuing.');
  localStorage.removeItem(key(scope));
}
export function archiveSwarmReviewRecovery(scope: string, value: PendingSwarmReview, disposition: 'rejected' | 'key-conflict', runRevision: number) {
  if (localStorage.getItem(key(scope)) !== JSON.stringify(value) || !Number.isSafeInteger(runRevision) ||
    (disposition === 'rejected' ? runRevision <= value.request.expectedRevision : runRevision < value.request.expectedRevision)) throw new Error('The original review request or definitive outcome could not be verified.');
  const prior = localStorage.getItem(archiveKey(scope));
  const entries = prior ? JSON.parse(prior) as unknown : [];
  if (!Array.isArray(entries) || entries.length >= 200) throw new Error('The bounded rejected-review archive is full. Preserve it before continuing.');
  const next = JSON.stringify([...entries, {request: value, disposition, runRevision}]);
  if (bytes(next) > 1 << 20) throw new Error('The bounded rejected-review archive is full. Preserve it before continuing.');
  localStorage.setItem(archiveKey(scope), next);
  if (localStorage.getItem(archiveKey(scope)) !== next || localStorage.getItem(key(scope)) !== JSON.stringify(value)) throw new Error('The rejected review request could not be archived. The pending request was preserved.');
  localStorage.removeItem(key(scope));
}

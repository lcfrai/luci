import { beforeEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { coordinatorContinuationsApi as api, type CoordinatorContinuation, type CoordinatorContinuationRequest } from './coordinatorContinuations';
export const continuation = (patch: Partial<CoordinatorContinuation> = {}): CoordinatorContinuation => ({ id: 'continue-one', projectId: 'p-1', actor: { id: 'alice', displayName: 'Alice' }, idempotencyKey: 'request-one', originTurnId: 'turn-1', contextDigest: 'a'.repeat(64), policyRevision: 2, revision: 4, status: 'queued', message: 'Tasks queued under reviewed policy', runId: 'run-one', queueIds: ['queue-one'], actions: [{ id: 'continue-one-create-run', kind: 'create-run', status: 'confirmed', referenceId: 'run-one', startedAt: '2026-10-12T00:00:00Z' }], createdAt: '2026-10-12T00:00:00Z', updatedAt: '2026-10-12T00:00:01Z', ...patch });
const request = (): CoordinatorContinuationRequest => ({ idempotencyKey: 'request-one', originTurnId: 'turn-1', expectedContextDigest: 'a'.repeat(64), expectedPolicyRevision: 2, maxParallel: 2, run: { worktreeId: 'w-1', harness: 'codex', model: 'm', permissionMode: 'bypass', outcome: 'preview', implementationContext: { projectId: 'p-1', briefRevision: 1, planRevision: 1, specificationRevision: 2, designPlanRevision: 1, taskIds: ['T-1'] } } });
beforeEach(() => { vi.restoreAllMocks(); vi.spyOn(productApi, 'request').mockResolvedValue(continuation()); });
it('uses exact scoped routes and carries cancellation only for reads', async () => {
  const controller = new AbortController();
  vi.mocked(productApi.request).mockResolvedValueOnce({ continuations: [continuation()] });
  expect(await api.list('p-1', 'alice', controller.signal)).toHaveLength(1);
  expect(productApi.request).toHaveBeenCalledWith('/v1/product/projects/p-1/orchestrator/continuations', undefined, 'GET', 30_000, controller.signal);
  expect((await api.get('p-1', 'alice', 'continue-one')).id).toBe('continue-one');
  expect((await api.create('p-1', 'alice', request())).queueIds).toEqual(['queue-one']);
  expect((await api.advance('p-1', 'alice', 'continue-one', 4)).revision).toBe(4);
});
it.each(['project', 'actor', 'id', 'revision', 'policy', 'status', 'action', 'queue', 'date', 'navigation'])('rejects an untrusted %s response', async field => {
  const row = continuation();
  if (field === 'project') row.projectId = 'foreign'; if (field === 'actor') row.actor.id = 'bob'; if (field === 'id') row.id = 'other';
  if (field === 'revision') row.revision = 0; if (field === 'policy') row.policyRevision = 0; if (field === 'status') row.status = 'deployed' as never;
  if (field === 'action') row.actions[0].id = 'foreign'; if (field === 'queue') row.queueIds.push('queue-one'); if (field === 'date') row.createdAt = 'not a date';
  if (field === 'navigation') row.nextAction = { id: 'bad', label: 'Bad', reason: '', lens: 'foreign' as never };
  vi.mocked(productApi.request).mockResolvedValue(row); await expect(api.get('p-1', 'alice', 'continue-one')).rejects.toThrow(/could not be verified/);
});
it.each(['key', 'origin', 'context', 'policy'])('retains uncertainty for mismatching create %s receipt', async field => {
  const row = continuation(); if (field === 'key') row.idempotencyKey = 'other'; if (field === 'origin') row.originTurnId = 'other'; if (field === 'context') row.contextDigest = 'other'; if (field === 'policy') row.policyRevision = 3;
  vi.mocked(productApi.request).mockResolvedValue(row); await expect(api.create('p-1', 'alice', request())).rejects.toThrow(/did not match this exact request/);
});
it('rejects duplicate history rows and older advance results', async () => {
  vi.mocked(productApi.request).mockResolvedValueOnce({ continuations: [continuation(), continuation()] }); await expect(api.list('p-1', 'alice')).rejects.toThrow(/history/);
  vi.mocked(productApi.request).mockResolvedValueOnce(continuation()); await expect(api.advance('p-1', 'alice', 'continue-one', 5)).rejects.toThrow(/older revision/);
});
it('refuses a verification claim without its exact continuation delivery identity', async () => {
  vi.mocked(productApi.request).mockResolvedValueOnce(continuation({ status: 'checks-verified' }));
  await expect(api.get('p-1', 'alice', 'continue-one')).rejects.toThrow(/could not be verified/);
  vi.mocked(productApi.request).mockResolvedValueOnce(continuation({ status: 'checks-verified', delivery: { actionId: 'foreign-checks', checkId: 'check-one', status: 'verified', commit: 'a'.repeat(40), sourceDigest: 'b'.repeat(64), integrationDigest: 'c'.repeat(64), message: 'Verified', startedAt: '2026-10-12T00:00:00Z', updatedAt: '2026-10-12T00:00:00Z' } }));
  await expect(api.get('p-1', 'alice', 'continue-one')).rejects.toThrow(/could not be verified/);
});

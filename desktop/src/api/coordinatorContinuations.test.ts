import { beforeEach, expect, it, vi } from 'vitest';
import { productApi } from './product';
import { coordinatorContinuationsApi as api, type CoordinatorContinuation, type CoordinatorContinuationRequest, type CoordinatorSavedRoute } from './coordinatorContinuations';

it('refuses malformed cloud capacity advice without starting a task', async () => {
  vi.spyOn(productApi, 'request').mockResolvedValue({ continuations: [], savedRoute: null, cloudCapacity: {
    provider: 'e2b-embed', region: 'us-east-1', configuredGlobalSlots: 4, configuredProjectSlots: 2,
    configuredAvailableSlots: -1, estimatedTaskCostMicros: 100, estimatedMonthMicros: 0,
    monthlyBudgetMicros: 1000, maxTaskSeconds: 60, observedAt: '2026-10-13T12:00:00Z', limitation: 'unverified'
  } });
  await expect(api.listState('p-1', 'alice')).rejects.toThrow(/capacity observation/);
  expect(productApi.request).toHaveBeenCalledTimes(1);
});
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

const saved = (): CoordinatorSavedRoute => ({ revision: 3, route: { worktreeId: 'w-1', harness: 'codex', model: 'm', permissionMode: 'bypass', outcome: 'preview' }, maxParallel: 4, updatedAt: '2026-10-12T00:00:00Z', checkPlan: { commands: [{ label: 'npm', executable: 'npm', args: ['test'] }], requirementIds: ['REQ-1'], requirementSnapshots: [{ id: 'REQ-1', title: 'Book', acceptance: 'A visitor can book' }], acknowledgeProjectCommands: true } });
it('retains exact saved-route revision and command-coverage delegation for the next current selection', async () => {
  const route = saved(); vi.mocked(productApi.request).mockResolvedValueOnce({ continuations: [], savedRoute: route });
  expect((await api.listState('p-1', 'alice')).savedRoute).toEqual(route);
  const input = { ...request(), expectedRouteRevision: route.revision, checkPlan: route.checkPlan, maxParallel: route.maxParallel };
  await api.create('p-1', 'alice', input); expect(productApi.request).toHaveBeenLastCalledWith('/v1/product/projects/p-1/orchestrator/continuations', input);
});
it.each(['consent', 'coverage', 'duplicate-coverage', 'executable', 'arguments', 'limit', 'permission'])('rejects untrusted saved-route %s before it can be reused', async field => {
  const value = saved(); if (field === 'consent') value.checkPlan!.acknowledgeProjectCommands = false as never;
  if (field === 'coverage') value.checkPlan!.requirementSnapshots[0].id = 'OTHER';
  if (field === 'duplicate-coverage') value.checkPlan!.requirementIds.push('REQ-1');
  if (field === 'executable') value.checkPlan!.commands[0].executable = '../run';
  if (field === 'arguments') value.checkPlan!.commands[0].args = ['a'.repeat(32769)];
  if (field === 'limit') value.maxParallel = 201; if (field === 'permission') value.route.permissionMode = 'unrestricted' as never;
  vi.mocked(productApi.request).mockResolvedValueOnce({ continuations: [], savedRoute: value });
  await expect(api.listState('p-1', 'alice')).rejects.toThrow(/saved continuation route/);
});

const previewRow = (): CoordinatorContinuation => continuation({ status: 'preview-ready', delivery: { actionId: 'continue-one-checks', checkId: 'check-one', status: 'verified', commit: 'a'.repeat(40), sourceDigest: 'b'.repeat(64), integrationDigest: 'c'.repeat(64), message: 'Independent checks verified', startedAt: '2026-10-12T00:00:00Z', updatedAt: '2026-10-12T00:00:01Z', preview: { actionId: 'continue-one-preview', releaseId: 'rel-one', status: 'ready', message: 'Owned local HTTP responded', startedAt: '2026-10-12T00:00:01Z', updatedAt: '2026-10-12T00:00:02Z', evidence: { releaseId: 'rel-one', projectId: 'p-1', checkId: 'check-one', target: 'local', outcome: 'preview', commit: 'a'.repeat(40), sourceDigest: 'b'.repeat(64), approvals: { specificationApprovalId: 'spec-a', specificationRevision: 2, specificationDigest: 'd'.repeat(64), designApprovalId: 'design-a', designRevision: 1, designDigest: 'e'.repeat(64), briefDigest: 'f'.repeat(64), sketchDigest: '0'.repeat(64) }, observedAt: '2026-10-12T00:00:02Z', url: 'http://127.0.0.1:3000', limitations: 'Local preview only', joinedAt: '' } } } });
it('accepts an exact local preview witness without claiming public deployment', async () => {
  const value = previewRow(); vi.mocked(productApi.request).mockResolvedValue(value); expect(await api.get('p-1', 'alice', value.id)).toEqual(value);
});
it.each(['wrong target', 'missing target', 'unverified check'])('refuses preview evidence with %s', async fault => {
  const value = previewRow();
  if (fault === 'wrong target') value.delivery!.preview!.evidence!.target = 'aws';
  if (fault === 'missing target') delete (value.delivery!.preview!.evidence as { target?: string }).target;
  if (fault === 'unverified check') { value.status = 'checking'; value.delivery!.status = 'running'; }
  vi.mocked(productApi.request).mockResolvedValue(value); await expect(api.get('p-1', 'alice', value.id)).rejects.toThrow(/could not be verified/);
});

const deployedRow = (): CoordinatorContinuation => {
  const value = previewRow(); value.status = 'deployed';
  value.delivery!.gitPublication = { actionId: 'continue-one-git', status: 'verified', repository: 'https://github.com/test/app', branch: 'main', commit: value.delivery!.commit, startedAt: value.createdAt, updatedAt: value.updatedAt, message: 'Exact remote branch observed' };
  value.delivery!.deployment = { actionId: 'continue-one-deploy', releaseId: 'cloud-rel-one', status: 'ready', message: 'Exact provider application and HTTPS observed', startedAt: value.createdAt, updatedAt: value.updatedAt, evidence: { ...value.delivery!.preview!.evidence!, releaseId: 'cloud-rel-one', target: 'target-one', outcome: 'publish', url: 'https://app.example.com' } };
  return value;
};
it('keeps exact source publication, local preview and observed public deployment evidence separate', async () => {
  const value = deployedRow(); vi.mocked(productApi.request).mockResolvedValue(value);
  expect(await api.get('p-1', 'alice', value.id)).toEqual(value);
  value.status = 'unknown'; value.delivery!.deployment!.status = 'unknown'; delete value.delivery!.deployment!.evidence;
  expect(await api.get('p-1', 'alice', value.id)).toEqual(value);
});
it.each(['no-deployment', 'no-evidence', 'no-preview', 'no-git', 'git-unknown', 'git-commit', 'git-action', 'action', 'release', 'project', 'check', 'commit', 'digest', 'target', 'outcome', 'observed', 'url', 'credentials'])('refuses unsupported deployment completion: %s', async fault => {
  const value = deployedRow(), delivery = value.delivery!, action = delivery.deployment!, evidence = action.evidence!;
  if (fault === 'no-deployment') delete delivery.deployment;
  if (fault === 'no-evidence') delete action.evidence;
  if (fault === 'no-preview') delete delivery.preview;
  if (fault === 'no-git') delete delivery.gitPublication;
  if (fault === 'git-unknown') delivery.gitPublication!.status = 'unknown';
  if (fault === 'git-commit') delivery.gitPublication!.commit = 'c'.repeat(40);
  if (fault === 'git-action') delivery.gitPublication!.actionId = 'foreign-git';
  if (fault === 'action') action.actionId = 'foreign-deploy';
  if (fault === 'release') evidence.releaseId = 'cloud-rel-other';
  if (fault === 'project') evidence.projectId = 'p-2';
  if (fault === 'check') evidence.checkId = 'check-other';
  if (fault === 'commit') evidence.commit = 'c'.repeat(40);
  if (fault === 'digest') evidence.sourceDigest = 'c'.repeat(64);
  if (fault === 'target') evidence.target = 'local';
  if (fault === 'outcome') evidence.outcome = 'preview';
  if (fault === 'observed') evidence.observedAt = 'not observed';
  if (fault === 'url') evidence.url = 'javascript:alert(1)';
  if (fault === 'credentials') evidence.url = 'https://secret@example.com';
  vi.mocked(productApi.request).mockResolvedValue(value);
  await expect(api.get('p-1', 'alice', value.id)).rejects.toThrow(/could not be verified/);
});

it.each(['valid', 'no-checks', 'no-preview', 'preview-outcome', 'missing-cost-consent'])('keeps deployment-plan prerequisites bound in a saved route: %s', async condition => {
  const route = saved(); route.route.outcome = 'publish';
  route.previewPlan = { dockerfile: 'Dockerfile', port: 3000, expectedConfigurationRevision: 3, acknowledgePreviewBuild: true };
  route.deploymentPlan = { targetId: 'target-one', targetFingerprint: 'd'.repeat(64), serverUuid: 'server-one', repository: 'https://github.com/test/app', branch: 'main', domain: 'https://app.example.com', port: 3000, healthPath: '/', dockerfile: 'Dockerfile', source: 'public', dataClientFramework: 'none', expectedConfigurationRevision: 3, expectedDataDomainRevision: 0, expectedDataBindingFingerprint: '', acknowledgeRepositoryPush: true, acknowledgePublic: true, acknowledgeDeploymentCosts: true };
  if (condition === 'no-checks') delete route.checkPlan;
  if (condition === 'no-preview') delete route.previewPlan;
  if (condition === 'preview-outcome') route.route.outcome = 'preview';
  if (condition === 'missing-cost-consent') route.deploymentPlan.acknowledgeDeploymentCosts = false;
  vi.mocked(productApi.request).mockResolvedValue({ continuations: [], savedRoute: route });
  if (condition === 'valid') expect((await api.listState('p-1', 'alice')).savedRoute).toEqual(route);
  else await expect(api.listState('p-1', 'alice')).rejects.toThrow(/saved continuation route/);
});

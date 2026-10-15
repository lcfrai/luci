import type { OrchestrationContractView, PrepareOrchestrationContract } from '../api/orchestrationContracts';
const hash = 'a'.repeat(64);
export const contractPrepareFixture = (): PrepareOrchestrationContract => ({
  idempotencyKey: 'prepare-request-123456', worktreeId: 'w-1', outcome: 'preview',
  implementationContext: { projectId: 'p-1', briefRevision: 2, planRevision: 5, specificationRevision: 3, designPlanRevision: 4, taskIds: ['task-1'] },
  route: { harness: 'claude-code', model: 'saved-model', permissionMode: 'ask', executionLocation: 'local-docker' },
});
export const contractViewFixture = (approved = false): OrchestrationContractView => ({
  contract: {
    id: 'contract-1', revision: approved ? 2 : 1, digest: hash, state: approved ? 'approved' : 'prepared', projectId: 'p-1', actor: { id: 'owner', displayName: 'Owner' },
    input: { implementation: { ...contractPrepareFixture().implementationContext, worktreeId: 'w-1', taskTitles: ['Build journal'], requirementIds: ['req-1'],
      briefDigest: hash, planDigest: hash, specificationDigest: hash, designPlanDigest: hash, sourceCommit: 'a'.repeat(40), sourceDigest: hash, sourceDirty: false },
    approvals: { specificationApprovalId: 'spec-approval', specificationRevision: 3, specificationDigest: hash, designApprovalId: 'design-approval', designRevision: 4, designDigest: hash, briefDigest: hash, sketchDigest: hash },
    route: { harness: 'claude-code', model: 'saved-model', permissionMode: 'ask', connection: { id: 'saved-connection', kind: 'compatible-api', revision: 7 } },
    outcome: 'preview', requirements: [{ id: 'req-1', title: 'Build journal', acceptance: 'Survive reload' }],
    tasks: [{ id: 'task-1', title: 'Build journal', dependsOn: [], writePaths: ['src'], acceptance: 'Survive reload', requirementIds: ['req-1'] }],
    controls: { authorityPolicySource: 'workspace-private-policy', authorityPolicyDigest: hash,
      executionLimits: { monthlyBudgetUsd: 0, sessionMemoryMiB: 1024, sessionCPUMillis: 1000, maxSessionsPerUser: 1, maxConcurrentChecksPerUser: 1, allowCloudProvisioning: false, allowPublicPublishing: false },
      coordinatorPolicyRevision: 0, coordinatorPolicyDigest: hash, schedulerPolicyRevision: 0, schedulerPolicyDigest: hash },
    standards: { status: 'legacy-unassessed', source: 'approved-freeform-context', revision: 4, digest: hash },
    decisionPolicy: { routine: 'saved coordinator policy only', humanInput: 'outside saved authority, contradictory intent, or missing evidence' } },
    ...(approved ? { approval: { id: 'approval-1', actor: { id: 'owner', displayName: 'Owner' }, at: '2026-10-16T00:00:00Z', digest: hash } } : {}),
  }, freshness: { status: 'current', reason: '' },
});

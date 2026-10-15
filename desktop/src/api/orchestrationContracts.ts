import { productApi } from './product';
import type { ImplementationContext, ImplementationContextRequest } from '../store/types';
import type { ProjectRun, RunApprovals } from './projectRuns';

/** Interface pinned by docs/luci/interfaces/orchestration-contract-v1.json at fb5e408b. */
export interface PrepareOrchestrationContract {
  idempotencyKey: string;
  worktreeId: string;
  implementationContext: ImplementationContextRequest;
  route: {
    harness: 'codex' | 'claude-code' | 'opencode'; model: string; permissionMode: 'ask' | 'bypass';
    effort?: string; executionLocation: 'local-docker' | 'cloud-microvm'; agentConnectionId?: string;
  };
  outcome: 'preview' | 'publish';
}
export interface OrchestrationContractReference { id: string; revision: number; digest: string; approvalId: string }
export interface OrchestrationContractView {
  contract: {
    id: string; revision: number; digest: string; state: 'prepared' | 'approved'; projectId: string;
    actor: { id: string; displayName: string };
    input: {
      implementation: ImplementationContext; approvals: RunApprovals; route: ProjectRun['input']['route']; outcome: 'preview' | 'publish';
      requirements: ProjectRun['input']['requirements'];
      tasks: { id: string; title: string; dependsOn?: string[]; writePaths?: string[]; acceptance?: string; requirementIds?: string[] }[];
      controls: { authorityPolicySource: 'workspace-private-policy' | 'team-private-policy'; authorityPolicyDigest: string;
        executionLimits: { monthlyBudgetUsd: number; sessionMemoryMiB: number; sessionCPUMillis: number; maxSessionsPerUser: number;
          maxConcurrentChecksPerUser: number; allowCloudProvisioning: boolean; allowPublicPublishing: boolean };
        coordinatorPolicyRevision: number; coordinatorPolicyDigest: string; schedulerPolicyRevision: number; schedulerPolicyDigest: string };
      standards: { status: 'legacy-unassessed' | 'not-configured'; source: 'approved-freeform-context' | 'none'; revision: number; digest: string };
      decisionPolicy: { routine: string; humanInput: string };
    };
    approval?: { id: string; actor: { id: string; displayName: string }; at: string; digest: string };
  };
  freshness: { status: 'current' | 'changed' | 'unavailable'; reason: string };
}

const sha = (value: unknown): value is string => typeof value === 'string' && /^[a-f0-9]{64}$/.test(value);
const text = (value: unknown): value is string => typeof value === 'string' && value.length > 0;
const revision = (value: unknown): value is number => Number.isSafeInteger(value) && (value as number) >= 0;
const strings = (value: unknown): value is string[] => Array.isArray(value) && value.every(x => typeof x === 'string');

/** Public response identity only. This never replaces current server admission. */
export function ownedOrchestrationContract(projectId: string, value: OrchestrationContractView, id?: string): OrchestrationContractView {
  const c = value?.contract;
  const input = c?.input;
  if (!c || c.projectId !== projectId || input?.implementation?.projectId !== projectId || !text(c.id) || id && c.id !== id ||
      !revision(c.revision) || c.revision < 1 || !sha(c.digest) || !text(c.actor?.id) ||
      !['prepared', 'approved'].includes(c.state) || !['current', 'changed', 'unavailable'].includes(value.freshness?.status) ||
      typeof value.freshness?.reason !== 'string' || !['preview', 'publish'].includes(input.outcome) ||
      !Array.isArray(input.requirements) || input.requirements.length < 1 ||
      input.requirements.some(requirement => !text(requirement.id) || !text(requirement.title) || typeof requirement.acceptance !== 'string') ||
      !Array.isArray(input.tasks) || input.tasks.length < 1 || input.tasks.length > 200 ||
      input.tasks.some(task => !text(task.id) || !text(task.title) || task.dependsOn !== undefined && !strings(task.dependsOn) ||
        task.writePaths !== undefined && !strings(task.writePaths) || task.requirementIds !== undefined && !strings(task.requirementIds) ||
        task.acceptance !== undefined && typeof task.acceptance !== 'string') ||
      !input.controls || !revision(input.controls.coordinatorPolicyRevision) || !revision(input.controls.schedulerPolicyRevision) || !sha(input.controls.coordinatorPolicyDigest) || !sha(input.controls.schedulerPolicyDigest) ||
      !sha(input.controls.authorityPolicyDigest) || !['workspace-private-policy', 'team-private-policy'].includes(input.controls.authorityPolicySource) ||
      !['legacy-unassessed', 'not-configured'].includes(input.standards?.status) || !['approved-freeform-context', 'none'].includes(input.standards?.source) || !revision(input.standards?.revision) ||
      !(input.standards.digest === '' || sha(input.standards.digest)) || typeof input.decisionPolicy?.routine !== 'string' || typeof input.decisionPolicy?.humanInput !== 'string' ||
      !text(input.route?.harness) || typeof input.route.model !== 'string' || !['ask', 'bypass'].includes(input.route.permissionMode) ||
      c.state === 'approved' && (!text(c.approval?.id) || !text(c.approval?.actor?.id) || c.approval?.digest !== c.digest)) {
    throw new Error('The execution contract response could not be verified for this project. Refresh before continuing.');
  }
  return value;
}

export function approvedContractReference(view: OrchestrationContractView): OrchestrationContractReference {
  const { contract: c, freshness } = ownedOrchestrationContract(view.contract.projectId, view);
  if (freshness.status !== 'current' || c.state !== 'approved' || !c.approval) {
    throw new Error(freshness.reason || 'Approve the current execution contract before saving its run.');
  }
  return { id: c.id, revision: c.revision, digest: c.digest, approvalId: c.approval.id };
}

const path = (projectId: string, id?: string) => `/v1/product/projects/${encodeURIComponent(projectId)}/orchestration-contracts${id ? '/' + encodeURIComponent(id) : ''}`;
export const orchestrationContractsApi = {
  async prepare(projectId: string, request: PrepareOrchestrationContract) {
    const value = ownedOrchestrationContract(projectId, await productApi.request<OrchestrationContractView>(path(projectId) + '/prepare', request));
    const c = value.contract, context = request.implementationContext;
    if (c.input.route.harness !== request.route.harness || c.input.route.permissionMode !== request.route.permissionMode || c.input.outcome !== request.outcome ||
        request.route.model && c.input.route.model !== request.route.model || c.input.implementation.worktreeId !== request.worktreeId ||
        c.input.implementation.briefRevision !== context.briefRevision || c.input.implementation.planRevision !== context.planRevision ||
        c.input.implementation.specificationRevision !== context.specificationRevision || c.input.implementation.designPlanRevision !== context.designPlanRevision ||
        c.input.implementation.taskIds.length !== context.taskIds.length || c.input.implementation.taskIds.some(id => !context.taskIds.includes(id))) {
      throw new Error('The prepared execution contract differs from your selected route or reviewed inputs. Inspect the current project before continuing.');
    }
    return value;
  },
  async get(projectId: string, id: string) {
    return ownedOrchestrationContract(projectId, await productApi.request<OrchestrationContractView>(path(projectId, id)), id);
  },
  async approve(projectId: string, view: OrchestrationContractView) {
    const current = ownedOrchestrationContract(projectId, view);
    return ownedOrchestrationContract(projectId, await productApi.request<OrchestrationContractView>(path(projectId, current.contract.id) + '/approve', {
      expectedRevision: current.contract.revision, expectedDigest: current.contract.digest,
    }), current.contract.id);
  },
};

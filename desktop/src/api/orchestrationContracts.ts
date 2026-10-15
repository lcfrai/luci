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
    actor: { id: string; displayName: string }; implementation: ImplementationContext;
    approvals: RunApprovals; route: ProjectRun['input']['route']; outcome: 'preview' | 'publish';
    tasks: { id: string; title: string; dependsOn: string[]; writePaths: string[]; acceptance: string; requirementIds: string[] }[];
    controls: { coordinatorPolicyRevision: number; coordinatorPolicyDigest: string; schedulerPolicyRevision: number; schedulerPolicyDigest: string };
    standards: { status: 'legacy-unassessed' | 'not-configured'; source: 'approved-design-notes' | 'none'; revision: number; digest: string };
    decisionPolicy: { routine: string; humanInput: string };
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
  if (!c || c.projectId !== projectId || c.implementation?.projectId !== projectId || !text(c.id) || id && c.id !== id ||
      !revision(c.revision) || c.revision < 1 || !sha(c.digest) || !text(c.actor?.id) ||
      !['prepared', 'approved'].includes(c.state) || !['current', 'changed', 'unavailable'].includes(value.freshness?.status) ||
      typeof value.freshness?.reason !== 'string' || !['preview', 'publish'].includes(c.outcome) ||
      !Array.isArray(c.tasks) || c.tasks.length < 1 || c.tasks.length > 200 ||
      c.tasks.some(task => !text(task.id) || !text(task.title) || !strings(task.dependsOn) || !strings(task.writePaths) || !strings(task.requirementIds) || typeof task.acceptance !== 'string') ||
      !c.controls || !revision(c.controls.coordinatorPolicyRevision) || !revision(c.controls.schedulerPolicyRevision) || !sha(c.controls.coordinatorPolicyDigest) || !sha(c.controls.schedulerPolicyDigest) ||
      !['legacy-unassessed', 'not-configured'].includes(c.standards?.status) || !['approved-design-notes', 'none'].includes(c.standards?.source) || !revision(c.standards?.revision) ||
      !(c.standards.digest === '' || sha(c.standards.digest)) || typeof c.decisionPolicy?.routine !== 'string' || typeof c.decisionPolicy?.humanInput !== 'string' ||
      !text(c.route?.harness) || typeof c.route.model !== 'string' || !['ask', 'bypass'].includes(c.route.permissionMode) ||
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
    if (c.route.harness !== request.route.harness || c.route.permissionMode !== request.route.permissionMode || c.outcome !== request.outcome ||
        request.route.model && c.route.model !== request.route.model || c.implementation.worktreeId !== request.worktreeId ||
        c.implementation.briefRevision !== context.briefRevision || c.implementation.planRevision !== context.planRevision ||
        c.implementation.specificationRevision !== context.specificationRevision || c.implementation.designPlanRevision !== context.designPlanRevision ||
        c.implementation.taskIds.length !== context.taskIds.length || c.implementation.taskIds.some(id => !context.taskIds.includes(id))) {
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

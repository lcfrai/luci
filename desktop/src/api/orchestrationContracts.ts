import { productApi } from './product';
import { validAutomaticRepairLimit } from './coordinatorPolicy';
import type { ImplementationContext, ImplementationContextRequest } from '../store/types';
import { validNativeDelegationRoute, type ProjectRun, type RunApprovals } from './projectRuns';

/** Interface pinned by docs/luci/interfaces/orchestration-contract-v1.json at fb5e408b. */
export interface PrepareOrchestrationContract {
  idempotencyKey: string;
  worktreeId: string;
  implementationContext: ImplementationContextRequest;
  route: {
    harness: 'codex' | 'claude-code' | 'opencode'; model: string; permissionMode: 'ask' | 'bypass';
    effort?: string; executionLocation: 'local-docker' | 'cloud-microvm'; agentConnectionId?: string;
    nativeDelegation?: 'codex-native-helpers-2-v1';
  };
  outcome: 'preview' | 'publish';
}
export interface OrchestrationContractReference { id: string; revision: number; digest: string; approvalId: string }
export interface BoundStandardControl {
  id:string; requirement:string; category:string; applicability:string; owner:string; evidenceExpected:string;
  severity?:string; reviewCadence?:string; assessmentStatus?:string; assessmentNote?:string;
  evidence?:{link:{documentId:string; originalDigest:string; extractionRevision:number; page:number; segmentId:string; textDigest:string; targetKind:string; targetId:string};text?:string}[];
}
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
        coordinatorPolicyRevision: number; coordinatorPolicyDigest: string; schedulerPolicyRevision: number; schedulerPolicyDigest: string; automaticRepairLimit?: number };
      standards: { status: 'legacy-unassessed' | 'not-configured' | 'team-library-unassessed' | 'team-library-assessed'; source: 'approved-freeform-context' | 'none' | 'team-control-library'; revision: number; digest: string;
        teamId?:string;assessmentRevision?:number;assessmentDigest?:string;contentDigest?:string;requirements?:BoundStandardControl[] };
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
function boundedStandards(value:OrchestrationContractView['contract']['input']['standards']):boolean {
  if (!value || !['legacy-unassessed','not-configured','team-library-unassessed','team-library-assessed'].includes(value.status) ||
      !['approved-freeform-context','none','team-control-library'].includes(value.source) || !revision(value.revision) || !(value.digest===''||sha(value.digest))) return false;
  if(value.source!=='team-control-library') return value.status==='legacy-unassessed'||value.status==='not-configured';
  if(!['team-library-unassessed','team-library-assessed'].includes(value.status)||!text(value.teamId)||!sha(value.digest)||!sha(value.contentDigest)||
      !Array.isArray(value.requirements)||value.requirements.length<1||value.requirements.length>2000||
      value.requirements.some(row=>!text(row.id)||!text(row.requirement)||typeof row.applicability!=='string'||typeof row.evidenceExpected!=='string'||
        row.evidence!==undefined&&(!Array.isArray(row.evidence)||row.evidence.some(item=>!text(item.link?.documentId)||!sha(item.link?.textDigest)||item.text!==undefined&&typeof item.text!=='string')))||
      value.status==='team-library-assessed'&&(!revision(value.assessmentRevision)||!sha(value.assessmentDigest))) return false;
  return new TextEncoder().encode(JSON.stringify(value)).length<=64*1024;
}

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
      input.controls.automaticRepairLimit !== undefined && !validAutomaticRepairLimit(input.controls.automaticRepairLimit) ||
      !sha(input.controls.authorityPolicyDigest) || !['workspace-private-policy', 'team-private-policy'].includes(input.controls.authorityPolicySource) ||
      !boundedStandards(input.standards) || typeof input.decisionPolicy?.routine !== 'string' || typeof input.decisionPolicy?.humanInput !== 'string' ||
      !text(input.route?.harness) || typeof input.route.model !== 'string' || !['ask', 'bypass'].includes(input.route.permissionMode) ||
      !validNativeDelegationRoute(input.route) ||
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
        (c.input.route.nativeDelegation ?? '') !== (request.route.nativeDelegation ?? '') ||
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

import { productApi, type Brief, type ImplementationPlan } from "./product";
import type { ProjectSpecification } from "./definition";
import type { DesignPlan } from "./designPlan";

export interface OrchestratorRunSummary {
  id: string; state: string; valid: boolean; reason: string;
  tasks?: { id: string; title: string; status: string }[];
  requirements?: { id: string; title: string; acceptance: string }[];
  specificationRevision?: number; designRevision?: number; outcome?: "preview" | "publish";
  // Earlier immutable turn snapshots predate the recorded activity projection.
  activity?: "running" | "unknown" | "pending" | "none";
  activityReason?: string;
  dispatchCancelled?: boolean;
  scopeOmitted?: boolean; taskCount?: number; requirementCount?: number;
  operationsOmitted?: boolean; operationCount?: number;
  operations?: { id: string; childId: string; status: "reserved" | "dispatching" | "running" | "unknown"; updatedAt: string }[];
}
export interface OrchestratorContext {
  projectId: string; digest: string; brief: Brief; specification: ProjectSpecification;
  designPlan: DesignPlan; plan: ImplementationPlan; sketchRevision: number;
  sourceCommit: string; sourceDirty: boolean; sourceKnown: boolean;
  currentRun?: OrchestratorRunSummary;
  activeRuns?: OrchestratorRunSummary[];
  checksSummary: string; releasesSummary: string;
}
export interface OrchestratorProposal {
  target: "specification" | "design-plan" | "plan"; summary: string;
  specification: ProjectSpecification | null; designPlan: DesignPlan | null; plan: ImplementationPlan | null;
}
export interface OrchestratorAnswer {
  summary: string; question: string; options: string[]; rationale: string; blocking: boolean;
  impact: string[]; proposal: OrchestratorProposal | null; nextAction: string;
}
export type TurnStatus = "reserved" | "queued" | "running" | "succeeded" | "failed" | "cancelled" | "unknown";
export interface OrchestratorTurn {
  id: string; projectId: string; sequence: number; actor: { id: string; displayName: string };
  message: string; jobId: string; replyToTurnId?: string; contextDigest: string;
  context: OrchestratorContext; status: TurnStatus; createdAt: string; updatedAt: string;
  answer?: OrchestratorAnswer; appliedRevision?: number; questionResolvedBy?: string;
}
export interface OrchestratorView {
  projectId: string; revision: number; context: OrchestratorContext; turns: OrchestratorTurn[];
  nextAction: { id: string; label: string; reason: string; lens: "discover" | "design" | "develop" | "deploy"; runId?: string };
  blockingQuestions: { turnId: string; question: string; rationale: string; options: string[] }[];
  hasEarlier: boolean; beforeSequence: number;
  storageDigest: string; archivePresent: boolean;
}
export interface OrchestratorRequest {
  expectedRevision: number; expectedContextDigest: string; idempotencyKey: string;
  message: string; replyToTurnId?: string;
  harness: "codex" | "claude-code" | "opencode"; model?: string; agentConnectionId?: string;
}
export interface OrchestratorJob {
  id: string; projectId: string; kind: "orchestrator"; status: TurnStatus; message: string;
  orchestratorTurnId: string; orchestratorContext: OrchestratorContext; orchestratorResult?: OrchestratorAnswer;
  harness?: string; model?: string; agentConnectionId?: string; agentConnectionRevision?: number;
  createdAt: string; updatedAt: string;
}
const route = (id: string) => `/v1/product/projects/${encodeURIComponent(id)}`;
function validActiveRuns(value: OrchestratorContext): boolean {
  if (value.activeRuns === undefined) return true;
  return Array.isArray(value.activeRuns) && new Set(value.activeRuns.map(run => run?.id)).size === value.activeRuns.length &&
    value.activeRuns.every(run => run && typeof run.id === "string" && run.id.length > 0 &&
      ["running", "unknown", "pending"].includes(run.activity ?? "") && typeof run.dispatchCancelled === "boolean" &&
      Array.isArray(run.operations) && (run.operationsOmitted === true
        ? run.operations.length === 0 && Number.isSafeInteger(run.operationCount) && run.operationCount! > 0
        : run.operations.length > 0) &&
      (!run.scopeOmitted || (Number.isSafeInteger(run.taskCount) && run.taskCount! >= 0 && Number.isSafeInteger(run.requirementCount) && run.requirementCount! >= 0)) &&
      new Set(run.operations.map(operation => operation?.id)).size === run.operations.length &&
      run.operations.every(operation => operation && typeof operation.id === "string" && operation.id.length > 0 &&
        typeof operation.childId === "string" && operation.childId.length > 0 &&
        ["reserved", "dispatching", "running", "unknown"].includes(operation.status) &&
        typeof operation.updatedAt === "string" && Number.isFinite(Date.parse(operation.updatedAt))));
}
function verifiedView(value: OrchestratorView, projectId: string): OrchestratorView {
  if (value?.projectId !== projectId || value.context?.projectId !== projectId || !Number.isSafeInteger(value.revision) ||
      !Array.isArray(value.turns) || !Array.isArray(value.blockingQuestions) ||
      value.turns.some(turn => turn.projectId !== projectId || turn.context?.projectId !== projectId) ||
      new Set(value.turns.map(turn => turn.id)).size !== value.turns.length ||
      !validActiveRuns(value.context) ||
      (value.nextAction?.runId !== undefined && ![value.context.currentRun?.id, ...(value.context.activeRuns ?? []).map(run => run.id)].includes(value.nextAction.runId)) ||
      !["discover", "design", "develop", "deploy"].includes(value.nextAction?.lens)) {
    throw new Error("Luci's conversation could not be verified for this project. Refresh before continuing.");
  }
  return value;
}
function verifiedJob(value: OrchestratorJob, projectId: string): OrchestratorJob {
  if (value?.projectId !== projectId || value.kind !== "orchestrator" || value.orchestratorContext?.projectId !== projectId) {
    throw new Error("The assistance response belongs to a different conversation. Refresh before continuing.");
  }
  return value;
}
export const orchestratorApi = {
  async get(projectId: string, before?: number) {
    return verifiedView(await productApi.request<OrchestratorView>(`${route(projectId)}/orchestrator${before === undefined ? "" : `?before=${encodeURIComponent(before)}`}`), projectId);
  },
  async send(projectId: string, input: OrchestratorRequest) {
    return verifiedJob(await productApi.request<OrchestratorJob>(`${route(projectId)}/assist/orchestrator`, input), projectId);
  },
  async job(projectId: string, jobId: string) {
    return verifiedJob(await productApi.request<OrchestratorJob>(`${route(projectId)}/assist/${encodeURIComponent(jobId)}`), projectId);
  },
  async cancel(projectId: string, jobId: string) {
    return verifiedJob(await productApi.request<OrchestratorJob>(`${route(projectId)}/assist/${encodeURIComponent(jobId)}/cancel`, {}), projectId);
  },
  async reconcile(projectId: string, jobId: string) {
    return verifiedJob(await productApi.request<OrchestratorJob>(`${route(projectId)}/assist/${encodeURIComponent(jobId)}/reconcile`, {}), projectId);
  },
  async export(projectId: string) {
    const value = await productApi.request<{ version: 1; projectId: string; storageDigest: string; ledger: unknown; archive?: unknown }>(`${route(projectId)}/orchestrator/export`);
    if (value?.version !== 1 || value.projectId !== projectId || !value.storageDigest || !value.ledger) throw new Error("The conversation export could not be verified for this project.");
    return value;
  },
  async rotate(projectId: string, input: { expectedRevision: number; expectedStorageDigest: string; acknowledgeExportedHistory: true }) {
    return verifiedView(await productApi.request<OrchestratorView>(`${route(projectId)}/orchestrator/rotate`, input), projectId);
  },
  async apply(projectId: string, turnId: string, input: { expectedRevision: number; expectedContextDigest: string }) {
    return verifiedView(await productApi.request<OrchestratorView>(`${route(projectId)}/orchestrator/turns/${encodeURIComponent(turnId)}/apply`, input), projectId);
  },
};

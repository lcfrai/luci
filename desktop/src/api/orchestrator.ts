import { productApi, type Brief, type ImplementationPlan } from "./product";
import type { ProjectSpecification } from "./definition";
import type { DesignPlan } from "./designPlan";
import type { ProjectRun } from "./projectRuns";
import { validRunAttention, type ProjectRunAttention } from "./runAttention";

export interface OrchestratorRunSummary {
  id: string; state: string; valid: boolean; reason: string;
  // Current saved inputs, separate from recorded progress or acceptance.
  contextFreshness?: ProjectRun["contextFreshness"];
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
  needsAttention?: ProjectRunAttention[];
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
export interface OrchestratorActivity {
  accepted: { eventId: string; sequence: number; intentId: string; originTurnId: string;
    actor: { id: string; displayName: string }; target: string; appliedTargetDigest: string; acceptedContextDigest?: string;
    taskIds: string[]; affectsAllTasks: boolean }[];
  work: { id: string; originTurnId: string; contextDigest: string;
    acceptedEvents: { eventId: string; sequence: number; originIntentId: string; archiveDigest?: string }[];
    status: string; message: string; runId?: string; updatedAt: string;
    finding?: { status: string; checkId?: string; commit?: string; previewStatus?: string; previewReleaseId?: string;
      deployStatus?: string; deployReleaseId?: string } }[];
  earlierAvailable: boolean;
}
export interface OrchestratorView {
  projectId: string; revision: number; context: OrchestratorContext; turns: OrchestratorTurn[];
  nextAction: { id: string; label: string; reason: string; lens: "discover" | "design" | "develop" | "deploy"; runId?: string };
  blockingQuestions: { turnId: string; question: string; rationale: string; options: string[] }[];
  hasEarlier: boolean; beforeSequence: number;
  storageDigest: string; archivePresent: boolean;
  activity?: OrchestratorActivity;
}
export interface OrchestratorRequest {
  expectedRevision: number; expectedContextDigest: string; idempotencyKey: string;
  message: string; replyToTurnId?: string;
  harness: "codex" | "claude-code" | "opencode"; model?: string; agentConnectionId?: string;
}
export interface OrchestratorAdvanceRequest {
  expectedRevision: number; expectedContextDigest: string; expectedPolicyRevision: number;
}
export interface OrchestratorJob {
  id: string; projectId: string; kind: "orchestrator"; status: TurnStatus; message: string;
  orchestratorTurnId: string; orchestratorContext: OrchestratorContext; orchestratorResult?: OrchestratorAnswer;
  harness?: string; model?: string; agentConnectionId?: string; agentConnectionRevision?: number;
  createdAt: string; updatedAt: string;
}
export interface OrchestratorResolution {
  projectId: string; status: "accepted" | "not-sent" | "unknown";
  job?: OrchestratorJob; turnId?: string; jobId?: string; message: string;
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
      !validActiveRuns(value.context) || !validActivity(value.activity) ||
      (value.context.needsAttention !== undefined && !validRunAttention(value.context.needsAttention, 200)) ||
      (value.nextAction?.runId !== undefined && ![value.context.currentRun?.id, ...(value.context.activeRuns ?? []).map(run => run.id), ...(value.context.needsAttention ?? []).map(run => run.id)].includes(value.nextAction.runId)) ||
      !["discover", "design", "develop", "deploy"].includes(value.nextAction?.lens)) {
    throw new Error("Luci's conversation could not be verified for this project. Refresh before continuing.");
  }
  return value;
}
function validActivity(activity: OrchestratorActivity | undefined): boolean {
  if (activity === undefined) return true;
  if (!Array.isArray(activity.accepted) || !Array.isArray(activity.work) ||
      activity.accepted.length > 8 || activity.work.length > 8 || typeof activity.earlierAvailable !== "boolean") return false;
  if (new Set(activity.accepted.map(event => event.eventId)).size !== activity.accepted.length ||
      new Set(activity.work.map(work => work.id)).size !== activity.work.length) return false;
  return activity.accepted.every(event => event && typeof event.eventId === "string" && !!event.eventId &&
    Number.isSafeInteger(event.sequence) && event.sequence > 0 && !!event.intentId && !!event.originTurnId &&
    !!event.actor?.id && typeof event.actor.displayName === "string" && !!event.target &&
    !!event.appliedTargetDigest && (event.acceptedContextDigest === undefined || !!event.acceptedContextDigest) && Array.isArray(event.taskIds) &&
    event.taskIds.every(id => typeof id === "string" && !!id) && typeof event.affectsAllTasks === "boolean") &&
    activity.work.every(work => work && !!work.id && !!work.originTurnId && !!work.contextDigest &&
      Array.isArray(work.acceptedEvents) && work.acceptedEvents.every(event => event && !!event.eventId &&
        Number.isSafeInteger(event.sequence) && event.sequence > 0 && !!event.originIntentId) &&
      typeof work.status === "string" && !!work.status && typeof work.message === "string" &&
      typeof work.updatedAt === "string" && Number.isFinite(Date.parse(work.updatedAt)) &&
      (work.finding === undefined || (!!work.finding.status &&
        [work.finding.checkId, work.finding.commit, work.finding.previewStatus, work.finding.previewReleaseId,
          work.finding.deployStatus, work.finding.deployReleaseId].every(value => value === undefined || typeof value === "string"))));
}
function verifiedJob(value: OrchestratorJob, projectId: string): OrchestratorJob {
  if (value?.projectId !== projectId || value.kind !== "orchestrator" || value.orchestratorContext?.projectId !== projectId) {
    throw new Error("The assistance response belongs to a different conversation. Refresh before continuing.");
  }
  return value;
}
export const orchestratorApi = {
  async get(projectId: string, before?: number, signal?: AbortSignal) {
    const path = `${route(projectId)}/orchestrator${before === undefined ? "" : `?before=${encodeURIComponent(before)}`}`;
    const value = await (signal
      ? productApi.request<OrchestratorView>(path, undefined, "GET", 120_000, signal)
      : productApi.request<OrchestratorView>(path));
    return verifiedView(value, projectId);
  },
  async send(projectId: string, input: OrchestratorRequest) {
    return verifiedJob(await productApi.request<OrchestratorJob>(`${route(projectId)}/assist/orchestrator`, input), projectId);
  },
  async resolveRequest(projectId: string, input: OrchestratorRequest) {
    const value = await productApi.request<OrchestratorResolution>(`${route(projectId)}/orchestrator/resolve`, input);
    if (value?.projectId !== projectId || !["accepted", "not-sent", "unknown"].includes(value.status) || typeof value.message !== "string" || (value.status === "accepted" && !value.job)) {
      throw new Error("The original request could not be verified. Its saved identity has been retained.");
    }
    if (value.job) verifiedJob(value.job, projectId);
    return value;
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
  async advance(projectId: string, turnId: string, input: OrchestratorAdvanceRequest) {
    return verifiedView(await productApi.request<OrchestratorView>(`${route(projectId)}/orchestrator/turns/${encodeURIComponent(turnId)}/advance`, input), projectId);
  },
};

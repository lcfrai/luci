import { productApi } from "./product";

export interface CoordinatorIntent {
  id: string; turnId: string; actor: { id: string; displayName: string }; luciId: string;
  baseContextDigest: string; proposalDigest: string; target: "specification" | "design-plan" | "plan";
  requestedChange: string; changedPaths: string[]; requirementIds: string[]; taskIds: string[];
  affectsAllTasks: boolean; dependsOn: string[]; status: "pending" | "applied" | "superseded"; createdAt: string;
  appliedRevision?: number; applyBaseContextDigest?: string; candidateProposalDigest?: string;
  appliedTargetDigest?: string; acceptedContextDigest?: string; supersededBy?: string; resolutionReason?: string;
}
export interface CoordinatorEvent {
  id: string; sequence: number; previousEventId?: string; kind: "proposed" | "applied" | "superseded";
  intentId: string; actor: { id: string; displayName: string }; at: string; causes: string[]; contextDigest: string;
}
export interface CoordinatorConflict {
  intentId: string; otherIntentId?: string; paths: string[];
  reason: "origin-unavailable" | "current-context-conflict" | "dependency-unresolved" | "opposing-intent";
}
export interface CoordinatorIntentsView {
  projectId: string; revision: number; contextDigest: string; intents: CoordinatorIntent[];
  events: CoordinatorEvent[]; conflicts: CoordinatorConflict[]; allowedActions: ("propose-own-turn" | "supersede-own-intent")[];
}
export interface RecordCoordinatorIntent {
  expectedRevision: number; expectedContextDigest: string; turnId: string; idempotencyKey: string; dependsOn: string[];
}
export interface ResolveCoordinatorIntent {
  expectedRevision: number; expectedContextDigest: string; supersededBy: string; reason: string;
}
const route = (id: string) => `/v1/product/projects/${encodeURIComponent(id)}/orchestrator/coordination`;
const text = (value: unknown, limit: number, empty = false): value is string => typeof value === "string" && (empty || value.length > 0) && value.length <= limit;
const hash = (value: unknown): value is string => typeof value === "string" && /^[a-f0-9]{64}$/.test(value);
const list = (value: unknown, limit: number): value is string[] => Array.isArray(value) && value.length <= limit && value.every(item => text(item, 16000)) && new Set(value).size === value.length;
const actor = (value: CoordinatorIntent["actor"]) => value && text(value.id, 128) && text(value.displayName, 512);
const timestamp = (value: unknown) => typeof value === "string" && Number.isFinite(Date.parse(value));
function validIntent(value: CoordinatorIntent) {
  return value && text(value.id, 128) && value.id.startsWith("intent-") && text(value.turnId, 128) && value.turnId.startsWith("turn-") &&
    actor(value.actor) && text(value.luciId, 128) && value.luciId.startsWith("luci-") && hash(value.baseContextDigest) && hash(value.proposalDigest) &&
    ["specification", "design-plan", "plan"].includes(value.target) && text(value.requestedChange, 16000, true) &&
    list(value.changedPaths, 1024) && list(value.requirementIds, 200) && list(value.taskIds, 200) && list(value.dependsOn, 16) &&
    typeof value.affectsAllTasks === "boolean" && ["pending", "applied", "superseded"].includes(value.status) && timestamp(value.createdAt) &&
    [value.applyBaseContextDigest, value.candidateProposalDigest, value.appliedTargetDigest, value.acceptedContextDigest].every(v => v === undefined || hash(v)) &&
    (value.status !== "applied" || (Number.isSafeInteger(value.appliedRevision) && value.appliedRevision! > 0 && hash(value.appliedTargetDigest) && hash(value.applyBaseContextDigest) && hash(value.candidateProposalDigest))) &&
    (value.status !== "superseded" || (text(value.supersededBy, 128) && value.supersededBy !== value.id && text(value.resolutionReason, 2000)));
}
function verifiedIntent(value: CoordinatorIntent) {
  if (!validIntent(value)) throw new Error("The team intent response could not be verified. Read the current coordination state before making another change.");
  return value;
}
function verifiedView(value: CoordinatorIntentsView, projectId: string) {
  if (!value || value.projectId !== projectId || !Number.isSafeInteger(value.revision) || value.revision < 0 || !hash(value.contextDigest) ||
      !Array.isArray(value.intents) || value.intents.length > 64 || !value.intents.every(validIntent) ||
      new Set(value.intents.map(intent => intent.id)).size !== value.intents.length || new Set(value.intents.map(intent => intent.turnId)).size !== value.intents.length ||
      !Array.isArray(value.events) || value.events.length > 192 || !Array.isArray(value.conflicts) || value.conflicts.length > 4096 ||
      !list(value.allowedActions, 2) || value.allowedActions.some(action => !["propose-own-turn", "supersede-own-intent"].includes(action))) {
    throw new Error("Team coordination could not be verified for this project. Refresh before continuing.");
  }
  const ids = new Set(value.intents.map(intent => intent.id));
  if (value.intents.some(intent => intent.dependsOn.some(id => !ids.has(id) || id === intent.id) || (intent.supersededBy && !ids.has(intent.supersededBy))) ||
      new Set(value.events.map(event => event.id)).size !== value.events.length ||
      value.events.some((event, index) => !event || !text(event.id, 128) || event.sequence !== index + 1 ||
        (event.previousEventId ?? "") !== (index ? value.events[index - 1].id : "") || !ids.has(event.intentId) || !actor(event.actor) || !timestamp(event.at) ||
        !["proposed", "applied", "superseded"].includes(event.kind) || !hash(event.contextDigest) || !list(event.causes, 17) || event.causes.some(id => !ids.has(id))) ||
      value.conflicts.some(conflict => !conflict || !ids.has(conflict.intentId) || (conflict.otherIntentId && !ids.has(conflict.otherIntentId)) || !list(conflict.paths, 1024) ||
        !["origin-unavailable", "current-context-conflict", "dependency-unresolved", "opposing-intent"].includes(conflict.reason))) {
    throw new Error("Team coordination has incomplete causal records. Refresh before continuing.");
  }
  return value;
}
export const coordinatorIntentsApi = {
  async get(projectId: string, signal?: AbortSignal) {
    return verifiedView(await productApi.request<CoordinatorIntentsView>(route(projectId), undefined, "GET", 120_000, signal), projectId);
  },
  async record(projectId: string, actorId: string, input: RecordCoordinatorIntent) {
    const value = verifiedIntent(await productApi.request<CoordinatorIntent>(`${route(projectId)}/intents`, input));
    if (value.actor.id !== actorId || value.turnId !== input.turnId || value.dependsOn.join("\n") !== input.dependsOn.join("\n")) {
      throw new Error("The recorded intent did not match this person's proposal. Read its outcome before continuing.");
    }
    return value;
  },
  async resolve(projectId: string, actorId: string, intentId: string, input: ResolveCoordinatorIntent) {
    const value = verifiedIntent(await productApi.request<CoordinatorIntent>(`${route(projectId)}/intents/${encodeURIComponent(intentId)}/resolve`, input));
    if (value.id !== intentId || value.actor.id !== actorId || value.status !== "superseded" || value.supersededBy !== input.supersededBy || value.resolutionReason !== input.reason) {
      throw new Error("The resolution response did not confirm this exact change. Read its outcome before continuing.");
    }
    return value;
  },
};

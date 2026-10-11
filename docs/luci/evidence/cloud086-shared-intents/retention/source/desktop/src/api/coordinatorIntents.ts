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
  reason: "origin-unavailable" | "current-context-conflict" | "dependency-unresolved" | "opposing-intent" | "merged-context-invalid";
}
export interface CoordinatorArchive {
  digest: string; bytes: number; events: { id: string; sequence: number; originIntentId: string }[];
}
export interface CoordinatorIntentsView {
  projectId: string; revision: number; contextDigest: string; intents: CoordinatorIntent[];
  events: CoordinatorEvent[]; archives?: CoordinatorArchive[]; historicalCauses?: CoordinatorIntent[]; eventSequence?: number; eventHead?: string; conflicts: CoordinatorConflict[]; allowedActions: ("propose-own-turn" | "supersede-own-intent")[];
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
  const archives = value.archives ?? [], historical = value.historicalCauses ?? [];
  const sequence = new Map<number, string>(), eventIds = new Set<string>(), archivedOrigins = new Map<string, string>();
  const fail = () => { throw new Error("Team coordination has incomplete causal records. Refresh before continuing."); };
  if ((value.archives !== undefined && !Array.isArray(value.archives)) || archives.length > 64 ||
      (value.historicalCauses !== undefined && !Array.isArray(value.historicalCauses)) || historical.length > 1024) fail();
  const addEvent = (id: string, position: number) => {
    if (!text(id, 128) || !id.startsWith("coord-event-") || !Number.isSafeInteger(position) || position < 1 || position > 12480 || sequence.has(position) || eventIds.has(id)) fail();
    sequence.set(position, id); eventIds.add(id);
  };
  const archiveDigests = new Set<string>();
  let archiveBytes = 0;
  for (const archive of archives) {
    if (!archive || !hash(archive.digest) || archiveDigests.has(archive.digest) || !Number.isSafeInteger(archive.bytes) || archive.bytes < 1 || archive.bytes > 25 * 1024 * 1024 ||
        !Array.isArray(archive.events) || archive.events.length < 1 || archive.events.length > 192) fail();
    archiveDigests.add(archive.digest); archiveBytes += archive.bytes;
    for (const ref of archive.events) {
      if (!ref || !text(ref.originIntentId, 128) || !ref.originIntentId.startsWith("intent-") || ids.has(ref.originIntentId) ||
          (archivedOrigins.has(ref.originIntentId) && archivedOrigins.get(ref.originIntentId) !== archive.digest)) fail();
      archivedOrigins.set(ref.originIntentId, archive.digest);
      addEvent(ref.id, ref.sequence);
    }
  }
  if (archiveBytes > 256 * 1024 * 1024) fail();
  const causes = new Set(ids);
  for (const witness of historical) {
    if (!validIntent(witness) || witness.status !== "applied" || causes.has(witness.id) || !archivedOrigins.has(witness.id)) fail();
    causes.add(witness.id);
  }
  for (const event of value.events) {
    if (!event) fail();
    addEvent(event.id, event.sequence);
  }
  for (let i = 1; i <= sequence.size; i++) if (!sequence.has(i)) fail();
  const modern = value.eventSequence !== undefined || value.eventHead !== undefined || archives.length > 0 || historical.length > 0;
  if (modern && (!Number.isSafeInteger(value.eventSequence) || value.eventSequence !== sequence.size || value.eventHead !== (sequence.get(sequence.size) ?? ""))) fail();
  if (value.intents.some(intent => intent.dependsOn.some(id => !causes.has(id) || id === intent.id) || (intent.supersededBy && !ids.has(intent.supersededBy))) ||
      value.events.some((event, index) => !ids.has(event.intentId) || !actor(event.actor) || !timestamp(event.at) ||
        (index > 0 && event.sequence <= value.events[index - 1].sequence) || (event.previousEventId ?? "") !== (sequence.get(event.sequence - 1) ?? "") ||
        !["proposed", "applied", "superseded"].includes(event.kind) || !hash(event.contextDigest) || !list(event.causes, 17) || event.causes.some(id => !causes.has(id))) ||
      value.conflicts.some(conflict => !conflict || !ids.has(conflict.intentId) || (conflict.otherIntentId && !causes.has(conflict.otherIntentId)) || !list(conflict.paths, 1024) ||
        !["origin-unavailable", "current-context-conflict", "dependency-unresolved", "opposing-intent", "merged-context-invalid"].includes(conflict.reason))) fail();
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

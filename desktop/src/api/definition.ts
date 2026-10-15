import { productApi } from "./product";
import type { DiscoverySourceLink } from "./discoveryDocuments";

export interface Requirement { id: string; title: string; priority: "must" | "should" | "could"; acceptance: string }
export interface UserJourney { id: string; name: string; actor: string; steps: string; requirementIds: string[] }
export interface AccessRole { id: string; name: string; canDo: string; cannotDo: string }
export interface DataEntity { id: string; name: string; fields: string; privacy: string }
export interface Integration { id: string; name: string; purpose: string; constraint: string }
export interface GlossaryTerm { id: string; term: string; meaning: string }
export interface ProductDecision { id: string; title: string; decision: string; reason: string; alternatives: string }
export interface OpenQuestion { id: string; question: string; answer: string }
export interface ProjectSpecification {
 evidence?: DiscoverySourceLink[];
  projectId: string; version: 1; revision: number; briefRevision: number;
  startingPoint: "new" | "existing"; purpose: string; audience: string; constraints: string;
  outOfScope: string[]; requirements: Requirement[]; journeys: UserJourney[]; roles: AccessRole[];
  entities: DataEntity[]; integrations: Integration[]; glossary: GlossaryTerm[];
  decisions: ProductDecision[]; questions: OpenQuestion[];
  status: "draft" | "approved" | "outdated"; digest: string; updatedAt: string;
}
export interface DefinitionAssistance {
  id: string; projectId: string; status: "reserved" | "queued" | "running" | "succeeded" | "failed" | "cancelled";
  kind?: "interview" | "repository"; harness?: "codex" | "claude-code" | "opencode";
  model?: string; authMode?: "chatgpt" | "api" | "configured"; appliedRevision?: number;
  agentConnectionId?: string; agentConnectionRevision?: number; agentConnectionKind?: string; agentConnectionProtocol?: string;
  source?: RepositoryAnalysisSource;
  requestedBy?: string; ownRequest?: boolean; replyToAnalysisId?: string; replySourceChanged?: boolean; idempotencyKey?: string; userMessage?: string;
  message: string; result?: { question: string; options: string[]; rationale: string; proposal: ProjectSpecification | null; findings?: { kind: "observed" | "inference" | "gap"; summary: string; paths: string[] }[] };
  createdAt: string; updatedAt: string;
}
export interface RepositoryAnalysisSource {
  mode: "worktree" | "commit"; commit?: string; digest: string; fileCount: number;
  includedBytes: number; skippedFiles: number; partial: true; limitation: string;
  files: { path: string; digest: string; bytes: number }[];
}
export interface RepositoryAnalysisRequest {
  expectedRevision: number; expectedBriefRevision: number; sourceMode: "worktree" | "commit";
  harness: "codex" | "claude-code" | "opencode"; model: string;
  authMode?: "chatgpt" | "api" | "configured"; agentConnectionId?: string; expectedAgentConnectionRevision?: number; acknowledgeSourceRead: true; message?: string;
  replyToAnalysisId?: string; idempotencyKey?: string;
}
export interface DefinitionAgentOptions {
  harness?: "codex" | "claude-code" | "opencode"; model?: string; agentConnectionId?: string;
}
const path = (id: string) => `/v1/product/projects/${encodeURIComponent(id)}`;
function repositoryReceipt(value: DefinitionAssistance, projectId: string, key?: string): DefinitionAssistance {
  if (!value || value.projectId !== projectId || value.kind !== "repository" || value.ownRequest !== true || typeof value.id !== "string" || !value.id ||
      !["reserved", "queued", "running", "succeeded", "failed", "cancelled"].includes(value.status) ||
      (key !== undefined && value.idempotencyKey !== key)) {
    throw new Error("The repository analysis receipt could not be verified. Keep its saved request and check its status.");
  }
  return value;
}
export const definitionApi = {
  get: (id: string, signal?: AbortSignal) => signal
    ? productApi.request<ProjectSpecification>(`${path(id)}/specification`, undefined, "GET", 120_000, signal)
    : productApi.request<ProjectSpecification>(`${path(id)}/specification`),
  save: (id: string, specification: ProjectSpecification, expectedRevision: number, expectedBriefRevision: number) =>
    productApi.request<ProjectSpecification>(`${path(id)}/specification`, { expectedRevision, expectedBriefRevision, specification }),
  approve: (id: string, revision: number, briefRevision: number) =>
    productApi.request<ProjectSpecification>(`${path(id)}/specification/approve`, { revision, briefRevision }),
  history: (id: string) => productApi.request<{ revisions: ProjectSpecification[] }>(`${path(id)}/specification/history`),
  assist: (id: string, expectedRevision: number, message: string, agent: DefinitionAgentOptions = {}) =>
    productApi.request<DefinitionAssistance>(`${path(id)}/assist`, { expectedRevision, message, ...agent }),
  assistance: (id: string, jobId: string) => productApi.request<DefinitionAssistance>(`${path(id)}/assist/${encodeURIComponent(jobId)}`),
  cancelAssistance: (id: string, jobId: string) => productApi.request<DefinitionAssistance>(`${path(id)}/assist/${encodeURIComponent(jobId)}/cancel`, {}),
  listAssistance: (id: string) => productApi.request<{ jobs: DefinitionAssistance[] }>(`${path(id)}/assist`),
  analyseRepository: async (id: string, input: RepositoryAnalysisRequest) => repositoryReceipt(await productApi.request<DefinitionAssistance>(`${path(id)}/assist/repository`, input), id, input.idempotencyKey),
  repositoryRequest: async (id: string, key: string) => {
    if (!/^[A-Za-z0-9_-]{16,128}$/.test(key)) throw new Error("The saved analysis request identity is invalid.");
    return repositoryReceipt(await productApi.request<DefinitionAssistance>(`${path(id)}/assist/repository/requests/${encodeURIComponent(key)}`), id, key);
  },
  applyAnalysis: (id: string, jobId: string, input: { expectedRevision: number; expectedBriefRevision: number; acknowledgePartial: true }) => productApi.request<ProjectSpecification>(`${path(id)}/assist/${encodeURIComponent(jobId)}/apply`, input),
};

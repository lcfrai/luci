import type { RepositoryAnalysisRequest } from "../../api/definition";

// Local to the exact account/workspace/project scope supplied by the caller.
// Neither drafts nor unresolved request identities expire or trigger a send.
const prefix = "luci.repository-analysis.v1:";
const recordLimit = 64 * 1024, totalLimit = 2 * 1024 * 1024, countLimit = 128;
const unavailable = "Repository analysis recovery is unavailable. Keep this page open and copy your answer; enable browser storage before sending or checking recovery again.";
const lockUnavailable = "This browser cannot safely coordinate analysis recovery between tabs. Use a browser with Web Locks over HTTPS or localhost before sending; keep a copy of your answer.";
const full = "Repository analysis recovery is full. Resolve saved requests or explicitly clear unneeded answers before sending. Your existing recovery records have been kept.";
const changedDraft = "This answer changed in another tab. Keep a copy of your edits and reload its saved answer before continuing.";
const changedRequest = "Another repository analysis request is saved in this browser. Check that original request before sending or clearing a different one.";

export interface RepositoryAnalysisDraft {
  message: string;
  replyToAnalysisId: string;
  sourceMode: "worktree" | "commit";
  harness: RepositoryAnalysisRequest["harness"] | "";
  model: string;
  agentConnectionId?: string;
}
export type PendingRepositoryAnalysisRequest = RepositoryAnalysisRequest & { idempotencyKey: string; replyToAnalysisId?: string };
export interface RepositoryAnalysisRecovery {
  draft: RepositoryAnalysisDraft | null;
  pending: PendingRepositoryAnalysisRequest | null;
  error: string;
}
const bytes = (value: string) => new TextEncoder().encode(value).length;
const text = (value: unknown, limit: number): value is string => typeof value === "string" && value.length <= limit && bytes(value) <= limit;
const fields = (value: unknown, allowed: string[]): value is Record<string, unknown> => !!value && typeof value === "object" &&
  (Object.getPrototypeOf(value) === Object.prototype || Object.getPrototypeOf(value) === null) &&
  Object.entries(Object.getOwnPropertyDescriptors(value)).every(([key, descriptor]) => allowed.includes(key) && "value" in descriptor);
const source = (value: unknown) => value === "worktree" || value === "commit";
const harness = (value: unknown) => value === "codex" || value === "claude-code" || value === "opencode";
const optionalText = (value: unknown, limit: number) => value === undefined || text(value, limit);
function validDraft(value: unknown): value is RepositoryAnalysisDraft {
  return fields(value, ["message", "replyToAnalysisId", "sourceMode", "harness", "model", "agentConnectionId"]) &&
    text(value.message, 16000) && text(value.replyToAnalysisId, 256) && source(value.sourceMode) &&
    (value.harness === "" || harness(value.harness)) && text(value.model, 512) && optionalText(value.agentConnectionId, 256);
}
function validRequest(value: unknown): value is PendingRepositoryAnalysisRequest {
  return fields(value, ["expectedRevision", "expectedBriefRevision", "sourceMode", "harness", "model", "authMode", "agentConnectionId", "expectedAgentConnectionRevision", "acknowledgeSourceRead", "message", "replyToAnalysisId", "idempotencyKey"]) &&
    Number.isSafeInteger(value.expectedRevision) && Number(value.expectedRevision) >= 0 &&
    Number.isSafeInteger(value.expectedBriefRevision) && Number(value.expectedBriefRevision) >= 0 &&
    source(value.sourceMode) && harness(value.harness) && text(value.model, 512) && !!value.model.trim() &&
    (value.authMode === undefined || (typeof value.authMode === "string" && ["chatgpt", "api", "configured"].includes(value.authMode))) &&
    optionalText(value.agentConnectionId, 256) &&
    (value.expectedAgentConnectionRevision === undefined || (Number.isSafeInteger(value.expectedAgentConnectionRevision) && Number(value.expectedAgentConnectionRevision) > 0)) &&
    value.acknowledgeSourceRead === true &&
    optionalText(value.message, 16000) && optionalText(value.replyToAnalysisId, 256) &&
    typeof value.idempotencyKey === "string" && /^[A-Za-z0-9_-]{16,128}$/.test(value.idempotencyKey);
}
function scopeKey(scope: string) {
  if (!text(scope, 8192) || !scope.trim()) throw new Error("Your account and workspace must be known before analysis recovery can be saved. Keep a copy of your answer.");
  return prefix + encodeURIComponent(scope) + ":";
}
// Flat, allowlisted DTOs only. Stable ordering makes CAS independent of callers'
// property insertion order and matches the request's JSON wire representation.
function encode(value: RepositoryAnalysisDraft | PendingRepositoryAnalysisRequest) {
  return JSON.stringify(Object.fromEntries(Object.entries(value).sort(([a], [b]) => a.localeCompare(b))));
}
function equal(a: RepositoryAnalysisDraft | PendingRepositoryAnalysisRequest | null, b: RepositoryAnalysisDraft | PendingRepositoryAnalysisRequest | null) {
  return a === null || b === null ? a === b : encode(a) === encode(b);
}
function read<T>(key: string, valid: (value: unknown) => value is T): T | null {
  const raw = localStorage.getItem(key);
  if (raw === null) return null;
  if (raw.length > recordLimit || bytes(raw) > recordLimit) throw new Error(unavailable);
  const record: unknown = JSON.parse(raw);
  if (!fields(record, ["version", "value"]) || record.version !== 1 || !valid(record.value)) throw new Error(unavailable);
  return record.value;
}
function readStored(scope: string): RepositoryAnalysisRecovery {
  const key = scopeKey(scope);
  return { draft: read(key + "draft", validDraft), pending: read(key + "pending", validRequest), error: "" };
}
function locks(): LockManager {
  try {
    if (typeof navigator === "undefined" || typeof navigator.locks?.request !== "function") throw new Error(lockUnavailable);
    return navigator.locks;
  } catch { throw new Error(lockUnavailable); }
}
export function readRepositoryAnalysisRecovery(scope: string): RepositoryAnalysisRecovery {
  try {
    const result = readStored(scope);
    try { locks(); } catch { return { ...result, error: lockUnavailable }; }
    return result;
  } catch { return { draft: null, pending: null, error: unavailable }; }
}
function write(key: string, value: RepositoryAnalysisDraft | PendingRepositoryAnalysisRequest | null) {
  if (value === null) {
    localStorage.removeItem(key);
    if (localStorage.getItem(key) !== null) throw new Error(unavailable);
    return;
  }
  const raw = `{"version":1,"value":${encode(value)}}`;
  if (bytes(raw) > recordLimit) throw new Error(full);
  let size = bytes(key) + bytes(raw), count = 1;
  for (let i = 0; i < localStorage.length; i++) {
    const other = localStorage.key(i);
    if (other?.startsWith(prefix) && other !== key) {
      count++; size += bytes(other) + bytes(localStorage.getItem(other) ?? "");
      if (size > totalLimit || count > countLimit) throw new Error(full);
    }
  }
  if (size > totalLimit) throw new Error(full);
  localStorage.setItem(key, raw);
  if (localStorage.getItem(key) !== raw) throw new Error(unavailable);
}
async function locked<T>(scope: string, action: (key: string) => T): Promise<T> {
  const key = scopeKey(scope), manager = locks();
  let entered = false;
  try {
    // One origin-wide lock also makes the aggregate storage budget atomic.
    // All callers must await this durable reservation before sending a request.
    return await manager.request(prefix + "write", { mode: "exclusive" }, () => { entered = true; return action(key); });
  } catch (cause) {
    if (!entered) throw new Error(lockUnavailable);
    const message = cause instanceof Error ? cause.message : "";
    throw new Error([full, changedDraft, changedRequest, unavailable].includes(message) ? message : unavailable);
  }
}
export async function saveRepositoryAnalysisDraft(scope: string, expected: RepositoryAnalysisDraft | null, next: RepositoryAnalysisDraft | null): Promise<RepositoryAnalysisDraft | null> {
  if ((expected !== null && !validDraft(expected)) || (next !== null && !validDraft(next))) throw new Error(unavailable);
  const snapshot = expected === null ? null : JSON.parse(encode(expected)) as RepositoryAnalysisDraft;
  const value = next === null ? null : JSON.parse(encode(next)) as RepositoryAnalysisDraft;
  return locked(scope, key => {
    const state = readStored(scope);
    if (!equal(state.draft, snapshot)) throw new Error(changedDraft);
    write(key + "draft", value);
    return value;
  });
}
export async function retainRepositoryAnalysisRequest(scope: string, request: PendingRepositoryAnalysisRequest): Promise<PendingRepositoryAnalysisRequest> {
  if (!validRequest(request)) throw new Error(unavailable);
  const value = JSON.parse(encode(request)) as PendingRepositoryAnalysisRequest;
  return locked(scope, key => {
    const state = readStored(scope);
    if (state.pending && !equal(state.pending, value)) throw new Error(changedRequest);
    if (!state.pending) write(key + "pending", value);
    return state.pending ?? value;
  });
}
/** Clear only after a verified canonical server receipt. Missing/unknown lookup
 * results or warnings alone never establish that the paid request was not sent. */
export async function finishRepositoryAnalysisRequest(scope: string, request: PendingRepositoryAnalysisRequest): Promise<void> {
  if (!validRequest(request)) throw new Error(unavailable);
  const value = JSON.parse(encode(request)) as PendingRepositoryAnalysisRequest;
  return locked(scope, key => {
    const state = readStored(scope);
    if (state.pending && !equal(state.pending, value)) throw new Error(changedRequest);
    if (state.pending) write(key + "pending", null);
  });
}
export function isRepositoryAnalysisRecoveryKey(scope: string, key: string | null): boolean {
  const start = scopeKey(scope);
  return key === null || key === start + "draft" || key === start + "pending";
}

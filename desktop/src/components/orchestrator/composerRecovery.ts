import type { OrchestratorRequest } from "../../api/orchestrator";

// Browser-local, account/workspace/project scoped. Unsent drafts expire after
// 30 days; unresolved dispatch identities NEVER expire or become fresh drafts.
// Each request has its own immutable slot so another tab cannot overwrite it.
const prefix = "luci.orchestrator-composer.v1:";
const draftTTL = 30 * 24 * 60 * 60 * 1000;
const recordLimit = 128 * 1024, totalLimit = 2 * 1024 * 1024, recordCountLimit = 128;
export interface ComposerDraft { message: string; replyTo: string; updatedAt: number; slot: string }
export interface PendingComposerRequest { request: OrchestratorRequest; createdAt: number }
export interface ComposerRecovery { draft: ComposerDraft | null; pending: PendingComposerRequest[]; error: string }
const keyFor = (scope: string) => prefix + encodeURIComponent(scope) + ":";
const problem = "Local message recovery is unavailable. Keep a copy of your message and enable browser storage before sending.";
const text = (value: unknown, max: number) => typeof value === "string" && value.length <= max;
function validRequest(value: OrchestratorRequest): boolean {
  return !!value && Number.isSafeInteger(value.expectedRevision) && value.expectedRevision >= 0 &&
    /^[a-f0-9]{64}$/.test(value.expectedContextDigest) && /^[!-~]{16,128}$/.test(value.idempotencyKey) &&
    text(value.message, 16000) && !!value.message.trim() &&
    (value.replyToTurnId === undefined || text(value.replyToTurnId, 256)) &&
    ["codex", "claude-code", "opencode"].includes(value.harness) &&
    (value.model === undefined || text(value.model, 512)) &&
    (value.agentConnectionId === undefined || text(value.agentConnectionId, 256)) &&
    Object.keys(value).every(key => ["expectedRevision", "expectedContextDigest", "idempotencyKey", "message", "replyToTurnId", "harness", "model", "agentConnectionId"].includes(key));
}
function readDrafts(scope: string): ComposerDraft[] {
  const start = keyFor(scope) + "draft", keys: string[] = [];
  for (let i = 0; i < localStorage.length; i++) {
    const key = localStorage.key(i);
    if (key === start || key?.startsWith(start + ":")) keys.push(key);
  }
  const drafts: ComposerDraft[] = [];
  for (const key of keys.sort()) {
    const raw = localStorage.getItem(key);
    if (!raw || raw.length * 2 > recordLimit) throw new Error(problem);
    const value = JSON.parse(raw) as ComposerDraft;
    if (!value || !text(value.message, 16000) || !text(value.replyTo, 256) || !Number.isFinite(value.updatedAt)) throw new Error(problem);
    if (Date.now() - value.updatedAt > draftTTL) { localStorage.removeItem(key); continue; }
    drafts.push({ message: value.message, replyTo: value.replyTo, updatedAt: value.updatedAt, slot: key });
  }
  return drafts;
}
export function readComposerRecovery(scope: string): ComposerRecovery {
  try {
    const pending: PendingComposerRequest[] = [], start = keyFor(scope) + "send:";
    for (let i = 0; i < localStorage.length; i++) {
      const key = localStorage.key(i);
      if (!key?.startsWith(start)) continue;
      const raw = localStorage.getItem(key);
      if (!raw || raw.length * 2 > recordLimit) throw new Error(problem);
      const value = JSON.parse(raw) as PendingComposerRequest;
      if (!value || !validRequest(value.request) || !Number.isFinite(value.createdAt) || key !== start + value.request.idempotencyKey) throw new Error(problem);
      pending.push(value);
    }
    return { draft: readDrafts(scope)[0] ?? null, pending: pending.sort((a, b) => a.createdAt - b.createdAt), error: "" };
  } catch { return { draft: null, pending: [], error: problem }; }
}
function write(key: string, value: unknown) {
  const raw = JSON.stringify(value);
  let size = (raw.length + key.length) * 2, count = 1;
  for (let i = 0; i < localStorage.length; i++) {
    const existing = localStorage.key(i);
    if (existing?.startsWith(prefix) && existing !== key) { size += (existing.length + (localStorage.getItem(existing)?.length ?? 0)) * 2; count++; }
  }
  if (raw.length * 2 > recordLimit || size > totalLimit || count > recordCountLimit) throw new Error("Local message recovery is full. Resolve pending requests or discard unneeded unsent messages before sending.");
  localStorage.setItem(key, raw);
  if (localStorage.getItem(key) !== raw) throw new Error(problem);
}
export function saveComposerDraft(scope: string, expected: ComposerDraft | null, message: string, replyTo: string): ComposerDraft | null {
  const state = readComposerRecovery(scope);
  if (state.error) throw new Error(state.error);
  if (state.pending.length) throw new Error("Resolve the original request before editing this message.");
  if (JSON.stringify(state.draft) !== JSON.stringify(expected)) throw new Error("This message changed in another tab. Restore its saved version before continuing.");
  if (!text(message, 16000) || !text(replyTo, 256)) throw new Error(problem);
  const value = message || replyTo ? { message, replyTo, updatedAt: Date.now() } : null;
  try {
    const slot = expected?.slot ?? keyFor(scope) + "draft";
    if (value) write(slot, value); else localStorage.removeItem(slot);
    return value ? { ...value, slot } : null;
  } catch (cause) { throw new Error(cause instanceof Error && cause.message.includes("recovery is full") ? cause.message : problem); }
}
export function retainComposerRequest(scope: string, request: OrchestratorRequest): PendingComposerRequest {
  const state = readComposerRecovery(scope);
  if (state.error) throw new Error(state.error);
  if (state.pending.length) throw new Error("Resolve the original request before sending another message.");
  if (!validRequest(request)) throw new Error(problem);
  const value = { request, createdAt: Date.now() };
  try { write(keyFor(scope) + "send:" + request.idempotencyKey, value); }
  catch (cause) { throw new Error(cause instanceof Error && cause.message.includes("recovery is full") ? cause.message : problem); }
  return value;
}
/** Call only after verified server acknowledgment or a durable not-sent receipt. */
export function finishComposerRequest(scope: string, request: OrchestratorRequest, restoreUnsent = false): void {
  const key = keyFor(scope) + "send:" + request.idempotencyKey, raw = localStorage.getItem(key);
  if (raw) {
    const pending = JSON.parse(raw) as PendingComposerRequest;
    if (JSON.stringify(pending.request) !== JSON.stringify(request)) throw new Error(problem);
    // Remove the old draft before the identity. A failed write must leave the
    // uncertainty marker intact, never resurrect a sent message as a fresh send.
    const drafts = readDrafts(scope), matching = drafts.filter(draft => draft.message.trim() === request.message && draft.replyTo === (request.replyToTurnId ?? ""));
    if (restoreUnsent) {
      // Simultaneous tabs can produce different fenced requests. Preserve each
      // message in its own draft slot instead of overwriting the first one.
      write(matching[0]?.slot ?? keyFor(scope) + "draft:" + request.idempotencyKey, { message: request.message, replyTo: request.replyToTurnId ?? "", updatedAt: Date.now() });
    } else for (const draft of matching) localStorage.removeItem(draft.slot);
    localStorage.removeItem(key);
  }
}
export function isComposerRecoveryKey(scope: string, key: string | null) { return key === null || key.startsWith(keyFor(scope)); }

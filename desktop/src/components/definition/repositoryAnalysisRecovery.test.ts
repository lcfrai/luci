import { afterEach, beforeEach, expect, it, vi } from "vitest";
import {
  finishRepositoryAnalysisRequest, isRepositoryAnalysisRecoveryKey, readRepositoryAnalysisRecovery,
  retainRepositoryAnalysisRequest, saveRepositoryAnalysisDraft,
  type PendingRepositoryAnalysisRequest, type RepositoryAnalysisDraft,
} from "./repositoryAnalysisRecovery";

const scope = JSON.stringify(["https://luci.example", "alice", "team-a", "project-a"]);
const draft = (message = "Keep the existing checkout flow"): RepositoryAnalysisDraft => ({
  message, replyToAnalysisId: "analysis-1", sourceMode: "worktree", harness: "codex", model: "selected-model", agentConnectionId: "connection-1",
});
const request = (key = "analysis-request-key-1"): PendingRepositoryAnalysisRequest => ({
  expectedRevision: 3, expectedBriefRevision: 2, sourceMode: "commit", harness: "claude-code", model: "original-model",
  authMode: "configured", agentConnectionId: "original-connection", acknowledgeSourceRead: true,
  message: " Preserve this exact answer \n", replyToAnalysisId: "analysis-original", idempotencyKey: key,
});
function installLocks() {
  // Model browser Web Locks: concurrent callers queue, and a failure still
  // releases the lock. Each test gets a fresh origin-wide queue.
  let tail: Promise<unknown> = Promise.resolve();
  const run = vi.fn((_name: string, _options: unknown, action: () => unknown) => {
    const result = tail.then(action); tail = result.catch(() => {}); return result;
  });
  Object.defineProperty(navigator, "locks", { configurable: true, value: { request: run } });
  return run;
}
beforeEach(() => { vi.restoreAllMocks(); localStorage.clear(); installLocks(); });
afterEach(() => { vi.restoreAllMocks(); Reflect.deleteProperty(navigator, "locks"); });

it("scopes every draft and pending identity to account, workspace and project without expiring records", async () => {
  await saveRepositoryAnalysisDraft(scope, null, draft()); await retainRepositoryAnalysisRequest(scope, request());
  vi.spyOn(Date, "now").mockReturnValue(Date.now() + 10 * 365 * 86400000);
  expect(readRepositoryAnalysisRecovery(scope)).toEqual({ draft: draft(), pending: request(), error: "" });
  for (const other of [scope.replace("alice", "bob"), scope.replace("team-a", "team-b"), scope.replace("project-a", "project-b")]) {
    expect(readRepositoryAnalysisRecovery(other)).toEqual({ draft: null, pending: null, error: "" });
  }
  const keys = Object.keys(localStorage);
  expect(keys).toHaveLength(2);
  expect(keys.every(key => isRepositoryAnalysisRecoveryKey(scope, key))).toBe(true);
  expect(isRepositoryAnalysisRecoveryKey(scope, null)).toBe(true);
  expect(isRepositoryAnalysisRecoveryKey(scope, keys[0] + "-unrelated")).toBe(false);
});

it("keeps an exact pending request separate from later edits, including a changed reply target and provider", async () => {
  const original = await saveRepositoryAnalysisDraft(scope, null, draft());
  await retainRepositoryAnalysisRequest(scope, request());
  const later = { ...draft("A second thought typed while waiting"), replyToAnalysisId: "another-analysis", harness: "opencode" as const, sourceMode: "commit" as const, model: "another-model" };
  await saveRepositoryAnalysisDraft(scope, original, later);
  expect(readRepositoryAnalysisRecovery(scope)).toEqual({ draft: later, pending: request(), error: "" });
  await finishRepositoryAnalysisRequest(scope, request());
  expect(readRepositoryAnalysisRecovery(scope)).toEqual({ draft: later, pending: null, error: "" });
});

it("serializes simultaneous cross-tab draft writes so only one stale expectation can succeed", async () => {
  const original = await saveRepositoryAnalysisDraft(scope, null, draft());
  const [first, second] = await Promise.allSettled([
    saveRepositoryAnalysisDraft(scope, original, draft("Alice's update")),
    saveRepositoryAnalysisDraft(scope, original, draft("Bob's stale replacement")),
  ]);
  expect(first.status).toBe("fulfilled");
  expect(second).toMatchObject({ status: "rejected", reason: expect.objectContaining({ message: expect.stringContaining("another tab") }) });
  expect(readRepositoryAnalysisRecovery(scope).draft?.message).toBe("Alice's update");
  await expect(saveRepositoryAnalysisDraft(scope, original, null)).rejects.toThrow("another tab");
});

it("reserves only one simultaneous request and refuses to replace an identity even when just one field differs", async () => {
  const [first, second] = await Promise.allSettled([retainRepositoryAnalysisRequest(scope, request()), retainRepositoryAnalysisRequest(scope, request("other-request-key-2"))]);
  expect(first.status).toBe("fulfilled"); expect(second.status).toBe("rejected");
  await expect(retainRepositoryAnalysisRequest(scope, { ...request(), model: "changed-model" })).rejects.toThrow("original request");
  const write = vi.spyOn(Storage.prototype, "setItem");
  await expect(retainRepositoryAnalysisRequest(scope, request())).resolves.toEqual(request());
  expect(write).not.toHaveBeenCalled();
  await expect(finishRepositoryAnalysisRequest(scope, { ...request(), expectedRevision: 4 })).rejects.toThrow("original request");
  expect(readRepositoryAnalysisRecovery(scope).pending).toEqual(request());
});

it("copies input before the lock is acquired, and returns detached recovered values", async () => {
  const original = request(); const retaining = retainRepositoryAnalysisRequest(scope, original);
  original.message = "Do not change the original reserved payload";
  const retained = await retaining; expect(retained).toEqual(request());
  retained.model = "Mutated returned object";
  const read = readRepositoryAnalysisRecovery(scope); read.pending!.message = "Mutated read";
  expect(readRepositoryAnalysisRecovery(scope).pending).toEqual(request());
});

it("retains the exact consented connection revision and rejects same-key changes without changing legacy requests", async () => {
  const selected = { ...request(), expectedAgentConnectionRevision: 7 };
  await expect(retainRepositoryAnalysisRequest(scope, selected)).resolves.toEqual(selected);
  expect(readRepositoryAnalysisRecovery(scope).pending).toEqual(selected);
  await expect(retainRepositoryAnalysisRequest(scope, { ...selected, expectedAgentConnectionRevision: 8 })).rejects.toThrow("original request");
  await expect(finishRepositoryAnalysisRequest(scope, { ...selected, expectedAgentConnectionRevision: 8 })).rejects.toThrow("original request");
  await expect(finishRepositoryAnalysisRequest(scope, request())).rejects.toThrow("original request");
  await finishRepositoryAnalysisRequest(scope, selected);
  expect(readRepositoryAnalysisRecovery(scope).pending).toBeNull();
  await retainRepositoryAnalysisRequest(scope, request());
  expect(readRepositoryAnalysisRecovery(scope).pending).toEqual(request());
  expect(readRepositoryAnalysisRecovery(scope).pending).not.toHaveProperty("expectedAgentConnectionRevision");
});

it("rejects non-positive, non-integer or non-numeric connection revisions without storing a pending request", async () => {
  for (const revision of [0, -1, 1.5, Number.MAX_SAFE_INTEGER + 1, "7", null]) {
    const invalid = { ...request(), expectedAgentConnectionRevision: revision } as unknown as PendingRepositoryAnalysisRequest;
    await expect(retainRepositoryAnalysisRequest(scope, invalid)).rejects.toThrow();
    expect(readRepositoryAnalysisRecovery(scope).pending).toBeNull();
  }
  expect(localStorage.length).toBe(0);
});

it("does not erase either record when pending removal fails, and finishing never removes an unchanged draft", async () => {
  await saveRepositoryAnalysisDraft(scope, null, draft()); await retainRepositoryAnalysisRequest(scope, request());
  const remove = vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => { throw new DOMException("Denied", "SecurityError"); });
  await expect(finishRepositoryAnalysisRequest(scope, request())).rejects.toThrow("enable browser storage");
  expect(readRepositoryAnalysisRecovery(scope)).toEqual({ draft: draft(), pending: request(), error: "" });
  remove.mockRestore(); await finishRepositoryAnalysisRequest(scope, request()); await finishRepositoryAnalysisRequest(scope, request());
  expect(readRepositoryAnalysisRecovery(scope).draft).toEqual(draft());
});

it("blocks sending without Web Locks while still exposing existing data for explicit recovery", async () => {
  await saveRepositoryAnalysisDraft(scope, null, draft()); await retainRepositoryAnalysisRequest(scope, request());
  Reflect.deleteProperty(navigator, "locks");
  expect(readRepositoryAnalysisRecovery(scope)).toMatchObject({ draft: draft(), pending: request(), error: expect.stringContaining("Web Locks over HTTPS or localhost") });
  await expect(retainRepositoryAnalysisRequest(scope, request())).rejects.toThrow("Web Locks");
  await expect(saveRepositoryAnalysisDraft(scope, draft(), draft("later"))).rejects.toThrow("Web Locks");
  expect(localStorage.length).toBe(2);
});

it("fails closed on inaccessible or silently failing storage instead of claiming a durable reservation", async () => {
  const write = vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => {});
  await expect(retainRepositoryAnalysisRequest(scope, request())).rejects.toThrow("enable browser storage");
  write.mockImplementation(() => { throw new DOMException("Full", "QuotaExceededError"); });
  await expect(retainRepositoryAnalysisRequest(scope, request())).rejects.toThrow("enable browser storage");
  vi.spyOn(Storage.prototype, "getItem").mockImplementation(() => { throw new DOMException("Denied", "SecurityError"); });
  expect(readRepositoryAnalysisRecovery(scope).error).toContain("Keep this page open");
});

it("explains when browser lock access is denied instead of blaming local storage", async () => {
  Object.defineProperty(navigator, "locks", { configurable: true, get: () => { throw new DOMException("Denied", "SecurityError"); } });
  await expect(retainRepositoryAnalysisRequest(scope, request())).rejects.toThrow("Web Locks");
  Object.defineProperty(navigator, "locks", { configurable: true, value: { request: () => Promise.reject(new DOMException("Denied", "SecurityError")) } });
  await expect(retainRepositoryAnalysisRequest(scope, request())).rejects.toThrow("Web Locks");
  expect(localStorage.length).toBe(0);
});

it("preserves corrupt or oversized saved records and refuses mutation until recovery is resolved", async () => {
  await retainRepositoryAnalysisRequest(scope, request());
  const key = Object.keys(localStorage)[0];
  for (const raw of ["{broken", JSON.stringify({ version: 2, value: request() }), "x".repeat(65537)]) {
    localStorage.setItem(key, raw);
    expect(readRepositoryAnalysisRecovery(scope).error).not.toBe("");
    await expect(saveRepositoryAnalysisDraft(scope, null, draft())).rejects.toThrow();
    await expect(retainRepositoryAnalysisRequest(scope, request())).rejects.toThrow();
    expect(localStorage.getItem(key)).toBe(raw);
  }
});

it("rejects unknown credential fields, invalid identities and mismatched request shapes without storing them", async () => {
  const invalid = [
    { ...request(), apiKey: "secret-value" }, { ...request(), token: "secret-value" },
    { ...request(), idempotencyKey: "" }, { ...request(), idempotencyKey: "analysis/request:key!" },
    { ...request(), expectedRevision: -1 },
    { ...request(), expectedBriefRevision: 1.5 }, { ...request(), acknowledgeSourceRead: false },
    { ...request(), authMode: "invented-mode" }, { ...request(), model: "" },
    { ...request(), sourceMode: "unowned-path" },
  ];
  for (const value of invalid) await expect(retainRepositoryAnalysisRequest(scope, value as PendingRepositoryAnalysisRequest)).rejects.toThrow();
  await expect(saveRepositoryAnalysisDraft(scope, null, { ...draft(), password: "secret-value" } as RepositoryAnalysisDraft)).rejects.toThrow();
  await expect(retainRepositoryAnalysisRequest("", request())).rejects.toThrow("account and workspace");
  expect(localStorage.length).toBe(0);
});

it("enforces UTF-8 field and aggregate record limits without evicting older private recovery", async () => {
  await expect(saveRepositoryAnalysisDraft(scope, null, draft("😀".repeat(4001)))).rejects.toThrow();
  await expect(retainRepositoryAnalysisRequest(scope, { ...request(), message: "é".repeat(8001) })).rejects.toThrow();
  await saveRepositoryAnalysisDraft(scope, null, draft("😀".repeat(4000)));
  for (let index = 0; index < 127; index++) localStorage.setItem("luci.repository-analysis.v1:other-" + index, "retained-private-record");
  const before = Object.fromEntries(Object.entries(localStorage));
  await expect(retainRepositoryAnalysisRequest(scope, request())).rejects.toThrow("recovery is full");
  expect(Object.fromEntries(Object.entries(localStorage))).toEqual(before);
});

it("does not accept a saved request with unknown nested secret fields on read", async () => {
  await retainRepositoryAnalysisRequest(scope, request());
  const key = Object.keys(localStorage)[0], raw = JSON.stringify({ version: 1, value: { ...request(), credentials: { token: "do-not-read" } } });
  localStorage.setItem(key, raw);
  expect(readRepositoryAnalysisRecovery(scope)).toEqual({ draft: null, pending: null, error: expect.any(String) });
  await expect(finishRepositoryAnalysisRequest(scope, request())).rejects.toThrow();
  expect(localStorage.getItem(key)).toBe(raw);
});

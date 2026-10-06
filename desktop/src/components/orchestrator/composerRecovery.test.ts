import { beforeEach, expect, it, vi } from "vitest";
import type { OrchestratorRequest } from "../../api/orchestrator";
import { finishComposerRequest, readComposerRecovery, retainComposerRequest, saveComposerDraft } from "./composerRecovery";

const request = (key = "original-request-key-123"): OrchestratorRequest => ({ expectedRevision: 3, expectedContextDigest: "a".repeat(64), idempotencyKey: key, message: "My private idea", replyToTurnId: "turn-1", harness: "codex", model: "original-model", agentConnectionId: "original-connection" });
beforeEach(() => { vi.restoreAllMocks(); localStorage.clear(); });
it("scopes private drafts and retains the reply target without any credentials", () => {
  saveComposerDraft("actor/team/project-a", null, "My private idea", "turn-1");
  expect(readComposerRecovery("actor/team/project-a").draft).toMatchObject({ message: "My private idea", replyTo: "turn-1" });
  for (const scope of ["other/team/project-a", "actor/other/project-a", "actor/team/project-b"]) expect(readComposerRecovery(scope).draft).toBeNull();
});
it("expires unsent drafts after 30 days but never expires an unresolved paid request", () => {
  const now = Date.now(); vi.spyOn(Date, "now").mockReturnValue(now);
  saveComposerDraft("draft", null, "Local thought", ""); retainComposerRequest("pending", request());
  vi.mocked(Date.now).mockReturnValue(now + 31 * 86400000);
  expect(readComposerRecovery("draft").draft).toBeNull();
  expect(readComposerRecovery("pending").pending[0].request).toEqual(request());
});
it("blocks replacing a pending request, and retains every original public request field", () => {
  retainComposerRequest("scope", request());
  expect(() => retainComposerRequest("scope", request("different-request-key"))).toThrow("Resolve the original request");
  expect(() => saveComposerDraft("scope", null, "A fresh copy", "")).toThrow("Resolve the original request");
  expect(readComposerRecovery("scope").pending[0].request).toEqual(request());
});
it("does not discard identity if clearing acknowledged draft storage fails", () => {
  saveComposerDraft("scope", null, request().message, "turn-1"); retainComposerRequest("scope", request());
  vi.spyOn(Storage.prototype, "removeItem").mockImplementation(() => { throw new Error("Storage unavailable"); });
  expect(() => finishComposerRequest("scope", request())).toThrow();
  expect(readComposerRecovery("scope").pending).toHaveLength(1);
});
it("only turns a pending request back into an editable draft after a fenced not-sent receipt", () => {
  retainComposerRequest("scope", request()); finishComposerRequest("scope", request(), true);
  expect(readComposerRecovery("scope")).toMatchObject({ pending: [], draft: { message: request().message, replyTo: "turn-1" } });
});
it("rejects another tab's changed draft and fails closed on corrupt pending identities", () => {
  const old = saveComposerDraft("scope", null, "First", "");
  saveComposerDraft("scope", old, "Other tab's update", "");
  expect(() => saveComposerDraft("scope", old, "Stale overwrite", "")).toThrow("another tab");
  retainComposerRequest("scope", request());
  const key = Object.keys(localStorage).find(key => key.includes(":send:"))!;
  localStorage.setItem(key, "broken");
  expect(readComposerRecovery("scope").error).toContain("recovery is unavailable");
  expect(() => retainComposerRequest("scope", request("another-original-key"))).toThrow();
});
it("blocks dispatch before storage failure or the bounded recovery budget can lose its identity", () => {
  vi.spyOn(Storage.prototype, "setItem").mockImplementation(() => { throw new Error("Quota exceeded"); });
  expect(() => retainComposerRequest("scope", request())).toThrow("recovery is unavailable");
  vi.restoreAllMocks();
  for (let i = 0; i < 128; i++) localStorage.setItem(`luci.orchestrator-composer.v1:fixture${i}`, "x");
  expect(() => retainComposerRequest("scope", request())).toThrow("recovery is full");
});

it("keeps distinct not-sent messages when simultaneous tabs retained separate request identities", () => {
  const first = request(), second = { ...request("second-concurrent-key"), message: "Another simultaneous thought" };
  retainComposerRequest("scope", first);
  // Model both tabs passing the empty-store check before either writes: each
  // immutable key remains discoverable, rather than last-write losing one.
  const key = Object.keys(localStorage).find(key => key.includes(":send:"))!;
  localStorage.setItem(key.replace(first.idempotencyKey, second.idempotencyKey), JSON.stringify({ request: second, createdAt: Date.now() }));
  expect(readComposerRecovery("scope").pending).toHaveLength(2);
  finishComposerRequest("scope", first, true); finishComposerRequest("scope", second, true);
  const firstDraft = readComposerRecovery("scope").draft!;
  expect(firstDraft.message).toBe(first.message);
  saveComposerDraft("scope", firstDraft, "", "");
  expect(readComposerRecovery("scope").draft?.message).toBe(second.message);
  expect(readComposerRecovery("scope").pending).toEqual([]);
});

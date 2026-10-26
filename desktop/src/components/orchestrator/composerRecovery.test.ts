import { beforeEach, expect, it, vi } from "vitest";
import type { OrchestratorRequest } from "../../api/orchestrator";
import { finishComposerRequest, isComposerRecoveryKey, readComposerRecovery, retainComposerRequest, saveComposerDraft } from "./composerRecovery";
import { bindNativePreferenceIdentity } from "../../preferences/serviceIdentity";

const request = (key = "original-request-key-123"): OrchestratorRequest => ({ expectedRevision: 3, expectedContextDigest: "a".repeat(64), idempotencyKey: key, message: "My private idea", replyToTurnId: "turn-1", harness: "codex", model: "original-model", agentConnectionId: "original-connection" });
beforeEach(() => { vi.restoreAllMocks(); localStorage.clear(); bindNativePreferenceIdentity("", null); });
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

const nativeScope = (base: string) => [base, "actor-a", "local", "connection-a", "host-a", "workspace-user-a", "E:/projects-a", "project-a"];
const nativeState = "E:/luci-state-a";

it("recovers an unsent native message and its reply after the owned API port changes", () => {
  const first = "http://127.0.0.1:41001", next = "http://127.0.0.1:41002";
  bindNativePreferenceIdentity(first, nativeState);
  const saved = saveComposerDraft(JSON.stringify(nativeScope(first)), null, "Keep this idea through restart", "turn-a")!;
  bindNativePreferenceIdentity(next, nativeState);
  const scope = JSON.stringify(nativeScope(next)), recovered = readComposerRecovery(scope);
  expect(recovered).toMatchObject({ error: "", pending: [], draft: { message: saved.message, replyTo: "turn-a", slot: saved.slot } });
  expect(isComposerRecoveryKey(scope, saved.slot)).toBe(true);
  expect(saveComposerDraft(scope, recovered.draft, "Continue the same idea", "turn-a")?.slot).toBe(saved.slot);
  expect(localStorage.length).toBe(1);
});

it("retains the exact unresolved native request across ports without making it editable or creating a new identity", () => {
  const first = "http://127.0.0.1:42001", next = "http://127.0.0.1:42002";
  bindNativePreferenceIdentity(first, nativeState);
  const pending = retainComposerRequest(JSON.stringify(nativeScope(first)), request());
  const key = localStorage.key(0)!, raw = localStorage.getItem(key);
  bindNativePreferenceIdentity(next, nativeState);
  const scope = JSON.stringify(nativeScope(next));
  expect(readComposerRecovery(scope).pending).toEqual([pending]);
  expect(localStorage.getItem(key)).toBe(raw); expect(localStorage.length).toBe(1);
  expect(() => saveComposerDraft(scope, null, "Replacement", "")).toThrow("Resolve the original request");
  expect(() => retainComposerRequest(scope, request("another-request-key-123"))).toThrow("Resolve the original request");
  // Only the existing explicit fenced not-sent path changes the saved request.
  finishComposerRequest(scope, pending.request, true);
  expect(readComposerRecovery(scope)).toMatchObject({ pending: [], draft: { message: request().message, replyTo: "turn-1" } });
});

it.each([
  [0, "http://127.0.0.1:43002/v1/teams/team-b/workspace"],
  [1, "actor-b"], [2, "remote"], [3, "connection-b"], [4, "host-b"],
  [5, "workspace-user-b"], [6, "E:/projects-b"], [7, "project-b"],
] as const)("keeps native recovery isolated when scope field %i changes to %s", (field, value) => {
  const first = "http://127.0.0.1:43001", next = "http://127.0.0.1:43002";
  bindNativePreferenceIdentity(first, nativeState);
  saveComposerDraft(JSON.stringify(nativeScope(first + "/v1/teams/team-a/workspace")), null, "Private team idea", "turn-a");
  retainComposerRequest(JSON.stringify(nativeScope(first + "/v1/teams/team-a/workspace")), request());
  bindNativePreferenceIdentity(next, nativeState);
  const same = nativeScope(next + "/v1/teams/team-a/workspace");
  expect(readComposerRecovery(JSON.stringify(same)).pending).toHaveLength(1);
  const other = [...same]; other[field] = value;
  expect(readComposerRecovery(JSON.stringify(other))).toEqual({ draft: null, pending: [], error: "" });
});

it.each([
  ["unbound loopback", "http://127.0.0.1:44001", "http://127.0.0.1:44002", null, null],
  ["external override", "http://127.0.0.1:45001", "http://127.0.0.1:45002", nativeState, null],
  ["remote service", "https://first.example.test", "https://second.example.test", nativeState, nativeState],
  ["other loopback service", "http://127.0.0.1:46001", "http://127.0.0.1:46002", nativeState, nativeState],
  ["different native state", "http://127.0.0.1:47001", "http://127.0.0.1:47002", nativeState, "E:/luci-state-b"],
] as const)("does not join recovery across %s", (kind, first, next, firstState, nextState) => {
  const bound = kind === "other loopback service" ? "http://127.0.0.1:46999" : first;
  bindNativePreferenceIdentity(bound, firstState);
  const saved = saveComposerDraft(JSON.stringify(nativeScope(first)), null, "Private idea", "")!;
  bindNativePreferenceIdentity(kind === "other loopback service" ? bound : next, nextState);
  const scope = JSON.stringify(nativeScope(next));
  expect(readComposerRecovery(scope)).toEqual({ draft: null, pending: [], error: "" });
  expect(isComposerRecoveryKey(scope, saved.slot)).toBe(false);
});

it("does not scan or migrate an earlier transport-address bucket into native recovery", () => {
  const base = "http://127.0.0.1:48001", scope = JSON.stringify(nativeScope(base));
  const oldKey = "luci.orchestrator-composer.v1:" + encodeURIComponent(scope) + ":draft";
  const raw = JSON.stringify({ message: "Earlier transport bucket", replyTo: "", updatedAt: Date.now() });
  localStorage.setItem(oldKey, raw);
  bindNativePreferenceIdentity(base, nativeState);
  expect(readComposerRecovery(scope)).toEqual({ draft: null, pending: [], error: "" });
  expect(localStorage.getItem(oldKey)).toBe(raw); expect(localStorage.length).toBe(1);
});

it("never merges an unknown nested team route with a legitimate native team route", () => {
  const origin = "http://127.0.0.1:49001";
  bindNativePreferenceIdentity(origin, nativeState);
  const nested = JSON.stringify(nativeScope(origin + "/v1/teams/team-a/workspace/v1/teams/team-b/workspace"));
  const legitimate = JSON.stringify(nativeScope(origin + "/v1/teams/team-b/workspace"));
  const saved = saveComposerDraft(nested, null, "Private nested-route idea", "turn-a")!;
  retainComposerRequest(nested, request());
  expect(readComposerRecovery(legitimate)).toEqual({ draft: null, pending: [], error: "" });
  expect(isComposerRecoveryKey(legitimate, saved.slot)).toBe(false);
  saveComposerDraft(legitimate, null, "Legitimate team idea", "turn-b");
  expect(readComposerRecovery(nested)).toMatchObject({ draft: { message: "Private nested-route idea", replyTo: "turn-a" }, pending: [{ request: request() }] });
  expect(readComposerRecovery(legitimate)).toMatchObject({ draft: { message: "Legitimate team idea", replyTo: "turn-b" }, pending: [] });
});

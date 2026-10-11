import { afterEach, expect, it, vi } from "vitest";
import { coordinatorIntentsApi, type CoordinatorIntent, type CoordinatorIntentsView } from "./coordinatorIntents";
import { productApi } from "./product";

const hash = "a".repeat(64);
function intent(patch: Partial<CoordinatorIntent> = {}): CoordinatorIntent { return { id: "intent-a", turnId: "turn-a", actor: { id: "alice", displayName: "Alice" }, luciId: `luci-${hash}`, baseContextDigest: hash, proposalDigest: hash, target: "design-plan", requestedChange: "Blue buttons", changedPaths: ["palette"], requirementIds: [], taskIds: [], affectsAllTasks: true, dependsOn: [], status: "pending", createdAt: "2026-10-12T00:00:00Z", ...patch }; }
function view(patch: Partial<CoordinatorIntentsView> = {}): CoordinatorIntentsView { return { projectId: "p-1", revision: 2, contextDigest: hash, intents: [intent(), intent({ id: "intent-b", turnId: "turn-b" })], events: [], conflicts: [], allowedActions: ["propose-own-turn", "supersede-own-intent"], ...patch }; }
afterEach(() => vi.restoreAllMocks());

it("binds a display read to its project and preserves cancellation", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue(view({ projectId: "p/1" }));
  const signal = new AbortController().signal;
  expect((await coordinatorIntentsApi.get("p/1", signal)).revision).toBe(2);
  expect(request).toHaveBeenCalledExactlyOnceWith("/v1/product/projects/p%2F1/orchestrator/coordination", undefined, "GET", 120000, signal);
});
it.each([
  { projectId: "other" }, { revision: -1 }, { contextDigest: "bad" }, { intents: [intent(), intent()] },
  { intents: [intent({ actor: { id: "", displayName: "Alice" } })] }, { intents: [intent({ status: "applied" })] },
  { intents: [intent({ dependsOn: ["unknown"] })] }, { intents: [intent({ status: "superseded", supersededBy: "intent-b", resolutionReason: "" })] },
  { allowedActions: ["deploy"] }, { conflicts: [{ intentId: "foreign", reason: "opposing-intent", paths: [] }] },
  { events: [{ id: "coord-event-1", sequence: 2, kind: "proposed", intentId: "intent-a", actor: { id: "alice", displayName: "Alice" }, at: "2026-10-12T00:00:00Z", causes: [], contextDigest: hash }] },
])("rejects foreign or incomplete coordination evidence %j", async patch => {
  vi.spyOn(productApi, "request").mockResolvedValue({ ...view(), ...patch });
  await expect(coordinatorIntentsApi.get("p-1")).rejects.toThrow();
});
it("retains authoritative causal order without manufacturing accepted changes", async () => {
  const original = view({ events: [
    { id: "coord-event-1", sequence: 1, kind: "proposed", intentId: "intent-a", actor: { id: "alice", displayName: "Alice" }, at: "2026-10-12T00:00:00Z", causes: [], contextDigest: hash },
    { id: "coord-event-2", sequence: 2, previousEventId: "coord-event-1", kind: "proposed", intentId: "intent-b", actor: { id: "bob", displayName: "Bob" }, at: "2026-10-12T00:00:01Z", causes: ["intent-a"], contextDigest: hash },
  ] });
  const request = vi.spyOn(productApi, "request").mockResolvedValue(original);
  expect(await coordinatorIntentsApi.get("p-1")).toBe(original);
  request.mockResolvedValue({ ...original, events: [{ ...original.events[0] }, { ...original.events[1], previousEventId: "wrong" }] });
  await expect(coordinatorIntentsApi.get("p-1")).rejects.toThrow("causal records");
});
it("records only the exact own turn and retained dependency identities", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue(intent({ dependsOn: ["intent-b"] }));
  const input = { expectedRevision: 2, expectedContextDigest: hash, turnId: "turn-a", idempotencyKey: "exact-request-key", dependsOn: ["intent-b"] };
  await coordinatorIntentsApi.record("p-1", "alice", input);
  expect(request).toHaveBeenCalledExactlyOnceWith("/v1/product/projects/p-1/orchestrator/coordination/intents", input);
  request.mockResolvedValue(intent({ actor: { id: "bob", displayName: "Bob" }, dependsOn: ["intent-b"] }));
  await expect(coordinatorIntentsApi.record("p-1", "alice", input)).rejects.toThrow("person's proposal");
});
it("binds a resolution to the exact own intent, replacement, reason and revision", async () => {
  const input = { expectedRevision: 2, expectedContextDigest: hash, supersededBy: "intent-b", reason: "Follow the agreed accessible palette." };
  const response = intent({ status: "superseded", supersededBy: input.supersededBy, resolutionReason: input.reason });
  const request = vi.spyOn(productApi, "request").mockResolvedValue(response);
  expect(await coordinatorIntentsApi.resolve("p-1", "alice", "intent-a", input)).toEqual(response);
  expect(request).toHaveBeenCalledExactlyOnceWith("/v1/product/projects/p-1/orchestrator/coordination/intents/intent-a/resolve", input);
  for (const patch of [{ id: "intent-other" }, { actor: { id: "bob", displayName: "Bob" } }, { status: "pending" }, { supersededBy: "intent-other" }, { resolutionReason: "A different decision" }]) {
    request.mockResolvedValue({ ...response, ...patch });
    await expect(coordinatorIntentsApi.resolve("p-1", "alice", "intent-a", input)).rejects.toThrow();
  }
});

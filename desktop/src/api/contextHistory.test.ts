import { afterEach, expect, it, vi } from "vitest";
import { productApi } from "./product";
import { contextHistoryApi, type ContextChangeEvent } from "./contextHistory";

afterEach(() => vi.restoreAllMocks());
const event: ContextChangeEvent = {
  id: `context-${"a".repeat(24)}`, projectId: "p-1", kind: "specification", action: "save", revision: 2,
  digest: "b".repeat(64), at: "2026-10-06T13:15:00Z", actor: { id: "person-alice", displayName: "Alice" }, status: "applied",
};

it("reads authenticated context history through the selected product route without a mutation payload", async () => {
  const result = { events: [event], pruned: false };
  const request = vi.spyOn(productApi, "request").mockResolvedValue(result);
  await expect(contextHistoryApi.list("p-1")).resolves.toEqual(result);
  expect(request).toHaveBeenCalledExactlyOnceWith("/v1/product/projects/p-1/context-history");
  request.mockResolvedValue({ events: [], pruned: false });
  await contextHistoryApi.list("p 2");
  expect(request.mock.lastCall).toEqual(["/v1/product/projects/p%202/context-history"]);
});

it.each([
  { events: [{ ...event, projectId: "p-foreign" }], pruned: false },
  { events: [event, event], pruned: false },
  { events: [{ ...event, actor: { id: "person-alice", displayName: "" } }], pruned: false },
  { events: [{ ...event, status: "approved" }], pruned: false },
  { events: [{ ...event, revision: 0 }], pruned: false },
  { events: [{ ...event, digest: "unchecked" }], pruned: false },
  { events: [{ ...event, at: "tomorrow" }], pruned: false },
  { events: [{ ...event, kind: "release" }], pruned: false },
  { events: [event] },
  { events: Array.from({ length: 2001 }, (_, index) => ({ ...event, id: `context-${index.toString(16).padStart(24, "0")}` })), pruned: true },
])("rejects unverifiable or foreign attribution", async result => {
  vi.spyOn(productApi, "request").mockResolvedValue(result);
  await expect(contextHistoryApi.list("p-1")).rejects.toThrow("could not be verified for this project");
});

it("preserves a server error without retrying or inventing empty history", async () => {
  const request = vi.spyOn(productApi, "request").mockRejectedValue(new Error("Private context attribution could not be verified."));
  await expect(contextHistoryApi.list("p-1")).rejects.toThrow("Private context attribution could not be verified.");
  expect(request).toHaveBeenCalledTimes(1);
});

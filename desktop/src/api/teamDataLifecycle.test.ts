import { afterEach, expect, it, vi } from "vitest";
import { productApi } from "./product";
import { teamDataLifecycleApi } from "./teamDataLifecycle";
const requestId = "delete-ABCDEFGHIJKLMNOPQRSTUV";
const team = { id: "team-one", name: "One", revision: 3 };
const pending = { revision: 1, status: "requested", requestId, requestedBy: "user-owner", requestedAt: "2026-10-06T01:00:00Z", updatedAt: "2026-10-06T01:00:00Z" };
const cancelled = { ...pending, revision: 2, status: "cancelled", cancelledBy: "user-owner", cancelledAt: "2026-10-06T02:00:00Z", updatedAt: "2026-10-06T02:00:00Z" };
afterEach(() => vi.restoreAllMocks());
it("sends explicit request acknowledgement and exact cancellation identity without client actor claims", async () => {
  const call = vi.spyOn(productApi, "request").mockResolvedValueOnce({ team, dataLifecycle: { status: "none", revision: 0 } }).mockResolvedValueOnce({ team, dataLifecycle: pending }).mockResolvedValueOnce({ team, dataLifecycle: cancelled });
  await teamDataLifecycleApi.get(team.id); await teamDataLifecycleApi.request(team.id, 2); await teamDataLifecycleApi.cancel(team.id, 3, requestId);
  expect(call.mock.calls).toEqual([
    ["/v1/teams/team-one/data-lifecycle"],
    ["/v1/teams/team-one/data-lifecycle/request", { expectedRevision: 2, acknowledgeDeletion: true }],
    ["/v1/teams/team-one/data-lifecycle/cancel", { expectedRevision: 3, requestId }],
  ]);
});
it.each([
  { team: { ...team, id: "team-other" }, dataLifecycle: pending },
  { team, dataLifecycle: { ...pending, status: "completed" } },
  { team, dataLifecycle: { ...pending, status: "none" } },
  { team, dataLifecycle: { ...pending, requestedBy: undefined } },
  { team, dataLifecycle: { ...pending, requestId: "client-invented" } },
  { team, dataLifecycle: { ...pending, cancelledBy: "other" } },
  { team, dataLifecycle: { ...cancelled, cancelledAt: undefined } },
])("refuses foreign, incomplete or unproven completion receipts", async value => {
  vi.spyOn(productApi, "request").mockResolvedValue(value);
  await expect(teamDataLifecycleApi.get(team.id)).rejects.toThrow("could not be verified");
});
it("does not claim cancellation of a different pending request", async () => {
  vi.spyOn(productApi, "request").mockResolvedValue({ team, dataLifecycle: { ...cancelled, requestId: "delete-ZBCDEFGHIJKLMNOPQRSTUV" } });
  await expect(teamDataLifecycleApi.cancel(team.id, 3, requestId)).rejects.toThrow("Cancellation was not confirmed");
});

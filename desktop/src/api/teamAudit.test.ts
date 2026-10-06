import { afterEach, expect, it, vi } from "vitest";
import { productApi } from "./product";
import { teamAuditApi } from "./teamAudit";
const event = { id: "event-one", at: "2026-10-06T00:00:00Z", actorId: "user-alice", action: "team.member.save", target: "team-one", outcome: "success", actor: { id: "user-alice", displayName: "Alice" } };
const result = { teamId: "team-one", events: [event], limit: 200, installationRetentionLimit: 1000 };
afterEach(() => vi.restoreAllMocks());
it("reads team audit without a mutation payload", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue(result);
  await expect(teamAuditApi.get("team-one")).resolves.toEqual(result);
  expect(request).toHaveBeenCalledExactlyOnceWith("/v1/teams/team-one/audit");
});
it.each([
  { ...result, teamId: "team-other" },
  { ...result, events: [{ ...event, target: "team-other" }] },
  { ...result, events: [{ ...event, action: "auth.login" }] },
  { ...result, events: [{ ...event, actor: { ...event.actor, id: "user-other" } }] },
  { ...result, events: [event, event] },
  { ...result, events: [{ ...event, actor: undefined, actorId: undefined }] },
])("rejects foreign or unverifiable management activity", async value => {
  vi.spyOn(productApi, "request").mockResolvedValue(value);
  await expect(teamAuditApi.get("team-one")).rejects.toThrow("could not be verified for the selected team");
});

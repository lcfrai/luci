import { afterEach, expect, it, vi } from "vitest";
import { productApi } from "./product";
import { emptyGrants, type Team } from "./teams";
import { teamInvitationsApi, type TeamInvitation } from "./teamInvitations";
import { rememberInvitedTeam, teamWorkspaceStorageKey } from "./teamWorkspacePreference";

const team: Team = { id: "team-aaaaaaaaaaaaaaaaaaaaaa", name: "Writers", revision: 3, createdAt: "2026-10-06T00:00:00Z", updatedAt: "2026-10-06T00:00:00Z" };
const invitation: TeamInvitation = { id: "invite-one", role: "member", createdBy: "user-alice", createdAt: "2026-10-06T00:00:00Z", expiresAt: "2026-10-08T00:00:00Z", revoked: false, team: { id: team.id, name: team.name, role: "editor", grants: { ...emptyGrants } }, status: "pending" };
afterEach(() => { vi.restoreAllMocks(); localStorage.clear(); });

it("uses management invitation endpoints with explicit revision, role and capabilities", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue({ team, invitations: [invitation] });
  await teamInvitationsApi.list(team.id);
  expect(request.mock.lastCall).toEqual([`/v1/teams/${team.id}/invitations`]);
  request.mockResolvedValue({ team, invitation, inviteCode: "c".repeat(43) });
  await teamInvitationsApi.create(team.id, 2, "editor", { ...emptyGrants, execute: true });
  expect(request.mock.lastCall).toEqual([`/v1/teams/${team.id}/invitations`, { expectedRevision: 2, role: "editor", grants: { ...emptyGrants, execute: true } }]);
  request.mockResolvedValue({ team, invitation: { ...invitation, revoked: true, status: "revoked" } });
  await teamInvitationsApi.revoke(team.id, 3, invitation.id);
  expect(request.mock.lastCall).toEqual([`/v1/teams/${team.id}/invitations/${invitation.id}`, { expectedRevision: 3 }, "DELETE"]);
  request.mockResolvedValue({ team, membership: { userId: "user-bob" }, permissions: { read: true } });
  await teamInvitationsApi.accept("private-code");
  expect(request.mock.lastCall).toEqual(["/v1/teams/invitations/accept", { inviteCode: "private-code" }]);
});

it("rejects foreign team records and ambiguous creation or revocation without replay", async () => {
  const request = vi.spyOn(productApi, "request").mockResolvedValue({ team: { ...team, id: "foreign" }, invitations: [] });
  await expect(teamInvitationsApi.list(team.id)).rejects.toThrow("another team");
  request.mockResolvedValue({ team, invitations: [{ ...invitation, team: { ...invitation.team, id: "foreign" } }] });
  await expect(teamInvitationsApi.list(team.id)).rejects.toThrow("could not be verified");
  request.mockResolvedValue({ team, invitation });
  await expect(teamInvitationsApi.create(team.id, 2, "viewer", emptyGrants)).rejects.toThrow("code was not received");
  request.mockResolvedValue({ team, invitation: { ...invitation, revoked: false } });
  await expect(teamInvitationsApi.revoke(team.id, 3, invitation.id)).rejects.toThrow("revocation could not be verified");
  expect(request).toHaveBeenCalledTimes(4);
});

it("keeps invitation arrival preferences isolated by installation and account", () => {
  const base = "https://luci.example", routed = `${base}/v1/teams/team-bbbbbbbbbbbbbbbbbbbbbb/workspace`;
  rememberInvitedTeam(routed, "alice", team.id);
  expect(localStorage.getItem(teamWorkspaceStorageKey(base, "alice"))).toBe(team.id);
  expect(localStorage.getItem(teamWorkspaceStorageKey(base, "bob"))).toBeNull();
  expect(localStorage.getItem(teamWorkspaceStorageKey("https://other.example", "alice"))).toBeNull();
  rememberInvitedTeam(base, "alice", "untrusted-selector");
  expect(localStorage.getItem(teamWorkspaceStorageKey(base, "alice"))).toBe(team.id);
});

import { StrictMode } from "react";
import { act, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, expect, it, vi } from "vitest";
import { teamInvitationsApi, type TeamInvitation, type CreatedTeamInvitation } from "../../api/teamInvitations";
import { emptyGrants, personalPermissions, type Team, type TeamDetail, type TeamMember } from "../../api/teams";
import { TeamInvitations, JoinTeamInvitation } from "./TeamInvitations";

const team: Team = { id: "team-aaaaaaaaaaaaaaaaaaaaaa", name: "Writers", revision: 2, createdAt: "2026-10-06T00:00:00Z", updatedAt: "2026-10-06T00:00:00Z" };
const membership: TeamMember = { userId: "user-alice", role: "owner", grants: { ...emptyGrants, admin: true }, revision: 1 };
const invitation: TeamInvitation = { id: "invite-one", role: "member", createdBy: membership.userId, createdAt: "2026-10-06T00:00:00Z", expiresAt: "2026-10-08T00:00:00Z", revoked: false, team: { id: team.id, name: team.name, role: "viewer", grants: { ...emptyGrants } }, status: "pending" };
const created: CreatedTeamInvitation = { team: { ...team, revision: 3 }, invitation, inviteCode: "private-one-time-invitation-code-123456789" };
const props = () => ({ team, membership, permissions: personalPermissions, onChange: vi.fn().mockResolvedValue(undefined) });
async function openInvitations() { await userEvent.click(screen.getByText(`Invite someone to ${team.name}`)); await screen.findByRole("button", { name: "Create team invitation" }); await waitFor(() => expect((screen.getByRole("button", { name: "Create team invitation" }) as HTMLButtonElement).disabled).toBe(false)); }
beforeEach(() => {
  vi.restoreAllMocks();
  vi.spyOn(teamInvitationsApi, "list").mockResolvedValue({ team, invitations: [] });
  vi.spyOn(teamInvitationsApi, "create").mockResolvedValue(created);
  vi.spyOn(teamInvitationsApi, "revoke").mockResolvedValue({ team: { ...team, revision: 4 }, invitation: { ...invitation, status: "revoked", revoked: true } });
});

it("creates explicit team capabilities and exposes a one-time code only until hidden or revoked", async () => {
  const input = props(); render(<TeamInvitations {...input} />);
  expect(teamInvitationsApi.list).not.toHaveBeenCalled();
  await openInvitations();
  await userEvent.selectOptions(screen.getByLabelText("Invited person's team role"), "editor");
  await userEvent.click(screen.getByLabelText("Run agents and checks"));
  await userEvent.click(screen.getByRole("button", { name: "Create team invitation" }));
  expect((await screen.findByLabelText("New team invitation code")).textContent).toBe(created.inviteCode);
  expect(screen.getByText(/existing accounts use Join with invitation/).textContent).toContain('managed workspace access screen');
  expect(teamInvitationsApi.create).toHaveBeenCalledExactlyOnceWith(team.id, 2, "editor", { ...emptyGrants, execute: true });
  expect(input.onChange).toHaveBeenCalledTimes(1);
  expect(JSON.stringify(localStorage)).not.toContain(created.inviteCode);
  await userEvent.click(screen.getByRole("button", { name: `Revoke team invitation ${invitation.id}` }));
  await screen.findByText("Invitation revoked. Its code can no longer grant access.");
  expect(screen.queryByLabelText("New team invitation code")).toBeNull();
  expect(teamInvitationsApi.revoke).toHaveBeenCalledExactlyOnceWith(team.id, 3, invitation.id);
});

it("offers owner invitations only to current owners and reads no invitations for nonadmins", async () => {
  const input = props(); const view = render(<TeamInvitations {...input} membership={{ ...membership, role: "editor" }} />);
  await openInvitations();
  expect(screen.queryByRole("option", { name: "Owner" })).toBeNull();
  view.rerender(<TeamInvitations {...input} permissions={{ ...personalPermissions, admin: false }} />);
  expect(screen.queryByRole("button", { name: "Create team invitation" })).toBeNull();
  expect(teamInvitationsApi.list).toHaveBeenCalledTimes(1);
});

it("requires administration on an owner invitation and never implies paid capabilities", async () => {
  render(<TeamInvitations {...props()} />); await openInvitations();
  await userEvent.selectOptions(screen.getByLabelText("Invited person's team role"), "owner");
  expect((screen.getByLabelText("Manage team access") as HTMLInputElement).checked).toBe(true);
  expect((screen.getByLabelText("Manage team access") as HTMLInputElement).disabled).toBe(true);
  await userEvent.click(screen.getByRole("button", { name: "Create team invitation" }));
  await screen.findByLabelText("New team invitation code");
  expect(teamInvitationsApi.create).toHaveBeenCalledExactlyOnceWith(team.id, 2, "owner", { ...emptyGrants, admin: true });
});

it("keeps a failed or stale request recoverable and never automatically creates a replacement", async () => {
  vi.mocked(teamInvitationsApi.create).mockRejectedValue(new Error("Team access changed. Reload the team before saving."));
  render(<TeamInvitations {...props()} />); await openInvitations();
  await userEvent.click(screen.getByRole("button", { name: "Create team invitation" }));
  await screen.findByRole("alert");
  expect(screen.queryByLabelText("New team invitation code")).toBeNull();
  expect(teamInvitationsApi.create).toHaveBeenCalledTimes(1);
  await userEvent.click(screen.getByRole("button", { name: "Refresh team invitations" }));
  await waitFor(() => expect(teamInvitationsApi.list).toHaveBeenCalledTimes(2));
  expect(teamInvitationsApi.create).toHaveBeenCalledTimes(1);
});

it("clears one-time codes immediately when the account changes and ignores late creation", async () => {
  let complete!: (value: CreatedTeamInvitation) => void;
  vi.mocked(teamInvitationsApi.create).mockReturnValue(new Promise(resolve => { complete = resolve; }));
  const input = props(); const view = render(<TeamInvitations {...input} />); await openInvitations();
  await userEvent.dblClick(screen.getByRole("button", { name: "Create team invitation" }));
  expect(teamInvitationsApi.create).toHaveBeenCalledTimes(1);
  view.rerender(<TeamInvitations {...input} membership={{ ...membership, userId: "user-bob" }} />);
  await act(async () => complete(created));
  expect(screen.queryByLabelText("New team invitation code")).toBeNull();
  expect(input.onChange).not.toHaveBeenCalled();
});

it("hides a created code without deleting or recovering it from list history", async () => {
  render(<TeamInvitations {...props()} />); await openInvitations();
  await userEvent.click(screen.getByRole("button", { name: "Create team invitation" }));
  await screen.findByLabelText("New team invitation code");
  await userEvent.click(screen.getByRole("button", { name: "Hide team invitation code" }));
  expect(screen.queryByLabelText("New team invitation code")).toBeNull();
  expect(teamInvitationsApi.revoke).not.toHaveBeenCalled();
});

it("joins an existing account once, clears its code and preserves success if workspace refresh fails", async () => {
  const accept = vi.spyOn(teamInvitationsApi, "accept").mockResolvedValue({ team, membership, permissions: personalPermissions } as TeamDetail);
  const onJoined = vi.fn().mockRejectedValue(new Error("Connection interrupted while refreshing workspaces"));
  render(<StrictMode><JoinTeamInvitation onJoined={onJoined} /></StrictMode>);
  await userEvent.type(screen.getByLabelText("Team invitation code"), "private-code");
  await userEvent.dblClick(screen.getByRole("button", { name: "Join team" }));
  await screen.findByText(/You joined Writers/);
  expect(accept).toHaveBeenCalledExactlyOnceWith("private-code");
  expect(onJoined).toHaveBeenCalledExactlyOnceWith(team);
  expect(screen.queryByLabelText("Team invitation code")).toBeNull();
  expect((await screen.findByRole("alert")).textContent).toContain("refreshing workspaces");
});

it("retains an unsuccessful join code for correction without claiming membership", async () => {
  vi.spyOn(teamInvitationsApi, "accept").mockRejectedValue(new Error("This team invitation is expired or revoked."));
  const onJoined = vi.fn(); render(<JoinTeamInvitation onJoined={onJoined} />);
  await userEvent.type(screen.getByLabelText("Team invitation code"), "old-code");
  await userEvent.click(screen.getByRole("button", { name: "Join team" }));
  await screen.findByRole("alert");
  expect((screen.getByLabelText("Team invitation code") as HTMLInputElement).value).toBe("old-code");
  expect(onJoined).not.toHaveBeenCalled();
});

it("clears the code and ignores acceptance when the required managed team changes", async () => {
  let complete!: (value: TeamDetail) => void;
  vi.spyOn(teamInvitationsApi, 'accept').mockReturnValue(new Promise(resolve => { complete = resolve; }));
  const onJoined = vi.fn();
  const view = render(<JoinTeamInvitation expectedTeamId={team.id} onJoined={onJoined} />);
  await userEvent.type(screen.getByLabelText('Team invitation code'), 'old-binding-code');
  await userEvent.click(screen.getByRole('button', { name: 'Join team' }));
  view.rerender(<JoinTeamInvitation expectedTeamId="team-bbbbbbbbbbbbbbbbbbbbbb" onJoined={onJoined} />);
  expect(screen.getByLabelText('Team invitation code')).toHaveProperty('value', '');
  await act(async () => complete({ team, membership, permissions: personalPermissions, members: [], executionBoundary: 'dedicated-team' }));
  expect(onJoined).not.toHaveBeenCalled();
  expect(screen.queryByText(/You joined/)).toBeNull();
});

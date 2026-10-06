import { productApi } from "./product";
import type { Team, TeamDetail, TeamGrants, TeamMember } from "./teams";

export interface TeamInvitation {
  id: string;
  role: "member";
  createdBy: string;
  createdAt: string;
  expiresAt: string;
  revoked: boolean;
  acceptedAt?: string;
  acceptedBy?: string;
  team: { id: string; name: string; role: TeamMember["role"]; grants: TeamGrants };
  status: "pending" | "accepted" | "revoked" | "expired" | "unavailable";
}
export interface TeamInvitationList { team: Team; invitations: TeamInvitation[] }
export interface TeamInvitationResult { team: Team; invitation: TeamInvitation }
export interface CreatedTeamInvitation extends TeamInvitationResult { inviteCode: string }
const path = (id: string) => `/v1/teams/${encodeURIComponent(id)}/invitations`;
function verifyTeam<T extends { team: Team }>(result: T, teamId: string): T {
  if (!result?.team || result.team.id !== teamId || !Number.isSafeInteger(result.team.revision) || result.team.revision < 1)
    throw new Error("This invitation response belongs to another team. Refresh team access before continuing.");
  return result;
}
function verifyInvitation(value: TeamInvitation, teamId: string): void {
  if (!value || value.role !== "member" || value.team?.id !== teamId || !["owner", "editor", "viewer"].includes(value.team.role) ||
      !value.team.grants || !["execute", "secrets", "deploy", "admin"].every(key => typeof value.team.grants[key as keyof TeamGrants] === "boolean") ||
      !["pending", "accepted", "revoked", "expired", "unavailable"].includes(value.status) ||
      !Number.isFinite(Date.parse(value.expiresAt)) || !Number.isFinite(Date.parse(value.createdAt)) || typeof value.id !== "string")
    throw new Error("This team invitation could not be verified. Refresh team access before continuing.");
}
export const teamInvitationsApi = {
  async list(teamId: string): Promise<TeamInvitationList> {
    const value = verifyTeam(await productApi.request<TeamInvitationList>(path(teamId)), teamId);
    if (!Array.isArray(value.invitations) || value.invitations.length > 1000) throw new Error("Team invitations could not be verified.");
    value.invitations.forEach(row => verifyInvitation(row, teamId));
    return value;
  },
  async create(teamId: string, expectedRevision: number, role: TeamMember["role"], grants: TeamGrants): Promise<CreatedTeamInvitation> {
    const value = verifyTeam(await productApi.request<CreatedTeamInvitation>(path(teamId), { expectedRevision, role, grants }), teamId);
    verifyInvitation(value.invitation, teamId);
    if (typeof value.inviteCode !== "string" || value.inviteCode.length < 32 || value.inviteCode.length > 256)
      throw new Error("The invitation code was not received. Refresh and revoke the invitation before creating a replacement.");
    return value;
  },
  async revoke(teamId: string, expectedRevision: number, id: string): Promise<TeamInvitationResult> {
    const value = verifyTeam(await productApi.request<TeamInvitationResult>(`${path(teamId)}/${encodeURIComponent(id)}`, { expectedRevision }, "DELETE"), teamId);
    verifyInvitation(value.invitation, teamId);
    if (value.invitation.id !== id || !value.invitation.revoked) throw new Error("Invitation revocation could not be verified. Refresh team access.");
    return value;
  },
  async accept(inviteCode: string): Promise<TeamDetail> {
    const value = await productApi.request<TeamDetail>("/v1/teams/invitations/accept", { inviteCode });
    if (!value?.team?.id || !value.membership?.userId || !value.permissions?.read)
      throw new Error("Team membership could not be verified. Refresh your workspaces before trying again.");
    return value;
  },
};

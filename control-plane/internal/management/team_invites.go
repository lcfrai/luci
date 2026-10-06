package management

import (
	"errors"
	"net/http"
	"sort"
	"strings"
	"time"
)

// TeamInvitation pins a private membership grant. Acceptance never reads a
// team, role or capability from the new account's request or project content.
type TeamInvitation struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Role   string     `json:"role"`
	Grants TeamGrants `json:"grants"`
}
type teamInvitationView struct {
	Invitation
	Status string `json:"status"`
}

func validateTeamInvitations(state storedState) error {
	if len(state.Invitations) > 1000 {
		return errors.New("invalid invitation state")
	}
	for id, row := range state.Invitations {
		if row.Team == nil {
			if row.IssuerMembershipRevision != 0 {
				return errors.New("team invitation binding is missing")
			}
			continue
		}
		_, exists := state.Teams[row.Team.ID]
		if !exists || row.ID != id || row.Role != "member" || row.IssuerMembershipRevision < 1 ||
			!validTeamRole(row.Team.Role) || row.Team.Role == "owner" && !row.Team.Grants.Admin ||
			strings.TrimSpace(row.Team.Name) == "" || len(row.Team.Name) > 100 ||
			row.CreatedAt.IsZero() || !row.ExpiresAt.After(row.CreatedAt) || row.ExpiresAt.Sub(row.CreatedAt) > 48*time.Hour ||
			len(row.CodeHash) != 64 || row.AcceptedAt != nil && row.AcceptedBy == "" {
			return errors.New("invalid team invitation state")
		}
	}
	return nil
}

func (g *Gateway) registerTeamInvitations(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/teams/{teamId}/invitations", g.listTeamInvitations)
	mux.HandleFunc("POST /v1/teams/{teamId}/invitations", g.createTeamInvitation)
	mux.HandleFunc("DELETE /v1/teams/{teamId}/invitations/{id}", g.revokeTeamInvitation)
	mux.HandleFunc("POST /v1/teams/invitations/accept", g.acceptExistingTeamInvitation)
}

func (g *Gateway) teamInvitationIssuerCurrentLocked(row invitationRecord) bool {
	if row.Team == nil || row.Role != "member" || !validTeamRole(row.Team.Role) || row.Team.Role == "owner" && !row.Team.Grants.Admin {
		return false
	}
	access, err := g.resolveTeamAccessLocked(row.Team.ID, row.CreatedBy, "")
	return err == nil && access.Permissions.Admin && access.Member.Revision == row.IssuerMembershipRevision && (row.Team.Role != "owner" || access.Member.Role == "owner")
}
func (g *Gateway) teamInvitationViewLocked(row invitationRecord) teamInvitationView {
	status := "pending"
	switch {
	case row.AcceptedAt != nil:
		status = "accepted"
	case row.Revoked:
		status = "revoked"
	case !time.Now().Before(row.ExpiresAt):
		status = "expired"
	case !g.teamInvitationIssuerCurrentLocked(row):
		status = "unavailable"
	}
	return teamInvitationView{Invitation: row.Invitation, Status: status}
}
func (g *Gateway) listTeamInvitations(w http.ResponseWriter, r *http.Request) {
	u, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	access, err := g.resolveTeamAccessLocked(r.PathValue("teamId"), u.ID, "")
	if err != nil || !access.Permissions.Admin {
		fail(w, 403, "team_admin_required", "Only this team's administrators can manage invitations.")
		return
	}
	rows := []teamInvitationView{}
	for _, row := range g.state.Invitations {
		if row.Team != nil && row.Team.ID == access.Team.ID {
			rows = append(rows, g.teamInvitationViewLocked(row))
		}
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.After(rows[j].CreatedAt) })
	jsonReply(w, 200, map[string]any{"team": access.Team, "invitations": rows})
}
func (g *Gateway) createTeamInvitation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision int        `json:"expectedRevision"`
		Role             string     `json:"role"`
		Grants           TeamGrants `json:"grants"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !validTeamRole(req.Role) || req.Role == "owner" && !req.Grants.Admin {
		fail(w, 400, "invalid_team_role", "Choose owner, editor or viewer and explicit capabilities; owners retain administration.")
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	access, ok := g.requireTeamAdminLocked(w, r, req.ExpectedRevision)
	if !ok {
		return
	}
	if req.Role == "owner" && access.Member.Role != "owner" {
		fail(w, 403, "team_owner_required", "Only a team owner can invite another owner.")
		return
	}
	if len(g.state.Teams[access.Team.ID].Members) >= 128 {
		fail(w, 409, "team_member_limit", "This team has reached its 128-member limit.")
		return
	}
	now := time.Now().UTC()
	before := g.state
	g.state = cloneIdentityState(before)
	active := 0
	for id, row := range g.state.Invitations {
		if row.ExpiresAt.Before(now.Add(-30 * 24 * time.Hour)) {
			delete(g.state.Invitations, id)
			continue
		}
		if row.Team != nil && row.Team.ID == access.Team.ID && row.AcceptedAt == nil && !row.Revoked && now.Before(row.ExpiresAt) {
			active++
		}
	}
	if len(g.state.Invitations) >= 1000 || active >= 128 {
		g.state = before
		fail(w, 409, "invitation_limit", "Too many invitations are retained. Revoke unused invitations before creating more.")
		return
	}
	code := randomToken()
	invitation := invitationRecord{Invitation: Invitation{ID: opaqueID("invite-"), Role: "member", CreatedBy: access.Member.UserID, CreatedAt: now, ExpiresAt: now.Add(48 * time.Hour), Team: &TeamInvitation{ID: access.Team.ID, Name: access.Team.Name, Role: req.Role, Grants: req.Grants}}, CodeHash: hashToken(code), IssuerMembershipRevision: access.Member.Revision}
	g.state.Invitations[invitation.ID] = invitation
	row := g.state.Teams[access.Team.ID]
	bumpTeam(&row)
	g.state.Teams[row.Team.ID] = row
	if !g.commitTeamLocked(w, before, access.Member.UserID, "team.invitation.create", row.Team.ID) {
		return
	}
	jsonReply(w, 201, map[string]any{"team": row.Team, "invitation": g.teamInvitationViewLocked(invitation), "inviteCode": code})
}
func (g *Gateway) revokeTeamInvitation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision int `json:"expectedRevision"`
	}
	if !decode(w, r, &req) {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	access, ok := g.requireTeamAdminLocked(w, r, req.ExpectedRevision)
	if !ok {
		return
	}
	invitation, ok := g.state.Invitations[r.PathValue("id")]
	if !ok || invitation.Team == nil || invitation.Team.ID != access.Team.ID {
		fail(w, 404, "unknown_invitation", "This team invitation is unavailable.")
		return
	}
	if invitation.Team.Role == "owner" && access.Member.Role != "owner" {
		fail(w, 403, "team_owner_required", "Only an owner can manage an owner invitation.")
		return
	}
	if invitation.AcceptedAt != nil {
		fail(w, 409, "invitation_accepted", "This invitation was already accepted. Manage the person's team membership instead.")
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	invitation.Revoked = true
	g.state.Invitations[invitation.ID] = invitation
	row := g.state.Teams[access.Team.ID]
	bumpTeam(&row)
	g.state.Teams[row.Team.ID] = row
	if !g.commitTeamLocked(w, before, access.Member.UserID, "team.invitation.revoke", row.Team.ID) {
		return
	}
	jsonReply(w, 200, map[string]any{"team": row.Team, "invitation": g.teamInvitationViewLocked(invitation)})
}

// Caller has cloned state and holds g.mu; consumption, membership and session
// issuance share the existing atomic management-state transaction.
func (g *Gateway) joinInvitedTeamLocked(invitation invitationRecord, userID string) error {
	if !g.teamInvitationIssuerCurrentLocked(invitation) {
		return errors.New("This invitation's team access changed. Ask a team administrator for a new invitation.")
	}
	row := g.state.Teams[invitation.Team.ID]
	if _, exists := row.Members[userID]; exists {
		return errors.New("You already belong to this team. Ask an administrator to update your existing permissions.")
	}
	if len(row.Members) >= 128 {
		return errors.New("This team has reached its 128-member limit. Ask its owner to free a place before retrying.")
	}
	bumpTeam(&row)
	row.Members[userID] = TeamMember{UserID: userID, Role: invitation.Team.Role, Grants: invitation.Team.Grants, Revision: row.Team.Revision, JoinedAt: row.Team.UpdatedAt, UpdatedAt: row.Team.UpdatedAt}
	g.state.Teams[row.Team.ID] = row
	return validateTeams(g.state)
}
func (g *Gateway) acceptExistingTeamInvitation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InviteCode string `json:"inviteCode"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !g.allowLogin(r, "invitation:"+hashToken(req.InviteCode)) {
		fail(w, 429, "rate_limited", "Too many attempts. Try again later.")
		return
	}
	actor, _, _ := ActorFromContext(r.Context())
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now().UTC()
	invitation, ok := g.validInvitationLocked(req.InviteCode, now)
	user, present := g.state.Users[actor.ID]
	if !ok || invitation.Team == nil || !present || user.User.Disabled {
		fail(w, 403, "invalid_invitation", "This team invitation is invalid, expired, used or revoked. Ask a team administrator for a new invitation.")
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	if err := g.joinInvitedTeamLocked(invitation, actor.ID); err != nil {
		g.state = before
		fail(w, 409, "team_invitation_unavailable", err.Error())
		return
	}
	invitation.AcceptedAt = &now
	invitation.AcceptedBy = actor.ID
	g.state.Invitations[invitation.ID] = invitation
	if !g.commitTeamLocked(w, before, actor.ID, "team.invitation.accept", invitation.Team.ID) {
		return
	}
	access, _ := g.resolveTeamAccessLocked(invitation.Team.ID, actor.ID, "")
	jsonReply(w, 200, g.teamViewLocked(access))
}

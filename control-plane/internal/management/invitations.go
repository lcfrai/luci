package management

import (
	"net/http"
	"sort"
	"strings"
	"time"
)

type Invitation struct {
	ID         string     `json:"id"`
	Role       string     `json:"role"`
	CreatedBy  string     `json:"createdBy"`
	CreatedAt  time.Time  `json:"createdAt"`
	ExpiresAt  time.Time  `json:"expiresAt"`
	Revoked    bool       `json:"revoked"`
	AcceptedAt *time.Time `json:"acceptedAt,omitempty"`
	AcceptedBy string     `json:"acceptedBy,omitempty"`
}
type invitationRecord struct {
	Invitation
	CodeHash string `json:"codeHash"`
}

func mayInvite(issuer User, role string) bool {
	return !issuer.Disabled && validRole(role) && (issuer.Role == "super-admin" || (issuer.Role == "admin" && role == "member"))
}
func (g *Gateway) listInvitations(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r, false); !ok {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	rows := make([]Invitation, 0, len(g.state.Invitations))
	for _, row := range g.state.Invitations {
		rows = append(rows, row.Invitation)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].CreatedAt.After(rows[j].CreatedAt) })
	jsonReply(w, 200, map[string]any{"invitations": rows})
}
func (g *Gateway) createInvitation(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdmin(w, r, false)
	if !ok {
		return
	}
	var req struct {
		Role string `json:"role"`
	}
	if !decode(w, r, &req) {
		return
	}
	if req.Role == "" {
		req.Role = "member"
	}
	if !mayInvite(actor, req.Role) {
		fail(w, 403, "permission_denied", "Administrators may invite members; only Super Admins may invite administrators.")
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	current, ok := g.state.Users[actor.ID]
	if !ok || !mayInvite(current.User, req.Role) {
		fail(w, 403, "identity_changed", "Your account no longer permits this invitation.")
		return
	}
	now := time.Now().UTC()
	before := g.state
	g.state = cloneIdentityState(before)
	for id, row := range g.state.Invitations {
		if row.ExpiresAt.Before(now.Add(-30 * 24 * time.Hour)) {
			delete(g.state.Invitations, id)
		}
	}
	if len(g.state.Invitations) >= 1000 {
		g.state = before
		fail(w, 409, "invitation_limit", "This instance has too many retained invitations.")
		return
	}
	code := randomToken()
	row := invitationRecord{Invitation: Invitation{ID: opaqueID("invite-"), Role: req.Role, CreatedBy: actor.ID, CreatedAt: now, ExpiresAt: now.Add(48 * time.Hour)}, CodeHash: hashToken(code)}
	g.state.Invitations[row.ID] = row
	g.auditLocked(actor.ID, "invitation.create", row.ID, "succeeded")
	if g.persistLocked() != nil {
		g.state = before
		fail(w, 500, "save_failed", "The invitation could not be saved.")
		return
	}
	jsonReply(w, 201, map[string]any{"invitation": row.Invitation, "inviteCode": code})
}
func (g *Gateway) revokeInvitation(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdmin(w, r, false)
	if !ok {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	row, ok := g.state.Invitations[r.PathValue("id")]
	if !ok {
		fail(w, 404, "unknown_invitation", "This invitation does not exist.")
		return
	}
	issuer, ok := g.state.Users[actor.ID]
	if !ok || !mayInvite(issuer.User, row.Role) {
		fail(w, 403, "permission_denied", "You cannot manage this invitation's role.")
		return
	}
	before := g.state
	g.state = cloneIdentityState(before)
	row.Revoked = true
	g.state.Invitations[row.ID] = row
	g.auditLocked(actor.ID, "invitation.revoke", row.ID, "succeeded")
	if g.persistLocked() != nil {
		g.state = before
		fail(w, 500, "save_failed", "The invitation could not be revoked.")
		return
	}
	jsonReply(w, 200, map[string]bool{"revoked": true})
}
func (g *Gateway) validInvitationLocked(code string, now time.Time) (invitationRecord, bool) {
	if len(code) < 32 || len(code) > 256 {
		return invitationRecord{}, false
	}
	hashed := hashToken(code)
	for _, row := range g.state.Invitations {
		if row.CodeHash != hashed || row.Revoked || row.AcceptedAt != nil || !now.Before(row.ExpiresAt) {
			continue
		}
		issuer, ok := g.state.Users[row.CreatedBy]
		return row, ok && mayInvite(issuer.User, row.Role)
	}
	return invitationRecord{}, false
}
func (g *Gateway) acceptInvitation(w http.ResponseWriter, r *http.Request) {
	var req struct {
		InviteCode  string `json:"inviteCode"`
		Username    string `json:"username"`
		DisplayName string `json:"displayName"`
		Password    string `json:"password"`
	}
	if !decode(w, r, &req) {
		return
	}
	if !g.allowLogin(r, "invitation:"+hashToken(req.InviteCode)) {
		fail(w, 429, "rate_limited", "Too many attempts. Try again later.")
		return
	}
	if !validIdentity(req.Username, req.DisplayName, req.Password) {
		fail(w, 400, "invalid_identity", "Use a valid username, display name, and password of 12–1,024 characters.")
		return
	}
	g.mu.Lock()
	_, ok := g.validInvitationLocked(req.InviteCode, time.Now())
	g.mu.Unlock()
	if !ok {
		fail(w, 403, "invalid_invitation", "This invitation is invalid, expired, used or revoked. Ask an administrator for a new invitation.")
		return
	}
	if !g.passwordSlot(w) {
		return
	}
	defer g.releasePasswordSlot()
	hash := passwordHash(req.Password)
	g.mu.Lock()
	defer g.mu.Unlock()
	now := time.Now().UTC()
	invitation, ok := g.validInvitationLocked(req.InviteCode, now)
	if !ok {
		fail(w, 403, "invalid_invitation", "This invitation is invalid, expired, used or revoked. Ask an administrator for a new invitation.")
		return
	}
	if len(g.state.Users) >= 1000 {
		fail(w, 409, "user_limit", "This instance supports at most 1,000 users.")
		return
	}
	for _, row := range g.state.Users {
		if strings.EqualFold(row.User.Username, req.Username) {
			fail(w, 409, "username_exists", "Choose another username.")
			return
		}
	}
	before := g.state
	g.state = cloneIdentityState(before)
	u := User{ID: opaqueID("user-"), Username: strings.ToLower(req.Username), DisplayName: strings.TrimSpace(req.DisplayName), Role: invitation.Role}
	g.state.Users[u.ID] = userRecord{User: u, PasswordHash: hash}
	invitation.AcceptedAt = &now
	invitation.AcceptedBy = u.ID
	g.state.Invitations[invitation.ID] = invitation
	g.auditLocked(u.ID, "invitation.accept", invitation.ID, "succeeded")
	token, err := g.issueSessionLocked(u)
	if err != nil {
		g.state = before
		fail(w, 500, "save_failed", "The account could not be saved. Your invitation remains available.")
		return
	}
	jsonReply(w, 201, map[string]any{"token": token, "user": u})
}

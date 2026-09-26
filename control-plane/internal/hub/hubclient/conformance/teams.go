package conformance

import (
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// The teams group: server tasks 15 and 16a (contract §7.2, §7.3). The rows
// after the part's four are carried/task-05.md items 4, 5, 8-10, 12 and 13.
var teamRows = []row{
	{name: "me_binds_invites", fn: meBindsInvites},
	{name: "outsider_sees_404", fn: outsiderSees404},
	{name: "member_cannot_invite", fn: memberCannotInvite},
	{name: "last_admin", fn: lastAdmin},
	{name: "revoked_invite_never_binds", fn: revokedInviteNeverBinds},
	{name: "reinvite_after_revoke", fn: reinviteAfterRevoke},
	{name: "pending_is_per_team", fn: pendingIsPerTeam},
	{name: "me_binds_every_pending", fn: meBindsEveryPending},
	{name: "member_check_is_this_team", fn: memberCheckIsThisTeam},
	{name: "revoke_is_team_scoped", fn: revokeIsTeamScoped},
	{name: "me_binds_only_own_email", fn: meBindsOnlyOwnEmail},
	{name: "member_cannot_revoke", fn: func(t *testing.T, e Env) { cannotRevoke(t, e, e.Member) }},
	{name: "viewer_cannot_revoke", fn: func(t *testing.T, e Env) { cannotRevoke(t, e, e.Viewer) }},
	{name: "revoke_twice_is_204", fn: revokeTwiceIs204},
	{name: "me_raises_a_lower_role", fn: meRaisesALowerRole},
	{name: "org_admin_is_a_non_member", fn: orgAdminTeams},
}

// invite is T's admin inviting email as role.
func invite(t *testing.T, e Env, email string, role hubapi.Role) hubapi.Invite {
	t.Helper()
	inv := must(e.Admin.CreateInvite(bg, e.TeamID, hubapi.CreateInviteRequest{Email: email, Role: role}))(t)
	if inv.State != hubapi.InvitePending || inv.Email != email || inv.Role != role {
		t.Fatalf("CreateInvite = %+v, want a pending %s invite for %s", inv, role, email)
	}
	return inv
}

// a fresh invite binds on the invitee's Me, once.
func meBindsInvites(t *testing.T, e Env) {
	out := meOf(t, e.Outsider).User
	inv := invite(t, e, out.Email, hubapi.RoleMember)
	me := meOf(t, e.Outsider)
	if me.InvitesBound != 1 {
		t.Errorf("the invitee's Me bound %d invites, want 1", me.InvitesBound)
	}
	if tm, in := teamIn(me.Teams, e.TeamID); !in || tm.Role != hubapi.RoleMember {
		t.Errorf("after binding, T in the invitee's teams = %+v (%v), want role member", tm, in)
	}
	if again := meOf(t, e.Outsider); again.InvitesBound != 0 {
		t.Errorf("a second Me bound %d invites, want 0: an invite binds once", again.InvitesBound)
	}
	got := inviteOf(t, e.Admin, e.TeamID, inv.ID)
	if got.State != hubapi.InviteBound || got.BoundUser == nil || got.BoundUser.ID != out.ID {
		t.Errorf("the invite after binding = %+v, want bound to %s", got, out.ID)
	}
}

// Outsider.Team(T) → 404 team_not_found.
func outsiderSees404(t *testing.T, e Env) {
	_, err := e.Outsider.Team(bg, e.TeamID)
	wantErr(t, err, 404, hubapi.CodeTeamNotFound, "teamId", e.TeamID)
}

// Member.CreateInvite → 403 forbidden, required=admin; and still 403, never
// 409, for an email with a pending invite to T (task-04-review-2.md m2).
func memberCannotInvite(t *testing.T, e Env) {
	_, err := e.Member.CreateInvite(bg, e.TeamID, hubapi.CreateInviteRequest{Email: freshEmail(), Role: hubapi.RoleViewer})
	wantErr(t, err, 403, hubapi.CodeForbidden, "required", "admin")
	pending := invite(t, e, freshEmail(), hubapi.RoleViewer)
	_, err = e.Member.CreateInvite(bg, e.TeamID, hubapi.CreateInviteRequest{Email: pending.Email, Role: hubapi.RoleViewer})
	wantErr(t, err, 403, hubapi.CodeForbidden, "required", "admin")
	inviteOf(t, e.Admin, e.TeamID, pending.ID) // still exactly one
}

// Admin.RemoveMember(T, admin) → 409 last_admin; so is demoting the last admin.
func lastAdmin(t *testing.T, e Env) {
	admin := meOf(t, e.Admin).User.ID
	wantErr(t, e.Admin.RemoveMember(bg, e.TeamID, admin), 409, hubapi.CodeLastAdmin)
	_, err := e.Admin.SetMember(bg, e.TeamID, admin, hubapi.RoleMember)
	wantErr(t, err, 409, hubapi.CodeLastAdmin)
	if tm, _ := teamIn(meOf(t, e.Admin).Teams, e.TeamID); tm.Role != hubapi.RoleAdmin {
		t.Errorf("after both refusals the admin's role is %q, want admin", tm.Role)
	}
}

// revoke is T's admin revoking inviteID, which must succeed.
func revoke(t *testing.T, e Env, inviteID string) {
	t.Helper()
	ok(t, e.Admin.RevokeInvite(bg, e.TeamID, inviteID))
}

// wantPending is T's invite inviteID, still pending and bound to nobody.
func wantPending(t *testing.T, e Env, inviteID string) {
	t.Helper()
	if got := inviteOf(t, e.Admin, e.TeamID, inviteID); got.State != hubapi.InvitePending || got.BoundUser != nil {
		t.Errorf("invite %s = %s bound to %v, want still pending and bound to nobody", inviteID, got.State, got.BoundUser)
	}
}

// notInTeam asserts h's Me binds nothing and does not list teamID, and h's
// Team(teamID) is 404.
func notInTeam(t *testing.T, h hubclient.Hub, teamID string) {
	t.Helper()
	me := meOf(t, h)
	if me.InvitesBound != 0 {
		t.Errorf("Me bound %d invites, want 0", me.InvitesBound)
	}
	if _, in := teamIn(me.Teams, teamID); in {
		t.Errorf("Me lists team %s, which this caller is not in", teamID)
	}
	_, err := h.Team(bg, teamID)
	wantErr(t, err, 404, hubapi.CodeTeamNotFound, "teamId", teamID)
}

// carried item 4: a revoked invite never binds, and the row stays, revoked.
func revokedInviteNeverBinds(t *testing.T, e Env) {
	inv := invite(t, e, meOf(t, e.Outsider).User.Email, hubapi.RoleAdmin)
	revoke(t, e, inv.ID)
	notInTeam(t, e.Outsider, e.TeamID)
	if got := inviteOf(t, e.Admin, e.TeamID, inv.ID); got.State != hubapi.InviteRevoked || got.BoundUser != nil {
		t.Errorf("the revoked invite = %s bound to %v, want revoked, bound to nobody", got.State, got.BoundUser)
	}
}

// carried item 4: the same email again after a revoke is a new pending invite.
func reinviteAfterRevoke(t *testing.T, e Env) {
	email := freshEmail()
	first := invite(t, e, email, hubapi.RoleMember)
	revoke(t, e, first.ID)
	again := invite(t, e, email, hubapi.RoleMember)
	if again.ID == first.ID {
		t.Errorf("the re-invite reused the revoked invite's id %s", first.ID)
	}
	wantPending(t, e, again.ID)
}

// carried item 4: a pending invite in T does not block one to another team.
func pendingIsPerTeam(t *testing.T, e Env) {
	email := freshEmail()
	invite(t, e, email, hubapi.RoleMember)
	u := newTeam(t, e.Admin)
	got := must(e.Admin.CreateInvite(bg, u.ID, hubapi.CreateInviteRequest{Email: email, Role: hubapi.RoleMember}))(t)
	if got.State != hubapi.InvitePending || got.TeamID != u.ID {
		t.Errorf("the second team's invite = %+v, want pending in %s", got, u.ID)
	}
}

// carried item 4: one Me binds every pending invite for the caller's email,
// each with its own role.
func meBindsEveryPending(t *testing.T, e Env) {
	email := meOf(t, e.Outsider).User.Email
	invite(t, e, email, hubapi.RoleMember)
	u := newTeam(t, e.Admin)
	must(e.Admin.CreateInvite(bg, u.ID, hubapi.CreateInviteRequest{Email: email, Role: hubapi.RoleViewer}))(t)
	me := meOf(t, e.Outsider)
	if me.InvitesBound != 2 {
		t.Errorf("one Me bound %d invites, want 2", me.InvitesBound)
	}
	for id, role := range map[string]hubapi.Role{e.TeamID: hubapi.RoleMember, u.ID: hubapi.RoleViewer} {
		if tm, in := teamIn(me.Teams, id); !in || tm.Role != role {
			t.Errorf("team %s in Me = %+v (%v), want role %s", id, tm, in, role)
		}
	}
}

// carried item 4: the "already a member" check is this team's, not the org's.
func memberCheckIsThisTeam(t *testing.T, e Env) {
	u := newTeam(t, e.Admin)
	email := meOf(t, e.Member).User.Email
	got := must(e.Admin.CreateInvite(bg, u.ID, hubapi.CreateInviteRequest{Email: email, Role: hubapi.RoleMember}))(t)
	if got.State != hubapi.InvitePending {
		t.Errorf("T's member invited to another team = %+v, want pending", got)
	}
	ok(t, e.Admin.RevokeInvite(bg, u.ID, got.ID)) // leave nothing for the member's next Me
}

// carried item 5: revoking names the team; another team's invite is 404.
func revokeIsTeamScoped(t *testing.T, e Env) {
	u := newTeam(t, e.Member) // the member is U's admin; T's admin is not in U
	other := must(e.Member.CreateInvite(bg, u.ID, hubapi.CreateInviteRequest{Email: freshEmail(), Role: hubapi.RoleAdmin}))(t)
	wantErr(t, e.Admin.RevokeInvite(bg, e.TeamID, other.ID), 404, hubapi.CodeInviteNotFound, "inviteId", other.ID)
	if got := inviteOf(t, e.Member, u.ID, other.ID); got.State != hubapi.InvitePending {
		t.Errorf("U's invite after a revoke through T = %s, want pending", got.State)
	}
}

// carried item 8: Me binds only an invite to the caller's own email.
func meBindsOnlyOwnEmail(t *testing.T, e Env) {
	inv := invite(t, e, freshEmail(), hubapi.RoleAdmin)
	notInTeam(t, e.Outsider, e.TeamID)
	me := meOf(t, e.Member)
	if me.InvitesBound != 0 {
		t.Errorf("the member's Me bound %d invites, want 0", me.InvitesBound)
	}
	if tm, _ := teamIn(me.Teams, e.TeamID); tm.Role != hubapi.RoleMember {
		t.Errorf("the member's role in T = %q, want member", tm.Role)
	}
	wantPending(t, e, inv.ID)
}

// carried item 9: only an admin revokes, and the role check comes before the
// lookup: a missing and a bound invite are 403 too.
func cannotRevoke(t *testing.T, e Env, h hubclient.Hub) {
	pending := invite(t, e, freshEmail(), hubapi.RoleViewer)
	bound := invite(t, e, meOf(t, e.Outsider).User.Email, hubapi.RoleViewer)
	meOf(t, e.Outsider) // binds it
	for _, id := range []string{pending.ID, "iv-0000000000000000", bound.ID} {
		wantErr(t, h.RevokeInvite(bg, e.TeamID, id), 403, hubapi.CodeForbidden, "required", "admin")
	}
	wantPending(t, e, pending.ID)
	if got := inviteOf(t, e.Admin, e.TeamID, bound.ID); got.State != hubapi.InviteBound {
		t.Errorf("the bound invite after a refused revoke = %s, want bound", got.State)
	}
}

// carried item 10: a second revoke is nil, one row, one team.invite_revoked;
// the second revoke is the admin's alone.
func revokeTwiceIs204(t *testing.T, e Env) {
	before := head(t, e.Admin)
	inv := invite(t, e, freshEmail(), hubapi.RoleMember)
	revoke(t, e, inv.ID)
	revoke(t, e, inv.ID)
	wantErr(t, e.Member.RevokeInvite(bg, e.TeamID, inv.ID), 403, hubapi.CodeForbidden, "required", "admin")
	wantErr(t, e.Outsider.RevokeInvite(bg, e.TeamID, inv.ID), 404, hubapi.CodeTeamNotFound, "teamId", e.TeamID)
	if got := inviteOf(t, e.Admin, e.TeamID, inv.ID); got.State != hubapi.InviteRevoked {
		t.Errorf("the invite = %s, want revoked", got.State)
	}
	if n := count(since(t, e.Admin, before), "team.invite_revoked", inv.ID); n != 1 {
		t.Errorf("%d team.invite_revoked events for one invite revoked twice, want 1", n)
	}
}

// carried item 12: binding raises a lower role already held to the invite's.
func meRaisesALowerRole(t *testing.T, e Env) {
	out := meOf(t, e.Outsider).User
	invite(t, e, out.Email, hubapi.RoleAdmin)
	must(e.Admin.SetMember(bg, e.TeamID, out.ID, hubapi.RoleViewer))(t)
	me := meOf(t, e.Outsider)
	if me.InvitesBound != 1 {
		t.Errorf("Me bound %d invites, want 1", me.InvitesBound)
	}
	if tm, in := teamIn(me.Teams, e.TeamID); !in || tm.Role != hubapi.RoleAdmin {
		t.Errorf("T in Me = %+v (%v), want role admin: the invite's role raises a viewer's", tm, in)
	}
}

// carried item 13 (teams' half) and re-review 3 m1: an org admin in no team
// gets the outsider's answer on every team route, Me included.
func orgAdminTeams(t *testing.T, e Env) {
	pending := invite(t, e, freshEmail(), hubapi.RoleMember)
	memberID := meOf(t, e.Member).User.ID
	before := must(e.Admin.Team(bg, e.TeamID))(t)
	for _, s := range strangers(t, e) {
		t.Run(s.name, func(t *testing.T) {
			h, tid := s.hub, e.TeamID
			notFound := func(err error) { t.Helper(); wantErr(t, err, 404, hubapi.CodeTeamNotFound, "teamId", tid) }
			_, err := h.Team(bg, tid)
			notFound(err)
			_, err = h.Invites(bg, tid)
			notFound(err)
			_, err = h.CreateInvite(bg, tid, hubapi.CreateInviteRequest{Email: freshEmail(), Role: hubapi.RoleAdmin})
			notFound(err)
			notFound(h.RevokeInvite(bg, tid, pending.ID))
			_, err = h.SetMember(bg, tid, memberID, hubapi.RoleViewer)
			notFound(err)
			notFound(h.RemoveMember(bg, tid, memberID))
			_, err = h.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: tid, Name: "Intruder " + uniq()})
			notFound(err)
			if _, in := teamIn(must(h.Teams(bg))(t), tid); in {
				t.Errorf("Teams lists T to a caller in no team")
			}
			if me := meOf(t, h); me.InvitesBound != 0 {
				t.Errorf("Me bound %d invites, want 0", me.InvitesBound)
			} else if _, in := teamIn(me.Teams, tid); in {
				t.Errorf("Me.Teams lists T to a caller in no team")
			}
		})
	}
	after := must(e.Admin.Team(bg, e.TeamID))(t)
	if len(after.Members) != len(before.Members) {
		t.Errorf("T has %d members after the strangers' calls, want %d", len(after.Members), len(before.Members))
	}
	for _, m := range after.Members {
		if m.User.ID == memberID && m.Role != hubapi.RoleMember {
			t.Errorf("the member's role is %s after the strangers' calls, want member", m.Role)
		}
	}
	wantPending(t, e, pending.ID)
}

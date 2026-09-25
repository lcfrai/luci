package hubclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
)

var bg = context.Background()

// must turns (v, err) into v, failing the test on an error: must(f.X(…))(t).
func must[T any](v T, err error) func(t *testing.T) T {
	return func(t *testing.T) T {
		t.Helper()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return v
	}
}

func noErr(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// wantProblem asserts err is an *Error with this status and code, and with
// each detail named in kv ("key", "value", …). It returns the *Error.
func wantProblem(t *testing.T, err error, status int, code string, kv ...string) *Error {
	t.Helper()
	var he *Error
	if !errors.As(err, &he) {
		t.Fatalf("err = %v, want an *Error %d %s", err, status, code)
	}
	if he.Status != status || he.Problem.Error != code {
		t.Fatalf("got %d %s (%q), want %d %s", he.Status, he.Problem.Error, he.Problem.Message, status, code)
	}
	if he.Problem.Message == "" {
		t.Errorf("%s has no message", code)
	}
	for i := 0; i+1 < len(kv); i += 2 {
		if got := he.Problem.Details[kv[i]]; got != kv[i+1] {
			t.Errorf("%s details[%q] = %q, want %q (details %v)", code, kv[i], got, kv[i+1], he.Problem.Details)
		}
	}
	return he
}

// newFake is a Fake whose clock moves one millisecond per reading, so every
// time it writes is distinct and ordered.
func newFake() *Fake {
	f := NewFake()
	var mu sync.Mutex
	at := time.Date(2026, 9, 25, 1, 0, 0, 0, time.UTC)
	f.SetNow(func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		at = at.Add(time.Millisecond)
		return at
	})
	return f
}

// withTeam: Ada creates team "Portal" and Grace joins it as a member through
// an invite; Linus is in no team. It returns the three views and the team.
func withTeam(t *testing.T) (f, ada, grace, linus *Fake, team hubapi.Team) {
	t.Helper()
	f = newFake()
	ada, grace, linus = f.As(FakeAda), f.As(FakeGrace), f.As(FakeLinus)
	team = must(ada.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "Portal"}))(t)
	must(ada.CreateInvite(bg, team.ID, hubapi.CreateInviteRequest{Email: FakeGrace.Email, Role: hubapi.RoleMember}))(t)
	must(grace.Me(bg))(t)
	must(linus.Me(bg))(t) // provisioned, in no team
	return f, ada, grace, linus, team
}

func withProject(t *testing.T, view *Fake, teamID, name string, remotes ...hubapi.RemoteRequest) hubapi.Project {
	t.Helper()
	return must(view.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: teamID, Name: name, Remotes: remotes}))(t)
}

func branchBrief(t *testing.T, view *Fake, projectID, ref string) hubapi.BriefHead {
	t.Helper()
	return must(view.CreateBrief(bg, projectID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeBranch, BranchRef: ref}))(t)
}

func content(status string) json.RawMessage {
	return json.RawMessage(`{"status":"` + status + `","questions":[],"answers":[]}`)
}

// write writes one revision of briefID on base and returns it.
func write(t *testing.T, view *Fake, projectID, briefID string, base int, status string) hubapi.BriefRevision {
	t.Helper()
	got := must(view.WriteRevisions(bg, projectID, []hubapi.RevisionWrite{{BriefID: briefID, BaseRevision: base, Content: content(status)}}))(t)
	if len(got) != 1 {
		t.Fatalf("WriteRevisions answered %d revisions for one write", len(got))
	}
	return got[0]
}

func headOf(t *testing.T, view *Fake, briefID string) hubapi.BriefHead {
	t.Helper()
	return must(view.Brief(bg, briefID))(t).Head
}

func TestFakeMeProvisionsAndBindsInvites(t *testing.T) {
	f := newFake()
	ada, grace := f.As(FakeAda), f.As(FakeGrace)
	team := must(ada.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "Portal"}))(t)
	inv := must(ada.CreateInvite(bg, team.ID, hubapi.CreateInviteRequest{Email: "grace@example.test", Role: hubapi.RoleMember}))(t)
	if inv.State != hubapi.InvitePending || inv.BoundUser != nil || inv.CreatedBy.ID != FakeAda.ID || !hubapi.ValidID(hubapi.PrefixInvite, inv.ID) {
		t.Fatalf("CreateInvite = %+v, want a pending iv- invite by Ada", inv)
	}

	me := must(grace.Me(bg))(t)
	if me.InvitesBound != 1 {
		t.Errorf("first Me: invitesBound = %d, want 1", me.InvitesBound)
	}
	if me.User != (hubapi.User{ID: FakeGrace.ID, DisplayName: FakeGrace.DisplayName, Email: FakeGrace.Email}) || me.OrgAdmin {
		t.Errorf("Me.User = %+v orgAdmin %v, want Grace, not an org admin", me.User, me.OrgAdmin)
	}
	if me.Org.DisplayName != "Example Org" || me.Org.TenantID != "00000000-0000-0000-0000-00000000000e" || !hubapi.ValidID(hubapi.PrefixOrg, me.Org.ID) {
		t.Errorf("Me.Org = %+v, want the one Example Org", me.Org)
	}
	if len(me.Teams) != 1 || me.Teams[0].ID != team.ID || me.Teams[0].Role != hubapi.RoleMember {
		t.Fatalf("Me.Teams = %+v, want Portal as member", me.Teams)
	}
	again := must(grace.Me(bg))(t)
	if again.InvitesBound != 0 || len(again.Teams) != 1 || again.Teams[0].Role != hubapi.RoleMember {
		t.Errorf("second Me = bound %d, teams %+v; want 0 bound and the same one team", again.InvitesBound, again.Teams)
	}
	invites := must(ada.Invites(bg, team.ID))(t)
	if len(invites) != 1 || invites[0].State != hubapi.InviteBound || invites[0].BoundUser == nil || invites[0].BoundUser.ID != FakeGrace.ID {
		t.Errorf("Invites after binding = %+v, want one bound to Grace", invites)
	}

	// Me binds only invites for the caller's own email (§7.2: an invite "whose
	// `email` equals the caller's email"; task 04 re-review N1). Vera's admin
	// invite is pending when Linus signs in for the first time: he binds
	// nothing, joins no team and cannot see Portal, and Vera's invite is left
	// pending, bound to nobody. (It is revoked below, before Vera signs in.)
	withdrawn := must(ada.CreateInvite(bg, team.ID, hubapi.CreateInviteRequest{Email: "vera@example.test", Role: hubapi.RoleAdmin}))(t)
	linus := f.As(FakeLinus)
	fresh := must(linus.Me(bg))(t) // first sight: provisioned, no teams, not nil
	if fresh.Teams == nil || len(fresh.Teams) != 0 || fresh.InvitesBound != 0 {
		t.Errorf("Linus's first Me while Vera's admin invite is pending = %+v, want teams [] and nothing bound", fresh)
	}
	_, err := linus.Team(bg, team.ID)
	wantProblem(t, err, 404, "team_not_found", "teamId", team.ID)
	for _, inv := range must(ada.Invites(bg, team.ID))(t) {
		if inv.ID == withdrawn.ID && (inv.State != hubapi.InvitePending || inv.BoundUser != nil) {
			t.Errorf("Vera's invite after Linus's Me = %+v, want it still pending, bound to nobody", inv)
		}
	}

	// A bound invite keeps a higher role already held (contract §7.2): Linus is
	// invited as viewer, then made admin by id, then signs in.
	must(ada.CreateInvite(bg, team.ID, hubapi.CreateInviteRequest{Email: FakeLinus.Email, Role: hubapi.RoleViewer}))(t)
	must(ada.SetMember(bg, team.ID, FakeLinus.ID, hubapi.RoleAdmin))(t)
	if got := must(linus.Me(bg))(t); got.InvitesBound != 1 || len(got.Teams) != 1 || got.Teams[0].Role != hubapi.RoleAdmin {
		t.Errorf("Linus's Me = bound %d teams %+v, want 1 bound and admin kept", got.InvitesBound, got.Teams)
	}

	// A revoked invite never binds: revoking is how an admin withdraws a grant
	// before the person signs in (contract §7.2 binds `pending` invites only;
	// task 04 review I1). Vera's withdrawn admin invite leaves her in no team.
	vera := f.As(FakeUser{ID: "u-00000000000000c1", DisplayName: "Vera Example", Email: "vera@example.test"})
	noErr(t, ada.RevokeInvite(bg, team.ID, withdrawn.ID))
	if got := must(vera.Me(bg))(t); got.InvitesBound != 0 || len(got.Teams) != 0 {
		t.Errorf("Vera's Me after her invite was revoked = bound %d teams %+v, want 0 and none", got.InvitesBound, got.Teams)
	}
	_, err = vera.Team(bg, team.ID)
	wantProblem(t, err, 404, "team_not_found", "teamId", team.ID)
	var kept *hubapi.Invite
	for _, inv := range must(ada.Invites(bg, team.ID))(t) {
		if inv.ID == withdrawn.ID {
			kept = &inv
		}
	}
	if kept == nil || kept.State != hubapi.InviteRevoked || kept.BoundUser != nil {
		t.Errorf("the revoked invite after Vera's Me = %+v, want it kept, revoked, bound to nobody", kept)
	}

	// One Me binds every pending invite for the email, across teams (§7.2:
	// "every pending invite in the caller's org"; fix round 1's PS-1).
	studio := must(ada.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "Studio"}))(t)
	mo := f.As(FakeUser{ID: "u-00000000000000c2", DisplayName: "Mo Example", Email: "mo@example.test"})
	must(ada.CreateInvite(bg, team.ID, hubapi.CreateInviteRequest{Email: "mo@example.test", Role: hubapi.RoleViewer}))(t)
	must(ada.CreateInvite(bg, studio.ID, hubapi.CreateInviteRequest{Email: "mo@example.test", Role: hubapi.RoleMember}))(t)
	if got := must(mo.Me(bg))(t); got.InvitesBound != 2 || len(got.Teams) != 2 ||
		got.Teams[0].ID != team.ID || got.Teams[0].Role != hubapi.RoleViewer || got.Teams[1].ID != studio.ID || got.Teams[1].Role != hubapi.RoleMember {
		t.Errorf("Mo's Me with two pending invites = bound %d teams %+v, want 2: Portal viewer, Studio member", got.InvitesBound, got.Teams)
	}

	// A bound invite raises a lower role already held to its own (§7.2: the
	// caller "becomes a member with the invite's role — or keeps a higher role
	// already held"; fix round 2's MY-1). Linus is invited to Studio as admin,
	// seated there as a viewer by id, then signs in: he is Studio's admin.
	must(ada.CreateInvite(bg, studio.ID, hubapi.CreateInviteRequest{Email: FakeLinus.Email, Role: hubapi.RoleAdmin}))(t)
	must(ada.SetMember(bg, studio.ID, FakeLinus.ID, hubapi.RoleViewer))(t)
	if got := must(linus.Me(bg))(t); got.InvitesBound != 1 || len(got.Teams) != 2 ||
		got.Teams[0].ID != team.ID || got.Teams[0].Role != hubapi.RoleAdmin || got.Teams[1].ID != studio.ID || got.Teams[1].Role != hubapi.RoleAdmin {
		t.Errorf("Linus's Me with a Studio admin invite over his viewer's seat = bound %d teams %+v, want 1: Portal admin, Studio raised to admin", got.InvitesBound, got.Teams)
	}

	// Binding reads this call's email, refreshed from the principal, never the
	// one first seen (§5.1 step 5 refreshes display_name and email; task-16a.md:61
	// "binding that reads the stored users.email instead of this token"; fix round
	// 2, PS2-2). Nia signs in, then her email and name change: an invite for the
	// new address binds, and one for the old address no longer does.
	nia := FakeUser{ID: "u-00000000000000c3", DisplayName: "Nia Old", Email: "nia.old@example.test"}
	must(f.As(nia).Me(bg))(t)
	must(ada.CreateInvite(bg, team.ID, hubapi.CreateInviteRequest{Email: "nia@example.test", Role: hubapi.RoleMember}))(t)
	oldAddress := must(ada.CreateInvite(bg, studio.ID, hubapi.CreateInviteRequest{Email: "nia.old@example.test", Role: hubapi.RoleViewer}))(t)
	nia.DisplayName, nia.Email = "Nia Example", "Nia@Example.test"
	if got := must(f.As(nia).Me(bg))(t); got.User != (hubapi.User{ID: nia.ID, DisplayName: "Nia Example", Email: "nia@example.test"}) ||
		got.InvitesBound != 1 || len(got.Teams) != 1 || got.Teams[0].ID != team.ID || got.Teams[0].Role != hubapi.RoleMember {
		t.Errorf("Nia's Me after her email changed = %+v, want the new name and address, one bound: Portal member", got)
	}
	for _, inv := range must(ada.Invites(bg, studio.ID))(t) {
		if inv.ID == oldAddress.ID && (inv.State != hubapi.InvitePending || inv.BoundUser != nil) {
			t.Errorf("the invite for Nia's old address = %+v, want it still pending, bound to nobody", inv)
		}
	}

	// orgAdmin is the principal's, and an email is matched lowercased (§5.1).
	boss := f.As(FakeUser{ID: "u-00000000000000b1", DisplayName: "Org Boss", Email: "Boss@Example.TEST", OrgAdmin: true})
	must(ada.CreateInvite(bg, team.ID, hubapi.CreateInviteRequest{Email: "BOSS@example.test", Role: hubapi.RoleViewer}))(t)
	if got := must(boss.Me(bg))(t); !got.OrgAdmin || got.User.Email != "boss@example.test" || got.InvitesBound != 1 {
		t.Errorf("the org admin's Me = %+v, want orgAdmin, a lowercased email, one invite bound", got)
	}
	// An org admin reads no team it is not in (§5.3: "read a team, its members,
	// its invites" is 404 for O, the org admin who is not a member; fix round 2's
	// MY-2). The boss is Portal's viewer and nothing of Studio's.
	_, err = boss.Team(bg, studio.ID)
	wantProblem(t, err, 404, "team_not_found", "teamId", studio.ID)
	_, err = boss.Invites(bg, studio.ID)
	wantProblem(t, err, 404, "team_not_found", "teamId", studio.ID)
	if got := must(boss.Teams(bg))(t); len(got) != 1 || got[0].ID != team.ID || got[0].Role != hubapi.RoleViewer {
		t.Errorf("the org admin's Teams = %+v, want Portal alone, as viewer", got)
	}
}

func TestFakeTeamRoles(t *testing.T) {
	_, ada, grace, linus, team := withTeam(t)
	x := hubapi.CreateInviteRequest{Email: "x@example.test", Role: hubapi.RoleMember}

	// A member lacks admin: 403 naming the role.
	_, err := grace.CreateInvite(bg, team.ID, x)
	wantProblem(t, err, 403, "forbidden", "required", "admin")
	_, err = grace.SetMember(bg, team.ID, FakeAda.ID, hubapi.RoleViewer)
	wantProblem(t, err, 403, "forbidden", "required", "admin")
	wantProblem(t, grace.RemoveMember(bg, team.ID, FakeAda.ID), 403, "forbidden", "required", "admin")
	// The role check comes before the body (route 11 in task-16a.md:20 lists 403
	// before 400 and 409): a member's malformed invite, or one for a member, is
	// the same 403 (fix round 2, found beside PS2-1).
	for _, req := range []hubapi.CreateInviteRequest{{Email: "no-at-sign", Role: hubapi.RoleMember}, {Email: "y@example.test", Role: "owner"}, {Email: FakeAda.Email, Role: hubapi.RoleMember}} {
		_, err = grace.CreateInvite(bg, team.ID, req)
		wantProblem(t, err, 403, "forbidden", "required", "admin")
	}

	// An outsider learns nothing: 404 on every team method, even for an id that
	// does not exist or is malformed.
	_, err = linus.CreateInvite(bg, team.ID, x)
	wantProblem(t, err, 404, "team_not_found", "teamId", team.ID)
	_, err = linus.Team(bg, team.ID)
	wantProblem(t, err, 404, "team_not_found", "teamId", team.ID)
	_, err = linus.Invites(bg, team.ID)
	wantProblem(t, err, 404, "team_not_found")
	_, err = linus.SetMember(bg, team.ID, FakeLinus.ID, hubapi.RoleAdmin)
	wantProblem(t, err, 404, "team_not_found")
	wantProblem(t, linus.RemoveMember(bg, team.ID, FakeAda.ID), 404, "team_not_found")
	wantProblem(t, linus.RevokeInvite(bg, team.ID, "iv-0000000000000000"), 404, "team_not_found")
	_, err = ada.Team(bg, "tm-0000000000000000")
	wantProblem(t, err, 404, "team_not_found", "teamId", "tm-0000000000000000")
	_, err = ada.Team(bg, "not-an-id")
	wantProblem(t, err, 404, "team_not_found", "teamId", "not-an-id")

	// The last admin can neither leave nor be demoted.
	wantProblem(t, ada.RemoveMember(bg, team.ID, FakeAda.ID), 409, "last_admin", "teamId", team.ID)
	_, err = ada.SetMember(bg, team.ID, FakeAda.ID, hubapi.RoleMember)
	wantProblem(t, err, 409, "last_admin", "teamId", team.ID)

	// Names are unique per org, case-insensitively, and 1–120 after trimming.
	_, err = linus.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "  pORTAL "})
	wantProblem(t, err, 409, "conflict", "field", "name")
	for _, name := range []string{"", "   ", strings.Repeat("é", 121)} {
		_, err = ada.CreateTeam(bg, hubapi.CreateTeamRequest{Name: name})
		wantProblem(t, err, 400, "bad_request", "field", "name")
	}
	if tm := must(ada.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "  " + strings.Repeat("é", 120) + " "}))(t); tm.Name != strings.Repeat("é", 120) || tm.Role != hubapi.RoleAdmin {
		t.Errorf("a 120-character name = %q role %s, want it trimmed and the caller admin", tm.Name, tm.Role)
	}

	// Invites: a second pending one for an email is a conflict, as is one for a
	// member; a bound one cannot be revoked; a pending one can.
	must(ada.CreateInvite(bg, team.ID, x))(t)
	_, err = ada.CreateInvite(bg, team.ID, hubapi.CreateInviteRequest{Email: "X@example.test", Role: hubapi.RoleViewer})
	wantProblem(t, err, 409, "conflict")
	_, err = ada.CreateInvite(bg, team.ID, hubapi.CreateInviteRequest{Email: FakeGrace.Email, Role: hubapi.RoleViewer})
	wantProblem(t, err, 409, "conflict", "field", "email")
	for _, bad := range []hubapi.CreateInviteRequest{{Email: "no-at-sign", Role: hubapi.RoleMember}, {Email: "a@b@c", Role: hubapi.RoleMember}, {Email: "y@example.test", Role: "owner"}} {
		_, err = ada.CreateInvite(bg, team.ID, bad)
		field := "email"
		if bad.Role == "owner" {
			field = "role"
		}
		wantProblem(t, err, 400, "bad_request", "field", field)
	}
	invites := must(ada.Invites(bg, team.ID))(t) // newest first
	if len(invites) != 2 || invites[0].Email != "x@example.test" || invites[1].State != hubapi.InviteBound {
		t.Fatalf("Invites = %+v, want x's pending then Grace's bound", invites)
	}
	wantProblem(t, ada.RevokeInvite(bg, team.ID, invites[1].ID), 409, "conflict")
	noErr(t, ada.RevokeInvite(bg, team.ID, invites[0].ID))
	if got := must(ada.Invites(bg, team.ID))(t); got[0].State != hubapi.InviteRevoked {
		t.Errorf("a revoked invite reads %s, want revoked (the row is kept)", got[0].State)
	}
	// A DELETE is idempotent: revoking an invite that is already revoked is 204
	// and changes nothing (contract §7.3 names only the bound case; the ruling is
	// task 04 re-review m2's, carried to task 16a).
	noErr(t, ada.RevokeInvite(bg, team.ID, invites[0].ID))
	if got := must(ada.Invites(bg, team.ID))(t); len(got) != 2 || got[0].ID != invites[0].ID || got[0].State != hubapi.InviteRevoked || got[0].BoundUser != nil {
		t.Errorf("Invites after a second revoke = %+v, want x's still revoked, first, and no new row", got)
	}
	wantProblem(t, ada.RevokeInvite(bg, team.ID, "iv-0000000000000000"), 404, "invite_not_found", "inviteId", "iv-0000000000000000")

	// "One pending invite" is per team and pending only (contract §7.3; the
	// index team_invites_one_pending is (team_id, email) WHERE state = 'pending'):
	// a revoked invite no longer blocks the email, and a pending one in Portal
	// never blocks it in another team (task 04 review I1).
	again := must(ada.CreateInvite(bg, team.ID, x))(t)
	if again.State != hubapi.InvitePending || again.ID == invites[0].ID {
		t.Errorf("a re-invite after a revoke = %+v, want a new pending invite", again)
	}
	// Revoking an invite is the team admin's alone (contract §5.3, "create or
	// revoke an invite": 403 for M and V; task 04 re-review N2). Grace, a member,
	// cannot withdraw Ada's invite; the viewer's row is below.
	wantProblem(t, grace.RevokeInvite(bg, team.ID, again.ID), 403, "forbidden", "required", "admin")
	stillPending := func(who string) {
		t.Helper()
		if got := must(ada.Invites(bg, team.ID))(t); got[0].ID != again.ID || got[0].State != hubapi.InvitePending || got[0].BoundUser != nil {
			t.Errorf("x's invite after %s's revoke = %+v, want it first and still pending", who, got[0])
		}
	}
	stillPending("Grace")
	// The role check comes before the invite lookup (route 12 in task-16a.md:21
	// lists 403 before invite_not_found and the bound 409): a member gets the same
	// 403 for an id that names nothing and for a bound invite (fix round 2, PS2-1).
	wantProblem(t, grace.RevokeInvite(bg, team.ID, "iv-0000000000000000"), 403, "forbidden", "required", "admin")
	wantProblem(t, grace.RevokeInvite(bg, team.ID, invites[1].ID), 403, "forbidden", "required", "admin")
	lt := must(linus.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "Linus's"}))(t)
	theirs := must(linus.CreateInvite(bg, lt.ID, x))(t)
	if theirs.State != hubapi.InvitePending || theirs.TeamID != lt.ID {
		t.Errorf("x's invite to Linus's team while pending in Portal = %+v, want pending in his team", theirs)
	}
	// A team's revoke route reaches only that team's invites: Ada, admin of
	// Portal, cannot withdraw Linus's team's invite through Portal (contract §5.3,
	// "create or revoke an invite": A of that team; task 04 review I2).
	wantProblem(t, ada.RevokeInvite(bg, team.ID, theirs.ID), 404, "invite_not_found", "inviteId", theirs.ID)
	if got := must(linus.Invites(bg, lt.ID))(t); len(got) != 1 || got[0].ID != theirs.ID || got[0].State != hubapi.InvitePending {
		t.Errorf("Linus's invites after Ada's cross-team revoke = %+v, want his one invite still pending", got)
	}
	// The member check is this team's members only (§7.3; task 16a's "a member
	// of this team"): Grace, a member of Portal, is invitable to Linus's team
	// (fix round 1's PS-2).
	if g := must(linus.CreateInvite(bg, lt.ID, hubapi.CreateInviteRequest{Email: FakeGrace.Email, Role: hubapi.RoleViewer}))(t); g.State != hubapi.InvitePending {
		t.Errorf("inviting Portal's Grace to Linus's team = %+v, want pending", g)
	}

	// Members: a user of the org is added by id with addedBy the caller; a role is
	// one of the three; an unknown user is member_not_found.
	_, err = ada.SetMember(bg, team.ID, "u-00000000000000ff", hubapi.RoleMember)
	wantProblem(t, err, 404, "member_not_found", "userId", "u-00000000000000ff")
	_, err = ada.SetMember(bg, team.ID, FakeLinus.ID, "owner")
	wantProblem(t, err, 400, "bad_request", "field", "role")
	added := must(ada.SetMember(bg, team.ID, FakeLinus.ID, hubapi.RoleViewer))(t)
	if added.User.ID != FakeLinus.ID || added.Role != hubapi.RoleViewer || added.AddedBy == nil || added.AddedBy.ID != FakeAda.ID {
		t.Errorf("SetMember(add) = %+v, want Linus viewer added by Ada", added)
	}
	detail := must(linus.Team(bg, team.ID))(t) // a viewer reads the team
	if detail.Team.Role != hubapi.RoleViewer || len(detail.Members) != 3 {
		t.Fatalf("Team as Linus = %+v, want his role viewer and three members", detail)
	}
	var names []string
	for _, m := range detail.Members {
		names = append(names, m.User.DisplayName)
	}
	if !slices.Equal(names, []string{"Ada Example", "Grace Example", "Linus Example"}) {
		t.Errorf("members = %v, want them by display name", names)
	}
	if detail.Members[0].AddedBy != nil {
		t.Errorf("the creator's addedBy = %+v, want null", detail.Members[0].AddedBy)
	}
	_, err = linus.CreateInvite(bg, team.ID, hubapi.CreateInviteRequest{Email: "z@example.test", Role: hubapi.RoleMember})
	wantProblem(t, err, 403, "forbidden", "required", "admin")
	wantProblem(t, linus.RevokeInvite(bg, team.ID, again.ID), 403, "forbidden", "required", "admin")
	stillPending("Linus")

	must(ada.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "attic"}))(t)
	var order []string
	for _, tm := range must(ada.Teams(bg))(t) {
		order = append(order, tm.Name)
	}
	if want := []string{"attic", "Portal", strings.Repeat("é", 120)}; !slices.Equal(order, want) {
		t.Errorf("Ada's teams = %v, want %v (by lower(name))", order, want)
	}
	// With a second admin, the first may step down; anyone may leave.
	must(ada.SetMember(bg, team.ID, FakeGrace.ID, hubapi.RoleAdmin))(t)
	if got := must(ada.SetMember(bg, team.ID, FakeAda.ID, hubapi.RoleMember))(t); got.Role != hubapi.RoleMember {
		t.Errorf("Ada stepping down = %s, want member", got.Role)
	}
	noErr(t, linus.RemoveMember(bg, team.ID, FakeLinus.ID))
	_, err = linus.Team(bg, team.ID)
	wantProblem(t, err, 404, "team_not_found")
	wantProblem(t, grace.RemoveMember(bg, team.ID, FakeLinus.ID), 404, "member_not_found", "userId", FakeLinus.ID)
	wantProblem(t, grace.RemoveMember(bg, team.ID, FakeGrace.ID), 409, "last_admin")
	noErr(t, ada.RemoveMember(bg, team.ID, FakeAda.ID))
	if teams := must(ada.Teams(bg))(t); len(teams) != 2 || teams[0].Name != "attic" || teams[1].Name != strings.Repeat("é", 120) {
		t.Errorf("Ada's teams after leaving Portal = %+v, want only her other two", teams)
	}
}

const linkedMessage = "Another team already shares this repository. Ask someone in that team to invite you."

func TestFakeProjectsAndRemotes(t *testing.T) {
	_, ada, grace, linus, team := withTeam(t)
	gh := hubapi.RemoteRequest{RepoKey: "github.com/lcfrai/panaceaapp", Host: hubapi.HostGitHub}
	p := withProject(t, ada, team.ID, "Engagement Portal", gh)
	if !hubapi.ValidID(hubapi.PrefixProject, p.ID) || p.TeamID != team.ID || p.CreatedBy.ID != FakeAda.ID ||
		len(p.Remotes) != 1 || p.Remotes[0].RepoKey != gh.RepoKey || p.Remotes[0].Host != hubapi.HostGitHub || p.Remotes[0].AddedAt == "" {
		t.Fatalf("CreateProject = %+v, want a pj- project of the team with its one remote", p)
	}

	// One repository, one project in the org. The other project's id is named
	// only to a caller who can read it.
	_, err := ada.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: team.ID, Name: "Second", Remotes: []hubapi.RemoteRequest{gh}})
	if he := wantProblem(t, err, 409, "repo_already_linked", "repoKey", gh.RepoKey, "projectId", p.ID); he.Problem.Message != linkedMessage {
		t.Errorf("message = %q, want the contract's sentence verbatim", he.Problem.Message)
	}
	own := must(linus.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "Linus's"}))(t)
	_, err = linus.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: own.ID, Name: "Mine", Remotes: []hubapi.RemoteRequest{gh}})
	if he := wantProblem(t, err, 409, "repo_already_linked", "repoKey", gh.RepoKey); he.Problem.Details["projectId"] != "" {
		t.Errorf("an outsider was told the project id %q", he.Problem.Details["projectId"])
	}

	// ProjectForRepo answers only a project the caller can read.
	if got, ok, err := ada.ProjectForRepo(bg, gh.RepoKey); err != nil || !ok || got.ID != p.ID {
		t.Errorf("Ada's ProjectForRepo = %+v %v %v, want the project", got, ok, err)
	}
	if got, ok, err := grace.ProjectForRepo(bg, gh.RepoKey); err != nil || !ok || got.ID != p.ID {
		t.Errorf("Grace's ProjectForRepo = %+v %v %v, want the project", got, ok, err)
	}
	if got, ok, err := linus.ProjectForRepo(bg, gh.RepoKey); err != nil || ok || got.ID != "" {
		t.Errorf("Linus's ProjectForRepo = %+v %v %v, want ok false and no error", got, ok, err)
	}
	if _, ok, err := ada.ProjectForRepo(bg, "github.com/lcfrai/unlinked"); err != nil || ok {
		t.Errorf("an unlinked key = %v %v, want ok false", ok, err)
	}

	// Phase 3's Azure DevOps form is accepted; the contract's old one is 400 (A1).
	ado := hubapi.RemoteRequest{RepoKey: "azure-devops/kpmgcomau/panacea app/panaceaapp", Host: hubapi.HostAzureDevOps}
	if r := must(ada.AddRemote(bg, p.ID, ado))(t); r.RepoKey != ado.RepoKey || r.Host != hubapi.HostAzureDevOps || r.AddedAt == "" {
		t.Errorf("AddRemote = %+v", r)
	}
	old := hubapi.RemoteRequest{RepoKey: "kpmgcomau.visualstudio.com/p/r", Host: hubapi.HostAzureDevOps}
	_, err = ada.AddRemote(bg, p.ID, old)
	wantProblem(t, err, 400, "bad_request", "field", "repoKey")
	_, err = ada.AddRemote(bg, p.ID, hubapi.RemoteRequest{RepoKey: gh.RepoKey, Host: "gitlab"})
	wantProblem(t, err, 400, "bad_request", "field", "host")
	_, err = ada.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: team.ID, Name: "Third", Remotes: []hubapi.RemoteRequest{old}})
	wantProblem(t, err, 400, "bad_request", "field", "remotes")
	_, _, err = ada.ProjectForRepo(bg, old.RepoKey)
	wantProblem(t, err, 400, "bad_request", "field", "repo")
	_, err = ada.AddRemote(bg, p.ID, gh) // already this project's
	wantProblem(t, err, 409, "repo_already_linked", "repoKey", gh.RepoKey)
	if got := must(ada.Project(bg, p.ID))(t); len(got.Remotes) != 2 || got.Remotes[1].RepoKey != ado.RepoKey {
		t.Errorf("Project.Remotes = %+v, want the two in the order added", got.Remotes)
	}
	if got, ok, _ := grace.ProjectForRepo(bg, ado.RepoKey); !ok || got.ID != p.ID {
		t.Errorf("ProjectForRepo(ado) = %v %v, want the project", got.ID, ok)
	}

	// Cross-team visibility: an outsider is told nothing exists.
	_, err = linus.Project(bg, p.ID)
	wantProblem(t, err, 404, "project_not_found", "projectId", p.ID)
	_, err = linus.AddRemote(bg, p.ID, hubapi.RemoteRequest{RepoKey: "github.com/linus/x", Host: hubapi.HostGitHub})
	wantProblem(t, err, 404, "project_not_found", "projectId", p.ID)
	_, err = linus.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: team.ID, Name: "Intruder"})
	wantProblem(t, err, 404, "team_not_found", "teamId", team.ID)
	// Roles: adding a remote needs admin; creating a project needs member.
	_, err = grace.AddRemote(bg, p.ID, hubapi.RemoteRequest{RepoKey: "github.com/grace/x", Host: hubapi.HostGitHub})
	wantProblem(t, err, 403, "forbidden", "required", "admin")
	gp := withProject(t, grace, team.ID, "Grace's")
	if gp.Remotes == nil || len(gp.Remotes) != 0 {
		t.Errorf("a project with no remotes has Remotes %#v, want []", gp.Remotes)
	}
	must(ada.SetMember(bg, team.ID, FakeLinus.ID, hubapi.RoleViewer))(t)
	_, err = linus.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: team.ID, Name: "Viewer's"})
	wantProblem(t, err, 403, "forbidden", "required", "member")
	if got := must(linus.Project(bg, p.ID))(t); got.ID != p.ID {
		t.Errorf("a viewer reads the project: got %v", got.ID)
	}
	// Names: unique per team, case-insensitively; a name may repeat across teams.
	_, err = grace.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: team.ID, Name: " engagement PORTAL"})
	wantProblem(t, err, 409, "conflict", "field", "name")
	withProject(t, linus, own.ID, "Engagement Portal")
	_, err = grace.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: team.ID, Name: "  "})
	wantProblem(t, err, 400, "bad_request", "field", "name")
	// Remotes in one request: distinct, at most 20.
	dup := hubapi.RemoteRequest{RepoKey: "github.com/lcfrai/dup", Host: hubapi.HostGitHub}
	_, err = grace.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: team.ID, Name: "Dup", Remotes: []hubapi.RemoteRequest{dup, dup}})
	wantProblem(t, err, 400, "bad_request", "field", "remotes")
	var many []hubapi.RemoteRequest
	for i := range 21 {
		many = append(many, hubapi.RemoteRequest{RepoKey: "github.com/lcfrai/r" + strconv.Itoa(i), Host: hubapi.HostGitHub})
	}
	_, err = grace.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: team.ID, Name: "Many", Remotes: many})
	wantProblem(t, err, 400, "bad_request", "field", "remotes")
	if got := withProject(t, grace, team.ID, "Twenty", many[:20]...); len(got.Remotes) != 20 {
		t.Errorf("twenty remotes: got %d", len(got.Remotes))
	}
}

func TestFakeBriefExistsAndArchiveFreesTheRef(t *testing.T) {
	_, ada, grace, linus, team := withTeam(t)
	p := withProject(t, ada, team.ID, "P")
	const feat = "refs/heads/feat"
	b1 := branchBrief(t, grace, p.ID, feat)
	if !hubapi.ValidID(hubapi.PrefixBrief, b1.ID) || b1.ProjectID != p.ID || b1.Scope != hubapi.ScopeBranch || b1.BranchRef != feat ||
		b1.HeadRevision != 0 || b1.HeadKey != nil || b1.HeadDigest != nil || b1.HeadStatus != nil ||
		b1.LatestCompleteRevision != 0 || b1.ArchivedAt != nil || b1.CreatedBy.ID != FakeGrace.ID || b1.CreatedAt == "" || b1.UpdatedAt == "" {
		t.Fatalf("CreateBrief = %+v, want a fresh bf- head at 0 with null head fields", b1)
	}
	_, err := ada.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeBranch, BranchRef: feat})
	wantProblem(t, err, 409, "brief_exists", "briefId", b1.ID)
	pb := must(ada.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeProject}))(t)
	if pb.Scope != hubapi.ScopeProject || pb.BranchRef != "" {
		t.Errorf("a project brief = %+v", pb)
	}
	_, err = ada.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeProject})
	wantProblem(t, err, 409, "brief_exists", "briefId", pb.ID)
	for _, bad := range []struct {
		req   hubapi.CreateBriefRequest
		field string
	}{
		{hubapi.CreateBriefRequest{Scope: hubapi.ScopeProject, BranchRef: feat}, "branchRef"},
		{hubapi.CreateBriefRequest{Scope: hubapi.ScopeBranch, BranchRef: "main"}, "branchRef"},
		{hubapi.CreateBriefRequest{Scope: hubapi.ScopeBranch, BranchRef: "refs/heads/a..b"}, "branchRef"},
		{hubapi.CreateBriefRequest{Scope: hubapi.ScopeBranch}, "branchRef"},
		{hubapi.CreateBriefRequest{Scope: "repo", BranchRef: feat}, "scope"},
	} {
		_, err = ada.CreateBrief(bg, p.ID, bad.req)
		wantProblem(t, err, 400, "bad_request", "field", bad.field)
	}

	// Archive frees the ref; the archived brief is still readable, and refuses writes.
	arch := must(grace.Archive(bg, b1.ID))(t)
	if arch.ArchivedAt == nil || arch.ID != b1.ID || arch.UpdatedAt != *arch.ArchivedAt || arch.UpdatedAt <= b1.UpdatedAt {
		t.Fatalf("Archive = %+v, want archivedAt set and updatedAt moved to it", arch)
	}
	b2 := branchBrief(t, ada, p.ID, feat)
	if b2.ID == b1.ID {
		t.Fatal("a new brief on a freed ref reused the archived one's id")
	}
	_, err = grace.WriteRevisions(bg, p.ID, []hubapi.RevisionWrite{{BriefID: b1.ID, BaseRevision: 0, Content: content("open")}})
	wantProblem(t, err, 409, "brief_archived", "briefId", b1.ID)
	_, err = grace.Archive(bg, b1.ID)
	wantProblem(t, err, 409, "brief_archived", "briefId", b1.ID)
	_, err = grace.Rebind(bg, b1.ID, "refs/heads/elsewhere")
	wantProblem(t, err, 409, "brief_archived", "briefId", b1.ID)
	// Onto the ref b2 now holds: brief_archived still, not brief_exists (part D5:
	// a write to an archived brief is always refused, whatever the ref).
	_, err = grace.Rebind(bg, b1.ID, feat)
	wantProblem(t, err, 409, "brief_archived", "briefId", b1.ID)
	if h := headOf(t, ada, b1.ID); h.ArchivedAt == nil || *h.ArchivedAt != *arch.ArchivedAt {
		t.Errorf("the archived brief reads %+v", h)
	}

	// Rebind moves the ref and no revision; the next revision carries the new ref.
	r1 := write(t, ada, p.ID, b2.ID, 0, "open")
	rb := must(ada.Rebind(bg, b2.ID, "refs/heads/renamed"))(t)
	if rb.BranchRef != "refs/heads/renamed" || rb.HeadRevision != 1 || rb.UpdatedAt <= r1.SavedAt {
		t.Errorf("Rebind = %+v, want the new ref at head 1 and updatedAt moved", rb)
	}
	if same := must(ada.Rebind(bg, b2.ID, "refs/heads/renamed"))(t); same.UpdatedAt != rb.UpdatedAt || same.BranchRef != rb.BranchRef {
		t.Errorf("a rebind to its own ref = %+v, want a no-op", same)
	}
	r2 := write(t, ada, p.ID, b2.ID, 1, "open")
	if r1.BranchRef != feat || r2.BranchRef != "refs/heads/renamed" {
		t.Errorf("revision refs = %q, %q; want each the ref it was written under", r1.BranchRef, r2.BranchRef)
	}
	if res := must(ada.Resolve(bg, []string{r1.Key}))(t); res.Results[0].Revision == nil || res.Results[0].Revision.BranchRef != feat {
		t.Errorf("revision 1 after the rebind = %+v, want its own ref kept", res.Results[0])
	}
	b3 := branchBrief(t, ada, p.ID, feat) // the rebind freed feat
	_, err = ada.Rebind(bg, b3.ID, "refs/heads/renamed")
	wantProblem(t, err, 409, "brief_exists", "briefId", b2.ID)
	_, err = ada.Rebind(bg, pb.ID, "refs/heads/x")
	wantProblem(t, err, 400, "bad_request")
	_, err = ada.Rebind(bg, b3.ID, "refs/heads/a..b")
	wantProblem(t, err, 400, "bad_request", "field", "branchRef")

	// An outsider is told nothing exists.
	_, err = linus.Brief(bg, b1.ID)
	wantProblem(t, err, 404, "brief_not_found", "briefId", b1.ID)
	_, err = linus.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeBranch, BranchRef: "refs/heads/l"})
	wantProblem(t, err, 404, "project_not_found", "projectId", p.ID)
	_, err = linus.Archive(bg, b2.ID)
	wantProblem(t, err, 404, "brief_not_found", "briefId", b2.ID)
	_, err = linus.Rebind(bg, b2.ID, "refs/heads/l")
	wantProblem(t, err, 404, "brief_not_found", "briefId", b2.ID)

	// A viewer reads every brief and writes none (contract §5.3).
	must(ada.SetMember(bg, team.ID, FakeLinus.ID, hubapi.RoleViewer))(t)
	must(linus.Brief(bg, b2.ID))(t)
	_, err = linus.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeBranch, BranchRef: "refs/heads/l"})
	wantProblem(t, err, 403, "forbidden", "required", "member")
	_, err = linus.WriteRevisions(bg, p.ID, []hubapi.RevisionWrite{{BriefID: b2.ID, BaseRevision: 2, Content: content("open")}})
	wantProblem(t, err, 403, "forbidden", "required", "member")
	_, err = linus.Rebind(bg, b2.ID, "refs/heads/l")
	wantProblem(t, err, 403, "forbidden", "required", "member")
	_, err = linus.Archive(bg, b2.ID)
	wantProblem(t, err, 403, "forbidden", "required", "member")
	if h := headOf(t, ada, b2.ID); h.HeadRevision != 2 || h.ArchivedAt != nil || h.BranchRef != "refs/heads/renamed" {
		t.Errorf("after the viewer's refused writes the head is %+v, want it untouched", h)
	}
}

func TestFakeRevisionNumberingAndConflict(t *testing.T) {
	_, ada, grace, _, team := withTeam(t)
	p := withProject(t, ada, team.ID, "P")
	b := branchBrief(t, grace, p.ID, "refs/heads/feat")

	r1 := write(t, grace, p.ID, b.ID, 0, "open")
	if r1.Revision != 1 || r1.Key != "brief:"+b.ID+"@1" || r1.BriefID != b.ID || r1.ProjectID != p.ID ||
		r1.Scope != hubapi.ScopeBranch || r1.Status != "open" || r1.SavedBy.ID != FakeGrace.ID || r1.SavedAt == "" || r1.ErasedAt != nil {
		t.Fatalf("first revision = %+v", r1)
	}
	_, err := ada.WriteRevisions(bg, p.ID, []hubapi.RevisionWrite{{BriefID: b.ID, BaseRevision: 0, Content: content("open")}})
	wantProblem(t, err, 409, "revision_conflict", "briefId", b.ID, "headRevision", "1")
	_, err = ada.WriteRevisions(bg, p.ID, []hubapi.RevisionWrite{{BriefID: b.ID, BaseRevision: 5, Content: content("open")}})
	wantProblem(t, err, 409, "revision_conflict", "briefId", b.ID, "headRevision", "1")
	r2 := write(t, ada, p.ID, b.ID, 1, "complete")
	r3 := write(t, ada, p.ID, b.ID, 2, "open")
	if r2.Revision != 2 || r3.Revision != 3 || r2.SavedBy.ID != FakeAda.ID {
		t.Errorf("revisions = %d, %d; want 2, 3", r2.Revision, r3.Revision)
	}
	h := headOf(t, ada, b.ID)
	if h.HeadRevision != 3 || h.HeadKey == nil || *h.HeadKey != r3.Key || h.HeadStatus == nil || *h.HeadStatus != "open" {
		t.Errorf("head = %+v, want revision 3", h)
	}
	// A write moves the head's updatedAt to the revision's savedAt (one transaction).
	if h.UpdatedAt != r3.SavedAt || h.UpdatedAt <= b.UpdatedAt || h.CreatedAt != b.CreatedAt {
		t.Errorf("head times = created %s updated %s; want created kept, updated = revision 3's savedAt %s", h.CreatedAt, h.UpdatedAt, r3.SavedAt)
	}

	// History: oldest first, gapless, paged by revision.
	all := must(ada.Revisions(bg, b.ID, hubapi.PageRequest{}))(t)
	var nums []int
	for _, s := range all.Items {
		nums = append(nums, s.Revision)
	}
	if !slices.Equal(nums, []int{1, 2, 3}) || all.Next != nil || all.Items[1].Digest != r2.Digest || all.Items[1].Key != r2.Key || all.Items[1].Status != "complete" {
		t.Errorf("Revisions = %+v, want 1, 2, 3 with no next", all)
	}
	first := must(ada.Revisions(bg, b.ID, hubapi.PageRequest{Limit: 1}))(t)
	if len(first.Items) != 1 || first.Items[0].Revision != 1 || first.Next == nil {
		t.Fatalf("Revisions limit 1 = %+v, want [1] and a next", first)
	}
	second := must(ada.Revisions(bg, b.ID, hubapi.PageRequest{After: *first.Next, Limit: 1}))(t)
	if len(second.Items) != 1 || second.Items[0].Revision != 2 {
		t.Errorf("Revisions after %q = %+v, want [2]", *first.Next, second)
	}
	for _, bad := range []hubapi.PageRequest{{Limit: -1}, {Limit: 201}, {After: "x"}} {
		_, err = ada.Revisions(bg, b.ID, bad)
		wantProblem(t, err, 400, "bad_request")
	}
	// The cursor is any canonical decimal ≥ 0 that fits a Go int (part D4, as
	// fix rounds 1 and 2 word it; task 19's store takes afterRevision int): "0"
	// is the whole history, the head or past it an empty last page; any other
	// spelling is 400 (review M2), and so is a decimal too big for an int, which
	// no list ever gave (re-review m1).
	if got := must(ada.Revisions(bg, b.ID, hubapi.PageRequest{After: "0"}))(t); len(got.Items) != 3 || got.Next != nil {
		t.Errorf("Revisions after 0 = %+v, want 1, 2, 3 with no next", got)
	}
	for _, past := range []string{"3", "99"} {
		if got := must(ada.Revisions(bg, b.ID, hubapi.PageRequest{After: past}))(t); got.Items == nil || len(got.Items) != 0 || got.Next != nil {
			t.Errorf("Revisions after %s = %+v, want items [] and no next", past, got)
		}
	}
	for _, spelled := range []string{"01", "-1", "+1", " 1", "1.0", "99999999999999999999"} {
		_, err = ada.Revisions(bg, b.ID, hubapi.PageRequest{After: spelled})
		wantProblem(t, err, 400, "bad_request", "field", "after")
	}
}

// heads reads each brief's head revision.
func heads(t *testing.T, view *Fake, briefs []hubapi.BriefHead) []int {
	t.Helper()
	var out []int
	for _, b := range briefs {
		out = append(out, headOf(t, view, b.ID).HeadRevision)
	}
	return out
}

func TestFakeBatchIsAllOrNothing(t *testing.T) {
	_, ada, grace, _, team := withTeam(t)
	p := withProject(t, ada, team.ID, "P")
	other := withProject(t, ada, team.ID, "Other")
	var bs []hubapi.BriefHead
	for i := range 4 {
		b := branchBrief(t, grace, p.ID, "refs/heads/b"+strconv.Itoa(i))
		write(t, grace, p.ID, b.ID, 0, "open")
		bs = append(bs, b)
	}
	stranger := branchBrief(t, grace, other.ID, "refs/heads/b0")
	batch := func(bases ...int) []hubapi.RevisionWrite {
		var ws []hubapi.RevisionWrite
		for i, base := range bases {
			ws = append(ws, hubapi.RevisionWrite{BriefID: bs[i].ID, BaseRevision: base, Content: content("complete")})
		}
		return ws
	}
	unmoved := func(why string) {
		t.Helper()
		if got := heads(t, ada, bs); !slices.Equal(got, []int{1, 1, 1, 1}) {
			t.Errorf("%s: heads = %v, want every head still at 1", why, got)
		}
	}

	// Item 3's base is stale: 409 and nothing moved.
	_, err := grace.WriteRevisions(bg, p.ID, batch(1, 1, 0, 1))
	wantProblem(t, err, 409, "revision_conflict", "briefId", bs[2].ID, "headRevision", "1")
	unmoved("a stale item 3")
	// Two stale items: the first in brief-id order is named.
	firstStale := min(bs[1].ID, bs[3].ID)
	_, err = grace.WriteRevisions(bg, p.ID, batch(1, 0, 1, 0))
	wantProblem(t, err, 409, "revision_conflict", "briefId", firstStale)
	unmoved("two stale items")
	// Any other refusal in the batch also writes nothing.
	ws := batch(1, 1, 1, 1)
	ws[1].Content = json.RawMessage(`{"status":"done"}`)
	_, err = grace.WriteRevisions(bg, p.ID, ws)
	wantProblem(t, err, 400, "bad_request", "field", "revisions[1].content.status")
	ws = batch(1, 1, 1, 1)
	ws[3].BriefID = stranger.ID // another project's brief
	_, err = grace.WriteRevisions(bg, p.ID, ws)
	wantProblem(t, err, 404, "brief_not_found", "briefId", stranger.ID)
	ws = batch(1, 1, 1, 1)
	ws[3].BriefID = bs[0].ID
	_, err = grace.WriteRevisions(bg, p.ID, ws)
	wantProblem(t, err, 400, "bad_request", "field", "revisions")
	ws = batch(1, 1, 1, 1)
	ws[2].Content = json.RawMessage(`{"status":"open","big":"` + strings.Repeat("x", hubapi.MaxEnvelopeBytes) + `"}`)
	_, err = grace.WriteRevisions(bg, p.ID, ws)
	wantProblem(t, err, 400, "bad_request", "field", "revisions[2].content")
	_, err = grace.WriteRevisions(bg, p.ID, nil)
	wantProblem(t, err, 400, "bad_request", "field", "revisions")
	unmoved("the refused batches")

	// The whole batch, on the right bases, moves every head once.
	got := must(grace.WriteRevisions(bg, p.ID, batch(1, 1, 1, 1)))(t)
	if len(got) != 4 || !slices.Equal(heads(t, ada, bs), []int{2, 2, 2, 2}) {
		t.Errorf("the good batch: %d revisions, heads %v; want 4 and every head at 2", len(got), heads(t, ada, bs))
	}
}

func TestFakeBatchHoldsSixtyFour(t *testing.T) {
	_, ada, grace, _, team := withTeam(t)
	p := withProject(t, ada, team.ID, "P")
	var bs []hubapi.BriefHead
	for i := range hubapi.MaxRevisionBatch + 1 {
		bs = append(bs, branchBrief(t, grace, p.ID, fmt.Sprintf("refs/heads/b%02d", i)))
	}
	// Request order is descending brief id, so an answer in brief-id order
	// (the order the heads are checked in) cannot pass for request order.
	slices.SortFunc(bs, func(a, b hubapi.BriefHead) int { return strings.Compare(b.ID, a.ID) })
	var ws []hubapi.RevisionWrite
	for _, b := range bs[:hubapi.MaxRevisionBatch] {
		ws = append(ws, hubapi.RevisionWrite{BriefID: b.ID, BaseRevision: 0, Content: content("complete")})
	}
	got := must(grace.WriteRevisions(bg, p.ID, ws))(t)
	if len(got) != 64 {
		t.Fatalf("64 items: %d revisions, want 64", len(got))
	}
	for i, r := range got {
		if r.BriefID != ws[i].BriefID || r.Revision != 1 {
			t.Fatalf("revision %d is %s@%d, want %s@1 (request order)", i, r.BriefID, r.Revision, ws[i].BriefID)
		}
	}

	// 65 items, every one on its right base: 400 naming revisions, nothing written.
	ws = ws[:0]
	for i, b := range bs {
		ws = append(ws, hubapi.RevisionWrite{BriefID: b.ID, BaseRevision: min(1, 64-i), Content: content("open")})
	}
	_, err := grace.WriteRevisions(bg, p.ID, ws)
	wantProblem(t, err, 400, "bad_request", "field", "revisions")
	for i, h := range heads(t, ada, bs) {
		if want := min(1, 64-i); h != want {
			t.Fatalf("after the refused 65: brief %d is at %d, want %d", i, h, want)
		}
	}

	// 65 heads: the default page is 50 (contract §1), and 200 holds them all.
	if page := must(ada.List(bg, p.ID, hubapi.BriefQuery{}))(t); len(page.Items) != 50 || page.Next == nil || *page.Next != page.Items[49].ID {
		t.Errorf("List at the default limit: %d items, next %v; want 50 and the 50th's id", len(page.Items), page.Next)
	}
	if page := must(ada.List(bg, p.ID, hubapi.BriefQuery{Page: hubapi.PageRequest{Limit: 200}}))(t); len(page.Items) != 65 || page.Next != nil {
		t.Errorf("List at limit 200: %d items, next %v; want 65 and no next", len(page.Items), page.Next)
	}
}

func briefIDs(page hubapi.Page[hubapi.BriefHead]) []string {
	var out []string
	for _, h := range page.Items {
		out = append(out, h.ID)
	}
	return out
}

func TestFakeListBriefs(t *testing.T) {
	_, ada, grace, linus, team := withTeam(t)
	p := withProject(t, ada, team.ID, "P")
	pb := must(grace.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeProject}))(t)
	b1 := branchBrief(t, grace, p.ID, "refs/heads/one")
	gone := branchBrief(t, grace, p.ID, "refs/heads/gone")
	b2 := branchBrief(t, grace, p.ID, "refs/heads/two")
	must(grace.Archive(bg, gone.ID))(t)
	elsewhere := withProject(t, ada, team.ID, "Elsewhere")
	foreign := branchBrief(t, ada, elsewhere.ID, "refs/heads/one")
	lt := must(linus.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "Linus's"}))(t)
	theirs := branchBrief(t, linus, withProject(t, linus, lt.ID, "L").ID, "refs/heads/one")

	list := func(q hubapi.BriefQuery) []string {
		t.Helper()
		page := must(ada.List(bg, p.ID, q))(t)
		if page.Items == nil {
			t.Errorf("List(%+v).Items is nil, want []", q)
		}
		return briefIDs(page)
	}
	if got := list(hubapi.BriefQuery{}); !slices.Equal(got, []string{pb.ID, b1.ID, b2.ID}) {
		t.Errorf("List{} = %v, want the three live heads oldest first", got)
	}
	if got := list(hubapi.BriefQuery{Scope: hubapi.ScopeBranch}); !slices.Equal(got, []string{b1.ID, b2.ID}) {
		t.Errorf("Scope branch = %v", got)
	}
	if got := list(hubapi.BriefQuery{Scope: hubapi.ScopeProject}); !slices.Equal(got, []string{pb.ID}) {
		t.Errorf("Scope project = %v", got)
	}
	if got := list(hubapi.BriefQuery{IncludeArchived: true}); !slices.Equal(got, []string{pb.ID, b1.ID, gone.ID, b2.ID}) {
		t.Errorf("IncludeArchived = %v, want all four oldest first", got)
	}
	if got := list(hubapi.BriefQuery{BranchRef: "refs/heads/one"}); !slices.Equal(got, []string{b1.ID}) {
		t.Errorf("BranchRef one = %v, want only this project's", got)
	}
	if got := list(hubapi.BriefQuery{BranchRef: "refs/heads/gone"}); len(got) != 0 {
		t.Errorf("BranchRef of an archived brief = %v, want none unless asked", got)
	}
	if got := list(hubapi.BriefQuery{BranchRef: "refs/heads/gone", IncludeArchived: true}); !slices.Equal(got, []string{gone.ID}) {
		t.Errorf("BranchRef gone, archived included = %v", got)
	}

	// Paging: limit 1 walks the three with next, and the last page has none.
	var walked []string
	q := hubapi.BriefQuery{Page: hubapi.PageRequest{Limit: 1}}
	for range 4 {
		page := must(ada.List(bg, p.ID, q))(t)
		walked = append(walked, briefIDs(page)...)
		if page.Next == nil {
			break
		}
		q.Page.After = *page.Next
	}
	if !slices.Equal(walked, []string{pb.ID, b1.ID, b2.ID}) || q.Page.After != b1.ID {
		t.Errorf("paging by 1 = %v (last cursor %q), want the three and no next on the last", walked, q.Page.After)
	}

	// Refusals name what they refuse; an outsider sees no project.
	for _, bad := range []struct {
		q     hubapi.BriefQuery
		field string
	}{
		{hubapi.BriefQuery{Scope: "repo"}, "scope"},
		{hubapi.BriefQuery{BranchRef: "one"}, "branchRef"},
		{hubapi.BriefQuery{Page: hubapi.PageRequest{Limit: 201}}, "limit"},
		{hubapi.BriefQuery{Page: hubapi.PageRequest{After: "bf-0000000000000000"}}, "after"},
		// A brief of another project is no cursor of this list, whether the
		// caller can read it or not: the same 400 as an id that names nothing, so
		// the cursor is no existence oracle across teams (task 04 review M1).
		{hubapi.BriefQuery{Page: hubapi.PageRequest{After: foreign.ID}}, "after"},
		{hubapi.BriefQuery{Page: hubapi.PageRequest{After: theirs.ID}}, "after"},
	} {
		_, err := ada.List(bg, p.ID, bad.q)
		wantProblem(t, err, 400, "bad_request", "field", bad.field)
	}
	_, err := linus.List(bg, p.ID, hubapi.BriefQuery{})
	wantProblem(t, err, 404, "project_not_found", "projectId", p.ID)
}

// revAt is the revision number of r, or 0 for null.
func revAt(r *hubapi.BriefRevision) int {
	if r == nil {
		return 0
	}
	return r.Revision
}

func TestFakeForBranchCarriesLatestComplete(t *testing.T) {
	_, ada, grace, linus, team := withTeam(t)
	p := withProject(t, ada, team.ID, "P")
	const feat = "refs/heads/feat"

	// Nothing at all: never a 404, both null.
	fb := must(grace.ForBranch(bg, p.ID, feat))(t)
	if fb.BranchBrief != nil || fb.ProjectBrief != nil {
		t.Fatalf("ForBranch with no briefs = %+v, want both null", fb)
	}
	b := branchBrief(t, grace, p.ID, feat)
	pb := must(grace.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeProject}))(t)
	fb = must(grace.ForBranch(bg, p.ID, feat))(t)
	if fb.BranchBrief == nil || fb.BranchBrief.Head.ID != b.ID || fb.BranchBrief.Revision != nil || fb.BranchBrief.LatestComplete != nil {
		t.Fatalf("a fresh branch brief = %+v, want revision and latestComplete null", fb.BranchBrief)
	}

	write(t, grace, p.ID, b.ID, 0, "complete")
	write(t, grace, p.ID, b.ID, 1, "open")
	write(t, grace, p.ID, pb.ID, 0, "open")
	check := func(want, wantComplete int) {
		t.Helper()
		fb := must(grace.ForBranch(bg, p.ID, feat))(t)
		one := must(grace.Brief(bg, b.ID))(t)
		for name, bw := range map[string]*hubapi.BriefWithRevision{"ForBranch": fb.BranchBrief, "Brief": &one} {
			if bw == nil || revAt(bw.Revision) != want || revAt(bw.LatestComplete) != wantComplete || bw.Head.LatestCompleteRevision != wantComplete {
				t.Errorf("%s = %+v, want revision @%d and latestComplete @%d", name, bw, want, wantComplete)
				continue
			}
			if bw.LatestComplete.Status != "complete" || bw.LatestComplete.Content == nil {
				t.Errorf("%s latestComplete = %+v, want a complete revision with content", name, bw.LatestComplete)
			}
		}
		if fb.ProjectBrief == nil || fb.ProjectBrief.Head.ID != pb.ID || revAt(fb.ProjectBrief.Revision) != 1 || fb.ProjectBrief.LatestComplete != nil {
			t.Errorf("projectBrief = %+v, want revision @1 and latestComplete null", fb.ProjectBrief)
		}
	}
	check(2, 1)
	write(t, grace, p.ID, b.ID, 2, "complete")
	check(3, 3)
	if one := must(grace.Brief(bg, b.ID))(t); one.Revision == nil || one.LatestComplete == nil || one.Revision.Key != one.LatestComplete.Key {
		t.Errorf("a complete head: revision and latestComplete differ: %+v", one)
	}

	// Another ref: no branch brief, the project brief still. An archived branch
	// brief no longer answers for its ref.
	if fb := must(grace.ForBranch(bg, p.ID, "refs/heads/other"))(t); fb.BranchBrief != nil || fb.ProjectBrief == nil {
		t.Errorf("ForBranch(other) = %+v, want only the project brief", fb)
	}
	must(grace.Archive(bg, b.ID))(t)
	if fb := must(grace.ForBranch(bg, p.ID, feat))(t); fb.BranchBrief != nil {
		t.Errorf("ForBranch after archive = %+v, want branchBrief null", fb.BranchBrief)
	}
	for _, ref := range []string{"", "feat", "refs/heads/a..b"} {
		_, err := grace.ForBranch(bg, p.ID, ref)
		wantProblem(t, err, 400, "bad_request", "field", "branchRef")
	}
	_, err := linus.ForBranch(bg, p.ID, feat)
	wantProblem(t, err, 404, "project_not_found", "projectId", p.ID)
}

func TestFakeDigestIsTheEnvelopes(t *testing.T) {
	_, ada, grace, _, team := withTeam(t)
	p := withProject(t, ada, team.ID, "P")
	b := branchBrief(t, grace, p.ID, "refs/heads/feat")
	pb := must(grace.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeProject}))(t)
	// Not canonical as sent: spaces, and keys out of order.
	sent := json.RawMessage(` { "status" : "complete", "zeta": [1, 2], "alpha": {"b": true, "a": "x"} } `)
	got := must(grace.WriteRevisions(bg, p.ID, []hubapi.RevisionWrite{
		{BriefID: b.ID, BaseRevision: 0, Content: sent},
		{BriefID: pb.ID, BaseRevision: 0, Content: sent},
	}))(t)
	for i, r := range got {
		env := hubapi.BriefEnvelope{BriefID: r.BriefID, Revision: r.Revision, ProjectID: p.ID, Scope: r.Scope, BranchRef: r.BranchRef, Content: sent}
		want, canonical, err := env.Digest()
		noErr(t, err)
		if r.Digest != want {
			t.Errorf("revision %d digest = %s, want the envelope's %s", i, r.Digest, want)
		}
		parsed := must(hubapi.ParseBriefEnvelope(canonical))(t)
		if string(r.Content) != string(parsed.Content) {
			t.Errorf("revision %d content = %s, want the canonical %s", i, r.Content, parsed.Content)
		}
		h := headOf(t, ada, r.BriefID)
		if h.HeadDigest == nil || *h.HeadDigest != want || h.HeadKey == nil || *h.HeadKey != r.Key || h.HeadStatus == nil || *h.HeadStatus != "complete" {
			t.Errorf("head %d = %+v, want the revision's key, digest and status", i, h)
		}
		res := must(ada.Resolve(bg, []string{r.Key}))(t).Results[0]
		if res.Revision == nil || res.Revision.Digest != want || string(res.Revision.Content) != string(parsed.Content) {
			t.Errorf("Resolve(%s) = %+v, want the same digest and content", r.Key, res.Revision)
		}
	}
	if got[0].Scope != hubapi.ScopeBranch || got[0].BranchRef != "refs/heads/feat" || got[1].Scope != hubapi.ScopeProject || got[1].BranchRef != "" {
		t.Errorf("scopes and refs = %+v / %+v", got[0], got[1])
	}
	// The content a caller holds is its own copy.
	got[0].Content[0] = 'X'
	if res := must(ada.Resolve(bg, []string{got[0].Key}))(t).Results[0]; res.Revision.Content[0] != '{' {
		t.Error("changing a returned content changed the fake's stored revision")
	}
}

func TestFakeLatestCompleteMovesOnlyOnComplete(t *testing.T) {
	_, ada, grace, _, team := withTeam(t)
	p := withProject(t, ada, team.ID, "P")
	b := branchBrief(t, grace, p.ID, "refs/heads/feat")
	var got []int
	for i, status := range []string{"open", "complete", "open", "skipped"} {
		write(t, grace, p.ID, b.ID, i, status)
		h := headOf(t, grace, b.ID)
		got = append(got, h.LatestCompleteRevision)
		if h.HeadStatus == nil || *h.HeadStatus != status {
			t.Errorf("after %s, headStatus = %v", status, h.HeadStatus)
		}
	}
	if !slices.Equal(got, []int{0, 2, 2, 2}) {
		t.Errorf("latestCompleteRevision = %v, want 0, 2, 2, 2", got)
	}
}

var orgBoss = FakeUser{ID: "u-00000000000000b1", DisplayName: "Org Boss", Email: "boss@example.test", OrgAdmin: true}

func TestFakeResolveStates(t *testing.T) {
	f, ada, grace, linus, team := withTeam(t)
	p := withProject(t, ada, team.ID, "P")
	a := branchBrief(t, grace, p.ID, "refs/heads/a")
	c := branchBrief(t, grace, p.ID, "refs/heads/c")
	e := branchBrief(t, grace, p.ID, "refs/heads/e")
	ra := write(t, grace, p.ID, a.ID, 0, "complete")
	write(t, grace, p.ID, c.ID, 0, "open")
	re := write(t, grace, p.ID, e.ID, 0, "complete")
	write(t, grace, p.ID, e.ID, 1, "open")
	lt := must(linus.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "Linus's"}))(t)
	lp := withProject(t, linus, lt.ID, "L")
	lb := branchBrief(t, linus, lp.ID, "refs/heads/a")
	rl := write(t, linus, lp.ID, lb.ID, 0, "complete")

	// Erasure is the org admin's (contract §5.3): a team admin is refused.
	_, err := ada.EraseBrief(e.ID, "Client request")
	wantProblem(t, err, 403, "forbidden", "required", "org_admin")
	boss := f.As(orgBoss)
	_, err = boss.EraseBrief("bf-0000000000000000", "Client request")
	wantProblem(t, err, 404, "brief_not_found", "briefId", "bf-0000000000000000")
	_, err = boss.EraseBrief(e.ID, "")
	wantProblem(t, err, 400, "bad_request", "field", "reason")
	er := must(boss.EraseBrief(e.ID, "Client request 2026-11-02, matter 4471"))(t)
	if er.BriefID != e.ID || er.ErasedRevisions != 2 || er.ErasedAt == "" {
		t.Errorf("EraseBrief = %+v, want two revisions erased", er)
	}
	// Erasure is no read (§5.3: the org admin has "no read of project content it
	// is not a member of"; fix round 2, beside MY-2): the boss, in no team,
	// erased e, yet its brief and its project are the same 404 as for anyone.
	_, err = boss.Brief(bg, e.ID)
	wantProblem(t, err, 404, "brief_not_found", "briefId", e.ID)
	_, err = boss.Project(bg, p.ID)
	wantProblem(t, err, 404, "project_not_found", "projectId", p.ID)
	first := headOf(t, ada, e.ID)
	if again := must(boss.EraseBrief(e.ID, "again"))(t); again.ErasedRevisions != 0 {
		t.Errorf("a second erase erased %d, want 0", again.ErasedRevisions)
	}
	// "The head is archived if it was not" (§7.12): a second erase moves nothing.
	if h := headOf(t, ada, e.ID); first.ArchivedAt == nil || h.ArchivedAt == nil || *h.ArchivedAt != *first.ArchivedAt || h.UpdatedAt != first.UpdatedAt {
		at := func(p *string) string {
			if p == nil {
				return "null"
			}
			return *p
		}
		t.Errorf("a second erase moved the head: archivedAt %s -> %s, updatedAt %s -> %s (task 04 review M3)",
			at(first.ArchivedAt), at(h.ArchivedAt), first.UpdatedAt, h.UpdatedAt)
	}

	keys := []string{
		ra.Key,                 // resolved
		"brief:" + c.ID + "@9", // missing: no such revision of a readable brief
		rl.Key,                 // missing: another team's, not a 404
		re.Key,                 // erased
		"b-42",                 // invalid
		"design:cv-3f2a91c0@1", // design keys: missing until task 05
		ra.Key,                 // a repeat is answered in its slot
	}
	res := must(grace.Resolve(bg, keys))(t)
	want := []struct {
		kind hubapi.KeyKind
		res  hubapi.Resolution
	}{{"brief", "resolved"}, {"brief", "missing"}, {"brief", "missing"}, {"brief", "erased"}, {"", "invalid"}, {"design", "missing"}, {"brief", "resolved"}}
	if len(res.Results) != len(keys) {
		t.Fatalf("Resolve: %d results for %d keys", len(res.Results), len(keys))
	}
	for i, r := range res.Results {
		kind := hubapi.KeyKind("")
		if r.Kind != nil {
			kind = *r.Kind
		}
		if r.Key != keys[i] || kind != want[i].kind || r.Resolution != want[i].res || r.Approval != nil {
			t.Errorf("result %d = key %q kind %q %s, want %q %q %s", i, r.Key, kind, r.Resolution, keys[i], want[i].kind, want[i].res)
		}
		if hasRev := r.Revision != nil; hasRev != (want[i].res == "resolved" || want[i].res == "erased") {
			t.Errorf("result %d (%s) revision = %+v", i, r.Resolution, r.Revision)
		}
	}
	if r := res.Results[0].Revision; r == nil || r.Digest != ra.Digest || string(r.Content) != string(ra.Content) {
		t.Errorf("the resolved revision = %+v, want the one written", r)
	}
	if r := res.Results[3].Revision; r == nil || r.Content != nil || r.ErasedAt == nil || r.Key != re.Key ||
		r.Digest != re.Digest || r.SavedAt != re.SavedAt || r.SavedBy != re.SavedBy || r.Revision != 1 {
		t.Errorf("the erased revision = %+v, want content null, erasedAt set, key, digest, savedAt, savedBy kept", r)
	}
	if res.Results[4].Kind != nil {
		t.Errorf("an invalid key's kind = %v, want null", *res.Results[4].Kind)
	}
	wantHeads := map[string]hubapi.HeadInfo{
		a.ID: {HeadRevision: 1, LatestCompleteRevision: 1, Archived: false},
		e.ID: {HeadRevision: 2, LatestCompleteRevision: 1, Archived: true},
	}
	if len(res.Heads) != len(wantHeads) || res.Heads[a.ID] != wantHeads[a.ID] || res.Heads[e.ID] != wantHeads[e.ID] {
		t.Errorf("heads = %+v, want exactly %+v", res.Heads, wantHeads)
	}

	// The erased brief is archived and still readable, its text gone everywhere.
	one := must(grace.Brief(bg, e.ID))(t)
	if one.Head.ArchivedAt == nil || one.Revision == nil || one.Revision.Content != nil || one.Revision.ErasedAt == nil ||
		one.LatestComplete == nil || one.LatestComplete.Content != nil {
		t.Errorf("Brief(erased) = %+v, want archived, every revision content null", one)
	}
	for _, s := range must(grace.Revisions(bg, e.ID, hubapi.PageRequest{}))(t).Items {
		if s.ErasedAt == nil {
			t.Errorf("history row %d has no erasedAt", s.Revision)
		}
	}
	// The same key, asked by the outsider, is missing; nothing asked is nothing.
	if out := must(linus.Resolve(bg, []string{ra.Key}))(t); out.Results[0].Resolution != hubapi.Missing || len(out.Heads) != 0 || out.Heads == nil {
		t.Errorf("Linus resolving Grace's key = %+v, want missing and heads {}", out)
	}
	if none := must(grace.Resolve(bg, nil))(t); none.Results == nil || len(none.Results) != 0 || none.Heads == nil {
		t.Errorf("Resolve(nil) = %+v, want results [] and heads {}", none)
	}
}

// everyMethod calls each of the Hub's methods once through h, and the fake's
// EraseBrief, with arguments that name something that exists where one is needed.
func everyMethod(h *Fake, teamID, projectID, briefID string) map[string]error {
	e := func(_ any, err error) error { return err }
	c := bg
	return map[string]error{
		"Me":               e(h.Me(c)),
		"ProjectForRepo":   func() error { _, _, err := h.ProjectForRepo(c, "github.com/lcfrai/panaceaapp"); return err }(),
		"Project":          e(h.Project(c, projectID)),
		"CreateProject":    e(h.CreateProject(c, hubapi.CreateProjectRequest{TeamID: teamID, Name: "S"})),
		"AddRemote":        e(h.AddRemote(c, projectID, hubapi.RemoteRequest{RepoKey: "github.com/a/b", Host: hubapi.HostGitHub})),
		"Teams":            e(h.Teams(c)),
		"CreateTeam":       e(h.CreateTeam(c, hubapi.CreateTeamRequest{Name: "S"})),
		"Team":             e(h.Team(c, teamID)),
		"SetMember":        e(h.SetMember(c, teamID, FakeGrace.ID, hubapi.RoleMember)),
		"RemoveMember":     h.RemoveMember(c, teamID, FakeGrace.ID),
		"Invites":          e(h.Invites(c, teamID)),
		"CreateInvite":     e(h.CreateInvite(c, teamID, hubapi.CreateInviteRequest{Email: "s@example.test", Role: hubapi.RoleMember})),
		"RevokeInvite":     h.RevokeInvite(c, teamID, "iv-0000000000000000"),
		"CreateBrief":      e(h.CreateBrief(c, projectID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeBranch, BranchRef: "refs/heads/s"})),
		"List":             e(h.List(c, projectID, hubapi.BriefQuery{})),
		"ForBranch":        e(h.ForBranch(c, projectID, "refs/heads/s")),
		"Brief":            e(h.Brief(c, briefID)),
		"Rebind":           e(h.Rebind(c, briefID, "refs/heads/t")),
		"Archive":          e(h.Archive(c, briefID)),
		"Revisions":        e(h.Revisions(c, briefID, hubapi.PageRequest{})),
		"WriteRevisions":   e(h.WriteRevisions(c, projectID, []hubapi.RevisionWrite{{BriefID: briefID, Content: content("open")}})),
		"Resolve":          e(h.Resolve(c, []string{"b-42"})),
		"PutBlob":          e(h.PutBlob(c, projectID, hubapi.DigestOf(nil).String(), "text/plain; charset=utf-8", 0, strings.NewReader(""))),
		"GetBlob":          func() error { _, _, err := h.GetBlob(c, projectID, hubapi.DigestOf(nil).String()); return err }(),
		"Approve":          e(h.Approve(c, projectID, hubapi.ApprovalRequest{})),
		"Approval":         e(h.Approval(c, "ap-0000000000000000")),
		"Approvals":        e(h.Approvals(c, projectID, hubapi.ApprovalQuery{})),
		"LinkSession":      e(h.LinkSession(c, projectID, hubapi.SessionLinkRequest{})),
		"SessionLinks":     e(h.SessionLinks(c, projectID, hubapi.SessionLinkQuery{})),
		"Reserve":          e(h.Reserve(c, projectID, hubapi.ReserveRequest{})),
		"Confirm":          e(h.Confirm(c, "rl-0000000000000000", hubapi.ConfirmRequest{})),
		"Abandon":          e(h.Abandon(c, "rl-0000000000000000", hubapi.AbandonRequest{})),
		"JoinDeployment":   e(h.JoinDeployment(c, "rl-0000000000000000", hubapi.DeploymentJoinRequest{})),
		"Release":          e(h.Release(c, "rl-0000000000000000")),
		"Releases":         e(h.Releases(c, projectID, hubapi.ReleaseQuery{})),
		"RecordEvidence":   e(h.RecordEvidence(c, hubapi.EvidenceRequest{})),
		"EvidenceByDigest": e(h.EvidenceByDigest(c, "github.com/lcfrai/panaceaapp", []string{hubapi.DigestOf(nil).String()})),
		"Changes":          e(h.Changes(c, "latest", 10)),
		"EraseBrief":       e(h.EraseBrief(briefID, "switch test")),
	}
}

func TestFakeSwitches(t *testing.T) {
	f, ada, _, _, team := withTeam(t)
	p := withProject(t, ada, team.ID, "P")
	b := branchBrief(t, ada, p.ID, "refs/heads/feat")
	hub := reflect.TypeFor[Hub]()
	check := func(sentinel error, why string) {
		t.Helper()
		calls := everyMethod(ada, team.ID, p.ID, b.ID) // ada is a view; f holds the switches
		for i := range hub.NumMethod() {
			if _, ok := calls[hub.Method(i).Name]; !ok {
				t.Errorf("everyMethod does not call %s", hub.Method(i).Name)
			}
		}
		if len(calls) != hub.NumMethod()+1 {
			t.Errorf("everyMethod calls %d methods, want Hub's %d and EraseBrief", len(calls), hub.NumMethod())
		}
		for name, err := range calls {
			if !errors.Is(err, sentinel) {
				t.Errorf("%s: %s = %v, want %v", why, name, err, sentinel)
			}
			if Code(err) != "" {
				t.Errorf("%s: %s is a Hub problem %q; a switch answers before the Hub", why, name, Code(err))
			}
		}
	}
	f.SetSignedOut(true)
	check(ErrSignedOut, "signed out")
	f.SetUnavailable(true)
	check(ErrSignedOut, "signed out and unavailable") // no token, so no request
	f.SetSignedOut(false)
	check(ErrUnavailable, "unavailable")
	f.SetUnavailable(false)

	// Cleared, and nothing any switched call did stuck.
	if h := headOf(t, ada, b.ID); h.HeadRevision != 0 || h.ArchivedAt != nil || h.BranchRef != "refs/heads/feat" {
		t.Errorf("after the switched calls the brief is %+v, want it untouched", h)
	}
	if teams := must(ada.Teams(bg))(t); len(teams) != 1 {
		t.Errorf("after the switched calls Ada has %d teams, want 1", len(teams))
	}
	if members := must(ada.Team(bg, team.ID))(t).Members; len(members) != 2 {
		t.Errorf("after the switched calls the team has %d members, want 2", len(members))
	}
}

func TestFakeCallsListsEveryCall(t *testing.T) {
	f := newFake()
	ada, grace := f.As(FakeAda), f.As(FakeGrace)
	team := must(ada.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "Portal"}))(t)
	must(grace.Me(bg))(t)
	_, err := grace.Team(bg, team.ID) // a refused call is still a call
	wantProblem(t, err, 404, "team_not_found")
	f.SetSignedOut(true)
	_, _ = ada.Teams(bg) // so is a switched one
	f.SetSignedOut(false)
	must(ada.Resolve(bg, []string{"b-42"}))(t)
	_, err = ada.ForBranch(bg, "pj-0000000000000000", "refs/heads/x")
	wantProblem(t, err, 404, "project_not_found")
	want := []string{`CreateTeam({"name":"Portal"})`, "Me()", "Team(" + team.ID + ")", "Teams()", `Resolve(["b-42"])`,
		"ForBranch(pj-0000000000000000, refs/heads/x)"}
	got := f.Calls()
	if !slices.Equal(got, want) {
		t.Errorf("Calls() =\n  %q\nwant\n  %q", got, want)
	}
	got[0] = "changed"
	if f.As(FakeLinus).Calls()[0] == "changed" {
		t.Error("Calls returned the fake's own slice")
	}
}

// Under -race: every view shares one state, so a fake whose lock is missing
// or held too briefly is a data race here.
func TestFakeIsSafeForConcurrentUse(t *testing.T) {
	_, ada, grace, _, team := withTeam(t)
	p := withProject(t, ada, team.ID, "P")
	const writers, revisions = 16, 5
	var wg sync.WaitGroup
	errs := make(chan error, writers)
	for i := range writers {
		view := ada
		if i%2 == 1 {
			view = grace
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			b, err := view.CreateBrief(bg, p.ID, hubapi.CreateBriefRequest{Scope: hubapi.ScopeBranch, BranchRef: fmt.Sprintf("refs/heads/c%d", i)})
			if err != nil {
				errs <- err
				return
			}
			for base := range revisions {
				r, err := view.WriteRevisions(bg, p.ID, []hubapi.RevisionWrite{{BriefID: b.ID, BaseRevision: base, Content: content("open")}})
				if err == nil {
					_, err = view.Resolve(bg, []string{r[0].Key})
				}
				if err == nil {
					_, err = view.List(bg, p.ID, hubapi.BriefQuery{})
				}
				if err != nil {
					errs <- err
					return
				}
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
	page := must(ada.List(bg, p.ID, hubapi.BriefQuery{Page: hubapi.PageRequest{Limit: 200}}))(t)
	if len(page.Items) != writers {
		t.Fatalf("%d briefs, want %d", len(page.Items), writers)
	}
	for _, h := range page.Items {
		if h.HeadRevision != revisions {
			t.Errorf("%s is at %d, want %d", h.BranchRef, h.HeadRevision, revisions)
		}
	}
}

package conformance_test

import (
	"context"
	"regexp"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient/conformance"
)

// fakeQuota is the per-project blob quota the fake is given, so blob_quota
// runs without uploading a gibibyte.
const fakeQuota = 100 << 10

// newFakeEnv is the part's Env (task-05.md "fake_test.go — complete"), plus
// the org admin, the erasure and the quota this task added to Env. With clock
// false it has no Advance, as a real Hub has none.
func newFakeEnv(t *testing.T, clock bool) conformance.Env {
	t.Helper()
	// context.Background, not t.Context: the module's floor is Go 1.23 and
	// t.Context arrived in 1.24.
	ctx := context.Background()
	f := hubclient.NewFake()
	now := time.Date(2026, 9, 23, 4, 0, 0, 0, time.UTC)
	var mu sync.Mutex
	f.SetNow(func() time.Time { mu.Lock(); defer mu.Unlock(); return now })
	f.SetReservationTTL(15 * time.Minute)
	f.SetProjectQuota(fakeQuota)
	admin := f.As(hubclient.FakeAda)
	team, err := admin.CreateTeam(ctx, hubapi.CreateTeamRequest{Name: "Conformance"})
	if err != nil {
		t.Fatal(err)
	}
	member, viewer := f.As(hubclient.FakeGrace), f.As(hubclient.FakeLinus)
	outsider := f.As(hubclient.FakeUser{ID: "u-00000000000000a4", DisplayName: "Out Sider", Email: "out@example.test"})
	orgAdmin := f.As(hubclient.FakeUser{ID: "u-00000000000000a5", DisplayName: "Org Boss", Email: "boss@example.test", OrgAdmin: true})
	for _, p := range []struct {
		who  *hubclient.Fake
		u    hubclient.FakeUser
		role hubapi.Role
	}{{member, hubclient.FakeGrace, hubapi.RoleMember}, {viewer, hubclient.FakeLinus, hubapi.RoleViewer}} {
		if _, err := admin.CreateInvite(ctx, team.ID, hubapi.CreateInviteRequest{Email: p.u.Email, Role: p.role}); err != nil {
			t.Fatal(err)
		}
		if _, err := p.who.Me(ctx); err != nil { // binds the invite
			t.Fatal(err)
		}
	}
	env := conformance.Env{Admin: admin, Member: member, Viewer: viewer, Outsider: outsider,
		TeamID: team.ID, ReservationTTL: 15 * time.Minute, OrgAdmin: orgAdmin, BlobQuota: fakeQuota,
		Erase: func(briefID, reason string) error { _, err := orgAdmin.EraseBrief(briefID, reason); return err }}
	if clock {
		env.Advance = func(d time.Duration) { mu.Lock(); defer mu.Unlock(); now = now.Add(d) }
	}
	return env
}

func TestFakeConforms(t *testing.T) {
	conformance.Run(t, func(t *testing.T) conformance.Env { return newFakeEnv(t, true) })
}

// rowName is "<test>/<group>/<row>": a group of AllGroups, a row in
// snake_case, and no "#nn" suffix, which t.Run adds to a duplicate name.
var rowName = regexp.MustCompile(`^TestRunNamesEveryRow/([a-z]+)/([a-z0-9]+(_[a-z0-9]+)*)$`)

// wantRows is every row, by group, in order: the part's table and the carried
// items (task-05.md As built D3). A row dropped from a group's table, or one
// added without its name here, fails TestRunNamesEveryRow: ">= 40" alone let
// one of 83 go (task 05 mutant PS-1).
var wantRows = map[conformance.Group][]string{
	conformance.GroupTeams: {"me_binds_invites", "outsider_sees_404", "member_cannot_invite", "last_admin",
		"revoked_invite_never_binds", "reinvite_after_revoke", "pending_is_per_team", "me_binds_every_pending",
		"member_check_is_this_team", "revoke_is_team_scoped", "me_binds_only_own_email", "member_cannot_revoke",
		"viewer_cannot_revoke", "revoke_twice_is_204", "me_raises_a_lower_role", "org_admin_is_a_non_member"},
	conformance.GroupProjects: {"repo_linked_once", "repo_key_forms", "for_repo_is_readable_only", "org_admin_is_a_non_member"},
	conformance.GroupBriefs: {"brief_exists", "archive_frees_ref", "archived_refuses_writes", "numbering_and_conflict",
		"batch_all_or_nothing", "viewer_cannot_write", "list_filters", "latest_complete", "batch_of_sixty_four",
		"self_rebind_is_noop", "rebind_archived_first", "resolve_zero_keys", "list_cursor_is_this_projects",
		"revisions_cursor", "org_admin_is_a_non_member"},
	conformance.GroupRefs: {"five_states", "design_key_resolves", "org_admin_is_a_non_member"},
	conformance.GroupDesigns: {"blob_hash_checked", "blob_type_allowlist", "blob_too_large", "blob_other_project_404",
		"blob_roles", "blob_quota", "put_blob_rewinds", "approval_checks_in_order", "approver_is_the_caller",
		"approval_of_an_older_revision", "approvals_list_and_get", "session_link_repeat_returns_existing",
		"session_link_checks"},
	conformance.GroupReleases: {"pin_ship_head", "pin_falls_back_to_project_brief", "pin_no_brief",
		"pin_no_complete_revision", "pin_does_not_move", "reserve_refusals", "confirm_by_reserver_only",
		"confirm_idempotent", "confirm_other_run_409", "one_release_per_run", "panacea_keys_are_per_project",
		"abandon_from_reserved_only", "expiry",
		"join_rules", "rollback_inherits", "source_unknown", "source_unpinned", "by_deployment_id",
		"environment_null_until_joined", "environment_null_abandons_and_expires", "rollback_confirm_names_no_source",
		"kind_filter", "list_filters_and_order"},
	conformance.GroupEvidence: {"repo_not_linked", "viewer_forbidden", "evidence_checks", "oldest_first", "unreadable_repo_is_empty"},
	conformance.GroupFeed: {"latest_then_new_events", "outsider_sees_nothing", "more_when_limit_cuts",
		"audit_only_kinds_never_appear", "every_write_is_its_events"},
}

func TestRunNamesEveryRow(t *testing.T) {
	var mu sync.Mutex
	var names []string
	conformance.Run(t, func(t *testing.T) conformance.Env {
		mu.Lock()
		names = append(names, t.Name())
		mu.Unlock()
		t.SkipNow() // record the row, run nothing
		return conformance.Env{}
	})
	if len(names) < 40 {
		t.Errorf("Run named %d rows, want at least 40", len(names))
	}
	seen, groups := map[string]bool{}, map[string]int{}
	for _, n := range names {
		m := rowName.FindStringSubmatch(n)
		if m == nil || seen[n] {
			t.Errorf("row %q is not a distinct <group>/<row>", n)
			continue
		}
		seen[n] = true
		groups[m[1]]++
	}
	for _, g := range conformance.AllGroups {
		if groups[string(g)] == 0 {
			t.Errorf("group %s has no rows", g)
		}
	}
	if len(groups) != len(conformance.AllGroups) {
		t.Errorf("rows name groups %v, want exactly %v", groups, conformance.AllGroups)
	}
	var want []string
	for _, g := range conformance.AllGroups {
		for _, r := range wantRows[g] {
			want = append(want, "TestRunNamesEveryRow/"+string(g)+"/"+r)
		}
	}
	if !slices.Equal(names, want) {
		for _, n := range want {
			if !slices.Contains(names, n) {
				t.Errorf("row %s is not run", n)
			}
		}
		for _, n := range names {
			if !slices.Contains(want, n) {
				t.Errorf("row %s runs but is not in wantRows", n)
			}
		}
		t.Errorf("Run named %d rows, want the %d of wantRows in their order", len(names), len(want))
	}
	t.Logf("%d rows in %d groups: %v", len(names), len(groups), groups)
}

// TestClockRowsSkipWithoutAdvance runs the releases group on an Env with no
// Advance: the two ⏱ rows report skipped, and a row beside them still runs.
func TestClockRowsSkipWithoutAdvance(t *testing.T) {
	var mu sync.Mutex
	skipped := map[string]bool{}
	t.Run("releases", func(t *testing.T) {
		conformance.Run(t, func(t *testing.T) conformance.Env {
			t.Cleanup(func() { mu.Lock(); defer mu.Unlock(); skipped[t.Name()] = t.Skipped() })
			return newFakeEnv(t, false)
		}, conformance.GroupReleases)
	})
	const prefix = "TestClockRowsSkipWithoutAdvance/releases/releases/"
	for row, want := range map[string]bool{"expiry": true, "environment_null_abandons_and_expires": true, "pin_ship_head": false} {
		got, ran := skipped[prefix+row]
		if !ran || got != want {
			t.Errorf("%s: skipped %v (ran %v), want skipped %v", row, got, ran, want)
		}
	}
	if len(skipped) < 20 {
		t.Errorf("only %d release rows ran", len(skipped))
	}
}

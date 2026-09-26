package conformance_test

import (
	"context"
	"regexp"
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

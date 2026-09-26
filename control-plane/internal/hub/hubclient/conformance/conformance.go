// Package conformance is the behaviour every hubclient.Hub must show a caller:
// the fake (fake_test.go) and the real Hub through the HTTP client
// (hub_integration_test.go, task 34). A row is written once and run against both.
// It is a normal package importing testing — like net/http/httptest — because a
// _test.go file cannot be imported by the integration test of another package.
//
// The rows live one file per group (teams.go … feed.go); this file holds Env,
// Group, Run and the helpers every row shares.
package conformance

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// Env is one fresh world per row: a team T, and four principals of one org.
//
// What a row may assume, on the fake and on a real Hub alike (task 34 runs one
// Hub per test binary, and a fresh team per row): T is new, and Admin, Member
// and Viewer hold those roles in it; Outsider and OrgAdmin are in no team of the
// row's making. Other rows' teams may still exist, so a row names what it checks
// by id — "T is not among the outsider's teams", never "the outsider has none" —
// and every identifier the org holds unique (a repository key, a Panacea
// project, a pipeline run, a deployment id, a canvas id) is minted fresh with
// uniq. A row leaves no pending invite for a principal's email, which the next
// row's Me would bind.
type Env struct {
	Admin    hubclient.Hub // admin of T
	Member   hubclient.Hub // member of T
	Viewer   hubclient.Hub // viewer of T
	Outsider hubclient.Hub // same org, in no team
	// TeamID is T.
	TeamID string
	// Advance moves the Hub's clock. nil on a real Hub, where every row that
	// needs it is skipped with t.Skip and says so.
	Advance func(d time.Duration)
	// ReservationTTL is what this Hub was configured with.
	ReservationTTL time.Duration

	// Added by task 05 for carried/task-05.md items 1 and 13 (declared in the
	// part's As built): the org admin, and the erasure only an org admin makes.
	//
	// OrgAdmin is an org admin of the same org, in no team (on a real Hub,
	// devissuer's Ada, task-11.md:77). nil skips the rows that need it.
	OrgAdmin hubclient.Hub
	// Erase erases a brief as an org admin (contract §7.12): the fake's
	// EraseBrief, or the real Hub's route once task 27 lands. nil skips the
	// erased cases, and the row says so.
	Erase func(briefID, reason string) error
	// BlobQuota is the per-project blob quota this Hub was configured with
	// (contract §7.6, PANACEA_APP_HUB_PROJECT_QUOTA_BYTES). 0 skips blob_quota:
	// no row uploads a gibibyte to reach the default.
	BlobQuota int64
}

type Group string

const (
	GroupTeams    Group = "teams"    // server tasks 15, 16a
	GroupProjects Group = "projects" // 17
	GroupBriefs   Group = "briefs"   // 18, 19
	GroupRefs     Group = "refs"     // 20 (brief keys), 22 (design keys)
	GroupDesigns  Group = "designs"  // 21, 22
	GroupReleases Group = "releases" // 23, 24
	GroupEvidence Group = "evidence" // 25
	GroupFeed     Group = "feed"     // 26
)

// AllGroups is every group, in dependency order.
var AllGroups = []Group{GroupTeams, GroupProjects, GroupBriefs, GroupRefs, GroupDesigns,
	GroupReleases, GroupEvidence, GroupFeed}

// row is one assertion of the table: the part's rows, and the carried ones.
type row struct {
	name  string
	clock bool // ⏱: needs Env.Advance, skipped without it
	fn    func(t *testing.T, e Env)
}

// rowsOf is g's rows, or nil for a group that does not exist.
func rowsOf(g Group) []row {
	switch g {
	case GroupTeams:
		return teamRows
	case GroupProjects:
		return projectRows
	case GroupBriefs:
		return briefRows
	case GroupRefs:
		return refRows
	case GroupDesigns:
		return designRows
	case GroupReleases:
		return releaseRows
	case GroupEvidence:
		return evidenceRows
	case GroupFeed:
		return feedRows
	}
	return nil
}

// Run runs every row of the named groups (all of them when none are named),
// each as t.Run("<group>/<row>"), each with a fresh Env from newEnv. A group
// that does not exist fails the run. A row marked ⏱ is skipped, and says so,
// when the Env has no Advance: a Hub whose clock cannot move never "passes" a
// row it did not run.
func Run(t *testing.T, newEnv func(t *testing.T) Env, groups ...Group) {
	t.Helper()
	if len(groups) == 0 {
		groups = AllGroups
	}
	for _, g := range groups {
		rows := rowsOf(g)
		if rows == nil {
			t.Errorf("conformance: there is no group %q", g)
			continue
		}
		for _, r := range rows {
			t.Run(string(g)+"/"+r.name, func(t *testing.T) {
				e := newEnv(t)
				if r.clock && e.Advance == nil {
					t.Skip("conformance: this row moves the clock, and this Env has no Advance")
				}
				r.fn(t, e)
			})
		}
	}
}

// ── helpers every row shares ────────────────────────────────────────────────

// bg, not t.Context: the module's floor is Go 1.23 and t.Context arrived in 1.24.
var bg = context.Background()

// must turns (v, err) into v, failing the row on an error: must(h.X(…))(t).
func must[T any](v T, err error) func(t *testing.T) T {
	return func(t *testing.T) T {
		t.Helper()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		return v
	}
}

func ok(t *testing.T, err error) {
	t.Helper()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// wantErr asserts err is a *hubclient.Error with this status and code, a
// message, and each detail named in kv ("key", "value", …). It returns it.
func wantErr(t *testing.T, err error, status int, code string, kv ...string) *hubclient.Error {
	t.Helper()
	var he *hubclient.Error
	if !errors.As(err, &he) {
		t.Fatalf("err = %v, want a *hubclient.Error %d %s", err, status, code)
	}
	if he.Status != status || he.Problem.Error != code {
		t.Fatalf("got %d %s (%q, details %v), want %d %s", he.Status, he.Problem.Error, he.Problem.Message, he.Problem.Details, status, code)
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

// uniq is 16 fresh hex characters: what a row mints for anything the org
// holds unique, since a real Hub keeps one org across rows.
func uniq() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic("conformance: crypto/rand: " + err.Error())
	}
	return hex.EncodeToString(b[:])
}

// tick makes the next write's time distinct from the last (carried/task-05.md
// item 3: two rows made in one millisecond may list in either order on a Hub
// that orders by time). It moves the clock when it can, else waits.
func tick(e Env) {
	if e.Advance != nil {
		e.Advance(time.Millisecond)
		return
	}
	time.Sleep(2 * time.Millisecond)
}

func sp(s string) *string { return &s }

// sprint is v as text: a JSON number is float64 through the HTTP client and
// may be an integer in the fake; both print "3".
func sprint(v any) string { return fmt.Sprint(v) }

// erasedContent is an erased revision's content: nil in the fake, the four
// bytes null through encoding/json (carried/task-05.md item 2).
func erasedContent(c json.RawMessage) bool { return len(c) == 0 || string(c) == "null" }

// who is a named view, for rows that ask several principals the same thing.
type who struct {
	name string
	hub  hubclient.Hub
}

// strangers is the outsider and, when the Env has one, the org admin in no
// team: contract §5.3 gives both the same answer on every route but audit and
// erasure (hub-contract.md:331, :337).
func strangers(t *testing.T, e Env) []who {
	t.Helper()
	out := []who{{"outsider", e.Outsider}}
	if e.OrgAdmin != nil {
		return append(out, who{"org admin", e.OrgAdmin})
	}
	t.Log("conformance: this Env has no OrgAdmin; the org admin's column is not run")
	return out
}

// ── principals and fixtures ─────────────────────────────────────────────────

func meOf(t *testing.T, h hubclient.Hub) hubapi.Me {
	t.Helper()
	return must(h.Me(bg))(t)
}

// teamIn is the team with this id among teams.
func teamIn(teams []hubapi.Team, id string) (hubapi.Team, bool) {
	for _, tm := range teams {
		if tm.ID == id {
			return tm, true
		}
	}
	return hubapi.Team{}, false
}

// freshEmail is an address no principal signs in as, so its invite stays pending.
func freshEmail() string { return "fresh-" + uniq() + "@example.test" }

// newRepo is a GitHub repository key no project of the org has linked.
func newRepo() string { return "github.com/conformance/r-" + uniq() }

// newTeam is a second team, made by h, who is its admin.
func newTeam(t *testing.T, h hubclient.Hub) hubapi.Team {
	t.Helper()
	return must(h.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "Team " + uniq()}))(t)
}

// newProjectIn is a project of teamID, made by h, with one fresh GitHub remote.
func newProjectIn(t *testing.T, h hubclient.Hub, teamID string) hubapi.Project {
	t.Helper()
	return must(h.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: teamID, Name: "Project " + uniq(),
		Remotes: []hubapi.RemoteRequest{{RepoKey: newRepo(), Host: hubapi.HostGitHub}}}))(t)
}

// newProject is a project of T, made by T's admin.
func newProject(t *testing.T, e Env) hubapi.Project {
	t.Helper()
	return newProjectIn(t, e.Admin, e.TeamID)
}

// newBrief is a branch brief on ref, or the project brief when ref is "".
func newBrief(t *testing.T, h hubclient.Hub, projectID, ref string) hubapi.BriefHead {
	t.Helper()
	req := hubapi.CreateBriefRequest{Scope: hubapi.ScopeBranch, BranchRef: ref}
	if ref == "" {
		req.Scope = hubapi.ScopeProject
	}
	return must(h.CreateBrief(bg, projectID, req))(t)
}

func content(status string) json.RawMessage {
	return json.RawMessage(`{"status":"` + status + `","questions":[],"answers":[]}`)
}

// write writes one revision of briefID on base and returns it.
func write(t *testing.T, h hubclient.Hub, projectID, briefID string, base int, status string) hubapi.BriefRevision {
	t.Helper()
	got := must(h.WriteRevisions(bg, projectID, []hubapi.RevisionWrite{{BriefID: briefID, BaseRevision: base, Content: content(status)}}))(t)
	if len(got) != 1 {
		t.Fatalf("WriteRevisions answered %d revisions for one write", len(got))
	}
	return got[0]
}

// inviteOf is T's invite with this id, as the admin lists it.
func inviteOf(t *testing.T, h hubclient.Hub, teamID, inviteID string) hubapi.Invite {
	t.Helper()
	var found []hubapi.Invite
	for _, inv := range must(h.Invites(bg, teamID))(t) {
		if inv.ID == inviteID {
			found = append(found, inv)
		}
	}
	if len(found) != 1 {
		t.Fatalf("Invites(%s) holds invite %s %d times, want once", teamID, inviteID, len(found))
	}
	return found[0]
}

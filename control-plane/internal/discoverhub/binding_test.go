package discoverhub

import (
	"context"
	"encoding/json"
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

const (
	forkDir   = "/work/portal"
	localFork = "p-9b0d44"
	forkKey   = "github.com/me/portal"
)

var bg = context.Background()

// forkWorld is a bare world with one local project, "portal", at forkDir,
// whose remotes are keys, in the order given.
func forkWorld(t *testing.T, keys ...RepoKey) *world {
	t.Helper()
	w := newBareWorld(t)
	w.locate.addProject(localFork, forkDir, "portal")
	w.repos.set(forkDir, keys...)
	return w
}

func mustBinding(t *testing.T, s *Service, projectID string) Binding {
	t.Helper()
	b, err := s.Binding(bg, projectID)
	if err != nil {
		t.Fatalf("Binding(%s): %v", projectID, err)
	}
	return b
}

// remotesOf is the Hub project's remotes as the fake holds them.
func remotesOf(t *testing.T, w *world, projectID string) []string {
	t.Helper()
	p, err := w.hub.Fake.Project(w.ctx, projectID)
	if err != nil {
		t.Fatalf("Project(%s): %v", projectID, err)
	}
	var out []string
	for _, r := range p.Remotes {
		out = append(out, string(r.Host)+" "+r.RepoKey)
	}
	return out
}

func TestBindingFindsTheProjectByItsPrimaryRemote(t *testing.T) {
	w := newWorld(t)
	b := mustBinding(t, w.service(), localProject)
	if b.State != BindingBound || b.HubProjectID != w.project || b.HubProjectName != projectName {
		t.Fatalf("Binding = %+v, want bound to %s %q", b, w.project, projectName)
	}
	if b.Matches == nil || len(b.Matches) != 0 || !slices.Equal(b.RepoKeys, []string{teamKey}) || b.DefaultName != "" {
		t.Errorf("bound binding: Matches %v (want [] not nil), RepoKeys %v, DefaultName %q", b.Matches, b.RepoKeys, b.DefaultName)
	}

	// origin linked to pj-1, upstream to pj-2, and upstream listed first:
	// still pj-1, and upstream is never looked up.
	other := w.createProject("Other", "github.com/team/other")
	w.repos.set(projectDir, gh("upstream", "github.com/team/other"), gh("origin", teamKey))
	m := w.mark()
	b = mustBinding(t, w.service(), localProject)
	if b.State != BindingBound || b.HubProjectID != w.project || b.HubProjectID == other {
		t.Fatalf("Binding with upstream first = %+v, want bound to %s", b, w.project)
	}
	if got := w.since(m); !slices.Equal(got, []string{"ProjectForRepo(" + teamKey + ")"}) {
		t.Errorf("Hub calls = %v, want only origin's lookup", got)
	}
	if !slices.Equal(b.RepoKeys, []string{teamKey, "github.com/team/other"}) {
		t.Errorf("RepoKeys = %v, want origin's first", b.RepoKeys)
	}
}

func TestAnotherRemotesProjectIsAChoiceNotABinding(t *testing.T) {
	w := forkWorld(t, gh("origin", forkKey), gh("upstream", teamKey))
	team := w.createProject(projectName, teamKey)
	s := w.service()
	b := mustBinding(t, s, localFork)
	want := []RemoteMatch{{Remote: "upstream", RepoKey: teamKey, HubProjectID: team, HubProjectName: projectName}}
	if b.State != BindingUnbound || b.HubProjectID != "" || b.HubProjectName != "" || !reflect.DeepEqual(b.Matches, want) {
		t.Fatalf("fork Binding = %+v, want unbound with Matches %v", b, want)
	}
	if len(b.Teams) != 1 || b.Teams[0].ID != w.team || b.Teams[0].Name != teamName || b.Teams[0].Role != string(hubapi.RoleAdmin) {
		t.Errorf("Teams = %+v, want the one team, role admin", b.Teams)
	}
	// Nothing binds on its own: the fork's branch inherits nothing from the
	// team's project, and a read through it is ErrNotBound. (A write, task 08,
	// is ErrNotBound by the same Binding; task 08 is not built yet.)
	if _, err := s.Inheritable(bg, Target{ProjectID: localFork, BranchID: localBranch}); !errors.Is(err, ErrNotBound) {
		t.Errorf("Inheritable on the fork = %v, want ErrNotBound", err)
	}
}

func TestBindRegistersThePrimaryRemoteOnly(t *testing.T) {
	w := forkWorld(t, gh("origin", forkKey), gh("upstream", "github.com/team/portal-new"))
	s := w.service()
	b, err := s.Bind(bg, localFork, w.team, "")
	if err != nil || b.State != BindingBound {
		t.Fatalf("Bind = %+v, %v; want bound", b, err)
	}
	if got := remotesOf(t, w, b.HubProjectID); !slices.Equal(got, []string{"github " + forkKey}) {
		t.Errorf("the new project's remotes = %v, want origin's key alone", got)
	}
	if again := mustBinding(t, s, localFork); again.State != BindingBound || again.HubProjectID != b.HubProjectID {
		t.Errorf("Binding after Bind = %+v, want bound to %s", again, b.HubProjectID)
	}

	// No origin, two recognised remotes: the first by name, alone.
	w2 := forkWorld(t, gh("b", "github.com/team/bee"), gh("a", "github.com/team/ay"))
	s2 := w2.service()
	b2, err := s2.Bind(bg, localFork, w2.team, "")
	if err != nil || b2.State != BindingBound {
		t.Fatalf("Bind with no origin = %+v, %v", b2, err)
	}
	if got := remotesOf(t, w2, b2.HubProjectID); !slices.Equal(got, []string{"github github.com/team/ay"}) {
		t.Errorf("remotes = %v, want a's key alone", got)
	}
	if again := mustBinding(t, s2, localFork); again.State != BindingBound {
		t.Errorf("Binding after Bind = %+v, want bound", again)
	}
}

func TestThePrimaryRemoteIsOriginInAnyOrder(t *testing.T) {
	for _, name := range []string{"company", "azure", "github", "team"} {
		t.Run(name, func(t *testing.T) {
			// git's alphabetical order: the team remote before origin.
			w := forkWorld(t, gh(name, teamKey), gh("origin", forkKey))
			team := w.createProject(projectName, teamKey)
			s := w.service()
			b := mustBinding(t, s, localFork)
			want := []RemoteMatch{{Remote: name, RepoKey: teamKey, HubProjectID: team, HubProjectName: projectName}}
			if b.State != BindingUnbound || len(b.RepoKeys) == 0 || b.RepoKeys[0] != forkKey || !reflect.DeepEqual(b.Matches, want) {
				t.Fatalf("Binding = %+v, want unbound, RepoKeys[0] %s, Matches %v", b, forkKey, want)
			}
			bound, err := s.Bind(bg, localFork, w.team, "")
			if err != nil {
				t.Fatalf("Bind: %v", err)
			}
			if got := remotesOf(t, w, bound.HubProjectID); !slices.Equal(got, []string{"github " + forkKey}) {
				t.Errorf("remotes = %v, want origin's key alone", got)
			}
		})
	}

	in := []RepoKey{gh("zeta", "github.com/z/z"), gh("alpha", "github.com/a/a")}
	before := slices.Clone(in)
	got := orderRemotes(in)
	if len(got) != 2 || got[0].Remote != "alpha" || got[1].Remote != "zeta" {
		t.Errorf("orderRemotes(zeta, alpha) = %v, want alpha first", got)
	}
	if !slices.Equal(in, before) {
		t.Errorf("orderRemotes changed its input: %v, was %v", in, before)
	}
	three := []RepoKey{gh("upstream", "github.com/u/u"), gh("origin", "github.com/o/o"), gh("company", "github.com/c/c")}
	if got := orderRemotes(three); got[0].Remote != "origin" || got[1].Remote != "company" || got[2].Remote != "upstream" {
		t.Errorf("orderRemotes = %v, want origin, then the rest by name", got)
	}
}

func TestAForkSharesUnderItsOwnName(t *testing.T) {
	w := forkWorld(t, gh("origin", forkKey), gh("upstream", teamKey))
	w.createProject("portal", teamKey)
	s := w.service()
	b := mustBinding(t, s, localFork)
	if got := DefaultName("portal", b); got != "portal (me/portal)" || b.DefaultName != got {
		t.Fatalf("DefaultName = %q, Binding.DefaultName = %q; want %q", got, b.DefaultName, "portal (me/portal)")
	}
	bound, err := s.Bind(bg, localFork, w.team, "")
	if err != nil || bound.State != BindingBound || bound.HubProjectName != "portal (me/portal)" || bound.DefaultName != "" {
		t.Fatalf("Bind(team, \"\") = %+v, %v; want bound as %q with no DefaultName", bound, err, "portal (me/portal)")
	}
	if got := remotesOf(t, w, bound.HubProjectID); !slices.Equal(got, []string{"github " + forkKey}) {
		t.Errorf("remotes = %v, want origin's key alone", got)
	}
	if again := mustBinding(t, s, localFork); again.State != BindingBound || again.DefaultName != "" {
		t.Errorf("Binding after Bind = %+v, want bound, DefaultName \"\"", again)
	}

	w2 := forkWorld(t, gh("origin", forkKey), gh("upstream", teamKey))
	w2.createProject("portal", teamKey)
	if b2, err := w2.service().Bind(bg, localFork, w2.team, "  Portal fork "); err != nil || b2.HubProjectName != "Portal fork" {
		t.Errorf("Bind(team, \"Portal fork\") = %+v, %v; want that name, trimmed", b2, err)
	}
	// A name of white space alone is no name: the default (t07 mutant P2).
	w3 := forkWorld(t, gh("origin", forkKey), gh("upstream", teamKey))
	w3.createProject("portal", teamKey)
	if b3, err := w3.service().Bind(bg, localFork, w3.team, " \t "); err != nil || b3.HubProjectName != "portal (me/portal)" {
		t.Errorf("Bind(team, white space) = %+v, %v; want the default name", b3, err)
	}

	noMatch := Binding{State: BindingUnbound, RepoKeys: []string{forkKey}, Matches: []RemoteMatch{}}
	if got := DefaultName("portal", noMatch); got != "portal" {
		t.Errorf("DefaultName with no Matches = %q, want portal", got)
	}
	withMatch := Binding{State: BindingUnbound, RepoKeys: []string{forkKey}, Matches: []RemoteMatch{{Remote: "upstream", RepoKey: teamKey}}}
	got := DefaultName(strings.Repeat("x", 125), withMatch)
	if utf8.RuneCountInString(got) != 120 || got != strings.Repeat("x", 108)+" (me/portal)" {
		t.Errorf("DefaultName of a 125-character name = %q (%d), want 108 x and the key", got, utf8.RuneCountInString(got))
	}
	// Code points, not bytes (re-review b10-2 R-M2): each é is two bytes.
	got = DefaultName(strings.Repeat("é", 125), withMatch)
	if !utf8.ValidString(got) || got != strings.Repeat("é", 108)+" (me/portal)" {
		t.Errorf("DefaultName of 125 é = %q (%d runes), want 108 é and the key", got, utf8.RuneCountInString(got))
	}
	if got := DefaultName(strings.Repeat("é", 125), noMatch); got != strings.Repeat("é", 120) {
		t.Errorf("DefaultName of 125 é with no Matches = %d runes, want 120", utf8.RuneCountInString(got))
	}
	// A key that cannot fit (a GitHub key may be 39 + 1 + 100 after its
	// host): no qualifier at all, and NameTakenError covers a clash.
	huge := Binding{State: BindingUnbound, RepoKeys: []string{"github.com/" + strings.Repeat("o", 39) + "/" + strings.Repeat("r", 100)},
		Matches: withMatch.Matches}
	if got := DefaultName("portal", huge); got != "portal" {
		t.Errorf("DefaultName with a key over the budget = %q, want the local name alone", got)
	}
}

func TestANameTheTeamHoldsIsNameTaken(t *testing.T) {
	w := forkWorld(t, gh("origin", forkKey), gh("upstream", teamKey))
	w.createProject("portal", teamKey)
	s := w.service()
	before := mustBinding(t, s, localFork)
	nameTaken := func() *hubclient.Error {
		return &hubclient.Error{Status: 409, Problem: httpx.Problem{Error: hubapi.CodeConflict,
			Message: "A project in this team already has that name.", Details: map[string]string{"field": "name"}}}
	}
	w.hub.failCreateProject(nameTaken())
	_, err := s.Bind(bg, localFork, w.team, "")
	var nt *NameTakenError
	if !errors.As(err, &nt) || nt.Name != "portal (me/portal)" {
		t.Fatalf("Bind = %v, want *NameTakenError{portal (me/portal)}", err)
	}
	// Bind re-read the binding past the memo, to tell a teammate's race from
	// a name clash (review t07 I1); the memo it did not change answers now.
	m := w.mark()
	if after := mustBinding(t, s, localFork); !reflect.DeepEqual(after, before) {
		t.Errorf("Binding after the refusal = %+v, want it unchanged: %+v", after, before)
	}
	if got := w.since(m); len(got) != 0 {
		t.Errorf("Hub calls after the refusal = %v, want none: the memo stands", got)
	}
	if b, err := s.Bind(bg, localFork, w.team, "Portal fork"); err != nil || b.State != BindingBound {
		t.Errorf("Bind under another name = %+v, %v; want bound", b, err)
	}

	// The fake's own rule (it enforces hub-contract.md:710, case-insensitive).
	w2 := forkWorld(t, gh("origin", forkKey), gh("upstream", teamKey))
	w2.createProject("portal", teamKey)
	w2.createProject("Portal (Me/Portal)", "github.com/team/else")
	if _, err := w2.service().Bind(bg, localFork, w2.team, ""); !errors.As(err, &nt) || nt.Name != "portal (me/portal)" {
		t.Errorf("Bind into a team holding the name = %v, want *NameTakenError", err)
	}

	// A 409 with no field, and a 400 whose field is name (a length refusal),
	// come back as they came (re-review b10-2, mutant 2).
	for _, e := range []*hubclient.Error{
		{Status: 409, Problem: httpx.Problem{Error: hubapi.CodeConflict, Message: "conflict"}},
		{Status: 400, Problem: httpx.Problem{Error: hubapi.CodeBadRequest, Message: "A name is 1 to 120 characters.", Details: map[string]string{"field": "name"}}},
	} {
		w3 := forkWorld(t, gh("origin", forkKey))
		w3.hub.failCreateProject(e)
		_, err := w3.service().Bind(bg, localFork, w3.team, "")
		var he *hubclient.Error
		if !errors.As(err, &he) || he != e || errors.As(err, &nt) {
			t.Errorf("Bind with %d %s = %v, want the Hub's error as it came", e.Status, e.Problem.Error, err)
		}
	}

	// The re-read that tells a teammate's race from a name clash fails: its
	// error comes back as it came, never a NameTakenError nobody confirmed,
	// and the memo stands.
	w4 := forkWorld(t, gh("origin", forkKey), gh("upstream", teamKey))
	w4.createProject("portal", teamKey)
	s4 := w4.service()
	before4 := mustBinding(t, s4, localFork)
	w4.hub.failCreateProject(nameTaken())
	w4.hub.SetUnavailable(true)
	if _, err := s4.Bind(bg, localFork, w4.team, ""); !errors.Is(err, hubclient.ErrUnavailable) || errors.As(err, &nt) {
		t.Errorf("Bind whose re-read fails = %v, want hubclient.ErrUnavailable as it came", err)
	}
	if after := mustBinding(t, s4, localFork); !reflect.DeepEqual(after, before4) {
		t.Errorf("Binding after the failed re-read = %+v, want the memo unchanged: %+v", after, before4)
	}
}

func TestBindingWithNoRemoteSaysSo(t *testing.T) {
	w := forkWorld(t)
	s := w.service()
	m := w.mark()
	b := mustBinding(t, s, localFork)
	if b.State != BindingNoRemote || b.RepoKeys == nil || len(b.RepoKeys) != 0 || b.Teams == nil || b.Matches == nil {
		t.Fatalf("Binding = %+v, want no_remote with [] slices", b)
	}
	if _, err := s.Bind(bg, localFork, w.team, ""); !errors.Is(err, ErrNoRemote) {
		t.Errorf("Bind = %v, want ErrNoRemote", err)
	}
	if got := w.since(m); len(got) != 0 || len(w.hub.resolves()) != 0 {
		t.Errorf("Hub calls = %v, want none", got)
	}
	raw, _ := json.Marshal(b)
	if !strings.Contains(string(raw), `"repoKeys":[]`) || !strings.Contains(string(raw), `"matches":[]`) {
		t.Errorf("Binding JSON = %s, want [] not null", raw)
	}
}

func TestBindingOffWhenTheHubIsNotConfigured(t *testing.T) {
	w := newWorld(t)
	s := w.service(withoutHub())
	m := w.mark()
	if b := mustBinding(t, s, localProject); b.State != BindingHubOff || b.HubProjectID != "" {
		t.Errorf("Binding = %+v, want hub_off", b)
	}
	ref, states, err := s.HeadRef(bg, w.branch())
	if err != nil || ref.Resolution != "none" || ref.Key != "" || states != nil {
		t.Errorf("HeadRef = %+v, %v, %v; want a none Ref, nil states, nil error (rule 6)", ref, states, err)
	}
	// Inheritable answers none with no error, the answer HeadRef reports
	// (re-review b05-2 N-c).
	if inh, err := s.Inheritable(bg, w.branch()); err != nil || !reflect.DeepEqual(inh, Inheritable{}) {
		t.Errorf("Inheritable = %+v, %v; want none, nil", inh, err)
	}
	v, err := s.View(bg, w.branch())
	if err != nil || v.Binding.State != BindingHubOff || v.Brief != nil || v.Inherited != nil || v.Preview != nil {
		t.Errorf("View = %+v, %v; want hub_off and no brief", v, err)
	}
	if !reflect.DeepEqual(v.Step, discover.NewContent().Step(discover.ScopeBranch)) {
		t.Errorf("View.Step = %+v, want the first step of NewContent", v.Step)
	}
	if _, err := s.Bind(bg, localProject, w.team, ""); !errors.Is(err, ErrHubOff) {
		t.Errorf("Bind = %v, want ErrHubOff", err)
	}
	if _, err := s.History(bg, w.b42, "", 0); !errors.Is(err, ErrHubOff) {
		t.Errorf("History = %v, want ErrHubOff", err)
	}
	if got := w.since(m); len(got) != 0 {
		t.Errorf("Hub calls = %v, want none", got)
	}
}

func TestBindRaceReturnsTheTeammatesProject(t *testing.T) {
	const newKey = "github.com/team/newrepo"
	w := forkWorld(t, gh("origin", newKey))
	s := w.service()
	if b := mustBinding(t, s, localFork); b.State != BindingUnbound {
		t.Fatalf("Binding = %+v, want unbound", b)
	}
	// A teammate Shares the repository after this Binding was memoised.
	theirs := w.createProject("New repo", newKey)
	m := w.mark()
	b, err := s.Bind(bg, localFork, w.team, "")
	if err != nil || b.State != BindingBound || b.HubProjectID != theirs {
		t.Fatalf("Bind = %+v, %v; want bound to the teammate's %s", b, err, theirs)
	}
	creates := 0
	for _, c := range w.since(m) {
		if strings.HasPrefix(c, "CreateProject(") {
			creates++
		}
	}
	if creates != 1 {
		t.Errorf("CreateProject calls = %d, want the one the fake refused", creates)
	}
	if again := mustBinding(t, s, localFork); again.HubProjectID != theirs {
		t.Errorf("Binding after the race = %+v, want %s", again, theirs)
	}

	// Another team, one the caller is not in, holds the repository: the
	// re-read is still unbound, so the Hub's refusal comes back as it came,
	// never an unbound Binding that reads as a Share that worked (t07 P1).
	w2 := forkWorld(t, gh("origin", newKey))
	grace := w2.hub.Fake.As(hubclient.FakeGrace)
	other, err := grace.CreateTeam(bg, hubapi.CreateTeamRequest{Name: "Another team"})
	if err != nil {
		t.Fatalf("CreateTeam as Grace: %v", err)
	}
	if _, err := grace.CreateProject(bg, hubapi.CreateProjectRequest{TeamID: other.ID, Name: "Theirs",
		Remotes: []hubapi.RemoteRequest{{RepoKey: newKey, Host: hubapi.HostGitHub}}}); err != nil {
		t.Fatalf("CreateProject as Grace: %v", err)
	}
	b2, err := w2.service().Bind(bg, localFork, w2.team, "")
	if hubclient.Code(err) != hubapi.CodeRepoAlreadyLinked || b2.State != "" {
		t.Errorf("Bind into a repository another team holds = %+v, %v; want repo_already_linked as it came", b2, err)
	}

	// The commonest race (review t07 I1): the teammate's clone has the same
	// directory name, so they Shared under the same default name. The Hub
	// may check the per-team name before the repository (the fake does,
	// fake_projects.go:487-495; the contract fixes no order), so the refusal
	// reads as a name clash. Bind re-reads past the memo and binds to theirs,
	// never telling the person to choose another name for a repository their
	// team already shares.
	const portal2 = "github.com/team/portal2"
	w3 := forkWorld(t, gh("origin", portal2))
	s3 := w3.service()
	if b := mustBinding(t, s3, localFork); b.State != BindingUnbound || b.DefaultName != "portal" {
		t.Fatalf("Binding = %+v, want unbound with the default name portal", b)
	}
	theirs3 := w3.createProject("portal", portal2)
	b3, err := s3.Bind(bg, localFork, w3.team, "")
	if err != nil || b3.State != BindingBound || b3.HubProjectID != theirs3 || b3.HubProjectName != "portal" {
		t.Fatalf("Bind after a teammate Shared under the same name = %+v, %v; want bound to their %s", b3, err, theirs3)
	}
	if again := mustBinding(t, s3, localFork); again.State != BindingBound || again.HubProjectID != theirs3 {
		t.Errorf("Binding after the race = %+v, want %s: the stale memo is forgotten", again, theirs3)
	}
}

// TestABindingReadInFlightDoesNotOutliveABind (review t07 m2): a Binding
// lookup that read unbound before a Bind created the project, and returns
// after the Bind forgot the memo, must not store its stale unbound, or the
// lens reads unbound for the memo's 60 s after a Share that worked.
func TestABindingReadInFlightDoesNotOutliveABind(t *testing.T) {
	const newKey = "github.com/team/newrepo"
	w := forkWorld(t, gh("origin", newKey))
	s := w.service()
	var bound Binding
	var bindErr error
	w.locate.duringName(func() { bound, bindErr = s.Bind(bg, localFork, w.team, "New repo") })
	inFlight := mustBinding(t, s, localFork)
	if bindErr != nil || bound.State != BindingBound {
		t.Fatalf("Bind during the lookup = %+v, %v; want bound", bound, bindErr)
	}
	if inFlight.State != BindingUnbound {
		t.Fatalf("the lookup in flight = %+v, want the unbound it read before the Bind", inFlight)
	}
	if b := mustBinding(t, s, localFork); b.State != BindingBound || b.HubProjectID != bound.HubProjectID {
		t.Errorf("Binding after the Bind = %+v, want bound to %s, not the in-flight read's unbound", b, bound.HubProjectID)
	}
}

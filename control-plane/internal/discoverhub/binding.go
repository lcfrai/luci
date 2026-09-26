package discoverhub

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

type BindingState string

const (
	BindingBound    BindingState = "bound"
	BindingUnbound  BindingState = "unbound"   // it has remotes; no Hub project has its primary one (Matches may name projects that have another)
	BindingNoRemote BindingState = "no_remote" // OQ-1
	BindingHubOff   BindingState = "hub_off"
)

// RemoteMatch is a Hub project found through a remote that is NOT the
// primary one: typically a fork's clone, whose origin is the person's fork
// and whose upstream is the team's repository (review M8). It is shown as a
// fact beside the choice to share, never bound to on its own, because
// binding a fork to the team's project would put the fork's briefs there.
type RemoteMatch struct {
	Remote         string `json:"remote"` // "upstream"
	RepoKey        string `json:"repoKey"`
	HubProjectID   string `json:"hubProjectId"`
	HubProjectName string `json:"hubProjectName"`
}

type TeamChoice struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"` // the caller's; Bind needs member or above
}

type Binding struct {
	State          BindingState  `json:"state"`
	HubProjectID   string        `json:"hubProjectId"`   // "" unless bound
	HubProjectName string        `json:"hubProjectName"` // "" unless bound
	RepoKeys       []string      `json:"repoKeys"`       // what was looked up, in order; [0] is the primary remote's
	Teams          []TeamChoice  `json:"teams"`          // unbound only: where it could be created
	Matches        []RemoteMatch `json:"matches"`        // unbound only: projects found through a non-primary remote; [] otherwise
	DefaultName    string        `json:"defaultName"`    // unbound only: DefaultName(local name, this binding), what Share pre-fills (b10 re-review I-2); "" otherwise

	primaryHost string // RepoIdentity's host for RepoKeys[0]; what Bind registers it under
}

// emptyBinding is a binding in state with every slice [], never nil.
func emptyBinding(state BindingState) Binding {
	return Binding{State: state, RepoKeys: []string{}, Teams: []TeamChoice{}, Matches: []RemoteMatch{}}
}

// clone copies b's slices, so no caller can reach the memo's.
func (b Binding) clone() Binding {
	b.RepoKeys, b.Teams, b.Matches = slices.Clone(b.RepoKeys), slices.Clone(b.Teams), slices.Clone(b.Matches)
	return b
}

// orderRemotes is the one place the PRIMARY remote is chosen (review M8;
// b10 re-review I-1): the entry whose Remote is "origin" when there is one,
// otherwise the first by Remote name; the rest follow by Remote name. The
// primary is the one Binding binds by and the only one Bind registers. It
// lives here, beside its only callers, and not in RepoIdentity, so no
// implementation's order can move it: git lists "company" before "origin",
// and a fork whose team remote sorts first would otherwise bind to the
// team's project, the defect M8 exists to stop. Pure; it does not mutate
// keys.
func orderRemotes(keys []RepoKey) []RepoKey {
	out := slices.Clone(keys)
	slices.SortStableFunc(out, func(a, b RepoKey) int {
		switch {
		case a.Remote == b.Remote:
			return 0
		case a.Remote == "origin":
			return -1
		case b.Remote == "origin":
			return 1
		}
		return strings.Compare(a.Remote, b.Remote)
	})
	return out
}

// Binding orders RepoIdentity's answer with orderRemotes, then looks the
// project up by its PRIMARY remote's key (RepoKeys[0]) and is bound only to
// the project that key names (review M8). When there is none, it looks up
// each other remote's key in order and lists every hit in Matches, state
// unbound: the person chooses (task 18), nothing binds on its own. It is
// memoised in memory per local project for 60 s, and forgotten on Bind. A
// nil Hub is hub_off, with no lookup at all (rule 6); an error, Hub or local,
// is returned unchanged and memoises nothing.
func (s *Service) Binding(ctx context.Context, projectID string) (Binding, error) {
	if s.cfg.Hub == nil {
		return emptyBinding(BindingHubOff), nil
	}
	now := s.cfg.Now()
	s.mu.Lock()
	m, ok := s.memo[projectID]
	s.mu.Unlock()
	if ok && now.Before(m.expires) {
		return m.b.clone(), nil
	}
	b, err := s.lookup(ctx, projectID)
	if err != nil {
		return Binding{}, err
	}
	s.mu.Lock()
	s.memo[projectID] = memoEntry{b: b, expires: now.Add(bindingMemo)}
	s.mu.Unlock()
	return b.clone(), nil
}

// lookup is Binding without the memo.
func (s *Service) lookup(ctx context.Context, projectID string) (Binding, error) {
	path, err := s.cfg.Locate.ProjectPath(ctx, projectID)
	if err != nil {
		return Binding{}, err
	}
	keys, err := s.cfg.Repos.RepoKeys(ctx, path)
	if err != nil {
		return Binding{}, err
	}
	ordered := orderRemotes(keys)
	b := emptyBinding(BindingNoRemote)
	for _, k := range ordered {
		b.RepoKeys = append(b.RepoKeys, k.Key)
	}
	if len(ordered) == 0 {
		return b, nil
	}
	b.primaryHost = ordered[0].Host
	p, ok, err := s.cfg.Hub.ProjectForRepo(ctx, ordered[0].Key)
	if err != nil {
		return Binding{}, err
	}
	if ok {
		b.State, b.HubProjectID, b.HubProjectName = BindingBound, p.ID, p.Name
		return b, nil
	}
	b.State = BindingUnbound
	for _, k := range ordered[1:] {
		p, ok, err := s.cfg.Hub.ProjectForRepo(ctx, k.Key)
		if err != nil {
			return Binding{}, err
		}
		if ok {
			b.Matches = append(b.Matches, RemoteMatch{Remote: k.Remote, RepoKey: k.Key, HubProjectID: p.ID, HubProjectName: p.Name})
		}
	}
	me, err := s.cfg.Hub.Me(ctx)
	if err != nil {
		return Binding{}, err
	}
	for _, t := range me.Teams {
		b.Teams = append(b.Teams, TeamChoice{ID: t.ID, Name: t.Name, Role: string(t.Role)})
	}
	b.DefaultName = DefaultName(s.cfg.Locate.ProjectName(projectID), b)
	return b, nil
}

// forget drops projectID's memo.
func (s *Service) forget(projectID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.memo, projectID)
}

// Bind creates the Hub project in teamID with the PRIMARY remote's key as
// its only remote (review M8: a fork's clone must not claim the team's
// repository through its upstream remote), named name, or DefaultName when
// name is "" (after trimming). It reads the binding through Binding's memo:
// bound already is that binding, no_remote is ErrNoRemote, a nil Hub is
// ErrHubOff. repo_already_linked (a race with a teammate) forgets the memo,
// re-reads and returns that binding instead of failing; when the re-read is
// still not bound (the other project is one the caller cannot read), the
// Hub's error is returned as it came. A Hub 409 that refuses the name (b10
// re-review I-2: `name` is "unique per team", hub-contract.md:710) is
// *NameTakenError, nothing bound and the memo unchanged, so the person can
// Share again under another name. Every other error is returned unchanged.
// Joining a project that a non-primary remote matched is not built (OQ-12).
func (s *Service) Bind(ctx context.Context, projectID, teamID, name string) (Binding, error) {
	if s.cfg.Hub == nil {
		return Binding{}, ErrHubOff
	}
	b, err := s.Binding(ctx, projectID)
	if err != nil {
		return Binding{}, err
	}
	switch b.State {
	case BindingBound:
		return b, nil
	case BindingNoRemote:
		return Binding{}, ErrNoRemote
	}
	if name = strings.TrimSpace(name); name == "" {
		name = DefaultName(s.cfg.Locate.ProjectName(projectID), b)
	}
	req := hubapi.CreateProjectRequest{TeamID: teamID, Name: name,
		Remotes: []hubapi.RemoteRequest{{RepoKey: b.RepoKeys[0], Host: hubapi.GitHost(b.primaryHost)}}}
	p, err := s.cfg.Hub.CreateProject(ctx, req)
	switch {
	case err == nil:
		s.forget(projectID)
		out := emptyBinding(BindingBound)
		out.HubProjectID, out.HubProjectName = p.ID, p.Name
		out.RepoKeys, out.primaryHost = slices.Clone(b.RepoKeys), b.primaryHost
		return out, nil
	case hubclient.Code(err) == hubapi.CodeRepoAlreadyLinked:
		s.forget(projectID)
		again, rerr := s.Binding(ctx, projectID)
		if rerr != nil {
			return Binding{}, rerr
		}
		if again.State == BindingBound {
			return again, nil
		}
		return Binding{}, err
	case isNameTaken(err):
		return Binding{}, &NameTakenError{Name: name}
	}
	return Binding{}, err
}

// maxNameRunes is the Hub's cap on a project name: 1 to 120 characters
// after trimming, counted in code points (hub-contract.md:710; the fake's
// name120 counts utf8.RuneCountInString).
const maxNameRunes = 120

// DefaultName is the name Bind uses when the person gives none, and the one
// task 18 pre-fills. The local project's name (project.Scanner's directory
// name), unless the binding has Matches: then a fork's clone and the team's
// clone share a directory name ("portal"), and the team's project, if a
// teammate Shared it, holds that name already. So a binding with Matches
// gets the local name, a space, and the primary key without its host in
// parentheses: "portal (me/portal)". The whole is cut to the Hub's 120
// characters, counted in code points (re-review b10-2 R-M2), by shortening
// the local name, never the key; a key too long to leave the local name even
// one code point (a GitHub key may be 140 after its host) drops the
// qualifier, and the default is the local name alone. It is a default, not a
// guarantee: two unrelated repositories named "portal" can still meet in one
// team, which NameTakenError covers.
func DefaultName(localName string, b Binding) string {
	local := strings.TrimSpace(localName)
	if len(b.Matches) > 0 && len(b.RepoKeys) > 0 {
		q := " (" + withoutHost(b.RepoKeys[0]) + ")"
		if room := maxNameRunes - utf8.RuneCountInString(q); room >= 1 {
			return strings.TrimSpace(strings.TrimSpace(cutRunes(local, room)) + q)
		}
	}
	return strings.TrimSpace(cutRunes(local, maxNameRunes))
}

// withoutHost is a repo key minus its first segment: "github.com/me/portal"
// is "me/portal", "azure-devops/org/proj/repo" is "org/proj/repo".
func withoutHost(key string) string {
	if i := strings.IndexByte(key, '/'); i >= 0 {
		return key[i+1:]
	}
	return key
}

// cutRunes is s's first n code points, never a split UTF-8 sequence.
func cutRunes(s string, n int) string {
	for i := range s {
		if n == 0 {
			return s[:i]
		}
		n--
	}
	return s
}

// NameTakenError is a Bind the Hub refused because teamID already holds a
// project called Name. Matched by isNameTaken: a 409 whose details.field is
// "name". The contract states the rule (hub-contract.md:710) but names no
// code for it; team create's is 409 `conflict` with details.field "name"
// (:672), and the Hub's fake refuses a project name the same way
// (fake_projects.go:487-491 at hub 3df9fab8), so isNameTaken is the seam that
// ask HA4-12 fixes.
type NameTakenError struct{ Name string }

func (e *NameTakenError) Error() string {
	return "discoverhub: this team already has a project called " + strconv.Quote(e.Name)
}

// isNameTaken: a 409, and details.field "name". A 400 on the name (its
// length) is not one: it comes back as it came (re-review b10-2, mutant 2).
func isNameTaken(err error) bool {
	var he *hubclient.Error
	return errors.As(err, &he) && he.Status == 409 && he.Problem.Details["field"] == "name"
}

// BranchRef is "refs/heads/" + the branch's name; "" for "", and for a
// branch the Locator does not know.
func (s *Service) BranchRef(branchID string) string {
	if branchID == "" {
		return ""
	}
	name := s.cfg.Locate.BranchName(branchID)
	if name == "" {
		return ""
	}
	return "refs/heads/" + name
}

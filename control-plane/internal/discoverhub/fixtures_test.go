package discoverhub

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/discover"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/provenance"
)

// The fact sheet's world (prototype-fact-sheet.md §1-§3, §7) in
// hubclient.NewFake(), seeded by newWorld:
//
//   - one team, and the Hub project "Engagement Portal", whose one remote is
//     github.com/team/portal, the local project's origin;
//   - b-7, the project brief: revisions 1-6, 5 and 6 complete (§3);
//   - b-42, the branch brief on refs/heads/evidence-upload: revisions 1-6,
//     5 and 6 complete (§2).
//
// The pins, chosen once (re-review b06-2 R2-M1; the fact sheet names none
// for b-42@6): b-42 revisions 1-5 pin b-7@5, the project's latest complete
// revision when they were written, and b-42@6 pins b-7@6, written after it.
//
// The fake mints its own ids (ids.Wide), so "b-7" and "b-42" are w.b7 and
// w.b42 here, never those strings; "b-42" appears only as a local form.

const (
	localProject = "p-6f1c0e" // the local id of the engagement-portal clone
	localBranch  = "b-3a9d21" // evidence-upload
	localTimeout = "b-7c4e02" // fix-session-timeout
	localMain    = "b-11aa30" // main
	projectDir   = "/work/engagement-portal"
	teamKey      = "github.com/team/portal"
	evidenceRef  = "refs/heads/evidence-upload"
	timeoutRef   = "refs/heads/fix-session-timeout"
	answeredAt   = "2026-09-24T10:00:00.000Z"
	projectName  = "Engagement Portal"
	teamName     = "Audit Engagements"
	missingKey40 = "brief:bf-00000000000000b4@5" // fact sheet §7: b-40@5, which no Hub holds
)

// The answers of fact sheet §2-§3.
var (
	b7Purpose     = "Client-facing portal for audit engagements: requests, evidence and status."
	b7Audience    = "KPMG staff broadly"
	b7Audience6   = "Client users, and the KPMG staff who serve them"
	b7Constraints = "Existing client data; the published API"
	b7Done        = []string{"Tests pass", "No change to the published API without a version bump"}
	b7OutOfScope  = "Billing and invoicing"

	b42Purpose     = "Let client users upload evidence files against an open request."
	b42Constraints = "Existing uploads and their download links"
	b42DoneA       = []string{"Upload works for PDF, XLSX and images", "A virus scan runs before any file is visible", "Tests pass"}
	b42DoneB       = []string{"Upload works for PDF, XLSX and images", "A virus scan runs before any file is visible", "An interrupted upload resumes", "Tests pass"}
	b42OutOfScope  = "Bulk upload; the reviewer side of evidence"
)

// ── doubles ─────────────────────────────────────────────────────────────────

type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *testClock {
	return &testClock{t: time.Date(2026, 9, 24, 10, 0, 0, 0, time.UTC)}
}

func (c *testClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *testClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// fakeLocator is task 14's *project.Scanner adapter, by table. It records
// every id it is asked about, so a test can show no Hub id reached it.
type fakeLocator struct {
	mu       sync.Mutex
	paths    map[string]string // local project -> path
	names    map[string]string // local project -> directory name
	branches map[string]string // local branch -> name
	asked    []string
	onName   func() // set by duringName; run once, by the next ProjectName, before it returns
}

func newLocator() *fakeLocator {
	return &fakeLocator{paths: map[string]string{}, names: map[string]string{}, branches: map[string]string{}}
}

func (l *fakeLocator) ProjectPath(_ context.Context, projectID string) (string, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.asked = append(l.asked, projectID)
	p, ok := l.paths[projectID]
	if !ok {
		return "", project.ErrUnknownProject
	}
	return p, nil
}

func (l *fakeLocator) ProjectName(projectID string) string {
	l.mu.Lock()
	hook := l.onName
	l.onName = nil
	l.asked = append(l.asked, projectID)
	name := l.names[projectID]
	l.mu.Unlock()
	if hook != nil {
		hook()
	}
	return name
}

// duringName runs f once, inside the next ProjectName call. Binding's lookup
// asks for the name last, after the Hub answered, so f runs while that
// lookup's answer is in flight and not yet memoised (review t07 m2).
func (l *fakeLocator) duringName(f func()) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.onName = f
}

func (l *fakeLocator) BranchName(branchID string) string {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.asked = append(l.asked, branchID)
	return l.branches[branchID]
}

func (l *fakeLocator) setBranch(id, name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.branches[id] = name
}

func (l *fakeLocator) addProject(id, path, name string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.paths[id], l.names[id] = path, name
}

func (l *fakeLocator) seen() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.Clone(l.asked)
}

// fakeRepos answers each path's remotes in the order given, as git would
// list them (alphabetically, in task 14; any order here).
type fakeRepos struct {
	mu    sync.Mutex
	byDir map[string][]RepoKey
}

func (r *fakeRepos) RepoKeys(_ context.Context, path string) ([]RepoKey, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return slices.Clone(r.byDir[path]), nil
}

func (r *fakeRepos) set(path string, keys ...RepoKey) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.byDir[path] = keys
}

func gh(remote, key string) RepoKey { return RepoKey{Key: key, Host: "github", Remote: remote} }

// evalRecorder is a provenance.Evaluate that records every query and answers
// every criterion of it with the state and because it holds now.
type evalRecorder struct {
	mu      sync.Mutex
	state   provenance.CriterionState
	because []string
	queries []provenance.EvalQuery
}

func (e *evalRecorder) set(state provenance.CriterionState, because ...string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.state, e.because = state, because
}

func (e *evalRecorder) eval(_ context.Context, q provenance.EvalQuery) ([]provenance.Verdict, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.queries = append(e.queries, q)
	out := make([]provenance.Verdict, len(q.Criteria))
	for i, c := range q.Criteria {
		out[i] = provenance.Verdict{CriterionID: c.ID, State: e.state, Because: slices.Clone(e.because)}
	}
	return out, nil
}

func (e *evalRecorder) calls() []provenance.EvalQuery {
	e.mu.Lock()
	defer e.mu.Unlock()
	return slices.Clone(e.queries)
}

// ── contents ────────────────────────────────────────────────────────────────

func person(k discover.Key, text string) discover.Answer {
	return discover.Answer{Key: k, Text: text, Source: discover.SourceTyped, Author: discover.AuthorPerson,
		Criteria: []discover.Criterion{}, DraftedFrom: []string{}, AnsweredAt: answeredAt}
}

func skipped(k discover.Key) discover.Answer {
	return discover.Answer{Key: k, Skipped: true, Author: discover.AuthorPerson,
		Criteria: []discover.Criterion{}, DraftedFrom: []string{}, AnsweredAt: answeredAt}
}

func doneAnswer(tag string, items []string) discover.Answer {
	a := person(discover.KeyDone, strings.Join(items, "\n"))
	for i, it := range items {
		a.Criteria = append(a.Criteria, discover.Criterion{ID: fmt.Sprintf("c-%s%07d", tag, i+1), Text: it})
	}
	return a
}

// step is one saved revision: the answer it records and what it says changed.
type step struct {
	a      discover.Answer
	kind   discover.ChangeKind
	status discover.Status
}

func b7Steps() []step {
	return []step{
		{person(discover.KeyPurpose, b7Purpose), discover.ChangeRecord, discover.StatusOpen},
		{person(discover.KeyAudience, b7Audience), discover.ChangeRecord, discover.StatusOpen},
		{person(discover.KeyConstraints, b7Constraints), discover.ChangeRecord, discover.StatusOpen},
		{doneAnswer("p", b7Done), discover.ChangeRecord, discover.StatusOpen},
		{person(discover.KeyOutOfScope, b7OutOfScope), discover.ChangeRecord, discover.StatusComplete},
		{person(discover.KeyAudience, b7Audience6), discover.ChangeEdit, discover.StatusComplete},
	}
}

func b42Steps() []step {
	return []step{
		{person(discover.KeyPurpose, b42Purpose), discover.ChangeRecord, discover.StatusOpen},
		{skipped(discover.KeyAudience), discover.ChangeSkip, discover.StatusOpen},
		{person(discover.KeyConstraints, b42Constraints), discover.ChangeRecord, discover.StatusOpen},
		{doneAnswer("a", b42DoneA), discover.ChangeRecord, discover.StatusOpen},
		{person(discover.KeyOutOfScope, b42OutOfScope), discover.ChangeRecord, discover.StatusComplete},
		{doneAnswer("a", b42DoneB), discover.ChangeEdit, discover.StatusComplete},
	}
}

// ── the world ───────────────────────────────────────────────────────────────

type world struct {
	t       *testing.T
	ctx     context.Context
	hub     *fakeHub
	clock   *testClock
	repos   *fakeRepos
	locate  *fakeLocator
	team    string
	project string // the Hub id of Engagement Portal ("" in a bare world)
	b7, b42 string
	head    map[string]int                  // brief id -> head revision
	inBrief map[string]string               // brief id -> Hub project id
	answers map[string][]discover.Answer    // brief id -> the head's answers
	written map[string]hubapi.BriefRevision // key -> the fake's answer to the write
	pins    map[string]*discover.Inherit    // key -> the pin its content holds
	content map[string]discover.Content     // key -> the content written
}

// newBareWorld is the fake, one team the caller administers, and the local
// doubles, with no Hub project and no local project.
func newBareWorld(t *testing.T) *world {
	t.Helper()
	w := &world{t: t, ctx: context.Background(), hub: newFakeHub(), clock: newClock(),
		repos: &fakeRepos{byDir: map[string][]RepoKey{}}, locate: newLocator(),
		head: map[string]int{}, inBrief: map[string]string{}, answers: map[string][]discover.Answer{},
		written: map[string]hubapi.BriefRevision{}, pins: map[string]*discover.Inherit{}, content: map[string]discover.Content{}}
	team, err := w.hub.Fake.CreateTeam(w.ctx, hubapi.CreateTeamRequest{Name: teamName})
	if err != nil {
		t.Fatalf("CreateTeam: %v", err)
	}
	w.team = team.ID
	w.locate.setBranch(localBranch, "evidence-upload")
	w.locate.setBranch(localTimeout, "fix-session-timeout")
	w.locate.setBranch(localMain, "main")
	return w
}

// newWorld is the fact sheet's world, in the order its revisions were saved:
// b-7 1-5, b-42 1-5 (pinning b-7@5), b-7 6, b-42 6 (pinning b-7@6).
func newWorld(t *testing.T) *world {
	t.Helper()
	w := newBareWorld(t)
	w.project = w.createProject(projectName, teamKey)
	w.locate.addProject(localProject, projectDir, "engagement-portal")
	w.repos.set(projectDir, gh("origin", teamKey))
	w.b7 = w.createBrief(w.project, hubapi.ScopeProject, "")
	w.b42 = w.createBrief(w.project, hubapi.ScopeBranch, evidenceRef)
	s7, s42 := b7Steps(), b42Steps()
	for _, s := range s7[:5] {
		w.save(w.b7, s, nil)
	}
	for _, s := range s42[:5] {
		w.save(w.b42, s, w.pinTo(w.b7, 5))
	}
	w.save(w.b7, s7[5], nil)
	w.save(w.b42, s42[5], w.pinTo(w.b7, 6))
	return w
}

func (w *world) createProject(name string, remotes ...string) string {
	w.t.Helper()
	req := hubapi.CreateProjectRequest{TeamID: w.team, Name: name, Remotes: []hubapi.RemoteRequest{}}
	for _, r := range remotes {
		req.Remotes = append(req.Remotes, hubapi.RemoteRequest{RepoKey: r, Host: hubapi.HostGitHub})
	}
	p, err := w.hub.Fake.CreateProject(w.ctx, req)
	if err != nil {
		w.t.Fatalf("CreateProject(%s): %v", name, err)
	}
	return p.ID
}

func (w *world) createBrief(projectID, scope, ref string) string {
	w.t.Helper()
	h, err := w.hub.Fake.CreateBrief(w.ctx, projectID, hubapi.CreateBriefRequest{Scope: scope, BranchRef: ref})
	if err != nil {
		w.t.Fatalf("CreateBrief(%s %s): %v", scope, ref, err)
	}
	w.inBrief[h.ID] = projectID
	return h.ID
}

// pinTo is the Inherit a branch revision holds for revision n of briefID.
func (w *world) pinTo(briefID string, n int) *discover.Inherit {
	key := w.key(briefID, n)
	return &discover.Inherit{Key: key, Digest: w.written[key].Digest.String()}
}

func (w *world) key(briefID string, n int) string { return provenance.BriefKey(briefID, n) }

// save writes the brief's next revision: its head's answers with s.a
// recorded over the answer of the same key, through the embedded fake, so
// the wrapper's logs never see the seeding.
func (w *world) save(briefID string, s step, pin *discover.Inherit) hubapi.BriefRevision {
	w.t.Helper()
	answers := slices.Clone(w.answers[briefID])
	if i := slices.IndexFunc(answers, func(a discover.Answer) bool { return a.Key == s.a.Key }); i >= 0 {
		answers[i] = s.a
	} else {
		answers = append(answers, s.a)
	}
	c := discover.Content{V: discover.ContentVersion, Status: s.status, Questions: discover.BaseQuestions(),
		Answers: answers, Inherits: pin, Change: discover.Change{Kind: s.kind, Key: s.a.Key}}
	raw, err := json.Marshal(c)
	if err != nil {
		w.t.Fatalf("marshal content: %v", err)
	}
	revs, err := w.hub.Fake.WriteRevisions(w.ctx, w.inBrief[briefID],
		[]hubapi.RevisionWrite{{BriefID: briefID, BaseRevision: w.head[briefID], Content: raw}})
	if err != nil {
		w.t.Fatalf("WriteRevisions(%s): %v", briefID, err)
	}
	w.head[briefID]++
	w.answers[briefID] = answers
	r := revs[0]
	w.written[r.Key], w.pins[r.Key], w.content[r.Key] = r, pin, c
	return r
}

// revision is key's revision as discover reads it, from what was written.
func (w *world) revision(key string) *discover.Revision {
	w.t.Helper()
	r, ok := w.written[key]
	if !ok {
		w.t.Fatalf("no revision %s was written", key)
	}
	return &discover.Revision{BriefID: r.BriefID, Number: r.Revision, Key: r.Key,
		Scope: discover.Scope(r.Scope), BranchRef: r.BranchRef, Content: w.content[key]}
}

// document is discover.Resolve of a branch key against its pin, or of a
// project key alone: the text a key names.
func (w *world) document(key string) discover.Document {
	w.t.Helper()
	r := w.revision(key)
	var doc discover.Document
	var err error
	if r.Scope == discover.ScopeBranch {
		doc, err = discover.Resolve(w.revision(w.pins[key].Key), r)
	} else {
		doc, err = discover.Resolve(r, nil)
	}
	if err != nil {
		w.t.Fatalf("discover.Resolve(%s): %v", key, err)
	}
	return doc
}

// service is a Service over this world's wrapper; opts adjust the Config.
func (w *world) service(opts ...func(*Config)) *Service {
	w.t.Helper()
	cfg := Config{Hub: w.hub, Repos: w.repos, Locate: w.locate, Now: w.clock.Now,
		NewID: func(p string) string { return p + "-0000000000000001" }}
	for _, o := range opts {
		o(&cfg)
	}
	s, err := New(cfg)
	if err != nil {
		w.t.Fatalf("New: %v", err)
	}
	return s
}

func withEvaluate(e provenance.Evaluate) func(*Config) { return func(c *Config) { c.Evaluate = e } }
func withoutHub() func(*Config)                        { return func(c *Config) { c.Hub = nil } }

// erase is the org admin's erasure of briefID (Hub contract §7.12).
func (w *world) erase(briefID string) {
	w.t.Helper()
	admin := hubclient.FakeUser{ID: hubclient.FakeGrace.ID, DisplayName: hubclient.FakeGrace.DisplayName,
		Email: hubclient.FakeGrace.Email, OrgAdmin: true}
	if _, err := w.hub.Fake.As(admin).EraseBrief(briefID, "a test of the erased state"); err != nil {
		w.t.Fatalf("EraseBrief(%s): %v", briefID, err)
	}
}

// mark and since read the fake's own Calls() from a point on.
func (w *world) mark() int { return len(w.hub.Calls()) }

func (w *world) since(m int) []string { return w.hub.Calls()[m:] }

func (w *world) branch() Target   { return Target{ProjectID: localProject, BranchID: localBranch} }
func (w *world) projectT() Target { return Target{ProjectID: localProject} }

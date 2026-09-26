package discoverhub

import (
	"context"
	"slices"
	"sync"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubapi"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/hub/hubclient"
)

// fakeHub is phase 4's thin wrapper over the Hub's own fake. It is phase 4's,
// not the Hub's: nothing here is written under internal/hub.
//
// H0 already declares HA4-2's List on hubclient.Briefs, and the embedded
// fake answers it (hub 3df9fab8, fake_briefs.go:176-216), so the wrapper has
// no List of its own: the fake's shows through, and a Rebind, an Archive or
// SetUnavailable made through the fake reaches it. What the wrapper adds is
// four hooks H0's fake does not have (phase-h-hub-parts/task-04.md:188-226)
// and one log:
//
//   - renameProject(id, name): Project and ProjectForRepo answer Name
//     replaced, standing in for route 16, PATCH /v1/projects/{id}, which
//     hubclient.Projects has no method for.
//   - failResolve(n, err): the n-th Resolve through the wrapper (1-based,
//     counted from the hook) returns err without reaching the fake.
//   - failList(n, err): the same for List, so a walk can fail on a later
//     page while its first page answered.
//   - failCreateProject(err): the next CreateProject returns err, once.
//   - resolves(): every Resolve's keys, in order, including the ones
//     failResolve stopped, which the fake's Calls() never sees (re-review
//     b05-2 R2-I1, item 2). lists() does the same for List.
type fakeHub struct {
	*hubclient.Fake

	mu          sync.Mutex
	names       map[string]string // project id -> name, set by renameProject
	resolveSeen int
	resolveAt   int
	resolveErr  error
	listSeen    int
	listAt      int
	listErr     error
	createErr   error
	resolveLog  [][]string
	listLog     []hubapi.BriefQuery
}

func newFakeHub() *fakeHub {
	return &fakeHub{Fake: hubclient.NewFake(), names: map[string]string{}}
}

var _ Hub = (*fakeHub)(nil)

func (f *fakeHub) renameProject(id, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.names[id] = name
}

func (f *fakeHub) failResolve(n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.resolveSeen, f.resolveAt, f.resolveErr = 0, n, err
}

func (f *fakeHub) failList(n int, err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.listSeen, f.listAt, f.listErr = 0, n, err
}

func (f *fakeHub) failCreateProject(err error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.createErr = err
}

// resolves is every Resolve's keys through the wrapper, oldest first.
func (f *fakeHub) resolves() [][]string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([][]string, len(f.resolveLog))
	for i, k := range f.resolveLog {
		out[i] = slices.Clone(k)
	}
	return out
}

// lists is every List's query through the wrapper, oldest first.
func (f *fakeHub) lists() []hubapi.BriefQuery {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.listLog)
}

func (f *fakeHub) Resolve(ctx context.Context, keys []string) (hubapi.RefsResponse, error) {
	f.mu.Lock()
	f.resolveLog = append(f.resolveLog, slices.Clone(keys))
	f.resolveSeen++
	fail := f.resolveAt > 0 && f.resolveSeen == f.resolveAt
	err := f.resolveErr
	f.mu.Unlock()
	if fail {
		return hubapi.RefsResponse{}, err
	}
	return f.Fake.Resolve(ctx, keys)
}

func (f *fakeHub) List(ctx context.Context, projectID string, q hubapi.BriefQuery) (hubapi.Page[hubapi.BriefHead], error) {
	f.mu.Lock()
	f.listLog = append(f.listLog, q)
	f.listSeen++
	fail := f.listAt > 0 && f.listSeen == f.listAt
	err := f.listErr
	f.mu.Unlock()
	if fail {
		return hubapi.Page[hubapi.BriefHead]{}, err
	}
	return f.Fake.List(ctx, projectID, q)
}

func (f *fakeHub) CreateProject(ctx context.Context, req hubapi.CreateProjectRequest) (hubapi.Project, error) {
	f.mu.Lock()
	err := f.createErr
	f.createErr = nil
	f.mu.Unlock()
	if err != nil {
		return hubapi.Project{}, err
	}
	return f.Fake.CreateProject(ctx, req)
}

func (f *fakeHub) Project(ctx context.Context, projectID string) (hubapi.Project, error) {
	p, err := f.Fake.Project(ctx, projectID)
	return f.renamed(p), err
}

func (f *fakeHub) ProjectForRepo(ctx context.Context, repoKey string) (hubapi.Project, bool, error) {
	p, ok, err := f.Fake.ProjectForRepo(ctx, repoKey)
	return f.renamed(p), ok, err
}

func (f *fakeHub) renamed(p hubapi.Project) hubapi.Project {
	f.mu.Lock()
	defer f.mu.Unlock()
	if name, ok := f.names[p.ID]; ok && p.ID != "" {
		p.Name = name
	}
	return p
}

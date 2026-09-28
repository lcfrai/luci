package deploywatch_test

// The harness. Every stand-in here exists because the real thing has not landed
// in this tree: Hub H0's hubclient and hubapi, and phase 3 task 13's githost.
// They live in _test.go files on purpose.

import (
	"context"
	"io"
	"log/slog"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/connection"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploytest"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploywatch"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/project"
)

// clock is the fake time every part of the harness reads, so a cadence can be
// pinned in seconds without a test waiting any.
type clock struct {
	mu sync.Mutex
	t  time.Time
}

func newClock() *clock { return &clock{t: time.Unix(1700000000, 0).UTC()} }

func (c *clock) now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.t
}

func (c *clock) advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

type fakeLocals struct {
	mu   sync.Mutex
	list []project.Project
}

func (f *fakeLocals) Projects(context.Context) ([]project.Project, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]project.Project(nil), f.list...), nil
}

func (f *fakeLocals) drop(id string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	kept := f.list[:0]
	for _, p := range f.list {
		if p.ID != id {
			kept = append(kept, p)
		}
	}
	f.list = kept
}

type fakeRemotes struct {
	mu      sync.Mutex
	origins map[string]string
}

func (f *fakeRemotes) OriginURL(_ context.Context, path string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.origins[path], nil
}

// testParse stands in for githost.ParseRemote: good enough to prove the binding
// matches on the KEY, never on the URL.
func testParse(raw string) (binding.Remote, error) {
	s := strings.ToLower(strings.TrimSuffix(strings.TrimSpace(raw), ".git"))
	for _, p := range []string{"https://", "http://", "ssh://", "git@"} {
		s = strings.TrimPrefix(s, p)
	}
	s = strings.ReplaceAll(s, "github.com:", "github.com/")
	if strings.HasPrefix(s, "github.com/") && len(strings.Split(s, "/")) == 3 {
		return binding.Remote{RepoKey: s}, nil
	}
	return binding.Remote{}, binding.ErrUnknownHost
}

// connStub is the connection gate's answer and a record of what the watcher
// told it.
type connStub struct {
	mu       sync.Mutex
	state    connection.State
	observed []error
	checks   int
}

func (c *connStub) Observe(err error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.observed = append(c.observed, err)
}

func (c *connStub) Status() connection.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.state == "" {
		c.state = connection.Connected
	}
	return connection.Status{State: c.state}
}

func (c *connStub) Check(context.Context) connection.Status {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.checks++
	return connection.Status{State: c.state}
}

func (c *connStub) failures() []error {
	c.mu.Lock()
	defer c.mu.Unlock()
	var out []error
	for _, e := range c.observed {
		if e != nil {
			out = append(out, e)
		}
	}
	return out
}

// deployerWrap lets one method of the Fake be replaced for one test, which is
// how a hang, a refusal or a SameEnvironment of the target's own is staged.
type deployerWrap struct {
	*deploytest.Fake
	environments func(ctx context.Context, p deploy.ProjectRef) ([]deploy.Environment, error)
	deployments  func(ctx context.Context, p deploy.ProjectRef) ([]deploy.Deployment, error)
	sameEnv      func(a, b string) bool
	sameEnvSeen  *[][2]string
	mu           *sync.Mutex
}

func (d *deployerWrap) Environments(ctx context.Context, p deploy.ProjectRef) ([]deploy.Environment, error) {
	if d.environments != nil {
		return d.environments(ctx, p)
	}
	return d.Fake.Environments(ctx, p)
}

func (d *deployerWrap) Deployments(ctx context.Context, p deploy.ProjectRef) ([]deploy.Deployment, error) {
	if d.deployments != nil {
		return d.deployments(ctx, p)
	}
	return d.Fake.Deployments(ctx, p)
}

func (d *deployerWrap) SameEnvironment(a, b string) bool {
	if d.sameEnvSeen != nil {
		d.mu.Lock()
		*d.sameEnvSeen = append(*d.sameEnvSeen, [2]string{a, b})
		d.mu.Unlock()
	}
	if d.sameEnv != nil {
		return d.sameEnv(a, b)
	}
	return d.Fake.SameEnvironment(a, b)
}

func discardLog() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// harness holds everything one test needs and builds the watcher over the REAL
// binding.Resolver, so the one-Projects-read-per-cycle claim is about the code
// that ships and not about a stub.
type harness struct {
	t        *testing.T
	clock    *clock
	fake     *deploytest.Fake
	deployer deploy.Deployer
	locals   *fakeLocals
	remotes  *fakeRemotes
	conn     *connStub
	hub      *hubStub
	res      *binding.Resolver
	w        *deploywatch.Watcher
	cfg      deploywatch.Config
}

func newHarness(t *testing.T) *harness {
	t.Helper()
	c := newClock()
	f := deploytest.NewFake(c.now)
	h := &harness{
		t:       t,
		clock:   c,
		fake:    f,
		locals:  &fakeLocals{},
		remotes: &fakeRemotes{origins: map[string]string{}},
		conn:    &connStub{},
		hub:     newHubStub(),
	}
	h.deployer = f
	return h
}

// local seeds one phase 2 project with an origin URL.
func (h *harness) local(id, origin string) string {
	h.locals.list = append(h.locals.list, project.Project{ID: id, Name: id, Path: "/w/" + id})
	h.remotes.origins["/w/"+id] = origin
	return id
}

// panaceaProject seeds one deploy-target project with the repoUrl it reports.
func (h *harness) panaceaProject(ref, repoURL string) {
	h.fake.AddProject(deploy.Project{Ref: deploy.ProjectRef(ref), DisplayName: ref, RepoURL: repoURL})
}

// build makes the resolver and the watcher. cfg is the caller's amendments.
func (h *harness) build(cfg deploywatch.Config) *deploywatch.Watcher {
	h.t.Helper()
	res, err := binding.New(binding.Config{
		Projects:    h.locals,
		Remotes:     h.remotes,
		Deployer:    h.deployer,
		ParseRemote: testParse,
		ChoicesDir:  h.t.TempDir(),
		Now:         h.clock.now,
		Log:         discardLog(),
	})
	if err != nil {
		h.t.Fatalf("binding.New: %v", err)
	}
	h.res = res
	if cfg.Deployer == nil {
		cfg.Deployer = h.deployer
	}
	cfg.Projects, cfg.Bindings, cfg.Connection = h.locals, res, h.conn
	cfg.Now, cfg.Log = h.clock.now, discardLog()
	w, err := deploywatch.New(cfg)
	if err != nil {
		h.t.Fatalf("deploywatch.New: %v", err)
	}
	h.w, h.cfg = w, cfg
	return w
}

// tick runs one cycle at the current time, then advances by the active
// interval: the watcher's ticker fires every ActiveInterval, and a test drives
// the same beat without paying it.
func (h *harness) tick() {
	h.w.Cycle(context.Background())
	step := h.cfg.ActiveInterval
	if step <= 0 {
		step = deploywatch.DefaultActiveInterval
	}
	h.clock.advance(step)
}

// ticksFor runs cycles until d of fake time has passed.
func (h *harness) ticksFor(d time.Duration) {
	step := h.cfg.ActiveInterval
	if step <= 0 {
		step = deploywatch.DefaultActiveInterval
	}
	for elapsed := time.Duration(0); elapsed < d; elapsed += step {
		h.tick()
	}
}

// calls counts the Fake's recorded calls whose text holds every fragment.
func (h *harness) calls(fragments ...string) int {
	n := 0
	for _, c := range h.fake.Calls() {
		all := true
		for _, f := range fragments {
			if !strings.Contains(c, f) {
				all = false
				break
			}
		}
		if all {
			n++
		}
	}
	return n
}

// ship starts a run on the Fake the way task 09 will, so a tracked run is a
// run the target really has.
func (h *harness) ship(ref, branch string) string {
	h.t.Helper()
	res, err := h.fake.Ship(context.Background(), deploy.ProjectRef(ref), deploy.ShipRequest{
		Branch: branch,
		Commit: "0123456789abcdef0123456789abcdef01234567",
	})
	if err != nil {
		h.t.Fatalf("fake.Ship: %v", err)
	}
	return res.Run
}

func (h *harness) snapshot(id string) deploywatch.Snapshot {
	h.t.Helper()
	s, ok := h.w.Snapshot(id)
	if !ok {
		h.t.Fatalf("no snapshot for %q", id)
	}
	return s
}

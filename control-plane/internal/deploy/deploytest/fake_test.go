package deploytest_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploytest"
)

const (
	proj   = deploy.ProjectRef("engagement-portal")
	commit = "0123456789abcdef0123456789abcdef01234567"
)

var base = time.Date(2026, 9, 25, 10, 0, 0, 0, time.UTC)

// testClock is the Fake's injectable clock: tests move it, nothing reads the wall.
type testClock struct {
	mu sync.Mutex
	t  time.Time
}

func (c *testClock) now() time.Time { c.mu.Lock(); defer c.mu.Unlock(); return c.t }
func (c *testClock) add(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.t = c.t.Add(d)
}

// fakeSeeder arranges Conformance's state on a Fake through its exported seeding methods only.
type fakeSeeder struct{ f *deploytest.Fake }

func (s fakeSeeder) Project(p deploy.Project) { s.f.AddProject(p) }
func (s fakeSeeder) Environments(p deploy.ProjectRef, envs []deploy.Environment) {
	s.f.SetEnvironments(p, envs)
}
func (s fakeSeeder) BranchEnvironment(p deploy.ProjectRef, branch, env string) {
	s.f.SetBranchEnvironment(p, branch, env)
}
func (s fakeSeeder) Deployment(p deploy.ProjectRef, d deploy.Deployment) { s.f.AddDeployment(p, d) }
func (s fakeSeeder) NoDeployments(p deploy.ProjectRef)                   { s.f.ClearDeployments(p) }
func (s fakeSeeder) RollbackTarget(p deploy.ProjectRef, id string, w deploy.Warmth) {
	s.f.SetRollbackTarget(p, id, w, 0)
}
func (s fakeSeeder) Scans(p deploy.ProjectRef, env, id string, r deploy.ScanReport) {
	s.f.SetScans(p, env, id, r)
}
func (s fakeSeeder) PostureWord(p deploy.ProjectRef, branch, word string) {
	s.f.SetPostureWord(p, branch, word)
}

var _ deploytest.Seeder = fakeSeeder{}

func fakeHarness(t *testing.T) (deploy.Deployer, deploytest.Seeder) {
	f := deploytest.NewFake((&testClock{t: base}).now)
	return f, fakeSeeder{f}
}

func TestFakeConformance(t *testing.T) {
	deploytest.Conformance(t, fakeHarness)
}

// newShipFake is a Fake with one project whose branch feat/x deploys to feat-x.
func newShipFake(c *testClock) *deploytest.Fake {
	f := deploytest.NewFake(c.now)
	f.AddProject(deploy.Project{Ref: proj, DefaultBranch: "main"})
	f.SetBranchEnvironment(proj, "feat/x", "feat-x")
	return f
}

func TestFakeFailNextIsOneShot(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	e := &deploy.Error{Kind: deploy.KindUnavailable, Op: "ship", Status: 503}
	f.FailNext("Ship", e)
	if _, err := f.Ship(ctx, proj, deploy.ShipRequest{Branch: "feat/x", Commit: commit}); err != e {
		t.Fatalf("first Ship: err = %v, want the armed error", err)
	}
	res, err := f.Ship(ctx, proj, deploy.ShipRequest{Branch: "feat/x", Commit: commit})
	if err != nil || res.Run == "" {
		t.Fatalf("second Ship: (%+v, %v), want a named run and no error", res, err)
	}
}

// TestFakeFailNextQueuesAndIsPerMethod: two arms fail two calls, in order, and arming one
// method never fails another.
func TestFakeFailNextQueuesAndIsPerMethod(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	e1 := &deploy.Error{Kind: deploy.KindUnavailable, Op: "environments"}
	e2 := &deploy.Error{Kind: deploy.KindForbidden, Op: "environments"}
	f.FailNext("Environments", e1)
	f.FailNext("Environments", e2)
	if _, err := f.Deployments(ctx, proj); err != nil {
		t.Fatalf("Deployments failed on an Environments arm: %v", err)
	}
	for i, want := range []error{e1, e2, nil} {
		if _, err := f.Environments(ctx, proj); err != want {
			t.Fatalf("Environments call %d: err = %v, want %v", i+1, err, want)
		}
	}
}

// TestFakeFailNextNamesAMethod: a misspelt method name panics instead of arming nothing, which
// would let a retry test pass without ever exercising the retry.
func TestFakeFailNextNamesAMethod(t *testing.T) {
	for _, name := range []string{"ship", "Deploy", ""} {
		func() {
			defer func() {
				if recover() == nil {
					t.Errorf("FailNext(%q) did not panic", name)
				}
			}()
			deploytest.NewFake(nil).FailNext(name, errors.New("x"))
		}()
	}
}

// TestFakeBeforeNextRunsOnceOutsideTheLock: tasks 09 and 10 act from inside a Ship or Rollback
// (a second Ship, a retry, a cancel). The hook gets the call's context, runs without the Fake's
// lock (so it may call the Fake), and runs once.
func TestFakeBeforeNextRunsOnceOutsideTheLock(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	type key struct{}
	ctx := context.WithValue(context.Background(), key{}, "the call's")
	runs := 0
	f.BeforeNext("Ship", func(hctx context.Context) {
		runs++
		if hctx.Value(key{}) != "the call's" {
			t.Errorf("the hook did not get the call's context")
		}
		if _, err := f.Deployments(hctx, proj); err != nil {
			t.Errorf("Deployments inside the hook: %v", err)
		}
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		for i := 0; i < 2; i++ {
			if _, err := f.Ship(ctx, proj, deploy.ShipRequest{Branch: "feat/x", Commit: commit}); err != nil {
				t.Errorf("Ship %d: %v", i+1, err)
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Ship did not return: the hook ran under the Fake's lock")
	}
	if runs != 1 {
		t.Errorf("the hook ran %d times, want 1", runs)
	}
}

// TestFakeHonoursACancelledContext: a call whose context is done answers KindUnavailable with
// the context's error and changes nothing, as a transport would. Task 10's cancel-in-hook case
// relies on it to tell a detached act from one the desktop's abort cancelled.
func TestFakeHonoursACancelledContext(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx, cancel := context.WithCancel(context.Background())
	f.BeforeNext("Ship", func(context.Context) { cancel() })
	_, err := f.Ship(ctx, proj, deploy.ShipRequest{Branch: "feat/x", Commit: commit})
	if deploy.KindOf(err) != deploy.KindUnavailable || !errors.Is(err, context.Canceled) {
		t.Fatalf("Ship on a cancelled context: err = %v, want KindUnavailable wrapping context.Canceled", err)
	}
	if ds, _ := f.Deployments(context.Background(), proj); len(ds) != 0 {
		t.Errorf("a cancelled Ship added %d deployments", len(ds))
	}
}

// TestFakeCallsRecordArgumentsNeverThePassword: Calls is "Method(project,args…)" in call order,
// the shape tasks 07 and 12 assert (Scans(engagement-portal,feat-x,d-feat)). A posture password
// is never logged, stored or echoed, so it is not in Calls either. SameEnvironment is the
// target's rule, not a call to it, and is not recorded.
func TestFakeCallsRecordArgumentsNeverThePassword(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	f.SetScans(proj, "feat-x", "d-feat", deploy.ScanReport{DeploymentID: "d-feat"})
	_, _ = f.Scans(ctx, proj, "feat-x", "d-feat")
	_ = f.SameEnvironment("a", "b")
	res, err := f.SetPosture(ctx, proj, "feat/x", deploy.PostureChange{Mode: deploy.PosturePassword, Password: "hunter2-secret"})
	// Unarmed, a change is enforced (SetPostureEnforced's default) and says so; the answer never
	// echoes the password (review t01 I-2, mutant MB).
	if err != nil || !res.Enforced || res.Message == "" || strings.Contains(fmt.Sprintf("%+v", res), "hunter2") {
		t.Fatalf("SetPosture on an unarmed Fake = (%+v, %v), want Enforced with a message and no password", res, err)
	}
	_, _, _ = f.Latest(ctx, proj)
	want := []string{
		"Scans(engagement-portal,feat-x,d-feat)",
		"SetPosture(engagement-portal,feat/x,password)",
		"Latest(engagement-portal)",
	}
	if got := f.Calls(); !slices.Equal(got, want) {
		t.Fatalf("Calls() = %q, want %q", got, want)
	}
	for _, c := range f.Calls() {
		if strings.Contains(c, "hunter2") {
			t.Fatalf("Calls() holds the password: %q", c)
		}
	}
	p, err := f.Posture(ctx, proj, "feat/x")
	if err != nil || !p.PasswordSet || p.Mode != deploy.PosturePassword {
		t.Fatalf("Posture after a password change = (%+v, %v)", p, err)
	}
}

// TestFakeEnvironmentForDerivesNothing: an unseeded branch is refused with a message that names
// it; the Fake holds no copy of any target's naming rule, and Ship of that branch is refused too.
func TestFakeEnvironmentForDerivesNothing(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	for _, branch := range []string{"feat-y", "main"} {
		env, err := f.EnvironmentFor(ctx, proj, branch)
		var de *deploy.Error
		if !errors.As(err, &de) || de.Kind != deploy.KindRejected || !strings.Contains(de.Message, branch) {
			t.Errorf("EnvironmentFor(%q) = (%q, %v), want KindRejected naming the branch", branch, env, err)
		}
	}
	if _, err := f.Ship(ctx, proj, deploy.ShipRequest{Branch: "feat-y", Commit: commit}); deploy.KindOf(err) != deploy.KindRejected {
		t.Errorf("Ship of an unseeded branch: err = %v, want KindRejected", err)
	}
	if env, err := f.EnvironmentFor(ctx, proj, "feat/x"); err != nil || env != "feat-x" {
		t.Errorf("EnvironmentFor(feat/x) = (%q, %v), want the seeded feat-x", env, err)
	}
}

// TestFakeScansKnowsNoAliases: the key is the exact env string. "production" is not "prod" to
// the Fake, so a caller that renames env fails loudly (review C3).
func TestFakeScansKnowsNoAliases(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	f.SetScans(proj, "prod", "d1", deploy.ScanReport{DeploymentID: "d1", Overall: "PASS"})
	for _, env := range []string{"production", "", "Prod"} {
		if _, err := f.Scans(ctx, proj, env, "d1"); deploy.KindOf(err) != deploy.KindNotFound {
			t.Errorf("Scans(%q, d1): err = %v, want KindNotFound", env, err)
		}
	}
	if r, err := f.Scans(ctx, proj, "prod", "d1"); err != nil || r.Overall != "PASS" {
		t.Errorf("Scans(prod, d1) = (%+v, %v)", r, err)
	}
}

// TestFakeShipRefusesWhatPhase7NeverSends: a symbolic ref or an empty branch is a caller's bug.
func TestFakeShipRefusesWhatPhase7NeverSends(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	for _, req := range []deploy.ShipRequest{
		{Branch: "feat/x", Commit: "main"},
		{Branch: "feat/x", Commit: strings.ToUpper(commit)},
		{Branch: "", Commit: commit},
	} {
		if _, err := f.Ship(ctx, proj, req); deploy.KindOf(err) != deploy.KindRejected {
			t.Errorf("Ship(%+v): err = %v, want KindRejected", req, err)
		}
	}
	if ds, _ := f.Deployments(ctx, proj); len(ds) != 0 {
		t.Errorf("refused Ships added %d deployments", len(ds))
	}
}

// TestFakeShipAdvanceCancel walks one run: Ship makes a pending deployment whose ID is the run,
// in the seeded environment, stamped by the injected clock; Advance moves run and row; a
// terminal state stamps CompletedAt; Cancel of a finished run is not an error.
func TestFakeShipAdvanceCancel(t *testing.T) {
	c := &testClock{t: base}
	f := newShipFake(c)
	ctx := context.Background()
	res, err := f.Ship(ctx, proj, deploy.ShipRequest{Branch: "feat/x", Commit: commit})
	if err != nil || !strings.HasPrefix(res.Run, "run-") {
		t.Fatalf("Ship = (%+v, %v)", res, err)
	}
	d, ok, err := f.Latest(ctx, proj)
	if err != nil || !ok || d.ID != res.Run || d.Build != deploy.BuildPending || d.Environment != "feat-x" ||
		d.Commit != commit || !d.CreatedAt.Equal(base) || !d.CompletedAt.IsZero() {
		t.Fatalf("Latest after Ship = (%+v, %v, %v)", d, ok, err)
	}
	f.Advance(proj, res.Run, deploy.BuildRunning)
	if rp, err := f.Run(ctx, proj, res.Run); err != nil || rp.Build != deploy.BuildRunning {
		t.Fatalf("Run after Advance(running) = (%+v, %v)", rp, err)
	}
	// The row follows every state, not only a terminal one: the target's history shows a running
	// build as running, and task 07's watcher reads it (review t01 I-2, mutant MA).
	if d, _, _ := f.Latest(ctx, proj); d.Build != deploy.BuildRunning || d.BuildRaw != "running" || !d.CompletedAt.IsZero() {
		t.Fatalf("Latest after Advance(running) = %+v, want the row running and not finished", d)
	}
	c.add(4*time.Minute + 12*time.Second)
	f.Advance(proj, res.Run, deploy.BuildSucceeded)
	d, _, _ = f.Latest(ctx, proj)
	if d.Build != deploy.BuildSucceeded || !d.CompletedAt.Equal(base.Add(4*time.Minute+12*time.Second)) {
		t.Fatalf("Latest after Advance(succeeded) = %+v", d)
	}
	if cr, err := f.Cancel(ctx, proj, res.Run); err != nil || cr.Outcome != deploy.AlreadyFinished {
		t.Fatalf("Cancel of a finished run = (%+v, %v), want AlreadyFinished", cr, err)
	}
	second, _ := f.Ship(ctx, proj, deploy.ShipRequest{Branch: "feat/x", Commit: commit})
	if second.Run == res.Run {
		t.Fatalf("two Ships named one run %q", second.Run)
	}
	// A run accepted but not started cannot be cancelled yet: Panacea answers already_completed
	// for a run with no condition and stops nothing (handlers/projects.go:3669). The refusal is
	// KindConflict with a sentence, and moves nothing (review t04 I2).
	cr, err := f.Cancel(ctx, proj, second.Run)
	if e := (*deploy.Error)(nil); !errors.As(err, &e) || e.Kind != deploy.KindConflict || e.Message == "" || cr != (deploy.CancelResult{}) {
		t.Fatalf("Cancel of a pending run = (%+v, %v), want KindConflict with a sentence", cr, err)
	}
	if rp, _ := f.Run(ctx, proj, second.Run); rp.Build != deploy.BuildPending {
		t.Fatalf("a refused Cancel moved the pending run to %q", rp.Build)
	}
	f.Advance(proj, second.Run, deploy.BuildRunning)
	if cr, err := f.Cancel(ctx, proj, second.Run); err != nil || cr.Outcome != deploy.Cancelled {
		t.Fatalf("Cancel of a running run = (%+v, %v), want Cancelled", cr, err)
	}
	if rp, _ := f.Run(ctx, proj, second.Run); rp.Build != deploy.BuildFailed {
		t.Errorf("a cancelled run reads %q, want failed (the target does not tell them apart)", rp.Build)
	}
	// A cancelled build finished: its row is failed, stamped by the clock, as the target records
	// a cancelled run's duration.
	if d, _, _ := f.Latest(ctx, proj); d.ID != second.Run || d.Build != deploy.BuildFailed ||
		!d.CompletedAt.Equal(base.Add(4*time.Minute+12*time.Second)) {
		t.Errorf("Latest after Cancel = %+v, want the cancelled row failed and completed now", d)
	}
	// Cancel of a run that is now failed is a finished run, not a second cancel (mutant ME).
	if cr, err := f.Cancel(ctx, proj, second.Run); err != nil || cr.Outcome != deploy.AlreadyFinished {
		t.Errorf("Cancel of a cancelled run = (%+v, %v), want AlreadyFinished", cr, err)
	}
	f.AddProject(deploy.Project{Ref: "other"})
	if _, err := f.Run(ctx, "other", res.Run); deploy.KindOf(err) != deploy.KindNotFound {
		t.Errorf("Run of another project's run: err = %v, want KindNotFound", err)
	}
}

// TestFakeTieGoesToTheLaterAdd: newest first is by CreatedAt, and at one instant the later add
// is the newer. Two Ships on a frozen clock make the second one Latest, as a watcher expects.
func TestFakeTieGoesToTheLaterAdd(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	f.AddDeployment(proj, deploy.Deployment{ID: "d-a", Environment: "feat-x", CreatedAt: base})
	f.AddDeployment(proj, deploy.Deployment{ID: "d-b", Environment: "feat-x", CreatedAt: base})
	f.AddDeployment(proj, deploy.Deployment{ID: "d-old", Environment: "feat-x", CreatedAt: base.Add(-time.Hour)})
	ds, _ := f.Deployments(ctx, proj)
	var got []string
	for _, d := range ds {
		got = append(got, d.ID)
	}
	if want := []string{"d-b", "d-a", "d-old"}; !slices.Equal(got, want) {
		t.Fatalf("Deployments order = %q, want %q", got, want)
	}
	res, _ := f.Ship(ctx, proj, deploy.ShipRequest{Branch: "feat/x", Commit: commit})
	if d, _, _ := f.Latest(ctx, proj); d.ID != res.Run {
		t.Fatalf("Latest after a Ship at the newest instant = %q, want %q", d.ID, res.Run)
	}
}

// TestFakeReseedMovesTheRun: AddDeployment of an ID that exists replaces the row, and the run
// follows the row, so Run and Latest never give two answers for one build, and Cancel decides
// from the state the row shows (review t01 I-1). Tasks 07 and 10 move builds on by re-seeding.
func TestFakeReseedMovesTheRun(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	later := base.Add(time.Minute)
	f.AddDeployment(proj, deploy.Deployment{ID: "d1", Environment: "feat-x", Build: deploy.BuildRunning,
		BuildRaw: "running", CreatedAt: base})
	f.AddDeployment(proj, deploy.Deployment{ID: "d1", Environment: "feat-x", Build: deploy.BuildFailed,
		BuildRaw: "failed", FailedStep: "build", ErrorText: "exit status 1", CreatedAt: later})
	rp, err := f.Run(ctx, proj, "d1")
	if err != nil || rp.Run != "d1" || rp.Build != deploy.BuildFailed || rp.BuildRaw != "failed" ||
		rp.FailedStep != "build" || rp.ErrorText != "exit status 1" || !rp.StartedAt.Equal(later) {
		t.Fatalf("Run after re-seeding d1 as failed = (%+v, %v), want the re-seeded row's state", rp, err)
	}
	f.AddDeployment(proj, deploy.Deployment{ID: "d1", Environment: "feat-x", Build: deploy.BuildSucceeded,
		BuildRaw: "succeeded", CreatedAt: later, CompletedAt: later.Add(time.Minute)})
	if rp, _ := f.Run(ctx, proj, "d1"); rp.Build != deploy.BuildSucceeded || rp.FailedStep != "" || rp.ErrorText != "" {
		t.Fatalf("Run after re-seeding d1 as succeeded = %+v, want succeeded with no failed step", rp)
	}
	if cr, err := f.Cancel(ctx, proj, "d1"); err != nil || cr.Outcome != deploy.AlreadyFinished {
		t.Errorf("Cancel of a re-seeded succeeded run = (%+v, %v), want AlreadyFinished", cr, err)
	}
	if d, _, _ := f.Latest(ctx, proj); d.Build != deploy.BuildSucceeded || d.BuildRaw != "succeeded" {
		t.Errorf("after Cancel the re-seeded succeeded row reads %+v, want it untouched", d)
	}
	// A re-seed REPLACES the row: three seeds of d1 are one row in the history, not three, or a
	// history reader counts one build three times (fix round 1, mutant P2).
	if ds, _ := f.Deployments(ctx, proj); len(ds) != 1 || ds[0].ID != "d1" || ds[0].Build != deploy.BuildSucceeded {
		t.Errorf("Deployments after three seeds of d1 = %+v, want the one succeeded row", ds)
	}
}

// TestFakeClearDeploymentsForgetsTheRuns: a project cleared to "never deployed" has no runs
// either, so Run and Cancel of a run it held are KindNotFound, never an answer about a build with
// no row (the I-1 class: two answers for one run; fix round 1, mutant P1).
func TestFakeClearDeploymentsForgetsTheRuns(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	f.AddDeployment(proj, deploy.Deployment{ID: "d1", Environment: "feat-x", Build: deploy.BuildRunning,
		BuildRaw: "running", CreatedAt: base})
	f.ClearDeployments(proj)
	if d, ok, err := f.Latest(ctx, proj); ok || err != nil {
		t.Fatalf("Latest after ClearDeployments = (%+v, %v, %v), want never deployed", d, ok, err)
	}
	if rp, err := f.Run(ctx, proj, "d1"); deploy.KindOf(err) != deploy.KindNotFound {
		t.Errorf("Run of a cleared run = (%+v, %v), want KindNotFound", rp, err)
	}
	if cr, err := f.Cancel(ctx, proj, "d1"); deploy.KindOf(err) != deploy.KindNotFound {
		t.Errorf("Cancel of a cleared run = (%+v, %v), want KindNotFound", cr, err)
	}
}

// TestFakeShipNeverReplacesASeededDeployment: history seeded under a name the Fake's own runs use
// ("run-1") is kept; the Ship takes the next free name, so it never overwrites a seeded row.
func TestFakeShipNeverReplacesASeededDeployment(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	f.AddDeployment(proj, deploy.Deployment{ID: "run-1", Environment: "feat-x", Build: deploy.BuildSucceeded,
		BuildRaw: "succeeded", CreatedAt: base.Add(-time.Hour)})
	res, err := f.Ship(ctx, proj, deploy.ShipRequest{Branch: "feat/x", Commit: commit})
	if err != nil || res.Run == "" || res.Run == "run-1" {
		t.Fatalf("Ship = (%+v, %v), want a run name no seeded row holds", res, err)
	}
	ds, _ := f.Deployments(ctx, proj)
	if len(ds) != 2 || ds[0].ID != res.Run || ds[1].ID != "run-1" || ds[1].Build != deploy.BuildSucceeded {
		t.Fatalf("Deployments after the Ship = %+v, want the Ship's row, then run-1 intact", ds)
	}
	if rp, _ := f.Run(ctx, proj, "run-1"); rp.Build != deploy.BuildSucceeded {
		t.Errorf("the seeded run-1 reads %+v after a Ship, want succeeded", rp)
	}
}

// TestFakeCancelOfAFinishedRunChangesNothing: succeeded, failed and historical are finished, so
// Cancel answers AlreadyFinished and moves neither the run nor the row (review t01 I-2, ME).
func TestFakeCancelOfAFinishedRunChangesNothing(t *testing.T) {
	c := &testClock{t: base}
	f := newShipFake(c)
	ctx := context.Background()
	done := base.Add(3 * time.Minute)
	for i, b := range []deploy.BuildState{deploy.BuildSucceeded, deploy.BuildFailed, deploy.BuildHistorical} {
		f.AddDeployment(proj, deploy.Deployment{ID: string(b), Environment: fmt.Sprintf("env-%d", i), Build: b,
			BuildRaw: string(b), CreatedAt: base, CompletedAt: done})
	}
	c.add(time.Hour)
	for _, b := range []deploy.BuildState{deploy.BuildSucceeded, deploy.BuildFailed, deploy.BuildHistorical} {
		if cr, err := f.Cancel(ctx, proj, string(b)); err != nil || cr.Outcome != deploy.AlreadyFinished || cr.Message == "" {
			t.Errorf("Cancel of a %s run = (%+v, %v), want AlreadyFinished with a sentence", b, cr, err)
		}
		if rp, _ := f.Run(ctx, proj, string(b)); rp.Build != b {
			t.Errorf("Cancel moved the %s run to %q", b, rp.Build)
		}
	}
	ds, _ := f.Deployments(ctx, proj)
	for _, d := range ds {
		if string(d.Build) != d.ID || !d.CompletedAt.Equal(done) {
			t.Errorf("Cancel of a finished run changed its row: %+v", d)
		}
	}
}

// TestFakeAdvanceKeepsARecordedCompletedAt: succeeded and failed stamp a ZERO CompletedAt with the
// clock; a recorded one is kept, and historical stamps nothing (review t01 I-2, mutant MG).
func TestFakeAdvanceKeepsARecordedCompletedAt(t *testing.T) {
	c := &testClock{t: base}
	f := newShipFake(c)
	ctx := context.Background()
	done := base.Add(3 * time.Minute)
	f.AddDeployment(proj, deploy.Deployment{ID: "d-done", Environment: "feat-a", Build: deploy.BuildSucceeded,
		BuildRaw: "succeeded", CreatedAt: base, CompletedAt: done})
	f.AddDeployment(proj, deploy.Deployment{ID: "d-hist", Environment: "feat-b", Build: deploy.BuildRunning,
		BuildRaw: "running", CreatedAt: base})
	c.add(time.Hour)
	f.Advance(proj, "d-done", deploy.BuildSucceeded)
	f.Advance(proj, "d-hist", deploy.BuildHistorical)
	ds, _ := f.Deployments(ctx, proj)
	for _, d := range ds {
		switch {
		case d.ID == "d-done" && !d.CompletedAt.Equal(done):
			t.Errorf("Advance(succeeded) of a finished row moved CompletedAt from %v to %v", done, d.CompletedAt)
		case d.ID == "d-hist" && (d.Build != deploy.BuildHistorical || !d.CompletedAt.IsZero()):
			t.Errorf("Advance(historical) = %+v, want historical with no CompletedAt", d)
		}
	}
}

// TestFakeBeforeNextBindsArmsAtEntry: a call takes the FailNext arm made before it began. An arm
// made inside its hook fails the next call to begin, and a same-method call the hook makes does
// not take the outer call's arm (review t01 M-5; tasks 09 and 10 build on this hook).
func TestFakeBeforeNextBindsArmsAtEntry(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	req := deploy.ShipRequest{Branch: "feat/x", Commit: commit}
	outer := &deploy.Error{Kind: deploy.KindConflict, Op: "ship", Status: 409}
	later := &deploy.Error{Kind: deploy.KindUnavailable, Op: "ship", Status: 503}
	var inner error = errors.New("the hook did not run")
	f.FailNext("Ship", outer)
	f.BeforeNext("Ship", func(hctx context.Context) {
		_, inner = f.Ship(hctx, proj, req)
		f.FailNext("Ship", later)
	})
	if _, err := f.Ship(ctx, proj, req); err != outer {
		t.Fatalf("the outer Ship: err = %v, want the arm made before it began", err)
	}
	if inner != nil {
		t.Errorf("the hook's own Ship: err = %v, want success: the outer call had taken its arm", inner)
	}
	if _, err := f.Ship(ctx, proj, req); err != later {
		t.Errorf("the Ship after: err = %v, want the arm made inside the hook", err)
	}
}

// TestFakeReturnsCopies: neither the seeding test, editing its own values after the seed, nor a
// caller, editing what it read, can change the Fake's state (review t01 I-2, mutant MC).
func TestFakeReturnsCopies(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	zero, crit, highs, findings := 0, 2, 3, 5
	gates := map[string]deploy.Gate{"iac": {Verdict: "failed", Criticals: &crit, Highs: &highs, Findings: &findings}}
	prov := []deploy.Provisioning{{Component: "database", Status: "succeeded"}}
	f.AddDeployment(proj, deploy.Deployment{ID: "d1", Environment: "feat-x", CreatedAt: base,
		LiveReady: &zero, Gates: gates, Provisioning: prov})
	unchanged := func(when string) {
		t.Helper()
		for _, read := range []string{"Deployments", "Latest"} {
			var d deploy.Deployment
			if read == "Deployments" {
				ds, _ := f.Deployments(ctx, proj)
				d = ds[0]
			} else {
				d, _, _ = f.Latest(ctx, proj)
			}
			iac := d.Gates["iac"]
			if *d.LiveReady != 0 || *iac.Criticals != 2 || *iac.Highs != 3 || *iac.Findings != 5 || len(d.Gates) != 1 ||
				d.Provisioning[0].Status != "succeeded" {
				t.Fatalf("%s: %s reads %+v, want LiveReady 0, iac 2/3/5, one gate, database succeeded", when, read, d)
			}
		}
	}
	zero, crit, highs, findings = 7, 7, 7, 7
	gates["sast"] = deploy.Gate{Verdict: "passed"}
	prov[0].Status = "failed"
	unchanged("after the seeding test changed its values")
	ds, _ := f.Deployments(ctx, proj)
	*ds[0].LiveReady = 9
	*ds[0].Gates["iac"].Criticals, *ds[0].Gates["iac"].Highs, *ds[0].Gates["iac"].Findings = 9, 9, 9
	ds[0].Gates["sast"] = deploy.Gate{Verdict: "passed"}
	ds[0].Provisioning[0].Status = "failed"
	d, _, _ := f.Latest(ctx, proj)
	*d.LiveReady = 9
	*d.Gates["iac"].Criticals, *d.Gates["iac"].Highs, *d.Gates["iac"].Findings = 9, 9, 9
	d.Provisioning[0].Status = "failed"
	unchanged("after a caller changed what it read")
}

// TestFakeSeedersCopyInAndOut: every other seeder that takes a pointer, a map or a slice keeps
// its own copy, in and out, so "deep-copied on the way in and the way out" holds for each one.
func TestFakeSeedersCopyInAndOut(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	two, four, code, yes, explicit := 2, 4, 137, true, true
	envs := []deploy.Environment{{Name: "feat-x", Replicas: &deploy.Replicas{Ready: 1, Desired: 1}}}
	counts := map[string]int{"critical": 1}
	report := deploy.ScanReport{DeploymentID: "d1", DetailsRetained: &yes,
		Blocking:   []deploy.ScanNote{{Scan: "iac", Reason: "criticals"}},
		Incomplete: []deploy.ScanNote{{Scan: "dast", Reason: "not run"}},
		Scans:      []deploy.Scan{{Name: "iac", Status: "FAIL", Counts: counts}}}
	diag := deploy.Diagnosis{Health: "down", Findings: []deploy.Finding{{Problem: "crash", Evidence: []string{"OOMKilled"}}}}
	stream := []deploy.StreamEvent{{Log: &deploy.LogLine{Step: "build", Line: "one"}},
		{Readiness: &deploy.ReadinessTick{Measured: true, Ready: &two, Restarts: &four, ExitCode: &code}}}
	grants := []deploy.AccessGrant{{Grantee: "ada", Group: "ada", Role: deploy.RoleOwner, RoleRaw: "owner"}}
	pods := []deploy.Pod{{Name: "app-1", Phase: "waiting"}}
	events := []deploy.Event{{Type: "Warning", Reason: "BackOff"}}
	f.SetEnvironments(proj, envs)
	f.SetScans(proj, "feat-x", "d1", report)
	f.SetDiagnosis(proj, "feat-x", diag)
	f.SetStream(proj, deploy.StreamReadiness, "feat-x", stream)
	f.SetAutoDeployState(proj, deploy.AutoDeploy{Branch: "feat/x", Explicit: &explicit})
	f.SetAccess(proj, grants)
	f.SetPods(proj, "feat-x", pods)
	f.SetEvents(proj, "feat-x", events)
	unchanged := func(when string) {
		t.Helper()
		if e, _ := f.Environments(ctx, proj); len(e) != 1 || e[0].Replicas.Ready != 1 {
			t.Errorf("%s: Environments = %+v, want feat-x 1 ready", when, e)
		}
		if r, _ := f.Scans(ctx, proj, "feat-x", "d1"); len(r.Scans) != 1 || r.Scans[0].Status != "FAIL" ||
			r.Scans[0].Counts["critical"] != 1 || len(r.Scans[0].Counts) != 1 || !*r.DetailsRetained ||
			r.Blocking[0].Reason != "criticals" || r.Incomplete[0].Reason != "not run" {
			t.Errorf("%s: Scans = %+v, want iac FAIL with 1 critical, details retained, both notes", when, r)
		}
		if dg, _ := f.Diagnosis(ctx, proj, "feat-x"); dg.Findings[0].Evidence[0] != "OOMKilled" {
			t.Errorf("%s: Diagnosis = %+v, want the OOMKilled evidence", when, dg)
		}
		s, _ := f.Stream(ctx, proj, deploy.StreamReadiness, "feat-x")
		ev1, _ := s.Next()
		ev2, _ := s.Next()
		if ev1.Log == nil || ev1.Log.Line != "one" || ev2.Readiness == nil || *ev2.Readiness.Ready != 2 ||
			*ev2.Readiness.Restarts != 4 || *ev2.Readiness.ExitCode != 137 {
			t.Errorf("%s: the stream yields %+v then %+v, want line one then 2 ready, 4 restarts, exit 137", when, ev1, ev2)
		}
		if a, _ := f.AutoDeploy(ctx, proj, "feat/x"); a.Explicit == nil || !*a.Explicit {
			t.Errorf("%s: AutoDeploy = %+v, want explicitly on", when, a)
		}
		if g, _ := f.Access(ctx, proj); len(g) != 1 || g[0].Role != deploy.RoleOwner {
			t.Errorf("%s: Access = %+v, want ada as owner", when, g)
		}
		if p, _ := f.Pods(ctx, proj, "feat-x"); len(p) != 1 || p[0].Name != "app-1" {
			t.Errorf("%s: Pods = %+v, want app-1", when, p)
		}
		if e, _ := f.Events(ctx, proj, "feat-x"); len(e) != 1 || e[0].Reason != "BackOff" {
			t.Errorf("%s: Events = %+v, want BackOff", when, e)
		}
	}
	envs[0].Replicas.Ready, counts["critical"], counts["high"], yes = 9, 9, 9, false
	report.Scans[0].Status, report.Blocking[0].Reason, report.Incomplete[0].Reason = "PASS", "changed", "changed"
	diag.Findings[0].Evidence[0] = "changed"
	stream[0].Log.Line, two, four, code, explicit = "changed", 9, 9, 9, false
	grants[0].Role, pods[0].Name, events[0].Reason = deploy.RoleViewer, "changed", "changed"
	unchanged("after the seeding test changed its values")
	e, _ := f.Environments(ctx, proj)
	e[0].Replicas.Ready = 9
	r, _ := f.Scans(ctx, proj, "feat-x", "d1")
	r.Scans[0].Status, r.Scans[0].Counts["critical"], *r.DetailsRetained = "PASS", 9, false
	r.Blocking[0].Reason, r.Incomplete[0].Reason = "changed", "changed"
	dg, _ := f.Diagnosis(ctx, proj, "feat-x")
	dg.Findings[0].Evidence[0] = "changed"
	s, _ := f.Stream(ctx, proj, deploy.StreamReadiness, "feat-x")
	ev1, _ := s.Next()
	ev2, _ := s.Next()
	ev1.Log.Line, *ev2.Readiness.Ready, *ev2.Readiness.Restarts, *ev2.Readiness.ExitCode = "changed", 9, 9, 9
	a, _ := f.AutoDeploy(ctx, proj, "feat/x")
	*a.Explicit = false
	g, _ := f.Access(ctx, proj)
	g[0].Role = deploy.RoleViewer
	p, _ := f.Pods(ctx, proj, "feat-x")
	p[0].Name = "changed"
	ev, _ := f.Events(ctx, proj, "feat-x")
	ev[0].Reason = "changed"
	unchanged("after a caller changed what it read")
}

// TestFakeUnknownProjectIsNotFound: a project nothing seeded does not exist, so a deleted
// project never reads as an empty one.
func TestFakeUnknownProjectIsNotFound(t *testing.T) {
	f := deploytest.NewFake(nil)
	ctx := context.Background()
	if _, err := f.Environments(ctx, "gone"); deploy.KindOf(err) != deploy.KindNotFound {
		t.Errorf("Environments(gone): err = %v, want KindNotFound", err)
	}
	if _, ok, err := f.Latest(ctx, "gone"); ok || deploy.KindOf(err) != deploy.KindNotFound {
		t.Errorf("Latest(gone): ok = %v, err = %v, want KindNotFound", ok, err)
	}
	if _, err := f.Rollback(ctx, proj, "d1"); deploy.KindOf(err) != deploy.KindNotFound {
		t.Errorf("Rollback of an unknown deployment: err = %v, want KindNotFound", err)
	}
}

// TestFakeIsSafeForConcurrentUse is for -race: the watcher reads while Ship acts.
func TestFakeIsSafeForConcurrentUse(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			res, err := f.Ship(ctx, proj, deploy.ShipRequest{Branch: "feat/x", Commit: commit})
			if err != nil {
				t.Errorf("Ship: %v", err)
				return
			}
			f.Advance(proj, res.Run, deploy.BuildSucceeded)
			_, _ = f.Deployments(ctx, proj)
			_, _, _ = f.Latest(ctx, proj)
			_ = f.Calls()
		}()
	}
	wg.Wait()
	ds, _ := f.Deployments(ctx, proj)
	seen := map[string]bool{}
	for _, d := range ds {
		seen[d.ID] = true
	}
	if len(ds) != 8 || len(seen) != 8 {
		t.Fatalf("8 concurrent Ships made %d deployments with %d distinct ids", len(ds), len(seen))
	}
}

// TestFakeGovernor: a grant to someone holding a role replaces it (Panacea is single-role),
// revoke takes the row's Group, and a posture change the Fake cannot honour is refused.
func TestFakeGovernor(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	f.SetAccess(proj, []deploy.AccessGrant{{Grantee: "team-old", Group: "project-engagement-portal-viewer",
		Legacy: true, Role: deploy.RoleViewer, RoleRaw: "viewer"}})
	for _, role := range []deploy.AccessRole{deploy.RoleViewer, deploy.RoleOwner} {
		if err := f.Grant(ctx, proj, deploy.GrantRequest{Grantee: "ada", Role: role}); err != nil {
			t.Fatalf("Grant(ada, %s): %v", role, err)
		}
	}
	rows, _ := f.Access(ctx, proj)
	if len(rows) != 2 || rows[1].Grantee != "ada" || rows[1].Group != "ada" || rows[1].Role != deploy.RoleOwner {
		t.Fatalf("Access after two grants = %+v, want the legacy row and ada as owner only", rows)
	}
	if err := f.Revoke(ctx, proj, "team-old"); err != nil {
		t.Fatalf("Revoke by a legacy row's grantee: %v, want a silent no-op", err)
	}
	if rows, _ = f.Access(ctx, proj); len(rows) != 2 {
		t.Fatalf("Revoke by grantee removed a row: %+v", rows)
	}
	_ = f.Revoke(ctx, proj, "project-engagement-portal-viewer")
	if rows, _ = f.Access(ctx, proj); len(rows) != 1 || rows[0].Grantee != "ada" {
		t.Fatalf("Revoke by group left %+v", rows)
	}
	if err := f.Grant(ctx, proj, deploy.GrantRequest{Grantee: "bob", Role: deploy.RoleUnknown}); deploy.KindOf(err) != deploy.KindRejected {
		t.Errorf("Grant of RoleUnknown: err = %v, want KindRejected", err)
	}
	bad := []deploy.PostureChange{
		{Mode: "public"},
		{Mode: deploy.PostureKPMGSSO, Password: "x"},
		{Mode: deploy.PosturePassword}, // required for, and only sent with, password (t11:47)
		{Mode: deploy.PosturePassword, RotateBypassToken: true, Password: "x"},
	}
	for _, c := range bad {
		if _, err := f.SetPosture(ctx, proj, "feat/x", c); deploy.KindOf(err) != deploy.KindRejected {
			t.Errorf("SetPosture(%s): err = %v, want KindRejected", c.Mode, err)
		}
	}
	f.SetPostureEnforced(proj, "feat/x", false)
	pr, err := f.SetPosture(ctx, proj, "feat/x", deploy.PostureChange{Mode: deploy.PostureBypassToken, RotateBypassToken: true})
	if err != nil || pr.Enforced || len(pr.BypassTokens) != 1 || pr.Posture.BypassTokenCount != 1 ||
		pr.Posture.Mode != deploy.PostureBypassToken || !pr.Posture.ModeKnown {
		t.Fatalf("SetPosture(bypass-token, rotate) not enforced = (%+v, %v)", pr, err)
	}
	if p, _ := f.Posture(ctx, proj, "feat/x"); p.ModeRaw != "bypass-token" || p.Environment != "feat-x" {
		t.Errorf("Posture after the change = %+v", p)
	}
}

// TestFakeURLPrefix: a set prefix carries no host (the target's to compute). A clear answers
// RouteReverted true with a sentence by default; SetURLRouteReverts(false) seeds the target that
// releases the prefix but could not revert the live route (Panacea's routeReverted false,
// handlers/projects.go:4427-4450), which a lens drawing the clear as done would hide.
func TestFakeURLPrefix(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	u, err := f.SetURLPrefix(ctx, proj, "feat/x", "uat-test")
	if want := (deploy.URLPrefix{Branch: "feat/x", Prefix: "uat-test"}); err != nil || u != want {
		t.Fatalf("SetURLPrefix = (%+v, %v), want %+v", u, err, want)
	}
	done, err := f.ClearURLPrefix(ctx, proj, "feat/x")
	if err != nil || done.Branch != "feat/x" || !done.RouteReverted || done.Message == "" {
		t.Fatalf("ClearURLPrefix unarmed = (%+v, %v), want reverted with a sentence", done, err)
	}
	f.SetURLRouteReverts(proj, "feat/y", false)
	stale, err := f.ClearURLPrefix(ctx, proj, "feat/y")
	if err != nil || stale.Branch != "feat/y" || stale.RouteReverted || stale.Message == "" || stale.Message == done.Message {
		t.Fatalf("ClearURLPrefix not reverted = (%+v, %v), want RouteReverted false with its own sentence", stale, err)
	}
	if again, _ := f.ClearURLPrefix(ctx, proj, "feat/x"); !again.RouteReverted {
		t.Errorf("SetURLRouteReverts(feat/y) reached feat/x: %+v", again)
	}
	f.SetURLRouteReverts(proj, "feat/y", true)
	if again, _ := f.ClearURLPrefix(ctx, proj, "feat/y"); !again.RouteReverted {
		t.Errorf("SetURLRouteReverts(true) did not re-arm the default: %+v", again)
	}
}

// TestFakeStream yields the seeded events in order, then io.EOF; an unseeded stream is
// KindNotFound; a done context ends Next.
func TestFakeStream(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	three := 3
	f.SetStream(proj, deploy.StreamRunLogs, "run-9", []deploy.StreamEvent{
		{Log: &deploy.LogLine{Step: "build", Line: "step 1"}},
		{Readiness: &deploy.ReadinessTick{Measured: true, Ready: &three}},
		{Problem: "pipeline run not found: run-9"},
	})
	s, err := f.Stream(ctx, proj, deploy.StreamRunLogs, "run-9")
	if err != nil {
		t.Fatalf("Stream: %v", err)
	}
	var got []string
	for {
		ev, err := s.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("Next: %v", err)
		}
		switch {
		case ev.Log != nil:
			got = append(got, ev.Log.Line)
		case ev.Readiness != nil:
			got = append(got, "ready")
		default:
			got = append(got, ev.Problem)
		}
	}
	if want := []string{"step 1", "ready", "pipeline run not found: run-9"}; !slices.Equal(got, want) {
		t.Fatalf("stream events = %q, want %q", got, want)
	}
	if err := s.Close(); err != nil {
		t.Errorf("Close: %v", err)
	}
	if _, err := f.Stream(ctx, proj, deploy.StreamReadiness, "run-9"); deploy.KindOf(err) != deploy.KindNotFound {
		t.Errorf("an unseeded stream kind: err = %v, want KindNotFound", err)
	}
	// Close ends a stream that still holds events (review t01 I-2, mutant MD).
	open, _ := f.Stream(ctx, proj, deploy.StreamRunLogs, "run-9")
	if ev, err := open.Next(); err != nil || ev.Log == nil {
		t.Fatalf("the first Next = (%+v, %v), want the first line", ev, err)
	}
	_ = open.Close()
	if ev, err := open.Next(); err != io.EOF {
		t.Errorf("Next after Close = (%+v, %v), want io.EOF", ev, err)
	}
	// A Problem ends the stream: "the stream ends after it" (deploy.StreamEvent). Nothing seeded
	// after one is yielded, as no target sends it (review t01 M-1).
	f.SetStream(proj, deploy.StreamRunLogs, "run-8", []deploy.StreamEvent{
		{Problem: "pipeline run not found: run-8"},
		{Log: &deploy.LogLine{Step: "build", Line: "after the problem"}},
	})
	ended, _ := f.Stream(ctx, proj, deploy.StreamRunLogs, "run-8")
	if ev, err := ended.Next(); err != nil || ev.Problem == "" {
		t.Fatalf("the first Next = (%+v, %v), want the Problem", ev, err)
	}
	if ev, err := ended.Next(); err != io.EOF {
		t.Errorf("Next after a Problem = (%+v, %v), want io.EOF", ev, err)
	}
	cctx, cancel := context.WithCancel(ctx)
	s2, _ := f.Stream(cctx, proj, deploy.StreamRunLogs, "run-9")
	cancel()
	if _, err := s2.Next(); !errors.Is(err, context.Canceled) {
		t.Errorf("Next after the context ended: err = %v, want context.Canceled", err)
	}
}

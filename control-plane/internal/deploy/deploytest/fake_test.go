package deploytest_test

import (
	"context"
	"errors"
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
	_, _ = f.SetPosture(ctx, proj, "feat/x", deploy.PostureChange{Mode: deploy.PosturePassword, Password: "hunter2-secret"})
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
	if cr, err := f.Cancel(ctx, proj, second.Run); err != nil || cr.Outcome != deploy.Cancelled {
		t.Fatalf("Cancel of a pending run = (%+v, %v), want Cancelled", cr, err)
	}
	if rp, _ := f.Run(ctx, proj, second.Run); rp.Build != deploy.BuildFailed {
		t.Errorf("a cancelled run reads %q, want failed (the target does not tell them apart)", rp.Build)
	}
	f.AddProject(deploy.Project{Ref: "other"})
	if _, err := f.Run(ctx, "other", res.Run); deploy.KindOf(err) != deploy.KindNotFound {
		t.Errorf("Run of another project's run: err = %v, want KindNotFound", err)
	}
}

// TestFakeReturnsCopies: a caller that edits what it read cannot change the Fake's state.
func TestFakeReturnsCopies(t *testing.T) {
	f := newShipFake(&testClock{t: base})
	ctx := context.Background()
	zero, crit := 0, 2
	f.AddDeployment(proj, deploy.Deployment{ID: "d1", Environment: "feat-x", CreatedAt: base,
		LiveReady: &zero, Gates: map[string]deploy.Gate{"iac": {Verdict: "failed", Criticals: &crit}}})
	ds, _ := f.Deployments(ctx, proj)
	*ds[0].LiveReady = 9
	*ds[0].Gates["iac"].Criticals = 9
	ds[0].Gates["sast"] = deploy.Gate{Verdict: "passed"}
	again, _ := f.Deployments(ctx, proj)
	if *again[0].LiveReady != 0 || *again[0].Gates["iac"].Criticals != 2 || len(again[0].Gates) != 1 {
		t.Fatalf("the Fake's state changed through a returned value: %+v", again[0])
	}
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
	cctx, cancel := context.WithCancel(ctx)
	s2, _ := f.Stream(cctx, proj, deploy.StreamRunLogs, "run-9")
	cancel()
	if _, err := s2.Next(); !errors.Is(err, context.Canceled) {
		t.Errorf("Next after the context ended: err = %v, want context.Canceled", err)
	}
}

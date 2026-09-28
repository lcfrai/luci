package deploywatch_test

import (
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/connection"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploywatch"
)

// cursorValue reads the cursor as the number it is. A string compare would call
// "10" smaller than "9".
func cursorValue(t *testing.T, s string) uint64 {
	t.Helper()
	if s == "" {
		return 0
	}
	n, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		t.Fatalf("cursor %q is not a decimal uint64: %v", s, err)
	}
	return n
}

// TestDeploymentsNotEveryCycle: the history is unpaginated and runs several
// enrichment passes on the target, so it is read when something moved, never
// every poll.
func TestDeploymentsNotEveryCycle(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	h.build(deploywatch.Config{})
	h.tick()
	h.tick()
	h.clock.advance(15 * time.Second)
	h.tick()
	if got := h.calls("Deployments(ep-a)"); got != 1 {
		t.Fatalf("three polls made %d Deployments reads, want 1", got)
	}
	h.fake.AddDeployment("ep-a", deploy.Deployment{
		ID: "d-new", Environment: "prod", Build: deploy.BuildSucceeded, CreatedAt: h.clock.now(),
	})
	h.clock.advance(15 * time.Second)
	h.tick()
	if got := h.calls("Deployments(ep-a)"); got != 2 {
		t.Fatalf("after Latest moved the watcher made %d Deployments reads, want 2", got)
	}
}

// TestHistoryIntervalForcesRead: history that never refreshes misses an
// environment whose readiness moved without a new build.
func TestHistoryIntervalForcesRead(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	h.build(deploywatch.Config{})
	h.tick()
	h.clock.advance(61 * time.Second)
	h.tick()
	if got := h.calls("Deployments(ep-a)"); got != 2 {
		t.Fatalf("after 61 s the watcher made %d Deployments reads, want 2", got)
	}
}

// TestTrackedRunDrivesActiveCadence: a run this user started is polled every
// 3 s until its build is terminal, and then not.
func TestTrackedRunDrivesActiveCadence(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	w := h.build(deploywatch.Config{})
	run := h.ship("ep-a", "main")
	w.Track("p-a", run)
	h.tick()
	h.tick()
	h.tick()
	if got := h.calls("Run(ep-a," + run + ")"); got != 3 {
		t.Fatalf("three cycles made %d Run reads, want 3 (the active cadence)", got)
	}
	h.fake.Advance("ep-a", run, deploy.BuildSucceeded)
	h.tick() // reads the terminal build and untracks
	before := h.calls("Run(ep-a," + run + ")")
	h.tick()
	h.tick()
	if got := h.calls("Run(ep-a," + run + ")"); got != before {
		t.Fatalf("a terminal run was still polled: %d reads, want %d", got, before)
	}
	if got := h.snapshot("p-a").Runs[run].Build; got != deploy.BuildSucceeded {
		t.Fatalf("Runs[%s].Build = %q, want succeeded", run, got)
	}
}

// TestUnavailableEndsCycleForAll: ten projects times three timeouts would stall
// the loop for minutes, so the first KindUnavailable ends the cycle's Panacea
// phase for every project.
func TestUnavailableEndsCycleForAll(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	h.seedBound("p-b", "ep-b")
	h.build(deploywatch.Config{})
	h.tick() // one good cycle
	cursorBefore := h.w.Cursor()
	h.fake.FailNext("Environments", &deploy.Error{Kind: deploy.KindUnavailable, Op: "environments"})
	h.clock.advance(15 * time.Second)
	before := h.calls("Environments(ep-b)")
	h.tick()
	if got := h.calls("Environments(ep-b)"); got != before {
		t.Fatalf("the second project was still read in an outage cycle: %d reads, want %d", got, before)
	}
	for _, id := range []string{"p-a", "p-b"} {
		if s := h.snapshot(id); !s.Stale {
			t.Fatalf("%s is not stale after an outage", id)
		} else if s.LastError == nil || s.LastError.Kind != deploy.KindUnavailable {
			t.Fatalf("%s carries LastError %v, want unavailable", id, s.LastError)
		}
	}
	if h.w.Cursor() == cursorBefore {
		t.Fatal("an outage moved no cursor, so the desktop never learns of it")
	}
}

// TestUnauthorizedObserved: a lens that keeps drawing "connected" while every
// read 401s is the failure this catches.
func TestUnauthorizedObserved(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	h.seedBound("p-b", "ep-b")
	h.build(deploywatch.Config{})
	h.tick()
	h.fake.FailNext("Environments", &deploy.Error{Kind: deploy.KindUnauthorized, Op: "environments", Status: 401})
	h.clock.advance(15 * time.Second)
	before := h.calls("Environments(ep-b)")
	h.tick()
	failures := h.conn.failures()
	if len(failures) != 1 || deploy.KindOf(failures[0]) != deploy.KindUnauthorized {
		t.Fatalf("Connection.Observe saw %v, want one unauthorized", failures)
	}
	if got := h.calls("Environments(ep-b)"); got != before {
		t.Fatalf("the cycle went on after a 401: %d reads of the second project, want %d", got, before)
	}
}

// TestStaleKeepsLastData: a blip must not clear the lens.
func TestStaleKeepsLastData(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	h.build(deploywatch.Config{})
	h.tick()
	good := h.snapshot("p-a")
	h.fake.FailNext("Environments", &deploy.Error{Kind: deploy.KindUnavailable, Op: "environments"})
	h.clock.advance(15 * time.Second)
	h.tick()
	s := h.snapshot("p-a")
	if len(s.Environments) != len(good.Environments) || s.Environments[0].Name != good.Environments[0].Name {
		t.Fatalf("Environments changed in a failing cycle: %v", s.Environments)
	}
	if !s.Stale {
		t.Fatal("the snapshot is not stale")
	}
	if !s.FetchedAt.Equal(good.FetchedAt) {
		t.Fatalf("FetchedAt moved to %s in a failing cycle, want %s", s.FetchedAt, good.FetchedAt)
	}
}

// TestCursorMonotonic: a cursor that resets makes the desktop miss a change.
func TestCursorMonotonic(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	h.seedBound("p-b", "ep-b")
	h.build(deploywatch.Config{})
	var last uint64
	for i := 0; i < 50; i++ {
		if i == 20 {
			h.locals.drop("p-b") // a project goes away mid-run
		}
		if i == 30 {
			h.fake.FailNext("Environments", &deploy.Error{Kind: deploy.KindUnavailable, Op: "environments"})
		}
		h.tick()
		c := cursorValue(t, h.w.Cursor())
		if c < last {
			t.Fatalf("cursor went backwards: %d then %d", last, c)
		}
		last = c
	}
	h.clock.advance(time.Minute)
	h.tick()
	steady := h.w.Cursor()
	h.tick()
	if h.w.Cursor() != steady {
		t.Fatalf("an unchanged cycle moved the cursor: %q then %q", steady, h.w.Cursor())
	}
}

// TestOwnerProbe: whether this credential controls the project is ASKED of the
// target, never inferred from a group membership (7P-9).
func TestOwnerProbe(t *testing.T) {
	for _, tc := range []struct {
		name string
		err  error
		want string
	}{
		{name: "success", want: "yes"},
		{name: "conflict is still an owner", err: &deploy.Error{Kind: deploy.KindConflict, Op: "autoDeploy"}, want: "yes"},
		{name: "forbidden", err: &deploy.Error{Kind: deploy.KindForbidden, Op: "autoDeploy",
			Message: "your key is scoped to one project and cannot change this setting"}, want: "no"},
		{name: "unavailable is not an answer", err: &deploy.Error{Kind: deploy.KindUnavailable, Op: "autoDeploy"}, want: "unknown"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.seedBound("p-a", "ep-a")
			h.build(deploywatch.Config{Governor: h.fake})
			if tc.err != nil {
				h.fake.FailNext("AutoDeploy", tc.err)
			}
			h.tick()
			if got := h.snapshot("p-a").OwnerControls; got != tc.want {
				t.Fatalf("OwnerControls = %q, want %q", got, tc.want)
			}
		})
	}
}

// TestWatcherPassesSameEnvironment: the rule for whether two environment names
// name one environment is the TARGET's, and a watcher that answered with one of
// its own would be a second rule (b02 round 2, R4).
func TestWatcherPassesSameEnvironment(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	var seen [][2]string
	h.deployer = &deployerWrap{
		Fake:        h.fake,
		sameEnv:     func(a, b string) bool { return true },
		sameEnvSeen: &seen,
		mu:          &sync.Mutex{},
	}
	w := h.build(deploywatch.Config{})
	if !w.SameEnvironment("x", "y") {
		t.Fatal("the watcher answered with a rule of its own; the bare Fake would say false")
	}
	if len(seen) != 1 || seen[0] != [2]string{"x", "y"} {
		t.Fatalf("the target's rule saw %v, want one call with (x, y)", seen)
	}
}

// TestNotConnectedPollsNothing: the gate is step 0, and a screen that says it
// checks again on its own must do so without a desktop timer.
func TestNotConnectedPollsNothing(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	h.conn.state = connection.AwaitingApproval
	h.build(deploywatch.Config{})
	h.tick()
	if got := len(h.fake.Calls()); got != 0 {
		t.Fatalf("the gate let %d calls through while not connected", got)
	}
	if h.conn.checks != 1 {
		t.Fatalf("Connection.Check ran %d times in the first cycle, want 1", h.conn.checks)
	}
	h.ticksFor(57 * time.Second)
	if h.conn.checks != 1 {
		t.Fatalf("Connection.Check ran %d times by 57 s, want 1 (the recheck is 60 s)", h.conn.checks)
	}
	h.tick()
	if h.conn.checks != 2 {
		t.Fatalf("Connection.Check ran %d times at 60 s, want 2", h.conn.checks)
	}
}

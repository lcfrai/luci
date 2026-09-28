package deploywatch_test

import (
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploywatch"
)

// TestScansReadWithTheDeploymentsEnvironment: Panacea reads a missing env as
// production, so a branch release asked for with "" or with the card's name
// shows PRODUCTION's verdict (review C3).
func TestScansReadWithTheDeploymentsEnvironment(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	h.fake.AddDeployment("ep-a", deploy.Deployment{
		ID: "d-feat", Environment: "feat-x", Build: deploy.BuildSucceeded, CreatedAt: h.clock.now(),
	})
	want := deploy.ScanReport{DeploymentID: "d-feat", Overall: "PASS"}
	h.fake.SetScans("ep-a", "feat-x", "d-feat", want)
	h.build(deploywatch.Config{})
	h.tick()
	if got := h.calls("Scans(ep-a,feat-x,d-feat)"); got != 1 {
		t.Fatalf("the watcher made %d Scans(ep-a,feat-x,d-feat) calls, want 1; calls were %v",
			got, h.fake.Calls())
	}
	if got := h.snapshot("p-a").Scans["d-feat"].Overall; got != "PASS" {
		t.Fatalf("Scans[d-feat].Overall = %q, want PASS", got)
	}
}

// TestScansNotEveryPoll: the third fact on the environment strip costs one
// request per build, not one per poll.
func TestScansNotEveryPoll(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	h.fake.SetScans("ep-a", "prod", "d-p-a", deploy.ScanReport{DeploymentID: "d-p-a", Overall: "PASS"})
	h.build(deploywatch.Config{})
	h.tick()
	h.clock.advance(15 * time.Second)
	h.tick()
	h.clock.advance(15 * time.Second)
	h.tick()
	if got := h.calls("Scans(ep-a,prod,d-p-a)"); got != 1 {
		t.Fatalf("three polls made %d Scans reads, want 1", got)
	}
}

// TestScansFailureIsNotStale: a scan read the target refuses is one fact
// missing, never the project going stale - and the retry is at the history
// interval, not in every poll (carried b03-2 m-2).
func TestScansFailureIsNotStale(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	h.fake.AddDeployment("ep-a", deploy.Deployment{
		ID: "d-feat", Environment: "feat-x", Build: deploy.BuildSucceeded, CreatedAt: h.clock.now(),
	})
	// No SetScans: the Fake answers KindNotFound for the unseeded pair.
	h.hub.add("pj-a", release("r-1", "d-feat", "confirmed", "run", h.clock.now(), 15*time.Minute, "u-me"))
	h.build(deploywatch.Config{})
	h.tick()
	s := h.snapshot("p-a")
	if _, ok := s.Scans["d-feat"]; ok {
		t.Fatal("a refused scan read was stored as a report")
	}
	if s.Stale || s.LastError != nil {
		t.Fatalf("a refused scan read staled the project: stale=%v err=%v", s.Stale, s.LastError)
	}
	if got := h.calls("Scans(ep-a,feat-x,d-feat)"); got != 1 {
		t.Fatalf("the first poll made %d Scans calls, want 1", got)
	}
	for _, at := range []time.Duration{15 * time.Second, 30 * time.Second, 45 * time.Second} {
		h.clock.advance(15 * time.Second)
		h.tick()
		if got := h.calls("Scans(ep-a,feat-x,d-feat)"); got != 1 {
			t.Fatalf("at %s the watcher had made %d Scans calls, want 1: a failed read is retried every poll", at, got)
		}
	}
	h.clock.advance(16 * time.Second) // past 61 s since the failed attempt
	h.tick()
	if got := h.calls("Scans(ep-a,feat-x,d-feat)"); got != 2 {
		t.Fatalf("at 61 s the watcher had made %d Scans calls, want 2", got)
	}
}

// TestScansSkipsEmptyEnvironment: a legacy row with no environment is its own
// group under the raw-string grouping, so the guard is reached; asking would
// show production's verdict under that release (re-review b03 M-1; carried
// b03-2 m-3).
func TestScansSkipsEmptyEnvironment(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	h.fake.ClearDeployments("ep-a")
	h.fake.AddDeployment("ep-a", deploy.Deployment{
		ID: "d-old", Environment: "", Build: deploy.BuildHistorical, CreatedAt: h.clock.now().Add(-2 * time.Hour),
	})
	h.fake.AddDeployment("ep-a", deploy.Deployment{
		ID: "d-feat", Environment: "feat-x", Build: deploy.BuildSucceeded, CreatedAt: h.clock.now(),
	})
	h.fake.SetScans("ep-a", "feat-x", "d-feat", deploy.ScanReport{DeploymentID: "d-feat", Overall: "PASS"})
	h.build(deploywatch.Config{})
	h.tick()
	if got := h.calls("Scans(ep-a,feat-x,d-feat)"); got != 1 {
		t.Fatalf("the real environment's newest deployment was read %d times, want 1", got)
	}
	if got := h.calls("Scans", "d-old"); got != 0 {
		t.Fatalf("the watcher read scans for the legacy row %d times, want 0; calls were %v",
			got, h.fake.Calls())
	}
	if _, ok := h.snapshot("p-a").Scans["d-old"]; ok {
		t.Fatal("the legacy row carries a scan report, which can only be production's")
	}
}

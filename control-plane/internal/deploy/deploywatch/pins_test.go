package deploywatch_test

import (
	"fmt"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploywatch"
)

// seedHistory adds n deployments, oldest last, all created before the clock's
// now by at least age.
func (h *harness) seedHistory(ref string, n int, age time.Duration) []string {
	h.fake.ClearDeployments(deployRef(ref))
	var ids []string
	for i := 0; i < n; i++ {
		id := fmt.Sprintf("d-%02d", i)
		h.fake.AddDeployment(deployRef(ref), deploy.Deployment{
			ID: id, Environment: "prod", Build: deploy.BuildSucceeded,
			CreatedAt: h.clock.now().Add(-age - time.Duration(i)*time.Minute),
		})
		ids = append(ids, id)
	}
	return ids
}

// TestPinsBeyondThePage: the Hub returns the 200 newest releases, so a project
// with more has deployments whose release is not in the page - and the view
// would call a real pin absent (review M11).
func TestPinsBeyondThePage(t *testing.T) {
	h := newHarness(t)
	h.seedBoundToHub("p-a", "ep-a", "pj-a")
	ids := h.seedHistory("ep-a", 25, 30*time.Minute)
	oldest := ids[len(ids)-1]
	reserved := h.clock.now().Add(-2 * time.Hour)
	for i := 0; i < 200; i++ {
		h.hub.add("pj-a", release(fmt.Sprintf("r-%03d", i), fmt.Sprintf("run-%03d", i),
			"confirmed", "run", reserved, 15*time.Minute, "u-me"))
	}
	// The 201st release is the oldest deployment's, and is beyond the page.
	h.hub.add("pj-a", release("r-old", oldest, "confirmed", "run", reserved, 15*time.Minute, "u-me"))
	h.build(h.hubCfg())

	h.tick()
	s := h.snapshot("p-a")
	if !s.ReleasesMore {
		t.Fatal("a 201-release project did not report ReleasesMore")
	}
	if got := targetedCount(h); got != 20 {
		t.Fatalf("cycle 1 made %d targeted reads, want 20", got)
	}
	h.tick()
	if got := targetedCount(h); got != 25 {
		t.Fatalf("cycle 2 brought the total to %d targeted reads, want 25", got)
	}
	h.tick()
	if got := targetedCount(h); got != 25 {
		t.Fatalf("cycle 3 made more targeted reads: %d, want 25 in total", got)
	}
	s = h.snapshot("p-a")
	if r := s.PinsByRun[oldest]; r == nil || r.ID != "r-old" {
		t.Fatalf("PinsByRun[%s] = %v, want the release beyond the page", oldest, r)
	}
	for _, id := range ids[:len(ids)-1] {
		r, read := s.PinsByRun[id]
		if !read {
			t.Fatalf("PinsByRun has no entry for %s, so the view says pin not loaded for good", id)
		}
		if r != nil {
			t.Fatalf("PinsByRun[%s] = %v, want nil: the Hub has no release for it", id, r)
		}
	}
}

// TestNoTargetedReadWhenThePageIsWhole: a project with fewer than 200 releases
// costs no targeted read at all.
func TestNoTargetedReadWhenThePageIsWhole(t *testing.T) {
	h := newHarness(t)
	h.seedBoundToHub("p-a", "ep-a", "pj-a")
	h.seedHistory("ep-a", 5, 30*time.Minute)
	reserved := h.clock.now().Add(-2 * time.Hour)
	for i := 0; i < 199; i++ {
		h.hub.add("pj-a", release(fmt.Sprintf("r-%03d", i), fmt.Sprintf("run-%03d", i),
			"confirmed", "run", reserved, 15*time.Minute, "u-me"))
	}
	h.build(h.hubCfg())
	h.tick()
	h.tick()
	if h.snapshot("p-a").ReleasesMore {
		t.Fatal("a 199-release project reported ReleasesMore")
	}
	if got := targetedCount(h); got != 0 {
		t.Fatalf("a whole page still cost %d targeted reads", got)
	}
}

// TestLateConfirmedPinIsNotCachedAbsent: before a deployment settles, a release
// still reserved would answer zero items and cache a false nil for the
// binding's life (plan re-review b09-1, M-1).
func TestLateConfirmedPinIsNotCachedAbsent(t *testing.T) {
	h := newHarness(t)
	h.seedBoundToHub("p-a", "ep-a", "pj-a")
	h.fake.ClearDeployments("ep-a")
	h.fake.AddDeployment("ep-a", deploy.Deployment{
		ID: "d-new", Environment: "prod", Build: deploy.BuildSucceeded,
		CreatedAt: h.clock.now().Add(-time.Minute),
	})
	reserved := h.clock.now().Add(-2 * time.Hour)
	for i := 0; i < 200; i++ {
		h.hub.add("pj-a", release(fmt.Sprintf("r-%03d", i), fmt.Sprintf("run-%03d", i),
			"confirmed", "run", reserved, 15*time.Minute, "u-me"))
	}
	// The newest release is this user's Ship, still reserved, with no run yet.
	h.hub.add("pj-a", release("r-new", "", "reserved", "run", h.clock.now(), 15*time.Minute, "u-me"))
	h.build(h.hubCfg())
	h.tick()
	if !h.snapshot("p-a").ReleasesMore {
		t.Fatal("a 201-release project did not report ReleasesMore")
	}
	if got := targetedCount(h); got != 0 {
		t.Fatalf("an unsettled deployment cost %d targeted reads, want 0", got)
	}
	if _, present := h.snapshot("p-a").PinsByRun["d-new"]; present {
		t.Fatal("an unsettled deployment was given a PinsByRun entry, so a real pin can read as absent")
	}
	// The confirm lands, and 200 more reservations push it off the page.
	h.hub.confirm("pj-a", "r-new", "d-new")
	h.clock.advance(21 * time.Minute)
	h.tick()
	if got := targetedCount(h); got != 1 {
		t.Fatalf("after the deployment settled the watcher made %d targeted reads, want 1", got)
	}
	if r := h.snapshot("p-a").PinsByRun["d-new"]; r == nil || r.ID != "r-new" {
		t.Fatalf("PinsByRun[d-new] = %v, want r-new", r)
	}
}

// TestJoinOnlyOwnConfirmedRuns: a rollback has no deployment of its own (7P-4),
// a reservation is not a release, and a teammate's release is theirs to join.
func TestJoinOnlyOwnConfirmedRuns(t *testing.T) {
	h := newHarness(t)
	h.seedBoundToHub("p-a", "ep-a", "pj-a")
	h.fake.AddDeployment("ep-a", deploy.Deployment{
		ID: "d-team", Environment: "prod", Build: deploy.BuildSucceeded, CreatedAt: h.clock.now(),
	})
	at := h.clock.now().Add(-time.Hour)
	h.hub.add("pj-a", release("r-mine", "d-p-a", "confirmed", "run", at, 15*time.Minute, "u-me"))
	h.hub.add("pj-a", release("r-held", "d-p-a", "reserved", "run", at, 15*time.Minute, "u-me"))
	h.hub.add("pj-a", release("r-team", "d-team", "confirmed", "run", at, 15*time.Minute, "u-them"))
	h.hub.add("pj-a", release("r-back", "d-p-a", "confirmed", "rollback", at, 15*time.Minute, "u-me"))
	h.build(h.hubCfg())
	h.tick()
	h.tick()
	h.tick()
	joins := h.hub.joinsMade()
	if len(joins) != 1 || joins[0] != [2]string{"r-mine", "d-p-a"} {
		t.Fatalf("the watcher wrote %v, want one join of r-mine to d-p-a", joins)
	}
}

// TestJoinWaitsForDeployment: joining by observation of a run name that does
// not yet exist writes a join to nothing.
func TestJoinWaitsForDeployment(t *testing.T) {
	h := newHarness(t)
	h.seedBoundToHub("p-a", "ep-a", "pj-a")
	at := h.clock.now().Add(-time.Hour)
	h.hub.add("pj-a", release("r-mine", "d-soon", "confirmed", "run", at, 15*time.Minute, "u-me"))
	h.build(h.hubCfg())
	h.tick()
	if got := h.hub.joinsMade(); len(got) != 0 {
		t.Fatalf("the watcher joined a release whose deployment does not exist: %v", got)
	}
	h.fake.AddDeployment("ep-a", deploy.Deployment{
		ID: "d-soon", Environment: "prod", Build: deploy.BuildSucceeded, CreatedAt: h.clock.now(),
	})
	h.clock.advance(15 * time.Second)
	h.tick()
	if got := h.hub.joinsMade(); len(got) != 1 || got[0] != [2]string{"r-mine", "d-soon"} {
		t.Fatalf("the watcher wrote %v, want one join once the deployment appeared", got)
	}
}

// targetedCount is how many Releases queries carried a PipelineRun.
func targetedCount(h *harness) int {
	n := 0
	for _, q := range h.hub.seen() {
		if q.q.PipelineRun != "" {
			n++
		}
	}
	return n
}

var _ = deploywatch.ReleasesOK

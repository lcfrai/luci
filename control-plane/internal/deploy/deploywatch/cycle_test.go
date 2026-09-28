package deploywatch_test

import (
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploywatch"
)

const ghKey = "github.com/kpmg/"

// seedBound wires one local project to one Panacea project by repository, with
// one environment and one deployment, and returns the local id.
func (h *harness) seedBound(id, ref string) string {
	origin := "https://" + ghKey + id
	h.local(id, origin)
	h.panaceaProject(ref, origin)
	h.fake.SetEnvironments(deploy.ProjectRef(ref), []deploy.Environment{
		{Name: "production", Branch: "main", Primary: true, Status: "Running"},
	})
	h.fake.SetBranchEnvironment(deploy.ProjectRef(ref), "main", "prod")
	h.fake.AddDeployment(deploy.ProjectRef(ref), deploy.Deployment{
		ID: "d-" + id, Environment: "prod", Build: deploy.BuildSucceeded,
		CreatedAt: h.clock.now().Add(-time.Hour),
	})
	return id
}

// TestOneProjectsReadPerCycle: N local projects cost ONE GET /api/v1/projects a
// recheck, not one per project, and not one per cycle. The recheck is measured
// in time, so a tracked run's 3 s cadence does not make it come sooner, and an
// unbound project does not make it come every cycle (carried b09-2 M-A).
func TestOneProjectsReadPerCycle(t *testing.T) {
	for _, tc := range []struct {
		name    string
		track   bool
		unbound bool
	}{
		{name: "quiet"},
		{name: "with a tracked run", track: true},
		{name: "with an unbound project", unbound: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.seedBound("p-a", "ep-a")
			h.seedBound("p-b", "ep-b")
			h.seedBound("p-c", "ep-c")
			if tc.unbound {
				// A local project no Panacea project deploys: NoMatch, for good.
				h.local("p-d", "https://"+ghKey+"unshared")
			}
			w := h.build(deploywatch.Config{})
			if tc.track {
				w.Track("p-a", h.ship("ep-a", "main"))
			}
			h.tick() // the first cycle resolves
			if got := h.calls("Projects()"); got != 1 {
				t.Fatalf("first cycle made %d Projects reads, want 1", got)
			}
			h.ticksFor(57 * time.Second) // the cycles at 3 s through 57 s
			if got := h.calls("Projects()"); got != 1 {
				t.Fatalf("by 57 s the watcher made %d Projects reads, want 1", got)
			}
			h.tick() // the cycle at 60 s: the recheck
			if got := h.calls("Projects()"); got != 2 {
				t.Fatalf("at 60 s the watcher made %d Projects reads, want 2", got)
			}
		})
	}
}

// TestEnvironmentsBeforeLatest: the order is load-bearing, because a missing
// project must surface as its own error and not as "never deployed".
func TestEnvironmentsBeforeLatest(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	w := h.build(deploywatch.Config{})
	w.WantBranch("p-a", "feat/x")
	h.fake.SetBranchEnvironment("ep-a", "feat/x", "feat-x")
	h.tick()
	var order []string
	for _, c := range h.fake.Calls() {
		switch {
		case c == "Environments(ep-a)":
			order = append(order, "environments")
		case c == "EnvironmentFor(ep-a,feat/x)":
			order = append(order, "environmentFor")
		case c == "Latest(ep-a)":
			order = append(order, "latest")
		}
	}
	want := []string{"environments", "environmentFor", "latest"}
	if len(order) != len(want) {
		t.Fatalf("calls were %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("calls were %v, want %v", order, want)
		}
	}
}

// TestWantedBranchResolvedOnce: one request per new branch, and the re-read is
// at the branch's recheck - which is TIME, not a count of polls.
func TestWantedBranchResolvedOnce(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	h.fake.SetBranchEnvironment("ep-a", "feat/x", "feat-x")
	w := h.build(deploywatch.Config{})
	w.Track("p-a", h.ship("ep-a", "main")) // the fast cadence: a poll every 3 s
	w.WantBranch("p-a", "feat/x")
	h.tick()
	if got := h.calls("EnvironmentFor(ep-a,feat/x)"); got != 1 {
		t.Fatalf("first poll made %d EnvironmentFor calls, want 1", got)
	}
	if got := h.snapshot("p-a").BranchEnvs["feat/x"]; got != "feat-x" {
		t.Fatalf("BranchEnvs[feat/x] = %q, want feat-x", got)
	}
	h.ticksFor(57 * time.Second)
	if got := h.calls("EnvironmentFor(ep-a,feat/x)"); got != 1 {
		t.Fatalf("by 57 s the watcher made %d EnvironmentFor calls, want 1", got)
	}
	h.tick()
	if got := h.calls("EnvironmentFor(ep-a,feat/x)"); got != 2 {
		t.Fatalf("at 60 s the watcher made %d EnvironmentFor calls, want 2", got)
	}
}

// TestWantedBranchCapDropsBoth: the 17th wanted branch drops the least recently
// wanted, from BOTH maps (carried b02-2 M-5).
func TestWantedBranchCapDropsBoth(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	w := h.build(deploywatch.Config{})
	for i := 0; i < 16; i++ {
		br := "feat/" + string(rune('a'+i))
		h.fake.SetBranchEnvironment("ep-a", br, "env-"+string(rune('a'+i)))
		w.WantBranch("p-a", br)
		h.clock.advance(time.Millisecond)
	}
	h.tick()
	s := h.snapshot("p-a")
	if len(s.BranchEnvs) != 16 {
		t.Fatalf("BranchEnvs holds %d branches, want 16", len(s.BranchEnvs))
	}
	w.WantBranch("p-a", "feat/seventeen") // never seeded: a refusal, in BranchEnvErrs
	h.tick()
	s = h.snapshot("p-a")
	if _, kept := s.BranchEnvs["feat/a"]; kept {
		t.Fatal("the least recently wanted branch was kept in BranchEnvs")
	}
	if len(s.BranchEnvs)+len(s.BranchEnvErrs) != 16 {
		t.Fatalf("the two branch maps hold %d + %d entries, want 16 together",
			len(s.BranchEnvs), len(s.BranchEnvErrs))
	}
	if s.BranchEnvErrs["feat/seventeen"] == nil {
		t.Fatal("the refused branch is in neither map")
	}
}

// TestRefusedBranchNotReaskedEveryCycle: a branch name Panacea refuses costs
// one request a minute, not one a poll, and never stales the project.
func TestRefusedBranchNotReaskedEveryCycle(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	w := h.build(deploywatch.Config{})
	w.WantBranch("p-a", "live-check/p7") // not seeded: the Fake refuses
	h.tick()
	if got := h.calls("EnvironmentFor(ep-a,live-check/p7)"); got != 1 {
		t.Fatalf("first poll made %d EnvironmentFor calls, want 1", got)
	}
	s := h.snapshot("p-a")
	if s.BranchEnvErrs["live-check/p7"] == nil {
		t.Fatal("the refusal is not in BranchEnvErrs")
	}
	if _, ok := s.BranchEnvs["live-check/p7"]; ok {
		t.Fatal("a refused branch is in BranchEnvs as well")
	}
	if s.Stale {
		t.Fatal("one bad branch name staled the whole project")
	}
	if h.calls("Latest(ep-a)") == 0 {
		t.Fatal("the refusal ended the poll: Latest was never read")
	}
	h.ticksFor(57 * time.Second)
	if got := h.calls("EnvironmentFor(ep-a,live-check/p7)"); got != 1 {
		t.Fatalf("by 57 s the watcher made %d EnvironmentFor calls, want 1", got)
	}
	h.fake.SetBranchEnvironment("ep-a", "live-check/p7", "live-check-p7")
	h.tick() // 60 s: the recheck
	if got := h.calls("EnvironmentFor(ep-a,live-check/p7)"); got != 2 {
		t.Fatalf("at 60 s the watcher made %d EnvironmentFor calls, want 2", got)
	}
	s = h.snapshot("p-a")
	if s.BranchEnvs["live-check/p7"] != "live-check-p7" {
		t.Fatalf("after a good answer BranchEnvs[live-check/p7] = %q", s.BranchEnvs["live-check/p7"])
	}
	if s.BranchEnvErrs["live-check/p7"] != nil {
		t.Fatal("the old refusal was kept beside the good answer")
	}
}

package deploywatch_test

import (
	"context"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploywatch"
)

// The budget tests run on real time, in milliseconds. This host stalls under
// load (whole runs have been voided by it), so each one measures its own cycle
// and says so rather than failing, when the machine took an order of magnitude
// longer than the budget it was given.
func skipIfStalled(t *testing.T, took, budget time.Duration) {
	t.Helper()
	if took > 10*budget {
		t.Skipf("the host stalled: one %s cycle took %s, so the timing this test pins is not measurable here", budget, took)
	}
}

// TestCycleDeadlineCutsAHungRead: one goroutine means one hung read holds every
// project, and the client's own retries could hold it for about 46 s. The cut
// is the budget's, but the FAILURE is Panacea's, because the read began with a
// whole attempt left (review M14; plan re-review b09-1, I-1).
func TestCycleDeadlineCutsAHungRead(t *testing.T) {
	h := newHarness(t)
	h.realTime = true
	h.seedBound("p-a", "ep-a")
	h.seedBound("p-b", "ep-b")
	h.deployer = &deployerWrap{
		Fake: h.fake,
		environments: func(ctx context.Context, p deploy.ProjectRef) ([]deploy.Environment, error) {
			if p != "ep-a" {
				return h.fake.Environments(ctx, p)
			}
			<-ctx.Done()
			return nil, &deploy.Error{Kind: deploy.KindUnavailable, Op: "environments", Cause: ctx.Err()}
		},
	}
	w := h.build(deploywatch.Config{
		ReadTimeout: 20 * time.Millisecond, ActiveInterval: 10 * time.Millisecond,
		CycleDeadline: 50 * time.Millisecond,
	})
	start := time.Now()
	w.Cycle(context.Background())
	took := time.Since(start)
	if took > time.Second {
		t.Fatalf("one cycle took %s: a hung read held the loop past its budget", took)
	}
	if got := h.calls("Environments(ep-b)"); got != 0 {
		t.Fatalf("the second project was read %d times in a cycle the first hung", got)
	}
	for _, id := range []string{"p-a", "p-b"} {
		s := h.snapshot(id)
		if !s.Stale || s.LastError == nil || s.LastError.Kind != deploy.KindUnavailable {
			t.Fatalf("%s: stale=%v err=%v, want stale with unavailable", id, s.Stale, s.LastError)
		}
	}
	// A tracked run goes first next cycle, so a hang in another project's reads
	// never comes before it.
	w.Track("p-b", "run-x")
	h.fake.FailNext("Environments", &deploy.Error{Kind: deploy.KindNotFound, Op: "environments"})
	before := len(h.fake.Calls())
	w.Cycle(context.Background())
	calls := h.fake.Calls()[before:]
	first := ""
	for _, c := range calls {
		if c == "Environments(ep-a)" || c == "Environments(ep-b)" {
			first = c
			break
		}
	}
	if first != "Environments(ep-b)" {
		t.Fatalf("the next cycle's first environments read was %q, want the tracked project's", first)
	}
	skipIfStalled(t, took, 50*time.Millisecond)
}

// TestNewRefusesABudgetTooSmall: a budget no larger than one attempt turns
// every hung read into a budget stop, which stales nothing - so an outage would
// be silent.
func TestNewRefusesABudgetTooSmall(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-a", "ep-a")
	res := h.build(deploywatch.Config{}) // a good one, to make the resolver
	if res == nil {
		t.Fatal("harness did not build")
	}
	_, err := deploywatch.New(deploywatch.Config{
		Deployer: h.fake, Projects: h.locals, Bindings: h.spy, Connection: h.conn,
		ReadTimeout: 20 * time.Millisecond, ActiveInterval: 10 * time.Millisecond,
		CycleDeadline: 25 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("New took a CycleDeadline below ReadTimeout plus ActiveInterval")
	}
	_, err = deploywatch.New(deploywatch.Config{
		Deployer: h.fake, Projects: h.locals, Bindings: h.spy, Connection: h.conn,
		ReadTimeout: 20 * time.Millisecond, ActiveInterval: 10 * time.Millisecond,
		CycleDeadline: 50 * time.Millisecond, HubReadTimeout: 40 * time.Millisecond,
	})
	if err == nil {
		t.Fatal("New took a HubReadTimeout that leaves no room for a Panacea attempt")
	}
}

// TestBudgetStopIsNotAnOutage: a cycle that runs out of budget under normal
// load must not be reported as "Panacea has not answered" for every project,
// every cycle (plan re-review b09-1, I-1).
func TestBudgetStopIsNotAnOutage(t *testing.T) {
	h := newHarness(t)
	h.realTime = true
	h.seedBoundToHub("p-a", "ep-a", "pj-a")
	h.seedBoundToHub("p-b", "ep-b", "pj-b")
	h.seedBoundToHub("p-c", "ep-c", "pj-c")
	slow := false
	h.deployer = &deployerWrap{
		Fake: h.fake,
		environments: func(ctx context.Context, p deploy.ProjectRef) ([]deploy.Environment, error) {
			if slow {
				select {
				case <-time.After(22 * time.Millisecond):
				case <-ctx.Done():
				}
			}
			return h.fake.Environments(ctx, p)
		},
	}
	cfg := h.hubCfg()
	cfg.ReadTimeout, cfg.ActiveInterval = 20*time.Millisecond, 10*time.Millisecond
	cfg.HubReadTimeout, cfg.CycleDeadline = 20*time.Millisecond, 80*time.Millisecond
	w := h.build(cfg)
	w.Cycle(context.Background()) // one good cycle each
	before := map[string]deploywatch.Snapshot{}
	for _, id := range []string{"p-a", "p-b", "p-c"} {
		before[id] = h.snapshot(id)
		if before[id].ReleasesState != "ok" {
			t.Fatalf("%s: ReleasesState %q after a good cycle, want ok", id, before[id].ReleasesState)
		}
	}
	slow = true
	start := time.Now()
	w.Cycle(context.Background())
	took := time.Since(start)
	stopped := 0
	for _, id := range []string{"p-a", "p-b", "p-c"} {
		s := h.snapshot(id)
		if s.Stale || s.LastError != nil {
			t.Fatalf("%s was staled by a budget stop: err=%v", id, s.LastError)
		}
		if s.ReleasesState != "ok" {
			t.Fatalf("%s: ReleasesState %q after a budget stop, want ok", id, s.ReleasesState)
		}
		if s.FetchedAt.Equal(before[id].FetchedAt) {
			stopped++
		}
	}
	if len(h.conn.failures()) != 0 {
		t.Fatalf("a budget stop was reported to the connection: %v", h.conn.failures())
	}
	skipIfStalled(t, took, 80*time.Millisecond)
	if stopped == 0 {
		t.Fatalf("no project was stopped in a cycle of %s with three 22 ms reads and an 80 ms budget", took)
	}
	if stopped == 3 {
		t.Fatal("every project was stopped, so the budget bought nothing")
	}
}

// TestHubReadTimeoutBoundsTheHub: a Hub that never answers ends as its own
// read's timeout - never as a budget stop that hides it, and never as Panacea.
func TestHubReadTimeoutBoundsTheHub(t *testing.T) {
	h := newHarness(t)
	h.realTime = true
	h.seedBoundToHub("p-a", "ep-a", "pj-a")
	cfg := h.hubCfg()
	cfg.ReadTimeout, cfg.ActiveInterval = 20*time.Millisecond, 10*time.Millisecond
	cfg.HubReadTimeout, cfg.CycleDeadline = 20*time.Millisecond, 100*time.Millisecond
	w := h.build(cfg)
	h.hub.block = true
	start := time.Now()
	w.Cycle(context.Background())
	took := time.Since(start)
	if took > time.Second {
		t.Fatalf("one cycle took %s: a blocked Hub held the loop", took)
	}
	s := h.snapshot("p-a")
	if s.ReleasesState != "unavailable" {
		t.Fatalf("ReleasesState = %q, want unavailable", s.ReleasesState)
	}
	if s.Stale {
		t.Fatal("a blocked Hub staled the Panacea data")
	}
	if len(s.Environments) == 0 {
		t.Fatal("the Panacea reads did not happen while the Hub was blocked")
	}
}

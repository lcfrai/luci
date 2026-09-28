package deploywatch_test

import (
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/deploywatch"
)

// hubCfg is a watcher with the Hub stand-in wired, and the same classifier
// main.go gives task 06 - one vocabulary for "the Hub did not answer".
func (h *harness) hubCfg() deploywatch.Config {
	return deploywatch.Config{
		Releases:         h.hub,
		Me:               h.hub,
		ClassifyHubError: classifyHubStub,
	}
}

// seedBoundToHub is seedBound with a Hub project on the binding, which the
// real resolver cannot reach in this tree: there is no Hub to ask.
func (h *harness) seedBoundToHub(id, ref, hubProjectID string) string {
	h.seedBound(id, ref)
	h.hubBindings = append(h.hubBindings, binding.Binding{
		ProjectID: id, State: binding.Bound, Panacea: deployRef(ref), HubProjectID: hubProjectID,
	})
	return id
}

// TestReleasesStateNamesKeySpelling: two surfaces that disagree is the failure
// here - a repository told it is "not a shared project in the Hub yet" above a
// Ship bar that names the real cause (review I6; carried b05-2 N-1, N-2).
func TestReleasesStateNamesKeySpelling(t *testing.T) {
	for _, tc := range []struct {
		name string
		miss string
		want string
	}{
		{name: "the Hub refused the key", miss: binding.MissKeySpelling, want: "hub_key_spelling"},
		{name: "the Hub has no such project", miss: binding.MissNotLinked, want: "no_hub_project"},
		{name: "the Hub has not answered yet", miss: "", want: "unavailable"},
		{name: "the user is signed out", miss: binding.MissSignedOut, want: "signed_out"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newHarness(t)
			h.seedBound("p-a", "ep-a")
			cfg := h.hubCfg()
			h.build(cfg)
			h.overrideBinding("p-a", binding.Binding{
				ProjectID: "p-a", State: binding.Bound, Panacea: "ep-a", HubMiss: tc.miss,
			})
			h.tick()
			if got := h.snapshot("p-a").ReleasesState; got != tc.want {
				t.Fatalf("ReleasesState = %q, want %q", got, tc.want)
			}
			if len(h.hub.seen()) != 0 {
				t.Fatalf("the watcher asked the Hub %d times for a binding that already answers", len(h.hub.seen()))
			}
		})
	}
}

// TestSignedOutKeepsReleases: a sign-out is not evidence that any release
// changed, so the pins stay on the lens beside a sentence that sends the user
// to sign in (carried b05-2 N-2, b11-2 I-1).
func TestSignedOutKeepsReleases(t *testing.T) {
	h := newHarness(t)
	h.seedBoundToHub("p-a", "ep-a", "pj-a")
	h.hub.add("pj-a", release("r-1", "d-p-a", "confirmed", "run", h.clock.now().Add(-time.Hour), 15*time.Minute, "u-me"))
	h.build(h.hubCfg())
	h.tick()
	good := h.snapshot("p-a")
	if good.ReleasesState != "ok" || len(good.Releases) != 1 || good.ReleasesAt.IsZero() {
		t.Fatalf("the first cycle did not read releases: %+v", good)
	}
	h.overrideBinding("p-a", binding.Binding{
		ProjectID: "p-a", State: binding.Bound, Panacea: "ep-a",
		HubProjectID: "pj-a", HubMiss: binding.MissSignedOut,
	})
	before := len(h.hub.seen())
	h.clock.advance(61 * time.Second)
	h.tick()
	s := h.snapshot("p-a")
	if s.ReleasesState != "signed_out" {
		t.Fatalf("ReleasesState = %q, want signed_out", s.ReleasesState)
	}
	if len(s.Releases) != 1 || !s.ReleasesAt.Equal(good.ReleasesAt) {
		t.Fatalf("a sign-out cleared the releases: %d rows, ReleasesAt %s", len(s.Releases), s.ReleasesAt)
	}
	if len(h.hub.seen()) != before {
		t.Fatal("the watcher read the Hub while the binding already said signed out")
	}
}

// TestHubDownKeepsReleases: one dead Hub costs ONE read in a phase, not one per
// project, and it is not retried until the poll interval has passed (carried
// b09-2 I-1, point 2).
func TestHubDownKeepsReleases(t *testing.T) {
	h := newHarness(t)
	h.seedBoundToHub("p-a", "ep-a", "pj-a")
	h.seedBoundToHub("p-b", "ep-b", "pj-b")
	h.hub.add("pj-a", release("r-1", "d-p-a", "confirmed", "run", h.clock.now().Add(-time.Hour), 15*time.Minute, "u-me"))
	h.build(h.hubCfg())
	h.tick()
	good := h.snapshot("p-a")
	h.hub.fail(errHubDown)
	before := len(h.hub.seen())
	h.clock.advance(61 * time.Second)
	h.tick()
	if got := len(h.hub.seen()) - before; got != 1 {
		t.Fatalf("a dead Hub cost %d reads in one phase, want 1 for every project together", got)
	}
	s := h.snapshot("p-a")
	if s.ReleasesState != "unavailable" {
		t.Fatalf("ReleasesState = %q, want unavailable", s.ReleasesState)
	}
	if len(s.Releases) != len(good.Releases) {
		t.Fatalf("a Hub blip emptied the pins: %d rows, want %d", len(s.Releases), len(good.Releases))
	}
	if s.Stale {
		t.Fatal("a Hub blip staled the Panacea data")
	}
	before = len(h.hub.seen())
	h.tick()
	h.tick()
	if got := len(h.hub.seen()); got != before {
		t.Fatalf("the Hub was retried %d times in the next two cycles, want none before the poll interval", got-before)
	}
	h.clock.advance(16 * time.Second)
	h.tick()
	if len(h.hub.seen()) == before {
		t.Fatal("the Hub was never retried after the poll interval")
	}
}

// TestReleaseEventRefreshesReleases: a teammate's rollback is on the lens
// within one read of the change feed plus one cycle, not after the history
// interval - and the listener itself makes no request (review M12).
func TestReleaseEventRefreshesReleases(t *testing.T) {
	h := newHarness(t)
	h.seedBoundToHub("p-a", "ep-a", "pj-a")
	h.seedBoundToHub("p-b", "ep-b", "pj-b")
	w := h.build(h.hubCfg())
	h.tick()
	before := len(h.hub.seen())
	w.HubEvents([]deploywatch.Event{
		{Kind: "release.confirmed", ProjectID: "pj-a"},
		{Kind: "brief.revised", ProjectID: "pj-b"},
		{Kind: "release.abandoned", ProjectID: "pj-other"},
	})
	if got := len(h.hub.seen()); got != before {
		t.Fatalf("HubEvents made %d Hub calls on the poller's goroutine, want 0", got-before)
	}
	h.tick()
	var asked []string
	for _, q := range h.hub.seen()[before:] {
		asked = append(asked, q.projectID)
	}
	if len(asked) != 1 || asked[0] != "pj-a" {
		t.Fatalf("the next cycle read releases for %v, want [pj-a] only", asked)
	}
}

// TestProjectEventReResolvesUnshared: a Share turns Ship on in seconds, and a
// key the Hub itself refuses is never re-resolved for it - no share can fix a
// key the Hub will not hold (review M17).
func TestProjectEventReResolvesUnshared(t *testing.T) {
	h := newHarness(t)
	h.seedBound("p-1", "ep-1")
	h.seedBound("p-2", "ep-2")
	h.seedBound("p-3", "ep-3")
	h.build(h.hubCfg())
	h.tick()
	h.overrideBinding("p-1", binding.Binding{ProjectID: "p-1", State: binding.Bound, Panacea: "ep-1", HubMiss: binding.MissNotLinked})
	h.overrideBinding("p-2", binding.Binding{ProjectID: "p-2", State: binding.Bound, Panacea: "ep-2", HubMiss: binding.MissKeySpelling})
	h.overrideBinding("p-3", binding.Binding{ProjectID: "p-3", State: binding.Bound, Panacea: "ep-3", HubProjectID: "pj-c"})
	h.tick() // the overrides land
	h.clearResolved()
	h.w.HubEvents([]deploywatch.Event{{Kind: "project.created", ProjectID: "pj-new"}})
	h.tick()
	if got := h.resolvedIDs(); len(got) != 1 || got[0] != "p-1" {
		t.Fatalf("the next cycle re-resolved %v, want [p-1] only", got)
	}
	// The Share landed: p-1 is a Hub project now, so it is no longer among the
	// unshared that a project event marks.
	h.overrideBinding("p-1", binding.Binding{
		ProjectID: "p-1", State: binding.Bound, Panacea: "ep-1", HubProjectID: "pj-new",
	})
	h.tick()
	if got := h.snapshot("p-1").Binding.HubProjectID; got != "pj-new" {
		t.Fatalf("p-1's binding carries HubProjectID %q, want pj-new", got)
	}
	h.clearResolved()
	h.w.HubEvents([]deploywatch.Event{{Kind: "project.remote_removed", ProjectID: "pj-c"}})
	h.tick()
	if got := h.resolvedIDs(); len(got) != 1 || got[0] != "p-3" {
		t.Fatalf("the next cycle re-resolved %v, want [p-3] only", got)
	}
}

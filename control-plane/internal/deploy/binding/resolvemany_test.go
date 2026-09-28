package binding_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
)

func countCalls(calls []string, method string) int {
	n := 0
	for _, c := range calls {
		if c == method || strings.HasPrefix(c, method+"(") || strings.HasPrefix(c, method+" ") {
			n++
		}
	}
	return n
}

func fiveLocals(t *testing.T) (*harness, []string) {
	t.Helper()
	h := newHarness(t).withHub(acceptAll)
	ids := []string{"p-1", "p-2", "p-3", "p-4", "p-5"}
	h.local("p-1", "https://github.com/lcfrai/a")
	h.local("p-2", "git@github.com:lcfrai/a.git") // same repository as p-1
	h.local("p-3", "https://github.com/lcfrai/b")
	h.local("p-4", "https://github.com/lcfrai/c")
	h.local("p-5", "https://dev.azure.com/org/proj/_git/d")
	for _, r := range []string{"a", "b", "c"} {
		h.panaceaProject("proj-"+r, "https://github.com/lcfrai/"+r)
	}
	h.panaceaProject("proj-d", "https://org.visualstudio.com/proj/_git/d")
	return h, ids
}

// The mutant: one GET /api/v1/projects per local project per cycle (review
// M16), and a batch that drops an id or answers one project with another's.
func TestResolveManyReadsProjectsOnce(t *testing.T) {
	h, ids := fiveLocals(t)
	r := h.build(t)

	got, err := r.ResolveMany(context.Background(), ids)
	if err != nil {
		t.Fatalf("ResolveMany: %v", err)
	}
	if len(got) != 5 {
		t.Fatalf("entries = %d, want 5", len(got))
	}
	if n := countCalls(h.fake.Calls(), "Projects"); n != 1 {
		t.Errorf("Deployer.Projects called %d times, want 1", n)
	}
	if h.locals.calls != 1 {
		t.Errorf("LocalProjects.Projects called %d times, want 1", h.locals.calls)
	}
	if n := len(h.hub.seen()); n != 4 {
		t.Errorf("ProjectForRepo called %d times, want 4 (one per distinct key)", n)
	}
	want := map[string]deploy.ProjectRef{"p-1": "proj-a", "p-2": "proj-a", "p-3": "proj-b", "p-4": "proj-c", "p-5": "proj-d"}
	for id, ref := range want {
		if b := got[id]; b.State != binding.Bound || b.Panacea != ref {
			t.Errorf("%s: %q %q, want bound/%s", id, b.State, b.Panacea, ref)
		}
	}
}

// The mutant: a Panacea blip that leaves some ids out of the batch entirely.
func TestResolveManyPanaceaDownGivesEveryIDItsLastBinding(t *testing.T) {
	h, ids := fiveLocals(t)
	r := h.build(t)
	if _, err := r.ResolveMany(context.Background(), ids); err != nil {
		t.Fatalf("seed: %v", err)
	}
	h.fake.FailNext("Projects", &deploy.Error{Kind: deploy.KindUnavailable, Op: "projects"})

	got, err := r.ResolveMany(context.Background(), ids)
	if deploy.KindOf(err) != deploy.KindUnavailable {
		t.Fatalf("err = %v, want one unavailable error", err)
	}
	if len(got) != 5 {
		t.Fatalf("entries = %d, want 5", len(got))
	}
	for _, id := range ids {
		if got[id].State != binding.Unavailable || got[id].Panacea == "" {
			t.Errorf("%s: %q %q, want unavailable with its last binding", id, got[id].State, got[id].Panacea)
		}
	}
}

// Carried b09-2 I-1. The mutant: unbounded Hub lookups inside the watcher's
// cycle - a Hub that never answers spends the whole cycle budget, every cycle,
// and every other read in the cycle is starved.
func TestResolveManyBoundsHubLookups(t *testing.T) {
	h, ids := fiveLocals(t)
	h.hubTimeout = 30 * time.Millisecond
	h.hub.delay = 10 * time.Second
	r := h.build(t)

	start := time.Now()
	if _, err := r.ResolveMany(context.Background(), ids); err != nil {
		t.Fatalf("ResolveMany: %v", err)
	}
	if d := time.Since(start); d > 2*time.Second {
		t.Fatalf("ResolveMany took %s against a Hub that never answers", d)
	}
	if n := len(h.hub.seen()); n != 1 {
		t.Errorf("ProjectForRepo called %d times; after the first timeout the rest are skipped", n)
	}
}

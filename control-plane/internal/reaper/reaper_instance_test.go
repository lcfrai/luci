package reaper_test

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/backend"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/registry"
)

// Per-instance ownership (task-25-26-reconcile.md item 11). Two control planes
// on one Docker daemon each see every managed container, because
// LabelManaged says "a Panacea control plane made this" and not which one.
// Before LabelInstance, each read the other's live sandboxes as its own
// orphans and destroyed them, at its very first sweep.
//
// The fake List here returns BOTH instances' sandboxes. That is deliberate:
// dockerbe.List is scoped to its own instance too, and this test is what proves
// the reaper does not lean on that — a backend whose List is not scoped still
// cannot make one control plane destroy another's sandboxes.
func TestEachInstanceSparesTheOtherInstancesSandboxes(t *testing.T) {
	t.Parallel()
	storeA, storeB := newStore(t), newStore(t)
	seed(t, storeA, registry.Session{ID: "s-a", SandboxID: "sb-a", Status: registry.StatusRunning})
	seed(t, storeB, registry.Session{ID: "s-b", SandboxID: "sb-b", Status: registry.StatusRunning})
	be := &fakeBackend{sandboxes: []backend.Sandbox{
		ownedBy(ourInstance, "sb-a", backend.StateRunning),          // A's live session
		ownedBy(ourInstance, "sb-a-orphan", backend.StateRunning),   // A's, no record
		ownedBy(theirInstance, "sb-b", backend.StateRunning),        // B's live session
		ownedBy(theirInstance, "sb-b-orphan", backend.StateRunning), // B's, no record
	}}
	viaA, viaB := &destroyedBy{Backend: be}, &destroyedBy{Backend: be}
	reaperA, logsA := reaperFor(t, ourInstance, storeA, viaA)
	reaperB, logsB := reaperFor(t, theirInstance, storeB, viaB)

	// Twice each, interleaved: the startup sweep and one tick, for both.
	for i := 0; i < 2; i++ {
		if err := reaperA.Sweep(context.Background()); err != nil {
			t.Fatalf("A sweep %d: %v", i, err)
		}
		if err := reaperB.Sweep(context.Background()); err != nil {
			t.Fatalf("B sweep %d: %v", i, err)
		}
	}

	for _, live := range []string{"sb-a", "sb-b"} {
		if be.wasDestroyed(live) {
			t.Errorf("%s was destroyed; one control plane killed a running agent that belongs to a session it cannot see", live)
		}
	}
	for _, orphan := range []string{"sb-a-orphan", "sb-b-orphan"} {
		if !be.wasDestroyed(orphan) {
			t.Errorf("%s survived: ownership scoping stopped its own instance reaping it, which leaks every crashed create", orphan)
		}
	}
	if got := get(t, storeA, "s-a").Status; got != registry.StatusRunning {
		t.Errorf("s-a = %q, want running", got)
	}
	if got := get(t, storeB, "s-b").Status; got != registry.StatusRunning {
		t.Errorf("s-b = %q, want running", got)
	}
	// Each orphan was destroyed by its own instance and only by it: a destroy of
	// the other's orphan would be the same defect in a quieter form. (The fake
	// keeps a destroyed sandbox in its list, so a second sweep destroys an
	// orphan again; the set, not the count, is the assertion.)
	if got := viaA.set(); !reflect.DeepEqual(got, map[string]bool{"sb-a-orphan": true}) {
		t.Errorf("A destroyed %v, want only its own orphan sb-a-orphan", got)
	}
	if got := viaB.set(); !reflect.DeepEqual(got, map[string]bool{"sb-b-orphan": true}) {
		t.Errorf("B destroyed %v, want only its own orphan sb-b-orphan", got)
	}
	for _, c := range []struct {
		name, other string
		logs        string
	}{{"A", "sb-b", logsA.String()}, {"B", "sb-a", logsB.String()}} {
		if !strings.Contains(c.logs, "sparing a foreign sandbox: another control plane owns it") {
			t.Errorf("%s spared the other's sandboxes without saying so; logs:\n%s", c.name, c.logs)
		}
		if n := strings.Count(c.logs, "sandbox="+c.other+" "); n != 1 {
			t.Errorf("%s logged the other's %s %d times over two sweeps, want once; logs:\n%s", c.name, c.other, n, c.logs)
		}
	}
}

// destroyedBy records which reaper destroyed what, over a backend two reapers
// share.
type destroyedBy struct {
	backend.Backend
	mu  sync.Mutex
	ids []string
}

func (d *destroyedBy) Destroy(ctx context.Context, id string) error {
	d.mu.Lock()
	d.ids = append(d.ids, id)
	d.mu.Unlock()
	return d.Backend.Destroy(ctx, id)
}

func (d *destroyedBy) set() map[string]bool {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := map[string]bool{}
	for _, id := range d.ids {
		out[id] = true
	}
	return out
}

// A managed sandbox with no instance label was made by a control plane from
// before this label existed — another agent's older build on this machine, or
// this user's own pre-upgrade session. Nothing can prove whose it is, so
// nothing destroys it: not rule 4 as an orphan, and not rule 1a either, even
// published off loopback, because destroying it would be destroying someone
// else's sandbox on a guess. It is logged once as foreign, so an operator can
// find it; a line per sweep would be two a minute forever.
func TestALegacySandboxWithNoInstanceLabelIsNeverDestroyed(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	legacy := ownedBy("", "sb-old", backend.StateRunning)
	exposed := ownedBy("", "sb-exposed", backend.StateRunning)
	exposed.Endpoint = ""
	exposed.NonLoopback = true
	be := &fakeBackend{sandboxes: []backend.Sandbox{legacy, exposed}}
	r, logs := newReaper(t, store, be, 0, 0)

	for i := 0; i < 3; i++ {
		if err := r.Sweep(context.Background()); err != nil {
			t.Fatalf("Sweep %d: %v", i, err)
		}
	}
	if be.destroyCount() != 0 {
		t.Fatalf("destroyed %v; a sandbox with no instance label belongs to nobody this control plane can name", be.destroyed)
	}
	for _, id := range []string{"sb-old", "sb-exposed"} {
		if n := strings.Count(logs.String(), "sandbox="+id); n != 1 {
			t.Errorf("%s appeared in %d log lines over three sweeps, want exactly 1 (logged once as foreign); logs:\n%s", id, n, logs.String())
		}
	}
	if !strings.Contains(logs.String(), "foreign") {
		t.Errorf("the spared sandboxes were not called foreign; logs:\n%s", logs.String())
	}
}

// The logged-once set follows the daemon: a foreign sandbox that has gone is
// forgotten, so a long-lived control plane beside a busy one does not keep
// every sandbox the other ever made. A real Ref is never reused, so the only
// way to see the forgetting is to take one away and bring the same Ref back.
func TestTheForeignSetForgetsWhatLeftTheDaemon(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	legacy := ownedBy("", "sb-old", backend.StateRunning)
	be := &fakeBackend{}
	r, logs := newReaper(t, store, be, 0, 0)

	for _, present := range []bool{true, false, true} {
		be.mu.Lock()
		be.sandboxes = nil
		if present {
			be.sandboxes = []backend.Sandbox{legacy}
		}
		be.mu.Unlock()
		if err := r.Sweep(context.Background()); err != nil {
			t.Fatalf("Sweep: %v", err)
		}
	}
	if n := strings.Count(logs.String(), "sandbox=sb-old"); n != 2 {
		t.Errorf("logged %d times across present/absent/present, want 2: a sandbox that left the daemon was never forgotten", n)
	}
}

// Another instance's sandbox published off loopback is a security defect —
// and its owner's rule 1a destroys it. Ours must not: a control plane that
// destroys whatever it judges unsafe on a shared daemon is the same defect as
// one that destroys whatever it cannot find in its own registry.
func TestAnotherInstancesNonLoopbackSandboxIsLeftToItsOwner(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	theirs := ownedBy(theirInstance, "sb-theirs", backend.StateRunning)
	theirs.Endpoint = ""
	theirs.NonLoopback = true
	ours := managed("sb-ours", backend.StateRunning)
	ours.Endpoint = ""
	ours.NonLoopback = true
	be := &fakeBackend{sandboxes: []backend.Sandbox{theirs, ours}}
	r, _ := newReaper(t, store, be, 0, 0)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if be.wasDestroyed("sb-theirs") {
		t.Error("destroyed another instance's sandbox")
	}
	if !be.wasDestroyed("sb-ours") {
		t.Error("our own LAN-published sandbox survived; scoping must not blind rule 1a to what it owns")
	}
}

// The trap in "Instance != ours": a reaper built with no instance id would own
// every sandbox that also carries none — every legacy sandbox on the daemon —
// because "" == "". New refuses to read it that way: no instance, owns nothing.
func TestAReaperWithNoInstanceOwnsNothing(t *testing.T) {
	t.Parallel()
	store := newStore(t)
	be := &fakeBackend{sandboxes: []backend.Sandbox{
		ownedBy("", "sb-old", backend.StateRunning),
		ownedBy(ourInstance, "sb-someones", backend.StateRunning),
	}}
	r, logs := reaperFor(t, "", store, be)

	if err := r.Sweep(context.Background()); err != nil {
		t.Fatalf("Sweep: %v", err)
	}
	if be.destroyCount() != 0 {
		t.Fatalf("a reaper with no instance id destroyed %v", be.destroyed)
	}
	if !strings.Contains(logs.String(), "no instance id") {
		t.Errorf("a reaper that can destroy nothing said nothing about why; logs:\n%s", logs.String())
	}
	// Said once, at New — not once per sandbox as though each were a finding.
	if strings.Contains(logs.String(), "sparing a foreign sandbox") {
		t.Errorf("a reaper with no instance logged sandboxes as foreign; with no instance nothing is ours, so that says nothing New did not; logs:\n%s", logs.String())
	}
}

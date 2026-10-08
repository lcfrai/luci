package product

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestSwarmPathResolvesExistingParentsAndOwnedCheckouts(t *testing.T) {
	s, mux, p, run, base, _ := swarmHTTPFixture(t)
	first := mustSwarmPathFixture(t, s, p.ID, run.ID, "op-path-first")
	// Check each existing-prefix depth. On Windows the old filepath.Join
	// relative path first failed when .luci-swarms already existed.
	for _, parent := range []string{
		filepath.Join(s.projectRoot(), ".luci-swarms"),
		filepath.Join(s.projectRoot(), ".luci-swarms", p.ID),
		filepath.Dir(first),
	} {
		if err := os.MkdirAll(parent, 0700); err != nil {
			t.Fatal(err)
		}
		if got := mustSwarmPathFixture(t, s, p.ID, run.ID, "op-path-first"); got != first {
			t.Fatal("existing parents changed owned path", got, first)
		}
	}
	run = swarmReserveHTTP(t, mux, base, run, "task-a", "portable-worker-a-123456")
	run = swarmReserveHTTP(t, mux, base, run, "task-b", "portable-worker-b-123456")
	for _, op := range run.Operations {
		if op.Allocation != "ready" {
			t.Fatalf("existing-prefix allocation failed: %+v", op)
		}
		worker := mustSwarmPathFixture(t, s, p.ID, run.ID, op.ID)
		if err := validateOwnedFixtureWorktree(t, worker); err != nil {
			t.Fatal("worker lacks confined independent metadata", err)
		}
	}
	owned, err := s.ProjectSwarmWorktrees(context.Background(), p.ID)
	if err != nil || len(owned) != 2 {
		t.Fatal("allocated workers cannot be resolved after parent creation", owned, err)
	}
	if _, err := s.swarmPath(p.ID, run.ID, "op-../escape"); err == nil {
		t.Fatal("portable construction relaxed identity traversal refusal")
	}
}

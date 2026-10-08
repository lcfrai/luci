package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func sharedPolicyServer(t *testing.T, root, name, shared string) *Server {
	t.Helper()
	s, err := New(Options{StateDir: filepath.Join(root, name, "state"), ProjectsDir: filepath.Join(root, name, "projects"), SwarmSchedulerPolicyDir: shared})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Close)
	return s
}
func TestSwarmSchedulerSharedHostPolicySerializesWorkersAndKeepsReadsInert(t *testing.T) {
	root := t.TempDir()
	shared := filepath.Join(root, "host-execution")
	a := sharedPolicyServer(t, root, "worker-a", shared)
	b := sharedPolicyServer(t, root, "worker-b", shared)
	mux := http.NewServeMux()
	b.registerSwarmScheduler(mux)
	before := auditTree(t, root)
	journeyRequest(t, mux, "GET", "/v1/product/swarm-scheduler", nil, 200)
	journeyRequest(t, mux, "HEAD", "/v1/product/swarm-scheduler", nil, 200)
	if !reflect.DeepEqual(before, auditTree(t, root)) {
		t.Fatal("host policy read created files or lock")
	}
	low, high := defaultSwarmSchedulerPolicy(), defaultSwarmSchedulerPolicy()
	low.MaxActive = 2
	high.MaxActive = 6
	var wg sync.WaitGroup
	results := make(chan error, 2)
	wg.Add(2)
	go func() { defer wg.Done(); results <- a.updateSharedSwarmPolicy(context.Background(), 1, low, true) }()
	go func() { defer wg.Done(); results <- b.updateSharedSwarmPolicy(context.Background(), 1, high, false) }()
	wg.Wait()
	close(results)
	success, conflicts := 0, 0
	for err := range results {
		if err == nil {
			success++
		} else if errors.Is(err, ErrProjectRunConflict) {
			conflicts++
		} else {
			t.Fatal(err)
		}
	}
	if success != 1 || conflicts != 1 {
		t.Fatalf("cross-worker CAS %d successes %d conflicts", success, conflicts)
	}
	ap, err := a.readSharedSwarmPolicy()
	if err != nil {
		t.Fatal(err)
	}
	bp, err := b.readSharedSwarmPolicy()
	if err != nil || !reflect.DeepEqual(ap, bp) || ap.Revision != 2 {
		t.Fatalf("worker policy drift %+v %+v %v", ap, bp, err)
	}
	// A saved worker-local ledger cannot replace the host's current settings.
	b.swarmScheduler.mu.Lock()
	row, err := b.readSwarmScheduler()
	if err == nil {
		row.Policy = defaultSwarmSchedulerPolicy()
		row.Paused = !ap.Paused
		err = b.saveSwarmScheduler(&row)
	}
	b.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	read, err := b.SwarmSchedulerPolicy()
	if err != nil || read != ap.Policy {
		t.Fatalf("local snapshot overrode host policy %+v %v", read, err)
	}
	journeyRequest(t, mux, "PUT", "/v1/product/swarm-scheduler", map[string]any{"expectedRevision": 2, "policy": defaultSwarmSchedulerPolicy(), "dispatchPaused": true}, 200)
	enabled, err := a.SwarmSchedulerDispatchEnabled()
	if err != nil || enabled {
		t.Fatal("global pause did not reach other worker", err)
	}
	if _, err = os.Stat(filepath.Join(a.stateDir, "swarm-scheduler.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatal("shared policy mutation created unrelated account queue", err)
	}
}
func TestSwarmSchedulerSharedPolicyRejectsUnsafeAndCopiedState(t *testing.T) {
	root := t.TempDir()
	a := sharedPolicyServer(t, root, "worker-a", filepath.Join(root, "host-a"))
	if err := a.updateSharedSwarmPolicy(context.Background(), 1, defaultSwarmSchedulerPolicy(), false); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(root, "host-a", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	copied := filepath.Join(root, "host-b")
	if err = os.Mkdir(copied, 0700); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(copied, "policy.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = New(Options{StateDir: filepath.Join(root, "worker-b", "state"), ProjectsDir: filepath.Join(root, "worker-b", "projects"), SwarmSchedulerPolicyDir: copied}); err == nil {
		t.Fatal("adopted another host policy")
	}
	if _, err = New(Options{StateDir: filepath.Join(root, "worker-c", "state"), ProjectsDir: filepath.Join(root, "worker-c", "projects"), SwarmSchedulerPolicyDir: filepath.Join(root, "worker-c", "projects", "agent-writable")}); err == nil {
		t.Fatal("accepted project-controlled policy directory")
	}
	if err = os.WriteFile(filepath.Join(root, "host-a", "policy.json"), append(data, []byte(` {"extra":true}`)...), 0600); err != nil {
		t.Fatal(err)
	}
	before := auditTree(t, filepath.Join(root, "host-a"))
	if _, err = a.SwarmSchedulerPolicy(); err == nil {
		t.Fatal("accepted trailing policy document")
	}
	if !reflect.DeepEqual(before, auditTree(t, filepath.Join(root, "host-a"))) {
		t.Fatal("corrupt policy was repaired")
	}
}

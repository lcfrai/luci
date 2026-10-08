package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestExecutionLifecycleLockChildProcess(t *testing.T) {
	root := os.Getenv("LUCI_TEST_LIFECYCLE_ROOT")
	if root == "" {
		t.Skip("helper process for the actual cross-process lifecycle lock test")
	}
	g := executionTestGate(t, root)
	g.scope = os.Getenv("LUCI_TEST_LIFECYCLE_SCOPE")
	release, err := g.lockChildLifecycle(context.Background(), "exact-child")
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	_, _ = io.WriteString(os.Stdout, "locked\n")
	_, _ = io.Copy(io.Discard, os.Stdin)
}

func TestSwarmFencedRecoveryRefusesActualLiveAllocatorWithoutWaitingOrEffects(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t, "api")
	a := f.env.api
	a.deps.ReadyTimeout = 5 * time.Second
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	sessionsStubReady(t, nil)
	watchDone := sessionsWatchDone(t)
	pending := &pendingCreateBackend{Backend: f.env.be, entered: make(chan backend.Spec, 1), proceed: make(chan struct{}), active: map[string]bool{}}
	probe := &controlledLaunchFencer{Backend: pending, identity: backend.LaunchFenceIdentity{Protocol: 1, EngineID: "controlled-live-engine", InstanceID: "11111111-1111-4111-8111-111111111111"}, proofs: map[string]backend.LaunchFenceProof{}}
	a.deps.Backend = probe
	var once sync.Once
	unblock := func() { once.Do(func() { close(pending.proceed) }) }
	defer unblock()
	body, _ := json.Marshal(f.requests[0])
	creation := make(chan *httptest.ResponseRecorder, 1)
	go func() { creation <- f.env.do(t, "POST", "/v1/sessions", string(body)) }()
	select {
	case <-pending.entered:
	case <-time.After(10 * time.Second):
		t.Fatal("actual allocator did not enter controlled Create")
	}
	run, err := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
	if err != nil {
		t.Fatal(err)
	}
	op := f.ops[0]
	request, _ := json.Marshal(map[string]any{"expectedRevision": run.Revision, "operationId": op.ID, "childId": op.ChildID, "stopUncertainLaunch": true})
	recovery := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		recovery <- f.env.do(t, "POST", f.base()+"/"+f.run.ID+"/swarm/tasks/api/abandon", string(request))
	}()
	select {
	case response := <-recovery:
		if response.Code != 409 {
			t.Fatal("live allocator was not refused", response.Code, response.Body.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("explicit recovery waited on the active allocator instead of returning a bounded refusal")
	}
	ledger, err := a.execution.readFenceLedger()
	if err != nil || len(ledger.Leases) != 1 || ledger.Leases[0].State != "backend-dispatching" || ledger.Leases[0].FenceProtocol != 1 || len(ledger.Fences) != 0 {
		t.Fatal("live recovery changed its retained launch", ledger, err)
	}
	probe.mu.Lock()
	fences := len(probe.requests)
	probe.mu.Unlock()
	if fences != 0 || f.env.be.destroyCount() != 0 {
		t.Fatal("recovery called backend mutation while original allocator remained active")
	}
	unblock()
	select {
	case response := <-creation:
		if response.Code != 201 {
			t.Fatal("original allocator was interrupted", response.Code, response.Body.String())
		}
	case <-time.After(10 * time.Second):
		t.Fatal("released allocator did not complete")
	}
	sessionsAwaitWatch(t, watchDone)
	row, err := f.store.Get(op.ChildID)
	if err != nil || row.Status != registry.StatusRunning {
		t.Fatal("live child was changed by refused recovery", row, err)
	}
	ledger, err = a.execution.readFenceLedger()
	if err != nil || len(ledger.Leases) != 1 || ledger.Leases[0].State != "backend-settled" || len(ledger.Retirements) != 0 {
		t.Fatal("original successful launch did not retain its exact capacity", ledger, err)
	}
}

func TestExecutionLifecycleLockExcludesLiveControllerAndReleasesAfterProcessDeath(t *testing.T) {
	root := t.TempDir()
	g := executionTestGate(t, filepath.Join(root, "capacity"))
	if err := g.mutate(context.Background(), func(*executionLeaseLedger) error { return nil }); err != nil {
		t.Fatal(err)
	}
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command(exe, "-test.run=^TestExecutionLifecycleLockChildProcess$")
	cmd.Env = append(os.Environ(), "LUCI_TEST_LIFECYCLE_ROOT="+filepath.Dir(g.path), "LUCI_TEST_LIFECYCLE_SCOPE="+g.scope, "TMPDIR="+root, "TMP="+root, "TEMP="+root)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatal(err)
	}
	defer stdin.Close()
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatal(err)
	}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	waited := false
	defer func() {
		if !waited {
			_ = cmd.Process.Kill()
			_ = cmd.Wait()
		}
	}()
	ready := make(chan bool, 1)
	go func() {
		scanner := bufio.NewScanner(stdout)
		for scanner.Scan() {
			if scanner.Text() == "locked" {
				ready <- true
				return
			}
		}
		ready <- false
	}()
	select {
	case ok := <-ready:
		if !ok {
			t.Fatal("child did not acquire the actual lifecycle file lock")
		}
	case <-time.After(10 * time.Second):
		t.Fatal("child lock acquisition did not complete")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	_, err = g.lockChildLifecycle(ctx, "exact-child")
	cancel()
	if !errors.Is(err, errExecutionLifecycleBusy) {
		t.Fatal("second controller did not receive bounded busy refusal", err)
	}
	// The child lock neither serializes unrelated children nor takes the
	// host-capacity ledger lock for the duration of a backend request.
	release, err := g.lockChildLifecycle(context.Background(), "different-child")
	if err != nil {
		t.Fatal("unrelated child was blocked", err)
	}
	release()
	if err := g.mutate(context.Background(), func(*executionLeaseLedger) error { return nil }); err != nil {
		t.Fatal("host ledger was held by child lifecycle lock", err)
	}
	if err := cmd.Process.Kill(); err != nil {
		t.Fatal(err)
	}
	_ = cmd.Wait() // Deliberate controller death is the crash being tested.
	waited = true
	release, err = g.lockChildLifecycle(context.Background(), "exact-child")
	if err != nil {
		t.Fatal("OS did not release the dead controller's lifecycle lock", err)
	}
	release()
}

package api

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// Explicit daemon facts authored by the controlled backend fixture. No attach
// EOF, model output, process success or real Docker observation is fabricated.
func controlledTaskContainerExit(row registry.Session, code int, oom bool) *backend.TaskExitObservation {
	return &backend.TaskExitObservation{Ref: row.SandboxRef, Image: row.TaskTerminalImage, ContractDigest: row.Args[1], CreatedAt: row.TaskTerminalCreatedAt, StartedAt: row.TaskTerminalStartedAt, FinishedAt: row.TaskTerminalStartedAt.Add(time.Second), ContainerExitCode: code, OOMKilled: oom, NormalZero: code == 0 && !oom}
}

func writeControlledTerminalReceipt(t *testing.T, row registry.Session, directory string, report []byte) {
	t.Helper()
	if !terminalSession(row) {
		return
	}
	data, err := json.Marshal(map[string]any{"version": 1, "contractDigest": row.Args[1], "reportDigest": reviewedHash(report)})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "terminal-receipt.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func TestTaskContainerExitNormalZeroSurvivesObserverExpiryCleanupAndRestart(t *testing.T) {
	f, row, _ := settledTaskFixture(t)
	var err error
	row, err = f.store.Update(row.ID, func(current *registry.Session) error {
		current.CreatedAt = current.CreatedAt.Add(-2 * time.Hour)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, valid := taskTerminalObservationDeadline(row.CreatedAt, time.Now()); valid {
		t.Fatal("fixture did not exceed the original attach observation window")
	}
	completionReport(t, f, row)
	be := completionReaper(t, f, row, nil)
	stored, err := f.store.Get(row.ID)
	if err != nil || !stored.HasTaskContainerResult() || !stored.TaskTerminalResult.NormalZero || stored.TaskTerminalExitZero || !be.absent {
		t.Fatal("durable daemon fact depended on a live typed Exit/EOF", stored.TaskTerminalResult, err)
	}
	f.reopen(t)
	for attempt := 0; attempt < 2; attempt++ {
		if err := f.env.api.ObserveTaskSessionsOnce(context.Background()); err != nil {
			t.Fatal(err)
		}
	}
	handoffReloadRun(t, f)
	stored, err = f.store.Get(row.ID)
	if err != nil || !stored.TaskObservationComplete || f.run.Operations[0].Handoff == nil || f.run.Operations[0].Handoff.Status != "captured" || controlledAllocationCount(f.env.be) != 1 || be.removes != 1 || be.forceCalls != 0 {
		t.Fatal("restart did not observe the retained original without relaunch", err)
	}
}

func TestTaskContainerUnsuccessfulExitIsRetainedCleanedAndObservedAsFailed(t *testing.T) {
	for _, oom := range []bool{false, true} {
		t.Run(map[bool]string{false: "nonzero", true: "oom"}[oom], func(t *testing.T) {
			f, row, _ := settledTaskFixture(t)
			be := &taskCleanupLifecycleBackend{Backend: f.env.api.deps.Backend, sandbox: backend.Sandbox{ID: row.SandboxID, Ref: row.SandboxRef, State: backend.StateStopped}, terminal: controlledTaskContainerExit(row, 1, oom)}
			f.env.api.deps.Backend = be
			if err := f.env.api.ConfirmStoppedTaskCleanup(context.Background(), row); err != nil {
				t.Fatal("known unsuccessful terminal fact retained the stopped CID forever", err)
			}
			if err := be.DestroyStopped(context.Background(), be.sandbox); err != nil {
				t.Fatal(err)
			}
			if err := f.env.api.ObserveTaskSessionsOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			handoffReloadRun(t, f)
			stored, err := f.store.Get(row.ID)
			if err != nil || !stored.TaskObservationComplete || !stored.HasTaskContainerResult() || stored.TaskTerminalResult.NormalZero || stored.TaskTerminalResult.OOMKilled != oom || f.run.Operations[0].Status != "failed" || f.run.Operations[0].Handoff != nil || be.removes != 1 || be.forceCalls != 0 || controlledAllocationCount(f.env.be) != 1 {
				t.Fatal("negative terminal fact neither failed nor settled exact original", stored, err)
			}
		})
	}
}

func TestTaskContainerExitIdentityUncertaintyHoldsOriginalAndGenericCompatibility(t *testing.T) {
	f, row, _ := settledTaskFixture(t)
	malformed := row.Clone()
	malformed.Args = []string{taskTerminalWrapper}
	if observed, err := f.env.api.observeTaskContainerExit(context.Background(), malformed); observed || err == nil {
		t.Fatal("malformed retained contract was not held", observed, err)
	}
	be := &taskCleanupLifecycleBackend{Backend: f.env.api.deps.Backend, sandbox: backend.Sandbox{ID: row.SandboxID, Ref: row.SandboxRef, State: backend.StateStopped}, terminal: controlledTaskContainerExit(row, 0, false)}
	be.terminal.Ref = "another-original"
	f.env.api.deps.Backend = be
	if err := f.env.api.ConfirmStoppedTaskCleanup(context.Background(), row); err == nil {
		t.Fatal("changed original CID became a terminal fact")
	}
	stored, err := f.store.Get(row.ID)
	if err != nil || stored.TaskTerminalResult != nil || stored.Status != registry.StatusRunning || be.removes != 0 {
		t.Fatal("uncertainty changed original lifecycle", err)
	}
	legacy := row.Clone()
	legacy.Command, legacy.Args, legacy.TaskTerminalImage = "/bin/bash", []string{"-l", "-i"}, ""
	if observed, err := f.env.api.observeTaskContainerExit(context.Background(), legacy); observed || !errors.Is(err, backend.ErrUnsupported) {
		t.Fatal("generic interactive session gained task termination semantics", observed, err)
	}
}

func TestTaskContainerPreAllocationFailureUsesCanonicalDiagnosticWithoutExitFact(t *testing.T) {
	f := newSwarmSessionFixtureTasks(t, "api")
	a := f.env.api
	actor := management.ExecutionActorRef{Local: true, ProjectID: f.owner.ID}
	f.persistQueuedAttempt(t, "queue-terminal-preallocation", actor)
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
	calls := 0
	a.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
		calls++
		if calls >= 3 {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{}, nil
	}
	policy, err := a.deps.Product.SwarmSchedulerPolicy()
	if err != nil {
		t.Fatal(err)
	}
	admitted, err := a.admitQueuedSwarm(context.Background(), product.SwarmAdmissionRequest{QueueID: "queue-terminal-preallocation", ProjectID: f.owner.ID, RunID: f.run.ID, Actor: actor, Policy: policy})
	if err != nil || admitted.State != "admitted" {
		t.Fatal(admitted, err)
	}
	result, err := a.dispatchQueuedSwarm(context.Background(), product.SwarmDispatchRequest{QueueID: "queue-terminal-preallocation", ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: f.ops[0].ID, Actor: actor})
	if err != nil || result.State != "failed" || result.SessionID == "" {
		t.Fatal(result, err)
	}
	if err := a.ObserveTaskSessionsOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	row, err := f.store.Get(result.SessionID)
	if err != nil || !row.TaskObservationComplete || !row.StartFailure.Valid() || row.SandboxRef != "" || row.TaskTerminalResult != nil || controlledAllocationCount(f.env.be) != 0 {
		t.Fatal("pre-allocation failure waited for nonexistent daemon evidence or invented termination", row, err)
	}
}

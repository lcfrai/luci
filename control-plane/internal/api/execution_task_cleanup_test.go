package api

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestExecutionTaskLeaseSurvivesEmptyActiveInventoryAndReopenUntilBackendAbsent(t *testing.T) {
	f, row, ledger := settledTaskFixture(t)
	a := f.env.api
	row, err := f.store.Update(row.ID, func(current *registry.Session) error { current.Status = registry.StatusExited; return nil })
	if err != nil {
		t.Fatal(err)
	}
	a.deps.Backend = swarmObserverBackend{Backend: a.deps.Backend, get: func(context.Context, string) (backend.Sandbox, error) {
		return backend.Sandbox{ID: row.SandboxID, State: backend.StateStopped}, nil
	}}
	// This fresh inventory is complete for active workloads. Stopped container
	// objects are deliberately not included by the real resource observer.
	a.deps.ResourceSnapshot = executionFixtureSnapshot
	for attempt := 0; attempt < 2; attempt++ {
		if err := a.execution.mutate(context.Background(), func(current *executionLeaseLedger) error {
			if err := a.execution.reconcileOwned(context.Background(), current); err != nil {
				return err
			}
			if len(current.Leases) != 1 || current.Leases[0].ID != ledger.Leases[0].ID {
				t.Fatal("active-only inventory discarded task cleanup proof", current)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
		if err := a.execution.release(context.Background(), ledger.Leases[0].ID); !errors.Is(err, ErrExecutionLeaseHeld) {
			t.Fatal("stopped container released its task reservation", err)
		}
		a.execution = newExecutionAdmission(a)
	}
	if err := a.ConfirmStoppedTaskCleanup(context.Background(), row); err != nil {
		t.Fatal("reopened allocator lost exact cleanup proof", err)
	}
	for _, failure := range []error{backend.ErrUnavailable, errors.Join(backend.ErrNotFound, errors.New("transport interrupted"))} {
		a.deps.Backend = swarmObserverBackend{Backend: a.deps.Backend, get: func(ctx context.Context, _ string) (backend.Sandbox, error) {
			if deadline, bounded := ctx.Deadline(); !bounded || time.Until(deadline) > 2*time.Second {
				t.Fatal("reconciliation backend lookup has no shared bounded context")
			}
			return backend.Sandbox{}, failure
		}}
		if err := a.execution.mutate(context.Background(), func(current *executionLeaseLedger) error {
			if err := a.execution.reconcileOwned(context.Background(), current); err != nil {
				return err
			}
			if len(current.Leases) != 1 {
				t.Fatal("uncertain backend observation discarded cleanup proof", current)
			}
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	a.deps.Backend = swarmObserverBackend{Backend: a.deps.Backend, get: func(context.Context, string) (backend.Sandbox, error) {
		return backend.Sandbox{}, backend.ErrNotFound
	}}
	if err := a.execution.release(context.Background(), ledger.Leases[0].ID); err != nil {
		t.Fatal("confirmed removal did not release task capacity", err)
	}
	if err := a.execution.mutate(context.Background(), func(current *executionLeaseLedger) error {
		if len(current.Leases) != 0 {
			t.Fatal("removed task reservation retained", current)
		}
		return nil
	}); err != nil {
		t.Fatal(err)
	}
}

func TestExecutionRunCleanupPolicyRequiresExactPrivateOperation(t *testing.T) {
	f, row, _ := settledTaskFixture(t)
	if absent, err := f.env.api.executionRequiresBackendAbsence(row); err != nil || !absent {
		t.Fatal("actual task did not require removal", absent, err)
	}
	for _, scenario := range []string{"missing run", "missing operation", "wrong operation", "wrong worktree", "changed request"} {
		t.Run(scenario, func(t *testing.T) {
			changed := row.Clone()
			switch scenario {
			case "missing run":
				changed.ProjectRunID = ""
			case "missing operation":
				changed.ProjectRunOperationID = ""
			case "wrong operation":
				changed.ProjectRunOperationID = "op-other"
			case "wrong worktree":
				changed.WorktreeID = "w-other"
			case "changed request":
				changed.Model += "-changed"
			}
			if absent, err := f.env.api.executionRequiresBackendAbsence(changed); err == nil || !absent {
				t.Fatal("unverified task received ordinary-session release policy", absent, err)
			}
		})
	}
	env, run, op, _, serial := reservedProjectRunSession(t, false)
	admission, err := env.api.deps.Product.BeginProjectRunSession(run.ProjectID, run.ID, op.ID, serial, env.st.Get)
	if err != nil {
		t.Fatal(err)
	}
	serial.ID = admission.SessionID
	admission.Release()
	if absent, err := env.api.executionRequiresBackendAbsence(serial); err != nil || absent {
		t.Fatal("positively identified serial run lost existing cleanup behavior", absent, err)
	}
}

func TestExactBackendAbsenceRequiresUnambiguousRemoval(t *testing.T) {
	env := sessionsNewEnv(t)
	for _, tc := range []struct {
		name            string
		sb              backend.Sandbox
		err             error
		absent, failure bool
	}{
		{"stopped still present", backend.Sandbox{ID: "s-exact", State: backend.StateStopped}, nil, false, false},
		{"gone", backend.Sandbox{ID: "s-exact", State: backend.StateGone}, nil, true, false},
		{"not found", backend.Sandbox{}, backend.ErrNotFound, true, false},
		{"wrapped not found", backend.Sandbox{}, fmt.Errorf("exact lookup: %w", backend.ErrNotFound), true, false},
		{"joined unknown transport", backend.Sandbox{}, errors.Join(backend.ErrNotFound, errors.New("transport interrupted")), false, true},
		{"wrapped joined not found", backend.Sandbox{}, fmt.Errorf("lookup: %w", errors.Join(backend.ErrNotFound, backend.ErrUnavailable)), false, true},
		{"wrong identity", backend.Sandbox{ID: "s-other", State: backend.StateGone}, nil, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			env.api.deps.Backend = swarmObserverBackend{Backend: env.be, get: func(context.Context, string) (backend.Sandbox, error) { return tc.sb, tc.err }}
			absent, err := env.api.exactBackendTerminal(context.Background(), "s-exact", true)
			if absent != tc.absent || (err != nil) != tc.failure {
				t.Fatal("incorrect backend absence witness", absent, err)
			}
		})
	}
}

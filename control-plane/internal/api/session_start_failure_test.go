package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestSessionStartFailureBeforeBackendIsBoundedAndDurable(t *testing.T) {
	for _, tc := range []struct{ name, code string }{{"unknown", "admission_unavailable"}, {"paused", "dispatch_paused"}, {"policy", "resource_policy_changed"}} {
		t.Run(tc.name, func(t *testing.T) {
			env := sessionsNewEnv(t)
			row := sessionsFullRecord()
			row.Status = registry.StatusStarting
			row.SandboxRef = ""
			row.Endpoint = ""
			row.AttachToken = ""
			cause := errors.New("private-token-secret at /private/host/path")
			refusal := error(cause)
			if tc.name != "unknown" {
				refusal = admissionFailure(tc.code, cause)
			}
			_, err := env.api.allocatePreparedSession(context.Background(), preparedSessionAllocation{Record: row, Spec: backend.Spec{ID: row.ID, SessionID: row.ID}, BeforeAllocate: func(context.Context) error { return refusal }})
			var failed *sessionAllocationError
			if !errors.As(err, &failed) || !errors.Is(err, cause) || failed.StartFailure != registry.NewSessionStartFailure("admission", tc.code) {
				t.Fatal("lost bounded stage/internal cause", err, failed)
			}
			saved, err := env.api.deps.Store.Get(row.ID)
			if err != nil || saved.Status != registry.StatusExited || saved.StartFailure != failed.StartFailure || controlledAllocationCount(env.be) != 0 {
				t.Fatal("refusal allocated or lost exact history", err, saved.Status)
			}
			w := httptest.NewRecorder()
			env.api.writeSessionAllocationProblem(w, httptest.NewRequest("POST", "/v1/sessions", nil), failed)
			raw, _ := json.Marshal(toJSON(saved, sessionBasePath))
			for _, value := range []string{w.Body.String(), string(raw)} {
				if strings.Contains(value, "private-token-secret") || strings.Contains(value, "/private/host/path") || !strings.Contains(value, tc.code) {
					t.Fatal("failure response lost safe reason or leaked cause", value)
				}
			}
		})
	}
}

func TestSessionStartFailureNeverClassifiesBackendCallAsNotStarted(t *testing.T) {
	env := sessionsNewEnv(t)
	row := sessionsFullRecord()
	row.Status = registry.StatusStarting
	row.SandboxRef = ""
	row.Endpoint = ""
	env.be.createErr = errors.New("controlled ambiguous Create response")
	_, err := env.api.allocatePreparedSession(context.Background(), preparedSessionAllocation{Record: row, Spec: backend.Spec{ID: row.ID, SessionID: row.ID}})
	var failure *sessionAllocationError
	if !errors.As(err, &failure) || failure.Kind != "backend" || !failure.StartFailure.IsZero() || controlledAllocationCount(env.be) != 1 {
		t.Fatal("ambiguous backend call mislabeled", err)
	}
	saved, _ := env.api.deps.Store.Get(row.ID)
	if !saved.StartFailure.IsZero() {
		t.Fatal("late backend diagnostic fabricated never-started")
	}
}

func TestSessionStartFailureCannotOverwriteChangedGeneration(t *testing.T) {
	env := sessionsNewEnv(t)
	row := sessionsFullRecord()
	row.Status = registry.StatusStarting
	row.SandboxRef = ""
	if err := env.api.deps.Store.Create(row); err != nil {
		t.Fatal(err)
	}
	if _, err := env.api.deps.Store.Update(row.ID, func(current *registry.Session) error {
		current.SandboxRef = "new-generation-ref"
		current.Status = registry.StatusRunning
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	diagnostic := env.api.recordSessionStartFailure(row, "admission", errors.New("late refusal"))
	current, _ := env.api.deps.Store.Get(row.ID)
	if !diagnostic.IsZero() || current.Status != registry.StatusRunning || !current.StartFailure.IsZero() {
		t.Fatal("old diagnostic changed current execution")
	}
}

func TestSessionStartFailureClearsOnOrdinaryInteractiveRestart(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	row := sessionsStarting("s-0000000b")
	row.Status = registry.StatusExited
	row.SandboxRef = ""
	row.StartFailure = registry.NewSessionStartFailure("admission", "dispatch_paused")
	row.ExitReason = row.StartFailure.Message
	sessionsSeed(t, env.st, row)
	w := env.do(t, "POST", "/v1/sessions/"+row.ID+"/restart", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	sessionsAwaitWatch(t, done)
	current, err := env.st.Get(row.ID)
	if err != nil || current.Status != registry.StatusRunning || current.SandboxRef == "" || !current.StartFailure.IsZero() || current.ExitReason != "" {
		t.Fatal("new generation retained failed-start diagnosis", err, current.Status, current.StartFailure)
	}
}

func TestSessionStartFailureBoundTaskLateBackendFailureRemainsUnknown(t *testing.T) {
	for _, stage := range []string{"create", "acknowledgment"} {
		t.Run(stage, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t, "api")
			a := f.env.api
			a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
			a.deps.ResourceSnapshot = executionFixtureSnapshot
			if stage == "create" {
				f.env.be.createErr = errors.New("controlled lost Create reply")
			} else {
				a.deps.Backend = acknowledgedCreateBackend{Backend: f.env.be, returnedID: "s-foreign"}
			}
			raw, _ := json.Marshal(f.requests[0])
			w := f.env.do(t, "POST", "/v1/sessions", string(raw))
			if w.Code < 400 {
				t.Fatal("ambiguous backend launch reported success", w.Code)
			}
			row, err := f.store.Get(f.ops[0].ChildID)
			if err != nil || row.Status != registry.StatusExited || row.ExitReason != "project-run-outcome-unknown" || !row.StartFailure.IsZero() || controlledAllocationCount(f.env.be) != 1 {
				t.Fatal("backend ambiguity mislabeled never-started", err, row.Status, row.ExitReason, row.StartFailure)
			}
			a.deps.Backend = &executionGoneBackend{Backend: f.env.be}
			// Even a currently absent object is not a completed Create/Start receipt.
			if err := a.execution.releaseSession(context.Background(), row.ID); !errors.Is(err, ErrExecutionLeaseHeld) {
				t.Fatal("late call uncertainty released capacity", err)
			}
			if err := a.ObserveTaskSessionsOnce(context.Background()); err == nil {
				t.Fatal("unconfirmed launch produced completion")
			}
			run, err := a.deps.Product.GetProjectSwarmRun(f.owner.ID, f.run.ID)
			if err != nil || run.Operations[0].Status != "unknown" || !run.Operations[0].StartFailure.IsZero() || run.Operations[0].Handoff != nil {
				t.Fatal("observation invented a worker result", err, run.Operations[0].Status)
			}
			f.reopen(t)
			if err := f.env.api.execution.releaseSession(context.Background(), row.ID); !errors.Is(err, ErrExecutionLeaseHeld) {
				t.Fatal("restart released ambiguous launch", err)
			}
		})
	}
}

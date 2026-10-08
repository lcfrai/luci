package api

import (
	"context"
	"encoding/json"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"net/http"
	"path/filepath"
	"testing"
)

func restartAdmissionFixture(t *testing.T) (*swarmSessionFixture, registry.Session) {
	t.Helper()
	f := newSwarmSessionFixtureTasks(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	body, _ := json.Marshal(map[string]any{"worktreeId": project.WorktreeID(f.owner.Path), "harness": "shell"})
	created := sessionsDecodeOne(t, f.env.do(t, http.MethodPost, "/v1/sessions", string(body)), http.StatusCreated)
	sessionsAwaitWatch(t, done)
	row, e := f.store.Get(created.ID)
	if e != nil {
		t.Fatal(e)
	}
	f.env.api.deps.ExecutionStateDir = filepath.Join(t.TempDir(), "capacity")
	f.env.api.deps.ResourceSnapshot = executionFixtureSnapshot
	f.env.api.execution = newExecutionAdmission(f.env.api)
	return f, row
}

func TestRestartAdmissionDeniedBeforeStoppingOriginal(t *testing.T) {
	for _, reason := range []string{"access", "capacity"} {
		t.Run(reason, func(t *testing.T) {
			f, prior := restartAdmissionFixture(t)
			if reason == "access" {
				f.env.api.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
					return management.ExecutionAuthority{}, management.ErrExecutionAuthority
				}
			} else {
				f.env.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
					s := executionFixtureSnapshot()
					s.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(1))
					return s
				}
			}
			response := f.env.do(t, http.MethodPost, "/v1/sessions/"+prior.ID+"/restart", "")
			expected := http.StatusConflict
			if reason == "access" {
				expected = http.StatusForbidden
			}
			if response.Code != expected {
				t.Fatalf("unexpected refusal %d: %s", response.Code, response.Body.String())
			}
			if f.env.be.destroyCount() != 0 || len(f.env.be.specs) != 1 {
				t.Fatal("refused restart changed backend workload")
			}
			after, _ := f.store.Get(prior.ID)
			if after.SandboxID != prior.SandboxID || after.AttachToken != prior.AttachToken || after.Status != prior.Status {
				t.Fatal("refused restart changed original record")
			}
		})
	}
}

func TestRestartAdmissionRechecksAfterDestroyBeforeReplacement(t *testing.T) {
	f, prior := restartAdmissionFixture(t)
	allowed := true
	calls := 0
	f.env.api.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
		calls++
		if !allowed {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{}, nil
	}
	f.env.be.onDestroy = func(string) { allowed = false }
	response := f.env.do(t, http.MethodPost, "/v1/sessions/"+prior.ID+"/restart", "")
	if response.Code != http.StatusConflict {
		t.Fatalf("unexpected response %d: %s", response.Code, response.Body.String())
	}
	if calls < 3 || f.env.be.destroyCount() != 1 || len(f.env.be.specs) != 1 {
		t.Fatalf("late authorization was not enforced: auth=%d destroys=%d creates=%d", calls, f.env.be.destroyCount(), len(f.env.be.specs))
	}
	after, _ := f.store.Get(prior.ID)
	if after.Status != registry.StatusExited {
		t.Fatalf("stopped original reported active: %+v", after)
	}
}

func TestRestartAdmissionKeepsNewSandboxLeaseUntilConfirmedExit(t *testing.T) {
	f, prior := restartAdmissionFixture(t)
	done := sessionsWatchDone(t)
	response := f.env.do(t, http.MethodPost, "/v1/sessions/"+prior.ID+"/restart", "")
	if response.Code != http.StatusOK {
		t.Fatalf("restart %d: %s", response.Code, response.Body.String())
	}
	sessionsAwaitWatch(t, done)
	after, e := f.store.Get(prior.ID)
	if e != nil {
		t.Fatal(e)
	}
	if after.SandboxID == prior.SandboxID || len(f.env.be.specs) != 2 || f.env.be.specs[1].ID != after.SandboxID {
		t.Fatal("restart did not retain a distinct exact allocation")
	}
	var found bool
	e = f.env.api.execution.mutate(context.Background(), func(ledger *executionLeaseLedger) error {
		for _, lease := range ledger.Leases {
			if lease.SessionID == prior.ID {
				found = true
				if lease.State != "backend-dispatching" {
					t.Fatalf("replacement omitted durable dispatch boundary: %s", lease.State)
				}
				if lease.sandboxID() != after.SandboxID {
					t.Fatal("replacement lease still identifies old sandbox")
				}
			}
		}
		return nil
	})
	if e != nil || !found {
		t.Fatalf("live replacement lost capacity reservation: %v", e)
	}
}

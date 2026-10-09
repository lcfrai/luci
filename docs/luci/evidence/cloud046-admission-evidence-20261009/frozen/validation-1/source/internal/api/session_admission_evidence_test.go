package api

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// The first admission succeeds. Recording the exact child changes the external
// sample before final admission, without changing policy or bypassing dispatch.
func TestAdmissionEvidenceRetainsFinalSampleAcrossReopen(t *testing.T) {
	for _, kind := range []string{"cpu", "host-memory", "execution-memory"} {
		t.Run(kind, func(t *testing.T) {
			f := newSwarmSessionFixtureTasks(t, "api")
			a := f.env.api
			a.deps.Product.SetSwarmSchedulerRuntime(product.SwarmSchedulerRuntime{})
			if err := a.deps.Product.RunSwarmSchedulerOnce(context.Background()); err != nil {
				t.Fatal(err)
			}
			a.deps.Resources = backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}
			a.deps.AuthorizeExecution = func(context.Context, management.ExecutionActorRef, string, bool) (management.ExecutionAuthority, error) {
				return management.ExecutionAuthority{}, nil
			}
			var created atomic.Bool
			var initialReads, finalReads atomic.Int64
			a.deps.Store = &queuedStartFailureStore{FileStore: f.store, afterCreate: func() { created.Store(true) }}
			initial, final := executionFixtureSnapshot(), executionFixtureSnapshot()
			sampleAt := time.Now().Add(-100 * time.Millisecond).UTC()
			final.Host.ObservedAt, final.Execution.ObservedAt = sampleAt, sampleAt
			code, metric := "memory_pressure", "host_available_memory"
			observed, required := float64(uint64(512)<<20), float64(uint64(5)<<30)
			switch kind {
			case "cpu":
				code, metric = "cpu_pressure", "host_cpu_utilization_percent"
				final.Host.CPU.UtilizationPercent = executionPointer(81.25)
				observed, required = 81.25, 75
			case "host-memory":
				final.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(512) << 20)
			case "execution-memory":
				metric = "execution_total_memory"
				final.Execution.TotalMemoryBytes = executionPointer(uint64(2) << 30)
				observed, required = float64(uint64(2)<<30), float64(uint64(3)<<30)
			}
			a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				if created.Load() {
					finalReads.Add(1)
					return final
				}
				initialReads.Add(1)
				return initial
			}
			actor := management.ExecutionActorRef{Local: true, ProjectID: f.owner.ID}
			policy, err := a.deps.Product.SwarmSchedulerPolicy()
			if err != nil {
				t.Fatal(err)
			}
			admitted, err := a.admitQueuedSwarm(context.Background(), product.SwarmAdmissionRequest{QueueID: "queue-evidence", ProjectID: f.owner.ID, RunID: f.run.ID, Actor: actor, Policy: policy})
			if err != nil || admitted.State != "admitted" {
				t.Fatal("initial capacity did not admit", admitted, err)
			}
			result, err := a.dispatchQueuedSwarm(context.Background(), product.SwarmDispatchRequest{QueueID: "queue-evidence", ProjectID: f.owner.ID, RunID: f.run.ID, OperationID: f.ops[0].ID, Actor: actor})
			if err != nil || result.State != "failed" || result.ReasonCode != code || controlledAllocationCount(f.env.be) != 0 || initialReads.Load() == 0 || finalReads.Load() == 0 {
				t.Fatal("final refusal did not retain a never-allocated attempt", result, err)
			}
			check := func(store registry.Store) string {
				t.Helper()
				row, err := store.Get(result.SessionID)
				if err != nil || row.Status != registry.StatusExited || row.StartFailure != registry.NewSessionStartFailure("admission", code) {
					t.Fatal("canonical failure changed", err, row.StartFailure)
				}
				raw, err := json.Marshal(row)
				if err != nil {
					t.Fatal(err)
				}
				var stored map[string]json.RawMessage
				if err := json.Unmarshal(raw, &stored); err != nil {
					t.Fatal(err)
				}
				var evidence map[string]any
				if len(stored["startAdmissionEvidence"]) == 0 || json.Unmarshal(stored["startAdmissionEvidence"], &evidence) != nil {
					t.Fatal("actual final admission operands were not retained")
				}
				observedKey, requiredKey := "observedBytes", "requiredBytes"
				if kind == "cpu" {
					observedKey, requiredKey = "observedPercent", "maximumPercent"
				}
				if evidence["metric"] != metric || evidence[observedKey] != observed || evidence[requiredKey] != required || evidence["observedAt"] != sampleAt.Format(time.RFC3339Nano) {
					t.Fatal("evidence was resampled or did not capture the actual refusal", evidence)
				}
				decided, err := time.Parse(time.RFC3339Nano, evidence["decidedAt"].(string))
				if err != nil || decided.Before(sampleAt) || decided.Sub(sampleAt) > 5*time.Second {
					t.Fatal("decision time does not bind the accepted sample", evidence)
				}
				return string(stored["startAdmissionEvidence"])
			}
			before := check(f.store)
			reopened, err := registry.NewFileStore(f.storePath)
			if err != nil || check(reopened) != before {
				t.Fatal("reopening changed the actual decision evidence", err)
			}
		})
	}
}

func admissionEvidenceFixture() registry.StartAdmissionEvidence {
	now := time.Now().UTC()
	return registry.StartAdmissionEvidence{Metric: "host_cpu_utilization_percent", ObservedAt: now.Add(-time.Second), DecidedAt: now, ObservedPercent: 81.25, MaximumPercent: 75}
}

func TestAdmissionEvidenceSingleGetUsesCurrentHostAuthority(t *testing.T) {
	env := sessionsNewEnv(t)
	row := sessionsStarting("s-0000000b")
	row.Status, row.SandboxRef = registry.StatusExited, ""
	row.StartFailure = registry.NewSessionStartFailure("admission", "cpu_pressure")
	row.StartAdmissionEvidence = admissionEvidenceFixture()
	sessionsSeed(t, env.st, row)
	for _, tc := range []struct {
		name, role           string
		team, disabled, want bool
	}{
		{"local host", "", false, false, true},
		{"administrator", "admin", false, false, true},
		{"super administrator", "super-admin", false, false, true},
		{"same user demoted", "member", false, false, false},
		{"same user disabled", "admin", false, true, false},
		{"team administrator", "admin", true, false, false},
		{"team super administrator", "super-admin", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			if tc.role != "" {
				authority := management.ExecutionAuthority{Actor: management.User{ID: "same-user", Role: tc.role, Disabled: tc.disabled}}
				if tc.team {
					authority.Team = &management.TeamAccess{Team: management.Team{ID: "team-private"}}
				}
				ctx = management.WithExecutionAuthority(ctx, authority)
			}
			get := func(path string) *httptest.ResponseRecorder {
				r := httptest.NewRequest("GET", sessionsTarget(path), nil).WithContext(ctx)
				w := httptest.NewRecorder()
				env.mux.ServeHTTP(w, r)
				if w.Code != 200 {
					t.Fatal(w.Code, w.Body.String())
				}
				return w
			}
			w := get("/v1/sessions/" + row.ID)
			var value map[string]json.RawMessage
			if json.Unmarshal(w.Body.Bytes(), &value) != nil {
				t.Fatal("invalid session JSON")
			}
			if _, present := value["startAdmissionEvidence"]; present != tc.want {
				t.Fatal("historical host telemetry crossed current authority", w.Body.String())
			}
			var failure registry.SessionStartFailure
			if json.Unmarshal(value["startFailure"], &failure) != nil || failure != row.StartFailure {
				t.Fatal("fixed public failure changed")
			}
			if strings.Contains(get("/v1/sessions").Body.String(), "startAdmissionEvidence") {
				t.Fatal("host telemetry leaked through list")
			}
		})
	}
	// Shared DTOs serve mutation and other session responses, never telemetry.
	raw, err := json.Marshal(toJSON(row, sessionBasePath))
	if err != nil || strings.Contains(string(raw), "startAdmissionEvidence") {
		t.Fatal("shared session DTO leaked evidence", err)
	}
	w := env.do(t, "PATCH", "/v1/sessions/"+row.ID, `{"title":"Retained attempt"}`)
	if w.Code != 200 || strings.Contains(w.Body.String(), "startAdmissionEvidence") {
		t.Fatal("mutation response leaked evidence", w.Code)
	}
	// Historical evidence is immutable by value; public projection cannot alias it.
	r := httptest.NewRequest("GET", sessionsTarget("/v1/sessions/"+row.ID), nil)
	exposed := sessionAdmissionEvidence(r, row)
	exposed.ObservedPercent = 99
	if row.StartAdmissionEvidence.ObservedPercent != 81.25 {
		t.Fatal("projection changed retained operands")
	}
	for _, change := range []func(*registry.Session){
		func(s *registry.Session) { s.Status = registry.StatusRunning },
		func(s *registry.Session) { s.SandboxRef = "allocated" },
		func(s *registry.Session) {
			s.StartFailure = registry.NewSessionStartFailure("admission", "memory_pressure")
		},
		func(s *registry.Session) { s.StartAdmissionEvidence.Metric = "private-context" },
	} {
		changed := row
		change(&changed)
		if sessionAdmissionEvidence(r, changed) != nil {
			t.Fatal("invalid generation or malformed evidence was exposed")
		}
	}
}

func TestAdmissionEvidenceMalformedDoesNotPreventDurableFailure(t *testing.T) {
	for _, tc := range []struct {
		name   string
		change func(*registry.StartAdmissionEvidence)
	}{
		{"nonfinite", func(e *registry.StartAdmissionEvidence) { e.ObservedPercent = math.NaN() }},
		{"infinite", func(e *registry.StartAdmissionEvidence) { e.MaximumPercent = math.Inf(1) }},
		{"unbounded metric", func(e *registry.StartAdmissionEvidence) { e.Metric = "/private/context/token" }},
		{"wrong predicate", func(e *registry.StartAdmissionEvidence) { e.ObservedPercent = 50 }},
		{"future sample", func(e *registry.StartAdmissionEvidence) { e.ObservedAt = e.DecidedAt.Add(time.Second) }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			store, err := registry.NewFileStore(filepath.Join(dir, "sessions.json"))
			if err != nil {
				t.Fatal(err)
			}
			env := sessionsNewEnvOver(t, dir, store, store)
			row := sessionsFullRecord()
			row.Status, row.SandboxRef = registry.StatusStarting, ""
			evidence := admissionEvidenceFixture()
			tc.change(&evidence)
			_, err = env.api.allocatePreparedSession(context.Background(), preparedSessionAllocation{Record: row, Spec: backend.Spec{ID: row.ID, SessionID: row.ID}, BeforeAllocate: func(context.Context) error {
				return &executionAdmissionError{code: "cpu_pressure", cause: errors.New("private cause"), evidence: evidence}
			}})
			var failed *sessionAllocationError
			if !errors.As(err, &failed) || !failed.StartFailure.Valid() || controlledAllocationCount(env.be) != 0 {
				t.Fatal("invalid optional evidence prevented bounded refusal", err)
			}
			saved, err := store.Get(row.ID)
			if err != nil || saved.Status != registry.StatusExited || !saved.StartAdmissionEvidence.IsZero() || saved.StartFailure != registry.NewSessionStartFailure("admission", "cpu_pressure") {
				t.Fatal("invalid operands persisted or failure changed", err)
			}
		})
	}
}

func TestAdmissionEvidenceClearsWithNewInteractiveGeneration(t *testing.T) {
	env := sessionsNewEnv(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	row := sessionsStarting("s-0000000b")
	row.Status, row.SandboxRef = registry.StatusExited, ""
	row.StartFailure = registry.NewSessionStartFailure("admission", "cpu_pressure")
	row.StartAdmissionEvidence = admissionEvidenceFixture()
	sessionsSeed(t, env.st, row)
	w := env.do(t, "POST", "/v1/sessions/"+row.ID+"/restart", "")
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	sessionsAwaitWatch(t, done)
	current, err := env.st.Get(row.ID)
	if err != nil || current.Status != registry.StatusRunning || !current.StartFailure.IsZero() || !current.StartAdmissionEvidence.IsZero() {
		t.Fatal("new generation retained stale diagnostics", err)
	}
}

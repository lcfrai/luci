//go:build windows

package management

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/managedmodel"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func managedRouteFixture(t *testing.T) (*Gateway, *managedModelManager) {
	t.Helper()
	g := &Gateway{dir: t.TempDir(), state: storedState{Users: map[string]userRecord{"admin": {User: User{ID: "admin", Role: "admin"}}, "member": {User: User{ID: "member", Role: "member"}}}}, opts: Options{ManagedModelDependents: func(context.Context, string, string, bool) error { return nil }}}
	m := &managedModelManager{gateway: g, platform: inertManagedPlatform{}, path: filepath.Join(g.dir, "status.json"), boot: "boot", status: managedmodel.Status{State: "disabled", Generation: "0", ObservedAt: time.Now().UTC()}}
	g.localModels = m
	return g, m
}
func modelRequest(g *Gateway, method, path, actor, role string, body any) *httptest.ResponseRecorder {
	raw, _ := json.Marshal(body)
	r := httptest.NewRequest(method, path, bytes.NewReader(raw))
	r = r.WithContext(withActor(r.Context(), User{ID: actor, Role: role}, Policy{}))
	w := httptest.NewRecorder()
	mux := http.NewServeMux()
	g.registerManagedModel(mux)
	mux.ServeHTTP(w, r)
	return w
}
func TestManagedModelSetupSaveIsScopedRevisionedAndHasNoLaunchSideEffect(t *testing.T) {
	g, m := managedRouteFixture(t)
	body := map[string]any{"expectedRevision": 0, "enabled": true, "startWithLuci": false, "executablePath": `C:\Tools\Ollama\ollama.exe`, "modelStore": `E:\Models`, "port": 24567, "model": "installed:latest"}
	if w := modelRequest(g, "POST", "/v1/setup/local-model", "member", "member", body); w.Code != 403 {
		t.Fatal("member changed host settings", w.Code)
	}
	if w := modelRequest(g, "POST", "/v1/setup/local-model", "member", "admin", body); w.Code != 403 {
		t.Fatal("stale or private actor promoted host authority", w.Code)
	}
	w := modelRequest(g, "POST", "/v1/setup/local-model", "admin", "admin", body)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var snapshot struct {
		Available, CanManage bool
		Revision             int
		Config               managedModelConfig
		Status               managedmodel.Status
	}
	if json.Unmarshal(w.Body.Bytes(), &snapshot) != nil || !snapshot.Available || !snapshot.CanManage || snapshot.Revision != 1 || snapshot.Status.State != "stopped" || snapshot.Status.Generation != "0" || snapshot.Config.ExecutionMode != "gpu" || snapshot.Config.Profile != managedmodel.FixedProfile() {
		t.Fatal("mutation did not return the agreed full snapshot", w.Body.String())
	}
	if m.owner != nil || m.done != nil || m.saved.AuthorizingActorID != "admin" {
		t.Fatal("Save launched a process or failed to retain authorizing actor")
	}
	if w = modelRequest(g, "POST", "/v1/setup/local-model", "admin", "admin", body); w.Code != 409 || m.saved.Revision != 1 {
		t.Fatal("stale revision overwrote config")
	}
}

func TestManagedModelExecutionModeSaveIsExplicitStoppedAndRevisionBound(t *testing.T) {
	g, m := managedRouteFixture(t)
	body := map[string]any{"expectedRevision": 0, "enabled": true, "startWithLuci": false, "executablePath": `C:\Tools\Ollama\ollama.exe`, "modelStore": `E:\Models`, "port": 24567, "model": "installed:latest", "executionMode": "cpu"}
	for _, tc := range []struct {
		name string
		mode any
	}{
		{"automatic_fallback", "auto"},
		{"unknown_case", "CPU"},
		{"null", nil},
		{"empty", ""},
		{"non_string", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			body["executionMode"] = tc.mode
			if w := modelRequest(g, "POST", "/v1/setup/local-model", "admin", "admin", body); w.Code != 400 || m.saved.Revision != 0 || m.owner != nil || m.done != nil {
				t.Fatal("invalid mode changed settings or started a provider", w.Code)
			}
		})
	}
	body["executionMode"] = "cpu"
	w := modelRequest(g, "POST", "/v1/setup/local-model", "admin", "admin", body)
	profile, _ := managedmodel.ProfileForExecutionMode("cpu")
	if w.Code != 200 || m.saved.Revision != 1 || m.saved.Config.ExecutionMode != "cpu" || m.saved.Config.Profile != profile || m.saved.Config.Model != "installed:latest" || m.owner != nil || m.done != nil || m.status.State != "stopped" {
		t.Fatal("CPU save changed the alias, started work or failed to bind its profile", w.Code, w.Body.String())
	}
	body["executionMode"] = "gpu"
	if w = modelRequest(g, "POST", "/v1/setup/local-model", "admin", "admin", body); w.Code != 409 || m.saved.Config.ExecutionMode != "cpu" {
		t.Fatal("stale revision replaced the CPU profile")
	}
	body["expectedRevision"] = 1
	m.done = make(chan struct{})
	if w = modelRequest(g, "POST", "/v1/setup/local-model", "admin", "admin", body); w.Code != 409 || m.saved.Config.ExecutionMode != "cpu" {
		t.Fatal("active lifecycle permitted a mode change")
	}
	m.done = nil
	w = modelRequest(g, "POST", "/v1/setup/local-model", "member", "member", body)
	if w.Code != 403 || m.saved.Revision != 1 {
		t.Fatal("mode change widened host authority")
	}
	delete(body, "executionMode")
	w = modelRequest(g, "POST", "/v1/setup/local-model", "admin", "admin", body)
	if w.Code != 200 || m.saved.Revision != 2 || m.saved.Config.ExecutionMode != "gpu" || m.saved.Config.Profile != managedmodel.FixedProfile() {
		t.Fatal("legacy omission did not retain the original GPU contract")
	}
}

func TestManagedSavedExecutionModeDistinguishesLegacyFromInvalidPresence(t *testing.T) {
	cpu, _ := managedmodel.ProfileForExecutionMode("cpu")
	for _, tc := range []struct {
		name, expected string
		mode           any
		omitted        bool
		profile        managedmodel.Profile
		allow          bool
	}{
		{"legacy_missing", "gpu", nil, true, managedmodel.FixedProfile(), true},
		{"explicit_gpu", "gpu", "gpu", false, managedmodel.FixedProfile(), true},
		{"explicit_cpu", "cpu", "cpu", false, cpu, true},
		{"null", "", nil, false, managedmodel.FixedProfile(), false},
		{"empty", "", "", false, managedmodel.FixedProfile(), false},
		{"unknown", "", "auto", false, managedmodel.FixedProfile(), false},
		{"cpu_with_gpu_profile", "", "cpu", false, managedmodel.FixedProfile(), false},
		{"gpu_with_cpu_profile", "", "gpu", false, cpu, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			config := map[string]any{"enabled": true, "startWithLuci": true, "executablePath": `C:\Installed\ollama.exe`, "modelStore": `E:\InstalledModels`, "port": 11435, "model": "installed:latest", "profile": tc.profile}
			if !tc.omitted {
				config["executionMode"] = tc.mode
			}
			raw, err := json.Marshal(map[string]any{"revision": 7, "authorizingActorId": "host-admin", "config": config})
			if err != nil {
				t.Fatal(err)
			}
			saved, err := decodeManagedModelSaved(raw)
			if (err == nil) != tc.allow {
				t.Fatal("persisted invalid mode was defaulted or legacy mode lost", err)
			}
			if tc.allow && (saved.Revision != 7 || saved.AuthorizingActorID != "host-admin" || saved.Config.ExecutionMode != tc.expected || saved.Config.Profile != tc.profile || saved.Config.Model != "installed:latest" || !saved.Config.StartWithLuci) {
				t.Fatal("mode normalization changed persisted identity, profile or autostart intent")
			}
		})
	}
}
func TestManagedModelStatusRedactsPathsAndCurrentActorRevocationBlocksStart(t *testing.T) {
	g, m := managedRouteFixture(t)
	m.saved = managedModelSaved{Revision: 1, AuthorizingActorID: "admin", Config: managedModelConfig{Enabled: true, ExecutablePath: `C:\Tools\Ollama\ollama.exe`, ModelStore: `E:\Models`, Port: 24567, Model: "installed:latest", Profile: managedmodel.FixedProfile()}}
	w := modelRequest(g, "GET", "/v1/setup/local-model", "member", "member", nil)
	var out map[string]any
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &out) != nil {
		t.Fatal(w.Code)
	}
	config := out["config"].(map[string]any)
	if _, ok := config["executablePath"]; ok {
		t.Fatal("member received host executable path")
	}
	if _, ok := config["modelStore"]; ok {
		t.Fatal("member received model-store path")
	}
	row := g.state.Users["admin"]
	row.User.Disabled = true
	g.state.Users["admin"] = row
	if _, e := m.beginStart(1); e == nil || m.owner != nil || m.done != nil || m.status.Failure == nil || m.status.Failure.Code != "authority_changed" {
		t.Fatal("revoked persisted authority started a service")
	}
}

type delayedManagedStart struct{ entered, cancelled chan struct{} }

func (s delayedManagedStart) start(ctx context.Context, _ managedModelConfig, _ uint64) (managedModelService, error) {
	close(s.entered)
	<-ctx.Done()
	close(s.cancelled)
	return nil, ctx.Err()
}
func TestManagedCloseCancelsDelayedStartupBeforeDependentWaitWithoutNewBudget(t *testing.T) {
	g, m := managedRouteFixture(t)
	start := delayedManagedStart{make(chan struct{}), make(chan struct{})}
	m.platform = start
	m.status.Generation = "7"
	m.status.State = "starting"
	m.saved.Revision = 1
	run, cancelRun := context.WithCancel(context.Background())
	m.cancel = cancelRun
	m.done = make(chan struct{})
	done := m.done
	go m.run(run, managedModelConfig{authorize: func() error { return nil }}, 7, done)
	<-start.entered
	deadline := time.Now().Add(time.Second)
	ctx, cancel := context.WithDeadline(context.Background(), deadline)
	defer cancel()
	g.opts.ManagedModelDependents = func(c context.Context, _ string, _ string, _ bool) error {
		select {
		case <-start.cancelled:
		case <-c.Done():
			return errors.New("startup was not cancelled before dependent wait")
		}
		d, _ := c.Deadline()
		if !d.Equal(deadline) {
			t.Fatal("dependent cleanup received a new budget")
		}
		return nil
	}
	if e := m.stopExpected(ctx, true, 1, "7"); e != nil {
		t.Fatal(e)
	}
	select {
	case <-done:
	default:
		t.Fatal("startup original owner did not settle before close")
	}
}
func TestManagedControlPublicationDoesNotRenewStaleReadiness(t *testing.T) {
	_, m := managedRouteFixture(t)
	m.boot = strings.Repeat("a", 64)
	m.saved.Revision = 1
	observed := time.Now().Add(-6 * time.Second).UTC()
	m.status = managedmodel.Status{State: "available", Generation: "7", ObservedAt: observed, Endpoint: "http://127.0.0.1:24567/v1", Model: "installed:latest"}
	m.withdrawing = true
	if e := m.publishLocked(); e != nil {
		t.Fatal(e)
	}
	m.withdrawing = false
	if e := m.publishLocked(); e != nil {
		t.Fatal(e)
	}
	if e := managedmodel.Configure(m.path, m.boot); e != nil {
		t.Fatal(e)
	}
	defer managedmodel.ReleaseAuthority(m.path, m.boot)
	original, e := managedmodel.Read(m.path)
	if e != nil || !original.Status.ObservedAt.Equal(observed) {
		t.Fatal("control-only status republish invented a new observation", e)
	}
	if _, e = managedmodel.Check(&managedmodel.Reference{Revision: 1}, m.status.Endpoint, m.status.Model); e == nil {
		t.Fatal("stale readiness was admitted after failed-stop restoration")
	}
}

type managedTestHostError struct {
	diagnostic *managedmodel.HostCapacityDiagnostic
}

func (e *managedTestHostError) Error() string { return "private probe detail" }
func (e *managedTestHostError) HostCapacityDiagnostic() *managedmodel.HostCapacityDiagnostic {
	return e.diagnostic
}

type managedRestartService struct {
	sampleError       error
	stopError         error
	sampled, retiring chan struct{}
	release           <-chan struct{}
}

func (s *managedRestartService) sample(context.Context) (managedModelSample, error) {
	if s.sampled != nil {
		select {
		case s.sampled <- struct{}{}:
		default:
		}
	}
	if s.sampleError != nil {
		return managedModelSample{failure: "host_capacity_unavailable"}, s.sampleError
	}
	return managedModelSample{at: time.Now()}, nil
}
func (s *managedRestartService) stop(ctx context.Context) error {
	if s.retiring != nil {
		select {
		case s.retiring <- struct{}{}:
		default:
		}
	}
	if s.release != nil {
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return s.stopError
}

type managedRestartPlatform struct {
	services    <-chan managedModelService
	generations chan uint64
}

func (p managedRestartPlatform) start(ctx context.Context, _ managedModelConfig, gen uint64) (managedModelService, error) {
	select {
	case p.generations <- gen:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case service := <-p.services:
		return service, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func TestManagedMonitorFailureRetiresBeforeFreshExplicitRestart(t *testing.T) {
	for _, failedRetirement := range []bool{false, true} {
		name := "settled"
		if failedRetirement {
			name = "quarantined"
		}
		t.Run(name, func(t *testing.T) {
			g, m := managedRouteFixture(t)
			g.opts.ExecutionStateDir = t.TempDir()
			m.saved = managedModelSaved{Revision: 3, AuthorizingActorID: "admin", Config: managedModelConfig{Enabled: true, Port: 24567, Model: "installed:latest"}}
			diagnostic := &managedmodel.HostCapacityDiagnostic{Phase: "monitor", Reason: "cpu_unavailable"}
			release := make(chan struct{})
			first := &managedRestartService{sampleError: errors.Join(errors.New("private observation context"), &managedTestHostError{diagnostic}), retiring: make(chan struct{}, 1), release: release}
			if failedRetirement {
				first.stopError = errors.New("retirement unknown")
			}
			second := &managedRestartService{sampled: make(chan struct{}, 1)}
			services := make(chan managedModelService, 2)
			services <- first
			services <- second
			generations := make(chan uint64, 2)
			m.platform = managedRestartPlatform{services, generations}
			defer func() {
				select {
				case <-release:
				default:
					close(release)
				}
				m.mu.Lock()
				cancel, done := m.cancel, m.done
				m.mu.Unlock()
				if cancel != nil {
					cancel()
				}
				if done != nil {
					select {
					case <-done:
					case <-time.After(2 * time.Second):
						t.Error("test run did not finish")
					}
				}
			}()
			await := func(done <-chan struct{}) {
				t.Helper()
				select {
				case <-done:
				case <-time.After(2 * time.Second):
					t.Fatal("controlled lifecycle did not progress")
				}
			}
			if generation, e := m.beginStart(3); e != nil || generation != "1" {
				t.Fatal("first explicit start refused", e)
			}
			m.mu.Lock()
			done := m.done
			m.mu.Unlock()
			await(first.retiring)
			if _, e := m.beginStart(3); e == nil {
				t.Fatal("replacement started before retirement completed")
			}
			close(release)
			await(done)
			m.mu.Lock()
			owner, run, cancel := m.owner, m.done, m.cancel
			failure, state, generation := m.status.Failure, m.status.State, m.generation
			m.mu.Unlock()
			if failure == nil || failure.HostCapacity == nil || failure.HostCapacity.Reason != "cpu_unavailable" || generation != 1 || m.saved.Revision != 3 {
				t.Fatal("failure evidence or generation changed")
			}
			if failedRetirement {
				if owner != first || run != done || cancel == nil || state != "unknown" || failure.Code != "cleanup_unknown" {
					t.Fatal("unknown owner was released")
				}
				if _, e := m.beginStart(3); e == nil {
					t.Fatal("unknown retirement admitted replacement")
				}
				return
			}
			if owner != nil || run != nil || cancel != nil || state != "blocked" || failure.Code != "host_capacity_unavailable" {
				t.Fatal("retired run left stale start bookkeeping")
			}
			if _, e := m.beginStart(2); e == nil {
				t.Fatal("stale revision admitted replacement")
			}
			m.mu.Lock()
			m.stopping = true
			m.mu.Unlock()
			if _, e := m.beginStart(3); e == nil {
				t.Fatal("start bypassed concurrent stop")
			}
			m.mu.Lock()
			m.stopping = false
			m.mu.Unlock()
			if next, e := m.beginStart(3); e != nil || next != "2" {
				t.Fatal("fresh explicit restart refused", e)
			}
			await(second.sampled)
			if <-generations != 1 || <-generations != 2 || len(generations) != 0 {
				t.Fatal("unexpected allocation generation")
			}
			ctx, cancelStop := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancelStop()
			if e := m.stopExpected(ctx, false, 3, "2"); e != nil {
				t.Fatal("replacement did not retire normally", e)
			}
		})
	}
}

type managedDiagnosticStartPlatform struct {
	owner managedModelService
	err   error
}

func (p managedDiagnosticStartPlatform) start(context.Context, managedModelConfig, uint64) (managedModelService, error) {
	return p.owner, p.err
}

func TestManagedHostDiagnosticsPropagateJoinedStartupFailureWithoutAliasing(t *testing.T) {
	for _, unknownOwner := range []bool{false, true} {
		name := "settled"
		if unknownOwner {
			name = "cleanup_unknown"
		}
		t.Run(name, func(t *testing.T) {
			_, m := managedRouteFixture(t)
			physical, commit, cpu, age := uint64(9<<30), uint64(10<<30), 51.0, int64(20)
			diagnostic := &managedmodel.HostCapacityDiagnostic{Phase: "pre_resume", Reason: "cpu_busy", PhysicalAvailableBytes: &physical, CommitAvailableBytes: &commit, CPUPercent: &cpu, SampleAgeMillis: &age}
			var owner managedModelService
			if unknownOwner {
				owner = &managedRestartService{}
			}
			m.platform = managedDiagnosticStartPlatform{owner: owner, err: errors.Join(errors.New("private runtime context"), &managedModelStartError{code: "host_capacity_unavailable", cause: &managedTestHostError{diagnostic}})}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			m.generation, m.done, m.cancel = 7, make(chan struct{}), cancel
			done := m.done
			m.run(ctx, managedModelConfig{}, 7, done)
			select {
			case <-done:
			default:
				t.Fatal("startup completion not joined")
			}
			failure := m.status.Failure
			if failure == nil || failure.Code != "host_capacity_unavailable" || failure.HostCapacity == nil || failure.HostCapacity.Phase != "pre_resume" || failure.HostCapacity.Reason != "cpu_busy" {
				t.Fatal("joined host diagnostic lost")
			}
			physical, commit, cpu, age = 0, 0, 0, 0
			if *failure.HostCapacity.PhysicalAvailableBytes != 9<<30 || *failure.HostCapacity.CommitAvailableBytes != 10<<30 || *failure.HostCapacity.CPUPercent != 51 || *failure.HostCapacity.SampleAgeMillis != 20 {
				t.Fatal("published diagnostic aliases source snapshot")
			}
			if unknownOwner {
				if m.status.State != "unknown" || m.owner != owner || m.done != done || m.cancel == nil {
					t.Fatal("unknown startup owner was cleared")
				}
			} else if m.status.State != "blocked" || m.owner != nil || m.done != nil || m.cancel != nil {
				t.Fatal("settled startup left bookkeeping")
			}
			raw, e := json.Marshal(m.projectionLocked(true))
			if e != nil || strings.Contains(string(raw), "private") {
				t.Fatal("raw startup cause leaked")
			}
		})
	}
}

type managedObservationService struct {
	err     error
	stopErr error
}

func (s *managedObservationService) sample(context.Context) (managedModelSample, error) {
	return managedModelSample{failure: "startup_timeout"}, s.err
}
func (s *managedObservationService) stop(context.Context) error { return s.stopErr }

func TestManagedObservationDiagnosticsSurviveManagerFailureAndCleanup(t *testing.T) {
	for _, test := range []struct {
		name    string
		monitor bool
		unknown bool
	}{
		{"startup_settled", false, false},
		{"startup_unknown", false, true},
		{"monitor_settled", true, false},
		{"monitor_unknown", true, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, m := managedRouteFixture(t)
			status := 200
			detail := &managedmodel.ObservationDiagnostic{Stage: "show", Reason: "timeout", HTTPStatus: &status}
			cause := errors.Join(errors.New("private URL/vendor context"), managedObservationCarrier{detail})
			owner := &managedObservationService{err: cause}
			if test.unknown {
				owner.stopErr = errors.New("private cleanup context")
			}
			if test.monitor {
				m.platform = managedDiagnosticStartPlatform{owner: owner}
			} else {
				var retained managedModelService
				if test.unknown {
					retained = owner
				}
				m.platform = managedDiagnosticStartPlatform{owner: retained, err: &managedModelStartError{code: "startup_timeout", cause: cause}}
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			m.generation, m.done, m.cancel = 9, make(chan struct{}), cancel
			done := m.done
			m.run(ctx, managedModelConfig{authorize: func() error { return nil }}, 9, done)
			select {
			case <-done:
			default:
				t.Fatal("failure retirement not joined")
			}
			failure := m.status.Failure
			if failure == nil || failure.Observation == nil || failure.Observation.Stage != "show" || failure.Observation.Reason != "timeout" || failure.Observation.HTTPStatus == nil || *failure.Observation.HTTPStatus != 200 {
				t.Fatal("fixed observation diagnostic lost through manager/cleanup")
			}
			if test.monitor && test.unknown {
				if failure.Code != "cleanup_unknown" {
					t.Fatal("failed retirement was hidden")
				}
			} else if failure.Code != "startup_timeout" {
				t.Fatal("startup timeout collapsed to generic observation failure")
			}
			status, detail.Stage, detail.Reason = 599, "private-stage", "private-reason"
			if failure.Observation.Stage != "show" || failure.Observation.Reason != "timeout" || *failure.Observation.HTTPStatus != 200 {
				t.Fatal("published failure aliases private source")
			}
			if test.unknown {
				if m.status.State != "unknown" || m.owner != owner || m.done != done || m.cancel == nil {
					t.Fatal("unknown retirement lost quarantine")
				}
			} else if m.status.State != "blocked" || m.owner != nil || m.done != nil || m.cancel != nil {
				t.Fatal("positive retirement left stale bookkeeping")
			}
			raw, e := json.Marshal(m.projectionLocked(true))
			if e != nil || strings.Contains(string(raw), "private") || strings.Contains(string(raw), "vendor") {
				t.Fatal("raw failure context exposed")
			}
		})
	}
}

package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"sync"
	"testing"
	"time"
)

// Some Docker clients return success with an empty formatted response when the
// selected engine cannot answer. A process exit alone must never start onboarding
// installation or advertise a usable runtime.
func TestSetupReadinessRequiresDockerEngineAndImageEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, engine, image   string
		wantEngine, wantImage string
	}{
		{"empty-engine-and-image", "", "", "missing", "missing"},
		{"whitespace-engine-and-image", " \r\n", "\t\n", "missing", "missing"},
		{"engine-without-image", "28.3.3\n", "", "ready", "missing"},
		{"verified-engine-and-image", "28.3.3\n", "sha256:" + strings.Repeat("a", 64) + "\n", "ready", "ready"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builds := 0
			s, mux := cloudTestServer(t, func(_ context.Context, exe string, args []string, _ string) (string, error) {
				if exe == "docker" {
					switch args[0] {
					case "info":
						return tc.engine, nil
					case "image":
						return tc.image, nil
					case "build":
						builds++
						t.Fatal("readiness checking started a runtime build")
					}
				}
				return "installed-and-signed-in", nil
			})
			w := cloudRequest(mux, http.MethodPost, "/v1/setup/check", "{}")
			var status Status
			if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &status) != nil {
				t.Fatalf("readiness response: %d %s", w.Code, w.Body)
			}
			for _, want := range []struct{ id, status string }{{"docker", tc.wantEngine}, {"sandbox-image", tc.wantImage}} {
				found := false
				for _, check := range status.Checks {
					if check.ID != want.id {
						continue
					}
					found = true
					if check.Status != want.status || strings.TrimSpace(check.Message) == "" || (want.status == "missing" && check.Action == "") {
						t.Fatalf("%s readiness lost proof or recovery: %#v", want.id, check)
					}
				}
				if !found {
					t.Fatalf("missing %s check", want.id)
				}
			}
			w = cloudRequest(mux, http.MethodGet, "/v1/setup/runtime/install", "")
			var job RuntimeJob
			if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &job) != nil || job.Status != "idle" || job.ID != "" || builds != 0 {
				t.Fatalf("readiness created a runtime operation: %d %s", w.Code, w.Body)
			}
			if len(s.status().Checks) != len(status.Checks) {
				t.Fatal("readiness was not persisted for the UI")
			}
		})
	}
}

func readinessProbeID(executable string, args []string) string {
	if executable == "docker" {
		if len(args) > 0 && args[0] == "info" {
			return "docker"
		}
		return "sandbox-image"
	}
	if executable == "az" {
		return "azure"
	}
	return executable
}

func readReadinessResponse(t *testing.T, response *httptest.ResponseRecorder) Status {
	t.Helper()
	var value Status
	if response.Code != http.StatusOK || json.Unmarshal(response.Body.Bytes(), &value) != nil {
		t.Fatalf("readiness response could not be read: status %d", response.Code)
	}
	return value
}

func TestSetupReadinessRunsIndependentProbesConcurrentlyAndPublishesTogether(t *testing.T) {
	type startedProbe struct {
		id       string
		deadline bool
		budget   time.Duration
	}
	started := make(chan startedProbe, 5)
	release := make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	s, _ := cloudTestServer(t, func(ctx context.Context, executable string, args []string, _ string) (string, error) {
		deadline, bounded := ctx.Deadline()
		id := readinessProbeID(executable, args)
		started <- startedProbe{id, bounded, time.Until(deadline)}
		select {
		case <-release:
			return "verified-" + id, nil
		case <-ctx.Done():
			return "", ctx.Err()
		}
	})
	before := s.status().Checks
	response := httptest.NewRecorder()
	finished := make(chan struct{})
	go func() {
		s.handleCheck(response, httptest.NewRequest(http.MethodPost, "/v1/setup/check", nil))
		close(finished)
	}()
	seen := map[string]bool{}
	timer := time.NewTimer(2 * time.Second)
	defer timer.Stop()
	for len(seen) < 5 {
		select {
		case probe := <-started:
			if seen[probe.id] || !probe.deadline || probe.budget <= 0 || probe.budget > 8*time.Second {
				t.Fatalf("probe did not retain its independent eight-second bound: %s", probe.id)
			}
			seen[probe.id] = true
		case <-timer.C:
			t.Fatal("independent readiness probes did not all start before another probe finished")
		}
	}
	if !reflect.DeepEqual(before, s.status().Checks) {
		t.Fatal("pending readiness published partially completed checks")
	}
	select {
	case <-finished:
		t.Fatal("readiness responded before its probes settled")
	default:
	}
	unblock()
	select {
	case <-finished:
	case <-time.After(2 * time.Second):
		t.Fatal("readiness did not publish the completed probes")
	}
	value := readReadinessResponse(t, response)
	wantOrder := []string{"git", "docker", "sandbox-image", "aws", "azure"}
	if runtime.GOOS == "windows" {
		wantOrder = append(wantOrder, "docker-backing-storage")
	}
	for i, check := range value.Checks {
		if i >= len(wantOrder) || check.ID != wantOrder[i] || check.Status != "ready" {
			t.Fatalf("readiness lost deterministic ordering or completed proof at index %d", i)
		}
	}
	if len(value.Checks) != len(wantOrder) || !reflect.DeepEqual(value.Checks, s.status().Checks) {
		t.Fatal("readiness did not publish the complete ordered snapshot")
	}
}

func TestSetupReadinessPreservesRequestCancellationAndEarlierDeadline(t *testing.T) {
	for _, explicitCancel := range []bool{false, true} {
		name := "earlier-request-deadline"
		if explicitCancel {
			name = "request-cancel"
		}
		t.Run(name, func(t *testing.T) {
			var ctx context.Context
			var cancel context.CancelFunc
			if explicitCancel {
				ctx, cancel = context.WithCancel(context.Background())
			} else {
				ctx, cancel = context.WithTimeout(context.Background(), 100*time.Millisecond)
			}
			defer cancel()
			parentDeadline, hasParentDeadline := ctx.Deadline()
			started := make(chan bool, 5)
			s, _ := cloudTestServer(t, func(probe context.Context, _ string, _ []string, _ string) (string, error) {
				deadline, bounded := probe.Deadline()
				started <- bounded && (!hasParentDeadline || deadline.Equal(parentDeadline))
				<-probe.Done()
				// A runner's late zero exit does not make a canceled probe ready.
				return "late-proof", nil
			})
			s.mu.Lock()
			for i := range s.checks {
				s.checks[i].Status = "ready"
				s.checks[i].Message = "retained-current-proof"
			}
			s.mu.Unlock()
			before := s.status().Checks
			response := httptest.NewRecorder()
			finished := make(chan struct{})
			go func() {
				s.handleCheck(response, httptest.NewRequest(http.MethodPost, "/v1/setup/check", nil).WithContext(ctx))
				close(finished)
			}()
			for i := 0; i < 5; i++ {
				select {
				case preserved := <-started:
					if !preserved {
						t.Fatal("readiness extended or lost the caller deadline")
					}
				case <-time.After(2 * time.Second):
					t.Fatal("readiness cancellation probe did not start")
				}
			}
			if explicitCancel {
				cancel()
			}
			select {
			case <-finished:
			case <-time.After(2 * time.Second):
				t.Fatal("readiness did not settle after request cancellation")
			}
			if response.Body.Len() != 0 || !reflect.DeepEqual(before, s.status().Checks) {
				t.Fatal("canceled readiness published or overwrote the current snapshot")
			}
		})
	}
}

func TestSetupReadinessProbeTimeoutPublishesMissingOnLiveRequest(t *testing.T) {
	s, _ := cloudTestServer(t, func(ctx context.Context, _ string, _ []string, _ string) (string, error) {
		<-ctx.Done()
		// Even a misbehaving runner's successful late output is unverified.
		return "late-proof", nil
	})
	request := httptest.NewRequest(http.MethodPost, "/v1/setup/check", nil)
	response := httptest.NewRecorder()
	s.handleCheck(response, request)
	if request.Context().Err() != nil {
		t.Fatal("individual readiness timeout canceled the HTTP parent")
	}
	value := readReadinessResponse(t, response)
	for _, check := range value.Checks {
		if check.ID == "docker-backing-storage" {
			if check.Status != "ready" {
				t.Fatal("independent cached storage measurement was lost", check)
			}
			continue
		}
		if check.Status != "missing" || check.Action == "" {
			t.Fatalf("a timed out individual probe advertised readiness: %s", check.ID)
		}
	}
	wantChecks := 5
	if runtime.GOOS == "windows" {
		wantChecks++
	}
	if len(value.Checks) != wantChecks || !reflect.DeepEqual(value.Checks, s.status().Checks) {
		t.Fatal("live readiness request did not publish all settled timeout outcomes")
	}
}

func TestSetupReadinessRejectsEmptyAndFailedProbeOutput(t *testing.T) {
	s, _ := cloudTestServer(t, func(_ context.Context, executable string, args []string, _ string) (string, error) {
		switch readinessProbeID(executable, args) {
		case "docker":
			return "unverified-engine", errors.New("engine unavailable")
		case "sandbox-image":
			return "unverified-image", context.DeadlineExceeded
		default:
			return " \t\r\n", nil
		}
	})
	response := httptest.NewRecorder()
	s.handleCheck(response, httptest.NewRequest(http.MethodPost, "/v1/setup/check", nil))
	for _, check := range readReadinessResponse(t, response).Checks {
		if check.ID == "docker-backing-storage" {
			if check.Status != "ready" {
				t.Fatal("independent cached storage measurement was lost", check)
			}
			continue
		}
		if check.Status != "missing" || check.Action == "" || strings.TrimSpace(check.Message) == "" {
			t.Fatalf("readiness accepted empty or failed proof: %s", check.ID)
		}
	}
}

func TestSetupReadinessCloudSkipsLocalRuntimeProbes(t *testing.T) {
	calls := make(chan string, 5)
	s, _ := cloudTestServer(t, func(_ context.Context, executable string, args []string, _ string) (string, error) {
		calls <- readinessProbeID(executable, args)
		return "verified-cloud-account", nil
	})
	s.mu.Lock()
	s.workspace = Workspace{Mode: "cloud"}
	s.mu.Unlock()
	response := httptest.NewRecorder()
	s.handleCheck(response, httptest.NewRequest(http.MethodPost, "/v1/setup/check", nil))
	close(calls)
	seen := map[string]bool{}
	for id := range calls {
		if seen[id] || (id != "aws" && id != "azure") {
			t.Fatalf("cloud readiness ran a local prerequisite: %s", id)
		}
		seen[id] = true
	}
	if len(seen) != 2 {
		t.Fatal("cloud readiness did not check both host account connections")
	}
	for _, check := range s.status().Checks {
		want := "unknown"
		if check.ID == "aws" || check.ID == "azure" {
			want = "ready"
		}
		if check.Status != want {
			t.Fatalf("cloud readiness changed skipped local proof: %s", check.ID)
		}
	}
}

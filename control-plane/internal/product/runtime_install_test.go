package product

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// Older setup/storage cases still inspect their bounded synthetic command
// transcripts. This explicit test adapter provides confirmed fake settlement;
// production has no Runner/CLI fallback when admission is unavailable.
func runtimeInstallerFixture(t *testing.T, s *Server) {
	t.Helper()
	s.runtimeDir = runtimeSourceFixture(t)
	s.SetRuntimeInstaller(runtimeinstall.Runtime{
		CaptureActor: runtimeActorFixture,
		Run: func(ctx context.Context, req runtimeinstall.Request, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
			out := runtimeOutcomeFixture(req, "building", true)
			if err := publish(out); err != nil {
				return out, err
			}
			ctx = context.WithValue(ctx, runtimeBuildBudgetKey{}, runtimeBuildBudget{})
			_, err := s.runner(ctx, "docker", []string{"build", "--file", "images/sandbox/Dockerfile", "--tag", req.ImageTag, "."}, req.SourceDirectory)
			var image string
			if err == nil {
				image, err = s.runner(ctx, "docker", []string{"image", "inspect", req.ImageTag, "--format", "{{.Id}}"}, req.SourceDirectory)
			}
			if err == nil && !strings.HasPrefix(strings.TrimSpace(image), "sha256:") {
				err = errors.New("Docker returned no verified image ID after the build")
			}
			out.ResourcesHeld, out.CleanupConfirmed = false, true
			if err != nil {
				out.Phase, out.Message = "failed", runtimeBuildFailure(ctx, err)
				return out, err
			}
			out.Phase, out.ImageID, out.Message = "ready", strings.TrimSpace(image), "Confirmed synthetic runtime image."
			return out, nil
		},
	})
}
func runtimeActorFixture(context.Context) (management.ExecutionActorRef, error) {
	return management.ExecutionActorRef{Local: true}, nil
}
func runtimeOutcomeFixture(req runtimeinstall.Request, phase string, held bool) runtimeinstall.Outcome {
	id := req.Identity
	id.AllocationID = "allocation-" + req.JobID
	id.EngineID = "fixture-engine"
	id.InstanceID = "fixture-instance"
	id.SourceDigest = strings.Repeat("a", 64)
	return runtimeinstall.Outcome{Identity: id, Phase: phase, ResourcesHeld: held, CleanupConfirmed: !held, Message: "Controlled " + phase}
}
func awaitRuntimeStatus(t *testing.T, mux http.Handler, status string) RuntimeJob {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		job := readRuntimeJob(t, mux)
		if job.Status == status {
			return job
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("runtime did not reach %s: %+v", status, readRuntimeJob(t, mux))
	return RuntimeJob{}
}
func runtimeSignal(t *testing.T, ch <-chan struct{}) {
	t.Helper()
	select {
	case <-ch:
	case <-time.After(3 * time.Second):
		t.Fatal("runtime barrier timed out")
	}
}

func TestRuntimeInstallerUnavailableNeverFallsBackToDockerCLI(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	s.runtimeDir = runtimeSourceFixture(t)
	var commands atomic.Int32
	s.runner = func(context.Context, string, []string, string) (string, error) { commands.Add(1); return "", nil }
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 503)
	if commands.Load() != 0 || readRuntimeJob(t, mux).Status != "idle" {
		t.Fatal("missing admission started an unbudgeted command")
	}
}

func TestRuntimeInstallerExplicitCancellationRetainsUnknownBuildAndBlocksReplay(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	s.runtimeDir = runtimeSourceFixture(t)
	started, cancelled, settle := make(chan struct{}), make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor: runtimeActorFixture, Run: func(ctx context.Context, req runtimeinstall.Request, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		calls.Add(1)
		out := runtimeOutcomeFixture(req, "building", true)
		if err := publish(out); err != nil {
			return out, err
		}
		close(started)
		<-ctx.Done()
		close(cancelled)
		<-settle
		out.Phase = "unknown"
		return out, ctx.Err()
	}})
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	runtimeSignal(t, started)
	original := readRuntimeJob(t, mux)
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install/cancel", map[string]string{"jobId": original.ID}, 202)
	runtimeSignal(t, cancelled)
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install/cancel", map[string]string{"jobId": original.ID}, 200)
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	pending := readRuntimeJob(t, mux)
	if pending.Status != "cancelling" || !runtimeInstallRetained(pending) || pending.ID != original.ID || calls.Load() != 1 {
		t.Fatal("cancel request prematurely settled or replayed the build", pending)
	}
	close(settle)
	unknown := awaitRuntimeStatus(t, mux, "unknown")
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	if !runtimeInstallRetained(unknown) || calls.Load() != 1 {
		t.Fatal("uncertain backend cancellation released admission")
	}
}

func TestRuntimeInstallerRequestDisconnectDoesNotCancelButCloseJoinsAndPersists(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	s.runtimeDir = runtimeSourceFixture(t)
	started, finished := make(chan struct{}), make(chan struct{})
	s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor: runtimeActorFixture, Run: func(ctx context.Context, req runtimeinstall.Request, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		out := runtimeOutcomeFixture(req, "building", true)
		if err := publish(out); err != nil {
			return out, err
		}
		close(started)
		<-ctx.Done()
		out.Phase, out.ResourcesHeld, out.CleanupConfirmed = "cancelled", false, true
		close(finished)
		return out, ctx.Err()
	}})
	ctx, disconnect := context.WithCancel(context.Background())
	request := httptest.NewRequest("POST", "/v1/setup/runtime/install", strings.NewReader("{}")).WithContext(ctx)
	request.Header.Set("Content-Type", "application/json")
	response := httptest.NewRecorder()
	mux.ServeHTTP(response, request)
	if response.Code != 202 {
		t.Fatal(response.Code, response.Body.String())
	}
	runtimeSignal(t, started)
	disconnect()
	select {
	case <-finished:
		t.Fatal("observation disconnect cancelled accepted installation")
	case <-time.After(20 * time.Millisecond):
	}
	s.Close()
	runtimeSignal(t, finished)
	job := readRuntimeJob(t, mux)
	if job.Status != "cancelled" || runtimeInstallRetained(job) {
		t.Fatal("Close returned without confirmed result", job)
	}
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot(), RuntimeDir: s.runtimeDir})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	if reopened.runtimeJob.Status != "cancelled" || reopened.runtimeJob.ID != job.ID {
		t.Fatal("Close result was not durable")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 503)
	s.Close()
}

func TestRuntimeInstallerStaleCancelCannotCancelSuccessor(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	s.runtimeDir = runtimeSourceFixture(t)
	started := make(chan struct{})
	var calls atomic.Int32
	s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor: runtimeActorFixture, Run: func(ctx context.Context, req runtimeinstall.Request, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		if calls.Add(1) == 1 {
			return runtimeOutcomeFixture(req, "failed", false), nil
		}
		out := runtimeOutcomeFixture(req, "building", true)
		if err := publish(out); err != nil {
			return out, err
		}
		close(started)
		<-ctx.Done()
		out.Phase, out.ResourcesHeld, out.CleanupConfirmed = "cancelled", false, true
		return out, ctx.Err()
	}})
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	old := awaitRuntimeStatus(t, mux, "failed")
	s.mu.RLock()
	done := s.runtimeInstall.done
	s.mu.RUnlock()
	if done != nil {
		runtimeSignal(t, done)
	}
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	runtimeSignal(t, started)
	next := readRuntimeJob(t, mux)
	if old.ID == next.ID {
		t.Fatal("new confirmed-ended job reused the old identity")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install/cancel", map[string]string{"jobId": old.ID}, 409)
	if readRuntimeJob(t, mux).CancelRequested {
		t.Fatal("stale cancellation changed successor")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install/cancel", map[string]string{"jobId": next.ID}, 202)
	awaitRuntimeStatus(t, mux, "cancelled")
}

func TestRuntimeInstallerCancelledBackendCannotPublishLateReady(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	s.runtimeDir = runtimeSourceFixture(t)
	started := make(chan struct{})
	s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor: runtimeActorFixture, Run: func(ctx context.Context, req runtimeinstall.Request, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		out := runtimeOutcomeFixture(req, "building", true)
		if err := publish(out); err != nil {
			return out, err
		}
		close(started)
		<-ctx.Done()
		out.Phase, out.ResourcesHeld, out.CleanupConfirmed, out.ImageID = "ready", false, true, "sha256:"+strings.Repeat("b", 64)
		if err := publish(out); err == nil {
			t.Error("cancelled worker was allowed to publish late success")
		}
		return out, nil
	}})
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	runtimeSignal(t, started)
	job := readRuntimeJob(t, mux)
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install/cancel", map[string]string{"jobId": job.ID}, 202)
	final := awaitRuntimeStatus(t, mux, "unknown")
	if final.Execution.Phase == "ready" {
		t.Fatal("late success replaced unconfirmed cancelled operation")
	}
}

func TestRuntimeInstallerReconciliationIsExplicitAndCannotStartAnotherBuild(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	s.runtimeDir = runtimeSourceFixture(t)
	var builds, observations atomic.Int32
	runtime := runtimeinstall.Runtime{CaptureActor: runtimeActorFixture, Run: func(ctx context.Context, req runtimeinstall.Request, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		builds.Add(1)
		return runtimeOutcomeFixture(req, "unknown", true), errors.New("transport interrupted")
	}}
	runtime.Reconcile = func(ctx context.Context, id runtimeinstall.Identity, actor management.ExecutionActorRef, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		observations.Add(1)
		return runtimeinstall.Outcome{Identity: id, Phase: "failed", ResourcesHeld: false, CleanupConfirmed: true, Message: "Controlled exact settlement proof"}, nil
	}
	s.SetRuntimeInstaller(runtime)
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	job := awaitRuntimeStatus(t, mux, "unknown")
	for range 3 {
		readRuntimeJob(t, mux)
	}
	if observations.Load() != 0 {
		t.Fatal("GET ran reconciliation")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install/reconcile", map[string]string{"jobId": job.ID}, 202)
	final := awaitRuntimeStatus(t, mux, "failed")
	if runtimeInstallRetained(final) || builds.Load() != 1 || observations.Load() != 1 {
		t.Fatal("reconciliation replayed the build or retained confirmed charge")
	}
	// Saved private result remains one bounded JSON object for this exact job.
	raw, err := os.ReadFile(filepath.Join(s.stateDir, "runtime-install.json"))
	if err != nil {
		t.Fatal(err)
	}
	var saved RuntimeJob
	if err = json.Unmarshal(raw, &saved); err != nil || saved.ID != job.ID {
		t.Fatal(saved, err)
	}
}

func TestRuntimeInstallerUnchargedUnknownPublicationSurvivesRestartUntilReconciled(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	s.runtimeDir = runtimeSourceFixture(t)
	var builds atomic.Int32
	runtime := runtimeinstall.Runtime{CaptureActor: runtimeActorFixture, Run: func(ctx context.Context, req runtimeinstall.Request, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		builds.Add(1)
		out := runtimeOutcomeFixture(req, "unknown", false)
		out.ImageID = "sha256:" + strings.Repeat("c", 64)
		out.Message = "Image publication acknowledgement was lost."
		return out, errors.New("tag response lost")
	}}
	runtime.Reconcile = func(ctx context.Context, id runtimeinstall.Identity, actor management.ExecutionActorRef, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		return runtimeinstall.Outcome{Identity: id, Phase: "ready", CleanupConfirmed: true, Message: "Exact image and tag observed."}, nil
	}
	s.SetRuntimeInstaller(runtime)
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	original := awaitRuntimeStatus(t, mux, "unknown")
	if runtimeInstallRetained(original) || !runtimeInstallUnresolved(original) {
		t.Fatal("tag uncertainty was conflated with charged work", original)
	}
	s.Close()
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot(), RuntimeDir: s.runtimeDir})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	restarted.SetRuntimeResourceSnapshot(setupStorageFixtureSnapshot)
	restarted.SetRuntimeInstaller(runtime)
	next := http.NewServeMux()
	restarted.registerSetupTools(next)
	repeated := journeyRequest(t, next, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	var same RuntimeJob
	if err = json.Unmarshal(repeated.Body.Bytes(), &same); err != nil || same.ID != original.ID || builds.Load() != 1 {
		t.Fatal("uncharged unknown receipt was overwritten", same, err)
	}
	journeyRequest(t, next, "POST", "/v1/setup/runtime/install/reconcile", map[string]string{"jobId": original.ID}, 202)
	final := awaitRuntimeStatus(t, next, "ready")
	if final.ID != original.ID || builds.Load() != 1 || runtimeInstallUnresolved(final) {
		t.Fatal("explicit observation rebuilt or lost exact identity", final)
	}
}

func TestRuntimeInstallerCancellationDuringPublicationKeepsUnchargedReceipt(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	s.runtimeDir = runtimeSourceFixture(t)
	publishing := make(chan struct{})
	s.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor: runtimeActorFixture, Run: func(ctx context.Context, req runtimeinstall.Request, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		out := runtimeOutcomeFixture(req, "publishing", false)
		out.ImageID = "sha256:" + strings.Repeat("d", 64)
		if err := publish(out); err != nil {
			return out, err
		}
		close(publishing)
		<-ctx.Done()
		out.Phase = "ready"
		return out, nil
	}})
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	runtimeSignal(t, publishing)
	original := readRuntimeJob(t, mux)
	journeyRequest(t, mux, "POST", "/v1/setup/runtime/install/cancel", map[string]string{"jobId": original.ID}, 202)
	final := awaitRuntimeStatus(t, mux, "unknown")
	if runtimeInstallRetained(final) || !runtimeInstallUnresolved(final) || final.ID != original.ID {
		t.Fatal("publication cancellation lost the uncharged receipt", final)
	}
	repeated := journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
	var same RuntimeJob
	_ = json.Unmarshal(repeated.Body.Bytes(), &same)
	if same.ID != original.ID {
		t.Fatal("uncertain tag publication was replayed")
	}
}

func TestRuntimeInstallerInitialReceiptReconcilesByJobIDButLegacyReceiptCannotInferNoWork(t *testing.T) {
	for _, legacy := range []bool{false, true} {
		name := "new queued receipt"
		if legacy {
			name = "legacy raw CLI receipt"
		}
		t.Run(name, func(t *testing.T) {
			s, _ := journeyFixture(t)
			initial := RuntimeJob{ID: "runtime-interrupted", Status: "installing", Image: s.sandboxImage, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
			if !legacy {
				initial.Execution = &runtimeinstall.Outcome{Identity: runtimeinstall.Identity{JobID: initial.ID, ImageTag: initial.Image}, Phase: "queued"}
			}
			if err := atomicJSON(filepath.Join(s.stateDir, "runtime-install.json"), initial); err != nil {
				t.Fatal(err)
			}
			s.Close()
			restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
			if err != nil {
				t.Fatal(err)
			}
			defer restarted.Close()
			var observations, builds atomic.Int32
			restarted.SetRuntimeInstaller(runtimeinstall.Runtime{CaptureActor: runtimeActorFixture, Run: func(context.Context, runtimeinstall.Request, runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
				builds.Add(1)
				return runtimeinstall.Outcome{}, errors.New("unexpected build")
			}, Reconcile: func(ctx context.Context, id runtimeinstall.Identity, actor management.ExecutionActorRef, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
				observations.Add(1)
				if id.JobID != initial.ID || id.AllocationID != "" {
					t.Fatal("initial receipt lookup was not bound to its job", id)
				}
				return runtimeinstall.Outcome{Identity: id, Phase: "cancelled", CleanupConfirmed: true, Message: "Valid shared ledger confirms no admitted work for this new-protocol job."}, nil
			}})
			mux := http.NewServeMux()
			restarted.registerSetupTools(mux)
			journeyRequest(t, mux, "POST", "/v1/setup/runtime/install", map[string]any{}, 202)
			want := 202
			if legacy {
				want = 409
			}
			journeyRequest(t, mux, "POST", "/v1/setup/runtime/install/reconcile", map[string]string{"jobId": initial.ID}, want)
			if legacy {
				if observations.Load() != 0 || readRuntimeJob(t, mux).Status != "unknown" {
					t.Fatal("legacy CLI work was cleared from missing ledger evidence")
				}
			} else {
				awaitRuntimeStatus(t, mux, "cancelled")
				if observations.Load() != 1 {
					t.Fatal("new interrupted receipt was not reconciled")
				}
			}
			if builds.Load() != 0 {
				t.Fatal("unknown receipt automatically started a replacement build")
			}
		})
	}
}

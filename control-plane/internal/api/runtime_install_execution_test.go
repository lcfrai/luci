package api

import (
	"context"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

type retirementInstallerFixture struct {
	*installerFixture
	mu      sync.Mutex
	state   backend.RuntimeImageEngineState
	retired backend.RuntimeImageRetiredOutput
	reads   int
}

func (f *retirementInstallerFixture) ObserveRuntimeImageEngine(context.Context) (backend.RuntimeImageEngineState, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.reads++
	return f.state, nil
}

func (f *retirementInstallerFixture) ObserveRetiredRuntimeImage(context.Context, backend.RuntimeImageIdentity, string) (backend.RuntimeImageRetiredOutput, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.retired, nil
}

func (f *retirementInstallerFixture) setState(state string, engine string) {
	f.mu.Lock()
	f.state = backend.RuntimeImageEngineState{State: state, EngineID: engine}
	f.mu.Unlock()
}

func TestRuntimeInstallerUnknownBuildRequiresObservedEngineRetirementAcrossRestart(t *testing.T) {
	a, f, req := installerRuntimeFixture(t)
	f.build = func(context.Context, backend.PreparedRuntimeImage) (backend.RuntimeImageBuildResult, error) {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	witness := &retirementInstallerFixture{installerFixture: f}
	a.deps.Backend = witness
	out, err := a.installRuntimeImage(context.Background(), req, installerPublish)
	if err == nil || out.Phase != "unknown" || !out.ResourcesHeld || f.builds != 1 {
		t.Fatal("original lost stream was not retained", out, err)
	}
	witness.setState("running", httpCheckEngine)
	watch, err := a.reconcileRuntimeImage(context.Background(), out.Identity, req.Actor, installerPublish)
	if err != nil || watch.ReasonCode != "runtime_engine_retirement_watch" || !watch.ResourcesHeld {
		t.Fatal("running engine incorrectly settled build", watch, err)
	}
	witness.setState("stopped", "")
	// Let the original process's bounded watcher finish its durable stop write
	// before reopening the ledger through a new admission object.
	watchKey := a.execution.path + "\x00" + out.AllocationID
	deadline := time.Now().Add(2 * time.Second)
	for {
		current, readErr := a.execution.runtimeImageLease(context.Background(), out.AllocationID)
		_, watching := runtimeEngineWatch.Load(watchKey)
		if readErr == nil && current.EngineRecoveryStage == "stopped" && !watching {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("bounded watcher did not retain the exact stopped engine", current, readErr, watching)
		}
		time.Sleep(time.Millisecond)
	}
	a.execution = newExecutionAdmission(a)
	stopped, err := a.reconcileRuntimeImage(context.Background(), out.Identity, req.Actor, installerPublish)
	if err != nil || stopped.ReasonCode != "runtime_engine_stopped" || !stopped.ResourcesHeld {
		t.Fatal("engine stop was not durably retained with its charge", stopped, err)
	}
	witness.setState("running", "foreign-engine")
	foreign, err := a.reconcileRuntimeImage(context.Background(), out.Identity, req.Actor, installerPublish)
	if err != nil || !foreign.ResourcesHeld || foreign.Phase != "unknown" || f.builds != 1 {
		t.Fatal("a different restored engine released the original charge", foreign, err)
	}
	if _, err := a.execution.runtimeImageLease(context.Background(), out.AllocationID); err != nil {
		t.Fatal("foreign engine removed original lease", err)
	}
	a.execution = newExecutionAdmission(a)
	witness.setState("running", httpCheckEngine)
	witness.mu.Lock()
	witness.retired = backend.RuntimeImageRetiredOutput{Publication: "tag-absent"}
	witness.mu.Unlock()
	a.deps.AuthorizeRuntimeInstaller = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	if _, err := a.reconcileRuntimeImage(context.Background(), out.Identity, req.Actor, installerPublish); err == nil {
		t.Fatal("revoked actor settled the original charge")
	}
	if _, err := a.execution.runtimeImageLease(context.Background(), out.AllocationID); err != nil {
		t.Fatal("revoked actor removed original lease", err)
	}
	a.deps.AuthorizeRuntimeInstaller = nil
	settled, err := a.reconcileRuntimeImage(context.Background(), out.Identity, req.Actor, installerPublish)
	if err != nil || settled.Phase != "cancelled" || settled.ReasonCode != "runtime_engine_retired" || settled.ResourcesHeld || !settled.CleanupConfirmed || f.builds != 1 || f.publishes != 0 {
		t.Fatal("exact engine retirement did not settle only the original build", settled, err, f)
	}
	if _, err := a.execution.runtimeImageLease(context.Background(), out.AllocationID); !errors.Is(err, errRuntimeImageLeaseAbsent) {
		t.Fatal("retired engine kept the exact capacity lease", err)
	}
}

func TestRuntimeInstallerStoppedEngineCanBeWitnessedAfterControllerRestart(t *testing.T) {
	a, f, req := installerRuntimeFixture(t)
	f.build = func(context.Context, backend.PreparedRuntimeImage) (backend.RuntimeImageBuildResult, error) {
		return backend.RuntimeImageBuildResult{}, backend.ErrUnavailable
	}
	witness := &retirementInstallerFixture{installerFixture: f}
	a.deps.Backend = witness
	out, err := a.installRuntimeImage(context.Background(), req, installerPublish)
	if err == nil || out.Phase != "unknown" || !out.ResourcesHeld {
		t.Fatal("lost build outcome was not retained", out, err)
	}
	lease, err := a.execution.runtimeImageLease(context.Background(), out.AllocationID)
	if err != nil {
		t.Fatal(err)
	}
	// This is the durable receipt left by the first process when it armed the
	// observer. Its goroutine is intentionally absent in the reopened process.
	if _, err := a.execution.advanceRuntimeImageEngineRetirement(context.Background(), lease, "watching"); err != nil {
		t.Fatal(err)
	}
	a.execution = newExecutionAdmission(a)
	witness.setState("stopped", "")
	stopped, err := a.reconcileRuntimeImage(context.Background(), out.Identity, req.Actor, installerPublish)
	if err != nil || stopped.ReasonCode != "runtime_engine_stopped" || !stopped.ResourcesHeld {
		t.Fatal("reopened controller lost the exact engine-stop receipt", stopped, err)
	}
	a.execution = newExecutionAdmission(a)
	witness.setState("running", httpCheckEngine)
	witness.mu.Lock()
	witness.retired = backend.RuntimeImageRetiredOutput{Publication: "tag-absent"}
	witness.mu.Unlock()
	settled, err := a.reconcileRuntimeImage(context.Background(), out.Identity, req.Actor, installerPublish)
	if err != nil || settled.Phase != "cancelled" || settled.ResourcesHeld || !settled.CleanupConfirmed || f.builds != 1 || f.publishes != 0 {
		t.Fatal("reopened controller replayed or failed exact settlement", settled, err, f)
	}
}

type installerFixture struct {
	backend.Backend
	builds, publishes, observes int
	build                       func(context.Context, backend.PreparedRuntimeImage) (backend.RuntimeImageBuildResult, error)
	beforeAdmission             func()
	publishError                error
	observation                 string
}

func (f *installerFixture) PrepareRuntimeImage(ctx context.Context, s backend.RuntimeImageSpec) (backend.PreparedRuntimeImage, error) {
	if err := ctx.Err(); err != nil {
		return backend.PreparedRuntimeImage{}, err
	}
	s.Identity.EngineID, s.Identity.InstanceID, s.Identity.SourceDigest = httpCheckEngine, httpCheckInstance, strings.Repeat("a", 64)
	s.ImageTag = "docker.io/luci/sandbox:fixture"
	return backend.PreparedRuntimeImage{Spec: s, RequiredDiskBytes: backend.RuntimeImageRequiredDiskBytes}, nil
}
func (f *installerFixture) BuildRuntimeImage(ctx context.Context, p backend.PreparedRuntimeImage) (backend.RuntimeImageBuildResult, error) {
	f.builds++
	if f.build != nil {
		return f.build(ctx, p)
	}
	return backend.RuntimeImageBuildResult{Settled: true, ImageID: "sha256:" + strings.Repeat("f", 64)}, nil
}
func (f *installerFixture) BuildRuntimeImageWithAdmission(ctx context.Context, p backend.PreparedRuntimeImage, afterPull func(context.Context) error) (backend.RuntimeImageBuildResult, error) {
	if f.beforeAdmission != nil {
		f.beforeAdmission()
	}
	if err := afterPull(ctx); err != nil {
		return backend.RuntimeImageBuildResult{Settled: true}, err
	}
	return f.BuildRuntimeImage(ctx, p)
}
func TestRuntimeInstallerFreshAdmissionAfterBasePullRefusesBeforeBuild(t *testing.T) {
	a, f, req := installerRuntimeFixture(t)
	f.beforeAdmission = func() {
		a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
			s := executionFixtureSnapshot()
			s.Execution.EngineID = httpCheckEngine
			s.Execution.ImageVirtualSizeBytes = nil
			s.Execution.DaemonDisk.AvailableBytes = executionPointer(uint64(64 << 30))
			s.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(1 << 30))
			return s
		}
	}
	out, err := a.installRuntimeImage(context.Background(), req, installerPublish)
	if err == nil || out.Phase != "blocked" || out.ReasonCode != "memory_pressure" || out.ResourcesHeld || !out.CleanupConfirmed ||
		out.Admission == nil || out.Admission.Stage != "after-pull" || f.builds != 0 || f.publishes != 0 {
		t.Fatal("after-pull host pressure issued or retained an unstarted build", out, err, f)
	}
	previewRetained(t, a, 0)
}
func (f *installerFixture) PublishRuntimeImage(ctx context.Context, p backend.PreparedRuntimeImage, id string) (backend.RuntimeImagePublishResult, error) {
	f.publishes++
	if err := ctx.Err(); err != nil {
		return backend.RuntimeImagePublishResult{}, err
	}
	return backend.RuntimeImagePublishResult{ImageID: id, ImageTag: p.Spec.ImageTag, Acknowledged: f.publishError == nil}, f.publishError
}
func (f *installerFixture) ObserveRuntimeImage(_ context.Context, _ backend.RuntimeImageIdentity, id, tag string) (backend.RuntimeImageObservation, error) {
	f.observes++
	return backend.RuntimeImageObservation{ImageID: id, ImageTag: tag, Publication: f.observation}, nil
}
func installerRuntimeFixture(t *testing.T) (*API, *installerFixture, runtimeinstall.Request) {
	t.Helper()
	a, _, _ := previewRuntimeFixture(t)
	a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Execution.EngineID = httpCheckEngine
		s.Execution.ImageVirtualSizeBytes = nil
		s.Execution.DaemonDisk.AvailableBytes = executionPointer(uint64(64 << 30))
		return s
	}
	f := &installerFixture{Backend: a.deps.Backend, observation: "published"}
	a.deps.Backend = f
	return a, f, runtimeinstall.Request{Identity: runtimeinstall.Identity{JobID: "runtime-one", ImageTag: "luci/sandbox:fixture"}, Actor: management.ExecutionActorRef{Local: true}, SourceDirectory: t.TempDir()}
}
func installerPublish(runtimeinstall.Outcome) error { return nil }
func TestRuntimeInstallerSuccessfulPublicationFollowsDurablePhases(t *testing.T) {
	a, f, req := installerRuntimeFixture(t)
	var phases []string
	out, err := a.installRuntimeImage(context.Background(), req, func(v runtimeinstall.Outcome) error {
		phases = append(phases, v.Phase)
		if v.Phase == "building" && f.builds != 0 || v.Phase == "publishing" && (f.builds != 1 || f.publishes != 0) || v.Phase == "ready" && f.publishes != 1 {
			t.Fatal("publication did not precede side effect", v, f)
		}
		if v.ImageTag != req.ImageTag {
			t.Fatal("changed caller-visible tag", v)
		}
		return nil
	})
	if err != nil || out.Phase != "ready" || out.ResourcesHeld || !out.CleanupConfirmed || strings.Join(phases, ",") != "reserved,building,publishing,ready" || f.builds != 1 || f.publishes != 1 {
		t.Fatal(out, err, phases, f)
	}
	previewRetained(t, a, 0)
	if _, err = a.execution.runtimeImageLease(context.Background(), out.AllocationID); !errors.Is(err, errRuntimeImageLeaseAbsent) {
		t.Fatal("final acknowledged lineage retained", err)
	}
}
func TestRuntimeInstallerCancelledOrRevokedBeforeBuildHasNoSideEffects(t *testing.T) {
	for _, stage := range []string{"already-cancelled", "reserved-cancel", "building-cancel", "building-revoke", "reserved-write-error", "building-write-error"} {
		t.Run(stage, func(t *testing.T) {
			a, f, req := installerRuntimeFixture(t)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			revoked := false
			a.deps.AuthorizeRuntimeInstaller = func(ctx context.Context, _ management.ExecutionActorRef) (management.ExecutionAuthority, error) {
				if ctx.Err() != nil {
					return management.ExecutionAuthority{}, ctx.Err()
				}
				if revoked {
					return management.ExecutionAuthority{}, management.ErrExecutionAuthority
				}
				return management.ExecutionAuthority{}, nil
			}
			if stage == "already-cancelled" {
				cancel()
			}
			out, err := a.installRuntimeImage(ctx, req, func(v runtimeinstall.Outcome) error {
				if stage == v.Phase+"-cancel" {
					cancel()
				}
				if stage == v.Phase+"-revoke" {
					revoked = true
				}
				if stage == v.Phase+"-write-error" {
					return errors.New("private product store unavailable")
				}
				return nil
			})
			if err == nil || f.builds != 0 || f.publishes != 0 || out.ResourcesHeld {
				t.Fatal(out, err, f)
			}
			if stage == "already-cancelled" {
				if _, e := os.Stat(a.execution.path); !errors.Is(e, os.ErrNotExist) {
					t.Fatal("pre-cancelled request touched ledger", e)
				}
			} else {
				previewRetained(t, a, 0)
			}
		})
	}
}
func TestRuntimeInstallerUnknownBuildRemainsChargedAfterReopenWithoutReplay(t *testing.T) {
	a, f, req := installerRuntimeFixture(t)
	f.build = func(context.Context, backend.PreparedRuntimeImage) (backend.RuntimeImageBuildResult, error) {
		return backend.RuntimeImageBuildResult{}, errors.New("lost build transport")
	}
	out, err := a.installRuntimeImage(context.Background(), req, installerPublish)
	if err == nil || out.Phase != "unknown" || !out.ResourcesHeld || f.builds != 1 || f.publishes != 0 {
		t.Fatal(out, err, f)
	}
	a.execution = newExecutionAdmission(a)
	for range 2 {
		seen, err := a.reconcileRuntimeImage(context.Background(), out.Identity, req.Actor, installerPublish)
		if err != nil || seen.Phase != "unknown" || !seen.ResourcesHeld {
			t.Fatal(seen, err)
		}
	}
	previewRetained(t, a, 1)
	if f.builds != 1 || f.publishes != 0 || f.observes != 0 {
		t.Fatal("unknown build was replayed or falsely settled by image observation", f)
	}
	req.JobID = "runtime-peer"
	blocked, err := a.installRuntimeImage(context.Background(), req, installerPublish)
	if err != nil || blocked.Phase != "blocked" || blocked.ReasonCode != "runtime_install_retained" || f.builds != 1 {
		t.Fatal(blocked, err, f)
	}
}
func TestRuntimeInstallerSettledLostPublicationReconcilesReadOnly(t *testing.T) {
	a, f, req := installerRuntimeFixture(t)
	f.publishError = errors.New("tag acknowledgement lost")
	out, err := a.installRuntimeImage(context.Background(), req, installerPublish)
	if err == nil || out.Phase != "unknown" || out.ResourcesHeld || !out.CleanupConfirmed {
		t.Fatal(out, err)
	}
	previewRetained(t, a, 0)
	a.execution = newExecutionAdmission(a)
	bad := out.Identity
	bad.ImageTag = "unrelated:tag"
	if _, err = a.reconcileRuntimeImage(context.Background(), bad, req.Actor, installerPublish); err == nil || f.observes != 0 {
		t.Fatal("foreign tag accepted", err)
	}
	f.observation = "tag-absent"
	seen, err := a.reconcileRuntimeImage(context.Background(), out.Identity, req.Actor, installerPublish)
	if err != nil || seen.Phase != "unknown" || seen.ResourcesHeld {
		t.Fatal(seen, err)
	}
	f.observation = "published"
	seen, err = a.reconcileRuntimeImage(context.Background(), out.Identity, req.Actor, installerPublish)
	if err != nil || seen.Phase != "ready" || seen.ImageID != out.ImageID || f.builds != 1 || f.publishes != 1 || f.observes != 2 {
		t.Fatal(seen, err, f)
	}
	if _, err = a.execution.runtimeImageLease(context.Background(), out.AllocationID); !errors.Is(err, errRuntimeImageLeaseAbsent) {
		t.Fatal(err)
	}
}
func TestRuntimeInstallerCancellationAfterConfirmedBuildDoesNotTag(t *testing.T) {
	a, f, req := installerRuntimeFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	f.build = func(context.Context, backend.PreparedRuntimeImage) (backend.RuntimeImageBuildResult, error) {
		cancel()
		return backend.RuntimeImageBuildResult{Settled: true, ImageID: "sha256:" + strings.Repeat("f", 64)}, nil
	}
	out, err := a.installRuntimeImage(ctx, req, installerPublish)
	if err == nil || out.Phase != "cancelled" || out.ResourcesHeld || !out.CleanupConfirmed || f.builds != 1 || f.publishes != 0 {
		t.Fatal(out, err, f)
	}
	previewRetained(t, a, 0)
}
func TestRuntimeInstallerReadOnlyReconcileRequiresFreshAuthorityAndExactActor(t *testing.T) {
	a, f, req := installerRuntimeFixture(t)
	f.publishError = errors.New("lost acknowledgement")
	out, _ := a.installRuntimeImage(context.Background(), req, installerPublish)
	for _, actor := range []management.ExecutionActorRef{{UserID: "foreign"}, {Local: true, ProjectID: "p-other"}, {Local: true, TeamID: "team-other"}} {
		if _, err := a.reconcileRuntimeImage(context.Background(), out.Identity, actor, installerPublish); err == nil {
			t.Fatal("foreign actor admitted", actor)
		}
	}
	a.deps.AuthorizeRuntimeInstaller = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	if _, err := a.reconcileRuntimeImage(context.Background(), out.Identity, req.Actor, installerPublish); err == nil || f.observes != 0 {
		t.Fatal("revocation bypassed", err)
	}
}
func TestRuntimeInstallerLostProgressWritesRemainRecoverableByDurableJobIdentity(t *testing.T) {
	for _, firstLost := range []string{"reserved", "building"} {
		t.Run(firstLost, func(t *testing.T) {
			a, f, req := installerRuntimeFixture(t)
			saved := req.Identity
			failed := false
			out, err := a.installRuntimeImage(context.Background(), req, func(v runtimeinstall.Outcome) error {
				if v.Phase == firstLost {
					failed = true
				}
				if failed {
					return errors.New("product writes unavailable")
				}
				saved = v.Identity
				return nil
			})
			if err == nil || out.ResourcesHeld || f.builds != 0 || f.publishes != 0 {
				t.Fatal(out, err, f)
			}
			previewRetained(t, a, 0)
			a.execution = newExecutionAdmission(a)
			seen, err := a.reconcileRuntimeImage(context.Background(), saved, req.Actor, installerPublish)
			if err != nil || seen.Phase != "failed" || seen.ResourcesHeld || !seen.CleanupConfirmed || seen.AllocationID != out.AllocationID || f.observes != 0 {
				t.Fatal(seen, err)
			}
			if _, err = a.execution.runtimeImageLease(context.Background(), out.AllocationID); !errors.Is(err, errRuntimeImageLeaseAbsent) {
				t.Fatal(err)
			}
		})
	}
}
func TestRuntimeInstallerJobWithoutAllocationIsDistinctFromMissingKnownLease(t *testing.T) {
	a, f, req := installerRuntimeFixture(t)
	out, err := a.reconcileRuntimeImage(context.Background(), req.Identity, req.Actor, installerPublish)
	if err != nil || out.Phase != "cancelled" || out.ResourcesHeld || !out.CleanupConfirmed {
		t.Fatal(out, err)
	}
	req.AllocationID = "runtime-known"
	out, err = a.reconcileRuntimeImage(context.Background(), req.Identity, req.Actor, installerPublish)
	if err == nil || out.JobID != "" || f.builds != 0 || f.publishes != 0 || f.observes != 0 {
		t.Fatal("known missing lease was cleared", out, err, f)
	}
}

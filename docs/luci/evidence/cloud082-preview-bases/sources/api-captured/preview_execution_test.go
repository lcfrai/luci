package api

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

type previewBackendFixture struct {
	backend.Backend
	mu                                      sync.Mutex
	builds, creates, starts, stops, removes int
	state                                   string
	build                                   func(context.Context, backend.PreparedPreview) (backend.PreviewBuildResult, error)
	guardedBuild                            func(context.Context, backend.PreparedPreview, func(context.Context) error) (backend.PreviewBuildResult, error)
	createError, startError, removeError    error
	afterCreate                             func()
}

func (f *previewBackendFixture) PreparePreview(ctx context.Context, s backend.PreviewSpec) (backend.PreparedPreview, error) {
	if e := ctx.Err(); e != nil {
		return backend.PreparedPreview{}, e
	}
	s.Identity.EngineID = httpCheckEngine
	s.Identity.InstanceID = httpCheckInstance
	return backend.PreparedPreview{Spec: s, RequiredDiskBytes: 1 << 30, StorageDriver: "overlay2"}, nil
}
func (f *previewBackendFixture) BuildPreview(ctx context.Context, p backend.PreparedPreview, beforeMutation func(context.Context) error) (backend.PreviewBuildResult, error) {
	if f.guardedBuild != nil {
		return f.guardedBuild(ctx, p, beforeMutation)
	}
	if err := beforeMutation(ctx); err != nil {
		return backend.PreviewBuildResult{Settled: true}, err
	}
	f.mu.Lock()
	f.builds++
	f.mu.Unlock()
	if f.build != nil {
		return f.build(ctx, p)
	}
	return backend.PreviewBuildResult{Settled: true, ImageID: "sha256:" + strings.Repeat("f", 64)}, nil
}
func (f *previewBackendFixture) CreatePreview(context.Context, backend.PreparedPreview) (string, error) {
	f.mu.Lock()
	f.creates++
	f.state = "created"
	f.mu.Unlock()
	if f.afterCreate != nil {
		f.afterCreate()
	}
	return strings.Repeat("c", 64), f.createError
}
func (f *previewBackendFixture) StartPreview(context.Context, backend.PreviewIdentity, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.starts++
	f.state = "running"
	return f.startError
}
func (f *previewBackendFixture) StopPreview(context.Context, backend.PreviewIdentity, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.stops++
	f.state = "exited"
	return nil
}
func (f *previewBackendFixture) RemovePreview(context.Context, backend.PreviewIdentity, string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.removes++
	if f.removeError != nil {
		return f.removeError
	}
	f.state = "absent"
	return nil
}
func (f *previewBackendFixture) InspectPreview(_ context.Context, _ backend.PreviewIdentity, ref string) (backend.PreviewObservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return backend.PreviewObservation{Ref: ref, State: f.state, URL: "http://127.0.0.1:54321"}, nil
}
func previewRuntimeFixture(t *testing.T) (*API, *previewBackendFixture, previewruntime.Request) {
	t.Helper()
	g := previewTestGate(t)
	root := t.TempDir()
	p, e := product.New(product.Options{StateDir: filepath.Join(root, "state"), ProjectsDir: filepath.Join(root, "projects")})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(p.Close)
	g.api.deps.Product = p
	f := &previewBackendFixture{Backend: g.api.deps.Backend}
	g.api.deps.Backend = f
	req := previewruntime.Request{Identity: previewruntime.Identity{ProjectID: "p-one", ReleaseID: "release-one", Target: "local", Commit: strings.Repeat("b", 40), SourceDigest: strings.Repeat("a", 64)}, Actor: management.ExecutionActorRef{Local: true, ProjectID: "p-one"}, ValidateSource: func(context.Context) error { return nil }}
	return g.api, f, req
}
func previewRetained(t *testing.T, a *API, want int) {
	t.Helper()
	v := a.execution.reservationObservation(context.Background())
	if v.executionReservationView == nil || v.executionReservationView.Count != want {
		t.Fatalf("held reservations = %+v, want %d", v, want)
	}
}
func TestPreviewRuntimeBuildServeRestartAndExactStop(t *testing.T) {
	a, f, req := previewRuntimeFixture(t)
	published := []previewruntime.Outcome{}
	publish := func(v previewruntime.Outcome) error { published = append(published, v); return nil }
	out, e := a.startPreview(context.Background(), req, publish)
	if e != nil || out.Phase != "running" || out.URL == "" || !out.ResourcesHeld || out.BuildAllocationID == "" || out.AllocationID == "" {
		t.Fatal(out, e)
	}
	previewRetained(t, a, 1)
	observed, e := a.observePreview(context.Background(), out.Identity)
	if e != nil || observed.Identity != out.Identity || observed.Phase != "running" {
		t.Fatal(observed, e)
	}
	req.Identity = out.Identity
	out, e = a.restartPreview(context.Background(), req, publish)
	if e != nil || out.Phase != "running" || f.starts != 2 || f.stops != 1 {
		t.Fatal(out, e, f.starts, f.stops)
	}
	a.execution = newExecutionAdmission(a)
	out, e = a.stopPreview(context.Background(), out.Identity, publish)
	if e != nil || out.Phase != "finished" || out.ResourcesHeld || !out.CleanupConfirmed {
		t.Fatal(out, e)
	}
	previewRetained(t, a, 0)
	if f.builds != 1 || f.creates != 1 || f.removes != 1 {
		t.Fatal("mutation replay", f)
	}
	for _, v := range published {
		if v.Phase == "running" && (!v.ResourcesHeld || v.CleanupConfirmed) {
			t.Fatal("serving uncharged", v)
		}
	}
}
func TestPreviewRuntimeUnknownBuildAndCreateRetainChargeAcrossRestart(t *testing.T) {
	for _, mode := range []string{"build", "create", "start"} {
		t.Run(mode, func(t *testing.T) {
			a, f, req := previewRuntimeFixture(t)
			if mode == "build" {
				f.build = func(context.Context, backend.PreparedPreview) (backend.PreviewBuildResult, error) {
					return backend.PreviewBuildResult{}, errors.New("lost build transport")
				}
			}
			if mode == "create" {
				f.createError = errors.New("lost create acknowledgement")
			}
			if mode == "start" {
				f.startError = errors.New("lost start acknowledgement")
			}
			out, e := a.startPreview(context.Background(), req, func(previewruntime.Outcome) error { return nil })
			if e == nil || !out.ResourcesHeld || out.Phase != "unknown" {
				t.Fatal(out, e)
			}
			previewRetained(t, a, 1)
			a.execution = newExecutionAdmission(a)
			if mode != "start" {
				if _, e = a.stopPreview(context.Background(), out.Identity, func(previewruntime.Outcome) error { return nil }); e == nil {
					t.Fatal("unknown operation retired")
				}
				previewRetained(t, a, 1)
				if f.removes != 0 {
					t.Fatal("unknown ownership removed")
				}
			} else {
				out, e = a.stopPreview(context.Background(), out.Identity, func(previewruntime.Outcome) error { return nil })
				if e != nil || out.ResourcesHeld {
					t.Fatal("exact known container could not settle", out, e)
				}
			}
			if f.builds != 1 || f.creates > 1 || f.starts > 1 {
				t.Fatal("unknown operation replayed")
			}
		})
	}
}
func TestPreviewRuntimePublicationAndRevocationFenceNextEffects(t *testing.T) {
	for _, mode := range []string{"reserve publication", "building publication", "created publication", "revoke before Start"} {
		t.Run(mode, func(t *testing.T) {
			a, f, req := previewRuntimeFixture(t)
			revoked := false
			a.deps.AuthorizeDeployment = func(context.Context, management.ExecutionActorRef, bool) (management.ExecutionAuthority, error) {
				if revoked {
					return management.ExecutionAuthority{}, management.ErrExecutionAuthority
				}
				return management.ExecutionAuthority{}, nil
			}
			publish := func(v previewruntime.Outcome) error {
				if mode == "reserve publication" && v.Phase == "reserved" || mode == "building publication" && v.Phase == "building" || mode == "created publication" && v.Phase == "created" {
					return errors.New("receipt write failed")
				}
				if mode == "revoke before Start" && v.Phase == "created" {
					revoked = true
				}
				return nil
			}
			out, e := a.startPreview(context.Background(), req, publish)
			if e == nil {
				t.Fatal("failed fence accepted", out)
			}
			if f.starts != 0 {
				t.Fatal("Start crossed failed receipt/authority")
			}
			if (mode == "reserve publication" || mode == "building publication") && f.builds != 0 {
				t.Fatal("Build crossed failed receipt")
			}
			previewRetained(t, a, 1)
		})
	}
}
func TestPreviewRuntimeBuildSettlementBeforeBlockedServingCanBeRetired(t *testing.T) {
	a, f, req := previewRuntimeFixture(t)
	out, e := a.startPreview(context.Background(), req, func(v previewruntime.Outcome) error {
		if v.Phase == "built" {
			a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				s := executionFixtureSnapshot()
				s.Execution.EngineID = httpCheckEngine
				s.Host.Memory.EffectiveAvailableBytes = executionPointer(uint64(1) << 30)
				return s
			}
		}
		return nil
	})
	if e != nil || out.Phase != "blocked" || out.ResourcesHeld || f.creates != 0 {
		t.Fatal(out, e, f.creates)
	}
	previewRetained(t, a, 0)
	out, e = a.stopPreview(context.Background(), out.Identity, func(previewruntime.Outcome) error { return nil })
	if e != nil || out.Phase != "finished" || !out.CleanupConfirmed {
		t.Fatal(out, e)
	}
}
func TestPreviewRuntimeReleaseLockPreventsStopDuringBuildToServeGap(t *testing.T) {
	a, f, req := previewRuntimeFixture(t)
	entered, release := make(chan previewruntime.Outcome, 1), make(chan struct{})
	done := make(chan error, 1)
	go func() {
		_, e := a.startPreview(context.Background(), req, func(v previewruntime.Outcome) error {
			if v.Phase == "built" {
				entered <- v
				<-release
			}
			return nil
		})
		done <- e
	}()
	id := (<-entered).Identity
	// A live Stop contender is refused while the build→serve gap remains held.
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if _, e := a.stopPreview(ctx, id, func(previewruntime.Outcome) error { return nil }); e == nil {
		t.Fatal("concurrent Stop entered release critical section")
	}
	close(release)
	if e := <-done; e != nil {
		t.Fatal(e)
	}
	if f.creates != 1 || f.starts != 1 || f.removes != 0 {
		t.Fatal("release gap allowed duplicate/missing work")
	}
}

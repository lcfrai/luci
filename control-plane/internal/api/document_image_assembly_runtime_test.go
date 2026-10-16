package api

import (
	"context"
	"errors"
	"io"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

type assemblyRuntimeStream struct {
	release     <-chan struct{}
	captured    chan struct{}
	closeErr    error
	closed      chan struct{}
	closeOnce   sync.Once
	ignoreClose bool
}

func (s *assemblyRuntimeStream) Close() error {
	s.closeOnce.Do(func() {
		if s.closed != nil {
			close(s.closed)
		}
	})
	return s.closeErr
}
func (s *assemblyRuntimeStream) Capture(context.Context, io.Writer) (backend.DocumentImageAssemblyCapture, error) {
	if s.release != nil {
		if s.ignoreClose {
			<-s.release
		} else {
			select {
			case <-s.release:
			case <-s.closed:
				return backend.DocumentImageAssemblyCapture{}, backend.ErrUnavailable
			}
		}
	}
	if s.captured != nil {
		close(s.captured)
	}
	return backend.DocumentImageAssemblyCapture{Digest: strings.Repeat("a", 64), Bytes: 1}, nil
}

type assemblyRuntimeSink struct{}

func (assemblyRuntimeSink) Write(p []byte) (int, error) { return len(p), nil }
func (assemblyRuntimeSink) Close() error                { return nil }

type assemblyRuntimeExecutor struct {
	stream                backend.DocumentImageAssemblyStream
	started               chan struct{}
	retired               chan struct{}
	startOnce, retireOnce sync.Once
	retireErr             error
	observation           string
	starts                int
	retiredIdentity       backend.DocumentImageAssemblyIdentity
	retiredRef            string
	retireGate            <-chan struct{}
}

func (f *assemblyRuntimeExecutor) PrepareDocumentImageAssembly(_ context.Context, s backend.DocumentImageAssemblySpec) (backend.PreparedDocumentImageAssembly, error) {
	s.Identity.EngineID = "fixture-engine"
	s.Identity.InstanceID = "fixture-instance"
	return backend.PreparedDocumentImageAssembly{Spec: s, BaseVirtualSizeBytes: 1, StorageDriver: "vfs"}, nil
}
func (*assemblyRuntimeExecutor) CreateDocumentImageAssembly(context.Context, backend.PreparedDocumentImageAssembly) (string, error) {
	return strings.Repeat("b", 64), nil
}
func (f *assemblyRuntimeExecutor) AttachDocumentImageAssembly(context.Context, backend.DocumentImageAssemblyIdentity, string) (backend.DocumentImageAssemblyStream, error) {
	return f.stream, nil
}
func (f *assemblyRuntimeExecutor) StartDocumentImageAssembly(context.Context, backend.DocumentImageAssemblyIdentity, string) error {
	f.starts++
	f.startOnce.Do(func() {
		if f.started != nil {
			close(f.started)
		}
	})
	return nil
}
func (f *assemblyRuntimeExecutor) InspectDocumentImageAssembly(_ context.Context, _ backend.DocumentImageAssemblyIdentity, ref string) (backend.DocumentImageAssemblyObservation, error) {
	state := f.observation
	if state == "" {
		state = "running"
	}
	code := 0
	return backend.DocumentImageAssemblyObservation{Ref: ref, State: state, ExitCode: &code}, nil
}
func (f *assemblyRuntimeExecutor) RetireDocumentImageAssembly(_ context.Context, identity backend.DocumentImageAssemblyIdentity, ref string) error {
	f.retireOnce.Do(func() {
		f.retiredIdentity, f.retiredRef = identity, ref
		if f.retired != nil {
			close(f.retired)
		}
	})
	if f.retireGate != nil {
		<-f.retireGate
	}
	return f.retireErr
}
func (*assemblyRuntimeExecutor) FenceDocumentImageAssembly(context.Context, backend.DocumentImageAssemblyFenceRequest) (backend.DocumentImageAssemblyFenceProof, error) {
	return backend.DocumentImageAssemblyFenceProof{}, backend.ErrLaunchFencePending
}
func (*assemblyRuntimeExecutor) VerifyDocumentImageAssemblyFence(context.Context, backend.DocumentImageAssemblyFenceRequest, backend.DocumentImageAssemblyFenceProof) error {
	return backend.ErrLaunchFencePending
}
func assemblyRuntimeFixture(t *testing.T) (documentImageAssemblyRequest, documentImageAssemblyHooks, *assemblyRuntimeExecutor, *int) {
	t.Helper()
	now := time.Now().UTC()
	actor := management.ExecutionActorRef{Local: true}
	spec := backend.DocumentImageAssemblySpec{BaseImageID: backend.DocumentImageAssemblyBaseID, InputPath: t.TempDir(), InputBytes: 1, HelperPath: filepath.Join(t.TempDir(), "helper"), HelperBytes: 1, Identity: backend.DocumentImageAssemblyIdentity{Protocol: 1, Profile: backend.DocumentImageAssemblyProfile, AllocationID: "assembly-fixture", ScopeDigest: strings.Repeat("c", 64), ActorDigest: backend.DocumentExtractionActorDigest("", "", "", true), SourceCommit: strings.Repeat("d", 40), SourceDigest: strings.Repeat("e", 64), InputDigest: strings.Repeat("f", 64), HelperDigest: strings.Repeat("a", 64), RecipeDigest: backend.DocumentImageAssemblyRecipeDigest()}}
	spec.Identity.CommandDigest = backend.DocumentImageAssemblyCommandDigest(spec)
	req := documentImageAssemblyRequest{Spec: spec, Actor: actor, CreatedAt: now, ComputeDeadline: now.Add(backend.DocumentImageAssemblyComputeBudget), WholeDeadline: now.Add(backend.DocumentImageAssemblyWholeBudget), ValidateCurrent: func(context.Context) error { return nil }}
	finished := new(int)
	h := documentImageAssemblyHooks{
		Authorize: func(context.Context, management.ExecutionActorRef, backend.DocumentImageAssemblyIdentity) error {
			return nil
		},
		Admit: func(_ context.Context, p backend.PreparedDocumentImageAssembly, r documentImageAssemblyRequest) (documentImageAssemblyLease, error) {
			return documentImageAssemblyLease{ID: p.Spec.Identity.AllocationID, Scope: p.Spec.Identity.ScopeDigest, Actor: r.Actor, Identity: p.Spec.Identity, Resources: backend.DocumentImageAssemblyResources(), BaseImageID: backend.DocumentImageAssemblyBaseID, CreateDiskBytes: 1, CaptureDiskBytes: backend.DocumentImageAssemblyOutputMaxBytes, HostLaunchBytes: 2 << 30, State: "reserved", CreatedAt: r.CreatedAt, UpdatedAt: r.CreatedAt, ComputeDeadline: r.ComputeDeadline, WholeDeadline: r.WholeDeadline}, nil
		},
		Lock:       func(context.Context, backend.DocumentImageAssemblyIdentity) (func(), error) { return func() {}, nil },
		Transition: func(context.Context, backend.DocumentImageAssemblyIdentity, string, string, string) error { return nil },
		HoldFence:  func(context.Context, backend.DocumentImageAssemblyFenceRequest) error { return nil },
		RecordFence: func(context.Context, backend.DocumentImageAssemblyFenceRequest, backend.DocumentImageAssemblyFenceProof) error {
			return nil
		},
		Finish: func(_ context.Context, _ backend.DocumentImageAssemblyIdentity, o documentImageAssemblyOutcome) error {
			if !o.CleanupConfirmed || !o.ReaderSettled || !o.WatchdogSettled || o.ResourcesHeld {
				return errors.New("unsafe finish")
			}
			*finished++
			return nil
		},
		Headroom: func(context.Context, string, uint64) error { return nil },
		CaptureSink: func(context.Context, backend.DocumentImageAssemblyIdentity) (io.WriteCloser, error) {
			return assemblyRuntimeSink{}, nil
		},
		Publish: func(context.Context, documentImageAssemblyOutcome) error { return nil },
	}
	f := &assemblyRuntimeExecutor{started: make(chan struct{}), retired: make(chan struct{})}
	f.stream = &assemblyRuntimeStream{release: f.started, closed: make(chan struct{})}
	return req, h, f, finished
}
func TestAssemblyRuntimeNoPartialHooksOrAuthorityBypass(t *testing.T) {
	req, h, f, finished := assemblyRuntimeFixture(t)
	h.Headroom = nil
	if _, err := runDocumentImageAssembly(context.Background(), f, h, req); err == nil || f.starts != 0 || *finished != 0 {
		t.Fatal("partial hooks enabled assembly")
	}
	req, h, f, finished = assemblyRuntimeFixture(t)
	calls := 0
	h.Authorize = func(context.Context, management.ExecutionActorRef, backend.DocumentImageAssemblyIdentity) error {
		calls++
		if calls == 4 {
			return errors.New("revoked before start")
		}
		return nil
	}
	out, err := runDocumentImageAssembly(context.Background(), f, h, req)
	if err == nil || f.starts != 0 || !out.CleanupConfirmed || out.ResourcesHeld || *finished != 1 {
		t.Fatal("revoked pre-Start grant dispatched or failed cleanup", out, err)
	}
}
func TestAssemblyRunningPressureStopsBeforeBlockedReaderAndPublisher(t *testing.T) {
	req, h, f, finished := assemblyRuntimeFixture(t)
	releaseReader, releasePublish := make(chan struct{}), make(chan struct{})
	f.stream = &assemblyRuntimeStream{release: releaseReader, ignoreClose: true, closed: make(chan struct{})}
	h.Headroom = func(_ context.Context, _ string, bound uint64) error {
		if bound == 4<<30 {
			return errors.New("pressure")
		}
		return nil
	}
	publishing := make(chan struct{})
	h.Publish = func(context.Context, documentImageAssemblyOutcome) error {
		close(publishing)
		<-releasePublish
		return errors.New("receipt unavailable")
	}
	results := make(chan documentImageAssemblyOutcome, 1)
	go func() { o, _ := runDocumentImageAssembly(context.Background(), f, h, req); results <- o }()
	select {
	case <-f.retired:
	case <-time.After(time.Second):
		t.Fatal("reader/publisher delayed exact retirement")
	}
	select {
	case <-publishing:
		t.Fatal("publisher reached before blocked reader settled")
	default:
	}
	close(releaseReader)
	select {
	case <-publishing:
	case <-time.After(time.Second):
		t.Fatal("missing retained receipt attempt")
	}
	if *finished != 0 {
		t.Fatal("failed publisher released resources")
	}
	close(releasePublish)
	out := <-results
	if !out.ResourcesHeld || !out.CleanupConfirmed || !out.ReaderSettled || out.Capture != (backend.DocumentImageAssemblyCapture{}) || *finished != 0 {
		t.Fatal("pressure accepted capture/released uncertain receipt", out)
	}
}
func TestAssemblyUnknownCleanupAndStreamCloseNeverReleaseCapacity(t *testing.T) {
	req, h, f, finished := assemblyRuntimeFixture(t)
	f.retireErr = errors.New("unknown exact removal")
	f.stream = &assemblyRuntimeStream{release: f.started, closed: make(chan struct{}), closeErr: errors.New("unknown connection closure")}
	h.Headroom = func(_ context.Context, _ string, bound uint64) error {
		if bound == 4<<30 {
			return errors.New("pressure")
		}
		return nil
	}
	out, err := runDocumentImageAssembly(context.Background(), f, h, req)
	if err == nil || !out.ResourcesHeld || out.CleanupConfirmed || *finished != 0 {
		t.Fatal("uncertain cleanup settled", out)
	}
}
func TestAssemblyEOFIsNotExitOrRegistration(t *testing.T) {
	req, h, f, finished := assemblyRuntimeFixture(t)
	captured := make(chan struct{})
	f.stream = &assemblyRuntimeStream{release: f.started, closed: make(chan struct{}), captured: captured}
	ctx, cancel := context.WithCancel(context.Background())
	results := make(chan documentImageAssemblyOutcome, 1)
	go func() { o, _ := runDocumentImageAssembly(ctx, f, h, req); results <- o }()
	<-f.started
	<-captured
	select {
	case o := <-results:
		t.Fatal("EOF fabricated completion", o)
	default:
	}
	cancel()
	out := <-results
	if out.State != "failed_owned_resources_retired" || out.Capture != (backend.DocumentImageAssemblyCapture{}) || *finished != 1 {
		t.Fatal("cancelled running producer became accepted output", out)
	}
	req, h, f, finished = assemblyRuntimeFixture(t)
	f.observation = "exited"
	out, err := runDocumentImageAssembly(context.Background(), f, h, req)
	if err != nil || out.State != "assembled_owned_resources_retired_not_registered" || out.ResourcesHeld || !out.CleanupConfirmed || *finished != 1 {
		t.Fatal("distinct complete/exit/retirement facts lost", out, err)
	}
}

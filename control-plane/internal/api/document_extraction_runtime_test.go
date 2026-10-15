package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// Typed inert adapter: exercises the actual private host ledger and lifecycle
// implementation without Docker, sockets, parser dependencies or native jobs.
type inertDocumentExecutor struct {
	mu                             sync.Mutex
	identity                       backend.DocumentExtractionIdentity
	ref, state                     string
	attached                       bool
	creates, starts, retires       int
	createErr, startErr, retireErr error
	started                        chan struct{}
	retireHook                     func() error
	stayRunning                    bool
	blockedReader                  <-chan struct{}
}

func (f *inertDocumentExecutor) DocumentImageID() string { return "sha256:" + strings.Repeat("a", 64) }
func (f *inertDocumentExecutor) PrepareDocument(ctx context.Context, spec backend.DocumentExtractionSpec) (backend.PreparedDocumentExtraction, error) {
	if !backend.ValidateDocumentExtractionSpec(spec) {
		return backend.PreparedDocumentExtraction{}, errors.New("invalid fixture request")
	}
	spec.Identity.EngineID, spec.Identity.InstanceID = "fixture-document-engine", "fixture-document-instance"
	return backend.PreparedDocumentExtraction{Spec: spec, ImageVirtualSizeBytes: 1 << 20, StorageDriver: "overlay2"}, ctx.Err()
}
func (f *inertDocumentExecutor) CreateDocument(_ context.Context, prepared backend.PreparedDocumentExtraction) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.creates++
	f.identity, f.ref, f.state = prepared.Spec.Identity, strings.Repeat("b", 64), "created"
	if f.createErr != nil {
		return "", f.createErr
	}
	return f.ref, nil
}
func (f *inertDocumentExecutor) StartDocument(_ context.Context, identity backend.DocumentExtractionIdentity, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.identity != identity || f.ref != ref || !f.attached {
		return errors.New("unowned or unattached Start")
	}
	f.starts++
	f.state = "exited"
	if f.stayRunning {
		f.state = "running"
	}
	close(f.started)
	return f.startErr
}
func (f *inertDocumentExecutor) InspectDocument(ctx context.Context, identity backend.DocumentExtractionIdentity, ref string) (backend.DocumentExtractionObservation, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.identity != identity || f.ref != ref {
		return backend.DocumentExtractionObservation{}, backend.ErrLaunchFenceOwnership
	}
	out := backend.DocumentExtractionObservation{Ref: ref, State: f.state}
	if f.state == "exited" {
		zero := 0
		out.ExitCode = &zero
	}
	return out, ctx.Err()
}

type inertDocumentStream struct {
	executor *inertDocumentExecutor
	closed   chan struct{}
	once     sync.Once
}

func (f *inertDocumentExecutor) AttachDocumentResult(_ context.Context, identity backend.DocumentExtractionIdentity, ref string) (backend.DocumentExtractionResultStream, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.identity != identity || f.ref != ref || f.state != "created" {
		return nil, backend.ErrLaunchFenceOwnership
	}
	f.attached = true
	return &inertDocumentStream{executor: f, closed: make(chan struct{})}, nil
}
func (s *inertDocumentStream) Capture(ctx context.Context) (backend.DocumentExtractionOutput, error) {
	if s.executor.blockedReader != nil {
		<-s.executor.blockedReader
		return backend.DocumentExtractionOutput{}, errors.New("inert blocked reader finally settled")
	}
	select {
	case <-s.executor.started:
		return backend.DocumentExtractionOutput{Data: []byte("complete inert capture"), Digest: fixtureFenceHash("complete inert capture")}, nil
	case <-s.closed:
		return backend.DocumentExtractionOutput{}, context.Canceled
	case <-ctx.Done():
		return backend.DocumentExtractionOutput{}, ctx.Err()
	}
}
func (s *inertDocumentStream) Close() { s.once.Do(func() { close(s.closed) }) }
func (f *inertDocumentExecutor) RetireDocument(ctx context.Context, identity backend.DocumentExtractionIdentity, ref string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.identity != identity || f.ref != ref {
		return backend.ErrLaunchFenceOwnership
	}
	f.retires++
	if f.retireHook != nil {
		if err := f.retireHook(); err != nil {
			return err
		}
	}
	if f.retireErr != nil {
		return f.retireErr
	}
	f.state = "absent"
	return ctx.Err()
}
func (f *inertDocumentExecutor) FenceDocumentLaunch(context.Context, backend.DocumentExtractionFenceRequest) (backend.DocumentExtractionFenceProof, error) {
	return backend.DocumentExtractionFenceProof{}, errors.New("unexpected implicit fence")
}
func (f *inertDocumentExecutor) VerifyDocumentLaunchFence(context.Context, backend.DocumentExtractionFenceRequest, backend.DocumentExtractionFenceProof) error {
	return errors.New("unexpected implicit fence")
}

func inertDocumentFixture(t *testing.T) (*API, *inertDocumentExecutor, product.DiscoveryExtractionRequest) {
	t.Helper()
	gate := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	a := gate.api
	pd, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir(), Runner: func(context.Context, string, []string, string) (string, error) {
		return "", errors.New("no native fixture command permitted")
	}})
	if err != nil {
		t.Fatal(err)
	}
	a.deps.Product = pd
	t.Cleanup(func() { pd.Close() })
	a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		snap := executionFixtureSnapshot()
		snap.Execution.EngineID = "fixture-document-engine"
		return snap
	}
	f := &inertDocumentExecutor{started: make(chan struct{})}
	a.deps.DocumentExtractor = f
	path := filepath.Join(t.TempDir(), "original")
	data := []byte("%PDF-inert")
	if err := os.WriteFile(path, data, 0600); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	req := product.DiscoveryExtractionRequest{ProjectID: "project-one", DocumentID: "doc-" + strings.Repeat("c", 64), AttemptID: "extract-one", AllocationID: "document-one", OriginalPath: path, OriginalUserID: 1001, OriginalGroupID: 1001, ExpectedDocumentRevision: 2, Actor: management.ExecutionActorRef{ProjectID: "project-one", Local: true}, Original: product.DiscoveryDocumentOriginal{Digest: fixtureFenceHash(string(data)), Bytes: len(data), MimeType: "application/pdf"}, CreatedAt: now, ComputeDeadline: now.Add(backend.DocumentComputeBudget), WholeDeadline: now.Add(backend.DocumentWholeBudget), ValidateCurrent: func(ctx context.Context) error { return ctx.Err() }}
	return a, f, req
}

func TestDocumentLifecycleDurableCapturePrecedesRetirementAndReadHasNoEffects(t *testing.T) {
	a, f, req := inertDocumentFixture(t)
	var last product.DiscoveryExtractionOutcome
	f.retireHook = func() error {
		if string(last.Result.Data) != "complete inert capture" || !last.ReaderSettled || !last.ResourcesHeld {
			return errors.New("retirement preceded durable settled capture")
		}
		return nil
	}
	out, err := a.executeDocumentExtraction(context.Background(), req, func(out product.DiscoveryExtractionOutcome) error { last = out; return nil })
	if err != nil || out.State != "complete" || out.ResourcesHeld || !out.CleanupConfirmed || !out.ReaderSettled || f.creates != 1 || f.starts != 1 || f.retires != 1 {
		t.Fatal("exact lifecycle not settled", out, err)
	}
	before, err := os.ReadFile(a.execution.path)
	if err != nil {
		t.Fatal(err)
	}
	read, err := a.readDocumentExtraction(context.Background(), req.ProjectID, req.DocumentID, req.AttemptID)
	if err != nil || read.State == "complete" || read.ExitCode != nil {
		t.Fatal("ledger read fabricated extraction evidence", read, err)
	}
	after, _ := os.ReadFile(a.execution.path)
	if string(before) != string(after) || f.creates != 1 || f.starts != 1 || f.retires != 1 {
		t.Fatal("read mutated lifecycle or dispatched work")
	}
	if replay, err := a.executeDocumentExtraction(context.Background(), req, func(product.DiscoveryExtractionOutcome) error { return nil }); err != nil || replay.State != "blocked" || f.creates != 1 {
		t.Fatal("finished allocation replayed", replay, err)
	}
}

func TestDocumentLifecycleRechecksExactContextBeforeCreateAndStart(t *testing.T) {
	for _, boundary := range []int{3, 4} {
		t.Run(map[int]string{3: "create", 4: "start"}[boundary], func(t *testing.T) {
			a, f, req := inertDocumentFixture(t)
			checks := 0
			req.ValidateCurrent = func(context.Context) error {
				checks++
				if checks == boundary {
					return errors.New("document CAS or cancellation changed")
				}
				return nil
			}
			out, err := a.executeDocumentExtraction(context.Background(), req, func(product.DiscoveryExtractionOutcome) error { return nil })
			if err != nil || out.ResourcesHeld || !out.CleanupConfirmed || !out.ReaderSettled || f.starts != 0 {
				t.Fatal("changed current context dispatched or leaked", out, err)
			}
			if boundary == 3 && f.creates != 0 || boundary == 4 && (f.creates != 1 || f.retires != 1) {
				t.Fatal("wrong effect boundary", f.creates, f.retires)
			}
		})
	}
}

func TestDocumentLifecycleUnknownCreateAndCleanupRetainClaimsWithoutReplay(t *testing.T) {
	for _, scenario := range []string{"create-ack", "start-ack", "cleanup"} {
		t.Run(scenario, func(t *testing.T) {
			a, f, req := inertDocumentFixture(t)
			switch scenario {
			case "create-ack":
				f.createErr = context.DeadlineExceeded
			case "start-ack":
				f.startErr = context.DeadlineExceeded
			case "cleanup":
				f.retireErr = backend.ErrUnavailable
			}
			out, err := a.executeDocumentExtraction(context.Background(), req, func(product.DiscoveryExtractionOutcome) error { return nil })
			if err != nil {
				t.Fatal(err)
			}
			lease, err := a.execution.documentLease(context.Background(), req.ProjectID, req.DocumentID, req.AttemptID)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "start-ack" {
				if out.ResourcesHeld || f.starts != 1 || f.retires != 1 || lease.State != "finished" || out.State == "complete" {
					t.Fatal("unconfirmed Start replayed or accepted", out, lease)
				}
			} else {
				if !out.ResourcesHeld || out.CleanupConfirmed || lease.State == "finished" {
					t.Fatal("uncertainty released claim", out, lease)
				}
				var demand executionCapacityDemand
				if err = addDocumentCapacity(a.deps.ResourceSnapshot(), []documentExtractionLease{lease}, "", "", &demand); err != nil || demand.AllMemory != 512<<20 {
					t.Fatal("unknown cleanup disappeared from shared capacity", err, demand)
				}
			}
			if scenario == "create-ack" && (f.creates != 1 || f.starts+f.retires != 0 || lease.State != "creating") {
				t.Fatal("lost Create was read back, retried or implicitly retired")
			}
		})
	}
}

func TestDocumentLifecycleRechecksCurrentExecutionAuthorityBeforeStart(t *testing.T) {
	a, f, req := inertDocumentFixture(t)
	authorizations := 0
	a.deps.AuthorizeChecks = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		authorizations++
		if authorizations == 3 {
			return management.ExecutionAuthority{}, management.ErrExecutionAuthority
		}
		return management.ExecutionAuthority{}, nil
	}
	out, err := a.executeDocumentExtraction(context.Background(), req, func(product.DiscoveryExtractionOutcome) error { return nil })
	if err != nil || f.creates != 1 || f.starts != 0 || f.retires != 1 || out.ResourcesHeld || !out.ReaderSettled {
		t.Fatal("revoked authority started or leaked parser", out, err)
	}
}

func TestDocumentLifecycleExitedCapturePublicationFailurePreservesLastContainerAndCapacity(t *testing.T) {
	a, f, req := inertDocumentFixture(t)
	out, err := a.executeDocumentExtraction(context.Background(), req, func(out product.DiscoveryExtractionOutcome) error {
		if len(out.Result.Data) > 0 {
			return errors.New("inert private publication failure")
		}
		return nil
	})
	if err == nil || f.retires != 0 || !out.ResourcesHeld || !out.ReaderSettled || out.CleanupConfirmed {
		t.Fatal("last capture holder retired before durable output", out, err)
	}
	lease, readErr := a.execution.documentLease(context.Background(), req.ProjectID, req.DocumentID, req.AttemptID)
	if readErr != nil || lease.State != "running" {
		t.Fatal("capture publication failure discarded claim", lease, readErr)
	}
}

func TestDocumentRunningEmergencyRetiresBeforeBlockedReaderSettles(t *testing.T) {
	a, f, req := inertDocumentFixture(t)
	f.stayRunning = true
	readerRelease := make(chan struct{})
	var readerOnce sync.Once
	defer readerOnce.Do(func() { close(readerRelease) })
	f.blockedReader = readerRelease
	retireRequested := make(chan struct{})
	f.retireHook = func() error { close(retireRequested); return nil }
	running := make(chan struct{})
	var runningOnce sync.Once
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	type result struct {
		out product.DiscoveryExtractionOutcome
		err error
	}
	finished := make(chan result, 1)
	go func() {
		out, err := a.executeDocumentExtraction(ctx, req, func(out product.DiscoveryExtractionOutcome) error {
			if out.Stage == "running" {
				runningOnce.Do(func() { close(running) })
			}
			return nil
		})
		finished <- result{out, err}
	}()
	select {
	case <-running:
	case <-time.After(2 * time.Second):
		t.Fatal("inert parser did not reach running")
	}
	cancel()
	select {
	case <-retireRequested:
	case <-time.After(2 * time.Second):
		t.Fatal("emergency stop waited for blocked capture reader")
	}
	select {
	case <-finished:
		t.Fatal("blocked reader falsely settled complete lifecycle")
	default:
	}
	lease, err := a.execution.documentLease(context.Background(), req.ProjectID, req.DocumentID, req.AttemptID)
	if err != nil || lease.State == "finished" {
		t.Fatal("blocked reader released shared capacity", lease, err)
	}
	if unlock, err := a.execution.lockChildLifecycle(context.Background(), req.AllocationID); err == nil {
		unlock()
		t.Fatal("blocked reader lost exact lifecycle lock")
	}
	readerOnce.Do(func() { close(readerRelease) })
	select {
	case value := <-finished:
		if value.err != nil || value.out.State == "complete" || !value.out.ReaderSettled || !value.out.CleanupConfirmed || value.out.ResourcesHeld {
			t.Fatal("cancelled parser became accepted evidence or failed settlement", value.out, value.err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("settled inert reader left lifecycle hanging")
	}
}

func TestDocumentRunningPublisherFailureCannotPreventEmergencyRetirement(t *testing.T) {
	a, f, req := inertDocumentFixture(t)
	f.stayRunning = true
	retireRequested := false
	f.retireHook = func() error { retireRequested = true; return nil }
	out, err := a.executeDocumentExtraction(context.Background(), req, func(out product.DiscoveryExtractionOutcome) error {
		if out.Stage == "running" {
			return errors.New("inert running publication failure")
		}
		if out.Stage == "cleanup" {
			if !retireRequested {
				t.Fatal("failed publisher ran before emergency stop")
			}
			return errors.New("inert cleanup publication failure")
		}
		return nil
	})
	if err == nil || !retireRequested || f.retires != 1 || !out.ResourcesHeld || out.State == "complete" {
		t.Fatal("running publication failure defeated stop or released uncertainty", out, err)
	}
	lease, readErr := a.execution.documentLease(context.Background(), req.ProjectID, req.DocumentID, req.AttemptID)
	if readErr != nil || lease.State == "finished" {
		t.Fatal("failed durable publication released capacity", lease, readErr)
	}
}

func TestDocumentHeadroomRequiresFreshWindowsPhysicalCommitAndCPUForStart(t *testing.T) {
	a, _, _ := inertDocumentFixture(t)
	for _, scenario := range []string{"ready", "physical", "commit", "cpu", "stale", "foreign-engine"} {
		t.Run(scenario, func(t *testing.T) {
			a.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
				snap := executionFixtureSnapshot()
				snap.Execution.EngineID = "fixture-document-engine"
				snap.Host.Platform = "windows"
				snap.Host.Memory.PhysicalAvailableBytes = executionPointer(uint64(12) << 30)
				snap.Host.Memory.CommitAvailableBytes = executionPointer(uint64(12) << 30)
				snap.Host.CPU.UtilizationPercent = executionPointer(float64(10))
				switch scenario {
				case "physical":
					snap.Host.Memory.PhysicalAvailableBytes = executionPointer(uint64(7) << 30)
				case "commit":
					snap.Host.Memory.CommitAvailableBytes = nil
				case "cpu":
					snap.Host.CPU.UtilizationPercent = executionPointer(float64(50))
				case "stale":
					snap.Host.ObservedAt = time.Now().Add(-6 * time.Second)
				case "foreign-engine":
					snap.Execution.EngineID = "foreign"
				}
				return snap
			}
			if err := a.documentExtractionHeadroom("fixture-document-engine", 8<<30); (err == nil) != (scenario == "ready") {
				t.Fatal("unverified start headroom admitted", scenario, err)
			}
		})
	}
}

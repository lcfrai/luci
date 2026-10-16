package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

// In-memory SDK and existing private-file/ledger fixtures only. Native runners,
// Docker engines, package/image/model/authentication execution are unavailable.
func assemblySharedFixture(t *testing.T) (*executionAdmission, documentImageAssemblyRequest, *assemblyRuntimeExecutor, documentImageAssemblyHooks) {
	t.Helper()
	g := executionTestGate(t, filepath.Join(t.TempDir(), "capacity"))
	pd, err := product.New(product.Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir(), Runner: func(context.Context, string, []string, string) (string, error) {
		return "", errors.New("fixture forbids native command")
	}})
	if err != nil {
		t.Fatal(err)
	}
	g.api.deps.Product = pd
	t.Cleanup(func() { pd.Close() })
	req, _, f, _ := assemblyRuntimeFixture(t)
	req.Spec.Identity.ScopeDigest = g.scope
	req.Spec.Identity.CommandDigest = backend.DocumentImageAssemblyCommandDigest(req.Spec)
	g.api.deps.ResourceSnapshot = func() resourcebudget.Snapshot {
		s := executionFixtureSnapshot()
		s.Execution.EngineID = "fixture-engine"
		return s
	}
	g.assemblyDiskProbe = func(path resourcebudget.DiskPath) (resourcebudget.Disk, error) {
		if path.Path != filepath.Dir(g.path) || path.Role != "document-image-assembly-capture" {
			return resourcebudget.Disk{}, errors.New("foreign capture volume")
		}
		return resourcebudget.Disk{ObservedAt: time.Now().UTC(), AvailableBytes: executionPointer(uint64(100) << 30)}, nil
	}
	h, err := g.api.documentImageAssemblySharedHooks(f, req)
	if err != nil {
		t.Fatal(err)
	}
	return g, req, f, h
}

func assemblySharedAdmit(t *testing.T, g *executionAdmission, req documentImageAssemblyRequest, f *assemblyRuntimeExecutor, h documentImageAssemblyHooks) documentImageAssemblyLease {
	t.Helper()
	p, err := f.PrepareDocumentImageAssembly(context.Background(), req.Spec)
	if err != nil {
		t.Fatal(err)
	}
	lease, err := h.Admit(context.Background(), p, req)
	if err != nil {
		t.Fatal(err)
	}
	return lease
}

func TestAssemblySharedLedgerBoundsCrossPurposeAndReadonlyTotals(t *testing.T) {
	g, req, f, h := assemblySharedFixture(t)
	lease := assemblySharedAdmit(t, g, req, f, h)
	document := documentCapacityFixture()
	if err := g.mutate(context.Background(), func(l *executionLeaseLedger) error { l.Documents = append(l.Documents, document); return nil }); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(g.path)
	if err != nil {
		t.Fatal(err)
	}
	view := g.reservationObservation(context.Background())
	if view.Status != "ready" || view.Count != 2 || view.MemoryMiB != 2560 || view.CPUMillis != 2000 {
		t.Fatal("readonly projection omitted document/assembly", view)
	}
	after, err := os.ReadFile(g.path)
	if err != nil || string(before) != string(after) {
		t.Fatal("readonly projection mutated history", err)
	}
	cached := g.snapshot.Load()
	if cached == nil || cached.Count != view.Count || cached.MemoryMiB != view.MemoryMiB {
		t.Fatal("cached/read-only projection disagreement", cached, view)
	}
	l, err := decodeExecutionLedger(before)
	if err != nil || l.ImageAssemblies[0].Identity != lease.Identity {
		t.Fatal("retained assembly identity lost", err)
	}
	l.ImageAssemblies = append(l.ImageAssemblies, lease)
	data, _ := json.Marshal(l)
	if _, err = decodeExecutionLedger(data); err == nil {
		t.Fatal("duplicate assembly accepted")
	}
	l.ImageAssemblies = l.ImageAssemblies[:1]
	l.Checks = []checkExecutionLease{{ID: lease.ID, Scope: lease.Scope, Actor: management.ExecutionActorRef{ProjectID: "project-one", Local: true}, Identity: backend.CheckIdentity{Protocol: 1, EngineID: "engine", InstanceID: httpCheckInstance, AllocationID: lease.ID, CheckID: "check-one", CommandIndex: 0, ScopeDigest: lease.Scope, CommandDigest: strings.Repeat("b", 64), ProjectID: "project-one"}, Resources: backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}, CreateDiskBytes: 1, State: "reserved", CreatedAt: lease.CreatedAt, UpdatedAt: lease.CreatedAt}}
	data, _ = json.Marshal(l)
	if _, err = decodeExecutionLedger(data); err == nil {
		t.Fatal("cross-purpose allocation collision accepted")
	}
	l.Checks = nil
	l.Leases = make([]executionLease, 4095)
	l.ImageAssemblies = append(l.ImageAssemblies, lease)
	data, _ = json.Marshal(l)
	if _, err = decodeExecutionLedger(data); err == nil {
		t.Fatal("joint lease history bound omitted assemblies")
	}
	if _, err = decodeExecutionLedger([]byte(strings.Repeat(" ", 2<<20+1))); err == nil {
		t.Fatal("oversized ledger parsed")
	}
}

func TestAssemblySharedSerialSlotBlocksEveryAdmissionFamilyAndUnknownExpiry(t *testing.T) {
	g, req, f, h := assemblySharedFixture(t)
	lease := assemblySharedAdmit(t, g, req, f, h)
	if err := g.mutate(context.Background(), func(l *executionLeaseLedger) error {
		row := &l.ImageAssemblies[0]
		row.CreatedAt = row.CreatedAt.Add(-time.Hour)
		row.UpdatedAt = row.CreatedAt
		row.ComputeDeadline = row.ComputeDeadline.Add(-time.Hour)
		row.WholeDeadline = row.WholeDeadline.Add(-time.Hour)
		row.State = "creating"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	actor := management.ExecutionActorRef{ProjectID: "project-one", Local: true}
	policy := executionFixturePolicy()
	t.Run("worker", func(t *testing.T) {
		d, err := g.reserve(context.Background(), "worker-other", actor.ProjectID, "fixture", 2, actor, management.ExecutionAuthority{}, policy)
		if !errors.Is(err, errDocumentImageAssemblySerial) || d.State == "admitted" {
			t.Fatal("worker did not reach shared serial fence", d, err)
		}
	})
	t.Run("check", func(t *testing.T) {
		p := backend.PreparedCheck{Spec: backend.CheckSpec{Identity: backend.CheckIdentity{Protocol: 1, EngineID: lease.Identity.EngineID, InstanceID: httpCheckInstance, AllocationID: "check-other", CheckID: "check-other", ProjectID: actor.ProjectID, ScopeDigest: g.scope, CommandDigest: strings.Repeat("b", 64)}, Resources: backend.Resources{MemoryMiB: 1024, CPUMillis: 1000}}, ImageVirtualSizeBytes: 1, StorageDriver: "overlay2"}
		d, err := g.reserveCheck(context.Background(), p, actor, management.ExecutionAuthority{}, policy)
		if !errors.Is(err, errDocumentImageAssemblySerial) || d.State == "admitted" {
			t.Fatal("check did not reach shared serial fence", d, err)
		}
	})
	t.Run("preview", func(t *testing.T) {
		p := previewTestPrepared(g, actor.ProjectID, "preview-other", "preview", 1024, 1000)
		p.Spec.Identity.EngineID = lease.Identity.EngineID
		d, _, err := g.reservePreview(context.Background(), p, actor, management.ExecutionAuthority{}, policy)
		if !errors.Is(err, errDocumentImageAssemblySerial) || d.State == "admitted" {
			t.Fatal("preview did not reach shared serial fence", d, err)
		}
	})
	t.Run("image", func(t *testing.T) {
		p := runtimeImagePrepared(g, "image-other")
		p.Spec.Identity.EngineID = lease.Identity.EngineID
		d, _, err := g.reserveRuntimeImage(context.Background(), p, management.ExecutionActorRef{Local: true}, management.ExecutionAuthority{}, policy)
		if !errors.Is(err, errDocumentImageAssemblySerial) || d.State == "admitted" {
			t.Fatal("installer did not reach shared serial fence", d, err)
		}
	})
	t.Run("document", func(t *testing.T) {
		now := time.Now().UTC()
		original := product.DiscoveryDocumentOriginal{Digest: strings.Repeat("c", 64), Bytes: 1, MimeType: "application/pdf"}
		r := product.DiscoveryExtractionRequest{ProjectID: actor.ProjectID, DocumentID: "doc-" + strings.Repeat("b", 64), AttemptID: "extract-other", AllocationID: "document-other", Actor: actor, Original: original, OriginalPath: filepath.Join(t.TempDir(), "original"), OriginalUserID: 1001, OriginalGroupID: 1001, ExpectedDocumentRevision: 2, CreatedAt: now, ComputeDeadline: now.Add(backend.DocumentComputeBudget), WholeDeadline: now.Add(backend.DocumentWholeBudget), ValidateCurrent: func(ctx context.Context) error { return ctx.Err() }}
		spec := backend.DocumentExtractionSpec{Identity: backend.DocumentExtractionIdentity{Protocol: 1, Profile: backend.DocumentExtractionProfile, EngineID: lease.Identity.EngineID, InstanceID: httpCheckInstance, ScopeDigest: g.scope, ActorDigest: backend.DocumentExtractionActorDigest("", "", actor.ProjectID, true), AllocationID: r.AllocationID, ProjectID: r.ProjectID, DocumentID: r.DocumentID, AttemptID: r.AttemptID, OriginalDigest: original.Digest}, ImageID: "sha256:" + strings.Repeat("a", 64), OriginalPath: r.OriginalPath, OriginalBytes: 1, MimeType: original.MimeType, UserID: 1001, GroupID: 1001}
		spec.Identity.CommandDigest = backend.DocumentExtractionCommandDigest(spec)
		d, err := g.reserveDocument(context.Background(), backend.PreparedDocumentExtraction{Spec: spec, ImageVirtualSizeBytes: 1, StorageDriver: "overlay2"}, r, management.ExecutionAuthority{}, policy)
		if !errors.Is(err, errDocumentImageAssemblySerial) || d.State == "admitted" {
			t.Fatal("parser did not reach shared serial fence", d, err)
		}
	})
	ledger, err := g.readReservationLedger(context.Background())
	if err != nil || len(ledger.ImageAssemblies) != 1 || ledger.ImageAssemblies[0].State != "creating" || executionLeaseCount(ledger) != 1 {
		t.Fatal("expiry released claim or other family allocated", ledger, err)
	}
}

func TestAssemblySharedAdmissionCurrentOperatorSourceAndImmutableCAS(t *testing.T) {
	g, req, f, h := assemblySharedFixture(t)
	lease := assemblySharedAdmit(t, g, req, f, h)
	if err := h.Transition(context.Background(), lease.Identity, "reserved", "running", strings.Repeat("b", 64)); err == nil {
		t.Fatal("phase skipped")
	}
	if err := h.Transition(context.Background(), lease.Identity, "reserved", "creating", ""); err != nil {
		t.Fatal(err)
	}
	if err := h.Transition(context.Background(), lease.Identity, "reserved", "creating", ""); err == nil {
		t.Fatal("stale transition replayed")
	}
	p, _ := f.PrepareDocumentImageAssembly(context.Background(), req.Spec)
	if _, err := h.Admit(context.Background(), p, req); err == nil {
		t.Fatal("allocated invocation adopted as reservation")
	}
	g.api.deps.AuthorizeRuntimeInstaller = func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	if err := h.Authorize(context.Background(), req.Actor, lease.Identity); err == nil {
		t.Fatal("revoked current installer authority retained")
	}
	g2, req2, f2, _ := assemblySharedFixture(t)
	req2.ValidateCurrent = func(context.Context) error { return errors.New("source changed") }
	h2, err := g2.api.documentImageAssemblySharedHooks(f2, req2)
	if err != nil {
		t.Fatal(err)
	}
	p2, _ := f2.PrepareDocumentImageAssembly(context.Background(), req2.Spec)
	if _, err = h2.Admit(context.Background(), p2, req2); err == nil {
		t.Fatal("changed source was admitted")
	}
}

type assemblySharedStream struct{ release <-chan struct{} }

func (s assemblySharedStream) Close() error { return nil }
func (s assemblySharedStream) Capture(ctx context.Context, w io.Writer) (backend.DocumentImageAssemblyCapture, error) {
	select {
	case <-s.release:
	case <-ctx.Done():
		return backend.DocumentImageAssemblyCapture{}, ctx.Err()
	}
	data := []byte("inert-capture-receipt-not-an-OCI-image")
	if _, err := w.Write(data); err != nil {
		return backend.DocumentImageAssemblyCapture{}, err
	}
	digest := sha256.Sum256(data)
	return backend.DocumentImageAssemblyCapture{Digest: hex.EncodeToString(digest[:]), Bytes: int64(len(data))}, nil
}

func TestAssemblySharedFullPrivateHooksRequireDurableCaptureAndFreshAbsence(t *testing.T) {
	for _, absent := range []bool{false, true} {
		t.Run(map[bool]string{false: "absence_unknown", true: "positive_retirement"}[absent], func(t *testing.T) {
			g, req, f, _ := assemblySharedFixture(t)
			f.observation = "exited"
			f.absentAfterRetire = absent
			f.stream = assemblySharedStream{release: f.started}
			out, err := g.api.executeDocumentImageAssembly(context.Background(), f, req)
			ledger, readErr := g.readReservationLedger(context.Background())
			if readErr != nil || len(ledger.ImageAssemblies) != 1 {
				t.Fatal("claim lost", ledger, readErr)
			}
			if absent {
				if err != nil || out.ResourcesHeld || ledger.ImageAssemblies[0].State != "finished" || out.State != "assembled_owned_resources_retired_not_registered" {
					t.Fatal("positive private receipt chain failed", out, err)
				}
			} else if err == nil || !out.ResourcesHeld || ledger.ImageAssemblies[0].State == "finished" {
				t.Fatal("caller retirement label released claim without absence", out, err)
			}
			store, e := g.assemblyPrivateCapture(ledger.ImageAssemblies[0].Identity)
			if e != nil {
				t.Fatal(e)
			}
			receipt, e := store.read()
			if e != nil || !receipt.CaptureExitVerified || receipt.Outcome.Capture != out.Capture {
				t.Fatal("complete actual capture/exit fact lost", receipt, e)
			}
		})
	}
}

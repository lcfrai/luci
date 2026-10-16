package api

import (
	"context"
	"errors"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func TestAssemblyPrivateCaptureExclusiveBoundAndMutationRefusal(t *testing.T) {
	g, req, f, h := assemblySharedFixture(t)
	lease := assemblySharedAdmit(t, g, req, f, h)
	store, err := g.assemblyPrivateCapture(lease.Identity)
	if err != nil {
		t.Fatal(err)
	}
	writer, err := store.open()
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.open(); err == nil {
		t.Fatal("existing capture reopened")
	}
	if _, err = writer.Write([]byte("one")); err != nil {
		t.Fatal(err)
	}
	bounded := writer.(*assemblyBoundedFile)
	// Inert counter boundary: never allocate or write a2GiB test payload.
	bounded.written = backend.DocumentImageAssemblyOutputMaxBytes
	if n, e := writer.Write([]byte("x")); n != 0 || e == nil {
		t.Fatal("capture limit applied after write", n, e)
	}
	if err = writer.Close(); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(store.path)
	if err != nil || info.Size() != 3 {
		t.Fatal("oversize write changed artifact", info, err)
	}
	capture := backend.DocumentImageAssemblyCapture{Bytes: 3, Digest: fixtureFenceHash("one")}
	if err = store.verify(context.Background(), capture); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(store.path, []byte("two"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = store.verify(context.Background(), capture); err == nil {
		t.Fatal("same-size mutated capture accepted")
	}
}

func TestAssemblyCaptureVolumeUsesAggregateRetainedClaimsAndFreshExactPath(t *testing.T) {
	g, req, f, h := assemblySharedFixture(t)
	lease := assemblySharedAdmit(t, g, req, f, h)
	foreign := lease
	foreign.Scope = strings.Repeat("d", 64)
	foreign.Identity.ScopeDigest = foreign.Scope
	foreign.ID = "foreign-assembly"
	foreign.Identity.AllocationID = foreign.ID
	foreign.CreatedAt = foreign.CreatedAt.Add(-time.Hour)
	foreign.UpdatedAt = foreign.CreatedAt
	foreign.ComputeDeadline = foreign.ComputeDeadline.Add(-time.Hour)
	foreign.WholeDeadline = foreign.WholeDeadline.Add(-time.Hour)
	rows := []documentImageAssemblyLease{lease, foreign}
	required, err := assemblyCaptureDemand(rows, 2048)
	if err != nil || required != 6<<30 {
		t.Fatal("foreign/expired captures omitted or duplicated", required, err)
	}
	now := time.Now().UTC()
	for _, disk := range []resourcebudget.Disk{{ObservedAt: now, AvailableBytes: executionPointer(uint64(5) << 30)}, {ObservedAt: now.Add(-6 * time.Second), AvailableBytes: executionPointer(uint64(100) << 30)}, {ObservedAt: now.Add(time.Second), AvailableBytes: executionPointer(uint64(100) << 30)}, {ObservedAt: now}} {
		if validateAssemblyCaptureDisk(disk, required, now) == nil {
			t.Fatal("unavailable/low/stale capture volume admitted", disk)
		}
	}
	g.assemblyDiskProbe = func(resourcebudget.DiskPath) (resourcebudget.Disk, error) {
		return resourcebudget.Disk{}, errors.New("exact volume unavailable")
	}
	var demand executionCapacityDemand
	if err = g.addAssemblyAdmissionCapacity(context.Background(), g.api.deps.ResourceSnapshot(), []documentImageAssemblyLease{lease}, g.scope, lease.ID, &demand, 2048); err == nil {
		t.Fatal("snapshot state role substituted for exact capture volume")
	}
}

func TestAssemblyPrivatePublicationFailureCannotReleaseUnallocatedClaim(t *testing.T) {
	g, req, f, h := assemblySharedFixture(t)
	lease := assemblySharedAdmit(t, g, req, f, h)
	store, err := g.assemblyPrivateCapture(lease.Identity)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Mkdir(store.receipt, 0700); err != nil {
		t.Fatal(err)
	}
	out := documentImageAssemblyOutcome{Identity: lease.Identity, InputClosureRef: lease.InputClosureRef, State: "failed_owned_resources_retired", Stage: "reserved", ReaderSettled: true, WatchdogSettled: true, CleanupConfirmed: true, ResourcesHeld: true}
	if err = h.Publish(context.Background(), out); err == nil {
		t.Fatal("failed private publication acknowledged")
	}
	out.ResourcesHeld = false
	if err = h.Finish(context.Background(), lease.Identity, out); err == nil {
		t.Fatal("missing durable receipt released claim")
	}
	l, err := g.readReservationLedger(context.Background())
	if err != nil || l.ImageAssemblies[0].State != "reserved" {
		t.Fatal("failed publication lost claim", l, err)
	}
}

type assemblySharedFenceExecutor struct {
	*assemblyRuntimeExecutor
	verifyErr error
}

func (f *assemblySharedFenceExecutor) FenceDocumentImageAssembly(_ context.Context, req backend.DocumentImageAssemblyFenceRequest) (backend.DocumentImageAssemblyFenceProof, error) {
	now := time.Now().UTC().Format(time.RFC3339Nano)
	return backend.DocumentImageAssemblyFenceProof{Identity: req.Identity, IntentID: req.IntentID, FenceRef: strings.Repeat("c", 64), ConfigDigest: strings.Repeat("d", 64), AcknowledgedAt: now, ObservedAt: now}, nil
}

func (f *assemblySharedFenceExecutor) VerifyDocumentImageAssemblyFence(_ context.Context, req backend.DocumentImageAssemblyFenceRequest, proof backend.DocumentImageAssemblyFenceProof) error {
	if f.verifyErr != nil {
		return f.verifyErr
	}
	if proof.Identity != req.Identity || proof.IntentID != req.IntentID || proof.AcknowledgedAt == "" {
		return backend.ErrLaunchFenceOwnership
	}
	return nil
}

func TestAssemblySharedFenceRequiresActualACKAndExactDurableIntent(t *testing.T) {
	g, req, f, h := assemblySharedFixture(t)
	lease := assemblySharedAdmit(t, g, req, f, h)
	verified := &assemblySharedFenceExecutor{assemblyRuntimeExecutor: f}
	h, err := g.api.documentImageAssemblySharedHooks(verified, req)
	if err != nil {
		t.Fatal(err)
	}
	if err = h.Transition(context.Background(), lease.Identity, "reserved", "creating", ""); err != nil {
		t.Fatal(err)
	}
	fence := backend.DocumentImageAssemblyFenceRequest{Identity: lease.Identity, IntentID: strings.Repeat("a", 32)}
	if err = h.HoldFence(context.Background(), fence); err != nil {
		t.Fatal(err)
	}
	if err = h.RecordFence(context.Background(), fence, backend.DocumentImageAssemblyFenceProof{}); err == nil {
		t.Fatal("readback/lost ACK substituted for actual fence ACK")
	}
	other := fence
	other.IntentID = strings.Repeat("b", 32)
	if err = h.HoldFence(context.Background(), other); err == nil {
		t.Fatal("second fence intent replaced durable lineage")
	}
	now := time.Now().UTC().Format(time.RFC3339Nano)
	proof := backend.DocumentImageAssemblyFenceProof{Identity: lease.Identity, IntentID: fence.IntentID, FenceRef: strings.Repeat("c", 64), ConfigDigest: strings.Repeat("d", 64), AcknowledgedAt: now, ObservedAt: now}
	if err = h.RecordFence(context.Background(), fence, proof); err != nil {
		t.Fatal(err)
	}
	out := documentImageAssemblyOutcome{Identity: lease.Identity, InputClosureRef: lease.InputClosureRef, Stage: "recovery", State: "interrupted_capture_unavailable_owned_resources_retired", ResourcesHeld: true, CleanupConfirmed: true, ReaderSettled: true, WatchdogSettled: true}
	if err = h.Publish(context.Background(), out); err != nil {
		t.Fatal(err)
	}
	out.ResourcesHeld = false
	verified.verifyErr = backend.ErrLaunchFencePending
	if err = h.Finish(context.Background(), lease.Identity, out); err == nil {
		t.Fatal("historical ACK released claim without current exact fence proof")
	}
	verified.verifyErr = nil
	if err = h.Finish(context.Background(), lease.Identity, out); err != nil {
		t.Fatal(err)
	}
	l, err := g.readReservationLedger(context.Background())
	if err != nil || l.ImageAssemblies[0].State != "finished" || l.ImageAssemblyFences[0].State != "retired" || f.starts != 0 {
		t.Fatal("fenced recovery started work or lost joint retirement", l, err)
	}
}

func TestAssemblyPrivateHookRecoveryPreservesPublishedCaptureAsUnavailableEvidence(t *testing.T) {
	for _, scenario := range []string{"capture_present", "capture_missing", "receipt_missing"} {
		t.Run(scenario, func(t *testing.T) {
			g, req, f, h := assemblySharedFixture(t)
			lease := assemblySharedAdmit(t, g, req, f, h)
			release, err := h.Lock(context.Background(), lease.Identity)
			if err != nil {
				t.Fatal(err)
			}
			// Match the real runtime: reserve/lock, open the exclusive capture,
			// then publish creating/created/starting/running before producing data.
			writer, err := h.CaptureSink(context.Background(), lease.Identity)
			if err != nil {
				release()
				t.Fatal(err)
			}
			defer writer.Close()
			ref := strings.Repeat("b", 64)
			for _, phase := range [][2]string{{"reserved", "creating"}, {"creating", "created"}, {"created", "starting"}, {"starting", "running"}} {
				phaseRef := ref
				if phase[1] == "creating" {
					phaseRef = ""
				}
				if err = h.Transition(context.Background(), lease.Identity, phase[0], phase[1], phaseRef); err != nil {
					release()
					t.Fatal(err)
				}
			}
			if _, err = writer.Write([]byte("one")); err != nil {
				release()
				t.Fatal(err)
			}
			if err = writer.Close(); err != nil {
				release()
				t.Fatal(err)
			}
			f.observation = "exited"
			capture := backend.DocumentImageAssemblyCapture{Digest: fixtureFenceHash("one"), Bytes: 3}
			zero := 0
			published := documentImageAssemblyOutcome{Identity: lease.Identity, InputClosureRef: lease.InputClosureRef, Ref: ref, Stage: "running", State: "captured_exit_zero_not_registered", Capture: capture, ExitCode: &zero, ReaderSettled: true, WatchdogSettled: true, ResourcesHeld: true}
			if err = h.Publish(context.Background(), published); err != nil {
				release()
				t.Fatal(err)
			}
			// Only durable files/ledger remain at this process-loss boundary. Explicit
			// release models kernel lock release; no subprocess or fake crash runs.
			release()
			store, err := g.assemblyPrivateCapture(lease.Identity)
			if err != nil {
				t.Fatal(err)
			}
			if scenario == "capture_missing" {
				if err = os.Remove(store.path); err != nil {
					t.Fatal(err)
				}
			}
			if scenario == "receipt_missing" {
				if err = os.Remove(store.receipt); err != nil {
					t.Fatal(err)
				}
			}
			ledger, err := g.readReservationLedger(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			retained := ledger.ImageAssemblies[0]
			verified := &assemblySharedFenceExecutor{assemblyRuntimeExecutor: f}
			req.ValidateCurrent = func(context.Context) error { return errors.New("past input evidence unavailable") }
			h, err = g.api.documentImageAssemblyRecoveryHooks(verified, req, retained)
			if err != nil {
				t.Fatal(err)
			}
			fence := backend.DocumentImageAssemblyFenceRequest{Identity: lease.Identity, ExpectedRef: ref, IntentID: strings.Repeat("a", 32)}
			out, err := recoverDocumentImageAssembly(context.Background(), verified, h, retained, fence, nil)
			if err != nil || out.ResourcesHeld || !out.CleanupConfirmed || out.InputClosureRef != retained.InputClosureRef || out.Capture != (backend.DocumentImageAssemblyCapture{}) || out.RetainedCapture != capture || out.ExitCode != nil || out.State != "interrupted_capture_unavailable_owned_resources_retired" {
				t.Fatal("published capture deadlocked cleanup or became current usable output", out, err)
			}
			ledger, err = g.readReservationLedger(context.Background())
			if err != nil || ledger.ImageAssemblies[0].State != "finished" || ledger.ImageAssemblies[0].Capture != capture || ledger.ImageAssemblyFences[0].State != "retired" || f.starts != 0 {
				t.Fatal("retained capture lineage lost or recovery started work", ledger, err)
			}
			receipt, err := store.read()
			if err != nil || receipt.Outcome.InputClosureRef != retained.InputClosureRef || receipt.CaptureExitVerified || receipt.Outcome.RetainedCapture != capture || receipt.RetainedCaptureExitVerified != (scenario != "receipt_missing") {
				t.Fatal("historical verified/unknown capture evidence fabricated or erased", receipt, err)
			}
		})
	}
}

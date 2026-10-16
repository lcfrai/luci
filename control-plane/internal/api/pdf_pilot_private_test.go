package api

import (
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestPDFPilotOneChargeSerialHandoffAndWholeDeadline(t *testing.T) {
	request := pdfPilotRequest{
		AttemptID: strings.Repeat("a", 32), ScopeDigest: strings.Repeat("b", 64),
		Actor:            management.ExecutionActorRef{ProjectID: "project", Local: true},
		ApprovalRevision: 1, ApprovalDigest: strings.Repeat("5", 64),
		EngineID: "selected-engine", InstanceID: "selected-instance", SourceCommit: strings.Repeat("c", 40),
		ImageID: "sha256:" + strings.Repeat("d", 64), ImageReceiptDigest: strings.Repeat("e", 64),
		PackageReceiptDigest: strings.Repeat("f", 64), FontDigest: strings.Repeat("1", 64),
		FontReceiptDigest: strings.Repeat("2", 64), RendererReceipt: strings.Repeat("3", 64),
		ReaderReceipt: strings.Repeat("4", 64),
	}
	if !validPDFPilotRequest(request) {
		t.Fatal("complete trusted-shape request refused")
	}
	started := time.Date(2026, 10, 16, 0, 0, 0, 0, time.UTC)
	current := pdfPilotCurrentBinding{
		Actor: request.Actor, ApprovalKind: "reviewed-pdf-pilot-input-v1", ApprovalRevision: 1,
		ApprovalDigest: request.ApprovalDigest, ObservedAt: started,
		SourceCommit: request.SourceCommit, ImageID: request.ImageID, ImageReceiptDigest: request.ImageReceiptDigest,
		PackageReceiptDigest: request.PackageReceiptDigest, FontDigest: request.FontDigest,
		FontReceiptDigest: request.FontReceiptDigest, RendererReceipt: request.RendererReceipt, ReaderReceipt: request.ReaderReceipt,
	}
	if !validPDFPilotAdmissionBinding(request, current, started.Add(time.Second)) {
		t.Fatal("exact current private binding refused")
	}
	changed := current
	changed.SourceCommit = strings.Repeat("6", 40)
	if validPDFPilotAdmissionBinding(request, changed, started.Add(time.Second)) {
		t.Fatal("stale source commit was admitted")
	}
	changed = current
	changed.Actor.ProjectID = "other-project"
	if validPDFPilotAdmissionBinding(request, changed, started.Add(time.Second)) || validPDFPilotAdmissionBinding(request, current, started.Add(6*time.Second)) {
		t.Fatal("revoked or stale actor observation was admitted")
	}
	lease := pdfPilotLease{
		Request: request, Deadlines: newPDFPilotDeadlines(started), ChargeBytes: backend.PDFPilotMemoryBytes,
		HostCaptureBytes: backend.PDFPilotHostCaptureBytes, StageDiskBytes: backend.PDFPilotStageDiskBytes,
		SourceBindingDigest: request.ApprovalDigest,
		Renderer:            pdfPilotChildLease{AllocationID: backend.PDFPilotAllocationID(request.ScopeDigest, request.AttemptID, backend.PDFPilotRenderer), State: "reserved"},
		Reader:              pdfPilotChildLease{AllocationID: backend.PDFPilotAllocationID(request.ScopeDigest, request.AttemptID, backend.PDFPilotReader), State: "reserved"},
	}
	if !validPDFPilotLease(lease) || lease.Deadlines.RendererPhaseEnd.Sub(started) != 40*time.Second || lease.Deadlines.ReaderPhaseEnd.Sub(started) != 80*time.Second || lease.Deadlines.WholeEnd.Sub(started) != 120*time.Second {
		t.Fatal("one-charge immutable whole-attempt reservation refused")
	}
	identity := backend.PDFPilotIdentity{
		Protocol: backend.PDFPilotProtocol, Role: backend.PDFPilotRenderer, ScopeDigest: request.ScopeDigest, AttemptID: request.AttemptID,
		AllocationID: lease.Renderer.AllocationID, ActorDigest: pdfPilotActorDigest(request.Actor), ApprovalDigest: request.ApprovalDigest,
		EngineID: request.EngineID, InstanceID: request.InstanceID, SourceCommit: request.SourceCommit,
		ImageID: request.ImageID, ImageReceiptDigest: request.ImageReceiptDigest, BinaryReceiptDigest: request.RendererReceipt,
		PackageReceiptDigest: request.PackageReceiptDigest, OriginalFontDigest: request.FontDigest,
		OriginalFontReceipt: request.FontReceiptDigest, OutputStageDigest: strings.Repeat("6", 64),
		CommandDigest: strings.Repeat("7", 64),
	}
	lease.Renderer.Identity, lease.Renderer.State = &identity, "create-unknown"
	if !validPDFPilotLease(lease) {
		t.Fatal("lost Create ACK must retain the exact renderer identity")
	}
	lease.Renderer.Ref, lease.Renderer.State = strings.Repeat("8", 64), "captured"
	lease.Renderer.StartIntentAt, lease.Renderer.ExitObservedAt, lease.Renderer.CaptureAt = started.Add(2*time.Second), started.Add(20*time.Second), started.Add(25*time.Second)
	lease.RendererOutputDigest, lease.RendererOutputBytes, lease.RendererFileIdentity = strings.Repeat("9", 64), 1024, strings.Repeat("a", 64)
	reader := identity
	reader.Role, reader.AllocationID = backend.PDFPilotReader, lease.Reader.AllocationID
	reader.BinaryReceiptDigest, reader.OutputStageDigest = request.ReaderReceipt, strings.Repeat("b", 64)
	reader.RendererOutputDigest = lease.RendererOutputDigest
	lease.Reader.Identity = &reader
	if validPDFPilotLease(lease) {
		t.Fatal("reader identity became eligible before exact renderer retirement")
	}
	lease.Renderer.State, lease.Renderer.RetirementDigest = "retired", strings.Repeat("c", 64)
	lease.Renderer.RetiredAt = started.Add(30 * time.Second)
	lease.Reader.State, lease.Reader.Ref = "created", strings.Repeat("d", 64)
	lease.Reader.StartIntentAt = started.Add(45 * time.Second)
	if !validPDFPilotLease(lease) {
		t.Fatal("positive renderer retirement and durable PDF did not admit the serial reader")
	}
	unknown := lease
	unknown.Reader.State, unknown.Reader.Ref = "create-unknown", ""
	unknown.Reader.StartIntentAt = time.Time{}
	if !validPDFPilotLease(unknown) {
		t.Fatal("unknown reader Create must retain one exact identity and charge")
	}
	unknown.Closed, unknown.OutcomeDigest = true, strings.Repeat("e", 64)
	if validPDFPilotLease(unknown) {
		t.Fatal("unknown Create was falsely settled by cancellation or elapsed time")
	}
	lease.Reader.State, lease.Reader.RetirementDigest = "retired", strings.Repeat("e", 64)
	lease.Reader.ExitObservedAt, lease.Reader.CaptureAt, lease.Reader.RetiredAt = started.Add(60*time.Second), started.Add(65*time.Second), started.Add(70*time.Second)
	lease.ReaderTextDigest, lease.ReaderTextBytes = strings.Repeat("f", 64), 40
	lease.ReaderPNGHash, lease.ReaderPNGBytes = strings.Repeat("1", 64), 2048
	lease.Closed, lease.Accepted, lease.OutcomeDigest = true, true, strings.Repeat("2", 64)
	if !validPDFPilotLease(lease) {
		t.Fatal("positively retired and independently captured reader outcome refused")
	}
	accepted := lease
	for _, tt := range []struct {
		name   string
		change func(*pdfPilotLease)
	}{
		{"renderer retired before start and reader overlaps", func(v *pdfPilotLease) {
			v.Renderer.RetiredAt = started.Add(time.Second)
			v.Reader.StartIntentAt = started.Add(3 * time.Second)
		}},
		{"renderer retired before exit", func(v *pdfPilotLease) {
			v.Renderer.RetiredAt = started.Add(10 * time.Second)
		}},
		{"renderer retired before capture", func(v *pdfPilotLease) {
			v.Renderer.RetiredAt = started.Add(22 * time.Second)
		}},
		{"reader starts before renderer capture", func(v *pdfPilotLease) {
			v.Reader.StartIntentAt = started.Add(24 * time.Second)
		}},
		{"reader starts before renderer retirement", func(v *pdfPilotLease) {
			v.Reader.StartIntentAt = started.Add(28 * time.Second)
		}},
		{"reader retired before start", func(v *pdfPilotLease) {
			v.Reader.RetiredAt = started.Add(44 * time.Second)
		}},
		{"reader retired before exit", func(v *pdfPilotLease) {
			v.Reader.RetiredAt = started.Add(55 * time.Second)
		}},
		{"reader retired before capture", func(v *pdfPilotLease) {
			v.Reader.RetiredAt = started.Add(62 * time.Second)
		}},
	} {
		t.Run(tt.name, func(t *testing.T) {
			invalid := accepted
			tt.change(&invalid)
			if validPDFPilotLease(invalid) {
				t.Fatal("out-of-order full lease was accepted")
			}
		})
	}
	retiredWithoutStart := accepted
	retiredWithoutStart.Renderer.StartIntentAt = time.Time{}
	retiredWithoutStart.Renderer.ExitObservedAt = time.Time{}
	retiredWithoutStart.Renderer.CaptureAt = time.Time{}
	retiredWithoutStart.Renderer.RetiredAt = started.Add(time.Second)
	retiredWithoutStart.RendererOutputDigest = ""
	retiredWithoutStart.RendererOutputBytes = 0
	retiredWithoutStart.RendererFileIdentity = ""
	retiredWithoutStart.Reader = pdfPilotChildLease{AllocationID: accepted.Reader.AllocationID, State: "reserved"}
	retiredWithoutStart.ReaderTextDigest, retiredWithoutStart.ReaderTextBytes = "", 0
	retiredWithoutStart.ReaderPNGHash, retiredWithoutStart.ReaderPNGBytes = "", 0
	retiredWithoutStart.Closed, retiredWithoutStart.Accepted, retiredWithoutStart.OutcomeDigest = false, false, ""
	if !validPDFPilotLease(retiredWithoutStart) {
		t.Fatal("positive retirement of a never-started child must remain readable")
	}
	readerRetiredWithoutStart := accepted
	readerRetiredWithoutStart.Reader.StartIntentAt = time.Time{}
	readerRetiredWithoutStart.Reader.ExitObservedAt = time.Time{}
	readerRetiredWithoutStart.Reader.CaptureAt = time.Time{}
	readerRetiredWithoutStart.Reader.RetiredAt = started.Add(40 * time.Second)
	readerRetiredWithoutStart.ReaderTextDigest, readerRetiredWithoutStart.ReaderTextBytes = "", 0
	readerRetiredWithoutStart.ReaderPNGHash, readerRetiredWithoutStart.ReaderPNGBytes = "", 0
	readerRetiredWithoutStart.Closed, readerRetiredWithoutStart.Accepted, readerRetiredWithoutStart.OutcomeDigest = false, false, ""
	if !validPDFPilotLease(readerRetiredWithoutStart) {
		t.Fatal("positive retirement of a never-started reader must remain readable")
	}
	lease.ChargeBytes = backend.PDFPilotMemoryBytes * 2
	if validPDFPilotLease(lease) {
		t.Fatal("concurrent two-child memory charge became valid")
	}
	lease.ChargeBytes = backend.PDFPilotMemoryBytes
	lease.Deadlines.WholeEnd = lease.Deadlines.WholeEnd.Add(time.Second)
	if validPDFPilotLease(lease) {
		t.Fatal("extended cleanup/whole deadline became valid")
	}
	lease.Deadlines = newPDFPilotDeadlines(started)
	lease.Reader.RetiredAt = started.Add(121 * time.Second)
	if validPDFPilotLease(lease) {
		t.Fatal("late retirement became an on-time accepted render")
	}
	lease.Accepted = false
	if !validPDFPilotLease(lease) {
		t.Fatal("late positive retirement should remain readable as failed historical evidence")
	}
}

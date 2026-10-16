package api

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// No PDF pilot is registered or runnable from this file. The shared ledger,
// source-catalogue admission, private stage owner and selected SDK adapter are
// separate reviewed insertions. Shape validation cannot promote provisional
// Debian metadata into trusted source or image evidence.
type pdfPilotRequest struct {
	AttemptID            string                       `json:"attemptId"`
	ScopeDigest          string                       `json:"scopeDigest"`
	Actor                management.ExecutionActorRef `json:"actor"`
	ApprovalRevision     uint64                       `json:"approvalRevision"`
	ApprovalDigest       string                       `json:"approvalDigest"`
	EngineID             string                       `json:"engineId"`
	InstanceID           string                       `json:"instanceId"`
	SourceCommit         string                       `json:"sourceCommit"`
	ImageID              string                       `json:"imageId"`
	ImageReceiptDigest   string                       `json:"imageReceiptDigest"`
	PackageReceiptDigest string                       `json:"packageReceiptDigest"`
	FontDigest           string                       `json:"fontDigest"`
	FontReceiptDigest    string                       `json:"fontReceiptDigest"`
	RendererReceipt      string                       `json:"rendererReceipt"`
	ReaderReceipt        string                       `json:"readerReceipt"`
}

func pdfPilotActorDigest(actor management.ExecutionActorRef) string {
	data, _ := json.Marshal(actor)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func validPDFPilotRequest(value pdfPilotRequest) bool {
	if value.Actor.ProjectID == "" || value.Actor.Local && (value.Actor.UserID != "" || value.Actor.TeamID != "") || !value.Actor.Local && value.Actor.UserID == "" ||
		backend.PDFPilotAllocationID(value.ScopeDigest, value.AttemptID, backend.PDFPilotRenderer) == "" ||
		value.ApprovalRevision == 0 || !checkHex(value.ApprovalDigest, 32) || !checkHex(value.SourceCommit, 20) || !strings.HasPrefix(value.ImageID, "sha256:") || !checkHex(strings.TrimPrefix(value.ImageID, "sha256:"), 32) ||
		!checkHex(value.ImageReceiptDigest, 32) || !checkHex(value.PackageReceiptDigest, 32) || !checkHex(value.FontDigest, 32) || !checkHex(value.FontReceiptDigest, 32) ||
		!checkHex(value.RendererReceipt, 32) || !checkHex(value.ReaderReceipt, 32) || value.EngineID == "" || len(value.EngineID) > 200 ||
		strings.TrimSpace(value.EngineID) != value.EngineID || strings.ContainsAny(value.EngineID, "\x00\r\n") || backend.ValidateInstance(value.InstanceID) != nil {
		return false
	}
	return true
}

// A later reviewed private source resolver must read this from authoritative
// product/source state INSIDE final admission mutation, not from the request,
// desktop, provisional Debian inventory or a saved preflight observation.
// Shape matching here cannot itself authenticate that resolver or grant work.
type pdfPilotCurrentBinding struct {
	Actor                management.ExecutionActorRef
	ApprovalKind         string
	ApprovalRevision     uint64
	ApprovalDigest       string
	ObservedAt           time.Time
	SourceCommit         string
	ImageID              string
	ImageReceiptDigest   string
	PackageReceiptDigest string
	FontDigest           string
	FontReceiptDigest    string
	RendererReceipt      string
	ReaderReceipt        string
}

func validPDFPilotAdmissionBinding(request pdfPilotRequest, current pdfPilotCurrentBinding, now time.Time) bool {
	return validPDFPilotRequest(request) && current.Actor == request.Actor && current.ApprovalKind == "reviewed-pdf-pilot-input-v1" &&
		current.ApprovalRevision == request.ApprovalRevision && current.ApprovalDigest == request.ApprovalDigest && !current.ObservedAt.IsZero() &&
		!current.ObservedAt.After(now) && now.Sub(current.ObservedAt) <= 5*time.Second &&
		current.SourceCommit == request.SourceCommit && current.ImageID == request.ImageID &&
		current.ImageReceiptDigest == request.ImageReceiptDigest && current.PackageReceiptDigest == request.PackageReceiptDigest &&
		current.FontDigest == request.FontDigest && current.FontReceiptDigest == request.FontReceiptDigest &&
		current.RendererReceipt == request.RendererReceipt && current.ReaderReceipt == request.ReaderReceipt
}

// One immutable 120-second envelope includes both serial child phases,
// bounded capture, publication and a reserved 40-second cleanup window.
type pdfPilotDeadlines struct {
	ReservedAt       time.Time `json:"reservedAt"`
	RendererPhaseEnd time.Time `json:"rendererPhaseEnd"`
	ReaderPhaseEnd   time.Time `json:"readerPhaseEnd"`
	WholeEnd         time.Time `json:"wholeEnd"`
}

func newPDFPilotDeadlines(reservedAt time.Time) pdfPilotDeadlines {
	return pdfPilotDeadlines{
		ReservedAt:       reservedAt,
		RendererPhaseEnd: reservedAt.Add(backend.PDFPilotRendererPhaseBudget),
		ReaderPhaseEnd:   reservedAt.Add(backend.PDFPilotRendererPhaseBudget + backend.PDFPilotReaderPhaseBudget),
		WholeEnd:         reservedAt.Add(backend.PDFPilotWholeBudget),
	}
}

func validPDFPilotDeadlines(value pdfPilotDeadlines) bool {
	return !value.ReservedAt.IsZero() && value == newPDFPilotDeadlines(value.ReservedAt) &&
		value.WholeEnd.Sub(value.ReaderPhaseEnd) == 40*time.Second
}

// Both allocation IDs exist from reservation. The reader's full identity is
// populated only after a durably captured exact renderer output and positive
// renderer retirement. A lost Create ACK retains the same ID indefinitely.
type pdfPilotChildLease struct {
	AllocationID     string                    `json:"allocationId"`
	Identity         *backend.PDFPilotIdentity `json:"identity,omitempty"`
	State            string                    `json:"state"`
	Ref              string                    `json:"ref,omitempty"`
	StartIntentAt    time.Time                 `json:"startIntentAt,omitempty"`
	ExitObservedAt   time.Time                 `json:"exitObservedAt,omitempty"`
	CaptureAt        time.Time                 `json:"captureAt,omitempty"`
	RetiredAt        time.Time                 `json:"retiredAt,omitempty"`
	RetirementDigest string                    `json:"retirementDigest,omitempty"`
}

func validPDFPilotChild(value pdfPilotChildLease, scopeDigest, attemptID string, role backend.PDFPilotRole) bool {
	if value.AllocationID != backend.PDFPilotAllocationID(scopeDigest, attemptID, role) || value.AllocationID == "" {
		return false
	}
	if value.Identity != nil && (!backend.ValidatePDFPilotIdentity(*value.Identity) || value.Identity.Role != role || value.Identity.ScopeDigest != scopeDigest || value.Identity.AttemptID != attemptID || value.Identity.AllocationID != value.AllocationID) {
		return false
	}
	switch value.State {
	case "reserved":
		return value.Ref == "" && value.RetirementDigest == "" && value.StartIntentAt.IsZero() && value.ExitObservedAt.IsZero() && value.CaptureAt.IsZero() && value.RetiredAt.IsZero()
	case "create-unknown":
		return value.Identity != nil && (value.Ref == "" || checkHex(value.Ref, 32)) && value.RetirementDigest == "" && value.StartIntentAt.IsZero() && value.ExitObservedAt.IsZero() && value.CaptureAt.IsZero() && value.RetiredAt.IsZero()
	case "created", "start-unknown", "running", "exited", "capture-unknown", "captured", "retire-unknown":
		return value.Identity != nil && checkHex(value.Ref, 32) && value.RetirementDigest == "" && value.RetiredAt.IsZero()
	case "retired":
		return value.Identity != nil && checkHex(value.Ref, 32) && checkHex(value.RetirementDigest, 32) && !value.RetiredAt.IsZero()
	default:
		return false
	}
}

func validPDFPilotChildTiming(value pdfPilotChildLease, start, phaseEnd time.Time) bool {
	if !value.StartIntentAt.IsZero() && (value.StartIntentAt.Before(start) || !value.StartIntentAt.Before(phaseEnd)) {
		return false
	}
	if !value.ExitObservedAt.IsZero() && (value.StartIntentAt.IsZero() || value.ExitObservedAt.Before(value.StartIntentAt)) {
		return false
	}
	if !value.CaptureAt.IsZero() && (value.ExitObservedAt.IsZero() || value.CaptureAt.Before(value.ExitObservedAt)) {
		return false
	}
	// Positive readback may settle an unknown child after the original whole
	// deadline. It remains historical evidence, never an on-time acceptance.
	if !value.RetiredAt.IsZero() && value.RetiredAt.Before(start) {
		return false
	}
	if value.State == "start-unknown" || value.State == "running" || value.State == "exited" || value.State == "capture-unknown" || value.State == "captured" {
		if value.StartIntentAt.IsZero() {
			return false
		}
	}
	if value.State == "exited" || value.State == "capture-unknown" || value.State == "captured" {
		if value.ExitObservedAt.IsZero() {
			return false
		}
	}
	if value.State == "captured" && value.CaptureAt.IsZero() {
		return false
	}
	return true
}

// This dedicated type is for a future additive PDFPilots slice in the shared
// execution ledger. It does not mutate that ledger or make any provider call.
type pdfPilotLease struct {
	Request              pdfPilotRequest    `json:"request"`
	Deadlines            pdfPilotDeadlines  `json:"deadlines"`
	ChargeBytes          uint64             `json:"chargeBytes"`
	HostCaptureBytes     uint64             `json:"hostCaptureBytes"`
	StageDiskBytes       uint64             `json:"stageDiskBytes"`
	SourceBindingDigest  string             `json:"sourceBindingDigest"`
	Renderer             pdfPilotChildLease `json:"renderer"`
	Reader               pdfPilotChildLease `json:"reader"`
	RendererOutputDigest string             `json:"rendererOutputDigest,omitempty"`
	RendererOutputBytes  int                `json:"rendererOutputBytes,omitempty"`
	RendererFileIdentity string             `json:"rendererFileIdentity,omitempty"`
	ReaderTextDigest     string             `json:"readerTextDigest,omitempty"`
	ReaderTextBytes      int                `json:"readerTextBytes,omitempty"`
	ReaderPNGHash        string             `json:"readerPngDigest,omitempty"`
	ReaderPNGBytes       int                `json:"readerPngBytes,omitempty"`
	Closed               bool               `json:"closed,omitempty"`
	Accepted             bool               `json:"accepted,omitempty"`
	OutcomeDigest        string             `json:"outcomeDigest,omitempty"`
}

func validPDFPilotLease(value pdfPilotLease) bool {
	r := value.Request
	if !validPDFPilotRequest(r) || !validPDFPilotDeadlines(value.Deadlines) || value.ChargeBytes != backend.PDFPilotMemoryBytes ||
		value.HostCaptureBytes != backend.PDFPilotHostCaptureBytes || value.StageDiskBytes != backend.PDFPilotStageDiskBytes ||
		value.SourceBindingDigest != r.ApprovalDigest || !validPDFPilotChild(value.Renderer, r.ScopeDigest, r.AttemptID, backend.PDFPilotRenderer) ||
		!validPDFPilotChild(value.Reader, r.ScopeDigest, r.AttemptID, backend.PDFPilotReader) ||
		!validPDFPilotChildTiming(value.Renderer, value.Deadlines.ReservedAt, value.Deadlines.RendererPhaseEnd) ||
		!validPDFPilotChildTiming(value.Reader, value.Deadlines.ReservedAt, value.Deadlines.ReaderPhaseEnd) {
		return false
	}
	if !value.Reader.StartIntentAt.IsZero() && (value.Renderer.State != "retired" || value.Renderer.RetiredAt.IsZero() || value.Reader.StartIntentAt.Before(value.Renderer.RetiredAt)) {
		return false
	}
	for _, child := range []pdfPilotChildLease{value.Renderer, value.Reader} {
		if child.Identity != nil && (child.Identity.ActorDigest != pdfPilotActorDigest(r.Actor) || child.Identity.ApprovalDigest != r.ApprovalDigest || child.Identity.EngineID != r.EngineID || child.Identity.InstanceID != r.InstanceID ||
			child.Identity.SourceCommit != r.SourceCommit || child.Identity.ImageID != r.ImageID || child.Identity.ImageReceiptDigest != r.ImageReceiptDigest ||
			child.Identity.PackageReceiptDigest != r.PackageReceiptDigest || child.Identity.OriginalFontDigest != r.FontDigest || child.Identity.OriginalFontReceipt != r.FontReceiptDigest) {
			return false
		}
	}
	if value.Renderer.Identity != nil && value.Renderer.Identity.BinaryReceiptDigest != r.RendererReceipt || value.Reader.Identity != nil && value.Reader.Identity.BinaryReceiptDigest != r.ReaderReceipt {
		return false
	}
	output := value.RendererOutputDigest != "" || value.RendererOutputBytes != 0 || value.RendererFileIdentity != ""
	if output && (!checkHex(value.RendererOutputDigest, 32) || !checkHex(value.RendererFileIdentity, 32) || value.RendererOutputBytes <= 0 || value.RendererOutputBytes > backend.PDFPilotMaxPDFBytes ||
		value.Renderer.CaptureAt.IsZero() ||
		value.Renderer.State != "captured" && value.Renderer.State != "retire-unknown" && value.Renderer.State != "retired") {
		return false
	}
	if value.Renderer.State == "captured" && !output {
		return false
	}
	if value.Reader.State != "reserved" && (value.Renderer.State != "retired" || !output || value.Reader.Identity == nil || value.Reader.Identity.RendererOutputDigest != value.RendererOutputDigest) {
		return false
	}
	if value.Reader.Identity != nil && (value.Renderer.State != "retired" || value.Reader.Identity.RendererOutputDigest != value.RendererOutputDigest) {
		return false
	}
	readerOutput := value.ReaderTextDigest != "" || value.ReaderTextBytes != 0 || value.ReaderPNGHash != "" || value.ReaderPNGBytes != 0
	if readerOutput && (!checkHex(value.ReaderTextDigest, 32) || !checkHex(value.ReaderPNGHash, 32) || value.ReaderTextBytes <= 0 || value.ReaderTextBytes > backend.PDFPilotMaxReaderTextBytes || value.ReaderPNGBytes <= 0 || value.ReaderPNGBytes > backend.PDFPilotMaxReaderPNGBytes ||
		value.Reader.CaptureAt.IsZero() ||
		value.Reader.State != "captured" && value.Reader.State != "retire-unknown" && value.Reader.State != "retired") {
		return false
	}
	if value.Reader.State == "captured" && !readerOutput {
		return false
	}
	if value.Accepted && (!value.Closed || value.Reader.State != "retired" || !output || !readerOutput ||
		value.Renderer.ExitObservedAt.Sub(value.Renderer.StartIntentAt) > backend.PDFPilotChildRunBudget || value.Reader.ExitObservedAt.Sub(value.Reader.StartIntentAt) > backend.PDFPilotChildRunBudget ||
		value.Renderer.CaptureAt.After(value.Deadlines.RendererPhaseEnd) || value.Reader.CaptureAt.After(value.Deadlines.ReaderPhaseEnd) ||
		value.Renderer.RetiredAt.After(value.Deadlines.WholeEnd) || value.Reader.RetiredAt.After(value.Deadlines.WholeEnd)) {
		return false
	}
	if value.Closed {
		return checkHex(value.OutcomeDigest, 32) && (value.Renderer.State == "reserved" || value.Renderer.State == "retired") && (value.Reader.State == "reserved" || value.Reader.State == "retired")
	}
	return value.OutcomeDigest == ""
}

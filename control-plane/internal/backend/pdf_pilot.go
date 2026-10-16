package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"path/filepath"
	"strings"
	"time"
)

// PDFPilot is a private, fixed-input source pilot. These types grant no Docker
// capability: there is deliberately no constructor, registered implementation,
// public route, image lookup, downloader or arbitrary command in this file.
const (
	PDFPilotProtocol            = 1
	PDFPilotMemoryBytes         = 512 << 20
	PDFPilotHostCaptureBytes    = 64 << 20
	PDFPilotStageDiskBytes      = 48 << 20
	PDFPilotCPUMillis           = 1000
	PDFPilotPIDs                = 32
	PDFPilotWholeBudget         = 120 * time.Second
	PDFPilotRendererPhaseBudget = 40 * time.Second
	PDFPilotReaderPhaseBudget   = 40 * time.Second
	PDFPilotChildRunBudget      = 30 * time.Second
	PDFPilotFontBytes           = 19484784
	PDFPilotMaxPDFBytes         = 16 << 20
	PDFPilotMaxReaderTextBytes  = 1 << 20
	PDFPilotMaxReaderPNGBytes   = 16 << 20
)

type PDFPilotRole string

const (
	PDFPilotRenderer PDFPilotRole = "renderer"
	PDFPilotReader   PDFPilotRole = "reader"
)

func pdfPilotHex(value string, bytes int) bool {
	if len(value) != bytes*2 || strings.ToLower(value) != value {
		return false
	}
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == bytes
}

// Both role IDs are derived before the first Create. A retry with the same
// identity may inspect/retire, but cannot issue a second Create.
func PDFPilotAllocationID(scopeDigest, attemptID string, role PDFPilotRole) string {
	if !pdfPilotHex(scopeDigest, 32) || !pdfPilotHex(attemptID, 16) || role != PDFPilotRenderer && role != PDFPilotReader {
		return ""
	}
	sum := sha256.Sum256([]byte("luci-pdf-pilot-v1\n" + scopeDigest + "\n" + attemptID + "\n" + string(role)))
	return hex.EncodeToString(sum[:])
}

// Trusted controller/source-catalogue facts, not fields from an HTTP request.
// A shape-valid digest is not proof that a selected source or image is trusted;
// final API reservation and the SDK adapter must verify each original receipt.
type PDFPilotIdentity struct {
	Protocol             int          `json:"protocol"`
	Role                 PDFPilotRole `json:"role"`
	ScopeDigest          string       `json:"scopeDigest"`
	AttemptID            string       `json:"attemptId"`
	AllocationID         string       `json:"allocationId"`
	ActorDigest          string       `json:"actorDigest"`
	ApprovalDigest       string       `json:"approvalDigest"`
	EngineID             string       `json:"engineId"`
	InstanceID           string       `json:"instanceId"`
	SourceCommit         string       `json:"sourceCommit"`
	ImageID              string       `json:"imageId"`
	ImageReceiptDigest   string       `json:"imageReceiptDigest"`
	BinaryReceiptDigest  string       `json:"binaryReceiptDigest"`
	PackageReceiptDigest string       `json:"packageReceiptDigest"`
	OriginalFontDigest   string       `json:"originalFontDigest"`
	OriginalFontReceipt  string       `json:"originalFontReceiptDigest"`
	OutputStageDigest    string       `json:"outputStageDigest"`
	CommandDigest        string       `json:"commandDigest"`
	RendererOutputDigest string       `json:"rendererOutputDigest,omitempty"`
}

func ValidatePDFPilotIdentity(value PDFPilotIdentity) bool {
	if value.Protocol != PDFPilotProtocol || value.AllocationID != PDFPilotAllocationID(value.ScopeDigest, value.AttemptID, value.Role) || value.AllocationID == "" ||
		!pdfPilotHex(value.ActorDigest, 32) || !pdfPilotHex(value.ApprovalDigest, 32) || !pdfPilotHex(value.SourceCommit, 20) || !strings.HasPrefix(value.ImageID, "sha256:") || !pdfPilotHex(strings.TrimPrefix(value.ImageID, "sha256:"), 32) ||
		!pdfPilotHex(value.ImageReceiptDigest, 32) || !pdfPilotHex(value.BinaryReceiptDigest, 32) || !pdfPilotHex(value.PackageReceiptDigest, 32) ||
		!pdfPilotHex(value.OriginalFontDigest, 32) || !pdfPilotHex(value.OriginalFontReceipt, 32) || !pdfPilotHex(value.OutputStageDigest, 32) || !pdfPilotHex(value.CommandDigest, 32) ||
		value.EngineID == "" || len(value.EngineID) > 200 || strings.TrimSpace(value.EngineID) != value.EngineID || strings.ContainsAny(value.EngineID, "\x00\r\n") || ValidateInstance(value.InstanceID) != nil {
		return false
	}
	if value.Role == PDFPilotRenderer {
		return value.RendererOutputDigest == ""
	}
	return pdfPilotHex(value.RendererOutputDigest, 32)
}

// The API supplies OutputStagePath from a retained private no-follow directory
// handle. No caller may supply a path or select a new role/command. The future
// adapter must independently verify the original directory/file identities,
// exact image and fixed command/mount/resource configuration before Start.
type PDFPilotSpec struct {
	Identity          PDFPilotIdentity
	OutputStagePath   string
	OriginalFontPath  string // renderer only; one exact read-only original
	OriginalFontBytes int    // renderer only; original fixed face, not a caller limit
	RendererPDFPath   string // reader only; retained digest, read-only mount
	RendererPDFBytes  int    // reader only
	UserID            int
	GroupID           int
}

func ValidatePDFPilotSpec(value PDFPilotSpec) bool {
	if !ValidatePDFPilotIdentity(value.Identity) || value.UserID <= 0 || value.GroupID <= 0 || value.UserID > 65535 || value.GroupID > 65535 ||
		!filepath.IsAbs(value.OutputStagePath) || filepath.Clean(value.OutputStagePath) != value.OutputStagePath ||
		filepath.Base(value.OutputStagePath) != string(value.Identity.Role) || filepath.Base(filepath.Dir(value.OutputStagePath)) != value.Identity.AttemptID {
		return false
	}
	if value.Identity.Role == PDFPilotRenderer {
		return filepath.IsAbs(value.OriginalFontPath) && value.OriginalFontBytes == PDFPilotFontBytes && value.RendererPDFPath == "" && value.RendererPDFBytes == 0
	}
	return value.OriginalFontPath == "" && value.OriginalFontBytes == 0 && filepath.IsAbs(value.RendererPDFPath) && value.RendererPDFBytes > 0 && value.RendererPDFBytes <= PDFPilotMaxPDFBytes
}

type PreparedPDFPilot struct {
	Spec                  PDFPilotSpec
	ImageVirtualSizeBytes uint64
	StorageDriver         string
	ConfigurationDigest   string
}

type PDFPilotObservation struct {
	Ref       string
	State     string // created, running, exited, absent; unknown is an error
	ExitCode  *int
	OOMKilled bool
}

type PDFPilotFenceRequest struct {
	Identity            PDFPilotIdentity
	IntentID            string
	ExpectedOriginalRef string
}

type PDFPilotFenceProof struct {
	Identity       PDFPilotIdentity
	IntentID       string
	FenceRef       string
	ConfigDigest   string
	AcknowledgedAt string
	ObservedAt     string
}

// Exactly one immutable child per role. Capture is deliberately absent:
// output is read through the API's retained, bounded private stage handle.
// The caller journals every effect before/after it and runs exact retirement
// on an independent watchdog context, never behind a blocked stage reader.
type PrivatePDFPilotExecutor interface {
	PreparePDFPilot(context.Context, PDFPilotSpec) (PreparedPDFPilot, error)
	CreatePDFPilot(context.Context, PreparedPDFPilot) (string, error)
	StartPDFPilot(context.Context, PDFPilotIdentity, string) error
	InspectPDFPilot(context.Context, PDFPilotIdentity, string) (PDFPilotObservation, error)
	RetirePDFPilot(context.Context, PDFPilotIdentity, string) error
	FencePDFPilotLaunch(context.Context, PDFPilotFenceRequest) (PDFPilotFenceProof, error)
	VerifyPDFPilotLaunchFence(context.Context, PDFPilotFenceRequest, PDFPilotFenceProof) error
}

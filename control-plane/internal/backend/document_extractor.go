package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const (
	DocumentExtractionProtocol = 1
	DocumentExtractionProfile  = "pdfium-tesseract-v1"
	DocumentOriginalMaxBytes   = 10 << 20
	DocumentResultMaxBytes     = 1 << 20
	DocumentComputeBudget      = 120 * time.Second
	DocumentCleanupBudget      = 60 * time.Second
	DocumentWholeBudget        = DocumentComputeBudget + DocumentCleanupBudget
)

// Controller-derived ownership. Document allocations are neither check
// commands nor agent sessions; their scope and original cannot cross-adopt.
type DocumentExtractionIdentity struct {
	Protocol       int    `json:"protocol"`
	Profile        string `json:"profile"`
	EngineID       string `json:"engineId"`
	InstanceID     string `json:"instanceId"`
	AllocationID   string `json:"allocationId"`
	ScopeDigest    string `json:"scopeDigest"`
	ActorDigest    string `json:"actorDigest"`
	ProjectID      string `json:"projectId"`
	DocumentID     string `json:"documentId"`
	AttemptID      string `json:"attemptId"`
	OriginalDigest string `json:"originalDigest"`
	CommandDigest  string `json:"commandDigest"`
}

// ImageID and OriginalPath are supplied by the trusted local configuration
// and product's private artifact resolver, never request argv or URLs.
type DocumentExtractionSpec struct {
	Identity        DocumentExtractionIdentity
	ImageID         string
	OriginalPath    string
	OriginalBytes   int
	MimeType        string
	UserID, GroupID int // Trusted nonroot identity, derived from private original ownership.
}
type PreparedDocumentExtraction struct {
	Spec                  DocumentExtractionSpec
	ImageVirtualSizeBytes uint64
	StorageDriver         string
}
type DocumentExtractionObservation struct {
	Ref       string
	State     string // created, running, exited, absent; unknown is an error.
	ExitCode  *int
	OOMKilled bool // True only when returned by exact owned container inspection.
}
type DocumentExtractionOutput struct {
	Data   []byte
	Digest string
}
type DocumentExtractionResultStream interface {
	// EOF and complete framing are data facts, never exit/retirement proof.
	Capture(context.Context) (DocumentExtractionOutput, error)
	Close()
}
type DocumentExtractionFenceRequest struct {
	Identity            DocumentExtractionIdentity
	IntentID            string
	ExpectedOriginalRef string
}
type DocumentExtractionFenceProof struct {
	Identity       DocumentExtractionIdentity `json:"identity"`
	IntentID       string                     `json:"intentId"`
	FenceRef       string                     `json:"fenceRef"`
	ConfigDigest   string                     `json:"configDigest"`
	AcknowledgedAt string                     `json:"acknowledgedAt"`
	ObservedAt     string                     `json:"observedAt"`
}

// Caller journals every before/after RPC under the shared lifecycle lock and
// retains shared capacity until exact positive absence/fence acknowledgment.
// No method pulls/builds an image, retries Start, or adopts a named container.
type DocumentExtractionRetirementExecutor interface {
	InspectDocument(context.Context, DocumentExtractionIdentity, string) (DocumentExtractionObservation, error)
	RetireDocument(context.Context, DocumentExtractionIdentity, string) error
	FenceDocumentLaunch(context.Context, DocumentExtractionFenceRequest) (DocumentExtractionFenceProof, error)
	VerifyDocumentLaunchFence(context.Context, DocumentExtractionFenceRequest, DocumentExtractionFenceProof) error
}

type DocumentExtractionExecutor interface {
	DocumentExtractionRetirementExecutor
	DocumentImageID() string // Trusted immutable configuration; no lookup/pull.
	PrepareDocument(context.Context, DocumentExtractionSpec) (PreparedDocumentExtraction, error)
	CreateDocument(context.Context, PreparedDocumentExtraction) (string, error)
	StartDocument(context.Context, DocumentExtractionIdentity, string) error
	AttachDocumentResult(context.Context, DocumentExtractionIdentity, string) (DocumentExtractionResultStream, error)
}

var documentLogicalID = regexp.MustCompile(`^[a-z0-9_-]{1,200}$`)
var documentArtifactID = regexp.MustCompile(`^doc-[a-f0-9]{64}$`)

func documentHex(value string) bool {
	decoded, err := hex.DecodeString(value)
	return err == nil && len(decoded) == 32 && strings.ToLower(value) == value
}
func ValidateDocumentExtractionIdentity(value DocumentExtractionIdentity, prepared bool) bool {
	if value.Protocol != DocumentExtractionProtocol || value.Profile != DocumentExtractionProfile || !documentLogicalID.MatchString(value.AllocationID) || !documentLogicalID.MatchString(value.ProjectID) || !documentArtifactID.MatchString(value.DocumentID) || !strings.HasPrefix(value.AttemptID, "extract-") || !documentLogicalID.MatchString(value.AttemptID) || !documentHex(value.ScopeDigest) || !documentHex(value.ActorDigest) || !documentHex(value.OriginalDigest) || !documentHex(value.CommandDigest) {
		return false
	}
	if !prepared && value.EngineID == "" && value.InstanceID == "" {
		return true
	}
	return value.EngineID != "" && len(value.EngineID) <= 200 && strings.TrimSpace(value.EngineID) == value.EngineID && !strings.ContainsAny(value.EngineID, "\x00\r\n") && ValidateInstance(value.InstanceID) == nil
}
func DocumentExtractionResources() Resources { return Resources{MemoryMiB: 512, CPUMillis: 1000} }
func DocumentExtractionActorDigest(userID, teamID, projectID string, local bool) string {
	data, _ := json.Marshal(struct {
		UserID, TeamID, ProjectID string
		Local                     bool
	}{userID, teamID, projectID, local})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func DocumentExtractionCommandDigest(spec DocumentExtractionSpec) string {
	spec.Identity.EngineID = ""
	spec.Identity.InstanceID = ""
	spec.Identity.CommandDigest = ""
	data, err := json.Marshal(spec)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
func ValidateDocumentExtractionSpec(spec DocumentExtractionSpec) bool {
	if spec.UserID < 1 || spec.GroupID < 1 || int64(spec.UserID) > 2147483647 || int64(spec.GroupID) > 2147483647 {
		return false
	}
	return ValidateDocumentExtractionIdentity(spec.Identity, false) && strings.HasPrefix(spec.ImageID, "sha256:") && documentHex(strings.TrimPrefix(spec.ImageID, "sha256:")) && filepath.IsAbs(spec.OriginalPath) && filepath.Clean(spec.OriginalPath) == spec.OriginalPath && !strings.ContainsRune(spec.OriginalPath, 0) && spec.OriginalBytes > 0 && spec.OriginalBytes <= DocumentOriginalMaxBytes && (spec.MimeType == "application/pdf" || spec.MimeType == "image/png" || spec.MimeType == "image/jpeg") && spec.Identity.CommandDigest == DocumentExtractionCommandDigest(spec)
}

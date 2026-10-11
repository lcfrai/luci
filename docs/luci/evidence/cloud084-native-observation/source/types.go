// Package e2bbe implements the owned allocation boundary for a pinned E2B Embed
// deployment. It is not selected by Luci's product wiring yet. Allocation is not
// upstream task execution, verified output, deployment, or a billing guarantee.
package e2bbe

import (
	"context"
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"time"
)

const RuntimeRevision = "cf536c284434fdd2f09bbab35ec807a5dacc87e4"

// EnvdVersion is the runtime pin's packages/envd/pkg/version.go value, not a
// minimum feature version inferred from the JavaScript SDK.
const EnvdVersion = "0.9.0"
const maxRecords = 4096
const maxResponse = 4 << 20

var (
	ErrUnsupported = errors.New("E2B Embed capability is not configured or supported")
	ErrConflict    = errors.New("microVM attempt or ownership changed")
	ErrUnknown     = errors.New("microVM outcome is unconfirmed; reservation remains held")
	ErrCapacity    = errors.New("microVM admission limits would be exceeded")
	ErrClosed      = errors.New("microVM controller is closed")
)

// Identity separates person/instance ownership, project, task, attempt and
// operation. A changed context requires a new reviewed attempt, never replay.
type Identity struct {
	OwnerID     string `json:"ownerId"`
	ProjectID   string `json:"projectId"`
	TaskID      string `json:"taskId"`
	AttemptID   string `json:"attemptId"`
	OperationID string `json:"operationId"`
}
type Request struct {
	Identity            Identity `json:"identity"`
	SourceDigest        string   `json:"sourceDigest"`
	ContextDigest       string   `json:"contextDigest"`
	BundleDigest        string   `json:"bundleDigest"`
	MaxSeconds          int      `json:"maxSeconds"`
	EstimatedCostMicros int64    `json:"estimatedCostMicros"`
}

// Deployment records an operator-verified boundary, not a provider claim made
// from a Terraform recipe. Scope is stable across host replacement; generation
// changes block new allocation until prior uncertain work is reconciled.
type Deployment struct {
	Provider                string `json:"provider"`
	Region                  string `json:"region"`
	InstanceType            string `json:"instanceType"`
	Scope                   string `json:"scope"`
	HostGeneration          string `json:"hostGeneration"`
	RuntimeRevision         string `json:"runtimeRevision"`
	ManagementReceiptDigest string `json:"managementReceiptDigest"`
	RestoreReceiptDigest    string `json:"restoreReceiptDigest"`
}
type Template struct {
	ID          string `json:"id"`
	BuildDigest string `json:"buildDigest"`
	MemoryMiB   int64  `json:"memoryMiB"`
	CPUMillis   int64  `json:"cpuMillis"`
}
type Limits struct {
	MaxConcurrent       int
	MaxPerProject       int
	MemoryMiB           int64
	CPUMillis           int64
	EstimatedCostMicros int64
	MaxTaskSeconds      int
}

type State string

const (
	Creating   State = "creating"
	Allocated  State = "allocated"
	Unknown    State = "unknown"
	Cancelling State = "cancelling"
	Absent     State = "absent"
)

type Record struct {
	Request         Request    `json:"request"`
	RequestDigest   string     `json:"requestDigest"`
	Deployment      Deployment `json:"deployment"`
	Template        Template   `json:"template"`
	State           State      `json:"state"`
	SandboxID       string     `json:"sandboxId,omitempty"`
	Revision        int        `json:"revision"`
	CancelRequested bool       `json:"cancelRequested"`
	ResourcesHeld   bool       `json:"resourcesHeld"`
	// ProviderBillingSettled is intentionally never inferred from VM absence.
	ProviderBillingSettled bool      `json:"providerBillingSettled"`
	CreatedAt              time.Time `json:"createdAt"`
	UpdatedAt              time.Time `json:"updatedAt"`
	// LaunchRequested is a durable no-replay fence, not a successful Start.
	LaunchRequested bool   `json:"launchRequested,omitempty"`
	LaunchDigest    string `json:"launchDigest,omitempty"`
	ProcessPID      uint32 `json:"processPid,omitempty"`
	// Protected launch readback and native process outcome remain independent
	// from artifact capture, reviewed integration and exact VM deletion.
	LaunchKernel CaptureIdentity   `json:"launchKernel,omitzero"`
	NativeExit   NativeExitReceipt `json:"nativeExit,omitzero"`
}

// Checkpoint supplies an exclusive fenced journal writer. Production wiring
// must verify the store is outside the replaceable runtime host and can restore.
type Checkpoint interface {
	Claim(ctx context.Context, claim CheckpointClaim) (CheckpointWriter, error)
}
type CheckpointClaim struct {
	OwnerID       string
	Scope         string
	JournalDigest string
	HasJournal    bool
}

// CheckpointWriter must durably acknowledge the exact complete journal before
// provider mutation. A released/stale writer must never publish another head.
type CheckpointWriter interface {
	Commit(ctx context.Context, digest string, journal []byte) (acknowledgedDigest string, err error)
	Close() error
}
type Config struct {
	Endpoint   string
	APIKey     string
	RootCAs    *x509.CertPool // nil uses system roots; verification cannot be disabled
	StateDir   string
	OwnerID    string
	Deployment Deployment
	Template   Template
	Limits     Limits
	Checkpoint Checkpoint
	// Authorize rechecks current authority/cost policy before each provider effect.
	// Secrets never enter a Request or the persisted journal.
	Authorize func(context.Context, Request, string) error
	// GuestOrigins is trusted operator configuration for the exact template.
	// Provider response URLs and task archives cannot select these origins.
	GuestOrigins func(sandboxID string) (envdOrigin, sandboxOrigin, envdVersion string, err error)
}

type Usage struct {
	Active              int   `json:"active"`
	MemoryMiB           int64 `json:"memoryMiB"`
	CPUMillis           int64 `json:"cpuMillis"`
	EstimatedCostMicros int64 `json:"estimatedCostMicros"`
}

var idPattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_-]{0,127}$`)
var digestPattern = regexp.MustCompile(`^[a-f0-9]{64}$`)

func validID(s string) bool { return idPattern.MatchString(s) }
func digest(v any) string {
	b, _ := json.Marshal(v)
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func bytesDigest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }
func validRequest(r Request) bool {
	i := r.Identity
	return validID(i.OwnerID) && validID(i.ProjectID) && validID(i.TaskID) && validID(i.AttemptID) && validID(i.OperationID) && digestPattern.MatchString(r.SourceDigest) && digestPattern.MatchString(r.ContextDigest) && digestPattern.MatchString(r.BundleDigest) && r.MaxSeconds > 0 && r.MaxSeconds <= 86400 && r.EstimatedCostMicros > 0 && r.EstimatedCostMicros <= 1<<50
}

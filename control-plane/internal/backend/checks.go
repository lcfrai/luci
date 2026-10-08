package backend

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
)

const CheckExecutionProtocol = 1

// CheckIdentity is controller-derived private ownership, not a browser-supplied
// Docker ID. Check containers are never managed agent/session containers.
type CheckIdentity struct {
	Protocol      int    `json:"protocol"`
	EngineID      string `json:"engineId"`
	InstanceID    string `json:"instanceId"`
	AllocationID  string `json:"allocationId"`
	CheckID       string `json:"checkId"`
	CommandIndex  int    `json:"commandIndex"`
	ScopeDigest   string `json:"scopeDigest"`
	ProjectID     string `json:"projectId"`
	CommandDigest string `json:"commandDigest"`
}

// CheckSpec is already approved and resolved by the product/API boundary.
// PrepareCheck resolves the configured image without pulling or allocating.
type CheckSpec struct {
	Identity        CheckIdentity
	Executable      string
	Args            []string
	SourceDirectory string
	SourceDigest    string
	Resources       Resources
	UserID, GroupID int
}

type PreparedCheck struct {
	Spec                  CheckSpec
	ImageID               string
	ImageVirtualSizeBytes uint64
	StorageDriver         string
}

type CheckObservation struct {
	Ref string
	// created, running, exited, or absent. Absence is valid only for an exact
	// acknowledged immutable reference; it never proves an uncertain Create.
	State    string
	ExitCode *int
}

type CheckOutput struct {
	Text      string
	Truncated bool
}

type CheckFenceRequest struct {
	Identity            CheckIdentity
	IntentID            string
	ExpectedOriginalRef string
}

type CheckFenceProof struct {
	Identity       CheckIdentity `json:"identity"`
	IntentID       string        `json:"intentId"`
	FenceRef       string        `json:"fenceRef"`
	ConfigDigest   string        `json:"configDigest"`
	AcknowledgedAt string        `json:"acknowledgedAt"`
	ObservedAt     string        `json:"observedAt"`
}

// The coordinator durably records each before/after RPC phase under a
// cross-process lifecycle lock. This interface never starts a hidden retry.
// RemoveCheck uses an acknowledged immutable reference and exact ownership;
// timeout, transport failure and ambiguous NotFound cannot release capacity.
type CheckExecutor interface {
	PrepareCheck(context.Context, CheckSpec) (PreparedCheck, error)
	CreateCheck(context.Context, PreparedCheck) (string, error)
	StartCheck(context.Context, CheckIdentity, string) error
	InspectCheck(context.Context, CheckIdentity, string) (CheckObservation, error)
	CheckLogs(context.Context, CheckIdentity, string, int) (CheckOutput, error)
	RemoveCheck(context.Context, CheckIdentity, string, bool) error
	FenceCheckLaunch(context.Context, CheckFenceRequest) (CheckFenceProof, error)
	VerifyCheckLaunchFence(context.Context, CheckFenceRequest, CheckFenceProof) error
}

// CheckCommandDigest binds the approved argv/source/limits and exact command
// identity. Engine/instance are captured by PrepareCheck and are independently
// pinned in the durable lease; the digest cannot depend on its own value.
func CheckCommandDigest(spec CheckSpec) string {
	spec.Identity.EngineID = ""
	spec.Identity.InstanceID = ""
	spec.Identity.CommandDigest = ""
	spec.Args = append([]string{}, spec.Args...)
	data, err := json.Marshal(spec)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

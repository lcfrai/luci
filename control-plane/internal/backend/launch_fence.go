package backend

import (
	"context"
	"errors"
)

const (
	LaunchFenceProtocol = 1
	MaxLaunchFences     = 64
	LabelLaunchFence    = "luci.launch-fence"
)

var (
	ErrLaunchFenced         = errors.New("the interrupted launch has a retained recovery fence")
	ErrLaunchFencePending   = errors.New("the launch recovery fence is not yet confirmed")
	ErrLaunchFenceOwnership = errors.New("launch recovery ownership could not be verified")
)

// LaunchFenceIdentity is captured before a new guarded launch. A current binary
// must never manufacture it for an older uncertain allocation. The API also
// holds its cross-process child lifecycle lock through Create and cleanup.
type LaunchFenceIdentity struct {
	Protocol   int    `json:"protocol"`
	EngineID   string `json:"engineId"`
	InstanceID string `json:"instanceId"`
}

// LaunchFenceRequest is resolved from private reviewed receipts, not supplied
// as arbitrary container names or references by a client. ExpectedOriginalRef
// is mandatory whenever a previous allocation recorded its immutable ID.
type LaunchFenceRequest struct {
	EngineID            string `json:"engineId"`
	InstanceID          string `json:"instanceId"`
	SandboxID           string `json:"sandboxId"`
	SessionID           string `json:"sessionId"`
	IntentID            string `json:"intentId"`
	ScopeDigest         string `json:"scopeDigest"`
	OperationDigest     string `json:"operationDigest"`
	ExpectedOriginalRef string `json:"expectedOriginalRef,omitempty"`
}

// LaunchFenceProof is deliberately comparable. AcknowledgedAt records a
// completed successful Create response; inspection of an unacknowledged fence
// is not interchangeable proof because daemon error cleanup may still follow.
type LaunchFenceProof struct {
	Protocol        int    `json:"protocol"`
	EngineID        string `json:"engineId"`
	InstanceID      string `json:"instanceId"`
	SandboxID       string `json:"sandboxId"`
	SessionID       string `json:"sessionId"`
	IntentID        string `json:"intentId"`
	ScopeDigest     string `json:"scopeDigest"`
	OperationDigest string `json:"operationDigest"`
	FenceRef        string `json:"fenceRef"`
	ConfigDigest    string `json:"configDigest"`
	AcknowledgedAt  string `json:"acknowledgedAt"`
	ObservedAt      string `json:"observedAt"`
}

// LaunchFencer is optional. Recovery requires current control authority, a
// durable exact intent and permanent host slot, plus exclusive ownership of
// the child lifecycle lock BEFORE calling it. No method starts an agent.
type LaunchFencer interface {
	LaunchFenceIdentity(context.Context) (LaunchFenceIdentity, error)
	FenceLaunch(context.Context, LaunchFenceRequest) (LaunchFenceProof, error)
	VerifyLaunchFence(context.Context, LaunchFenceRequest, LaunchFenceProof) error
}

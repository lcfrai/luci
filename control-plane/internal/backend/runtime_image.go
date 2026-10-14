package backend

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/runtimeoci"
)

const RuntimeImageExecutionProtocol = 1
const RuntimeImageRequiredDiskBytes uint64 = 32 << 30

// RuntimeImageIdentity names trusted installer work, not a project preview or
// a browser-selected Docker target. Prepare supplies the engine, instance and
// source digest when omitted; supplied values must match its observations.
type RuntimeImageIdentity struct {
	Protocol     int    `json:"protocol"`
	EngineID     string `json:"engineId"`
	InstanceID   string `json:"instanceId"`
	AllocationID string `json:"allocationId"`
	JobID        string `json:"jobId"`
	ScopeDigest  string `json:"scopeDigest"`
	SourceDigest string `json:"sourceDigest"`
}

type RuntimeImageSpec struct {
	Identity        RuntimeImageIdentity
	Resources       Resources
	SourceDirectory string
	ImageTag        string
}

type PreparedRuntimeImage struct {
	Spec              RuntimeImageSpec
	PreparedContext   []byte
	ContextDigest     string
	RequiredDiskBytes uint64
	// AdmissionMemoryMiB includes a conservative cold-base or packaged-load
	// daemon allowance. It is host/engine admission, not a daemon memory cap;
	// any Docker RUN hard cap remains Spec.Resources.MemoryMiB.
	AdmissionMemoryMiB int
	StorageDriver      string
	BaseImages         []string
	// Bundle holds the exact verified packaged tar descriptor for the ordinary
	// installed path. It is never serialized into the durable lease.
	Bundle *runtimeoci.Bundle
}

type RuntimeImageBuildResult struct {
	ImageID string
	// Settled requires complete terminal transport, confirmed absence of this
	// allocation's intermediate containers and verified immutable output when
	// successful. Cancellation or ambiguous transport is never settlement.
	Settled bool
}

type RuntimeImagePublishResult struct {
	ImageID      string
	ImageTag     string
	Acknowledged bool
}

type RuntimeImageObservation struct {
	ImageID  string
	ImageTag string
	// published, image-absent, tag-absent or tag-different. This read-only
	// observation never establishes settlement of an interrupted build.
	Publication string
}

// RuntimeImageEngineState is a read-only observation of the selected local
// Docker engine's lifetime. Stopped must be backed by the platform's actual
// engine host boundary, not just a failed Docker request. An engine-wide stop
// after the build began retires its classic builder, which has no build ID.
type RuntimeImageEngineState struct {
	State    string // running or stopped
	EngineID string // current engine ID when running
}

type RuntimeImageRetiredOutput struct {
	ImageID     string
	Publication string // published, tag-absent or tag-different
}

// RuntimeImageEngineRetirementExecutor supports an explicit maintenance
// recovery for a lost classic-build stream. The coordinator durably records
// running -> stopped -> running in order before accepting exact result readback.
// This interface never restarts the engine or modifies unrelated workloads.
type RuntimeImageEngineRetirementExecutor interface {
	ObserveRuntimeImageEngine(context.Context) (RuntimeImageEngineState, error)
	ObserveRetiredRuntimeImage(context.Context, RuntimeImageIdentity, string) (RuntimeImageRetiredOutput, error)
}

// Prepare is bounded and read-only. The trusted coordinator must durably reserve
// memory, CPU and RequiredDiskBytes before Build, retain uncertain reservations,
// and reauthorize/persist publication intent before Publish. No mutating RPC is
// retried here. The coordinator serializes Luci publication by engine/tag.
// Docker ImageTag lacks compare-and-swap: refusing observed differing tags does
// not provide atomic protection against other privileged daemon clients.
type RuntimeImageExecutor interface {
	PrepareRuntimeImage(context.Context, RuntimeImageSpec) (PreparedRuntimeImage, error)
	BuildRuntimeImage(context.Context, PreparedRuntimeImage) (RuntimeImageBuildResult, error)
	PublishRuntimeImage(context.Context, PreparedRuntimeImage, string) (RuntimeImagePublishResult, error)
	ObserveRuntimeImage(context.Context, RuntimeImageIdentity, string, string) (RuntimeImageObservation, error)
}

// A staged installer rechecks current authority and whole-machine capacity
// after acknowledged base pulls and immediately before its first build RPC.
// A callback refusal has a positive no-build result; an ambiguous pull does not.
type RuntimeImagePreBuildExecutor interface {
	RuntimeImageExecutor
	BuildRuntimeImageWithAdmission(context.Context, PreparedRuntimeImage, func(context.Context) error) (RuntimeImageBuildResult, error)
}

// RuntimeImageImportExecutor is the ordinary low-memory installed path. Each
// method is a separate daemon effect with its own durable coordinator phase.
// The caller closes prepared.Bundle after all effects/settlement handling.
type RuntimeImageImportExecutor interface {
	RuntimeImageExecutor
	PreparePackagedRuntimeImage(context.Context, RuntimeImageSpec) (PreparedRuntimeImage, error)
	LoadPackagedRuntimeImage(context.Context, PreparedRuntimeImage) (RuntimeImageBuildResult, error)
	DerivePackagedRuntimeImage(context.Context, PreparedRuntimeImage) (RuntimeImageBuildResult, error)
}

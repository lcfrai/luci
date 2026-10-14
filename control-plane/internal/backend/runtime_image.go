package backend

import "context"

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
	// AdmissionMemoryMiB includes a conservative cold-base/daemon allowance.
	// The Docker RUN hard cap remains Spec.Resources.MemoryMiB.
	AdmissionMemoryMiB int
	StorageDriver      string
	BaseImages         []string
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

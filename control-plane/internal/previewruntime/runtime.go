// Package previewruntime defines the shared, admitted local application runtime.
// Product and platform own reviewed source and durable release receipts; the API
// coordinator owns fresh authority, host admission and exact allocation leases.
package previewruntime

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

const ReviewedSourceKind = "reviewed-source"

type Identity struct {
	Kind              string `json:"kind,omitempty"`
	SnapshotDigest    string `json:"snapshotDigest,omitempty"`
	ProjectID         string `json:"projectId"`
	ReleaseID         string `json:"releaseId"`
	Target            string `json:"target"`
	Commit            string `json:"commit"`
	SourceDigest      string `json:"sourceDigest"`
	BuildAllocationID string `json:"buildAllocationId,omitempty"`
	AllocationID      string `json:"allocationId,omitempty"`
	EngineID          string `json:"engineId,omitempty"`
	InstanceID        string `json:"instanceId,omitempty"`
	ContainerID       string `json:"containerId,omitempty"`
	Image             string `json:"image,omitempty"`
}

type Request struct {
	// Fixed only by the controller-owned model-sink recipe, never browser input.
	Platform string
	Identity
	SourceDirectory string
	SourceArchive   []byte
	Dockerfile      string
	Port            int
	Environment     map[string]string
	// Network is a pre-existing, server-selected owned network; no client input.
	Network string
	// Labels retain historical platform/proxy ownership in addition to the
	// mandatory runtime allocation labels. They cannot override runtime labels.
	Labels               map[string]string
	Actor                management.ExecutionActorRef
	MemoryMiB, CPUMillis int
	// Rechecked immediately before each independent allocating side effect.
	ValidateSource func(context.Context) error
}

// Publication is assembled only from reviewed private registry setup. The
// binding identifies the descriptor/revision; it must not be a credential hash.
type Publication struct {
	Repository      string
	RegistryBinding string
	RegistryAuth    string
}

type Outcome struct {
	Identity
	Phase            string `json:"phase"`
	ImmutableImage   string `json:"immutableImage,omitempty"`
	URL              string `json:"url,omitempty"`
	ResourcesHeld    bool   `json:"resourcesHeld"`
	CleanupConfirmed bool   `json:"cleanupConfirmed"`
	ReasonCode       string `json:"reasonCode,omitempty"`
	Message          string `json:"message,omitempty"`
}

// Publish runs synchronously before every external mutation. Publication
// failure stops the operation and retains existing leases. Start is never
// replayed from a saved receipt. Observe is read-only; Stop reauthorizes the
// current caller and settles only exact owned allocation absence.
type Runtime struct {
	CaptureActor    func(context.Context, string) (management.ExecutionActorRef, error)
	BuildAndPublish func(context.Context, Request, Publication, func(Outcome) error) (Outcome, error)
	Start           func(context.Context, Request, func(Outcome) error) (Outcome, error)
	Observe         func(context.Context, Identity) (Outcome, error)
	Stop            func(context.Context, Identity, func(Outcome) error) (Outcome, error)
	Restart         func(context.Context, Request, func(Outcome) error) (Outcome, error)
}

func (i Identity) Retained() bool { return i.BuildAllocationID != "" || i.AllocationID != "" }

// Continues permits acknowledgement of an identity that was previously
// unknown, but never replacement of an already recorded allocation or object.
func Continues(before, after Identity) bool {
	if before.Kind == ReviewedSourceKind && (before.BuildAllocationID != "" || after.BuildAllocationID != "") || before.Kind == "" && after.SnapshotDigest != "" {
		return false
	}
	if before.Kind != after.Kind || before.ProjectID != after.ProjectID || before.ReleaseID != after.ReleaseID || before.Target != after.Target || before.Commit != after.Commit || before.SourceDigest != after.SourceDigest {
		return false
	}
	for _, pair := range [][2]string{{before.SnapshotDigest, after.SnapshotDigest}, {before.BuildAllocationID, after.BuildAllocationID}, {before.AllocationID, after.AllocationID}, {before.EngineID, after.EngineID}, {before.InstanceID, after.InstanceID}, {before.ContainerID, after.ContainerID}, {before.Image, after.Image}} {
		if pair[0] != "" && pair[0] != pair[1] {
			return false
		}
	}
	return true
}

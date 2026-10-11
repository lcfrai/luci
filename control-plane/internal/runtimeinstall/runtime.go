// Package runtimeinstall describes trusted runtime preparation independently of
// project deployment. Callers supply bundled source, never browser build options.
package runtimeinstall

import (
	"context"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

type Identity struct {
	JobID        string `json:"jobId"`
	AllocationID string `json:"allocationId,omitempty"`
	EngineID     string `json:"engineId,omitempty"`
	InstanceID   string `json:"instanceId,omitempty"`
	SourceDigest string `json:"sourceDigest,omitempty"`
	ImageID      string `json:"imageId,omitempty"`
	ImageTag     string `json:"imageTag,omitempty"`
}

type Request struct {
	Identity
	SourceDirectory string
	Actor           management.ExecutionActorRef
}

type Outcome struct {
	Identity
	Phase            string                `json:"phase"`
	ResourcesHeld    bool                  `json:"resourcesHeld"`
	CleanupConfirmed bool                  `json:"cleanupConfirmed"`
	ReasonCode       string                `json:"reasonCode,omitempty"`
	Message          string                `json:"message"`
	Admission        *AdmissionObservation `json:"admission,omitempty"`
}

// AdmissionObservation is a bounded projection of the actual refusing sample,
// not a later refresh, a resource reservation or permission to retry. It contains
// no daemon identity, project/container list, paths, commands or credentials.
type AdmissionObservation struct {
	Stage                string    `json:"stage"`
	ReasonCode           string    `json:"reasonCode"`
	DecidedAt            time.Time `json:"decidedAt"`
	SamplerAvailable     bool      `json:"samplerAvailable"`
	HostObservedAt       time.Time `json:"hostObservedAt,omitzero"`
	ExecutionObservedAt  time.Time `json:"executionObservedAt,omitzero"`
	HostFresh            bool      `json:"hostFresh"`
	ExecutionFresh       bool      `json:"executionFresh"`
	HostMemoryKnown      bool      `json:"hostMemoryKnown"`
	HostCPUKnown         bool      `json:"hostCPUKnown"`
	ExecutionMemoryKnown bool      `json:"executionMemoryKnown"`
	ExecutionCPUKnown    bool      `json:"executionCPUKnown"`
	InventoryComplete    bool      `json:"inventoryComplete"`
	EngineMatched        bool      `json:"engineMatched"`
	// Metric and operands are present only for the CPU/memory predicate that
	// actually refused admission; unavailable observations remain unavailable.
	Metric           string    `json:"metric,omitempty"`
	MetricObservedAt time.Time `json:"metricObservedAt,omitzero"`
	ObservedBytes    *uint64   `json:"observedBytes,omitempty"`
	RequiredBytes    *uint64   `json:"requiredBytes,omitempty"`
	ObservedPercent  *float64  `json:"observedPercent,omitempty"`
	MaximumPercent   *float64  `json:"maximumPercent,omitempty"`
}

// Publish must durably record the allocation/phase before the coordinator can
// issue its corresponding mutation. Cancellation is a request, not settlement
// evidence. An uncertain result keeps ResourcesHeld true and prevents replay.
type Publish func(Outcome) error

type Runtime struct {
	// CaptureActor checks current backing-management access and retains only an
	// identity reference; Run/Reconcile revalidate fresh authority as required.
	CaptureActor func(context.Context) (management.ExecutionActorRef, error)
	Run          func(context.Context, Request, Publish) (Outcome, error)
	// Reconcile observes only the exact retained operation. It must not rebuild
	// or publish/tag an image automatically after interruption or cancellation.
	Reconcile func(context.Context, Identity, management.ExecutionActorRef, Publish) (Outcome, error)
}

func (v Identity) Retained() bool { return v.AllocationID != "" }

// SameOperation prevents delayed results or a substituted backend identity from
// replacing the current job. Identity fields may become known once, not change.
func SameOperation(before, after Identity) bool {
	if before.JobID == "" || before.JobID != after.JobID || before.ImageTag != after.ImageTag {
		return false
	}
	for _, pair := range [][2]string{{before.AllocationID, after.AllocationID}, {before.EngineID, after.EngineID}, {before.InstanceID, after.InstanceID}, {before.SourceDigest, after.SourceDigest}, {before.ImageID, after.ImageID}} {
		if pair[0] != "" && pair[0] != pair[1] {
			return false
		}
	}
	return true
}

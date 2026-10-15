package product

import (
	"context"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Queue authority is identity, never a bearer token or a cached permission grant.
// The execution service reauthorizes it immediately before admission and launch.
type SwarmAdmissionRequest struct {
	QueueID   string
	ProjectID string
	RunID     string
	Actor     management.ExecutionActorRef
	Policy    SwarmSchedulerPolicy
}
type SwarmAdmissionResult struct {
	State      string `json:"state"` // admitted, waiting, blocked
	ReasonCode string `json:"reasonCode,omitempty"`
	Message    string `json:"message,omitempty"`
	MemoryMiB  int    `json:"memoryMiB"`
	CPUMillis  int    `json:"cpuMillis"`
}
type SwarmDispatchRequest struct {
	QueueID     string
	ProjectID   string
	RunID       string
	OperationID string
	Actor       management.ExecutionActorRef
}
type SwarmDispatchResult struct {
	SessionID  string `json:"sessionId,omitempty"`
	State      string `json:"state"` // running, waiting, blocked, failed, unknown
	ReasonCode string `json:"reasonCode,omitempty"`
	Message    string `json:"message,omitempty"`
}
type SwarmSupersededRequest struct {
	QueueID, ProjectID, RunID, TaskID, OperationID, ChildID string
	Actor                                                   management.ExecutionActorRef
}
type SwarmSchedulerRuntime struct {
	// ObserveTasks records existing terminal task output independently of queue
	// admission. It never starts work or supplies execution authority.
	ObserveTasks func(context.Context) error
	CaptureActor func(context.Context, string) (management.ExecutionActorRef, error)
	Admit        func(context.Context, SwarmAdmissionRequest) (SwarmAdmissionResult, error)
	Dispatch     func(context.Context, SwarmDispatchRequest) (SwarmDispatchResult, error)
	// ResumeCloud may complete only an already-owned, reconciled VM whose
	// upstream launch marker is still absent. It must never allocate a new VM.
	ResumeCloud func(context.Context, SwarmDispatchRequest) (SwarmDispatchResult, error)
	// StopOwned settles only the exact retained attempt after a durable stop
	// intent. False means its child or charge is still live/uncertain.
	StopOwned func(context.Context, SwarmSupersededRequest) (bool, error)
	Release   func(context.Context, string) error
	// CloudRetired proves exact per-task VM absence before a cloud attempt can
	// become terminal and release its scheduler slot. Nil refuses cloud release.
	CloudRetired func(context.Context, string, string, string, string) (bool, error)
	// CloudRecoveryPending reads only the controller's restored-identity fence.
	// It does not inspect a provider during queue reconciliation or GET routes.
	CloudRecoveryPending func(string, string, string, string) bool
	// Resources returns an already-observed, credential-free runtime snapshot.
	// A read must not sample the host, reserve capacity, or reconcile children.
	Resources func(context.Context) any
}
type SwarmSchedulerPolicy struct {
	MaxActive       int `json:"maxActive"`
	MaxPerProject   int `json:"maxPerProject"`
	HeadroomPercent int `json:"headroomPercent"`
	MinHeadroomMiB  int `json:"minHeadroomMiB"`
	// Zero preserves saved percentage-only policies. A positive cap bounds the
	// percentage RAM reserve, never the explicit floor or the CPU margin.
	MaxHeadroomMiB int `json:"maxHeadroomMiB,omitempty"`
	DiskReserveMiB int `json:"diskReserveMiB"`
}

func (p SwarmSchedulerPolicy) MemoryHeadroomBytes(total uint64) uint64 {
	reserve := total * uint64(p.HeadroomPercent) / 100
	if p.MaxHeadroomMiB > 0 {
		reserve = min(reserve, uint64(p.MaxHeadroomMiB)<<20)
	}
	return max(uint64(p.MinHeadroomMiB)<<20, reserve)
}

type SwarmQueueItem struct {
	Revision    int    `json:"revision"`
	ID          string `json:"id"`
	ProjectID   string `json:"projectId"`
	RunID       string `json:"runId"`
	TaskID      string `json:"taskId"`
	TaskTitle   string `json:"taskTitle"`
	Priority    string `json:"priority"`
	State       string `json:"state"`
	QueuedAt    string `json:"queuedAt"`
	UpdatedAt   string `json:"updatedAt"`
	ReasonCode  string `json:"reasonCode,omitempty"`
	Reason      string `json:"reason,omitempty"`
	OperationID string `json:"operationId,omitempty"`
	ChildID     string `json:"childId,omitempty"`
	MemoryMiB   int    `json:"memoryMiB,omitempty"`
	CPUMillis   int    `json:"cpuMillis,omitempty"`
}
type SwarmQueueCounts struct {
	Queued    int `json:"queued"`
	Paused    int `json:"paused"`
	Reserved  int `json:"reserved"`
	Running   int `json:"running"`
	Blocked   int `json:"blocked"`
	Unknown   int `json:"unknown"`
	Completed int `json:"completed"`
	Failed    int `json:"failed"`
	Cancelled int `json:"cancelled"`
}
type ProjectSwarmQueue struct {
	Revision       int              `json:"revision"`
	ObservedAt     string           `json:"observedAt"`
	DispatchPaused bool             `json:"dispatchPaused"`
	Counts         SwarmQueueCounts `json:"counts"`
	Items          []SwarmQueueItem `json:"items"`
	HasMore        bool             `json:"hasMore"`
}
type swarmQueueRecord struct {
	SwarmQueueItem
	Actor          management.ExecutionActorRef `json:"actor"`
	StopActor      management.ExecutionActorRef `json:"stopActor,omitzero"`
	InputDigest    string                       `json:"inputDigest"`
	KeyDigest      string                       `json:"keyDigest"`
	BatchKeyDigest string                       `json:"batchKeyDigest"`
	RequestDigest  string                       `json:"requestDigest"`
	// Internal automatic repair is tied to one retained failed finding and
	// stopped source attempt. These fields never replace its run receipts.
	RepairObligationID      string `json:"repairObligationId,omitempty"`
	RepairSourceOperationID string `json:"repairSourceOperationId,omitempty"`
	RepairReviewDigest      string `json:"repairReviewDigest,omitempty"`
	// A coordinator-created reviewer retry remains bound to one exact saved
	// continuation and repaired candidate through scheduler admission.
	AutoReviewContinuationID string `json:"autoReviewContinuationId,omitempty"`
	AutoReviewActorID        string `json:"autoReviewActorId,omitempty"`
	AutoReviewCandidateID    string `json:"autoReviewCandidateId,omitempty"`
	// An operator pause retains the exact prior non-active state. In particular,
	// repair_checkpoint_pending is an internal pause, not a queued dispatch.
	PausedFromState  string `json:"pausedFromState,omitempty"`
	PausedFromCode   string `json:"pausedFromCode,omitempty"`
	PausedFromReason string `json:"pausedFromReason,omitempty"`
}
type swarmSchedulerLedger struct {
	Version        int                  `json:"version"`
	PolicyRevision int                  `json:"policyRevision"`
	AccountScope   string               `json:"accountScope"`
	Revision       int                  `json:"revision"`
	ObservedAt     string               `json:"observedAt"`
	Paused         bool                 `json:"paused"`
	Policy         SwarmSchedulerPolicy `json:"policy"`
	Sequence       uint64               `json:"sequence"`
	LastServed     map[string]uint64    `json:"lastServed"`
	Items          []swarmQueueRecord   `json:"items"`
}
type swarmScheduler struct {
	mu             sync.Mutex
	tick           sync.Mutex
	runtime        SwarmSchedulerRuntime
	wake           chan struct{}
	cancel         context.CancelFunc
	now            func() time.Time
	cooldowns      map[string]time.Time
	policyRevision int
}

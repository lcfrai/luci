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
	Release     func(context.Context, string) error
	// CloudRetired proves exact per-task VM absence before a cloud attempt can
	// become terminal and release its scheduler slot. Nil refuses cloud release.
	CloudRetired func(context.Context, string, string, string, string) (bool, error)
	// Resources returns an already-observed, credential-free runtime snapshot.
	// A read must not sample the host, reserve capacity, or reconcile children.
	Resources func(context.Context) any
}
type SwarmSchedulerPolicy struct {
	MaxActive       int `json:"maxActive"`
	MaxPerProject   int `json:"maxPerProject"`
	HeadroomPercent int `json:"headroomPercent"`
	MinHeadroomMiB  int `json:"minHeadroomMiB"`
	DiskReserveMiB  int `json:"diskReserveMiB"`
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
	InputDigest    string                       `json:"inputDigest"`
	KeyDigest      string                       `json:"keyDigest"`
	BatchKeyDigest string                       `json:"batchKeyDigest"`
	RequestDigest  string                       `json:"requestDigest"`
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

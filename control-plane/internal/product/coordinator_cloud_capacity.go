package product

import "context"

type CoordinatorCloudAttempt struct {
	RunID               string `json:"runId"`
	TaskID              string `json:"taskId"`
	AttemptID           string `json:"attemptId"`
	OperationID         string `json:"operationId"`
	State               string `json:"state"`
	ResourcesHeld       bool   `json:"resourcesHeld"`
	EstimatedCostMicros int64  `json:"estimatedCostMicros"`
	UpdatedAt           string `json:"updatedAt"`
}

// CoordinatorCloudCapacity is a read-only, nonsecret configuration snapshot.
// It is advice for a person's selected project, never an admission receipt:
// fresh actor policy, provider status, quota and exact allocations are checked
// again before an irreversible Create.
type CoordinatorCloudCapacity struct {
	// SetupNeeds is read-only, actor-scoped advice. An empty Provider means no
	// capacity claim exists; the UI must not offer activation as ready.
	SetupNeeds               []string                  `json:"setupNeeds,omitempty"`
	Provider                 string                    `json:"provider"`
	Region                   string                    `json:"region"`
	ConfiguredGlobalSlots    int                       `json:"configuredGlobalSlots"`
	ConfiguredProjectSlots   int                       `json:"configuredProjectSlots"`
	ConfiguredAvailableSlots int                       `json:"configuredAvailableSlots"`
	EstimatedTaskCostMicros  int64                     `json:"estimatedTaskCostMicros"`
	EstimatedMonthMicros     int64                     `json:"estimatedMonthMicros"`
	MonthlyBudgetMicros      int64                     `json:"monthlyBudgetMicros"`
	MaxTaskSeconds           int                       `json:"maxTaskSeconds"`
	RetainedProjectVMs       int                       `json:"retainedProjectVMs"`
	RetainedAttempts         []CoordinatorCloudAttempt `json:"retainedAttempts"`
	MoreRetainedAttempts     int                       `json:"moreRetainedAttempts"`
	ObservedAt               string                    `json:"observedAt"`
	Limitation               string                    `json:"limitation"`
}

func (s *Server) SetCoordinatorCloudCapacity(read func(context.Context, string, string) (CoordinatorCloudCapacity, error)) {
	s.cloudTaskCapacity = read
}

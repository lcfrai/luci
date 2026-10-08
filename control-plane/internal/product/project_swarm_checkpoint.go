package product

import (
	"context"
	"errors"
	"time"
)

// A checkpoint creates a new attempt. BaseCommit remains the integration base;
// SeedCommit records the explicitly reviewed retained changes already present
// when that new agent starts. It never establishes task acceptance.
type ProjectSwarmCheckpoint struct {
	SourceOperationID string `json:"sourceOperationId"`
	SourceChildID     string `json:"sourceChildId"`
	SourceCommit      string `json:"sourceCommit"`
	SourceBaseCommit  string `json:"sourceBaseCommit"`
	TargetCommit      string `json:"targetCommit"`
	SeedCommit        string `json:"seedCommit,omitempty"`
	ReviewDigest      string `json:"reviewDigest"`
	ContextDigest     string `json:"contextDigest"`
	CreatedAt         string `json:"createdAt"`
}

type ProjectSwarmCheckpointFile struct {
	Path       string `json:"path"`
	Status     string `json:"status"`
	Before     string `json:"before"`
	After      string `json:"after"`
	BeforeMode string `json:"beforeMode"`
	AfterMode  string `json:"afterMode"`
}

type ProjectSwarmCheckpointReview struct {
	ProjectID        string                       `json:"projectId"`
	RunID            string                       `json:"runId"`
	OperationID      string                       `json:"operationId"`
	ChildID          string                       `json:"childId"`
	TaskID           string                       `json:"taskId"`
	ExpectedRevision int                          `json:"expectedRevision"`
	SourceCommit     string                       `json:"sourceCommit"`
	TargetCommit     string                       `json:"targetCommit"`
	ReviewDigest     string                       `json:"reviewDigest"`
	CanContinue      bool                         `json:"canContinue"`
	Reason           string                       `json:"reason"`
	Files            []ProjectSwarmCheckpointFile `json:"files"`
	TotalBytes       int                          `json:"totalBytes"`
}

// The API owns current authority and cross-process child lifecycle exclusion.
// Neither GET nor absence of a lease is proof that an old worker cannot write.
func (s *Server) SetProjectSwarmCheckpointGuard(authorize func(context.Context, ProjectSwarmRecoveryRequest) error, guard func(context.Context, ProjectSwarmRecoveryRequest) (func(), error)) {
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	s.projectSwarmCheckpointAuthorize, s.projectSwarmCheckpointGuard = authorize, guard
}

func swarmStartingCommit(op ProjectRunOperation) string {
	if op.Checkpoint != nil {
		return op.Checkpoint.SeedCommit
	}
	return op.BaseCommit
}

func validSwarmCheckpoint(run ProjectRun, op ProjectRunOperation) bool {
	c := op.Checkpoint
	if c == nil {
		return true
	}
	if c.TargetCommit != op.BaseCommit || !deliverySHA.MatchString(c.TargetCommit) || !deliverySHA.MatchString(c.SourceCommit) || !deliverySHA.MatchString(c.SourceBaseCommit) || !hexDigest(c.ReviewDigest) || !hexDigest(c.ContextDigest) {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, c.CreatedAt); err != nil {
		return false
	}
	source := swarmOperation(&run, c.SourceOperationID)
	if source == nil || source.ID == op.ID || source.ChildID != c.SourceChildID || source.TaskID != op.TaskID || source.BaseCommit != c.SourceBaseCommit || source.Implementation == nil || implementationDigest(source.Implementation) != c.ContextDigest || source.Attempt >= op.Attempt {
		return false
	}
	return (c.SeedCommit == "" && op.Allocation != "ready") || deliverySHA.MatchString(c.SeedCommit) && c.SeedCommit != c.TargetCommit && (op.Allocation != "ready" || op.Implementation != nil && op.Implementation.SourceCommit == c.SeedCommit)
}

// ClaimOperation requires ready allocation and an immutable implementation
// binding. Publishing a seed alone cannot admit any child. This narrow private
// provenance survives a copy interrupted before readiness; missing ledgers do
// not participate in the proof. Only explicit abandonment mints retirement.
func unstartedSwarmCheckpoint(run ProjectRun, op ProjectRunOperation) bool {
	return op.Checkpoint != nil && validSwarmCheckpoint(run, op) &&
		(op.Allocation == "allocating" || op.Allocation == "unknown") && (op.Status == "reserved" || op.Status == "unknown") &&
		op.Implementation == nil && !op.UnclaimedRetirement && op.LaunchRecovery == nil && op.LaunchRecoveryProof == nil && op.AbsenceRetirement == nil && op.Recovery == nil && op.Integration == nil && op.Handoff == nil
}

func sameSwarmCheckpointSelection(a, b *ProjectSwarmCheckpoint) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return a.SourceOperationID == b.SourceOperationID && a.SourceChildID == b.SourceChildID && a.SourceCommit == b.SourceCommit && a.SourceBaseCommit == b.SourceBaseCommit && a.TargetCommit == b.TargetCommit && a.ContextDigest == b.ContextDigest && a.ReviewDigest == b.ReviewDigest
}

func (s *ProjectRunStore) recordSwarmCheckpointSeed(projectID, path, runID string, revision int, operationID, seed string) (ProjectRun, error) {
	if !deliverySHA.MatchString(seed) {
		return ProjectRun{}, ErrProjectRunInvalid
	}
	return s.mutateSwarm(projectID, path, runID, revision, func(run *ProjectRun) error {
		op := swarmOperation(run, operationID)
		if op == nil || !unstartedSwarmCheckpoint(*run, *op) || op.Status != "reserved" || op.Allocation != "allocating" || op.Checkpoint.SeedCommit != "" {
			return ErrProjectRunConflict
		}
		op.Checkpoint.SeedCommit = seed
		return nil
	})
}

func eligibleSwarmCheckpoint(run ProjectRun, op *ProjectRunOperation) error {
	if op == nil || op.TaskID == "" || op.Allocation != "ready" || op.Implementation == nil {
		return errors.New("This attempt has no confirmed source and reviewed context to continue. Inspect its retained output instead.")
	}
	task := swarmTask(&run, op.TaskID)
	if task == nil || task.OperationID != op.ID || task.Integration != nil || op.Integration != nil {
		return errors.New("Continue the current task attempt before replacing it or integrating its result. Older attempts remain available for inspection.")
	}
	if swarmRetirementPending(*op) {
		return errors.New("Finish this attempt's launch recovery before reviewing a checkpoint. Its output is still retained.")
	}
	switch op.Status {
	case "failed", "cancelled", "review-required", "succeeded":
	default:
		return errors.New("Stop and observe this attempt before continuing its retained changes.")
	}
	return swarmTaskReady(run, *task)
}

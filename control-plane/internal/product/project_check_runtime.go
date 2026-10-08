package product

import (
	"context"
	"errors"
	"reflect"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

// ErrProjectCheckSuperseded means another lifecycle owner already changed or
// settled this exact command. The stale controller must stop without publishing
// its cached progress or final run over that owner's durable receipt.
var ErrProjectCheckSuperseded = errors.New("the exact check reservation belongs to a newer outcome")

// Only immutable actor identity survives the POST; every allocation resolves
// fresh policy. SourceDirectory is resolved by product, never a client field.
type ProjectCheckCommandRequest struct {
	ProjectID, CheckID            string
	CommandIndex                  int
	AllocationID                  string
	SourceDirectory, SourceDigest string
	Command                       CheckCommand
	MemoryMiB, CPUMillis          int
	Actor                         management.ExecutionActorRef
}

type ProjectCheckCommandOutcome struct {
	Status           string
	Stage            string
	ReasonCode       string
	Message          string
	AllocationID     string
	ExitCode         *int
	Output           string
	Truncated        bool
	ResourcesHeld    bool
	CleanupConfirmed bool
}

// Persist is invoked BEFORE removing the last container holding command logs.
// A failure retains the object/lease for exact observation and cleanup retry.
// Execute never holds the shared ledger lock while calling Persist.
type ProjectCheckRuntime struct {
	AcknowledgeCompleted func(context.Context, string, string) error
	Observe              func(context.Context) error
	Busy                 func(context.Context, string) (bool, error)
	CaptureActor         func(context.Context, string) (management.ExecutionActorRef, error)
	AuthorizeControl     func(context.Context, string) error
	Execute              func(context.Context, ProjectCheckCommandRequest, func(ProjectCheckCommandOutcome) error) (ProjectCheckCommandOutcome, error)
	Recovery             ProjectCheckRecoveryReader
	Recover              ProjectCheckRecoveryWriter
}

func (s *Server) SetProjectCheckRuntime(value ProjectCheckRuntime) {
	s.projectCheckRuntime = value
	if value.Observe != nil {
		go func() {
			ticker := time.NewTicker(2 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-s.workspaceContext.Done():
					return
				case <-ticker.C:
				}
				bounded, cancel := context.WithTimeout(s.workspaceContext, 3*time.Second)
				_ = value.Observe(bounded)
				cancel()
			}
		}()
	}
}

func validCheckOutcome(value ProjectCheckCommandOutcome) bool {
	if value.AllocationID == "" || len(value.AllocationID) > 200 || len(value.Output) > maxCheckOutput || len(value.Stage) > 80 || len(value.ReasonCode) > 100 || len(value.Message) > 1000 {
		return false
	}
	switch value.Status {
	case "passed":
		return value.ExitCode != nil && *value.ExitCode == 0
	case "failed":
		return value.ExitCode != nil && *value.ExitCode != 0
	case "blocked", "unknown", "cancelled", "running":
		return value.ExitCode == nil
	default:
		return false
	}
}

// Caller is the exact active job, or a bounded trusted reconciliation of its
// private lease. Diagnostic status alone never releases capacity or validates
// check evidence. This method preserves a completed command's actual output
// while a separate cleanup observation remains uncertain.
func (s *Server) recordProjectCheckCommand(job *projectCheckJob, index int, value ProjectCheckCommandOutcome) error {
	if !validCheckOutcome(value) {
		return errors.New("check outcome cannot be verified")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if index < 0 || index >= len(job.run.Commands) || job.run.Commands[index].AllocationID != value.AllocationID {
		return errors.New("check result no longer matches the exact approved command")
	}
	command := &job.run.Commands[index]
	command.Status, command.ExitCode, command.Output, command.Truncated = value.Status, value.ExitCode, value.Output, value.Truncated
	command.Stage, command.ReasonCode, command.Message, command.CleanupConfirmed = value.Stage, value.ReasonCode, value.Message, value.CleanupConfirmed
	held := value.ResourcesHeld
	job.run.ResourcesHeld = &held
	job.run.Stage, job.run.ReasonCode, job.run.Message = value.Stage, value.ReasonCode, value.Message
	job.run.Revision++
	job.run.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	return s.storeCheckRun(job.run)
}

// ProjectCheckObservation loads only the exact private check receipt selected
// by a host lease. It cannot enumerate another workspace or launch a command.
// A live local job retains ownership; background maintenance waits for it.
func (s *Server) ProjectCheckObservation(projectID, checkID, allocationID string) (ProjectCheckRun, int, ProjectCheckCommandOutcome, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if job := s.projectChecks[checkID]; job != nil && job.cancel != nil {
		return ProjectCheckRun{}, 0, ProjectCheckCommandOutcome{}, errors.New("the check is still owned by its active controller")
	}
	rows, err := s.readCheckHistory(projectID)
	if err != nil {
		return ProjectCheckRun{}, 0, ProjectCheckCommandOutcome{}, err
	}
	for _, row := range rows {
		if row.ID == checkID {
			for i, command := range row.Commands {
				if command.AllocationID == allocationID {
					held := row.ResourcesHeld != nil && *row.ResourcesHeld
					out := ProjectCheckCommandOutcome{Status: command.Status, Stage: command.Stage, ReasonCode: command.ReasonCode, Message: command.Message, AllocationID: allocationID, ExitCode: command.ExitCode, Output: command.Output, Truncated: command.Truncated, CleanupConfirmed: command.CleanupConfirmed, ResourcesHeld: held}
					return snapshotProjectCheck(row), i, out, nil
				}
			}
		}
	}
	return ProjectCheckRun{}, 0, ProjectCheckCommandOutcome{}, errors.New("the exact retained check receipt is unavailable")
}

// RecordProjectCheckObservation changes only the captured exact revision and
// command identity. Restart observation cannot overwrite a newer outcome or
// restart any remaining approved command.
func (s *Server) RecordProjectCheckObservation(before ProjectCheckRun, index int, out ProjectCheckCommandOutcome) (ProjectCheckRun, error) {
	if !validCheckOutcome(out) || index < 0 || index >= len(before.Commands) || before.Commands[index].AllocationID != out.AllocationID {
		return ProjectCheckRun{}, errors.New("check observation binding is invalid")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if job := s.projectChecks[before.ID]; job != nil && job.cancel != nil {
		return ProjectCheckRun{}, errors.New("the active check owns its result publication")
	}
	rows, err := s.readCheckHistory(before.ProjectID)
	if err != nil {
		return ProjectCheckRun{}, err
	}
	for _, row := range rows {
		if row.ID == before.ID {
			if row.Revision != before.Revision || row.SourceDigest != before.SourceDigest || index >= len(row.Commands) || row.Commands[index].AllocationID != out.AllocationID {
				return ProjectCheckRun{}, ErrProjectRunConflict
			}
			prior := snapshotProjectCheck(row)
			command := &row.Commands[index]
			command.Status, command.Stage, command.ReasonCode, command.Message = out.Status, out.Stage, out.ReasonCode, out.Message
			command.Output, command.Truncated, command.ExitCode, command.CleanupConfirmed = out.Output, out.Truncated, out.ExitCode, out.CleanupConfirmed
			held := out.ResourcesHeld
			row.ResourcesHeld = &held
			row.Stage, row.ReasonCode, row.Message = out.Stage, out.ReasonCode, out.Message
			row.Status = "unknown"
			if out.CleanupConfirmed && !held {
				row.Status = out.Status
				for i := range row.Commands {
					if i > index {
						row.Commands[i].Status = "skipped"
						if row.Status == "passed" {
							row.Status = "unknown"
							row.Message = "The completed command is recorded. Remaining commands were not replayed after the service restarted."
						}
					}
				}
				// Source freshness is intentionally not manufactured by observation;
				// the evidence reader independently compares the saved source digest.
			}
			if reflect.DeepEqual(row, prior) {
				return snapshotProjectCheck(row), nil
			}
			row.Revision++
			row.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
			if err := s.storeCheckRun(row); err != nil {
				return ProjectCheckRun{}, err
			}
			return snapshotProjectCheck(row), nil
		}
	}
	return ProjectCheckRun{}, errors.New("the retained check was removed")
}

package product

import (
	"context"
	"errors"
)

// Pending launch state belongs to the shared execution ledger, not the public
// operation status. A failed/exited child can still have a delayed Create/Start.
func (s *Server) SetProjectSwarmPendingLaunch(check func(context.Context, string, string, string, string) (bool, error)) {
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	s.projectSwarmPendingLaunch = check
}

// Caller holds dispatch exclusion; the callback releases its own short ledger
// lock before this function returns and before any product mutation starts.
func (s *Server) requireNoSwarmPendingLaunch(ctx context.Context, run ProjectRun, taskID string) error {
	task := swarmTask(&run, taskID)
	if task == nil {
		return ErrProjectRunNotFound
	}
	if task.OperationID == "" {
		return nil
	}
	op := swarmOperation(&run, task.OperationID)
	if op == nil {
		return ErrProjectRunInvalid
	}
	// BeginProjectRunSession changes reserved to dispatching under this same
	// dispatch exclusion before any execution claim or backend request. An
	// untouched reservation therefore has no launch to query or retire.
	if op.Status == "reserved" && op.LaunchRecovery == nil {
		return nil
	}
	if op.UnclaimedRetirement && op.Status == "cancelled" && op.LaunchRecovery == nil && op.LaunchRecoveryProof == nil && op.AbsenceRetirement == nil && op.Integration == nil && task.Integration == nil {
		return nil
	}
	if s.projectSwarmPendingLaunch == nil {
		return errors.New("The execution service cannot currently check this attempt's retained launch. Its identity must remain in place before another attempt is authorized.")
	}
	pending, e := s.projectSwarmPendingLaunch(ctx, run.ProjectID, run.ID, op.ID, op.ChildID)
	if e != nil {
		return e
	}
	if pending {
		return errors.New("This attempt still has an unacknowledged launch or recovery reservation. Review this attempt's recovery before retrying; its identity and output must remain retained.")
	}
	return nil
}

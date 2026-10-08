package product

import (
	"context"
	"errors"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// ObserveTaskCompletion records facts about an already-owned terminal child.
// This service callback is not a user action or execution authority: it cannot
// reserve, launch, accept, integrate or release a task. Registry records are its
// durable work source, so an interrupted callback can be retried after restart.
// A true result means the exact task's canonical receipt was validated (or the
// record is positively identified as a non-task run).
func (s *Server) ObserveTaskCompletion(ctx context.Context, observed registry.Session) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	if observed.Status != registry.StatusExited || observed.ProjectID == "" || observed.ProjectRunID == "" || observed.ProjectRunOperationID == "" {
		return false, ErrProjectRunInvalid
	}
	guard := s.branchGuard(observed.ProjectID)
	if !guard.TryRLock() {
		return false, ErrProjectRunPending
	}
	defer guard.RUnlock()
	if !s.projectRunDispatchMu.TryLock() {
		return false, ErrProjectRunPending
	}
	defer s.projectRunDispatchMu.Unlock()
	p, err := s.resolveProject(observed.ProjectID)
	if err != nil {
		return false, err
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, observed.ProjectRunID)
	if err != nil {
		return false, err
	}
	op := swarmOperation(&run, observed.ProjectRunOperationID)
	if op == nil || op.Kind != "session" || op.ChildID != observed.ID || !projectRunSessionBindingMatches(run, *op, observed) {
		return false, ErrProjectRunInvalid
	}
	if op.TaskID == "" {
		return true, nil
	}
	if run.Swarm == nil || op.Allocation != "ready" || op.Status == "reserved" || swarmRetirementPending(*op) || s.projectRunSessionObserver == nil {
		return false, ErrProjectRunPending
	}
	children, err := s.projectRunSessionObserver(ctx, run.ProjectID, run.ID, op.ID)
	if err != nil {
		return false, err
	}
	if len(children) != 1 || !sameTaskCompletionGeneration(children[0], observed) || !projectRunSessionBindingMatches(run, *op, children[0]) {
		return false, ErrProjectRunConflict
	}
	if !projectRunOperationTerminal(op.Status) {
		status, message := observedSwarmOperation(run, *op, children)
		run, err = s.projectRuns.ObserveOperation(run.ProjectID, p.Path, run.ID, run.Revision, op.ID, op.ChildID, status, message)
		if err != nil {
			return false, err
		}
		op = swarmOperation(&run, observed.ProjectRunOperationID)
	}
	if op == nil || !projectRunOperationTerminal(op.Status) {
		return false, ErrProjectRunPending // disappearance is not an ordinary exit
	}
	if op.Handoff != nil {
		// A previously published receipt can survive a crash before the registry
		// watermark. Re-prove exact stopped ownership on retry; the report
		// itself is never a lifecycle capability. Do not republish its claims.
		if s.taskHandoffCollector == nil {
			return false, ErrProjectRunPending
		}
		req, requestErr := s.taskHandoffRequest(run, *op)
		if requestErr != nil {
			return false, requestErr
		}
		capture, captureErr := s.taskHandoffCollector(ctx, req)
		if captureErr != nil {
			return false, captureErr
		}
		if capture.Release == nil {
			return false, ErrProjectRunPending
		}
		defer capture.Release()
	} else {
		run, err = s.captureTaskHandoff(ctx, p.Path, run, op.ID, false, false)
		if err != nil {
			return false, err
		}
		op = swarmOperation(&run, observed.ProjectRunOperationID)
	}
	if op == nil || op.Handoff == nil {
		return false, errors.New("the stopped task's report has not been retained")
	}
	view, err := s.projectRuns.taskHandoffView(run, *op)
	if err != nil || view.Receipt == nil || view.Receipt.ChildID != observed.ID || view.Digest != op.Handoff.Digest {
		if err == nil {
			err = ErrProjectRunInvalid
		}
		return false, err
	}
	return true, ctx.Err()
}

// Exact generation comparison deliberately excludes attach accounting from the
// request digest but includes UpdatedAt, sandbox ref and status. A delayed
// observation cannot publish facts for a replacement or newly changed record.
func sameTaskCompletionGeneration(a, b registry.Session) bool {
	return a.ID == b.ID && a.ProjectID == b.ProjectID && a.WorktreeID == b.WorktreeID &&
		a.ProjectRunID == b.ProjectRunID && a.ProjectRunOperationID == b.ProjectRunOperationID &&
		a.SandboxID == b.SandboxID && a.SandboxRef == b.SandboxRef && a.Status == b.Status &&
		a.UpdatedAt.Equal(b.UpdatedAt) && ProjectRunSessionRequestDigest(a) == ProjectRunSessionRequestDigest(b)
}

func observedSwarmOperation(run ProjectRun, op ProjectRunOperation, children []registry.Session) (string, string) {
	status, message := "unknown", "The reserved child cannot be identified. Its outcome is unknown; no replacement was launched."
	if len(children) == 1 && children[0].ID == op.ChildID && projectRunSessionBindingMatches(run, op, children[0]) {
		child := children[0]
		status, message = "running", "The existing agent is running in its isolated checkout."
		if child.Status == registry.StatusExited {
			status, message = "review-required", "The agent process ended. Its exit code and task correctness are not established by this observation. Review retained changes and verification evidence before integration."
			if child.ExitReason == "project-run-outcome-unknown" || child.ExitReason == "sandbox disappeared" {
				status, message = "unknown", "The agent runtime disappeared or its launch outcome is unconfirmed. Inspect the retained session; no replacement was launched and no task result was inferred."
			} else if child.ExitReason != "process exited" && child.ExitReason != "exited" && child.ExitReason != "" {
				status, message = "failed", "The agent stopped without a confirmed successful result. Retain and inspect its checkout before retrying."
			}
		}
	}
	return status, message
}

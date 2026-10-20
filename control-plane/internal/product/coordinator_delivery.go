package product

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Integration is authoritative only when every selected task names a finished
// operation and its reviewed integration receipt. Worker claims and board state
// cannot make a run ready for independent checks.
func coordinatorIntegratedRun(run ProjectRun, row coordinatorContinuationRecord) (string, bool) {
	if run.ID != row.RunID || run.ProjectID != row.ProjectID || run.Swarm == nil || run.Swarm.State != "ready" ||
		run.State == "cancelled" || run.State == "complete" || len(run.Swarm.Tasks) != len(row.Run.ImplementationContext.TaskIDs) || len(run.Input.Requirements) == 0 {
		return "", false
	}
	wanted := map[string]bool{}
	for _, id := range row.Run.ImplementationContext.TaskIDs {
		wanted[id] = true
	}
	// A contract-designated reviewer contributes an exact passed independent
	// receipt, not a source integration. The swarm gate verifies its candidate,
	// finding, handoff and repaired successor before combined checks can start.
	if !swarmAllIntegrated(run) {
		return "", false
	}
	for _, task := range run.Swarm.Tasks {
		in := task.Integration
		op := swarmOperation(&run, task.OperationID)
		if !wanted[task.ID] || op == nil || op.TaskID != task.ID || op.Allocation != "ready" {
			return "", false
		}
		if task.ReviewOnly {
			if in != nil || op.Handoff == nil || (op.Status != "succeeded" && op.Status != "review-required") {
				return "", false
			}
		} else if in == nil || in.State != "integrated" || !deliverySHA.MatchString(in.Commit) || !hexDigest(in.ReviewDigest) ||
			(op.Status != "succeeded" && op.Status != "review-required" && op.Status != "failed") {
			return "", false
		}
		delete(wanted, task.ID)
	}
	for _, op := range run.Operations {
		if swarmSlotPending(op) {
			return "", false
		}
	}
	if len(wanted) != 0 {
		return "", false
	}
	return reviewDigest(run.Swarm.Tasks), true
}

// Review-only workers contribute the passed receipt checked by
// coordinatorIntegratedRun, never a source commit. Keep ancestry checks on
// coding integrations so combined checks can follow an independent review.
func coordinatorIntegratedSourceCommits(run ProjectRun) ([]string, error) {
	if run.Swarm == nil {
		return nil, ErrOrchestratorConflict
	}
	commits := make([]string, 0, len(run.Swarm.Tasks))
	for _, task := range run.Swarm.Tasks {
		if task.ReviewOnly {
			if task.Integration != nil {
				return nil, ErrOrchestratorConflict
			}
			continue
		}
		if task.Integration == nil || task.Integration.State != "integrated" || !deliverySHA.MatchString(task.Integration.Commit) {
			return nil, ErrOrchestratorConflict
		}
		commits = append(commits, task.Integration.Commit)
	}
	return commits, nil
}

// Caller holds s.mu. Authority, exact review selection and delegated command
// consent are checked again at reservation, execution and evidence publication.
func (s *Server) coordinatorDeliveryAuthorityLocked(ctx context.Context, p project.Project, row coordinatorContinuationRecord) error {
	if err := s.continuationAuthority(ctx, p, row); err != nil {
		return err
	}
	policy, err := s.readCoordinatorPolicy(p)
	if err != nil {
		return err
	}
	if policy.Revision != row.PolicyRevision || policy.ExecutionMode != "reviewed-tasks" {
		return errCoordinatorPolicy
	}
	current, err := s.continuationContextLocked(p, row.Run.ImplementationContext)
	if err != nil {
		return err
	}
	if current != row.CanonicalDigest {
		return ErrOrchestratorConflict
	}
	saved, err := s.readOrchestrator(p)
	if err != nil {
		return err
	}
	if len(orchestratorQuestions(saved)) != 0 {
		return ErrOrchestratorQuestion
	}
	return s.coordinatorCheckCoverageLocked(p, row.Run.ImplementationContext, row.CheckPlan)
}

// Only an authorized continuation mutation reaches this method. GETs merely
// return the retained receipt. Reservation precedes the one allowed invocation;
// all later calls observe its exact ID, including after process death.
func (s *Server) continueCoordinatorDelivery(ctx context.Context, p project.Project, selected coordinatorContinuationRecord) (coordinatorContinuationRecord, error) {
	if repaired, handled, repairErr := s.continueCoordinatorCheckRepair(ctx, p, selected); handled || repairErr != nil { return repaired, repairErr }
	release, err := s.AcquireGitSessionAdmission(ctx, p.ID)
	if err != nil {
		return selected, errCoordinatorSourceBusy
	}
	defer release()
	s.mu.Lock()
	rows, err := s.readContinuations(p, selected.Actor.ID)
	index := findContinuation(rows, selected.ID)
	if err != nil || index < 0 {
		s.mu.Unlock()
		return selected, ErrOrchestratorInvalid
	}
	row := rows.Rows[index]
	if err = s.coordinatorDeliveryAuthorityLocked(ctx, p, row); err != nil {
		s.mu.Unlock()
		return row, err
	}
	if len(row.Actions) != 3 || row.Actions[2].Status != "confirmed" {
		s.mu.Unlock()
		return row, ErrOrchestratorConflict
	}
	actionID := coordinatorCheckActionID(row.ID, len(row.PriorDeliveries))
	if s.coordinatorActions[actionID] {
		s.mu.Unlock()
		return row, nil
	}
	if s.coordinatorActions == nil {
		s.coordinatorActions = map[string]bool{}
	}
	s.coordinatorActions[actionID] = true
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.coordinatorActions, actionID)
		s.mu.Unlock()
	}()

	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil {
		return row, err
	}
	if err = s.validateProjectSwarmReuseCurrent(ctx, p, run); err != nil {
		return row, err
	}
	integration, ready := coordinatorIntegratedRun(run, row)
	if !ready {
		if row.Delivery != nil {
			return row, ErrOrchestratorConflict
		}
		return row, nil
	}
	if row.CheckPlan == nil {
		return s.persistCoordinatorDelivery(ctx, p, row, "blocked", "All selected tasks are integrated. Save an explicit trusted check command plan to continue independent verification; no checks or delivery have been claimed.")
	}
	bounded, cancel := context.WithTimeout(ctx, 20*time.Second)
	digest, commit, dirty, err := s.checkSource(bounded, p.Path)
	if err == nil && !dirty && deliverySHA.MatchString(commit) && hexDigest(digest) {
		var integrated []string
		integrated, err = coordinatorIntegratedSourceCommits(run)
		if err == nil {
			for _, sourceCommit := range integrated {
				if _, err = s.journeyGit(bounded, p.Path, "merge-base", "--is-ancestor", sourceCommit, commit); err != nil {
					break
				}
			}
		}
	}
	cancel()
	if err != nil || dirty || !deliverySHA.MatchString(commit) || !hexDigest(digest) {
		return row, ErrOrchestratorConflict
	}
	invoke := row.Delivery == nil
	if invoke {
		now := time.Now().UTC().Format(time.RFC3339Nano)
		row.Delivery = &CoordinatorDelivery{ActionID: actionID, CheckID: coordinatorDeliveryCheckID(row.ID, len(row.PriorDeliveries)), Status: "pending", Commit: commit, SourceDigest: digest, IntegrationDigest: integration, StartedAt: now, UpdatedAt: now}
		row, err = s.persistCoordinatorDelivery(ctx, p, row, "pending", "The exact check identity and integrated source are reserved before command execution.")
		if err != nil {
			return row, err
		}
	} else if row.Delivery.Commit != commit || row.Delivery.SourceDigest != digest || row.Delivery.IntegrationDigest != integration {
		return row, ErrOrchestratorConflict
	}
	if row.Delivery.Status == "verified" {
		if row.Delivery.Deployment != nil || row.Delivery.Preview != nil && row.Delivery.Preview.Status == "ready" && row.DeploymentPlan != nil {
			return s.continueCoordinatorDeployment(ctx, p, row)
		}
		return s.continueCoordinatorPreview(ctx, p, row)
	}
	var invokeErr error
	if invoke {
		invokeErr = s.invokeCoordinatorChecks(ctx, p, row, run)
	}
	check, err := s.GetProjectCheckRun(p.ID, row.Delivery.CheckID)
	if errors.Is(err, ErrProjectRunNotFound) {
		status := "unknown"
		message := "This reserved check has no observable result. Its command will not be replayed; inspect the interrupted operation before authorizing new work."
		var refused *coordinatorStepError
		if errors.As(invokeErr, &refused) && refused.Status >= 400 && refused.Status < 500 {
			status = "blocked"
			message = "The exact reserved check was refused before admission. Review current command authority and configuration; no automatic replay is permitted."
		}
		return s.persistCoordinatorDelivery(ctx, p, row, status, message)
	}
	if err != nil {
		return row, err
	}
	if check.CoordinatorOriginDigest != coordinatorCheckBinding(row) || check.Commit != row.Delivery.Commit || check.SourceDigest != row.Delivery.SourceDigest {
		return row, ErrOrchestratorConflict
	}
	switch check.Status {
	case "queued", "running":
		return s.persistCoordinatorDelivery(ctx, p, row, "running", "The reserved independent check is running. Waiting for recorded command exit and cleanup; admission is not success.")
	case "failed", "blocked", "cancelled":
		return s.persistCoordinatorDelivery(ctx, p, row, "blocked", "The reserved independent check did not pass. Its result is retained; review the failure before continuing delivery.")
	case "passed":
		if check.ResourcesHeld == nil || *check.ResourcesHeld || check.SourceChanged || len(check.Commands) == 0 {
			return s.persistCoordinatorDelivery(ctx, p, row, "unknown", "The check has not established unchanged source and released execution resources.")
		}
		for _, command := range check.Commands {
			if !command.CleanupConfirmed || command.AllocationID == "" {
				return s.persistCoordinatorDelivery(ctx, p, row, "unknown", "The exact command cleanup has not been confirmed.")
			}
		}
	default:
		return s.persistCoordinatorDelivery(ctx, p, row, "unknown", "The exact check remains unconfirmed. Its retained execution must be observed; no command is replayed.")
	}
	evidence, err := s.SnapshotProjectRunCheck(ctx, p.ID, check.ID, run.Input.Approvals, run.Input.Requirements)
	if err != nil {
		return row, err
	}
	s.mu.Lock()
	err = s.coordinatorDeliveryAuthorityLocked(ctx, p, row)
	if err == nil {
		var current ProjectRun
		current, err = s.projectRuns.Get(p.ID, p.Path, row.RunID)
		if err == nil {
			currentIntegration, valid := coordinatorIntegratedRun(current, row)
			if !valid || currentIntegration != row.Delivery.IntegrationDigest {
				err = ErrOrchestratorConflict
			} else {
				_, err = s.projectRuns.JoinCheck(p.ID, p.Path, row.RunID, current.Revision, evidence)
			}
		}
	}
	s.mu.Unlock()
	if err != nil {
		return row, err
	}
	row, err = s.persistCoordinatorDelivery(ctx, p, row, "verified", "The exact completed check is linked to the reviewed integrated source and requirements. Preview, release policy and actual deployment remain to be completed.")
	if err != nil {
		return row, err
	}
	return s.continueCoordinatorPreview(ctx, p, row)
}

func (s *Server) persistCoordinatorDelivery(ctx context.Context, p project.Project, row coordinatorContinuationRecord, status, message string) (coordinatorContinuationRecord, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readContinuations(p, row.Actor.ID)
	index := findContinuation(rows, row.ID)
	if err != nil || index < 0 {
		return row, ErrOrchestratorInvalid
	}
	current := rows.Rows[index]
	if current.Revision != row.Revision {
		return current, ErrOrchestratorConflict
	}
	if err = s.coordinatorDeliveryAuthorityLocked(ctx, p, current); err != nil {
		return current, err
	}
	topStatus := status
	if status == "pending" || status == "running" {
		topStatus = "checking"
	} else if status == "verified" {
		topStatus = "checks-verified"
	}
	if row.Delivery != nil {
		copy := *row.Delivery
		row.Delivery = &copy
		row.Delivery.Status, row.Delivery.Message = status, message
	}
	if current.Status == topStatus && current.Message == message && reviewDigest(current.Delivery) == reviewDigest(row.Delivery) {
		return current, nil
	}
	if row.Delivery != nil {
		row.Delivery.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	}
	row.Status, row.Message = topStatus, message
	row.Revision++
	row.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	rows.Rows[index] = row
	return row, s.saveContinuations(rows)
}

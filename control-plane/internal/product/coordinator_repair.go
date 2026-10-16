package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func coordinatorRepairKey(obligationID string) string { return "coordinator-repair-" + obligationID }

// The failed finding is a retained claim. An automatic repair starts only
// from its exact stopped, still-current coding checkout, through the existing
// checkpoint guard and scheduler admission. A paused queue receipt owns the
// idempotency key before any checkout copy can occur.
func (s *Server) continueCoordinatorRepair(ctx context.Context, p project.Project, row coordinatorContinuationRecord) (bool, error) {
	if row.RunID == "" || row.Status != "queued" || row.CheckPlan == nil || len(row.Actions) != 3 || row.Actions[2].Status != "confirmed" || !s.coordinatorIndependentReviewCurrent(ctx, p, row) {
		return false, nil
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil || run.Swarm == nil || run.State == "cancelled" || run.State == "blocked" || run.Input.OrchestrationContract == nil {
		return false, err
	}
	if s.contractCurrentForRun(p, run) != nil || s.validateProjectSwarmReuseCurrent(ctx, p, run) != nil {
		return false, nil
	}
	for _, obligation := range run.Swarm.ReviewObligations {
		if obligation.State != "repair-required" {
			continue
		}
		task := swarmTask(&run, obligation.TaskID)
		source := swarmOperation(&run, obligation.CandidateOperationID)
		if task == nil || source == nil || source.ChildID != obligation.CandidateChildID || source.TaskID != task.ID {
			continue
		}
		key := coordinatorRepairKey(obligation.ID)
		item, found, readErr := s.coordinatorRepairIntent(p.ID, run.ID, key)
		if readErr != nil {
			return false, readErr
		}
		if found {
			if item.RepairObligationID != obligation.ID || item.RepairSourceOperationID != source.ID || item.TaskID != task.ID || item.InputDigest != queueInputDigest(run) {
				return false, ErrProjectRunKeyConflict
			}
			if item.State != "paused" || item.ReasonCode != "repair_checkpoint_pending" {
				continue // cancelled, uncertain and completed intents are never replayed
			}
			if obligation.RepairOperationID != "" && task.OperationID != obligation.RepairOperationID {
				return false, ErrProjectRunConflict
			}
			return s.continueCoordinatorRepairIntent(ctx, p, row, run, obligation, *source, item)
		}
		if obligation.RepairOperationID != "" || task.OperationID != source.ID || eligibleSwarmCheckpoint(run, source) != nil {
			continue
		}
		if budgetErr := swarmRepairBudgetForCheckpoint(run, source.ID); budgetErr != nil {
			if errors.Is(budgetErr, errAutomaticRepairLimit) {
				return s.coordinatorRepairBudgetAttention(p, row)
			}
			return false, budgetErr
		}
		if s.projectSwarmCheckpointAuthorize == nil || s.projectSwarmCheckpointGuard == nil {
			continue
		}
		capture, captureErr := s.captureSwarmCheckpoint(ctx, p, run, *source)
		if captureErr != nil || !capture.Review.CanContinue || !hexDigest(capture.Review.ReviewDigest) {
			continue // existing review/attention remains; no unproved copy intent
		}
		if !s.coordinatorIndependentReviewCurrent(ctx, p, row) {
			return false, nil
		}
		s.swarmScheduler.mu.Lock()
		runtime := s.swarmScheduler.runtime
		s.swarmScheduler.mu.Unlock()
		if runtime.CaptureActor == nil || runtime.Admit == nil || runtime.Dispatch == nil || runtime.Release == nil {
			continue
		}
		actor, actorErr := runtime.CaptureActor(ctx, p.ID)
		if actorErr != nil || actor.ProjectID != p.ID {
			continue
		}
		// Match the existing queue's dispatch/run exclusion order. A concurrent
		// revision or duplicate intent is read back, never appended twice.
		s.projectRunDispatchMu.Lock()
		latest, latestErr := s.projectRuns.Get(p.ID, p.Path, run.ID)
		if latestErr != nil || latest.Revision != run.Revision || !s.coordinatorIndependentReviewCurrent(ctx, p, row) {
			s.projectRunDispatchMu.Unlock()
			return false, latestErr
		}
		s.mu.RLock()
		budgetErr := s.swarmRepairPolicyCurrentLocked(p, latest)
		if budgetErr == nil {
			budgetErr = swarmRepairBudgetForCheckpoint(latest, source.ID)
		}
		if budgetErr != nil {
			s.mu.RUnlock()
			s.projectRunDispatchMu.Unlock()
			return false, nil
		}
		s.swarmScheduler.mu.Lock()
		ledger, ledgerErr := s.readSwarmScheduler()
		if ledgerErr == nil && len(ledger.Items) >= swarmQueueLimit {
			ledgerErr = ErrProjectRunLimit
		}
		if ledgerErr == nil {
			for _, existing := range ledger.Items {
				if existing.ProjectID != p.ID || existing.RunID != run.ID {
					continue
				}
				if existing.BatchKeyDigest == reviewDigest(key) || existing.TaskID == task.ID && !swarmQueueTerminal(existing.State) {
					ledgerErr = ErrProjectRunConflict
					break
				}
			}
		}
		if ledgerErr == nil && (ledger.PolicyRevision != run.Input.OrchestrationContract.Input.Controls.SchedulerPolicyRevision || reviewDigest(ledger.Policy) != run.Input.OrchestrationContract.Input.Controls.SchedulerPolicyDigest) {
			ledgerErr = ErrProjectRunConflict
		}
		if ledgerErr == nil {
			now := s.schedulerNow().Format(time.RFC3339Nano)
			record := swarmQueueRecord{SwarmQueueItem: SwarmQueueItem{Revision: 1, ID: newID("queue-"), ProjectID: p.ID, RunID: run.ID, TaskID: task.ID, TaskTitle: task.Title, Priority: "normal", State: "paused", QueuedAt: now, UpdatedAt: now, ReasonCode: "repair_checkpoint_pending", Reason: "The exact failed finding is awaiting a protected stopped-attempt checkpoint. No agent is admitted yet."}, Actor: actor, InputDigest: queueInputDigest(run), KeyDigest: reviewDigest(key + "/" + task.ID), BatchKeyDigest: reviewDigest(key), RequestDigest: implementationDigest(struct{ Obligation, Source, Review string }{obligation.ID, source.ID, capture.Review.ReviewDigest}), RepairObligationID: obligation.ID, RepairSourceOperationID: source.ID, RepairReviewDigest: capture.Review.ReviewDigest}
			ledger.Items = append(ledger.Items, record)
			ledgerErr = s.saveSwarmScheduler(&ledger)
		}
		s.swarmScheduler.mu.Unlock()
		s.mu.RUnlock()
		s.projectRunDispatchMu.Unlock()
		if ledgerErr != nil {
			if ledgerErr == ErrProjectRunConflict {
				return false, nil
			}
			return false, ledgerErr
		}
		s.WakeCoordinator()
		return true, nil
	}
	return false, nil
}

func (s *Server) coordinatorRepairIntent(projectID, runID, key string) (swarmQueueRecord, bool, error) {
	s.swarmScheduler.mu.Lock()
	ledger, err := s.readSwarmScheduler()
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		return swarmQueueRecord{}, false, err
	}
	for _, item := range ledger.Items {
		if item.ProjectID == projectID && item.RunID == runID && item.BatchKeyDigest == reviewDigest(key) {
			return item, true, nil
		}
	}
	return swarmQueueRecord{}, false, nil
}

// A coordinator-owned paused intent is not an operator pause. Only the exact
// retained repair receipt may move it to reserved or a held terminal outcome;
// generic schedulerUpdate deliberately refuses to reopen paused work.
func (s *Server) updateCoordinatorRepairIntent(id string, change func(*swarmQueueRecord) error) error {
	s.swarmScheduler.mu.Lock()
	defer s.swarmScheduler.mu.Unlock()
	row, err := s.readSwarmScheduler()
	if err != nil {
		return err
	}
	for i := range row.Items {
		item := &row.Items[i]
		if item.ID != id {
			continue
		}
		if item.State != "paused" || item.ReasonCode != "repair_checkpoint_pending" || item.RepairObligationID == "" {
			return ErrProjectRunConflict
		}
		if err := change(item); err != nil {
			return err
		}
		if item.State != "reserved" && item.State != "unknown" && item.State != "blocked" {
			return ErrProjectRunConflict
		}
		item.Revision++
		item.UpdatedAt = s.schedulerNow().Format(time.RFC3339Nano)
		return s.saveSwarmScheduler(&row)
	}
	return ErrProjectRunNotFound
}

func (s *Server) continueCoordinatorRepairIntent(ctx context.Context, p project.Project, row coordinatorContinuationRecord, run ProjectRun, obligation ProjectSwarmReviewObligation, source ProjectRunOperation, item swarmQueueRecord) (bool, error) {
	key := schedulerReserveKey(item)
	_, prior, found, err := s.projectRuns.FindOperationByKey(p.ID, p.Path, run.ID, key, "session")
	if err != nil {
		return false, err
	}
	if !found {
		if budgetErr := swarmRepairBudgetForCheckpoint(run, source.ID); budgetErr != nil {
			return false, s.updateCoordinatorRepairIntent(item.ID, func(current *swarmQueueRecord) error {
				schedulerReason(current, "blocked", "automatic_repair_limit_reached", errAutomaticRepairLimit.Error())
				return nil
			})
		}
		if !s.coordinatorIndependentReviewCurrent(ctx, p, row) {
			return false, nil
		}
		body, marshalErr := json.Marshal(swarmCheckpointRequest{ExpectedRevision: run.Revision, IdempotencyKey: key, ReviewDigest: item.RepairReviewDigest, ChildID: source.ChildID})
		if marshalErr != nil {
			return false, marshalErr
		}
		gated := context.WithValue(ctx, coordinatorIndependentReviewContextKey{}, coordinatorIndependentReviewGate{Row: row, Kind: "repair-checkpoint"})
		request, requestErr := http.NewRequestWithContext(gated, http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(body))
		if requestErr != nil {
			return false, requestErr
		}
		request.SetPathValue("id", p.ID)
		request.SetPathValue("runId", run.ID)
		request.SetPathValue("operationId", source.ID)
		request.Header.Set("Content-Type", "application/json")
		capture := &coordinatorResponse{header: make(http.Header)}
		s.reserveProjectSwarmCheckpoint(capture, request)
		_, prior, found, err = s.projectRuns.FindOperationByKey(p.ID, p.Path, run.ID, key, "session")
		if err != nil {
			return false, err
		}
		if !found {
			if capture.status >= 400 && capture.status < 500 && !capture.overflow {
				err = s.updateCoordinatorRepairIntent(item.ID, func(current *swarmQueueRecord) error {
					schedulerReason(current, "blocked", "repair_checkpoint_refused", "The protected checkpoint refused this exact failed finding. Inspect the saved source and review before planning a fresh attempt.")
					return nil
				})
			} else {
				err = s.updateCoordinatorRepairIntent(item.ID, func(current *swarmQueueRecord) error {
					schedulerReason(current, "unknown", "repair_checkpoint_unconfirmed", "Checkpoint outcome is unconfirmed. Inspect exact run and queue receipts; no automatic retry is allowed.")
					return nil
				})
			}
			return false, err
		}
	}
	if prior.Checkpoint == nil || prior.Checkpoint.SourceOperationID != source.ID || prior.Checkpoint.SourceChildID != source.ChildID || prior.Checkpoint.ReviewDigest != item.RepairReviewDigest || prior.TaskID != obligation.TaskID {
		err = s.updateCoordinatorRepairIntent(item.ID, func(current *swarmQueueRecord) error {
			schedulerReason(current, "unknown", "repair_identity_conflict", "The retained repair operation does not match the failed finding. No agent will start.")
			return nil
		})
		return false, err
	}
	if prior.Allocation != "ready" || prior.Status != "reserved" {
		err = s.updateCoordinatorRepairIntent(item.ID, func(current *swarmQueueRecord) error {
			current.OperationID, current.ChildID = prior.ID, prior.ChildID
			schedulerReason(current, "unknown", "repair_allocation_unconfirmed", "The exact repair checkout is not positively ready. Inspect its retained allocation before any start.")
			return nil
		})
		return false, err
	}
	if err = s.updateCoordinatorRepairIntent(item.ID, func(current *swarmQueueRecord) error {
		if current.State != "paused" || current.RepairObligationID != obligation.ID || current.RepairSourceOperationID != source.ID || current.RepairReviewDigest != item.RepairReviewDigest {
			return ErrProjectRunConflict
		}
		current.OperationID, current.ChildID = prior.ID, prior.ChildID
		schedulerReason(current, "reserved", "repair_reserved", "The fenced repair checkout is ready for the normal scheduler's current authority and resource admission.")
		return nil
	}); err != nil {
		return false, err
	}
	s.WakeSwarmScheduler()
	return true, nil
}

// A zero/exhausted budget is an observable decision on the existing retained
// continuation, not a fresh queue entry or a silently repeated scheduling turn.
func (s *Server) coordinatorRepairBudgetAttention(p project.Project, row coordinatorContinuationRecord) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readContinuations(p, row.Actor.ID)
	if err != nil {
		return false, err
	}
	index := findContinuation(rows, row.ID)
	if index < 0 {
		return false, ErrProjectRunNotFound
	}
	current := &rows.Rows[index]
	if current.Revision != row.Revision || current.RunID != row.RunID || current.PolicyRevision != row.PolicyRevision || current.Status != "queued" {
		return false, nil
	}
	if current.Message == errAutomaticRepairLimit.Error() {
		return false, nil
	}
	current.Message = errAutomaticRepairLimit.Error()
	current.Revision++
	current.UpdatedAt = s.schedulerNow().Format(time.RFC3339Nano)
	return false, s.saveContinuations(rows) // attention is not a newly dispatched effect
}

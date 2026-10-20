package product

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func coordinatorReviewerRetryKey(obligationID, candidateID string) string {
	return "coordinator-review-retry-" + reviewDigest(obligationID + "/" + candidateID)[:24]
}

// Idempotency is candidate/reviewer-local. A retained first review must not
// suppress another independent ready candidate's distinct reviewer intent.
func coordinatorReviewerRetryExists(items []swarmQueueRecord, projectID, runID, continuationID, actorID, reviewerID, candidateID, key string) (bool, error) {
	for _, prior := range items {
		if prior.ProjectID != projectID || prior.BatchKeyDigest != reviewDigest(key) { continue }
		if prior.RunID != runID || prior.TaskID != reviewerID || prior.AutoReviewContinuationID != continuationID || prior.AutoReviewActorID != actorID || prior.AutoReviewCandidateID != candidateID { return false, ErrProjectRunKeyConflict }
		return true, nil
	}
	return false, nil
}

// A changed repaired candidate requires a distinct reviewer attempt. The
// private queue is only an intent; the normal scheduler still owns resource
// admission, checkout allocation, exact assignment and child dispatch.
func (s *Server) continueCoordinatorReviewerRetry(ctx context.Context, p project.Project, row coordinatorContinuationRecord) (bool, error) {
	if row.RunID == "" || row.Status != "queued" || row.CheckPlan == nil || len(row.Actions) != 3 || row.Actions[2].Status != "confirmed" || !s.coordinatorIndependentReviewCurrent(ctx, p, row) {
		return false, nil
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, row.RunID)
	if err != nil || run.Swarm == nil || run.Swarm.State != "ready" || run.Input.OrchestrationContract == nil || run.State == "cancelled" || run.State == "blocked" {
		return false, err
	}
	if s.contractCurrentForRun(p, run) != nil || s.validateProjectSwarmReuseCurrent(ctx, p, run) != nil {
		return false, nil
	}
candidateLoop:
	for _, failed := range coordinatorRepairReviewCandidates(run) {
		if failed.RepairOperationID == "" {
			continue
		}
		candidateTask := swarmTask(&run, failed.TaskID)
		candidate := swarmOperation(&run, failed.RepairOperationID)
		if candidateTask == nil || candidate == nil || candidateTask.OperationID != candidate.ID || (!swarmCheckRepairCandidate(run, candidate.ID) && (candidate.Checkpoint == nil || candidate.Checkpoint.SourceOperationID != failed.CandidateOperationID)) || candidate.Allocation != "ready" || swarmRetirementPending(*candidate) {
			continue
		}
		if candidate.Status != "succeeded" && candidate.Status != "review-required" && candidate.Status != "failed" {
			continue
		}
		alreadyAssigned := false
		for _, item := range run.Swarm.ReviewObligations {
			if item.CandidateOperationID == candidate.ID {
				alreadyAssigned = true
				break
			}
		}
		if alreadyAssigned {
			continue
		}
		review, reviewErr := s.swarmReview(ctx, p, run, candidateTask.ID)
		if reviewErr != nil || !review.CanIntegrate || review.OperationID != candidate.ID || !hexDigest(review.ReviewDigest) || swarmReviewRepairSuccessor(run, candidateTask.ID, *candidate, review.SourceCommit) != nil {
			continue
		}
		for _, reviewer := range run.Swarm.Tasks {
			if !swarmContractReviewerRole(run, reviewer, candidateTask.ID) || reviewer.Integration != nil {
				continue
			}
			old := swarmOperation(&run, reviewer.OperationID)
			if old == nil || swarmSlotPending(*old) || swarmRetirementPending(*old) || old.Status != "succeeded" && old.Status != "review-required" && old.Status != "failed" && old.Status != "cancelled" {
				continue
			}
			key := coordinatorReviewerRetryKey(failed.ID, candidate.ID)
			s.swarmScheduler.mu.Lock()
			ledger, readErr := s.readSwarmScheduler()
			s.swarmScheduler.mu.Unlock()
			if readErr != nil {
				return false, readErr
			}
			queued, queuedErr := coordinatorReviewerRetryExists(ledger.Items, p.ID, run.ID, row.ID, row.Actor.ID, reviewer.ID, candidate.ID, key)
			if queuedErr != nil { return false, queuedErr }
			if queued { continue candidateLoop } // never duplicate; consider other candidates
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
			s.projectRunDispatchMu.Lock()
			latest, latestErr := s.projectRuns.Get(p.ID, p.Path, run.ID)
			if latestErr != nil || latest.Revision != run.Revision || !s.coordinatorIndependentReviewCurrent(ctx, p, row) {
				s.projectRunDispatchMu.Unlock()
				return false, latestErr
			}
			s.swarmScheduler.mu.Lock()
			ledger, latestErr = s.readSwarmScheduler()
			if latestErr == nil && len(ledger.Items) >= swarmQueueLimit {
				latestErr = ErrProjectRunLimit
			}
			if latestErr == nil && (ledger.PolicyRevision != run.Input.OrchestrationContract.Input.Controls.SchedulerPolicyRevision || reviewDigest(ledger.Policy) != run.Input.OrchestrationContract.Input.Controls.SchedulerPolicyDigest) {
				latestErr = ErrProjectRunConflict
			}
			if latestErr == nil {
				for _, prior := range ledger.Items {
					if prior.ProjectID == p.ID && (prior.BatchKeyDigest == reviewDigest(key) || prior.RunID == run.ID && prior.TaskID == reviewer.ID && !swarmQueueTerminal(prior.State)) {
						latestErr = ErrProjectRunConflict
						break
					}
				}
			}
			if latestErr == nil {
				now := s.schedulerNow().Format(time.RFC3339Nano)
				item := swarmQueueRecord{SwarmQueueItem: SwarmQueueItem{Revision: 1, ID: newID("queue-"), ProjectID: p.ID, RunID: run.ID, TaskID: reviewer.ID, TaskTitle: reviewer.Title, Priority: "normal", State: "queued", QueuedAt: now, UpdatedAt: now, ReasonCode: "review_repair_ready", Reason: "The changed repair candidate awaits a new independent reviewer under the saved contract."}, Actor: actor, InputDigest: queueInputDigest(run), KeyDigest: reviewDigest(key + "/" + reviewer.ID), BatchKeyDigest: reviewDigest(key), RequestDigest: implementationDigest(struct{ Run, Failed, Candidate, Review string }{run.ID, failed.ID, candidate.ID, review.ReviewDigest}), AutoReviewContinuationID: row.ID, AutoReviewActorID: row.Actor.ID, AutoReviewCandidateID: candidate.ID}
				ledger.Items = append(ledger.Items, item)
				latestErr = s.saveSwarmScheduler(&ledger)
			}
			s.swarmScheduler.mu.Unlock()
			s.projectRunDispatchMu.Unlock()
			if latestErr != nil {
				if errors.Is(latestErr, ErrProjectRunConflict) {
					return false, nil
				}
				return false, latestErr
			}
			s.WakeSwarmScheduler()
			return true, nil
		}
	}
	return false, nil
}

// The scheduler is not an authenticated browser request. It re-resolves the
// exact retained coordinator actor and reviewed row before any new retry step.
func (s *Server) coordinatorReviewerRetryAuthority(ctx context.Context, p project.Project, item swarmQueueRecord) (context.Context, coordinatorContinuationRecord, context.CancelFunc, error) {
	s.coordinatorBackground.mu.Lock()
	authorize := s.coordinatorBackground.authorize
	s.coordinatorBackground.mu.Unlock()
	if authorize == nil || item.AutoReviewContinuationID == "" {
		return nil, coordinatorContinuationRecord{}, nil, errCoordinatorAuthority
	}
	s.mu.RLock()
	rows, err := s.readContinuations(p, item.AutoReviewActorID)
	s.mu.RUnlock()
	if err != nil {
		return nil, coordinatorContinuationRecord{}, nil, err
	}
	index := findContinuation(rows, item.AutoReviewContinuationID)
	if index < 0 {
		return nil, coordinatorContinuationRecord{}, nil, ErrProjectRunConflict
	}
	row := rows.Rows[index]
	if row.ProjectID != p.ID || row.RunID != item.RunID || row.Actor.ID != item.AutoReviewActorID {
		return nil, coordinatorContinuationRecord{}, nil, ErrProjectRunConflict
	}
	bounded, cancel := context.WithTimeout(ctx, 30*time.Second)
	fresh, err := authorize(bounded, row.ExecutionActor, row.Run.Harness, row.Run.AgentConnectionID != nil && *row.Run.AgentConnectionID != "")
	if err != nil || fresh == nil {
		cancel()
		return nil, coordinatorContinuationRecord{}, nil, errCoordinatorAuthority
	}
	actor, ok := authenticatedContextActor(fresh)
	if !ok || actor.ID != row.Actor.ID || !s.coordinatorIndependentReviewCurrent(fresh, p, row) {
		cancel()
		return nil, coordinatorContinuationRecord{}, nil, errCoordinatorAuthority
	}
	return context.WithValue(fresh, coordinatorIndependentReviewContextKey{}, coordinatorIndependentReviewGate{Row: row, Kind: "reviewer-retry"}), row, cancel, nil
}

// Caller holds s.mu.RLock over the dispatch-intent mutation. Policy writes
// use s.mu.Lock, making this receipt the authorization linearization point.
func (s *Server) coordinatorReviewerRetryPolicyLocked(p project.Project, row coordinatorContinuationRecord) bool {
	policy, err := s.readCoordinatorPolicy(p)
	if err != nil || policy.Revision != row.PolicyRevision || policy.ExecutionMode != "reviewed-tasks" || policy.IntegrationMode != "reviewed-scopes" {
		return false
	}
	digest, err := s.continuationContextLocked(p, row.Run.ImplementationContext)
	return err == nil && digest == row.CanonicalDigest
}

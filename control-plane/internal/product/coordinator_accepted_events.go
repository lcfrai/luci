package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"sort"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Only an already approved exact selection may be reused for a later accepted
// event. This is a read: it neither prepares nor approves on behalf of the
// person. Final run creation compares all bound bytes and current authority.
func (s *Server) acceptedEventApprovedContract(ctx context.Context, p project.Project, actor string, selection ImplementationContextRequest, route *CoordinatorSavedRoute, exactTasks bool) *OrchestrationContractSelection {
	if route == nil || actor == "" {
		return nil
	}
	s.orchestrationContractMu.Lock()
	_, ledger, err := s.readOrchestrationContracts(p)
	s.orchestrationContractMu.Unlock()
	if err != nil {
		return nil
	}
	for index := len(ledger.Records) - 1; index >= 0; index-- {
		c := ledger.Records[index].Contract
		i := c.Input.Implementation
		if c.Actor.ID != actor || c.Approval == nil || c.State != "approved" || c.Approval.Actor.ID != actor || c.ProjectID != p.ID ||
			i.ProjectID != selection.ProjectID || i.WorktreeID != route.Route.WorktreeID || i.BriefRevision != selection.BriefRevision || i.PlanRevision != selection.PlanRevision ||
			i.SpecificationRevision != selection.SpecificationRevision || i.DesignPlanRevision != selection.DesignPlanRevision || exactTasks && !sameReviewedTaskIDs(i.TaskIDs, selection.TaskIDs) ||
			c.Input.Route.Harness != route.Route.Harness || (route.Route.Model != "" && c.Input.Route.Model != route.Route.Model) || c.Input.Route.PermissionMode != route.Route.PermissionMode || (route.Route.Effort != "" && c.Input.Route.Effort != route.Route.Effort) ||
			(c.Input.Route.ExecutionLocation != route.Route.ExecutionLocation && !(route.Route.ExecutionLocation == "" && c.Input.Route.ExecutionLocation == "local-docker")) || c.Input.Outcome != route.Route.Outcome {
			continue
		}
		if !exactTasks {
			included := map[string]bool{}
			for _, id := range i.TaskIDs {
				included[id] = true
			}
			for _, id := range selection.TaskIDs {
				if !included[id] {
					included = nil
					break
				}
			}
			if included == nil {
				continue
			}
		}
		if route.Route.AgentConnectionID != nil && c.Input.Route.Connection.ID != *route.Route.AgentConnectionID || ctx.Err() != nil {
			continue
		}
		// One bounded currentness check for the newest matching approval. An
		// older record cannot override a newer stale or revoked selection.
		if s.contractView(ctx, p, c).Freshness.Status != "current" {
			return nil
		}
		return &OrchestrationContractSelection{ID: c.ID, Revision: c.Revision, Digest: c.Digest, ApprovalID: c.Approval.ID}
	}
	return nil
}

// An accepted intent selects work from the current approved plan. A removed
// task is a blocker, never a reason to broaden execution silently.
func coordinatorAffectedTaskIDs(tasks []ProjectTask, events []CoordinationEventObservation) []string {
	if len(events) == 0 {
		return nil
	}
	known, selected := map[string]bool{}, map[string]bool{}
	for _, task := range tasks {
		known[task.ID] = true
	}
	all := false
	for _, event := range events {
		all = all || event.Intent.AffectsAllTasks
		for _, id := range event.Intent.TaskIDs {
			selected[id] = true
		}
	}
	for id := range selected {
		if !known[id] {
			return nil // A removed/renamed task needs explicit review.
		}
	}
	if all {
		for id := range known {
			selected[id] = true
		}
	}
	// A task using an amended predecessor cannot continue from an old-version
	// result. Expand only along saved plan dependencies; independent projects
	// and independent tasks retain their own identities and execution.
	for changed := true; changed; {
		changed = false
		for _, task := range tasks {
			if selected[task.ID] {
				continue
			}
			for _, dependency := range task.DependsOn {
				if selected[dependency] {
					selected[task.ID] = true
					changed = true
					break
				}
			}
		}
	}
	if len(selected) == 0 || len(selected) > 200 {
		return nil
	}
	out := make([]string, 0, len(selected))
	for id := range selected {
		if !known[id] {
			return nil
		}
		out = append(out, id)
	}
	sort.Strings(out)
	return out
}

// The request is sorted for durable replay while captured task order follows
// the approved plan. Both sides must still name the exact same unique tasks.
func sameReviewedTaskIDs(a, b []string) bool {
	if len(a) != len(b) || len(a) == 0 {
		return false
	}
	seen := make(map[string]bool, len(a))
	for _, id := range a {
		if id == "" || seen[id] {
			return false
		}
		seen[id] = true
	}
	for _, id := range b {
		if !seen[id] {
			return false
		}
		delete(seen, id)
	}
	return len(seen) == 0
}

func sameImplementationSelection(a, b ImplementationContextRequest) bool {
	return a.ProjectID == b.ProjectID && a.BriefRevision == b.BriefRevision && a.PlanRevision == b.PlanRevision && a.SpecificationRevision == b.SpecificationRevision && a.DesignPlanRevision == b.DesignPlanRevision && sameReviewedTaskIDs(a.TaskIDs, b.TaskIDs)
}

// pendingAcceptedCoordinatorEvents is a read of exact applied event receipts.
// Both background dispatch and the review projection use this same selection;
// a UI cannot guess a different event or task set from presentation history.
func (s *Server) pendingAcceptedCoordinatorEvents(ctx context.Context, p project.Project, rows coordinatorContinuations, view CoordinationView) ([]CoordinationEventObservation, error) {
	refs := make([]CoordinationEventReceipt, 0, len(view.Events)+len(view.Archives))
	for _, archive := range view.Archives {
		for _, event := range archive.Events {
			refs = append(refs, CoordinationEventReceipt{EventID: event.ID, Sequence: event.Sequence, OriginIntentID: event.OriginIntentID, ArchiveDigest: archive.Digest})
		}
	}
	for _, event := range view.Events {
		refs = append(refs, CoordinationEventReceipt{EventID: event.ID, Sequence: event.Sequence, OriginIntentID: event.IntentID})
	}
	sort.Slice(refs, func(i, j int) bool { return refs[i].Sequence < refs[j].Sequence })
	consumed := map[string]bool{}
	for _, row := range rows.Rows {
		for _, receipt := range row.AcceptedEvents {
			consumed[receipt.EventID] = true
		}
	}
	events := []CoordinationEventObservation{}
	for _, receipt := range refs {
		if consumed[receipt.EventID] {
			continue
		}
		observation, err := s.CoordinationEventByIdentity(ctx, p.ID, receipt)
		if err != nil {
			return nil, err
		}
		if observation.Event.Kind != "applied" || observation.Intent.Status != "applied" || observation.Intent.Actor.ID != observation.Event.Actor.ID {
			continue
		}
		manual := false
		for _, row := range rows.Rows {
			manual = manual || row.OriginTurnID == observation.Intent.TurnID
		}
		if !manual {
			events = append(events, observation)
		}
	}
	if len(events) > 16 {
		return nil, ErrOrchestratorCapacity
	}
	return events, nil
}

// A single actor's saved route is an explicit preference, never standing
// authority. Current team/edit/execute grants, approvals and check coverage
// are re-read by the production continuation handler before reservation.
func (s *Server) continueAcceptedCoordinatorEvent(ctx context.Context, p project.Project, rows coordinatorContinuations, authorize CoordinatorReauthorizer) (bool, error) {
	if rows.SavedRoute == nil || len(rows.Rows) == 0 || !validCoordinatorRoute(rows.SavedRoute) {
		return false, nil
	}
	var ref management.ExecutionActorRef
	for i := len(rows.Rows) - 1; i >= 0; i-- {
		if rows.Rows[i].ExecutionActor.UserID == rows.ActorID {
			ref = rows.Rows[i].ExecutionActor
			break
		}
	}
	if ref.UserID == "" {
		return false, nil
	}
	route := rows.SavedRoute
	fresh, err := authorize(ctx, ref, route.Route.Harness, route.Route.AgentConnectionID != nil && *route.Route.AgentConnectionID != "")
	if err != nil {
		return false, nil // Revoked authority cannot create work.
	}
	actor, ok := authenticatedContextActor(fresh)
	if !ok || actor.ID != rows.ActorID || s.coordinatorAuthority(fresh, p.ID) != nil {
		return false, nil
	}
	view, err := s.CoordinationViewForProject(fresh, p.ID)
	if err != nil {
		return false, err
	}
	// An accepted context edit can leave generated project files dirty. Do not
	// reserve a run that preparation would refuse, or turn a refused row into
	// a permanent duplicate barrier. Wait for an exact reviewed source commit.
	snapshot, err := s.OrchestratorSnapshot(fresh, p.ID)
	if err != nil {
		return false, err
	}
	if !snapshot.SourceKnown || !deliverySHA.MatchString(snapshot.SourceCommit) {
		return false, nil
	}
	events, err := s.pendingAcceptedCoordinatorEvents(fresh, p, rows, view)
	if err != nil {
		return false, err
	}
	if len(events) == 0 || len(events) > 16 {
		return false, nil
	}
	s.mu.RLock()
	coordination, frontierErr := s.readCoordination(p)
	frontier := ""
	if frontierErr == nil {
		frontier, frontierErr = s.latestAppliedCoordinationEventLocked(p, coordination)
	}
	s.mu.RUnlock()
	if frontierErr != nil || frontier != events[len(events)-1].Event.ID {
		return false, frontierErr
	}
	ids := make([]string, 0, len(events))
	for _, event := range events {
		ids = append(ids, event.Event.ID)
	}
	s.mu.RLock()
	claimed, claimErr := s.acceptedEventClaimedLocked(p, ids, events[len(events)-1].Intent.TurnID)
	s.mu.RUnlock()
	if claimErr != nil || claimed {
		return false, claimErr
	}
	s.mu.RLock()
	state, stateErr := s.readApprovedJourney(p.Path, p.ID)
	policy, policyErr := s.readCoordinatorPolicy(p)
	s.mu.RUnlock()
	if stateErr != nil {
		return false, stateErr
	}
	if policyErr != nil {
		return false, policyErr
	}
	if policy.ExecutionMode != "reviewed-tasks" {
		return false, nil
	}
	selected := coordinatorAffectedTaskIDs(currentPlan(state, p.ID).Tasks, events)
	if len(selected) == 0 {
		return false, nil
	}
	selection := ImplementationContextRequest{ProjectID: p.ID, BriefRevision: state.Brief.Revision, PlanRevision: currentPlan(state, p.ID).Revision, SpecificationRevision: currentSpecification(state).Revision, DesignPlanRevision: currentDesignPlan(state).Revision, TaskIDs: selected}
	if _, err = implementationSnapshot(state, selection, ""); err != nil {
		return false, nil // Accepted intent can precede required context approval.
	}
	for _, prior := range rows.Rows {
		old := prior.Run.ImplementationContext
		if old.BriefRevision == selection.BriefRevision && old.PlanRevision == selection.PlanRevision &&
			old.SpecificationRevision == selection.SpecificationRevision && old.DesignPlanRevision == selection.DesignPlanRevision {
			continue
		}
		if actions := prior.Actions; len(actions) > 0 && (actions[len(actions)-1].Status == "pending" || actions[len(actions)-1].Status == "unknown") {
			return false, nil // Preserve uncertain old publication for exact readback; no new run yet.
		}
	}
	lastEventID := events[len(events)-1].Event.ID
	pendingCommit := false
	for _, commit := range rows.ContextCommits {
		pendingCommit = pendingCommit || commit.EventID == lastEventID && commit.Status == "pending"
	}
	if snapshot.SourceDirty || pendingCommit {
		// Reconcile only the already accepted canonical context under its exact
		// journal. This does not approve the changed execution contract or start
		// replacement work. Unknown Git effects stay pending for readback.
		confirmed, commitErr := s.commitAcceptedCoordinatorContext(fresh, p, lastEventID, snapshot.SourceCommit)
		if commitErr != nil || !confirmed {
			return false, commitErr
		}
		return false, nil // A later observation must bind a fresh approved contract to the committed source.
	}
	if s.acceptedEventApprovedContract(fresh, p, rows.ActorID, selection, route, false) == nil {
		return false, nil // No current exact approval: leave accepted intent for human review.
	}
	// A changed downstream task can depend on previously integrated work. Reuse
	// only exact reviewed prerequisites; otherwise include those ancestors in
	// the new run so the person need not shuttle a dependency by hand.
	expanded, expandErr := s.coordinatorExpandAffectedPrerequisites(fresh, p, state, selection.TaskIDs)
	if expandErr != nil {
		return false, expandErr
	}
	selection.TaskIDs = expanded
	if _, err = implementationSnapshot(state, selection, ""); err != nil {
		return false, nil // A new prerequisite may need renewed context review.
	}
	// The reusable route cannot approve a changed Design, task graph, source or
	// controls. Require a matching current, actor-owned approval before any Git
	// commit, supersession or replacement launch. Final create revalidates the
	// complete contract against the selected run under its normal gate.
	contract := s.acceptedEventApprovedContract(fresh, p, rows.ActorID, selection, route, true)
	if contract == nil {
		return false, nil
	}
	if err = s.blockSupersededQueuedTasks(p.ID, selection); err != nil {
		return false, err // Retry the same accepted event before creating new work.
	}
	settled, stopErr := s.stopSupersededActiveTasks(ctx, p.ID, selection)
	if stopErr != nil || !settled {
		return false, stopErr // No revised admission while an old child/charge is uncertain.
	}
	run := projectRunCreateRequest{WorktreeID: route.Route.WorktreeID, Harness: route.Route.Harness, Model: route.Route.Model, PermissionMode: route.Route.PermissionMode, Effort: route.Route.Effort, ExecutionLocation: route.Route.ExecutionLocation, AgentConnectionID: route.Route.AgentConnectionID, Outcome: route.Route.Outcome, ImplementationContext: selection, OrchestrationContract: contract}
	req := coordinatorContinuationRequest{IdempotencyKey: "accepted-" + events[len(events)-1].Event.ID, OriginTurnID: events[len(events)-1].Intent.TurnID, ExpectedContextDigest: view.ContextDigest, ExpectedPolicyRevision: policy.Revision, Run: run, MaxParallel: route.MaxParallel, ExpectedRouteRevision: &route.Revision, CheckPlan: route.CheckPlan, PreviewPlan: route.PreviewPlan, DeploymentPlan: route.DeploymentPlan}
	for _, event := range events {
		req.AcceptedEvents = append(req.AcceptedEvents, event.Receipt)
	}
	raw, err := json.Marshal(req)
	if err != nil {
		return false, err
	}
	httpRequest, err := http.NewRequestWithContext(fresh, http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(raw))
	if err != nil {
		return false, err
	}
	httpRequest.SetPathValue("id", p.ID)
	httpRequest.Header.Set("Content-Type", "application/json")
	capture := &coordinatorResponse{header: make(http.Header)}
	s.createCoordinatorContinuation(capture, httpRequest)
	if capture.overflow {
		return false, ErrOrchestratorCapacity
	}
	if capture.status == http.StatusCreated || capture.status == http.StatusOK {
		return true, nil
	}
	if capture.status >= 400 && capture.status < 500 {
		return false, nil // Preserve current-policy refusal; do not retry a mutation.
	}
	return false, errors.New("accepted coordinator event could not be reserved; inspect the retained event and continuation before retry")
}

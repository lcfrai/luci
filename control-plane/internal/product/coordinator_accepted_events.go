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
	if all {
		for id := range known {
			selected[id] = true
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
		observation, e := s.CoordinationEventByIdentity(fresh, p.ID, receipt)
		if e != nil {
			return false, e
		}
		if observation.Event.Kind != "applied" || observation.Intent.Status != "applied" || observation.Intent.Actor.ID != observation.Event.Actor.ID {
			continue
		}
		// A manually reserved continuation for this exact accepted turn is
		// already its work; never create a second run for the same intent.
		manual := false
		for _, row := range rows.Rows {
			manual = manual || row.OriginTurnID == observation.Intent.TurnID
		}
		if !manual {
			events = append(events, observation)
		}
	}
	if len(events) == 0 || len(events) > 16 || events[len(events)-1].Event.ID != view.EventHead {
		return false, nil // An intervening event needs resolution or a new review.
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
	lastEventID := events[len(events)-1].Event.ID
	reservedCommit := false
	for _, commit := range rows.ContextCommits {
		reservedCommit = reservedCommit || commit.EventID == lastEventID
	}
	if snapshot.SourceDirty || reservedCommit {
		confirmed, commitErr := s.commitAcceptedCoordinatorContext(fresh, p, lastEventID, snapshot.SourceCommit)
		if commitErr != nil || !confirmed {
			return false, commitErr
		}
	}
	run := projectRunCreateRequest{WorktreeID: route.Route.WorktreeID, Harness: route.Route.Harness, Model: route.Route.Model, PermissionMode: route.Route.PermissionMode, Effort: route.Route.Effort, ExecutionLocation: route.Route.ExecutionLocation, AgentConnectionID: route.Route.AgentConnectionID, Outcome: route.Route.Outcome, ImplementationContext: selection}
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

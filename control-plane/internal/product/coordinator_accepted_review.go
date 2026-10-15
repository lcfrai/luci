package product

import (
	"context"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// This is a read-only, actor-private explanation of the next accepted change.
// Task IDs are offered for review only after the committed source and exact
// prerequisite reuse have been observed. Dispatch repeats every check.
type CoordinatorAcceptedReview struct {
	EventID   string                        `json:"eventId"`
	Status    string                        `json:"status"`
	Reason    string                        `json:"reason"`
	Selection *ImplementationContextRequest `json:"selection,omitempty"`
}

func (s *Server) acceptedCoordinatorReview(ctx context.Context, p project.Project, rows coordinatorContinuations) *CoordinatorAcceptedReview {
	if rows.SavedRoute == nil || !validCoordinatorRoute(rows.SavedRoute) || len(rows.Rows) == 0 {
		return nil
	}
	view, err := s.CoordinationViewForProject(ctx, p.ID)
	if err != nil {
		return nil
	}
	events, err := s.pendingAcceptedCoordinatorEvents(ctx, p, rows, view)
	if err != nil || len(events) == 0 {
		return nil
	}
	last := events[len(events)-1].Event.ID
	out := &CoordinatorAcceptedReview{EventID: last, Status: "waiting", Reason: "The accepted change is retained. Refresh its source, approval and earlier work before another dispatch."}
	s.mu.RLock()
	coordination, err := s.readCoordination(p)
	frontier := ""
	if err == nil {
		frontier, err = s.latestAppliedCoordinationEventLocked(p, coordination)
	}
	state, stateErr := s.readApprovedJourney(p.Path, p.ID)
	policy, policyErr := s.readCoordinatorPolicy(p)
	s.mu.RUnlock()
	if err != nil || frontier != last || stateErr != nil || policyErr != nil || policy.ExecutionMode != "reviewed-tasks" {
		return out
	}
	selected := coordinatorAffectedTaskIDs(currentPlan(state, p.ID).Tasks, events)
	if len(selected) == 0 {
		out.Reason = "The changed task is missing from the current approved plan. Review the plan before continuing."
		return out
	}
	selection := ImplementationContextRequest{ProjectID: p.ID, BriefRevision: state.Brief.Revision, PlanRevision: currentPlan(state, p.ID).Revision, SpecificationRevision: currentSpecification(state).Revision, DesignPlanRevision: currentDesignPlan(state).Revision, TaskIDs: selected}
	if _, err := implementationSnapshot(state, selection, ""); err != nil {
		out.Reason = "Review and approve the changed definition, design and plan before continuing."
		return out
	}
	for _, prior := range rows.Rows {
		old := prior.Run.ImplementationContext
		if old.BriefRevision == selection.BriefRevision && old.PlanRevision == selection.PlanRevision && old.SpecificationRevision == selection.SpecificationRevision && old.DesignPlanRevision == selection.DesignPlanRevision {
			continue
		}
		if n := len(prior.Actions); n > 0 && (prior.Actions[n-1].Status == "pending" || prior.Actions[n-1].Status == "unknown") {
			out.Reason = "An earlier continuation has an unconfirmed effect. Read its exact outcome before replacing work."
			return out
		}
	}
	snapshot, err := s.OrchestratorSnapshot(ctx, p.ID)
	if err != nil || !snapshot.SourceKnown || !deliverySHA.MatchString(snapshot.SourceCommit) {
		out.Reason = "The accepted source identity is unavailable. Review the project source before continuing."
		return out
	}
	for _, commit := range rows.ContextCommits {
		if commit.EventID == last && commit.Status == "pending" {
			out.Reason = "The accepted context commit has an unconfirmed outcome. Luci will read back its exact bytes without replaying the write."
			return out
		}
	}
	if snapshot.SourceDirty {
		out.Reason = "The accepted context changed source files. Luci is reconciling only those reviewed bytes before a new contract can be approved."
		return out
	}
	expanded, err := s.coordinatorExpandAffectedPrerequisites(ctx, p, state, selected)
	if err != nil {
		out.Reason = "The exact source or prerequisite evidence is unavailable. Review project source and retained work."
		return out
	}
	selection.TaskIDs = expanded
	if _, err := implementationSnapshot(state, selection, ""); err != nil {
		out.Reason = "A prerequisite needs renewed context review before a contract can be approved."
		return out
	}
	out.Selection = &selection
	if s.acceptedEventApprovedContract(ctx, p, rows.ActorID, selection, rows.SavedRoute, true) == nil {
		out.Status = "review-required"
		out.Reason = "The accepted change is committed. Review and approve its new execution contract; Luci will then resume the saved route under current authority."
	} else {
		out.Status = "approved"
		out.Reason = "The changed execution contract is approved. Luci is rechecking current authority and retained work before continuing."
	}
	return out
}

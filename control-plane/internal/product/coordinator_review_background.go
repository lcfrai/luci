package product

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// This context value is set only by the saved-route coordinator. The approval
// handlers recheck it while holding the project state lock, including the
// exact applied turn and current policy. It cannot be supplied by HTTP input.
type coordinatorDelegatedReviewKey struct{}
type coordinatorDelegatedReviewGate struct {
	ActorID        string
	TurnID         string
	PolicyRevision int
	TargetRevision int
	TargetDigest   string
}

func (s *Server) validateCoordinatorDelegatedReviewLocked(ctx context.Context, p project.Project, state journeyState, gate coordinatorDelegatedReviewGate, target string, revision int, digest string) error {
	actor, ok := authenticatedContextActor(ctx)
	if !ok || actor.ID != gate.ActorID || gate.TargetRevision != revision || gate.TargetDigest != digest || !hexDigest(digest) {
		return errCoordinatorPolicy
	}
	if s.coordinatorAuthority(ctx, p.ID) != nil {
		return errCoordinatorAuthority
	}
	policy, err := s.readCoordinatorPolicy(p)
	if err != nil || policy.Revision != gate.PolicyRevision || policy.Mode != "auto-drafts" || policy.ReviewMode != "approved-amendments" || policy.Actor == nil || policy.Actor.ID != gate.ActorID {
		return errCoordinatorPolicy
	}
	allowed := false
	for _, selected := range policy.AllowedTargets {
		allowed = allowed || selected == target
	}
	if !allowed {
		return errCoordinatorPolicy
	}
	ledger, err := s.readOrchestrator(p)
	if err != nil {
		return err
	}
	matched := false
	for _, turn := range ledger.Turns {
		if turn.ID == gate.TurnID && turn.Actor.ID == gate.ActorID && turn.Status == "succeeded" && turn.Answer != nil && !turn.Answer.Blocking && turn.Answer.Proposal != nil && turn.Answer.Proposal.Target == target && turn.AppliedRevision > 0 && turn.ApplyTargetRevision == revision && turn.ApplyTargetDigest == digest && turn.ApplyIntent == "" {
			matched = true
			break
		}
	}
	if !matched {
		return errCoordinatorPolicy
	}
	// Only a previously approved document may be renewed automatically. Its
	// historical review remains durable even after saving this new draft
	// revokes the current private approval selection.
	prior := false
	store, err := s.readApprovalStore(p.Path, p.ID)
	if err != nil {
		return errCoordinatorPolicy
	}
	switch target {
	case "specification":
		for _, row := range state.SpecificationRevisions {
			prior = prior || store.LastSpecification != nil && row.Status == "approved" && row.Revision < revision && row.Revision == store.LastSpecification.Revision && row.Digest == store.LastSpecification.ContentDigest && row.Digest == specificationContentDigest(row)
		}
	case "design-plan":
		for _, row := range state.DesignPlanRevisions {
			prior = prior || store.LastDesign != nil && row.Status == "approved" && row.Revision < revision && row.Revision == store.LastDesign.Revision && row.Digest == store.LastDesign.ContentDigest && row.Digest == designPlanContentDigest(row)
		}
	}
	if !prior {
		return errCoordinatorPolicy
	}
	return nil
}

// Review is an opt-in continuation of an already reviewed document, never an
// initial approval. It uses the ordinary validation/export endpoint and does
// not grant execute, integration, preview or deployment authority.
func (s *Server) continueSavedCoordinatorReview(ctx context.Context, p project.Project, rows coordinatorContinuations, authorize CoordinatorReauthorizer) (bool, error) {
	if rows.SavedRoute == nil || !validCoordinatorRoute(rows.SavedRoute) {
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
	route := rows.SavedRoute.Route
	fresh, err := authorize(ctx, ref, route.Harness, route.AgentConnectionID != nil && *route.AgentConnectionID != "")
	if err != nil {
		return false, nil
	}
	actor, ok := authenticatedContextActor(fresh)
	if !ok || actor.ID != rows.ActorID || s.coordinatorAuthority(fresh, p.ID) != nil {
		return false, nil
	}
	s.mu.RLock()
	policy, err := s.readCoordinatorPolicy(p)
	state, stateErr := s.readApprovedJourney(p.Path, p.ID)
	ledger, ledgerErr := s.readOrchestrator(p)
	s.mu.RUnlock()
	if err != nil || stateErr != nil || ledgerErr != nil {
		return false, errors.Join(err, stateErr, ledgerErr)
	}
	if policy.Mode != "auto-drafts" || policy.ReviewMode != "approved-amendments" || policy.Actor == nil || policy.Actor.ID != actor.ID {
		return false, nil
	}
	// A prior uncertain approval receipt cannot be treated as an absent write.
	// Read the authoritative state; never replay a new review over an orphaned
	// delegated receipt whose journey commit could not be confirmed.
	if state.approvalAuthority != nil {
		approval := *state.approvalAuthority
		for _, receipt := range []*journeyApprovalReceipt{approval.Specification, approval.Design} {
			if receipt != nil && receipt.DelegatedPolicyRevision == policy.Revision &&
				((receipt.Kind == "specification" && !specificationApprovalMatches(state)) || (receipt.Kind == "design" && !designApprovalMatches(state))) {
				return false, errors.New("delegated context review outcome is unconfirmed; inspect retained private approval before continuing")
			}
		}
	}
	view, err := s.OrchestratorView(fresh, p.ID, 0)
	if err != nil || len(view.BlockingQuestions) > 0 {
		return false, err
	}
	for i := len(ledger.Turns) - 1; i >= 0; i-- {
		turn := ledger.Turns[i]
		if turn.Actor.ID != actor.ID || turn.Status != "succeeded" || turn.Answer == nil || turn.Answer.Blocking || turn.Answer.Proposal == nil || turn.AppliedRevision == 0 || turn.ApplyIntent != "" {
			continue
		}
		target := turn.Answer.Proposal.Target
		if target != "specification" && target != "design-plan" {
			continue
		}
		revision, current := targetSnapshot(state, target)
		if revision == 0 || revision != turn.ApplyTargetRevision || reviewDigest(current) != turn.ApplyTargetDigest || !hexDigest(turn.ApplyTargetDigest) {
			continue
		}
		status := ""
		briefRevision := state.Brief.Revision
		if target == "specification" {
			status = state.Specification.Status
		} else {
			status = state.DesignPlan.Status
		}
		if status != "draft" {
			continue
		}
		refusalKey := p.ID + ":" + actor.ID
		fingerprint := reviewDigest([]any{turn.ID, turn.ApplyTargetDigest, revision, policy.Revision})
		b := &s.coordinatorBackground
		b.mu.Lock()
		alreadyRefused := b.reviewRefusals[refusalKey] == fingerprint
		b.mu.Unlock()
		if alreadyRefused {
			continue
		}
		gate := coordinatorDelegatedReviewGate{ActorID: actor.ID, TurnID: turn.ID, PolicyRevision: policy.Revision, TargetRevision: revision, TargetDigest: turn.ApplyTargetDigest}
		s.mu.RLock()
		gateErr := s.validateCoordinatorDelegatedReviewLocked(fresh, p, state, gate, target, revision, turn.ApplyTargetDigest)
		s.mu.RUnlock()
		if gateErr != nil {
			continue
		}
		body := map[string]any{"revision": revision}
		if target == "specification" {
			body["briefRevision"] = briefRevision
		}
		encoded, err := json.Marshal(body)
		if err != nil {
			return false, err
		}
		request, err := http.NewRequestWithContext(context.WithValue(fresh, coordinatorDelegatedReviewKey{}, gate), http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(encoded))
		if err != nil {
			return false, err
		}
		request.SetPathValue("id", p.ID)
		request.Header.Set("Content-Type", "application/json")
		release, err := s.AcquireGitSessionAdmission(fresh, p.ID)
		if err != nil {
			return false, nil // Another source writer is active; retain the exact draft.
		}
		if s.coordinatorAuthority(fresh, p.ID) != nil {
			release()
			return false, nil
		}
		response := &coordinatorResponse{header: make(http.Header)}
		if target == "specification" {
			s.approveSpecification(response, request)
		} else {
			s.approveDesignPlan(response, request)
		}
		release()
		if response.overflow {
			return false, ErrOrchestratorCapacity
		}
		if response.status >= 200 && response.status < 300 {
			b.mu.Lock()
			delete(b.reviewRefusals, refusalKey)
			b.mu.Unlock()
			return true, nil
		}
		if response.status >= 400 && response.status < 500 {
			b.mu.Lock()
			if b.reviewRefusals == nil {
				b.reviewRefusals = map[string]string{}
			}
			b.reviewRefusals[refusalKey] = fingerprint
			b.mu.Unlock()
			return false, nil
		}
		return false, errors.New("delegated context review outcome is unconfirmed; inspect current approval before continuing")
	}
	return false, nil
}

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

// A saved route allows the person's Luci to continue a completed, unambiguous
// context proposal even when its browser is closed. This invokes the same
// guarded draft-only endpoint as the interactive client; it neither approves
// context nor grants execution by itself. Current identity, edit permission,
// policy, conflict and exact context CAS are rechecked on every attempt.
func (s *Server) continueSavedCoordinatorDraft(ctx context.Context, p project.Project, rows coordinatorContinuations, authorize CoordinatorReauthorizer) (bool, error) {
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
	s.mu.RUnlock()
	if err != nil {
		return false, err
	}
	if policy.Mode != "auto-drafts" {
		return false, nil
	}
	view, err := s.OrchestratorView(fresh, p.ID, 0)
	if err != nil {
		return false, err
	}
	if len(view.BlockingQuestions) > 0 {
		return false, nil
	}
	for _, turn := range view.Turns {
		if turn.Actor.ID != actor.ID || turn.Status != "succeeded" || turn.AppliedRevision != 0 ||
			turn.Answer == nil || turn.Answer.Blocking || turn.Answer.Proposal == nil {
			continue
		}
		allowed := false
		for _, target := range policy.AllowedTargets {
			allowed = allowed || target == turn.Answer.Proposal.Target
		}
		if !allowed {
			continue
		}
		refusalKey := p.ID + ":" + actor.ID
		fingerprint := reviewDigest([]any{turn.ID, turn.ContextDigest, view.Revision, view.Context.Digest, policy.Revision})
		b := &s.coordinatorBackground
		b.mu.Lock()
		alreadyRefused := b.draftRefusals[refusalKey] == fingerprint
		b.mu.Unlock()
		if alreadyRefused {
			return false, nil
		}
		body, err := json.Marshal(map[string]any{"expectedRevision": view.Revision,
			"expectedContextDigest": view.Context.Digest, "expectedPolicyRevision": policy.Revision})
		if err != nil {
			return false, err
		}
		req, err := http.NewRequestWithContext(fresh, http.MethodPost, "http://coordinator.invalid/", bytes.NewReader(body))
		if err != nil {
			return false, err
		}
		req.SetPathValue("id", p.ID)
		req.SetPathValue("turnId", turn.ID)
		req.Header.Set("Content-Type", "application/json")
		capture := &coordinatorResponse{header: make(http.Header)}
		s.advanceOrchestrator(capture, req)
		if capture.overflow {
			return false, ErrOrchestratorCapacity
		}
		if capture.status >= 200 && capture.status < 300 {
			b.mu.Lock()
			delete(b.draftRefusals, refusalKey)
			b.mu.Unlock()
			return true, nil
		}
		if capture.status >= 400 && capture.status < 500 {
			b.mu.Lock()
			if b.draftRefusals == nil {
				b.draftRefusals = map[string]string{}
			}
			b.draftRefusals[refusalKey] = fingerprint
			b.mu.Unlock()
			return false, nil // A conflict or revoked policy cannot be retried as a new mutation.
		}
		return false, errors.New("automatic context draft outcome is unconfirmed; inspect the retained turn before continuing")
	}
	return false, nil
}

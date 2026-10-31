package product

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func TestPersonalWorkHandoffRequiresControlAndRetainsReviewedPurpose(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "reviewed-personal-work")
	ctx := homeActor("alice")
	view, err := s.SavePersonalPreferences(ctx, 0, personalTestPreferences(p.ID), personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	turn, _, err := s.ReservePersonalTurn(ctx, PersonalTurnRequest{ExpectedRevision: view.Revision, ExpectedContextDigest: view.Context.Digest, IdempotencyKey: "personal-work-original-123456", Message: "PRIVATE PORTFOLIO INTENT"}, "assist-personal-work", personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	scope, _ := s.PersonalScope(ctx)
	answer := &PersonalAnswer{Summary: "Review the project request", Options: []string{}, Discussion: &PersonalDiscussion{ProjectID: p.ID, Message: "Review delivery priorities"}}
	if err := s.CompletePersonalTurn(scope, turn.ID, turn.JobID, "succeeded", answer); err != nil {
		t.Fatal(err)
	}
	view, err = s.PersonalView(ctx, 0, personalAllow)
	if err != nil {
		t.Fatal(err)
	}
	req := PersonalHandoffRequest{Purpose: PersonalHandoffWorkRequest, ExpectedRevision: view.Revision, ProjectID: p.ID, Message: "Build the reviewed search screen", ProjectRevision: 0, ProjectContextDigest: strings.Repeat("a", 64), IdempotencyKey: "personal-work-handoff-123456"}
	if _, _, err := s.ReservePersonalHandoff(ctx, turn.ID, req, personalAllow); !errors.Is(err, management.ErrExecutionAuthority) {
		t.Fatal("read-only context admitted work", err)
	}
	controlCalls := 0
	allow := func(current context.Context, projectID string, control bool) error {
		if projectID != p.ID {
			return management.ErrExecutionAuthority
		}
		if control {
			controlCalls++
		}
		return current.Err()
	}
	saved, existing, err := s.ReservePersonalHandoff(ctx, turn.ID, req, allow)
	if err != nil || existing || controlCalls != 1 || saved.Handoff == nil || saved.Handoff.Purpose != PersonalHandoffWorkRequest {
		t.Fatal("work purpose not retained", saved, existing, controlCalls, err)
	}
	repeated, existing, err := s.ReservePersonalHandoff(ctx, turn.ID, req, allow)
	if err != nil || !existing || repeated.Handoff.ID != saved.Handoff.ID {
		t.Fatal("original work request duplicated", err)
	}
	changed := req
	changed.Purpose = ""
	if _, _, err := s.ReservePersonalHandoff(ctx, turn.ID, changed, allow); !errors.Is(err, ErrPersonalChanged) {
		t.Fatal("retained purpose was changed", err)
	}
	loaded, err := s.PersonalView(ctx, 0, personalAllow)
	if err != nil || loaded.Turns[0].Handoff.Purpose != PersonalHandoffWorkRequest {
		t.Fatal("purpose lost on durable read", err)
	}
	raw, _ := json.Marshal(saved.Handoff)
	if strings.Contains(string(raw), "PRIVATE PORTFOLIO INTENT") || !strings.Contains(string(raw), req.Message) {
		t.Fatal("handoff contains private history or lost selected text")
	}
}

func TestPersonalWorkPurposeDoesNotUpgradeExistingDiscussion(t *testing.T) {
	original := OrchestratorTurn{ID: "turn-discussion-original", DiscussionOnly: false}
	normal := OrchestratorTurn{ID: "turn-ordinary-new"}
	if PersonalHandoffMatchesTurn(PersonalHandoffWorkRequest, original) || !PersonalHandoffMatchesTurn("", original) || PersonalHandoffMatchesTurn("", normal) || !PersonalHandoffMatchesTurn(PersonalHandoffWorkRequest, normal) || PersonalHandoffMatchesTurn("unknown", normal) {
		t.Fatal("purpose mismatch or legacy discussion downgrade")
	}
	if !personalDiscussionOnly(PersonalHandoffContext(context.Background(), "")) || personalDiscussionOnly(PersonalHandoffContext(context.Background(), PersonalHandoffWorkRequest)) {
		t.Fatal("ordinary work and discussion context mixed")
	}
}

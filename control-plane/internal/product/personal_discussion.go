package product

import (
	"context"
	"strings"
)

type personalDiscussionPurpose struct{}

// Only the controller's reviewed personal handoff supplies this purpose. It is
// not decoded from a public project request. Its persisted restriction survives
// restart and is checked by completion and every Apply/Advance mutation gate.
func PersonalDiscussionContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, personalDiscussionPurpose{}, true)
}
func personalDiscussionOnly(ctx context.Context) bool {
	value, _ := ctx.Value(personalDiscussionPurpose{}).(bool)
	return value
}

// A controller-minted canonical identity retains the restrictive purpose even
// if a restored record lacks the newer optional field. Public callers cannot
// mint or rename canonical turns. Existing ordinary turn IDs keep their rules.
func (turn OrchestratorTurn) IsDiscussionOnly() bool {
	return turn.DiscussionOnly || strings.HasPrefix(turn.ID, "turn-discussion-")
}

const PersonalHandoffWorkRequest = "work-request"

func validPersonalHandoffPurpose(purpose string) bool {
	return purpose == "" || purpose == PersonalHandoffWorkRequest
}

// Omitted purpose retains the original discussion-only contract. No existing
// discussion is converted: a work request creates a new ordinary project turn.
func PersonalHandoffContext(ctx context.Context, purpose string) context.Context {
	if purpose == PersonalHandoffWorkRequest {
		return ctx
	}
	return PersonalDiscussionContext(ctx)
}

func PersonalHandoffMatchesTurn(purpose string, turn OrchestratorTurn) bool {
	return validPersonalHandoffPurpose(purpose) && turn.IsDiscussionOnly() == (purpose != PersonalHandoffWorkRequest)
}

type PersonalHandoffRequest struct {
	Purpose              string `json:"purpose,omitempty"`
	ExpectedRevision     int    `json:"expectedRevision"`
	ProjectID            string `json:"projectId"`
	Message              string `json:"message"`
	ProjectRevision      int    `json:"projectRevision"`
	ProjectContextDigest string `json:"projectContextDigest"`
	IdempotencyKey       string `json:"idempotencyKey"`
}

func (s *Server) ReservePersonalHandoff(ctx context.Context, turnID string, req PersonalHandoffRequest, access PortfolioGoalAccess) (PersonalTurn, bool, error) {
	turn, err := s.PersonalTurn(ctx, turnID, access)
	if err != nil {
		return PersonalTurn{}, false, err
	}
	if !validPersonalHandoffPurpose(req.Purpose) || turn.Status != "succeeded" || turn.Answer == nil || req.ProjectRevision < 0 || !hexDigest(req.ProjectContextDigest) || !validRunKey(req.IdempotencyKey) || req.Message == "" || len(req.Message) > 16000 || !validArtifactStoredText(req.Message) {
		return PersonalTurn{}, false, ErrPersonalChanged
	}
	selected := false
	for _, id := range turn.Context.Preferences.ProjectIDs {
		selected = selected || id == req.ProjectID
	}
	if !selected {
		return PersonalTurn{}, false, ErrPortfolioGoalInvalid
	}
	if err := access(ctx, req.ProjectID, req.Purpose == PersonalHandoffWorkRequest); err != nil {
		return PersonalTurn{}, false, err
	}
	scope, err := s.PersonalScope(ctx)
	if err != nil {
		return PersonalTurn{}, false, err
	}
	digest := reviewDigest(req)
	s.personalMu.Lock()
	defer s.personalMu.Unlock()
	row, err := s.readPersonal(scope)
	if err != nil {
		return PersonalTurn{}, false, err
	}
	for i := range row.Turns {
		candidate := &row.Turns[i]
		if candidate.ID != turnID {
			continue
		}
		if candidate.Handoff != nil {
			if candidate.Handoff.RequestDigest != digest {
				return PersonalTurn{}, false, ErrPersonalChanged
			}
			return *candidate, true, nil
		}
		if row.Revision != req.ExpectedRevision || candidate.Status != "succeeded" {
			return PersonalTurn{}, false, ErrPersonalChanged
		}
		candidate.Handoff = &PersonalHandoff{Purpose: req.Purpose, ID: newID("personal-handoff-"), ProjectID: req.ProjectID, Message: req.Message, ProjectRevision: req.ProjectRevision, ProjectContextDigest: req.ProjectContextDigest, ProjectRequestKey: "personal-" + reviewDigest([]string{scope, turnID, req.IdempotencyKey}), RequestDigest: digest, Status: "dispatching"}
		if err := s.savePersonal(&row); err != nil {
			return PersonalTurn{}, false, err
		}
		return *candidate, false, nil
	}
	return PersonalTurn{}, false, ErrPersonalUnavailable
}
func (s *Server) CompletePersonalHandoff(ctx context.Context, turnID, handoffID, projectTurnID, projectJobID string, access PortfolioGoalAccess) error {
	turn, err := s.PersonalTurn(ctx, turnID, access)
	if err != nil {
		return err
	}
	if turn.Handoff == nil || turn.Handoff.ID != handoffID || projectTurnID != "" && (!prototypeReleaseID.MatchString(projectTurnID) || !prototypeReleaseID.MatchString(projectJobID)) {
		return ErrPersonalChanged
	}
	scope, err := s.PersonalScope(ctx)
	if err != nil {
		return err
	}
	s.personalMu.Lock()
	defer s.personalMu.Unlock()
	row, err := s.readPersonal(scope)
	if err != nil {
		return err
	}
	if found, err := s.completeArchivedPersonalHandoff(&row, turnID, handoffID, projectTurnID, projectJobID); found || err != nil {
		return err
	}
	for i := range row.Turns {
		if row.Turns[i].ID != turnID {
			continue
		}
		handoff := row.Turns[i].Handoff
		if handoff == nil || handoff.ID != handoffID {
			return ErrPersonalChanged
		}
		if handoff.Status == "forwarded" {
			if handoff.ProjectTurnID == projectTurnID && handoff.ProjectJobID == projectJobID {
				return nil
			}
			return ErrPersonalChanged
		}
		if projectTurnID == "" {
			handoff.Status = "unknown"
		} else {
			handoff.Status, handoff.ProjectTurnID, handoff.ProjectJobID = "forwarded", projectTurnID, projectJobID
		}
		return s.savePersonal(&row)
	}
	return ErrPersonalUnavailable
}

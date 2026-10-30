package product

import (
	"context"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func emptyGTMContent() GTMContent {
	return GTMContent{PricingAssumptions: []string{}, Segments: []GTMSegment{}, Competitors: []GTMCompetitor{}, Channels: []GTMChannel{}, Milestones: []GTMMilestone{}, Experiments: []GTMExperiment{}, ArtifactLinks: []GTMArtifactLink{}}
}

func validGTMOrchestratorOrigin(origin *GTMOrchestratorOrigin) bool {
	return origin == nil || len(origin.TurnID) > 5 && len(origin.TurnID) <= 128 && prototypeReleaseID.MatchString(origin.TurnID) && origin.TurnID[:5] == "turn-" && hexDigest(origin.ProposalDigest) && hexDigest(origin.ContextDigest)
}

func gtmOrchestratorAuthority(ctx context.Context, projectID string, edit bool) error {
	if _, ok := authenticatedContextActor(ctx); !ok {
		return ErrOrchestratorInvalid
	}
	if access, scoped := management.TeamAccessFromContext(ctx); scoped {
		if access.ProjectID != projectID || !access.Permissions.Read || edit && !access.Permissions.Edit {
			return ErrOrchestratorConflict
		}
	}
	return nil
}

// Caller holds s.mu. Capture only this project's private saved draft and a
// bounded set of exact retained artifact references, never another account's.
func (s *Server) orchestratorGTMContextLocked(ctx context.Context, p project.Project) (*OrchestratorGTMContext, error) {
	if _, ok := authenticatedContextActor(ctx); !ok {
		return nil, nil
	}
	if err := gtmOrchestratorAuthority(ctx, p.ID, false); err != nil {
		return nil, err
	}
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	_, ledger, err := s.readGTMLedger(p)
	if err != nil {
		return nil, err
	}
	source, _, _, _, err := s.artifactSource(ctx, p, "")
	if err != nil {
		return nil, err
	}
	value := &OrchestratorGTMContext{Content: emptyGTMContent(), SourceDigest: reviewDigest(source), ArtifactLinks: []GTMArtifactLink{}}
	if len(ledger.Revisions) > 0 {
		last := ledger.Revisions[len(ledger.Revisions)-1]
		value.Revision, value.Digest, value.Content = last.Revision, last.Digest, last.Content
	}
	_, artifacts, err := s.readArtifactLedger(p)
	if err != nil {
		return nil, err
	}
	// Prefer saved exact links, then newest retained versions. A positive
	// omitted count is explicit: the model cannot invent the missing versions.
	seen := map[string]bool{}
	add := func(link GTMArtifactLink) {
		key := reviewDigest(link)
		if seen[key] {
			return
		}
		seen[key] = true
		if len(value.ArtifactLinks) < 50 {
			value.ArtifactLinks = append(value.ArtifactLinks, link)
		} else {
			value.ArtifactsOmitted++
		}
	}
	retained := map[string]bool{}
	for _, record := range artifacts.Records {
		for _, rev := range record.Revisions {
			retained[reviewDigest(GTMArtifactLink{ID: record.ID, Revision: rev.Revision, Digest: rev.Digest})] = true
		}
	}
	for _, saved := range value.Content.ArtifactLinks {
		if retained[reviewDigest(saved)] {
			add(saved)
		}
	}
	for _, record := range artifacts.Records {
		for i := len(record.Revisions) - 1; i >= 0; i-- {
			rev := record.Revisions[i]
			add(GTMArtifactLink{ID: record.ID, Revision: rev.Revision, Digest: rev.Digest})
		}
	}
	return value, nil
}

func validateOrchestratorGTM(snapshot OrchestratorContext, content *GTMContent) error {
	if snapshot.GTM == nil || content == nil || !hexDigest(snapshot.GTM.SourceDigest) || validateGTM(*content) != nil {
		return ErrOrchestratorInvalid
	}
	requirements, artifacts := map[string]bool{}, map[string]bool{}
	for _, r := range snapshot.Specification.Requirements {
		requirements[r.ID] = true
	}
	for _, link := range snapshot.GTM.ArtifactLinks {
		artifacts[reviewDigest(link)] = true
	}
	for _, segment := range content.Segments {
		for _, id := range segment.RequirementIDs {
			if !requirements[id] {
				return ErrOrchestratorInvalid
			}
		}
	}
	for _, link := range content.ArtifactLinks {
		if !artifacts[reviewDigest(link)] {
			return ErrOrchestratorInvalid
		}
	}
	previous, proposed := map[string]GTMExperiment{}, map[string]GTMExperiment{}
	for _, experiment := range snapshot.GTM.Content.Experiments {
		previous[experiment.ID] = experiment
	}
	for _, experiment := range content.Experiments {
		proposed[experiment.ID] = experiment
		old, exists := previous[experiment.ID]
		if !exists && (experiment.Status != "planned" || experiment.Result != "") || exists && (experiment.Status != old.Status || experiment.Result != old.Result) {
			return ErrOrchestratorInvalid
		}
	}
	// A model amendment cannot erase reported outcomes, either. Humans can
	// edit experiment records in the existing explicit GTM editor.
	for _, old := range previous {
		if _, exists := proposed[old.ID]; !exists && (old.Status != "planned" || old.Result != "") {
			return ErrOrchestratorInvalid
		}
	}
	return nil
}

func gtmOrchestratorRequest(row GTMWorkspace, baseDigest string) gtmSaveRequest {
	return gtmSaveRequest{ExpectedRevision: row.Revision - 1, ExpectedDigest: baseDigest, ExpectedSourceDigest: row.SourceDigest, IdempotencyKey: "orchestrator-" + row.OrchestratorOrigin.TurnID, Content: row.Content}
}

func validOrchestratorGTMPending(turn storedOrchestratorTurn) bool {
	row := turn.ApplyGTM
	if row == nil {
		return turn.ApplyGTMBaseDigest == "" && (turn.Answer == nil || turn.Answer.Proposal == nil || turn.Answer.Proposal.Target != "gtm" || turn.ApplyIntent == "" && turn.AppliedRevision == 0)
	}
	if turn.ApplyIntent != "" && turn.ApplyIntent != "applying" || turn.AppliedRevision < 0 || turn.AppliedRevision > 0 && turn.AppliedRevision != row.Revision || turn.ApplyIntent == "applying" && turn.AppliedRevision != 0 {
		return false
	}
	if turn.Answer == nil || turn.Answer.Proposal == nil || turn.Answer.Proposal.Target != "gtm" || turn.ApplyIntent != "applying" && turn.AppliedRevision == 0 || row.OrchestratorOrigin == nil || !validGTMOrchestratorOrigin(row.OrchestratorOrigin) || row.OrchestratorOrigin.TurnID != turn.ID || row.ProjectID != turn.ProjectID || row.Revision < 1 || row.Revision != turn.ApplyTargetRevision || reviewDigest(*row) != turn.ApplyTargetDigest || row.Digest != gtmDigest(*row) || validateGTM(row.Content) != nil || !validArtifactSource(row.Source) || row.SourceDigest != reviewDigest(row.Source) || row.Actor.ID == "" || row.Actor.DisplayName == "" {
		return false
	}
	if row.Revision == 1 && turn.ApplyGTMBaseDigest != "" || row.Revision > 1 && !hexDigest(turn.ApplyGTMBaseDigest) {
		return false
	}
	if _, err := time.Parse(time.RFC3339Nano, row.At); err != nil {
		return false
	}
	req := gtmOrchestratorRequest(*row, turn.ApplyGTMBaseDigest)
	return row.RequestKeyDigest == reviewDigest([]string{"orchestrator-gtm", row.ProjectID, turn.ID}) && row.RequestDigest == reviewDigest(struct {
		Request gtmSaveRequest
		Origin  *GTMOrchestratorOrigin
	}{req, row.OrchestratorOrigin})
}

// The private intent carries the exact candidate, including timestamp and actor,
// before canonical persistence. Both failure boundaries can resume that same
// row; a guessed manual idempotency key can never manufacture its origin.
func (s *Server) beginOrchestratorGTMLocked(ctx context.Context, p project.Project, saved *orchestratorLedger, index int, snapshot OrchestratorContext, answer OrchestratorAnswer) error {
	if err := gtmOrchestratorAuthority(ctx, p.ID, true); err != nil {
		return err
	}
	if snapshot.GTM == nil || answer.Proposal == nil || answer.Proposal.GTM == nil {
		return ErrOrchestratorInvalid
	}
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	turn := &saved.Turns[index]
	actor, _ := authenticatedContextActor(ctx)
	origin := &GTMOrchestratorOrigin{TurnID: turn.ID, ProposalDigest: reviewDigest(answer.Proposal), ContextDigest: snapshot.Digest}
	req := gtmSaveRequest{ExpectedRevision: snapshot.GTM.Revision, ExpectedDigest: snapshot.GTM.Digest, ExpectedSourceDigest: snapshot.GTM.SourceDigest, IdempotencyKey: "orchestrator-" + turn.ID, Content: *answer.Proposal.GTM}
	prepared, err := s.prepareGTMRevisionLocked(ctx, p, actor, req, origin)
	if err != nil {
		return ErrOrchestratorConflict
	}
	if prepared.Replay {
		return ErrOrchestratorConflict
	}
	turn.ApplyGTM = &prepared.Row
	turn.ApplyGTMBaseDigest = snapshot.GTM.Digest
	turn.ApplyIntent, turn.ApplyTargetRevision, turn.ApplyTargetDigest = "applying", prepared.Row.Revision, reviewDigest(prepared.Row)
	saved.Revision++
	if err := s.saveOrchestrator(*saved); err != nil {
		return err
	}
	if err := persistGTMRevisionLocked(ctx, prepared); err != nil {
		return err
	}
	return s.finishOrchestratorGTMLocked(ctx, p, saved, index)
}

func (s *Server) validateGTMApplyOriginLocked(p project.Project, turn storedOrchestratorTurn) error {
	if !validOrchestratorGTMPending(turn) || turn.ApplyGTM == nil {
		return ErrOrchestratorInvalid
	}
	ledger, err := s.readCoordination(p)
	if err != nil {
		return err
	}
	intent := coordinationIntent(&ledger, turn.ID)
	if intent == nil {
		var archived *storedCoordinationIntent
		archived, _, err = s.archivedCoordinationIntent(p, ledger, func(id coordinationArchivedIdentity) bool { return id.TurnID == turn.ID })
		if err != nil {
			return err
		}
		intent = archived
	}
	origin := turn.ApplyGTM.OrchestratorOrigin
	if intent == nil {
		if s.coordinatorAuthorize != nil || origin.ContextDigest != turn.ContextDigest || origin.ProposalDigest != reviewDigest(turn.Answer.Proposal) {
			return ErrOrchestratorConflict
		}
	} else if intent.ApplyBaseContextDigest != origin.ContextDigest || intent.CandidateProposalDigest != origin.ProposalDigest {
		return ErrOrchestratorConflict
	}
	return nil
}

func (s *Server) confirmArchivedOrchestratorGTMLocked(ctx context.Context, p project.Project, turn storedOrchestratorTurn) error {
	if err := gtmOrchestratorAuthority(ctx, p.ID, true); err != nil {
		return err
	}
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	if err := s.validateGTMApplyOriginLocked(p, turn); err != nil {
		return err
	}
	actor, _ := authenticatedContextActor(ctx)
	if actor.ID != turn.ApplyGTM.Actor.ID {
		return ErrOrchestratorConflict
	}
	_, ledger, err := s.readGTMLedger(p)
	if err != nil {
		return err
	}
	for _, row := range ledger.Revisions {
		if row.RequestKeyDigest == turn.ApplyGTM.RequestKeyDigest && reviewDigest(row) == turn.ApplyTargetDigest {
			return nil
		}
	}
	return ErrOrchestratorConflict
}

func (s *Server) resumeOrchestratorGTMLocked(ctx context.Context, p project.Project, saved *orchestratorLedger, index int) error {
	if err := gtmOrchestratorAuthority(ctx, p.ID, true); err != nil {
		return err
	}
	s.teamControlsMu.Lock()
	defer s.teamControlsMu.Unlock()
	turn := &saved.Turns[index]
	if err := s.validateGTMApplyOriginLocked(p, *turn); err != nil {
		return err
	}
	actor, _ := authenticatedContextActor(ctx)
	if actor.ID != turn.ApplyGTM.Actor.ID {
		return ErrOrchestratorConflict
	}
	_, ledger, err := s.readGTMLedger(p)
	if err != nil {
		return err
	}
	for _, row := range ledger.Revisions {
		if row.RequestKeyDigest == turn.ApplyGTM.RequestKeyDigest {
			if reviewDigest(row) != turn.ApplyTargetDigest {
				return ErrOrchestratorConflict
			}
			return s.finishOrchestratorGTMLocked(ctx, p, saved, index)
		}
	}
	if turn.AppliedRevision > 0 {
		return ErrOrchestratorConflict
	}
	// Intent persisted but canonical append did not. Revalidate source, links
	// and CAS with the very same content; preserve original actor/time exactly.
	pending := *turn.ApplyGTM
	prepared, err := s.prepareGTMRevisionLocked(ctx, p, pending.Actor, gtmOrchestratorRequest(pending, turn.ApplyGTMBaseDigest), pending.OrchestratorOrigin)
	if err != nil || prepared.Replay {
		return ErrOrchestratorConflict
	}
	prepared.Row.At = pending.At
	prepared.Row.Digest = gtmDigest(prepared.Row)
	if reviewDigest(prepared.Row) != turn.ApplyTargetDigest {
		return ErrOrchestratorConflict
	}
	prepared.Ledger.Revisions[len(prepared.Ledger.Revisions)-1] = pending
	if err := persistGTMRevisionLocked(ctx, prepared); err != nil {
		return err
	}
	return s.finishOrchestratorGTMLocked(ctx, p, saved, index)
}

func (s *Server) finishOrchestratorGTMLocked(ctx context.Context, p project.Project, saved *orchestratorLedger, index int) error {
	turn := &saved.Turns[index]
	if turn.AppliedRevision == 0 || turn.ApplyIntent != "" {
		turn.AppliedRevision, turn.ApplyIntent = turn.ApplyTargetRevision, ""
		turn.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
		saved.Revision++
		if err := s.saveOrchestrator(*saved); err != nil {
			return err
		}
	}
	return s.finishCoordinationApplyLocked(ctx, p, *turn)
}

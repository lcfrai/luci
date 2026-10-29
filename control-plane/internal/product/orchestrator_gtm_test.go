package product

import (
	"encoding/json"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func gtmOrchestratorTurnForTest(t *testing.T, s *Server, id string) (OrchestratorTurn, OrchestratorContext) {
	t.Helper()
	turn, snapshot := reserveOrchestratorTest(t, s, id, orchestratorActor(t, "alice"), "Draft a small pilot GTM strategy using saved project context", "")
	content := gtmFixtureContent()
	answer := OrchestratorAnswer{Summary: "A pilot strategy to review", Options: []string{}, Impact: []string{"GTM assumptions only"}, Proposal: &OrchestratorProposal{Target: "gtm", Summary: "Draft the audience and pilot", GTM: &content}}
	if err := s.CompleteOrchestratorTurn(id, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	return turn, snapshot
}

func TestOrchestratorGTMDraftApplyIsPrivateRevisionedAndReplayable(t *testing.T) {
	s, mux := journeyFixture(t)
	projectView := createJourneyProject(t, mux, "gtm-draft-apply")
	ctx := orchestratorActor(t, "alice")
	turn, snapshot := gtmOrchestratorTurnForTest(t, s, projectView.ID)
	p, err := s.resolveProject(projectView.ID)
	if err != nil {
		t.Fatal(err)
	}
	before, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); err != nil {
		t.Fatal(err)
	}
	_, ledger, err := s.readGTMLedger(p)
	if err != nil || len(ledger.Revisions) != 1 {
		t.Fatal("missing private revision", err)
	}
	row := ledger.Revisions[0]
	if row.OrchestratorOrigin == nil || row.OrchestratorOrigin.TurnID != turn.ID || row.OrchestratorOrigin.ContextDigest != snapshot.Digest || row.Actor.ID != "alice" || row.Content.Offer != "A small paid pilot" {
		t.Fatal("lost exact proposal origin", row)
	}
	after, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("GTM apply rewrote approved project documents", err)
	}
	if err := s.applyOrchestratorProposal(ctx, p.ID, turn.ID, view.Revision, view.Context.Digest); err != nil {
		t.Fatal("same receipt replay", err)
	}
	_, ledger, err = s.readGTMLedger(p)
	if err != nil || len(ledger.Revisions) != 1 {
		t.Fatal("replay duplicated strategy", err)
	}
	fresh, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil || fresh.Context.GTM == nil || fresh.Context.GTM.Revision != 1 || fresh.Context.Digest == snapshot.Digest {
		t.Fatal("GTM not part of saved model context", err)
	}
	requirements, tasks, all := coordinationImpact(storedOrchestratorTurn{OrchestratorTurn: fresh.Turns[0]})
	if len(requirements) != 0 || len(tasks) != 0 || all {
		t.Fatal("commercial draft fabricated implementation impact")
	}
}

func TestOrchestratorGTMRecoveryResumesExactTurnAtBothWriteBoundaries(t *testing.T) {
	for _, boundary := range []string{"intent_before_append", "append_before_receipt"} {
		t.Run(boundary, func(t *testing.T) {
			s, mux := journeyFixture(t)
			projectView := createJourneyProject(t, mux, "gtm-recover-"+boundary)
			ctx := orchestratorActor(t, "alice")
			turn, snapshot := gtmOrchestratorTurnForTest(t, s, projectView.ID)
			p, _ := s.resolveProject(projectView.ID)
			saved, err := s.readOrchestrator(p)
			if err != nil {
				t.Fatal(err)
			}
			index := len(saved.Turns) - 1
			stored := &saved.Turns[index]
			actor, _ := authenticatedContextActor(ctx)
			req := gtmSaveRequest{ExpectedSourceDigest: snapshot.GTM.SourceDigest, IdempotencyKey: "orchestrator-" + turn.ID, Content: *stored.Answer.Proposal.GTM}
			origin := &GTMOrchestratorOrigin{TurnID: turn.ID, ProposalDigest: reviewDigest(stored.Answer.Proposal), ContextDigest: snapshot.Digest}
			s.mu.Lock()
			s.teamControlsMu.Lock()
			prepared, err := s.prepareGTMRevisionLocked(ctx, p, actor, req, origin)
			s.teamControlsMu.Unlock()
			s.mu.Unlock()
			if err != nil {
				t.Fatal(err)
			}
			stored.ApplyGTM = &prepared.Row
			stored.ApplyIntent, stored.ApplyTargetRevision, stored.ApplyTargetDigest = "applying", prepared.Row.Revision, reviewDigest(prepared.Row)
			saved.Revision++
			if err := s.saveOrchestrator(saved); err != nil {
				t.Fatal(err)
			}
			if boundary == "append_before_receipt" {
				if err := persistGTMRevisionLocked(ctx, prepared); err != nil {
					t.Fatal(err)
				}
			}
			revoked := management.TeamAccess{ProjectID: p.ID}
			revoked.Permissions.Read = true
			if err := s.applyOrchestratorProposal(management.WithTeamAccess(ctx, revoked), p.ID, turn.ID, saved.Revision, snapshot.Digest); !errors.Is(err, ErrOrchestratorConflict) {
				t.Fatal("revoked edit could resume a durable intent", err)
			}
			if err := s.applyOrchestratorProposal(orchestratorActor(t, "bob"), p.ID, turn.ID, saved.Revision, snapshot.Digest); !errors.Is(err, ErrOrchestratorConflict) {
				t.Fatal("another editor adopted the original actor's apply", err)
			}
			// A caller's old view cannot cause a second append during recovery.
			for i := 0; i < 2; i++ {
				if err := s.applyOrchestratorProposal(ctx, p.ID, turn.ID, 0, strings.Repeat("c", 64)); err != nil {
					t.Fatal("resume", err)
				}
			}
			_, ledger, err := s.readGTMLedger(p)
			if err != nil || len(ledger.Revisions) != 1 || reviewDigest(ledger.Revisions[0]) != reviewDigest(prepared.Row) {
				t.Fatal("did not retain original actor/time/row", err)
			}
			saved, err = s.readOrchestrator(p)
			if err != nil || saved.Turns[index].ApplyIntent != "" || saved.Turns[index].AppliedRevision != 1 {
				t.Fatal("unsettled receipt", err)
			}
		})
	}
}

func TestOrchestratorGTMDraftCannotApplyAfterProjectSourceChanges(t *testing.T) {
	s, mux := journeyFixture(t)
	projectView := createJourneyProject(t, mux, "gtm-project-source")
	ctx := orchestratorActor(t, "alice")
	turn, _ := gtmOrchestratorTurnForTest(t, s, projectView.ID)
	p, _ := s.resolveProject(projectView.ID)
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.Brief.Purpose = "A different product scope"
	state.Brief.Revision++
	if err := writeJourney(p.Path, state); err != nil {
		t.Fatal(err)
	}
	current, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.applyOrchestratorProposal(ctx, p.ID, turn.ID, current.Revision, current.Context.Digest); !errors.Is(err, ErrOrchestratorConflict) {
		t.Fatal("old draft adopted unrelated current source", err)
	}
	_, ledger, err := s.readGTMLedger(p)
	if err != nil || len(ledger.Revisions) != 0 {
		t.Fatal("stale proposal wrote a GTM revision", err)
	}
}

func TestOrchestratorGTMRequiresCurrentCASAndCannotUseManualOrigin(t *testing.T) {
	s, mux := journeyFixture(t)
	projectView := createJourneyProject(t, mux, "gtm-cas")
	ctx := orchestratorActor(t, "alice")
	turn, snapshot := gtmOrchestratorTurnForTest(t, s, projectView.ID)
	before, _ := s.OrchestratorView(ctx, projectView.ID, 0)
	manual := gtmFixtureContent()
	manual.Offer = "A person's separate draft"
	h := coordinationHandler(s, "alice")
	// This deliberately guesses the model turn's public request key. The
	// manual route has no origin field and uses a different key domain.
	controlTestRequest(t, h, "POST", "/v1/product/projects/"+projectView.ID+"/distribute/gtm", map[string]any{"expectedRevision": 0, "expectedDigest": "", "expectedSourceDigest": snapshot.GTM.SourceDigest, "idempotencyKey": "orchestrator-" + turn.ID, "content": manual}, 200)
	if err := s.applyOrchestratorProposal(ctx, projectView.ID, turn.ID, before.Revision, before.Context.Digest); !errors.Is(err, ErrOrchestratorConflict) {
		t.Fatal("stale model overwrote manual revision", err)
	}
	p, _ := s.resolveProject(projectView.ID)
	_, ledger, err := s.readGTMLedger(p)
	if err != nil || len(ledger.Revisions) != 1 || ledger.Revisions[0].OrchestratorOrigin != nil || ledger.Revisions[0].Content.Offer != manual.Offer {
		t.Fatal("manual row falsely adopted as model apply", err)
	}
	access := management.TeamAccess{ProjectID: p.ID}
	access.Permissions.Read = true
	revoked := management.WithTeamAccess(ctx, access)
	if err := gtmOrchestratorAuthority(revoked, p.ID, true); err == nil {
		t.Fatal("read-only membership could apply")
	}
}

func TestOrchestratorGTMRejectsInventedEvidenceAndMultipleTargets(t *testing.T) {
	snapshot := OrchestratorContext{ProjectID: "project-1", GTM: &OrchestratorGTMContext{SourceDigest: strings.Repeat("a", 64), Content: emptyGTMContent(), ArtifactLinks: []GTMArtifactLink{}}}
	for _, tc := range []struct {
		name   string
		mutate func(*OrchestratorAnswer)
	}{
		{"unknown_requirement", func(a *OrchestratorAnswer) { a.Proposal.GTM.Segments[0].RequirementIDs = []string{"missing"} }},
		{"invented_artifact", func(a *OrchestratorAnswer) {
			a.Proposal.GTM.ArtifactLinks = []GTMArtifactLink{{ID: "artifact-missing", Revision: 1, Digest: strings.Repeat("b", 64)}}
		}},
		{"fabricated_recorded_experiment", func(a *OrchestratorAnswer) { a.Proposal.GTM.Experiments[0].Status = "recorded" }},
		{"fabricated_result", func(a *OrchestratorAnswer) { a.Proposal.GTM.Experiments[0].Result = "Ten customers paid" }},
		{"mixed_target", func(a *OrchestratorAnswer) { a.Proposal.Plan = &ProjectPlan{} }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := gtmFixtureContent()
			a := OrchestratorAnswer{Summary: "A draft", Proposal: &OrchestratorProposal{Target: "gtm", Summary: "Pilot", GTM: &c}}
			tc.mutate(&a)
			if ValidateOrchestratorAnswer(snapshot, a) == nil {
				t.Fatal("untrusted evidence/target accepted")
			}
		})
	}
	content := gtmFixtureContent()
	if err := validateOrchestratorGTM(snapshot, &content); err != nil {
		t.Fatal("honest planned experiment rejected", err)
	}
	snapshot.GTM.Content = gtmFixtureContent()
	snapshot.GTM.Content.Experiments[0].Status = "recorded"
	snapshot.GTM.Content.Experiments[0].Result = "Participant said no"
	content = emptyGTMContent()
	if validateOrchestratorGTM(snapshot, &content) == nil {
		t.Fatal("model erased an existing reported result")
	}
}

func TestOrchestratorGTMRebasePreservesDisjointChangesAndRefusesSourceDrift(t *testing.T) {
	base := OrchestratorContext{ProjectID: "project-1", GTM: &OrchestratorGTMContext{SourceDigest: strings.Repeat("a", 64), Content: gtmFixtureContent(), ArtifactLinks: []GTMArtifactLink{}}}
	proposed := gtmFixtureContent()
	proposed.Offer = "A narrower pilot"
	turn := storedOrchestratorTurn{OrchestratorTurn: OrchestratorTurn{Context: base, Answer: &OrchestratorAnswer{Proposal: &OrchestratorProposal{Target: "gtm", Summary: "Narrow pilot", GTM: &proposed}}}}
	current := base
	copyGTM := *base.GTM
	current.GTM = &copyGTM
	current.GTM.Content.Positioning = "A person's new positioning"
	answer, conflicts := coordinationRebase(turn, current)
	if len(conflicts) != 0 || answer.Proposal.GTM.Offer != proposed.Offer || answer.Proposal.GTM.Positioning != current.GTM.Content.Positioning {
		t.Fatal("disjoint merge lost content", conflicts)
	}
	current.GTM.Content.Offer = "A conflicting offer"
	if _, conflicts = coordinationRebase(turn, current); len(conflicts) == 0 {
		t.Fatal("same field conflict silently resolved")
	}
	current = base
	copyGTM = *base.GTM
	current.GTM = &copyGTM
	current.GTM.SourceDigest = strings.Repeat("b", 64)
	if _, conflicts = coordinationRebase(turn, current); !reflect.DeepEqual(conflicts, []string{"gtm.source"}) {
		t.Fatal("changed project context silently rebased", conflicts)
	}
}

func TestOrchestratorGTMLegacyOptionalEncodingRemainsAbsent(t *testing.T) {
	raw, err := json.Marshal(OrchestratorProposal{Target: "plan", Plan: &ProjectPlan{}})
	if err != nil || strings.Contains(string(raw), `"gtm"`) {
		t.Fatal("legacy proposal encoding changed", err)
	}
	raw, err = json.Marshal(OrchestratorContext{})
	if err != nil || strings.Contains(string(raw), `"gtm"`) {
		t.Fatal("legacy context encoding changed", err)
	}
	raw, err = json.Marshal(GTMWorkspace{})
	if err != nil || strings.Contains(string(raw), `"orchestratorOrigin"`) {
		t.Fatal("legacy GTM digest input changed", err)
	}
}

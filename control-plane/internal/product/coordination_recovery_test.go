package product

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCoordinationMergeDoesNotResurrectOmittedTaskFields(t *testing.T) {
	turn, current := coordinationMergeFixture("plan")
	turn.Context.Plan.Tasks[0].Acceptance = "Old acceptance"
	turn.Answer.Proposal.Plan.Tasks[0].Acceptance = "Old acceptance"
	turn.Context.Plan.Tasks[0].WritePaths = []string{"old.go"}
	turn.Answer.Proposal.Plan.Tasks[0].WritePaths = []string{"old.go"}
	turn.Answer.Proposal.Plan.Tasks[1].Title = "New second title"
	answer, conflicts := coordinationRebase(turn, current)
	if len(conflicts) != 0 || answer.Proposal.Plan.Tasks[0].Acceptance != "" || len(answer.Proposal.Plan.Tasks[0].WritePaths) != 0 || answer.Proposal.Plan.Tasks[1].Title != "New second title" {
		t.Fatal("omitted current task fields resurrected")
	}
}

func TestCoordinationLargePlanReplacementMarksAllAffectedWithinBound(t *testing.T) {
	turn, _ := coordinationMergeFixture("plan")
	turn.Context.Plan.Tasks = nil
	turn.Answer.Proposal.Plan.Tasks = nil
	for i := 0; i < 200; i++ {
		turn.Context.Plan.Tasks = append(turn.Context.Plan.Tasks, ProjectTask{ID: fmt.Sprintf("old-%d", i), Title: "Old", Status: "todo"})
		turn.Answer.Proposal.Plan.Tasks = append(turn.Answer.Proposal.Plan.Tasks, ProjectTask{ID: fmt.Sprintf("new-%d", i), Title: "New", Status: "todo"})
	}
	_, tasks, all := coordinationImpact(turn)
	if !all || len(tasks) > 200 {
		t.Fatal("impact overflows durable/public bound or hides affected work")
	}
}

func TestCoordinationRecoveryCannotOmitAnotherKnownPendingOrigin(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "coordination-lost-origins")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	a := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "Private")
	coordinationProposalTest(t, s, p.ID, "bob", "purpose", "Public")
	path, _ := s.coordinationPath(p.ID, false)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	coordinationApplyTest(t, s, p.ID, "alice", a.ID, 409)
	view, err := s.CoordinationViewForProject(orchestratorActor(t, "alice"), p.ID)
	if err != nil || len(view.Intents) != 2 || len(view.Conflicts) == 0 {
		t.Fatal("known opposition lost during recovery", err)
	}
}

func TestCoordinationRotationRetainsAuthoritativeOrigins(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "coordination-rotation")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	ctx := orchestratorActor(t, "alice")
	origin := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "Keep this proposed idea")
	for i := 0; i < 12; i++ {
		turn, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Explain current context", "")
		answer := OrchestratorAnswer{Summary: "A read-only explanation"}
		if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
			t.Fatal(err)
		}
	}
	view, err := s.OrchestratorView(ctx, p.ID, 0)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.rotateOrchestratorHistory(ctx, p.ID, orchestratorRotateRequest{ExpectedRevision: view.Revision, ExpectedStorageDigest: view.StorageDigest, AcknowledgeExportedHistory: true}); err != nil {
		t.Fatal(err)
	}
	coord, err := s.CoordinationViewForProject(ctx, p.ID)
	if err != nil || len(coord.Intents) != 1 || coord.Intents[0].TurnID != origin.ID {
		t.Fatal("rotation lost authoritative intent", err)
	}
	coordinationApplyTest(t, s, p.ID, "alice", origin.ID, 200)
}

func TestCoordinationCorruptCausalRowsCannotSatisfyDependencies(t *testing.T) {
	for _, fault := range []string{"fabricated-applied", "fabricated-superseded", "missing-origin", "forged-origin-actor", "duplicate-event"} {
		t.Run(fault, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "coordination-corruption")
			s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
			ctx := orchestratorActor(t, "alice")
			origin := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "A retained idea")
			saved, err := s.readCoordination(p)
			if err != nil {
				t.Fatal(err)
			}
			switch fault {
			case "fabricated-applied":
				saved.Intents[0].Status = "applied"
				saved.Intents[0].AppliedRevision = 1
				saved.Intents[0].AppliedTargetDigest = strings.Repeat("a", 64)
				saved.Intents[0].ApplyBaseContextDigest = strings.Repeat("b", 64)
				saved.Intents[0].CandidateProposalDigest = strings.Repeat("c", 64)
			case "fabricated-superseded":
				saved.Intents[0].Status = "superseded"
			case "forged-origin-actor":
				saved.Intents[0].Actor.ID = "bob"
				saved.Intents[0].LuciID = coordinationLuciID(saved.AccountScope, "bob")
			case "duplicate-event":
				event := saved.Events[0]
				event.Sequence = 2
				event.PreviousEventID = event.ID
				saved.Events = append(saved.Events, event)
			case "missing-origin":
				turns, _ := s.readOrchestrator(p)
				turns.Turns = nil
				if err = s.saveOrchestrator(turns); err != nil {
					t.Fatal(err)
				}
			}
			if err = s.saveCoordination(saved); err != nil {
				t.Fatal(err)
			}
			if _, err = s.CoordinationViewForProject(ctx, p.ID); err == nil {
				t.Fatal("corrupt authoritative row accepted", origin.ID)
			}
		})
	}
}

func TestCoordinationMissingCompletionReceiptNeedsAuthorityAndExactRecovery(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "coordination-missing-write")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	ctx := orchestratorActor(t, "alice")
	origin := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "Recover the completed proposal")
	path, err := s.coordinationPath(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.Remove(path); err != nil {
		t.Fatal(err)
	}
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return errors.New("revoked") })
	view, _ := s.OrchestratorView(ctx, p.ID, 0)
	if err = s.applyOrchestratorProposal(ctx, p.ID, origin.ID, view.Revision, view.Context.Digest); !errors.Is(err, errCoordinatorAuthority) {
		t.Fatal("missing intent bypassed current authority", err)
	}
	if _, err = os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("refused action wrote intent")
	}
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	coordinationApplyTest(t, s, p.ID, "alice", origin.ID, 200)
	coord, err := s.CoordinationViewForProject(ctx, p.ID)
	if err != nil || len(coord.Events) != 2 || coord.Intents[0].Status != "applied" {
		t.Fatal("missing intent was not recovered exactly", err)
	}
}

func TestCoordinationUnconfirmedCanonicalApplyCannotRebaseAndReplay(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "coordination-unconfirmed")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	ctx := orchestratorActor(t, "alice")
	origin := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "Exactly one application")
	view, _ := s.OrchestratorView(ctx, p.ID, 0)
	export := filepath.Join(p.Path, ".luci", "design.json")
	if err := os.MkdirAll(filepath.Dir(export), 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(export); err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	if err := os.Mkdir(export, 0700); err != nil {
		t.Fatal(err)
	}
	if err := s.applyOrchestratorProposal(ctx, p.ID, origin.ID, view.Revision, view.Context.Digest); err == nil {
		t.Fatal("fixture did not fail after canonical write")
	}
	state, err := readJourney(p.Path, p.ID)
	if err != nil || state.Specification == nil || state.Specification.Revision != 1 {
		t.Fatal("canonical fixture not reached", err)
	}
	if err = os.Remove(export); err != nil {
		t.Fatal(err)
	}
	state.Specification.Revision = 2
	state.Specification.Audience = "Later human edit"
	if err = writeJourney(p.Path, state); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(filepath.Join(p.Path, ".luci", "state.json"))
	current, _ := s.OrchestratorView(ctx, p.ID, 0)
	if err = s.applyOrchestratorProposal(ctx, p.ID, origin.ID, current.Revision, current.Context.Digest); !errors.Is(err, ErrOrchestratorConflict) {
		t.Fatal("unconfirmed mutation reapplied", err)
	}
	after, _ := os.ReadFile(filepath.Join(p.Path, ".luci", "state.json"))
	if string(before) != string(after) {
		t.Fatal("retry modified canonical state")
	}
	saved, _ := s.readOrchestrator(p)
	turn := coordinationTurn(saved, origin.ID)
	if turn.ApplyIntent == "" || turn.ApplyTargetRevision != 1 || turn.AppliedRevision != 0 {
		t.Fatal("original uncertainty discarded")
	}
}

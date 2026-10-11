package product

import (
	"context"
	"fmt"
	"testing"
)

// The identical test compiles against the pre-retention source. All proposals
// complete and apply through the existing product lifecycle; no synthetic
// accepted rows, archive invocation or manual rotation makes room for them.
func TestCoordinationAutomaticRetentionAllowsOrdinaryProposalsPastActiveLimit(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "coordination-retention-progress")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	var first OrchestratorTurn
	for i := 0; i < 70; i++ {
		turn := coordinationProposalTest(t, s, p.ID, "alice", "purpose", fmt.Sprintf("Ordinary accepted amendment %d", i))
		if i == 0 {
			first = turn
		}
		coordinationApplyTest(t, s, p.ID, "alice", turn.ID, 200)
	}
	view := coordinationHTTPView(t, coordinationHandler(s, "alice"), p.ID)
	archives, ok := view["archives"].([]any)
	if !ok || len(archives) == 0 || len(view["intents"].([]any)) > 64 {
		t.Fatal("ordinary proposal progress did not retain bounded archive history")
	}
	if view["eventSequence"] != float64(140) {
		t.Fatal("retention duplicated or dropped an event")
	}
	current, err := s.OrchestratorView(orchestratorActor(t, "alice"), p.ID, 0)
	if err != nil || current.Context.Specification.Revision != 70 || current.Context.Specification.Purpose != "Ordinary accepted amendment 69" {
		t.Fatal("canonical amendments were lost or replayed", err)
	}
	coordinationApplyTest(t, s, p.ID, "alice", first.ID, 200)
	after, err := s.OrchestratorView(orchestratorActor(t, "alice"), p.ID, 0)
	if err != nil || after.Context.Specification.Revision != 70 {
		t.Fatal("archived apply replay changed context", err)
	}
}

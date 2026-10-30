package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"
)

func outcomeGoal(id string, projects ...string) PortfolioGoal {
	goal := portfolioGoalDraft(projects[0])
	goal.ID, goal.Revision, goal.UpdatedAt = id, 3, "2026-10-30T00:00:00Z"
	goal.Projects = []PortfolioGoalProject{}
	for _, id := range projects {
		goal.Projects = append(goal.Projects, PortfolioGoalProject{ProjectID: id, Priority: "normal", DependsOn: []string{}})
	}
	return goal
}
func outcomeRead(_ context.Context, id, _ string) (PortfolioProject, []PortfolioIntervention, error) {
	return PortfolioProject{ID: id, Status: "ready", Coordinator: PortfolioCoordinator{Status: "none"}}, []PortfolioIntervention{}, nil
}

func TestGoalOutcomeRoutesReuseAttentionAndPinOwnContinuationWithoutExecuting(t *testing.T) {
	now := time.Date(2026, 10, 30, 0, 0, 0, 0, time.UTC)
	reads := 0
	read := func(_ context.Context, id, actor string) (PortfolioProject, []PortfolioIntervention, error) {
		reads++
		if actor != "alice" {
			t.Fatal("substituted continuation owner")
		}
		row, _, _ := outcomeRead(context.Background(), id, actor)
		row.Coordinator = PortfolioCoordinator{Status: "recorded", ID: "continue-alice", Revision: 7, State: "working", RunID: "run-aaaaaaaaaaaaaaaaaaaaaaaa", UpdatedAt: now.Format(time.RFC3339Nano)}
		items := []PortfolioIntervention{
			{ID: reviewDigest("question"), ProjectID: id, Kind: "question", Reason: "A decision is unanswered.", NextAction: OrchestratorAction{ID: "answer-question", Label: "Review questions", Lens: "discover"}, Evidence: []PortfolioEvidence{{Source: "questions", ID: id}}},
			{ID: reviewDigest("unknown"), ProjectID: id, Kind: "unconfirmed", Reason: "An existing worker outcome is unconfirmed.", NextAction: OrchestratorAction{ID: "inspect-run", Label: "Review project work", Lens: "develop", RunID: "run-aaaaaaaaaaaaaaaaaaaaaaaa"}, Evidence: []PortfolioEvidence{{Source: "queue-item", ID: "queue-one", Revision: 4}}},
		}
		return row, items, nil
	}
	view := PortfolioGoalsView{Goals: []PortfolioGoal{outcomeGoal("goal-one", "p-one"), outcomeGoal("goal-two", "p-one")}}
	got, err := portfolioGoalOutcomes(homeActor("alice"), view, portfolioGoalAllow, read, now)
	if err != nil || reads != 1 || len(got.OutcomeRouting.Routes) != 2 {
		t.Fatalf("deduped read: %+v %v reads=%d", got, err, reads)
	}
	first := got.OutcomeRouting.Routes[0]
	if first.GoalID != "goal-one" || first.GoalRevision != 3 || first.Status != "unknown" || first.NextAction.ID != "inspect-run" || first.NextAction.RunID != "run-aaaaaaaaaaaaaaaaaaaaaaaa" || first.Coordinator.ID != "continue-alice" || first.Coordinator.Revision != 7 || first.Evidence[0].ID != "queue-one" || got.OutcomeRouting.UnknownProjects != 1 {
		t.Fatal("lost exact goal/owned-work identity, uncertainty or existing attention order", first)
	}
	got.OutcomeRouting.Routes[0].Coordinator.ID = "mutated-return"
	if got.OutcomeRouting.Routes[1].Coordinator.ID != "continue-alice" {
		t.Fatal("returned route aliases another goal's observation")
	}
}

func TestGoalOutcomeRoutesBoundUniqueReadsAndReportEveryOmittedLink(t *testing.T) {
	ids := []string{}
	for i := 0; i < 16; i++ {
		ids = append(ids, fmt.Sprintf("p-%02d", i))
	}
	view := PortfolioGoalsView{Goals: []PortfolioGoal{outcomeGoal("goal-a", ids...), outcomeGoal("goal-b", ids...), outcomeGoal("goal-c", ids...)}}
	reads := map[string]int{}
	read := func(ctx context.Context, id, actor string) (PortfolioProject, []PortfolioIntervention, error) {
		reads[id]++
		return outcomeRead(ctx, id, actor)
	}
	got, err := portfolioGoalOutcomes(homeActor("alice"), view, portfolioGoalAllow, read, time.Now())
	if err != nil || len(reads) != 16 || got.OutcomeRouting.ReadProjects != 16 || len(got.OutcomeRouting.Routes) != 32 || got.OutcomeRouting.OmittedRoutes != 16 || got.OutcomeRouting.Status != "partial" {
		t.Fatal("route bound/omission lost", got, err)
	}
	for _, count := range reads {
		if count != 1 {
			t.Fatal("repeated source read for another goal")
		}
	}
	other := []string{}
	for i := 16; i < 32; i++ {
		other = append(other, fmt.Sprintf("p-%02d", i))
	}
	view.Goals = []PortfolioGoal{outcomeGoal("goal-a", ids...), outcomeGoal("goal-b", other...)}
	reads = map[string]int{}
	got, err = portfolioGoalOutcomes(homeActor("alice"), view, portfolioGoalAllow, read, time.Now())
	if err != nil || len(reads) != 16 || got.OutcomeRouting.OmittedProjects != 16 || got.OutcomeRouting.OmittedRoutes != 16 {
		t.Fatal("unique project bound lost", got, err)
	}
}

func TestGoalOutcomeRoutesRecheckReadAuthorityAndWithholdRevokedGoal(t *testing.T) {
	view := PortfolioGoalsView{Goals: []PortfolioGoal{outcomeGoal("goal-one", "p-one")}}
	checks, reads := 0, 0
	access := func(_ context.Context, _ string, control bool) error {
		checks++
		if control {
			t.Fatal("read projection requested control authority")
		}
		if checks >= 2 {
			return errors.New("revoked")
		}
		return nil
	}
	read := func(ctx context.Context, id, actor string) (PortfolioProject, []PortfolioIntervention, error) {
		reads++
		return outcomeRead(ctx, id, actor)
	}
	got, err := portfolioGoalOutcomes(homeActor("alice"), view, access, read, time.Now())
	if err == nil || reads != 1 || len(got.Goals) != 0 || got.OutcomeRouting != nil {
		t.Fatal("late revoked project/goal response retained", got, err)
	}
}

func TestGoalOutcomeMissingEvidenceAndUnsupportedActionsStayUnknown(t *testing.T) {
	view := PortfolioGoalsView{Goals: []PortfolioGoal{outcomeGoal("goal-one", "p-one")}}
	got, err := portfolioGoalOutcomes(homeActor("alice"), view, portfolioGoalAllow, func(context.Context, string, string) (PortfolioProject, []PortfolioIntervention, error) {
		return PortfolioProject{}, nil, errors.New("secret internal path")
	}, time.Now())
	if err != nil || got.OutcomeRouting.UnknownProjects != 1 || got.OutcomeRouting.Routes[0].Status != "unknown" || strings.Contains(got.OutcomeRouting.Routes[0].Reason, "secret") {
		t.Fatal("missing source fabricated idle/completion or leaked an error", got, err)
	}
	row, _, _ := outcomeRead(context.Background(), "p-one", "alice")
	unsafe := PortfolioIntervention{ID: reviewDigest("unsafe"), ProjectID: "p-one", Kind: "review", Reason: "Unsafe action", NextAction: OrchestratorAction{ID: "apply", Label: "Start all workers", Lens: "develop"}, Evidence: []PortfolioEvidence{{Source: "run", ID: "run-one"}}}
	route := outcomeRoute(view.Goals[0], "p-one", row, []PortfolioIntervention{unsafe}, false, time.Now())
	if route.Status != "unknown" || route.NextAction.ID != "open-conversation" {
		t.Fatal("unsupported action became a control", route)
	}
}

func TestGoalOutcomeDerivedViewCannotChangeLegacyPrivateLedgerJSON(t *testing.T) {
	legacy := `{"version":1,"accountScope":"scope","actorId":"alice","revision":0,"goals":[],"applications":[],"partial":false}`
	var stored portfolioGoalsLedger
	if err := json.Unmarshal([]byte(legacy), &stored); err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(stored)
	if err != nil || string(data) != legacy || strings.Contains(string(data), "outcomeRouting") {
		t.Fatal("derived read fields migrated or contaminated the private store", string(data), err)
	}
}

package product

import (
	"context"
	"errors"
	"reflect"
	"testing"
)

func portfolioGoalDraft(projectID string) PortfolioGoal {
	return PortfolioGoal{Title: "Ship the booking workflow", Outcome: "Customers can book a supported appointment without staff re-entry.", Deadline: "2026-12-01", Projects: []PortfolioGoalProject{{ProjectID: projectID, Priority: "high", RequestedSlots: 2, DependsOn: []string{}}}}
}

func portfolioGoalAllow(context.Context, string, bool) error { return nil }

func TestPortfolioGoalsValidateSelectedProjectsAndDependencyIntentions(t *testing.T) {
	for name, mutate := range map[string]func(*PortfolioGoal){
		"unknown_dependency": func(g *PortfolioGoal) { g.Projects[0].DependsOn = []string{"p-missing"} },
		"self_dependency":    func(g *PortfolioGoal) { g.Projects[0].DependsOn = []string{"p-one"} },
		"duplicate_project":  func(g *PortfolioGoal) { g.Projects = append(g.Projects, g.Projects[0]) },
		"cycle": func(g *PortfolioGoal) {
			g.Projects = append(g.Projects, PortfolioGoalProject{ProjectID: "p-two", Priority: "normal", DependsOn: []string{"p-one"}})
			g.Projects[0].DependsOn = []string{"p-two"}
		},
		"missing_dependency_array":   func(g *PortfolioGoal) { g.Projects[0].DependsOn = nil },
		"invalid_date":               func(g *PortfolioGoal) { g.Deadline = "2026-02-30" },
		"unsupported_priority":       func(g *PortfolioGoal) { g.Projects[0].Priority = "urgent" },
		"capacity_over_policy_bound": func(g *PortfolioGoal) { g.Projects[0].RequestedSlots = MaxSwarmParallelTasks + 1 },
		"control_text":               func(g *PortfolioGoal) { g.Outcome = "Bad\x00outcome" },
	} {
		t.Run(name, func(t *testing.T) {
			goal := portfolioGoalDraft("p-one")
			mutate(&goal)
			if !errors.Is(validatePortfolioGoal(goal), ErrPortfolioGoalInvalid) {
				t.Fatal("invalid intention accepted")
			}
		})
	}
	valid := portfolioGoalDraft("p-one")
	valid.Projects = append(valid.Projects, PortfolioGoalProject{ProjectID: "p-two", Priority: "normal", RequestedSlots: 0, DependsOn: []string{"p-one"}})
	if err := validatePortfolioGoal(valid); err != nil {
		t.Fatal(err)
	}
}

func TestPortfolioGoalsPersistActorPrivateCASWithoutRequiringExecution(t *testing.T) {
	s, err := New(Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	viewer := func(_ context.Context, _ string, edit bool) error {
		if edit {
			t.Fatal("saving a private intention requested execution control")
		}
		return nil
	}
	goal, err := s.SavePortfolioGoal(homeActor("alice"), PortfolioGoalsSave{Goal: portfolioGoalDraft("p-one")}, viewer)
	if err != nil || goal.Revision != 1 || goal.ID == "" {
		t.Fatalf("save: %+v %v", goal, err)
	}
	view, err := s.ReadPortfolioGoals(homeActor("alice"), viewer)
	if err != nil || view.Revision != 1 || len(view.Goals) != 1 || !reflect.DeepEqual(view.Goals[0], goal) {
		t.Fatalf("reload: %+v %v", view, err)
	}
	other, err := s.ReadPortfolioGoals(homeActor("bob"), viewer)
	if err != nil || len(other.Goals) != 0 {
		t.Fatal("another actor inherited the goal", err)
	}
	goal.Title = "Preserve this edit"
	if _, err := s.SavePortfolioGoal(homeActor("alice"), PortfolioGoalsSave{ExpectedRevision: 0, Goal: goal}, viewer); !errors.Is(err, ErrPortfolioGoalConflict) {
		t.Fatal("stale collection overwrote the goal", err)
	}
	view, _ = s.ReadPortfolioGoals(homeActor("alice"), viewer)
	if view.Goals[0].Title == goal.Title {
		t.Fatal("CAS refusal changed saved intent")
	}
	denied := func(context.Context, string, bool) error { return errors.New("revoked") }
	view, err = s.ReadPortfolioGoals(homeActor("alice"), denied)
	if err != nil || !view.Partial || len(view.Goals) != 0 {
		t.Fatal("revoked selected project leaked", err)
	}
}

func TestPortfolioGoalReviewApplyChangesExactQueuePrioritiesWithoutStartingWork(t *testing.T) {
	s, mux, run, base, _, starts, _ := schedulerFixture(t)
	items := enqueueSchedulerFixture(t, mux, run, base, []string{"task-a", "task-b"})
	goal, err := s.SavePortfolioGoal(homeActor("alice"), PortfolioGoalsSave{Goal: portfolioGoalDraft(run.ProjectID)}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.ReviewPortfolioGoal(homeActor("alice"), goal.ID, goal.Revision, portfolioGoalAllow)
	if err != nil || len(plan.Actions) != 2 || plan.ExecutionStarted || plan.CapacityPolicyChanged {
		t.Fatalf("review: %+v %v", plan, err)
	}
	// Editing project context does not grant execution and does not invalidate
	// an unrelated bookkeeping preference. Dispatch retains its separate source
	// freshness gate; this Apply never calls it or replaces a run's capture.
	p, err := s.resolveProject(run.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	s.mu.Lock()
	state, readErr := readJourney(p.Path, p.ID)
	if readErr == nil {
		state.Brief.Purpose = "A newer draft needing its own review"
		state.Brief.Revision++
		readErr = writeJourney(p.Path, state)
	}
	s.mu.Unlock()
	if readErr != nil {
		t.Fatal(readErr)
	}
	before, _ := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	for _, item := range before.Items {
		if item.Priority != "normal" {
			t.Fatal("review mutated priority")
		}
	}
	req := PortfolioGoalApply{GoalID: goal.ID, ExpectedGoalRevision: goal.Revision, ExpectedPlanDigest: plan.Digest, IdempotencyKey: "explicit-goal-apply"}
	result, err := s.ApplyPortfolioGoal(homeActor("alice"), req, portfolioGoalAllow)
	if err != nil || result.Status != "applied" || *starts != 0 {
		t.Fatalf("apply: %+v %v starts=%d", result, err, *starts)
	}
	after, _ := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	for _, item := range after.Items {
		if item.Priority != "high" || item.State != "queued" || item.Revision != items[0].Revision+1 {
			t.Fatal("changed more than the reviewed priority", item)
		}
	}
	replay, err := s.ApplyPortfolioGoal(homeActor("alice"), req, portfolioGoalAllow)
	if err != nil || replay.ID != result.ID {
		t.Fatal("same request did not return its retained outcome", err)
	}
	unchanged, _ := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if !reflect.DeepEqual(after, unchanged) || *starts != 0 {
		t.Fatal("replay changed queue or started execution")
	}
	req.ExpectedPlanDigest = reviewDigest("other")
	if _, err := s.ApplyPortfolioGoal(homeActor("alice"), req, portfolioGoalAllow); !errors.Is(err, ErrPortfolioGoalConflict) {
		t.Fatal("key reused for another review", err)
	}
}

func TestPortfolioGoalApplyRechecksControlAndRetainsPartialOutcome(t *testing.T) {
	s, mux, run, base, _, starts, _ := schedulerFixture(t)
	enqueueSchedulerFixture(t, mux, run, base, []string{"task-a", "task-b"})
	goal, err := s.SavePortfolioGoal(homeActor("alice"), PortfolioGoalsSave{Goal: portfolioGoalDraft(run.ProjectID)}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.ReviewPortfolioGoal(homeActor("alice"), goal.ID, goal.Revision, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	controls := 0
	revoking := func(_ context.Context, _ string, edit bool) error {
		if edit {
			controls++
			if controls >= 4 {
				return errors.New("control revoked")
			}
		}
		return nil
	}
	result, err := s.ApplyPortfolioGoal(homeActor("alice"), PortfolioGoalApply{GoalID: goal.ID, ExpectedGoalRevision: goal.Revision, ExpectedPlanDigest: plan.Digest, IdempotencyKey: "partial-goal-apply"}, revoking)
	if err != nil || result.Status != "partial" || result.Results[0].Status != "applied" || result.Results[1].Status != "blocked" || *starts != 0 {
		t.Fatalf("partial result: %+v %v", result, err)
	}
	retained, err := s.ReadPortfolioApplication(homeActor("alice"), result.ID, portfolioGoalAllow)
	if err != nil || !reflect.DeepEqual(result, retained) {
		t.Fatal("partial outcome lost", err)
	}
}

func TestPortfolioPriorityCASPreservesStateGuardsAndClearsOldWitness(t *testing.T) {
	item := swarmQueueRecord{SwarmQueueItem: SwarmQueueItem{Revision: 7, State: "queued", Priority: "normal"}}
	marker := reviewDigest("owned-portfolio-intent")
	for _, state := range []string{"running", "unknown", "completed", "cancelled"} {
		item.State = state
		if setSwarmQueuePriority(&item, 7, "high", marker) == nil || item.Priority != "normal" {
			t.Fatal("active/terminal queue mutated", state)
		}
	}
	item.State = "queued"
	if setSwarmQueuePriority(&item, 6, "high", marker) == nil {
		t.Fatal("stale revision accepted")
	}
	if err := setSwarmQueuePriority(&item, 7, "high", marker); err != nil || item.PriorityChangeDigest != marker {
		t.Fatal("owned marker missing", err)
	}
	if err := setSwarmQueuePriority(&item, 7, "high", ""); err != nil || item.PriorityChangeDigest != "" {
		t.Fatal("ordinary control retained stale ownership witness", err)
	}
}

func TestPortfolioGoalChangedQueueRequiresNewReviewBeforeAnyApplication(t *testing.T) {
	s, mux, run, base, _, starts, _ := schedulerFixture(t)
	items := enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	goal, err := s.SavePortfolioGoal(homeActor("alice"), PortfolioGoalsSave{Goal: portfolioGoalDraft(run.ProjectID)}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.ReviewPortfolioGoal(homeActor("alice"), goal.ID, goal.Revision, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+run.ProjectID+"/swarm-queue/"+items[0].ID+"/priority", map[string]any{"expectedRevision": items[0].Revision, "priority": "normal"}, 200)
	_, err = s.ApplyPortfolioGoal(homeActor("alice"), PortfolioGoalApply{GoalID: goal.ID, ExpectedGoalRevision: goal.Revision, ExpectedPlanDigest: plan.Digest, IdempotencyKey: "stale-queue-plan"}, portfolioGoalAllow)
	if !errors.Is(err, ErrPortfolioGoalConflict) || *starts != 0 {
		t.Fatal("stale review applied or started work", err)
	}
	view, err := s.ReadPortfolioGoals(homeActor("alice"), portfolioGoalAllow)
	if err != nil || len(view.Applications) != 0 {
		t.Fatal("unattempted stale plan fabricated an application", err)
	}
}

func TestPortfolioUnknownApplicationReadbackNeverReplaysMatchingPriority(t *testing.T) {
	s, mux, run, base, _, starts, _ := schedulerFixture(t)
	enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	goal, err := s.SavePortfolioGoal(homeActor("alice"), PortfolioGoalsSave{Goal: portfolioGoalDraft(run.ProjectID)}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.ReviewPortfolioGoal(homeActor("alice"), goal.ID, goal.Revision, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	application, err := s.ApplyPortfolioGoal(homeActor("alice"), PortfolioGoalApply{GoalID: goal.ID, ExpectedGoalRevision: goal.Revision, ExpectedPlanDigest: plan.Digest, IdempotencyKey: "recover-owned-priority"}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	// Represent a process that saved its intent and priority, then lost the
	// result acknowledgement. The GET must only observe the existing witness.
	saved, err := s.readPortfolioGoals("alice")
	if err != nil {
		t.Fatal(err)
	}
	saved.Applications[0].Results[0].Status = "unknown"
	saved.Applications[0].Results[0].AppliedRevision = 0
	saved.Applications[0].Status = "unknown"
	if err := s.savePortfolioGoals("alice", &saved); err != nil {
		t.Fatal(err)
	}
	before, _ := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	observed, err := s.ReadPortfolioApplication(homeActor("alice"), application.ID, portfolioGoalAllow)
	if err != nil || observed.Status != "applied" {
		t.Fatal("exact private marker not recognized", err)
	}
	after, _ := s.ProjectSwarmQueueSnapshot(run.ProjectID)
	if !reflect.DeepEqual(before, after) || *starts != 0 {
		t.Fatal("readback replayed a write or start")
	}
	// A separate user control selects the same value. That is not proof that
	// the earlier portfolio write is still the observable owned mutation.
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+run.ProjectID+"/swarm-queue/"+after.Items[0].ID+"/priority", map[string]any{"expectedRevision": after.Items[0].Revision, "priority": "high"}, 200)
	observed, err = s.ReadPortfolioApplication(homeActor("alice"), application.ID, portfolioGoalAllow)
	if err != nil || observed.Status != "unknown" {
		t.Fatal("matching value fabricated priority ownership", err)
	}
}

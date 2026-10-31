package product

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func focusInstallRequest(plan PortfolioFocusPlan, key string) PortfolioGoalApply {
	runs := map[string]string{}
	for _, allocation := range plan.Profile.Allocations {
		for _, dependency := range allocation.WaitUntilAccepted {
			runs[dependency.ProjectID] = dependency.RunID
		}
	}
	selected := []PortfolioFocusRunSelection{}
	for projectID, runID := range runs {
		selected = append(selected, PortfolioFocusRunSelection{ProjectID: projectID, RunID: runID})
	}
	return PortfolioGoalApply{Mode: "focus-install", GoalID: plan.Profile.GoalID, ExpectedGoalRevision: plan.Profile.GoalRevision, ExpectedPlanDigest: plan.Digest, IdempotencyKey: key, Focus: &PortfolioFocusRequest{ExpectedRevision: plan.ExpectedFocusRevision, UpstreamRuns: selected}}
}

func TestPortfolioFocusInstallPersistsPrivateCASAndExplicitRemoval(t *testing.T) {
	options := Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()}
	s, err := New(options)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := homeActor("alice")
	if err := s.CheckPortfolioFocusAdmission(context.Background(), "missing-scheduled-queue"); !errors.Is(err, ErrProjectRunNotFound) {
		t.Fatal("missing scheduled ownership accepted", err)
	}
	goal, err := s.SavePortfolioGoal(ctx, PortfolioGoalsSave{Goal: portfolioGoalDraft("p-one")}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.ReviewPortfolioFocus(ctx, goal.ID, goal.Revision, PortfolioFocusRequest{}, portfolioGoalAllow)
	if err != nil || plan.ExpectedFocusRevision != 0 || plan.ExecutionStarted {
		t.Fatal(plan, err)
	}
	request := focusInstallRequest(plan, "explicit-focus-install")
	installed, err := s.ApplyPortfolioFocus(ctx, request, portfolioGoalAllow)
	if err != nil || installed.Revision != 1 || installed.Profile == nil || installed.Profile.Digest != plan.Profile.Digest {
		t.Fatal(installed, err)
	}
	other, err := s.ReadPortfolioFocus(homeActor("bob"), portfolioGoalAllow)
	if err != nil || other.Revision != 0 || other.Profile != nil {
		t.Fatal("private focus crossed actor scope", other, err)
	}
	teamContext := management.WithTeamAccess(ctx, management.TeamAccess{Team: management.Team{ID: "team-other"}})
	otherTeam, err := s.ReadPortfolioFocus(teamContext, portfolioGoalAllow)
	if err != nil || otherTeam.Revision != 0 || otherTeam.Profile != nil {
		t.Fatal("personal focus crossed team scope", otherTeam, err)
	}
	reopened, err := New(options)
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	loaded, err := reopened.ReadPortfolioFocus(ctx, portfolioGoalAllow)
	if err != nil || loaded.Revision != installed.Revision || loaded.Profile.Digest != installed.Profile.Digest {
		t.Fatal("focus did not survive reopen", loaded, err)
	}
	replay, err := s.ApplyPortfolioFocus(ctx, request, portfolioGoalAllow)
	if err != nil || replay.Revision != 1 {
		t.Fatal("same key repeated installation", replay, err)
	}
	request.IdempotencyKey = "another-focus-install"
	if _, err := s.ApplyPortfolioFocus(ctx, request, portfolioGoalAllow); !errors.Is(err, ErrPortfolioGoalConflict) {
		t.Fatal("stale focus CAS accepted", err)
	}
	remove := PortfolioGoalApply{Mode: "focus-remove", GoalID: goal.ID, ExpectedGoalRevision: goal.Revision, IdempotencyKey: "explicit-focus-remove", Focus: &PortfolioFocusRequest{ExpectedRevision: 1, ExpectedProfileDigest: installed.Profile.Digest}}
	removed, err := s.ApplyPortfolioFocus(ctx, remove, portfolioGoalAllow)
	if err != nil || removed.Revision != 2 || removed.Profile != nil {
		t.Fatal(removed, err)
	}
	if err := s.CheckPortfolioFocusAdmission(context.Background(), "missing-scheduled-queue"); !errors.Is(err, ErrProjectRunNotFound) {
		t.Fatal("removed focus waived scheduled ownership", err)
	}
	if _, err := s.ApplyPortfolioFocus(ctx, focusInstallRequest(plan, "explicit-focus-install"), portfolioGoalAllow); !errors.Is(err, ErrPortfolioGoalConflict) {
		t.Fatal("removed profile resurrected through old request", err)
	}
}

func TestPortfolioFocusRequiresPersistedOwnershipWithoutRestrictingOtherActors(t *testing.T) {
	s, mux, run, base, _, _, _ := schedulerFixture(t)
	items := enqueueSchedulerFixture(t, mux, run, base, []string{"task-a", "task-b"})
	goal, err := s.SavePortfolioGoal(homeActor("alice"), PortfolioGoalsSave{Goal: portfolioGoalDraft(run.ProjectID)}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.ReviewPortfolioFocus(homeActor("alice"), goal.ID, goal.Revision, PortfolioFocusRequest{}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyPortfolioFocus(homeActor("alice"), focusInstallRequest(plan, "unrelated-actor-focus"), portfolioGoalAllow); err != nil {
		t.Fatal(err)
	}
	for i, actor := range []management.ExecutionActorRef{{Local: true, ProjectID: run.ProjectID}, {UserID: "bob", TeamID: "other-team", ProjectID: run.ProjectID}} {
		if err := s.schedulerUpdate(items[i].ID, func(row *swarmQueueRecord) error { row.Actor = actor; return nil }); err != nil {
			t.Fatal(err)
		}
		if err := s.CheckPortfolioFocusAdmission(context.Background(), items[i].ID); err != nil {
			t.Fatal("another actor's focus restricted a real queued owner", actor, err)
		}
	}
	if err := s.CheckPortfolioFocusAdmission(context.Background(), "missing-scheduled-queue"); !errors.Is(err, ErrProjectRunNotFound) {
		t.Fatal("unrelated focus waived missing ownership", err)
	}
}

func TestPortfolioFocusApplyRequiresCurrentControlAndExactGoalRevision(t *testing.T) {
	s, err := New(Options{StateDir: t.TempDir(), ProjectsDir: t.TempDir()})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := homeActor("alice")
	goal, err := s.SavePortfolioGoal(ctx, PortfolioGoalsSave{Goal: portfolioGoalDraft("p-one")}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.ReviewPortfolioFocus(ctx, goal.ID, goal.Revision, PortfolioFocusRequest{}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	denied := func(_ context.Context, _ string, control bool) error {
		if control {
			return management.ErrExecutionAuthority
		}
		return nil
	}
	if _, err := s.ApplyPortfolioFocus(ctx, focusInstallRequest(plan, "no-control-install"), denied); !errors.Is(err, management.ErrExecutionAuthority) {
		t.Fatal("read access became queue control", err)
	}
	goal.Title = "New intent"
	if _, err := s.SavePortfolioGoal(ctx, PortfolioGoalsSave{ExpectedRevision: 1, Goal: goal}, portfolioGoalAllow); err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyPortfolioFocus(ctx, focusInstallRequest(plan, "changed-goal-install"), portfolioGoalAllow); !errors.Is(err, ErrPortfolioGoalConflict) {
		t.Fatal("changed goal installed old focus", err)
	}
	view, err := s.ReadPortfolioFocus(ctx, portfolioGoalAllow)
	if err != nil || view.Revision != 0 {
		t.Fatal("refusal wrote a policy", view, err)
	}
}

func TestPortfolioFocusCountsOriginalAttemptsWithoutControllingOtherActors(t *testing.T) {
	actor := management.ExecutionActorRef{UserID: "alice", TeamID: "team-one", ProjectID: "p-one"}
	item := swarmQueueRecord{SwarmQueueItem: SwarmQueueItem{ID: "candidate", ProjectID: "p-one"}, Actor: actor}
	active := swarmQueueRecord{SwarmQueueItem: SwarmQueueItem{ID: "active", ProjectID: "p-one", RunID: "run-a", OperationID: "op-a", State: "unknown"}, Actor: actor}
	duplicate := active
	duplicate.ID = "same-original"
	other := active
	other.ID = "other-owner"
	other.Actor.UserID = "bob"
	row := swarmSchedulerLedger{Items: []swarmQueueRecord{active, duplicate, other}}
	allocation := PortfolioFocusAllocation{ProjectID: "p-one", MaxActive: 2}
	if !focusCapacityAllows(row, item, allocation) {
		t.Fatal("same original counted twice or other owner charged")
	}
	allocation.MaxActive = 1
	if focusCapacityAllows(row, item, allocation) {
		t.Fatal("unknown original did not hold its slot")
	}
	active.ID = item.ID
	row.Items = []swarmQueueRecord{active}
	if !focusCapacityAllows(row, item, allocation) {
		t.Fatal("current reservation charged itself twice")
	}
	item.RunID, item.OperationID = active.RunID, active.OperationID
	duplicate = active
	duplicate.ID = "old-alias-of-current"
	row.Items = []swarmQueueRecord{active, duplicate}
	if !focusCapacityAllows(row, item, allocation) {
		t.Fatal("same original operation charged itself through another retained queue row")
	}
	allocation.MaxActive = 0
	if !focusCapacityAllows(row, item, allocation) {
		t.Fatal("legacy unspecified became zero capacity")
	}
}

func TestPortfolioFocusBoundedObservationWindowRotatesPastBlockedProjects(t *testing.T) {
	row := swarmSchedulerLedger{}
	for i, actor := range []string{"alice", "bob", "carol", "dave", "erin", "frank", "grace", "helen", "ian", "jane", "kate", "leo", "mary", "nick", "olga", "pete", "quinn"} {
		profile := &PortfolioFocusProfile{Digest: reviewDigest(actor), Allocations: []PortfolioFocusAllocation{{ProjectID: "p-one"}}}
		row.Focus = append(row.Focus, swarmFocusRecord{ActorID: actor, Profile: profile})
		row.Items = append(row.Items, swarmQueueRecord{SwarmQueueItem: SwarmQueueItem{ID: actor, ProjectID: "p-one", State: "queued", Revision: i + 1}, Actor: management.ExecutionActorRef{UserID: actor}})
	}
	first, total := focusObservationWindow(row, 0)
	if total != 17 || len(first) != 16 {
		t.Fatal("observation bounds changed", total, len(first))
	}
	last, total := focusObservationWindow(row, 16)
	if total != 17 || len(last) != 16 {
		t.Fatal("rotated observation bound changed")
	}
	additional := false
	for key := range last {
		if _, found := first[key]; !found {
			additional = true
		}
	}
	if !additional {
		t.Fatal("blocked first projects permanently starved the next project")
	}
}

func TestPortfolioFocusReservationOrderUsesChronologicalNanoseconds(t *testing.T) {
	actor := management.ExecutionActorRef{UserID: "alice", ProjectID: "p-one"}
	early := swarmQueueRecord{SwarmQueueItem: SwarmQueueItem{ID: "early", ProjectID: "p-one", RunID: "run-one", OperationID: "op-early", State: "reserved", QueuedAt: "2026-10-31T00:00:01Z"}, Actor: actor}
	later := early
	later.ID, later.OperationID, later.QueuedAt = "later", "op-later", "2026-10-31T00:00:01.1Z"
	row := swarmSchedulerLedger{Items: []swarmQueueRecord{later, early}}
	unclaimed := map[string]bool{"early": true, "later": true}
	allocation := PortfolioFocusAllocation{ProjectID: "p-one", MaxActive: 1}
	if !focusCapacityAllows(row, early, allocation, unclaimed) || focusCapacityAllows(row, later, allocation, unclaimed) {
		t.Fatal("variable RFC3339 fractions reversed original reservation order")
	}
	early.QueuedAt = "unknown"
	if focusCapacityAllows(row, early, allocation, unclaimed) {
		t.Fatal("invalid timestamp invented start entitlement")
	}
}

func focusAcceptedRunFixture(t *testing.T) ProjectRun {
	t.Helper()
	store, id, path, input := projectRunStoreFixture(t)
	run := createStoredRun(t, store, id, path, input)
	check := ProjectRunCheckEvidence{CheckID: "check-focus", ProjectID: id, Commit: strings.Repeat("2", 40), SourceDigest: strings.Repeat("b", 64), Approvals: input.Approvals, Requirements: input.Requirements, Execution: "docker", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Commands: []ProjectRunCommandEvidence{{CheckCommand: CheckCommand{Label: "Check", Executable: "node"}, Status: "passed", ExitCode: 0}}}
	checked, err := store.JoinCheck(id, path, run.ID, run.Revision, check)
	if err != nil {
		t.Fatal(err)
	}
	joined, err := store.JoinRelease(id, path, run.ID, checked.Revision, ProjectRunReleaseEvidence{ReleaseID: "release-focus", ProjectID: id, Target: "native-docker", Outcome: input.Outcome, Commit: check.Commit, SourceDigest: check.SourceDigest, Approvals: input.Approvals, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano)})
	if err != nil {
		t.Fatal(err)
	}
	accepted, err := store.Accept(id, path, run.ID, joined.Revision)
	if err != nil {
		t.Fatal(err)
	}
	return accepted
}

func TestPortfolioFocusDependenciesRequireExactAcceptedOutcomeSnapshots(t *testing.T) {
	run := focusAcceptedRunFixture(t)
	dependency := PortfolioFocusDependency{ProjectID: run.ProjectID, RunID: run.ID, InputDigest: reviewDigest(run.Input)}
	if !acceptedFocusRun(run, dependency) {
		t.Fatal("canonical accepted run refused")
	}
	for name, change := range map[string]func(*ProjectRun){
		"different_run":   func(r *ProjectRun) { r.ID = "run-" + strings.Repeat("c", 24) },
		"different_input": func(r *ProjectRun) { r.Input.Outcome = "publish" },
		"not_accepted":    func(r *ProjectRun) { r.Acceptance = nil; r.State = "awaiting-release" },
		"unknown_health":  func(r *ProjectRun) { r.Observation = nil },
		"changed_check":   func(r *ProjectRun) { r.Checks = nil },
		"changed_release": func(r *ProjectRun) { r.Releases = nil },
	} {
		t.Run(name, func(t *testing.T) {
			copy := run
			change(&copy)
			if acceptedFocusRun(copy, dependency) {
				t.Fatal("unverified dependency opened admission")
			}
		})
	}
}

func TestPortfolioFocusChangesRealSchedulerAdmissionAndRemoval(t *testing.T) {
	s, mux, run, base, admitted, starts, _ := schedulerFixture(t)
	s.SetPortfolioFocusReadAuthorizer(func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		return management.ExecutionAuthority{}, nil
	})
	_ = enqueueSchedulerFixture(t, mux, run, base, []string{"task-a", "task-b"})
	s.swarmScheduler.mu.Lock()
	ledger, err := s.readSwarmScheduler()
	if err != nil {
		s.swarmScheduler.mu.Unlock()
		t.Fatal(err)
	}
	for i := range ledger.Items {
		ledger.Items[i].Actor = management.ExecutionActorRef{UserID: "alice", ProjectID: run.ProjectID}
	}
	err = s.saveSwarmScheduler(&ledger)
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	goal := portfolioGoalDraft(run.ProjectID)
	goal.Projects[0].RequestedSlots = 1
	saved, err := s.SavePortfolioGoal(homeActor("alice"), PortfolioGoalsSave{Goal: goal}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.ReviewPortfolioFocus(homeActor("alice"), saved.ID, saved.Revision, PortfolioFocusRequest{}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	installed, err := s.ApplyPortfolioFocus(homeActor("alice"), focusInstallRequest(plan, "real-scheduler-focus"), portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	*admitted = true
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *starts != 1 {
		t.Fatal("focus was metadata only", *starts)
	}
	if _, err := s.ApplyPortfolioFocus(homeActor("alice"), PortfolioGoalApply{Mode: "focus-remove", GoalID: saved.ID, ExpectedGoalRevision: saved.Revision, IdempotencyKey: "remove-real-focus", Focus: &PortfolioFocusRequest{ExpectedRevision: installed.Revision, ExpectedProfileDigest: installed.Profile.Digest}}, portfolioGoalAllow); err != nil {
		t.Fatal(err)
	}
	if *starts != 1 {
		t.Fatal("removal itself started work")
	}
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *starts != 2 {
		t.Fatal("ordinary scheduler did not resume eligible work after removal", *starts)
	}
}

func TestPortfolioFocusChecksFreshDependencyReadAuthorityWithoutReplacingRun(t *testing.T) {
	s, mux, run, base, _, _, _ := schedulerFixture(t)
	_ = enqueueSchedulerFixture(t, mux, run, base, []string{"task-a"})
	downstream := createJourneyProject(t, mux, "focus-dependent")
	downstreamInput := run.Input
	downstreamInput.Implementation.ProjectID = downstream.ID
	downstreamInput.Implementation.WorktreeID = project.WorktreeID(downstream.Path)
	downstreamRun := createStoredRun(t, s.projectRuns, downstream.ID, downstream.Path, downstreamInput)
	draft := portfolioGoalDraft(downstream.ID)
	draft.Projects[0].DependsOn = []string{run.ProjectID}
	draft.Projects = append(draft.Projects, PortfolioGoalProject{ProjectID: run.ProjectID, Priority: "normal", DependsOn: []string{}})
	goal, err := s.SavePortfolioGoal(homeActor("alice"), PortfolioGoalsSave{Goal: draft}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.ReviewPortfolioFocus(homeActor("alice"), goal.ID, goal.Revision, PortfolioFocusRequest{UpstreamRuns: []PortfolioFocusRunSelection{{ProjectID: run.ProjectID, RunID: run.ID}}}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyPortfolioFocus(homeActor("alice"), focusInstallRequest(plan, "dependency-focus-install"), portfolioGoalAllow); err != nil {
		t.Fatal(err)
	}
	s.swarmScheduler.mu.Lock()
	ledger, err := s.readSwarmScheduler()
	if err != nil {
		s.swarmScheduler.mu.Unlock()
		t.Fatal(err)
	}
	item := ledger.Items[0]
	item.ID, item.ProjectID = "queue-focus-dependent", downstream.ID
	item.RunID, item.InputDigest = downstreamRun.ID, queueInputDigest(downstreamRun)
	item.Actor = management.ExecutionActorRef{UserID: "alice", ProjectID: downstream.ID}
	ledger.Items = append(ledger.Items, item)
	err = s.saveSwarmScheduler(&ledger)
	s.swarmScheduler.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	reads := 0
	s.SetPortfolioFocusReadAuthorizer(func(_ context.Context, actor management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		reads++
		if actor.UserID != "alice" || actor.ProjectID != run.ProjectID && actor.ProjectID != downstream.ID {
			t.Fatal("dependency read changed owner/project", actor)
		}
		return management.ExecutionAuthority{}, nil
	})
	if err := s.CheckPortfolioFocusAdmission(context.Background(), item.ID); err == nil || reads != 4 {
		t.Fatal("unaccepted run opened admission or access was not checked around read", reads, err)
	}
	s.SetPortfolioFocusReadAuthorizer(func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	})
	if err := s.CheckPortfolioFocusAdmission(context.Background(), item.ID); !errors.Is(err, management.ErrExecutionAuthority) {
		t.Fatal("revoked dependency read was ignored", err)
	}
	loaded, err := s.ReadPortfolioFocus(homeActor("alice"), portfolioGoalAllow)
	if err != nil || loaded.Profile == nil || loaded.Profile.Allocations[0].WaitUntilAccepted == nil {
		t.Fatal("observation rewrote the retained policy", loaded, err)
	}
}

func TestPortfolioFocusLoweredCeilingOrdersExistingReservationsThenContinues(t *testing.T) {
	s, mux, run, base, admitted, starts, children := schedulerFixture(t)
	items := enqueueSchedulerFixture(t, mux, run, base, []string{"task-a", "task-b"})
	p, err := s.resolveProject(run.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	current := run
	operations := map[string]ProjectRunOperation{}
	for _, item := range items {
		var op ProjectRunOperation
		current, op, err = s.reserveSwarmTask(context.Background(), p, run.ID, item.TaskID, "scheduler-"+item.ID, current.Revision)
		if err != nil {
			t.Fatal(err)
		}
		operations[item.ID] = op
		if err := s.schedulerUpdate(item.ID, func(row *swarmQueueRecord) error {
			row.Actor = management.ExecutionActorRef{UserID: "alice", ProjectID: run.ProjectID}
			row.OperationID, row.ChildID = op.ID, op.ChildID
			row.State = "dispatching" // durable intent, canonical operation remains reserved
			return nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	s.SetPortfolioFocusReadAuthorizer(func(context.Context, management.ExecutionActorRef) (management.ExecutionAuthority, error) {
		return management.ExecutionAuthority{}, nil
	})
	draft := portfolioGoalDraft(run.ProjectID)
	draft.Projects[0].RequestedSlots = 1
	goal, err := s.SavePortfolioGoal(homeActor("alice"), PortfolioGoalsSave{Goal: draft}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	plan, err := s.ReviewPortfolioFocus(homeActor("alice"), goal.ID, goal.Revision, PortfolioFocusRequest{}, portfolioGoalAllow)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ApplyPortfolioFocus(homeActor("alice"), focusInstallRequest(plan, "lower-reserved-focus"), portfolioGoalAllow); err != nil {
		t.Fatal(err)
	}
	first, second := items[0], items[1]
	firstAt, firstErr := time.Parse(time.RFC3339Nano, first.QueuedAt)
	secondAt, secondErr := time.Parse(time.RFC3339Nano, second.QueuedAt)
	if firstErr != nil || secondErr != nil {
		t.Fatal("invalid canonical queue timestamp")
	}
	if secondAt.Before(firstAt) || secondAt.Equal(firstAt) && second.ID < first.ID {
		first, second = second, first
	}
	if err := s.CheckPortfolioFocusAdmission(context.Background(), first.ID); err != nil {
		t.Fatal("earliest unclaimed original stranded", err)
	}
	if err := s.CheckPortfolioFocusAdmission(context.Background(), second.ID); err == nil {
		t.Fatal("later reservation exceeded ceiling")
	}
	*admitted = true
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *starts != 1 {
		t.Fatal("lowered ceiling did not permit exactly one original", *starts)
	}
	if err := s.schedulerSetReason(first.ID, "unknown", "dispatch_unknown", "Original outcome unconfirmed"); err != nil {
		t.Fatal(err)
	}
	if err := s.CheckPortfolioFocusAdmission(context.Background(), second.ID); err == nil {
		t.Fatal("claimed/running original stopped holding capacity")
	}
	firstOperation := operations[first.ID]
	child := children[firstOperation.ID]
	child.Status, child.ExitReason = registry.StatusExited, "exited"
	children[firstOperation.ID] = child
	if err := s.RunSwarmSchedulerOnce(context.Background()); err != nil {
		t.Fatal(err)
	}
	if *starts != 2 {
		t.Fatal("next retained original did not progress after settlement", *starts)
	}
	final, err := s.GetProjectSwarmRun(run.ProjectID, run.ID)
	if err != nil || len(final.Operations) != 2 {
		t.Fatal("reservation was replayed/reallocated", final, err)
	}
}

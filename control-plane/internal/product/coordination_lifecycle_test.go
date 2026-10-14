package product

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"sync"
	"sync/atomic"
	"testing"
)

func TestCoordinationOpposingIntentNeedsOwnedResolutionAndCannotBypass(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "opposing-intents")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	a := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "Private notes")
	s.SetCoordinatorAuthorizer(nil)
	b := coordinationProposalTest(t, s, p.ID, "bob", "purpose", "Public notes")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	alice, bob := coordinationHandler(s, "alice"), coordinationHandler(s, "bob")
	ia := coordinationRegisterTest(t, alice, p.ID, a.ID, "alice-conflict", nil)
	// Omitting intent registration is not a route around a durable opposition.
	coordinationApplyTest(t, s, p.ID, "bob", b.ID, 409)
	ib := coordinationRegisterTest(t, bob, p.ID, b.ID, "bob-conflict", nil)
	view := coordinationHTTPView(t, alice, p.ID)
	if len(view["conflicts"].([]any)) == 0 {
		t.Fatal("opposing meanings disappeared")
	}
	snapshot, err := s.OrchestratorSnapshot(orchestratorActor(t, "alice"), p.ID)
	if err != nil || snapshot.TeamIntents == nil || snapshot.TeamIntents.Pending != 2 || len(snapshot.TeamIntents.Conflicts) == 0 {
		t.Fatalf("personal Luci lost unresolved teammate opposition: %+v %v", snapshot.TeamIntents, err)
	}
	coordinationApplyTest(t, s, p.ID, "alice", a.ID, 409)
	coordinationApplyTest(t, s, p.ID, "bob", b.ID, 409)
	resolve := "/v1/product/projects/" + p.ID + "/orchestrator/coordination/intents/" + ia["id"].(string) + "/resolve"
	req := map[string]any{"expectedRevision": view["revision"], "expectedContextDigest": view["contextDigest"], "supersededBy": ib["id"], "reason": "The author explicitly chose the public alternative."}
	journeyRequest(t, bob, "POST", resolve, req, 403)
	journeyRequest(t, alice, "POST", resolve, req, 200)
	journeyRequest(t, alice, "POST", resolve, req, 200)
	coordinationApplyTest(t, s, p.ID, "alice", a.ID, 409)
	coordinationApplyTest(t, s, p.ID, "bob", b.ID, 200)
	final := coordinationHTTPView(t, bob, p.ID)
	if len(final["events"].([]any)) != 4 {
		t.Fatal("resolution replay changed history")
	}
}

func TestCoordinationNewReviewedProposalSupersedesOnlyOwnUnusableOrigin(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "unusable-prior-origin")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	orchard := func(audience, constraints, prefix string) OrchestratorTurn {
		t.Helper()
		turn, snapshot := reserveOrchestratorTest(t, s, p.ID, orchestratorActor(t, "alice"), "Complete the Orchard task list brief", "")
		spec := snapshot.Specification
		spec.Status, spec.Digest = "draft", ""
		spec.Audience, spec.Constraints = audience, constraints
		spec.Requirements = []SpecificationRequirement{
			{ID: "req-001", Title: "Add a task", Priority: "must", Acceptance: prefix + " adding a named task shows it in the list."},
			{ID: "req-002", Title: "Complete a task", Priority: "must", Acceptance: prefix + " toggling a task changes its completion state."},
			{ID: "req-003", Title: "Delete a task", Priority: "must", Acceptance: prefix + " deleting a task removes it from the list."},
			{ID: "req-004", Title: "Persist tasks", Priority: "must", Acceptance: prefix + " reloading restores the remaining saved tasks."},
		}
		answer := OrchestratorAnswer{Summary: "Four reviewable Orchard task behaviors", Options: []string{}, Impact: []string{}, Proposal: &OrchestratorProposal{Target: "specification", Summary: "Complete task list", Specification: &spec}, NextAction: "specification"}
		if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
			t.Fatal(err)
		}
		return turn
	}
	old := orchard("Earlier audience", "Earlier constraints", "Earlier draft:")
	// This answer was completed when routine nonblocking questions were still
	// permitted. Preserve its exact historical bytes and durable origin, then
	// apply today's quality gate to a newer reviewed answer.
	s.mu.Lock()
	stored, err := s.readOrchestrator(p)
	if err == nil {
		prior := coordinationTurn(stored, old.ID)
		if prior == nil {
			err = ErrOrchestratorInvalid
		} else {
			prior.Answer.Question = "Which implementation step should come first?"
			err = s.saveOrchestrator(stored)
		}
	}
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	newer := orchard("People planning their own tasks", "Local storage only", "Reviewed draft:")
	initial, err := s.CoordinationViewForProject(orchestratorActor(t, "alice"), p.ID)
	if err != nil || len(initial.Conflicts) == 0 || len(initial.Intents[0].ChangedPaths) != 6 || len(initial.Intents[1].ChangedPaths) != 6 {
		t.Fatal("fixture did not retain two overlapping six-path Orchard intents", err)
	}
	coordinationApplyTest(t, s, p.ID, "alice", newer.ID, 200)
	view, err := s.CoordinationViewForProject(orchestratorActor(t, "alice"), p.ID)
	if err != nil || len(view.Intents) != 2 || len(view.Events) != 4 {
		t.Fatal("accepted replacement lost exact coordination history", err, view)
	}
	if view.Intents[0].Status != "superseded" || view.Intents[0].SupersededBy != view.Intents[1].ID || view.Intents[1].Status != "applied" || view.Events[2].Kind != "superseded" || view.Events[3].Kind != "applied" {
		t.Fatal("older unusable origin was not durably superseded by the accepted revision", view)
	}
	history, err := s.OrchestratorHistory(p.ID)
	if err != nil || len(history) != 2 || history[0].Answer.Question != "Which implementation step should come first?" {
		t.Fatal("prior raw conversation was rewritten or lost", err)
	}
	current, err := s.OrchestratorView(orchestratorActor(t, "alice"), p.ID, 0)
	if err != nil || current.Context.Specification.Audience != "People planning their own tasks" || len(current.Context.Specification.Requirements) != 4 {
		t.Fatal("accepted reviewed target was not saved", err)
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	restarted.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	coordinationApplyTest(t, restarted, p.ID, "alice", newer.ID, 200)
	replayed, err := restarted.CoordinationViewForProject(orchestratorActor(t, "alice"), p.ID)
	if err != nil || len(replayed.Events) != 4 {
		t.Fatal("restart replay duplicated the canonical apply or supersession", err)
	}

	// A quality-valid competing human proposal retains the existing explicit
	// resolution requirement even when both requests came from one actor.
	s2, mux2 := journeyFixture(t)
	p2 := createJourneyProject(t, mux2, "valid-prior-opposition")
	s2.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	coordinationProposalTest(t, s2, p2.ID, "alice", "purpose", "One valid direction")
	newValid := coordinationProposalTest(t, s2, p2.ID, "alice", "purpose", "A different valid direction")
	current2, err := s2.OrchestratorView(orchestratorActor(t, "alice"), p2.ID, 0)
	if err != nil || !errors.Is(s2.applyOrchestratorProposal(orchestratorActor(t, "alice"), p2.ID, newValid.ID, current2.Revision, current2.Context.Digest), ErrOrchestratorOpposition) {
		t.Fatal("valid opposing request lost its explicit resolution gate", err)
	}

	// An older low-quality model answer from a different person still carries
	// that person's unanswered request; Alice cannot supersede it by applying
	// her own proposal.
	s3, mux3 := journeyFixture(t)
	p3 := createJourneyProject(t, mux3, "teammate-prior-opposition")
	s3.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	teammate := coordinationProposalTest(t, s3, p3.ID, "bob", "purpose", "A different team direction")
	s3.mu.Lock()
	third, readErr := s3.readOrchestrator(p3)
	if readErr == nil {
		coordinationTurn(third, teammate.ID).Answer.Question = "Which implementation step should come first?"
		readErr = s3.saveOrchestrator(third)
	}
	s3.mu.Unlock()
	if readErr != nil {
		t.Fatal(readErr)
	}
	alice := coordinationProposalTest(t, s3, p3.ID, "alice", "purpose", "Alice's new direction")
	current3, err := s3.OrchestratorView(orchestratorActor(t, "alice"), p3.ID, 0)
	if err != nil || !errors.Is(s3.applyOrchestratorProposal(orchestratorActor(t, "alice"), p3.ID, alice.ID, current3.Revision, current3.Context.Digest), ErrOrchestratorOpposition) {
		t.Fatal("another person's intent was silently superseded", err)
	}
}

func TestOrchestratorSnapshotShowsPendingTeamIntentsWithoutAcceptingThem(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "team-intent-snapshot")
	other := createJourneyProject(t, mux, "other-intent-snapshot")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	before, err := s.OrchestratorSnapshot(orchestratorActor(t, "alice"), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	turn := coordinationProposalTest(t, s, p.ID, "bob", "purpose", "A shared notebook")
	intent := coordinationRegisterTest(t, coordinationHandler(s, "bob"), p.ID, turn.ID, "team-visibility", nil)
	current, err := s.OrchestratorSnapshot(orchestratorActor(t, "alice"), p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Digest != before.Digest || current.Specification.Purpose != before.Specification.Purpose {
		t.Fatal("unaccepted teammate request changed canonical context")
	}
	if current.TeamIntents == nil || current.TeamIntents.Pending != 1 || current.TeamIntents.Omitted != 0 || len(current.TeamIntents.Recent) != 1 || current.TeamIntents.Recent[0].ID != intent["id"] || current.TeamIntents.Recent[0].Actor != "bob" || current.TeamIntents.Recent[0].Target != "specification" {
		t.Fatalf("pending teammate request missing or misattributed: %+v", current.TeamIntents)
	}
	foreign, err := s.OrchestratorSnapshot(orchestratorActor(t, "alice"), other.ID)
	if err != nil || foreign.TeamIntents != nil {
		t.Fatalf("peer request leaked into another project: %+v %v", foreign.TeamIntents, err)
	}
	coordinationApplyTest(t, s, p.ID, "bob", turn.ID, 200)
	conversation, err := s.OrchestratorView(orchestratorActor(t, "alice"), p.ID, 0)
	if err != nil || conversation.Activity == nil || len(conversation.Activity.Accepted) != 1 ||
		conversation.Activity.Accepted[0].AppliedTargetDigest == "" || conversation.Activity.Accepted[0].AcceptedContextDigest != "" {
		t.Fatalf("older applied event lost its exact target or invented a whole-context digest: %+v %v", conversation.Activity, err)
	}
	after, err := s.OrchestratorSnapshot(orchestratorActor(t, "alice"), p.ID)
	if err != nil || after.TeamIntents == nil || after.TeamIntents.Pending != 0 || len(after.TeamIntents.Recent) != 0 || after.Digest == before.Digest {
		t.Fatalf("accepted request remained pending or canonical change disappeared: %+v %v", after.TeamIntents, err)
	}
}

func TestCoordinationConcurrentApplyCASAndReadOnlyRestart(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "concurrent-intents")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	a := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "Concurrent notebook")
	b := coordinationProposalTest(t, s, p.ID, "bob", "audience", "Editors")
	coordinationRegisterTest(t, coordinationHandler(s, "alice"), p.ID, a.ID, "one", nil)
	coordinationRegisterTest(t, coordinationHandler(s, "bob"), p.ID, b.ID, "two", nil)
	view, _ := s.OrchestratorView(orchestratorActor(t, "alice"), p.ID, 0)
	contexts := []context.Context{orchestratorActor(t, "alice"), orchestratorActor(t, "bob")}
	turns := []string{a.ID, b.ID}
	start := make(chan struct{})
	results := make(chan error, 2)
	var wg sync.WaitGroup
	for i := range turns {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-start
			results <- s.applyOrchestratorProposal(contexts[i], p.ID, turns[i], view.Revision, view.Context.Digest)
		}(i)
	}
	close(start)
	wg.Wait()
	close(results)
	success := 0
	for err := range results {
		if err == nil {
			success++
		} else if !errors.Is(err, ErrOrchestratorConflict) && !errors.Is(err, ErrOrchestratorBusy) {
			t.Fatal(err)
		}
	}
	if success != 1 {
		t.Fatal("concurrent stale CAS admitted more than one mutation")
	}
	for i := range turns {
		current, _ := s.OrchestratorView(contexts[i], p.ID, 0)
		if err := s.applyOrchestratorProposal(contexts[i], p.ID, turns[i], current.Revision, current.Context.Digest); err != nil {
			t.Fatal(err)
		}
	}
	path, err := s.coordinationPath(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	restarted.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	got, err := restarted.CoordinationViewForProject(contexts[0], p.ID)
	if err != nil || len(got.Events) != 4 {
		t.Fatal("restart lost causal records", err)
	}
	after, _ := os.ReadFile(path)
	if string(before) != string(after) {
		t.Fatal("read repaired or dispatched work")
	}
	other := createJourneyProject(t, mux, "foreign-coordination")
	destination, _ := s.coordinationPath(other.ID, true)
	if err := os.WriteFile(destination, before, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CoordinationViewForProject(contexts[0], other.ID); err == nil {
		t.Fatal("copied project coordination adopted")
	}
	if err := os.WriteFile(path, []byte(`{"version":1,"unexpected":true}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CoordinationViewForProject(contexts[0], p.ID); err == nil {
		t.Fatal("corrupt ledger replaced")
	}
}

func TestCoordinationCurrentAuthorityAndRequestIdentity(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "intent-authority")
	var allowed atomic.Bool
	allowed.Store(true)
	s.SetCoordinatorAuthorizer(func(context.Context, string) error {
		if !allowed.Load() {
			return errors.New("revoked")
		}
		return nil
	})
	s.SetCoordinatorAuthorizer(nil)
	a := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "Notebook")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error {
		if !allowed.Load() {
			return errors.New("revoked")
		}
		return nil
	})
	alice, bob := coordinationHandler(s, "alice"), coordinationHandler(s, "bob")
	view := coordinationHTTPView(t, alice, p.ID)
	url := "/v1/product/projects/" + p.ID + "/orchestrator/coordination/intents"
	req := map[string]any{"expectedRevision": view["revision"], "expectedContextDigest": view["contextDigest"], "turnId": a.ID, "idempotencyKey": "durable-request-one", "dependsOn": []string{}}
	journeyRequest(t, bob, "POST", url, req, 403)
	first := journeyRequest(t, alice, "POST", url, req, 201).Body.Bytes()
	again := journeyRequest(t, alice, "POST", url, req, 200).Body.Bytes()
	if string(first) != string(again) {
		t.Fatal("idempotent admission changed")
	}
	req["dependsOn"] = []string{"intent-other"}
	journeyRequest(t, alice, "POST", url, req, 409)
	allowed.Store(false)
	req["dependsOn"] = []string{}
	journeyRequest(t, alice, "POST", url, req, 403)
	view2, _ := s.OrchestratorView(orchestratorActor(t, "alice"), p.ID, 0)
	if err := s.applyOrchestratorProposal(orchestratorActor(t, "alice"), p.ID, a.ID, view2.Revision, view2.Context.Digest); !errors.Is(err, errCoordinatorAuthority) {
		t.Fatal("revoked coordinator applied", err)
	}
	allowed.Store(true)
	coordinationApplyTest(t, s, p.ID, "alice", a.ID, 200)
}

func TestCoordinationFinishAfterCanonicalSaveReopensWithoutReplay(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "intent-crash")
	s.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	turn := coordinationProposalTest(t, s, p.ID, "alice", "purpose", "Durable notebook")
	ctx := orchestratorActor(t, "alice")
	coordinationRegisterTest(t, coordinationHandler(s, "alice"), p.ID, turn.ID, "crash-once", nil)
	s.mu.Lock()
	saved, _ := s.readOrchestrator(p)
	snapshot, _ := s.orchestratorSnapshotLocked(ctx, p)
	_, err := s.prepareCoordinationApplyLocked(ctx, p, *coordinationTurn(saved, turn.ID), snapshot)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	path, _ := s.coordinationPath(p.ID, false)
	pending, _ := os.ReadFile(path)
	coordinationApplyTest(t, s, p.ID, "alice", turn.ID, 200)
	// Simulate loss of the final coordination write only; canonical/private
	// orchestrator receipts remain durable and are the sole recovery proof.
	if err := os.WriteFile(path, pending, 0600); err != nil {
		t.Fatal(err)
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	restarted.SetCoordinatorAuthorizer(func(context.Context, string) error { return nil })
	view, err := restarted.CoordinationViewForProject(ctx, p.ID)
	if err != nil || view.Intents[0].Status != "pending" {
		t.Fatal("read guessed completion", err)
	}
	coordinationApplyTest(t, restarted, p.ID, "alice", turn.ID, 200)
	coordinationApplyTest(t, restarted, p.ID, "alice", turn.ID, 200)
	view, err = restarted.CoordinationViewForProject(ctx, p.ID)
	if err != nil || len(view.Events) != 2 || view.Intents[0].Status != "applied" {
		t.Fatal("recovery duplicated or lost acceptance", err)
	}
	actual, _ := restarted.OrchestratorView(ctx, p.ID, 0)
	if actual.Context.Specification.Revision != 1 {
		t.Fatal("recovery reapplied canonical mutation")
	}
	var raw map[string]any
	after, _ := os.ReadFile(path)
	if json.Unmarshal(after, &raw) != nil {
		t.Fatal("receipt invalid")
	}
}

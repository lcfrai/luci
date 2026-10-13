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

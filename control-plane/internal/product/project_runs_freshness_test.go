package product

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestProjectRunHTTPFreshnessProjectsContextChangesWithoutObservingOrWriting(t *testing.T) {
	for _, change := range []string{"brief", "design", "plan", "renewed-review", "unreadable"} {
		t.Run(change, func(t *testing.T) {
			s, mux, p, req, _, calls := projectRunHTTPFixture(t)
			base := "/v1/product/projects/" + p.ID
			run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs", req, 201))
			if run.ContextFreshness.Status != "current" || !slices.Contains(run.AllowedActions, "start-coding") {
				t.Fatal("unchanged prepared run is not current", run)
			}
			// Multiple rows use the same current project snapshot, while retaining
			// their own immutable selections and revisions.
			req.IdempotencyKey = "second-http-create-key-123456789"
			second := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs", req, 201))
			reserved := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs/"+second.ID+"/operations", map[string]any{"expectedRevision": second.Revision, "idempotencyKey": "freshness-operation-key-123456", "kind": "session"}, 201))
			op := reserved.Operations[0]
			claimed, _, _, err := s.projectRuns.ClaimOperation(p.ID, p.Path, second.ID, reserved.Revision, op.ID)
			if err != nil {
				t.Fatal(err)
			}
			second.ProjectRun, err = s.projectRuns.ObserveOperation(p.ID, p.Path, second.ID, claimed.Revision, op.ID, op.ChildID, "running", "Coding child observed running")
			if err != nil {
				t.Fatal(err)
			}
			state, err := s.readApprovedJourney(p.Path, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			switch change {
			case "brief":
				brief := state.Brief
				brief.Constraints += " Keep an audit trail."
				journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": brief.Revision, "purpose": brief.Purpose, "audience": brief.Audience, "constraints": brief.Constraints, "criteria": brief.Criteria}, 200)
			case "design":
				plan := *state.DesignPlan
				plan.Screens[0].Purpose += " Show saved changes."
				journeyRequest(t, mux, "POST", base+"/design-plan", map[string]any{"expectedRevision": plan.Revision, "expectedBriefRevision": state.Brief.Revision, "expectedSpecificationRevision": state.Specification.Revision, "plan": plan}, 200)
			case "plan":
				tasks := snapshotProjectTasks(state.Plan.Tasks)
				tasks[0].Status = "doing"
				journeyRequest(t, mux, "POST", base+"/plan", map[string]any{"expectedRevision": state.Plan.Revision, "expectedSpecificationRevision": state.Specification.Revision, "tasks": tasks}, 200)
			case "renewed-review":
				// Identical content with a fresh private approval is a different
				// review selection; public digests alone cannot revive an old run.
				if err := s.recordJourneyApproval(p.Path, p.ID, "design", "reviewer", &state); err != nil {
					t.Fatal(err)
				}
			case "unreadable":
				if err := os.WriteFile(filepath.Join(p.Path, ".luci", "state.json"), []byte("incomplete context file"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			s.SetProjectRunSessionObserver(func(context.Context, string, string, string) ([]registry.Session, error) {
				t.Error("history read observed a coding child")
				return nil, nil
			})
			s.SetProjectRunNativeReleaseObserver(func(context.Context, string, string) (ProjectRunNativeReleaseObservation, error) {
				t.Error("history read observed a release")
				return ProjectRunNativeReleaseObservation{}, nil
			})
			before, projectBefore, beforeCalls := auditTree(t, s.stateDir), auditTree(t, p.Path), calls.Load()
			var list struct {
				Runs []ProjectRunView `json:"runs"`
			}
			decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/runs", nil, 200).Body.Bytes(), &list)
			if len(list.Runs) != 2 {
				t.Fatal(list)
			}
			list.Runs = append(list.Runs, projectRunHTTPResponse(t, journeyRequest(t, mux, "GET", base+"/runs/"+run.ID, nil, 200)))
			for _, view := range list.Runs {
				want := "changed"
				if change == "unreadable" {
					want = "unavailable"
				}
				if view.ContextFreshness.Status != want || view.ContextFreshness.Reason == "" || slices.Contains(view.AllowedActions, "start-coding") || slices.Contains(view.AllowedActions, "continue-coding") {
					t.Fatal("stale context still invited coding", view)
				}
				original := run.ProjectRun
				if view.ID == second.ID {
					original = second.ProjectRun
				}
				if !reflect.DeepEqual(view.ProjectRun, original) || !reflect.DeepEqual(view.AllowedActions, []string{"reconcile", "cancel"}) {
					t.Fatal("freshness replaced history or unrelated actions", view)
				}
			}
			if calls.Load() != beforeCalls || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) || !reflect.DeepEqual(projectBefore, auditTree(t, p.Path)) {
				t.Fatal("history read resolved a provider or wrote private/project state")
			}
		})
	}
}

func TestProjectRunFreshnessIgnoresSourceAndModelAndRetainsLifecycleEvidence(t *testing.T) {
	s, mux, p, req, selected, calls := projectRunHTTPFixture(t)
	base := "/v1/product/projects/" + p.ID + "/runs"
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201))
	if err := os.WriteFile(filepath.Join(p.Path, "agent-changes.txt"), []byte("new working source"), 0600); err != nil {
		t.Fatal(err)
	}
	selected.Connection.Revision++
	beforeCalls := calls.Load()
	view := projectRunHTTPResponse(t, journeyRequest(t, mux, "GET", base+"/"+run.ID, nil, 200))
	if view.ContextFreshness.Status != "current" || !strings.Contains(view.ContextFreshness.Reason, "checked separately") || calls.Load() != beforeCalls || !reflect.DeepEqual(view.Input, run.Input) {
		t.Fatal("context freshness observed or rebound the source/model", view)
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	// A same-revision external edit must be detected by content, not just the
	// public revision counters. It does not invalidate historical acceptance.
	state.Plan.Tasks[0].Title = "Different task content without a revision bump"
	changed := projectRunContextFreshness(run.ProjectRun, state, nil)
	if changed.Status != "changed" {
		t.Fatal(changed)
	}
	for _, lifecycle := range []struct{ state, operation string }{
		{"ready", "reserved"}, {"coding", "running"}, {"awaiting-checks", "cancelled"}, {"blocked", "failed"}, {"cancelled", "running"}, {"complete", "succeeded"},
	} {
		t.Run(lifecycle.state, func(t *testing.T) {
			history := run.ProjectRun
			history.State = lifecycle.state
			history.Operations = []ProjectRunOperation{{ID: "op-historical", ChildID: "s-historical", Status: lifecycle.operation}}
			history.Checks = []ProjectRunCheckEvidence{{CheckID: "check-historical", OriginalAvailable: true}}
			history.Releases = []ProjectRunReleaseEvidence{{ReleaseID: "release-historical"}}
			history.Observation = &ProjectRunObservation{State: "awaiting-release", Valid: true, Reason: "Matching evidence was observed", ObservedAt: "2026-10-06T00:00:00Z"}
			if history.State == "complete" {
				history.Acceptance = &ProjectRunAcceptance{CheckID: "check-historical", ReleaseID: "release-historical", AcceptedAt: "2026-10-06T00:01:00Z"}
			}
			fresh := projectRunViewWithOriginals(history, map[string]bool{"check-historical": true}, ProjectRunContextFreshness{Status: "current"}, nil)
			stale := projectRunViewWithOriginals(history, map[string]bool{"check-historical": true}, changed, nil)
			if !reflect.DeepEqual(stale.ProjectRun, history) {
				t.Fatal("projection changed lifecycle or historical evidence", stale)
			}
			for _, action := range fresh.AllowedActions {
				requiresCurrent := slices.Contains([]string{"start-coding", "continue-coding", "join-check", "join-release", "accept"}, action)
				if slices.Contains(stale.AllowedActions, action) == requiresCurrent {
					t.Fatalf("freshness changed unrelated action or retained coding: %s, %+v", action, stale)
				}
			}
			blocked := projectRunViewWithOriginals(history, map[string]bool{"check-historical": true}, ProjectRunContextFreshness{Status: "current"}, &ProjectRunCodingBlocker{Kind: "question", TurnID: "turn-question", Reason: "Resolve the decision"})
			if !reflect.DeepEqual(blocked.ProjectRun, history) {
				t.Fatal("decision blocker rewrote recorded lifecycle or acceptance", blocked)
			}
			for _, action := range fresh.AllowedActions {
				if slices.Contains(blocked.AllowedActions, action) == slices.Contains([]string{"start-coding", "continue-coding", "join-check", "join-release", "accept"}, action) {
					t.Fatalf("decision blocker changed unrelated action or retained coding: %s", action)
				}
			}
		})
	}
}

func TestProjectRunHTTPProjectsMaterialQuestionWithoutChangingContextOrHistory(t *testing.T) {
	s, mux, p, req, _, calls := projectRunHTTPFixture(t)
	base := "/v1/product/projects/" + p.ID
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base+"/runs", req, 201))
	ctx := orchestratorActor(t, "reviewer")
	turn, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "Clarify access before coding", "")
	answer := OrchestratorAnswer{Summary: "A material decision remains.", Question: "Are saved notes private or shared with the team?", Rationale: "Data access depends on this decision.", Blocking: true, Options: []string{"Private", "Shared"}, Impact: []string{"Data access"}, NextAction: "answer-question"}
	if err := s.CompleteOrchestratorTurn(p.ID, turn.ID, turn.JobID, "succeeded", &answer); err != nil {
		t.Fatal(err)
	}
	assertRead := func(kind, turnID string) ProjectRunView {
		t.Helper()
		before, projectBefore, beforeCalls := auditTree(t, s.stateDir), auditTree(t, p.Path), calls.Load()
		var list struct {
			Runs []ProjectRunView `json:"runs"`
		}
		decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/runs", nil, 200).Body.Bytes(), &list)
		if len(list.Runs) != 1 {
			t.Fatal(list)
		}
		view := projectRunHTTPResponse(t, journeyRequest(t, mux, "GET", base+"/runs/"+run.ID, nil, 200))
		for _, value := range []ProjectRunView{view, list.Runs[0]} {
			if value.ContextFreshness.Status != "current" || !reflect.DeepEqual(value.ProjectRun, run.ProjectRun) {
				t.Fatal("conversation decision changed saved context or immutable history", value)
			}
			if kind == "" {
				if value.CodingBlocker != nil || !slices.Contains(value.AllowedActions, "start-coding") {
					t.Fatal("answered decision kept coding blocked", value)
				}
			} else if value.CodingBlocker == nil || value.CodingBlocker.Kind != kind || value.CodingBlocker.TurnID != turnID || value.CodingBlocker.Reason == "" || slices.Contains(value.AllowedActions, "start-coding") || slices.Contains(value.AllowedActions, "continue-coding") {
				t.Fatal("decision blocker missing or still invited coding", value)
			}
		}
		if calls.Load() != beforeCalls || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) || !reflect.DeepEqual(projectBefore, auditTree(t, p.Path)) {
			t.Fatal("decision projection dispatched, hashed through a provider or wrote history")
		}
		return view
	}
	assertRead("question", turn.ID)
	beforeCalls := calls.Load()
	journeyRequest(t, mux, "POST", base+"/runs/"+run.ID+"/operations", map[string]any{"expectedRevision": run.Revision, "idempotencyKey": "blocked-question-operation-key", "kind": "session"}, 409)
	if calls.Load() != beforeCalls {
		t.Fatal("blocked reservation consulted a provider")
	}
	// Cancellation of a conversation request cannot invent an answer.
	reply, _ := reserveOrchestratorTest(t, s, p.ID, ctx, "A reply that never completed", turn.ID)
	if err := s.CompleteOrchestratorTurn(p.ID, reply.ID, reply.JobID, "cancelled", nil); err != nil {
		t.Fatal(err)
	}
	assertRead("question", turn.ID)
	reply, _ = reserveOrchestratorTest(t, s, p.ID, ctx, "Keep notes private", turn.ID)
	if err := s.CompleteOrchestratorTurn(p.ID, reply.ID, reply.JobID, "succeeded", &OrchestratorAnswer{Summary: "The existing reviewed private access remains correct.", NextAction: "review-context"}); err != nil {
		t.Fatal(err)
	}
	assertRead("", "")
	path, err := s.orchestratorPath(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("unreadable decision ledger"), 0600); err != nil {
		t.Fatal(err)
	}
	assertRead("unavailable", "")
	if err := os.WriteFile(path, saved, 0600); err != nil {
		t.Fatal(err)
	}
	assertRead("", "")
}

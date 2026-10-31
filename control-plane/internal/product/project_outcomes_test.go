package product

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func outcomeAcceptedFixture(t *testing.T) ProjectRun {
	t.Helper()
	_, id, _, input := projectRunStoreFixture(t)
	check := ProjectRunCheckEvidence{CheckID: "check-one", ProjectID: id, Commit: strings.Repeat("1", 40), SourceDigest: strings.Repeat("a", 64), Approvals: input.Approvals, Requirements: input.Requirements, Execution: "host", Commands: []ProjectRunCommandEvidence{{CheckCommand: CheckCommand{Label: "Build", Executable: "go", Args: []string{"build"}}, Status: "passed"}}}
	release := ProjectRunReleaseEvidence{ReleaseID: "release-one", CheckID: check.CheckID, ProjectID: id, Commit: check.Commit, SourceDigest: check.SourceDigest, Approvals: input.Approvals, Outcome: "preview", Target: "local", ObservedAt: "2026-10-31T00:00:00Z"}
	return ProjectRun{ID: "run-one", ProjectID: id, State: "complete", Input: input, Checks: []ProjectRunCheckEvidence{check}, Releases: []ProjectRunReleaseEvidence{release}, Acceptance: &ProjectRunAcceptance{CheckID: check.CheckID, ReleaseID: release.ReleaseID, AcceptedAt: "2026-10-31T01:00:00Z"}}
}

func TestProjectOutcomesRejoinsAcceptanceWithoutInventingCurrentHealth(t *testing.T) {
	run := outcomeAcceptedFixture(t)
	view := summarizeProjectOutcomes(context.Background(), run.ProjectID, []ProjectRun{run})
	if view.Status != "recorded" || *view.RecordedAcceptances != 1 || *view.UnconfirmedAcceptedObservations != 1 || view.LatestAccepted == nil || view.LatestAccepted.CheckID != run.Acceptance.CheckID || view.LatestAccepted.ReleaseID != run.Acceptance.ReleaseID {
		t.Fatalf("lost exact historical join: %+v", view)
	}
	run.Observation = &ProjectRunObservation{Valid: true}
	view = summarizeProjectOutcomes(context.Background(), run.ProjectID, []ProjectRun{run})
	if *view.UnconfirmedAcceptedObservations != 0 || *view.RecordedAcceptances != 1 {
		t.Fatal(view)
	}
	run.Observation = &ProjectRunObservation{Valid: false, Reason: "Current source changed after acceptance"}
	view = summarizeProjectOutcomes(context.Background(), run.ProjectID, []ProjectRun{run})
	if *view.RecordedAcceptances != 1 || *view.UnconfirmedAcceptedObservations != 1 {
		t.Fatal("current invalid observation erased historical acceptance", view)
	}
	run.Acceptance.ReleaseID = "missing-release"
	view = summarizeProjectOutcomes(context.Background(), run.ProjectID, []ProjectRun{run})
	if view.Status != "unavailable" || view.RecordedAcceptances != nil {
		t.Fatal("missing acceptance reference was counted", view)
	}
	run.Acceptance.ReleaseID = run.Releases[0].ReleaseID
	run.Releases[0].SourceDigest = strings.Repeat("b", 64)
	view = summarizeProjectOutcomes(context.Background(), run.ProjectID, []ProjectRun{run})
	if view.Status != "unavailable" || view.RecordedAcceptances != nil || view.LatestAccepted != nil {
		t.Fatalf("mismatched acceptance became success/zero: %+v", view)
	}
}

func TestProjectOutcomesCountsRetainedReviewAndUniqueUnresolvedRepairs(t *testing.T) {
	run := outcomeAcceptedFixture(t)
	run.State, run.Acceptance, run.Checks, run.Releases = "awaiting-checks", nil, nil, nil
	run.Swarm = &ProjectSwarm{CheckRepairs: []ProjectSwarmCheckRepair{{Tasks: []ProjectSwarmCheckRepairTask{{TaskID: "task-one"}, {TaskID: "task-one"}, {TaskID: "task-two"}}}}}
	view := summarizeProjectOutcomes(context.Background(), run.ProjectID, []ProjectRun{run})
	if view.Status != "recorded" || *view.RetainedRuns != 1 || *view.ReviewRequiredRuns != 1 || *view.UnresolvedCheckRepairTasks != 2 || *view.RecordedAcceptances != 0 || *view.OmittedRuns != 0 {
		t.Fatal(view)
	}
	run.State = "cancelled"
	view = summarizeProjectOutcomes(context.Background(), run.ProjectID, []ProjectRun{run})
	if *view.UnresolvedCheckRepairTasks != 0 {
		t.Fatal("cancelled work counted as active repair burden")
	}
}

func TestProjectOutcomesUnknownScopeBoundsAndLegacyOmission(t *testing.T) {
	run := outcomeAcceptedFixture(t)
	for _, rows := range [][]ProjectRun{{run, run}, {run}, make([]ProjectRun, projectRunHistoryLimit+1)} {
		if view := summarizeProjectOutcomes(context.Background(), "foreign", rows); view.Status != "unavailable" || view.RetainedRuns != nil {
			t.Fatal(view)
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if view := summarizeProjectOutcomes(ctx, run.ProjectID, []ProjectRun{run}); view.Status != "unavailable" {
		t.Fatal(view)
	}
	view := summarizeProjectOutcomes(context.Background(), run.ProjectID, nil)
	if view.Status != "recorded" || *view.RecordedAcceptances != 0 {
		t.Fatal(view)
	}
	encoded, err := json.Marshal(ProjectOverview{})
	if err != nil || strings.Contains(string(encoded), `"outcomes"`) {
		t.Fatalf("default overview encoding changed: %s %v", encoded, err)
	}
}

func TestProjectOverviewOutcomesRequiresExplicitReadOptIn(t *testing.T) {
	_, mux := overviewFixture(t)
	p := createJourneyProject(t, mux, "outcomes-opt-in")
	ordinary := journeyRequest(t, mux, "GET", "/v1/product/projects/"+p.ID+"/overview", nil, 200).Body.Bytes()
	if strings.Contains(string(ordinary), `"outcomes"`) {
		t.Fatal("regular polling scans outcome history")
	}
	with := journeyRequest(t, mux, "GET", "/v1/product/projects/"+p.ID+"/overview?includeOutcomes=true", nil, 200).Body.Bytes()
	var view ProjectOverview
	if err := json.Unmarshal(with, &view); err != nil || view.Outcomes == nil {
		t.Fatalf("missing opt-in: %s %v", with, err)
	}
}

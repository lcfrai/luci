package product

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

func TestProjectOverviewLatestRunStagesUseRecordedEvidenceAndCurrentContext(t *testing.T) {
	s, mux, p, req, _, calls := projectRunHTTPFixture(t)
	s.registerProjectOverview(s.guardProjectSourceRoutes(mux))
	base := "/v1/product/projects/" + p.ID + "/runs"
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201)).ProjectRun
	inspect := func(state, phase string) {
		t.Helper()
		before := auditTree(t, s.stateDir)
		beforeCalls := calls.Load()
		view := overviewGet(t, mux, p.ID)
		if view.LatestRun.Status != "recorded" || view.LatestRun.ID != run.ID || view.LatestRun.State != state || view.LatestRun.ContextStatus != "current" || view.Phase.ID != phase || view.NextAction.RunID != run.ID {
			t.Fatalf("wrong run stage: %+v", view)
		}
		if calls.Load() != beforeCalls || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
			t.Fatal("overview resolved a model or wrote state")
		}
	}
	inspect("ready", "develop")
	var err error
	run, err = s.projectRuns.Observe(p.ID, p.Path, run.ID, run.Revision, ProjectRunObservation{State: "awaiting-checks", Valid: false, Reason: "Coding has stopped; check the candidate."})
	if err != nil {
		t.Fatal(err)
	}
	inspect("awaiting-checks", "verify")
	// These are controlled private store receipts for testing projection, not an
	// assertion that this test executed a real application build or deployment.
	check := ProjectRunCheckEvidence{CheckID: "check-overview", ProjectID: p.ID, Commit: strings.Repeat("2", 40), SourceDigest: strings.Repeat("b", 64), Approvals: run.Input.Approvals, Requirements: run.Input.Requirements, Execution: "docker", CreatedAt: time.Now().UTC().Format(time.RFC3339Nano), Commands: []ProjectRunCommandEvidence{{CheckCommand: CheckCommand{Label: "Verify", Executable: "node", Args: []string{"test.js"}}, Status: "passed", ExitCode: 0}}}
	run, err = s.projectRuns.JoinCheck(p.ID, p.Path, run.ID, run.Revision, check)
	if err != nil {
		t.Fatal(err)
	}
	inspect("awaiting-release", "deploy")
	release := ProjectRunReleaseEvidence{ReleaseID: "release-overview", ProjectID: p.ID, Commit: check.Commit, SourceDigest: check.SourceDigest, Target: "native-docker", Outcome: "preview", Approvals: run.Input.Approvals, ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), URL: "http://127.0.0.1:1234"}
	run, err = s.projectRuns.JoinRelease(p.ID, p.Path, run.ID, run.Revision, release)
	if err != nil {
		t.Fatal(err)
	}
	run, err = s.projectRuns.Accept(p.ID, p.Path, run.ID, run.Revision)
	if err != nil {
		t.Fatal(err)
	}
	inspect("complete", "complete")
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.Plan.Tasks[0].Title = "An amended task without a revision bump"
	if err = writeJourney(p.Path, state); err != nil {
		t.Fatal(err)
	}
	changed := overviewGet(t, mux, p.ID)
	if changed.LatestRun.State != "complete" || changed.LatestRun.ContextStatus != "changed" || changed.Phase.ID == "complete" || changed.NextAction.ID != "review-plan" {
		t.Fatalf("historical acceptance falsely certified amended context: %+v", changed)
	}
}
func TestProjectOverviewLatestRunPointerIgnoresOlderActivityAndSurvivesRestart(t *testing.T) {
	s, mux, p, req, _, _ := projectRunHTTPFixture(t)
	s.registerProjectOverview(s.guardProjectSourceRoutes(mux))
	base := "/v1/product/projects/" + p.ID + "/runs"
	older := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201)).ProjectRun
	req.IdempotencyKey = "overview-newer-run-create"
	latest := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201)).ProjectRun
	if _, err := s.projectRuns.Observe(p.ID, p.Path, older.ID, older.Revision, ProjectRunObservation{State: "awaiting-release", Valid: false, Reason: "Older recorded work"}); err != nil {
		t.Fatal(err)
	}
	got := overviewGet(t, mux, p.ID)
	if got.LatestRun.ID != latest.ID || got.LatestRun.State != "ready" || got.Phase.ID != "develop" {
		t.Fatalf("older activity replaced newest scope: %+v", got)
	}
	next, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer next.Close()
	nextMux := http.NewServeMux()
	next.registerProjectOverview(next.guardProjectSourceRoutes(nextMux))
	restored := overviewGet(t, nextMux, p.ID)
	if restored.LatestRun.ID != latest.ID || restored.LatestRun.Status != "recorded" {
		t.Fatalf("restart lost head: %+v", restored.LatestRun)
	}
	// No serialized operation payload, task text, prompt, credential or source
	// snapshot is retained in the latest-run index.
	path, _ := s.projectRuns.overviewRunHeadPath(p.ID, false)
	bytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var indexed map[string]any
	if json.Unmarshal(bytes, &indexed) != nil || len(bytes) > overviewRunHeadLimit || strings.Contains(string(bytes), "implementation") || strings.Contains(string(bytes), "taskTitles") {
		t.Fatal("latest-run pointer is not compact")
	}
}
func TestProjectOverviewRunIndexUnavailableNeverRepairsOrScansLegacyHistory(t *testing.T) {
	s, mux, p, req, _, _ := projectRunHTTPFixture(t)
	s.registerProjectOverview(s.guardProjectSourceRoutes(mux))
	base := "/v1/product/projects/" + p.ID + "/runs"
	run := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201)).ProjectRun
	path, _ := s.projectRuns.overviewRunHeadPath(p.ID, false)
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	// An invalid unrelated historical file would make the old full List fail.
	// This endpoint neither reads that history nor rewrites it on observation.
	if err := os.WriteFile(filepath.Join(s.stateDir, "project-runs", p.ID, "run-old-invalid.json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	before := auditTree(t, s.stateDir)
	got := overviewGet(t, mux, p.ID)
	if got.Status != "partial" || got.LatestRun.Status != "unavailable" || got.LatestRun.ID != "" || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatalf("legacy progress was guessed or repaired: %+v", got.LatestRun)
	}
	// Updating an old run without a trustworthy head cannot claim it is newest.
	if _, err := s.projectRuns.Observe(p.ID, p.Path, run.ID, run.Revision, ProjectRunObservation{State: "awaiting-checks", Reason: "Explicit observation"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("unknown legacy index silently reconstructed from one historical row")
	}
}
func TestProjectOverviewRunReadDoesNotInspectHistoricalPayloads(t *testing.T) {
	s, mux, p, req, _, _ := projectRunHTTPFixture(t)
	s.registerProjectOverview(s.guardProjectSourceRoutes(mux))
	base := "/v1/product/projects/" + p.ID + "/runs"
	old := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201)).ProjectRun
	req.IdempotencyKey = "overview-other-latest-create"
	latest := projectRunHTTPResponse(t, journeyRequest(t, mux, "POST", base, req, 201)).ProjectRun
	// In-place corruption of an older run keeps the directory generation. A
	// complete history read would fail; the exact latest pointer remains valid.
	if err := os.WriteFile(filepath.Join(s.stateDir, "project-runs", p.ID, old.ID+".json"), []byte("{"), 0600); err != nil {
		t.Fatal(err)
	}
	s.runner = func(context.Context, string, []string, string) (string, error) {
		t.Fatal("overview ran a source/provider command")
		return "", nil
	}
	got := overviewGet(t, mux, p.ID)
	if got.LatestRun.Status != "recorded" || got.LatestRun.ID != latest.ID {
		t.Fatalf("overview read unrelated run payloads: %+v", got.LatestRun)
	}
}

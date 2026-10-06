package product

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func projectRunEvidenceFixture(t *testing.T) (*Server, project.Project, ProjectRunApprovalSelection, []RequirementSnapshot, ProjectCheckRun) {
	t.Helper()
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	p := createJourneyProject(t, mux, "run-evidence")
	spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-save")
	if err := os.WriteFile(filepath.Join(p.Path, "check.js"), []byte(`console.log('private-output-must-not-be-copied');process.exit(0)`), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/git/commit", reviewedCommitRequest(t, mux, "/v1/product/projects/"+p.ID+"/git/commit", "Save reviewed check candidate"), 200)
	useActualEvidenceProcesses(t, s)
	var row ProjectCheckRun
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/checks", evidenceRequest(spec, design, "check.js"), 202).Body.Bytes(), &row)
	row = finishEvidenceRun(t, s, p.ID, row.ID)
	selected, err := s.ApprovalSelection(p.ID)
	if err != nil {
		t.Fatal(err)
	}
	return s, p, selected, append([]RequirementSnapshot(nil), row.RequirementSnapshots...), row
}

func TestProjectRunCheckSnapshotRetainsActualOutcomesAfterHistoryPruning(t *testing.T) {
	s, p, selected, requirements, row := projectRunEvidenceFixture(t)
	snapshot, err := s.SnapshotProjectRunCheck(context.Background(), p.ID, row.ID, selected, requirements)
	if err != nil || snapshot.Commit != row.Commit || snapshot.SourceDigest != row.SourceDigest || snapshot.Commands[0].ExitCode != 0 || !snapshot.OriginalAvailable {
		t.Fatal("passing native command was not joined", snapshot, err)
	}
	saved, _ := json.Marshal(snapshot)
	if strings.Contains(string(saved), "private-output-must-not-be-copied") || strings.Contains(string(saved), `"output"`) {
		t.Fatal("run snapshot retained raw command output")
	}
	s.mu.Lock()
	for i := 0; i < 21; i++ {
		copy := snapshotProjectCheck(row)
		copy.ID = fmt.Sprintf("check-new-%d", i)
		if err := s.storeCheckRun(copy); err != nil {
			s.mu.Unlock()
			t.Fatal(err)
		}
	}
	s.mu.Unlock()
	available, err := s.ProjectRunCheckOriginalAvailable(p.ID, row.ID)
	if err != nil || available {
		t.Fatal("pruned original was advertised as available", err)
	}
	var reopened ProjectRunCheckEvidence
	if err := json.Unmarshal(saved, &reopened); err != nil || !reflect.DeepEqual(snapshot, reopened) || reopened.Commands[0].Status != "passed" {
		t.Fatal("bounded persisted snapshot lost provenance", err)
	}
	if _, err := s.SnapshotProjectRunCheck(context.Background(), p.ID, row.ID, selected, requirements); err == nil {
		t.Fatal("missing original was joined as fresh success")
	}
}

func TestProjectRunCheckJoinRejectsIncompleteFailedDirtyAndStaleEvidence(t *testing.T) {
	s, p, selected, requirements, row := projectRunEvidenceFixture(t)
	_, rows, err := s.readProjectEvidence(context.Background(), p.ID, p.Path)
	if err != nil || len(rows) != 1 {
		t.Fatal(err)
	}
	row = rows[0]
	source := EvidenceSource{Commit: row.Commit, SourceDigest: row.SourceDigest, Available: true}
	for _, tc := range []struct {
		name   string
		mutate func(*ProjectCheckRun, *[]RequirementSnapshot, *EvidenceSource)
	}{
		{"foreign-project", func(r *ProjectCheckRun, _ *[]RequirementSnapshot, _ *EvidenceSource) { r.ProjectID = "foreign" }},
		{"no-private-selection", func(r *ProjectCheckRun, _ *[]RequirementSnapshot, _ *EvidenceSource) { r.ApprovalSelection = nil }},
		{"source-drift", func(r *ProjectCheckRun, _ *[]RequirementSnapshot, _ *EvidenceSource) { r.SourceChanged = true }},
		{"dirty-check", func(r *ProjectCheckRun, _ *[]RequirementSnapshot, _ *EvidenceSource) { r.Dirty = true }},
		{"dirty-current", func(_ *ProjectCheckRun, _ *[]RequirementSnapshot, s *EvidenceSource) { s.Dirty = true }},
		{"wrong-commit", func(_ *ProjectCheckRun, _ *[]RequirementSnapshot, s *EvidenceSource) {
			s.Commit = strings.Repeat("a", 40)
		}},
		{"failed-command", func(r *ProjectCheckRun, _ *[]RequirementSnapshot, _ *EvidenceSource) {
			n := 1
			r.Commands[0].ExitCode = &n
		}},
		{"no-command", func(r *ProjectCheckRun, _ *[]RequirementSnapshot, _ *EvidenceSource) { r.Commands = nil }},
		{"incomplete-coverage", func(_ *ProjectCheckRun, req *[]RequirementSnapshot, _ *EvidenceSource) {
			*req = append(*req, RequirementSnapshot{ID: "AC-missing", Title: "Missing", Acceptance: "Actual behavior"})
		}},
		{"acceptance-drift", func(_ *ProjectCheckRun, req *[]RequirementSnapshot, _ *EvidenceSource) {
			(*req)[0].Acceptance = "Different behavior"
		}},
		{"no-requirements", func(_ *ProjectCheckRun, req *[]RequirementSnapshot, _ *EvidenceSource) { *req = nil }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			copy, reqs, current := snapshotProjectCheck(row), append([]RequirementSnapshot(nil), requirements...), source
			tc.mutate(&copy, &reqs, &current)
			if _, err := projectRunCheckSnapshot(copy, p.ID, selected, reqs, current); err == nil {
				t.Fatal("invalid check admitted")
			}
		})
	}
	state, _ := s.readApprovedJourney(p.Path, p.ID)
	if err := s.recordJourneyApproval(p.Path, p.ID, "design", "fixture-reviewer", &state); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SnapshotProjectRunCheck(context.Background(), p.ID, row.ID, selected, requirements); err == nil {
		t.Fatal("old private selection admitted after identical-content reapproval")
	}
	current, _ := s.ApprovalSelection(p.ID)
	if _, err := s.SnapshotProjectRunCheck(context.Background(), p.ID, row.ID, current, requirements); err == nil {
		t.Fatal("new private selection recycled an old check")
	}
}

func TestProjectRunLocalReleaseJoinNeedsPrivateExactCandidateAndFreshOwnedHealth(t *testing.T) {
	s, p, selected, requirements, check := projectRunEvidenceFixture(t)
	candidate, err := s.SnapshotProjectRunCheck(context.Background(), p.ID, check.ID, selected, requirements)
	if err != nil {
		t.Fatal(err)
	}
	var health atomic.Int32
	health.Store(200)
	var hits atomic.Int32
	app := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { hits.Add(1); w.WriteHeader(int(health.Load())) }))
	defer app.Close()
	row := storedRelease{Release: Release{ID: "rel-run-preview", ProjectID: p.ID, Target: "local", Commit: check.Commit, Status: "ready", URL: app.URL}, ContainerID: strings.Repeat("b", 64)}
	if err := s.storeRelease(row); err != nil {
		t.Fatal(err)
	}
	original := s.runner
	var inspection atomic.Value
	inspection.Store(previewInspection(row))
	s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
		if exe != "docker" {
			return original(ctx, exe, args, dir)
		}
		if len(args) != 4 || args[0] != "inspect" {
			return "", errors.New("join attempted a resource mutation")
		}
		return inspection.Load().(string), nil
	}
	join := func(outcome string, c ProjectRunCheckEvidence) (ProjectRunReleaseEvidence, error) {
		return s.SnapshotProjectRunRelease(context.Background(), p.ID, row.ID, "local", outcome, selected, c)
	}
	if _, err := join("preview", candidate); err == nil {
		t.Fatal("historical release without provenance was backfilled")
	}
	if err := s.RecordProjectRunReleaseProvenance(context.Background(), p.ID, row.ID, row.Target, row.Commit); err != nil {
		t.Fatal(err)
	}
	if err := s.RecordProjectRunReleaseProvenance(context.Background(), p.ID, row.ID, row.Target, row.Commit); err == nil {
		t.Fatal("immutable admission provenance was overwritten")
	}
	path, _ := s.releaseRunProvenancePath(row.ID, false)
	assertPrivateFile(t, path)
	before := auditTree(t, s.stateDir)
	joined, err := join("preview", candidate)
	if err != nil || joined.Commit != candidate.Commit || joined.SourceDigest != candidate.SourceDigest || joined.ObservedAt == "" || joined.CheckID != check.ID || joined.Outcome != "preview" {
		t.Fatal("exact observed release was not joined", joined, err)
	}
	if !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("observation rewrote private histories")
	}
	if _, err := join("publish", candidate); err == nil {
		t.Fatal("local preview advertised as public deployment")
	}
	wrong := candidate
	wrong.SourceDigest = strings.Repeat("f", 64)
	if _, err := join("preview", wrong); err == nil {
		t.Fatal("wrong checked digest admitted")
	}
	inspection.Store(strings.Replace(previewInspection(row), p.ID, "foreign-project", 1))
	count := hits.Load()
	if _, err := join("preview", candidate); err == nil || hits.Load() != count {
		t.Fatal("foreign container reached application or joined")
	}
	inspection.Store(previewInspection(row))
	health.Store(503)
	if _, err := join("preview", candidate); err == nil {
		t.Fatal("saved ready status manufactured current health")
	}
	health.Store(200)
	if err := os.WriteFile(filepath.Join(p.Path, "check.js"), []byte("changed"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := join("preview", candidate); err == nil {
		t.Fatal("source changed after checks but release joined")
	}
}

func TestProjectRunReleaseProvenanceRejectsCopiedProjectAndAccount(t *testing.T) {
	s, p, _, _, check := projectRunEvidenceFixture(t)
	if err := s.RecordProjectRunReleaseProvenance(context.Background(), p.ID, "rel-private", "local", check.Commit); err != nil {
		t.Fatal(err)
	}
	receipt, err := s.readReleaseRunProvenance(p.ID, "rel-private")
	if err != nil {
		t.Fatal(err)
	}
	other, err := New(Options{StateDir: t.TempDir(), ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	path, err := other.releaseRunProvenancePath("rel-private", true)
	if err != nil {
		t.Fatal(err)
	}
	if err := atomicJSON(path, receipt); err != nil {
		t.Fatal(err)
	}
	if _, err := other.readReleaseRunProvenance(p.ID, "rel-private"); err == nil {
		t.Fatal("copied account receipt conferred source provenance")
	}
	path, _ = s.releaseRunProvenancePath("rel-private", false)
	receipt.ProjectScope = strings.Repeat("0", 64)
	if err := atomicJSON(path, receipt); err != nil {
		t.Fatal(err)
	}
	if _, err := s.readReleaseRunProvenance(p.ID, "rel-private"); err == nil {
		t.Fatal("foreign project scope conferred provenance")
	}
}

func TestProjectRunCloudOwnershipRequiresCurrentApplicationSourceIdentity(t *testing.T) {
	row := CloudRelease{Release: Release{ID: "cloud-rel-check", ProjectID: "owned", Target: "target-test", Commit: strings.Repeat("a", 40)}, CloudReleaseRequest: CloudReleaseRequest{TargetID: "target-test", Repository: "https://github.com/example/app.git", Branch: "main", Commit: strings.Repeat("a", 40), Domain: "https://app.example.com"}, ApplicationID: "app-owned", DeploymentID: "deployment-owned"}
	current := map[string]string{"uuid": row.ApplicationID, "status": "running:healthy", "git_repository": row.Repository, "git_branch": row.Branch, "git_commit_sha": row.Release.Commit, "fqdn": row.Domain}
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "GET" || r.URL.Path != "/api/v1/applications/"+row.ApplicationID {
			t.Error("observation mutated provider or changed application")
		}
		_ = json.NewEncoder(w).Encode(current)
	}))
	defer provider.Close()
	client, err := NewCoolifyClient(provider.URL, testCoolifyToken)
	if err != nil {
		t.Fatal(err)
	}
	if err := verifyProjectRunCloudApplication(context.Background(), client, row); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"uuid", "status", "git_repository", "git_branch", "git_commit_sha", "fqdn"} {
		saved := current[key]
		current[key] = "changed"
		if err := verifyProjectRunCloudApplication(context.Background(), client, row); err == nil {
			t.Fatal("changed provider identity admitted", key)
		}
		current[key] = saved
	}
	delete(current, "git_commit_sha")
	if err := verifyProjectRunCloudApplication(context.Background(), client, row); err == nil {
		t.Fatal("provider without current commit manufactured source evidence")
	}
}

func TestProjectRunNativeReleaseJoinRejectsForeignAndStaleObservation(t *testing.T) {
	s, p, selected, requirements, row := projectRunEvidenceFixture(t)
	check, err := s.SnapshotProjectRunCheck(context.Background(), p.ID, row.ID, selected, requirements)
	if err != nil {
		t.Fatal(err)
	}
	const releaseID = "deploy-run"
	if err := s.RecordProjectRunReleaseProvenance(context.Background(), p.ID, releaseID, "native-docker", row.Commit); err != nil {
		t.Fatal(err)
	}
	join := func() (ProjectRunReleaseEvidence, error) {
		return s.SnapshotProjectRunRelease(context.Background(), p.ID, releaseID, "native-docker", "preview", selected, check)
	}
	if _, err := join(); err == nil {
		t.Fatal("missing native observer manufactured ready evidence")
	}
	observation := ProjectRunNativeReleaseObservation{ReleaseID: releaseID, ProjectID: p.ID, Target: "native-docker", Commit: row.Commit, URL: "http://127.0.0.1:3456", ObservedAt: time.Now().UTC().Format(time.RFC3339Nano), Ready: true, Limitations: "Local health fixture"}
	s.SetProjectRunNativeReleaseObserver(func(_ context.Context, projectID, id string) (ProjectRunNativeReleaseObservation, error) {
		if projectID != p.ID || id != releaseID {
			t.Error("observer selected foreign child")
		}
		return observation, nil
	})
	if _, err := join(); err != nil {
		t.Fatal(err)
	}
	correct := observation
	for _, test := range []string{"project", "commit", "release", "target", "time", "ready"} {
		observation = correct
		switch test {
		case "project":
			observation.ProjectID = "foreign"
		case "commit":
			observation.Commit = strings.Repeat("f", 40)
		case "release":
			observation.ReleaseID = "deploy-foreign"
		case "target":
			observation.Target = "native-panacea"
		case "time":
			observation.ObservedAt = time.Now().Add(-2 * time.Minute).UTC().Format(time.RFC3339Nano)
		case "ready":
			observation.Ready = false
		}
		if _, err := join(); err == nil {
			t.Fatal("invalid native observation admitted", test)
		}
	}
	observation = correct
	s.SetProjectRunNativeReleaseObserver(func(_ context.Context, _, _ string) (ProjectRunNativeReleaseObservation, error) {
		if err := os.WriteFile(filepath.Join(p.Path, "check.js"), []byte("source changed during release observation"), 0600); err != nil {
			t.Fatal(err)
		}
		return observation, nil
	})
	if _, err := join(); err == nil {
		t.Fatal("source changed during provider observation but old candidate was joined")
	}
}

package product

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func externalFixture(t *testing.T) (*Server, *http.ServeMux, project.Project, ImplementationContextRequest) {
	t.Helper()
	s, mux := journeyFixture(t)
	s.registerProjectWork(mux)
	s.SetExternalHandoffAuthorizer(func(ctx context.Context, id string) error { return ctx.Err() })
	p := createJourneyProject(t, mux, "external-context")
	spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-external")
	var plan ProjectPlan
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/plan", map[string]any{"expectedRevision": 0, "expectedSpecificationRevision": spec.Revision, "tasks": []ProjectTask{{ID: "build", Title: "Implement saved work", Status: "todo", RequirementIDs: []string{"AC-external"}, Acceptance: "Saved work returns after reopening"}, {ID: "later", Title: "Unselected independent task", Status: "todo"}}}, 200).Body.Bytes(), &plan)
	return s, mux, p, ImplementationContextRequest{ProjectID: p.ID, BriefRevision: spec.BriefRevision, PlanRevision: plan.Revision, SpecificationRevision: spec.Revision, DesignPlanRevision: design.Revision, TaskIDs: []string{"build"}}
}
func createExternalFixture(t *testing.T, mux http.Handler, p project.Project, selection ImplementationContextRequest) ExternalHandoff {
	t.Helper()
	var h ExternalHandoff
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/external-handoffs", map[string]any{"selection": selection, "idempotencyKey": "external-fixture-key-0001"}, 201).Body.Bytes(), &h)
	return h
}
func externalReviewFixture(t *testing.T, mux http.Handler, p project.Project, h ExternalHandoff) ExternalReturnReview {
	t.Helper()
	var r ExternalReturnReview
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", "/v1/product/projects/"+p.ID+"/external-handoffs/"+h.ID+"/review", nil, 200).Body.Bytes(), &r)
	return r
}
func TestExternalHandoffStandaloneBundleRoundTripAndRestart(t *testing.T) {
	s, mux, p, selection := externalFixture(t)
	// No agent, provider, executor, release or container is needed by this path.
	s.projectCheckRuntime = ProjectCheckRuntime{}
	if err := os.WriteFile(filepath.Join(p.Path, "app.txt"), []byte("before\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, ".env"), []byte("PRIVATE_SHOULD_NOT_EXPORT=secret\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before := auditTree(t, p.Path)
	h := createExternalFixture(t, mux, p, selection)
	if !reflect.DeepEqual(before, auditTree(t, p.Path)) {
		t.Fatal("handoff changed project files")
	}
	if h.ProjectID != p.ID || len(h.Implementation.TaskIDs) != 1 || h.Implementation.TaskIDs[0] != "build" || h.Source.SourceDigest == "" || h.Source.GitReviewDigest == "" || !h.Source.Dirty {
		t.Fatal("missing reviewed assignment/source", h)
	}
	base := "/v1/product/projects/" + p.ID + "/external-handoffs/" + h.ID
	bundle := journeyRequest(t, mux, "GET", base+"/bundle", nil, 200).Body.Bytes()
	zr, err := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string][]byte{}
	for _, f := range zr.File {
		rd, err := f.Open()
		if err != nil {
			t.Fatal(err)
		}
		b, err := io.ReadAll(rd)
		rd.Close()
		if err != nil {
			t.Fatal(err)
		}
		seen[f.Name] = b
		if strings.Contains(string(b), "PRIVATE_SHOULD_NOT_EXPORT") {
			t.Fatal("secret exported")
		}
	}
	for _, f := range h.Files {
		data, ok := seen[f.Path]
		hash := sha256.Sum256(data)
		if !ok || len(data) != f.Bytes || hex.EncodeToString(hash[:]) != f.SHA256 {
			t.Fatal("bundle manifest mismatch", f.Path)
		}
	}
	if len(seen) != len(h.Files)+1 || seen["manifest.json"] == nil || seen["report-template.json"] == nil || !strings.Contains(string(seen["LUCI.md"]), "build") || seen["AGENTS.md"] != nil || seen["CLAUDE.md"] != nil {
		t.Fatal("missing portable bundle or overwriting tool instructions")
	}
	sourceFiles := map[string]ExternalSourceFile{}
	if err = json.Unmarshal(seen["source-files.json"], &sourceFiles); err != nil {
		t.Fatal(err)
	}
	if sourceFiles["app.txt"].SHA256 == "" || sourceFiles[".env"].SHA256 != "" {
		t.Fatal("wrong source inventory", sourceFiles)
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/external-handoffs", map[string]any{"selection": selection, "idempotencyKey": "external-fixture-key-0001"}, 200)
	if err = os.WriteFile(filepath.Join(p.Path, "app.txt"), []byte("after\n"), 0600); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(filepath.Join(p.Path, "tests.txt"), []byte("actual source changes, no automated evidence\n"), 0600); err != nil {
		t.Fatal(err)
	}
	review := externalReviewFixture(t, mux, p, h)
	if !review.ContextCurrent || len(review.ChangedFiles) != 2 || review.ChangedFiles[0].Path != "app.txt" || review.ChangedFiles[0].Status != "modified" || review.ChangedFiles[1].Status != "added" {
		t.Fatal("ordinary code edit not reviewable", review)
	}
	req := map[string]any{"expectedReviewDigest": review.ReviewDigest, "idempotencyKey": "external-return-key-0001", "report": ExternalReport{ProjectID: p.ID, HandoffID: h.ID, HandoffDigest: h.Digest, Summary: "External tool claims it implemented the task.", Checks: []ExternalReportCheck{{Command: "printf do-not-execute", Outcome: "claimed passed", Output: "untrusted tool output"}}, Notes: "Run independent verification."}}
	var returned ExternalReturn
	decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/returns", req, 201).Body.Bytes(), &returned)
	if returned.EvidenceStatus != "unverified" || returned.Source != review.Source || returned.HandoffDigest != h.Digest || !reflect.DeepEqual(returned.ChangedFiles, review.ChangedFiles) {
		t.Fatal("return invented verification or lost source", returned)
	}
	journeyRequest(t, mux, "POST", base+"/returns", req, 200)
	s.mu.RLock()
	checks, err := s.readCheckHistory(p.ID)
	s.mu.RUnlock()
	if err != nil || len(checks) != 0 {
		t.Fatal("external report manufactured checks", checks, err)
	}
	again, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	next := http.NewServeMux()
	again.registerProjects(next)
	var restored ExternalHandoff
	decodeEvidenceResponse(t, journeyRequest(t, next, "GET", base, nil, 200).Body.Bytes(), &restored)
	if restored.Digest != h.Digest || len(restored.Returns) != 1 || restored.Returns[0].ID != returned.ID || !bytes.Equal(bundle, journeyRequest(t, next, "GET", base+"/bundle", nil, 200).Body.Bytes()) {
		t.Fatal("immutable bundle/return lost after restart")
	}
}
func TestExternalReturnRefusesSourceAndApprovedContextDrift(t *testing.T) {
	s, mux, p, selection := externalFixture(t)
	h := createExternalFixture(t, mux, p, selection)
	base := "/v1/product/projects/" + p.ID + "/external-handoffs/" + h.ID
	original := journeyRequest(t, mux, "GET", base+"/bundle", nil, 200).Body.Bytes()
	review := externalReviewFixture(t, mux, p, h)
	req := map[string]any{"expectedReviewDigest": review.ReviewDigest, "idempotencyKey": "external-return-key-0002", "report": ExternalReport{ProjectID: p.ID, HandoffID: h.ID, HandoffDigest: h.Digest, Summary: "Reported outcome", Checks: []ExternalReportCheck{}}}
	if err := os.WriteFile(filepath.Join(p.Path, "new.txt"), []byte("late source change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", base+"/returns", req, 409)
	fresh := externalReviewFixture(t, mux, p, h)
	if fresh.ReviewDigest == review.ReviewDigest || !fresh.ContextCurrent {
		t.Fatal(fresh)
	}
	// Changing canonical context invalidates the old handoff, even if a caller
	// changes the exported approval status back to approved.
	s.mu.Lock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.Specification.Purpose = "Amended requirement scope"
	state.Specification.Status = "approved"
	err = writeJourney(p.Path, state)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	drift := externalReviewFixture(t, mux, p, h)
	if drift.ContextCurrent {
		t.Fatal("public approved bytes revived private authority")
	}
	req["expectedReviewDigest"] = drift.ReviewDigest
	journeyRequest(t, mux, "POST", base+"/returns", req, 409)
	if !bytes.Equal(original, journeyRequest(t, mux, "GET", base+"/bundle", nil, 200).Body.Bytes()) {
		t.Fatal("context amendment rewrote historical capture")
	}
	other := createJourneyProject(t, mux, "other-project")
	journeyRequest(t, mux, "GET", "/v1/product/projects/"+other.ID+"/external-handoffs/"+h.ID, nil, 404)
}
func TestExternalHandoffAuthorityAndUnsafeStorageRefusals(t *testing.T) {
	s, mux, p, selection := externalFixture(t)
	h := createExternalFixture(t, mux, p, selection)
	base := "/v1/product/projects/" + p.ID + "/external-handoffs/" + h.ID
	var calls atomic.Int32
	s.SetExternalHandoffAuthorizer(func(context.Context, string) error {
		if calls.Add(1) >= 3 {
			return errors.New("editor grant revoked")
		}
		return nil
	})
	review := externalReviewFixture(t, mux, p, h)
	before := auditTree(t, s.stateDir)
	journeyRequest(t, mux, "POST", base+"/returns", map[string]any{"expectedReviewDigest": review.ReviewDigest, "idempotencyKey": "external-return-key-0003", "report": ExternalReport{ProjectID: p.ID, HandoffID: h.ID, HandoffDigest: h.Digest, Summary: "No grant", Checks: []ExternalReportCheck{}}}, 403)
	if calls.Load() != 3 || !reflect.DeepEqual(before, auditTree(t, s.stateDir)) {
		t.Fatal("late authority loss persisted report")
	}
	s.SetExternalHandoffAuthorizer(func(context.Context, string) error { return errors.New("editor grant revoked") })
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/external-handoffs", map[string]any{"selection": selection, "idempotencyKey": "external-fixture-key-0001"}, 403)
	journeyRequest(t, mux, "GET", base, nil, 200)
	dir, err := s.externalDirectory(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	payload := filepath.Join(dir, h.ID+".bundle.json")
	var body externalPayload
	if err = externalReadJSON(payload, ReviewedContextBytesLimit*2, &body); err != nil {
		t.Fatal(err)
	}
	body.Files[0].Data = append(body.Files[0].Data, []byte("tampered")...)
	if err = durableRunJSON(payload, body); err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "GET", base+"/bundle", nil, 409)
	if externalBundleFile("../AGENTS.md") || externalBundleFile("AGENTS.md") || externalBundleFile(".luci\\plan.json") {
		t.Fatal("zip traversal/tool instruction target allowed")
	}
}
func TestExternalHandoffRequiresExactApprovedSelection(t *testing.T) {
	s, mux, p, selection := externalFixture(t)
	for _, change := range []func(*ImplementationContextRequest){func(r *ImplementationContextRequest) { r.ProjectID = "p-other" }, func(r *ImplementationContextRequest) { r.SpecificationRevision++ }, func(r *ImplementationContextRequest) { r.TaskIDs = []string{"missing"} }, func(r *ImplementationContextRequest) { r.TaskIDs = []string{"build", "build"} }} {
		changed := selection
		change(&changed)
		status := 409
		if changed.ProjectID != p.ID {
			status = 400
		}
		journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/external-handoffs", map[string]any{"selection": changed, "idempotencyKey": "external-bad-selection-0001"}, status)
	}
	s.mu.Lock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	state.DesignPlan.Screens[0].Purpose = "Unapproved design"
	err = writeJourney(p.Path, state)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/external-handoffs", map[string]any{"selection": selection, "idempotencyKey": "external-bad-selection-0002"}, 409)
}

func TestExternalHandoffReportScopeAndBaselineIntegrity(t *testing.T) {
	s, mux, p, selection := externalFixture(t)
	h := createExternalFixture(t, mux, p, selection)
	base := "/v1/product/projects/" + p.ID + "/external-handoffs/" + h.ID
	review := externalReviewFixture(t, mux, p, h)
	for _, field := range []string{"project", "handoff", "digest"} {
		t.Run(field, func(t *testing.T) {
			report := ExternalReport{ProjectID: p.ID, HandoffID: h.ID, HandoffDigest: h.Digest, Summary: "Wrong packet must not bind to this project.", Checks: []ExternalReportCheck{}}
			switch field {
			case "project":
				report.ProjectID = "p-other"
			case "handoff":
				report.HandoffID = "handoff-" + strings.Repeat("b", 24)
			case "digest":
				report.HandoffDigest = strings.Repeat("c", 64)
			}
			journeyRequest(t, mux, "POST", base+"/returns", map[string]any{"expectedReviewDigest": review.ReviewDigest, "idempotencyKey": "external-wrong-packet-" + field, "report": report}, 400)
		})
	}
	s.mu.Lock()
	row, err := s.readExternal(p, h.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(row.Handoff.Returns) != 0 {
		t.Fatal("wrong scope report persisted")
	}
	row.SourceFiles["invented-baseline.txt"] = ExternalSourceFile{SHA256: strings.Repeat("d", 64), Bytes: 1}
	err = s.writeExternal(p, row)
	s.mu.Unlock()
	if err != nil {
		t.Fatal(err)
	}
	journeyRequest(t, mux, "GET", base, nil, 409)
	journeyRequest(t, mux, "GET", base+"/review", nil, 409)
}

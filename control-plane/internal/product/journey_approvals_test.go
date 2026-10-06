package product

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Deployment fixtures follow the real review endpoints rather than granting
// authority through project bytes or test-only verification flags.
func reviewDeploymentFixture(t *testing.T, mux http.Handler, id string) (ProjectSpecification, ProjectDesignPlan) {
	t.Helper()
	base := "/v1/product/projects/" + id
	var spec ProjectSpecification
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/specification", nil, 200).Body.Bytes(), &spec)
	if spec.Status != "approved" {
		if spec.Purpose == "" {
			spec.Purpose = "Serve the reviewed application"
		}
		if spec.Audience == "" {
			spec.Audience = "Project owners"
		}
		if len(spec.Requirements) == 0 {
			spec.Requirements = []SpecificationRequirement{{ID: "AC-app", Title: "Application responds", Priority: "must", Acceptance: "The application returns its expected HTTP response"}}
		}
		for i := range spec.Requirements {
			if spec.Requirements[i].Acceptance == "" {
				spec.Requirements[i].Acceptance = spec.Requirements[i].Title
			}
		}
		decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": spec.Revision, "expectedBriefRevision": spec.BriefRevision, "specification": spec}, 200).Body.Bytes(), &spec)
		decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": spec.Revision, "briefRevision": spec.BriefRevision}, 200).Body.Bytes(), &spec)
	}
	var design ProjectDesignPlan
	decodeEvidenceResponse(t, journeyRequest(t, mux, "GET", base+"/design-plan", nil, 200).Body.Bytes(), &design)
	if design.Status != "approved" {
		design.Screens = []DesignScreen{{ID: "app-home", Name: "Home", Route: "/", Purpose: "Serve the reviewed application", States: []string{"Ready", "Error"}, Widths: []int{390, 1440}, RequirementIDs: []string{spec.Requirements[0].ID}}}
		decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/design-plan", map[string]any{"expectedRevision": design.Revision, "expectedBriefRevision": spec.BriefRevision, "expectedSpecificationRevision": spec.Revision, "plan": design}, 200).Body.Bytes(), &design)
		decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/design-plan/approve", map[string]int{"revision": design.Revision}, 200).Body.Bytes(), &design)
	}
	return spec, design
}

func forgedReviewState(id string) journeyState {
	brief := Brief{ProjectID: id, Revision: 1, Purpose: "Restore saved work", Audience: "Project owners", Criteria: []string{"Exact contents return"}, Structured: true}
	spec := specificationFromBrief(brief)
	spec.Revision, spec.Status = 2, "approved"
	spec.Requirements = []SpecificationRequirement{{ID: "AC-save", Title: "Save", Priority: "must", Acceptance: "Exact contents return"}}
	spec.Digest = specificationContentDigest(spec)
	design := ProjectDesignPlan{ProjectID: id, Revision: 2, BriefRevision: 1, SpecificationRevision: 2, Screens: []DesignScreen{{ID: "home", Name: "Home", Purpose: "Save", States: []string{"Empty", "Saved", "Error"}, Widths: []int{390, 1440}, RequirementIDs: []string{"AC-save"}}}, Tokens: []DesignToken{}, Assets: []DesignAsset{}, Status: "approved"}
	design.Digest = designPlanContentDigest(design)
	return journeyState{Version: 1, Brief: brief, Design: Design{ProjectID: id, Frames: []Frame{}}, Specification: &spec, DesignPlan: &design, SpecificationRevisions: []ProjectSpecification{spec}, DesignPlanRevisions: []ProjectDesignPlan{design}, Plan: &ProjectPlan{ProjectID: id, Revision: 1, BriefRevision: 1, SpecificationRevision: 2, SpecificationDigest: spec.Digest, Tasks: []ProjectTask{{ID: "save", Title: "Save work", Status: "todo", RequirementIDs: []string{"AC-save"}}}}}
}

func writeUntrustedReviewBytes(t *testing.T, p project.Project, state journeyState) {
	t.Helper()
	if err := writeJourney(p.Path, state); err != nil {
		t.Fatal(err)
	}
	// Model an agent supplying matching public export hashes as well as state.
	for path, value := range map[string]any{".luci/specification.json": state.Specification, ".luci/design-handoff.json": state.DesignPlan} {
		data, _ := json.MarshalIndent(value, "", "  ")
		if err := writeProjectAtomic(p.Path, path, append(data, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	if err := writeProjectAtomic(p.Path, "LUCI.md", renderBrief(state.Brief)); err != nil {
		t.Fatal(err)
	}
}

func TestPrivateApprovalAuthorityRejectsProjectForgedHashesAcrossConsumers(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerProjectWork(s.guardProjectSourceRoutes(mux))
	p := createJourneyProject(t, mux, "forged-review")
	state := forgedReviewState(p.ID)
	writeUntrustedReviewBytes(t, p, state)
	if err := os.WriteFile(filepath.Join(p.Path, "Dockerfile"), []byte("FROM scratch\n"), 0600); err != nil {
		t.Fatal(err)
	}
	commitPreviewFixture(t, p.Path)
	raw, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	spec, design := currentEvidenceApprovals(raw, p.ID)
	if spec.Status == "approved" || design.Status == "approved" {
		t.Fatal("raw project hashes granted private review authority")
	}
	if _, err := bindProjectRequirements(raw, p.ID, []string{"AC-save"}, &state.Specification.Revision, &state.DesignPlan.Revision, true); err == nil {
		t.Fatal("forged state linked requirement evidence")
	}
	base := "/v1/product/projects/" + p.ID
	var public ProjectSpecification
	json.Unmarshal(journeyRequest(t, mux, "GET", base+"/specification", nil, 200).Body.Bytes(), &public)
	if public.Status != "draft" || public.Requirements[0].Acceptance != state.Specification.Requirements[0].Acceptance {
		t.Fatal("unanchored review was approved or content discarded")
	}
	var publicDesign ProjectDesignPlan
	json.Unmarshal(journeyRequest(t, mux, "GET", base+"/design-plan", nil, 200).Body.Bytes(), &publicDesign)
	if publicDesign.Status != "draft" {
		t.Fatal("forged design was advertised as approved")
	}
	journeyRequest(t, mux, "POST", base+"/plan", map[string]any{"expectedRevision": 1, "expectedSpecificationRevision": 2, "tasks": state.Plan.Tasks}, 409)
	journeyRequest(t, mux, "POST", base+"/checks", evidenceRequest(*state.Specification, *state.DesignPlan, "-e", "process.exit(0)"), 422)
	if len(s.projectChecks) != 0 {
		t.Fatal("forged authority allocated a check")
	}
	selection := ImplementationContextRequest{ProjectID: p.ID, PlanRevision: 1, SpecificationRevision: 2, DesignPlanRevision: 2, TaskIDs: []string{"save"}}
	if _, err := s.BindImplementationContext(context.Background(), selection, "fixture-worktree", p.Path); err == nil {
		t.Fatal("forged review admitted a bound implementation run")
	}
	head, err := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.ValidateDeploymentSource(context.Background(), p.ID, strings.TrimSpace(head)); err == nil || !strings.Contains(strings.ToLower(err.Error()), "approve") {
		t.Fatal("forged reviewed commit admitted native deployment")
	}
	before := auditTree(t, p.Path)
	journeyRequest(t, mux, "POST", base+"/releases", map[string]any{"target": "local"}, 409)
	if !reflect.DeepEqual(before, auditTree(t, p.Path)) {
		t.Fatal("approval refusal changed source")
	}
	if _, err := s.approvalPath(p.ID, false); !os.IsNotExist(err) {
		t.Fatal("untrusted reads or consumers created private authority")
	}
	// A raw unauthenticated approve call cannot mint a receipt, even if it
	// supplies client actor/bearer headers and a fully valid review body.
	r := httptest.NewRequest("POST", base+"/specification/approve", strings.NewReader(`{"revision":2,"briefRevision":1}`))
	r.SetPathValue("id", p.ID)
	r.Header.Set("X-Luci-Actor", "client-supplied-reviewer")
	w := httptest.NewRecorder()
	s.approveSpecification(w, r)
	if w.Code != 401 {
		t.Fatal("unauthenticated approval minted authority")
	}
}

func TestPrivateApprovalHTTPReviewSurvivesReopenButRawAndCopiedStateDoNot(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "reviewed-project")
	spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-save")
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	gotSpec, gotDesign := currentEvidenceApprovals(state, p.ID)
	if gotSpec.Status != "approved" || gotDesign.Status != "approved" || gotSpec.Digest != spec.Digest || gotDesign.Digest != design.Digest {
		t.Fatal("authenticated review was not admitted")
	}
	path, err := s.approvalPath(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	assertPrivateFile(t, path)
	if strings.HasPrefix(path, p.Path+string(filepath.Separator)) {
		t.Fatal("private approval resides in the agent workspace")
	}
	store, err := s.readApprovalStore(p.Path, p.ID)
	if err != nil || store.Specification.ActorID != "fixture-reviewer" || store.Design.SpecificationApproval != store.Specification.ID {
		t.Fatal("actor or exact parent approval not retained")
	}
	before, _ := os.ReadFile(path)
	journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/specification/approve", map[string]int{"revision": spec.Revision, "briefRevision": spec.BriefRevision}, 200)
	after, _ := os.ReadFile(path)
	if !bytes.Equal(before, after) {
		t.Fatal("genuinely anchored idempotent approval replaced authority")
	}
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer reopened.Close()
	reloaded, err := reopened.readApprovedJourney(p.Path, p.ID)
	if err != nil || !specificationApprovalMatches(reloaded) || !designApprovalMatches(reloaded) {
		t.Fatal("controller reopen lost exact private approvals")
	}
	raw, err := readJourney(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	if specificationApprovalMatches(raw) || designApprovalMatches(raw) {
		t.Fatal("serialized state carried private authority")
	}
	data, _ := json.Marshal(state)
	var decoded journeyState
	json.Unmarshal(data, &decoded)
	if decoded.approvalAuthority != nil || strings.Contains(string(data), store.Specification.ID) {
		t.Fatal("approval receipts leaked into project serialization")
	}
	q := createJourneyProject(t, mux, "review-copy")
	copyState := forgedReviewState(q.ID)
	writeUntrustedReviewBytes(t, q, copyState)
	qPath, err := s.approvalPath(q.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(qPath, before, 0600); err != nil {
		t.Fatal(err)
	}
	copyRead, err := s.readApprovedJourney(q.Path, q.ID)
	if err != nil || specificationApprovalMatches(copyRead) || designApprovalMatches(copyRead) {
		t.Fatal("foreign project copied review authority")
	}
	other, err := New(Options{StateDir: t.TempDir(), ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	otherPath, err := other.approvalPath(p.ID, true)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(otherPath, before, 0600); err != nil {
		t.Fatal(err)
	}
	accountRead, err := other.readApprovedJourney(p.Path, p.ID)
	if err != nil || specificationApprovalMatches(accountRead) || designApprovalMatches(accountRead) {
		t.Fatal("foreign account copied review authority")
	}
}

func TestPrivateApprovalDependenciesAndLatestSelectionRejectSameRevisionEditsAndReplay(t *testing.T) {
	for _, change := range []string{"brief-bytes", "sketch-bytes", "specification-bytes", "design-bytes", "objects-deleted", "specification-save", "brief-save", "sketch-save", "design-save"} {
		t.Run(change, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "review-drift")
			spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-save")
			original, err := readJourney(p.Path, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			originalBytes, err := os.ReadFile(filepath.Join(p.Path, ".luci", "state.json"))
			if err != nil {
				t.Fatal(err)
			}
			base := "/v1/product/projects/" + p.ID
			switch change {
			case "brief-bytes":
				original.Brief.Constraints = "Changed without advancing revision"
			case "sketch-bytes":
				original.Design.Scene = json.RawMessage(`{"elements":[],"appState":{"viewBackgroundColor":"#123456"}}`)
			case "specification-bytes":
				original.Specification.Requirements[0].Acceptance = "Changed and public digest recomputed"
				original.Specification.Digest = specificationContentDigest(*original.Specification)
			case "design-bytes":
				original.DesignPlan.ImplementationNotes = "Changed and public digest recomputed"
				original.DesignPlan.Digest = designPlanContentDigest(*original.DesignPlan)
			case "objects-deleted":
				original.Specification, original.DesignPlan = nil, nil
				original.Brief.Structured = false
			case "specification-save":
				journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": spec.Revision, "expectedBriefRevision": spec.BriefRevision, "specification": spec}, 200)
			case "brief-save":
				journeyRequest(t, mux, "POST", base+"/brief", map[string]any{"revision": spec.BriefRevision, "purpose": original.Brief.Purpose, "audience": original.Brief.Audience, "criteria": original.Brief.Criteria}, 200)
			case "sketch-save":
				journeyRequest(t, mux, "POST", base+"/design", map[string]any{"revision": 0, "frames": []Frame{}}, 200)
			case "design-save":
				journeyRequest(t, mux, "POST", base+"/design-plan", map[string]any{"expectedRevision": design.Revision, "expectedBriefRevision": spec.BriefRevision, "expectedSpecificationRevision": spec.Revision, "plan": design}, 200)
			}
			if strings.HasSuffix(change, "-bytes") || change == "objects-deleted" {
				data, _ := json.Marshal(original)
				if err := os.WriteFile(filepath.Join(p.Path, ".luci", "state.json"), data, 0600); err != nil {
					t.Fatal(err)
				}
			} else {
				// Replay the exact earlier legitimately approved file after a Save.
				if err := os.WriteFile(filepath.Join(p.Path, ".luci", "state.json"), originalBytes, 0600); err != nil {
					t.Fatal(err)
				}
			}
			checked, err := s.readApprovedJourney(p.Path, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			specNow, designNow := currentEvidenceApprovals(checked, p.ID)
			if designNow.Status == "approved" {
				t.Fatal("edited dependency or replayed state retained design authority")
			}
			if change == "objects-deleted" && requireJourneyReviews(checked) == nil {
				t.Fatal("deleting structured objects downgraded to a legacy deployment bypass")
			}
			if (strings.HasPrefix(change, "specification") || strings.HasPrefix(change, "brief")) && specNow.Status == "approved" {
				t.Fatal("edited or revoked specification retained authority")
			}
			if !strings.HasSuffix(change, "-bytes") && (change == "sketch-save" || change == "design-save") && specNow.Status != "approved" {
				t.Fatal("design-only save revoked an independent unchanged specification")
			}
		})
	}
}

func TestPrivateApprovalMissingCorruptAndLinkedStoreFailClosedAndExplicitReviewRepairs(t *testing.T) {
	for _, mode := range []string{"missing", "corrupt", "linked"} {
		t.Run(mode, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "review-store")
			spec, _ := approveEvidenceJourney(t, mux, p.ID, "AC-save")
			path, err := s.approvalPath(p.ID, false)
			if err != nil {
				t.Fatal(err)
			}
			if mode == "missing" {
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "corrupt" {
				if err := os.WriteFile(path, []byte("not JSON"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			if mode == "linked" {
				outside := filepath.Join(t.TempDir(), "authority.json")
				if err := os.WriteFile(outside, []byte(`{"outside":true}`), 0600); err != nil {
					t.Fatal(err)
				}
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(outside, path); err != nil {
					t.Skip("host does not permit file symlinks; directory-link gate is tested separately")
				}
			}
			before := auditTree(t, p.Path)
			read, err := s.readApprovedJourney(p.Path, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if specificationApprovalMatches(read) || designApprovalMatches(read) || !reflect.DeepEqual(before, auditTree(t, p.Path)) {
				t.Fatal("unanchored private state granted approval or rewrote source")
			}
			want := 200
			if mode == "linked" {
				want = 500
			}
			journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/specification/approve", map[string]int{"revision": spec.Revision, "briefRevision": spec.BriefRevision}, want)
			if mode != "linked" {
				read, err = s.readApprovedJourney(p.Path, p.ID)
				if err != nil || !specificationApprovalMatches(read) || designApprovalMatches(read) {
					t.Fatal("explicit review failed to repair authority or silently reapproved design")
				}
			}
		})
	}
}

func TestPrivateApprovalHardlinkedStoreCannotGrantOrReplaceAuthority(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "review-hardlink")
	spec, _ := approveEvidenceJourney(t, mux, p.ID, "AC-save")
	path, err := s.approvalPath(p.ID, false)
	if err != nil {
		t.Fatal(err)
	}
	alias := filepath.Join(t.TempDir(), "authority-alias.json")
	if err := os.Link(path, alias); err != nil {
		t.Fatal("create host hardlink fixture:", err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	sourceBefore := auditTree(t, p.Path)
	read, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil || specificationApprovalMatches(read) || designApprovalMatches(read) {
		t.Fatal("aliased private storage conferred authority")
	}
	base := "/v1/product/projects/" + p.ID
	journeyRequest(t, mux, "POST", base+"/specification/approve", map[string]int{"revision": spec.Revision, "briefRevision": spec.BriefRevision}, 500)
	journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": spec.Revision, "expectedBriefRevision": spec.BriefRevision, "specification": spec}, 500)
	for _, name := range []string{path, alias} {
		after, err := os.ReadFile(name)
		if err != nil || !bytes.Equal(before, after) {
			t.Fatal("unsafe private file was replaced or modified")
		}
	}
	if !reflect.DeepEqual(sourceBefore, auditTree(t, p.Path)) {
		t.Fatal("unsafe authority failure changed project content")
	}
}

func TestPrivateApprovalPartialContextExportRetryRepairsWithoutNewAuthority(t *testing.T) {
	for _, kind := range []string{"specification", "design"} {
		t.Run(kind, func(t *testing.T) {
			s, mux := journeyFixture(t)
			p := createJourneyProject(t, mux, "review-export")
			spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-save")
			base := "/v1/product/projects/" + p.ID
			var draftRevision int
			var target, route string
			var body map[string]int
			if kind == "specification" {
				var draft ProjectSpecification
				decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/specification", map[string]any{"expectedRevision": spec.Revision, "expectedBriefRevision": spec.BriefRevision, "specification": spec}, 200).Body.Bytes(), &draft)
				draftRevision = draft.Revision
				target = ".luci/specification.md"
				route = base + "/specification/approve"
				body = map[string]int{"revision": draft.Revision, "briefRevision": draft.BriefRevision}
			} else {
				var draft ProjectDesignPlan
				decodeEvidenceResponse(t, journeyRequest(t, mux, "POST", base+"/design-plan", map[string]any{"expectedRevision": design.Revision, "expectedBriefRevision": spec.BriefRevision, "expectedSpecificationRevision": spec.Revision, "plan": design}, 200).Body.Bytes(), &draft)
				draftRevision = draft.Revision
				target = ".luci/design-handoff.md"
				route = base + "/design-plan/approve"
				body = map[string]int{"revision": draft.Revision}
			}
			path := filepath.Join(p.Path, filepath.FromSlash(target))
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			if err := os.Mkdir(path, 0700); err != nil {
				t.Fatal(err)
			}
			journeyRequest(t, mux, "POST", route, body, 500)
			current, err := s.readApprovedJourney(p.Path, p.ID)
			if err != nil {
				t.Fatal(err)
			}
			if kind == "specification" {
				if !specificationApprovalMatches(current) || current.Specification.Revision != draftRevision+1 {
					t.Fatal("completed private/canonical approval lost after export failure")
				}
				body["revision"] = current.Specification.Revision
			} else {
				if !designApprovalMatches(current) || current.DesignPlan.Revision != draftRevision+1 {
					t.Fatal("completed private/canonical design approval lost after export failure")
				}
				body["revision"] = current.DesignPlan.Revision
			}
			privatePath, err := s.approvalPath(p.ID, false)
			if err != nil {
				t.Fatal(err)
			}
			privateBefore, err := os.ReadFile(privatePath)
			if err != nil {
				t.Fatal(err)
			}
			if err := os.Remove(path); err != nil {
				t.Fatal(err)
			}
			journeyRequest(t, mux, "POST", route, body, 200)
			privateAfter, err := os.ReadFile(privatePath)
			if err != nil || !bytes.Equal(privateBefore, privateAfter) {
				t.Fatal("export retry replaced exact existing approval")
			}
			export, err := os.ReadFile(path)
			if err != nil || !strings.Contains(string(export), "Status: approved") {
				t.Fatal("explicit retry did not repair actual review export")
			}
		})
	}
}

func TestPrivateApprovalAnalyzedSaveRevokesBeforeOldSnapshotReplay(t *testing.T) {
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "review-analysis")
	spec, _ := approveEvidenceJourney(t, mux, p.ID, "AC-save")
	original, err := os.ReadFile(filepath.Join(p.Path, ".luci", "state.json"))
	if err != nil {
		t.Fatal(err)
	}
	analysisWrite(t, p.Path, "app.ts", "export const application = true;")
	snapshot, err := s.RepositoryAnalysisSnapshot(context.Background(), p.ID, "worktree")
	if err != nil {
		t.Fatal(err)
	}
	proposal := snapshot.Specification
	proposal.Status, proposal.Digest = "draft", ""
	proposal.Purpose = "Analyzed updated application"
	if _, err := s.ApplyRepositoryAnalysisDraft(context.Background(), p.ID, spec.Revision, spec.BriefRevision, "worktree", snapshot.Source.Digest, proposal); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(p.Path, ".luci", "state.json"), original, 0600); err != nil {
		t.Fatal(err)
	}
	current, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil || specificationApprovalMatches(current) || designApprovalMatches(current) {
		t.Fatal("analysis draft save retained replayable prior approvals")
	}
}

func TestPrivateApprovalBackupImportsContentWithoutPrivateAuthority(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerBackups(mux)
	p := createJourneyProject(t, mux, "review-backup")
	spec, design := approveEvidenceJourney(t, mux, p.ID, "AC-save")
	private, err := s.readApprovalStore(p.Path, p.ID)
	if err != nil {
		t.Fatal(err)
	}
	commitPreviewFixture(t, p.Path)
	backup := journeyRequest(t, mux, "POST", "/v1/product/projects/"+p.ID+"/backups", map[string]any{}, 200)
	archive, err := zip.NewReader(bytes.NewReader(backup.Body.Bytes()), int64(backup.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range archive.File {
		if strings.Contains(entry.Name, "review-approvals") {
			t.Fatal("private approval storage entered project archive")
		}
		reader, err := entry.Open()
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(reader)
		reader.Close()
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(data, []byte(private.Specification.ID)) || bytes.Contains(data, []byte(private.Design.ID)) {
			t.Fatal("private approval identities entered project archive")
		}
	}
	var restored project.Project
	decodeEvidenceResponse(t, backupUpload(t, mux, "review-restored", backup.Body.Bytes(), 201).Body.Bytes(), &restored)
	current, err := s.readApprovedJourney(restored.Path, restored.ID)
	if err != nil {
		t.Fatal(err)
	}
	if current.Specification == nil || current.DesignPlan == nil || len(current.SpecificationRevisions) == 0 || len(current.DesignPlanRevisions) == 0 || current.Specification.Purpose != spec.Purpose || current.DesignPlan.Screens[0].Name != design.Screens[0].Name {
		t.Fatal("import discarded reviewed content or historical revisions")
	}
	if specificationApprovalMatches(current) || designApprovalMatches(current) || currentSpecification(current).Status != "draft" || currentDesignPlan(current).Status != "draft" || requireJourneyReviews(current) == nil {
		t.Fatal("archive import carried review authority into a new project")
	}
	if _, err := s.approvalPath(restored.ID, false); !os.IsNotExist(err) {
		t.Fatal("archive import minted private authority")
	}
}

func TestPrivateApprovalAllReleaseEntrypointsRefuseChangedCommittedReviewExports(t *testing.T) {
	for _, target := range []string{"local", "cloud", "pages"} {
		for _, kind := range []string{"specification", "design"} {
			t.Run(target+"/"+kind, func(t *testing.T) {
				var s *Server
				var mux *http.ServeMux
				var p project.Project
				var route string
				var request any
				var assertNoUpstream func()
				switch target {
				case "local":
					s, mux = journeyFixture(t)
					p = createJourneyProject(t, mux, "review-local-export")
					reviewDeploymentFixture(t, mux, p.ID)
					analysisWrite(t, p.Path, "Dockerfile", "FROM scratch\n")
					request = map[string]any{"target": "local"}
					route = "/v1/product/projects/" + p.ID + "/releases"
					calls := 0
					s.runner = func(ctx context.Context, exe string, args []string, dir string) (string, error) {
						if exe == "docker" {
							calls++
							return "", errors.New("unexpected Docker request")
						}
						return RunCommand(ctx, exe, args, dir)
					}
					assertNoUpstream = func() {
						if calls != 0 {
							t.Fatal("changed reviewed export reached Docker")
						}
					}
				case "cloud":
					provider := &cloudProviderFixture{}
					var req CloudReleaseRequest
					s, mux, p, req, _ = cloudReleaseFixture(t, provider)
					route = "/v1/product/projects/" + p.ID + "/releases/cloud"
					request = &req
					assertNoUpstream = func() {
						provider.mu.Lock()
						defer provider.mu.Unlock()
						if provider.projects != 0 || provider.applications != 0 || provider.deployments != 0 || provider.updates != 0 {
							t.Fatal("changed reviewed export reached cloud resources")
						}
					}
				case "pages":
					var req PagesReleaseRequest
					var provider *pagesFixture
					s, mux, p, req, provider = pagesFixtureServer(t)
					route = "/v1/product/projects/" + p.ID + "/releases/pages"
					request = &req
					assertNoUpstream = func() {
						if provider.calls != 0 {
							t.Fatal("changed reviewed export reached GitHub publishing")
						}
					}
				}
				state, err := s.readApprovedJourney(p.Path, p.ID)
				if err != nil || requireJourneyReviews(state) != nil {
					t.Fatal("real HTTP review fixture was not approved")
				}
				privatePath, err := s.approvalPath(p.ID, false)
				if err != nil {
					t.Fatal(err)
				}
				privateBefore, err := os.ReadFile(privatePath)
				if err != nil {
					t.Fatal(err)
				}
				var path string
				var value any
				if kind == "specification" {
					export := *state.Specification
					export.Purpose = "Unreviewed committed purpose"
					export.Digest = specificationContentDigest(export)
					path, value = ".luci/specification.json", export
				} else {
					export := *state.DesignPlan
					export.ImplementationNotes = "Unreviewed committed instructions"
					export.Digest = designPlanContentDigest(export)
					path, value = ".luci/design-handoff.json", export
				}
				data, err := json.Marshal(value)
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(p.Path, filepath.FromSlash(path)), data, 0600); err != nil {
					t.Fatal(err)
				}
				commitPreviewFixture(t, p.Path)
				head, err := s.journeyGit(context.Background(), p.Path, "rev-parse", "HEAD")
				if err != nil {
					t.Fatal(err)
				}
				commit := strings.TrimSpace(head)
				switch req := request.(type) {
				case *CloudReleaseRequest:
					req.Commit = commit
				case *PagesReleaseRequest:
					req.Commit = commit
				}
				response := journeyRequest(t, mux, "POST", route, request, 409)
				if !strings.Contains(response.Body.String(), "review_approval_required") {
					t.Fatal("release refused for an unrelated gate")
				}
				assertNoUpstream()
				if err := s.ValidateDeploymentSource(context.Background(), p.ID, commit); err == nil {
					t.Fatal("native application deployment accepted changed review exports")
				}
				privateAfter, err := os.ReadFile(privatePath)
				if err != nil || !bytes.Equal(privateBefore, privateAfter) {
					t.Fatal("release refusal changed private review authority")
				}
			})
		}
	}
}

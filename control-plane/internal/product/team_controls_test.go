package product

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

const controlTestSheet = `<worksheet xmlns="http://schemas.openxmlformats.org/spreadsheetml/2006/main"><sheetData><row r="1"><c r="A1" t="inlineStr"><is><t>ID</t></is></c><c r="B1" t="inlineStr"><is><t>Requirement</t></is></c><c r="C1" t="inlineStr"><is><t>Category</t></is></c><c r="D1" t="inlineStr"><is><t>Applicability</t></is></c><c r="E1" t="inlineStr"><is><t>Owner</t></is></c><c r="F1" t="inlineStr"><is><t>Evidence Expected</t></is></c></row><row r="2"><c r="A2" t="inlineStr"><is><t>SEC-1</t></is></c><c r="B2" t="inlineStr"><is><t>Show access controls</t></is></c><c r="C2" t="inlineStr"><is><t>Security</t></is></c><c r="D2" t="inlineStr"><is><t>Every release</t></is></c><c r="E2" t="inlineStr"><is><t>Team</t></is></c><c r="F2" t="inlineStr"><is><t>Reviewed access test</t></is></c><c r="G2" t="inlineStr"><is><t>Unknown source column</t></is></c></row></sheetData></worksheet>`

func controlTestWorkbook(t *testing.T, sheet string, extras map[string]string) []byte {
	t.Helper()
	var out bytes.Buffer
	archive := zip.NewWriter(&out)
	files := map[string]string{"xl/workbook.xml": `<workbook xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships"><sheets><sheet name="Controls" r:id="rId1"/></sheets></workbook>`, "xl/_rels/workbook.xml.rels": `<Relationships><Relationship Id="rId1" Target="worksheets/sheet1.xml" Type="http://schemas.openxmlformats.org/officeDocument/2006/relationships/worksheet"/></Relationships>`, "xl/worksheets/sheet1.xml": sheet}
	for name, data := range extras {
		files[name] = data
	}
	for name, data := range files {
		writer, err := archive.Create(name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = writer.Write([]byte(data)); err != nil {
			t.Fatal(err)
		}
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	return out.Bytes()
}
func controlTestHandler(t *testing.T) (*Server, http.Handler, string) {
	t.Helper()
	s, mux := journeyFixture(t)
	p := createJourneyProject(t, mux, "control-project")
	access := management.TeamAccess{Team: management.Team{ID: "team-test-control", Revision: 1}, Member: management.TeamMember{UserID: "admin", Revision: 1}, ProjectID: p.ID, ProjectRevision: 1, Permissions: management.TeamPermissions{Read: true, Edit: true, Admin: true}}
	handler := contextHistoryActorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mux.ServeHTTP(w, r.WithContext(management.WithTeamAccess(r.Context(), access)))
	}), management.User{ID: "admin", Username: "admin", Role: "member"})
	return s, handler, p.ID
}
func controlTestRequest(t *testing.T, h http.Handler, method, path string, body any, want int) *httptest.ResponseRecorder {
	t.Helper()
	var data []byte
	switch value := body.(type) {
	case []byte:
		data = value
	default:
		data, _ = json.Marshal(value)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, r)
	if w.Code != want {
		t.Fatalf("%s %s got %d want %d: %s", method, path, w.Code, want, w.Body.String())
	}
	return w
}
func controlTestColumns() map[string]string {
	return map[string]string{"id": "A", "requirement": "B", "category": "C", "applicability": "D", "owner": "E", "evidenceExpected": "F"}
}

func TestTeamControlsXLSXBoundedSafetyAndPreview(t *testing.T) {
	raw := controlTestWorkbook(t, controlTestSheet, nil)
	book, err := parseControlWorkbook(context.Background(), raw)
	if err != nil || len(book.Sheets) != 1 || len(book.Sheets[0].Rows) != 1 || book.Sheets[0].Rows[0].Cells["G"] != "Unknown source column" {
		t.Fatalf("workbook: %+v %v", book, err)
	}
	if _, err := parseControlWorkbook(context.Background(), controlTestWorkbook(t, controlTestSheet, map[string]string{"xl/vbaProject.bin": "macro"})); err == nil {
		t.Fatal("macro accepted")
	}
	if _, err := parseControlWorkbook(context.Background(), controlTestWorkbook(t, controlTestSheet, map[string]string{"xl/externalLinks/externalLink1.xml": "external"})); err == nil {
		t.Fatal("external link accepted")
	}
	if _, err := parseControlWorkbook(context.Background(), controlTestWorkbook(t, controlTestSheet, map[string]string{"xl/worksheets/_rels/sheet1.xml.rels": `<Relationships><Relationship Id="rId2" Target="https://outside.example" TargetMode="External"/></Relationships>`})); err == nil {
		t.Fatal("external relationship accepted")
	}
	duplicateEmpty := strings.Replace(controlTestSheet, `<c r="G2" t="inlineStr"><is><t>Unknown source column</t></is></c>`, `<c r="G2" t="inlineStr"><is><t></t></is></c><c r="G2" t="inlineStr"><is><t>late value</t></is></c>`, 1)
	if _, err := parseControlWorkbook(context.Background(), controlTestWorkbook(t, duplicateEmpty, nil)); err == nil {
		t.Fatal("duplicate empty cell accepted")
	}
	deep := strings.Repeat("<x>", 33) + strings.Repeat("</x>", 33)
	if err := boundedControlXML(context.Background(), []byte(deep), false); err == nil {
		t.Fatal("deep XML accepted before object decoding")
	}
	formula := strings.Replace(controlTestSheet, `<c r="G2" t="inlineStr"><is><t>Unknown source column</t></is></c>`, `<c r="G2"><f>NOW()</f><v>0</v></c>`, 1)
	book, err = parseControlWorkbook(context.Background(), controlTestWorkbook(t, formula, nil))
	if err != nil || len(book.Issues) != 1 {
		t.Fatalf("formula was not flagged: %+v %v", book, err)
	}
	if _, err := parseControlWorkbook(context.Background(), append(raw, bytes.Repeat([]byte{'x'}, controlWorkbookLimit)...)); err == nil {
		t.Fatal("oversized workbook accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := parseControlWorkbook(ctx, raw); err == nil {
		t.Fatal("cancel ignored")
	}
}

func TestTeamControlsCleanSerializedCollectionsAndOptionalMappings(t *testing.T) {
	_, handler, _ := controlTestHandler(t)
	raw := controlTestWorkbook(t, controlTestSheet, nil)
	preview := controlTestRequest(t, handler, "POST", "/v1/product/team-controls/preview", raw, 200)
	var previewJSON struct {
		Workbook struct {
			Issues []string `json:"issues"`
		} `json:"workbook"`
	}
	if err := json.Unmarshal(preview.Body.Bytes(), &previewJSON); err != nil || previewJSON.Workbook.Issues == nil || !bytes.Contains(preview.Body.Bytes(), []byte(`"issues":[]`)) {
		t.Fatalf("clean preview must contain an array: %s %v", preview.Body.String(), err)
	}
	columns := controlTestColumns()
	columns["severity"], columns["reviewCadence"] = "", ""
	input := map[string]any{"expectedRevision": 0, "sourceSHA256": controlSourceHash(raw), "sheet": "Controls", "columns": columns, "workbookBase64": base64.StdEncoding.EncodeToString(raw)}
	first := controlTestRequest(t, handler, "POST", "/v1/product/team-controls/compare", input, 200)
	if !bytes.Contains(first.Body.Bytes(), []byte(`"changed":[]`)) || !bytes.Contains(first.Body.Bytes(), []byte(`"removed":[]`)) {
		t.Fatal(first.Body.String())
	}
	var diff struct {
		DiffDigest string `json:"diffDigest"`
	}
	if err := json.Unmarshal(first.Body.Bytes(), &diff); err != nil {
		t.Fatal(err)
	}
	input["idempotencyKey"], input["expectedDiffDigest"] = "optional-fields-key-123456", diff.DiffDigest
	controlTestRequest(t, handler, "POST", "/v1/product/team-controls/import", input, 200)
	input["expectedRevision"] = 1
	noOp := controlTestRequest(t, handler, "POST", "/v1/product/team-controls/compare", input, 200)
	for _, field := range []string{`"added":[]`, `"changed":[]`, `"removed":[]`} {
		if !bytes.Contains(noOp.Body.Bytes(), []byte(field)) {
			t.Fatalf("missing %s: %s", field, noOp.Body.String())
		}
	}
	columns["severity"] = "A"
	controlTestRequest(t, handler, "POST", "/v1/product/team-controls/compare", input, 422)
}

func TestTeamControlsCompareImportAssessmentAndRevocation(t *testing.T) {
	s, h, id := controlTestHandler(t)
	raw := controlTestWorkbook(t, controlTestSheet, nil)
	root := "/v1/product/team-controls"
	preview := controlTestRequest(t, h, "POST", root+"/preview", raw, 200)
	if !strings.Contains(preview.Body.String(), "SEC-1") {
		t.Fatal("preview omitted sample")
	}
	base := map[string]any{"expectedRevision": 0, "sourceSHA256": controlSourceHash(raw), "sheet": "Controls", "columns": controlTestColumns(), "workbookBase64": base64.StdEncoding.EncodeToString(raw)}
	compared := controlTestRequest(t, h, "POST", root+"/compare", base, 200)
	var diff struct {
		DiffDigest string   `json:"diffDigest"`
		Added      []string `json:"added"`
		Changed    []string `json:"changed"`
	}
	if json.Unmarshal(compared.Body.Bytes(), &diff) != nil || len(diff.Added) != 1 || diff.Added[0] != "SEC-1" {
		t.Fatal(compared.Body.String())
	}
	base["idempotencyKey"] = "control-import-key-123456"
	base["expectedDiffDigest"] = diff.DiffDigest
	imported := controlTestRequest(t, h, "POST", root+"/import", base, 200)
	var library TeamControlRevision
	if json.Unmarshal(imported.Body.Bytes(), &library) != nil || library.Revision != 1 || library.Controls[0].Unmapped["G"] != "Unknown source column" {
		t.Fatal(imported.Body.String())
	}
	firstPath, firstLedger, err := s.readTeamControls("team-test-control")
	otherPath, otherLedger, otherErr := s.readTeamControls("team-other-control")
	if err != nil || otherErr != nil || firstPath == otherPath || len(firstLedger.Revisions) != 1 || len(otherLedger.Revisions) != 0 {
		t.Fatal("team libraries share state", err, otherErr)
	}
	controlTestRequest(t, h, "POST", root+"/import", base, 200)
	base["idempotencyKey"] = "control-import-key-654321"
	controlTestRequest(t, h, "POST", root+"/import", base, 409)
	view := controlTestRequest(t, h, "GET", root, nil, 200)
	if !strings.Contains(view.Body.String(), library.Digest) {
		t.Fatal("library unreadable")
	}
	projectPath := "/v1/product/projects/" + id + "/control-assessment"
	view = controlTestRequest(t, h, "GET", projectPath, nil, 200)
	var assessmentView struct {
		ContextDigest string `json:"contextDigest"`
	}
	if json.Unmarshal(view.Body.Bytes(), &assessmentView) != nil || assessmentView.ContextDigest == "" {
		t.Fatal(view.Body.String())
	}
	controlTestRequest(t, h, "POST", projectPath, map[string]any{"expectedRevision": 0, "libraryRevision": 1, "libraryDigest": library.Digest, "contextDigest": assessmentView.ContextDigest, "idempotencyKey": "unsupported-no-evidence-123", "results": []ControlAssessmentResult{{ControlID: "SEC-1", Status: "supported", EvidenceRefs: []DiscoveryEvidenceLink{}}}}, 422)
	saved := controlTestRequest(t, h, "POST", projectPath, map[string]any{"expectedRevision": 0, "libraryRevision": 1, "libraryDigest": library.Digest, "contextDigest": assessmentView.ContextDigest, "idempotencyKey": "control-assessment-key-123", "results": []ControlAssessmentResult{{ControlID: "SEC-1", Status: "missing", EvidenceRefs: []DiscoveryEvidenceLink{}, Note: "Needs a reviewed access test"}}}, 200)
	var assessment ControlAssessment
	if json.Unmarshal(saved.Body.Bytes(), &assessment) != nil || assessment.Revision != 1 {
		t.Fatal(saved.Body.String())
	}
	report := controlTestRequest(t, h, "GET", projectPath+"/report.xlsx", nil, 200)
	if report.Header().Get("Content-Type") != "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet" || len(report.Header().Get("X-Content-SHA256")) != 64 {
		t.Fatal("report lacked exact private XLSX identity")
	}
	archive, err := zip.NewReader(bytes.NewReader(report.Body.Bytes()), int64(report.Body.Len()))
	if err != nil {
		t.Fatal(err)
	}
	var worksheet string
	for _, part := range archive.File {
		if part.Name == "xl/worksheets/sheet1.xml" {
			reader, e := part.Open()
			if e != nil {
				t.Fatal(e)
			}
			raw, e := io.ReadAll(reader)
			reader.Close()
			if e != nil {
				t.Fatal(e)
			}
			worksheet = string(raw)
		}
	}
	if !strings.Contains(worksheet, "Show access controls") || !strings.Contains(worksheet, "Needs a reviewed access test") || !strings.Contains(worksheet, `>current<`) || strings.Contains(worksheet, "<f>") {
		t.Fatal("report lost source, finding or truth state")
	}
	proposal := controlTestRequest(t, h, "POST", projectPath+"/gaps/SEC-1/proposal", map[string]any{"assessmentRevision": 1, "target": "design", "summary": "Design needs access review evidence"}, 200)
	if !strings.Contains(proposal.Body.String(), "pending-review") {
		t.Fatal(proposal.Body.String())
	}
	var pending ControlGapProposal
	if json.Unmarshal(proposal.Body.Bytes(), &pending) != nil {
		t.Fatal(proposal.Body.String())
	}
	view = controlTestRequest(t, h, "GET", projectPath, nil, 200)
	var nextAssessment struct {
		Assessment ControlAssessment `json:"assessment"`
	}
	if json.Unmarshal(view.Body.Bytes(), &nextAssessment) != nil || nextAssessment.Assessment.Revision != 2 {
		t.Fatal(view.Body.String())
	}
	plan := controlTestRequest(t, h, "GET", "/v1/product/projects/"+id+"/design-plan", nil, 200)
	var design ProjectDesignPlan
	if json.Unmarshal(plan.Body.Bytes(), &design) != nil {
		t.Fatal(plan.Body.String())
	}
	applyPath := projectPath + "/gaps/SEC-1/proposal/" + pending.ID + "/apply"
	applyRequest := map[string]any{"expectedAssessmentRevision": 2, "expectedContextDigest": assessmentView.ContextDigest, "expectedTargetRevision": design.Revision}
	applied := controlTestRequest(t, h, "POST", applyPath, applyRequest, 200)
	if !strings.Contains(applied.Body.String(), "applied-draft") {
		t.Fatal(applied.Body.String())
	}
	controlTestRequest(t, h, "POST", applyPath, applyRequest, 200)
	plan = controlTestRequest(t, h, "GET", "/v1/product/projects/"+id+"/design-plan", nil, 200)
	if json.Unmarshal(plan.Body.Bytes(), &design) != nil || design.Status != "draft" || !strings.Contains(design.ImplementationNotes, pending.ID) {
		t.Fatal(plan.Body.String())
	}
	view = controlTestRequest(t, h, "GET", projectPath, nil, 200)
	if !strings.Contains(view.Body.String(), `"state":"stale"`) {
		t.Fatal("applying a gap must stale the previous assessment", view.Body.String())
	}
	reimportRaw := controlTestWorkbook(t, strings.Replace(controlTestSheet, "Show access controls", "Show restricted access controls", 1), nil)
	reimport := map[string]any{"expectedRevision": 1, "sourceSHA256": controlSourceHash(reimportRaw), "sheet": "Controls", "columns": controlTestColumns(), "workbookBase64": base64.StdEncoding.EncodeToString(reimportRaw)}
	compared = controlTestRequest(t, h, "POST", root+"/compare", reimport, 200)
	if json.Unmarshal(compared.Body.Bytes(), &diff) != nil || len(diff.Changed) != 1 || diff.Changed[0] != "SEC-1" {
		t.Fatal("reimport diff", compared.Body.String())
	}
	reimport["idempotencyKey"] = "control-reimport-key-123456"
	reimport["expectedDiffDigest"] = diff.DiffDigest
	controlTestRequest(t, h, "POST", root+"/import", reimport, 200)
	staleReport := controlTestRequest(t, h, "GET", projectPath+"/report.xlsx", nil, 200)
	if staleReport.Header().Get("X-Luci-Assessment-State") != "stale" {
		t.Fatal("old assessment export claimed current after reimport")
	}
	plan = controlTestRequest(t, h, "GET", "/v1/product/projects/"+id+"/design-plan", nil, 200)
	if json.Unmarshal(plan.Body.Bytes(), &design) != nil || design.Status != "outdated" {
		t.Fatal("changed team library did not stale Design", plan.Body.String())
	}
	// Read and edit enforcement must be local as well as at the team gateway.
	denied := contextHistoryActorHandler(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		access := management.TeamAccess{Team: management.Team{ID: "team-test-control"}, ProjectID: "another", Permissions: management.TeamPermissions{Read: true, Edit: true}}
		mux := http.NewServeMux()
		s.Register(mux)
		mux.ServeHTTP(w, r.WithContext(management.WithTeamAccess(r.Context(), access)))
	}), management.User{ID: "admin", Username: "admin", Role: "member"})
	controlTestRequest(t, denied, "GET", projectPath, nil, 403)
}

func TestTeamControlsDiscoverGapCreatesUnapprovedDraftOnce(t *testing.T) {
	_, h, id := controlTestHandler(t)
	raw := controlTestWorkbook(t, controlTestSheet, nil)
	root := "/v1/product/team-controls"
	assessmentPath := "/v1/product/projects/" + id + "/control-assessment"
	input := map[string]any{"expectedRevision": 0, "sourceSHA256": controlSourceHash(raw), "sheet": "Controls", "columns": controlTestColumns(), "workbookBase64": base64.StdEncoding.EncodeToString(raw)}
	var diff ControlDiffReceiptForTest
	compared := controlTestRequest(t, h, "POST", root+"/compare", input, 200)
	if json.Unmarshal(compared.Body.Bytes(), &diff) != nil {
		t.Fatal(compared.Body.String())
	}
	input["idempotencyKey"] = "discover-import-key-123456"
	input["expectedDiffDigest"] = diff.DiffDigest
	var library TeamControlRevision
	imported := controlTestRequest(t, h, "POST", root+"/import", input, 200)
	if json.Unmarshal(imported.Body.Bytes(), &library) != nil {
		t.Fatal(imported.Body.String())
	}
	view := controlTestRequest(t, h, "GET", assessmentPath, nil, 200)
	var current struct {
		ContextDigest string `json:"contextDigest"`
	}
	if json.Unmarshal(view.Body.Bytes(), &current) != nil {
		t.Fatal(view.Body.String())
	}
	controlTestRequest(t, h, "POST", assessmentPath, map[string]any{"expectedRevision": 0, "libraryRevision": 1, "libraryDigest": library.Digest, "contextDigest": current.ContextDigest, "idempotencyKey": "discover-assessment-key-123456", "results": []ControlAssessmentResult{{ControlID: "SEC-1", Status: "unknown", EvidenceRefs: []DiscoveryEvidenceLink{}, Note: "Missing source"}}}, 200)
	var proposed ControlGapProposal
	proposal := controlTestRequest(t, h, "POST", assessmentPath+"/gaps/SEC-1/proposal", map[string]any{"assessmentRevision": 1, "target": "discover", "summary": "Clarify access-control acceptance and evidence"}, 200)
	if json.Unmarshal(proposal.Body.Bytes(), &proposed) != nil {
		t.Fatal(proposal.Body.String())
	}
	specification := controlTestRequest(t, h, "GET", "/v1/product/projects/"+id+"/specification", nil, 200)
	var spec ProjectSpecification
	if json.Unmarshal(specification.Body.Bytes(), &spec) != nil {
		t.Fatal(specification.Body.String())
	}
	applyPath := assessmentPath + "/gaps/SEC-1/proposal/" + proposed.ID + "/apply"
	req := map[string]any{"expectedAssessmentRevision": 2, "expectedContextDigest": current.ContextDigest, "expectedTargetRevision": spec.Revision}
	controlTestRequest(t, h, "POST", applyPath, req, 200)
	controlTestRequest(t, h, "POST", applyPath, req, 200)
	specification = controlTestRequest(t, h, "GET", "/v1/product/projects/"+id+"/specification", nil, 200)
	if json.Unmarshal(specification.Body.Bytes(), &spec) != nil || spec.Status != "draft" || len(spec.Questions) != 1 || spec.Questions[0].ID != proposed.ID {
		t.Fatal("gap did not become one unapproved Discover question", specification.Body.String())
	}
}

type ControlDiffReceiptForTest struct {
	DiffDigest string `json:"diffDigest"`
}

func TestTeamControlsDesignSaveRequiresObservedLibrary(t *testing.T) {
	s, h, id := controlTestHandler(t)
	path, err := teamControlPath(s, "team-test-control")
	if err != nil {
		t.Fatal(err)
	}
	first := TeamControlRevision{Revision: 1, Controls: []TeamControl{}, Digest: reviewDigest([]TeamControl{}), SourceSHA256: strings.Repeat("a", 64), KeyDigest: strings.Repeat("c", 64)}
	if err := writeTeamControls(path, teamControlLedger{Version: 1, TeamID: "team-test-control", Revisions: []TeamControlRevision{first}}); err != nil {
		t.Fatal(err)
	}
	url := "/v1/product/projects/" + id + "/design-plan"
	var plan ProjectDesignPlan
	if err := json.Unmarshal(controlTestRequest(t, h, "GET", url, nil, 200).Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.CurrentControlLibraryRevision != 1 || plan.CurrentControlLibraryDigest != first.Digest {
		t.Fatal("current team library missing from design read")
	}
	request := map[string]any{"expectedRevision": plan.Revision, "expectedBriefRevision": plan.BriefRevision, "expectedSpecificationRevision": plan.SpecificationRevision, "plan": plan, "expectedControlLibraryRevision": plan.CurrentControlLibraryRevision, "expectedControlLibraryDigest": plan.CurrentControlLibraryDigest}
	second := first
	second.Revision = 2
	second.Controls = []TeamControl{{ID: "SEC-1", Requirement: "Review access", Sheet: "Controls", Row: 2}}
	second.Digest = reviewDigest(second.Controls)
	if err := writeTeamControls(path, teamControlLedger{Version: 1, TeamID: "team-test-control", Revisions: []TeamControlRevision{first, second}}); err != nil {
		t.Fatal(err)
	}
	controlTestRequest(t, h, "POST", url, request, 409)
	if err := json.Unmarshal(controlTestRequest(t, h, "GET", url, nil, 200).Body.Bytes(), &plan); err != nil {
		t.Fatal(err)
	}
	if plan.CurrentControlLibraryRevision != 2 || plan.CurrentControlLibraryDigest != second.Digest {
		t.Fatal("latest team library missing from design read")
	}
	request["plan"] = plan
	request["expectedControlLibraryRevision"] = plan.CurrentControlLibraryRevision
	request["expectedControlLibraryDigest"] = plan.CurrentControlLibraryDigest
	var saved ProjectDesignPlan
	if err := json.Unmarshal(controlTestRequest(t, h, "POST", url, request, 200).Body.Bytes(), &saved); err != nil {
		t.Fatal(err)
	}
	if saved.ControlLibraryRevision != 2 || saved.ControlLibraryDigest != second.Digest {
		t.Fatal("saved design not source-bound")
	}
}

func TestTeamControlsBindAndStaleOrchestrationContract(t *testing.T) {
	s, handler, req, base, _, _ := contractFixture(t)
	teamID := "team-contract-controls"
	path, err := teamControlPath(s, teamID)
	if err != nil {
		t.Fatal(err)
	}
	control := TeamControl{ID: "SEC-1", Requirement: "Review access controls", Sheet: "Controls", Row: 2, Unmapped: map[string]string{}, UnmappedHeaders: map[string]string{}}
	first := TeamControlRevision{Revision: 1, Digest: reviewDigest([]TeamControl{control}), SourceSHA256: strings.Repeat("a", 64), Sheet: "Controls", Columns: controlTestColumns(), Controls: []TeamControl{control}, Added: []string{"SEC-1"}, Changed: []string{}, Removed: []string{}, DiffDigest: strings.Repeat("b", 64), ActorID: "team-owner", KeyDigest: strings.Repeat("c", 64)}
	if err := writeTeamControls(path, teamControlLedger{Version: 1, TeamID: teamID, Revisions: []TeamControlRevision{first}}); err != nil {
		t.Fatal(err)
	}
	access := management.TeamAccess{Team: management.Team{ID: teamID, Revision: 1}, Member: management.TeamMember{UserID: "contract-owner", Revision: 1}, ProjectID: req.ImplementationContext.ProjectID, ProjectRevision: 1, Permissions: management.TeamPermissions{Read: true, Edit: true, Execute: true}}
	teamHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handler.ServeHTTP(w, r.WithContext(management.WithTeamAccess(r.Context(), access)))
	})
	view := contractResponse(t, journeyRequest(t, teamHandler, "POST", base+"/orchestration-contracts/prepare", contractRequest(req), 201).Body.Bytes())
	if view.Contract.Input.Standards.Revision != 1 || view.Contract.Input.Standards.Digest != first.Digest || view.Contract.Input.Standards.Status != "team-library-unassessed" {
		t.Fatal("contract lacked exact library", view.Contract.Input.Standards)
	}
	secondControl := control
	secondControl.Requirement = "Review changed access controls"
	second := first
	second.Revision = 2
	second.Controls = []TeamControl{secondControl}
	second.Digest = reviewDigest(second.Controls)
	second.Changed = []string{"SEC-1"}
	second.Added = []string{}
	if err := writeTeamControls(path, teamControlLedger{Version: 1, TeamID: teamID, Revisions: []TeamControlRevision{first, second}}); err != nil {
		t.Fatal(err)
	}
	p, err := s.resolveProject(req.ImplementationContext.ProjectID)
	if err != nil {
		t.Fatal(err)
	}
	if freshness := s.orchestrationFreshness(p, view.Contract); freshness.Status != "changed" {
		t.Fatal("import did not stale future dispatch", freshness)
	}
}

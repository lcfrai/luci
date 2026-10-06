package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func TestReviewedContextInspectionKeepsHistoricalDocumentsAfterAmendmentWithoutWrites(t *testing.T) {
	env, saved, _ := createReviewedFixture(t, "codex")
	capture, err := env.api.readReviewedContext(saved.ID, saved.ImplementationContext, saved.ReviewedContext)
	if err != nil {
		t.Fatal(err)
	}
	expected := map[string][]byte{}
	for _, file := range capture.Files {
		expected[file.Path] = file.Data
	}
	base := "/v1/product/projects/" + saved.ProjectID
	var current product.ProjectSpecification
	if json.Unmarshal(env.do(t, "GET", base+"/specification", "").Body.Bytes(), &current) != nil {
		t.Fatal("invalid current specification")
	}
	current.Purpose = "New collaborative purpose B must remain separate"
	body, _ := json.Marshal(map[string]any{"expectedRevision": current.Revision, "expectedBriefRevision": current.BriefRevision, "specification": current})
	if w := env.do(t, "POST", base+"/specification", string(body)); w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &current) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	var design product.ProjectDesignPlan
	if w := env.do(t, "GET", base+"/design-plan", ""); w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &design) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	design.Screens[0].Purpose = "New collaborative screen B"
	body, _ = json.Marshal(map[string]any{"expectedRevision": design.Revision, "expectedBriefRevision": current.BriefRevision, "expectedSpecificationRevision": current.Revision, "plan": design})
	if w := env.do(t, "POST", base+"/design-plan", string(body)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	body, _ = json.Marshal(map[string]any{"expectedRevision": saved.ImplementationContext.PlanRevision, "tasks": []product.ProjectTask{{ID: "save-note", Title: "New collaborative plan B", Status: "todo"}}})
	if w := env.do(t, "POST", base+"/plan", string(body)); w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	beforeRecord, _ := env.st.Get(saved.ID)
	beforeCreates, beforeDestroys := len(env.be.specs), env.be.destroyCount()
	beforeFiles := inspectFixtureFiles(t, env.conversations)
	beforeCurrent := env.do(t, "GET", base+"/context", "").Body.String()
	for _, kind := range []string{"brief", "definition", "design", "plan"} {
		w := env.do(t, "GET", "/v1/sessions/"+saved.ID+"/reviewed-context?document="+kind, "")
		var result reviewedContextInspection
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &result) != nil {
			t.Fatal(kind, w.Code, w.Body.String())
		}
		want := expected[reviewedDocumentPath(kind)]
		var compact bytes.Buffer
		if err := json.Compact(&compact, want); err != nil {
			t.Fatal(err)
		}
		if result.SessionID != saved.ID || result.ProjectID != saved.ProjectID || result.WorktreeID != saved.WorktreeID || result.Version != 1 || result.Digest != saved.ReviewedContext.Digest || !reflect.DeepEqual(result.ImplementationContext, saved.ImplementationContext) || result.Document.Kind != kind || result.Document.SHA256 != reviewedHash(want) || result.Document.Bytes != len(want) || !bytes.Equal(result.Document.Value, compact.Bytes()) {
			t.Fatal("inspection substituted current context or changed captured scope", kind)
		}
		if strings.Contains(w.Body.String(), current.Purpose) || strings.Contains(w.Body.String(), env.conversations) || strings.Contains(w.Body.String(), sessionGatewayKey) || strings.Contains(w.Body.String(), "u-reviewed-task-fixture") || w.Header().Get("Cache-Control") != "no-store" {
			t.Fatal("inspection leaked private state, cached content or substituted B", kind)
		}
	}
	afterRecord, _ := env.st.Get(saved.ID)
	if !reflect.DeepEqual(beforeRecord, afterRecord) || !reflect.DeepEqual(beforeFiles, inspectFixtureFiles(t, env.conversations)) || beforeCreates != len(env.be.specs) || beforeDestroys != env.be.destroyCount() || beforeCurrent != env.do(t, "GET", base+"/context", "").Body.String() {
		t.Fatal("read-only inspection changed session, bundle, provider state, current context or sandbox lifecycle")
	}
}

func inspectFixtureFiles(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	if err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		data, err := os.ReadFile(path)
		if err == nil {
			files[path] = reviewedHash(data)
		}
		return err
	}); err != nil {
		t.Fatal(err)
	}
	return files
}

func TestReviewedContextInspectionRefusesSelectorsLegacyAndWholeBundleCorruption(t *testing.T) {
	env, saved, _ := createReviewedFixture(t, "codex")
	base := "/v1/sessions/" + saved.ID + "/reviewed-context"
	for _, query := range []string{"document=sketch", "document=../../secret", "document=brief&document=plan", "path=.luci/brief.json", "document=", "document=brief&extra=1"} {
		if w := env.do(t, "GET", base+"?"+query, ""); w.Code != 400 {
			t.Fatal("arbitrary or ambiguous selector accepted", query, w.Code)
		}
	}
	if w := env.do(t, "GET", base, ""); w.Code != 200 || !strings.Contains(w.Body.String(), `"kind":"definition"`) {
		t.Fatal("default inspection did not choose definition", w.Code)
	}
	for _, kind := range []string{"legacy", "manual", "foreign"} {
		t.Run(kind, func(t *testing.T) {
			if _, err := env.st.Update(saved.ID, func(row *registry.Session) error {
				row.ImplementationContext, row.ReviewedContext, row.ProjectID = saved.ImplementationContext.Clone(), saved.ReviewedContext.Clone(), saved.ProjectID
				switch kind {
				case "legacy":
					row.ReviewedContext = nil
				case "manual":
					row.ImplementationContext = nil
				case "foreign":
					row.ProjectID = "p-ffffffff"
				}
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			if w := env.do(t, "GET", base, ""); w.Code != 409 || strings.Contains(w.Body.String(), "Restore saved work") || strings.Contains(w.Body.String(), env.conversations) {
				t.Fatal("unverifiable or foreign context exposed captured document", kind, w.Code, w.Body.String())
			}
		})
	}
	if _, err := env.st.Update(saved.ID, func(row *registry.Session) error { *row = saved.Clone(); return nil }); err != nil {
		t.Fatal(err)
	}
	dir, _ := env.api.reviewedContextPath(saved.ID)
	// Even a different, undisplayed bundle member must verify. Do not tell a
	// person that the bundle is intact merely because the chosen file survived.
	if err := os.WriteFile(filepath.Join(dir, ".luci", "design.json"), []byte("corrupt supplemental drawing"), 0600); err != nil {
		t.Fatal(err)
	}
	if w := env.do(t, "GET", base+"?document=brief", ""); w.Code != 409 || strings.Contains(w.Body.String(), "Restore saved work") || strings.Contains(w.Body.String(), dir) {
		t.Fatal("inspection failed to verify the entire bundle", w.Code, w.Body.String())
	}
	if len(env.be.specs) != 1 || env.be.destroyCount() != 0 {
		t.Fatal("failed inspection changed sandbox lifecycle")
	}
}

func TestReviewedContextMaximumPlanShapeFitsExistingTeamResponseLimitWithoutTruncation(t *testing.T) {
	env, saved, _ := createReviewedFixture(t, "codex")
	capture, err := env.api.readReviewedContext(saved.ID, saved.ImplementationContext, saved.ReviewedContext)
	if err != nil {
		t.Fatal(err)
	}
	// This is a bounded serialization fixture, not a claim of model execution
	// or renewed approval. Fill every supported plan dimension and use maximum
	// escaping titles; keep the selected task narrow as a real bundle must do.
	ids := make([]string, 100)
	for i := range ids {
		ids[i] = fmt.Sprintf("R%03d", i) + strings.Repeat("x", 76)
	}
	plan := product.ProjectPlan{ProjectID: saved.ProjectID, Revision: saved.ImplementationContext.PlanRevision, BriefRevision: saved.ImplementationContext.BriefRevision, SpecificationRevision: saved.ImplementationContext.SpecificationRevision, SpecificationDigest: saved.ImplementationContext.SpecificationDigest, Tasks: []product.ProjectTask{}}
	for i := 0; i < 200; i++ {
		plan.Tasks = append(plan.Tasks, product.ProjectTask{ID: fmt.Sprintf("T%03d", i) + strings.Repeat("y", 76), Title: strings.Repeat("<", 2000), Status: "todo", RequirementIDs: ids})
	}
	compact, _ := json.Marshal(plan)
	pretty, _ := json.MarshalIndent(plan, "", "  ")
	capture.Binding.PlanDigest = reviewedHash(compact)
	capture.Binding.TaskIDs = []string{plan.Tasks[0].ID}
	capture.Binding.TaskTitles = []string{plan.Tasks[0].Title}
	capture.Binding.RequirementIDs = ids
	for i := range capture.Files {
		if capture.Files[i].Path == ".luci/plan.json" {
			capture.Files[i].Data = append(pretty, '\n')
		}
	}
	copyID := "s-fffffff1"
	descriptor, _, err := env.api.prepareReviewedContext(copyID, capture)
	if err != nil {
		t.Fatal(err)
	}
	copy := saved.Clone()
	copy.ID, copy.ImplementationContext, copy.ReviewedContext = copyID, capture.Binding, descriptor
	if err := env.st.Create(copy); err != nil {
		t.Fatal(err)
	}
	w := env.do(t, "GET", "/v1/sessions/"+copyID+"/reviewed-context?document=plan", "")
	var result reviewedContextInspection
	if w.Code != 200 || w.Body.Len() >= reviewedInspectionLimit || json.Unmarshal(w.Body.Bytes(), &result) != nil || !bytes.Equal(result.Document.Value, compact) {
		t.Fatal("maximum supported plan was truncated or exceeded transport headroom", w.Code, w.Body.Len())
	}
	var decoded product.ProjectPlan
	if json.Unmarshal(result.Document.Value, &decoded) != nil || !reflect.DeepEqual(decoded, plan) {
		t.Fatal("inspection changed the exact unfinished task text or requirement links")
	}
}

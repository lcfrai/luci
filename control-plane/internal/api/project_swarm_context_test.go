package api

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// The fixture uses authenticated review endpoints, actual Git copies and the
// session API. Its backend is a lifecycle double; no model or Docker execution
// is claimed by these assertions about the actual delivered context files.
func TestProjectSwarmCapturedTaskContractsSurviveCanonicalAmendment(t *testing.T) {
	f := newSwarmSessionFixture(t)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	stored := []registry.Session{}
	originals := []map[string][]byte{}
	roots := []string{}
	for i, req := range f.requests {
		body, _ := json.Marshal(req)
		created := sessionsDecodeOne(t, f.env.do(t, "POST", "/v1/sessions", string(body)), 201)
		sessionsAwaitWatch(t, done)
		saved, err := f.store.Get(created.ID)
		if err != nil || saved.ReviewedContext == nil || !reflect.DeepEqual(saved.ImplementationContext.TaskIDs, []string{f.ops[i].TaskID}) {
			t.Fatal("session lost its singleton reviewed task selection", saved, err)
		}
		capture, err := f.env.api.readReviewedContext(saved.ID, saved.ImplementationContext, saved.ReviewedContext)
		if err != nil || len(capture.Files) != 9 || saved.ReviewedContext.Version != 2 || saved.ImplementationContext.DependencyContextDigest == "" {
			t.Fatal("session did not retain the complete verified capture", err)
		}
		var inspection reviewedContextInspection
		response := f.env.do(t, "GET", "/v1/sessions/"+saved.ID+"/reviewed-context?document=plan", "")
		if response.Code != 200 || json.Unmarshal(response.Body.Bytes(), &inspection) != nil || inspection.Version != 2 || inspection.Digest != saved.ReviewedContext.Digest || inspection.ImplementationContext.DependencyContextDigest != saved.ImplementationContext.DependencyContextDigest {
			t.Fatal("V2 captured document is not inspectable through its actual HTTP contract", response.Code, response.Body.String())
		}
		root := ""
		for _, mount := range f.env.be.spec(t, i).Mounts {
			if mount.ContainerPath == reviewedContextContainerDir && mount.ReadOnly {
				root = mount.HostPath
			}
		}
		if root == "" || root == f.owner.Path || i > 0 && root == roots[0] {
			t.Fatal("task context was not delivered through an independent read-only mount")
		}
		files := map[string][]byte{}
		for _, file := range capture.Files {
			delivered, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(file.Path)))
			if err != nil || !bytes.Equal(delivered, file.Data) {
				t.Fatal("mount differs from its verified captured receipt", file.Path, err)
			}
			files[file.Path] = delivered
		}
		marker := bytes.Index(files["LUCI.md"], []byte("\n```json\n"))
		if marker < 0 {
			t.Fatal("actual delivered brief has no selected-task contract")
		}
		var brief struct {
			SelectedTasks      []product.ProjectTask              `json:"selectedTasks"`
			LinkedRequirements []product.SpecificationRequirement `json:"linkedRequirements"`
		}
		if err := json.NewDecoder(bytes.NewReader(files["LUCI.md"][marker+len("\n```json\n"):])).Decode(&brief); err != nil {
			t.Fatal(err)
		}
		if len(brief.SelectedTasks) != 1 || brief.SelectedTasks[0].ID != f.ops[i].TaskID || brief.SelectedTasks[0].Title != saved.ImplementationContext.TaskTitles[0] || len(brief.SelectedTasks[0].WritePaths) != 1 || brief.SelectedTasks[0].Acceptance == "" {
			t.Fatal("worker received another task's assignment or incomplete contract", brief)
		}
		if len(brief.LinkedRequirements) != 1 || brief.LinkedRequirements[0].ID != "AC-save" || brief.LinkedRequirements[0].Acceptance != "A saved note returns after restarting the app." {
			t.Fatal("worker did not receive the original complete linked acceptance", brief.LinkedRequirements)
		}
		fullPlan, err := os.ReadFile(filepath.Join(f.owner.Path, ".luci", "plan.json"))
		if err != nil || !bytes.Equal(fullPlan, files[".luci/plan.json"]) {
			t.Fatal("narrowing the task brief changed the complete captured plan", err)
		}
		var plan product.ProjectPlan
		if json.Unmarshal(files[".luci/plan.json"], &plan) != nil || len(plan.Tasks) != 3 || !reflect.DeepEqual(plan.Tasks[2].DependsOn, []string{"api", "ui"}) {
			t.Fatal("supporting plan or dependency context was truncated")
		}
		stored, originals, roots = append(stored, saved), append(originals, files), append(roots, root)
	}
	if bytes.Equal(originals[0]["LUCI.md"], originals[1]["LUCI.md"]) || stored[0].ReviewedContext.Digest == stored[1].ReviewedContext.Digest {
		t.Fatal("distinct task assignments share the same captured contract")
	}
	// A real collaborative edit produces B in the main project while the two
	// already-started workers keep their exact A files and private receipts.
	var plan product.ProjectPlan
	if err := json.Unmarshal(originals[0][".luci/plan.json"], &plan); err != nil {
		t.Fatal(err)
	}
	plan.Tasks[0].Title = "Teammate amended the API task"
	plan.Tasks[0].Acceptance = "Later API behavior B needs fresh review."
	body, _ := json.Marshal(map[string]any{"expectedRevision": plan.Revision, "expectedSpecificationRevision": plan.SpecificationRevision, "tasks": plan.Tasks})
	w := f.env.do(t, "POST", "/v1/product/projects/"+f.owner.ID+"/plan", string(body))
	if w.Code != 200 {
		t.Fatal("canonical amendment", w.Code, w.Body.String())
	}
	current, err := os.ReadFile(filepath.Join(f.owner.Path, ".luci", "plan.json"))
	if err != nil || !bytes.Contains(current, []byte(plan.Tasks[0].Acceptance)) {
		t.Fatal("teammate amendment was not actually saved", err)
	}
	for i, saved := range stored {
		for name, original := range originals[i] {
			after, err := os.ReadFile(filepath.Join(roots[i], filepath.FromSlash(name)))
			if err != nil || !bytes.Equal(after, original) {
				t.Fatal("canonical amendment changed an active worker's captured context", saved.ID, name, err)
			}
		}
		after, err := f.store.Get(saved.ID)
		if err != nil || !reflect.DeepEqual(saved, after) {
			t.Fatal("amendment changed or replaced an existing worker receipt", err)
		}
		w = f.env.do(t, "GET", "/v1/sessions/"+saved.ID+"/reviewed-context?document=plan", "")
		var inspected reviewedContextInspection
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &inspected) != nil {
			t.Fatal("historical full plan became unreadable after amendment", w.Code, w.Body.String())
		}
		var compact bytes.Buffer
		if err := json.Compact(&compact, originals[i][".luci/plan.json"]); err != nil || !bytes.Equal(inspected.Document.Value, compact.Bytes()) {
			t.Fatal("historical inspection substituted current plan B", err)
		}
		body, _ := json.Marshal(f.requests[i])
		if replay := f.env.do(t, "POST", "/v1/sessions", string(body)); replay.Code != 200 {
			t.Fatal("recovering an existing child was incorrectly treated as fresh dispatch", replay.Code, replay.Body.String())
		}
	}
	if len(f.env.be.specs) != 2 || f.env.be.destroyCount() != 0 {
		t.Fatal("amendment or receipt recovery allocated/replaced a worker")
	}
}

func TestProjectSwarmUnstartedReservationRefusesAmendedContext(t *testing.T) {
	f := newSwarmSessionFixture(t)
	var plan product.ProjectPlan
	w := f.env.do(t, "GET", "/v1/product/projects/"+f.owner.ID+"/plan", "")
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &plan) != nil {
		t.Fatal(w.Code, w.Body.String())
	}
	plan.Tasks[0].Acceptance = "Amended before dispatch: do not silently adopt B."
	body, _ := json.Marshal(map[string]any{"expectedRevision": plan.Revision, "expectedSpecificationRevision": plan.SpecificationRevision, "tasks": plan.Tasks})
	w = f.env.do(t, "POST", "/v1/product/projects/"+f.owner.ID+"/plan", string(body))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	body, _ = json.Marshal(f.requests[0])
	w = f.env.do(t, "POST", "/v1/sessions", string(body))
	rows, err := f.store.List()
	if w.Code != 409 || err != nil || len(rows) != 0 || len(f.env.be.specs) != 0 {
		t.Fatal("stale reservation substituted current context or allocated work", w.Code, w.Body.String(), err)
	}
}

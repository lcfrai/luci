package api

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func reviewedSessionPlan(t *testing.T, env *sessionsEnv, projectID string) product.ImplementationContextRequest {
	t.Helper()
	base := "/v1/product/projects/" + projectID
	request := func(method, route string, body any, out any) {
		t.Helper()
		data := ""
		if body != nil {
			b, err := json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
			data = string(b)
		}
		var w *httptest.ResponseRecorder
		if strings.HasSuffix(route, "/approve") {
			// Exercise the actual trusted worker identity boundary; request JSON
			// cannot invent the actor that owns a private approval receipt.
			const token = "reviewed-task-fixture-worker-token-at-least-forty-characters"
			actor, _ := json.Marshal(management.User{ID: "u-reviewed-task-fixture", Username: "reviewer", Role: "super-admin"})
			policy, _ := json.Marshal(management.Policy{MaxSessionsPerUser: 10, MaxConcurrentChecksPerUser: 2, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, AllowedHarnesses: []string{"codex", "shell"}, BackingMode: "docker", InstanceName: "Reviewed task fixture"})
			r := httptest.NewRequest(method, sessionsTarget(base+route), strings.NewReader(data))
			r.Header.Set("Content-Type", "application/json")
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString(actor))
			r.Header.Set("X-Luci-Policy", base64.RawURLEncoding.EncodeToString(policy))
			w = httptest.NewRecorder()
			management.WorkerHandler(token, env.mux).ServeHTTP(w, r)
		} else {
			w = env.do(t, method, base+route, data)
		}
		if w.Code != 200 {
			t.Fatal(route, w.Code, w.Body.String())
		}
		if out != nil && json.Unmarshal(w.Body.Bytes(), out) != nil {
			t.Fatal("invalid reviewed response")
		}
	}
	var spec product.ProjectSpecification
	request("GET", "/specification", nil, &spec)
	spec.Purpose = "Restore saved work"
	spec.Audience = "Project owners"
	spec.Requirements = []product.SpecificationRequirement{{ID: "AC-save", Title: "Restore a note", Priority: "must", Acceptance: "A saved note returns after restarting the app."}}
	request("POST", "/specification", map[string]any{"expectedRevision": spec.Revision, "expectedBriefRevision": spec.BriefRevision, "specification": spec}, &spec)
	request("POST", "/specification/approve", map[string]int{"revision": spec.Revision, "briefRevision": spec.BriefRevision}, &spec)
	var design product.ProjectDesignPlan
	request("GET", "/design-plan", nil, &design)
	design.Screens = []product.DesignScreen{{ID: "home", Name: "Saved notes", Route: "/", Purpose: "Save and restore a note", States: []string{"Empty", "Saved", "Error and retry"}, Widths: []int{390, 1440}, RequirementIDs: []string{"AC-save"}}}
	request("POST", "/design-plan", map[string]any{"expectedRevision": design.Revision, "expectedBriefRevision": spec.BriefRevision, "expectedSpecificationRevision": spec.Revision, "plan": design}, &design)
	request("POST", "/design-plan/approve", map[string]int{"revision": design.Revision}, &design)
	var plan product.ProjectPlan
	request("POST", "/plan", map[string]any{"expectedRevision": 0, "expectedSpecificationRevision": spec.Revision, "tasks": []product.ProjectTask{{ID: "save-note", Title: "Implement durable notes", Status: "todo", RequirementIDs: []string{"AC-save"}}, {ID: "polish", Title: "Polish transitions", Status: "todo"}}}, &plan)
	return product.ImplementationContextRequest{ProjectID: projectID, BriefRevision: spec.BriefRevision, PlanRevision: plan.Revision, SpecificationRevision: spec.Revision, DesignPlanRevision: design.Revision, TaskIDs: []string{"save-note"}}
}

func TestReviewedTaskSessionBindsRealSavedInputsAndRefusesStaleRestart(t *testing.T) {
	env, p, wt, _ := sessionsPrimaryBranchFixture(t)
	selection := reviewedSessionPlan(t, env, p.ID)
	c := sessionSaveConnection(t, env, 0, "", "codex", true)
	sessionsStubReady(t, nil)
	done := sessionsWatchDone(t)
	body, _ := json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": "codex", "agentConnectionId": c.ID, "implementationContext": selection, "openingPrompt": "Keep the interaction concise."})
	created := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions", string(body)), 201)
	sessionsAwaitWatch(t, done)
	saved, err := env.st.Get(created.ID)
	if err != nil {
		t.Fatal(err)
	}
	bound := created.ImplementationContext
	if bound == nil || bound.ProjectID != p.ID || bound.WorktreeID != wt.ID || bound.PlanRevision != selection.PlanRevision || bound.SpecificationRevision != selection.SpecificationRevision || bound.DesignPlanRevision != selection.DesignPlanRevision || len(bound.SourceDigest) != 64 || len(bound.SourceCommit) != 40 || !reflect.DeepEqual(bound.RequirementIDs, []string{"AC-save"}) || !reflect.DeepEqual(bound.TaskTitles, []string{"Implement durable notes"}) || !reflect.DeepEqual(bound, saved.ImplementationContext) {
		t.Fatal("reviewed inputs lost", bound)
	}
	if !strings.Contains(saved.OpeningPrompt, ".luci/design-handoff.json") || !strings.Contains(saved.OpeningPrompt, ".luci/design-handoff.md") || !strings.Contains(saved.OpeningPrompt, bound.DesignPlanDigest) || !strings.Contains(saved.OpeningPrompt, "Keep the interaction concise.") {
		t.Fatal("agent did not receive reviewed handoff and extra direction")
	}
	copy := toJSON(saved, "/v1")
	copy.ImplementationContext.TaskTitles[0] = "mutated client snapshot"
	if saved.ImplementationContext.TaskTitles[0] != "Implement durable notes" {
		t.Fatal("public snapshot aliases stored task context")
	}
	if err := os.WriteFile(filepath.Join(p.Path, "app.cjs"), []byte("// actual working source may change\n"), 0600); err != nil {
		t.Fatal(err)
	}
	done = sessionsWatchDone(t)
	restarted := sessionsDecodeOne(t, env.do(t, "POST", "/v1/sessions/"+created.ID+"/restart", "{}"), 200)
	sessionsAwaitWatch(t, done)
	if !reflect.DeepEqual(restarted.ImplementationContext, bound) {
		t.Fatal("restart rewrote immutable starting source")
	}
	beforeDestroy := env.be.destroyCount()
	beforeCreate := len(env.be.specs)
	changed, _ := json.Marshal(map[string]any{"expectedRevision": selection.PlanRevision, "expectedSpecificationRevision": selection.SpecificationRevision, "tasks": []product.ProjectTask{{ID: "save-note", Title: "Changed reviewed task", Status: "todo", RequirementIDs: []string{"AC-save"}}}})
	w := env.do(t, "POST", "/v1/product/projects/"+p.ID+"/plan", string(changed))
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = env.do(t, "POST", "/v1/sessions/"+created.ID+"/restart", "{}")
	if w.Code != 409 || env.be.destroyCount() != beforeDestroy || len(env.be.specs) != beforeCreate {
		t.Fatal("stale task restart changed sandbox", w.Code, w.Body.String())
	}
}

func TestReviewedTaskAdmissionRejectsForeignStaleUnknownAndShellBeforeAllocation(t *testing.T) {
	env, p, wt, _ := sessionsPrimaryBranchFixture(t)
	selection := reviewedSessionPlan(t, env, p.ID)
	variants := map[string]product.ImplementationContextRequest{"foreign": selection, "stale-plan": selection, "stale-design": selection, "unknown-task": selection, "duplicate-task": selection}
	v := variants["foreign"]
	v.ProjectID = "p-deadbeef"
	variants["foreign"] = v
	v = variants["stale-plan"]
	v.PlanRevision++
	variants["stale-plan"] = v
	v = variants["stale-design"]
	v.DesignPlanRevision++
	variants["stale-design"] = v
	v = variants["unknown-task"]
	v.TaskIDs = []string{"unknown"}
	variants["unknown-task"] = v
	v = variants["duplicate-task"]
	v.TaskIDs = []string{"save-note", "save-note"}
	variants["duplicate-task"] = v
	for name, invalid := range variants {
		t.Run(name, func(t *testing.T) {
			body, _ := json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": "codex", "implementationContext": invalid})
			w := env.do(t, "POST", "/v1/sessions", string(body))
			if w.Code != 409 || len(env.be.specs) != 0 {
				t.Fatal("invalid task allocated a sandbox", w.Code, w.Body.String())
			}
		})
	}
	body, _ := json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": "shell", "implementationContext": selection})
	if w := env.do(t, "POST", "/v1/sessions", string(body)); w.Code != 409 || len(env.be.specs) != 0 {
		t.Fatal("shell accepted a coding task", w.Code)
	}
	forged, _ := json.Marshal(map[string]any{"worktreeId": wt.ID, "harness": "codex", "implementationContext": map[string]any{"projectId": p.ID, "planDigest": "client-supplied"}})
	if w := env.do(t, "POST", "/v1/sessions", string(forged)); w.Code != 400 || len(env.be.specs) != 0 {
		t.Fatal("client-controlled digest accepted", w.Code)
	}
	if _, err := env.api.deps.Product.BindImplementationContext(context.Background(), selection, "w-linked", t.TempDir()); err == nil {
		t.Fatal("linked checkout silently inherited main context")
	}
	rows, err := env.st.List()
	if err != nil || len(rows) != 0 {
		t.Fatal("refused admission persisted session records", err)
	}
}

func TestReviewedTaskResumeCannotRebindOrMoveContext(t *testing.T) {
	env, p, wt, _ := sessionsPrimaryBranchFixture(t)
	selection := reviewedSessionPlan(t, env, p.ID)
	bound, err := env.api.deps.Product.BindImplementationContext(context.Background(), selection, wt.ID, p.Path)
	if err != nil {
		t.Fatal(err)
	}
	source := registry.Session{ProjectID: p.ID, WorktreeID: wt.ID, ImplementationContext: bound}
	for _, variant := range []string{"move", "rebind"} {
		t.Run(variant, func(t *testing.T) {
			req := sessionCreateRequest{WorktreeID: wt.ID, Harness: "codex"}
			if variant == "move" {
				req.WorktreeID = "w-other"
			} else {
				changed := selection
				changed.TaskIDs = []string{"polish"}
				req.ImplementationContext = &changed
			}
			w := httptest.NewRecorder()
			r := httptest.NewRequest(http.MethodPost, "/v1/sessions", nil)
			if _, ok := env.api.bindSessionImplementation(w, r, &req, p.Path, source); ok || w.Code != 409 || len(env.be.specs) != 0 {
				t.Fatal("resume reinterpreted task context", w.Code)
			}
		})
	}
}

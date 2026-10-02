package api_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/api"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// catalogNewAPIWithSessions builds the handler over a real FileStore seeded with the
// given sessions, which is how sessionCount gets something to count.
func catalogNewAPIWithSessions(t *testing.T, sessions ...registry.Session) http.Handler {
	t.Helper()
	d, st := catalogNewDeps(t)
	for _, s := range sessions {
		if err := st.Create(s); err != nil {
			t.Fatalf("seed %s: %v", s.ID, err)
		}
	}
	a, err := api.New(d)
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}
	return a
}

// catalogDecodeObject reads a JSON object out of a response body. Used where an
// assertion should name one field rather than diff two hundred characters.
func catalogDecodeObject(t *testing.T, body string) map[string]any {
	t.Helper()
	var out map[string]any
	if err := json.Unmarshal([]byte(body), &out); err != nil {
		t.Fatalf("body %q is not a JSON object: %v", body, err)
	}
	return out
}

// catalogListOf pulls one array field out of such an object.
func catalogListOf(t *testing.T, body, field string) []map[string]any {
	t.Helper()
	raw, ok := catalogDecodeObject(t, body)[field]
	if !ok {
		t.Fatalf("body %s has no %q field", body, field)
	}
	items, ok := raw.([]any)
	if !ok {
		t.Fatalf("%q is %T, want an array — a nil slice marshals as null and the rail cannot map over it", field, raw)
	}
	out := make([]map[string]any, 0, len(items))
	for i, it := range items {
		m, ok := it.(map[string]any)
		if !ok {
			t.Fatalf("%s[%d] is %T, want an object", field, i, it)
		}
		out = append(out, m)
	}
	return out
}

func TestHealthzBodyIsExactlyTheContractShape(t *testing.T) {
	// Contract 3.3, byte for byte. A supervisor greps this.
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/v1/healthz", "", nil)

	want := `{"status":"ok","version":"0.2.0","backend":"docker","backendReady":true}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n\t%q\nwant\n\t%q", got, want)
	}
}

func TestHealthzIs200EvenWhenTheBackendIsNotReady(t *testing.T) {
	// Contract 3.3 is explicit: the control plane is up, the backend is not,
	// and restarting the control plane would not fix Docker Desktop being shut.
	h := catalogNewAPI(t, func(d *api.Deps) { d.BackendReady = func() bool { return false } })
	rec := catalogDo(t, h, http.MethodGet, "/v1/healthz", "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	want := `{"status":"ok","version":"0.2.0","backend":"docker","backendReady":false}` + "\n"
	if got := rec.Body.String(); got != want {
		t.Errorf("body =\n\t%q\nwant\n\t%q", got, want)
	}
}

func TestMachinesDescribesTheLocalDockerMachine(t *testing.T) {
	// Contract 3.4. "container" rather than "microvm" is the honest value for
	// what Phase 2 builds, and limitations says what it means in a sentence.
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/v1/machines", "", nil)

	machines := catalogListOf(t, rec.Body.String(), "machines")
	if len(machines) != 1 {
		t.Fatalf("got %d machines, want exactly 1 — cloud machines are Phase 8", len(machines))
	}
	m := machines[0]
	for field, want := range map[string]any{
		"id":        api.LocalMachineID,
		"name":      "This PC",
		"kind":      "local",
		"state":     "running",
		"backend":   "docker",
		"isolation": "container",
		"default":   true,
	} {
		if got := m[field]; got != want {
			t.Errorf("machines[0].%s = %v, want %v", field, got, want)
		}
	}
	if _, present := m["message"]; present {
		t.Error("a running machine carries a message; message explains an error state only")
	}
	lim, _ := m["limitations"].([]any)
	if len(lim) == 0 {
		t.Error("limitations is empty — the local machine is badged unsandboxed and must say why")
	}
}

func TestMachinesReportsErrorAndAMessageWhenTheBackendIsDown(t *testing.T) {
	// Contract 3.4: Phase 2 emits running or error only, and error carries a
	// message naming the cause. The other four states need Phase 8 (contract 8.7).
	h := catalogNewAPI(t, func(d *api.Deps) { d.BackendReady = func() bool { return false } })
	rec := catalogDo(t, h, http.MethodGet, "/v1/machines", "", nil)

	m := catalogListOf(t, rec.Body.String(), "machines")[0]
	if got := m["state"]; got != "error" {
		t.Fatalf("state = %v, want \"error\"", got)
	}
	msg, _ := m["message"].(string)
	if !strings.Contains(msg, "docker") {
		t.Errorf("message = %q, want it to name the backend that is not responding", msg)
	}
}

func TestMachinesIsolationFollowsTheBackendKind(t *testing.T) {
	// katabe is dormant until Phase 8, but the mapping must not be a literal.
	//
	// The third row is the one ruling 6 makes binding: isolationFor is the
	// package's ONLY kind-to-enum mapping, task 11 calls it rather than
	// declaring a second, and the value an unrecognised kind gets is "none" —
	// a member of contract §3.4's three-value enum, not a fourth value like
	// "unknown" invented at one call site.
	for kind, wantIsolation := range map[backend.Kind]string{
		backend.KindDocker: "container",
		backend.KindKata:   "microvm",
		"podman":           "none",
	} {
		t.Run(string(kind), func(t *testing.T) {
			h := catalogNewAPI(t, func(d *api.Deps) { d.Backend = catalogBackend{kind: kind} })
			rec := catalogDo(t, h, http.MethodGet, "/v1/machines", "", nil)

			m := catalogListOf(t, rec.Body.String(), "machines")[0]
			if got := m["isolation"]; got != wantIsolation {
				t.Errorf("isolation = %v for the %s backend, want %q", got, kind, wantIsolation)
			}
			if got := m["backend"]; got != string(kind) {
				t.Errorf("backend = %v, want %q", got, kind)
			}
			// Every kind says what it costs the user, including the one this
			// build does not know: "unsandboxed" with no sentence under it is
			// a badge nobody can act on.
			if lim, _ := m["limitations"].([]any); len(lim) == 0 {
				t.Errorf("limitations is empty for the %s backend", kind)
			}
		})
	}
}

func TestProjectsCarriesIdNameAndAbsolutePath(t *testing.T) {
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/v1/projects", "", nil)

	ps := catalogListOf(t, rec.Body.String(), "projects")
	if len(ps) != 1 {
		t.Fatalf("got %d projects, want 1", len(ps))
	}
	if ps[0]["id"] != catalogTestProject || ps[0]["name"] != "engagement-portal" {
		t.Errorf("projects[0] = %v", ps[0])
	}
	if _, ok := ps[0]["path"].(string); !ok {
		t.Error("projects[0].path is missing; the desktop shows it under the name")
	}
}

func TestProjectsWithNoProjectsIsAnEmptyArrayNotNull(t *testing.T) {
	// Contract 3.5: an unconfigured LUCI_PROJECTS_DIR is 200 and an
	// empty list. null here is a blank rail and a console error.
	h := catalogNewAPI(t, func(d *api.Deps) { d.Projects = &catalogProjects{} })
	rec := catalogDo(t, h, http.MethodGet, "/v1/projects", "", nil)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 — empty is a state the cockpit renders", rec.Code)
	}
	if got, want := rec.Body.String(), `{"projects":[]}`+"\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestAnEnumerationFailureIs500ThatLeaksNothing(t *testing.T) {
	// a.internal is the only 500 on this surface and every later task uses it,
	// so it is worth one test rather than none. Contract §3.2 makes the message
	// deliberately generic: the body is read by the cockpit, the cause goes to
	// the log, and a path on disk belongs in exactly one of those.
	h := catalogNewAPI(t, func(d *api.Deps) {
		d.Projects = &catalogProjects{err: errors.New(`open C:\work\engagement-portal\.git: permission denied`)}
	})
	rec := catalogDo(t, h, http.MethodGet, "/v1/projects", "", nil)

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	if got := catalogDecodeObject(t, rec.Body.String())["error"]; got != "internal" {
		t.Errorf("error = %v, want \"internal\"", got)
	}
	if strings.Contains(rec.Body.String(), `C:\work`) {
		t.Errorf("body = %s, want no path from the underlying error in it", rec.Body.String())
	}
}

func TestBranchesCarryIsHeadAndTheProjectId(t *testing.T) {
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/v1/projects/"+catalogTestProject+"/branches", "", nil)

	bs := catalogListOf(t, rec.Body.String(), "branches")
	if len(bs) != 2 {
		t.Fatalf("got %d branches, want 2", len(bs))
	}
	heads := 0
	for _, b := range bs {
		if b["projectId"] != catalogTestProject {
			t.Errorf("branch %v carries the wrong projectId", b["id"])
		}
		if b["isHead"] == true {
			heads++
		}
	}
	if heads != 1 {
		t.Errorf("got %d branches with isHead, want exactly 1", heads)
	}
}

func TestAnUnknownProjectIs404ProjectNotFound(t *testing.T) {
	// project.ErrUnknownProject is task 8's sentinel and is matched with
	// errors.Is. A handler that string-matched would pass this test and fail
	// the first time the message changed, so the fake wraps it with %w.
	h := catalogNewAPI(t, nil)
	for _, target := range []string{"/branches", "/worktrees"} {
		t.Run(target, func(t *testing.T) {
			rec := catalogDo(t, h, http.MethodGet, "/v1/projects/p-deadbeef"+target, "", nil)
			if rec.Code != http.StatusNotFound {
				t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
			}
			if !strings.Contains(rec.Body.String(), "project_not_found") {
				t.Errorf("body = %s, want the project_not_found code", rec.Body.String())
			}
		})
	}
}

func TestWorktreesCarrySessionCountAndReady(t *testing.T) {
	// ready ships false on every worktree until something PATCHes it. That is
	// known defect 9 fixed rather than deleted: the field exists, so the verb can.
	h := catalogNewAPIWithSessions(t,
		registry.Session{ID: "s-00000001", WorktreeID: catalogTestWorktree, Status: registry.StatusRunning},
		registry.Session{ID: "s-00000002", WorktreeID: catalogTestWorktree, Status: registry.StatusIdle},
		registry.Session{ID: "s-00000003", WorktreeID: catalogOtherWorktree, Status: registry.StatusStarting},
	)
	rec := catalogDo(t, h, http.MethodGet, "/v1/projects/"+catalogTestProject+"/worktrees", "", nil)

	byID := map[string]map[string]any{}
	for _, w := range catalogListOf(t, rec.Body.String(), "worktrees") {
		byID[w["id"].(string)] = w
	}
	if got := byID[catalogTestWorktree]["sessionCount"]; got != float64(2) {
		t.Errorf("sessionCount = %v, want 2", got)
	}
	if got := byID[catalogOtherWorktree]["sessionCount"]; got != float64(1) {
		t.Errorf("sessionCount = %v, want 1", got)
	}
	for id, w := range byID {
		if w["ready"] != false {
			t.Errorf("%s.ready = %v, want false before anything sets it", id, w["ready"])
		}
		if _, ok := w["isPrimary"]; !ok {
			t.Errorf("%s has no isPrimary field", id)
		}
	}
}

func TestSessionCountIgnoresExitedSessions(t *testing.T) {
	// Contract 3.7: a count of live things the user owns. Counting exited
	// sessions turns it into a history, which docs/AGENTIC-UX.md deletes.
	h := catalogNewAPIWithSessions(t,
		registry.Session{ID: "s-00000001", WorktreeID: catalogTestWorktree, Status: registry.StatusExited},
		registry.Session{ID: "s-00000002", WorktreeID: catalogTestWorktree, Status: registry.StatusRunning},
	)
	rec := catalogDo(t, h, http.MethodGet, "/v1/projects/"+catalogTestProject+"/worktrees", "", nil)

	for _, w := range catalogListOf(t, rec.Body.String(), "worktrees") {
		if w["id"] == catalogTestWorktree && w["sessionCount"] != float64(1) {
			t.Fatalf("sessionCount = %v, want 1 — the exited session was counted", w["sessionCount"])
		}
	}
}

func TestAStoreListFailureIs500AndNeverAZeroCount(t *testing.T) {
	// Decision (f). The tempting alternative is sessionCount 0 and a 200, and
	// it is a fabricated number: the rail would show a worktree with no live
	// sessions on it when nobody knows how many there are.
	d, st := catalogNewDeps(t)
	d.Store = catalogBrokenList{FileStore: st, err: errors.New("registry: the document is unreadable")}
	a, err := api.New(d)
	if err != nil {
		t.Fatalf("api.New: %v", err)
	}

	rec := catalogDo(t, a, http.MethodGet, "/v1/projects/"+catalogTestProject+"/worktrees", "", nil)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500; body = %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "sessionCount") {
		t.Errorf("body = %s, want no counts in it at all", rec.Body.String())
	}
}

func TestWorktreesBranchIdFilterIncludingAMissWhichIsAnEmptyArray(t *testing.T) {
	h := catalogNewAPI(t, nil)

	hit := catalogDo(t, h, http.MethodGet, "/v1/projects/"+catalogTestProject+"/worktrees?branchId="+catalogTestBranch, "", nil)
	ws := catalogListOf(t, hit.Body.String(), "worktrees")
	if len(ws) != 1 || ws[0]["id"] != catalogTestWorktree {
		t.Fatalf("filtered worktrees = %v, want just %s", ws, catalogTestWorktree)
	}

	miss := catalogDo(t, h, http.MethodGet, "/v1/projects/"+catalogTestProject+"/worktrees?branchId=b-nope", "", nil)
	if got, want := miss.Body.String(), `{"worktrees":[]}`+"\n"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
}

func TestPatchWorktreeSetsReadyAndTheListThenReportsIt(t *testing.T) {
	h := catalogNewAPI(t, nil)
	target := "/v1/projects/" + catalogTestProject + "/worktrees/" + catalogTestWorktree

	rec := catalogDo(t, h, http.MethodPatch, target, `{"ready":true}`, nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200; body = %s", rec.Code, rec.Body.String())
	}
	// The response is the worktree object itself, not an envelope.
	got := catalogDecodeObject(t, rec.Body.String())
	if got["id"] != catalogTestWorktree || got["ready"] != true {
		t.Fatalf("PATCH returned %v, want the worktree with ready true", got)
	}

	list := catalogDo(t, h, http.MethodGet, "/v1/projects/"+catalogTestProject+"/worktrees", "", nil)
	for _, w := range catalogListOf(t, list.Body.String(), "worktrees") {
		want := w["id"] == catalogTestWorktree
		if w["ready"] != want {
			t.Errorf("after the PATCH, %s.ready = %v, want %v", w["id"], w["ready"], want)
		}
	}

	// And it clears again, because "mark ready" that cannot be undone is a
	// verb the user will not press twice.
	off := catalogDo(t, h, http.MethodPatch, target, `{"ready":false}`, nil)
	if off.Code != http.StatusOK || catalogDecodeObject(t, off.Body.String())["ready"] != false {
		t.Fatalf("clearing ready = %d %s", off.Code, off.Body.String())
	}
}

func TestPatchWorktreeRejectsABodyThatSetsSomethingElse(t *testing.T) {
	// Every one of these is 400 bad_request and not 400 invalid_json. The
	// controller's ruling: an unknown field is WELL-FORMED JSON that is
	// semantically wrong, and a code the cockpit switches on must not say
	// something untrue — the same principle that refuses sandbox_not_running for
	// a sandbox that is running. Contract §3.7 names bad_request for this case.
	target := "/v1/projects/" + catalogTestProject + "/worktrees/" + catalogTestWorktree
	for name, body := range map[string]string{
		"unknown field beside ready": `{"ready":true,"name":"renamed"}`,
		"only a name":                `{"name":"renamed"}`,
		"empty object":               `{}`,
		"null ready":                 `{"ready":null}`,
		"ready is not a boolean":     `{"ready":"yes"}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodPatch, target, body, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
			if got := catalogDecodeObject(t, rec.Body.String())["error"]; got != "bad_request" {
				t.Errorf("error = %v, want \"bad_request\"; body = %s", got, rec.Body.String())
			}
		})
	}
}

func TestPatchWorktreeStillAnswersInvalidJsonForAMalformedBody(t *testing.T) {
	// The other half of the ruling above, and the reason it is a ruling about
	// LABELS rather than about which decoder runs: httpx.DecodeJSON still owns
	// every body rule in the phase, so a body that is not JSON is still
	// invalid_json. Two codes, two distinct meanings, one decoder.
	target := "/v1/projects/" + catalogTestProject + "/worktrees/" + catalogTestWorktree
	for name, body := range map[string]string{
		"truncated object": `{"ready":true`,
		"not json at all":  `ready please`,
		"two objects":      `{"ready":true}{"ready":false}`,
	} {
		t.Run(name, func(t *testing.T) {
			rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodPatch, target, body, nil)
			if rec.Code != http.StatusBadRequest {
				t.Fatalf("status = %d, want 400; body = %s", rec.Code, rec.Body.String())
			}
			if got := catalogDecodeObject(t, rec.Body.String())["error"]; got != "invalid_json" {
				t.Errorf("error = %v, want \"invalid_json\"; body = %s", got, rec.Body.String())
			}
		})
	}
}

func TestPatchAnUnknownWorktreeIs404WorktreeNotFound(t *testing.T) {
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodPatch,
		"/v1/projects/"+catalogTestProject+"/worktrees/w-deadbeef", `{"ready":true}`, nil)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404; body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "worktree_not_found") {
		t.Errorf("body = %s, want the worktree_not_found code", rec.Body.String())
	}
}

func TestPatchIsScopedToTheProjectInThePath(t *testing.T) {
	// Without the membership check any project id in the path would do, and
	// the path segment would be decoration rather than authorisation.
	const secondProject = "p-11112222"
	h := catalogNewAPI(t, func(d *api.Deps) {
		p := catalogSampleProjects()
		p.projects = append(p.projects, project.Project{ID: secondProject, Name: "other", Path: `C:\work\other`})
		p.worktrees[secondProject] = []project.Worktree{
			{ID: "w-99998888", ProjectID: secondProject, BranchID: "b-11112222-main",
				Name: "other", Path: `C:\work\other`, IsPrimary: true},
		}
		d.Projects = p
	})

	rec := catalogDo(t, h, http.MethodPatch,
		"/v1/projects/"+secondProject+"/worktrees/"+catalogTestWorktree, `{"ready":true}`, nil)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404 — %s does not live under %s", rec.Code, catalogTestWorktree, secondProject)
	}
}

func TestHarnessesComputeAvailableServerSideAndNeverEmitNull(t *testing.T) {
	// Contract 3.8: available is installed && flagsVerified, computed here so
	// the desktop never re-derives it, and the empty slices are [] because
	// null.map is a blank picker.
	rec := catalogDo(t, catalogNewAPI(t, nil), http.MethodGet, "/v1/harnesses", "", nil)

	hs := catalogListOf(t, rec.Body.String(), "harnesses")
	if len(hs) != len(harness.Catalogue()) {
		t.Fatalf("got %d harnesses, want %d", len(hs), len(harness.Catalogue()))
	}
	seen := map[string]bool{}
	for _, h := range hs {
		id, _ := h["id"].(string)
		seen[id] = true
		want := h["installed"] == true && h["flagsVerified"] == true
		if h["available"] != want {
			t.Errorf("%s.available = %v, want installed && flagsVerified = %v", id, h["available"], want)
		}
		if h["available"] == false && h["note"] == "" {
			t.Errorf("%s is unavailable with no note; the picker renders the note as the reason", id)
		}
		for _, field := range []string{"models", "efforts", "permissionModes"} {
			if _, ok := h[field].([]any); !ok {
				t.Errorf("%s.%s is %T, want an array even when empty", id, field, h[field])
			}
		}
	}
	for _, id := range []harness.ID{harness.ClaudeCode, harness.Shell, harness.Codex,
		harness.OpenCode, harness.GeminiCLI} {
		if !seen[string(id)] {
			t.Errorf("the surface dropped %q", id)
		}
	}
	if !seen[string(harness.Shell)] {
		t.Error("the shell is a harness; a picker that models 'no agent' as an absence cannot render it")
	}
}

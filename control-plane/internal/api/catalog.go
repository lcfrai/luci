// The read-only surface: everything that needs no sandbox, no session record
// and no WebSocket. Contract §3.3 to §3.8, plus the worktree PATCH from the
// plan's adjudicated Cross-group decision 2.
package api

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// catalogRoutes is the read-only surface's route table.
//
// A table rather than a sequence of literal calls, so the patterns of one
// surface are one value a reader can take in at once. That is a style choice
// and tasks 11 and 12 need not copy it: what contract §10a's audit actually
// needs is the *routes recorder in api.go, which sees a literal
// mux.HandleFunc(…) exactly as well.
func (a *API) catalogRoutes() []route {
	return []route{
		// /healthz is NOT here: api.New mounts it on the outer mux, outside the
		// origin guard. The handler still lives in this file.
		{"GET /v1/machines", http.HandlerFunc(a.handleMachines)},
		{"GET /v1/projects", http.HandlerFunc(a.handleProjects)},
		{"GET /v1/projects/{projectId}/branches", http.HandlerFunc(a.handleBranches)},
		{"GET /v1/projects/{projectId}/worktrees", http.HandlerFunc(a.handleWorktrees)},
		{"PATCH /v1/projects/{projectId}/worktrees/{worktreeId}", http.HandlerFunc(a.handlePatchWorktree)},
		{"GET /v1/harnesses", http.HandlerFunc(a.handleHarnesses)},
	}
}

// registerCatalog mounts the read-only routes. api.New calls it; tasks 11 and
// 12 add one sibling each.
func (a *API) registerCatalog(mux *routes) {
	for _, rt := range a.catalogRoutes() {
		mux.Handle(rt.Pattern, rt.Handler)
	}
}

// nonNil and stringsOf below are surface-agnostic despite living in the
// catalog's file: tasks 11 and 12 call them for their own JSON rather than
// declaring a second copy, on the same reasoning ruling 6 applies to
// isolationFor. They move to a file of their own when a third surface lands.

// nonNil renders a nil slice as an empty one. json.Marshal writes nil as null,
// and null.map(...) in the cockpit's rail is a blank screen with a console
// error instead of the empty state docs/FUNCTIONAL-SPEC.md specifies.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

// stringsOf flattens a slice of a string-kinded type, never to nil. Same
// reason: harness.Effort marshals as a string, but a nil []harness.Effort
// still marshals as null.
func stringsOf[T ~string](in []T) []string {
	out := make([]string, 0, len(in))
	for _, v := range in {
		out = append(out, string(v))
	}
	return out
}

type healthJSON struct {
	Status       string `json:"status"`
	Version      string `json:"version"`
	Backend      string `json:"backend"`
	BackendReady bool   `json:"backendReady"`
}

type machineJSON struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Kind string `json:"kind"`
	// State is contract §3.4's six-value enum. Phase 2 emits "running" or
	// "error" only; the other four need cloud machines, i.e. Phase 8 (§8.7).
	// The JSON field carries the full enum so the cockpit needs no change then.
	State string `json:"state"`
	// Message names the cause of an error state and is absent otherwise.
	Message          string   `json:"message,omitempty"`
	Backend          string   `json:"backend"`
	Isolation        string   `json:"isolation"`
	Default          bool     `json:"default"`
	Limitations      []string `json:"limitations"`
	SessionMemoryMiB int      `json:"sessionMemoryMiB"`
	SessionCPUMillis int      `json:"sessionCPUMillis"`
}

// worktreeJSON is project.Worktree plus the two fields git cannot supply:
// sessionCount from the registry, and ready from the flag store.
type worktreeJSON struct {
	ID           string `json:"id"`
	ProjectID    string `json:"projectId"`
	BranchID     string `json:"branchId"`
	Name         string `json:"name"`
	Path         string `json:"path"`
	IsPrimary    bool   `json:"isPrimary"`
	SessionCount int    `json:"sessionCount"`
	Ready        bool   `json:"ready"`
}

// harnessJSON is harness.Descriptor plus available, computed server-side so
// the desktop never re-derives installed && flagsVerified and gets it wrong.
type harnessJSON struct {
	ID                    string          `json:"id"`
	Name                  string          `json:"name"`
	Command               string          `json:"command"`
	Installed             bool            `json:"installed"`
	FlagsVerified         bool            `json:"flagsVerified"`
	Available             bool            `json:"available"`
	Models                []harness.Model `json:"models"`
	DefaultModel          string          `json:"defaultModel"`
	Efforts               []string        `json:"efforts"`
	PermissionModes       []string        `json:"permissionModes"`
	SupportsOpeningPrompt bool            `json:"supportsOpeningPrompt"`
	SupportsResume        bool            `json:"supportsResume"`
	Note                  string          `json:"note"`
}

type machinesEnvelope struct {
	Machines []machineJSON `json:"machines"`
}

type projectsEnvelope struct {
	Projects []project.Project `json:"projects"`
}

type branchesEnvelope struct {
	Branches []project.Branch `json:"branches"`
}

type worktreesEnvelope struct {
	Worktrees []worktreeJSON `json:"worktrees"`
}

type harnessesEnvelope struct {
	Harnesses []harnessJSON `json:"harnesses"`
}

// handleHealthz answers GET /v1/healthz. api.New mounts it on the outer mux,
// outside the origin guard; the reasoning is in the comment there.
//
// status is "ok" whenever this process is answering, including when the
// backend is not: contract §3.3 is explicit that a supervisor restarting the
// control plane would not fix Docker Desktop being closed, so the two facts
// are two fields.
//
// It reads a cached bool and starts nothing. That is contract §10a on the one
// route with no origin guard in front of it (Group C ruling 2).
func (a *API) handleHealthz(w http.ResponseWriter, r *http.Request) {
	a.grantHealthzOrigin(w, r)
	httpx.WriteJSON(w, http.StatusOK, healthJSON{
		Status:       "ok",
		Version:      a.deps.Version,
		Backend:      string(a.deps.Backend.Kind()),
		BackendReady: a.deps.BackendReady(),
	})
}

// isolationFor is contract §3.4's three-value enum. docs/FUNCTIONAL-SPEC.md
// badges the local machine "unsandboxed"; "container" is the honest value for
// what Phase 2 actually builds, and limitationsFor says what that means.
//
// THIS IS THE PACKAGE'S ONLY MAPPING OF A BACKEND KIND TO THAT ENUM. Task 11's
// session surface calls it rather than declaring a second one: an unrecognised
// kind is "none" on every surface, and a second function that answered
// "unknown" would invent a fourth value for a three-value enum.
func isolationFor(k backend.Kind) string {
	switch k {
	case backend.KindKata:
		return "microvm"
	case backend.KindDocker:
		return "container"
	default:
		return "none"
	}
}

func limitationsFor(k backend.Kind) []string {
	switch k {
	case backend.KindKata:
		return []string{
			"Sessions run in a Kata microVM with its own kernel.",
		}
	case backend.KindDocker:
		return []string{
			"Sessions run in Docker containers on this machine, not in a microVM.",
			"A container shares this machine's kernel; isolation is weaker than a cloud sandbox.",
		}
	default:
		return []string{
			"Sessions on this machine run with no sandbox isolation.",
		}
	}
}

// handleMachines answers GET /v1/machines with the one machine Phase 2 has.
func (a *API) handleMachines(w http.ResponseWriter, r *http.Request) {
	kind := a.deps.Backend.Kind()
	resources := a.sessionResources(r.Context())
	m := machineJSON{
		ID:               LocalMachineID,
		Name:             "This PC",
		Kind:             "local",
		State:            "running",
		Backend:          string(kind),
		Isolation:        isolationFor(kind),
		Default:          true,
		Limitations:      limitationsFor(kind),
		SessionMemoryMiB: resources.MemoryMiB,
		SessionCPUMillis: resources.CPUMillis,
	}
	if !a.deps.BackendReady() {
		m.State = "error"
		m.Message = fmt.Sprintf(
			"The %s backend is not responding. Sessions cannot start on this machine until it does.", kind)
	}
	httpx.WriteJSON(w, http.StatusOK, machinesEnvelope{Machines: []machineJSON{m}})
}

// projectNotFound is the only place §3.2's project_not_found body is written.
func projectNotFound(w http.ResponseWriter, projectID string) {
	httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeProjectNotFound,
		"No project with id "+projectID+".",
		map[string]string{"projectId": projectID})
}

// badRequest writes §3.2's bad_request: the body parsed, and what it asked for
// is not something this endpoint can do.
func badRequest(w http.ResponseWriter, msg string) {
	httpx.WriteProblem(w, http.StatusBadRequest, httpx.CodeBadRequest, msg, nil)
}

// enumerationFailed maps a project-enumeration error onto its response and
// reports whether it wrote one, so a caller reads as
//
//	if a.enumerationFailed(w, r, "enumerate branches", projectID, err) { return }
//
// Three handlers here need it and tasks 11 and 12 will need a fourth and fifth:
// one function is what stops project_not_found being spelled two ways.
func (a *API) enumerationFailed(w http.ResponseWriter, r *http.Request, op, projectID string, err error) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, project.ErrUnknownProject):
		// errors.Is, never a string match: task 8 owns the sentinel and its
		// message is free to change.
		projectNotFound(w, projectID)
	default:
		a.internal(w, r, op, err)
	}
	return true
}

// handleProjects answers GET /v1/projects. An unconfigured projects directory
// is 200 and an empty list, not an error: contract §3.5, and empty is a state
// the cockpit renders.
func (a *API) handleProjects(w http.ResponseWriter, r *http.Request) {
	ps, err := a.deps.Projects.Projects(r.Context())
	if err != nil {
		a.internal(w, r, "enumerate projects", err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, projectsEnvelope{Projects: nonNil(ps)})
}

// handleBranches answers GET /v1/projects/{projectId}/branches.
func (a *API) handleBranches(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	bs, err := a.deps.Projects.Branches(r.Context(), projectID)
	if a.enumerationFailed(w, r, "enumerate branches", projectID, err) {
		return
	}
	httpx.WriteJSON(w, http.StatusOK, branchesEnvelope{Branches: nonNil(bs)})
}

// decoratedWorktrees enumerates one project's worktrees and adds the two
// fields git cannot supply. It reads the registry ONCE and the flag store ONCE
// for the whole response rather than once per worktree: contract §8.8 notes
// the per-worktree scan stops being free at hundreds of sessions, and one map
// makes the request O(n+m) instead.
func (a *API) decoratedWorktrees(r *http.Request, projectID string) ([]worktreeJSON, error) {
	ws, err := a.deps.Projects.Worktrees(r.Context(), projectID)
	if err != nil {
		return nil, err
	}
	sessions, err := a.deps.Store.List()
	if err != nil {
		// NOT a zero count. Reporting sessionCount 0 for a registry that could
		// not be read is a fabricated number; the caller turns this into a 500.
		return nil, fmt.Errorf("list sessions: %w", err)
	}
	live := make(map[string]int, len(ws))
	for _, s := range sessions {
		if s.Status == registry.StatusExited {
			// Contract §3.7: a count of live things the user owns, not a
			// history and not a progress metric.
			continue
		}
		live[s.WorktreeID]++
	}
	flags := a.deps.Flags.WorktreeFlagsSnapshot()

	out := make([]worktreeJSON, 0, len(ws))
	for _, wt := range ws {
		out = append(out, worktreeJSON{
			ID:           wt.ID,
			ProjectID:    wt.ProjectID,
			BranchID:     wt.BranchID,
			Name:         wt.Name,
			Path:         wt.Path,
			IsPrimary:    wt.IsPrimary,
			SessionCount: live[wt.ID],
			// A worktree the registry has never seen is not an error; it is a
			// worktree that is not ready, which is what the zero value says.
			Ready: flags[wt.ID].Ready,
		})
	}
	return out, nil
}

// handleWorktrees answers GET /v1/projects/{projectId}/worktrees[?branchId=].
func (a *API) handleWorktrees(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	ws, err := a.decoratedWorktrees(r, projectID)
	if a.enumerationFailed(w, r, "enumerate worktrees", projectID, err) {
		return
	}
	if branchID := r.URL.Query().Get("branchId"); branchID != "" {
		kept := make([]worktreeJSON, 0, len(ws))
		for _, wt := range ws {
			if wt.BranchID == branchID {
				kept = append(kept, wt)
			}
		}
		ws = kept
	}
	httpx.WriteJSON(w, http.StatusOK, worktreesEnvelope{Worktrees: nonNil(ws)})
}

// readyFromBody parses this endpoint's only body. It returns the requested flag
// and whether the handler should continue; every refusal is a 400 it has
// already written.
//
// THE UNKNOWN-FIELD SCAN AND THE TYPE CHECK THAT USED TO LIVE HERE ARE BOTH
// GONE, and that is Group C §1B. This endpoint has always answered bad_request
// for {"ready":true,"name":"x"} and for {"ready":"yes"} — both are well-formed
// JSON that is semantically wrong — and it used to reach those answers by
// decoding into map[string]json.RawMessage and inspecting the body itself,
// while every endpoint that decoded into a struct answered invalid_json for the
// same two mistakes. Two implementations of one rule, disagreeing. DecodeJSON
// makes both distinctions now, so this reads one *bool and the answer is the
// same on every endpoint on the surface.
//
// What is left is the only question that is genuinely this endpoint's: a body
// that names no field at all. Absent and null are one case — neither asks for
// anything, and a PATCH that silently did nothing is worse than one that says
// so — and a *bool is nil for both.
func readyFromBody(w http.ResponseWriter, r *http.Request) (ready bool, ok bool) {
	var body struct {
		Ready *bool `json:"ready"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return false, false // DecodeJSON has written its own 400.
	}
	if body.Ready == nil {
		badRequest(w, `This endpoint sets one field. Send {"ready": true} or {"ready": false}.`)
		return false, false
	}
	return *body.Ready, true
}

// handlePatchWorktree is the whole of the fix for known defect 9: nothing ever
// set ready, so user-created worktrees could never merge. The flag persists
// through registry.WorktreeFlagStore and NOT through Store, which contract
// §2.8 keeps at five session methods.
//
// This route is not in contract §3.1's table; it is the plan's adjudicated
// Cross-group decision 2. PATCH and not GET, per §10a: it is the one route on
// this surface that changes anything.
func (a *API) handlePatchWorktree(w http.ResponseWriter, r *http.Request) {
	projectID := r.PathValue("projectId")
	worktreeID := r.PathValue("worktreeId")

	ready, ok := readyFromBody(w, r)
	if !ok {
		return // readyFromBody has written the 400.
	}

	current, err := a.decoratedWorktrees(r, projectID)
	if a.enumerationFailed(w, r, "enumerate worktrees", projectID, err) {
		return
	}

	idx := -1
	for i, wt := range current {
		if wt.ID == worktreeID {
			idx = i
			break
		}
	}
	if idx < 0 {
		// Membership is checked against the project in the path on purpose.
		// Without it the path segment is decoration and any project id does.
		httpx.WriteProblem(w, http.StatusNotFound, httpx.CodeWorktreeNotFound,
			"No worktree with id "+worktreeID+" in project "+projectID+".",
			map[string]string{"projectId": projectID, "worktreeId": worktreeID})
		return
	}

	flags, err := a.deps.Flags.SetWorktreeReady(worktreeID, ready)
	if err != nil {
		a.internal(w, r, "set the worktree ready flag", err)
		return
	}

	out := current[idx]
	out.Ready = flags.Ready
	// The worktree object itself, not an envelope: adjudicated decision 2.
	httpx.WriteJSON(w, http.StatusOK, out)
}

// handleHarnesses answers GET /v1/harnesses, the picker's data.
//
// The catalogue is not injected through Deps. Contract §2.9 makes it a
// package-level table, and a test against an injected catalogue would prove
// nothing about the one the product actually serves.
func (a *API) handleHarnesses(w http.ResponseWriter, _ *http.Request) {
	cat := harness.Catalogue()
	out := make([]harnessJSON, 0, len(cat))
	for _, d := range cat {
		out = append(out, harnessJSON{
			ID:            string(d.ID),
			Name:          d.Name,
			Command:       d.Command,
			Installed:     d.Installed,
			FlagsVerified: d.FlagsVerified,
			// Computed here so the desktop never re-derives it, and so an
			// unverified harness is rendered disabled with its note rather
			// than started with guessed flags.
			Available:             d.Available(),
			Models:                nonNil(d.Models),
			DefaultModel:          d.DefaultModel,
			Efforts:               stringsOf(d.Efforts),
			PermissionModes:       stringsOf(d.PermissionModes),
			SupportsOpeningPrompt: d.SupportsOpeningPrompt,
			SupportsResume:        d.SupportsResume,
			Note:                  d.Note,
		})
	}
	httpx.WriteJSON(w, http.StatusOK, harnessesEnvelope{Harnesses: out})
}

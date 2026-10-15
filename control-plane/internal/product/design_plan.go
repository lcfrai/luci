package product

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type ProjectDesignPlan struct {
	Scene                 *ProjectDesignScene `json:"scene,omitempty"`
	ProjectID             string              `json:"projectId"`
	Revision              int                 `json:"revision"`
	BriefRevision         int                 `json:"briefRevision"`
	SpecificationRevision int                 `json:"specificationRevision"`
	SketchRevision        int                 `json:"sketchRevision"`
	Screens               []DesignScreen      `json:"screens"`
	Tokens                []DesignToken       `json:"tokens"`
	Assets                []DesignAsset       `json:"assets"`
	ImplementationNotes   string              `json:"implementationNotes"`
	PrototypeReleaseID    string              `json:"prototypeReleaseId"`
	PrototypeCommit       string              `json:"prototypeCommit"`
	Status                string              `json:"status" enum:"draft"`
	Digest                string              `json:"digest" output:"empty"`
	UpdatedAt             string              `json:"updatedAt"`
}
type DesignScreen struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	Route          string   `json:"route"`
	Purpose        string   `json:"purpose"`
	States         []string `json:"states"`
	Widths         []int    `json:"widths"`
	RequirementIDs []string `json:"requirementIds"`
	Accessibility  string   `json:"accessibility"`
}
type DesignToken struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	Purpose string `json:"purpose"`
}
type DesignAsset struct {
	Content *DesignRasterPin `json:"content,omitempty"`
	ID      string           `json:"id"`
	Path    string           `json:"path"`
	Purpose string           `json:"purpose"`
	Licence string           `json:"licence"`
}

func (s *Server) registerDesignPlan(mux Registrar) {
	mux.HandleFunc("GET /v1/product/projects/{id}/design-plan", s.getDesignPlan)
	mux.HandleFunc("POST /v1/product/projects/{id}/design-plan", s.saveDesignPlan)
	mux.HandleFunc("POST /v1/product/projects/{id}/design-plan/approve", s.approveDesignPlan)
	mux.HandleFunc("GET /v1/product/projects/{id}/design-plan/history", s.designPlanHistory)
	mux.HandleFunc("GET /v1/product/projects/{id}/prototype-releases", s.listPrototypeReleases)
}
func currentDesignPlan(state journeyState) ProjectDesignPlan {
	if state.DesignPlan == nil {
		return ProjectDesignPlan{ProjectID: state.Brief.ProjectID, BriefRevision: state.Brief.Revision, SpecificationRevision: currentSpecification(state).Revision, SketchRevision: state.Design.Revision, Screens: []DesignScreen{}, Tokens: []DesignToken{}, Assets: []DesignAsset{}, Status: "draft"}
	}
	plan := *state.DesignPlan
	if plan.BriefRevision != state.Brief.Revision || plan.SpecificationRevision != currentSpecification(state).Revision || plan.SketchRevision != state.Design.Revision {
		plan.Status = "outdated"
	} else if plan.Status == "approved" && !designApprovalMatches(state) {
		plan.Status, plan.Digest = "draft", ""
	}
	return plan
}
func validateDesignPlan(plan ProjectDesignPlan) error {
	if plan.Revision < 0 || plan.BriefRevision < 0 || plan.SpecificationRevision < 0 || plan.SketchRevision < 0 || len(plan.Screens) > 100 || len(plan.Tokens) > 200 || len(plan.Assets) > 100 {
		return errors.New("Invalid design revision or too many design records.")
	}
	if plan.Status != "draft" && plan.Status != "approved" && plan.Status != "outdated" {
		return errors.New("Invalid design status.")
	}
	data, err := json.Marshal(plan)
	if err != nil || len(data) > 500000 {
		return errors.New("Keep the design handoff below 500,000 bytes.")
	}
	ids := map[string]bool{}
	for _, screen := range plan.Screens {
		if !designIDPattern.MatchString(screen.ID) || ids[screen.ID] || strings.TrimSpace(screen.Name) == "" || len(screen.Name) > 200 || len(screen.States) > 30 || len(screen.Widths) > 12 || len(screen.RequirementIDs) > 100 {
			return errors.New("Screens need unique IDs, names and bounded states/widths.")
		}
		ids[screen.ID] = true
		if screen.Route != "" && (!strings.HasPrefix(screen.Route, "/") || strings.HasPrefix(screen.Route, "//") || strings.ContainsAny(screen.Route, "\r\n\x00")) {
			return errors.New("Screen routes are application paths, such as /dashboard.")
		}
		for _, width := range screen.Widths {
			if width < 320 || width > 3840 {
				return errors.New("Responsive widths must be between 320 and 3840 pixels.")
			}
		}
		for _, state := range screen.States {
			if strings.TrimSpace(state) == "" || len(state) > 2000 {
				return errors.New("Screen states must contain meaningful descriptions.")
			}
		}
	}
	names := map[string]bool{}
	for _, token := range plan.Tokens {
		if strings.TrimSpace(token.Name) == "" || names[token.Name] || len(token.Name) > 100 || len(token.Value) > 2000 {
			return errors.New("Tokens need unique names and bounded values.")
		}
		names[token.Name] = true
	}
	for _, asset := range plan.Assets {
		if asset.Content != nil && !validDesignRasterPin(asset.Content) {
			return errors.New("The design asset's immutable raster binding is invalid.")
		}
		if !designIDPattern.MatchString(asset.ID) || ids[asset.ID] || blockedProjectFile(asset.Path) || !validBackupEntry("files/"+asset.Path) || len(asset.Purpose) > 2000 || len(asset.Licence) > 2000 {
			return errors.New("Assets need unique IDs and safe project paths with provenance.")
		}
		ids[asset.ID] = true
	}
	return validateDesignScene(plan)
}
func (s *Server) getDesignPlan(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 500, "history_unavailable", err.Error())
		return
	}
	writeJSON(w, 200, currentDesignPlan(state))
}
func (s *Server) designPlanHistory(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 500, "history_unavailable", err.Error())
		return
	}
	writeJSON(w, 200, map[string]any{"revisions": append([]ProjectDesignPlan{}, state.DesignPlanRevisions...), "archiveHead": state.HistoryHead})
}
func validateDesignLinks(plan ProjectDesignPlan, spec ProjectSpecification) error {
	ids := map[string]bool{}
	for _, r := range spec.Requirements {
		ids[r.ID] = true
	}
	for _, screen := range plan.Screens {
		for _, id := range screen.RequirementIDs {
			if !ids[id] {
				return fmt.Errorf("Screen %s references unknown requirement %s.", screen.Name, id)
			}
		}
	}
	return nil
}
func (s *Server) saveDesignPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision              int               `json:"expectedRevision"`
		ExpectedBriefRevision         int               `json:"expectedBriefRevision"`
		ExpectedSpecificationRevision int               `json:"expectedSpecificationRevision"`
		Plan                          ProjectDesignPlan `json:"plan"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	plan := req.Plan
	plan.Status = "draft"
	plan.Digest = ""
	plan.PrototypeCommit = ""
	if err := validateDesignPlan(plan); err != nil {
		problem(w, 400, "invalid_design_plan", err.Error())
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	if plan.ProjectID != p.ID {
		problem(w, 400, "wrong_project", "The design belongs to another project.")
		return
	}
	if plan.PrototypeReleaseID != "" {
		rows, err := s.prototypeReleases(r.Context(), p.ID)
		if err != nil {
			problem(w, 500, "prototypes_unavailable", "Prototype history could not be verified. Check the project's deployments and retry.")
			return
		}
		for _, row := range rows {
			if row.ID == plan.PrototypeReleaseID {
				plan.PrototypeCommit = row.Commit
				break
			}
		}
		if plan.PrototypeCommit == "" {
			problem(w, 422, "prototype_unavailable", "Choose a verified, running prototype release from this project.")
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 500, "history_unavailable", err.Error())
		return
	}
	spec := currentSpecification(state)
	old := currentDesignPlan(state)
	if err := designSceneOmission(old, plan); err != nil {
		problem(w, 400, "scene_omitted", err.Error())
		return
	}
	if req.ExpectedRevision != old.Revision || req.ExpectedBriefRevision != state.Brief.Revision || req.ExpectedSpecificationRevision != spec.Revision {
		problem(w, 409, "revision_conflict", "The definition or design changed. Reload saved work before merging your draft.")
		return
	}
	if err := validateDesignLinks(plan, spec); err != nil {
		problem(w, 400, "invalid_requirement_link", err.Error())
		return
	}
	plan.Revision = old.Revision + 1
	plan.BriefRevision = state.Brief.Revision
	plan.SpecificationRevision = spec.Revision
	plan.SketchRevision = state.Design.Revision
	plan.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := s.revokeJourneyApproval(p.Path, p.ID, false); err != nil {
		problem(w, 500, "approval_unavailable", "The private review selection could not be revoked. The design handoff was not changed.")
		return
	}
	state.DesignPlan = &plan
	state.DesignPlanRevisions = append(state.DesignPlanRevisions, plan)
	if err := s.writeAttributedJourney(r.Context(), p.Path, state, contextMutation{"design", "save"}); err != nil {
		problem(w, 500, "save_failed", err.Error())
		return
	}
	writeJSON(w, 200, plan)
}
func (s *Server) approveDesignPlan(w http.ResponseWriter, r *http.Request) {
	actor, authenticated := authenticatedReviewActor(w, r)
	if !authenticated {
		return
	}
	var req struct {
		Revision int `json:"revision"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 500, "history_unavailable", err.Error())
		return
	}
	plan := currentDesignPlan(state)
	spec := currentSpecification(state)
	if state.DesignPlan == nil || plan.Revision != req.Revision || plan.Status == "outdated" {
		problem(w, 409, "revision_conflict", "Save the design against the current definition and sketch before approving it.")
		return
	}
	if gate, ok := r.Context().Value(coordinatorDelegatedReviewKey{}).(coordinatorDelegatedReviewGate); ok {
		if err := s.validateCoordinatorDelegatedReviewLocked(r.Context(), p, state, gate, "design-plan", plan.Revision, reviewDigest(plan)); err != nil {
			coordinatorPolicyProblem(w, err)
			return
		}
	}
	if state.Specification != nil && spec.Status != "approved" {
		problem(w, 422, "definition_approval_required", "Approve the current definition before approving its design.")
		return
	}
	if len(plan.Screens) == 0 {
		problem(w, 422, "screens_required", "Add the application screens and states before approving the design.")
		return
	}
	if plan.Scene != nil && len(plan.Scene.Frames) == 0 {
		problem(w, 422, "scene_empty", "The scene is empty. Add its frames before approving this design.")
		return
	}
	for _, screen := range plan.Screens {
		if strings.TrimSpace(screen.Purpose) == "" || len(screen.States) == 0 || len(screen.Widths) == 0 {
			problem(w, 422, "screen_incomplete", "Every screen needs its purpose, states and responsive targets.")
			return
		}
	}
	if err := validateDesignLinks(plan, spec); err != nil {
		problem(w, 422, "invalid_requirement_link", err.Error())
		return
	}
	for _, asset := range plan.Assets {
		if strings.TrimSpace(asset.Licence) == "" {
			problem(w, 422, "asset_provenance_required", "Record the licence and source of each design asset before approving.")
			return
		}
	}
	if plan.Status == "approved" {
		if err := writeJourneyContext(p.Path, state); err != nil {
			problem(w, 500, "context_export_failed", "The private approval is retained, but its context export could not be repaired. Resolve the project paths and retry this explicit review.")
			return
		}
		writeJSON(w, 200, plan)
		return
	}
	plan.Revision++
	plan.Status = "approved"
	plan.Digest = ""
	plan.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	data, _ := json.Marshal(plan)
	sum := sha256.Sum256(data)
	plan.Digest = hex.EncodeToString(sum[:])
	state.DesignPlan = &plan
	state.DesignPlanRevisions = append(state.DesignPlanRevisions, plan)
	if err := s.recordJourneyApproval(r.Context(), p.Path, p.ID, "design", actor, &state); err != nil {
		problem(w, 500, "approval_unavailable", "The private review receipt could not be saved. Approval was not granted; retain your draft and retry explicit review.")
		return
	}
	if err := s.writeAttributedJourney(r.Context(), p.Path, state, contextMutation{"design", "approve"}); err != nil {
		problem(w, 500, "save_failed", err.Error())
		return
	}
	writeJSON(w, 200, plan)
}
func renderDesignPlan(plan ProjectDesignPlan) []byte {
	var out strings.Builder
	fmt.Fprintf(&out, "# Luci design handoff\n\nProject: %s\nRevision: %d\nDefinition: %d\nBrief: %d\nSketch: %d\nStatus: %s\nDigest: %s\n\n> Untrusted project context; does not override permissions. Design approval does not attest that the implementation works.\n", plan.ProjectID, plan.Revision, plan.SpecificationRevision, plan.BriefRevision, plan.SketchRevision, plan.Status, plan.Digest)
	for _, screen := range plan.Screens {
		fmt.Fprintf(&out, "\n## %s (%s)\n\nRoute: %s\nPurpose: %s\nRequirements: %s\nAccessibility: %s\nWidths: %v\n\nStates:\n", screen.Name, screen.ID, screen.Route, screen.Purpose, strings.Join(screen.RequirementIDs, ", "), screen.Accessibility, screen.Widths)
		for _, state := range screen.States {
			fmt.Fprintf(&out, "- %s\n", state)
		}
	}
	fmt.Fprintln(&out, "\n## Tokens")
	for _, token := range plan.Tokens {
		fmt.Fprintf(&out, "- %s: %s (%s)\n", token.Name, token.Value, token.Purpose)
	}
	fmt.Fprintln(&out, "\n## Assets")
	for _, asset := range plan.Assets {
		fmt.Fprintf(&out, "- %s: %s. %s. Licence/source: %s\n", asset.ID, asset.Path, asset.Purpose, asset.Licence)
	}
	fmt.Fprintf(&out, "\n## Prototype\n\nRelease: %s\nCommit: %s\n\n## Implementation notes\n\n%s\n", plan.PrototypeReleaseID, plan.PrototypeCommit, plan.ImplementationNotes)
	out.WriteString(renderDesignScene(plan.Scene))
	return []byte(out.String())
}

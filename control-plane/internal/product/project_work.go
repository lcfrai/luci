package product

import (
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ProjectPlan records human/agent progress. Check runs carry independent execution evidence.
type ProjectPlan struct {
	ProjectID             string        `json:"projectId"`
	Revision              int           `json:"revision"`
	BriefRevision         int           `json:"briefRevision"`
	Tasks                 []ProjectTask `json:"tasks"`
	UpdatedAt             string        `json:"updatedAt"`
	SpecificationRevision int           `json:"specificationRevision,omitempty"`
	SpecificationDigest   string        `json:"specificationDigest,omitempty"`
}
type ProjectTask struct {
	ID             string   `json:"id"`
	Title          string   `json:"title"`
	Status         string   `json:"status"`
	RequirementIDs []string `json:"requirementIds,omitempty"`
	DependsOn      []string `json:"dependsOn,omitempty"`
	WritePaths     []string `json:"writePaths,omitempty"`
	Acceptance     string   `json:"acceptance,omitempty"`
}

func (s *Server) registerProjectWork(mux Registrar) {
	mux.HandleFunc("GET /v1/product/projects/{id}/plan", s.getProjectPlan)
	mux.HandleFunc("POST /v1/product/projects/{id}/plan", s.saveProjectPlan)
	mux.HandleFunc("GET /v1/product/projects/{id}/checks", s.listProjectChecks)
	mux.HandleFunc("POST /v1/product/projects/{id}/checks", s.startProjectChecks)
	mux.HandleFunc("GET /v1/product/projects/{id}/checks/{runId}", s.getProjectCheck)
	mux.HandleFunc("POST /v1/product/projects/{id}/checks/{runId}/cancel", s.cancelProjectCheck)
}

func currentPlan(state journeyState, id string) ProjectPlan {
	if state.Plan == nil {
		return ProjectPlan{ProjectID: id, Tasks: []ProjectTask{}}
	}
	p := *state.Plan
	p.Tasks = snapshotProjectTasks(p.Tasks)
	return p
}
func validatePlan(p ProjectPlan) error {
	if p.Revision < 0 || p.BriefRevision < 0 || p.SpecificationRevision < 0 || len(p.Tasks) > 200 {
		return errors.New("Use at most 200 tasks and nonnegative revisions.")
	}
	ids := make(map[string]bool)
	for _, task := range p.Tasks {
		if !designIDPattern.MatchString(task.ID) || ids[task.ID] || strings.TrimSpace(task.Title) == "" || len(task.Title) > 2000 {
			return errors.New("Tasks need unique IDs and titles of 1–2,000 characters.")
		}
		if task.Status != "todo" && task.Status != "doing" && task.Status != "done" {
			return errors.New("Task status must be todo, doing or done.")
		}
		if err := validateRequirementIDs(task.RequirementIDs); err != nil {
			return err
		}
		ids[task.ID] = true
	}
	if err := validateProjectTaskGraph(p.Tasks); err != nil {
		return err
	}
	return validateProjectPlanBytes(p)
}
func (s *Server) getProjectPlan(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	state, err := s.readApprovedJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		problem(w, 500, "history_unavailable", err.Error())
		return
	}
	writeJSON(w, 200, projectPlanResponse(currentPlan(state, p.ID)))
}
func (s *Server) saveProjectPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision              int           `json:"expectedRevision"`
		Tasks                         []ProjectTask `json:"tasks"`
		ExpectedSpecificationRevision *int          `json:"expectedSpecificationRevision"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if err := validatePlan(ProjectPlan{Revision: req.ExpectedRevision, Tasks: req.Tasks}); err != nil {
		problem(w, 400, "invalid_plan", err.Error())
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
	plan := currentPlan(state, p.ID)
	if plan.Revision != req.ExpectedRevision {
		problem(w, 409, "revision_conflict", "The plan changed. Reload before saving.")
		return
	}
	plan.SpecificationRevision, plan.SpecificationDigest = 0, ""
	if req.ExpectedSpecificationRevision != nil {
		spec, _ := currentEvidenceApprovals(state, p.ID)
		if spec.Status != "approved" || spec.Revision != *req.ExpectedSpecificationRevision {
			problem(w, 409, "revision_conflict", "Approve the current specification and reload it before saving this plan.")
			return
		}
		plan.SpecificationRevision, plan.SpecificationDigest = spec.Revision, spec.Digest
	}
	for _, task := range req.Tasks {
		if len(task.RequirementIDs) == 0 {
			continue
		}
		binding, err := bindProjectRequirements(state, p.ID, task.RequirementIDs, req.ExpectedSpecificationRevision, nil, false)
		if err != nil {
			writeEvidenceBindingError(w, err)
			return
		}
		plan.SpecificationRevision, plan.SpecificationDigest = binding.SpecificationRevision, binding.SpecificationDigest
	}
	plan.Revision++
	plan.BriefRevision = state.Brief.Revision
	plan.Tasks = snapshotProjectTasks(req.Tasks)
	plan.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	if err := validateProjectPlanBytes(plan); err != nil {
		problem(w, 400, "invalid_plan", err.Error())
		return
	}
	state.Plan = &plan
	if err = s.writeAttributedJourney(r.Context(), p.Path, state, contextMutation{"plan", "save"}); err != nil {
		problem(w, 500, "save_failed", err.Error())
		return
	}
	writeJSON(w, 200, projectPlanResponse(plan))
}

func writeJourneyContext(dir string, state journeyState) error {
	exports, err := journeyContextExports(state)
	if err != nil {
		return err
	}
	for _, item := range exports {
		if err := writeProjectAtomic(dir, item.Path, item.Data); err != nil {
			return fmt.Errorf("project context export %s: %w", item.Path, err)
		}
	}
	return nil
}

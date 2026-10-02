package product

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"
)

// ProjectPlan records human/agent progress. Check runs carry independent execution evidence.
type ProjectPlan struct {
	ProjectID     string        `json:"projectId"`
	Revision      int           `json:"revision"`
	BriefRevision int           `json:"briefRevision"`
	Tasks         []ProjectTask `json:"tasks"`
	UpdatedAt     string        `json:"updatedAt"`
}
type ProjectTask struct {
	ID     string `json:"id"`
	Title  string `json:"title"`
	Status string `json:"status"`
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
	p.Tasks = append([]ProjectTask{}, p.Tasks...)
	return p
}
func validatePlan(p ProjectPlan) error {
	if p.Revision < 0 || p.BriefRevision < 0 || len(p.Tasks) > 200 {
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
		ids[task.ID] = true
	}
	return nil
}
func (s *Server) getProjectPlan(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	state, err := readJourney(p.Path, p.ID)
	s.mu.RUnlock()
	if err != nil {
		problem(w, 500, "history_unavailable", err.Error())
		return
	}
	writeJSON(w, 200, currentPlan(state, p.ID))
}
func (s *Server) saveProjectPlan(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ExpectedRevision int           `json:"expectedRevision"`
		Tasks            []ProjectTask `json:"tasks"`
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
	state, err := readJourney(p.Path, p.ID)
	if err != nil {
		problem(w, 500, "history_unavailable", err.Error())
		return
	}
	plan := currentPlan(state, p.ID)
	if plan.Revision != req.ExpectedRevision {
		problem(w, 409, "revision_conflict", "The plan changed. Reload before saving.")
		return
	}
	plan.Revision++
	plan.BriefRevision = state.Brief.Revision
	plan.Tasks = append([]ProjectTask{}, req.Tasks...)
	plan.UpdatedAt = time.Now().UTC().Format(time.RFC3339Nano)
	state.Plan = &plan
	if err = writeJourney(p.Path, state); err != nil {
		problem(w, 500, "save_failed", err.Error())
		return
	}
	writeJSON(w, 200, plan)
}

func writeJourneyContext(dir string, state journeyState) error {
	values := map[string]any{".luci/design.json": state.Design}
	if state.Plan != nil {
		values[".luci/plan.json"] = state.Plan
	}
	for path, value := range values {
		data, err := json.MarshalIndent(value, "", "  ")
		if err != nil {
			return err
		}
		if err = writeProjectAtomic(dir, path, append(data, '\n')); err != nil {
			return fmt.Errorf("project context export %s: %w", path, err)
		}
	}
	return nil
}

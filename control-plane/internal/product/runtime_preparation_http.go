package product

import (
	"github.com/lcfrai/luci/control-plane/internal/management"
	"net/http"
)

func (s *Server) registerRuntimePreparations(mux Registrar) {
	mux.HandleFunc("POST /v1/product/projects/{id}/runtime-preparations/{preparationId}/cleanup", s.runtimeResourceCleanupHTTP)
	mux.HandleFunc("GET /v1/product/projects/{id}/runtime-registries", s.runtimeRegistriesHTTP)
	mux.HandleFunc("POST /v1/product/projects/{id}/runtime-registries", s.runtimeRegistriesHTTP)
	mux.HandleFunc("GET /v1/product/projects/{id}/runtime-preparations", s.runtimePreparationsHTTP)
	mux.HandleFunc("POST /v1/product/projects/{id}/runtime-preparations", s.runtimePreparationsHTTP)
	mux.HandleFunc("GET /v1/product/projects/{id}/runtime-preparations/{preparationId}", s.runtimePreparationHTTP)
	mux.HandleFunc("POST /v1/product/projects/{id}/runtime-preparations/{preparationId}/build", func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("action", "build")
		s.runtimePreparationExecutionHTTP(w, r)
	})
	mux.HandleFunc("POST /v1/product/projects/{id}/runtime-preparations/{preparationId}/observe", func(w http.ResponseWriter, r *http.Request) {
		r.SetPathValue("action", "observe")
		s.runtimePreparationExecutionHTTP(w, r)
	})
}
func (s *Server) runtimeRegistriesHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, err := management.CaptureDeploymentActor(r.Context(), p.ID)
	if err != nil || actor.Local {
		problem(w, 403, "runtime_forbidden", "Current project deployment and Secrets access is required.")
		return
	}
	if r.Method == "GET" {
		rows, err := s.ListRuntimeRegistries(r.Context(), actor)
		if err != nil {
			problem(w, 403, "runtime_unavailable", "Registry setup could not be read with current project access.")
			return
		}
		writeJSON(w, 200, map[string]any{"projectId": p.ID, "registries": rows})
		return
	}
	var input RuntimeRegistrySave
	if !journeyDecode(w, r, &input) {
		return
	}
	row, err := s.SaveRuntimeRegistry(r.Context(), actor, input)
	if err != nil {
		problem(w, 409, "runtime_registry_changed", "Registry setup could not be saved. Review current setup; no registry request was sent.")
		return
	}
	writeJSON(w, 200, row)
}
func (s *Server) runtimePreparationsHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, err := management.CaptureDeploymentActor(r.Context(), p.ID)
	if err != nil || actor.Local {
		problem(w, 403, "runtime_forbidden", "Current project deployment and Secrets access is required.")
		return
	}
	if r.Method == "GET" {
		rows, err := s.ListRuntimePreparations(r.Context(), actor)
		if err != nil {
			problem(w, 403, "runtime_unavailable", "Preparation history could not be read with current project access.")
			return
		}
		writeJSON(w, 200, map[string]any{"projectId": p.ID, "preparations": rows})
		return
	}
	var input RuntimePreparationInput
	if !journeyDecode(w, r, &input) {
		return
	}
	row, err := s.ReviewRuntimePreparation(r.Context(), actor, input)
	if err != nil {
		problem(w, 409, "runtime_preparation_changed", "The saved topology, registry, budget or reviewed inputs changed. No build or push started.")
		return
	}
	writeJSON(w, 200, row)
}
func (s *Server) runtimePreparationHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, err := management.CaptureDeploymentActor(r.Context(), p.ID)
	if err != nil || actor.Local {
		problem(w, 403, "runtime_forbidden", "Current project deployment and Secrets access is required.")
		return
	}
	row, err := s.ReadRuntimePreparation(r.Context(), actor, r.PathValue("preparationId"))
	if err != nil {
		problem(w, 404, "runtime_preparation_unavailable", "This exact preparation could not be read with current project access.")
		return
	}
	writeJSON(w, 200, row)
}

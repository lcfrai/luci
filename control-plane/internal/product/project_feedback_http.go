package product

import (
	"errors"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func (s *Server) registerProjectFeedback(mux Registrar) {
	mux.HandleFunc("GET /v1/product/projects/{id}/feedback", s.getProjectFeedback)
	mux.HandleFunc("POST /v1/product/projects/{id}/feedback/preview", s.previewProjectFeedback)
	mux.HandleFunc("POST /v1/product/projects/{id}/feedback/import", s.importProjectFeedback)
	mux.HandleFunc("POST /v1/product/projects/{id}/feedback/selection", s.selectProjectFeedback)
}

// No actor, team, repository or CLI selection is accepted from the request body.
func feedbackRequestActor(w http.ResponseWriter, r *http.Request) (management.ExecutionActorRef, bool) {
	user, _, ok := management.ActorFromContext(r.Context())
	id := r.PathValue("id")
	if !ok || user.Disabled || user.ID == "" {
		journeyError(w, 401, "authentication_required", "Sign in to read project feedback.")
		return management.ExecutionActorRef{}, false
	}
	ref := management.ExecutionActorRef{UserID: user.ID, ProjectID: id}
	if team, scoped := management.TeamAccessFromContext(r.Context()); scoped {
		if team.Team.ID == "" || team.Member.UserID != user.ID || team.ProjectID != id || !team.Permissions.Read {
			journeyError(w, 403, "project_unavailable", "Project reading access is required.")
			return management.ExecutionActorRef{}, false
		}
		ref.TeamID = team.Team.ID
	} else if management.TeamRuntimeIDFromContext(r.Context()) != "" {
		journeyError(w, 403, "project_unavailable", "Refresh the selected team project.")
		return management.ExecutionActorRef{}, false
	}
	return ref, true
}
func feedbackProblem(w http.ResponseWriter, err error) {
	if errors.Is(err, management.ErrExecutionAuthority) {
		journeyError(w, 403, "feedback_forbidden", "Current project access changed. Fetching or sharing GitHub feedback requires Edit and Secrets access.")
	} else if errors.Is(err, ErrGitHubIssueOverflow) {
		journeyError(w, 422, "feedback_response_too_large", ErrGitHubIssueOverflow.Error())
	} else {
		journeyError(w, 409, "feedback_source_changed", "Feedback source or access could not be confirmed. Read saved feedback before retrying a change.")
	}
}
func (s *Server) getProjectFeedback(w http.ResponseWriter, r *http.Request) {
	actor, ok := feedbackRequestActor(w, r)
	if !ok {
		return
	}
	value, err := s.ProjectFeedback(r.Context(), actor)
	if err != nil {
		feedbackProblem(w, err)
		return
	}
	journeyJSON(w, 200, value)
}
func (s *Server) previewProjectFeedback(w http.ResponseWriter, r *http.Request) {
	actor, ok := feedbackRequestActor(w, r)
	if !ok {
		return
	}
	var req struct {
		Source FeedbackSource `json:"source"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	value, err := s.PreviewProjectFeedback(r.Context(), actor, req.Source)
	if err != nil {
		feedbackProblem(w, err)
		return
	}
	journeyJSON(w, 200, value)
}
func (s *Server) importProjectFeedback(w http.ResponseWriter, r *http.Request) {
	actor, ok := feedbackRequestActor(w, r)
	if !ok {
		return
	}
	var req FeedbackImportRequest
	if !journeyDecode(w, r, &req) {
		return
	}
	value, err := s.ImportProjectFeedback(r.Context(), actor, req)
	if err != nil {
		feedbackProblem(w, err)
		return
	}
	journeyJSON(w, 200, value)
}
func (s *Server) selectProjectFeedback(w http.ResponseWriter, r *http.Request) {
	actor, ok := feedbackRequestActor(w, r)
	if !ok {
		return
	}
	var req struct {
		ExpectedRevision int      `json:"expectedRevision"`
		IDs              []string `json:"ids"`
	}
	if !journeyDecode(w, r, &req) {
		return
	}
	if req.IDs == nil {
		journeyError(w, 400, "feedback_selection_required", "Choose the exact saved observations to discuss.")
		return
	}
	value, err := s.SelectProjectFeedback(r.Context(), actor, req.ExpectedRevision, req.IDs)
	if err != nil {
		feedbackProblem(w, err)
		return
	}
	journeyJSON(w, 200, value)
}

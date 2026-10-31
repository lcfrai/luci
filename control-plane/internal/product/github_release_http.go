package product

import (
	"errors"
	"net/http"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func (s *Server) registerGitHubPublications(mux Registrar) {
	base := "/v1/product/projects/{id}/distribute/github-releases"
	mux.HandleFunc("GET "+base, s.handleGitHubPublications)
	mux.HandleFunc("GET "+base+"/{publicationId}", s.handleGitHubPublications)
	mux.HandleFunc("POST "+base+"/review", s.handleGitHubPublications)
	for _, action := range []string{"draft", "review", "publish", "observe"} {
		mux.HandleFunc("POST "+base+"/{publicationId}/"+action, s.handleGitHubPublications)
	}
}
func githubPublicationActor(r *http.Request, write bool) (management.ExecutionActorRef, error) {
	if write {
		return management.CaptureDeploymentActor(r.Context(), r.PathValue("id"))
	}
	user, _, ok := management.ActorFromContext(r.Context())
	if !ok || user.Disabled || user.ID == "" {
		return management.ExecutionActorRef{}, management.ErrExecutionAuthority
	}
	actor := management.ExecutionActorRef{UserID: user.ID, ProjectID: r.PathValue("id")}
	if team, scoped := management.TeamAccessFromContext(r.Context()); scoped {
		if team.ProjectID != actor.ProjectID || team.Member.UserID != user.ID || !team.Permissions.Read {
			return actor, management.ErrExecutionAuthority
		}
		actor.TeamID = team.Team.ID
	} else if management.TeamRuntimeIDFromContext(r.Context()) != "" {
		return actor, management.ErrExecutionAuthority
	}
	return actor, nil
}
func (s *Server) handleGitHubPublications(w http.ResponseWriter, r *http.Request) {
	if !artifactAccess(w, r, r.Method != http.MethodGet) {
		return
	}
	actor, err := githubPublicationActor(r, r.Method != http.MethodGet)
	if err != nil || actor.Local || actor.UserID == "" {
		journeyError(w, 403, "github_release_forbidden", "Current project publishing access is required.")
		return
	}
	if r.Method == http.MethodGet {
		if id := r.PathValue("publicationId"); id != "" {
			row, err := s.GitHubPublication(r.Context(), actor, id)
			if err != nil {
				journeyError(w, 409, "github_release_unavailable", errGitHubRelease.Error())
				return
			}
			journeyJSON(w, 200, row)
			return
		}
		if s.feedbackAccess(r.Context(), actor, false) != nil {
			journeyError(w, 403, "github_release_forbidden", "Current project read access is required.")
			return
		}
		p, err := s.resolveProject(actor.ProjectID)
		if err != nil {
			artifactProblem(w, err)
			return
		}
		s.mu.RLock()
		s.githubReleasesMu.Lock()
		_, index, err := s.readGitHubPublicationIndex(p, actor)
		rows := []GitHubPublication{}
		if err == nil {
			for _, id := range index.IDs {
				row, readErr := s.readGitHubPublication(p, actor, id)
				if readErr != nil {
					err = readErr
					break
				}
				rows = append(rows, row.GitHubPublication)
			}
		}
		s.githubReleasesMu.Unlock()
		s.mu.RUnlock()
		if err != nil {
			journeyError(w, 409, "github_release_unavailable", errGitHubRelease.Error())
			return
		}
		if s.feedbackAccess(r.Context(), actor, false) != nil {
			journeyError(w, 403, "github_release_forbidden", "Current project read access is required.")
			return
		}
		journeyJSON(w, 200, map[string]any{"projectId": p.ID, "publications": rows, "earlierAvailable": index.EarlierAvailable})
		return
	}
	var row GitHubPublication
	if r.Pattern == "POST /v1/product/projects/{id}/distribute/github-releases/review" {
		var input GitHubPublicationReview
		if !journeyDecode(w, r, &input) {
			return
		}
		row, err = s.PrepareGitHubPublication(r.Context(), actor, input)
	} else {
		var input GitHubPublicationAction
		if !journeyDecode(w, r, &input) {
			return
		}
		parts := strings.Split(r.URL.Path, "/")
		action := parts[len(parts)-1]
		row, err = s.ActGitHubPublication(r.Context(), actor, r.PathValue("publicationId"), action, input)
	}
	if err != nil {
		status := 409
		if errors.Is(err, management.ErrExecutionAuthority) {
			status = 403
		}
		message := errGitHubRelease.Error()
		var notice githubPublicationNotice
		if errors.As(err, &notice) {
			message = notice.Error()
		}
		journeyError(w, status, "github_release_changed", message)
		return
	}
	journeyJSON(w, 200, row)
}

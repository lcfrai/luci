package api

import (
	"net/http"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/httpx"
)

func (a *API) validateSwarmWorktreeSession(w http.ResponseWriter, r *http.Request, projectID string, req sessionCreateRequest) bool {
	if a.deps.Product == nil || projectID == "" {
		return true
	}
	runID, operationID, err := a.deps.Product.ProjectSwarmWorktreeOwner(r.Context(), projectID, req.WorktreeID)
	if err != nil {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "The task checkout ownership could not be verified. Refresh its project run before starting a session.", nil)
		return false
	}
	if runID == "" {
		return true
	}
	if req.ProjectRunID != runID || req.ProjectRunOperationID != operationID || req.AuthOnly || strings.TrimSpace(req.Resume) != "" {
		httpx.WriteProblem(w, http.StatusConflict, httpx.CodeConflict, "This isolated task checkout belongs to its reserved run operation. Start or inspect that exact task from the project run; a manual session or fork cannot use it.", nil)
		return false
	}
	return true
}

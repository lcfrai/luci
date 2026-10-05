package api

import (
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/httpx"
)

func (a *API) sessionBranchAdmission(w http.ResponseWriter, r *http.Request, worktreeID, expectedBranchID string) (func(), bool) {
	if a.deps.Product == nil {
		return func() {}, true
	}
	projectID, _ := a.sessionLocate(r.Context(), worktreeID)
	if projectID == "" {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, "The session's project could not be checked. Refresh the workspace before starting a session.", nil)
		return nil, false
	}
	release, err := a.deps.Product.AcquireGitSessionAdmission(r.Context(), projectID)
	if err != nil {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, err.Error(), nil)
		return nil, false
	}
	if expectedBranchID != "" {
		if err := a.deps.Product.ValidateSessionBranch(r.Context(), projectID, worktreeID, expectedBranchID); err != nil {
			release()
			httpx.WriteProblem(w, 409, httpx.CodeConflict, err.Error(), nil)
			return nil, false
		}
	}
	return release, true
}

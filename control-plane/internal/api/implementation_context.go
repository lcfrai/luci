package api

import (
	"net/http"
	"reflect"
	"sort"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func sameImplementationSelection(a, b *product.ImplementationContextRequest) bool {
	x, y := *a, *b
	x.TaskIDs = append([]string{}, a.TaskIDs...)
	y.TaskIDs = append([]string{}, b.TaskIDs...)
	sort.Strings(x.TaskIDs)
	sort.Strings(y.TaskIDs)
	return reflect.DeepEqual(x, y)
}

func (a *API) validateSessionImplementation(w http.ResponseWriter, projectID, worktreeID, path string, c *registry.ImplementationContext) bool {
	if c == nil {
		return true
	}
	if a.deps.Product == nil {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, "Reviewed task runs are unavailable in this workspace.", nil)
		return false
	}
	if err := a.deps.Product.ValidateImplementationContext(projectID, worktreeID, path, c); err != nil {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, err.Error(), nil)
		return false
	}
	return true
}

func (a *API) bindSessionImplementation(w http.ResponseWriter, r *http.Request, req *sessionCreateRequest, path string, source registry.Session) (*registry.ImplementationContext, bool) {
	selection := req.ImplementationContext
	if source.ImplementationContext != nil {
		if source.WorktreeID != req.WorktreeID {
			httpx.WriteProblem(w, 409, httpx.CodeConflict, "A reviewed task cannot be moved into a different worktree by resuming its conversation. Start a new run from the matching plan.", nil)
			return nil, false
		}
		if !a.validateSessionImplementation(w, source.ProjectID, req.WorktreeID, path, source.ImplementationContext) {
			return nil, false
		}
		original := product.ImplementationContextSelection(source.ImplementationContext)
		if selection != nil && !sameImplementationSelection(selection, original) {
			httpx.WriteProblem(w, 409, httpx.CodeConflict, "Resuming retains the original reviewed tasks. Start a new session to work on a different plan.", nil)
			return nil, false
		}
		selection = original
	}
	if selection == nil {
		return nil, true
	}
	if a.deps.Product == nil || req.AuthOnly || strings.TrimSpace(req.Harness) == "shell" {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, "Choose a coding agent in the project workspace for a reviewed implementation task.", nil)
		return nil, false
	}
	projectID, _ := a.sessionLocate(r.Context(), req.WorktreeID)
	if selection.ProjectID != projectID {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, "The selected task belongs to a different project. Open its saved plan to start the run.", nil)
		return nil, false
	}
	bound, err := a.deps.Product.BindImplementationContext(r.Context(), *selection, req.WorktreeID, path)
	if err != nil {
		httpx.WriteProblem(w, 409, httpx.CodeConflict, err.Error(), nil)
		return nil, false
	}
	prompt := product.ImplementationOpeningPrompt(bound)
	if strings.TrimSpace(req.OpeningPrompt) != "" {
		prompt += "\n\nAdditional direction supplied for this run:\n" + req.OpeningPrompt
	}
	req.OpeningPrompt = prompt
	return bound, true
}

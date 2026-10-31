package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// handleDeploymentBlueprint is a read-authorized, plan-only POST. The guarded
// API applies the usual origin/identity middleware. No plan or manifest is
// saved, and no provider process, network request or model is started here.
func (a *API) handleDeploymentBlueprint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	actor, _, ok := management.ActorFromContext(r.Context())
	if !ok || actor.Disabled || actor.ID == "" {
		httpx.WriteProblem(w, 403, "blueprint_forbidden", "Current project access is required.", nil)
		return
	}
	if a.deps.Product == nil {
		httpx.WriteProblem(w, 503, "blueprint_unavailable", "Deployment planning is unavailable.", nil)
		return
	}
	id := r.PathValue("id")
	teamID := management.TeamRuntimeIDFromContext(r.Context())
	authorize := func() bool {
		if a.deps.AuthorizeProjectRead == nil {
			return teamID == ""
		}
		_, err := a.deps.AuthorizeProjectRead(r.Context(), management.ExecutionActorRef{UserID: actor.ID, TeamID: teamID, ProjectID: id})
		return err == nil
	}
	if id == "" || !authorize() {
		httpx.WriteProblem(w, 403, "blueprint_forbidden", "Current project access is required.", nil)
		return
	}
	if _, err := a.deps.Product.ResolveProject(id); err != nil {
		httpx.WriteProblem(w, 404, "project_not_found", "The requested project is unavailable.", nil)
		return
	}
	var request deploymentplan.Blueprint
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		httpx.WriteProblem(w, 400, "blueprint_invalid", "Provide a versioned blueprint within 128 KiB using known fields.", nil)
		return
	}
	if err := decoder.Decode(&struct{}{}); err != io.EOF {
		httpx.WriteProblem(w, 400, "blueprint_invalid", "Provide exactly one JSON blueprint.", nil)
		return
	}
	plan, err := a.deps.Product.DeploymentBlueprint(r.Context(), id, request)
	if err != nil {
		httpx.WriteProblem(w, 422, "blueprint_invalid", err.Error(), nil)
		return
	}
	if r.Context().Err() != nil || !authorize() {
		httpx.WriteProblem(w, 403, "blueprint_forbidden", "Project access changed while planning.", nil)
		return
	}
	httpx.WriteJSON(w, 200, plan)
}

// Saved blueprints are project editing, not Deploy or Execute permission. Each
// storage read/write resolves grants again; retained actor context is no grant.
func (a *API) blueprintStorageAccess(r *http.Request) product.BlueprintAccess {
	return func(ctx context.Context, id string, edit bool) error {
		actor, _, ok := management.ActorFromContext(ctx)
		if ctx.Err() != nil || !ok || actor.Disabled || actor.ID == "" || id != r.PathValue("id") {
			return management.ErrExecutionAuthority
		}
		teamID := management.TeamRuntimeIDFromContext(ctx)
		if access, scoped := management.TeamAccessFromContext(ctx); scoped {
			if access.ProjectID != id || !access.Permissions.Read || edit && !access.Permissions.Edit {
				return management.ErrExecutionAuthority
			}
			teamID = access.Team.ID
		}
		if a.deps.AuthorizeProjectRead == nil {
			if teamID != "" || a.deps.ManagedExecution {
				return management.ErrExecutionAuthority
			}
			return nil
		}
		authority, err := a.deps.AuthorizeProjectRead(ctx, management.ExecutionActorRef{UserID: actor.ID, TeamID: teamID, ProjectID: id})
		if err != nil || authority.Actor.ID != actor.ID || authority.Actor.Disabled {
			return management.ErrExecutionAuthority
		}
		if teamID != "" && (authority.Team == nil || authority.Team.Team.ID != teamID || authority.Team.ProjectID != id || !authority.Team.Permissions.Read || edit && !authority.Team.Permissions.Edit) {
			return management.ErrExecutionAuthority
		}
		return nil
	}
}

func (a *API) handleSavedDeploymentBlueprint(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	access := a.blueprintStorageAccess(r)
	id, edit := r.PathValue("id"), r.Method == http.MethodPut
	if err := access(r.Context(), id, edit); err != nil {
		httpx.WriteProblem(w, 403, "blueprint_forbidden", "Current project read or edit access is required.", nil)
		return
	}
	if a.deps.Product == nil {
		httpx.WriteProblem(w, 503, "blueprint_unavailable", "Saved deployment planning is unavailable.", nil)
		return
	}
	var view product.DeploymentBlueprintView
	var err error
	if edit {
		var request product.DeploymentBlueprintSave
		decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 136<<10))
		decoder.DisallowUnknownFields()
		if decoder.Decode(&request) != nil || decoder.Decode(&struct{}{}) != io.EOF {
			httpx.WriteProblem(w, 400, "blueprint_invalid", "Provide one bounded saved blueprint request using known fields.", nil)
			return
		}
		view, err = a.deps.Product.SaveDeploymentBlueprint(r.Context(), id, request, access)
	} else {
		view, err = a.deps.Product.ReadDeploymentBlueprint(r.Context(), id, access)
	}
	if err != nil {
		status, code := 422, "blueprint_invalid"
		if errors.Is(err, management.ErrExecutionAuthority) {
			status, code = 403, "blueprint_forbidden"
		}
		if errors.Is(err, product.ErrBlueprintConflict) || errors.Is(err, product.ErrBlueprintContextChanged) {
			status, code = 409, "blueprint_changed"
		}
		httpx.WriteProblem(w, status, code, err.Error(), nil)
		return
	}
	if err := access(r.Context(), id, edit); err != nil {
		httpx.WriteProblem(w, 403, "blueprint_forbidden", "Project access changed. Any already saved revision remains retained.", nil)
		return
	}
	httpx.WriteJSON(w, 200, view)
}

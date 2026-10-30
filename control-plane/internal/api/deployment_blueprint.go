package api

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/deploymentplan"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/management"
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

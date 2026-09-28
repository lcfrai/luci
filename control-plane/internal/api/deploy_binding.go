package api

// The one route the binding is written by. There is no GET of its own: the
// view (task 08) carries the same object, and two ways to read one fact drift
// the first time one of them is changed alone.

import (
	"errors"
	"net/http"

	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/deploy/binding"
	"github.com/kpmg-au-emu/panacea-app/control-plane/internal/httpx"
)

// bindingBody is the wire shape. HubMiss is absent by construction - the field
// is tagged json:"-" on binding.Binding, and this struct never names it - and
// TestHubMissIsNotOnTheBindingWire looks for the key rather than the field, so
// a struct that grew one would still be caught.
type bindingBody struct {
	ProjectID      string          `json:"projectId"`
	RepoKey        *string         `json:"repoKey"`
	HubProjectID   *string         `json:"hubProjectId"`
	PanaceaProject *string         `json:"panaceaProject"`
	State          string          `json:"state"`
	ChosenBy       *string         `json:"chosenBy"`
	Candidates     []candidateBody `json:"candidates"`
	Message        string          `json:"message"`
}

type candidateBody struct {
	Ref         string `json:"ref"`
	DisplayName string `json:"displayName"`
	RepoURL     string `json:"repoUrl"`
}

func bindingBodyOf(b binding.Binding) bindingBody {
	body := bindingBody{
		ProjectID:      b.ProjectID,
		RepoKey:        nilIfEmpty(b.RepoKey),
		HubProjectID:   nilIfEmpty(b.HubProjectID),
		PanaceaProject: nilIfEmpty(string(b.Panacea)),
		State:          string(b.State),
		ChosenBy:       nilIfEmpty(b.ChosenBy),
		Candidates:     []candidateBody{},
		Message:        b.Message,
	}
	for _, c := range b.Candidates {
		body.Candidates = append(body.Candidates, candidateBody{
			Ref: string(c.Ref), DisplayName: c.DisplayName, RepoURL: c.RepoURL,
		})
	}
	return body
}

type bindingRequest struct {
	PanaceaProject string `json:"panaceaProject"`
}

func (a *API) deployBindingRoutes() []route {
	return []route{
		{"PUT /v1/projects/{projectId}/deploy/binding", http.HandlerFunc(a.handleDeployBinding)},
	}
}

func (a *API) registerDeployBinding(mux *routes) {
	for _, rt := range a.deployBindingRoutes() {
		mux.Handle(rt.Pattern, rt.Handler)
	}
}

// handleDeployBinding records the user's choice between the Panacea projects
// that deploy one repository, and answers the recomputed binding.
func (a *API) handleDeployBinding(w http.ResponseWriter, r *http.Request) {
	const op = "deploy binding"
	if a.deps.DeployBinding == nil {
		a.writeDeployError(w, r, op, deploy.ErrNotConnected)
		return
	}
	projectID := r.PathValue("projectId")
	var req bindingRequest
	if !httpx.DecodeJSON(w, r, &req) {
		return
	}
	if req.PanaceaProject == "" {
		badRequest(w, "Name the Panacea project this repository deploys to.")
		return
	}

	b, err := a.deps.DeployBinding.Choose(r.Context(), projectID, deploy.ProjectRef(req.PanaceaProject))
	if errors.Is(err, binding.ErrUnknownProject) {
		projectNotFound(w, projectID)
		return
	}
	if err != nil {
		a.writeDeployError(w, r, op, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, bindingBodyOf(b))
}

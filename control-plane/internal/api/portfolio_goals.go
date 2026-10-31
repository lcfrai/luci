package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// The API owns the current identity and project catalogue, never a browser's
// submitted permission flags. Queue writes use the EXISTING control authorizer.
func (a *API) portfolioGoalAccess(r *http.Request) (product.PortfolioGoalAccess, error) {
	projects, actorID, err := a.authorizedHomeProjects(r.Context())
	if err != nil {
		return nil, err
	}
	selected := map[string]bool{}
	for _, p := range projects {
		selected[p.ID] = true
	}
	teamID := management.TeamRuntimeIDFromContext(r.Context())
	return func(ctx context.Context, projectID string, control bool) error {
		actor, _, ok := management.ActorFromContext(ctx)
		if !ok || actor.Disabled || actor.ID != actorID || !selected[projectID] {
			return management.ErrExecutionAuthority
		}
		if err := a.reauthorizeHomeProject(ctx, actorID, projectID); err != nil {
			return err
		}
		if control {
			if a.deps.AuthorizeExecutionControl == nil {
				return management.ErrExecutionAuthority
			}
			return a.deps.AuthorizeExecutionControl(ctx, management.ExecutionActorRef{UserID: actorID, TeamID: teamID, ProjectID: projectID})
		}
		return ctx.Err()
	}, nil
}

func portfolioGoalsDecode(w http.ResponseWriter, r *http.Request, value any) bool {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(value); err == nil {
		if decoder.Decode(&struct{}{}) == io.EOF {
			return true
		}
	}
	httpx.WriteProblem(w, 400, "portfolio_goal_invalid", "Provide one bounded portfolio request using known fields.", nil)
	return false
}

func portfolioGoalProblem(w http.ResponseWriter, err error, mayHaveChanged bool) {
	status, code, message := 503, "portfolio_goal_unavailable", "Your private goal or queue result could not be verified."
	switch {
	case errors.Is(err, management.ErrExecutionAuthority):
		status, code, message = 403, "portfolio_goal_forbidden", "Current project access and the existing queue-control permission are required."
	case errors.Is(err, product.ErrPortfolioGoalInvalid):
		status, code, message = 400, "portfolio_goal_invalid", product.ErrPortfolioGoalInvalid.Error()
	case errors.Is(err, product.ErrPortfolioGoalConflict):
		status, code, message = 409, "portfolio_goal_changed", product.ErrPortfolioGoalConflict.Error()
	}
	if mayHaveChanged {
		message += " Some reviewed priority writes may already be saved. Read the retained application before attempting anything else."
	}
	httpx.WriteProblem(w, status, code, message, nil)
}

func portfolioFocusProblem(w http.ResponseWriter, err error) {
	status, code := 503, "portfolio_focus_unavailable"
	if errors.Is(err, management.ErrExecutionAuthority) {
		status, code = 403, "portfolio_focus_forbidden"
	}
	if errors.Is(err, product.ErrPortfolioGoalInvalid) {
		status, code = 400, "portfolio_focus_invalid"
	}
	if errors.Is(err, product.ErrPortfolioGoalConflict) {
		status, code = 409, "portfolio_focus_changed"
	}
	httpx.WriteProblem(w, status, code, "The focus policy change could not be confirmed. Current project control is required. A policy may already have been saved; read latest saved goals before a new review. No running agent was stopped and no work was started by this request.", nil)
}

func (a *API) handlePortfolioGoals(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if a.deps.Product == nil {
		portfolioGoalProblem(w, product.ErrPortfolioGoalUnavailable, false)
		return
	}
	access, err := a.portfolioGoalAccess(r)
	if err != nil {
		portfolioGoalProblem(w, err, false)
		return
	}
	var value any
	if r.Method == http.MethodPost {
		var request product.PortfolioGoalsSave
		if !portfolioGoalsDecode(w, r, &request) {
			return
		}
		value, err = a.deps.Product.SavePortfolioGoal(r.Context(), request, access)
	} else {
		value, err = a.deps.Product.ReadPortfolioGoalsWithOutcomes(r.Context(), access)
	}
	if err != nil {
		goal, _ := value.(product.PortfolioGoal)
		portfolioGoalProblem(w, err, r.Method == http.MethodPost && goal.ID != "")
		return
	}
	if r.Context().Err() == nil {
		httpx.WriteJSON(w, 200, value)
	}
}

func (a *API) handlePortfolioGoalReview(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if a.deps.Product == nil {
		portfolioGoalProblem(w, product.ErrPortfolioGoalUnavailable, false)
		return
	}
	access, err := a.portfolioGoalAccess(r)
	if err != nil {
		portfolioGoalProblem(w, err, false)
		return
	}
	var request struct {
		ExpectedGoalRevision int                           `json:"expectedGoalRevision"`
		Mode                 string                        `json:"mode,omitempty"`
		Focus                product.PortfolioFocusRequest `json:"focus,omitempty"`
	}
	if !portfolioGoalsDecode(w, r, &request) {
		return
	}
	var plan any
	if request.Mode == "focus" {
		plan, err = a.deps.Product.ReviewPortfolioFocus(r.Context(), r.PathValue("goalId"), request.ExpectedGoalRevision, request.Focus, access)
	} else if request.Mode == "" && request.Focus.ExpectedRevision == 0 && request.Focus.ExpectedProfileDigest == "" && len(request.Focus.UpstreamRuns) == 0 {
		plan, err = a.deps.Product.ReviewPortfolioGoal(r.Context(), r.PathValue("goalId"), request.ExpectedGoalRevision, access)
	} else {
		err = product.ErrPortfolioGoalInvalid
	}
	if err != nil {
		portfolioGoalProblem(w, err, false)
		return
	}
	if r.Context().Err() == nil {
		httpx.WriteJSON(w, 200, plan)
	}
}

func (a *API) handlePortfolioGoalApply(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if a.deps.Product == nil {
		portfolioGoalProblem(w, product.ErrPortfolioGoalUnavailable, false)
		return
	}
	access, err := a.portfolioGoalAccess(r)
	if err != nil {
		portfolioGoalProblem(w, err, false)
		return
	}
	var request product.PortfolioGoalApply
	if !portfolioGoalsDecode(w, r, &request) {
		return
	}
	if request.GoalID != r.PathValue("goalId") {
		portfolioGoalProblem(w, product.ErrPortfolioGoalInvalid, false)
		return
	}
	if request.Mode == "focus-install" || request.Mode == "focus-remove" {
		focus, err := a.deps.Product.ApplyPortfolioFocus(r.Context(), request, access)
		if err != nil {
			portfolioFocusProblem(w, err)
			return
		}
		// Readback rechecks all retained profile projects after the atomic save.
		latest, err := a.deps.Product.ReadPortfolioFocus(r.Context(), access)
		if err != nil || latest.Revision != focus.Revision {
			portfolioFocusProblem(w, product.ErrPortfolioGoalUnavailable)
			return
		}
		httpx.WriteJSON(w, 200, map[string]any{"focus": latest})
		return
	}
	application, err := a.deps.Product.ApplyPortfolioGoal(r.Context(), request, access)
	if err != nil {
		portfolioGoalProblem(w, err, application.ID != "")
		return
	}
	// The worker service also rechecks each project immediately before writes.
	// A late grant loss hides response content without claiming rollback.
	for _, projectID := range application.ProjectIDs {
		if err := access(r.Context(), projectID, false); err != nil {
			portfolioGoalProblem(w, err, application.ID != "")
			return
		}
	}
	if r.Context().Err() == nil {
		httpx.WriteJSON(w, 200, application)
	}
}

func (a *API) handlePortfolioGoalApplication(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if a.deps.Product == nil {
		portfolioGoalProblem(w, product.ErrPortfolioGoalUnavailable, false)
		return
	}
	access, err := a.portfolioGoalAccess(r)
	if err != nil {
		portfolioGoalProblem(w, err, false)
		return
	}
	application, err := a.deps.Product.ReadPortfolioApplication(r.Context(), r.PathValue("applicationId"), access)
	if err != nil {
		portfolioGoalProblem(w, err, false)
		return
	}
	if r.Context().Err() == nil {
		httpx.WriteJSON(w, 200, application)
	}
}

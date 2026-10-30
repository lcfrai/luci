package api

import (
	"net/http"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func (a *API) handlePortfolio(w http.ResponseWriter, r *http.Request) {
	if a.deps.Product == nil {
		httpx.WriteProblem(w, 503, "portfolio_unavailable", "Project attention is unavailable.", nil)
		return
	}
	projects, actor, err := a.authorizedHomeProjects(r.Context())
	if err != nil {
		httpx.WriteProblem(w, 503, "portfolio_unavailable", "Current project access could not be checked.", nil)
		return
	}
	total := len(projects)
	projects = projects[:min(len(projects), product.PortfolioProjectLimit)]
	rows := []product.PortfolioProject{}
	items := []product.PortfolioIntervention{}
	for _, p := range projects {
		if err := r.Context().Err(); err != nil {
			return
		}
		if a.reauthorizeHomeProject(r.Context(), actor, p.ID) != nil {
			httpx.WriteProblem(w, 403, "project_unavailable", "Project access changed. Refresh your workspace.", nil)
			return
		}
		row, attention, err := a.deps.Product.ReadPortfolioProject(r.Context(), p, actor)
		if err != nil {
			if r.Context().Err() == nil {
				httpx.WriteProblem(w, 503, "portfolio_unavailable", "Project attention could not be read.", nil)
			}
			return
		}
		rows = append(rows, row)
		items = append(items, attention...)
	}
	// A grant may disappear while another project's bounded records are read.
	// Refuse the entire snapshot rather than retaining newly revoked content.
	for _, p := range projects {
		if a.reauthorizeHomeProject(r.Context(), actor, p.ID) != nil {
			httpx.WriteProblem(w, 403, "project_unavailable", "Project access changed. Refresh your workspace.", nil)
			return
		}
	}
	view, err := product.AssemblePortfolio(rows, items, total, time.Now())
	if err != nil {
		httpx.WriteProblem(w, 503, "portfolio_unavailable", "Project attention could not be summarized.", nil)
		return
	}
	if r.Context().Err() == nil {
		httpx.WriteJSON(w, http.StatusOK, view)
	}
}

package product

import (
	"errors"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func (s *Server) registerOutcomeCosts(mux Registrar) {
	const path = "/v1/product/projects/{id}/runs/outcome-costs"
	mux.HandleFunc("GET "+path, s.handleOutcomeCosts)
	mux.HandleFunc("POST "+path, s.handleOutcomeCosts)
}
func (s *Server) handleOutcomeCosts(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	if !artifactAccess(w, r, r.Method != http.MethodGet && r.Method != http.MethodHead) {
		return
	}
	// Reuse the existing authenticated current-project actor capture. No GitHub
	// request or publishing authority is used; financial access is checked below.
	actor, err := githubPublicationActor(r, false)
	if err != nil {
		journeyError(w, 403, "outcome_costs_forbidden", "Current project Edit and Secrets access is required.")
		return
	}
	var value OutcomeCosts
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		value, err = s.OutcomeCosts(r.Context(), actor)
	} else {
		var input OutcomeCostSave
		if !journeyDecode(w, r, &input) {
			return
		}
		value, err = s.SaveOutcomeCost(r.Context(), actor, input)
	}
	if err != nil {
		status := 409
		if errors.Is(err, management.ErrExecutionAuthority) {
			status = 403
		}
		journeyError(w, status, "outcome_costs_changed", "The original operation, current access or saved reconciliation revision could not be confirmed. Read the retained receipts before another save; no provider charge was changed.")
		return
	}
	journeyJSON(w, 200, value)
}

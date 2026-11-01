package product

import (
	"errors"
	"net/http"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

func (s *Server) registerCommercialSignals(mux Registrar) {
	base := "/v1/product/projects/{id}/distribute/commercial-signals"
	mux.HandleFunc("GET "+base, s.getCommercialSignals)
	mux.HandleFunc("GET "+base+"/release-choices", s.getCommercialReleaseChoices)
	mux.HandleFunc("PUT "+base+"/stripe", s.saveCommercialStripe)
	mux.HandleFunc("PUT "+base+"/sources", s.saveCommercialSources)
	mux.HandleFunc("POST "+base+"/refresh", s.refreshCommercialSignals)
}
func commercialProblem(w http.ResponseWriter, err error) {
	if errors.Is(err, errCommercialStripeAccess) || errors.Is(err, errCommercialStripeRate) {
		journeyError(w, 422, "commercial_provider_read", err.Error())
		return
	}
	if errors.Is(err, management.ErrExecutionAuthority) {
		journeyError(w, 403, "commercial_access", "Current project Read, Edit and Secrets access is required for provider reads. Saved aggregates remain project-readable.")
		return
	}
	if errors.Is(err, errCommercialWindowLimit) {
		journeyError(w, 422, "commercial_window_limit", errCommercialWindowLimit.Error())
		return
	}
	journeyError(w, 409, "commercial_unconfirmed", ErrCommercial.Error())
}
func (s *Server) getCommercialSignals(w http.ResponseWriter, r *http.Request) {
	actor, ok := feedbackRequestActor(w, r)
	if !ok {
		return
	}
	view, err := s.CommercialSignals(r.Context(), actor)
	if err != nil {
		commercialProblem(w, err)
		return
	}
	journeyJSON(w, 200, view)
}
func (s *Server) getCommercialReleaseChoices(w http.ResponseWriter, r *http.Request) {
	actor, ok := feedbackRequestActor(w, r)
	if !ok {
		return
	}
	rows, err := s.CommercialReleaseChoices(r.Context(), actor)
	if err != nil {
		commercialProblem(w, err)
		return
	}
	journeyJSON(w, 200, map[string]any{"projectId": actor.ProjectID, "releases": rows, "retainedRecentOnly": true})
}
func (s *Server) saveCommercialStripe(w http.ResponseWriter, r *http.Request) {
	actor, ok := feedbackRequestActor(w, r)
	if !ok {
		return
	}
	var input CommercialStripeSave
	if !journeyDecode(w, r, &input) {
		return
	}
	view, err := s.SaveCommercialStripe(r.Context(), actor, input)
	if err != nil {
		commercialProblem(w, err)
		return
	}
	journeyJSON(w, 200, view)
}
func (s *Server) saveCommercialSources(w http.ResponseWriter, r *http.Request) {
	actor, ok := feedbackRequestActor(w, r)
	if !ok {
		return
	}
	var input CommercialConfigurationSave
	if !journeyDecode(w, r, &input) {
		return
	}
	view, err := s.SaveCommercialConfiguration(r.Context(), actor, input)
	if err != nil {
		commercialProblem(w, err)
		return
	}
	journeyJSON(w, 200, view)
}
func (s *Server) refreshCommercialSignals(w http.ResponseWriter, r *http.Request) {
	actor, ok := feedbackRequestActor(w, r)
	if !ok {
		return
	}
	var input CommercialRefreshRequest
	if !journeyDecode(w, r, &input) {
		return
	}
	view, err := s.RefreshCommercialSignals(r.Context(), actor, input)
	if err != nil {
		commercialProblem(w, err)
		return
	}
	journeyJSON(w, 200, view)
}

package product

import (
	"encoding/json"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"io"
	"net/http"
)

func (s *Server) registerPublicHTTPS(mux Registrar) {
	base := "/v1/product/projects/{id}/deploy/public-https"
	mux.HandleFunc("GET "+base, s.handlePublicHTTPS)
	mux.HandleFunc("GET "+base+"/{commissionId}", s.handlePublicHTTPS)
	mux.HandleFunc("POST "+base+"/reviews", s.handlePublicHTTPS)
	mux.HandleFunc("POST "+base+"/{commissionId}/action", s.handlePublicHTTPS)
}
func publicHTTPSDecode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 16<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if d.Decode(v) != nil || !errors.Is(d.Decode(new(any)), io.EOF) {
		journeyError(w, 400, "public_https_input", ErrPublicHTTPSInput.Error())
		return false
	}
	return true
}
func publicHTTPSProblem(w http.ResponseWriter, e error) {
	switch {
	case errors.Is(e, management.ErrExecutionAuthority):
		journeyError(w, 403, "public_https_forbidden", "Current project Read, Edit, Deploy and Secrets authority is required for this action; public creation/resume also needs publishing and provisioning policy.")
	case errors.Is(e, ErrPublicHTTPSDomain):
		journeyError(w, 422, "public_https_domain", e.Error())
	case errors.Is(e, ErrPublicHTTPSStorage):
		journeyError(w, 422, "public_https_storage", e.Error())
	case errors.Is(e, ErrPublicHTTPSApplication):
		journeyError(w, 422, "public_https_application", e.Error())
	case errors.Is(e, ErrPublicHTTPSInput):
		journeyError(w, 422, "public_https_input", e.Error())
	case errors.Is(e, ErrPublicHTTPSCapacity):
		journeyError(w, 429, "public_https_capacity", e.Error())
	default:
		journeyError(w, 409, "public_https_unconfirmed", ErrPublicHTTPS.Error())
	}
}
func (s *Server) handlePublicHTTPS(w http.ResponseWriter, r *http.Request) {
	if !artifactAccess(w, r, r.Method != http.MethodGet) {
		return
	}
	actor, e := githubPublicationActor(r, r.Method != http.MethodGet)
	if e != nil || actor.Local || actor.UserID == "" {
		publicHTTPSProblem(w, management.ErrExecutionAuthority)
		return
	}
	if r.Method == http.MethodGet {
		if id := r.PathValue("commissionId"); id != "" {
			row, e := s.PublicHTTPSGet(r.Context(), actor, id)
			if e != nil {
				publicHTTPSProblem(w, e)
				return
			}
			journeyJSON(w, 200, row)
			return
		}
		if len(r.URL.Query()) > 1 || r.URL.Query().Has("reviewKey") && len(r.URL.Query()["reviewKey"]) != 1 {
			publicHTTPSProblem(w, ErrPublicHTTPS)
			return
		}
		for key := range r.URL.Query() {
			if key != "reviewKey" {
				publicHTTPSProblem(w, ErrPublicHTTPS)
				return
			}
		}
		key := r.URL.Query().Get("reviewKey")
		rows, e := s.PublicHTTPSList(r.Context(), actor, key)
		if e != nil {
			publicHTTPSProblem(w, e)
			return
		}
		journeyJSON(w, 200, map[string]any{"projectId": actor.ProjectID, "commissions": rows, "retainedRecentOnly": key == ""})
		return
	}
	var status PublicHTTPSStatus
	if r.PathValue("commissionId") == "" {
		var input PublicHTTPSReviewInput
		if !publicHTTPSDecode(w, r, &input) {
			return
		}
		status, e = s.ReviewPublicHTTPS(r.Context(), actor, input)
	} else {
		var input PublicHTTPSActionInput
		if !publicHTTPSDecode(w, r, &input) {
			return
		}
		status, e = s.PublicHTTPSAction(r.Context(), actor, r.PathValue("commissionId"), input)
	}
	if e != nil {
		publicHTTPSProblem(w, e)
		return
	}
	journeyJSON(w, 200, status)
}

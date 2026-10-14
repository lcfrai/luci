package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"

	"github.com/lcfrai/luci/control-plane/internal/api"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// This route is installed only inside a token-bound managed worker. The
// external gateway denies the path, including its /api alias.
func workerHostPressure(w http.ResponseWriter, r *http.Request, a *api.API) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	switch r.Method {
	case http.MethodGet:
		total, err := strconv.ParseUint(r.URL.Query().Get("total"), 10, 64)
		if err != nil || total == 0 {
			http.Error(w, "invalid host sample", http.StatusBadRequest)
			return
		}
		observation, err := a.HostPressureObservation(r.Context(), total)
		if err != nil {
			http.Error(w, "owned pressure state unavailable", http.StatusServiceUnavailable)
			return
		}
		_ = json.NewEncoder(w).Encode(observation)
	case http.MethodPost:
		data, err := io.ReadAll(io.LimitReader(r.Body, 4097))
		if err != nil || len(data) > 4096 {
			http.Error(w, "invalid exact stop request", http.StatusBadRequest)
			return
		}
		var candidate management.HostPressureCandidate
		if json.Unmarshal(data, &candidate) != nil {
			http.Error(w, "invalid exact stop request", http.StatusBadRequest)
			return
		}
		acted, stopErr := a.StopHostPressureCandidate(r.Context(), candidate)
		out := struct {
			Acted bool   `json:"acted"`
			Error string `json:"error,omitempty"`
		}{Acted: acted}
		if stopErr != nil {
			out.Error = "retained"
		}
		_ = json.NewEncoder(w).Encode(out)
	default:
		w.Header().Set("Allow", "GET, POST")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

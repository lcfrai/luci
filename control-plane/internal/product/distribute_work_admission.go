package product

import (
	"context"
	"net/http"
	"time"
)

// The managed API supplies one host-shared, cross-process permit. Standalone
// product fixtures use their local bounded inputs without a host runtime.
func (s *Server) SetDistributeWorkAdmission(acquire func(context.Context) (func(), error)) {
	s.distributeWorkAdmission = acquire
}

func (s *Server) boundedDistributeWork(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedContextActor(r.Context()); !ok {
			// Preserve each handler's ordinary authentication response without
			// letting anonymous requests occupy the host-shared compute permit.
			next(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		if s.distributeWorkAdmission != nil {
			release, err := s.distributeWorkAdmission(ctx)
			if err != nil {
				journeyError(w, http.StatusServiceUnavailable, "distribute_capacity_pending", "Workbook or artifact processing is waiting for computer capacity. Your saved work is unchanged; retry when capacity is available.")
				return
			}
			defer release()
		}
		next(w, r.WithContext(ctx))
	}
}

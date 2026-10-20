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

// PDF has a separate fixed charge. A missing PDF permit never borrows the
// smaller workbook/Office permit or the standalone fixture exception.
func (s *Server) SetDistributePDFWorkAdmission(acquire func(context.Context) (func(), error)) {
	s.distributePDFWorkAdmission = acquire
}

func (s *Server) boundedDistributeWork(next http.HandlerFunc) http.HandlerFunc {
	return s.boundedDistributeWorkClass(next, false)
}

func (s *Server) boundedDistributePDFWork(next http.HandlerFunc) http.HandlerFunc {
	return s.boundedDistributeWorkClass(next, true)
}

func (s *Server) boundedDistributeWorkClass(next http.HandlerFunc, pdf bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if _, ok := authenticatedContextActor(r.Context()); !ok {
			// Preserve each handler's ordinary authentication response without
			// letting anonymous requests occupy the host-shared compute permit.
			next(w, r)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		acquire := s.distributeWorkAdmission
		if pdf {
			acquire = s.distributePDFWorkAdmission
			if acquire == nil {
				journeyError(w, http.StatusServiceUnavailable, "distribute_pdf_capacity_unavailable", "PDF processing requires its dedicated host capacity permit. Your saved artifact is unchanged.")
				return
			}
		}
		if acquire != nil {
			release, err := acquire(ctx)
			if err != nil {
				journeyError(w, http.StatusServiceUnavailable, "distribute_capacity_pending", "Workbook or artifact processing is waiting for computer capacity. Your saved work is unchanged; retry when capacity is available.")
				return
			}
			defer release()
		}
		next(w, r.WithContext(ctx))
	}
}

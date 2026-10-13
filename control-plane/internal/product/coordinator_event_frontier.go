package product

import "github.com/lcfrai/luci/control-plane/internal/project"

// A later proposal or supersession changes the coordination head without
// changing accepted context. It cannot indefinitely stall already accepted
// work. A later APPLIED event does change the execution frontier and must be
// included in the exact continuation before source publication or reservation.
// Caller holds s.mu; archived pages are read by their verified references.
func (s *Server) latestAppliedCoordinationEventLocked(p project.Project, saved coordinationLedger) (string, error) {
	for i := len(saved.Events) - 1; i >= 0; i-- {
		if saved.Events[i].Kind == "applied" {
			return saved.Events[i].ID, nil
		}
	}
	for i := len(saved.Archives) - 1; i >= 0; i-- {
		page, err := s.readCoordinationArchive(p, saved, saved.Archives[i])
		if err != nil {
			return "", err
		}
		for j := len(page.Ledger.Events) - 1; j >= 0; j-- {
			if page.Ledger.Events[j].Kind == "applied" {
				return page.Ledger.Events[j].ID, nil
			}
		}
	}
	return "", nil
}

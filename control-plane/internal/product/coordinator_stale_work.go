package product

import (
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

// Old context may still have an owned worker completing or retained output to
// review. It cannot advance old checks, preview or delivery after the accepted
// canonical context changes. This is a durable no-replay block, not deletion
// of the run, worker or its scheduler charge.
func (s *Server) blockStaleCoordinatorRow(p project.Project, observed coordinatorContinuationRecord) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	rows, err := s.readContinuations(p, observed.Actor.ID)
	if err != nil {
		return false, err
	}
	i := findContinuation(rows, observed.ID)
	if i < 0 || rows.Rows[i].Revision != observed.Revision {
		return false, nil // next pass uses the authoritative current row
	}
	state, err := s.readApprovedJourney(p.Path, p.ID)
	if err != nil {
		return false, err
	}
	selection := rows.Rows[i].Run.ImplementationContext
	if selection.BriefRevision == state.Brief.Revision && selection.PlanRevision == currentPlan(state, p.ID).Revision &&
		selection.SpecificationRevision == currentSpecification(state).Revision && selection.DesignPlanRevision == currentDesignPlan(state).Revision {
		return false, nil
	}
	if actions := rows.Rows[i].Actions; len(actions) > 0 && (actions[len(actions)-1].Status == "pending" || actions[len(actions)-1].Status == "unknown") {
		// A lost external mutation response must retain its original pending
		// receipt for exact readback; a context edit cannot turn it into an
		// invented failure or cause a second publication.
		return true, nil
	}
	rows.Rows[i].Status = "blocked"
	rows.Rows[i].Message = "Accepted project context superseded this run. Its exact child and output remain retained; the coordinator will stop affected work and reserve a reviewed revision after settlement."
	rows.Rows[i].Revision++
	rows.Rows[i].UpdatedAt = s.schedulerNow().Format(time.RFC3339Nano)
	return true, s.saveContinuations(rows)
}

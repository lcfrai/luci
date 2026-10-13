package product

import "github.com/lcfrai/luci/control-plane/internal/project"

const orchestratorActivityLimit = 8

// The primary conversation reads the same durable project event log and
// actor-owned continuation ledger used by coordination and dispatch. This
// makes an accepted peer change and its resulting work visible after restart
// without appending synthetic human turns or claiming a provider outcome.
// The last archived page is read only if the active ledger has fewer than
// eight accepted updates; older history remains available in coordination.
func (s *Server) orchestratorActivityLocked(p project.Project, actorID string) (*OrchestratorActivity, error) {
	if actorID == "" {
		return nil, nil
	}
	coordination, err := s.readCoordination(p)
	if err != nil {
		return nil, err
	}
	continuations, err := s.readContinuations(p, actorID)
	if err != nil {
		return nil, err
	}
	out := &OrchestratorActivity{Accepted: []OrchestratorAcceptedUpdate{}, Work: []OrchestratorWorkUpdate{}}
	appendAccepted := func(events []CoordinationEvent, intents []storedCoordinationIntent) error {
		byID := make(map[string]CoordinationIntent, len(intents))
		for _, intent := range intents {
			byID[intent.ID] = intent.CoordinationIntent
		}
		for i := len(events) - 1; i >= 0; i-- {
			if events[i].Kind != "applied" {
				continue
			}
			if len(out.Accepted) >= orchestratorActivityLimit {
				out.EarlierAvailable = true
				return nil
			}
			intent, found := byID[events[i].IntentID]
			if !found || intent.Status != "applied" || intent.AppliedTargetDigest == "" {
				return ErrOrchestratorInvalid
			}
			out.Accepted = append(out.Accepted, OrchestratorAcceptedUpdate{
				EventID: events[i].ID, Sequence: events[i].Sequence, IntentID: intent.ID,
				OriginTurnID: intent.TurnID, Actor: intent.Actor, Target: intent.Target,
				AppliedTargetDigest: intent.AppliedTargetDigest, AcceptedContextDigest: intent.AcceptedContextDigest,
				TaskIDs: append([]string{}, intent.TaskIDs...), AffectsAllTasks: intent.AffectsAllTasks,
			})
		}
		return nil
	}
	if err = appendAccepted(coordination.Events, coordination.Intents); err != nil {
		return nil, err
	}
	if len(out.Accepted) < orchestratorActivityLimit && len(coordination.Archives) > 0 {
		last := coordination.Archives[len(coordination.Archives)-1]
		page, readErr := s.readCoordinationArchive(p, coordination, last)
		if readErr != nil {
			return nil, readErr
		}
		if err = appendAccepted(page.Ledger.Events, page.Ledger.Intents); err != nil {
			return nil, err
		}
		out.EarlierAvailable = len(coordination.Archives) > 1 || out.EarlierAvailable
	} else if len(coordination.Archives) > 0 {
		out.EarlierAvailable = true
	}
	for i := len(continuations.Rows) - 1; i >= 0; i-- {
		if len(out.Work) >= orchestratorActivityLimit {
			out.EarlierAvailable = true
			break
		}
		row := continuations.Rows[i]
		update := OrchestratorWorkUpdate{
			ID: row.ID, OriginTurnID: row.OriginTurnID, ContextDigest: row.ContextDigest,
			AcceptedEvents: append([]CoordinationEventReceipt{}, row.AcceptedEvents...),
			Status:         row.Status, Message: row.Message, RunID: row.RunID, UpdatedAt: row.UpdatedAt,
		}
		if delivery := row.Delivery; delivery != nil {
			finding := &OrchestratorWorkFinding{Status: delivery.Status, CheckID: delivery.CheckID, Commit: delivery.Commit}
			if delivery.Preview != nil {
				finding.PreviewStatus, finding.PreviewReleaseID = delivery.Preview.Status, delivery.Preview.ReleaseID
			}
			if delivery.Deployment != nil {
				finding.DeployStatus, finding.DeployReleaseID = delivery.Deployment.Status, delivery.Deployment.ReleaseID
			}
			update.Finding = finding
		}
		out.Work = append(out.Work, update)
	}
	return out, nil
}

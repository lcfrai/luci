package product

import (
	"context"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/project"
)

func (s *Server) continueCoordinatorRuntimeResources(ctx context.Context, p project.Project, row coordinatorContinuationRecord, preparation storedRuntimePreparation) (bool, error) {
	if preparation.Plan.Resources == nil {
		return true, nil
	}
	if preparation.Resources != nil && preparation.Resources.State == "ready" {
		return true, nil
	}
	key := row.ID + "-runtime-resources"
	s.mu.Lock()
	if s.coordinatorActions == nil {
		s.coordinatorActions = map[string]bool{}
	}
	live := s.coordinatorActions[key]
	if !live {
		s.coordinatorActions[key] = true
	}
	s.mu.Unlock()
	if live {
		return false, nil
	}
	original := preparation.Resources == nil
	bounded, cancel := context.WithTimeout(context.WithoutCancel(ctx), 30*time.Minute)
	stop := context.AfterFunc(s.workspaceContext, cancel)
	go func() {
		ready := false
		defer cancel()
		defer stop()
		defer func() {
			s.mu.Lock()
			delete(s.coordinatorActions, key)
			s.mu.Unlock()
			if original || ready {
				s.WakeCoordinator()
			}
		}()
		current, err := s.coordinatorKubernetesAuthority(bounded, p, row, original)
		if err != nil {
			return
		}
		var result RuntimePreparation
		if original {
			result, err = s.RunRuntimeResourcePreparation(current, row.ExecutionActor, preparation.Plan.ID, row.RunID, row.Delivery.CheckID)
		} else {
			result, err = s.ObserveRuntimeResourcePreparation(current, row.ExecutionActor, preparation.Plan.ID)
		}
		ready = err == nil && result.Resources != nil && result.Resources.State == "ready"
	}()
	return false, nil
}

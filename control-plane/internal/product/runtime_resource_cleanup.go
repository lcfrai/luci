package product

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Explicit cleanup is different from observation. It may delete only a
// positively completed original transfer Pod, never a volume/Secret, failed or
// running replacement, model upload, image build or missing resource intent.
func (s *Server) CleanupRuntimeResourcePreparation(ctx context.Context, actor management.ExecutionActorRef, id, digest string) (RuntimePreparation, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	ctx = WithKubernetesTargetResolution(ctx)
	if err := s.kubernetesAuthority(ctx, actor, false, true, 0); err != nil {
		return RuntimePreparation{}, err
	}
	_, row, err := s.readRuntimePreparation(actor, id)
	if err != nil {
		return RuntimePreparation{}, err
	}
	if row.Plan.Digest != digest || row.Plan.Resources == nil || row.Resources == nil {
		return row.RuntimePreparation, ErrRuntimePreparation
	}
	key := "runtime-resources-" + id
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
		return row.RuntimePreparation, ErrRuntimePreparation
	}
	defer func() { s.mu.Lock(); delete(s.coordinatorActions, key); s.mu.Unlock() }()
	w := &runtimeResourceWriter{server: s, row: row}
	for _, model := range row.Plan.Resources.Models {
		state := runtimeModelTransfer(w.snapshot().Resources, model.ServiceID)
		if state == nil || !state.OriginalZeroExit || state.Receipt == nil || state.Download == nil {
			return s.runtimeResourceResult(ctx, actor, w, ErrRuntimePreparation)
		}
		original := runtimeOwnedResource(w.snapshot().Resources, "Pod", state.PodName)
		if original == nil || original.UID != state.PodUID {
			return s.runtimeResourceResult(ctx, actor, w, ErrRuntimePreparation)
		}
		if original.AbsentObserved {
			continue
		}
		target, err := s.runtimeResourceTarget(ctx, w.snapshot(), false)
		if err != nil {
			return s.runtimeResourceResult(ctx, actor, w, err)
		}
		observed := s.observeRuntimeModel(ctx, w, target, model)
		if observed != nil && !errors.Is(observed, errRuntimeResourcePending) {
			return s.runtimeResourceResult(ctx, actor, w, observed)
		}
		original = runtimeOwnedResource(w.snapshot().Resources, "Pod", state.PodName)
		if original.AbsentObserved {
			continue
		}
		if original.DeleteAttempted {
			return s.runtimeResourceResult(ctx, actor, w, errRuntimeResourcePending)
		}
		if err = s.deleteRuntimeTransferPod(ctx, w, model); err != nil {
			return s.runtimeResourceResult(ctx, actor, w, err)
		}
	}
	if err = s.observeRuntimeResourceObjects(ctx, w); err != nil {
		return s.runtimeResourceResult(ctx, actor, w, err)
	}
	return s.runtimeResourceResult(ctx, actor, w, nil)
}
func (s *Server) runtimeResourceCleanupHTTP(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	actor, err := management.CaptureDeploymentActor(r.Context(), p.ID)
	if err != nil || actor.Local {
		problem(w, 403, "runtime_forbidden", "Current project deployment and Secrets access is required.")
		return
	}
	var input struct {
		PreparationDigest string `json:"preparationDigest"`
	}
	if !journeyDecode(w, r, &input) {
		return
	}
	row, err := s.CleanupRuntimeResourcePreparation(r.Context(), actor, r.PathValue("preparationId"), input.PreparationDigest)
	if err != nil {
		problem(w, 409, "runtime_cleanup_unconfirmed", "Original helper cleanup is not confirmed. Read or observe this preparation; no upload, build or replacement resource was started.")
		return
	}
	writeJSON(w, 200, row)
}

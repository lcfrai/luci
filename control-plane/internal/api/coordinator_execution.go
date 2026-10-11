package api

import (
	"context"
	"github.com/lcfrai/luci/control-plane/internal/management"
)

// Configuration delegates routine actions; it cannot create grants, provider
// access or cost authority. Re-resolve those for each typed execution step.
func (a *API) authorizeCoordinatorExecution(ctx context.Context, projectID, harness string, secrets bool) error {
	actor, err := a.captureQueuedActor(ctx, projectID)
	if err != nil {
		return err
	}
	_, err = a.authorizeExecution(ctx, actor, harness, secrets)
	return err
}

func (a *API) reauthorizeCoordinator(ctx context.Context, ref management.ExecutionActorRef, harness string, secrets bool) (context.Context, error) {
	authority, err := a.authorizeExecution(ctx, ref, harness, secrets)
	if err != nil {
		return nil, err
	}
	if ref.Local || ref.UserID == "" || authority.Actor.ID != ref.UserID {
		return nil, management.ErrExecutionAuthority
	}
	fresh := management.WithExecutionAuthority(ctx, authority)
	if err = a.authorizeExecutionRetirement(fresh, ref.ProjectID); err != nil {
		return nil, err
	}
	return fresh, nil
}

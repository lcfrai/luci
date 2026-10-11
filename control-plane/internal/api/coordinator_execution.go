package api

import "context"

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

package api

import (
	"context"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/ids"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"time"
)

type sessionRestartAdmission struct {
	resources  backend.Resources
	revalidate func(context.Context) error
	bind       func(context.Context, string) error
	complete   func(context.Context, string) error
	release    func()
}

// Reserve the replacement while the original sandbox remains intact. A failed
// admission must not turn a running session into a stopped one.
func (a *API) prepareSessionRestart(ctx context.Context, prior registry.Session, connection product.AgentConnectionReceipt) (sessionRestartAdmission, error) {
	result := sessionRestartAdmission{resources: a.sessionResources(ctx), revalidate: func(context.Context) error { return nil }, bind: func(context.Context, string) error { return nil }, complete: func(context.Context, string) error { return nil }, release: func() {}}
	result.revalidate = func(ctx context.Context) error { return a.validateExecutionResourceLimits(ctx, result.resources) }
	if err := result.revalidate(ctx); err != nil {
		return result, err
	}
	if a.deps.ResourceSnapshot == nil {
		if a.deps.ManagedExecution || a.deps.AuthorizeExecution != nil {
			return result, errors.New("Current execution capacity is unavailable. The original session has not been stopped.")
		}
		return result, nil
	}
	actor, err := a.captureQueuedActor(ctx, prior.ProjectID)
	if err != nil {
		return result, err
	}
	authority, err := a.authorizeExecution(ctx, actor, prior.Harness, connection.ID != "")
	if err != nil {
		return result, err
	}
	if a.deps.Product == nil || a.execution == nil {
		return result, errors.New("Current execution policy is unavailable. The original session has not been stopped.")
	}
	leaseID := ids.New("restart")
	reserve := func(ctx context.Context, current management.ExecutionAuthority) (executionDecision, error) {
		if actor.UserID != "" {
			ctx = management.WithExecutionAuthority(ctx, current)
		}
		policy, err := a.deps.Product.SwarmSchedulerPolicy()
		if err != nil {
			return executionDecision{}, err
		}
		provider, limit := providerCapacity(product.ProjectRunRoute{Harness: prior.Harness, Connection: connection}, policy)
		return a.execution.reserveReplacement(ctx, leaseID, prior.ProjectID, provider, limit, actor, current, policy, prior)
	}
	decision, err := reserve(ctx, authority)
	if err != nil {
		return result, err
	}
	if decision.State != "admitted" {
		return result, errors.New(decision.Message)
	}
	result.resources = decision.Resources
	result.revalidate = func(ctx context.Context) error {
		current, err := a.authorizeExecution(ctx, actor, prior.Harness, connection.ID != "")
		if err != nil {
			return err
		}
		decision, err := reserve(ctx, current)
		if err != nil {
			return err
		}
		if decision.State != "admitted" {
			return errors.New(decision.Message)
		}
		if decision.Resources != result.resources {
			return errors.New("Worker resource policy changed before restart. Review its current limits and retry.")
		}
		return a.validateExecutionResourceLimits(ctx, result.resources)
	}
	result.bind = func(ctx context.Context, sandboxID string) error {
		if err := result.revalidate(ctx); err != nil {
			return err
		}
		if err := a.execution.bindSandbox(ctx, leaseID, prior.ID, sandboxID); err != nil {
			return err
		}
		return a.execution.markBackendStarting(ctx, leaseID, prior.ID, sandboxID)
	}
	result.complete = func(ctx context.Context, sandboxID string) error {
		return a.execution.markBackendSettled(ctx, leaseID, prior.ID, sandboxID)
	}
	result.release = func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := a.execution.release(cleanup, leaseID); err != nil && !errors.Is(err, ErrExecutionLeaseHeld) {
			a.sessionLog().Warn("retain restart capacity after unresolved cleanup", "err", err)
		}
	}
	return result, nil
}

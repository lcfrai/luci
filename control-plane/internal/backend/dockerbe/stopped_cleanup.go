package dockerbe

import (
	"context"
	"errors"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// DestroyStopped is deliberately separate from explicit stop. It removes only
// an exactly owned immutable container which is already stopped, without
// force. A concurrent start is left to the engine's non-force refusal.
func (b *Backend) DestroyStopped(ctx context.Context, observed backend.Sandbox) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if observed.ID == "" || observed.Ref == "" || observed.Kind != backend.KindDocker ||
		observed.Instance != b.instance || observed.State != backend.StateStopped ||
		observed.CreatedOnly || observed.CreationInFlight || observed.NonLoopback ||
		observed.Labels[backend.LabelManaged] != "true" || observed.Labels[backend.LabelInstance] != b.instance ||
		observed.Labels[backend.LabelSandboxID] != observed.ID || observed.Labels[backend.LabelSessionID] == "" {
		return fmt.Errorf("dockerbe: stopped cleanup ownership is unconfirmed: %w", backend.ErrNotRunning)
	}
	if _, creating := b.creates.Load(observed.ID); creating {
		return fmt.Errorf("dockerbe: stopped cleanup create is unresolved: %w", backend.ErrNotRunning)
	}
	// Inspect and remove the immutable reference, never the reusable name.
	resp, err := b.api.ContainerInspect(ctx, observed.Ref)
	if err != nil {
		if ctx.Err() == nil && stoppedCleanupNotFound(err) {
			return nil
		}
		return fmt.Errorf("dockerbe: inspect stopped container: %w", err)
	}
	if resp.ContainerJSONBase == nil || resp.ID != observed.Ref || resp.State == nil ||
		stateFrom(resp.State.Status) != backend.StateStopped || resp.State.Running || resp.State.Restarting ||
		resp.Config == nil {
		return fmt.Errorf("dockerbe: stopped container identity or state changed: %w", backend.ErrNotRunning)
	}
	for _, key := range []string{backend.LabelManaged, backend.LabelInstance, backend.LabelSandboxID, backend.LabelSessionID} {
		if resp.Config.Labels[key] != observed.Labels[key] {
			return fmt.Errorf("dockerbe: stopped container ownership changed: %w", backend.ErrNotRunning)
		}
	}
	if _, err := b.endpointFromPortMap(portsOf(resp)); err != nil {
		return err
	}
	if _, creating := b.creates.Load(observed.ID); creating {
		return fmt.Errorf("dockerbe: stopped cleanup create is unresolved: %w", backend.ErrNotRunning)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	err = b.api.ContainerRemove(ctx, observed.Ref, container.RemoveOptions{Force: false, RemoveVolumes: true})
	if err == nil || ctx.Err() == nil && stoppedCleanupNotFound(err) {
		return nil
	}
	return fmt.Errorf("dockerbe: remove stopped container: %w", err)
}

// A joined transport failure and NotFound is uncertainty, not absence. Docker's
// ordinary single-chain NotFound wrappers remain idempotent.
func stoppedCleanupNotFound(err error) bool {
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, backend.ErrUnavailable) {
		return false
	}
	for current := err; current != nil; current = errors.Unwrap(current) {
		if _, multiple := current.(interface{ Unwrap() []error }); multiple {
			return false
		}
	}
	return client.IsErrNotFound(err)
}

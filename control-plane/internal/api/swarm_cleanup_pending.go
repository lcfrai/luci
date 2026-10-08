package api

import (
	"context"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// This is narrower than ErrExecutionLeaseHeld, which also covers unavailable
// storage, missing ownership, ambiguous launches and transport errors. It only
// classifies a positively identified original stopped Docker allocation; it
// does not remove its object, retire a lease or authorize a replacement.
func (g *executionAdmission) stoppedCleanupIsLocal(ctx context.Context, lease executionLease, row registry.Session, observed backend.Sandbox) bool {
	if ctx.Err() != nil || lease.State != "backend-settled" || lease.LaunchGuard != 1 || lease.FenceProtocol != backend.LaunchFenceProtocol ||
		lease.FenceEngineID == "" || lease.FenceInstanceID == "" || row.Backend != string(backend.KindDocker) || row.Status != registry.StatusExited ||
		row.SandboxRef == "" || row.SandboxID != lease.SandboxID || row.ID != lease.SessionID || row.ProjectID != lease.ProjectID ||
		observed.ID != row.SandboxID || observed.Ref != row.SandboxRef || observed.Kind != backend.KindDocker || observed.State != backend.StateStopped ||
		observed.Instance != lease.FenceInstanceID || observed.Labels[backend.LabelManaged] != "true" ||
		observed.Labels[backend.LabelInstance] != lease.FenceInstanceID || observed.Labels[backend.LabelSandboxID] != row.SandboxID || observed.Labels[backend.LabelSessionID] != row.ID {
		return false
	}
	fencer, ok := g.api.deps.Backend.(backend.LaunchFencer)
	if !ok {
		return false
	}
	// A host-wide outage or changed engine must remain a global admission stop.
	// The check shares the existing release/ledger exclusion and has one short
	// deadline, rather than retrying or running an independent probe loop.
	identityCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	identity, err := fencer.LaunchFenceIdentity(identityCtx)
	return err == nil && identityCtx.Err() == nil && identity.Protocol == lease.FenceProtocol && identity.EngineID == lease.FenceEngineID && identity.InstanceID == lease.FenceInstanceID
}

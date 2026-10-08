package dockerbe

import (
	"context"
	"math"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/system"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

type resourceLimitClient interface {
	Info(context.Context) (system.Info, error)
}

var _ backend.ResourceLimitValidator = (*Backend)(nil)

// ValidateResourceLimits is read-only and deliberately separate from Create:
// ordinary capability refusal can be reported before a durable dispatch claim.
func (b *Backend) ValidateResourceLimits(ctx context.Context, resources backend.Resources) error {
	_, err := b.memoryLimitIdentity(ctx, resources)
	return err
}

func (b *Backend) memoryLimitIdentity(ctx context.Context, resources backend.Resources) (string, error) {
	if resources.MemoryMiB == 0 {
		return "", nil // no requested memory cap; never invent a negative cap
	}
	if resources.MemoryMiB < 0 || int64(resources.MemoryMiB) > math.MaxInt64>>20 {
		return "", backend.ErrResourceLimitsUnsupported
	}
	api, ok := b.api.(resourceLimitClient)
	if !ok {
		return "", backend.ErrResourceLimitsUnsupported
	}
	ctx, cancel := context.WithTimeout(ctx, checkObservationBudget)
	defer cancel()
	info, err := api.Info(ctx)
	if err != nil || ctx.Err() != nil {
		return "", backend.ErrUnavailable
	}
	if info.ID == "" || len(info.ID) > 256 || strings.TrimSpace(info.ID) != info.ID {
		return "", backend.ErrUnavailable
	}
	if !info.MemoryLimit || !info.SwapLimit {
		return "", backend.ErrResourceLimitsUnsupported
	}
	return info.ID, nil
}

func (b *Backend) verifyCreatedWorkerLimits(ctx context.Context, spec backend.Spec, ref, engine string) (bool, error) {
	if ref == "" || engine == "" {
		return false, backend.ErrUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, checkObservationBudget)
	defer cancel()
	current, err := b.memoryLimitIdentity(ctx, spec.Resources)
	if err != nil {
		return false, err
	}
	if current != engine {
		return false, backend.ErrLaunchFenceOwnership
	}
	row, err := b.api.ContainerInspect(ctx, ref)
	if err != nil || ctx.Err() != nil {
		return false, backend.ErrUnavailable
	}
	owned := createdWorkerIdentityMatches(row, spec, ref, b.instance)
	current, err = b.memoryLimitIdentity(ctx, spec.Resources)
	if err != nil {
		return false, err
	}
	if current != engine {
		return false, backend.ErrLaunchFenceOwnership
	}
	if !owned || !createdWorkerLimitsMatch(row, spec, ref, b.instance) {
		return owned, backend.ErrResourceLimitsUnsupported
	}
	return true, nil
}

func createdWorkerIdentityMatches(row container.InspectResponse, spec backend.Spec, ref, instance string) bool {
	if row.ContainerJSONBase == nil || row.ID != ref || row.Name != "/"+namePrefix+spec.ID ||
		row.Config == nil || fenceMarked(row) {
		return false
	}
	labels := row.Config.Labels
	return labels[backend.LabelManaged] == "true" && labels[backend.LabelInstance] == instance &&
		labels[backend.LabelSandboxID] == spec.ID && labels[backend.LabelSessionID] == spec.SessionID
}

func createdWorkerLimitsMatch(row container.InspectResponse, spec backend.Spec, ref, instance string) bool {
	if !createdWorkerIdentityMatches(row, spec, ref, instance) || row.HostConfig == nil || row.State == nil ||
		row.State.Status != "created" || row.State.Running || row.State.Restarting || row.State.Paused || row.State.Dead || row.State.Pid != 0 {
		return false
	}
	host := row.HostConfig
	return host.Memory == spec.Resources.MemoryBytes() && host.MemorySwap == spec.Resources.MemoryBytes() &&
		host.NanoCPUs == spec.Resources.NanoCPUs() && host.PidsLimit != nil && *host.PidsLimit == 256
}

package dockerbe

import (
	"context"
	"strings"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/lcfrai/luci/control-plane/internal/backend"
)

func taskContainerStartedAt(row container.InspectResponse) time.Time {
	if row.ContainerJSONBase == nil || row.State == nil {
		return time.Time{}
	}
	at, _ := time.Parse(time.RFC3339Nano, row.State.StartedAt)
	return at
}

// Inspect by original immutable CID, never a reusable name. No logs, process
// probes, environment/credential reads, starts or image acquisition occur.
func (b *Backend) ObserveTaskExit(ctx context.Context, identity backend.TaskExitIdentity) (backend.TaskExitObservation, error) {
	if !fenceHex(identity.Ref, 32) || !fenceHex(identity.ContractDigest, 32) || !strings.HasPrefix(identity.Image, "sha256:") || !fenceHex(strings.TrimPrefix(identity.Image, "sha256:"), 32) || identity.CreatedAt.IsZero() || identity.StartedAt.IsZero() {
		return backend.TaskExitObservation{}, backend.ErrLaunchFenceOwnership
	}
	row, err := b.api.ContainerInspect(ctx, identity.Ref)
	if err != nil || ctx.Err() != nil {
		return backend.TaskExitObservation{}, backend.ErrUnavailable
	}
	if row.ContainerJSONBase == nil || row.Config == nil || row.State == nil || row.ID != identity.Ref || row.Image != identity.Image || row.RestartCount != 0 || fenceMarked(row) {
		return backend.TaskExitObservation{}, backend.ErrLaunchFenceOwnership
	}
	labels := row.Config.Labels
	if labels[backend.LabelManaged] != "true" || labels[backend.LabelInstance] != b.instance || labels[backend.LabelSessionID] != identity.SessionID || labels[backend.LabelSandboxID] != identity.SandboxID || labels[backend.TaskExitProtocolLabel] != backend.TaskExitProtocol || labels[backend.TaskExitContractLabel] != identity.ContractDigest || labels[backend.TaskExitImageLabel] != identity.Image || labels[backend.TaskNormalZeroCapability] != "v1" {
		return backend.TaskExitObservation{}, backend.ErrLaunchFenceOwnership
	}
	created, createErr := time.Parse(time.RFC3339Nano, row.Created)
	started := taskContainerStartedAt(row)
	finished, finishErr := time.Parse(time.RFC3339Nano, row.State.FinishedAt)
	if createErr != nil || finishErr != nil || !created.Equal(identity.CreatedAt) || !started.Equal(identity.StartedAt) || finished.Before(started) || started.Before(created) || finished.IsZero() {
		return backend.TaskExitObservation{}, backend.ErrLaunchFenceOwnership
	}
	if row.State.Status != "exited" || row.State.Running || row.State.Restarting || row.State.Paused || row.State.Dead || row.State.Pid != 0 || row.State.Error != "" || row.State.ExitCode < 0 || row.State.ExitCode > 255 {
		return backend.TaskExitObservation{}, backend.ErrNotRunning
	}
	return backend.TaskExitObservation{Ref: row.ID, Image: row.Image, ContractDigest: identity.ContractDigest, CreatedAt: created, StartedAt: started, FinishedAt: finished, OOMKilled: row.State.OOMKilled, ContainerExitCode: row.State.ExitCode, NormalZero: row.State.ExitCode == 0 && !row.State.OOMKilled}, nil
}

var _ backend.TaskExitObserver = (*Backend)(nil)

// A very short task can stop before the health listener is observed. Preserve
// its exact terminal allocation instead of converting it into failed Create
// and deleting the only daemon-held result. No outcome is inferred here.
func (b *Backend) completedStructuredTask(ctx context.Context, spec backend.Spec, ref string) (backend.Sandbox, bool) {
	if !backend.StructuredTaskSpec(spec) || !fenceHex(ref, 32) {
		return backend.Sandbox{}, false
	}
	row, err := b.api.ContainerInspect(ctx, ref)
	if err != nil || ctx.Err() != nil || row.ContainerJSONBase == nil || row.Config == nil || row.State == nil || row.ID != ref || row.Image != spec.Image || row.RestartCount != 0 || fenceMarked(row) {
		return backend.Sandbox{}, false
	}
	// A short stopped task is not an exception to the all-bindings loopback
	// guard. Never turn a LAN exposure error into successful Create.
	if _, err := b.endpointFromPortMap(portsOf(row)); err != nil {
		return backend.Sandbox{}, false
	}
	labels := row.Config.Labels
	if labels[backend.LabelInstance] != b.instance || labels[backend.LabelManaged] != "true" || labels[backend.LabelSessionID] != spec.SessionID || labels[backend.LabelSandboxID] != spec.ID || labels[backend.TaskExitContractLabel] != spec.Args[1] || labels[backend.TaskExitImageLabel] != spec.Image || labels[backend.TaskExitProtocolLabel] != backend.TaskExitProtocol || labels[backend.TaskNormalZeroCapability] != "v1" {
		return backend.Sandbox{}, false
	}
	created, err := time.Parse(time.RFC3339Nano, row.Created)
	started := taskContainerStartedAt(row)
	if err != nil || created.IsZero() || started.IsZero() || row.State.Status != "exited" || row.State.Running || row.State.Pid != 0 || row.State.Restarting || row.State.Paused || row.State.Dead {
		return backend.Sandbox{}, false
	}
	return backend.Sandbox{ID: spec.ID, Kind: backend.KindDocker, Ref: ref, Instance: b.instance, State: backend.StateStopped, CreatedAt: created, StartedAt: started, Labels: labels, OOMKilled: row.State.OOMKilled}, true
}

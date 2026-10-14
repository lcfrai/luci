package api

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// The selected engine's observed stop retires an in-flight Docker pull or
// helper mutation. Only after a subsequent exact-engine restore may Luci
// inspect and retire surviving owned objects. Endpoint timeouts and absence
// without that stopped witness never release this allocation.
func (a *API) reconcileRuntimePullEngine(ctx context.Context, lease runtimeImageExecutionLease, out runtimeinstall.Outcome, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
	out.ResourcesHeld = true
	observer, ok := a.deps.Backend.(backend.RuntimeImageEngineRetirementExecutor)
	if !ok {
		out.ReasonCode, out.Message = "runtime_pull_retirement_unavailable", "This exact pull remains charged because selected-engine retirement observation is unavailable."
		return out, publish(out)
	}
	state, err := observer.ObserveRuntimeImageEngine(ctx)
	if err != nil {
		out.ReasonCode, out.Message = "runtime_pull_engine_observation_unavailable", "The selected engine state is unavailable; this exact pull remains charged."
		return out, publish(out)
	}
	switch {
	case lease.EngineRecoveryStage == "" && state.State == "running" && state.EngineID == lease.Identity.EngineID:
		lease, err = a.execution.advanceRuntimeImageEngineRetirement(ctx, lease, "watching")
		if err != nil {
			return out, err
		}
		go a.watchRuntimeImageEngineStop(lease, observer)
	case lease.EngineRecoveryStage == "watching" && state.State == "stopped":
		lease, err = a.execution.advanceRuntimeImageEngineRetirement(ctx, lease, "stopped")
		if err != nil {
			return out, err
		}
	case lease.EngineRecoveryStage == "watching" && state.State == "running" && state.EngineID == lease.Identity.EngineID:
		go a.watchRuntimeImageEngineStop(lease, observer)
	case lease.EngineRecoveryStage == "stopped" && state.State == "running" && state.EngineID == lease.Identity.EngineID:
		// A lost cleanup acknowledgement requires another engine stop after the
		// cleanup intent, not a second remove based on object absence.
		if lease.ImportStage == "recovery-retiring" && !lease.EngineStoppedAt.After(lease.PullCleanupIntentAt) {
			out.ReasonCode, out.Message = "runtime_pull_cleanup_unconfirmed", "Cleanup outcome remains uncertain. Observe another exact selected-engine stop before reconciling it."
			return out, publish(out)
		}
		root, err := a.execution.prepareExecutionDirectory()
		if err != nil {
			return out, err
		}
		stage := filepath.Join(root, "runtime-pull-"+lease.ID)
		retirer, ok := a.deps.Backend.(backend.RuntimeImagePullRecoveryExecutor)
		if !ok {
			out.ReasonCode, out.Message = "runtime_pull_retirement_unavailable", "The exact helper retirement adapter is unavailable; its allocation remains held."
			return out, publish(out)
		}
		owned, err := retirer.ObserveRetiredPullOwnership(ctx, lease.Identity, lease.PullManifestSHA256, lease.PullBootstrapID, stage)
		if err != nil {
			out.ReasonCode, out.Message = "runtime_pull_ownership_unconfirmed", "Owned network or helper identity could not be verified after engine restore; its allocation remains held."
			return out, errors.Join(err, publish(out))
		}
		if lease.PullNetworkID != "" && owned.NetworkID != "" && lease.PullNetworkID != owned.NetworkID || lease.PullHelperID != "" && owned.ContainerID != "" && lease.PullHelperID != owned.ContainerID || owned.ContainerID != "" && owned.NetworkID == "" {
			return out, backend.ErrLaunchFenceOwnership
		}
		if owned.NetworkID != "" {
			if lease.ImportStage != "recovery-retiring" {
				lease, err = a.execution.beginRuntimeImagePullRetirement(ctx, lease, owned)
				if err != nil {
					return out, err
				}
				out.ReasonCode, out.Message = "runtime_pull_recovery_retiring", "Exact owned helper cleanup is recorded before its one request."
				if err = publish(out); err != nil {
					return out, err
				}
			}
			if err = retirer.RetireKnownPullOwnership(ctx, lease.Identity, lease.PullManifestSHA256, lease.PullBootstrapID, stage, owned); err != nil {
				if lease.ImportStage == "recovery-retiring" {
					record, cancel := previewRecordContext(ctx)
					if renewed, renewErr := a.execution.renewRuntimePullEngineRetirement(record, lease); renewErr == nil {
						lease = renewed
						go a.watchRuntimeImageEngineStop(lease, observer)
					}
					cancel()
				}
				out.ReasonCode, out.Message = "runtime_pull_cleanup_unconfirmed", "Owned helper cleanup has no confirmed terminal result; its charge remains held for another observed engine stop."
				return out, errors.Join(err, publish(out))
			}
			lease, err = a.execution.advanceRuntimeImagePull(ctx, lease, "recovery-retiring", "recovery-retired", "")
			if err != nil {
				return out, err
			}
		}
		if err = a.execution.retireRuntimePullStage(stage, lease.ID); err != nil {
			out.ReasonCode, out.Message = "runtime_pull_stage_cleanup_unconfirmed", "Private staging cleanup is unconfirmed; its charge remains held."
			return out, errors.Join(err, publish(out))
		}
		lease, err = a.execution.completeRuntimeImage(ctx, lease, true, "")
		if err != nil {
			return out, err
		}
		out.Phase, out.ResourcesHeld, out.CleanupConfirmed = "cancelled", false, true
		out.ReasonCode, out.Message = "runtime_pull_engine_retired", "The selected engine was observed stopped and restored; exact owned helper, network and private stage are retired. No pull or derivative was replayed."
		if err = publish(out); err != nil {
			return out, err
		}
		return out, a.execution.acknowledgeRuntimeImageCompletion(ctx, lease)
	}
	out.ReasonCode, out.Message = "runtime_pull_engine_watch", "The exact pull remains charged. Observe the selected Docker engine stopping and restoring before its uncertain effects can be reconciled."
	return out, publish(out)
}

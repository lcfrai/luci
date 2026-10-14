package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// pullRuntimeImage journals an intent before every daemon write. The normal
// path has no v1 ImageLoad or source-build fallback. An unconfirmed effect
// retains its exact allocation and is never reissued by this method.
func (a *API) pullRuntimeImage(ctx context.Context, req runtimeinstall.Request, publish runtimeinstall.Publish, out runtimeinstall.Outcome, lease runtimeImageExecutionLease, prepared backend.PreparedRuntimeImage, puller backend.RuntimeImagePullExecutor, admit func(bool) (executionDecision, runtimeImageExecutionLease, error)) (runtimeinstall.Outcome, error) {
	if lease.DistributionMode != "compressed-pull" || lease.ImportStage != "staging" || prepared.PullBundle == nil || lease.PullManifestSHA256 != prepared.PullBundle.ManifestSHA256 {
		return out, backend.ErrLaunchFenceOwnership
	}
	unknown := func(code, message string, cause error) (runtimeinstall.Outcome, error) {
		out.Phase, out.ReasonCode, out.Message = "unknown", code, message
		return out, errors.Join(cause, publish(out), backend.ErrUnavailable)
	}
	finish := func(cause error) (runtimeinstall.Outcome, error) {
		if err := publish(out); err != nil {
			return out, errors.Join(cause, err)
		}
		record, cancel := previewRecordContext(ctx)
		defer cancel()
		return out, errors.Join(cause, a.execution.acknowledgeRuntimeImageCompletion(record, lease))
	}
	settleWithoutHelper := func(code string, cause error) (runtimeinstall.Outcome, error) {
		if err := a.execution.retireRuntimePullStage(prepared.Spec.PullStagingDirectory, lease.ID); err != nil {
			return unknown("runtime_pull_stage_cleanup_unconfirmed", "Private staging cleanup is unconfirmed; its allocation remains held.", errors.Join(cause, err))
		}
		record, cancel := previewRecordContext(ctx)
		var err error
		lease, err = a.execution.completeRuntimeImage(record, lease, true, "")
		cancel()
		if err != nil {
			return out, errors.Join(cause, err)
		}
		out.Phase, out.ResourcesHeld, out.CleanupConfirmed = "failed", false, true
		out.ReasonCode, out.Message = code, "The exact runtime preparation settled before any helper was started. Its allocation is released."
		if ctx.Err() != nil {
			out.Phase = "cancelled"
		}
		return finish(cause)
	}
	advance := func(from, to, output string) error {
		var err error
		lease, err = a.execution.advanceRuntimeImagePull(ctx, lease, from, to, output)
		if err != nil {
			return err
		}
		out.Phase, out.Message = to, "The verified runtime install is continuing under its retained allocation."
		return publish(out)
	}
	authorize := func() error {
		_, err := a.authorizeRuntimeInstaller(ctx, req.Actor)
		return err
	}
	// Staging writes only bounded, reverified private files. This happens after
	// admission, before any Docker effect or bootstrap loading intent.
	if err := prepared.PullBundle.StageStoreContext(ctx, prepared.Spec.PullStagingDirectory); err != nil {
		return settleWithoutHelper("runtime_pull_stage_failed", err)
	}
	receipt, err := prepared.PullBundle.VerifyPullStage(prepared.Spec.PullStagingDirectory)
	if err != nil {
		return settleWithoutHelper("runtime_pull_stage_failed", err)
	}
	bootstrapAuthority, err := a.authorizeRuntimeInstaller(ctx, req.Actor)
	if err != nil {
		return settleWithoutHelper("runtime_bootstrap_not_started", err)
	}
	bootstrapPolicy, err := a.deps.Product.SwarmSchedulerPolicy()
	if err != nil {
		return settleWithoutHelper("runtime_bootstrap_not_started", err)
	}
	bootstrapDecision, nextLease, err := a.execution.reserveRuntimeImageBootstrap(ctx, prepared, req.Actor, bootstrapAuthority, bootstrapPolicy)
	if err != nil || bootstrapDecision.State != "admitted" {
		out.Admission = runtimeAdmissionRefusal(bootstrapDecision, "before-bootstrap")
		return settleWithoutHelper("runtime_bootstrap_admission_changed", err)
	}
	lease = nextLease
	if err := advance("staging", "bootstrap-loading", ""); err != nil {
		return unknown("runtime_bootstrap_unconfirmed", "Bootstrap intent is retained; no Docker effect will be replayed.", err)
	}
	if err := authorize(); err != nil {
		return settleWithoutHelper("runtime_bootstrap_not_started", err)
	}
	loaded, loadErr := puller.LoadPullBootstrap(ctx, prepared)
	if !loaded.Settled {
		return unknown("runtime_bootstrap_unconfirmed", "The small bootstrap load has no terminal result; its allocation remains held.", loadErr)
	}
	if loadErr != nil || loaded.ImageID != prepared.PullBundle.Manifest.Bootstrap.ImageID {
		return settleWithoutHelper("runtime_bootstrap_failed", errors.Join(loadErr, backend.ErrUnavailable))
	}
	if err := advance("bootstrap-loading", "bootstrap-loaded", loaded.ImageID); err != nil {
		return unknown("runtime_bootstrap_receipt_unconfirmed", "The bootstrap readback succeeded, but its durable receipt is unconfirmed.", err)
	}
	if err := authorize(); err != nil {
		return settleWithoutHelper("runtime_pull_network_not_started", err)
	}
	if err := advance("bootstrap-loaded", "network-creating", ""); err != nil {
		return unknown("runtime_pull_network_unconfirmed", "Network intent is retained; no create request will be replayed.", err)
	}
	networkID, err := puller.CreatePullNetwork(ctx, prepared)
	if err != nil {
		return unknown("runtime_pull_network_unconfirmed", "The exact private network create has no confirmed result; its allocation remains held.", err)
	}
	if err := advance("network-creating", "network-created", networkID); err != nil {
		return unknown("runtime_pull_network_receipt_unconfirmed", "The private network was observed but its durable receipt is unconfirmed.", err)
	}
	if err := authorize(); err != nil {
		return unknown("runtime_pull_helper_not_started", "Access changed with an owned private network retained for exact reconciliation.", err)
	}
	if err := advance("network-created", "helper-creating", ""); err != nil {
		return unknown("runtime_pull_helper_unconfirmed", "Helper intent is retained; no create request will be replayed.", err)
	}
	helperID, err := puller.CreatePullHelper(ctx, prepared, receipt, networkID)
	if err != nil {
		return unknown("runtime_pull_helper_unconfirmed", "The exact helper create has no confirmed result; its allocation remains held.", err)
	}
	if err := advance("helper-creating", "helper-created", helperID); err != nil {
		return unknown("runtime_pull_helper_receipt_unconfirmed", "The helper was observed but its durable receipt is unconfirmed.", err)
	}
	helper := backend.RuntimePullHelper{NetworkID: networkID, ContainerID: helperID}
	if err := authorize(); err != nil {
		return unknown("runtime_pull_helper_not_started", "Access changed with an owned helper retained for exact reconciliation.", err)
	}
	if err := advance("helper-created", "helper-starting", ""); err != nil {
		return unknown("runtime_pull_helper_start_unconfirmed", "Helper start intent is retained; it will not be replayed.", err)
	}
	helper.HostPort, err = puller.StartPullHelper(ctx, prepared, helper)
	if err != nil {
		return unknown("runtime_pull_helper_start_unconfirmed", "The helper start has no confirmed terminal result; its allocation remains held.", err)
	}
	if err := advance("helper-starting", "helper-started", helper.HostPort); err != nil {
		return unknown("runtime_pull_helper_start_receipt_unconfirmed", "The helper started but its durable receipt is unconfirmed.", err)
	}
	if err := puller.ProbePullHelper(ctx, prepared, helper); err != nil {
		return unknown("runtime_pull_route_unconfirmed", "The owned helper route could not be verified; it remains charged for reconciliation.", err)
	}
	if err := authorize(); err != nil {
		return unknown("runtime_pull_not_started", "Access changed before the image pull; the owned helper remains charged for cleanup.", err)
	}
	if decision, nextLease, admitErr := admit(false); admitErr != nil || decision.State != "admitted" {
		out.Admission = runtimeAdmissionRefusal(decision, "before-pull")
		return unknown("runtime_pull_admission_changed", "Current capacity or authority changed before the pull. The owned helper remains charged for cleanup.", admitErr)
	} else {
		lease = nextLease
	}
	if err := advance("helper-started", "pulling", ""); err != nil {
		return unknown("runtime_pull_unconfirmed", "Pull intent is retained; no image request will be replayed.", err)
	}
	pulled, pullErr := puller.PullPackagedRuntimeImage(ctx, prepared, helper)
	if !pulled.Settled || pullErr != nil || pulled.ImageID != prepared.PullBundle.Manifest.Image.ID {
		return unknown("runtime_pull_unconfirmed", "The exact image pull or its result has not settled with helper cleanup. It will not be retried automatically.", pullErr)
	}
	if err := advance("pulling", "pulled", pulled.ImageID); err != nil {
		return unknown("runtime_pull_receipt_unconfirmed", "The image readback succeeded, but its durable receipt is unconfirmed.", err)
	}
	if err := advance("pulled", "helper-retiring", ""); err != nil {
		return unknown("runtime_pull_retirement_unconfirmed", "Helper retirement intent is retained; cleanup is not assumed.", err)
	}
	if err := puller.RetirePullHelper(ctx, prepared, helper); err != nil {
		return unknown("runtime_pull_retirement_unconfirmed", "The exact helper or network retirement is unconfirmed; its allocation remains held.", err)
	}
	if err := advance("helper-retiring", "helper-retired", ""); err != nil {
		return unknown("runtime_pull_retirement_receipt_unconfirmed", "Helper retirement readback succeeded, but its durable receipt is unconfirmed.", err)
	}
	if err := a.execution.retireRuntimePullStage(prepared.Spec.PullStagingDirectory, lease.ID); err != nil {
		return unknown("runtime_pull_stage_cleanup_unconfirmed", "The helper retired, but private staging cleanup is unconfirmed.", err)
	}
	if err := authorize(); err != nil {
		return settleWithoutHelper("runtime_derivative_not_started", err)
	}
	if err := advance("helper-retired", "deriving", ""); err != nil {
		return unknown("runtime_derivative_unconfirmed", "Derivative intent is retained; it will not be replayed.", err)
	}
	derived, deriveErr := puller.DerivePullRuntimeImage(ctx, prepared)
	if !derived.Settled {
		return unknown("runtime_derivative_unconfirmed", "The owned derivative outcome is unconfirmed. The exact allocation remains held.", deriveErr)
	}
	if deriveErr != nil || derived.ImageID == "" {
		return settleWithoutHelper("runtime_derivative_failed", deriveErr)
	}
	record, cancel := previewRecordContext(ctx)
	lease, err = a.execution.completeRuntimeImage(record, lease, true, derived.ImageID)
	cancel()
	if err != nil {
		return out, err
	}
	out.ImageID, out.ResourcesHeld, out.CleanupConfirmed = derived.ImageID, false, true
	if err := authorize(); err != nil {
		out.Phase, out.ReasonCode, out.Message = "cancelled", "runtime_publication_not_started", "The owned image settled, but current access stopped publication."
		return finish(err)
	}
	out.Phase, out.Message = "publishing", "The exact owned runtime image is ready for tag publication."
	if err := publish(out); err != nil {
		return out, err
	}
	if err := authorize(); err != nil {
		out.Phase, out.ReasonCode, out.Message = "cancelled", "runtime_publication_not_started", "Current access stopped publication before its request."
		return finish(err)
	}
	publication, publishErr := puller.PublishRuntimeImage(ctx, prepared, derived.ImageID)
	if publishErr != nil || !publication.Acknowledged || publication.ImageID != derived.ImageID || publication.ImageTag != prepared.Spec.ImageTag {
		out.Phase, out.ReasonCode, out.Message = "unknown", "runtime_publication_unconfirmed", "Image publication is unconfirmed. No pull, derivative or tag request will replay automatically."
		return out, errors.Join(publishErr, publish(out), backend.ErrUnavailable)
	}
	out.Phase, out.ReasonCode, out.Message = "ready", "", "The verified compressed runtime image is ready."
	return finish(nil)
}

// This path is derived from the durable allocation under admission's private
// root, never from a caller-supplied path. RemoveAll does not traverse a leaf
// symlink; reject it anyway so a changed stage cannot be called settled.
func (g *executionAdmission) retireRuntimePullStage(stage, allocation string) error {
	root, err := g.prepareExecutionDirectory()
	if err != nil || !checkLogicalID(allocation) || stage != filepath.Join(root, "runtime-pull-"+allocation) {
		return backend.ErrLaunchFenceOwnership
	}
	info, err := os.Lstat(stage)
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return backend.ErrLaunchFenceOwnership
	}
	if err := os.RemoveAll(stage); err != nil {
		return err
	}
	if _, err := os.Lstat(stage); !errors.Is(err, os.ErrNotExist) {
		return errors.Join(err, backend.ErrUnavailable)
	}
	return nil
}

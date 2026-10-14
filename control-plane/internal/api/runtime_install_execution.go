package api

import (
	"context"
	"errors"
	"github.com/distribution/reference"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
	"path/filepath"
	"sync"
	"time"
)

var runtimeEngineWatch sync.Map

// The desktop restart can pass through its stopped interval between two UI
// requests. Keep one bounded, read-only watcher per exact charged allocation;
// the durable ledger receipt survives API restart. Missing the interval leaves
// the charge held and allows a later new watch, never a guessed settlement.
func (a *API) watchRuntimeImageEngineStop(lease runtimeImageExecutionLease, observer backend.RuntimeImageEngineRetirementExecutor) {
	key := a.execution.path + "\x00" + lease.ID
	if _, loaded := runtimeEngineWatch.LoadOrStore(key, struct{}{}); loaded {
		return
	}
	defer runtimeEngineWatch.Delete(key)
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		state, err := observer.ObserveRuntimeImageEngine(ctx)
		if err == nil && state.State == "stopped" {
			current, readErr := a.execution.runtimeImageLease(ctx, lease.ID)
			if readErr == nil && current.Identity == lease.Identity && current.EngineRecoveryStage == "watching" && current.EngineWatchStartedAt == lease.EngineWatchStartedAt {
				_, _ = a.execution.advanceRuntimeImageEngineRetirement(ctx, current, "stopped")
			}
			return
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (a *API) runtimeInstaller() runtimeinstall.Runtime {
	return runtimeinstall.Runtime{CaptureActor: a.captureRuntimeInstallerActor, Run: a.installRuntimeImage, Reconcile: a.reconcileRuntimeImage}
}

// Build demand is independent of a worker's configured size. Administrator
// check ceilings still constrain the actual allocation, including overrides.
func (a *API) runtimeBuildResources(policy *management.Policy) backend.Resources {
	resources := a.deps.RuntimeBuildResources
	if resources.MemoryMiB == 0 {
		resources.MemoryMiB = backend.DefaultRuntimeBuildResources.MemoryMiB
	}
	if resources.CPUMillis == 0 {
		resources.CPUMillis = backend.DefaultRuntimeBuildResources.CPUMillis
	}
	if policy != nil {
		if policy.CheckMemoryMiB > 0 {
			resources.MemoryMiB = min(resources.MemoryMiB, policy.CheckMemoryMiB)
		}
		if policy.CheckCPUMillis > 0 {
			resources.CPUMillis = min(resources.CPUMillis, policy.CheckCPUMillis)
		}
	}
	return resources
}

func (a *API) authorizeRuntimeInstaller(ctx context.Context, actor management.ExecutionActorRef) (management.ExecutionAuthority, error) {
	if err := ctx.Err(); err != nil {
		return management.ExecutionAuthority{}, err
	}
	if actor.ProjectID != "" || actor.TeamID != "" || a.deps.ExecutionReady != nil && !a.deps.ExecutionReady() {
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	if a.deps.AuthorizeRuntimeInstaller != nil {
		return a.deps.AuthorizeRuntimeInstaller(ctx, actor)
	}
	if actor.Local && actor.UserID == "" && !a.deps.ManagedExecution {
		return management.ExecutionAuthority{}, nil
	}
	return management.ExecutionAuthority{}, management.ErrExecutionAuthority
}
func (a *API) captureRuntimeInstallerActor(ctx context.Context) (management.ExecutionActorRef, error) {
	actor, err := management.CaptureRuntimeInstallerActor(ctx)
	if err == nil {
		_, err = a.authorizeRuntimeInstaller(ctx, actor)
	}
	return actor, err
}

// installRuntimeImage is the trusted bundled-runtime allocation path. It shares
// the machine ledger with workers/checks/previews, and never retries a build or
// tag request after an uncertain acknowledgement. A finished ledger row retains
// the engine-wide installer slot until publication is durably acknowledged.
func (a *API) installRuntimeImage(ctx context.Context, req runtimeinstall.Request, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
	out := runtimeinstall.Outcome{Identity: req.Identity, Phase: "blocked", Message: "Runtime installation has not started."}
	fail := func(code, message string, cause error) (runtimeinstall.Outcome, error) {
		out.ReasonCode, out.Message = code, message
		if publish != nil {
			if e := publish(out); e != nil {
				return out, e
			}
		}
		return out, cause
	}
	if publish == nil || !checkLogicalID(req.JobID) || req.ImageTag == "" || req.SourceDirectory == "" || req.Retained() || req.EngineID != "" || req.InstanceID != "" || req.SourceDigest != "" || req.ImageID != "" || req.Mode != "" && req.Mode != "packaged" && req.Mode != "compressed-pull" && req.Mode != "advanced-build" {
		return fail("runtime_install_request_invalid", "The bundled runtime request is invalid; nothing was started.", backend.ErrUnsupported)
	}
	if _, err := a.authorizeRuntimeInstaller(ctx, req.Actor); err != nil {
		return fail("runtime_install_forbidden", "Current administrator access is required.", err)
	}
	unlock, err := a.execution.lockChildLifecycle(ctx, "runtime-install-"+req.JobID)
	if err != nil {
		return out, err
	}
	defer unlock()
	executor, ok := a.deps.Backend.(backend.RuntimeImageExecutor)
	if !ok {
		return fail("runtime_install_unsupported", "The configured engine does not support bounded runtime installation.", backend.ErrUnsupported)
	}
	authority, err := a.authorizeRuntimeInstaller(ctx, req.Actor)
	if err != nil {
		return fail("runtime_install_forbidden", "Current administrator access is required.", err)
	}
	var buildPolicy *management.Policy
	if !req.Actor.Local {
		buildPolicy = &authority.Policy
	}
	resources := a.runtimeBuildResources(buildPolicy)
	allocation, err := previewAllocationID("runtime-build-")
	if err != nil {
		return out, err
	}
	spec := backend.RuntimeImageSpec{Identity: backend.RuntimeImageIdentity{Protocol: backend.RuntimeImageExecutionProtocol, JobID: req.JobID, AllocationID: allocation, ScopeDigest: a.execution.scope}, Resources: resources, SourceDirectory: req.SourceDirectory, ImageTag: req.ImageTag}
	if req.Mode == "compressed-pull" {
		root, e := a.execution.prepareExecutionDirectory()
		if e != nil {
			return fail("runtime_pull_stage_unavailable", "Private runtime staging is unavailable; no Docker work was started.", e)
		}
		spec.PullStagingDirectory = filepath.Join(root, "runtime-pull-"+allocation)
	}
	name, err := reference.ParseNormalizedNamed(req.ImageTag)
	if err != nil {
		return fail("runtime_image_tag_invalid", "The runtime image tag is invalid.", err)
	}
	canonicalTag := reference.TagNameOnly(name).String()
	var prepared backend.PreparedRuntimeImage
	var importer backend.RuntimeImageImportExecutor
	var puller backend.RuntimeImagePullExecutor
	if req.Mode == "packaged" {
		var supported bool
		importer, supported = a.deps.Backend.(backend.RuntimeImageImportExecutor)
		if !supported {
			return fail("runtime_import_unsupported", "This installation cannot verify and import its packaged runtime; the advanced build was not started.", backend.ErrUnsupported)
		}
		prepared, err = importer.PreparePackagedRuntimeImage(ctx, spec)
	} else if req.Mode == "compressed-pull" {
		var supported bool
		puller, supported = a.deps.Backend.(backend.RuntimeImagePullExecutor)
		if !supported {
			return fail("runtime_pull_unsupported", "This installation cannot verify its compressed runtime distribution; no Docker work was started.", backend.ErrUnsupported)
		}
		prepared, err = puller.PreparePullRuntimeImage(ctx, spec)
	} else {
		prepared, err = executor.PrepareRuntimeImage(ctx, spec)
	}
	if err != nil {
		return fail("runtime_install_prepare_failed", "The bundled runtime package or engine limits could not be verified. No Docker work was started.", err)
	}
	if prepared.Bundle != nil {
		defer prepared.Bundle.Close()
	}
	if prepared.PullBundle != nil {
		defer prepared.PullBundle.Close()
	}
	id := prepared.Spec.Identity
	if id.Protocol != spec.Identity.Protocol || id.JobID != req.JobID || id.AllocationID != allocation || id.ScopeDigest != a.execution.scope || !checkHex(id.SourceDigest, 32) || id.EngineID == "" || backend.ValidateInstance(id.InstanceID) != nil || prepared.Spec.ImageTag != canonicalTag || prepared.Spec.Resources != resources || prepared.Spec.SourceDirectory != req.SourceDirectory || prepared.Spec.PullStagingDirectory != spec.PullStagingDirectory || prepared.RequiredDiskBytes < backend.RuntimeImageRequiredDiskBytes || prepared.AdmissionMemoryMiB != 0 && (prepared.AdmissionMemoryMiB < resources.MemoryMiB || prepared.AdmissionMemoryMiB > 65536) || (req.Mode == "packaged") != (prepared.Bundle != nil) || (req.Mode == "compressed-pull") != (prepared.PullBundle != nil) || req.Mode == "packaged" && (prepared.ContextDigest != id.SourceDigest || prepared.Bundle.Manifest.CanonicalInputDigest != id.SourceDigest) || req.Mode == "compressed-pull" && (prepared.ContextDigest != id.SourceDigest || prepared.PullBundle.Manifest.CanonicalInputDigest != id.SourceDigest) {
		return fail("runtime_install_identity_changed", "The prepared runtime identity changed; nothing was started.", backend.ErrLaunchFenceOwnership)
	}
	admit := func(afterPull bool) (executionDecision, runtimeImageExecutionLease, error) {
		authority, e := a.authorizeRuntimeInstaller(ctx, req.Actor)
		if e != nil {
			return executionDecision{}, runtimeImageExecutionLease{}, e
		}
		policy, e := a.deps.Product.SwarmSchedulerPolicy()
		if e != nil {
			return executionDecision{}, runtimeImageExecutionLease{}, e
		}
		if afterPull {
			return a.execution.reserveRuntimeImageAfterPull(ctx, prepared, req.Actor, authority, policy)
		}
		return a.execution.reserveRuntimeImage(ctx, prepared, req.Actor, authority, policy)
	}
	decision, lease, err := admit(false)
	if err != nil || decision.State != "admitted" {
		if decision.Code == "" {
			decision.Code = "runtime_install_admission_unavailable"
			decision.Message = "Current administrator access or shared capacity could not be verified. No build was started."
			if req.Mode == "packaged" {
				decision.Message = "Current administrator access or shared capacity could not be verified. No image import was started."
			}
		}
		out.Admission = runtimeAdmissionRefusal(decision, "initial")
		return fail(decision.Code, decision.Message, err)
	}
	out.AllocationID, out.EngineID, out.InstanceID, out.SourceDigest = allocation, id.EngineID, id.InstanceID, id.SourceDigest
	out.Phase, out.ResourcesHeld, out.Message = "reserved", true, "Shared capacity is reserved; the runtime build has not started."
	if req.Mode == "packaged" {
		out.Message = "Shared capacity is reserved; the verified runtime import has not started."
	}
	// Before Build is issued we can prove there is no daemon work. Even a failed
	// progress write retains its no-work receipt until explicit reconciliation.
	unstarted := func(cause error) (runtimeinstall.Outcome, error) {
		record, cancel := previewRecordContext(ctx)
		defer cancel()
		settled, e := a.execution.completeRuntimeImage(record, lease, true, "")
		if e != nil {
			return out, errors.Join(cause, e)
		}
		out.Phase, out.ResourcesHeld, out.CleanupConfirmed = "blocked", false, true
		if ctx.Err() != nil {
			out.Phase = "cancelled"
		}
		out.ReasonCode, out.Message = "runtime_install_not_started", "Runtime preparation stopped before any image build was issued."
		if req.Mode == "packaged" {
			out.Message = "Runtime preparation stopped before any image import was issued."
		}
		if persistErr := publish(out); persistErr != nil {
			return out, errors.Join(cause, persistErr)
		}
		return out, errors.Join(cause, a.execution.acknowledgeRuntimeImageCompletion(record, settled))
	}
	if err = publish(out); err != nil {
		return unstarted(err)
	}
	nextDecision, nextLease, admitErr := admit(false)
	if admitErr != nil || nextDecision.State != "admitted" {
		stage, effect := "before-build", "runtime build"
		if req.Mode == "packaged" {
			stage, effect = "before-import", "runtime import"
		}
		out.Admission = runtimeAdmissionRefusal(nextDecision, stage)
		return unstarted(errors.Join(admitErr, errors.New(effect+" admission changed")))
	}
	lease = nextLease
	lease, err = a.execution.transitionRuntimeImage(ctx, lease, "building")
	if err != nil {
		return out, err
	}
	out.Phase, out.Message = "building", "Preparing the bundled runtime within the admitted CPU, memory and disk budget."
	if req.Mode == "packaged" {
		out.Phase, out.Message = "importing", "Importing the verified, tagless runtime payload. No local source compilation is started."
	}
	if err = publish(out); err != nil {
		return unstarted(err)
	}
	if _, err = a.authorizeRuntimeInstaller(ctx, req.Actor); err != nil {
		return unstarted(err)
	}
	if req.Mode == "packaged" {
		return a.importRuntimeImage(ctx, req, publish, out, lease, prepared, importer, admit)
	}
	if req.Mode == "compressed-pull" {
		return a.pullRuntimeImage(ctx, req, publish, out, lease, prepared, puller, admit)
	}
	staged, okay := executor.(backend.RuntimeImagePreBuildExecutor)
	if !okay {
		return unstarted(backend.ErrUnsupported)
	}
	afterPullCode, afterPullMessage := "", ""
	result, buildErr := staged.BuildRuntimeImageWithAdmission(ctx, prepared, func(stageCtx context.Context) error {
		if err := stageCtx.Err(); err != nil {
			return err
		}
		decision, nextLease, e := admit(true)
		if e == nil && decision.State == "admitted" {
			lease = nextLease
			return nil
		}
		afterPullCode, afterPullMessage = decision.Code, decision.Message
		if afterPullCode == "" {
			afterPullCode, afterPullMessage = "runtime_install_admission_unavailable", "Current administrator access or shared capacity changed after base-image preparation. No build was started."
		}
		out.Admission = runtimeAdmissionRefusal(decision, "after-pull")
		return errors.Join(e, errors.New("runtime build admission changed after base-image preparation"))
	})
	if !result.Settled {
		out.Phase = "unknown"
		return fail("runtime_build_unconfirmed", "The build outcome is unconfirmed. Its exact reservation remains held; no retry was started.", errors.Join(buildErr, backend.ErrUnavailable))
	}
	record, cancel := previewRecordContext(ctx)
	lease, err = a.execution.completeRuntimeImage(record, lease, true, result.ImageID)
	cancel()
	if err != nil {
		return out, err
	}
	out.ImageID, out.ResourcesHeld, out.CleanupConfirmed = result.ImageID, false, true
	finish := func(cause error) (runtimeinstall.Outcome, error) {
		if err := publish(out); err != nil {
			return out, errors.Join(cause, err)
		}
		record, cancel := previewRecordContext(ctx)
		defer cancel()
		return out, errors.Join(cause, a.execution.acknowledgeRuntimeImageCompletion(record, lease))
	}
	if buildErr != nil || result.ImageID == "" {
		out.Phase, out.ReasonCode, out.Message = "failed", "runtime_build_failed", "The runtime build settled without a usable image."
		if afterPullCode != "" {
			out.Phase, out.ReasonCode, out.Message = "blocked", afterPullCode, afterPullMessage
		} else if ctx.Err() != nil {
			out.Phase, out.ReasonCode, out.Message = "cancelled", "runtime_build_cancelled", "The exact runtime build ended without a usable image after cancellation."
		}
		return finish(buildErr)
	}
	if _, err = a.authorizeRuntimeInstaller(ctx, req.Actor); err != nil {
		out.Phase, out.ReasonCode, out.Message = "cancelled", "runtime_publication_not_started", "The build settled; publication stopped because the request or current administrator access changed."
		return finish(err)
	}
	out.Phase, out.Message = "publishing", "The runtime image is built; publishing its exact verified image."
	if err = publish(out); err != nil {
		return out, err
	}
	if _, err = a.authorizeRuntimeInstaller(ctx, req.Actor); err != nil {
		out.Phase, out.ReasonCode, out.Message = "cancelled", "runtime_publication_not_started", "The build settled; publication stopped before its request."
		return finish(err)
	}
	publication, publishErr := executor.PublishRuntimeImage(ctx, prepared, result.ImageID)
	if publishErr != nil || !publication.Acknowledged || publication.ImageID != result.ImageID || publication.ImageTag != prepared.Spec.ImageTag {
		out.Phase = "unknown"
		return fail("runtime_publication_unconfirmed", "The build is settled but image publication is unconfirmed. Review this exact installation; it will not rebuild or retag automatically.", errors.Join(publishErr, backend.ErrUnavailable))
	}
	out.Phase, out.ReasonCode, out.Message = "ready", "", "The bundled runtime image is ready."
	return finish(nil)
}

func (a *API) importRuntimeImage(ctx context.Context, req runtimeinstall.Request, publish runtimeinstall.Publish, out runtimeinstall.Outcome, lease runtimeImageExecutionLease, prepared backend.PreparedRuntimeImage, importer backend.RuntimeImageImportExecutor, admit func(bool) (executionDecision, runtimeImageExecutionLease, error)) (runtimeinstall.Outcome, error) {
	finish := func(cause error) (runtimeinstall.Outcome, error) {
		if err := publish(out); err != nil {
			return out, errors.Join(cause, err)
		}
		record, cancel := previewRecordContext(ctx)
		defer cancel()
		return out, errors.Join(cause, a.execution.acknowledgeRuntimeImageCompletion(record, lease))
	}
	settle := func(result backend.RuntimeImageBuildResult, cause error, failedCode string) (runtimeinstall.Outcome, bool, error) {
		if !result.Settled {
			out.Phase, out.ReasonCode, out.Message = "unknown", failedCode+"_unconfirmed", "The exact Docker effect has no confirmed terminal result. Its charge remains held; no import or derivative will be replayed."
			return out, true, errors.Join(cause, publish(out), backend.ErrUnavailable)
		}
		if cause != nil || result.ImageID == "" {
			record, cancel := previewRecordContext(ctx)
			var err error
			lease, err = a.execution.completeRuntimeImage(record, lease, true, "")
			cancel()
			if err != nil {
				return out, true, errors.Join(cause, err)
			}
			out.Phase, out.ResourcesHeld, out.CleanupConfirmed = "failed", false, true
			out.ReasonCode, out.Message = failedCode+"_failed", "The Docker effect returned a terminal failure; its exact allocation is settled without publishing a runtime tag."
			if ctx.Err() != nil {
				out.Phase, out.ReasonCode = "cancelled", failedCode+"_cancelled"
			}
			out, err = finish(cause)
			return out, true, err
		}
		return out, false, nil
	}
	if lease.DistributionMode != "packaged" || lease.ImportStage != "importing" || prepared.Bundle == nil {
		return out, backend.ErrLaunchFenceOwnership
	}
	if _, err := a.authorizeRuntimeInstaller(ctx, req.Actor); err != nil {
		return out, err
	}
	loaded, loadErr := importer.LoadPackagedRuntimeImage(ctx, prepared)
	if result, done, err := settle(loaded, loadErr, "runtime_import"); done {
		return result, err
	}
	if loaded.ImageID != prepared.Bundle.Manifest.Image.ID {
		return out, backend.ErrLaunchFenceOwnership
	}
	record, cancel := previewRecordContext(ctx)
	var err error
	lease, err = a.execution.advanceRuntimeImageImport(record, lease, loaded.ImageID)
	cancel()
	if err != nil {
		return out, err
	}
	out.Phase, out.Message = "deriving", "The shared payload is verified. Adding this installation's small ownership metadata before publication."
	if err = publish(out); err != nil {
		return out, err
	}
	if _, err = a.authorizeRuntimeInstaller(ctx, req.Actor); err != nil {
		return a.stopImportedRuntimeImage(ctx, publish, out, lease, err)
	}
	decision, nextLease, admitErr := admit(false)
	if admitErr != nil || decision.State != "admitted" {
		out.Phase, out.ReasonCode, out.Message = "blocked", "runtime_derivative_not_started", "The shared payload imported, but current capacity or authority prevents the owned derivative. No derivative was started."
		out.Admission = runtimeAdmissionRefusal(decision, "before-derivative")
		return a.stopImportedRuntimeImage(ctx, publish, out, lease, admitErr)
	}
	lease = nextLease
	record, cancel = previewRecordContext(ctx)
	lease, err = a.execution.beginRuntimeImageDerivation(record, lease)
	cancel()
	if err != nil {
		return out, err
	}
	derived, deriveErr := importer.DerivePackagedRuntimeImage(ctx, prepared)
	if result, done, err := settle(derived, deriveErr, "runtime_derivative"); done {
		return result, err
	}
	record, cancel = previewRecordContext(ctx)
	lease, err = a.execution.completeRuntimeImage(record, lease, true, derived.ImageID)
	cancel()
	if err != nil {
		return out, err
	}
	out.ImageID, out.ResourcesHeld, out.CleanupConfirmed = derived.ImageID, false, true
	if _, err = a.authorizeRuntimeInstaller(ctx, req.Actor); err != nil {
		out.Phase, out.ReasonCode, out.Message = "cancelled", "runtime_publication_not_started", "The owned image settled, but current administrator access or cancellation stopped publication."
		return finish(err)
	}
	out.Phase, out.Message = "publishing", "The exact owned runtime image is ready for tag publication."
	if err = publish(out); err != nil {
		return out, err
	}
	if _, err = a.authorizeRuntimeInstaller(ctx, req.Actor); err != nil {
		out.Phase, out.ReasonCode, out.Message = "cancelled", "runtime_publication_not_started", "Current administrator access or cancellation stopped publication before its request."
		return finish(err)
	}
	publication, publishErr := importer.PublishRuntimeImage(ctx, prepared, derived.ImageID)
	if publishErr != nil || !publication.Acknowledged || publication.ImageID != derived.ImageID || publication.ImageTag != prepared.Spec.ImageTag {
		out.Phase, out.ReasonCode, out.Message = "unknown", "runtime_publication_unconfirmed", "The image is settled but tag publication is unconfirmed. Review this exact installation; it will not import or retag automatically."
		return out, errors.Join(publishErr, publish(out), backend.ErrUnavailable)
	}
	out.Phase, out.ReasonCode, out.Message = "ready", "", "The verified packaged runtime is ready."
	return finish(nil)
}

// A positively imported shared payload has no allocation-specific effect until
// the derivative begins. A revoked actor or fresh capacity refusal at this
// boundary can settle only this lease without claiming an unknown Docker write.
func (a *API) stopImportedRuntimeImage(ctx context.Context, publish runtimeinstall.Publish, out runtimeinstall.Outcome, lease runtimeImageExecutionLease, cause error) (runtimeinstall.Outcome, error) {
	if lease.DistributionMode != "packaged" || lease.ImportStage != "imported" || lease.State != "building" {
		return out, errors.Join(cause, backend.ErrLaunchFenceOwnership)
	}
	record, cancel := previewRecordContext(ctx)
	settled, err := a.execution.completeRuntimeImage(record, lease, true, "")
	if err != nil {
		cancel()
		return out, errors.Join(cause, err)
	}
	out.ResourcesHeld, out.CleanupConfirmed = false, true
	if out.ReasonCode == "" {
		out.Phase, out.ReasonCode, out.Message = "cancelled", "runtime_derivative_not_started", "Current administrator access ended before the owned derivative was started."
	}
	if err = publish(out); err != nil {
		cancel()
		return out, errors.Join(cause, err)
	}
	err = a.execution.acknowledgeRuntimeImageCompletion(record, settled)
	cancel()
	return out, errors.Join(cause, err)
}

func (a *API) reconcileRuntimeImage(ctx context.Context, id runtimeinstall.Identity, actor management.ExecutionActorRef, publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
	out := runtimeinstall.Outcome{Identity: id, Phase: "unknown", Message: "The exact runtime installation still requires settlement."}
	invalid := func(err error) (runtimeinstall.Outcome, error) { return runtimeinstall.Outcome{}, err }
	if publish == nil || !checkLogicalID(id.JobID) || id.AllocationID != "" && !checkLogicalID(id.AllocationID) {
		return invalid(backend.ErrUnsupported)
	}
	if _, err := a.authorizeRuntimeInstaller(ctx, actor); err != nil {
		return invalid(err)
	}
	unlock, err := a.execution.lockChildLifecycle(ctx, "runtime-install-"+id.JobID)
	if err != nil {
		return invalid(err)
	}
	defer unlock()
	var lease runtimeImageExecutionLease
	if id.AllocationID == "" {
		if id.EngineID != "" || id.InstanceID != "" || id.ImageID != "" || id.SourceDigest != "" {
			return invalid(backend.ErrLaunchFenceOwnership)
		}
		// The product durably saves JobID before dispatch. If the first allocation
		// write failed, recover by that exact job under its lifecycle lock. Every
		// installer side effect requires this valid ledger; an absent known
		// allocation is deliberately NOT covered by this pre-identity case.
		// Initialization under the private ledger lock is also safe when this
		// job was cancelled before the first reservation ever created a ledger.
		e := a.execution.mutate(ctx, func(ledger *executionLeaseLedger) error {
			for _, v := range ledger.RuntimeImages {
				if v.Scope == a.execution.scope && v.Identity.JobID == id.JobID {
					if lease.ID != "" || v.Actor != actor {
						return backend.ErrLaunchFenceOwnership
					}
					lease = v
				}
			}
			return nil
		})
		if e != nil {
			return invalid(e)
		}
		if lease.ID == "" {
			if _, e = a.authorizeRuntimeInstaller(ctx, actor); e != nil {
				return invalid(e)
			}
			out.Phase, out.CleanupConfirmed, out.ReasonCode, out.Message = "cancelled", true, "runtime_not_started", "No retained runtime allocation exists for this unstarted job. No build or tag request was replayed."
			return out, publish(out)
		}
		out.AllocationID, out.EngineID, out.InstanceID, out.SourceDigest, out.ImageID = lease.ID, lease.Identity.EngineID, lease.Identity.InstanceID, lease.Identity.SourceDigest, lease.Image
	} else {
		lease, err = a.execution.runtimeImageLease(ctx, id.AllocationID)
		if err != nil {
			return invalid(err)
		}
	}
	name, tagErr := reference.ParseNormalizedNamed(id.ImageTag)
	if tagErr != nil || reference.TagNameOnly(name).String() != lease.ImageTag {
		return invalid(backend.ErrLaunchFenceOwnership)
	}
	i := lease.Identity
	if lease.Actor != actor || i.JobID != id.JobID || id.EngineID != "" && i.EngineID != id.EngineID || id.InstanceID != "" && i.InstanceID != id.InstanceID || id.SourceDigest != "" && i.SourceDigest != id.SourceDigest || id.ImageID != "" && lease.Image != id.ImageID {
		return invalid(backend.ErrLaunchFenceOwnership)
	}
	if id.AllocationID != "" && (id.EngineID == "" || id.InstanceID == "" || id.SourceDigest == "") {
		return invalid(backend.ErrLaunchFenceOwnership)
	}
	// Reserved cannot have issued any daemon mutation. A building receipt cannot
	// be settled by observing a tag or by request cancellation.
	if lease.State == "reserved" {
		lease, err = a.execution.completeRuntimeImage(ctx, lease, true, "")
		if err != nil {
			return invalid(err)
		}
	}
	if lease.State == "building" && lease.DistributionMode == "compressed-pull" {
		if lease.ImportStage == "network-created" || lease.ImportStage == "helper-created" || lease.ImportStage == "helper-started" || lease.ImportStage == "pulled" {
			root, e := a.execution.prepareExecutionDirectory()
			if e != nil {
				return invalid(e)
			}
			stage := filepath.Join(root, "runtime-pull-"+lease.ID)
			retirer, ok := a.deps.Backend.(backend.RuntimeImagePullRecoveryExecutor)
			if !ok {
				return invalid(backend.ErrUnsupported)
			}
			lease, e = a.execution.advanceRuntimeImagePull(ctx, lease, lease.ImportStage, "recovery-retiring", "")
			if e != nil {
				return invalid(e)
			}
			out.ResourcesHeld, out.ReasonCode, out.Message = true, "runtime_pull_recovery_retiring", "Retiring the exact owned helper and network; its charge remains held until positive readback."
			if e = publish(out); e != nil {
				return out, e
			}
			helper := backend.RuntimePullHelper{NetworkID: lease.PullNetworkID, ContainerID: lease.PullHelperID}
			if e = retirer.RetireKnownPullOwnership(ctx, lease.Identity, lease.PullManifestSHA256, lease.PullBootstrapID, stage, helper); e != nil {
				out.ReasonCode, out.Message = "runtime_pull_recovery_unconfirmed", "Owned helper or network retirement is unconfirmed. The allocation remains held; no cleanup request will be replayed."
				return out, errors.Join(e, publish(out))
			}
			lease, e = a.execution.advanceRuntimeImagePull(ctx, lease, "recovery-retiring", "recovery-retired", "")
			if e != nil {
				return invalid(e)
			}
		}
		// These stages have a positive no-live-helper boundary. Retire only the
		// exact private staging path before releasing their charge. Every other
		// stage may have an owned Docker mutation whose response was lost; the
		// generic build/tag observer cannot prove its network was retired.
		if lease.ImportStage == "staging" || lease.ImportStage == "bootstrap-loaded" || lease.ImportStage == "helper-retired" || lease.ImportStage == "recovery-retired" {
			root, e := a.execution.prepareExecutionDirectory()
			if e != nil {
				return invalid(e)
			}
			if e = a.execution.retireRuntimePullStage(filepath.Join(root, "runtime-pull-"+lease.ID), lease.ID); e != nil {
				out.ResourcesHeld, out.ReasonCode, out.Message = true, "runtime_pull_stage_cleanup_unconfirmed", "Private staging cleanup is unconfirmed; this exact allocation remains held."
				return out, publish(out)
			}
			lease, e = a.execution.completeRuntimeImage(ctx, lease, true, "")
			if e != nil {
				return invalid(e)
			}
			out.Phase, out.CleanupConfirmed, out.ReasonCode, out.Message = "cancelled", true, "runtime_pull_not_started", "The exact pull had no live helper or uncertain derivative; its private stage and charge are retired."
			if e = publish(out); e != nil {
				return out, e
			}
			return out, a.execution.acknowledgeRuntimeImageCompletion(ctx, lease)
		}
		if lease.ImportStage != "deriving" {
			return a.reconcileRuntimePullEngine(ctx, lease, out, publish)
		}
	}
	// The shared payload import has a positive terminal receipt, while the
	// allocation-specific derivative cannot start before a separate durable
	// "deriving" transition. The lifecycle lock excludes a concurrent old
	// controller; after restart this exact stage is safe to retire unchanged.
	if lease.State == "building" && lease.DistributionMode == "packaged" && lease.ImportStage == "imported" {
		lease, err = a.execution.completeRuntimeImage(ctx, lease, true, "")
		if err != nil {
			return invalid(err)
		}
		out.Phase, out.CleanupConfirmed, out.ReasonCode, out.Message = "cancelled", true, "runtime_derivative_not_started", "The shared payload was imported, but no owned derivative was started before interruption. This exact charge is released."
		if err = publish(out); err != nil {
			return out, err
		}
		return out, a.execution.acknowledgeRuntimeImageCompletion(ctx, lease)
	}
	if lease.State == "building" {
		observer, supported := a.deps.Backend.(backend.RuntimeImageEngineRetirementExecutor)
		if supported {
			state, observeErr := observer.ObserveRuntimeImageEngine(ctx)
			if observeErr == nil {
				switch {
				case lease.EngineRecoveryStage == "" && state.State == "running" && state.EngineID == lease.Identity.EngineID:
					lease, err = a.execution.advanceRuntimeImageEngineRetirement(ctx, lease, "watching")
					if err != nil {
						return invalid(err)
					}
					go a.watchRuntimeImageEngineStop(lease, observer)
				case lease.EngineRecoveryStage == "watching" && state.State == "stopped":
					lease, err = a.execution.advanceRuntimeImageEngineRetirement(ctx, lease, "stopped")
					if err != nil {
						lease, err = a.execution.runtimeImageLease(ctx, lease.ID)
						if err != nil || lease.EngineRecoveryStage != "stopped" || lease.Identity != i {
							return invalid(errors.Join(err, backend.ErrLaunchFenceOwnership))
						}
					}
				case lease.EngineRecoveryStage == "watching" && state.State == "running" && state.EngineID == lease.Identity.EngineID:
					go a.watchRuntimeImageEngineStop(lease, observer)
				case lease.EngineRecoveryStage == "stopped" && state.State == "running" && state.EngineID == lease.Identity.EngineID:
					retired, e := observer.ObserveRetiredRuntimeImage(ctx, lease.Identity, lease.ImageTag)
					if e == nil {
						if _, e = a.authorizeRuntimeInstaller(ctx, actor); e != nil {
							return invalid(e)
						}
						lease, e = a.execution.completeRuntimeImage(ctx, lease, true, retired.ImageID)
						if e != nil {
							return invalid(e)
						}
						out.ImageID, out.ResourcesHeld, out.CleanupConfirmed = retired.ImageID, false, true
						if retired.ImageID == "" {
							out.Phase, out.ReasonCode, out.Message = "cancelled", "runtime_engine_retired", "The selected Docker engine was observed stopped and restored; this exact build has no surviving output or intermediate."
							if e = publish(out); e != nil {
								return out, e
							}
							return out, a.execution.acknowledgeRuntimeImageCompletion(ctx, lease)
						}
						if retired.Publication == "published" {
							out.Phase, out.ReasonCode, out.Message = "ready", "", "The exact image and its tag survived the observed engine retirement."
							if e = publish(out); e != nil {
								return out, e
							}
							return out, a.execution.acknowledgeRuntimeImageCompletion(ctx, lease)
						}
						out.ReasonCode, out.Message = "runtime_publication_unconfirmed", "The build executor is retired and its charge released, but its exact image is not published by the configured tag. No tag request was replayed."
						return out, publish(out)
					}
				}
			}
		}
	}
	if lease.State != "finished" {
		out.ResourcesHeld = true
		switch lease.EngineRecoveryStage {
		case "watching":
			out.ReasonCode, out.Message = "runtime_engine_retirement_watch", "The exact build remains charged. Its selected Docker Desktop engine must be observed stopping during a bounded watch; check again to renew the watch before any later controlled restart. Luci will not restart it or touch unrelated work."
		case "stopped":
			out.ReasonCode, out.Message = "runtime_engine_stopped", "The selected Docker Desktop engine was observed stopped. Restore it, then reconcile this exact job for owned output and charge settlement."
		default:
			out.ReasonCode = "runtime_build_unconfirmed"
		}
		return out, publish(out)
	}
	out.ImageID, out.ResourcesHeld, out.CleanupConfirmed = lease.Image, false, true
	if lease.Image == "" {
		out.Phase, out.ReasonCode, out.Message = "failed", "runtime_build_failed", "The runtime build is settled without a published image."
	} else {
		executor, ok := a.deps.Backend.(backend.RuntimeImageExecutor)
		if !ok {
			return invalid(backend.ErrUnsupported)
		}
		publication := ""
		if lease.EngineRecoveryStage == "stopped" {
			retired, ok := a.deps.Backend.(backend.RuntimeImageEngineRetirementExecutor)
			if !ok {
				return invalid(backend.ErrUnsupported)
			}
			observed, e := retired.ObserveRetiredRuntimeImage(ctx, lease.Identity, lease.ImageTag)
			if e != nil || observed.ImageID != lease.Image {
				return invalid(errors.Join(e, backend.ErrLaunchFenceOwnership))
			}
			publication = observed.Publication
		} else {
			observation, e := executor.ObserveRuntimeImage(ctx, lease.Identity, lease.Image, lease.ImageTag)
			if e != nil {
				return invalid(e)
			}
			if observation.ImageID != lease.Image || observation.ImageTag != lease.ImageTag {
				return invalid(backend.ErrLaunchFenceOwnership)
			}
			publication = observation.Publication
		}
		if publication != "published" {
			out.ReasonCode, out.Message = "runtime_publication_unconfirmed", "The build is settled but its configured tag is not confirmed. No rebuild or tag mutation was attempted."
			return out, publish(out)
		}
		out.Phase, out.ReasonCode, out.Message = "ready", "", "The exact settled runtime image publication is confirmed."
	}
	if _, err = a.authorizeRuntimeInstaller(ctx, actor); err != nil {
		return invalid(err)
	}
	if err = publish(out); err != nil {
		return out, err
	}
	return out, a.execution.acknowledgeRuntimeImageCompletion(ctx, lease)
}

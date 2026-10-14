package api

import (
	"context"
	"errors"
	"github.com/distribution/reference"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

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
	if publish == nil || !checkLogicalID(req.JobID) || req.ImageTag == "" || req.SourceDirectory == "" || req.Retained() || req.EngineID != "" || req.InstanceID != "" || req.SourceDigest != "" || req.ImageID != "" {
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
	name, err := reference.ParseNormalizedNamed(req.ImageTag)
	if err != nil {
		return fail("runtime_image_tag_invalid", "The runtime image tag is invalid.", err)
	}
	canonicalTag := reference.TagNameOnly(name).String()
	prepared, err := executor.PrepareRuntimeImage(ctx, spec)
	if err != nil {
		return fail("runtime_install_prepare_failed", "The bundled runtime source or engine limits could not be verified. No build was started.", err)
	}
	id := prepared.Spec.Identity
	if id.Protocol != spec.Identity.Protocol || id.JobID != req.JobID || id.AllocationID != allocation || id.ScopeDigest != a.execution.scope || !checkHex(id.SourceDigest, 32) || id.EngineID == "" || backend.ValidateInstance(id.InstanceID) != nil || prepared.Spec.ImageTag != canonicalTag || prepared.Spec.Resources != resources || prepared.Spec.SourceDirectory != req.SourceDirectory || prepared.RequiredDiskBytes < backend.RuntimeImageRequiredDiskBytes || prepared.AdmissionMemoryMiB != 0 && (prepared.AdmissionMemoryMiB < resources.MemoryMiB || prepared.AdmissionMemoryMiB > 65536) {
		return fail("runtime_install_identity_changed", "The prepared runtime identity changed; nothing was started.", backend.ErrLaunchFenceOwnership)
	}
	admit := func() (executionDecision, runtimeImageExecutionLease, error) {
		authority, e := a.authorizeRuntimeInstaller(ctx, req.Actor)
		if e != nil {
			return executionDecision{}, runtimeImageExecutionLease{}, e
		}
		policy, e := a.deps.Product.SwarmSchedulerPolicy()
		if e != nil {
			return executionDecision{}, runtimeImageExecutionLease{}, e
		}
		return a.execution.reserveRuntimeImage(ctx, prepared, req.Actor, authority, policy)
	}
	decision, lease, err := admit()
	if err != nil || decision.State != "admitted" {
		if decision.Code == "" {
			decision.Code = "runtime_install_admission_unavailable"
			decision.Message = "Current administrator access or shared capacity could not be verified. No build was started."
		}
		out.Admission = runtimeAdmissionRefusal(decision, "initial")
		return fail(decision.Code, decision.Message, err)
	}
	out.AllocationID, out.EngineID, out.InstanceID, out.SourceDigest = allocation, id.EngineID, id.InstanceID, id.SourceDigest
	out.Phase, out.ResourcesHeld, out.Message = "reserved", true, "Shared capacity is reserved; the runtime build has not started."
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
		if persistErr := publish(out); persistErr != nil {
			return out, errors.Join(cause, persistErr)
		}
		return out, errors.Join(cause, a.execution.acknowledgeRuntimeImageCompletion(record, settled))
	}
	if err = publish(out); err != nil {
		return unstarted(err)
	}
	nextDecision, nextLease, admitErr := admit()
	if admitErr != nil || nextDecision.State != "admitted" {
		out.Admission = runtimeAdmissionRefusal(nextDecision, "before-build")
		return unstarted(errors.Join(admitErr, errors.New("runtime build admission changed")))
	}
	lease = nextLease
	lease, err = a.execution.transitionRuntimeImage(ctx, lease, "building")
	if err != nil {
		return out, err
	}
	out.Phase, out.Message = "building", "Preparing the bundled runtime within the admitted CPU, memory and disk budget."
	if err = publish(out); err != nil {
		return unstarted(err)
	}
	if _, err = a.authorizeRuntimeInstaller(ctx, req.Actor); err != nil {
		return unstarted(err)
	}
	result, buildErr := executor.BuildRuntimeImage(ctx, prepared)
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
	if lease.State != "finished" {
		out.ResourcesHeld = true
		out.ReasonCode = "runtime_build_unconfirmed"
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
		observation, e := executor.ObserveRuntimeImage(ctx, lease.Identity, lease.Image, lease.ImageTag)
		if e != nil {
			return invalid(e)
		}
		if observation.ImageID != lease.Image || observation.ImageTag != lease.ImageTag {
			return invalid(backend.ErrLaunchFenceOwnership)
		}
		if observation.Publication != "published" {
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

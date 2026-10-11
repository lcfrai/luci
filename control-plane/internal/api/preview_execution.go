package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewruntime"
)

func (a *API) previewRuntime() previewruntime.Runtime {
	return previewruntime.Runtime{CaptureActor: a.capturePreviewActor, Start: a.startPreview, Observe: a.observePreview, Stop: a.stopPreview, Restart: a.restartPreview}
}

func (a *API) authorizePreview(ctx context.Context, actor management.ExecutionActorRef, allocate bool) (management.ExecutionAuthority, error) {
	if err := ctx.Err(); err != nil {
		return management.ExecutionAuthority{}, err
	}
	if a.deps.ExecutionReady != nil && !a.deps.ExecutionReady() {
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	if a.deps.AuthorizeDeployment != nil {
		return a.deps.AuthorizeDeployment(ctx, actor, allocate)
	}
	if actor.Local && actor.UserID == "" && actor.TeamID == "" && !a.deps.ManagedExecution {
		return management.ExecutionAuthority{}, nil
	}
	return management.ExecutionAuthority{}, management.ErrExecutionAuthority
}

func (a *API) capturePreviewActor(ctx context.Context, projectID string) (management.ExecutionActorRef, error) {
	actor, err := management.CaptureDeploymentActor(ctx, projectID)
	if err == nil {
		_, err = a.authorizePreview(ctx, actor, false)
	}
	return actor, err
}

func previewAllocationID(prefix string) (string, error) {
	var v [12]byte
	if _, err := rand.Read(v[:]); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(v[:]), nil
}

func previewRecordContext(ctx context.Context) (context.Context, context.CancelFunc) {
	// Only settle/persist an already-issued operation under this context. It is
	// never passed to Build, Create or Start after the request was cancelled.
	return context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
}

func (a *API) startPreview(ctx context.Context, req previewruntime.Request, publish func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
	out := previewruntime.Outcome{Identity: req.Identity, Phase: "blocked", Message: "No preview allocation was started."}
	fail := func(code, message string, err error) (previewruntime.Outcome, error) {
		out.ReasonCode, out.Message = code, message
		if publish != nil {
			if e := publish(out); e != nil {
				return out, e
			}
		}
		return out, err
	}
	if publish == nil || req.ValidateSource == nil || req.Retained() || req.EngineID != "" || req.InstanceID != "" || req.ContainerID != "" || req.Image != "" || req.Actor.ProjectID != req.ProjectID || (req.Target != "local" && req.Target != "native-docker") {
		return fail("preview_request_invalid", "The reviewed preview request is invalid; no allocation was started.", errors.New("invalid preview request"))
	}
	releaseUnlock, lockErr := a.execution.lockChildLifecycle(ctx, "preview-release-"+req.ReleaseID)
	if lockErr != nil {
		return out, lockErr
	}
	defer releaseUnlock()
	executor, ok := a.deps.Backend.(backend.PreviewExecutor)
	if !ok {
		return fail("preview_runtime_unavailable", "This runtime does not support admitted local previews.", backend.ErrUnsupported)
	}
	if _, err := a.authorizePreview(ctx, req.Actor, true); err != nil {
		return fail("preview_forbidden", "Current deployment access is required.", err)
	}
	if err := req.ValidateSource(ctx); err != nil {
		return fail("preview_source_changed", "The reviewed source or configuration changed. Review the current build before deploying.", err)
	}
	buildID, err := previewAllocationID("preview-build-")
	if err != nil {
		return out, err
	}
	serveID, err := previewAllocationID("preview-")
	if err != nil {
		return out, err
	}
	resources := backend.Resources{MemoryMiB: req.MemoryMiB, CPUMillis: req.CPUMillis}
	if resources.MemoryMiB == 0 {
		resources.MemoryMiB = 1024
	}
	if resources.CPUMillis == 0 {
		resources.CPUMillis = 1000
	}
	id := backend.PreviewIdentity{Protocol: backend.PreviewExecutionProtocol, AllocationID: buildID, ReleaseID: req.ReleaseID, ScopeDigest: a.execution.scope, ProjectID: req.ProjectID, SourceDigest: req.SourceDigest, Commit: req.Commit}
	spec := backend.PreviewSpec{Identity: id, Kind: "preview-build", Resources: resources, SourceDirectory: req.SourceDirectory, SourceArchive: req.SourceArchive, Dockerfile: req.Dockerfile, Port: req.Port, Environment: req.Environment, Network: req.Network, Labels: req.Labels}
	prepared, err := executor.PreparePreview(ctx, spec)
	if err != nil {
		return fail("preview_build_unsupported", "The selected engine, source or base images cannot support this bounded preview build. Nothing was allocated.", err)
	}
	if !validPreviewIdentity(prepared.Spec.Identity) || prepared.Spec.Identity.AllocationID != buildID || prepared.Spec.Identity.ScopeDigest != a.execution.scope || prepared.Spec.Identity.ProjectID != req.ProjectID || prepared.Spec.Identity.ReleaseID != req.ReleaseID || prepared.Spec.Identity.SourceDigest != req.SourceDigest || prepared.Spec.Identity.Commit != req.Commit || prepared.Spec.Kind != "preview-build" || prepared.Spec.Resources != resources {
		return fail("preview_runtime_identity", "The prepared build identity changed; no allocation was started.", errors.New("invalid prepared preview"))
	}
	admit := func(p backend.PreparedPreview) (executionDecision, previewExecutionLease, error) {
		authority, e := a.authorizePreview(ctx, req.Actor, true)
		if e != nil {
			return executionDecision{}, previewExecutionLease{}, e
		}
		if e = req.ValidateSource(ctx); e != nil {
			return executionDecision{}, previewExecutionLease{}, e
		}
		policy, e := a.deps.Product.SwarmSchedulerPolicy()
		if e != nil {
			return executionDecision{}, previewExecutionLease{}, e
		}
		return a.execution.reservePreview(ctx, p, req.Actor, authority, policy)
	}
	decision, lease, err := admit(prepared)
	if err != nil || decision.State != "admitted" {
		return fail(decision.Code, "The preview build is waiting for current authority and shared machine capacity. No build was started.", err)
	}
	out.BuildAllocationID, out.AllocationID, out.EngineID, out.InstanceID = buildID, serveID, lease.Identity.EngineID, lease.Identity.InstanceID
	out.Phase, out.ResourcesHeld, out.Message = "reserved", true, "Shared capacity reserved; the reviewed image build has not started."
	if err = publish(out); err != nil {
		return out, err
	}
	unlock, err := a.execution.lockChildLifecycle(ctx, buildID)
	if err != nil {
		return out, err
	}
	defer unlock()
	// Re-read both admission and the exact phase after waiting for lifecycle.
	decision, lease, err = admit(prepared)
	if err != nil || decision.State != "admitted" {
		return fail(decision.Code, "Build admission changed; reserved capacity remains retained until explicit cleanup.", err)
	}
	lease, err = a.execution.transitionPreview(ctx, lease, "building", "", "")
	if err != nil {
		return out, err
	}
	out.Phase, out.Message = "building", "Building the exact reviewed source within the admitted resource limits."
	if err = publish(out); err != nil {
		return out, err
	}
	result, buildErr := executor.BuildPreview(ctx, prepared)
	if !result.Settled {
		if buildErr == nil {
			buildErr = errors.New("preview build settlement was not acknowledged")
		}
		out.Phase = "unknown"
		return fail("preview_build_unconfirmed", "The image build outcome is unconfirmed. Its capacity remains reserved; do not replay this build.", buildErr)
	}
	recordCtx, recordCancel := previewRecordContext(ctx)
	_, err = a.execution.completePreview(recordCtx, lease, true, result.ImageID)
	recordCancel()
	if err != nil {
		return out, err
	}
	out.Image, out.Phase, out.ResourcesHeld, out.CleanupConfirmed = result.ImageID, "built", false, true
	if buildErr != nil || result.ImageID == "" {
		out.Phase = "failed"
		out.ReasonCode = "preview_build_failed"
		out.Message = "The bounded image build ended without a usable application image."
	} else {
		out.Message = "The reviewed image build completed; serving capacity has not been allocated."
	}
	if err = publish(out); err != nil {
		return out, err
	}
	// Keep the uncharged build lineage until the whole release is explicitly retired.
	if buildErr != nil || result.ImageID == "" {
		return out, buildErr
	}
	if err = ctx.Err(); err != nil {
		return out, err
	}
	serveSpec := spec
	serveSpec.Kind = "preview"
	serveSpec.Identity = prepared.Spec.Identity
	serveSpec.Identity.AllocationID = serveID
	serveSpec.SourceDirectory = ""
	serveSpec.SourceArchive = nil
	serveSpec.ImageID = result.ImageID
	serving, err := executor.PreparePreview(ctx, serveSpec)
	if err != nil {
		out.Phase = "blocked"
		return fail("preview_serve_unsupported", "The built image cannot be served with the required isolation and resource limits.", err)
	}
	if serving.Spec.Identity != serveSpec.Identity || serving.Spec.Kind != "preview" || serving.Spec.Resources != resources || serving.Spec.ImageID != result.ImageID {
		return fail("preview_runtime_identity", "The prepared serving identity changed; no container was allocated.", errors.New("invalid prepared serving allocation"))
	}
	decision, lease, err = admit(serving)
	if err != nil || decision.State != "admitted" {
		out.Phase = "blocked"
		return fail(decision.Code, "The image is built; starting its preview awaits current authority and shared machine capacity.", err)
	}
	out.Phase, out.ResourcesHeld, out.CleanupConfirmed, out.Message = "reserved", true, false, "Serving capacity reserved; no application container has started."
	if err = publish(out); err != nil {
		return out, err
	}
	serveUnlock, err := a.execution.lockChildLifecycle(ctx, serveID)
	if err != nil {
		return out, err
	}
	defer serveUnlock()
	decision, lease, err = admit(serving)
	if err != nil || decision.State != "admitted" {
		return fail(decision.Code, "Serving admission changed; capacity remains retained for explicit cleanup.", err)
	}
	lease, err = a.execution.transitionPreview(ctx, lease, "creating", "", result.ImageID)
	if err != nil {
		return out, err
	}
	out.Phase, out.Message = "creating", "Creating one bounded application container for this reviewed release."
	if err = publish(out); err != nil {
		return out, err
	}
	ref, err := executor.CreatePreview(ctx, serving)
	if err != nil || !checkHex(ref, 32) {
		if err == nil {
			err = errors.New("preview Create returned no exact immutable identity")
		}
		out.Phase = "unknown"
		return fail("preview_create_unconfirmed", "Container creation was not acknowledged. Its capacity remains reserved; no retry was started.", err)
	}
	recordCtx, recordCancel = previewRecordContext(ctx)
	lease, err = a.execution.transitionPreview(recordCtx, lease, "created", ref, result.ImageID)
	recordCancel()
	if err != nil {
		return out, err
	}
	out.ContainerID, out.Phase, out.Message = ref, "created", "The exact container was created; application execution has not started."
	if err = publish(out); err != nil {
		return out, err
	}
	decision, lease, err = admit(serving)
	if err != nil || decision.State != "admitted" {
		return fail(decision.Code, "Container Start was refused by current authority or capacity. Use Stop to settle the retained allocation.", err)
	}
	lease, err = a.execution.transitionPreview(ctx, lease, "starting", ref, result.ImageID)
	if err != nil {
		return out, err
	}
	out.Phase, out.Message = "starting", "Starting the exact reviewed application container."
	if err = publish(out); err != nil {
		return out, err
	}
	if err = executor.StartPreview(ctx, lease.Identity, ref); err != nil {
		out.Phase = "unknown"
		return fail("preview_start_unconfirmed", "Application Start was not acknowledged. Its capacity remains held; inspect before taking another action.", err)
	}
	recordCtx, recordCancel = previewRecordContext(ctx)
	lease, err = a.execution.transitionPreview(recordCtx, lease, "running", ref, result.ImageID)
	recordCancel()
	if err != nil {
		return out, err
	}
	out.Phase, out.Message = "running", "The application container is running; verify its HTTP health and behavior."
	observed, err := executor.InspectPreview(ctx, lease.Identity, ref)
	if err == nil && observed.Ref == ref && observed.State == "running" {
		out.URL = observed.URL
	} else {
		out.Phase = "unknown"
		out.ReasonCode = "preview_observation_unavailable"
		out.Message = "The started application's current ownership or state could not be observed."
		if err == nil {
			err = errors.New("preview ownership or running state was not confirmed")
		}
	}
	if e := publish(out); e != nil {
		return out, e
	}
	return out, err
}

func matchesPreviewReference(v previewExecutionLease, id previewruntime.Identity) bool {
	return v.Identity.ProjectID == id.ProjectID && v.Identity.ReleaseID == id.ReleaseID && v.Identity.SourceDigest == id.SourceDigest && v.Identity.Commit == id.Commit && v.Identity.EngineID == id.EngineID && v.Identity.InstanceID == id.InstanceID &&
		(v.Kind == "preview-build" && v.ID == id.BuildAllocationID || v.Kind == "preview" && v.ID == id.AllocationID && v.Image == id.Image && (id.ContainerID == "" || v.Ref == id.ContainerID))
}

func (a *API) observePreview(ctx context.Context, id previewruntime.Identity) (previewruntime.Outcome, error) {
	out := previewruntime.Outcome{Identity: id, Phase: "unknown", ResourcesHeld: true, Message: "Preview ownership or execution remains unconfirmed."}
	if !id.Retained() {
		return out, errors.New("preview allocation identity missing")
	}
	if id.AllocationID != "" {
		v, e := a.execution.previewLease(ctx, id.AllocationID)
		if e == nil {
			if !matchesPreviewReference(v, id) {
				return out, errors.New("preview ownership changed")
			}
			out.Phase = v.State
			out.ResourcesHeld = v.State != "finished"
			out.CleanupConfirmed = v.State == "finished"
			if v.Ref == "" || v.State == "finished" {
				return out, nil
			}
			executor, ok := a.deps.Backend.(backend.PreviewExecutor)
			if !ok {
				return out, backend.ErrUnsupported
			}
			observed, e := executor.InspectPreview(ctx, v.Identity, v.Ref)
			if e != nil {
				return out, e
			}
			if observed.Ref != v.Ref {
				return out, errors.New("preview observation identity changed")
			}
			out.Phase = observed.State
			out.URL = observed.URL
			// An observed absence is read-only evidence, never a lease release.
			return out, nil
		}
		if !errors.Is(e, errPreviewLeaseAbsent) || id.ContainerID != "" {
			return out, e
		}
	}
	if id.BuildAllocationID != "" {
		v, e := a.execution.previewLease(ctx, id.BuildAllocationID)
		if e == nil && matchesPreviewReference(v, id) {
			out.Phase = v.State
			out.ResourcesHeld = v.State != "finished"
			out.CleanupConfirmed = v.State == "finished"
			return out, nil
		}
	}
	return out, errors.New("exact preview lease is unavailable; its retained history is not a capacity proof")
}

func (a *API) stopPreview(ctx context.Context, id previewruntime.Identity, publish func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
	out := previewruntime.Outcome{Identity: id, Phase: "unknown", ResourcesHeld: true, Message: "Preview cleanup is not confirmed."}
	actor, err := a.capturePreviewActor(ctx, id.ProjectID)
	if err != nil {
		return out, err
	}
	if publish == nil || !checkLogicalID(id.ReleaseID) || !id.Retained() {
		return out, errors.New("preview result publisher or identity missing")
	}
	releaseUnlock, err := a.execution.lockChildLifecycle(ctx, "preview-release-"+id.ReleaseID)
	if err != nil {
		return out, err
	}
	defer releaseUnlock()
	// The release lock excludes build→serve transitions. Missing serving state is
	// only safe when the finished builder proves it never acquired an allocation
	// and the persisted product receipt has no acknowledged container.
	build, err := a.execution.previewLease(ctx, id.BuildAllocationID)
	if err != nil || !matchesPreviewReference(build, id) {
		return out, errors.New("exact preview build lineage unavailable")
	}
	serve, serveErr := a.execution.previewLease(ctx, id.AllocationID)
	missingServe := errors.Is(serveErr, errPreviewLeaseAbsent)
	if serveErr != nil && !missingServe {
		return out, serveErr
	}
	if missingServe && id.ContainerID != "" {
		return out, errors.New("known preview container has no exact capacity receipt")
	}
	if !missingServe && !matchesPreviewReference(serve, id) {
		return out, errors.New("preview serving identity changed")
	}
	rows := []previewExecutionLease{build}
	if !missingServe {
		rows = append(rows, serve)
	}
	settled := make([]previewExecutionLease, 0, len(rows))
	for _, initial := range rows {
		unlock, e := a.execution.lockChildLifecycle(ctx, initial.ID)
		if e != nil {
			return out, e
		}
		v, e := func() (previewExecutionLease, error) {
			v, e := a.execution.previewLease(ctx, initial.ID)
			if e != nil {
				return v, e
			}
			if v != initial {
				return v, errors.New("preview phase changed during cleanup")
			}
			if _, e = a.authorizePreview(ctx, actor, false); e != nil {
				return v, e
			}
			if v.State == "finished" {
				return v, nil
			}
			if v.State == "reserved" {
				return a.execution.completePreview(ctx, v, true, v.Image)
			}
			if v.Kind == "preview-build" || v.Ref == "" {
				return v, errors.New("unacknowledged preview build/create requires exact settlement recovery; capacity is retained")
			}
			executor, ok := a.deps.Backend.(backend.PreviewExecutor)
			if !ok {
				return v, backend.ErrUnsupported
			}
			if v.State != "cleaning" {
				v, e = a.execution.transitionPreview(ctx, v, "cleaning", v.Ref, v.Image)
				if e != nil {
					return v, e
				}
			}
			out.Phase, out.Message = "cleaning", "Stopping only the exact owned preview allocation; cleanup remains unconfirmed."
			if e = publish(out); e != nil {
				return v, e
			}
			if e = executor.RemovePreview(ctx, v.Identity, v.Ref); e != nil {
				return v, e
			}
			observed, e := executor.InspectPreview(ctx, v.Identity, v.Ref)
			if e != nil || observed.Ref != v.Ref || observed.State != "absent" {
				return v, errors.New("exact preview absence is not confirmed; capacity remains held")
			}
			recordCtx, cancel := previewRecordContext(ctx)
			defer cancel()
			return a.execution.completePreview(recordCtx, v, true, v.Image)
		}()
		unlock()
		if e != nil {
			return out, e
		}
		settled = append(settled, v)
	}
	out.Phase, out.URL, out.ResourcesHeld, out.CleanupConfirmed, out.Message = "finished", "", false, true, "The exact preview allocations are settled; their capacity is released and release history is retained."
	if err = publish(out); err != nil {
		return out, err
	}
	// One atomic acknowledgement avoids deleting only half the dual-ID lineage.
	recordCtx, cancel := previewRecordContext(ctx)
	defer cancel()
	err = a.execution.acknowledgePreviewRelease(recordCtx, settled)
	return out, err
}

func (a *API) restartPreview(ctx context.Context, req previewruntime.Request, publish func(previewruntime.Outcome) error) (previewruntime.Outcome, error) {
	out := previewruntime.Outcome{Identity: req.Identity, Phase: "unknown", ResourcesHeld: true, Message: "The retained preview has not been restarted."}
	if publish == nil || req.ValidateSource == nil || req.AllocationID == "" {
		return out, errors.New("reviewed restart identity is missing")
	}
	actor, err := a.capturePreviewActor(ctx, req.ProjectID)
	if err != nil {
		return out, err
	}
	releaseUnlock, err := a.execution.lockChildLifecycle(ctx, "preview-release-"+req.ReleaseID)
	if err != nil {
		return out, err
	}
	defer releaseUnlock()
	unlock, err := a.execution.lockChildLifecycle(ctx, req.AllocationID)
	if err != nil {
		return out, err
	}
	defer unlock()
	v, err := a.execution.previewLease(ctx, req.AllocationID)
	if err != nil {
		return out, err
	}
	if !matchesPreviewReference(v, req.Identity) || v.Kind != "preview" || v.State != "running" {
		return out, errors.New("only an acknowledged running preview supports restart; settle uncertain work first")
	}
	executor, ok := a.deps.Backend.(backend.PreviewExecutor)
	if !ok {
		return out, backend.ErrUnsupported
	}
	admit := func() (previewExecutionLease, error) {
		if _, e := a.authorizePreview(ctx, actor, true); e != nil {
			return v, e
		}
		authority, e := a.authorizePreview(ctx, v.Actor, true)
		if e != nil {
			return v, e
		}
		if e = req.ValidateSource(ctx); e != nil {
			return v, e
		}
		policy, e := a.deps.Product.SwarmSchedulerPolicy()
		if e != nil {
			return v, e
		}
		// The durable original actor owns capacity; the current caller supplies
		// fresh control authority. The original actor is independently reauthorized
		// for the durable allocation policy; neither grants the other new rights.
		prepared := backend.PreparedPreview{Spec: backend.PreviewSpec{Identity: v.Identity, Kind: v.Kind, Resources: v.Resources, ImageID: v.Image}, RequiredDiskBytes: v.CreateDiskBytes}
		decision, next, e := a.execution.reservePreview(ctx, prepared, v.Actor, authority, policy)
		if e != nil {
			return v, e
		}
		if decision.State != "admitted" {
			return v, errors.New(decision.Message)
		}
		return next, nil
	}
	v, err = admit()
	if err != nil {
		return out, err
	}
	v, err = a.execution.transitionPreview(ctx, v, "stopping", v.Ref, v.Image)
	if err != nil {
		return out, err
	}
	out.Phase, out.Message = "stopping", "Stopping the exact owned preview before a reviewed restart; capacity remains reserved."
	if err = publish(out); err != nil {
		return out, err
	}
	if err = executor.StopPreview(ctx, v.Identity, v.Ref); err != nil {
		return out, err
	}
	observed, err := executor.InspectPreview(ctx, v.Identity, v.Ref)
	if err != nil || observed.Ref != v.Ref || observed.State != "exited" {
		return out, errors.New("preview stop is not confirmed; no restart was sent")
	}
	v, err = a.execution.transitionPreview(ctx, v, "stopped", v.Ref, v.Image)
	if err != nil {
		return out, err
	}
	out.Phase, out.URL = "stopped", ""
	if err = publish(out); err != nil {
		return out, err
	}
	v, err = admit()
	if err != nil {
		return out, err
	}
	v, err = a.execution.transitionPreview(ctx, v, "starting", v.Ref, v.Image)
	if err != nil {
		return out, err
	}
	out.Phase, out.Message = "starting", "Restarting the same admitted preview after fresh source and permission checks."
	if err = publish(out); err != nil {
		return out, err
	}
	if err = executor.StartPreview(ctx, v.Identity, v.Ref); err != nil {
		return out, err
	}
	v, err = a.execution.transitionPreview(ctx, v, "running", v.Ref, v.Image)
	if err != nil {
		return out, err
	}
	observed, err = executor.InspectPreview(ctx, v.Identity, v.Ref)
	if err != nil || observed.Ref != v.Ref || observed.State != "running" {
		return out, errors.New("restarted preview could not be freshly observed")
	}
	out.Phase, out.URL, out.Message = "running", observed.URL, "The exact preview restarted; recheck application health and behavior."
	return out, publish(out)
}

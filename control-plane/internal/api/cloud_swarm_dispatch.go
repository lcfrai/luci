package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// dispatchCloudSwarm is selected only by an explicitly reviewed cloud route.
// The controller checkpoints Create, transfer and Start separately; a failed
// step keeps its exact VM/reservation for observation instead of replaying it.
func (a *API) dispatchCloudSwarm(ctx context.Context, req product.SwarmDispatchRequest, run product.ProjectRun, op product.ProjectRunOperation) (product.SwarmDispatchResult, error) {
	blocked := func(code, message string) (product.SwarmDispatchResult, error) {
		return product.SwarmDispatchResult{State: "blocked", ReasonCode: code, Message: message}, nil
	}
	unknown := func(code string) (product.SwarmDispatchResult, error) {
		return product.SwarmDispatchResult{State: "unknown", SessionID: op.ChildID, ReasonCode: code, Message: "The cloud task's exact VM or upstream launch is unconfirmed. Its owned attempt is retained and will not be replayed."}, nil
	}
	cloud := a.deps.CloudTasks
	if cloud == nil {
		return blocked("cloud_microvm_unavailable", "A verified cloud microVM execution service is not configured.")
	}
	remaining, _ := cloud.Runtime.Controller.ReconcileRestored(ctx, 8)
	prior, found := a.cloudRecord(req.ProjectID, req.RunID, req.OperationID, op.ChildID)
	if remaining != 0 {
		if found {
			return unknown("cloud_restore_pending")
		}
		return product.SwarmDispatchResult{State: "waiting", ReasonCode: "cloud_restore_pending", Message: "Restored cloud VMs are being checked against their exact provider identities. Their ownership and charges remain visible; no new VM will start until reconciliation finishes."}, nil
	}
	if found {
		if prior.State == e2bbe.Creating || prior.State == e2bbe.Unknown {
			var reconcileErr error
			prior, reconcileErr = cloud.Runtime.Controller.Reconcile(ctx, op.ChildID)
			if reconcileErr != nil {
				return unknown("cloud_create_unconfirmed")
			}
		}
		if prior.CancelRequested || prior.State != e2bbe.Allocated {
			return unknown("cloud_attempt_unconfirmed")
		}
	}
	if found && prior.LaunchRequested {
		old := prior
		if old.State != e2bbe.Allocated || !old.LaunchRequested || old.ProcessPID == 0 ||
			e2bbe.ValidateCaptureIdentity(old.LaunchKernel, old) != nil {
			return unknown("cloud_attempt_unconfirmed")
		}
		if _, err := a.deps.Product.RecordProjectSwarmMicroVMLaunch(ctx, req.ProjectID, req.RunID, op.ID, old); err != nil {
			return unknown("cloud_launch_review_pending")
		}
		return product.SwarmDispatchResult{State: "running", SessionID: op.ChildID, ReasonCode: "cloud_running", Message: "The exact upstream agent is running in its owned microVM."}, nil
	}
	if run.Input.Route.Harness != "codex" && run.Input.Route.Harness != "claude-code" || run.Input.Route.PermissionMode != "bypass" {
		return blocked("cloud_harness_unsupported", "This cloud run needs an explicitly reviewed noninteractive Codex or Claude Code route.")
	}
	selected := run.Input.Route.Connection.ID
	connection, agentEnv, model, err := a.resolveSessionAgent(run.Input.Route.Harness, &selected, registry.Session{}, run.Input.Route.Model)
	if err != nil || connection.ID != selected || connection.Revision != run.Input.Route.Connection.Revision ||
		connection.Kind != run.Input.Route.Connection.Kind || connection.Protocol != run.Input.Route.Connection.Protocol ||
		connection.AuthMode != "api-key" || model != run.Input.Route.Model {
		return blocked("provider_changed", "The saved API-key model connection changed or is unavailable; review the cloud run before dispatch.")
	}
	keyName := "OPENAI_API_KEY"
	if run.Input.Route.Harness == "claude-code" {
		keyName = "ANTHROPIC_API_KEY"
	}
	if agentEnv[keyName] == "" {
		return blocked("provider_unavailable", "Cloud microVM tasks require an explicitly saved upstream API-key connection; local account files are not transferred.")
	}
	for key := range agentEnv {
		switch key {
		case "OPENAI_API_KEY", "ANTHROPIC_API_KEY", "OPENAI_BASE_URL", "ANTHROPIC_BASE_URL", "OPENAI_ORG_ID", "OPENAI_PROJECT_ID":
		default:
			return blocked("provider_unavailable", "This connection requires host-only configuration that cannot be transferred into the isolated cloud task.")
		}
	}
	if !cloud.allowsModelEndpoint(run.Input.Route.Harness, agentEnv) {
		return blocked("cloud_model_egress_unavailable", "The selected upstream model endpoint is outside this cloud microVM's operator-approved egress domains; no VM was started.")
	}
	if !found {
		if cloud.Runtime.Controller.VerifyTemplateBuild(ctx) != nil {
			return blocked("cloud_template_unavailable", "The selected microVM template build is not confirmed ready; no VM was started.")
		}
		if err = cloud.Runtime.Controller.VerifyNodeCapacity(ctx); errors.Is(err, e2bbe.ErrCapacity) {
			return product.SwarmDispatchResult{State: "waiting", ReasonCode: "cloud_host_capacity", Message: "The selected microVM host has no observed free VM slot, CPU or memory; no VM was started."}, nil
		} else if err != nil {
			return blocked("cloud_host_unavailable", "The selected microVM host's authenticated capacity could not be confirmed; no VM was started.")
		}
	}
	input, err := a.deps.Product.BuildProjectSwarmMicroVMInput(ctx, req.ProjectID, req.RunID, op.ID,
		cloud.Runtime.Controller.OwnerID(), cloud.MaxSeconds, cloud.EstimatedCostMicros)
	if err != nil {
		return blocked("cloud_input_changed", "The reviewed Git source or task context changed before cloud allocation; no VM was started.")
	}
	if req.Actor.UserID == "" || req.Actor.Local || req.Actor.ProjectID != req.ProjectID {
		return blocked("cloud_actor_unavailable", "A saved, current account identity is required before cloud allocation.")
	}
	input.Request.BillingUserID = req.Actor.UserID
	input.Request.BillingTeamID = req.Actor.TeamID
	if found && prior.Request != input.Request {
		return unknown("cloud_input_changed") // retained Create belongs to another immutable task input
	}
	enabled, err := a.deps.Product.SwarmSchedulerDispatchEnabled()
	if err != nil || !enabled {
		return blocked("dispatch_paused", "New cloud task dispatch is paused; the exact owned attempt remains retained.")
	}
	command, args, err := harness.BuildTask(harness.Request{Harness: harness.ID(run.Input.Route.Harness), Model: model,
		Permission: harness.PermissionBypass, ReviewedContext: true, OpeningPrompt: input.OpeningPrompt, Effort: harness.Effort(run.Input.Route.Effort)})
	if err != nil {
		return blocked("cloud_harness_unsupported", err.Error())
	}
	launch := e2bbe.LaunchSpec{Command: command, Args: args, Environment: agentEnv,
		AttachToken: cloud.attachToken(input.Request), ConnectionRevision: "r-" + strconv.Itoa(connection.Revision)}
	started, err := cloud.Runtime.Start(ctx, input.Request, bytes.NewReader(input.Archive), launch)
	if err != nil {
		if errors.Is(err, e2bbe.ErrCapacity) && started.RequestDigest == "" {
			return product.SwarmDispatchResult{State: "waiting", ReasonCode: "cloud_capacity", Message: "The configured cloud VM, project or spending limit has been reached."}, nil
		}
		a.sessionLog().Warn("cloud task start retained for reconciliation", "operationId", op.ID, "err", err)
		return unknown("cloud_launch_unconfirmed")
	}
	if started.Request.Identity.OperationID != op.ChildID || started.ProcessPID == 0 {
		return unknown("cloud_launch_unconfirmed")
	}
	guest, err := cloud.Runtime.Controller.Guest(ctx, op.ChildID)
	if err != nil {
		return unknown("cloud_launch_observation_pending")
	}
	observed, observeErr := cloud.Runtime.Controller.ObserveLaunchKernel(ctx, op.ChildID, guest)
	guest.Close()
	if observeErr != nil || e2bbe.ValidateCaptureIdentity(observed.LaunchKernel, observed) != nil {
		return unknown("cloud_launch_observation_pending")
	}
	if _, err = a.deps.Product.RecordProjectSwarmMicroVMLaunch(ctx, req.ProjectID, req.RunID, op.ID, observed); err != nil {
		a.sessionLog().Warn("cloud launch observed but product join pending", "operationId", op.ID, "err", err)
		return unknown("cloud_launch_review_pending")
	}
	return product.SwarmDispatchResult{State: "running", SessionID: op.ChildID, ReasonCode: "cloud_running",
		Message: fmt.Sprintf("%s is running in an owned cloud microVM. Changes and checks remain unverified.", run.Input.Route.Harness)}, nil
}

// resumeCloudSwarm is scheduler-owned recovery of an existing exact VM after
// a lost Create/input acknowledgement. It refuses a missing or changed record
// before the ordinary dispatch authority/source checks; a second Create is
// never used as a recovery probe.
func (a *API) resumeCloudSwarm(ctx context.Context, req product.SwarmDispatchRequest) (product.SwarmDispatchResult, error) {
	run, err := a.deps.Product.GetProjectSwarmRun(req.ProjectID, req.RunID)
	if err != nil || run.State == "cancelled" || run.Swarm == nil || run.Swarm.State != "ready" {
		return product.SwarmDispatchResult{State: "unknown", ReasonCode: "cloud_resume_not_authorized"}, nil
	}
	for _, op := range run.Operations {
		if op.ID != req.OperationID {
			continue
		}
		row, found := a.cloudRecord(req.ProjectID, req.RunID, op.ID, op.ChildID)
		if !found || row.State != e2bbe.Allocated || row.CancelRequested || row.LaunchRequested || !row.ResourcesHeld {
			return product.SwarmDispatchResult{State: "unknown", ReasonCode: "cloud_resume_unconfirmed"}, nil
		}
		return a.dispatchQueuedSwarm(ctx, req)
	}
	return product.SwarmDispatchResult{State: "unknown", ReasonCode: "cloud_resume_unconfirmed"}, nil
}

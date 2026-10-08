package api

import (
	"context"
	"errors"
	"os"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func (a *API) authorizeChecks(ctx context.Context, actor management.ExecutionActorRef) (management.ExecutionAuthority, error) {
	if err := ctx.Err(); err != nil {
		return management.ExecutionAuthority{}, err
	}
	if a.deps.ExecutionReady != nil && !a.deps.ExecutionReady() {
		return management.ExecutionAuthority{}, management.ErrExecutionAuthority
	}
	if a.deps.AuthorizeChecks != nil {
		return a.deps.AuthorizeChecks(ctx, actor)
	}
	if actor.Local && actor.UserID == "" && actor.TeamID == "" && !a.deps.ManagedExecution {
		return management.ExecutionAuthority{}, nil
	}
	return management.ExecutionAuthority{}, management.ErrExecutionAuthority
}

func (a *API) captureCheckActor(ctx context.Context, projectID string) (management.ExecutionActorRef, error) {
	actor, err := management.CaptureExecutionActor(ctx, projectID)
	if err != nil {
		return actor, err
	}
	_, err = a.authorizeChecks(ctx, actor)
	return actor, err
}

// executeProjectCheck is the only new-check allocation path. The product
// persists approved commands first. No Docker call is made until fresh actor
// authority and the shared cross-account resource ledger both admit it.
func (a *API) executeProjectCheck(ctx context.Context, req product.ProjectCheckCommandRequest, persist func(product.ProjectCheckCommandOutcome) error) (product.ProjectCheckCommandOutcome, error) {
	out := product.ProjectCheckCommandOutcome{Status: "blocked", Stage: "admission", AllocationID: req.AllocationID, Message: "No check command was started."}
	publish := func() (product.ProjectCheckCommandOutcome, error) {
		if persist == nil {
			return out, errors.New("check outcome publisher missing")
		}
		return out, persist(out)
	}
	fail := func(stage, code, message string, unknown bool) (product.ProjectCheckCommandOutcome, error) {
		out.Stage, out.ReasonCode, out.Message = stage, code, message
		if unknown {
			out.Status = "unknown"
		}
		return publish()
	}
	if req.Actor.ProjectID != req.ProjectID || !checkLogicalID(req.AllocationID) || !checkLogicalID(req.CheckID) || req.CommandIndex < 0 || req.CommandIndex >= 8 {
		return fail("admission", "check_scope_invalid", "The approved check command identity cannot be verified.", false)
	}
	executor, ok := a.deps.Backend.(backend.CheckExecutor)
	if !ok {
		return fail("admission", "check_execution_unavailable", "The configured runtime does not support isolated check execution.", false)
	}
	authority, err := a.authorizeChecks(ctx, req.Actor)
	if err != nil {
		return fail("admission", "check_execution_forbidden", management.ErrExecutionAuthority.Error(), false)
	}
	uid, gid := os.Getuid(), os.Getgid()
	if uid < 0 {
		uid = 1001
	}
	if gid < 0 {
		gid = 1001
	}
	spec := backend.CheckSpec{Identity: backend.CheckIdentity{Protocol: backend.CheckExecutionProtocol, AllocationID: req.AllocationID, CheckID: req.CheckID, CommandIndex: req.CommandIndex, ScopeDigest: a.execution.scope, ProjectID: req.ProjectID}, Executable: req.Command.Executable, Args: append([]string{}, req.Command.Args...), SourceDirectory: req.SourceDirectory, SourceDigest: req.SourceDigest, Resources: backend.Resources{MemoryMiB: req.MemoryMiB, CPUMillis: req.CPUMillis}, UserID: uid, GroupID: gid}
	spec.Identity.CommandDigest = backend.CheckCommandDigest(spec)
	prepared, err := executor.PrepareCheck(ctx, spec)
	if errors.Is(err, backend.ErrResourceLimitsUnsupported) {
		return fail("prepare", "resource_limits_unsupported", "The configured Docker engine cannot enforce the requested memory, combined memory/swap, CPU and process limits. No check command was started.", false)
	}
	if err != nil || prepared.Spec.Identity.CommandDigest != spec.Identity.CommandDigest || backend.CheckCommandDigest(prepared.Spec) != spec.Identity.CommandDigest || prepared.Spec.Identity.AllocationID != spec.Identity.AllocationID || !validCheckIdentity(prepared.Spec.Identity) {
		return fail("prepare", "check_runtime_unavailable", "The configured Docker engine and check image could not be verified. No command was started.", false)
	}
	policy, err := a.deps.Product.SwarmSchedulerPolicy()
	if err != nil {
		return fail("admission", "resource_policy_unavailable", "Current shared execution policy is unavailable. No command was started.", false)
	}
	decision, err := a.execution.reserveCheck(ctx, prepared, req.Actor, authority, policy)
	if errors.Is(err, errCheckResourcePolicy) {
		return fail("admission", "resource_policy_changed", "The saved check resource limits no longer satisfy current policy. Review the check before starting a new attempt.", false)
	}
	if err != nil {
		return fail("admission", "check_admission_unavailable", "Current check authority or shared resource admission could not be verified. No command was started.", false)
	}
	if decision.State != "admitted" {
		return fail("admission", decision.Code, decision.Message, false)
	}
	out.ResourcesHeld = true
	release, err := a.execution.lockChildLifecycle(ctx, req.AllocationID)
	if err != nil {
		return fail("lifecycle", "check_lifecycle_busy", "The exact check launch is being coordinated. Its reservation is retained.", true)
	}
	defer release()
	identity := prepared.Spec.Identity
	// A controller can wait here while another process settles and acknowledges
	// its reserved attempt. Validate ownership before publishing cached progress
	// or re-running admission, which otherwise could recreate an absent lease.
	if err = a.execution.requireReservedCheck(identity, req.Actor, prepared.Spec.Resources); err != nil {
		return out, err
	}
	progress := out
	progress.Status, progress.Stage, progress.ReasonCode, progress.Message = "running", "admission", "", "Shared capacity is reserved; the exact command launch is pending."
	if err = persist(progress); err != nil {
		return out, err
	}
	// Authority/capacity can change while source/image preparation is underway.
	// Recheck immediately before both irreversible RPC boundaries.
	admit := func() (string, string) {
		authority, err = a.authorizeChecks(ctx, req.Actor)
		if err != nil {
			return "check_execution_forbidden", management.ErrExecutionAuthority.Error()
		}
		policy, err = a.deps.Product.SwarmSchedulerPolicy()
		if err != nil {
			return "resource_policy_unavailable", "Current shared execution policy is unavailable."
		}
		decision, err = a.execution.reserveCheck(ctx, prepared, req.Actor, authority, policy)
		if errors.Is(err, errCheckResourcePolicy) {
			return "resource_policy_changed", "The saved check resource limits no longer satisfy current policy."
		}
		if err != nil {
			return "check_admission_unavailable", "Current check authority or shared resource admission could not be verified."
		}
		return decision.Code, decision.Message
	}
	if code, message := admit(); code != "" {
		out.Stage, out.ReasonCode, out.Message = "admission", code, message
		return a.finishUnstartedCheck(ctx, identity, out, persist, true)
	}
	if err = a.execution.transitionCheck(ctx, identity, "reserved", "creating", ""); err != nil {
		return fail("launch-journal", "check_launch_unconfirmed", "The check launch claim could not be saved. Its reservation remains retained.", true)
	}
	ref, err := executor.CreateCheck(ctx, prepared)
	if err != nil || !checkHex(ref, 32) {
		return fail("create", "check_create_unconfirmed", "Docker check creation is unconfirmed. Review this exact attempt before starting another command.", true)
	}
	ackCtx, ackCancel := context.WithTimeout(context.Background(), 5*time.Second)
	err = a.execution.transitionCheck(ackCtx, identity, "creating", "created", ref)
	ackCancel()
	if err != nil {
		return fail("launch-journal", "check_create_acknowledgement_unconfirmed", "The Docker creation acknowledgment could not be saved. Its exact attempt and capacity remain retained.", true)
	}
	if code, message := admit(); code != "" {
		out.Stage, out.ReasonCode, out.Message = "admission", code, message
		return a.finishAcknowledgedCheck(context.Background(), identity, ref, out, persist, true, true)
	}
	if err = a.execution.transitionCheck(ctx, identity, "created", "starting", ref); err != nil {
		return fail("launch-journal", "check_start_unconfirmed", "The check start claim could not be saved. Its exact container remains retained.", true)
	}
	if err = executor.StartCheck(ctx, identity, ref); err != nil {
		return fail("start", "check_start_unconfirmed", "Docker check start is unconfirmed. No start will be replayed; review the exact retained attempt.", true)
	}
	ackCtx, ackCancel = context.WithTimeout(context.Background(), 5*time.Second)
	err = a.execution.transitionCheck(ackCtx, identity, "starting", "running", ref)
	ackCancel()
	if err != nil {
		return fail("launch-journal", "check_start_acknowledgement_unconfirmed", "The check start acknowledgment could not be saved. The exact command outcome remains unconfirmed.", true)
	}
	cancelled := func() (product.ProjectCheckCommandOutcome, error) {
		out.Status, out.ExitCode, out.Stage, out.ReasonCode, out.Message = "cancelled", nil, "cleanup", "check_cancelled", "The check was cancelled. Output captured before exact removal may be incomplete."
		capture, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		logs, e := executor.CheckLogs(capture, identity, ref, 256<<10)
		cancel()
		if e == nil {
			out.Output, out.Truncated = logs.Text, logs.Truncated
		}
		return a.finishAcknowledgedCheck(context.Background(), identity, ref, out, persist, true, true)
	}
	for {
		if ctx.Err() != nil {
			return cancelled()
		}
		observed, observeErr := executor.InspectCheck(ctx, identity, ref)
		if ctx.Err() != nil {
			return cancelled()
		}
		if observeErr != nil || observed.Ref != ref {
			return fail("observe", "check_observation_unavailable", "The exact check process state could not be verified. Its capacity remains retained.", true)
		}
		if observed.State == "exited" && observed.ExitCode != nil {
			logs, logErr := executor.CheckLogs(ctx, identity, ref, 256<<10)
			if logErr != nil {
				return fail("output", "check_output_unavailable", "The stopped command output could not be captured. Its container remains retained for retry.", true)
			}
			out.Status, out.ExitCode, out.Output, out.Truncated = "passed", observed.ExitCode, logs.Text, logs.Truncated
			if *observed.ExitCode != 0 {
				out.Status = "failed"
			}
			out.Stage, out.ReasonCode, out.Message = "cleanup", "", "The command exit and output were recorded; exact container cleanup is pending."
			return a.finishAcknowledgedCheck(context.Background(), identity, ref, out, persist, false, true)
		}
		if observed.State != "running" {
			return fail("observe", "check_outcome_unconfirmed", "The check did not provide a verified terminal exit. No command will be replayed.", true)
		}
		timer := time.NewTimer(500 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return cancelled()
		case <-timer.C:
		}
	}
}

// A durable Create ACK pins the only object any delayed Start can address.
// Removal of that immutable object plus confirmed absence closes both RPCs;
// it never establishes anything about an unacknowledged Create.
func (a *API) finishAcknowledgedCheck(ctx context.Context, identity backend.CheckIdentity, ref string, out product.ProjectCheckCommandOutcome, persist func(product.ProjectCheckCommandOutcome) error, force, deferAcknowledgement bool, control ...func(context.Context) error) (product.ProjectCheckCommandOutcome, error) {
	cleanup, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	executor := a.deps.Backend.(backend.CheckExecutor)
	out.ResourcesHeld = true
	if err := persist(out); err != nil {
		return out, err
	}
	if err := a.execution.markCheckCleaning(cleanup, identity, ref); err != nil {
		return out, err
	}
	if len(control) > 0 && control[0] != nil {
		if err := control[0](cleanup); err != nil {
			return out, err
		}
	}
	if err := executor.RemoveCheck(cleanup, identity, ref, force); err != nil {
		out.Stage, out.ReasonCode, out.Message = "cleanup", "check_cleanup_unconfirmed", "The command result is retained, but exact Docker cleanup is unconfirmed. Its capacity remains held."
		return out, persist(out)
	}
	observed, err := executor.InspectCheck(cleanup, identity, ref)
	if err != nil || observed.Ref != ref || observed.State != "absent" {
		out.Stage, out.ReasonCode, out.Message = "cleanup", "check_cleanup_unconfirmed", "The exact Docker object has not been confirmed absent. Its capacity remains held."
		return out, persist(out)
	}
	if len(control) > 0 && control[0] != nil {
		if err := control[0](cleanup); err != nil {
			return out, err
		}
	}
	out.CleanupConfirmed = true
	if err := a.execution.completeCheck(cleanup, identity, ref, out); err != nil {
		return out, err
	}
	out.ResourcesHeld = false
	if err := persist(out); err != nil {
		return out, err
	}
	if deferAcknowledgement {
		return out, nil
	}
	if err := a.execution.acknowledgeCheckCompletion(cleanup, identity); err != nil {
		return out, err
	}
	return out, nil
}

func (a *API) finishUnstartedCheck(ctx context.Context, identity backend.CheckIdentity, out product.ProjectCheckCommandOutcome, persist func(product.ProjectCheckCommandOutcome) error, deferAcknowledgement bool) (product.ProjectCheckCommandOutcome, error) {
	out.CleanupConfirmed = true
	if err := persist(out); err != nil {
		return out, err
	}
	bounded, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := a.execution.completeCheck(bounded, identity, "", out); err != nil {
		return out, err
	}
	out.ResourcesHeld = false
	if err := persist(out); err != nil {
		return out, err
	}
	if deferAcknowledgement {
		return out, nil
	}
	if err := a.execution.acknowledgeCheckCompletion(bounded, identity); err != nil {
		return out, err
	}
	return out, nil
}

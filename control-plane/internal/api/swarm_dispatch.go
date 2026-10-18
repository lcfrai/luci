package api

import (
	"context"
	"errors"
	"path/filepath"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/harness"
	"github.com/lcfrai/luci/control-plane/internal/ids"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

func (a *API) authorizeExecution(ctx context.Context, actor management.ExecutionActorRef, harnessID string, secrets bool) (management.ExecutionAuthority, error) {
	if err := ctx.Err(); err != nil {
		return management.ExecutionAuthority{}, err
	}
	if a.deps.AuthorizeExecution != nil {
		return a.deps.AuthorizeExecution(ctx, actor, harnessID, secrets)
	}
	if actor.Local && actor.UserID == "" && actor.TeamID == "" && !a.deps.ManagedExecution {
		return management.ExecutionAuthority{}, nil
	}
	return management.ExecutionAuthority{}, management.ErrExecutionAuthority
}
func providerCapacity(route product.ProjectRunRoute, policy product.SwarmSchedulerPolicy) (string, int) {
	key := route.Connection.ID
	if route.Connection.Kind == "codex-subscription" || route.Connection.AuthMode == "chatgpt" {
		return "codex-subscription", 1
	}
	if key == "" {
		key = "harness:" + route.Harness
	}
	return key, policy.MaxActive
}
func (a *API) captureQueuedActor(ctx context.Context, projectID string) (management.ExecutionActorRef, error) {
	actor, err := management.CaptureExecutionActor(ctx, projectID)
	if err != nil {
		return actor, err
	}
	if actor.Local && a.deps.ManagedExecution {
		return actor, management.ErrExecutionAuthority
	}
	return actor, nil
}
func (a *API) admitQueuedSwarm(ctx context.Context, req product.SwarmAdmissionRequest) (product.SwarmAdmissionResult, error) {
	if a.deps.ExecutionReady != nil && !a.deps.ExecutionReady() {
		return product.SwarmAdmissionResult{State: "waiting", ReasonCode: "runtime_starting", Message: "The workspace is verifying current execution authority and capacity."}, nil
	}
	blocked := func(code, message string) (product.SwarmAdmissionResult, error) {
		return product.SwarmAdmissionResult{State: "blocked", ReasonCode: code, Message: message}, nil
	}
	if req.QueueID == "" || req.Actor.ProjectID != req.ProjectID {
		return blocked("invalid_scope", "This queued task has no matching project owner.")
	}
	run, err := a.deps.Product.GetProjectSwarmRun(req.ProjectID, req.RunID)
	if err != nil {
		return blocked("run_unavailable", "This reviewed run is no longer available.")
	}
	authority, err := a.authorizeExecution(ctx, req.Actor, run.Input.Route.Harness, run.Input.Route.Connection.ID != "")
	if err != nil {
		a.sessionLog().Debug("queued admission authority refused", "projectId", req.ProjectID, "err", err)
		return blocked("access_changed", management.ErrExecutionAuthority.Error())
	}
	if !product.OrchestrationAuthorityMatches(run, authority) {
		return blocked("control_changed", "The team's saved execution policy or grant changed. Review a new contract before starting this task.")
	}
	if err := harness.ValidateTask(harness.ID(run.Input.Route.Harness), harness.PermissionMode(run.Input.Route.PermissionMode)); err != nil {
		return blocked("task_permission_requires_interactive", err.Error())
	}
	if product.ValidateNativeDelegationCurrent(run.Input.Route, a.sessionImage()) != nil || run.Input.Route.NativeDelegation != "" && (a.deps.Backend == nil || a.deps.Backend.Kind() != backend.KindDocker) {
		return blocked("native_helper_profile_changed", product.ErrNativeDelegation.Error())
	}
	if run.Input.Route.ExecutionLocation == "cloud-microvm" {
		if a.deps.CloudTasks == nil {
			return blocked("cloud_microvm_unavailable", "A verified cloud microVM execution service is not configured.")
		}
		// The cloud controller rechecks its own per-task, per-project, global
		// VM and spending limits before an irreversible Create. Local host RAM
		// leases are for Docker and must not be fabricated for remote VMs.
		return product.SwarmAdmissionResult{State: "admitted"}, nil
	}
	provider, limit := providerCapacity(run.Input.Route, req.Policy)
	decision, err := a.execution.reserve(ctx, req.QueueID, req.ProjectID, provider, limit, req.Actor, authority, req.Policy)
	if err != nil {
		return product.SwarmAdmissionResult{}, err
	}
	return product.SwarmAdmissionResult{State: decision.State, ReasonCode: decision.Code, Message: decision.Message, MemoryMiB: decision.Resources.MemoryMiB, CPUMillis: decision.Resources.CPUMillis}, nil
}

// dispatchQueuedSwarm consumes only a private, exact run operation. The
// scheduler cannot submit arbitrary commands, mount paths or provider settings.
func (a *API) dispatchQueuedSwarm(ctx context.Context, req product.SwarmDispatchRequest) (product.SwarmDispatchResult, error) {
	result := product.SwarmDispatchResult{State: "blocked"}
	claimed := false
	refuse := func(code string, err error) (product.SwarmDispatchResult, error) {
		a.sessionLog().Debug("queued task admission stopped", "operation", code, "err", err)
		result.ReasonCode = code
		result.Message = "The queued task cannot start with its current reviewed inputs, model connection or execution authority. Review this task before continuing."
		if code == "dispatch_paused" {
			result.Message = "New queued dispatch is paused."
		}
		if code == "source_busy" {
			result.Message = "The project source is changing. Wait for that operation before continuing."
		}
		if code == "access_changed" {
			result.Message = management.ErrExecutionAuthority.Error()
		}
		if claimed {
			result.State, result.ReasonCode, result.Message = "unknown", "dispatch_unconfirmed", "The reserved task start is unconfirmed. Its exact child and capacity are retained for inspection and recovery."
		}
		return result, nil
	}
	if a.deps.Product == nil || req.QueueID == "" || req.Actor.ProjectID != req.ProjectID {
		return refuse("invalid_scope", errors.New("The queued task has no matching project scope."))
	}
	run, err := a.deps.Product.GetProjectSwarmRun(req.ProjectID, req.RunID)
	if err != nil {
		return refuse("run_unavailable", err)
	}
	var op *product.ProjectRunOperation
	for i := range run.Operations {
		if run.Operations[i].ID == req.OperationID {
			op = &run.Operations[i]
			break
		}
	}
	if op == nil || op.TaskID == "" || op.Implementation == nil || op.WorktreeID == "" || op.ChildID == "" {
		return refuse("operation_unavailable", errors.New("The queued task has no exact reserved coding operation."))
	}
	route := run.Input.Route
	// A stored local child is recovery of an already-owned effect. Resolve it
	// before the freshness gates that apply only to a new backend allocation.
	if route.ExecutionLocation != "cloud-microvm" {
		existing, lookupErr := a.deps.Product.FindProjectRunSession(req.ProjectID, req.RunID, req.OperationID, a.deps.Store.Get)
		if lookupErr != nil {
			return refuse("dispatch_unconfirmed", lookupErr)
		}
		if existing != nil {
			return existingQueuedDispatch(*existing), nil
		}
	}
	authority, err := a.authorizeExecution(ctx, req.Actor, route.Harness, route.Connection.ID != "")
	if err != nil {
		return refuse("access_changed", err)
	}
	if !product.OrchestrationAuthorityMatches(run, authority) {
		return refuse("control_changed", management.ErrExecutionAuthority)
	}
	if route.ExecutionLocation == "cloud-microvm" {
		_ = authority // current authority was checked before cloud selection
		return a.dispatchCloudSwarm(ctx, req, run, *op)
	}
	image := a.sessionImage()
	nativeProfile, err := a.nativeDelegationDispatchProfile(run, op.ID, image)
	if err != nil { return refuse("native_helper_profile_changed", err) }
	policy, err := a.deps.Product.SwarmSchedulerPolicy()
	if err != nil {
		return refuse("policy_unavailable", err)
	}
	provider, limit := providerCapacity(route, policy)
	capacity, err := a.execution.reserve(ctx, req.QueueID, req.ProjectID, provider, limit, req.Actor, authority, policy)
	if err != nil {
		return result, err
	}
	if capacity.State != "admitted" {
		result.State, result.ReasonCode, result.Message = capacity.State, capacity.Code, capacity.Message
		return result, nil
	}
	releaseBranch, err := a.deps.Product.AcquireGitSessionAdmission(ctx, req.ProjectID)
	if err != nil {
		return refuse("source_busy", err)
	}
	defer releaseBranch()
	path, err := a.deps.Worktrees.Path(ctx, op.WorktreeID)
	if err != nil {
		return refuse("worktree_unavailable", err)
	}
	owner, branch := a.sessionLocate(ctx, op.WorktreeID)
	if owner != req.ProjectID {
		return refuse("worktree_unavailable", errors.New("The task workspace ownership changed."))
	}
	ownedRun, ownedOp, err := a.deps.Product.ProjectSwarmWorktreeOwner(ctx, req.ProjectID, op.WorktreeID)
	if err != nil || ownedRun != req.RunID || ownedOp != req.OperationID {
		return refuse("worktree_unavailable", errors.New("The task workspace is not owned by its exact operation."))
	}
	if route.Harness == "codex" {
		a.codexAccountMu.Lock()
		defer a.codexAccountMu.Unlock()
	}
	selected := route.Connection.ID
	connection, agentEnv, model, err := a.resolveSessionAgent(route.Harness, &selected, registry.Session{}, route.Model)
	if err != nil {
		return refuse("provider_unavailable", err)
	}
	if model == "" {
		model = route.Model
	}
	if connection.ID != route.Connection.ID || connection.Revision != route.Connection.Revision || connection.Kind != route.Connection.Kind || connection.Protocol != route.Connection.Protocol || model != route.Model {
		return refuse("provider_changed", errors.New("The saved model connection changed. Review this build before dispatch."))
	}
	var account *codexConnection
	authMode := connection.AuthMode
	if route.Harness == "codex" && (connection.Kind == "legacy" || connection.Kind == "codex-subscription") {
		account, authMode, err = a.selectCodexConnection(ctx, registry.Session{}, false, "")
		if err != nil {
			return refuse("provider_unavailable", err)
		}
	}
	capture, err := a.deps.Product.CaptureProjectRunTaskContext(ctx, req.ProjectID, req.RunID, req.OperationID, op.WorktreeID, path, *product.ImplementationContextSelection(op.Implementation))
	if err != nil {
		return refuse("context_changed", err)
	}
	descriptor, known := harness.Lookup(harness.ID(route.Harness))
	if !known || route.Harness == "shell" {
		return refuse("harness_unavailable", errors.New("Choose an available coding harness for this reviewed task."))
	}
	conversation := ""
	if sessionHasConversation(descriptor) || descriptor.NativeState() {
		conversation = ids.UUID()
	}
	prompt := product.ImplementationOpeningPrompt(capture.Binding)
	command, argv, err := harness.BuildTask(harness.Request{DockerBoundary: a.deps.Backend.Kind() == backend.KindDocker, ProjectBrief: true, ReviewedContext: true, NativeDelegation: nativeProfile, Harness: harness.ID(route.Harness), Model: model, Effort: harness.Effort(route.Effort), Permission: harness.PermissionMode(route.PermissionMode), OpeningPrompt: prompt, Conversation: conversation})
	if err != nil {
		return refuse("harness_unavailable", err)
	}
	lineEndings, err := a.sessionGitLineEndings(ctx)
	if err != nil {
		return refuse("git_configuration", err)
	}
	gitFiles, err := a.sessionGitGlobalFiles(ctx, op.WorktreeID)
	if err != nil {
		return refuse("git_configuration", err)
	}
	admin, err := sessionWorktreeAdmin(path)
	if err != nil {
		return refuse("git_configuration", err)
	}
	sandboxEnv := sessionEnv(a.sessionGitIdentity(ctx, op.WorktreeID), lineEndings, gitFiles, a.deps.CABundle)
	for key, value := range agentEnv {
		sandboxEnv[key] = value
	}
	if route.Harness == "opencode" && sandboxEnv["OPENCODE_CONFIG_CONTENT"] == "" {
		return refuse("provider_unavailable", errors.New("An explicit model connection is required."))
	}
	argv = codexSubscriptionArgs(argv, sandboxEnv, authMode)
	if a.deps.Backend.Kind() == backend.KindDocker {
		nativeDockerEndpoint(sandboxEnv)
	}
	argv = nativeArgs(route.Harness, argv, sandboxEnv, false)
	nativeSandboxEnvironment(sandboxEnv)
	candidate := registry.Session{ProjectID: req.ProjectID, WorktreeID: op.WorktreeID, BranchID: branch, Harness: route.Harness, Model: model, Effort: route.Effort, PermissionMode: route.PermissionMode, OpeningPrompt: prompt, AuthMode: authMode, CodexConnectionID: codexConnectionID(account), AgentConnectionID: connection.ID, AgentConnectionRevision: connection.Revision, AgentConnectionKind: connection.Kind, AgentConnectionProtocol: connection.Protocol, ImplementationContext: capture.Binding, ProjectRunID: req.RunID, ProjectRunOperationID: req.OperationID}
	enabled, err := a.deps.Product.SwarmSchedulerDispatchEnabled()
	if err != nil || !enabled {
		return refuse("dispatch_paused", errors.New("New queued dispatch is paused."))
	}
	admission, err := a.deps.Product.BeginProjectRunSessionWithAssignment(req.ProjectID, req.RunID, req.OperationID, candidate, capture.ReviewAssignmentDigest, a.deps.Store.Get)
	if err != nil {
		return refuse("dispatch_changed", err)
	}
	if admission.Release == nil || admission.SessionID != op.ChildID {
		return result, errors.New("incomplete queued session admission")
	}
	defer admission.Release()
	if admission.Existing != nil {
		return existingQueuedDispatch(*admission.Existing), nil
	}
	claimed = true
	result.SessionID = admission.SessionID
	conversationStore := ""
	if conversation != "" && !descriptor.NativeState() {
		conversationStore, err = a.sessionConversationStore(op.WorktreeID)
		if err != nil {
			return refuse("conversation_storage", err)
		}
	}
	mounts := sessionMounts(path, admin, conversationStore, backend.Mount{}, a.deps.CABundle, gitFiles)
	outputMount, outputErr := a.prepareTaskHandoffOutput(admission.SessionID)
	if outputErr != nil {
		return refuse("task_output_unavailable", outputErr)
	}
	mounts = append(mounts, outputMount)
	if descriptor.NativeState() {
		store, err := a.prepareNativeState(op.WorktreeID, route.Harness, conversation, sessionResumeSource{}, false)
		if err != nil {
			return refuse("conversation_storage", err)
		}
		if err := seedCodexConnection(store, account); err != nil {
			return refuse("provider_unavailable", err)
		}
		mounts = nativeMount(mounts, store)
		nativeEnvironment(route.Harness, sandboxEnv, harness.PermissionMode(route.PermissionMode))
	}
	if err := a.deps.Product.ValidateImplementationContext(req.ProjectID, op.WorktreeID, path, capture.Binding); err != nil {
		return refuse("context_changed", err)
	}
	reviewed, mount, err := a.prepareReviewedContext(admission.SessionID, capture)
	if err != nil {
		return refuse("captured_context", err)
	}
	mounts = append(mounts, mount)
	candidate.ID, candidate.Title, candidate.MachineID = admission.SessionID, descriptor.Name+" · "+filepath.Base(filepath.Clean(path)), LocalMachineID
	candidate.Command, candidate.Args, candidate.ConversationID = command, argv, conversation
	candidate.Status, candidate.Backend, candidate.SandboxID = registry.StatusStarting, string(a.deps.Backend.Kind()), candidate.ID
	candidate.AttachToken, candidate.ReviewedContext = ids.Token(), reviewed
	if err := a.execution.bind(ctx, req.QueueID, candidate.ID); err != nil {
		return refuse("capacity_changed", err)
	}
	finalAuth := func(ctx context.Context) error {
		currentProfile, err := a.nativeDelegationDispatchProfile(run, op.ID, a.sessionImage())
		if err != nil || currentProfile != nativeProfile || a.sessionImage() != image { return admissionFailure("native_helper_profile_changed", product.ErrNativeDelegation) }
		enabled, err := a.deps.Product.SwarmSchedulerDispatchEnabled()
		if err != nil {
			return admissionFailure("dispatch_policy_unavailable", err)
		}
		if !enabled {
			return admissionFailure("dispatch_paused", errors.New("New queued dispatch is paused."))
		}
		current, err := a.authorizeExecution(ctx, req.Actor, route.Harness, connection.ID != "")
		if err != nil {
			return admissionFailure("execution_authority_changed", err)
		}
		if !product.OrchestrationAuthorityMatches(run, current) {
			return admissionFailure("execution_contract_authority_changed", errors.New("The current team grant or execution policy differs from this reviewed contract."))
		}
		if err := a.deps.Product.ValidateSwarmOperationExecutionContract(req.ProjectID, run, op.ID); err != nil {
			return admissionFailure("execution_contract_changed", err)
		}
		resources := a.sessionResources(ctx)
		if req.Actor.UserID != "" {
			resources = clampSessionResources(resources, current.Policy)
		}
		if resources != capacity.Resources {
			return admissionFailure("resource_policy_changed", errors.New("The worker resource policy changed before allocation."))
		}
		policy, err := a.deps.Product.SwarmSchedulerPolicy()
		if err != nil {
			return admissionFailure("resource_policy_unavailable", err)
		}
		provider, limit := providerCapacity(route, policy)
		decision, err := a.execution.reserve(ctx, req.QueueID, req.ProjectID, provider, limit, req.Actor, current, policy)
		if err != nil {
			return admissionContextFailure(ctx, err)
		}
		if decision.State != "admitted" {
			return admissionDecisionFailure(decision)
		}
		return nil
	}
	stored, err := a.allocatePreparedSession(ctx, preparedSessionAllocation{Record: candidate, Spec: backend.Spec{ID: candidate.ID, SessionID: candidate.ID, Image: image, Command: command, Args: argv, Env: sandboxEnv, AttachToken: candidate.AttachToken, Resources: capacity.Resources, WorkDir: sessionWorkDir, Mounts: mounts}, Account: account, Revalidate: admission.Revalidate, BeforeAllocate: finalAuth, BeforeBackendStart: func(ctx context.Context) error {
		return a.execution.markBackendStarting(ctx, req.QueueID, candidate.ID, candidate.ID)
	}, AfterBackendCreate: func(ctx context.Context, sandboxID string) error {
		return a.execution.markBackendSettled(ctx, req.QueueID, candidate.ID, sandboxID)
	}, HasReviewedContext: true, ExecutionLeaseID: req.QueueID})
	if err != nil {
		var failure *sessionAllocationError
		if errors.As(err, &failure) && failure.StartFailure.Valid() {
			result.State, result.ReasonCode, result.Message = "failed", failure.StartFailure.ReasonCode, failure.StartFailure.Message
		} else {
			result.State, result.ReasonCode, result.Message = "unknown", "dispatch_unconfirmed", "The task start could not be confirmed. Its capacity and exact child identity are retained for recovery."
		}
		return result, nil
	}
	result.State, result.SessionID = "running", stored.ID
	return result, nil
}

func (a *API) queuedResourceView(ctx context.Context) any {
	if ctx.Err() != nil {
		return nil
	}
	var buildPolicy *management.Policy
	if _, policy, ok := management.ActorFromContext(ctx); ok {
		buildPolicy = &policy
	}
	publicLimit := func(r backend.Resources) map[string]int {
		return map[string]int{"memoryMiB": r.MemoryMiB, "cpuMillis": r.CPUMillis}
	}
	limits := map[string]any{"worker": publicLimit(a.sessionResources(ctx)), "runtimeBuild": publicLimit(a.runtimeBuildResources(buildPolicy))}
	if a.deps.ResourceSnapshot == nil {
		reservations := a.execution.reservationObservation(ctx)
		if ctx.Err() != nil {
			return nil
		}
		return map[string]any{"status": "unavailable", "reservations": reservations, "limits": limits}
	}
	snapshot := a.deps.ResourceSnapshot()
	if ctx.Err() != nil {
		return nil
	}
	reservations := a.execution.reservationObservation(ctx)
	if ctx.Err() != nil {
		return nil
	}
	var memoryReserve any
	if policy, err := a.deps.Product.SwarmSchedulerPolicy(); err == nil && snapshot.Host.Memory.EffectiveTotalBytes != nil && snapshot.Execution.TotalMemoryBytes != nil {
		memoryReserve = map[string]any{"hostBytes": policy.MemoryHeadroomBytes(*snapshot.Host.Memory.EffectiveTotalBytes), "executionBytes": policy.MemoryHeadroomBytes(*snapshot.Execution.TotalMemoryBytes)}
	}
	return struct {
		Host          any                             `json:"host"`
		Execution     any                             `json:"execution"`
		Reservations  executionReservationObservation `json:"reservations"`
		Limits        any                             `json:"limits"`
		MemoryReserve any                             `json:"memoryReserve"`
	}{snapshot.Host, map[string]any{"observedAt": snapshot.Execution.ObservedAt, "status": snapshot.Execution.Status, "scope": snapshot.Execution.Scope, "totalMemoryBytes": snapshot.Execution.TotalMemoryBytes, "cpuMillis": snapshot.Execution.CPUMillis, "hostStorage": snapshot.Execution.HostStorage, "unavailable": snapshot.Execution.Unavailable}, reservations, limits, memoryReserve}
}

func existingQueuedDispatch(row registry.Session) product.SwarmDispatchResult {
	result := product.SwarmDispatchResult{State: "running", SessionID: row.ID}
	if failure := publicSessionStartFailure(row); failure.Valid() {
		result.State, result.ReasonCode, result.Message = "failed", failure.ReasonCode, failure.Message
	} else if row.Status == registry.StatusExited {
		result.State, result.ReasonCode, result.Message = "unknown", "existing_child_terminal", "The existing child has recorded an exit. Observe its exact operation and retained output; no replacement was launched."
	}
	return result
}

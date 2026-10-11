package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// Every persisted lease accounts for a pending allocation even before Docker
// sees it. Unknown outcomes retain their reservation across process restart.
type executionLease struct {
	ID              string                       `json:"id"`
	Scope           string                       `json:"scope"`
	Actor           management.ExecutionActorRef `json:"actor"`
	ProjectID       string                       `json:"projectId"`
	Provider        string                       `json:"provider"`
	ProviderLimit   int                          `json:"providerLimit"`
	SessionID       string                       `json:"sessionId,omitempty"`
	SandboxID       string                       `json:"sandboxId,omitempty"`
	State           string                       `json:"state"`
	LaunchGuard     int                          `json:"launchGuard,omitempty"`
	FenceProtocol   int                          `json:"fenceProtocol,omitempty"`
	FenceEngineID   string                       `json:"fenceEngineId,omitempty"`
	FenceInstanceID string                       `json:"fenceInstanceId,omitempty"`
	FenceIntentID   string                       `json:"fenceIntentId,omitempty"`
	Resources       backend.Resources            `json:"resources"`
	CreatedAt       time.Time                    `json:"createdAt"`
	UpdatedAt       time.Time                    `json:"updatedAt"`
}
type executionLeaseLedger struct {
	Version       int                          `json:"version"`
	Leases        []executionLease             `json:"leases"`
	Retirements   []executionRetirement        `json:"retirements,omitempty"`
	Fences        []executionFenceRecord       `json:"fences,omitempty"`
	Checks        []checkExecutionLease        `json:"checks,omitempty"`
	RuntimeImages []runtimeImageExecutionLease `json:"runtimeImages,omitempty"`
	Previews      []previewExecutionLease      `json:"previews,omitempty"`
	CheckFences   []checkExecutionFenceRecord  `json:"checkFences,omitempty"`
}
type executionAdmission struct {
	api            *API
	mu             sync.Mutex
	path, scope    string
	snapshot       atomic.Pointer[executionReservationView]
	fenceGuards    sync.Map
	checkObserver  checkExecutionObservations
	manualObserver manualLeaseObservations
}
type executionReservationView struct {
	ObservedAt time.Time `json:"observedAt"`
	Count      int       `json:"count"`
	MemoryMiB  int       `json:"memoryMiB"`
	CPUMillis  int       `json:"cpuMillis"`
}
type executionDecision struct {
	State, Code, Message string
	Resources            backend.Resources
	Evidence             registry.StartAdmissionEvidence
	RuntimeAdmission     *runtimeinstall.AdmissionObservation
}

func newExecutionAdmission(a *API) *executionAdmission {
	scope := sha256.Sum256([]byte(filepath.Clean(a.deps.ConversationsDir)))
	root := a.deps.ExecutionStateDir
	if root == "" {
		root = filepath.Join(a.deps.ConversationsDir, ".execution")
	}
	return &executionAdmission{api: a, path: filepath.Join(root, "leases.json"), scope: hex.EncodeToString(scope[:])}
}

// Reuse admission's path validation before either a capacity write or a child
// lifecycle lock, including restored sessions with no ledger directory yet.
func (g *executionAdmission) prepareExecutionDirectory() (string, error) {
	root := filepath.Dir(g.path)
	if !filepath.IsAbs(root) {
		return "", errors.New("execution state path must be absolute")
	}
	// Queue admission precedes the first conversation. Check the nearest
	// existing ancestor before creating missing private state directories.
	ancestor := root
	for {
		_, err := os.Lstat(ancestor)
		if err == nil {
			break
		}
		if !errors.Is(err, os.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(ancestor)
		if parent == ancestor {
			return "", errors.New("execution state has no existing ancestor")
		}
		ancestor = parent
	}
	if err := codexRegularPath(filepath.VolumeName(root)+string(filepath.Separator), ancestor); err != nil {
		return "", err
	}
	if _, err := hostpath.ResolveOutside(g.api.projectsDir(), ancestor); err != nil {
		return "", err
	}
	if err := os.MkdirAll(root, 0700); err != nil {
		return "", err
	}
	if _, err := hostpath.ResolveOutside(g.api.projectsDir(), root); err != nil {
		return "", err
	}
	if err := codexPrivateDir(root); err != nil {
		return "", err
	}
	return root, nil
}

func (g *executionAdmission) mutate(ctx context.Context, fn func(*executionLeaseLedger) error) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.mutateLocked(ctx, fn)
}

// Caller owns the local mutex; the shared durable lock still fences every write.
func (g *executionAdmission) mutateLocked(ctx context.Context, fn func(*executionLeaseLedger) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	root, err := g.prepareExecutionDirectory()
	if err != nil {
		return err
	}
	lockPath := filepath.Join(root, "leases.lock")
	if info, err := os.Lstat(lockPath); err == nil {
		if !info.Mode().IsRegular() {
			return errors.New("execution lock must be an ordinary private file")
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	file, err := os.OpenFile(lockPath, os.O_RDWR|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	info, err := os.Lstat(lockPath)
	if err != nil || !info.Mode().IsRegular() {
		return errors.New("execution lock ownership changed")
	}
	opened, err := file.Stat()
	if err != nil || !os.SameFile(info, opened) {
		return errors.New("execution lock identity changed")
	}
	if _, err := hostpath.ResolveOutside(g.api.projectsDir(), lockPath); err != nil {
		return err
	}
	if err := hostpath.OneName(lockPath); err != nil {
		return err
	}
	if err := codexPrivateFile(lockPath); err != nil {
		return err
	}
	release, err := lockExecutionLedger(ctx, file)
	if err != nil {
		return err
	}
	defer release()
	ledger := executionLeaseLedger{Version: 1, Leases: []executionLease{}}
	data, err := codexRead(root, g.path, 2<<20)
	if err == nil {
		ledger, err = decodeExecutionLedger(data)
		if err != nil {
			return err
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	if err := fn(&ledger); err != nil {
		return err
	}
	data, err = json.Marshal(ledger)
	if err != nil {
		return err
	}
	if _, err := decodeExecutionLedger(data); err != nil {
		return err
	}
	if len(data) > 2<<20 {
		return errors.New("execution capacity history exceeds its bounded private storage")
	}
	if err := writeExecutionLedger(g.path, data); err != nil {
		return err
	}
	view := &executionReservationView{ObservedAt: time.Now().UTC(), Count: len(ledger.Leases)}
	for _, lease := range ledger.Leases {
		view.MemoryMiB += lease.Resources.MemoryMiB
		view.CPUMillis += lease.Resources.CPUMillis
	}
	for _, check := range ledger.Checks {
		if check.State == "finished" {
			continue
		}
		view.Count++
		view.MemoryMiB += check.Resources.MemoryMiB
		view.CPUMillis += check.Resources.CPUMillis
	}
	addPreviewReservationView(view, ledger.Previews)
	addRuntimeImageReservationView(view, ledger.RuntimeImages)
	g.snapshot.Store(view)
	return nil
}

func decodeExecutionLedger(data []byte) (executionLeaseLedger, error) {
	var ledger executionLeaseLedger
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&ledger) != nil || decoder.Decode(new(any)) != io.EOF || ledger.Version != 1 || len(ledger.Leases)+len(ledger.Checks)+len(ledger.Previews)+len(ledger.RuntimeImages) > 4096 || len(ledger.Retirements) > 4096 {
		return ledger, errors.New("execution capacity state cannot be verified")
	}
	seen := map[string]bool{}
	for _, lease := range ledger.Leases {
		key := lease.Scope + ":" + lease.ID
		if !validExecutionLease(lease) || seen[key] {
			return ledger, errors.New("execution capacity ownership cannot be verified")
		}
		seen[key] = true
	}
	for _, check := range ledger.Checks {
		key := check.Scope + ":" + check.ID
		if !validCheckExecutionLease(check) || seen[key] {
			return ledger, errors.New("check capacity ownership cannot be verified")
		}
		seen[key] = true
	}
	for _, preview := range ledger.Previews {
		key := preview.Scope + ":" + preview.ID
		if !validPreviewExecutionLease(preview) || seen[key] {
			return ledger, errors.New("preview capacity ownership cannot be verified")
		}
		seen[key] = true
	}
	for _, install := range ledger.RuntimeImages {
		key := install.Scope + ":" + install.ID
		if !validRuntimeImageExecutionLease(install) || seen[key] {
			return ledger, errors.New("runtime image capacity ownership cannot be verified")
		}
		seen[key] = true
	}
	for _, retired := range ledger.Retirements {
		key := retired.Scope + ":" + retired.LeaseID
		if !validExecutionRetirement(retired) || seen[key] {
			return ledger, errors.New("execution retirement ownership cannot be verified")
		}
		seen[key] = true
	}
	if err := validateExecutionFenceRecords(ledger); err != nil {
		return ledger, err
	}
	if err := validateCheckFenceRecords(ledger); err != nil {
		return ledger, err
	}
	return ledger, nil
}

func (g *executionAdmission) reconcileOwned(ctx context.Context, ledger *executionLeaseLedger) error {
	// One shared bound prevents a backlog of unavailable task backends from
	// multiplying network deadlines while admission holds the host ledger.
	observeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	kept := ledger.Leases[:0]
	for _, lease := range ledger.Leases {
		if lease.Scope == g.scope && lease.SessionID != "" {
			row, err := g.api.deps.Store.Get(lease.SessionID)
			if err != nil && !errors.Is(err, registry.ErrNotFound) {
				return err
			}
			if err == nil && row.Status == registry.StatusExited && row.ID == lease.SessionID && row.ProjectID == lease.ProjectID && row.SandboxID == lease.SandboxID && executionLaunchSettled(lease) {
				requiresAbsence, ownershipErr := g.api.executionRequiresBackendAbsence(row)
				if ownershipErr != nil || requiresAbsence {
					// Resource inventory covers active workloads, not stopped Docker
					// objects. Manual tasks have no scheduler Release callback, so
					// their next admission must also confirm exact removal here.
					if ownershipErr == nil {
						absent, err := g.api.exactBackendTerminal(observeCtx, row.SandboxID, true)
						if err == nil && absent {
							continue
						}
					}
					// Unknown, timed-out and stopped-but-present allocations keep
					// both capacity and the durable proof for cleanup retries.
					kept = append(kept, lease)
					continue
				}
				// Only a fresh backend observation after our last transition can prove
				// an exited record has released its actual allocation.
				if g.api.deps.ResourceSnapshot != nil {
					snap := g.api.deps.ResourceSnapshot()
					present := false
					for _, container := range snap.Execution.Containers {
						if container.SandboxID == lease.sandboxID() || container.SandboxID == row.SandboxID {
							present = true
						}
					}
					if snap.Execution.Fresh(time.Now(), 5*time.Second) && snap.Execution.InventoryComplete && snap.Execution.ObservedAt.After(lease.UpdatedAt) && !present {
						continue
					}
				}
			}
		}
		kept = append(kept, lease)
	}
	ledger.Leases = kept
	return nil
}

func (lease executionLease) sandboxID() string {
	if lease.SandboxID != "" {
		return lease.SandboxID
	}
	return lease.SessionID
}
func (g *executionAdmission) reserve(ctx context.Context, id, projectID, provider string, providerLimit int, actor management.ExecutionActorRef, authority management.ExecutionAuthority, policy product.SwarmSchedulerPolicy) (executionDecision, error) {
	return g.reserveWorker(ctx, id, projectID, provider, providerLimit, actor, authority, policy, nil)
}

// Replacement reserves one slot for the same registry identity. The old sandbox
// remains alive until the caller has checked this decision; only its exact
// allocation is excluded from the proposed replacement's capacity calculation.
func (g *executionAdmission) reserveReplacement(ctx context.Context, id, projectID, provider string, providerLimit int, actor management.ExecutionActorRef, authority management.ExecutionAuthority, policy product.SwarmSchedulerPolicy, prior registry.Session) (executionDecision, error) {
	if prior.ID == "" || prior.ProjectID != projectID || prior.ProjectRunID != "" || prior.ProjectRunOperationID != "" {
		return executionDecision{}, errors.New("invalid replacement execution scope")
	}
	return g.reserveWorker(ctx, id, projectID, provider, providerLimit, actor, authority, policy, &prior)
}
func (g *executionAdmission) reserveWorker(ctx context.Context, id, projectID, provider string, providerLimit int, actor management.ExecutionActorRef, authority management.ExecutionAuthority, policy product.SwarmSchedulerPolicy, replacement *registry.Session) (executionDecision, error) {
	decision := executionDecision{State: "waiting", Code: "resource_observation_unavailable", Message: "Current machine capacity is not available. The task remains queued."}
	resources := g.api.sessionResources(ctx)
	if actor.UserID != "" {
		resources = clampSessionResources(resources, authority.Policy)
	}
	decision.Resources = resources
	if resources.MemoryMiB < 128 || resources.MemoryMiB > 32768 || resources.CPUMillis < 100 || resources.CPUMillis > 16000 || providerLimit < 1 || providerLimit > 16 || actor.ProjectID != projectID || id == "" || len(id) > 200 || len(provider) > 200 || provider == "" {
		return decision, errors.New("worker resource or ownership policy is outside supported execution bounds")
	}
	if g.api.deps.ResourceSnapshot == nil {
		return decision, nil
	}
	err := g.mutate(ctx, func(ledger *executionLeaseLedger) error {
		if err := g.reconcileOwned(ctx, ledger); err != nil {
			return err
		}
		if replacement != nil {
			for _, lease := range ledger.Leases {
				if lease.Scope == g.scope && lease.ID != id && lease.SessionID == replacement.ID && (lease.State != "backend-settled" || lease.LaunchGuard != 1) {
					decision.Code, decision.Message = "execution_launch_unconfirmed", "The previous worker's create/start outcome is unconfirmed. Its reservation is retained; restarting could create a second worker."
					return nil
				}
			}
		}
		existing := false
		for _, retired := range ledger.Retirements {
			if retired.Scope == g.scope && retired.LeaseID == id {
				return errors.New("this exact execution was permanently retired; reserve a new operation")
			}
		}
		for _, lease := range ledger.Leases {
			if lease.Scope == g.scope && lease.ID == id {
				if lease.Actor != actor || lease.ProjectID != projectID || lease.Provider != provider || lease.Resources != resources {
					return errors.New("reserved execution inputs changed")
				}
				existing = true
			}
		}
		snap := g.api.deps.ResourceSnapshot()
		now := time.Now()
		if !freshExecutionCapacity(snap, now) {
			return nil
		}
		waiting := func(code, message string) { decision.Code, decision.Message = code, message }
		active, projectActive, actorActive, providerActive := 0, 0, 0, 0
		var pendingMemory, allMemory uint64
		pendingAllocations := 0
		var cpu int64
		observed := map[string]bool{}
		counted := map[string]bool{}
		for _, container := range snap.Execution.Containers {
			if replacement != nil && container.Owned && container.SandboxID == replacement.SandboxID {
				continue
			}
			memory, cpus, ok := executionWorkloadDemand(container, now)
			if !ok {
				waiting("execution_limits_unknown", "A running workload has neither verified worker limits nor fresh resource usage. Waiting for a bounded observation before starting another task.")
				return nil
			}
			allMemory += memory
			cpu += cpus
			if container.SandboxID != "" {
				observed[container.SandboxID] = true
				if container.Owned && container.Kind == "agent" {
					active++
					counted[container.SandboxID] = true
				}
			}
		}
		for _, lease := range ledger.Leases {
			if lease.Scope == g.scope && (lease.ID == id || replacement != nil && lease.SessionID == replacement.ID) {
				continue
			}
			if lease.ProjectID == projectID && lease.Scope == g.scope {
				projectActive++
			}
			if lease.Scope == g.scope {
				actorActive++
			}
			if lease.Provider == provider && lease.Scope == g.scope {
				providerActive++
			}
			if lease.SessionID == "" || !observed[lease.sandboxID()] {
				active++
				bytes := uint64(lease.Resources.MemoryMiB) << 20
				pendingMemory += bytes
				pendingAllocations++
				allMemory += bytes
				cpu += int64(lease.Resources.CPUMillis)
			} else if !counted[lease.sandboxID()] {
				active++
				counted[lease.sandboxID()] = true
			}
		}
		// Include pre-ledger legacy sessions in account/project limits. Their real
		// container usage was already included above; do not double-charge memory.
		rows, err := g.api.deps.Store.List()
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Status == registry.StatusExited || replacement != nil && row.ID == replacement.ID {
				continue
			}
			covered := false
			for _, lease := range ledger.Leases {
				if lease.Scope == g.scope && lease.SessionID == row.ID {
					covered = true
					break
				}
			}
			if covered {
				continue
			}
			actorActive++
			if row.ProjectID == projectID {
				projectActive++
			}
			if provider == executionSessionProvider(row) {
				providerActive++
			}
			if !observed[row.SandboxID] {
				// A legacy starting row may predate its next Docker sample.
				// Its configured ceiling remains a reservation, not measured zero.
				active++
				bytes := uint64(resources.MemoryMiB) << 20
				pendingMemory += bytes
				allMemory += bytes
				cpu += int64(resources.CPUMillis)
				pendingAllocations++
			} else if !counted[row.SandboxID] {
				active++
				counted[row.SandboxID] = true
			}
		}
		if actor.UserID != "" && authority.Policy.MaxSessionsPerUser > 0 && actorActive >= authority.Policy.MaxSessionsPerUser {
			waiting("account_session_limit", "The account session limit is occupied. Stop a session or wait for its confirmed exit.")
			return nil
		}
		if active >= policy.MaxActive {
			waiting("global_slot_limit", "The configured global agent limit is occupied.")
			return nil
		}
		if projectActive >= policy.MaxPerProject {
			waiting("project_slot_limit", "This project has reached its active agent limit.")
			return nil
		}
		if provider != "" && providerActive >= providerLimit {
			waiting("provider_limit", "This model connection has reached its concurrency limit.")
			return nil
		}
		demand := executionCapacityDemand{PendingMemory: pendingMemory, AllMemory: allMemory, CPU: cpu, PendingAllocations: pendingAllocations}
		if err := addCheckCapacity(snap, ledger.Checks, "", "", &demand); err != nil {
			return err
		}
		if err := addPreviewCapacity(snap, ledger.Previews, "", "", &demand); err != nil {
			return err
		}
		if err := addRuntimeImageCapacity(ledger.RuntimeImages, "", "", &demand); err != nil {
			return err
		}
		if capacity := evaluateExecutionCapacityWithEvidence(snap, policy, resources, demand, now); capacity.Code != "" {
			waiting(capacity.Code, capacity.Message)
			decision.Evidence = capacity.Evidence
			return nil
		}
		if len(ledger.Leases)+len(ledger.Checks)+len(ledger.Previews)+len(ledger.RuntimeImages) >= 4096 {
			return errors.New("execution capacity history needs reconciliation")
		}
		if replacement != nil {
			// Swap the old manual reservation only after admission succeeds.
			// The replacement still names the old live sandbox until destroy
			// succeeds and bindSandbox names its newly allocated successor.
			kept := ledger.Leases[:0]
			for _, lease := range ledger.Leases {
				if lease.Scope == g.scope && lease.SessionID == replacement.ID && lease.ID != id {
					continue
				}
				kept = append(kept, lease)
			}
			ledger.Leases = kept
		}
		if !existing {
			lease := executionLease{ID: id, Scope: g.scope, Actor: actor, ProjectID: projectID, Provider: provider, ProviderLimit: providerLimit, State: "reserved", LaunchGuard: 1, Resources: resources, CreatedAt: now, UpdatedAt: now}
			if replacement != nil {
				lease.SessionID, lease.SandboxID, lease.State = replacement.ID, replacement.SandboxID, "replacing"
			}
			ledger.Leases = append(ledger.Leases, lease)
		}
		decision.State, decision.Code, decision.Message = "admitted", "", "Worker capacity reserved."
		return nil
	})
	return decision, err
}

func (g *executionAdmission) bind(ctx context.Context, id, sessionID string) error {
	return g.bindSandbox(ctx, id, sessionID, sessionID)
}
func (g *executionAdmission) bindSandbox(ctx context.Context, id, sessionID, sandboxID string) error {
	if sessionID == "" || sandboxID == "" {
		return errors.New("execution child identity missing")
	}
	return g.mutate(ctx, func(ledger *executionLeaseLedger) error {
		for i := range ledger.Leases {
			lease := &ledger.Leases[i]
			if lease.Scope == g.scope && lease.ID == id {
				if lease.State != "reserved" && lease.State != "allocating" && lease.State != "replacing" {
					return errors.New("backend launch phase cannot be reset to an unstarted reservation")
				}
				if lease.SessionID != "" && lease.SessionID != sessionID {
					return errors.New("execution lease already names another child")
				}
				lease.SessionID, lease.SandboxID, lease.State, lease.UpdatedAt = sessionID, sandboxID, "allocating", time.Now()
				return nil
			}
		}
		return errors.New("execution capacity reservation is missing")
	})
}

var ErrExecutionLeaseHeld = errors.New("The child execution remains active or unconfirmed; its capacity reservation is retained.")

func (g *executionAdmission) release(ctx context.Context, id string) error {
	held := false
	localCleanup := true
	err := g.mutate(ctx, func(ledger *executionLeaseLedger) error {
		kept := ledger.Leases[:0]
		for _, lease := range ledger.Leases {
			if lease.Scope == g.scope && lease.ID == id {
				if lease.SessionID != "" {
					if !executionLaunchSettled(lease) {
						held = true
						localCleanup = false
						kept = append(kept, lease)
						continue
					}
					row, err := g.api.deps.Store.Get(lease.SessionID)
					if err != nil || row.Status != registry.StatusExited || row.ID != lease.SessionID || row.ProjectID != lease.ProjectID || row.SandboxID == "" || row.SandboxID != lease.SandboxID {
						held = true
						localCleanup = false
						kept = append(kept, lease)
						continue
					}
					requiresAbsence, err := g.api.executionRequiresBackendAbsence(row)
					if err != nil {
						held = true
						localCleanup = false
						kept = append(kept, lease)
						continue
					}
					stopped, observed, err := g.api.exactBackendTerminalObservation(ctx, row.SandboxID, requiresAbsence)
					if err != nil || !stopped {
						held = true
						if err != nil || !requiresAbsence || !g.stoppedCleanupIsLocal(ctx, lease, row, observed) {
							localCleanup = false
						}
						kept = append(kept, lease)
						continue
					}
				}
				continue
			}
			kept = append(kept, lease)
		}
		ledger.Leases = kept
		return nil
	})
	if err == nil && held {
		if localCleanup && ctx.Err() == nil {
			return product.SwarmCleanupPending(ErrExecutionLeaseHeld)
		}
		return ErrExecutionLeaseHeld
	}
	return err
}

// Agent allocations must have hard limits. Existing ancillary workloads may be
// unbounded; use their fresh measured usage instead, with the host/daemon policy
// headroom still reserved. A missing or stale sample never becomes zero demand.
func executionWorkloadDemand(container resourcebudget.ContainerUsage, now time.Time) (uint64, int64, bool) {
	if container.MemoryLimitBytes != nil && container.CPULimitMillis != nil {
		return *container.MemoryLimitBytes, *container.CPULimitMillis, true
	}
	if container.Owned && container.Kind == "agent" {
		return 0, 0, false
	}
	if container.ObservedAt.IsZero() || container.ObservedAt.After(now) || now.Sub(container.ObservedAt) > 5*time.Second {
		return 0, 0, false
	}
	memory, cpu := container.MemoryLimitBytes, container.CPULimitMillis
	if memory == nil {
		memory = container.MemoryUsageBytes
	}
	if cpu == nil {
		cpu = container.CPUUsageMillis
	}
	if memory == nil || cpu == nil || *cpu < 0 {
		return 0, 0, false
	}
	return *memory, *cpu, true
}
func executionSessionProvider(row registry.Session) string {
	if row.AgentConnectionKind == "codex-subscription" || row.AuthMode == "chatgpt" || row.CodexConnectionID != "" {
		return "codex-subscription"
	}
	if row.AgentConnectionID != "" {
		return row.AgentConnectionID
	}
	return "harness:" + row.Harness
}

func validExecutionLease(lease executionLease) bool {
	scope, err := hex.DecodeString(lease.Scope)
	if err != nil || len(scope) != 32 || lease.LaunchGuard < 0 || lease.LaunchGuard > 1 || lease.ID == "" || len(lease.ID) > 200 || lease.ProjectID == "" || len(lease.ProjectID) > 200 || lease.Actor.ProjectID != lease.ProjectID || len(lease.Actor.UserID) > 200 || len(lease.Actor.TeamID) > 200 || lease.Provider == "" || len(lease.Provider) > 200 || lease.ProviderLimit < 1 || lease.ProviderLimit > 16 || lease.Resources.MemoryMiB < 128 || lease.Resources.MemoryMiB > 32768 || lease.Resources.CPUMillis < 100 || lease.Resources.CPUMillis > 16000 || lease.CreatedAt.IsZero() || lease.UpdatedAt.Before(lease.CreatedAt) || len(lease.SessionID) > 200 || len(lease.SandboxID) > 200 {
		return false
	}
	if lease.Actor.Local && (lease.Actor.UserID != "" || lease.Actor.TeamID != "") || !lease.Actor.Local && lease.Actor.UserID == "" {
		return false
	}
	if !validExecutionFenceLease(lease) {
		return false
	}
	switch lease.State {
	case "reserved":
		return lease.SessionID == "" && lease.SandboxID == ""
	case "allocating", "replacing", "backend-dispatching", "backend-settled", "fencing":
		return lease.SessionID != "" && lease.SandboxID != ""
	default:
		return false
	}
}

// Forget may remove the last registry witness. Release its proven-terminal
// leases first so neither hidden held capacity nor an unconfirmed child is lost.
func (g *executionAdmission) releaseSession(ctx context.Context, sessionID string) error {
	ids := []string{}
	if err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		for _, lease := range l.Leases {
			if lease.Scope == g.scope && lease.SessionID == sessionID {
				ids = append(ids, lease.ID)
			}
		}
		return nil
	}); err != nil {
		return err
	}
	for _, id := range ids {
		if err := g.release(ctx, id); err != nil {
			return err
		}
	}
	return nil
}

// This durable edge precedes every guarded Backend.Create. Recovery may prove a
// missing row never reached this edge; it may never guess after crossing it.
func (g *executionAdmission) markBackendStarting(ctx context.Context, id, sessionID, sandboxID string) error {
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		for _, v := range l.Retirements {
			if v.Scope == g.scope && (v.LeaseID == id || v.ChildID == sessionID) {
				return errors.New("this exact child was permanently retired before launch")
			}
		}
		for i := range l.Leases {
			v := &l.Leases[i]
			if v.Scope == g.scope && v.ID == id {
				if v.LaunchGuard != 1 || v.State != "allocating" || v.SessionID != sessionID || v.sandboxID() != sandboxID {
					return errors.New("execution launch guard no longer matches the reserved child")
				}
				row, err := g.api.deps.Store.Get(sessionID)
				if err != nil || row.ID != sessionID || row.ProjectID != v.ProjectID || row.SandboxID != sandboxID || row.Status != registry.StatusStarting {
					return errors.New("the exact reserved child is no longer starting; backend launch refused")
				}
				v.State = "backend-dispatching"
				v.UpdatedAt = time.Now().UTC()
				return nil
			}
		}
		return errors.New("execution capacity is absent or retired; backend launch refused")
	})
}

// A completed backend call means every create/start RPC has returned success.
// Absence before that acknowledgment cannot rule out a late worker appearing.
func (g *executionAdmission) markBackendSettled(ctx context.Context, id, sessionID, sandboxID string) error {
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i := range l.Leases {
			v := &l.Leases[i]
			if v.Scope == g.scope && v.ID == id {
				if v.LaunchGuard != 1 || v.State != "backend-dispatching" || v.SessionID != sessionID || v.SandboxID != sandboxID || sandboxID == "" {
					return errors.New("completed execution no longer matches its durable launch claim")
				}
				v.State, v.UpdatedAt = "backend-settled", time.Now().UTC()
				return nil
			}
		}
		return errors.New("completed execution has no retained capacity claim")
	})
}

// Before the guarded launch edge, removing a lease prevents markBackendStarting
// from ever dispatching it. After that edge only a durable successful full
// backend acknowledgment proves the allocator cannot still create/start a child.
func executionLaunchSettled(lease executionLease) bool {
	return lease.LaunchGuard == 1 && (lease.State == "reserved" || lease.State == "allocating" || lease.State == "replacing" || lease.State == "backend-settled")
}

// The launch edge and tombstone must survive a controller restart and an OS
// crash. File sync alone does not publish the rename durably on every platform.
func writeExecutionLedger(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".execution-*")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if err = codexPrivateFile(name); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return publishAssistanceReceipt(name, path)
}

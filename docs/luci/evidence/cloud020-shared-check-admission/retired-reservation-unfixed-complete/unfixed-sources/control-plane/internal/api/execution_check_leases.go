package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

var errCheckResourcePolicy = errors.New("the saved check resource limits no longer satisfy current policy")

// Checks share the worker ledger's publication and cross-process lock, while
// retaining an explicitly different owner and launch protocol. They are not
// registry sessions and never consume model-provider or agent concurrency.
type checkExecutionLease struct {
	ID              string                       `json:"id"`
	Scope           string                       `json:"scope"`
	Actor           management.ExecutionActorRef `json:"actor"`
	Identity        backend.CheckIdentity        `json:"identity"`
	Resources       backend.Resources            `json:"resources"`
	CreateDiskBytes uint64                       `json:"createDiskBytes"`
	State           string                       `json:"state"`
	ResultDigest    string                       `json:"resultDigest,omitempty"`
	Ref             string                       `json:"ref,omitempty"`
	CreatedAt       time.Time                    `json:"createdAt"`
	UpdatedAt       time.Time                    `json:"updatedAt"`
}

func checkHex(value string, n int) bool {
	b, err := hex.DecodeString(value)
	return err == nil && len(b) == n && strings.ToLower(value) == value
}

func checkLogicalID(value string) bool {
	if value == "" || len(value) > 200 {
		return false
	}
	for _, c := range value {
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '-' || c == '_') {
			return false
		}
	}
	return true
}

func validCheckIdentity(v backend.CheckIdentity) bool {
	return v.Protocol == backend.CheckExecutionProtocol && v.EngineID != "" && len(v.EngineID) <= 200 &&
		!strings.ContainsAny(v.EngineID, "\x00\r\n") && backend.ValidateInstance(v.InstanceID) == nil &&
		checkLogicalID(v.AllocationID) && checkLogicalID(v.CheckID) && v.CommandIndex >= 0 && v.CommandIndex < 8 &&
		checkHex(v.ScopeDigest, 32) && checkHex(v.CommandDigest, 32) && checkLogicalID(v.ProjectID)
}

func validCheckExecutionLease(v checkExecutionLease) bool {
	if v.ID != v.Identity.AllocationID || !validCheckIdentity(v.Identity) || v.Scope != v.Identity.ScopeDigest ||
		v.Actor.ProjectID != v.Identity.ProjectID || len(v.Actor.UserID) > 200 || len(v.Actor.TeamID) > 200 ||
		v.Actor.Local && (v.Actor.UserID != "" || v.Actor.TeamID != "") || !v.Actor.Local && v.Actor.UserID == "" ||
		v.Resources.MemoryMiB < 128 || v.Resources.MemoryMiB > 32768 || v.Resources.CPUMillis < 100 || v.Resources.CPUMillis > 16000 ||
		v.CreateDiskBytes == 0 || v.CreateDiskBytes > 1<<50 || v.CreatedAt.IsZero() || v.UpdatedAt.Before(v.CreatedAt) {
		return false
	}
	if v.State != "finished" && v.ResultDigest != "" {
		return false
	}
	switch v.State {
	case "finished":
		return checkHex(v.ResultDigest, 32) && (v.Ref == "" || checkHex(v.Ref, 32))
	case "reserved", "creating":
		return v.Ref == ""
	case "created", "starting", "running", "cleaning":
		return checkHex(v.Ref, 32)
	case "fencing":
		return v.Ref == "" || checkHex(v.Ref, 32)
	default:
		return false
	}
}

func sameCheckActor(a, b management.ExecutionActorRef, scopeA, scopeB string) bool {
	if a.Local || b.Local {
		return a.Local && b.Local && scopeA == scopeB
	}
	// The policy is per user, including that user's separate team workspaces.
	return a.UserID != "" && a.UserID == b.UserID
}

// addCheckCapacity is called under the one shared ledger lock by BOTH check
// and worker admission. Exact same-engine immutable refs avoid double counting
// visible containers; absent/incomplete observations retain all reserved demand.
func addCheckCapacity(snapshot resourcebudget.Snapshot, checks []checkExecutionLease, skipScope, skipID string, demand *executionCapacityDemand) error {
	for _, v := range checks {
		if v.State == "finished" {
			continue
		}
		if v.Scope == skipScope && v.ID == skipID {
			continue
		}
		observed := false
		for _, c := range snapshot.Execution.Containers {
			if v.Ref != "" && v.Identity.EngineID == snapshot.Execution.EngineID && c.ID == v.Ref && c.Kind == "check" {
				observed = true
				break
			}
		}
		if !observed {
			memory := uint64(v.Resources.MemoryMiB) << 20
			if demand.PendingMemory > ^uint64(0)-memory || demand.AllMemory > ^uint64(0)-memory || demand.PendingDiskBytes > ^uint64(0)-v.CreateDiskBytes {
				return errors.New("check execution capacity exceeds supported accounting")
			}
			demand.PendingMemory += memory
			demand.AllMemory += memory
			demand.CPU += int64(v.Resources.CPUMillis)
			demand.PendingDiskBytes += v.CreateDiskBytes
		}
	}
	return nil
}

func (g *executionAdmission) reserveCheck(ctx context.Context, prepared backend.PreparedCheck, actor management.ExecutionActorRef, authority management.ExecutionAuthority, policy product.SwarmSchedulerPolicy) (executionDecision, error) {
	identity, resources := prepared.Spec.Identity, prepared.Spec.Resources
	out := executionDecision{State: "waiting", Code: "resource_observation_unavailable", Message: "Current machine capacity is not available. No check command was started.", Resources: resources}
	if !validCheckIdentity(identity) || identity.ScopeDigest != g.scope || actor.ProjectID != identity.ProjectID ||
		resources.MemoryMiB < 128 || resources.MemoryMiB > 32768 || resources.CPUMillis < 100 || resources.CPUMillis > 16000 {
		return out, errors.New("check resource or ownership policy is outside supported bounds")
	}
	limit := 1
	if !actor.Local {
		if resources.MemoryMiB > authority.Policy.CheckMemoryMiB || resources.CPUMillis > authority.Policy.CheckCPUMillis || authority.Policy.MaxConcurrentChecksPerUser < 1 {
			return out, errCheckResourcePolicy
		}
		limit = authority.Policy.MaxConcurrentChecksPerUser
	}
	if g.api.deps.ResourceSnapshot == nil {
		return out, nil
	}
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		if err := g.reconcileOwned(ctx, l); err != nil {
			return err
		}
		snap, now := g.api.deps.ResourceSnapshot(), time.Now().UTC()
		if !freshExecutionCapacity(snap, now) || snap.Execution.EngineID != identity.EngineID {
			return nil
		}
		imageBytes := prepared.ImageVirtualSizeBytes
		estimate, err := estimateExecutionCreateDisk(resourcebudget.ExecutionSnapshot{StorageDriver: prepared.StorageDriver, ImageVirtualSizeBytes: &imageBytes})
		if err != nil {
			return err
		}
		existing, checkCount := false, 0
		ownRef := ""
		for _, v := range l.Checks {
			if v.State == "finished" {
				if v.Scope == g.scope && v.ID == identity.AllocationID {
					return errors.New("this exact check command is already finished")
				}
				continue
			}
			if v.Scope == g.scope && v.ID == identity.AllocationID {
				if v.Identity != identity || v.Actor != actor || v.Resources != resources || v.CreateDiskBytes != estimate.Bytes || (v.State != "reserved" && v.State != "created") {
					return errors.New("this exact check command is already claimed or its approved inputs changed")
				}
				existing, ownRef = true, v.Ref
				continue
			}
			if v.Scope == g.scope && v.Identity.ProjectID == identity.ProjectID {
				out.Code, out.Message = "checks_running", "This project already has an active or unconfirmed check allocation."
				return nil
			}
			if sameCheckActor(v.Actor, actor, v.Scope, g.scope) {
				checkCount++
			}
		}
		if checkCount >= limit {
			out.Code, out.Message = "check_limit", "The current per-user check limit is occupied. No command was started."
			return nil
		}
		demand := executionCapacityDemand{RequiredCreateDiskBytes: estimate.Bytes}
		observed := map[string]bool{}
		for _, c := range snap.Execution.Containers {
			if ownRef != "" && c.ID == ownRef && c.Kind == "check" {
				continue
			}
			memory, cpu, ok := executionWorkloadDemand(c, now)
			if !ok {
				out.Code, out.Message = "execution_limits_unknown", "A workload has no verified limits or fresh usage. No check command was started."
				return nil
			}
			demand.AllMemory += memory
			demand.CPU += cpu
			if c.SandboxID != "" {
				observed[c.SandboxID] = true
			}
		}
		for _, v := range l.Leases {
			if v.SessionID == "" || !observed[v.sandboxID()] {
				memory := uint64(v.Resources.MemoryMiB) << 20
				demand.PendingMemory += memory
				demand.AllMemory += memory
				demand.CPU += int64(v.Resources.CPUMillis)
				demand.PendingAllocations++
			}
		}
		// Preserve pending legacy workers that have not yet reached Docker.
		rows, err := g.api.deps.Store.List()
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Status == registry.StatusExited || observed[row.SandboxID] {
				continue
			}
			covered := false
			for _, v := range l.Leases {
				if v.Scope == g.scope && v.SessionID == row.ID {
					covered = true
					break
				}
			}
			if covered {
				continue
			}
			legacy := g.api.sessionResources(ctx)
			memory := uint64(legacy.MemoryMiB) << 20
			demand.PendingMemory += memory
			demand.AllMemory += memory
			demand.CPU += int64(legacy.CPUMillis)
			demand.PendingAllocations++
		}
		if err := addCheckCapacity(snap, l.Checks, g.scope, identity.AllocationID, &demand); err != nil {
			return err
		}
		if code, message := evaluateExecutionCapacity(snap, policy, resources, demand, now); code != "" {
			out.Code, out.Message = code, message
			return nil
		}
		if !existing {
			if len(l.Leases)+len(l.Checks) >= 4096 {
				return errors.New("execution capacity history needs reconciliation")
			}
			l.Checks = append(l.Checks, checkExecutionLease{ID: identity.AllocationID, Scope: g.scope, Actor: actor, Identity: identity, Resources: resources, CreateDiskBytes: estimate.Bytes, State: "reserved", CreatedAt: now, UpdatedAt: now})
		}
		out.State, out.Code, out.Message = "admitted", "", "Check capacity reserved."
		return nil
	})
	return out, err
}

// Transition is a compare-and-swap of an exact attempt; it can never invent a
// lease or regress an acknowledged command into a new allocation opportunity.
func (g *executionAdmission) transitionCheck(ctx context.Context, identity backend.CheckIdentity, from, to, ref string) error {
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i := range l.Checks {
			v := &l.Checks[i]
			if v.Scope != g.scope || v.ID != identity.AllocationID {
				continue
			}
			if v.Identity != identity || v.State != from {
				return errors.New("the exact check launch phase changed")
			}
			allowed := from == "reserved" && to == "creating" || from == "creating" && to == "created" || from == "created" && to == "starting" || from == "starting" && to == "running" || (from == "created" || from == "starting" || from == "running") && to == "cleaning"
			if !allowed || v.Ref != "" && v.Ref != ref {
				return errors.New("check phase transition or immutable ownership is invalid")
			}
			v.State, v.Ref, v.UpdatedAt = to, ref, time.Now().UTC()
			if !validCheckExecutionLease(*v) {
				return errors.New("check phase publication is invalid")
			}
			return nil
		}
		return errors.New("check capacity is absent or retired; launch refused")
	})
}

func (g *executionAdmission) markCheckCleaning(ctx context.Context, identity backend.CheckIdentity, ref string) error {
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i := range l.Checks {
			v := &l.Checks[i]
			if v.Scope != g.scope || v.ID != identity.AllocationID {
				continue
			}
			if v.Identity != identity || v.Ref != ref || !checkHex(ref, 32) {
				return errors.New("check cleanup ownership changed")
			}
			if v.State == "cleaning" {
				return nil
			}
			if v.State != "created" && v.State != "starting" && v.State != "running" {
				return errors.New("check create outcome is unconfirmed; exact launch recovery is required")
			}
			v.State, v.UpdatedAt = "cleaning", time.Now().UTC()
			return nil
		}
		return errors.New("the exact check capacity receipt is unavailable")
	})
}

func (g *executionAdmission) completeCheck(ctx context.Context, identity backend.CheckIdentity, ref string, outcome product.ProjectCheckCommandOutcome) error {
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i := range l.Checks {
			v := &l.Checks[i]
			if v.Scope != g.scope || v.ID != identity.AllocationID {
				continue
			}
			if v.Identity != identity || v.Ref != ref || !outcome.CleanupConfirmed || outcome.AllocationID != identity.AllocationID {
				return errors.New("check completion identity changed")
			}
			if !(v.State == "reserved" && ref == "" || v.State == "cleaning" && checkHex(ref, 32)) {
				return errors.New("check launch or cleanup remains unconfirmed")
			}
			v.State, v.UpdatedAt = "finished", time.Now().UTC()
			v.ResultDigest = checkOutcomeDigest(outcome)
			if !checkHex(v.ResultDigest, 32) {
				return errors.New("check completion result is invalid")
			}
			return nil
		}
		return errors.New("the exact check completion claim is unavailable")
	})
}

func (g *executionAdmission) acknowledgeCheckCompletion(ctx context.Context, identity backend.CheckIdentity) error {
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i, v := range l.Checks {
			if v.Scope != g.scope || v.ID != identity.AllocationID {
				continue
			}
			if v.Identity != identity || v.State != "finished" || !checkHex(v.ResultDigest, 32) {
				return errors.New("check completion has not been durably settled")
			}
			l.Checks = append(l.Checks[:i], l.Checks[i+1:]...)
			return nil
		}
		return nil
	})
}

func checkOutcomeDigest(value product.ProjectCheckCommandOutcome) string {
	value.ResourcesHeld = false
	value.CleanupConfirmed = true
	data, err := json.Marshal(value)
	if err != nil {
		return ""
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

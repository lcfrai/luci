package api

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

var errPreviewLeaseAbsent = errors.New("the exact preview lease is absent")

// Builders and serving containers have distinct immutable allocations. They
// use the worker/check ledger and lock; neither consumes an AI provider slot.
// Revision fences delayed controllers even if a phase happens to recur.
type previewExecutionLease struct {
	ID              string                       `json:"id"`
	Scope           string                       `json:"scope"`
	Actor           management.ExecutionActorRef `json:"actor"`
	Identity        backend.PreviewIdentity      `json:"identity"`
	Kind            string                       `json:"kind"`
	Resources       backend.Resources            `json:"resources"`
	HostLaunchBytes uint64                       `json:"hostLaunchBytes,omitempty"`
	CreateDiskBytes uint64                       `json:"createDiskBytes"`
	State           string                       `json:"state"`
	Ref             string                       `json:"ref,omitempty"`
	Image           string                       `json:"image,omitempty"`
	Revision        int                          `json:"revision"`
	CreatedAt       time.Time                    `json:"createdAt"`
	UpdatedAt       time.Time                    `json:"updatedAt"`
}

func validPreviewIdentity(v backend.PreviewIdentity) bool {
	return v.Protocol == backend.PreviewExecutionProtocol && v.EngineID != "" && len(v.EngineID) <= 200 &&
		!strings.ContainsAny(v.EngineID, "\x00\r\n") && backend.ValidateInstance(v.InstanceID) == nil &&
		checkLogicalID(v.AllocationID) && checkLogicalID(v.ReleaseID) && checkLogicalID(v.ProjectID) &&
		checkHex(v.ScopeDigest, 32) && checkHex(v.SourceDigest, 32) && (checkHex(v.Commit, 20) || checkHex(v.Commit, 32))
}

func validPreviewExecutionLease(v previewExecutionLease) bool {
	if !validPreviewIdentity(v.Identity) || v.ID != v.Identity.AllocationID || v.Scope != v.Identity.ScopeDigest ||
		v.Actor.ProjectID != v.Identity.ProjectID || len(v.Actor.UserID) > 200 || len(v.Actor.TeamID) > 200 ||
		v.Actor.Local && (v.Actor.UserID != "" || v.Actor.TeamID != "") || !v.Actor.Local && v.Actor.UserID == "" ||
		v.Resources.MemoryMiB < 128 || v.Resources.MemoryMiB > 32768 || v.Resources.CPUMillis < 100 || v.Resources.CPUMillis > 16000 ||
		v.CreateDiskBytes == 0 || v.CreateDiskBytes > 1<<50 || v.HostLaunchBytes > 4<<30 || v.Revision < 1 || v.CreatedAt.IsZero() || v.UpdatedAt.Before(v.CreatedAt) {
		return false
	}
	if v.Image != "" && (!strings.HasPrefix(v.Image, "sha256:") || !checkHex(strings.TrimPrefix(v.Image, "sha256:"), 32)) {
		return false
	}
	if v.Kind == "preview-build" {
		return v.Ref == "" && (v.State == "reserved" && v.Image == "" || v.State == "building" && v.Image == "" || v.State == "finished")
	}
	if v.Kind != "preview" || v.Image == "" {
		return false
	}
	switch v.State {
	case "reserved", "creating":
		return v.Ref == ""
	case "created", "starting", "running", "stopping", "stopped", "cleaning":
		return checkHex(v.Ref, 32)
	case "finished":
		return v.Ref == "" || checkHex(v.Ref, 32)
	default:
		return false
	}
}

// Include pending build and serving work in ALL admission paths. An exact
// immutable same-engine serving ref already observed in Docker is counted by
// normal workload demand; unknown build/Create work is never treated as zero.
func addPreviewCapacity(snapshot resourcebudget.Snapshot, rows []previewExecutionLease, skipScope, skipID string, demand *executionCapacityDemand) error {
	for _, v := range rows {
		if v.State == "finished" || v.Scope == skipScope && v.ID == skipID {
			continue
		}
		observed := false
		if v.Ref != "" && v.Identity.EngineID == snapshot.Execution.EngineID {
			for _, c := range snapshot.Execution.Containers {
				if c.ID == v.Ref {
					observed = true
					break
				}
			}
		}
		if !observed {
			memory := uint64(v.Resources.MemoryMiB) << 20
			if demand.PendingMemory > ^uint64(0)-memory || demand.AllMemory > ^uint64(0)-memory || demand.PendingDiskBytes > ^uint64(0)-v.CreateDiskBytes {
				return errors.New("preview capacity exceeds supported accounting")
			}
			demand.PendingMemory += memory
			demand.AllMemory += memory
			demand.CPU += int64(v.Resources.CPUMillis)
			demand.PendingDiskBytes += v.CreateDiskBytes
		}
		allowance := v.HostLaunchBytes
		if allowance == 0 && snapshot.Host.Platform == "windows" {
			allowance = 2048 << 20
		}
		demand.PendingHostLaunchBytes += pendingHostLaunch(snapshot, allowance, v.State == "running", observed, v.UpdatedAt)
	}
	return nil
}

func (g *executionAdmission) reservePreview(ctx context.Context, prepared backend.PreparedPreview, actor management.ExecutionActorRef, authority management.ExecutionAuthority, policy product.SwarmSchedulerPolicy) (executionDecision, previewExecutionLease, error) {
	id, resources := prepared.Spec.Identity, prepared.Spec.Resources
	out := executionDecision{State: "waiting", Code: "resource_observation_unavailable", Message: "Current machine capacity is unavailable. No preview allocation was started.", Resources: resources}
	var result previewExecutionLease
	candidate := previewExecutionLease{ID: id.AllocationID, Scope: g.scope, Actor: actor, Identity: id, Kind: prepared.Spec.Kind, Resources: resources, CreateDiskBytes: prepared.RequiredDiskBytes, State: "reserved", Image: prepared.Spec.ImageID, Revision: 1, CreatedAt: time.Now().UTC()}
	candidate.UpdatedAt = candidate.CreatedAt
	if !validPreviewExecutionLease(candidate) || id.ScopeDigest != g.scope {
		return out, result, errors.New("preview allocation identity or resource budget is invalid")
	}
	if !actor.Local && (resources.MemoryMiB > authority.Policy.CheckMemoryMiB || resources.CPUMillis > authority.Policy.CheckCPUMillis || authority.Policy.MaxConcurrentChecksPerUser < 1) {
		return out, result, errors.New("preview limits exceed current deployment policy")
	}
	if g.api.deps.ResourceSnapshot == nil {
		return out, result, nil
	}
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		if err := g.reconcileOwned(ctx, l); err != nil {
			return err
		}
		snap, now := g.api.deps.ResourceSnapshot(), time.Now().UTC()
		if !freshExecutionCapacity(snap, now) || snap.Execution.EngineID != id.EngineID {
			return nil
		}
		existing, ownRef, builds := false, "", 0
		for _, v := range l.Previews {
			if v.Scope == g.scope && v.ID == id.AllocationID {
				if v.Identity != id || v.Actor != actor || v.Resources != resources || v.Kind != candidate.Kind || v.CreateDiskBytes != candidate.CreateDiskBytes || v.Image != candidate.Image || (v.State != "reserved" && v.State != "created" && v.State != "running" && v.State != "stopped") {
					return errors.New("preview allocation already claimed, settled, or changed")
				}
				existing, ownRef, result = true, v.Ref, v
				continue
			}
			if v.State == "finished" {
				continue
			}
			if v.Kind == "preview-build" && candidate.Kind == "preview-build" {
				if v.Scope == g.scope && v.Identity.ProjectID == id.ProjectID {
					out.Code, out.Message = "preview_build_running", "This project has an active or unconfirmed preview build."
					return nil
				}
				if sameCheckActor(v.Actor, actor, v.Scope, g.scope) {
					builds++
				}
			}
		}
		limit := 1
		if !actor.Local {
			limit = authority.Policy.MaxConcurrentChecksPerUser
		}
		if candidate.Kind == "preview-build" && builds >= limit {
			out.Code, out.Message = "preview_build_limit", "The current preview build limit is occupied."
			return nil
		}
		demand := executionCapacityDemand{RequiredCreateDiskBytes: prepared.RequiredDiskBytes, CreateAcknowledged: ownRef != ""}
		observed := map[string]bool{}
		for _, c := range snap.Execution.Containers {
			if ownRef != "" && c.ID == ownRef {
				continue
			}
			memory, cpu, ok := executionWorkloadDemand(c, now)
			if !ok {
				out.Code, out.Message = "execution_limits_unknown", "A workload has neither verified limits nor fresh usage. No preview allocation was started."
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
		addWorkerHostLaunchCapacity(snap, l.Leases, "", "", g.api.deps.Backend.Kind(), &demand)
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
			if !covered {
				out.Code, out.Message = "execution_legacy_demand_unknown", "An existing worker's allocation is not verified. Wait for its exact Docker observation or recovery before starting a preview."
				return nil
			}
		}
		if err = addCheckCapacity(snap, l.Checks, "", "", &demand); err != nil {
			return err
		}
		if err = addPreviewCapacity(snap, l.Previews, g.scope, id.AllocationID, &demand); err != nil {
			return err
		}
		if err := addRuntimeImageCapacity(l.RuntimeImages, "", "", &demand); err != nil {
			return err
		}
		demand.HostLaunchBytes = windowsDockerHostLaunchBytes(snap, g.api.deps.Backend.Kind())
		capacity := evaluateExecutionCapacityWithEvidence(snap, policy, resources, demand, now)
		if capacity.Code != "" {
			out.Code, out.Message, out.Evidence = capacity.Code, capacity.Message, capacity.Evidence
			return nil
		}
		if !existing {
			if len(l.Leases)+len(l.Checks)+len(l.Previews)+len(l.RuntimeImages) >= 4096 {
				return errors.New("execution capacity history needs reconciliation")
			}
			candidate.CreatedAt, candidate.UpdatedAt = now, now
			candidate.HostLaunchBytes = demand.HostLaunchBytes
			l.Previews = append(l.Previews, candidate)
			result = candidate
		}
		out.State, out.Code, out.Message = "admitted", "capacity_reserved", "Shared capacity reserved for this exact preview allocation."
		return nil
	})
	return out, result, err
}

// Caller owns the exact lifecycle lock. Every mutation also compares the
// durable phase revision, preventing a delayed publication from undoing Stop.
func (g *executionAdmission) transitionPreview(ctx context.Context, before previewExecutionLease, next, ref, image string) (previewExecutionLease, error) {
	var out previewExecutionLease
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i, v := range l.Previews {
			if v.Scope != g.scope || v.ID != before.ID {
				continue
			}
			if v != before {
				return errors.New("the exact preview phase changed")
			}
			allowed := v.Kind == "preview-build" && v.State == "reserved" && next == "building" ||
				v.Kind == "preview" && (v.State == "reserved" && next == "creating" || v.State == "creating" && next == "created" || (v.State == "created" || v.State == "stopped") && next == "starting" || v.State == "starting" && next == "running" || v.State == "running" && next == "stopping" || v.State == "stopping" && next == "stopped" || (v.State == "created" || v.State == "starting" || v.State == "running" || v.State == "stopping" || v.State == "stopped") && next == "cleaning")
			if !allowed || v.Ref != "" && v.Ref != ref || v.Image != "" && v.Image != image {
				return errors.New("preview phase transition or immutable ownership is invalid")
			}
			v.State, v.Ref, v.Image, v.UpdatedAt = next, ref, image, time.Now().UTC()
			v.Revision++
			if !validPreviewExecutionLease(v) {
				return errors.New("preview phase publication is invalid")
			}
			l.Previews[i], out = v, v
			return nil
		}
		return errors.New("preview capacity receipt is absent or retired")
	})
	return out, err
}

// settled is supplied only by the exact backend build/cleanup proof. It may
// never be synthesized from a timeout, canceled request, empty list or health.
func (g *executionAdmission) completePreview(ctx context.Context, before previewExecutionLease, settled bool, image string) (previewExecutionLease, error) {
	var out previewExecutionLease
	if !settled {
		return out, errors.New("preview allocation settlement is unconfirmed; capacity remains held")
	}
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i, v := range l.Previews {
			if v.Scope != g.scope || v.ID != before.ID {
				continue
			}
			if v != before || !(v.State == "reserved" || v.Kind == "preview-build" && v.State == "building" || v.Kind == "preview" && v.State == "cleaning") || v.Image != "" && v.Image != image {
				return errors.New("preview completion changed or launch remains unconfirmed")
			}
			v.State, v.Image, v.UpdatedAt = "finished", image, time.Now().UTC()
			v.Revision++
			if !validPreviewExecutionLease(v) {
				return errors.New("preview completion identity is invalid")
			}
			l.Previews[i], out = v, v
			return nil
		}
		return errors.New("preview completion receipt unavailable")
	})
	return out, err
}

func (g *executionAdmission) acknowledgePreviewCompletion(ctx context.Context, before previewExecutionLease) error {
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i, v := range l.Previews {
			if v.Scope != g.scope || v.ID != before.ID {
				continue
			}
			if v != before || v.State != "finished" {
				return errors.New("preview completion has not been durably settled")
			}
			l.Previews = append(l.Previews[:i], l.Previews[i+1:]...)
			return nil
		}
		return nil
	})
}

func (g *executionAdmission) previewLease(ctx context.Context, id string) (previewExecutionLease, error) {
	if err := ctx.Err(); err != nil {
		return previewExecutionLease{}, err
	}
	l, err := g.readReservationLedger(ctx)
	if err != nil {
		return previewExecutionLease{}, err
	}
	for _, v := range l.Previews {
		if v.Scope == g.scope && v.ID == id {
			return v, nil
		}
	}
	return previewExecutionLease{}, errPreviewLeaseAbsent
}

func (g *executionAdmission) acknowledgePreviewRelease(ctx context.Context, rows []previewExecutionLease) error {
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		for _, want := range rows {
			found := false
			for _, v := range l.Previews {
				if v.Scope == g.scope && v.ID == want.ID {
					if v != want || v.State != "finished" {
						return errors.New("preview release completion changed")
					}
					found = true
					break
				}
			}
			if !found {
				return errors.New("preview release lineage disappeared before acknowledgement")
			}
		}
		kept := l.Previews[:0]
		for _, v := range l.Previews {
			remove := false
			for _, want := range rows {
				if v.Scope == g.scope && v.ID == want.ID {
					remove = true
					break
				}
			}
			if !remove {
				kept = append(kept, v)
			}
		}
		l.Previews = kept
		return nil
	})
}

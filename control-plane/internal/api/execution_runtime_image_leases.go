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
)

var errRuntimeImageLeaseAbsent = errors.New("exact runtime installer allocation is absent")

type runtimeImageExecutionLease struct {
	ID                   string                       `json:"id"`
	Scope                string                       `json:"scope"`
	Actor                management.ExecutionActorRef `json:"actor"`
	Identity             backend.RuntimeImageIdentity `json:"identity"`
	Resources            backend.Resources            `json:"resources"`
	AdmissionMemoryMiB   int                          `json:"admissionMemoryMiB,omitempty"`
	CreateDiskBytes      uint64                       `json:"createDiskBytes"`
	ImageTag             string                       `json:"imageTag"`
	DistributionMode     string                       `json:"distributionMode,omitempty"`
	ImportStage          string                       `json:"importStage,omitempty"`
	PayloadImageID       string                       `json:"payloadImageId,omitempty"`
	State                string                       `json:"state"`
	Image                string                       `json:"image,omitempty"`
	Revision             int                          `json:"revision"`
	CreatedAt            time.Time                    `json:"createdAt"`
	UpdatedAt            time.Time                    `json:"updatedAt"`
	EngineRecoveryStage  string                       `json:"engineRecoveryStage,omitempty"`
	EngineWatchStartedAt time.Time                    `json:"engineWatchStartedAt,omitzero"`
	EngineStoppedAt      time.Time                    `json:"engineStoppedAt,omitzero"`
}

func validRuntimeImageExecutionLease(v runtimeImageExecutionLease) bool {
	i := v.Identity
	return i.Protocol == backend.RuntimeImageExecutionProtocol && i.EngineID != "" && len(i.EngineID) <= 200 && !strings.ContainsAny(i.EngineID, "\x00\r\n") && backend.ValidateInstance(i.InstanceID) == nil &&
		checkLogicalID(i.AllocationID) && checkLogicalID(i.JobID) && checkHex(i.ScopeDigest, 32) && checkHex(i.SourceDigest, 32) && v.ID == i.AllocationID && v.Scope == i.ScopeDigest &&
		v.Actor.ProjectID == "" && len(v.Actor.UserID) <= 200 && len(v.Actor.TeamID) <= 200 && (v.Actor.Local && v.Actor.UserID == "" && v.Actor.TeamID == "" || !v.Actor.Local && v.Actor.UserID != "") &&
		v.Resources.MemoryMiB >= 128 && v.Resources.MemoryMiB <= 32768 && v.Resources.CPUMillis >= 100 && v.Resources.CPUMillis <= 16000 &&
		(v.AdmissionMemoryMiB == 0 || v.AdmissionMemoryMiB >= v.Resources.MemoryMiB && v.AdmissionMemoryMiB <= 65536) &&
		v.CreateDiskBytes >= backend.RuntimeImageRequiredDiskBytes && v.CreateDiskBytes <= 1<<50 && v.ImageTag != "" && len(v.ImageTag) <= 300 && !strings.ContainsAny(v.ImageTag, "\x00\r\n \t") &&
		v.Revision >= 1 && !v.CreatedAt.IsZero() && !v.UpdatedAt.Before(v.CreatedAt) &&
		(v.State == "reserved" && v.Image == "" || v.State == "building" && v.Image == "" || v.State == "finished" && (v.Image == "" || strings.HasPrefix(v.Image, "sha256:") && checkHex(strings.TrimPrefix(v.Image, "sha256:"), 32))) &&
		(v.DistributionMode == "" && v.ImportStage == "" && v.PayloadImageID == "" || v.DistributionMode == "packaged" && (v.ImportStage == "" && (v.State == "reserved" || v.State == "finished") && v.PayloadImageID == "" || v.ImportStage == "importing" && (v.State == "building" || v.State == "finished") && v.PayloadImageID == "" || (v.ImportStage == "imported" || v.ImportStage == "deriving") && (v.State == "building" || v.State == "finished") && strings.HasPrefix(v.PayloadImageID, "sha256:") && checkHex(strings.TrimPrefix(v.PayloadImageID, "sha256:"), 32))) &&
		(v.EngineRecoveryStage == "" && v.EngineWatchStartedAt.IsZero() && v.EngineStoppedAt.IsZero() ||
			v.EngineRecoveryStage == "watching" && !v.EngineWatchStartedAt.IsZero() && v.EngineStoppedAt.IsZero() ||
			v.EngineRecoveryStage == "stopped" && !v.EngineWatchStartedAt.IsZero() && !v.EngineStoppedAt.Before(v.EngineWatchStartedAt))
}

func (g *executionAdmission) advanceRuntimeImageEngineRetirement(ctx context.Context, before runtimeImageExecutionLease, stage string) (runtimeImageExecutionLease, error) {
	var out runtimeImageExecutionLease
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i, v := range l.RuntimeImages {
			if v.Scope == g.scope && v.ID == before.ID {
				if v != before || v.State != "building" || stage == "watching" && v.EngineRecoveryStage != "" || stage == "stopped" && v.EngineRecoveryStage != "watching" || stage != "watching" && stage != "stopped" {
					return errors.New("runtime engine retirement observation changed")
				}
				now := time.Now().UTC()
				if stage == "watching" {
					v.EngineWatchStartedAt = now
				} else {
					v.EngineStoppedAt = now
				}
				v.EngineRecoveryStage, v.UpdatedAt = stage, now
				v.Revision++
				if !validRuntimeImageExecutionLease(v) {
					return errors.New("runtime engine retirement receipt is invalid")
				}
				l.RuntimeImages[i], out = v, v
				return nil
			}
		}
		return errRuntimeImageLeaseAbsent
	})
	return out, err
}
func addRuntimeImageCapacity(rows []runtimeImageExecutionLease, skipScope, skipID string, demand *executionCapacityDemand) error {
	for _, v := range rows {
		if v.State == "finished" || v.Scope == skipScope && v.ID == skipID {
			continue
		}
		memory := uint64(max(v.Resources.MemoryMiB, v.AdmissionMemoryMiB)) << 20
		if demand.PendingMemory > ^uint64(0)-memory || demand.AllMemory > ^uint64(0)-memory || demand.PendingDiskBytes > ^uint64(0)-v.CreateDiskBytes {
			return errors.New("runtime installer demand exceeds supported capacity accounting")
		}
		demand.PendingMemory += memory
		demand.AllMemory += memory
		demand.CPU += int64(v.Resources.CPUMillis)
		demand.PendingDiskBytes += v.CreateDiskBytes
	}
	return nil
}
func addRuntimeImageReservationView(view *executionReservationView, rows []runtimeImageExecutionLease) {
	for _, v := range rows {
		if v.State != "finished" {
			view.Count++
			view.MemoryMiB += max(v.Resources.MemoryMiB, v.AdmissionMemoryMiB)
			view.CPUMillis += v.Resources.CPUMillis
		}
	}
}
func (g *executionAdmission) reserveRuntimeImage(ctx context.Context, prepared backend.PreparedRuntimeImage, actor management.ExecutionActorRef, authority management.ExecutionAuthority, policy product.SwarmSchedulerPolicy) (executionDecision, runtimeImageExecutionLease, error) {
	return g.reserveRuntimeImageStage(ctx, prepared, actor, authority, policy, false)
}

// Once both pinned base pulls have returned complete streams and exact image
// readbacks, their retained host demand is already in the fresh observation.
// Evaluate only the remaining capped build, while retaining the original
// conservative lease for competing admissions until exact build settlement.
func (g *executionAdmission) reserveRuntimeImageAfterPull(ctx context.Context, prepared backend.PreparedRuntimeImage, actor management.ExecutionActorRef, authority management.ExecutionAuthority, policy product.SwarmSchedulerPolicy) (executionDecision, runtimeImageExecutionLease, error) {
	return g.reserveRuntimeImageStage(ctx, prepared, actor, authority, policy, true)
}

func (g *executionAdmission) reserveRuntimeImageStage(ctx context.Context, prepared backend.PreparedRuntimeImage, actor management.ExecutionActorRef, authority management.ExecutionAuthority, policy product.SwarmSchedulerPolicy, afterPull bool) (executionDecision, runtimeImageExecutionLease, error) {
	id, resources := prepared.Spec.Identity, prepared.Spec.Resources
	admissionMemory := max(resources.MemoryMiB, prepared.AdmissionMemoryMiB)
	admissionResources := resources
	admissionResources.MemoryMiB = admissionMemory
	if afterPull {
		admissionResources.MemoryMiB = resources.MemoryMiB
	}
	out := executionDecision{State: "waiting", Code: "resource_observation_unavailable", Message: "Current machine capacity is unavailable; runtime installation has not started.", Resources: resources}
	var result runtimeImageExecutionLease
	candidate := runtimeImageExecutionLease{ID: id.AllocationID, Scope: g.scope, Actor: actor, Identity: id, Resources: resources,
		AdmissionMemoryMiB: admissionMemory, CreateDiskBytes: prepared.RequiredDiskBytes,
		ImageTag: prepared.Spec.ImageTag, State: "reserved", Revision: 1, CreatedAt: time.Now().UTC()}
	if prepared.Bundle != nil {
		candidate.DistributionMode = "packaged"
	}
	candidate.UpdatedAt = candidate.CreatedAt
	if !validRuntimeImageExecutionLease(candidate) || id.ScopeDigest != g.scope {
		return out, result, errors.New("runtime installer allocation identity or budget is invalid")
	}
	if !actor.Local && (resources.MemoryMiB > authority.Policy.CheckMemoryMiB || resources.CPUMillis > authority.Policy.CheckCPUMillis) {
		return out, result, errors.New("runtime installer resources exceed current operator policy")
	}
	if g.api.deps.ResourceSnapshot == nil {
		out.RuntimeAdmission = runtimeAdmissionObservation(nil, id.EngineID, time.Now().UTC())
		return out, result, nil
	}
	// Preserve the existing 32 GiB bounded installation demand +8 GiB reserve.
	policy.DiskReserveMiB = max(policy.DiskReserveMiB, 8192)
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		if err := g.reconcileOwned(ctx, l); err != nil {
			return err
		}
		snap, now := g.api.deps.ResourceSnapshot(), time.Now().UTC()
		out.RuntimeAdmission = runtimeAdmissionObservation(&snap, id.EngineID, now)
		if !freshExecutionCapacity(snap, now) || snap.Execution.EngineID != id.EngineID {
			return nil
		}
		existing := false
		for _, v := range l.RuntimeImages {
			if v.Scope == g.scope && v.ID == id.AllocationID {
				if v.Identity != id || v.Actor != actor || v.Resources != resources || v.AdmissionMemoryMiB != admissionMemory || v.CreateDiskBytes != candidate.CreateDiskBytes || v.ImageTag != candidate.ImageTag || v.DistributionMode != candidate.DistributionMode || v.State != "reserved" && v.State != "building" || afterPull && v.State != "building" {
					return errors.New("runtime installer allocation already claimed, changed or settled")
				}
				existing, result = true, v
				continue
			}
			// Finished retains the engine-wide identity slot through image publication.
			// It carries no resource charge; acknowledgement retires the exact lineage.
			if v.Identity.EngineID == id.EngineID {
				out.Code, out.Message = "runtime_install_retained", "Another runtime installation or its publication still requires settlement."
				return nil
			}
		}
		demand := executionCapacityDemand{RequiredCreateDiskBytes: prepared.RequiredDiskBytes}
		observed := map[string]bool{}
		for _, c := range snap.Execution.Containers {
			memory, cpu, ok := executionWorkloadDemand(c, now)
			if !ok {
				out.Code, out.Message = "execution_limits_unknown", "A workload has neither verified limits nor fresh usage. No runtime installation was started."
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
				out.Code, out.Message = "execution_legacy_demand_unknown", "An existing worker's allocation is not verified. Wait for its exact Docker observation or recovery before installing the runtime."
				return nil
			}
		}
		if err = addCheckCapacity(snap, l.Checks, "", "", &demand); err != nil {
			return err
		}
		if err = addPreviewCapacity(snap, l.Previews, "", "", &demand); err != nil {
			return err
		}
		if err := addRuntimeImageCapacity(l.RuntimeImages, g.scope, id.AllocationID, &demand); err != nil {
			return err
		}
		capacity := evaluateExecutionCapacityWithEvidence(snap, policy, admissionResources, demand, now)
		if capacity.Code != "" {
			out.Code, out.Message, out.Evidence = capacity.Code, capacity.Message, capacity.Evidence
			if prepared.Bundle != nil && capacity.Code == "memory_pressure" {
				out.Message = "Waiting for memory to import the verified runtime. Docker stages the image archive before registering its layers; this load allowance and the host reserve cannot fit now. You can keep using Luci while capacity recovers."
			}
			return nil
		}
		if !existing {
			if len(l.Leases)+len(l.Checks)+len(l.Previews)+len(l.RuntimeImages) >= 4096 {
				return errors.New("execution capacity history needs reconciliation")
			}
			candidate.CreatedAt, candidate.UpdatedAt = now, now
			l.RuntimeImages = append(l.RuntimeImages, candidate)
			result = candidate
		}
		out.State, out.Code, out.Message = "admitted", "capacity_reserved", "Shared capacity reserved for the exact bundled runtime build."
		if prepared.Bundle != nil {
			out.Message = "Shared capacity reserved for the verified runtime import."
		}
		return nil
	})
	return out, result, err
}
func (g *executionAdmission) transitionRuntimeImage(ctx context.Context, before runtimeImageExecutionLease, next string) (runtimeImageExecutionLease, error) {
	var out runtimeImageExecutionLease
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i, v := range l.RuntimeImages {
			if v.Scope == g.scope && v.ID == before.ID {
				if v != before || v.State != "reserved" || next != "building" {
					return errors.New("runtime installer phase already changed")
				}
				v.State, v.UpdatedAt = next, time.Now().UTC()
				if v.DistributionMode == "packaged" {
					v.ImportStage = "importing"
				}
				v.Revision++
				l.RuntimeImages[i], out = v, v
				return nil
			}
		}
		return errRuntimeImageLeaseAbsent
	})
	return out, err
}
func (g *executionAdmission) advanceRuntimeImageImport(ctx context.Context, before runtimeImageExecutionLease, payloadID string) (runtimeImageExecutionLease, error) {
	var out runtimeImageExecutionLease
	if !strings.HasPrefix(payloadID, "sha256:") || !checkHex(strings.TrimPrefix(payloadID, "sha256:"), 32) {
		return out, errors.New("runtime payload image ID invalid")
	}
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i, v := range l.RuntimeImages {
			if v.Scope == g.scope && v.ID == before.ID {
				if v != before || v.DistributionMode != "packaged" || v.State != "building" || v.ImportStage != "importing" {
					return errors.New("runtime import phase changed")
				}
				v.ImportStage, v.PayloadImageID, v.UpdatedAt = "imported", payloadID, time.Now().UTC()
				v.Revision++
				if !validRuntimeImageExecutionLease(v) {
					return errors.New("runtime import receipt invalid")
				}
				l.RuntimeImages[i], out = v, v
				return nil
			}
		}
		return errRuntimeImageLeaseAbsent
	})
	return out, err
}
func (g *executionAdmission) beginRuntimeImageDerivation(ctx context.Context, before runtimeImageExecutionLease) (runtimeImageExecutionLease, error) {
	var out runtimeImageExecutionLease
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i, v := range l.RuntimeImages {
			if v.Scope == g.scope && v.ID == before.ID {
				if v != before || v.DistributionMode != "packaged" || v.State != "building" || v.ImportStage != "imported" {
					return errors.New("runtime derivative phase changed")
				}
				v.ImportStage, v.UpdatedAt = "deriving", time.Now().UTC()
				v.Revision++
				l.RuntimeImages[i], out = v, v
				return nil
			}
		}
		return errRuntimeImageLeaseAbsent
	})
	return out, err
}
func (g *executionAdmission) completeRuntimeImage(ctx context.Context, before runtimeImageExecutionLease, settled bool, image string) (runtimeImageExecutionLease, error) {
	var out runtimeImageExecutionLease
	if !settled {
		return out, errors.New("runtime build settlement is unknown; capacity remains held")
	}
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i, v := range l.RuntimeImages {
			if v.Scope == g.scope && v.ID == before.ID {
				if v != before || (v.State != "reserved" && v.State != "building") {
					return errors.New("runtime build completion changed")
				}
				v.State, v.Image, v.UpdatedAt = "finished", image, time.Now().UTC()
				v.Revision++
				if !validRuntimeImageExecutionLease(v) {
					return errors.New("runtime build completion identity is invalid")
				}
				l.RuntimeImages[i], out = v, v
				return nil
			}
		}
		return errRuntimeImageLeaseAbsent
	})
	return out, err
}
func (g *executionAdmission) acknowledgeRuntimeImageCompletion(ctx context.Context, before runtimeImageExecutionLease) error {
	return g.mutate(ctx, func(l *executionLeaseLedger) error {
		for i, v := range l.RuntimeImages {
			if v.Scope == g.scope && v.ID == before.ID {
				if v != before || v.State != "finished" {
					return errors.New("runtime installer publication is not durably settled")
				}
				l.RuntimeImages = append(l.RuntimeImages[:i], l.RuntimeImages[i+1:]...)
				return nil
			}
		}
		return errRuntimeImageLeaseAbsent
	})
}
func (g *executionAdmission) runtimeImageLease(ctx context.Context, id string) (runtimeImageExecutionLease, error) {
	if err := ctx.Err(); err != nil {
		return runtimeImageExecutionLease{}, err
	}
	l, err := g.readReservationLedger(ctx)
	if err != nil {
		return runtimeImageExecutionLease{}, err
	}
	for _, v := range l.RuntimeImages {
		if v.Scope == g.scope && v.ID == id {
			return v, nil
		}
	}
	return runtimeImageExecutionLease{}, errRuntimeImageLeaseAbsent
}

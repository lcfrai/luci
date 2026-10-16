package api

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

func assemblyLeaseIndex(l *executionLeaseLedger, identity backend.DocumentImageAssemblyIdentity) (int, error) {
	for i, row := range l.ImageAssemblies {
		if row.Scope == identity.ScopeDigest && row.ID == identity.AllocationID {
			if row.Identity != identity {
				return -1, backend.ErrLaunchFenceOwnership
			}
			return i, nil
		}
	}
	return -1, backend.ErrLaunchFenceOwnership
}

func assemblyOtherClaims(l executionLeaseLedger, scope, id string) bool {
	if len(l.Leases) != 0 {
		return true
	}
	for _, row := range l.Checks {
		if row.State != "finished" {
			return true
		}
	}
	for _, row := range l.Previews {
		if row.State != "finished" {
			return true
		}
	}
	// Retained installers include publication/import/recovery authority and cannot
	// overlap this purpose even after their live resource charge has ended.
	if len(l.RuntimeImages) != 0 {
		return true
	}
	for _, row := range l.Documents {
		if row.State != "finished" {
			return true
		}
	}
	for _, row := range l.ImageAssemblies {
		if row.State != "finished" && !(row.Scope == scope && row.ID == id) {
			return true
		}
	}
	return false
}

func (g *executionAdmission) reserveDocumentImageAssembly(ctx context.Context, p backend.PreparedDocumentImageAssembly, req documentImageAssemblyRequest, authority management.ExecutionAuthority, policy product.SwarmSchedulerPolicy) (documentImageAssemblyLease, error) {
	var result documentImageAssemblyLease
	id, resources := p.Spec.Identity, backend.DocumentImageAssemblyResources()
	unprepared := p.Spec
	unprepared.Identity.EngineID, unprepared.Identity.InstanceID = "", ""
	if unprepared != req.Spec {
		return result, backend.ErrLaunchFenceOwnership
	}
	if !validAssemblyRequest(req) || !backend.ValidateDocumentImageAssemblySpec(p.Spec) || id.ScopeDigest != g.scope || id.ProjectID != "" || req.Actor.ProjectID != "" || req.Actor.TeamID != "" || id.ActorDigest != backend.DocumentExtractionActorDigest(req.Actor.UserID, req.Actor.TeamID, req.Actor.ProjectID, req.Actor.Local) {
		return result, backend.ErrLaunchFenceOwnership
	}
	if !req.Actor.Local && (resources.MemoryMiB > authority.Policy.CheckMemoryMiB || resources.CPUMillis > authority.Policy.CheckCPUMillis) {
		return result, errCheckResourcePolicy
	}
	if g.api.deps.ResourceSnapshot == nil {
		return result, backend.ErrUnavailable
	}
	imageBytes := p.BaseVirtualSizeBytes
	estimate, err := estimateExecutionCreateDisk(resourcebudget.ExecutionSnapshot{StorageDriver: p.StorageDriver, ImageVirtualSizeBytes: &imageBytes})
	if err != nil {
		return result, err
	}
	err = g.mutate(ctx, func(l *executionLeaseLedger) error {
		// Resolve the actual current operator/grant and source under admission,
		// never preserve the authority value supplied before waiting for its lock.
		current, err := g.api.authorizeRuntimeInstaller(ctx, req.Actor)
		if err != nil {
			return err
		}
		if !req.Actor.Local && (resources.MemoryMiB > current.Policy.CheckMemoryMiB || resources.CPUMillis > current.Policy.CheckCPUMillis) {
			return errCheckResourcePolicy
		}
		if err = req.ValidateCurrent(ctx); err != nil {
			return err
		}
		if !time.Now().Before(req.ComputeDeadline) {
			return context.DeadlineExceeded
		}
		if err = g.reconcileOwned(ctx, l); err != nil {
			return err
		}
		if assemblyOtherClaims(*l, g.scope, id.AllocationID) {
			return errDocumentImageAssemblySerial
		}
		for _, row := range l.ImageAssemblies {
			if row.Scope == g.scope && row.ID == id.AllocationID {
				// An invocation may resume admission only before any allocation.
				if row.Identity != id || row.Actor != req.Actor || row.State != "reserved" || row.Ref != "" || row.CreateDiskBytes != estimate.Bytes || row.CreatedAt != req.CreatedAt || row.ComputeDeadline != req.ComputeDeadline || row.WholeDeadline != req.WholeDeadline {
					return backend.ErrLaunchFenceOwnership
				}
				result = row
			}
		}
		snap, now := g.api.deps.ResourceSnapshot(), time.Now().UTC()
		if !freshExecutionCapacity(snap, now) || snap.Execution.EngineID != id.EngineID || len(snap.Execution.Containers) != 0 {
			return backend.ErrUnavailable
		}
		rows, err := g.api.deps.Store.List()
		if err != nil {
			return err
		}
		for _, row := range rows {
			if row.Status != registry.StatusExited {
				return errDocumentImageAssemblySerial
			}
		}
		candidate := documentImageAssemblyLease{ID: id.AllocationID, Scope: g.scope, Actor: req.Actor, Identity: id, Resources: resources, BaseImageID: p.Spec.BaseImageID, CreateDiskBytes: estimate.Bytes, CaptureDiskBytes: backend.DocumentImageAssemblyOutputMaxBytes, HostLaunchBytes: windowsDockerHostLaunchBytes(snap, backend.KindDocker), State: "reserved", CreatedAt: req.CreatedAt, UpdatedAt: now, ComputeDeadline: req.ComputeDeadline, WholeDeadline: req.WholeDeadline}
		if !validDocumentImageAssemblyLease(candidate) {
			return backend.ErrLaunchFenceOwnership
		}
		captureRows := l.ImageAssemblies
		if result.ID == "" {
			captureRows = append(append([]documentImageAssemblyLease{}, captureRows...), candidate)
		} else if result.HostLaunchBytes != candidate.HostLaunchBytes {
			return backend.ErrLaunchFenceOwnership
		}
		demand := executionCapacityDemand{RequiredCreateDiskBytes: estimate.Bytes, PendingDiskBytes: backend.DocumentImageAssemblyOutputMaxBytes, HostLaunchBytes: candidate.HostLaunchBytes}
		// Candidate capture is charged once above; skipping its immutable lease
		// removes only that duplicate. Exact-volume sum includes it exactly once.
		if err = g.addAssemblyAdmissionCapacity(ctx, snap, captureRows, g.scope, id.AllocationID, &demand, policy.DiskReserveMiB); err != nil {
			return err
		}
		if err = addCheckCapacity(snap, l.Checks, "", "", &demand); err != nil {
			return err
		}
		if err = addPreviewCapacity(snap, l.Previews, "", "", &demand); err != nil {
			return err
		}
		if err = addRuntimeImageCapacity(l.RuntimeImages, "", "", &demand); err != nil {
			return err
		}
		if err = addDocumentCapacity(snap, l.Documents, "", "", &demand); err != nil {
			return err
		}
		if code, message := evaluateExecutionCapacity(snap, policy, resources, demand, now); code != "" {
			return errors.New(message)
		}
		if result.ID == "" {
			if executionLeaseCount(*l) >= 4096 {
				return errors.New("execution capacity history needs reconciliation")
			}
			l.ImageAssemblies = append(l.ImageAssemblies, candidate)
			result = candidate
		}
		return nil
	})
	return result, err
}

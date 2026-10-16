package api

import (
	"context"
	"errors"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
	"time"
)

func (g *executionAdmission) reserveDocument(ctx context.Context, prepared backend.PreparedDocumentExtraction, request product.DiscoveryExtractionRequest, authority management.ExecutionAuthority, policy product.SwarmSchedulerPolicy) (executionDecision, error) {
	identity, resources := prepared.Spec.Identity, backend.DocumentExtractionResources()
	actor := request.Actor
	out := executionDecision{State: "waiting", Code: "resource_observation_unavailable", Message: "Current machine capacity is not available. No document extraction was started.", Resources: resources}
	if !backend.ValidateDocumentExtractionIdentity(identity, true) || !backend.ValidateDocumentExtractionSpec(prepared.Spec) || identity.ScopeDigest != g.scope || actor.ProjectID != identity.ProjectID || identity.ActorDigest != backend.DocumentExtractionActorDigest(actor.UserID, actor.TeamID, actor.ProjectID, actor.Local) || identity.AllocationID != request.AllocationID || identity.DocumentID != request.DocumentID || identity.AttemptID != request.AttemptID || identity.OriginalDigest != request.Original.Digest || prepared.Spec.OriginalPath != request.OriginalPath || prepared.Spec.OriginalBytes != request.Original.Bytes || prepared.Spec.MimeType != request.Original.MimeType ||
		resources.MemoryMiB < 128 || resources.MemoryMiB > 32768 || resources.CPUMillis < 100 || resources.CPUMillis > 16000 {
		return out, errors.New("document resource or ownership policy is outside supported bounds")
	}
	if !product.ValidateDiscoveryExtractionRequest(request) || !time.Now().Before(request.ComputeDeadline) {
		return out, errors.New("document attempt deadline or identity is invalid")
	}
	if !actor.Local && (resources.MemoryMiB > authority.Policy.CheckMemoryMiB || resources.CPUMillis > authority.Policy.CheckCPUMillis || authority.Policy.MaxConcurrentChecksPerUser < 1) {
		return out, errCheckResourcePolicy
	}
	if g.api.deps.ResourceSnapshot == nil {
		return out, nil
	}
	err := g.mutate(ctx, func(l *executionLeaseLedger) error {
		if err := g.reconcileOwned(ctx, l); err != nil {
			return err
		}
		snap, now := g.api.deps.ResourceSnapshot(), time.Now().UTC()
		if !now.Before(request.ComputeDeadline) {
			out.Code, out.Message = "document_deadline", "The exact extraction compute deadline elapsed; no replacement was allocated."
			return nil
		}
		if !freshExecutionCapacity(snap, now) || snap.Execution.EngineID != identity.EngineID {
			return nil
		}
		imageBytes := prepared.ImageVirtualSizeBytes
		estimate, err := estimateExecutionCreateDisk(resourcebudget.ExecutionSnapshot{StorageDriver: prepared.StorageDriver, ImageVirtualSizeBytes: &imageBytes})
		if err != nil {
			return err
		}
		existing := false
		ownRef := ""
		for _, v := range l.Documents {
			if v.State == "finished" {
				if v.Scope == g.scope && v.ID == identity.AllocationID {
					return errors.New("this exact document extraction is already finished")
				}
				continue
			}
			if v.Scope == g.scope && v.ID == identity.AllocationID {
				if v.Identity != identity || v.Actor != actor || v.Resources != resources || v.CreateDiskBytes != estimate.Bytes || v.ImageID != prepared.Spec.ImageID || !v.CreatedAt.Equal(request.CreatedAt) || !v.ComputeDeadline.Equal(request.ComputeDeadline) || !v.WholeDeadline.Equal(request.WholeDeadline) || (v.State != "reserved" && v.State != "created") {
					return errors.New("this exact document extraction is already claimed or its approved inputs changed")
				}
				existing, ownRef = true, v.Ref
				continue
			}
			out.Code, out.Message = "document_extraction_serial", "Another active or unconfirmed document extraction holds the serial slot."
			return nil
		}
		// ownRef is nonempty only for this identity/actor/resources-matched,
		// durable created lease. Its next RPC is Start of that immutable ref,
		// so revalidation must not reserve another pair of VFS image copies.
		demand := executionCapacityDemand{RequiredCreateDiskBytes: estimate.Bytes, CreateAcknowledged: ownRef != ""}
		if err:=g.addInputProofAdmissionCapacity(ctx,l.InputProofs,"","",&demand,policy.DiskReserveMiB);err!=nil{return err}
		observed := map[string]bool{}
		for _, c := range snap.Execution.Containers {
			if ownRef != "" && c.ID == ownRef && c.Kind == "document-extraction" {
				continue
			}
			memory, cpu, ok := executionWorkloadDemand(c, now)
			if !ok {
				out.Code, out.Message = "execution_limits_unknown", "A workload has no verified limits or fresh usage. No document extraction was started."
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
			out.Code, out.Message = "execution_legacy_demand_unknown", "An existing worker's allocation is not verified. Wait for its exact Docker observation or recovery before starting an extraction."
			return nil
		}
		if err := addDocumentCapacity(snap, l.Documents, g.scope, identity.AllocationID, &demand); err != nil {
			return err
		}
		if err := addCheckCapacity(snap, l.Checks, "", "", &demand); err != nil {
			return err
		}
		if err := addPreviewCapacity(snap, l.Previews, "", "", &demand); err != nil {
			return err
		}
		if err := addRuntimeImageCapacity(l.RuntimeImages, "", "", &demand); err != nil {
			return err
		}
		if err := g.addAssemblyAdmissionCapacity(ctx, snap, l.ImageAssemblies, "", "", &demand, policy.DiskReserveMiB); err != nil {
			return err
		}
		demand.HostLaunchBytes = windowsDockerHostLaunchBytes(snap, g.api.deps.Backend.Kind())
		if code, message := evaluateExecutionCapacity(snap, policy, resources, demand, now); code != "" {
			out.Code, out.Message = code, message
			return nil
		}
		if !existing {
			if executionLeaseCount(*l) >= 4096 {
				return errors.New("execution capacity history needs reconciliation")
			}
			l.Documents = append(l.Documents, documentExtractionLease{ID: identity.AllocationID, Scope: g.scope, Actor: actor, Identity: identity, ImageID: prepared.Spec.ImageID, Resources: resources, HostLaunchBytes: demand.HostLaunchBytes, CreateDiskBytes: estimate.Bytes, State: "reserved", CreatedAt: request.CreatedAt, UpdatedAt: now, ComputeDeadline: request.ComputeDeadline, WholeDeadline: request.WholeDeadline})
		}
		out.State, out.Code, out.Message = "admitted", "", "Document capacity reserved."
		return nil
	})
	return out, err
}

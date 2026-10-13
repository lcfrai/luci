package api

import (
	"context"
	"os"
	"sort"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
)

// observeCloudTaskCompletions rotates a bounded set of exact owned attempts.
// One slow guest cannot starve independent project completions. It makes no
// new VM or upstream Start request; the controller's durable one-shot markers
// govern every transport read and collector call after restart.
func (a *API) observeCloudTaskCompletions(ctx context.Context) error {
	cloud := a.deps.CloudTasks
	if cloud == nil {
		return nil
	}
	rows, _ := cloud.Runtime.Controller.Snapshot()
	active := make([]e2bbe.Record, 0, len(rows))
	for _, row := range rows {
		if row.Request.Identity.RunID != "" && row.Request.Identity.ProjectID != "" &&
			(row.State == e2bbe.Creating || row.State == e2bbe.Allocated || row.State == e2bbe.Cancelling || row.State == e2bbe.Unknown || row.State == e2bbe.Absent) {
			active = append(active, row)
		}
	}
	if len(active) == 0 {
		return nil
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].Request.Identity.OperationID < active[j].Request.Identity.OperationID
	})
	start := int((a.cloudObservationCursor.Add(1) - 1) % uint64(len(active)))
	limit := len(active)
	if limit > 4 {
		limit = 4
	}
	var first error
	for n := 0; n < limit && ctx.Err() == nil; n++ {
		row := active[(start+n)%len(active)]
		child, cancel := context.WithTimeout(ctx, 700*time.Millisecond)
		err := a.observeOneCloudTask(child, row)
		cancel()
		if err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (a *API) observeOneCloudTask(ctx context.Context, observed e2bbe.Record) error {
	cloud := a.deps.CloudTasks
	identity := observed.Request.Identity
	current, found := a.cloudRecord(identity.ProjectID, identity.RunID, identity.AttemptID, identity.OperationID)
	if !found || current.RequestDigest != observed.RequestDigest {
		return e2bbe.ErrConflict
	}
	run, err := a.deps.Product.GetProjectSwarmRun(identity.ProjectID, identity.RunID)
	if err != nil {
		return err
	}
	operationCancelled := false
	for _, op := range run.Operations {
		if op.ID == identity.AttemptID && op.ChildID == identity.OperationID && op.Status == "cancelled" {
			operationCancelled = true
			break
		}
	}
	if run.State == "cancelled" || operationCancelled || current.CancelRequested {
		for _, op := range run.Operations {
			if op.ID == identity.AttemptID && op.ChildID == identity.OperationID && op.Status == "cancelled" && current.State == e2bbe.Absent && !current.ResourcesHeld {
				return nil
			}
		}
		if current.State != e2bbe.Absent {
			current, err = cloud.Runtime.Retire(ctx, identity.OperationID, current.Revision)
			if err != nil {
				return err // exact ownership and charge remain visible for retry
			}
		}
		if current.State != e2bbe.Absent || current.ResourcesHeld {
			return e2bbe.ErrUnknown
		}
		_, err = a.deps.Product.RecordProjectSwarmMicroVMCancelled(ctx, identity.ProjectID, identity.RunID, identity.AttemptID, current)
		return err
	}
	for _, op := range run.Operations {
		if op.ID == identity.AttemptID && op.ChildID == identity.OperationID && op.Handoff != nil && current.State == e2bbe.Absent && !current.ResourcesHeld {
			return nil // a retained, exact handoff needs no repeated store read
		}
	}
	if current.State == e2bbe.Absent && current.Capture.ArchiveSHA256 == "" {
		return e2bbe.ErrUnknown // never describe uncaptured output as a reviewed result
	}
	if !current.LaunchRequested {
		if current.State == e2bbe.Creating || current.State == e2bbe.Unknown {
			_, err := cloud.Runtime.Controller.Reconcile(ctx, identity.OperationID)
			return err // exact provider readback only; scheduling owns any future Start
		}
		return nil // retain unconfirmed Create/transfer for explicit recovery
	}
	if current.State == e2bbe.Allocated && current.Capture.ArchiveSHA256 == "" {
		// Finish owns the native exit and one-shot collector readback. Its
		// staging is provisional until retirement and product source review.
		staging, err := os.MkdirTemp(cloud.StagingDir, "cloud-frozen-")
		if err != nil {
			return err
		}
		defer os.RemoveAll(staging)
		current, _, err = cloud.Runtime.Finish(ctx, identity.OperationID, cloud.attachToken(current.Request), staging)
		if err != nil {
			return err
		}
	}
	if current.Capture.ArchiveSHA256 == "" || e2bbe.ValidateNativeExitReceipt(current.NativeExit, current) != nil ||
		e2bbe.ValidateCaptureReceipt(current.Capture, current.LaunchKernel, current) != nil {
		return e2bbe.ErrUnknown
	}
	if current.State != e2bbe.Absent {
		var err error
		current, err = cloud.Runtime.Retire(ctx, identity.OperationID, current.Revision)
		if err != nil {
			return err // keep the exact owned VM and scheduler slot charged
		}
	}
	if current.State != e2bbe.Absent || current.ResourcesHeld {
		return e2bbe.ErrUnknown
	}
	run, err = a.deps.Product.GetProjectSwarmRun(identity.ProjectID, identity.RunID)
	if err != nil {
		return err
	}
	for _, op := range run.Operations {
		if op.ID == identity.AttemptID && op.ChildID == identity.OperationID && op.Handoff != nil {
			report, readErr := cloud.Runtime.Store.ReadStoredTaskReport(ctx, current.Request, current.Capture)
			if readErr != nil {
				return readErr
			}
			_, readErr = a.deps.Product.RecordProjectSwarmMicroVMHandoff(ctx, identity.ProjectID, identity.RunID, identity.AttemptID, current, report)
			return readErr
		}
	}
	staging, err := os.MkdirTemp(cloud.StagingDir, "cloud-reviewed-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staging)
	changes, err := cloud.Runtime.Store.MaterializeStoredReturnedSource(ctx, current.Request, current.Capture, staging)
	if err != nil {
		return err
	}
	if _, err = a.deps.Product.RecordProjectSwarmMicroVMExit(ctx, identity.ProjectID, identity.RunID, identity.AttemptID, current); err != nil {
		return err
	}
	if len(changes.Changes) > 0 {
		if _, err = a.deps.Product.ImportProjectSwarmMicroVMCandidate(ctx, identity.ProjectID, identity.RunID, identity.AttemptID, current, changes, staging); err != nil {
			return err // immutable artifacts remain for explicit source recovery
		}
	}
	report, err := cloud.Runtime.Store.ReadStoredTaskReport(ctx, current.Request, current.Capture)
	if err != nil {
		return err
	}
	_, err = a.deps.Product.RecordProjectSwarmMicroVMHandoff(ctx, identity.ProjectID, identity.RunID, identity.AttemptID, current, report)
	return err
}

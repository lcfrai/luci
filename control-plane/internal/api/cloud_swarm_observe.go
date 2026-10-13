package api

import (
	"context"
	"errors"
	"os"
	"sort"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func cloudProductTaskCancelled(run product.ProjectRun, record e2bbe.Record) bool {
	if run.State == "cancelled" {
		return true
	}
	identity := record.Request.Identity
	for _, op := range run.Operations {
		if op.ID == identity.AttemptID && op.ChildID == identity.OperationID && op.Status == "cancelled" {
			return true
		}
	}
	return false
}

func (a *API) cloudOwnedStopRequested(identity e2bbe.Identity) bool {
	queue, err := a.deps.Product.ProjectSwarmQueueSnapshot(identity.ProjectID)
	if err != nil {
		return false
	}
	for _, item := range queue.Items {
		if item.ProjectID == identity.ProjectID && item.RunID == identity.RunID && item.TaskID == identity.TaskID &&
			item.OperationID == identity.AttemptID && item.ChildID == identity.OperationID && (item.ReasonCode == "context_superseded_stop_pending" || item.ReasonCode == "operator_stop_pending") {
			return true
		}
	}
	return false
}

func cloudObservationCandidates(rows []e2bbe.Record) []e2bbe.Record {
	active := make([]e2bbe.Record, 0, len(rows))
	for _, row := range rows {
		if row.Request.Identity.RunID != "" && row.Request.Identity.ProjectID != "" &&
			(row.State == e2bbe.Creating || row.State == e2bbe.Allocated || row.State == e2bbe.Cancelling || row.State == e2bbe.Unknown ||
				row.State == e2bbe.Absent && row.ProductOutcome == "") {
			active = append(active, row)
		}
	}
	return active
}

func (a *API) reconcileRestoredCloud(ctx context.Context) error {
	child, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	_, err := a.deps.CloudTasks.Runtime.Controller.ReconcileRestored(child, 4)
	return err
}

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
	active := cloudObservationCandidates(rows)
	if len(active) == 0 {
		return a.reconcileRestoredCloud(ctx)
	}
	sort.Slice(active, func(i, j int) bool {
		return active[i].Request.Identity.OperationID < active[j].Request.Identity.OperationID
	})
	start := int((a.cloudObservationCursor.Add(1) - 1) % uint64(len(active)))
	limit := len(active)
	if limit > 4 {
		limit = 4
	}
	selected := make([]e2bbe.Record, 0, limit)
	for n := 0; n < limit; n++ {
		selected = append(selected, active[(start+n)%len(active)])
	}
	observedErr := runCloudObservationBatch(ctx, selected, a.observeOneCloudTask)
	// Recovery must advance even when no new queue entry asks for admission.
	// Completion observation runs first so one slow old provider identity does
	// not delay independent finished-task results on this tick.
	return errors.Join(observedErr, a.reconcileRestoredCloud(ctx))
}

func runCloudObservationBatch(ctx context.Context, selected []e2bbe.Record, observe func(context.Context, e2bbe.Record) error) error {
	if len(selected) > 4 {
		return e2bbe.ErrCapacity
	}
	results := make(chan error, len(selected))
	var workers sync.WaitGroup
	for _, row := range selected {
		if ctx.Err() != nil {
			break
		}
		workers.Add(1)
		go func() {
			defer workers.Done()
			child, cancel := context.WithTimeout(ctx, 25*time.Second)
			defer cancel()
			results <- observe(child, row)
		}()
	}
	workers.Wait()
	close(results)
	var failures []error
	for err := range results {
		if err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
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
	// Controller CancelRequested also records normal VM retirement after a
	// finished capture. Only the authoritative product run/operation can
	// classify this task as cancelled; otherwise a lost handoff response
	// would turn valid retained output into a false cancellation.
	supersededActive := false
	if current.Capture.ArchiveSHA256 == "" && a.cloudOwnedStopRequested(identity) {
		for _, op := range run.Operations {
			if op.ID == identity.AttemptID && op.ChildID == identity.OperationID &&
				(op.Status == "running" || op.Status == "dispatching" || op.Status == "reserved" || op.Status == "unknown") {
				supersededActive = true
				break
			}
		}
	}
	if cloudProductTaskCancelled(run, current) || supersededActive {
		for _, op := range run.Operations {
			if op.ID == identity.AttemptID && op.ChildID == identity.OperationID && op.Status == "cancelled" && current.State == e2bbe.Absent && !current.ResourcesHeld {
				_, err = cloud.Runtime.Controller.AcknowledgeProductOutcome(ctx, identity.OperationID, current.Revision, "cancelled")
				return err
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
		if _, err = a.deps.Product.RecordProjectSwarmMicroVMCancelled(ctx, identity.ProjectID, identity.RunID, identity.AttemptID, current); err != nil {
			return err
		}
		_, err = cloud.Runtime.Controller.AcknowledgeProductOutcome(ctx, identity.OperationID, current.Revision, "cancelled")
		return err
	}
	for _, op := range run.Operations {
		if op.ID == identity.AttemptID && op.ChildID == identity.OperationID && op.Handoff != nil && current.State == e2bbe.Absent && !current.ResourcesHeld {
			_, err = cloud.Runtime.Controller.AcknowledgeProductOutcome(ctx, identity.OperationID, current.Revision, "handoff")
			return err // a retained, exact handoff needs no repeated store read
		}
	}
	if current.State == e2bbe.Absent && current.Capture.ArchiveSHA256 == "" {
		if _, err = a.deps.Product.RecordProjectSwarmMicroVMNoResult(ctx, identity.ProjectID, identity.RunID, identity.AttemptID, current); err != nil {
			return err
		}
		_, err = cloud.Runtime.Controller.AcknowledgeProductOutcome(ctx, identity.OperationID, current.Revision, "no-result")
		return err // no native success or returned source is inferred from absence
	}
	if !current.LaunchRequested {
		if current.State == e2bbe.Creating || current.State == e2bbe.Unknown {
			_, err := cloud.Runtime.Controller.Reconcile(ctx, identity.OperationID)
			return err // exact provider readback only; scheduling owns any future Start
		}
		return nil // retain unconfirmed Create/transfer for explicit recovery
	}
	if current.State == e2bbe.Allocated && current.ProcessPID == 0 {
		// A lost envd Start ACK is read back by exact operation tag and pinned
		// bootstrap identity. It must never lead to a second Start request.
		guest, guestErr := cloud.Runtime.Controller.Guest(ctx, identity.OperationID)
		if guestErr != nil {
			return guestErr
		}
		current, err = cloud.Runtime.Controller.ObserveStartPID(ctx, identity.OperationID, guest)
		guest.Close()
		return err // the next pass verifies the protected launch kernel
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
			// Source review and current authority are separate from ownership.
			// Once the exact native exit and off-host archive are durable, a
			// refusal must not strand a billable VM. The captured result remains
			// unaccepted and available for later authorized recovery.
			if current.Capture.ArchiveSHA256 != "" {
				if settled, settleErr := cloud.Runtime.RetireCaptured(ctx, current); settleErr != nil {
					return settleErr
				} else if settled.State != e2bbe.Absent || settled.ResourcesHeld {
					return e2bbe.ErrUnknown
				}
			}
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
			if _, readErr = a.deps.Product.RecordProjectSwarmMicroVMHandoff(ctx, identity.ProjectID, identity.RunID, identity.AttemptID, current, report); readErr != nil {
				return readErr
			}
			_, readErr = cloud.Runtime.Controller.AcknowledgeProductOutcome(ctx, identity.OperationID, current.Revision, "handoff")
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
	if _, err = a.deps.Product.RecordProjectSwarmMicroVMHandoff(ctx, identity.ProjectID, identity.RunID, identity.AttemptID, current, report); err != nil {
		return err
	}
	_, err = cloud.Runtime.Controller.AcknowledgeProductOutcome(ctx, identity.OperationID, current.Revision, "handoff")
	return err
}

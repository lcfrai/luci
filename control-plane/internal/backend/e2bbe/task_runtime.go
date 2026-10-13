package e2bbe

import (
	"context"
	"io"
)

// TaskRuntime joins the owned VM, durable transfer, upstream harness and
// returned-candidate boundaries for a single task attempt. The product still
// owns reviewed source import, independent checks, integration and delivery.
// Each method can be re-entered after controller restart without repeating a
// Create, input POST, upstream Start or collector Start whose outcome is unsure.
type TaskRuntime struct {
	Controller *Controller
	Store      *DirectoryStore
}

// Start allocates one VM under current authority/budget, stages its exact
// source/context bundle off-host, then starts the selected upstream harness.
// An error after allocation returns the charged record for reviewed recovery.
func (t TaskRuntime) Start(ctx context.Context, request Request, input io.ReadSeeker, launch LaunchSpec) (Record, error) {
	if t.Controller == nil || t.Store == nil {
		return Record{}, ErrUnsupported
	}
	r, err := t.Controller.Allocate(ctx, request)
	if err != nil {
		return r, err
	}
	if r.State != Allocated {
		return r, ErrUnknown
	}
	if r.LaunchRequested {
		if r.LaunchDigest != launchDigest(r, launch) {
			return r, ErrConflict
		}
		if err = t.Controller.config.Authorize(ctx, r.Request, "start"); err != nil {
			return r, err
		}
		if r.ProcessPID == 0 {
			return r, ErrUnknown
		}
		return r, nil
	}
	guest, err := t.Controller.Guest(ctx, request.Identity.OperationID)
	if err != nil {
		return r, err
	}
	defer guest.Close()
	r, err = t.Controller.StageGuestInput(ctx, request.Identity.OperationID, guest, t.Store, input)
	if err != nil {
		return r, err
	}
	return t.Controller.StartGuest(ctx, request.Identity.OperationID, r.Revision, launch, guest)
}

// Finish observes native outcome, captures the frozen task source once,
// verifies exact input/output binding and materializes a private candidate.
// Successful staging does not mean reviewed integration or provider billing
// settlement. The VM remains owned/charged until Retire confirms absence.
func (t TaskRuntime) Finish(ctx context.Context, operationID, attachToken, staging string) (Record, SourceChanges, error) {
	if t.Controller == nil || t.Store == nil {
		return Record{}, SourceChanges{}, ErrUnsupported
	}
	guest, err := t.Controller.Guest(ctx, operationID)
	if err != nil {
		return Record{}, SourceChanges{}, err
	}
	defer guest.Close()
	r, err := t.Controller.ObserveLaunchKernel(ctx, operationID, guest)
	if err != nil {
		return r, SourceChanges{}, err
	}
	r, err = t.Controller.ObserveNativeExit(ctx, operationID, guest, attachToken)
	if err != nil {
		return r, SourceChanges{}, err
	}
	r, err = t.Controller.CollectGuestOutput(ctx, operationID, guest, t.Store)
	if err != nil {
		return r, SourceChanges{}, err
	}
	_, err = t.Controller.ReviewGuestOutput(ctx, operationID, t.Store)
	if err != nil {
		return r, SourceChanges{}, err
	}
	staged, err := t.Store.MaterializeStoredReturnedSource(ctx, r.Request, r.Capture, staging)
	if err != nil {
		return r, SourceChanges{}, err
	}
	current, readErr := t.Controller.record(operationID)
	if readErr != nil || current.Revision != r.Revision || current.RequestDigest != r.RequestDigest || current.Capture != r.Capture {
		return r, SourceChanges{}, ErrConflict
	}
	if err = t.Controller.config.Authorize(ctx, current.Request, "review-output"); err != nil {
		return current, SourceChanges{}, err
	}
	return r, staged, nil
}

// Retire removes only this exact VM. A missing deletion/readback retains its
// charged allocation; an absent VM still does not certify provider billing.
func (t TaskRuntime) Retire(ctx context.Context, operationID string, revision int) (Record, error) {
	if t.Controller == nil {
		return Record{}, ErrUnsupported
	}
	return t.Controller.Cancel(ctx, operationID, revision)
}

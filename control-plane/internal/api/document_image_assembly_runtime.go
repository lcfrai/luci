package api

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"io"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
)

// Private optional coordinator only. There is no registered route or autostart.
// Shared hooks must be reviewed adapters to the existing authority/ledger; nil
// hooks never create a replacement store or silently bypass normal admission.
func completeAssemblyHooks(h documentImageAssemblyHooks) bool {
	return h.Authorize != nil && h.Admit != nil && h.Lock != nil && h.Transition != nil && h.HoldFence != nil && h.RecordFence != nil && h.Finish != nil && h.Headroom != nil && h.CaptureSink != nil && h.Publish != nil
}

type assemblyReadResult struct {
	capture  backend.DocumentImageAssemblyCapture
	err      error
	closeErr error
}

func assemblyIntentID() (string, error) {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(id[:]), nil
}
func assemblyRequestCurrent(ctx context.Context, h documentImageAssemblyHooks, req documentImageAssemblyRequest, identity backend.DocumentImageAssemblyIdentity) error {
	if req.ValidateCurrent == nil || !validAssemblyInputClosureRef(req.InputClosureRef) || h.retireOnly {
		return backend.ErrUnsupported
	}
	if err := h.Authorize(ctx, req.Actor, identity); err != nil {
		return err
	}
	return req.ValidateCurrent(ctx)
}
func validAssemblyRequest(req documentImageAssemblyRequest) bool {
	return backend.ValidateDocumentImageAssemblySpec(req.Spec) && validAssemblyInputClosureRef(req.InputClosureRef) && req.ValidateCurrent != nil && !req.CreatedAt.IsZero() && req.ComputeDeadline.Sub(req.CreatedAt) == backend.DocumentImageAssemblyComputeBudget && req.WholeDeadline.Sub(req.ComputeDeadline) == backend.DocumentImageAssemblyCleanupBudget && !req.CreatedAt.After(time.Now().UTC()) && time.Now().Before(req.ComputeDeadline)
}

func runDocumentImageAssembly(ctx context.Context, executor backend.DocumentImageAssemblyExecutor, h documentImageAssemblyHooks, req documentImageAssemblyRequest) (out documentImageAssemblyOutcome, retErr error) {
	out = documentImageAssemblyOutcome{Identity: req.Spec.Identity, InputClosureRef: req.InputClosureRef, Stage: "admission", State: "not_started", ReaderSettled: true, CleanupConfirmed: true, WatchdogSettled: true}
	if executor == nil || h.retireOnly || !completeAssemblyHooks(h) || !validAssemblyRequest(req) {
		return out, backend.ErrUnsupported
	}
	compute, cancel := context.WithDeadline(ctx, req.ComputeDeadline)
	defer cancel()
	if err := assemblyRequestCurrent(compute, h, req, req.Spec.Identity); err != nil {
		return out, err
	}
	prepared, err := executor.PrepareDocumentImageAssembly(compute, req.Spec)
	if err != nil {
		return out, err
	}
	out.Identity = prepared.Spec.Identity
	if !backend.ValidateDocumentImageAssemblyIdentity(out.Identity, true) {
		return out, backend.ErrLaunchFenceOwnership
	}
	if err = assemblyRequestCurrent(compute, h, req, out.Identity); err != nil {
		return out, err
	}
	if err = h.Headroom(compute, out.Identity.EngineID, 8<<30); err != nil {
		return out, err
	}
	lease, err := h.Admit(compute, prepared, req)
	if err != nil {
		return out, err
	}
	out.ResourcesHeld = true
	if !validDocumentImageAssemblyLease(lease) || lease.Identity != out.Identity || lease.InputClosureRef != req.InputClosureRef || lease.Actor != req.Actor || lease.State != "reserved" || lease.CreatedAt != req.CreatedAt || lease.ComputeDeadline != req.ComputeDeadline || lease.WholeDeadline != req.WholeDeadline {
		return out, backend.ErrLaunchFenceOwnership
	}
	unlock, err := h.Lock(compute, out.Identity)
	if err != nil {
		return out, err
	}
	var watchdog *assemblyStopWatchdog
	var readerDone chan struct{}
	var closeDone chan struct{}
	defer func() {
		// Returning is not proof that the independent stop/reader has settled.
		cancel()
		cleanupCtx, stop := context.WithDeadline(context.Background(), req.WholeDeadline)
		defer stop()
		watchdog.wait(cleanupCtx, &out)
		// An unsettled/failed exact retirement has no later positive fact that
		// could safely release this lock. Process recovery must re-prove it.
		if !out.CleanupConfirmed {
			return
		}
		settled := func(done <-chan struct{}) bool {
			if done == nil {
				return true
			}
			select {
			case <-done:
				return true
			default:
				return false
			}
		}
		if out.WatchdogSettled && settled(readerDone) && settled(closeDone) {
			unlock()
			return
		}
		// Durable capacity remains held; late settlement releases only the OS
		// lock, never the lease, publishes success, or permits an automatic retry.
		go func() {
			if watchdog != nil {
				<-watchdog.done
			}
			if readerDone != nil {
				<-readerDone
			}
			if closeDone != nil {
				<-closeDone
			}
			unlock()
		}()
	}()
	state := "reserved"
	transition := func(next, ref string) error {
		watchdog.beat()
		if err := compute.Err(); err != nil {
			return err
		}
		if err := h.Transition(compute, out.Identity, state, next, ref); err != nil {
			return err
		}
		if err := compute.Err(); err != nil {
			return err
		}
		watchdog.beat()
		state = next
		out.Stage = next
		return nil
	}
	var stream backend.DocumentImageAssemblyStream
	var read assemblyReadResult
	var sink io.WriteCloser
	sinkOwned := false
	createAttempted, startAttempted := false, false
	// Emergency retirement is independent of capture/receipt I/O. A blocked
	// writer/reader can retain the lock and lease, but cannot defer this stop.
	cleanup := func(reason string, forceFence bool) error {
		cancel()
		out.State = "incomplete_resources_held"
		out.ReasonCode = reason
		out.ResourcesHeld = true
		cleanupCtx, stop := context.WithDeadline(context.Background(), req.WholeDeadline)
		defer stop()
		var retireErr error
		if watchdog != nil {
			watchdog.wait(cleanupCtx, &out)
			if out.WatchdogSettled && !out.WatchdogTriggered {
				retireErr = executor.RetireDocumentImageAssembly(cleanupCtx, out.Identity, out.Ref)
				out.CleanupConfirmed = retireErr == nil
			} else {
				out.CleanupConfirmed = out.WatchdogSettled && out.WatchdogRetirementConfirmed
			}
			if !out.CleanupConfirmed {
				retireErr = backend.ErrUnavailable
			}
		} else if out.Ref != "" {
			retireErr = executor.RetireDocumentImageAssembly(cleanupCtx, out.Identity, out.Ref)
			out.CleanupConfirmed = retireErr == nil
		} else {
			out.CleanupConfirmed = !createAttempted
		}
		// Unknown Create/Start ACKs require a durable intent and a REAL fence
		// Create ACK. Readback of a late object is never a replacement ACK.
		if createAttempted && (out.Ref == "" || forceFence) {
			intent, e := assemblyIntentID()
			if e == nil {
				fence := backend.DocumentImageAssemblyFenceRequest{Identity: out.Identity, IntentID: intent, ExpectedRef: out.Ref}
				e = h.HoldFence(cleanupCtx, fence)
				if e == nil {
					var proof backend.DocumentImageAssemblyFenceProof
					proof, e = executor.FenceDocumentImageAssembly(cleanupCtx, fence)
					if e == nil {
						e = executor.VerifyDocumentImageAssemblyFence(cleanupCtx, fence, proof)
					}
					if e == nil {
						e = h.RecordFence(cleanupCtx, fence, proof)
					}
				}
			}
			out.CleanupConfirmed = e == nil
			if e != nil {
				retireErr = e
			}
		}
		var closed chan error
		if stream != nil {
			closed = make(chan error, 1)
			closeDone = make(chan struct{})
			go func() { defer close(closeDone); closed <- stream.Close() }()
		}
		if sinkOwned {
			sinkOwned = false
			readerDone = make(chan struct{})
			out.ReaderSettled = false
			go func() { defer close(readerDone); read.closeErr = sink.Close() }()
		}
		if readerDone != nil {
			select {
			case <-readerDone:
				out.ReaderSettled = true
				if read.closeErr != nil {
					out.CleanupConfirmed = false
				}
			case <-cleanupCtx.Done():
				out.ReaderSettled = false
				// No replacement/automatic lease release. Keep the exact OS lock
				// until the original reader settles or this process dies.
			}
		}
		if closed != nil {
			select {
			case closeErr := <-closed:
				if closeErr != nil {
					out.CleanupConfirmed = false
				}
			case <-cleanupCtx.Done():
				out.CleanupConfirmed = false
			}
		}
		watchdog.observe(&out)
		if watchdog != nil && (!out.WatchdogSettled || (out.WatchdogTriggered && !out.WatchdogRetirementConfirmed)) {
			out.CleanupConfirmed = false
		}
		if out.CleanupConfirmed && out.ReaderSettled && out.WatchdogSettled {
			out.State = "failed_owned_resources_retired"
			if e := h.Publish(cleanupCtx, out); e != nil {
				return e
			}
			finished := out
			finished.ResourcesHeld = false
			if e := h.Finish(cleanupCtx, out.Identity, finished); e != nil {
				return e
			}
			out = finished
			return retireErr
		}
		_ = h.Publish(cleanupCtx, out)
		if retireErr != nil {
			return retireErr
		}
		return backend.ErrUnavailable
	}
	fail := func(reason string, cause error, forceFence bool) (documentImageAssemblyOutcome, error) {
		_ = cleanup(reason, forceFence)
		return out, cause
	}
	// Allocate the exclusive, already disk-reserved artifact before any daemon
	// effect. The adapter must enforce 2GiB before each write, not at final close.
	sink, err = h.CaptureSink(compute, out.Identity)
	sinkOwned = sink != nil
	if err != nil {
		return fail("capture_sink_unavailable", err, false)
	}
	if sink == nil {
		return fail("capture_sink_missing", backend.ErrUnsupported, false)
	}
	if err = assemblyRequestCurrent(compute, h, req, out.Identity); err != nil {
		return fail("authority_changed", err, false)
	}
	if err = h.Headroom(compute, out.Identity.EngineID, 8<<30); err != nil {
		return fail("admission_changed", err, false)
	}
	if err = transition("creating", ""); err != nil {
		return fail("create_intent_unpublished", err, false)
	}
	createAttempted = true
	out.CleanupConfirmed = false
	out.Ref, err = executor.CreateDocumentImageAssembly(compute, prepared)
	if err != nil || !checkHex(out.Ref, 32) {
		return fail("create_ack_unknown", backend.ErrUnavailable, true)
	}
	// Arm before any further hook and before Start is possible. This lane owns
	// only the positively acknowledged allocation, independently of foreground I/O.
	watchdog = startAssemblyStopWatchdog(compute, cancel, executor, out.Identity, out.Ref, req.WholeDeadline)
	out.WatchdogSettled = false
	if err = transition("created", out.Ref); err != nil {
		return fail("created_receipt_unpublished", err, false)
	}
	stream, err = executor.AttachDocumentImageAssembly(compute, out.Identity, out.Ref)
	if err != nil || stream == nil {
		return fail("prestart_capture_unavailable", backend.ErrUnavailable, false)
	}
	readerDone = make(chan struct{})
	out.ReaderSettled = false
	sinkOwned = false
	go func() {
		defer close(readerDone)
		read.capture, read.err = stream.Capture(compute, sink)
		read.closeErr = sink.Close()
	}()
	watchdog.beat()
	if err = assemblyRequestCurrent(compute, h, req, out.Identity); err != nil {
		return fail("authority_changed", err, false)
	}
	if err = h.Headroom(compute, out.Identity.EngineID, 8<<30); err != nil {
		return fail("admission_changed", err, false)
	}
	select {
	case <-readerDone:
		return fail("capture_ended_before_start", backend.ErrUnavailable, false)
	default:
	}
	if err = transition("starting", out.Ref); err != nil {
		return fail("start_intent_unpublished", err, false)
	}
	startAttempted = true
	if err = executor.StartDocumentImageAssembly(compute, out.Identity, out.Ref); err != nil {
		return fail("start_ack_unknown", err, true)
	}
	if err = transition("running", out.Ref); err != nil {
		return fail("running_receipt_unpublished", err, false)
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		watchdog.beat()
		if err = compute.Err(); err != nil {
			return fail("compute_interrupted", err, false)
		}
		if err = h.Headroom(compute, out.Identity.EngineID, 4<<30); err != nil {
			return fail("resource_brake", err, false)
		}
		if err = compute.Err(); err != nil {
			return fail("compute_interrupted", err, false)
		}
		watchdog.beat()
		observation, e := executor.InspectDocumentImageAssembly(compute, out.Identity, out.Ref)
		if e != nil {
			return fail("observation_unknown", e, false)
		}
		switch observation.State {
		case "running":
		case "exited":
			out.ExitCode, out.OOMKilled = observation.ExitCode, observation.OOMKilled
			if !startAttempted || observation.ExitCode == nil || *observation.ExitCode != 0 || observation.OOMKilled {
				return fail("assembler_exit_failed", backend.ErrUnavailable, false)
			}
			select {
			case <-readerDone:
				out.ReaderSettled = true
			case <-compute.Done():
				return fail("capture_unsettled", backend.ErrUnavailable, false)
			}
			if read.err != nil || read.closeErr != nil || !checkHex(read.capture.Digest, 32) || read.capture.Bytes < 1 || read.capture.Bytes > backend.DocumentImageAssemblyOutputMaxBytes {
				return fail("capture_incomplete", backend.ErrUnavailable, false)
			}
			// An emergency observation can never become an accepted assembly.
			if err = compute.Err(); err != nil {
				return fail("compute_interrupted", err, false)
			}
			// Positive exit and settled capture mean no parser is running. Settle
			// the emergency lane before ordinary publication/retirement/Finish.
			watchdog.stop()
			settleCtx, settleCancel := context.WithDeadline(context.Background(), req.WholeDeadline)
			settled := watchdog.wait(settleCtx, &out)
			settleCancel()
			if !settled || out.WatchdogTriggered {
				return fail("watchdog_interrupted", backend.ErrUnavailable, false)
			}
			out.Capture = read.capture
			out.State = "captured_exit_zero_not_registered"
			// For ordinary exited success preserve the complete artifact before
			// retirement. A failed publisher leaves the exited exact CID held.
			cleanupCtx, stop := context.WithDeadline(context.Background(), req.WholeDeadline)
			defer stop()
			if err = h.Publish(cleanupCtx, out); err != nil {
				return out, err
			}
			if err = executor.RetireDocumentImageAssembly(cleanupCtx, out.Identity, out.Ref); err != nil {
				return out, err
			}
			out.CleanupConfirmed = true
			out.State = "assembled_owned_resources_retired_not_registered"
			if err = h.Publish(cleanupCtx, out); err != nil {
				return out, err
			}
			finished := out
			finished.ResourcesHeld = false
			if err = h.Finish(cleanupCtx, out.Identity, finished); err != nil {
				return out, err
			}
			return finished, nil
		default:
			return fail("unexpected_runtime_state", backend.ErrUnavailable, false)
		}
		select {
		case <-compute.Done():
			return fail("compute_interrupted", compute.Err(), false)
		case <-readerDone:
			if read.err != nil {
				return fail("capture_failed", read.err, false)
			}
			// EOF is not completion: continue observing the exact exit status.
			select {
			case <-ticker.C:
			case <-compute.Done():
				return fail("compute_interrupted", compute.Err(), false)
			}
		case <-ticker.C:
		}
	}
}

// Recovery is retire/fence-only. There is no attachment, start, import or retry.
// A lost process cannot recreate capture; callers must expose it unavailable.
func recoverDocumentImageAssembly(ctx context.Context, executor backend.DocumentImageAssemblyExecutor, h documentImageAssemblyHooks, lease documentImageAssemblyLease, req backend.DocumentImageAssemblyFenceRequest, proof *backend.DocumentImageAssemblyFenceProof) (documentImageAssemblyOutcome, error) {
	out := documentImageAssemblyOutcome{Identity: lease.Identity, InputClosureRef: lease.InputClosureRef, Ref: lease.Ref, Stage: "recovery", State: "capture_unavailable_resources_held", ResourcesHeld: true, ReaderSettled: true, WatchdogSettled: true, RetainedCapture: lease.Capture}
	if executor == nil || !h.retireOnly || !completeAssemblyHooks(h) || !validDocumentImageAssemblyLease(lease) || lease.State == "finished" || req.Identity != lease.Identity || req.ExpectedRef != lease.Ref || !checkHex(req.IntentID, 16) {
		return out, backend.ErrUnsupported
	}
	// Recovery authorization cannot turn a revoked grant into new execution.
	if err := h.Authorize(ctx, lease.Actor, lease.Identity); err != nil {
		return out, err
	}
	unlock, err := h.Lock(ctx, lease.Identity)
	if err != nil {
		return out, err
	}
	defer unlock()
	// Recovery gets its own separately admitted cleanup window, never a compute
	// extension. The old expired lease remains charged throughout this window.
	cleanup, cancel := context.WithTimeout(ctx, backend.DocumentImageAssemblyCleanupBudget)
	defer cancel()
	if lease.Ref != "" {
		_ = executor.RetireDocumentImageAssembly(cleanup, lease.Identity, lease.Ref)
	}
	var actual backend.DocumentImageAssemblyFenceProof
	if proof != nil {
		actual = *proof
		err = executor.VerifyDocumentImageAssemblyFence(cleanup, req, actual)
	} else {
		if err = h.HoldFence(cleanup, req); err == nil {
			actual, err = executor.FenceDocumentImageAssembly(cleanup, req)
		}
		if err == nil {
			err = executor.VerifyDocumentImageAssemblyFence(cleanup, req, actual)
		}
		if err == nil {
			err = h.RecordFence(cleanup, req, actual)
		}
	}
	if err != nil {
		_ = h.Publish(cleanup, out)
		return out, err
	}
	out.CleanupConfirmed = true
	out.State = "interrupted_capture_unavailable_owned_resources_retired"
	if err = h.Publish(cleanup, out); err != nil {
		return out, err
	}
	finished := out
	finished.ResourcesHeld = false
	if err = h.Finish(cleanup, lease.Identity, finished); err != nil {
		return out, err
	}
	return finished, nil
}

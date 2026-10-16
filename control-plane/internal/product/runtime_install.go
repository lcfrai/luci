package product

import (
	"context"
	"errors"
	"net/http"
	"path/filepath"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/runtimeinstall"
)

// All fields are protected by Server.mu. Accepted work belongs to the server,
// not the observing request. Closing seals publication before returning.
type runtimeInstallState struct {
	proofIO sync.Mutex // persistence only; cancellation/retirement never waits for it
	proof *runtimeInputProofOwnership
	runtime runtimeinstall.Runtime
	cancel  context.CancelFunc
	done    chan struct{}
	closed  bool
	sealed  bool
}

func (s *Server) SetRuntimeInstaller(runtime runtimeinstall.Runtime) {
	s.mu.Lock()
	s.runtimeInstall.runtime = runtime
	s.mu.Unlock()
}

func runtimeInstallActive(status string) bool {
	return status == "installing" || status == "cancelling" || status == "reconciling"
}

func runtimeInstallRetained(job RuntimeJob) bool {
	return job.Execution != nil && job.Execution.ResourcesHeld || job.InputProof!=nil && job.InputProof.ResourcesHeld
}

// A lost image-publication ACK can be uncharged and still require exact
// reconciliation. Keep that identity instead of overwriting it with a new job.
func runtimeInstallUnresolved(job RuntimeJob) bool {
	return runtimeInstallRetained(job) || job.Status == "unknown" || job.InputProof!=nil && job.InputProof.PublicationUnknown
}

func (s *Server) installRuntime(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode string `json:"mode,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if req.Mode != "" && req.Mode != "packaged" && req.Mode != "compressed-pull" && req.Mode != "advanced-build" {
		problem(w, 400, "runtime_install_mode_invalid", "Select the compressed packaged runtime or an explicit diagnostic install mode.")
		return
	}
	mode := "compressed-pull"
	if req.Mode != "" {
		mode = req.Mode
	}
	s.mu.RLock()
	active, closed, runtime := s.runtimeJob, s.runtimeInstall.closed, s.runtimeInstall.runtime
	s.mu.RUnlock()
	if closed {
		problem(w, 503, "runtime_closing", "Luci is closing; runtime preparation cannot start.")
		return
	}
	if runtimeInstallActive(active.Status) || runtimeInstallUnresolved(active) {
		// Cached re-observation must not create work or cancel an existing build
		// when a new storage observation is unavailable.
		writeJSON(w, 202, active)
		return
	}
	if profile,selected,err:=s.selectedRuntimeInputProofProfile();selected {
		if err!=nil {problem(w,503,"runtime_input_proof_unavailable",err.Error());return}
		s.installRuntimeInputProof(w,r,profile);return
	}
	dir, err := s.runtimeSource()
	if err != nil {
		problem(w, 409, "runtime_source_missing", err.Error())
		return
	}
	if runtime.CaptureActor == nil || runtime.Run == nil {
		problem(w, 503, "runtime_admission_unavailable", "This service cannot safely prepare the runtime. Restart a complete Luci installation.")
		return
	}
	actor, err := runtime.CaptureActor(r.Context())
	if err != nil {
		problem(w, 403, "runtime_forbidden", err.Error())
		return
	}
	if err = s.runtimeStoragePreflight(); err != nil {
		problem(w, 409, "runtime_"+runtimeStorageErrorCode(err), err.Error())
		return
	}
	s.mu.Lock()
	if s.runtimeInstall.closed {
		s.mu.Unlock()
		problem(w, 503, "runtime_closing", "Luci is closing; runtime preparation cannot start.")
		return
	}
	if s.runtimeInstall.done != nil || runtimeInstallActive(s.runtimeJob.Status) || runtimeInstallUnresolved(s.runtimeJob) {
		job := s.runtimeJob
		s.mu.Unlock()
		writeJSON(w, 202, job)
		return
	}
	now := time.Now().UTC()
	job := RuntimeJob{ID: newID("runtime-"), Status: "installing", Image: s.sandboxImage, Message: "Checking shared capacity before preparing the bundled runtime.", CreatedAt: now, UpdatedAt: now}
	job.Execution = &runtimeinstall.Outcome{Identity: runtimeinstall.Identity{JobID: job.ID, ImageTag: job.Image}, Phase: "queued", Message: job.Message}
	if err = atomicJSON(filepath.Join(s.stateDir, "runtime-install.json"), job); err != nil {
		s.mu.Unlock()
		problem(w, 500, "runtime_save_failed", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), runtimeBuildTimeout)
	done := make(chan struct{})
	s.runtimeJob = job
	s.runtimeInstall.cancel, s.runtimeInstall.done = cancel, done
	s.mu.Unlock()
	go s.runRuntimeInstall(ctx, cancel, done, job.ID, func(publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		if err := ctx.Err(); err != nil {
			return runtimeinstall.Outcome{}, err
		}
		if err := s.runtimeStoragePreflight(); err != nil {
			return runtimeinstall.Outcome{}, err
		}
		return runtime.Run(ctx, runtimeinstall.Request{Identity: job.Execution.Identity, SourceDirectory: dir, Actor: actor, Mode: mode}, publish)
	})
	writeJSON(w, 202, job)
}

func (s *Server) publishRuntimeInstall(id string, outcome runtimeinstall.Outcome) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runtimeInstall.sealed || s.runtimeJob.ID != id || s.runtimeJob.Execution == nil || !runtimeinstall.SameOperation(s.runtimeJob.Execution.Identity, outcome.Identity) {
		return errors.New("runtime installation publication no longer owns this job")
	}
	if outcome.ResourcesHeld && outcome.CleanupConfirmed || outcome.Phase == "ready" && (outcome.ResourcesHeld || !outcome.CleanupConfirmed || outcome.ImageID == "") {
		return errors.New("runtime installation outcome has no confirmed settlement")
	}
	job := s.runtimeJob
	job.Execution = &outcome
	job.Message, job.UpdatedAt = outcome.Message, time.Now().UTC()
	switch outcome.Phase {
	case "ready", "failed", "blocked", "cancelled", "unknown":
		job.Status = outcome.Phase
	default:
		job.Status = "installing"
	}
	if job.CancelRequested && runtimeInstallActive(job.Status) {
		job.Status = "cancelling"
	}
	if err := atomicJSON(filepath.Join(s.stateDir, "runtime-install.json"), job); err != nil {
		// Keep the newest identity in memory; the coordinator must retain its
		// durable lease when its pre-mutation publication fails.
		job.Status, job.Message = "unknown", "Luci could not save the runtime operation. Its outcome must be checked before retrying: "+err.Error()
		s.runtimeJob = job
		return err
	}
	s.runtimeJob = job
	return nil
}

func (s *Server) runRuntimeInstall(ctx context.Context, cancel context.CancelFunc, done chan struct{}, id string, run func(runtimeinstall.Publish) (runtimeinstall.Outcome, error)) {
	defer func() {
		cancel()
		s.mu.Lock()
		if s.runtimeInstall.done == done {
			s.runtimeInstall.done, s.runtimeInstall.cancel = nil, nil
		}
		close(done)
		s.mu.Unlock()
	}()
	out, err := run(func(value runtimeinstall.Outcome) error {
		if value.Phase == "ready" && ctx.Err() != nil {
			return ctx.Err()
		}
		return s.publishRuntimeInstall(id, value)
	})
	if out.JobID != "" {
		if out.Phase == "ready" && ctx.Err() != nil {
			err = ctx.Err()
		} else if saveErr := s.publishRuntimeInstall(id, out); saveErr != nil {
			err = saveErr
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.runtimeInstall.sealed || s.runtimeJob.ID != id {
		return
	}
	job := s.runtimeJob
	if err != nil || runtimeInstallActive(job.Status) {
		if runtimeInstallRetained(job) {
			job.Status, job.Message = "unknown", "The runtime operation has not confirmed it stopped. Its shared capacity remains reserved. Check the interrupted installation before trying again."
		} else if job.Execution != nil && job.Execution.Identity.Retained() && (job.Status == "unknown" || job.Execution.Phase == "publishing") {
			// A finished builder can have an uncertain tag acknowledgement.
			// Its charge is released, but only explicit observation can settle
			// publication; cancellation cannot authorize a replacement job.
			job.Status = "unknown"
			if job.Execution.Phase == "publishing" || job.Message == "" {
				job.Message = "The runtime image publication is unconfirmed. Check the interrupted installation before trying again."
			}
		} else if errors.Is(ctx.Err(), context.Canceled) || job.CancelRequested {
			job.Status, job.Message = "cancelled", "Runtime preparation was cancelled. No further installation was started."
		} else {
			job.Status = "failed"
			if err == nil {
				err = errors.New("runtime operation ended without a terminal result")
			}
			job.Message = runtimeBuildFailure(ctx, err)
		}
		job.UpdatedAt = time.Now().UTC()
		if saveErr := atomicJSON(filepath.Join(s.stateDir, "runtime-install.json"), job); saveErr != nil {
			job.Status, job.Message = "unknown", "Luci could not save the runtime result. Check the interrupted installation: "+saveErr.Error()
		}
		s.runtimeJob = job
	}
}

func (s *Server) runtimeInstallControl(w http.ResponseWriter, r *http.Request) (string, runtimeinstall.Runtime, bool) {
	var req struct {
		JobID string `json:"jobId"`
	}
	if !readJSON(w, r, &req) {
		return "", runtimeinstall.Runtime{}, false
	}
	if req.JobID == "" {
		problem(w, 400, "runtime_job_required", "Select the runtime installation to change.")
		return "", runtimeinstall.Runtime{}, false
	}
	s.mu.RLock()
	runtime := s.runtimeInstall.runtime
	s.mu.RUnlock()
	if runtime.CaptureActor == nil {
		problem(w, 503, "runtime_admission_unavailable", "Runtime management is unavailable.")
		return "", runtime, false
	}
	// Revalidate current backing-management authority even on idempotent calls.
	if _, err := runtime.CaptureActor(r.Context()); err != nil {
		problem(w, 403, "runtime_forbidden", err.Error())
		return "", runtime, false
	}
	return req.JobID, runtime, true
}

func (s *Server) cancelRuntimeInstall(w http.ResponseWriter, r *http.Request) {
	id, _, ok := s.runtimeInstallControl(w, r)
	if !ok {
		return
	}
	s.mu.RLock();native:=s.runtimeJob.OperationKind==runtimeinstall.DebianInputProofKind;s.mu.RUnlock()
	if native{s.cancelRuntimeInputProof(w,r,id);return}
	s.mu.Lock()
	if s.runtimeJob.ID != id {
		s.mu.Unlock()
		problem(w, 409, "runtime_job_changed", "The selected runtime installation is no longer current.")
		return
	}
	job, cancel := s.runtimeJob, s.runtimeInstall.cancel
	if cancel == nil || job.CancelRequested || !runtimeInstallActive(job.Status) {
		s.mu.Unlock()
		writeJSON(w, 200, job)
		return
	}
	job.CancelRequested, job.Status, job.UpdatedAt = true, "cancelling", time.Now().UTC()
	if s.runtimeInstall.proof!=nil {s.runtimeInstall.proof.invalid.Store(true)}
	job.Message = "Cancellation requested. Shared capacity stays reserved until the owned build is confirmed stopped."
	err := atomicJSON(filepath.Join(s.stateDir, "runtime-install.json"), job)
	s.runtimeJob = job
	s.mu.Unlock()
	cancel()
	if err != nil {
		problem(w, 500, "runtime_save_failed", err.Error())
		return
	}
	writeJSON(w, 202, job)
}

func (s *Server) reconcileRuntimeInstall(w http.ResponseWriter, r *http.Request) {
	id, runtime, ok := s.runtimeInstallControl(w, r)
	if !ok {
		return
	}
	s.mu.RLock();isProof:=s.runtimeJob.OperationKind==runtimeinstall.DebianInputProofKind;s.mu.RUnlock()
	if isProof{s.reconcileRuntimeInputProof(w,r,id,runtime);return}
	if runtime.Reconcile == nil {
		problem(w, 503, "runtime_reconcile_unavailable", "This runtime cannot yet prove the interrupted operation's outcome. No replacement build was started.")
		return
	}
	actor, err := runtime.CaptureActor(r.Context())
	if err != nil {
		problem(w, 403, "runtime_forbidden", err.Error())
		return
	}
	s.mu.Lock()
	if s.runtimeInstall.closed {
		s.mu.Unlock()
		problem(w, 503, "runtime_closing", "Luci is closing.")
		return
	}
	if s.runtimeJob.ID != id {
		s.mu.Unlock()
		problem(w, 409, "runtime_job_changed", "The selected runtime installation is no longer current.")
		return
	}
	job := s.runtimeJob
	if s.runtimeInstall.done != nil || !runtimeInstallUnresolved(job) {
		s.mu.Unlock()
		writeJSON(w, 200, job)
		return
	}
	if job.Execution == nil || job.Execution.JobID != job.ID {
		s.mu.Unlock()
		problem(w, 409, "runtime_legacy_unconfirmed", "This interrupted installation predates durable build ownership. Confirm its old Docker build has ended before recovering it; no new build or automatic clearance was attempted.")
		return
	}
	job.Status, job.Message, job.UpdatedAt = "reconciling", "Checking only the retained runtime operation; no build or image publication will be started.", time.Now().UTC()
	if err = atomicJSON(filepath.Join(s.stateDir, "runtime-install.json"), job); err != nil {
		s.mu.Unlock()
		problem(w, 500, "runtime_save_failed", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	done := make(chan struct{})
	s.runtimeInstall.cancel, s.runtimeInstall.done = cancel, done
	s.runtimeJob = job
	s.mu.Unlock()
	go s.runRuntimeInstall(ctx, cancel, done, id, func(publish runtimeinstall.Publish) (runtimeinstall.Outcome, error) {
		return runtime.Reconcile(ctx, job.Execution.Identity, actor, publish)
	})
	writeJSON(w, 202, job)
}

func (s *Server) closeRuntimeInstaller() {
	s.mu.Lock()
	s.runtimeInstall.closed = true
	if s.runtimeInstall.proof!=nil{s.runtimeInstall.proof.invalid.Store(true)}
	cancel, done := s.runtimeInstall.cancel, s.runtimeInstall.done
	s.mu.Unlock()
	if cancel != nil {
		cancel()
	}
	if done != nil {
		timer := time.NewTimer(5 * time.Second)
		defer timer.Stop()
		select {
		case <-done:
		case <-timer.C:
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.runtimeInstall.sealed = true
	if s.runtimeInstall.done != nil && runtimeInstallActive(s.runtimeJob.Status) {
		job := s.runtimeJob
		job.Status, job.Message, job.UpdatedAt = "unknown", "Luci closed before runtime settlement was confirmed. Check the retained operation before preparing again.", time.Now().UTC()
		if job.OperationKind!=runtimeinstall.DebianInputProofKind{_ = atomicJSON(filepath.Join(s.stateDir, "runtime-install.json"), job)}
		s.runtimeJob = job
	}
}

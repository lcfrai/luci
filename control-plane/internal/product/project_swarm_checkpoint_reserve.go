package product

import (
	"context"
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type swarmCheckpointRequest struct {
	ExpectedRevision int    `json:"expectedRevision"`
	IdempotencyKey   string `json:"idempotencyKey"`
	ReviewDigest     string `json:"reviewDigest"`
	ChildID          string `json:"childId"`
}

func (s *Server) reserveProjectSwarmCheckpoint(w http.ResponseWriter, r *http.Request) {
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	var request swarmCheckpointRequest
	if !journeyDecode(w, r, &request) {
		return
	}
	if !validRunKey(request.IdempotencyKey) || !hexDigest(request.ReviewDigest) || !prototypeReleaseID.MatchString(request.ChildID) {
		projectRunProblem(w, ErrProjectRunInvalid)
		return
	}
	if _, err := management.CaptureExecutionControlActor(r.Context(), p.ID); err != nil {
		projectRunProblem(w, err)
		return
	}
	run, err := s.projectRuns.Get(p.ID, p.Path, r.PathValue("runId"))
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	op := swarmOperation(&run, r.PathValue("operationId"))
	if op == nil || op.ChildID != request.ChildID {
		projectRunProblem(w, ErrProjectRunNotFound)
		return
	}
	queueID, err := s.SwarmQueueIDForOperation(p.ID, run.ID, op.ID)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	proofRequest := ProjectSwarmRecoveryRequest{ProjectID: p.ID, RunID: run.ID, OperationID: op.ID, ChildID: op.ChildID, QueueID: queueID, UpdatedAt: op.UpdatedAt}
	if s.projectSwarmCheckpointAuthorize == nil || s.projectSwarmCheckpointGuard == nil {
		projectRunProblem(w, errors.New("Stopped-attempt authority is unavailable; retained output has not been changed."))
		return
	}
	if err = s.projectSwarmCheckpointAuthorize(r.Context(), proofRequest); err != nil {
		projectRunProblem(w, err)
		return
	}
	// A lost response is readback of the exact original reservation, even after
	// the task's current operation/revision advances. It is never a second copy.
	priorRun, prior, found, err := s.projectRuns.FindOperationByKey(p.ID, p.Path, run.ID, request.IdempotencyKey, "session")
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if found {
		if prior.Checkpoint == nil || prior.Checkpoint.SourceOperationID != op.ID || prior.Checkpoint.SourceChildID != request.ChildID || prior.Checkpoint.ReviewDigest != request.ReviewDigest {
			projectRunProblem(w, ErrProjectRunKeyConflict)
			return
		}
		journeyJSON(w, 200, map[string]any{"run": s.projectRunView(priorRun), "operation": publicSwarmOperation(prior)})
		return
	}
	if run.Revision != request.ExpectedRevision {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	if err = validateSwarmTaskRoute(run); err != nil {
		projectRunProblem(w, err)
		return
	}
	selected, err := s.resolveProjectRunSelection(r.Context(), run)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	guard := s.branchGuard(p.ID)
	if !guard.TryLock() {
		projectRunProblem(w, errors.New("Project source is busy. Wait before continuing retained changes."))
		return
	}
	defer guard.Unlock()
	if !s.projectRunDispatchMu.TryLock() {
		projectRunProblem(w, errors.New("An agent operation is still changing this build. Wait before continuing retained changes."))
		return
	}
	defer s.projectRunDispatchMu.Unlock()
	release, err := s.projectSwarmCheckpointGuard(r.Context(), proofRequest)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if release == nil {
		projectRunProblem(w, errors.New("The execution service did not retain the original attempt's lifecycle guard."))
		return
	}
	defer release()
	run, err = s.projectRuns.Get(p.ID, p.Path, run.ID)
	if err != nil || run.Revision != request.ExpectedRevision {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	op = swarmOperation(&run, proofRequest.OperationID)
	if op == nil || op.ChildID != proofRequest.ChildID || op.UpdatedAt != proofRequest.UpdatedAt {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	if err = s.requireNoSwarmPendingLaunch(r.Context(), run, op.TaskID); err != nil {
		projectRunProblem(w, err)
		return
	}
	sourceStatus, err := s.swarmSourceSnapshot(r.Context(), p)
	if err != nil || !sourceStatus.CanSwitch {
		projectRunProblem(w, errors.New("Project source is active or cannot be safely inspected. Wait before continuing this task."))
		return
	}
	capture, err := s.captureSwarmCheckpoint(r.Context(), p, run, *op)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if !capture.Review.CanContinue || capture.Review.ReviewDigest != request.ReviewDigest {
		projectRunProblem(w, errors.New("The retained files, reviewed context or project source changed. Review the complete checkpoint again; nothing was allocated."))
		return
	}
	// Recheck authority after filesystem/provider reads while the lifecycle and
	// source guards are still held. Execution has its own later authorization.
	if err = s.projectSwarmCheckpointAuthorize(r.Context(), proofRequest); err != nil {
		projectRunProblem(w, err)
		return
	}
	checkpoint := ProjectSwarmCheckpoint{SourceOperationID: op.ID, SourceChildID: op.ChildID, SourceCommit: capture.Review.SourceCommit, SourceBaseCommit: op.BaseCommit, TargetCommit: capture.Review.TargetCommit, ReviewDigest: capture.Review.ReviewDigest, ContextDigest: implementationDigest(op.Implementation), CreatedAt: time.Now().UTC().Format(time.RFC3339Nano)}
	var pathErr error
	run, next, created, err := s.projectRuns.reserveSwarmTask(p.ID, p.Path, run.ID, run.Revision, request.IdempotencyKey, op.TaskID, checkpoint.TargetCommit, func(id string) string {
		path, e := s.swarmPath(p.ID, run.ID, id)
		if e != nil {
			pathErr = e
			return ""
		}
		return project.WorktreeID(path)
	}, &checkpoint)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	if !created || pathErr != nil {
		projectRunProblem(w, ErrProjectRunConflict)
		return
	}
	// The ledger now owns the new identity and review digest before any copy.
	// Interruption is retained as unknown; neither GET nor duplicate POST can
	// repeat allocation or resurrect the original process.
	path, allocationErr := s.swarmPath(p.ID, run.ID, next.ID)
	if allocationErr == nil {
		allocationErr = os.MkdirAll(filepath.Dir(path), 0700)
	}
	if allocationErr == nil {
		_, allocationErr = hostpath.Confine(s.projectRoot(), filepath.Dir(path))
	}
	if allocationErr == nil {
		allocationErr = s.createSwarmCheckout(r.Context(), p.Path, path, next.BaseCommit)
	}
	var seed string
	if allocationErr == nil {
		seed, allocationErr = s.applySwarmCheckpoint(r.Context(), path, next.BaseCommit, capture.Review.Files)
	}
	if allocationErr == nil {
		var recorded ProjectRun
		recorded, allocationErr = s.projectRuns.recordSwarmCheckpointSeed(p.ID, p.Path, run.ID, run.Revision, next.ID, seed)
		if allocationErr == nil {
			run = recorded
			next = *swarmOperation(&run, next.ID)
		}
	}
	var binding *registry.ImplementationContext
	if allocationErr == nil {
		binding, allocationErr = s.swarmTaskBinding(r.Context(), run, next, path)
	}
	selected.WorktreeID = next.WorktreeID
	candidate := projectRunCandidate(selected, binding)
	run, err = s.projectRuns.CompleteSwarmAllocation(p.ID, p.Path, run.ID, run.Revision, next.ID, binding, candidate, allocationErr)
	if err != nil {
		projectRunProblem(w, err)
		return
	}
	next = *swarmOperation(&run, next.ID)
	journeyJSON(w, 201, map[string]any{"run": s.projectRunView(run), "operation": publicSwarmOperation(next)})
}

func checkpointFileParent(root *os.Root, name string) (*os.Root, string, error) {
	parts := strings.Split(name, "/")
	current := root
	owned := false
	for _, part := range parts[:len(parts)-1] {
		if err := current.Mkdir(part, 0700); err != nil && !errors.Is(err, os.ErrExist) {
			if owned {
				current.Close()
			}
			return nil, "", err
		}
		next, err := openSwarmChildRoot(current, part)
		if owned {
			current.Close()
		}
		if err != nil {
			return nil, "", err
		}
		current = next
		owned = true
	}
	if !owned {
		next, err := root.OpenRoot(".")
		if err != nil {
			return nil, "", err
		}
		current = next
	}
	return current, parts[len(parts)-1], nil
}

func (s *Server) applySwarmCheckpoint(ctx context.Context, path, base string, files []ProjectSwarmCheckpointFile) (string, error) {
	root, err := os.OpenRoot(path)
	if err != nil {
		return "", err
	}
	defer root.Close()
	for _, file := range files {
		if !safeSwarmWorkspacePath(file.Path) {
			return "", ErrProjectRunInvalid
		}
		parent, name, e := checkpointFileParent(root, file.Path)
		if e != nil {
			return "", e
		}
		if old, e := openSwarmRegularFile(parent, name); e == nil {
			old.Close()
			e = parent.Remove(name)
			if e != nil {
				parent.Close()
				return "", e
			}
		} else if !errors.Is(e, os.ErrNotExist) {
			parent.Close()
			return "", e
		}
		if file.AfterMode != "" {
			mode := os.FileMode(0600)
			if file.AfterMode == "100755" {
				mode = 0700
			}
			f, e := parent.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, mode)
			if e == nil {
				_, e = ioWriteCheckpoint(f, file.After)
				closeErr := f.Close()
				if e == nil {
					e = closeErr
				}
			}
			if e != nil {
				parent.Close()
				return "", e
			}
		}
		parent.Close()
		if file.AfterMode == "" {
			_, err = s.swarmGit(ctx, path, "update-index", "--force-remove", "--", file.Path)
		} else {
			var object string
			object, err = s.swarmGit(ctx, path, "hash-object", "-w", "--no-filters", "--", file.Path)
			object = strings.TrimSpace(object)
			if err == nil && !deliverySHA.MatchString(object) {
				err = ErrProjectRunInvalid
			}
			if err == nil {
				_, err = s.swarmGit(ctx, path, "update-index", "--add", "--cacheinfo", file.AfterMode, object, file.Path)
			}
		}
		if err != nil {
			return "", err
		}
	}
	tree, err := s.swarmGit(ctx, path, "write-tree")
	if err != nil {
		return "", err
	}
	tree = strings.TrimSpace(tree)
	if !deliverySHA.MatchString(tree) {
		return "", ErrProjectRunInvalid
	}
	seed, err := s.swarmGit(ctx, path, "-c", "user.name=Luci", "-c", "user.email=luci@localhost", "commit-tree", tree, "-p", base, "-m", "Continue from explicitly reviewed retained task changes")
	if err != nil {
		return "", err
	}
	seed = strings.TrimSpace(seed)
	if !deliverySHA.MatchString(seed) {
		return "", ErrProjectRunInvalid
	}
	_, err = s.swarmGit(ctx, path, "update-ref", "HEAD", seed, base)
	return seed, err
}

func ioWriteCheckpoint(f *os.File, content string) (int, error) {
	n, err := f.WriteString(content)
	if err == nil && n != len(content) {
		err = errors.New("The checkpoint file was not completely written.")
	}
	if err == nil {
		err = f.Sync()
	}
	return n, err
}

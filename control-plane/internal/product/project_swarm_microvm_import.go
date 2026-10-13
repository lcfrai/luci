package product

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

const microVMImportFileLimit = 8 << 20
const microVMImportTotalLimit = 64 << 20

type ProjectSwarmMicroVMImportScope struct {
	ProjectID, RunID, OperationID, ChildID, OwnerID, RequestDigest string
	RequireRetired                                                 bool
	CleanupOnly                                                    bool
}

// RecordProjectSwarmMicroVMCancelled settles a cancelled run only after the
// controller proves this exact owned VM absent and its resources unheld. It
// does not import source, accept output or infer provider billing settlement.
func (s *Server) RecordProjectSwarmMicroVMCancelled(ctx context.Context, projectID, runID, operationID string, record e2bbe.Record) (ProjectRun, error) {
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	if s.projectSwarmMicroVMImportGuard == nil {
		return ProjectRun{}, ErrProjectRunInvalid
	}
	req := record.Request
	scope := ProjectSwarmMicroVMImportScope{ProjectID: projectID, RunID: runID, OperationID: operationID, ChildID: req.Identity.OperationID, OwnerID: req.Identity.OwnerID, RequestDigest: record.RequestDigest, CleanupOnly: true}
	release, err := s.projectSwarmMicroVMImportGuard(ctx, scope)
	if err != nil || release == nil {
		return ProjectRun{}, ErrProjectRunConflict
	}
	defer release()
	p, err := s.resolveProject(projectID)
	if err != nil {
		return ProjectRun{}, err
	}
	run, err := s.projectRuns.Get(projectID, p.Path, runID)
	if err != nil {
		return ProjectRun{}, err
	}
	op := swarmOperation(&run, operationID)
	if op == nil || (run.State != "cancelled" && op.Status != "cancelled" && !record.CancelRequested) || op.ChildID != req.Identity.OperationID || op.TaskID != req.Identity.TaskID ||
		req.Identity.ProjectID != projectID || req.Identity.RunID != runID || req.Identity.AttemptID != operationID ||
		record.State != e2bbe.Absent || record.ResourcesHeld || record.RequestDigest == "" {
		return ProjectRun{}, ErrProjectRunConflict
	}
	if op.Status == "cancelled" {
		return run, nil
	}
	return s.projectRuns.mutateSwarm(projectID, p.Path, runID, run.Revision, func(current *ProjectRun) error {
		selected := swarmOperation(current, operationID)
		if selected == nil || (current.State != "cancelled" && selected.Status != "cancelled" && !record.CancelRequested) || selected.ChildID != op.ChildID || selected.TaskID != op.TaskID ||
			(selected.Status != "running" && selected.Status != "dispatching" && selected.Status != "reserved" && selected.Status != "unknown") {
			return ErrProjectRunConflict
		}
		selected.Status = "cancelled"
		selected.Message = "The cancelled cloud task's exact VM is absent and its controller reservation is released. Returned work was not accepted; provider billing settlement remains separate."
		return nil
	})
}

// RecordProjectSwarmMicroVMLaunch shows a verified task process in the usual
// run/queue view. An envd Start ACK alone is insufficient: the exact protected
// guest launch identity must be observed, and current authority rechecked.
func (s *Server) RecordProjectSwarmMicroVMLaunch(ctx context.Context, projectID, runID, operationID string, record e2bbe.Record) (ProjectRun, error) {
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	if s.projectSwarmMicroVMImportGuard == nil {
		return ProjectRun{}, ErrProjectRunInvalid
	}
	req := record.Request
	scope := ProjectSwarmMicroVMImportScope{ProjectID: projectID, RunID: runID, OperationID: operationID, ChildID: req.Identity.OperationID, OwnerID: req.Identity.OwnerID, RequestDigest: record.RequestDigest}
	release, err := s.projectSwarmMicroVMImportGuard(ctx, scope)
	if err != nil || release == nil {
		return ProjectRun{}, ErrProjectRunConflict
	}
	defer release()
	p, err := s.resolveProject(projectID)
	if err != nil {
		return ProjectRun{}, err
	}
	run, err := s.projectRuns.Get(projectID, p.Path, runID)
	if err != nil {
		return ProjectRun{}, err
	}
	op := swarmOperation(&run, operationID)
	if op == nil || op.Implementation == nil || op.Allocation != "ready" || op.ChildID == "" ||
		run.State == "cancelled" || (op.Status != "reserved" && op.Status != "dispatching" && op.Status != "running") ||
		req.Identity.ProjectID != projectID || req.Identity.RunID != runID || req.Identity.TaskID != op.TaskID ||
		req.Identity.AttemptID != op.ID || req.Identity.OperationID != op.ChildID ||
		req.SourceDigest != implementationDigest(swarmStartingCommit(*op)) || req.ContextDigest != implementationDigest(op.Implementation) ||
		!record.LaunchRequested || record.ProcessPID == 0 || e2bbe.ValidateCaptureIdentity(record.LaunchKernel, record) != nil {
		return ProjectRun{}, ErrProjectRunConflict
	}
	const message = "The cloud task's exact upstream process is running in its owned microVM. Its result and checks remain unverified."
	return s.projectRuns.ObserveOperation(projectID, p.Path, runID, run.Revision, op.ID, op.ChildID, "running", message)
}

// RecordProjectSwarmMicroVMExit makes a captured native exit visible to the
// existing review flow. It never treats a provider Create, envd Start, text
// status or retained archive alone as a completed task. The API guard must
// hold current authority and the exact cloud-child lifecycle fence.
func (s *Server) RecordProjectSwarmMicroVMExit(ctx context.Context, projectID, runID, operationID string, record e2bbe.Record) (ProjectRun, error) {
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	if s.projectSwarmMicroVMImportGuard == nil {
		return ProjectRun{}, ErrProjectRunInvalid
	}
	req := record.Request
	scope := ProjectSwarmMicroVMImportScope{ProjectID: projectID, RunID: runID, OperationID: operationID, ChildID: req.Identity.OperationID, OwnerID: req.Identity.OwnerID, RequestDigest: record.RequestDigest, RequireRetired: true}
	release, err := s.projectSwarmMicroVMImportGuard(ctx, scope)
	if err != nil || release == nil {
		return ProjectRun{}, ErrProjectRunConflict
	}
	defer release()
	p, err := s.resolveProject(projectID)
	if err != nil {
		return ProjectRun{}, err
	}
	run, err := s.projectRuns.Get(projectID, p.Path, runID)
	if err != nil {
		return ProjectRun{}, err
	}
	op := swarmOperation(&run, operationID)
	if op == nil || op.Implementation == nil || op.ChildID == "" || op.Allocation != "ready" || op.Integration != nil ||
		run.State == "cancelled" || swarmTask(&run, op.TaskID) == nil ||
		(op.Status != "running" && op.Status != "dispatching" && op.Status != "reserved" && op.Status != "review-required") ||
		req.Identity.ProjectID != projectID || req.Identity.RunID != runID || req.Identity.TaskID != op.TaskID || req.Identity.AttemptID != op.ID || req.Identity.OperationID != op.ChildID ||
		req.SourceDigest != implementationDigest(swarmStartingCommit(*op)) || req.ContextDigest != implementationDigest(op.Implementation) ||
		!record.LaunchRequested || !record.CaptureRequested || e2bbe.ValidateNativeExitReceipt(record.NativeExit, record) != nil ||
		e2bbe.ValidateCaptureReceipt(record.Capture, record.LaunchKernel, record) != nil {
		return ProjectRun{}, ErrProjectRunConflict
	}
	const message = "The cloud task's native exit and source capture are retained. Review its candidate and checks before integration."
	return s.projectRuns.ObserveOperation(projectID, p.Path, runID, run.Revision, op.ID, op.ChildID, "review-required", message)
}

// SetProjectSwarmMicroVMImportGuard requires a current scoped authority lease
// around source mutation. An unregistered cloud completion path cannot import.
func (s *Server) SetProjectSwarmMicroVMImportGuard(guard func(context.Context, ProjectSwarmMicroVMImportScope) (func(), error)) {
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	s.projectSwarmMicroVMImportGuard = guard
}

// ImportProjectSwarmMicroVMCandidate turns verified returned source into a
// committed candidate in this operation's isolated Git worktree. It does not
// integrate into primary, claim independent checks or approve a deployment.
// The API must have established current actor authority and used the exact
// controller-owned output store; the source lock/revision gates are repeated
// here before any write. A partial local Git write is retained for inspection,
// never replayed over a dirty worktree.
func (s *Server) ImportProjectSwarmMicroVMCandidate(ctx context.Context, projectID, runID, operationID string, record e2bbe.Record, changes e2bbe.SourceChanges, staging string) (string, error) {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return "", err
	}
	guard := s.branchGuard(projectID)
	if !guard.TryLock() {
		return "", errors.New("Project source is busy; retain the cloud candidate for later review.")
	}
	defer guard.Unlock()
	s.projectRunDispatchMu.Lock()
	defer s.projectRunDispatchMu.Unlock()
	if s.projectSwarmMicroVMImportGuard == nil {
		return "", ErrProjectRunInvalid
	}
	scope := ProjectSwarmMicroVMImportScope{ProjectID: projectID, RunID: runID, OperationID: operationID, ChildID: record.Request.Identity.OperationID, OwnerID: record.Request.Identity.OwnerID, RequestDigest: record.RequestDigest, RequireRetired: true}
	release, err := s.projectSwarmMicroVMImportGuard(ctx, scope)
	if err != nil || release == nil {
		return "", ErrProjectRunConflict
	}
	defer release()
	run, err := s.projectRuns.Get(projectID, p.Path, runID)
	if err != nil {
		return "", err
	}
	op := swarmOperation(&run, operationID)
	if op == nil || op.Implementation == nil || op.Allocation != "ready" || op.Integration != nil || op.ChildID == "" {
		return "", ErrProjectRunConflict
	}
	task := swarmTask(&run, op.TaskID)
	if task == nil || task.OperationID != op.ID || task.Integration != nil || run.State == "cancelled" {
		return "", ErrProjectRunConflict
	}
	if op.Status != "review-required" && op.Status != "failed" && op.Status != "succeeded" {
		return "", ErrProjectRunConflict
	}
	req := record.Request
	if req.Identity.ProjectID != projectID || req.Identity.RunID != runID || req.Identity.TaskID != op.TaskID || req.Identity.AttemptID != op.ID || req.Identity.OperationID != op.ChildID ||
		req.SourceDigest != implementationDigest(swarmStartingCommit(*op)) || req.ContextDigest != implementationDigest(op.Implementation) ||
		!record.CaptureRequested || e2bbe.ValidateCaptureReceipt(record.Capture, record.LaunchKernel, record) != nil ||
		e2bbe.ValidateNativeExitReceipt(record.NativeExit, record) != nil || changes.InputBundleSHA256 != req.BundleDigest ||
		changes.SourceManifestSHA256 != record.Capture.SourceManifestSHA256 {
		return "", ErrProjectRunConflict
	}
	changeBytes, err := json.Marshal(changes)
	if err != nil || !sameMicroVMBytes(string(changeBytes), record.Capture.ChangeManifestSHA256) {
		return "", ErrProjectRunConflict
	}
	if err = s.validateProjectSwarmReuseCurrent(ctx, p, run); err != nil {
		return "", err
	}
	path, err := s.swarmPath(projectID, runID, operationID)
	if err != nil || project.WorktreeID(path) != op.WorktreeID {
		return "", ErrProjectRunInvalid
	}
	if err = s.validateSwarmTaskGit(ctx, path); err != nil {
		return "", err
	}
	active, err := s.swarmWorktreeActive(ctx, projectID, op.WorktreeID)
	if err != nil || active {
		return "", errors.New("The task checkout still has a live or uncertain child; retain the cloud output until it is settled.")
	}
	base := swarmStartingCommit(*op)
	source, err := s.captureGitReview(ctx, path, "")
	if err != nil || source.Status != "" {
		return "", errors.New("The task checkout changed before cloud import. Preserve its bytes for explicit recovery.")
	}
	if source.Head != base {
		return s.verifyImportedMicroVMCandidate(ctx, path, base, source.Head, task.WritePaths, changes)
	}
	stageInfo, err := os.Lstat(staging)
	if err != nil || !stageInfo.IsDir() || stageInfo.Mode()&os.ModeSymlink != 0 || runtime.GOOS != "windows" && stageInfo.Mode().Perm()&0077 != 0 {
		return "", ErrProjectRunInvalid
	}
	root, err := os.OpenRoot(staging)
	if err != nil {
		return "", err
	}
	defer root.Close()
	workerRoot, err := os.OpenRoot(path)
	if err != nil {
		return "", err
	}
	defer workerRoot.Close()
	baseFiles, err := s.checkpointTree(ctx, path, base)
	if err != nil {
		return "", err
	}
	files := make([]ProjectSwarmCheckpointFile, 0, len(changes.Changes))
	var total int64
	for _, change := range changes.Changes {
		if !safeSwarmWorkspacePath(change.Path) || blockedProjectFile(change.Path) || strings.EqualFold(change.Path, "LUCI.md") ||
			!validProjectTaskWritePath(change.Path) || len(task.WritePaths) > 0 && !swarmPathAllowed(change.Path, task.WritePaths) {
			return "", errors.New("The cloud candidate changes a protected or out-of-scope project path.")
		}
		file := ProjectSwarmCheckpointFile{Path: change.Path, Status: change.Kind}
		prior, existed := baseFiles[change.Path]
		if existed != (change.Before != nil) {
			return "", errors.New("The cloud candidate's changed-path baseline differs from the assigned Git commit.")
		}
		if !existed {
			current, openErr := openSwarmWorkspaceFile(workerRoot, change.Path)
			if openErr == nil {
				current.Close()
				return "", errors.New("An untracked task file occupies the cloud candidate's new path.")
			}
			if !errors.Is(openErr, os.ErrNotExist) {
				return "", openErr
			}
		}
		if change.Before != nil {
			if change.Before.Path != change.Path || change.Before.Size > microVMImportFileLimit {
				return "", ErrProjectRunInvalid
			}
			file.BeforeMode = gitMode(change.Before.Mode)
			if prior.Mode != file.BeforeMode || prior.Size != change.Before.Size {
				return "", errors.New("The cloud candidate's starting file mode or size changed.")
			}
			file.Before, err = s.swarmGit(ctx, path, "show", base+":"+change.Path)
			if err != nil || int64(len(file.Before)) != change.Before.Size || !sameMicroVMBytes(file.Before, change.Before.SHA256) {
				return "", errors.New("The cloud candidate's starting bytes no longer match its exact source bundle.")
			}
		}
		if change.After != nil {
			if change.After.Path != change.Path || change.After.Size > microVMImportFileLimit {
				return "", ErrProjectRunInvalid
			}
			file.AfterMode = gitMode(change.After.Mode)
			var opened *os.File
			opened, err = openSwarmWorkspaceFile(root, change.Path)
			if err != nil {
				return "", err
			}
			stageFileInfo, statErr := opened.Stat()
			if statErr != nil || stageFileInfo.Mode().Perm() != os.FileMode(change.After.Mode) {
				opened.Close()
				return "", errors.New("The staged source mode differs from its verified manifest.")
			}
			body, readErr := io.ReadAll(io.LimitReader(opened, microVMImportFileLimit+1))
			closeErr := opened.Close()
			if readErr != nil || closeErr != nil || int64(len(body)) != change.After.Size || !sameMicroVMBytes(string(body), change.After.SHA256) {
				return "", errors.New("The staged cloud candidate changed after its verified capture.")
			}
			file.After = string(body)
		}
		if change.Before == nil && change.After == nil || change.Before != nil && change.After != nil && change.Kind != "modified" ||
			change.Before == nil && change.Kind != "added" || change.After == nil && change.Kind != "deleted" ||
			file.BeforeMode == "" && change.Before != nil || file.AfterMode == "" && change.After != nil {
			return "", ErrProjectRunInvalid
		}
		total += int64(len(file.Before) + len(file.After))
		if total > microVMImportTotalLimit || len(files) >= swarmCheckpointPathLimit {
			return "", errors.New("The cloud candidate exceeds the complete review limit; split it into smaller scoped tasks.")
		}
		files = append(files, file)
	}
	if len(files) == 0 {
		return "", errors.New("The cloud task returned no source changes to review.")
	}
	end, err := s.captureGitReview(ctx, path, "")
	if err != nil || end.Digest != source.Digest {
		return "", ErrProjectRunConflict
	}
	return s.applySwarmCheckpoint(ctx, path, base, files)
}

func gitMode(mode uint32) string {
	switch mode {
	case 0644:
		return "100644"
	case 0755:
		return "100755"
	}
	return ""
}

func sameMicroVMBytes(body, expected string) bool {
	hash := sha256.Sum256([]byte(body))
	return hex.EncodeToString(hash[:]) == expected
}

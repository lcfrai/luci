package product

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

// RecordProjectSwarmMicroVMHandoff publishes a controller-captured worker
// claim in the ordinary task review ledger. The caller must first retrieve
// rawReport from the exact controller-owned output store. Neither this claim
// nor the worker's reported checks approve integration or delivery.
func (s *Server) RecordProjectSwarmMicroVMHandoff(ctx context.Context, projectID, runID, operationID string, record e2bbe.Record, rawReport []byte) (ProjectRun, error) {
	p, err := s.resolveProject(projectID)
	if err != nil {
		return ProjectRun{}, err
	}
	branch := s.branchGuard(projectID)
	if !branch.TryRLock() {
		return ProjectRun{}, ErrProjectRunPending
	}
	defer branch.RUnlock()
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
	run, err := s.projectRuns.Get(projectID, p.Path, runID)
	if err != nil {
		return ProjectRun{}, err
	}
	op := swarmOperation(&run, operationID)
	if op == nil || op.Implementation == nil || op.ChildID == "" || op.Allocation != "ready" || op.Integration != nil ||
		run.State == "cancelled" || op.Status != "review-required" || swarmTask(&run, op.TaskID) == nil ||
		req.Identity.ProjectID != projectID || req.Identity.RunID != runID || req.Identity.TaskID != op.TaskID ||
		req.Identity.AttemptID != op.ID || req.Identity.OperationID != op.ChildID ||
		req.SourceDigest != implementationDigest(swarmStartingCommit(*op)) || req.ContextDigest != implementationDigest(op.Implementation) ||
		!record.LaunchRequested || !record.CaptureRequested || e2bbe.ValidateNativeExitReceipt(record.NativeExit, record) != nil ||
		e2bbe.ValidateCaptureReceipt(record.Capture, record.LaunchKernel, record) != nil {
		return ProjectRun{}, ErrProjectRunConflict
	}
	if record.Capture.ReportSHA256 == "" && len(rawReport) != 0 || record.Capture.ReportSHA256 != "" &&
		(len(rawReport) == 0 || !sameMicroVMBytes(string(rawReport), record.Capture.ReportSHA256)) {
		return ProjectRun{}, ErrProjectRunConflict
	}
	if op.Handoff != nil {
		view, err := s.projectRuns.taskHandoffView(run, *op)
		if err != nil || view.Receipt == nil || view.Receipt.CloudRequestDigest != record.RequestDigest ||
			view.Receipt.CloudCaptureDigest != record.Capture.ArchiveSHA256 ||
			view.Receipt.CloudReportDigest != record.Capture.ReportSHA256 ||
			view.Receipt.CloudExitFrameDigest != record.NativeExit.FrameSHA256 {
			return ProjectRun{}, ErrProjectRunConflict
		}
		return run, nil
	}
	path, err := s.swarmPath(projectID, runID, operationID)
	if err != nil || project.WorktreeID(path) != op.WorktreeID {
		return ProjectRun{}, ErrProjectRunInvalid
	}
	if err = s.validateSwarmTaskGit(ctx, path); err != nil {
		return ProjectRun{}, err
	}
	active, err := s.swarmWorktreeActive(ctx, projectID, op.WorktreeID)
	if err != nil || active {
		return ProjectRun{}, errors.New("The cloud task checkout still has live or uncertain work; retain its report until settlement.")
	}
	sourceDigest, commit, dirty, err := s.checkSource(ctx, path)
	if err != nil || dirty {
		return ProjectRun{}, errors.New("The cloud task source is dirty; retain its report until the candidate is reviewed.")
	}
	status, reason := "missing", "task_report_missing"
	var report *TaskHandoffReport
	if len(rawReport) != 0 {
		report, err = DecodeTaskHandoffReport(rawReport)
		if err == nil {
			status, reason = "captured", ""
		} else {
			status, reason = "invalid", "task_report_invalid"
		}
	}
	receipt := TaskHandoffReceipt{Version: 1, ProjectID: projectID, RunID: runID, TaskID: op.TaskID, OperationID: op.ID,
		ChildID: op.ChildID, WorktreeID: op.WorktreeID, RequestDigest: op.RequestDigest,
		CloudRequestDigest: record.RequestDigest, CloudCaptureDigest: record.Capture.ArchiveSHA256,
		CloudReportDigest: record.Capture.ReportSHA256, CloudExitFrameDigest: record.NativeExit.FrameSHA256,
		ImplementationDigest: implementationDigest(op.Implementation), ContextDigest: req.ContextDigest,
		SourceCommit: commit, SourceDigest: sourceDigest, Status: status, ReasonCode: reason,
		CapturedAt: record.Capture.CapturedAt.UTC().Format(time.RFC3339Nano), Reported: report}
	digest, err := s.projectRuns.writeHandoffObject(projectID, receipt, TaskHandoffReportLimit+16384)
	if err != nil {
		return ProjectRun{}, err
	}
	ref := taskHandoffReference(receipt, digest)
	return s.projectRuns.mutateSwarm(projectID, p.Path, runID, run.Revision, func(current *ProjectRun) error {
		selected := swarmOperation(current, operationID)
		if selected == nil || selected.ChildID != op.ChildID || selected.RequestDigest != op.RequestDigest ||
			selected.Status != "review-required" || selected.Integration != nil || selected.Handoff != nil || current.State == "cancelled" {
			return ErrProjectRunConflict
		}
		selected.Handoff = ref
		return nil
	})
}

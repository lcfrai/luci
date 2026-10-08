package api

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/hostpath"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

const taskHandoffOutputDirectory = ".task-output"
const taskHandoffContainerDirectory = "/home/dev/.luci-task-output"

func (a *API) taskHandoffOutputPath(sessionID string) (string, error) {
	if !reviewedSessionID.MatchString(sessionID) {
		return "", errors.New("invalid task output owner")
	}
	return filepath.Join(a.deps.ConversationsDir, sessionConversationOwner, taskHandoffOutputDirectory, sessionID), nil
}
func (a *API) prepareTaskHandoffOutput(sessionID string) (backend.Mount, error) {
	dir, err := a.taskHandoffOutputPath(sessionID)
	if err != nil {
		return backend.Mount{}, err
	}
	if _, err = a.assistDirectory(filepath.Join(sessionConversationOwner, taskHandoffOutputDirectory)); err != nil {
		return backend.Mount{}, err
	}
	// An existing directory is retained output, never a new task's storage.
	if err = os.Mkdir(dir, 0700); err != nil {
		return backend.Mount{}, err
	}
	if err = codexPrivateDir(dir); err != nil {
		return backend.Mount{}, err
	}
	return backend.Mount{HostPath: dir, ContainerPath: taskHandoffContainerDirectory, ReadOnly: false}, nil
}

func (a *API) collectTaskHandoff(ctx context.Context, req product.TaskHandoffCaptureRequest) (product.TaskHandoffCapture, error) {
	result := product.TaskHandoffCapture{Status: "unavailable", ReasonCode: "task_output_unavailable"}
	// Automatic collection observes already-owned output; it neither executes
	// work nor impersonates the original actor. Explicit mutations separately
	// require fresh project Edit authority in product before and after capture.
	release, err := a.lockStoppedTaskOutput(ctx, req.ProjectSwarmRecoveryRequest, func(ctx context.Context, _ product.ProjectSwarmRecoveryRequest) error { return ctx.Err() })
	if err != nil {
		return result, err
	}
	result.Release = release
	failed := true
	defer func() {
		if failed {
			release()
		}
	}()
	run, err := a.deps.Product.GetProjectSwarmRun(req.ProjectID, req.RunID)
	if err != nil {
		return result, err
	}
	matched := false
	for _, op := range run.Operations {
		if op.ID == req.OperationID && op.ChildID == req.ChildID && op.TaskID == req.TaskID && op.WorktreeID == req.WorktreeID && op.RequestDigest == req.RequestDigest && taskOutputImplementationDigest(op.Implementation) == req.ImplementationDigest {
			matched = true
		}
	}
	if !matched {
		return result, errTaskOutputUnsettled
	}
	row, err := a.deps.Store.Get(req.ChildID)
	if err != nil {
		return result, err
	}
	if row.ReviewedContext == nil {
		result.ReasonCode = "task_context_not_recorded"
		failed = false
		return result, nil
	}
	if _, err = a.readReviewedContext(row.ID, row.ImplementationContext, row.ReviewedContext); err != nil {
		return result, err
	}
	result.ContextDigest = row.ReviewedContext.Digest
	dir, err := a.taskHandoffOutputPath(req.ChildID)
	if err != nil {
		return result, err
	}
	if err = codexRegularPath(a.deps.ConversationsDir, dir); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			result.Status, result.ReasonCode = "missing", "task_report_missing"
			failed = false
			return result, nil
		}
		return result, err
	}
	if _, err = hostpath.ResolveOutside(a.projectsDir(), dir); err != nil {
		return result, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return result, err
	}
	defer root.Close()
	if _, err = root.Lstat("handoff.json"); errors.Is(err, os.ErrNotExist) {
		result.Status, result.ReasonCode = "missing", "task_report_missing"
		failed = false
		return result, nil
	}
	data, err := reviewedReadFile(root, dir, "handoff.json", product.TaskHandoffReportLimit)
	if errors.Is(err, os.ErrNotExist) {
		result.Status, result.ReasonCode = "missing", "task_report_missing"
		failed = false
		return result, nil
	}
	if err != nil {
		result.Status, result.ReasonCode = "invalid", "task_report_unsafe_or_oversized"
		failed = false
		return result, nil
	}
	report, err := product.DecodeTaskHandoffReport(data)
	if err != nil {
		result.Status, result.ReasonCode = "invalid", "task_report_schema_invalid"
		failed = false
		return result, nil
	}
	result.Status, result.ReasonCode, result.Report = "captured", "", report
	failed = false
	return result, nil
}

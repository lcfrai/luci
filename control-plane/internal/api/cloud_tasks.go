package api

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"os"
	"runtime"

	"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

// CloudTaskRuntime is selected only by an operator-verified E2B deployment.
// AttachKey stays on the durable Luci controller, never in a task archive or
// VM journal. It derives the same per-attempt bearer across API restarts.
type CloudTaskRuntime struct {
	Runtime             *e2bbe.TaskRuntime
	AttachKey           []byte
	MaxSeconds          int
	EstimatedCostMicros int64
	StagingDir          string
}

func (c *CloudTaskRuntime) validate() error {
	if c == nil || c.Runtime == nil || c.Runtime.Controller == nil || c.Runtime.Store == nil ||
		len(c.AttachKey) != 32 || c.MaxSeconds < 1 || c.MaxSeconds > 86400 ||
		c.EstimatedCostMicros < 1 || c.EstimatedCostMicros > 1<<50 {
		return errors.New("api: configured cloud microVM runtime lacks a durable controller, owner key or bounded task cost")
	}
	info, err := os.Lstat(c.StagingDir)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 || runtime.GOOS != "windows" && info.Mode().Perm()&0077 != 0 {
		return errors.New("api: cloud candidate staging must be an existing private directory on the controller")
	}
	return nil
}

func (c *CloudTaskRuntime) attachToken(request e2bbe.Request) string {
	encoded, _ := json.Marshal(request)
	digest := sha256.Sum256(encoded)
	mac := hmac.New(sha256.New, c.AttachKey)
	_, _ = mac.Write([]byte("luci-cloud-task-attach-v1:"))
	_, _ = mac.Write(digest[:])
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

func (a *API) cloudRecord(projectID, runID, operationID, childID string) (e2bbe.Record, bool) {
	if a.deps.CloudTasks == nil {
		return e2bbe.Record{}, false
	}
	rows, _ := a.deps.CloudTasks.Runtime.Controller.Snapshot()
	for _, row := range rows {
		i := row.Request.Identity
		if i.OperationID == childID && i.ProjectID == projectID && i.RunID == runID && i.AttemptID == operationID &&
			i.OwnerID == a.deps.CloudTasks.Runtime.Controller.OwnerID() {
			return row, true
		}
	}
	return e2bbe.Record{}, false
}

func (a *API) cloudRetired(_ context.Context, projectID, runID, operationID, childID string) (bool, error) {
	row, ok := a.cloudRecord(projectID, runID, operationID, childID)
	return ok && row.State == e2bbe.Absent && !row.ResourcesHeld, nil
}

func (a *API) guardMicroVMImport(ctx context.Context, scope product.ProjectSwarmMicroVMImportScope) (func(), error) {
	row, ok := a.cloudRecord(scope.ProjectID, scope.RunID, scope.OperationID, scope.ChildID)
	if !ok || row.RequestDigest != scope.RequestDigest || row.Request.Identity.OwnerID != scope.OwnerID ||
		(scope.CleanupOnly && (scope.RequireRetired || row.State != e2bbe.Absent || row.ResourcesHeld)) ||
		(scope.RequireRetired && (row.State != e2bbe.Absent || row.ResourcesHeld ||
			e2bbe.ValidateNativeExitReceipt(row.NativeExit, row) != nil || e2bbe.ValidateCaptureReceipt(row.Capture, row.LaunchKernel, row) != nil)) ||
		(!scope.RequireRetired && !scope.CleanupOnly && (row.State != e2bbe.Allocated || !row.ResourcesHeld ||
			e2bbe.ValidateCaptureIdentity(row.LaunchKernel, row) != nil)) {
		return nil, product.ErrProjectRunConflict
	}
	if scope.CleanupOnly {
		_, err := a.deps.Product.CloudSwarmCleanupOwner(scope)
		if err != nil {
			return nil, err
		}
		return func() {}, nil
	}
	actor, err := a.deps.Product.CloudSwarmQueueOwner(scope)
	if err != nil {
		return nil, err
	}
	run, err := a.deps.Product.GetProjectSwarmRun(scope.ProjectID, scope.RunID)
	if err != nil {
		return nil, err
	}
	if _, err = a.authorizeExecution(ctx, actor, run.Input.Route.Harness, true); err != nil {
		return nil, err
	}
	if a.deps.AuthorizeExecutionControl != nil {
		if err = a.deps.AuthorizeExecutionControl(ctx, actor); err != nil {
			return nil, err
		}
	}
	return func() {}, nil
}

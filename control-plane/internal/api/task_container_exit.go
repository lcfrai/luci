package api

import (
	"context"
	"errors"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

const taskContainerExitMethod = registry.TaskTerminalResultMethod

func taskContainerResultMatches(row registry.Session) bool {
	return row.HasTaskContainerResult()
}

func taskContainerNormalZero(row registry.Session) bool {
	return row.Status == registry.StatusExited && taskContainerResultMatches(row) && row.TaskTerminalResult.NormalZero
}

// Observe the original daemon allocation, not a user-controlled file or a
// reusable container name. This transport budget does not expire a worker or
// renew its authority. The existing reaper/completion service retries reads,
// never Create/Start, and persists the fact before original-container cleanup.
func (a *API) observeTaskContainerExit(ctx context.Context, original registry.Session) (bool, error) {
	if !terminalSession(original) || original.TaskTerminalImage == "" {
		return false, backend.ErrUnsupported
	}
	if len(original.Args) < 4 || !reviewedDigest.MatchString(original.Args[1]) {
		return false, errors.New("original task contract identity is unavailable")
	}
	if original.TaskTerminalResult != nil {
		return taskContainerResultMatches(original), nil
	}
	if a.execution == nil || original.ID == "" || original.CreatedAt.IsZero() || original.TaskTerminalCreatedAt.IsZero() || original.TaskTerminalStartedAt.IsZero() {
		return false, errors.New("original task allocation timestamps are unavailable")
	}
	observer, ok := a.deps.Backend.(backend.TaskExitObserver)
	if !ok {
		return false, backend.ErrUnsupported
	}
	bounded, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	release, err := a.execution.lockChildLifecycle(bounded, original.ID)
	if err != nil {
		return false, err
	}
	defer release()
	current, err := a.deps.Store.Get(original.ID)
	if err != nil {
		return false, err
	}
	if !taskTerminalObservationMatches(current, original) || current.TaskTerminalImage != original.TaskTerminalImage || !current.TaskTerminalCreatedAt.Equal(original.TaskTerminalCreatedAt) || !current.TaskTerminalStartedAt.Equal(original.TaskTerminalStartedAt) {
		return false, sessionErrMoved
	}
	if err := a.execution.confirmSettledTaskLease(bounded, current); err != nil {
		return false, err
	}
	if a.deps.Product == nil {
		return false, product.ErrProjectRunPending
	}
	run, err := a.deps.Product.GetProjectSwarmRun(current.ProjectID, current.ProjectRunID)
	if err != nil {
		return false, err
	}
	matched := false
	for _, operation := range run.Operations {
		if operation.ID == current.ProjectRunOperationID && operation.ChildID == current.ID && operation.TaskID != "" && operation.WorktreeID == current.WorktreeID && operation.RequestDigest == product.ProjectRunSessionRequestDigest(current) && operation.Allocation == "ready" && operation.AbsenceRetirement == nil {
			matched = true
		}
	}
	if !matched {
		return false, product.ErrProjectRunConflict
	}
	observed, err := observer.ObserveTaskExit(bounded, backend.TaskExitIdentity{SessionID: current.ID, SandboxID: current.SandboxID, Ref: current.SandboxRef, Image: current.TaskTerminalImage, ContractDigest: current.Args[1], CreatedAt: current.TaskTerminalCreatedAt, StartedAt: current.TaskTerminalStartedAt})
	if err != nil {
		return false, err
	}
	fact := registry.TaskTerminalResult{Method: taskContainerExitMethod, SessionCreatedAt: current.CreatedAt, Ref: observed.Ref, Image: observed.Image, ContractDigest: observed.ContractDigest, CreatedAt: observed.CreatedAt, StartedAt: observed.StartedAt, FinishedAt: observed.FinishedAt, NormalZero: observed.NormalZero, OOMKilled: observed.OOMKilled, ContainerExitCode: observed.ContainerExitCode}
	probe := current.Clone()
	probe.TaskTerminalResult = &fact
	if !taskContainerResultMatches(probe) {
		return false, product.ErrProjectRunConflict
	}
	_, err = a.deps.Store.Update(current.ID, func(row *registry.Session) error {
		if bounded.Err() != nil || !taskTerminalObservationMatches(*row, current) || row.TaskTerminalImage != current.TaskTerminalImage || !row.TaskTerminalCreatedAt.Equal(current.TaskTerminalCreatedAt) || !row.TaskTerminalStartedAt.Equal(current.TaskTerminalStartedAt) {
			return sessionErrMoved
		}
		if row.TaskTerminalResult != nil {
			if *row.TaskTerminalResult == fact {
				return nil
			}
			return sessionErrMoved
		}
		// An explicit user stop, allocation failure or changed lifecycle reason
		// cannot be upgraded to success merely because its container ended zero.
		if row.Status == registry.StatusExited && row.ExitReason != "" && row.ExitReason != "process exited" && row.ExitReason != "exited" && !(row.TaskTerminalExitZero && row.TaskTerminalObserverStarted) {
			fact.NormalZero = false
		}
		row.TaskTerminalResult = &fact
		row.Status = registry.StatusExited
		if !fact.NormalZero {
			row.TaskTerminalExitZero = false
		}
		if fact.OOMKilled {
			row.ExitReason = "task container OOM observed"
		}
		if row.ExitReason == "" || row.ExitReason == "process exited" || row.ExitReason == "exited" {
			if fact.NormalZero {
				row.ExitReason = "process exited"
			} else if fact.OOMKilled {
				row.ExitReason = "task container OOM observed"
			} else {
				row.ExitReason = "task container ended unsuccessfully or with unknown child outcome"
			}
		}
		row.Endpoint, row.AttachToken = "", ""
		row.TaskObservationComplete = false
		return nil
	})
	return err == nil, err
}

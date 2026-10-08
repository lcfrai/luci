package api

import (
	"context"
	"errors"
	"sync"

	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

type taskCompletionObservations struct {
	mu        sync.Mutex
	cursor    int
	pending   []registry.TaskObservationCandidate
	lastError string
}

type taskObservationReader interface {
	TaskObservationPage(cursor, scanLimit, resultLimit int) ([]registry.TaskObservationCandidate, int, error)
}

// ObserveTaskSessionsOnce is a bounded controller observation, not a user
// request. Durable exited registry rows without the completion watermark are
// its work list, including rows retained across restart or backend cleanup.
// No human actor, project grant, prompt or execution permission is reused.
func (a *API) ObserveTaskSessionsOnce(ctx context.Context) (result error) {
	if a.deps.Product == nil {
		return nil
	}
	reader, ok := a.deps.Store.(taskObservationReader)
	if !ok {
		return errors.New("the session store cannot supply bounded task observations")
	}
	if !a.taskObservations.mu.TryLock() {
		return nil
	}
	defer a.taskObservations.mu.Unlock()
	defer func() {
		message := ""
		if result != nil {
			message = result.Error()
			if len(message) > 4000 {
				message = message[:4000]
			}
		}
		if message != "" && message != a.taskObservations.lastError {
			a.sessionLog().Warn("task completion observation remains pending; retained ownership will be retried", "reason", message)
		}
		a.taskObservations.lastError = message
	}()
	if err := ctx.Err(); err != nil {
		return err
	}
	if len(a.taskObservations.pending) == 0 {
		candidates, next, err := reader.TaskObservationPage(a.taskObservations.cursor, 128, 8)
		if err != nil {
			return err
		}
		a.taskObservations.pending = candidates
		a.taskObservations.cursor = next
	}
	// Keep at most one bounded page of metadata, not full session/context rows.
	// Consume each candidate only when its attempt starts. A slow first child
	// may exhaust this turn, but the next turn starts with its unattempted peers
	// instead of wrapping the scan cursor and selecting that same child again.
	// Restart discards this optimization; durable unmarked rows remain eligible.
	var failures []error
	for len(a.taskObservations.pending) != 0 {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(failures, err)...)
		}
		candidate := a.taskObservations.pending[0]
		a.taskObservations.pending[0] = registry.TaskObservationCandidate{}
		a.taskObservations.pending = a.taskObservations.pending[1:]
		if err := a.observeTaskCompletionCandidate(ctx, candidate); err != nil {
			failures = append(failures, err)
		}
	}
	a.taskObservations.pending = nil
	return errors.Join(failures...)
}

func sameTaskObservationCandidate(candidate registry.TaskObservationCandidate, row registry.Session) bool {
	return candidate.ID == row.ID && candidate.ProjectID == row.ProjectID && candidate.RunID == row.ProjectRunID &&
		candidate.OperationID == row.ProjectRunOperationID && candidate.SandboxID == row.SandboxID &&
		candidate.SandboxRef == row.SandboxRef && candidate.Status == row.Status && candidate.UpdatedAt.Equal(row.UpdatedAt)
}

func (a *API) observeTaskCompletionCandidate(ctx context.Context, candidate registry.TaskObservationCandidate) error {
	row, err := a.deps.Store.Get(candidate.ID)
	if errors.Is(err, registry.ErrNotFound) {
		return nil // Already removed by an explicit, independently guarded Forget.
	}
	if err != nil {
		return err
	}
	if !sameTaskObservationCandidate(candidate, row) || row.Status != registry.StatusExited {
		return product.ErrProjectRunConflict
	}
	if row.TaskObservationComplete {
		return nil
	}
	complete, err := a.deps.Product.ObserveTaskCompletion(ctx, row)
	if err != nil || !complete {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	_, err = a.deps.Store.Update(row.ID, func(current *registry.Session) error {
		if !sameTaskObservationCandidate(candidate, *current) || current.Status != registry.StatusExited ||
			current.WorktreeID != row.WorktreeID || product.ProjectRunSessionRequestDigest(*current) != product.ProjectRunSessionRequestDigest(row) {
			return product.ErrProjectRunConflict
		}
		current.TaskObservationComplete = true
		return nil
	})
	return err
}

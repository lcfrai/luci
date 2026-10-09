package api

import (
	"errors"
	"sync"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// attachedExitObserver owns at most one asynchronous registry observation. Its
// callback never performs disk I/O on the socket-copy goroutine. finish closes
// the callback gate and joins any accepted observation after both sockets have
// been closed; cancellation cannot abandon an already accepted terminal event.
type attachedExitObserver struct {
	mu     sync.Mutex
	closed bool
	done   chan struct{}
	commit func()
}

func (a *API) newAttachedExitObserver(observed registry.Session) *attachedExitObserver {
	observed = observed.Clone()
	return &attachedExitObserver{commit: func() { a.markAttachedSessionExited(observed) }}
}

func (o *attachedExitObserver) observe() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.done != nil {
		return
	}
	o.done = make(chan struct{})
	go func() {
		defer close(o.done)
		o.commit()
	}()
}

func (o *attachedExitObserver) finish() {
	o.mu.Lock()
	o.closed = true
	done := o.done
	o.mu.Unlock()
	if done != nil {
		<-done
	}
}

// A typed sandbox Exit is a process observation, not proof that Docker removed
// its allocation or that work succeeded. This only publishes terminal status;
// it never destroys a sandbox, frees a lease, or interprets command output.
func (a *API) markAttachedSessionExited(observed registry.Session) {
	if observed.ID == "" || observed.Backend == "" || observed.SandboxID == "" ||
		observed.SandboxRef == "" || observed.AttachToken == "" {
		return
	}
	_, err := a.deps.Store.Update(observed.ID, func(current *registry.Session) error {
		// Mutable timestamps/counts may advance during this same attachment.
		// The private allocation identity instead fences every same-ID restart
		// and scope change. A cleared token is unknown, never a matching epoch.
		if current.Status == registry.StatusExited || current.ID != observed.ID ||
			current.Backend != observed.Backend || current.SandboxID != observed.SandboxID ||
			current.SandboxRef != observed.SandboxRef || current.AttachToken != observed.AttachToken ||
			current.ProjectID != observed.ProjectID || current.WorktreeID != observed.WorktreeID ||
			current.BranchID != observed.BranchID || current.ProjectRunID != observed.ProjectRunID ||
			current.ProjectRunOperationID != observed.ProjectRunOperationID {
			return sessionErrMoved
		}
		current.Status = registry.StatusExited
		if current.ExitReason == "" {
			current.ExitReason = "process exited"
		}
		current.Endpoint, current.AttachToken = "", ""
		return nil
	})
	if err != nil && !errors.Is(err, sessionErrMoved) && !errors.Is(err, registry.ErrNotFound) {
		a.sessionLog().Warn("could not record terminal process completion", "sessionId", observed.ID, "err", err)
	}
}

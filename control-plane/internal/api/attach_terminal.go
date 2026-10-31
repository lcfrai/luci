package api

import (
	"context"
	"errors"
	"slices"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/proxy"
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
	commit func(proxy.ProcessExit)
}

func (a *API) newAttachedExitObserver(observed registry.Session) *attachedExitObserver {
	observed = observed.Clone()
	return &attachedExitObserver{commit: func(exit proxy.ProcessExit) { a.markAttachedSessionExitedWithExit(observed, exit) }}
}

func (o *attachedExitObserver) observe() {
	o.observeExit(proxy.ProcessExit{})
}

func (o *attachedExitObserver) observeExit(exit proxy.ProcessExit) {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.closed || o.done != nil {
		return
	}
	o.done = make(chan struct{})
	go func() {
		defer close(o.done)
		o.commit(exit)
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
	a.markAttachedSessionExitedWithExit(observed, proxy.ProcessExit{})
}

func (a *API) markAttachedSessionExitedWithExit(observed registry.Session, exit proxy.ProcessExit) {
	// The passive reader owns the stronger EOF eligibility fact. Keep desktop
	// terminal-adapter observations generic so its exact EOF upgrade cannot race
	// an earlier diagnostic-only Exit frame. Legacy sessions retain typed detail.
	if terminalSession(observed) {
		exit = proxy.ProcessExit{}
	}
	a.markSessionTerminalObservation(observed, exit, false, nil)
}

// Only the passive controller observer calls this after typed Exit AND normal
// stream EOF within its original absolute observation budget.
func (a *API) markTaskTerminalExitedWithEOF(ctx context.Context, observed registry.Session, exit proxy.ProcessExit) {
	a.markSessionTerminalObservation(observed, exit, true, func() bool {
		_, valid := taskTerminalObservationDeadline(observed.CreatedAt, time.Now())
		return valid && ctx.Err() == nil
	})
}

func (a *API) markSessionTerminalObservation(observed registry.Session, exit proxy.ProcessExit, terminalEOF bool, stillWithinBudget func() bool) {
	if observed.ID == "" || observed.Backend == "" || observed.SandboxID == "" ||
		observed.SandboxRef == "" || observed.AttachToken == "" {
		return
	}
	_, err := a.deps.Store.Update(observed.ID, func(current *registry.Session) error {
		if stillWithinBudget != nil && !stillWithinBudget() {
			return sessionErrMoved
		}
		if terminalEOF && terminalSession(observed) && !current.TaskTerminalObserverStarted {
			return sessionErrMoved
		}
		// Mutable timestamps/counts may advance during this same attachment.
		// The private allocation identity instead fences every same-ID restart
		// and scope change. Live rows still require the captured private token;
		// the terminal-only diagnostic exception below grants no execution power.
		if current.ID != observed.ID ||
			current.Backend != observed.Backend || current.SandboxID != observed.SandboxID ||
			current.SandboxRef != observed.SandboxRef ||
			current.ProjectID != observed.ProjectID || current.WorktreeID != observed.WorktreeID ||
			current.BranchID != observed.BranchID || current.ProjectRunID != observed.ProjectRunID ||
			current.ProjectRunOperationID != observed.ProjectRunOperationID {
			return sessionErrMoved
		}
		if terminalSession(observed) && !taskTerminalObservationMatches(*current, observed) {
			return sessionErrMoved
		}
		if current.Status == registry.StatusExited {
			// The reaper may have observed the same original allocation first.
			// Upgrade only its generic process observation, never a user stop,
			// custom failure or newer allocation. Terminal token clearing is not
			// permission to operate on the backend: this changes metadata only.
			if !exit.Known() || current.ExitReason != "process exited" || current.AttachToken != "" || !current.CreatedAt.Equal(observed.CreatedAt) {
				return sessionErrMoved
			}
			current.ExitReason = exit.Reason()
			if taskTerminalObservationMatches(*current, observed) {
				current.TaskTerminalExitZero = terminalEOF && exit.Successful()
			}
			return nil
		}
		if current.AttachToken != observed.AttachToken {
			return sessionErrMoved
		}
		if terminalSession(observed) && !taskTerminalObservationMatches(*current, observed) {
			return sessionErrMoved
		}
		current.TaskTerminalExitZero = terminalEOF && terminalSession(observed) && exit.Successful()
		current.Status = registry.StatusExited
		if current.ExitReason == "" {
			current.ExitReason = exit.Reason()
		}
		current.Endpoint, current.AttachToken = "", ""
		return nil
	})
	if err != nil && !errors.Is(err, sessionErrMoved) && !errors.Is(err, registry.ErrNotFound) {
		a.sessionLog().Warn("could not record terminal process completion", "sessionId", observed.ID, "err", err)
	}
}

// In addition to the existing allocation/token guards, retain the exact original
// command contract (which binds attempt/role/source), context and creation time.
// A delayed observer for a same-ID replacement cannot supply this private fact.
func taskTerminalObservationMatches(current, observed registry.Session) bool {
	return terminalSession(current) && terminalSession(observed) &&
		current.ID == observed.ID && current.ProjectID == observed.ProjectID && current.WorktreeID == observed.WorktreeID &&
		current.ProjectRunID == observed.ProjectRunID && current.ProjectRunOperationID == observed.ProjectRunOperationID &&
		current.Backend == observed.Backend && current.SandboxID == observed.SandboxID && current.SandboxRef == observed.SandboxRef &&
		current.CreatedAt.Equal(observed.CreatedAt) && current.Command == observed.Command && slices.Equal(current.Args, observed.Args) &&
		taskOutputImplementationDigest(current.ImplementationContext) == taskOutputImplementationDigest(observed.ImplementationContext) &&
		current.ReviewedContext != nil && observed.ReviewedContext != nil && *current.ReviewedContext == *observed.ReviewedContext
}

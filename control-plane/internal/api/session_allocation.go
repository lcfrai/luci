package api

import (
	"context"
	"errors"
	"net/http"
	"os"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// preparedSessionAllocation contains resolved internal values. Neither the
// scheduler nor this allocator constructs an HTTP request or borrows a user's
// old request context to create a child.
type preparedSessionAllocation struct {
	Record             registry.Session
	Spec               backend.Spec
	Account            *codexConnection
	Revalidate         func() error
	BeforeAllocate     func(context.Context) error
	BeforeBackendStart func(context.Context) error
	AfterBackendCreate func(context.Context, string) error
	Staging            string
	HasReviewedContext bool
	ExecutionLeaseID   string
}

type sessionAllocationError struct {
	Kind      string
	SessionID string
	Cause     error
}

func (e *sessionAllocationError) Error() string { return e.Cause.Error() }
func (e *sessionAllocationError) Unwrap() error { return e.Cause }

func (a *API) allocatePreparedSession(ctx context.Context, prepared preparedSessionAllocation) (registry.Session, error) {
	row, spec := prepared.Record, prepared.Spec
	failure := func(kind string, err error) (registry.Session, error) {
		return registry.Session{}, &sessionAllocationError{kind, row.ID, err}
	}
	if row.ID == "" || row.ID != spec.ID || row.ID != spec.SessionID || row.Status != registry.StatusStarting || row.SandboxID != row.ID {
		return failure("invalid", errors.New("The prepared session has no matching durable execution identity."))
	}
	if !sessionRestarts.acquire(row.ID) {
		return failure("changed", errors.New("This session is already starting, stopping or restarting."))
	}
	defer sessionRestarts.release(row.ID)
	if err := a.deps.Store.Create(row); err != nil {
		if prepared.HasReviewedContext {
			if cleanup := a.removeReviewedContext(row.ID); cleanup != nil {
				a.sessionLog().Error("remove unrecorded reviewed context", "sessionId", row.ID, "err", cleanup)
			}
		}
		if prepared.Staging != "" {
			if cleanup := os.RemoveAll(prepared.Staging); cleanup != nil {
				a.sessionLog().Error("remove unrecorded staged conversation", "sessionId", row.ID, "err", cleanup)
			}
		}
		return failure("record", err)
	}
	if err := a.leaseCodexConnection(prepared.Account, row.ID); err != nil {
		a.markSessionExited(row.ID, "the saved ChatGPT connection could not be reserved")
		return failure("account", err)
	}
	if prepared.Revalidate != nil {
		if err := prepared.Revalidate(); err != nil {
			a.markSessionExited(row.ID, "the reserved project run changed before sandbox allocation")
			return failure("changed", err)
		}
	}
	if prepared.BeforeAllocate != nil {
		if err := prepared.BeforeAllocate(ctx); err != nil {
			a.markSessionExited(row.ID, "execution access or capacity changed before sandbox allocation")
			return failure("changed", err)
		}
	}
	if prepared.ExecutionLeaseID != "" {
		release, err := a.execution.acquireLaunchLifecycle(ctx, prepared.ExecutionLeaseID, row)
		if err != nil {
			a.markSessionExited(row.ID, "the exact worker lifecycle could not be reserved")
			return failure("changed", err)
		}
		defer release()
	}
	if prepared.BeforeBackendStart != nil {
		if err := prepared.BeforeBackendStart(ctx); err != nil {
			a.markSessionExited(row.ID, "execution launch was retired or could not be recorded")
			return failure("changed", err)
		}
	}
	createCtx, cancel := a.sessionCreateContext(ctx)
	defer cancel()
	sb, err := a.deps.Backend.Create(createCtx, spec)
	if err != nil {
		a.sessionLog().Error("create the sandbox", "sessionId", row.ID, "err", err)
		a.destroyAfterFailedCreate(row.ID, row.ID)
		a.markSessionExited(row.ID, "the sandbox could not be created")
		return failure("backend", err)
	}
	if sb.ID != spec.ID {
		err = errors.New("The backend returned a different allocation identity; the expected child remains unconfirmed.")
	} else if prepared.AfterBackendCreate != nil {
		ackCtx, ackCancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = prepared.AfterBackendCreate(ackCtx, sb.ID)
		ackCancel()
	}
	if err != nil {
		// This identity was bound before dispatch. Never destroy an arbitrary
		// backend-returned ID, especially when a mismatched ID caused failure.
		a.destroyAfterFailedCreate(row.ID, row.ID)
		a.markSessionExited(row.ID, "execution launch acknowledgment could not be recorded")
		return failure("record-backend", err)
	}
	stored, err := a.deps.Store.Update(row.ID, func(value *registry.Session) error {
		if value.SandboxID != row.ID || value.Status != registry.StatusStarting {
			return sessionErrMoved
		}
		value.SandboxID, value.SandboxRef, value.Endpoint = sb.ID, sb.Ref, sb.Endpoint
		return nil
	})
	if err != nil {
		if errors.Is(err, sessionErrMoved) || errors.Is(err, registry.ErrNotFound) {
			a.destroyAbandonedSandbox(sb.ID, row.ID)
			return failure("moved", err)
		}
		return failure("record-backend", err)
	}
	go a.watchSessionReady(row.ID, sb.ID, sb.Endpoint)
	return stored, nil
}

func (a *API) writeSessionAllocationProblem(w http.ResponseWriter, r *http.Request, err error) {
	var failure *sessionAllocationError
	if !errors.As(err, &failure) {
		a.internal(w, r, "allocate session", err)
		return
	}
	switch failure.Kind {
	case "account":
		codexAccountProblem(w, failure.Cause)
	case "changed", "invalid":
		httpx.WriteProblem(w, 409, httpx.CodeConflict, failure.Error(), nil)
	case "backend":
		a.writeBackendProblem(w, r, "create the sandbox", failure.Cause)
	case "moved":
		if errors.Is(failure.Cause, registry.ErrNotFound) {
			httpx.WriteProblem(w, 404, httpx.CodeSessionNotFound, "No session with that id.", map[string]string{"id": failure.SessionID})
		} else {
			httpx.WriteProblem(w, 409, httpx.CodeConflict, "The session was ended while its sandbox was being created.", map[string]string{"id": failure.SessionID})
		}
	case "record":
		switch {
		case errors.Is(failure.Cause, registry.ErrInvalidID):
			httpx.WriteProblem(w, 400, httpx.CodeBadRequest, "The registry rejected the session id.", nil)
		case errors.Is(failure.Cause, registry.ErrExists):
			httpx.WriteProblem(w, 409, httpx.CodeConflict, "A session with that id already exists.", map[string]string{"id": failure.SessionID})
		default:
			a.internal(w, r, "record session "+failure.SessionID, failure.Cause)
		}
	default:
		a.internal(w, r, "record the sandbox for session "+failure.SessionID, failure.Cause)
	}
}

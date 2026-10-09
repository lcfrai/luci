package api

import (
	"context"
	"errors"

	"github.com/lcfrai/luci/control-plane/internal/product"
	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// The code is public, the cause is internal. In particular an authorization or
// filesystem error must never become the persisted operator-facing message.
type executionAdmissionError struct {
	code     string
	cause    error
	evidence registry.StartAdmissionEvidence
}

func (e *executionAdmissionError) Error() string { return e.cause.Error() }
func (e *executionAdmissionError) Unwrap() error { return e.cause }
func admissionFailure(code string, cause error) error {
	if cause == nil {
		cause = errors.New("execution admission refused")
	}
	return &executionAdmissionError{code: code, cause: cause}
}
func admissionDecisionFailure(decision executionDecision) error {
	return &executionAdmissionError{code: decision.Code, cause: errors.New(decision.Message), evidence: decision.Evidence}
}

func publicSessionStartFailure(row registry.Session) registry.SessionStartFailure {
	if row.Status == registry.StatusExited && row.SandboxRef == "" && row.StartFailure.Valid() {
		return row.StartFailure
	}
	return registry.SessionStartFailure{}
}

func (a *API) recordSessionStartFailure(row registry.Session, stage string, cause error) registry.SessionStartFailure {
	code := ""
	var admission *executionAdmissionError
	if errors.As(cause, &admission) {
		code = admission.code
	}
	diagnostic := registry.NewSessionStartFailure(stage, code)
	var evidence registry.StartAdmissionEvidence
	if admission != nil && admission.evidence.ValidFor(diagnostic) {
		evidence = admission.evidence
	}
	a.sessionLog().Warn("session refused before backend allocation", "sessionId", row.ID, "stage", stage, "reasonCode", diagnostic.ReasonCode, "err", cause)
	_, err := a.deps.Store.Update(row.ID, func(current *registry.Session) error {
		if current.Status != registry.StatusStarting || current.SandboxRef != "" || current.SandboxID != row.SandboxID || current.ProjectID != row.ProjectID || current.WorktreeID != row.WorktreeID || current.ProjectRunID != row.ProjectRunID || current.ProjectRunOperationID != row.ProjectRunOperationID || product.ProjectRunSessionRequestDigest(*current) != product.ProjectRunSessionRequestDigest(row) {
			return sessionErrMoved
		}
		current.Status, current.ExitReason, current.StartFailure = registry.StatusExited, diagnostic.Message, diagnostic
		current.StartAdmissionEvidence = evidence
		current.Endpoint, current.AttachToken = "", ""
		return nil
	})
	if err != nil {
		a.sessionLog().Error("record pre-allocation failure", "sessionId", row.ID, "err", err)
		return registry.SessionStartFailure{} // An unrecorded diagnostic cannot settle dispatch.
	}
	return diagnostic
}

// Keep cancellation classified, without exposing an arbitrary wrapped message.
func admissionContextFailure(ctx context.Context, err error) error {
	if ctx.Err() != nil {
		return admissionFailure("admission_unavailable", ctx.Err())
	}
	return admissionFailure("admission_unavailable", err)
}

// A returned backend error can follow an accepted Create/Start RPC. Bound run
// children use the existing explicit unknown-outcome classification so a later
// registry observation cannot mistake a failed acknowledgment for a failed
// worker result. No absence or successful settlement is inferred here.
func (a *API) markSessionAllocationUnconfirmed(row registry.Session, fallback string) {
	if row.ProjectRunID != "" && row.ProjectRunOperationID != "" {
		fallback = "project-run-outcome-unknown"
	}
	a.markSessionExited(row.ID, fallback)
}

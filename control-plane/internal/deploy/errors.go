package deploy

import (
	"errors"
	"time"
)

// Kind is what went wrong, in the terms the control plane's routes map to problem codes.
type Kind string

const (
	KindNotConnected Kind = "not_connected" // no credential, or no address, is configured
	KindUnavailable  Kind = "unavailable"   // transport failure, timeout, 5xx, 429
	KindUnauthorized Kind = "unauthorized"  // the credential was rejected
	KindForbidden    Kind = "forbidden"     // authenticated, not allowed
	KindNotFound     Kind = "not_found"
	KindConflict     Kind = "conflict" // the target refused because of its current state (Panacea: 409, and 402 from the deploy route's budget and risk gates)
	KindRejected     Kind = "rejected" // the target refused the request as malformed
)

// Error is every failure a Deployer returns.
type Error struct {
	Kind Kind
	// Message is the deploy target's own sentence, safe to render verbatim, at most 500 bytes.
	// It is "" for KindUnavailable: a 5xx sentence can carry internal detail and is only logged.
	// The connector trims it (task 02); Error() renders it as given.
	Message string
	// Status is the upstream HTTP status received: 0 for a transport failure, and, for an error
	// the connector builds from an unusable 2xx answer (an empty run name on a 202, a rollback
	// answering for another id, an empty env), that 2xx. Task 05 and task 13 read its class, so
	// it is never left 0 once an answer arrived (re-review M-4).
	Status     int
	RetryAfter time.Duration // from a 429 or 503, else 0
	Op         string        // "ship", "rollback", "environments", … for the log line
	Cause      error         // logged, never rendered
}

// Error is "deploy: <op>: <kind>[: <message>]". It never formats Cause: the cause can hold the
// request that failed, and so the credential, and Error() is what reaches a log line. The op
// segment is left out when Op is "".
func (e *Error) Error() string {
	if e == nil {
		return "deploy: <nil>"
	}
	s := "deploy: "
	if e.Op != "" {
		s += e.Op + ": "
	}
	s += string(e.Kind)
	if e.Message != "" {
		s += ": " + e.Message
	}
	return s
}

// Unwrap is Cause, so errors.Is and errors.As reach it.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// KindOf is the Kind of err's *Error, or KindUnavailable for any other non-nil error, or "".
// A non-nil error never reads as "": an *Error with no Kind, and a typed-nil *Error, are
// KindUnavailable too, so no caller mistakes a failure for success (declared in the t01 report).
func KindOf(err error) Kind {
	if err == nil {
		return ""
	}
	var e *Error
	if errors.As(err, &e) && e != nil && e.Kind != "" {
		return e.Kind
	}
	return KindUnavailable
}

// ErrNotConnected is returned by every method of a Deployer that has no credential.
var ErrNotConnected = &Error{Kind: KindNotConnected, Op: "connect"}

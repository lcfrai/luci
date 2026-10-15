package api

import (
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/registry"
)

// implementationOutcome is a bounded projection of the owned native session.
// It does not inspect assistant prose, assert a changed candidate, or turn a
// terminal exit into a passing check. Project-run evidence is read separately.
type implementationOutcome struct {
	State    string `json:"state"`
	Evidence string `json:"evidence"`
}

func implementationOutcomeFor(s registry.Session) *implementationOutcome {
	if s.ImplementationContext == nil || s.AuthOnly || s.Harness == "shell" {
		return nil
	}
	out := &implementationOutcome{Evidence: "session-only"}
	if s.ProjectRunID != "" && s.ProjectRunOperationID != "" {
		out.Evidence = "project-run"
	}
	switch s.Status {
	case registry.StatusStarting:
		out.State = "starting"
	case registry.StatusRunning:
		out.State = "working"
	case registry.StatusIdle:
		// Idle means only that no terminal bytes crossed the relay recently.
		// The native agent may still be working or awaiting Ask permission.
		out.State = "quiet"
	case registry.StatusExited:
		reason := strings.ToLower(strings.TrimSpace(s.ExitReason))
		switch {
		case !s.StartFailure.IsZero(), reason == "sandbox never became ready":
			out.State = "failed"
		case reason == "process exited":
			// The native relay observed the process terminal state. The task
			// result and any candidate source remain unverified.
			out.State = "ended"
		case reason == "killed":
			out.State = "stopped"
		default:
			// Exit also covers lost sandboxes, interrupted launches, and
			// resource stops. Do not offer a fresh dispatch from those facts.
			out.State = "unknown"
		}
	default:
		out.State = "unknown"
	}
	return out
}

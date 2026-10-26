package main

import (
	"context"
	"errors"

	"github.com/lcfrai/luci/control-plane/internal/resourcebudget"
)

const backendInitializationUnknown = "The execution runtime could not be initialized (execution_runtime_initialization_unavailable)."

// A constructor may retain private transport errors for existing error handling.
// Only these fixed stages may reach a public cached capacity observation.
func safeBackendInitializationReason(err error) string {
	var diagnostic interface{ DockerInitializationStage() string }
	if errors.As(err, &diagnostic) {
		switch diagnostic.DockerInitializationStage() {
		case "docker_client_configuration_unavailable":
			return "Docker connection configuration could not be initialized. Review the configured connection (docker_client_configuration_unavailable)."
		case "docker_initial_ping_unavailable":
			return "Docker did not answer its initial connection check. Start Docker Desktop if it is stopped, then wait for current capacity observations (docker_initial_ping_unavailable)."
		}
	}
	return backendInitializationUnknown
}

func (b *recoveringBackend) ObserveResources(ctx context.Context) resourcebudget.ExecutionSnapshot {
	value, err := b.get()
	if err == nil {
		if observer, ok := value.(resourcebudget.ExecutionObserver); ok {
			return observer.ObserveResources(ctx)
		}
		return unavailableExecutionObservation("The execution runtime does not support resource observations (execution_resource_observer_unsupported).")
	}
	b.mu.Lock()
	reason := b.initializationReason
	if b.closed {
		reason = "The execution runtime is closed (execution_runtime_closed)."
	}
	b.mu.Unlock()
	if reason == "" {
		reason = backendInitializationUnknown
	}
	return unavailableExecutionObservation(reason)
}

func unavailableExecutionObservation(reason string) resourcebudget.ExecutionSnapshot {
	return resourcebudget.ExecutionSnapshot{Status: "unavailable", Scope: "execution-daemon", Unavailable: []resourcebudget.Issue{{Metric: "execution", Reason: reason}}}
}

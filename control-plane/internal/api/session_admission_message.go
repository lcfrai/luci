package api

// A manual Create or Restart has no durable scheduler queue entry to retain.
// Keep the shared decision unchanged for real queued dispatches.
func manualSessionAdmissionMessage(decision executionDecision) string {
	if decision.Code == "resource_observation_unavailable" {
		return "Current machine capacity is not available. No new worker was started. Refresh workspace capacity before trying again."
	}
	return decision.Message
}

// Revalidation occurs after preparation but before the backend allocation.
// Contextualize a value copy; durable dispatch still uses the shared decision.
func manualSessionAdmissionFailure(decision executionDecision) error {
	decision.Message = manualSessionAdmissionMessage(decision)
	return admissionDecisionFailure(decision)
}

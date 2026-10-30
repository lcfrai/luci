package product

// Both the ordinary single-item control and the reviewed portfolio action use
// this exact mutation. It neither persists nor wakes execution; its caller
// supplies current authority, the scheduler mutex and the durable ledger CAS.
func setSwarmQueuePriority(item *swarmQueueRecord, expectedRevision int, priority, intentDigest string) error {
	if item.Revision != expectedRevision || swarmQueueActive(item.State) || swarmQueueTerminal(item.State) {
		return ErrProjectRunConflict
	}
	if !swarmQueuePriority(priority) || intentDigest != "" && !hexDigest(intentDigest) {
		return ErrProjectRunInvalid
	}
	item.Priority = priority
	// A later ordinary priority write clears an earlier portfolio witness even
	// if the value happens to match. Matching priority alone proves no ownership.
	item.PriorityChangeDigest = intentDigest
	return nil
}

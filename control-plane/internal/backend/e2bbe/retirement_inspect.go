package e2bbe

// InspectSettledJournal verifies complete durable ownership/attempt evidence
// without opening a controller, claiming a writer or reconciling provider state.
// Provider inventory and account-service quiescence are separate prerequisites.
func InspectSettledJournal(data []byte, owner, scope string) error {
	journal, err := validateJournal(data, owner, scope)
	if err != nil {
		return err
	}
	for _, row := range journal.Records {
		if row.State != Absent || row.ResourcesHeld {
			return ErrUnknown
		}
	}
	return nil
}

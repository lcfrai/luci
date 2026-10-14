package api

import "errors"

// AzureRetirementLedgerSettled reuses the authoritative capacity decoder.
// It is read-only and never releases a lease or substitutes VM absence for
// retained ownership. The caller must separately quiesce the account service.
func AzureRetirementLedgerSettled(data []byte) error {
	ledger, err := decodeExecutionLedger(data)
	if err != nil {
		return err
	}
	if len(ledger.Leases) != 0 {
		return errors.New("execution reservations remain held")
	}
	for _, row := range ledger.Checks {
		if row.State != "finished" {
			return errors.New("check reservation remains held")
		}
	}
	for _, row := range ledger.Previews {
		if row.State != "finished" {
			return errors.New("preview reservation remains held")
		}
	}
	for _, row := range ledger.RuntimeImages {
		if row.State != "finished" {
			return errors.New("runtime image reservation remains held")
		}
	}
	return nil
}

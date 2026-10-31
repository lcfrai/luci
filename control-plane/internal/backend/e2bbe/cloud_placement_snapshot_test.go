package e2bbe

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestPlacementNegativeEvidenceRequiresOpenAcknowledgedOriginalJournal(t *testing.T) {
	controller := &Controller{journal: journal{Version: 1, OwnerID: "controller-one", Scope: "workspace-one", Records: map[string]Record{}}}
	if _, err := controller.OwnershipSnapshot(); !errors.Is(err, ErrUnknown) {
		t.Fatal("uncheckpointed memory certified original absence")
	}
	body, _ := json.Marshal(controller.journal)
	controller.checkpointDigest = bytesDigest(body)
	if rows, err := controller.OwnershipSnapshot(); err != nil || len(rows) != 0 {
		t.Fatal("acknowledged local original journal could not be inspected", err)
	}
	controller.closed = true
	if _, err := controller.OwnershipSnapshot(); !errors.Is(err, ErrUnknown) {
		t.Fatal("closed host certified original absence")
	}
	controller.closed = false
	controller.journal.Scope = "changed"
	if _, err := controller.OwnershipSnapshot(); !errors.Is(err, ErrUnknown) {
		t.Fatal("changed unacknowledged journal certified original absence")
	}
}

package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDocumentSharedLedgerRetainsExactOwnershipAndJointBound(t *testing.T) {
	lease := documentCapacityFixture()
	ledger := executionLeaseLedger{Version: 1, Documents: []documentExtractionLease{lease}}
	encoded, err := json.Marshal(ledger)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := decodeExecutionLedger(encoded)
	if err != nil || len(decoded.Documents) != 1 || decoded.Documents[0].Identity != lease.Identity {
		t.Fatalf("document ownership lost: %+v %v", decoded.Documents, err)
	}
	ledger.Documents = append(ledger.Documents, lease)
	encoded, _ = json.Marshal(ledger)
	if _, err := decodeExecutionLedger(encoded); err == nil {
		t.Fatal("duplicate document ownership accepted")
	}
	ledger.Documents = ledger.Documents[:1]
	ledger.DocumentFences = []documentExtractionFenceRecord{{Scope: strings.Repeat("f", 64)}}
	encoded, _ = json.Marshal(ledger)
	if _, err := decodeExecutionLedger(encoded); err == nil {
		t.Fatal("invalid document fence accepted")
	}
	ledger.DocumentFences = nil
	ledger.Documents = make([]documentExtractionLease, 4097)
	encoded, _ = json.Marshal(ledger)
	if _, err := decodeExecutionLedger(encoded); err == nil {
		t.Fatal("joint history bound accepted oversized documents")
	}
}

package api

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOrchestratorStrictSchemasExposeStructuredGTMWithoutNewExecutionFields(t *testing.T) {
	for _, native := range []bool{false, true} {
		raw, err := orchestratorResponseSchema(native)
		if err != nil || !json.Valid(raw) {
			t.Fatal("existing strict schema generator rejected GTM", native, err)
		}
		for _, field := range []string{`"gtm"`, `"positioning"`, `"experiments"`, `"commercialModel"`, `"artifactLinks"`} {
			if !strings.Contains(string(raw), field) {
				t.Fatal("missing structured field", native, field)
			}
		}
		if strings.Contains(string(raw), `"publishCampaign"`) || strings.Contains(string(raw), `"applyAutomatically"`) {
			t.Fatal("GTM schema acquired execution fields")
		}
	}
}

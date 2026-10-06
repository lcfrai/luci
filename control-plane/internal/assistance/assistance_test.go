package assistance

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestAssistantOutputRejectsMalformedOversizedAndTrailingModelText(t *testing.T) {
	valid := `{"question":"Who approves?","options":["Manager","Customer"],"rationale":"Approval affects access.","proposal":null}`
	got, err := DecodeResult([]byte(valid))
	if err != nil || got.Question != "Who approves?" {
		t.Fatal("valid structured response was lost")
	}
	for _, input := range []string{valid + ` {}`, strings.Repeat("x", MaxResultBytes+1), `{"question":"","options":[],"rationale":"","proposal":null}`, `{"question":"Why?","options":[],"rationale":"","proposal":null,"secret":"unrequested"}`} {
		if _, err := DecodeResult([]byte(input)); err == nil {
			t.Fatal("malformed model output was accepted")
		}
	}
}

func TestAssistantSchemaKeepsNestedContractFieldsRequired(t *testing.T) {
	type item struct {
		ID    string   `json:"id"`
		Links []string `json:"links"`
	}
	type specification struct {
		Revision     int    `json:"revision"`
		Requirements []item `json:"requirements"`
	}
	data, err := Schema(specification{})
	if err != nil {
		t.Fatal(err)
	}
	var decoded map[string]any
	if json.Unmarshal(data, &decoded) != nil {
		t.Fatal("schema is not JSON")
	}
	props := decoded["properties"].(map[string]any)
	proposal := props["proposal"].(map[string]any)["anyOf"].([]any)[0].(map[string]any)
	if proposal["additionalProperties"] != false || len(proposal["required"].([]any)) != 2 {
		t.Fatal("proposal contract fields are not strict")
	}
	requirements := proposal["properties"].(map[string]any)["requirements"].(map[string]any)["items"].(map[string]any)
	if requirements["additionalProperties"] != false || len(requirements["required"].([]any)) != 2 {
		t.Fatal("nested specification contract is not strict")
	}
}

func TestStructuredAssistantSchemaRequiresNullableProposalFields(t *testing.T) {
	type document struct {
		ProjectID string `json:"projectId"`
		Revision  int    `json:"revision"`
	}
	type proposal struct {
		Target        string    `json:"target"`
		Specification *document `json:"specification"`
		Design        *document `json:"designPlan"`
	}
	type answer struct {
		Summary  string    `json:"summary"`
		Blocking bool      `json:"blocking"`
		Proposal *proposal `json:"proposal"`
	}
	data, err := StrictSchema(answer{})
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	if json.Unmarshal(data, &schema) != nil {
		t.Fatal("invalid JSON schema")
	}
	props := schema["properties"].(map[string]any)
	choices := props["proposal"].(map[string]any)["anyOf"].([]any)
	if len(choices) != 2 || choices[1].(map[string]any)["type"] != "null" {
		t.Fatal("proposal cannot explicitly decline an amendment")
	}
	object := choices[0].(map[string]any)
	if object["additionalProperties"] != false || len(object["required"].([]any)) != 3 {
		t.Fatal("proposal permits undeclared document or missing provenance")
	}
	field := object["properties"].(map[string]any)["specification"].(map[string]any)["anyOf"].([]any)[0].(map[string]any)
	if field["additionalProperties"] != false || len(field["required"].([]any)) != 2 {
		t.Fatal("nullable document lost strict provenance")
	}
	if _, err = StrictSchema(nil); err == nil {
		t.Fatal("untyped schema accepted")
	}
}

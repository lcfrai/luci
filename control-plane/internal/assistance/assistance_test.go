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

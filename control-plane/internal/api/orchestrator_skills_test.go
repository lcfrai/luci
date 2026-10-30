package api

import (
	"strings"
	"testing"
)

func TestOrchestratorSchemaAndPromptKeepSkillRecommendationsSeparateFromPlanApply(t *testing.T) {
	for _, local := range []bool{false, true} {
		schema, err := orchestratorResponseSchema(local)
		if err != nil || !strings.Contains(string(schema), "suggestedSkillAssignments") || !strings.Contains(string(schema), "skillDigest") {
			t.Fatal("shared structured provider schema lost recommendations", local, err)
		}
	}
	for _, part := range []string{"saved roadmap outcomes", "exact catalogue skill ID/revision/digest", "saved separately by an explicit person action", "Do not invent durations"} {
		if !strings.Contains(orchestratorInstructions, part) {
			t.Fatal("prompt lost grounded decomposition/separate-save boundary", part)
		}
	}
}

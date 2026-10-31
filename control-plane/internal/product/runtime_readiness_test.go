package product

import (
	"encoding/json"
	"github.com/lcfrai/luci/control-plane/internal/backend"
	"strings"
	"testing"
)

func runtimeReadinessFixture() string {
	data, _ := json.Marshal(runtimeReadinessIdentity{ID: "sha256:" + strings.Repeat("a", 64), OS: "linux", Architecture: "amd64", Result: "v1", Schema: backend.CodexTaskResultSchemaDigest, NormalZero: "v1", Harnesses: backend.HarnessTaskResultVersion})
	return string(data)
}
func TestRuntimeReadinessRequiresCurrentTaskContracts(t *testing.T) {
	var current runtimeReadinessIdentity
	_ = json.Unmarshal([]byte(runtimeReadinessFixture()), &current)
	if got := runtimeReadinessCheck(runtimeReadinessFixture()); got.Status != "ready" || got.PreparationMode != "" {
		t.Fatal("current image was not recognized", got)
	}
	for _, tc := range []struct {
		name   string
		change func(*runtimeReadinessIdentity)
	}{
		{"legacy", func(v *runtimeReadinessIdentity) { v.Result = ""; v.Schema = ""; v.NormalZero = ""; v.Harnesses = "" }},
		{"schema", func(v *runtimeReadinessIdentity) { v.Schema = strings.Repeat("b", 64) }},
		{"normal-zero", func(v *runtimeReadinessIdentity) { v.NormalZero = "" }},
		{"harnesses", func(v *runtimeReadinessIdentity) { v.Harnesses = "" }},
		{"platform", func(v *runtimeReadinessIdentity) { v.Architecture = "arm64" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			row := current
			tc.change(&row)
			raw, _ := json.Marshal(row)
			got := runtimeReadinessCheck(string(raw))
			if got.Status != "missing" || got.PreparationMode != "advanced-build" || got.Action == "" {
				t.Fatal("old runtime was advertised ready", got)
			}
		})
	}
}
func TestRuntimeReadinessDoesNotUpgradeUnknownIdentity(t *testing.T) {
	for _, raw := range []string{"", "sha256:" + strings.Repeat("a", 64), runtimeReadinessFixture() + "{}", strings.Repeat("x", 4097), strings.Replace(runtimeReadinessFixture(), "sha256:", "unknown:", 1)} {
		got := runtimeReadinessCheck(raw)
		if got.Status != "missing" || got.PreparationMode != "" {
			t.Fatal("unknown identity requested a build", got)
		}
	}
	if strings.Contains(runtimeReadinessFormat, ".Config.Env") || strings.Contains(runtimeReadinessFormat, "json .Config.Labels") {
		t.Fatal("readiness may only project fixed public capability labels")
	}
}

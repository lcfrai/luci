package deploymentplan

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestOwnedModelAccessGroupIsExplicitAndLegacyOmissionIsUnchanged(t *testing.T) {
	raw, err := json.Marshal(ModelReference{Name: "old", ClaimName: "existing", MountPath: "/models"})
	if err != nil || strings.Contains(string(raw), "accessGroup") {
		t.Fatal("legacy serialization changed")
	}
	model := ModelReference{Name: "new", ClaimName: "owned", MountPath: "/model", AccessGroup: 1000}
	raw, err = json.Marshal(model)
	if err != nil || !strings.Contains(string(raw), "\"accessGroup\":1000") {
		t.Fatal("fixed reviewed access group omitted")
	}
	service := Service{ID: "model", Kind: "inference", Model: &model}
	if err = validateServiceRuntime(&service, "model", func(string, string, string) {}); err != nil {
		t.Fatal(err)
	}
	// Full blueprint model validation closes all other values; the helper always
	// creates0750 directories/0640 files owned by1000:1000, never world-readable.
	fixture := exampleBlueprint()
	fixture.Services[1].Model = &model
	plan, err := Compile(fixture)
	if err != nil {
		t.Fatal(err)
	}
	matched := false
	for _, a := range plan.Artifacts {
		if strings.HasSuffix(a.Name, ".deployment.json") && strings.Contains(a.Content, "\"fsGroup\": 1000") {
			matched = true
		}
	}
	if !matched {
		t.Fatal("consumer did not join fixed model reader group")
	}
	model.AccessGroup = 2000
	fixture.Services[1].Model = &model
	if _, err = Compile(fixture); err == nil {
		t.Fatal("unreviewed access group accepted")
	}
}

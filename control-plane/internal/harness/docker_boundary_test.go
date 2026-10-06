package harness_test

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/harness"
)

func TestCodexAskUsesOnlyBackendConfirmedDockerBoundary(t *testing.T) {
	for _, docker := range []bool{false, true} {
		_, args, err := harness.Build(harness.Request{Harness: harness.Codex, Permission: harness.PermissionAsk, DockerBoundary: docker})
		if err != nil {
			t.Fatal(err)
		}
		want := "workspace-write"
		if docker {
			want = "danger-full-access"
		}
		if !slices.Contains(args, want) || !slices.Contains(args, "on-request") || slices.Contains(args, "--dangerously-bypass-approvals-and-sandbox") {
			t.Fatal("Ask lost approval policy or selected wrong execution boundary")
		}
	}
	for _, mode := range []harness.PermissionMode{harness.PermissionPlan, harness.PermissionBypass} {
		_, host, err := harness.Build(harness.Request{Harness: harness.Codex, Permission: mode})
		if err != nil {
			t.Fatal(err)
		}
		_, docker, err := harness.Build(harness.Request{Harness: harness.Codex, Permission: mode, DockerBoundary: true})
		if err != nil {
			t.Fatal(err)
		}
		if !slices.Equal(host, docker) {
			t.Fatal("Docker Ask seam changed another permission mode")
		}
	}
}

func TestHarnessCataloguePublicCustomModelAndDockerAskNoticeReflectValidation(t *testing.T) {
	for _, d := range harness.Catalogue() {
		if d.Available() && d.SupportsCustomModel {
			if _, _, err := harness.Build(harness.Request{Harness: d.ID, Model: "saved-gateway-alias"}); err != nil {
				t.Fatal("public custom-model capability disagrees with Build")
			}
		}
		if d.ID == harness.Shell && d.SupportsCustomModel {
			t.Fatal("shell advertised model routing")
		}
		if d.ID == harness.Codex && (!strings.Contains(d.DockerAskNotice, "on-request") || !strings.Contains(d.DockerAskNotice, "container")) {
			t.Fatal("Docker approval boundary not disclosed")
		}
	}
	data, err := json.Marshal(harness.Catalogue())
	if err != nil || !strings.Contains(string(data), `"supportsCustomModel":true`) || !strings.Contains(string(data), `"dockerAskNotice"`) {
		t.Fatal("public descriptor missing supported capability")
	}
}

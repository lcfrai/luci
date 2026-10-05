package product

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// Some Docker clients return success with an empty formatted response when the
// selected engine cannot answer. A process exit alone must never start onboarding
// installation or advertise a usable runtime.
func TestSetupReadinessRequiresDockerEngineAndImageEvidence(t *testing.T) {
	for _, tc := range []struct {
		name, engine, image   string
		wantEngine, wantImage string
	}{
		{"empty-engine-and-image", "", "", "missing", "missing"},
		{"whitespace-engine-and-image", " \r\n", "\t\n", "missing", "missing"},
		{"engine-without-image", "28.3.3\n", "", "ready", "missing"},
		{"verified-engine-and-image", "28.3.3\n", "sha256:" + strings.Repeat("a", 64) + "\n", "ready", "ready"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			builds := 0
			s, mux := cloudTestServer(t, func(_ context.Context, exe string, args []string, _ string) (string, error) {
				if exe == "docker" {
					switch args[0] {
					case "info":
						return tc.engine, nil
					case "image":
						return tc.image, nil
					case "build":
						builds++
						t.Fatal("readiness checking started a runtime build")
					}
				}
				return "installed-and-signed-in", nil
			})
			w := cloudRequest(mux, http.MethodPost, "/v1/setup/check", "{}")
			var status Status
			if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &status) != nil {
				t.Fatalf("readiness response: %d %s", w.Code, w.Body)
			}
			for _, want := range []struct{ id, status string }{{"docker", tc.wantEngine}, {"sandbox-image", tc.wantImage}} {
				found := false
				for _, check := range status.Checks {
					if check.ID != want.id {
						continue
					}
					found = true
					if check.Status != want.status || strings.TrimSpace(check.Message) == "" || (want.status == "missing" && check.Action == "") {
						t.Fatalf("%s readiness lost proof or recovery: %#v", want.id, check)
					}
				}
				if !found {
					t.Fatalf("missing %s check", want.id)
				}
			}
			w = cloudRequest(mux, http.MethodGet, "/v1/setup/runtime/install", "")
			var job RuntimeJob
			if w.Code != http.StatusOK || json.Unmarshal(w.Body.Bytes(), &job) != nil || job.Status != "idle" || job.ID != "" || builds != 0 {
				t.Fatalf("readiness created a runtime operation: %d %s", w.Code, w.Body)
			}
			if len(s.status().Checks) != len(status.Checks) {
				t.Fatal("readiness was not persisted for the UI")
			}
		})
	}
}

package api

import (
	"bytes"
	"encoding/json"
	"github.com/lcfrai/luci/control-plane/internal/registry"
	"testing"
)

func TestSessionAttachReadinessProjection(t *testing.T) {
	for _, status := range []registry.Status{registry.StatusStarting, registry.StatusRunning, registry.StatusIdle, registry.StatusExited, "unknown"} {
		for _, endpoint := range []bool{false, true} {
			name := string(status) + "/without-endpoint"
			if endpoint {
				name = string(status) + "/with-endpoint"
			}
			t.Run(name, func(t *testing.T) {
				s := executionGenerationFixture()
				s.Status, s.Endpoint = status, ""
				if endpoint {
					s.Endpoint = "127.0.0.1:49200"
				}
				raw, err := json.Marshal(toJSON(s, sessionBasePath))
				if err != nil {
					t.Fatal(err)
				}
				fields := attachReadinessObject(t, raw, s)
				if endpoint && (status == registry.StatusStarting || status == registry.StatusRunning || status == registry.StatusIdle) {
					requireAttachReadiness(t, fields, true)
				} else if _, present := fields["attachReady"]; present {
					t.Fatal("untracked or terminal endpoint availability must be omitted")
				}
			})
		}
	}
}

func TestSessionAttachReadinessOmitsUnknownLegacyIdentity(t *testing.T) {
	for name, change := range map[string]func(*registry.Session){
		"missing-token":      func(s *registry.Session) { s.AttachToken = "" },
		"legacy-token":       func(s *registry.Session) { s.AttachToken = "legacy-private" },
		"invalid-token":      func(s *registry.Session) { s.AttachToken = string(bytes.Repeat([]byte{'!'}, 43)) },
		"missing-allocation": func(s *registry.Session) { s.SandboxID = "" },
		"missing-session":    func(s *registry.Session) { s.ID = "" },
		"cleaned-terminal":   func(s *registry.Session) { s.Status = registry.StatusExited; s.AttachToken = ""; s.Endpoint = "" },
	} {
		t.Run(name, func(t *testing.T) {
			s := executionGenerationFixture()
			change(&s)
			raw, err := json.Marshal(toJSON(s, sessionBasePath))
			if err != nil {
				t.Fatal(err)
			}
			fields := attachReadinessObject(t, raw, s)
			if _, exists := fields["attachReady"]; exists {
				t.Fatal("unknown allocation readiness must be omitted")
			}
			if _, exists := fields["executionGeneration"]; exists {
				t.Fatal("fixture unexpectedly has a known allocation generation")
			}
		})
	}
}

package api

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/backend"
	"github.com/lcfrai/luci/control-plane/internal/product"
)

func saveAssistanceGateway(t *testing.T, e *sessionsEnv) product.AgentConnection {
	t.Helper()
	request, _ := json.Marshal(map[string]any{
		"action": "save", "expectedRevision": 0, "name": "Configured test gateway", "kind": "litellm", "enabled": true,
		"endpoints": map[string]string{"responses": "https://gateway.example/v1", "anthropicMessages": "https://gateway.example", "chatCompletions": "https://gateway.example/v1"},
		"models":    map[string]string{"codex": "planner-alias", "claudeCode": "claude-planner-alias", "opencode": "planner-alias"},
		"apiKey":    "fixture-gateway-private-key",
	})
	w := e.do(t, "POST", "/v1/setup/agent-connections", string(request))
	var response struct{ Connections []product.AgentConnection }
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &response) != nil || len(response.Connections) != 1 {
		t.Fatal("Could not save the isolated gateway fixture")
	}
	return response.Connections[0]
}

func TestGuidedInterviewUsesExactGatewayForEachHarnessWithoutSubscriptionOrSourceMounts(t *testing.T) {
	for _, provider := range []string{"codex", "claude-code", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			e, p := assistanceFixture(t)
			route := saveAssistanceGateway(t, e)
			account, _ := e.api.readCodexConnection()
			started := make(chan backend.Spec, 1)
			e.be.onCreate = func(spec backend.Spec) {
				started <- spec
				assistOutput(t, spec, []byte(`{"question":"Who can approve?","options":[],"rationale":"Clarify permission.","proposal":null}`))
			}
			request, _ := json.Marshal(map[string]any{"expectedRevision": 0, "message": "Help define an approval tool", "harness": provider, "agentConnectionId": route.ID})
			job := decodeAssist(t, e, "POST", "/v1/product/projects/"+p.ID+"/assist", string(request), 202)
			spec := <-started
			if err := backend.ValidateSpec(spec); err != nil {
				t.Fatal("Selected provider failed actual sandbox admission")
			}
			if job.Harness != provider || job.AgentConnectionID != route.ID || job.AgentConnectionRevision != route.Revision || job.AgentConnectionKind != "litellm" {
				t.Fatal("Interview lost its explicit connection provenance")
			}
			for _, mount := range spec.Mounts {
				rel, _ := filepath.Rel(e.dir, mount.HostPath)
				if rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					t.Fatal("Interview mounted project source")
				}
				if mount.ContainerPath == "/luci-input" {
					for _, name := range []string{"prompt.txt", "selection.json"} {
						data, err := os.ReadFile(filepath.Join(mount.HostPath, name))
						if err != nil || strings.Contains(string(data), "fixture-gateway-private-key") {
							t.Fatal("Input missing or credential leaked into input")
						}
					}
				}
			}
			if _, err := os.Stat(filepath.Join(nativeStateMount(t, spec), "auth.json")); !os.IsNotExist(err) {
				t.Fatal("Gateway job inherited subscription authentication")
			}
			switch provider {
			case "codex":
				if spec.Env["OPENAI_BASE_URL"] != "https://gateway.example/v1" || spec.Env["ANTHROPIC_AUTH_TOKEN"] != "" {
					t.Fatal("Codex route was changed or mixed")
				}
			case "claude-code":
				if spec.Env["ANTHROPIC_AUTH_TOKEN"] != "fixture-gateway-private-key" || spec.Env["OPENAI_API_KEY"] != "" {
					t.Fatal("Claude bearer route was changed or mixed")
				}
			case "opencode":
				if job.Model != "luci-gateway/planner-alias" || spec.Env["OPENCODE_PERMISSION"] != `{"*":"deny"}` {
					t.Fatal("OpenCode model or tool boundary changed")
				}
			}
			got := awaitAssist(t, e, job)
			if got.Status != "succeeded" || got.Result == nil {
				t.Fatal("Configured interview did not retain its structured result")
			}
			current, _ := e.api.readCodexConnection()
			if current.ID != account.ID || current.ActiveSessionID != account.ActiveSessionID || string(current.Auth) != string(account.Auth) {
				t.Fatal("Gateway job changed the saved subscription")
			}
			public, _ := json.Marshal(got)
			if strings.Contains(string(public), "fixture-gateway-private-key") {
				t.Fatal("Public job contains its credential")
			}
		})
	}
}

func TestExplicitMissingInterviewConnectionNeverUsesSavedSubscription(t *testing.T) {
	e, p := assistanceFixture(t)
	e.be.mu.Lock()
	before := len(e.be.specs)
	e.be.mu.Unlock()
	request := `{"expectedRevision":0,"message":"Clarify my application","harness":"codex","agentConnectionId":"agent-aaaaaaaaaaaaaaaaaaaaaaaa"}`
	w := e.do(t, "POST", "/v1/product/projects/"+p.ID+"/assist", request)
	e.be.mu.Lock()
	after := len(e.be.specs)
	e.be.mu.Unlock()
	if w.Code != 409 || after != before {
		t.Fatal("Missing selected route allocated a sandbox or used another account")
	}
}

func TestInterviewSavedModelMismatchRefusesBeforeAllocatingAnyProvider(t *testing.T) {
	for _, provider := range []string{"codex", "claude-code", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			e, p := assistanceFixture(t)
			route := saveAssistanceGateway(t, e)
			e.be.mu.Lock()
			before := len(e.be.specs)
			e.be.mu.Unlock()
			request, _ := json.Marshal(map[string]any{"expectedRevision": 0, "message": "Clarify the application", "harness": provider, "model": "unselected-expensive-model", "agentConnectionId": route.ID})
			w := e.do(t, "POST", "/v1/product/projects/"+p.ID+"/assist", string(request))
			e.be.mu.Lock()
			after := len(e.be.specs)
			e.be.mu.Unlock()
			if w.Code != 409 || after != before {
				t.Fatal("A saved model mismatch allocated another provider")
			}
		})
	}
}

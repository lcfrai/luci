package product

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const connectionFixtureKey = "fixture-gateway-key-not-a-live-credential-12345"

type connectionCatalogue struct {
	Revision           int               `json:"revision"`
	ActiveConnectionID string            `json:"activeConnectionId"`
	Connections        []AgentConnection `json:"connections"`
}

func connectionRead(t *testing.T, mux http.Handler) connectionCatalogue {
	t.Helper()
	w := journeyRequest(t, mux, "GET", "/v1/setup/agent-connections", nil, 200)
	if strings.Contains(w.Body.String(), connectionFixtureKey) || strings.Contains(w.Body.String(), `"apiKey"`) {
		t.Fatal("public connection catalogue exposed private key material")
	}
	var result connectionCatalogue
	if err := json.Unmarshal(w.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	return result
}

func connectionSaveBody(revision int) map[string]any {
	return map[string]any{"action": "save", "expectedRevision": revision, "name": "Personal gateway", "kind": "litellm", "enabled": true,
		"endpoints": AgentConnectionEndpoints{Responses: "https://gateway.example.test/v1", AnthropicMessages: "https://gateway.example.test", ChatCompletions: "http://localhost:11434/v1"},
		"models":    AgentConnectionModels{Codex: "codex-alias", ClaudeCode: "claude-alias", OpenCode: "local-alias"}, "apiKey": connectionFixtureKey}
}

func connectionSave(t *testing.T, mux http.Handler, body map[string]any) connectionCatalogue {
	t.Helper()
	w := journeyRequest(t, mux, "POST", "/v1/setup/agent-connections", body, 200)
	if strings.Contains(w.Body.String(), connectionFixtureKey) {
		t.Fatal("save exposed private credential")
	}
	return connectionRead(t, mux)
}

func TestPersonalProviderStatusRemainsIndependentOfSelectedGateway(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	readPersonal := func(want bool) {
		t.Helper()
		w := journeyRequest(t, mux, "GET", "/v1/setup/agents", nil, 200)
		if strings.Contains(w.Body.String(), connectionFixtureKey) || strings.Contains(w.Body.String(), credentialFixtureKey) {
			t.Fatal("personal provider status exposed private credentials")
		}
		var response struct {
			Providers []AgentProvider `json:"providers"`
		}
		if json.Unmarshal(w.Body.Bytes(), &response) != nil || len(response.Providers) != 3 {
			t.Fatal("personal provider catalogue is incomplete")
		}
		for i, id := range []string{"anthropic", "openai", "local"} {
			if response.Providers[i].ID != id || response.Providers[i].Configured != want {
				t.Fatalf("personal provider %s status does not reflect its own saved credentials", id)
			}
		}
	}
	readPersonal(false)
	got := connectionSave(t, mux, connectionSaveBody(0))
	id := got.Connections[0].ID
	connectionSave(t, mux, map[string]any{"action": "select", "expectedRevision": 1, "id": id})
	for _, h := range []string{"codex", "claude-code", "opencode"} {
		if receipt, env, err := s.ResolveAgentConnection(h, nil); err != nil || receipt.ID != id || len(env) == 0 || len(s.AgentEnvironment(h)) == 0 {
			t.Fatal("selected gateway is not available to runtime consumers")
		}
	}
	readPersonal(false)
	if _, err := os.Stat(filepath.Join(s.stateDir, "agent-credentials.json")); !os.IsNotExist(err) {
		t.Fatal("gateway selection created personal credentials")
	}
	for _, provider := range []string{"anthropic", "openai"} {
		w := journeyRequest(t, mux, "POST", "/v1/setup/agents/"+provider, map[string]string{"apiKey": credentialFixtureKey}, 200)
		if strings.Contains(w.Body.String(), credentialFixtureKey) || strings.Contains(w.Body.String(), connectionFixtureKey) {
			t.Fatal("personal key save exposed a credential")
		}
	}
	journeyRequest(t, mux, "POST", "/v1/setup/agents/local", LocalAgentConfig{BaseURL: "http://127.0.0.1:11434/v1", Model: "personal-local", APIKey: credentialFixtureKey}, 200)
	readPersonal(true)
	privatePath := filepath.Join(s.stateDir, "agent-credentials.json")
	before, err := os.ReadFile(privatePath)
	if err != nil {
		t.Fatal("cannot capture own private credential state")
	}
	got = connectionSave(t, mux, map[string]any{"action": "select", "expectedRevision": 2, "id": ""})
	if got.ActiveConnectionID != "" || !got.Connections[0].Configured {
		t.Fatal("gateway off lost the retained independent gateway configuration")
	}
	readPersonal(true)
	after, err := os.ReadFile(privatePath)
	if err != nil || !reflect.DeepEqual(before, after) {
		t.Fatal("switching gateway off changed personal credentials")
	}
	for h, key := range map[string]string{"codex": "OPENAI_API_KEY", "claude-code": "ANTHROPIC_API_KEY", "opencode": "OPENCODE_LOCAL_API_KEY"} {
		if receipt, env, err := s.ResolveAgentConnection(h, nil); err != nil || receipt.Kind != "legacy" || env[key] != credentialFixtureKey {
			t.Fatal("gateway off did not preserve the personal runtime connection")
		}
	}
}

func TestAgentConnectionsPrivatePersistenceAndExistingCredentialsRemainIndependent(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	if got := connectionRead(t, mux); got.Revision != 0 || got.ActiveConnectionID != "" || len(got.Connections) != 0 {
		t.Fatal("fresh connection state not empty")
	}
	path := filepath.Join(s.stateDir, "agent-connections.json")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatal("GET created private state")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/agents/anthropic", map[string]string{"apiKey": credentialFixtureKey}, 200)
	legacyBefore, err := os.ReadFile(filepath.Join(s.stateDir, "agent-credentials.json"))
	if err != nil {
		t.Fatal(err)
	}
	got := connectionSave(t, mux, connectionSaveBody(0))
	c := got.Connections[0]
	if got.Revision != 1 || c.Revision != 1 || !c.Configured || !c.KeyPresent || !agentConnectionID.MatchString(c.ID) {
		t.Fatal("saved public metadata is incomplete")
	}
	assertPrivateFile(t, path)
	connectionSave(t, mux, map[string]any{"action": "select", "expectedRevision": 1, "id": c.ID})
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Close()
	receipt, env, err := restarted.ResolveAgentConnection("codex", nil)
	if err != nil || receipt.ID != c.ID || receipt.Revision != 1 || env["OPENAI_API_KEY"] != connectionFixtureKey {
		t.Fatal("selected private route did not survive restart")
	}
	legacyAfter, _ := os.ReadFile(filepath.Join(s.stateDir, "agent-credentials.json"))
	if !reflect.DeepEqual(legacyBefore, legacyAfter) {
		t.Fatal("saving gateway overwrote existing credentials")
	}
	empty := ""
	receipt, env, err = restarted.ResolveAgentConnection("claude-code", &empty)
	if err != nil || receipt.Kind != "legacy" || env["ANTHROPIC_API_KEY"] != credentialFixtureKey {
		t.Fatal("explicit opt-out lost existing private legacy credentials")
	}
	other, otherMux := journeyFixture(t)
	other.registerSetupTools(otherMux)
	if len(connectionRead(t, otherMux).Connections) != 0 {
		t.Fatal("another private workspace saw this user's connections")
	}
	if _, env, err := other.ResolveAgentConnection("codex", &c.ID); err == nil || len(env) != 0 {
		t.Fatal("another private workspace resolved foreign route")
	}
}

func TestAgentConnectionEditsGuardRevisionsAndNeverFallback(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	got := connectionSave(t, mux, connectionSaveBody(0))
	id := got.Connections[0].ID
	connectionSave(t, mux, map[string]any{"action": "select", "expectedRevision": 1, "id": id})
	body := connectionSaveBody(1)
	body["id"] = id
	body["name"] = "stale edit"
	journeyRequest(t, mux, "POST", "/v1/setup/agent-connections", body, 409)
	if got = connectionRead(t, mux); got.Revision != 2 || got.Connections[0].Name != "Personal gateway" {
		t.Fatal("stale edit changed retained state")
	}
	body = connectionSaveBody(2)
	body["id"] = id
	body["name"] = "Disabled gateway"
	body["enabled"] = false
	delete(body, "apiKey")
	got = connectionSave(t, mux, body)
	if got.Connections[0].Revision != 3 || !got.Connections[0].KeyPresent || got.ActiveConnectionID != id {
		t.Fatal("edit failed to retain omitted key and explicit active binding")
	}
	if _, env, err := s.ResolveAgentConnection("codex", nil); err == nil || len(env) != 0 {
		t.Fatal("disabled active connection silently fell back")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/agent-connections", map[string]any{"action": "select", "expectedRevision": 3, "id": id}, 409)
	body["expectedRevision"] = 3
	body["apiKey"] = ""
	body["endpoints"] = AgentConnectionEndpoints{}
	body["models"] = AgentConnectionModels{}
	got = connectionSave(t, mux, body)
	if got.Connections[0].KeyPresent || got.Connections[0].Configured {
		t.Fatal("explicit clear retained secret/configured state")
	}
	got = connectionSave(t, mux, map[string]any{"action": "delete", "expectedRevision": 4, "id": id})
	if got.ActiveConnectionID != "" || len(got.Connections) != 0 {
		t.Fatal("explicit deletion did not clear selection")
	}
	if _, env, err := s.ResolveAgentConnection("codex", &id); err == nil || len(env) != 0 {
		t.Fatal("deleted explicit route silently fell back")
	}
}

func TestAgentConnectionProtocolsProduceIsolatedPrivateEnvironments(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	got := connectionSave(t, mux, connectionSaveBody(0))
	id := got.Connections[0].ID
	for _, provider := range []string{"codex", "claude-code", "opencode"} {
		t.Run(provider, func(t *testing.T) {
			receipt, env, err := s.ResolveAgentConnection(provider, &id)
			if err != nil || receipt.ID != id || receipt.Model == "" || receipt.Protocol == "" || receipt.AuthMode != "api-key" {
				t.Fatal("explicit route not resolved")
			}
			public, _ := json.Marshal(receipt)
			if strings.Contains(string(public), connectionFixtureKey) {
				t.Fatal("receipt exposed credential")
			}
			switch provider {
			case "codex":
				if len(env) != 3 || env["OPENAI_API_KEY"] != connectionFixtureKey || env["LUCI_CODEX_MODEL"] != "codex-alias" {
					t.Fatal("Responses routing incorrect")
				}
			case "claude-code":
				if env["ANTHROPIC_AUTH_TOKEN"] != connectionFixtureKey || env["ANTHROPIC_API_KEY"] != "" || env["ANTHROPIC_DEFAULT_HAIKU_MODEL"] != "claude-alias" {
					t.Fatal("gateway Claude did not retain token/model routing")
				}
			case "opencode":
				if env["LUCI_GATEWAY_API_KEY"] != connectionFixtureKey || env["LUCI_OPENCODE_MODEL"] != "luci-gateway/local-alias" || strings.Contains(env["OPENCODE_CONFIG_CONTENT"], connectionFixtureKey) {
					t.Fatal("OpenCode key/config boundary incorrect")
				}
				var config struct {
					Model    string                     `json:"model"`
					Enabled  []string                   `json:"enabled_providers"`
					Provider map[string]json.RawMessage `json:"provider"`
				}
				if json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config) != nil || config.Model != env["LUCI_OPENCODE_MODEL"] || !reflect.DeepEqual(config.Enabled, []string{"luci-gateway"}) || len(config.Provider) != 1 {
					t.Fatal("OpenCode enabled implicit provider")
				}
			}
			for key := range env {
				env[key] = "caller mutation"
			}
			_, fresh, err := s.ResolveAgentConnection(provider, &id)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range fresh {
				if v == "caller mutation" {
					t.Fatal("caller mutated private retained environment")
				}
			}
		})
	}
	if _, env, err := s.ResolveAgentConnection("shell", nil); err != nil || len(env) != 0 {
		t.Fatal("shell inherited active route")
	}
	if _, env, err := s.ResolveAgentConnection("shell", &id); err == nil || len(env) != 0 {
		t.Fatal("shell accepted explicit provider credentials")
	}
	// A saved Responses-only route never guesses an Anthropic protocol.
	body := connectionSaveBody(1)
	body["id"] = id
	body["endpoints"] = AgentConnectionEndpoints{Responses: "https://gateway.example.test/v1"}
	body["models"] = AgentConnectionModels{Codex: "alias"}
	connectionSave(t, mux, body)
	if _, env, err := s.ResolveAgentConnection("claude-code", &id); err == nil || len(env) != 0 {
		t.Fatal("unsupported protocol fell through to another route")
	}
}

func TestAgentConnectionSubscriptionAndUnauthenticatedLocalAreExplicit(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	got := connectionSave(t, mux, map[string]any{"action": "save", "expectedRevision": 0, "name": "My subscription", "kind": "codex-subscription", "enabled": true, "models": AgentConnectionModels{Codex: "default"}})
	id := got.Connections[0].ID
	receipt, env, err := s.ResolveAgentConnection("codex", &id)
	if err != nil || receipt.AuthMode != "chatgpt" || len(env) != 0 {
		t.Fatal("subscription selection provided API credential")
	}
	if _, env, err := s.ResolveAgentConnection("claude-code", &id); err == nil || len(env) != 0 {
		t.Fatal("subscription route fell back to another provider")
	}
	got = connectionSave(t, mux, map[string]any{"action": "save", "expectedRevision": 1, "name": "Local inference", "kind": "compatible-api", "enabled": true, "endpoints": AgentConnectionEndpoints{ChatCompletions: "http://127.0.0.1:11434/v1"}, "models": AgentConnectionModels{OpenCode: "fixture-local"}})
	id = got.Connections[1].ID
	_, env, err = s.ResolveAgentConnection("opencode", &id)
	if err != nil || env["LUCI_GATEWAY_API_KEY"] != "local-no-key" {
		t.Fatal("local no-key route not explicit")
	}
}

func TestAgentConnectionValidationRejectsAmbiguousOrUnsafeEndpointsWithoutWriting(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	for _, base := range []string{"https://user:password@example.test/v1", "https://example.test/v1?key=value", "https://example.test/v1#fragment", "http://public.example.test/v1", "https://example.test:0/v1", "https://example.test:65536/v1", "https://example.test/v1%0aheader", "https://example.test/v1/responses", "https://example.test/v1/chat/completions", "file:///tmp/model", "https://example.test\\evil/v1"} {
		body := connectionSaveBody(0)
		body["endpoints"] = AgentConnectionEndpoints{Responses: base}
		w := journeyRequest(t, mux, "POST", "/v1/setup/agent-connections", body, 409)
		if strings.Contains(w.Body.String(), base) || strings.Contains(w.Body.String(), connectionFixtureKey) {
			t.Fatal("invalid private input echoed")
		}
	}
	for _, base := range []string{"https://example.test/v1", "http://localhost:8080/v1", "http://[::1]:8080/v1", "http://192.168.20.5:8000/v1"} {
		if !validConnectionEndpoint(base) {
			t.Fatal("valid endpoint rejected")
		}
	}
	if got := connectionRead(t, mux); got.Revision != 0 || len(got.Connections) != 0 {
		t.Fatal("invalid save published state")
	}
	if _, err := os.Stat(filepath.Join(s.stateDir, "agent-connections.json")); !os.IsNotExist(err) {
		t.Fatal("invalid save wrote credentials")
	}
}

func TestAgentConnectionLoadRejectsMalformedOversizedLinkedOrForeignState(t *testing.T) {
	for _, name := range []string{"unknown-field", "duplicate", "missing-active", "oversized", "linked"} {
		t.Run(name, func(t *testing.T) {
			s, mux := journeyFixture(t)
			s.registerSetupTools(mux)
			connectionSave(t, mux, connectionSaveBody(0))
			path := filepath.Join(s.stateDir, "agent-connections.json")
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			switch name {
			case "unknown-field":
				data = append(data[:len(data)-2], []byte(`,"unexpected":true}`)...)
			case "duplicate":
				var saved agentConnectionStore
				json.Unmarshal(data, &saved)
				saved.Connections = append(saved.Connections, saved.Connections[0])
				data, _ = json.Marshal(saved)
			case "missing-active":
				var saved agentConnectionStore
				json.Unmarshal(data, &saved)
				saved.ActiveConnectionID = "agent-ffffffffffffffffffffffff"
				data, _ = json.Marshal(saved)
			case "oversized":
				data = []byte(strings.Repeat(" ", agentConnectionLimit+1))
			case "linked":
				outside := filepath.Join(t.TempDir(), "outside.json")
				if err = os.WriteFile(outside, data, 0600); err != nil {
					t.Fatal(err)
				}
				if err = os.Remove(path); err != nil {
					t.Fatal(err)
				}
				if err = os.Link(outside, path); err != nil {
					t.Skip("hardlinks unavailable")
				}
			}
			if name != "linked" {
				if err = os.WriteFile(path, data, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()}); err == nil {
				restarted.Close()
				t.Fatal("untrusted connection state loaded")
			}
		})
	}
}

func TestAgentConnectionFailedPrivateSaveCannotPublishSelection(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	got := connectionSave(t, mux, connectionSaveBody(0))
	id := got.Connections[0].ID
	path := filepath.Join(s.stateDir, "agent-connections.json")
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	// Replacing a directory must fail without publishing the new route or
	// altering the retained directory, even after writing a private temporary.
	journeyRequest(t, mux, "POST", "/v1/setup/agent-connections", map[string]any{"action": "select", "expectedRevision": 1, "id": id}, 500)
	if got = connectionRead(t, mux); got.Revision != 1 || got.ActiveConnectionID != "" {
		t.Fatal("failed persistence published new selection")
	}
}

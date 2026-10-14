package product

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderCredentialsRemainSeparateAndLocalModelCannotFallback(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	key := "sk-openai-fixture-not-real-123456789"
	journeyRequest(t, mux, "POST", "/v1/setup/agents/anthropic", map[string]string{"apiKey": credentialFixtureKey}, 200)
	journeyRequest(t, mux, "POST", "/v1/setup/agents/openai", map[string]string{"apiKey": key}, 200)
	journeyRequest(t, mux, "POST", "/v1/setup/agents/local", LocalAgentConfig{BaseURL: "http://localhost:11434/v1", Model: "qwen3:8b", APIKey: "fixture-local-key"}, 200)
	assertPrivateFile(t, filepath.Join(s.stateDir, "agent-credentials.json"))
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	if restarted.AgentEnvironment("codex")["OPENAI_API_KEY"] != key {
		t.Fatal("Codex key did not persist")
	}
	env := restarted.AgentEnvironment("opencode")
	if env["OPENCODE_LOCAL_API_KEY"] != "fixture-local-key" || env["OPENAI_API_KEY"] != "" || env["ANTHROPIC_API_KEY"] != "" {
		t.Fatal("provider credential isolation failed")
	}
	var config struct {
		Model      string   `json:"model"`
		SmallModel string   `json:"small_model"`
		Enabled    []string `json:"enabled_providers"`
	}
	if json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config) != nil || config.Model != "luci-local/qwen3:8b" || config.SmallModel != config.Model || len(config.Enabled) != 1 || config.Enabled[0] != "luci-local" {
		t.Fatal("local model has a paid/default provider fallback")
	}
	if strings.Contains(env["OPENCODE_CONFIG_CONTENT"], "fixture-local-key") || !strings.Contains(env["OPENCODE_CONFIG_CONTENT"], "{env:OPENCODE_LOCAL_API_KEY}") {
		t.Fatal("OpenCode must resolve the documented environment placeholder without embedding the secret")
	}
	body := journeyRequest(t, mux, "GET", "/v1/setup/agents", nil, 200).Body.String()
	for _, secret := range []string{key, credentialFixtureKey, "fixture-local-key"} {
		if strings.Contains(body, secret) {
			t.Fatal("read exposed provider credentials")
		}
	}
	journeyRequest(t, mux, "POST", "/v1/setup/agents/openai", map[string]string{"apiKey": ""}, 200)
	if len(s.AgentEnvironment("codex")) != 0 || s.AgentEnvironment("claude-code")["ANTHROPIC_API_KEY"] != credentialFixtureKey || len(s.AgentEnvironment("opencode")) == 0 {
		t.Fatal("disconnect cleared or leaked another provider")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/agents/local", LocalAgentConfig{}, 200)
	if len(s.AgentEnvironment("opencode")) != 0 {
		t.Fatal("local disconnect retained connection")
	}
}

func TestLocalProviderRejectsUnsafeOrIncompleteEndpointsWithoutRequests(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	for _, endpoint := range []string{"http://example.com/v1", "file:///tmp/model", "https://user:password@example.com/v1", "https://example.com/v1?key=secret", "https://example.com/v1#secret", ""} {
		journeyRequest(t, mux, "POST", "/v1/setup/agents/local", LocalAgentConfig{BaseURL: endpoint, Model: "model"}, 400)
	}
	for _, endpoint := range []string{"http://127.0.0.1:11434/v1", "http://[::1]:11434/v1", "http://192.168.1.40:1234/v1", "https://models.example.org/v1"} {
		journeyRequest(t, mux, "POST", "/v1/setup/agents/local", LocalAgentConfig{BaseURL: endpoint, Model: "model"}, 200)
	}
}

func TestLocalModelEditPreservesOmittedPrivateKeyAndExplicitDisconnectClearsIt(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	const key = "fixture-local-private-key"
	journeyRequest(t, mux, "POST", "/v1/setup/agents/local", LocalAgentConfig{BaseURL: "http://localhost:11434/v1", Model: "first", APIKey: key}, 200)
	response := journeyRequest(t, mux, "POST", "/v1/setup/agents/local", map[string]string{
		"baseUrl": "http://localhost:11434/v1", "model": "second",
	}, 200)
	if strings.Contains(response.Body.String(), key) {
		t.Fatal("model save exposed retained private key")
	}
	env := s.AgentEnvironment("opencode")
	if env["OPENCODE_LOCAL_API_KEY"] != key || !strings.Contains(env["OPENCODE_CONFIG_CONTENT"], `"luci-local/second"`) {
		t.Fatal("omitted key did not survive model update")
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	if restarted.AgentEnvironment("opencode")["OPENCODE_LOCAL_API_KEY"] != key {
		t.Fatal("retained key did not survive restart")
	}
	path := filepath.Join(s.stateDir, "agent-credentials.json")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	denied := journeyRequest(t, mux, "POST", "/v1/setup/agents/local", map[string]string{
		"baseUrl": "http://127.0.0.1:11434/v1", "model": "third",
	}, 409)
	if !strings.Contains(denied.Body.String(), "local_key_required_for_endpoint_change") || strings.Contains(denied.Body.String(), key) {
		t.Fatal("endpoint change did not give safe key guidance")
	}
	after, err := os.ReadFile(path)
	if err != nil || !bytes.Equal(before, after) || s.AgentEnvironment("opencode")["OPENCODE_LOCAL_API_KEY"] != key {
		t.Fatal("omitted-key endpoint change mutated saved connection")
	}
	const replacement = "fixture-replacement-key"
	journeyRequest(t, mux, "POST", "/v1/setup/agents/local", map[string]string{
		"baseUrl": "http://127.0.0.1:11434/v1", "model": "third", "apiKey": replacement,
	}, 200)
	if s.AgentEnvironment("opencode")["OPENCODE_LOCAL_API_KEY"] != replacement {
		t.Fatal("explicit new endpoint key was not saved")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/agents/local", map[string]string{
		"baseUrl": "http://localhost:11434/v1", "model": "fourth", "apiKey": "",
	}, 200)
	if s.AgentEnvironment("opencode")["OPENCODE_LOCAL_API_KEY"] != "local-no-key" {
		t.Fatal("explicit empty key did not clear the saved credential")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/agents/local", LocalAgentConfig{}, 200)
	if len(s.AgentEnvironment("opencode")) != 0 {
		t.Fatal("explicit connection removal retained local credential")
	}
	reopened, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	if len(reopened.AgentEnvironment("opencode")) != 0 {
		t.Fatal("removed local credential returned after restart")
	}
}

package product

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const credentialFixtureKey = "sk-ant-api-fixture-not-a-real-secret-123456789"

func TestPersonalAgentCredentialPersistsPrivatelyWithoutReadExposure(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	w := journeyRequest(t, mux, "GET", "/v1/setup/agents", nil, 200)
	if strings.Contains(w.Body.String(), "true") {
		t.Fatal("unconfigured agent reported connected")
	}
	if _, err := os.Stat(filepath.Join(s.stateDir, "agent-credentials.json")); !os.IsNotExist(err) {
		t.Fatalf("read created credentials: %v", err)
	}
	w = journeyRequest(t, mux, "POST", "/v1/setup/agents/anthropic", map[string]string{"apiKey": credentialFixtureKey}, 200)
	if strings.Contains(w.Body.String(), credentialFixtureKey) || !strings.Contains(w.Body.String(), `"configured":true`) {
		t.Fatalf("save response exposed or omitted connection status: %s", w.Body.String())
	}
	path := filepath.Join(s.stateDir, "agent-credentials.json")
	assertPrivateFile(t, path)
	for _, harness := range []string{"shell", "codex", "gemini-cli", "opencode", ""} {
		if env := s.AgentEnvironment(harness); len(env) != 0 {
			t.Fatalf("credential escaped to %q", harness)
		}
	}
	env := s.AgentEnvironment("claude-code")
	if len(env) != 1 || env["ANTHROPIC_API_KEY"] != credentialFixtureKey {
		t.Fatal("Claude did not receive its explicitly configured personal key")
	}
	env["ANTHROPIC_API_KEY"] = "changed-by-caller"
	if s.AgentEnvironment("claude-code")["ANTHROPIC_API_KEY"] != credentialFixtureKey {
		t.Fatal("caller mutated stored credentials")
	}
	restarted, err := New(Options{StateDir: s.stateDir, ProjectsDir: s.projectRoot()})
	if err != nil {
		t.Fatal(err)
	}
	if restarted.AgentEnvironment("claude-code")["ANTHROPIC_API_KEY"] != credentialFixtureKey {
		t.Fatal("credential did not survive restart")
	}
	w = journeyRequest(t, mux, "GET", "/v1/setup/agents", nil, 200)
	if strings.Contains(w.Body.String(), credentialFixtureKey) {
		t.Fatal("GET exposed the credential")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/agents/anthropic", map[string]string{"apiKey": ""}, 200)
	if len(s.AgentEnvironment("claude-code")) != 0 {
		t.Fatal("cleared key was still injected into a new session")
	}
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("clear retained secret material: %v", err)
	}
	journeyRequest(t, mux, "POST", "/v1/setup/agents/anthropic", map[string]string{"apiKey": ""}, 200)
}

func TestAgentCredentialsValidateAndRefuseProjectOwnedStorage(t *testing.T) {
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	for _, key := range []string{"short", credentialFixtureKey + "\r\n", credentialFixtureKey + " injected", strings.Repeat("x", 8193)} {
		w := journeyRequest(t, mux, "POST", "/v1/setup/agents/anthropic", map[string]string{"apiKey": key}, 400)
		if strings.Contains(w.Body.String(), key) {
			t.Fatal("invalid-key response echoed input")
		}
	}
	if _, err := os.Stat(filepath.Join(s.stateDir, "agent-credentials.json")); !os.IsNotExist(err) {
		t.Fatal("invalid key was saved")
	}
	journeyRequest(t, mux, "POST", "/v1/setup/agents/anthropic", map[string]string{"apiKey": credentialFixtureKey}, 200)
	s.mu.Lock()
	s.config.ProjectsDir = s.stateDir
	s.mu.Unlock()
	journeyRequest(t, mux, "POST", "/v1/setup/agents/anthropic", map[string]string{"apiKey": credentialFixtureKey}, 409)
	if len(s.AgentEnvironment("claude-code")) != 0 {
		t.Fatal("key injected after its storage became project-owned")
	}
	w := journeyRequest(t, mux, "GET", "/v1/setup/agents", nil, 200)
	var response struct {
		Providers []AgentProvider `json:"providers"`
	}
	_ = json.Unmarshal(w.Body.Bytes(), &response)
	if len(response.Providers) != 3 || response.Providers[0].Configured {
		t.Fatal("unusable host storage was reported connected")
	}
}

func TestSetupCannotPutHostStateInsideAProjectMount(t *testing.T) {
	state := filepath.Join(t.TempDir(), "private-state")
	if _, err := New(Options{StateDir: state, ProjectsDir: filepath.Dir(state)}); err == nil {
		t.Fatal("startup accepted project-writable credential storage")
	}
	s, mux := journeyFixture(t)
	s.registerSetupTools(mux)
	mux.HandleFunc("POST /v1/setup/config", s.handleConfig)
	before := s.Config()
	changed := before
	changed.ProjectsDir = filepath.Dir(s.stateDir)
	journeyRequest(t, mux, "POST", "/v1/setup/config", changed, 409)
	if s.Config() != before {
		t.Fatal("unsafe folder changed the active configuration")
	}
}

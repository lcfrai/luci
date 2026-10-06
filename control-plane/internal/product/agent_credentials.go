package product

import (
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

type AgentProvider struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Configured    bool   `json:"configured"`
	AuthMode      string `json:"authMode,omitempty"`
	Model         string `json:"model,omitempty"`
	BaseURL       string `json:"baseUrl,omitempty"`
	SignInCommand string `json:"signInCommand,omitempty"`
}

type LocalAgentConfig struct {
	BaseURL string `json:"baseUrl,omitempty"`
	Model   string `json:"model,omitempty"`
	APIKey  string `json:"apiKey,omitempty"`
}

type agentCredentials struct {
	AnthropicAPIKey string           `json:"anthropicApiKey"`
	OpenAIAPIKey    string           `json:"openaiApiKey,omitempty"`
	Local           LocalAgentConfig `json:"local,omitempty"`
}

func validAgentKey(key string) bool {
	if key == "" {
		return true
	}
	if len(key) < 20 || len(key) > 8192 {
		return false
	}
	for _, char := range key {
		if char < 33 || char > 126 {
			return false
		}
	}
	return true
}

func (s *Server) agentCredentialPath() (string, error) {
	root := s.projectRoot()
	if _, err := hostpath.ResolveOutside(root, s.stateDir); err != nil {
		return "", errors.New("agent credentials require Luci state outside the projects folder")
	}
	return guardedProjectPath(s.stateDir, "agent-credentials.json", true)
}

func (s *Server) initAgentCredentials() error {
	if err := s.initAgentConnections(); err != nil {
		return err
	}
	// An unconfigured installation can still open with an unusual folder
	// layout. Credential operations themselves require host-owned state.
	if _, err := os.Lstat(filepath.Join(s.stateDir, "agent-credentials.json")); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	path, err := s.agentCredentialPath()
	if err != nil {
		return err
	}
	if err = hostpath.OneName(path); err != nil {
		return errors.New("agent credentials must be a private file with one name")
	}
	file, err := os.Open(path)
	if err != nil {
		return errors.New("cannot read saved agent credentials")
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, 32<<10+1))
	if err != nil || len(data) > 32<<10 {
		return errors.New("saved agent credentials exceed their limit or cannot be read")
	}
	var saved agentCredentials
	if json.Unmarshal(data, &saved) != nil || !validAgentKey(saved.AnthropicAPIKey) || !validAgentKey(saved.OpenAIAPIKey) || !validLocalAgent(saved.Local) {
		return errors.New("saved agent credentials are invalid")
	}
	s.agentAPIKey = saved.AnthropicAPIKey
	s.agentOpenAIKey = saved.OpenAIAPIKey
	s.agentLocal = saved.Local
	return nil
}

func (s *Server) registerAgentCredentials(mux Registrar) {
	s.registerAgentConnections(mux)
	mux.HandleFunc("GET /v1/setup/agents", s.getAgents)
	mux.HandleFunc("POST /v1/setup/agents/anthropic", s.setAnthropicCredential)
	mux.HandleFunc("POST /v1/setup/agents/openai", s.setOpenAICredential)
	mux.HandleFunc("POST /v1/setup/agents/local", s.setLocalAgent)
}

func (s *Server) getAgents(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	local := s.agentLocal
	s.mu.RUnlock()
	writeJSON(w, 200, map[string]any{"providers": []AgentProvider{
		{ID: "anthropic", Name: "Claude Code", Configured: len(s.legacyAgentEnvironment("claude-code")) > 0, AuthMode: "api-key"},
		{ID: "openai", Name: "Codex", Configured: len(s.legacyAgentEnvironment("codex")) > 0, AuthMode: "api-key-or-cli-sign-in", SignInCommand: "codex login --device-auth"},
		{ID: "local", Name: "OpenCode · local or compatible model", Configured: len(s.legacyAgentEnvironment("opencode")) > 0, AuthMode: "local", BaseURL: local.BaseURL, Model: local.Model},
	}})
}

func (s *Server) setAnthropicCredential(w http.ResponseWriter, r *http.Request) {
	s.setAgentKey(w, r, false)
}
func (s *Server) setOpenAICredential(w http.ResponseWriter, r *http.Request) {
	s.setAgentKey(w, r, true)
}
func (s *Server) setAgentKey(w http.ResponseWriter, r *http.Request, openAI bool) {
	var req struct {
		APIKey string `json:"apiKey"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !validAgentKey(req.APIKey) {
		problem(w, 400, "invalid_api_key", "Paste an API key of 20–8,192 printable characters without whitespace, or an empty string to disconnect.")
		return
	}
	path, err := s.agentCredentialPath()
	if err != nil {
		problem(w, 409, "host_state_required", err.Error())
		return
	}
	s.mu.Lock()
	saved := agentCredentials{AnthropicAPIKey: s.agentAPIKey, OpenAIAPIKey: s.agentOpenAIKey, Local: s.agentLocal}
	if openAI {
		saved.OpenAIAPIKey = req.APIKey
	} else {
		saved.AnthropicAPIKey = req.APIKey
	}
	if saved.AnthropicAPIKey == "" && saved.OpenAIAPIKey == "" && saved.Local.BaseURL == "" {
		err = os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
	} else {
		err = writePrivateAgentCredentials(path, saved)
	}
	if err != nil {
		s.mu.Unlock()
		// Never echo request data or OS diagnostics that may include content.
		problem(w, 500, "credential_save_failed", "Could not update the private agent credential file. Check access to Luci's state folder.")
		return
	}
	s.agentAPIKey, s.agentOpenAIKey = saved.AnthropicAPIKey, saved.OpenAIAPIKey
	s.mu.Unlock()
	s.getAgents(w, r)
}

// AgentEnvironment returns a new map for one sandbox. Secrets are not part of
// project files, session records, GET responses or command arguments. Clearing
// a key applies to new/restarted sessions; existing containers retain their own
// environment until stopped. Shells and unavailable harnesses get no secret.
func (s *Server) AgentEnvironment(harness string) map[string]string {
	_, env, err := s.ResolveAgentConnection(harness, nil)
	if err != nil {
		return nil
	}
	return env
}

func (s *Server) legacyAgentEnvironment(harness string) map[string]string {
	s.mu.RLock()
	key, openai, local, root := s.agentAPIKey, s.agentOpenAIKey, s.agentLocal, s.config.ProjectsDir
	s.mu.RUnlock()
	if _, err := hostpath.ResolveOutside(root, s.stateDir); err != nil {
		return nil
	}
	switch harness {
	case "claude-code":
		if key != "" {
			return map[string]string{"ANTHROPIC_API_KEY": key}
		}
	case "codex":
		// Codex supports an environment-backed custom provider, avoiding a
		// key in argv, on stdin in a PTY, or in a project config file.
		if openai != "" {
			return map[string]string{"OPENAI_API_KEY": openai}
		}
	case "opencode":
		if local.BaseURL == "" {
			return nil
		}
		config := map[string]any{
			"model": "luci-local/" + local.Model, "small_model": "luci-local/" + local.Model,
			"enabled_providers": []string{"luci-local"}, "autoupdate": false, "share": "disabled",
			"instructions": []string{"LUCI.md"},
			"provider": map[string]any{"luci-local": map[string]any{
				"npm": "@ai-sdk/openai-compatible", "name": "Luci configured model",
				"options": map[string]any{"baseURL": local.BaseURL, "apiKey": "{env:OPENCODE_LOCAL_API_KEY}"},
				"models":  map[string]any{local.Model: map[string]any{"name": local.Model}},
			}},
		}
		data, _ := json.Marshal(config)
		apiKey := local.APIKey
		if apiKey == "" {
			apiKey = "local-no-key"
		}
		return map[string]string{"OPENCODE_CONFIG_CONTENT": string(data), "OPENCODE_LOCAL_API_KEY": apiKey}
	}
	return nil
}

func validLocalAgent(c LocalAgentConfig) bool {
	if c.BaseURL == "" && c.Model == "" && c.APIKey == "" {
		return true
	}
	if len(c.BaseURL) > 2048 || len(c.Model) == 0 || len(c.Model) > 200 || strings.ContainsAny(c.Model, "\r\n\t ") {
		return false
	}
	for _, char := range c.Model {
		if char < 33 || char > 126 {
			return false
		}
	}
	if len(c.APIKey) > 8192 || strings.ContainsAny(c.APIKey, "\r\n\t ") {
		return false
	}
	for _, char := range c.APIKey {
		if char < 33 || char > 126 {
			return false
		}
	}
	u, err := url.Parse(c.BaseURL)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" {
		return false
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "host.docker.internal" || ip != nil && (ip.IsLoopback() || ip.IsPrivate()))
}

func (s *Server) setLocalAgent(w http.ResponseWriter, r *http.Request) {
	var req LocalAgentConfig
	if !readJSON(w, r, &req) {
		return
	}
	if !validLocalAgent(req) {
		problem(w, 400, "invalid_local_model", "Set a model ID and a compatible /v1 endpoint: HTTP on a local/private address or HTTPS. Empty all fields to disconnect.")
		return
	}
	path, err := s.agentCredentialPath()
	if err != nil {
		problem(w, 409, "host_state_required", err.Error())
		return
	}
	s.mu.Lock()
	err = writePrivateAgentCredentials(path, agentCredentials{AnthropicAPIKey: s.agentAPIKey, OpenAIAPIKey: s.agentOpenAIKey, Local: req})
	if err == nil {
		s.agentLocal = req
	}
	s.mu.Unlock()
	if err != nil {
		problem(w, 500, "credential_save_failed", "Could not save the model connection in private Luci state.")
		return
	}
	s.getAgents(w, r)
}

func writePrivateAgentCredentials(path string, value agentCredentials) error {
	return writePrivateAgentJSON(path, value)
}

func writePrivateAgentJSON(path string, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".agent-credentials-*")
	if err != nil {
		return err
	}
	tmp := file.Name()
	defer os.Remove(tmp)
	// Restrict the empty temporary file BEFORE writing a secret. The atomic
	// rename carries its private mode/ACL to the destination.
	if err = restrictPrivateFile(tmp); err != nil {
		file.Close()
		return err
	}
	if _, err = file.Write(append(data, '\n')); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(tmp, path)
}

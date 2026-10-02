package product

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

type AgentProvider struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	Configured bool   `json:"configured"`
}

type agentCredentials struct {
	AnthropicAPIKey string `json:"anthropicApiKey"`
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
	data, err := io.ReadAll(io.LimitReader(file, 16<<10+1))
	if err != nil || len(data) > 16<<10 {
		return errors.New("saved agent credentials exceed their limit or cannot be read")
	}
	var saved agentCredentials
	if json.Unmarshal(data, &saved) != nil || !validAgentKey(saved.AnthropicAPIKey) {
		return errors.New("saved agent credentials are invalid")
	}
	s.agentAPIKey = saved.AnthropicAPIKey
	return nil
}

func (s *Server) registerAgentCredentials(mux Registrar) {
	mux.HandleFunc("GET /v1/setup/agents", s.getAgents)
	mux.HandleFunc("POST /v1/setup/agents/anthropic", s.setAnthropicCredential)
}

func (s *Server) getAgents(w http.ResponseWriter, r *http.Request) {
	configured := len(s.AgentEnvironment("claude-code")) > 0
	writeJSON(w, 200, map[string]any{"providers": []AgentProvider{{ID: "anthropic", Name: "Claude Code", Configured: configured}}})
}

func (s *Server) setAnthropicCredential(w http.ResponseWriter, r *http.Request) {
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
	if req.APIKey == "" {
		err = os.Remove(path)
		if errors.Is(err, os.ErrNotExist) {
			err = nil
		}
	} else {
		err = writePrivateAgentCredentials(path, agentCredentials{AnthropicAPIKey: req.APIKey})
	}
	if err != nil {
		s.mu.Unlock()
		// Never echo request data or OS diagnostics that may include content.
		problem(w, 500, "credential_save_failed", "Could not update the private agent credential file. Check access to Luci's state folder.")
		return
	}
	s.agentAPIKey = req.APIKey
	s.mu.Unlock()
	s.getAgents(w, r)
}

// AgentEnvironment returns a new map for one sandbox. Secrets are not part of
// project files, session records, GET responses or command arguments. Clearing
// a key applies to new/restarted sessions; existing containers retain their own
// environment until stopped. Shells and unavailable harnesses get no secret.
func (s *Server) AgentEnvironment(harness string) map[string]string {
	if harness != "claude-code" {
		return nil
	}
	s.mu.RLock()
	key, root := s.agentAPIKey, s.config.ProjectsDir
	s.mu.RUnlock()
	if key == "" {
		return nil
	}
	if _, err := hostpath.ResolveOutside(root, s.stateDir); err != nil {
		return nil
	}
	return map[string]string{"ANTHROPIC_API_KEY": key}
}

func writePrivateAgentCredentials(path string, value agentCredentials) error {
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

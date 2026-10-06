package product

import (
	"bytes"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

const agentConnectionLimit = 256 << 10
const maxAgentConnections = 16

var agentConnectionID = regexp.MustCompile(`^agent-[a-f0-9]{24}$`)

type AgentConnectionEndpoints struct {
	Responses         string `json:"responses,omitempty"`
	AnthropicMessages string `json:"anthropicMessages,omitempty"`
	ChatCompletions   string `json:"chatCompletions,omitempty"`
}

type AgentConnectionModels struct {
	Codex      string `json:"codex,omitempty"`
	ClaudeCode string `json:"claudeCode,omitempty"`
	OpenCode   string `json:"opencode,omitempty"`
}

// AgentConnection is deliberately a public projection, never the saved key.
type AgentConnection struct {
	ID            string                   `json:"id"`
	Name          string                   `json:"name"`
	Kind          string                   `json:"kind"`
	Enabled       bool                     `json:"enabled"`
	Revision      int                      `json:"revision"`
	Endpoints     AgentConnectionEndpoints `json:"endpoints"`
	Models        AgentConnectionModels    `json:"models"`
	Configured    bool                     `json:"configured"`
	KeyPresent    bool                     `json:"keyPresent"`
	BlockedReason string                   `json:"blockedReason,omitempty"`
}

type AgentConnectionReceipt struct {
	ID       string `json:"id,omitempty"`
	Kind     string `json:"kind"`
	Protocol string `json:"protocol,omitempty"`
	Model    string `json:"model,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`
	AuthMode string `json:"authMode,omitempty"`
	Revision int    `json:"revision"`
}

type privateAgentConnection struct {
	ID        string                   `json:"id"`
	Name      string                   `json:"name"`
	Kind      string                   `json:"kind"`
	Enabled   bool                     `json:"enabled"`
	Revision  int                      `json:"revision"`
	Endpoints AgentConnectionEndpoints `json:"endpoints"`
	Models    AgentConnectionModels    `json:"models"`
	APIKey    string                   `json:"apiKey,omitempty"`
}

type agentConnectionStore struct {
	Version            int                      `json:"version"`
	Revision           int                      `json:"revision"`
	ActiveConnectionID string                   `json:"activeConnectionId"`
	Connections        []privateAgentConnection `json:"connections"`
}

func (s *Server) agentConnectionPath() (string, error) {
	if _, err := hostpath.ResolveOutside(s.projectRoot(), s.stateDir); err != nil {
		return "", errors.New("agent connections require private Luci state outside the projects folder")
	}
	return guardedProjectPath(s.stateDir, "agent-connections.json", true)
}

func validConnectionToken(v string, max int, empty bool) bool {
	if len(v) > max || !empty && v == "" {
		return false
	}
	for _, c := range v {
		if c < 33 || c > 126 {
			return false
		}
	}
	return true
}

func validConnectionEndpoint(value string) bool {
	if value == "" {
		return true
	}
	if len(value) > 2048 || strings.ContainsAny(value, "\r\n\t \\") {
		return false
	}
	u, err := url.Parse(value)
	if err != nil || u.Hostname() == "" || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || u.Opaque != "" {
		return false
	}
	if port := u.Port(); port != "" {
		n, err := strconv.Atoi(port)
		if err != nil || n < 1 || n > 65535 {
			return false
		}
	}
	for _, r := range u.Path {
		if unicode.IsControl(r) {
			return false
		}
	}
	// A base URL is supplied for this protocol; never guess a different API.
	for _, suffix := range []string{"/responses", "/messages", "/chat/completions"} {
		if strings.HasSuffix(strings.TrimRight(u.Path, "/"), suffix) {
			return false
		}
	}
	if u.Scheme == "https" {
		return true
	}
	ip := net.ParseIP(u.Hostname())
	return u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "host.docker.internal" || ip != nil && (ip.IsLoopback() || ip.IsPrivate()))
}

func validateAgentConnection(c privateAgentConnection) bool {
	if !agentConnectionID.MatchString(c.ID) || c.Revision < 1 || len(c.Name) == 0 || len(c.Name) > 160 || strings.TrimSpace(c.Name) != c.Name || !validConnectionToken(c.APIKey, 8192, true) {
		return false
	}
	for _, r := range c.Name {
		if unicode.IsControl(r) {
			return false
		}
	}
	if !slices.Contains([]string{"litellm", "compatible-api", "codex-subscription", "direct-api"}, c.Kind) || !validConnectionEndpoint(c.Endpoints.Responses) || !validConnectionEndpoint(c.Endpoints.AnthropicMessages) || !validConnectionEndpoint(c.Endpoints.ChatCompletions) {
		return false
	}
	for _, model := range []string{c.Models.Codex, c.Models.ClaudeCode, c.Models.OpenCode} {
		if !validConnectionToken(model, 200, true) {
			return false
		}
	}
	if c.Kind == "codex-subscription" {
		return c.APIKey == "" && c.Endpoints == (AgentConnectionEndpoints{}) && c.Models.ClaudeCode == "" && c.Models.OpenCode == ""
	}
	return true
}

func agentConnectionConfigured(c privateAgentConnection) bool {
	if c.Kind == "codex-subscription" {
		return true // Saved selection only; the native account still must be ready.
	}
	return c.Endpoints.Responses != "" && c.Models.Codex != "" && c.APIKey != "" || c.Endpoints.AnthropicMessages != "" && c.Models.ClaudeCode != "" && c.APIKey != "" || c.Endpoints.ChatCompletions != "" && c.Models.OpenCode != ""
}

func publicAgentConnection(c privateAgentConnection) AgentConnection {
	configured := agentConnectionConfigured(c)
	value := AgentConnection{ID: c.ID, Name: c.Name, Kind: c.Kind, Enabled: c.Enabled, Revision: c.Revision, Endpoints: c.Endpoints, Models: c.Models, Configured: configured, KeyPresent: c.APIKey != ""}
	if !c.Enabled {
		value.BlockedReason = "This connection is disabled. Enable it before starting a new agent."
	} else if !configured {
		value.BlockedReason = "Save a compatible protocol base URL, model and required API key."
	}
	return value
}

func (s *Server) initAgentConnections() error {
	s.agentConnections = agentConnectionStore{Version: 1, Connections: []privateAgentConnection{}}
	if _, err := os.Lstat(filepath.Join(s.stateDir, "agent-connections.json")); errors.Is(err, os.ErrNotExist) {
		return nil
	}
	file, err := s.agentConnectionPath()
	if err != nil {
		return err
	}
	if err := hostpath.OneName(file); err != nil {
		return errors.New("agent connections must be a private file with one name")
	}
	f, err := os.Open(file)
	if err != nil {
		return errors.New("cannot read saved agent connections")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, agentConnectionLimit+1))
	if err != nil || len(data) > agentConnectionLimit {
		return errors.New("saved agent connections exceed their private storage limit")
	}
	var saved agentConnectionStore
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(&saved) != nil || saved.Version != 1 || saved.Revision < 1 || len(saved.Connections) > maxAgentConnections {
		return errors.New("saved agent connections are invalid")
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return errors.New("saved agent connections are invalid")
	}
	seen := make(map[string]bool)
	for _, c := range saved.Connections {
		if !validateAgentConnection(c) || seen[c.ID] || c.Revision > saved.Revision {
			return errors.New("saved agent connections are invalid")
		}
		seen[c.ID] = true
	}
	if saved.ActiveConnectionID != "" && !seen[saved.ActiveConnectionID] {
		return errors.New("saved active agent connection is missing")
	}
	s.agentConnections = saved
	return nil
}

func (s *Server) registerAgentConnections(mux Registrar) {
	mux.HandleFunc("GET /v1/setup/agent-connections", s.getAgentConnections)
	mux.HandleFunc("POST /v1/setup/agent-connections", s.setAgentConnection)
}

func (s *Server) getAgentConnections(w http.ResponseWriter, r *http.Request) {
	_, pathErr := s.agentConnectionPath()
	s.mu.RLock()
	store := s.agentConnections
	connections := make([]AgentConnection, 0, len(store.Connections))
	for _, c := range store.Connections {
		value := publicAgentConnection(c)
		if pathErr != nil {
			value.Configured = false
			value.BlockedReason = "Agent connections require private Luci state outside the projects folder."
		}
		connections = append(connections, value)
	}
	s.mu.RUnlock()
	writeJSON(w, 200, map[string]any{"revision": store.Revision, "activeConnectionId": store.ActiveConnectionID, "connections": connections})
}

func (s *Server) setAgentConnection(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Action           string                   `json:"action"`
		ExpectedRevision int                      `json:"expectedRevision"`
		ID               string                   `json:"id"`
		Name             string                   `json:"name"`
		Kind             string                   `json:"kind"`
		Enabled          bool                     `json:"enabled"`
		Endpoints        AgentConnectionEndpoints `json:"endpoints"`
		Models           AgentConnectionModels    `json:"models"`
		APIKey           *string                  `json:"apiKey"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	file, err := s.agentConnectionPath()
	if err != nil {
		problem(w, 409, "host_state_required", "Agent connections require private Luci state outside the projects folder.")
		return
	}
	s.mu.Lock()
	store := s.agentConnections
	store.Connections = append([]privateAgentConnection{}, store.Connections...)
	if store.Version == 0 {
		store.Version = 1
	}
	if req.ExpectedRevision != store.Revision {
		s.mu.Unlock()
		problem(w, 409, "agent_connection_changed", "Your connections changed. Refresh them before saving this edit.")
		return
	}
	index := slices.IndexFunc(store.Connections, func(c privateAgentConnection) bool { return c.ID == req.ID })
	code, message := "", ""
	switch req.Action {
	case "save":
		var c privateAgentConnection
		if req.ID == "" {
			var id [12]byte
			if len(store.Connections) >= maxAgentConnections {
				code, message = "agent_connection_limit", "Keep up to 16 saved agent connections. Remove an unused one first."
			} else if _, err = rand.Read(id[:]); err != nil {
				code, message = "agent_connection_save_failed", "Could not create a private connection identity."
			} else {
				c.ID = "agent-" + hex.EncodeToString(id[:])
			}
		} else if index < 0 {
			code, message = "agent_connection_missing", "That saved agent connection no longer exists. Refresh your connections."
		} else {
			c = store.Connections[index]
		}
		c.Name, c.Kind, c.Enabled, c.Endpoints, c.Models, c.Revision = req.Name, req.Kind, req.Enabled, req.Endpoints, req.Models, store.Revision+1
		if req.APIKey != nil {
			c.APIKey = *req.APIKey
		}
		if code == "" && (!validateAgentConnection(c) || c.Enabled && !agentConnectionConfigured(c)) {
			code, message = "invalid_agent_connection", "Use a supported connection kind, protocol base URLs, explicit models and the required private key. Subscription connections cannot contain an API key or endpoint."
		}
		if code == "" {
			if index < 0 {
				store.Connections = append(store.Connections, c)
			} else {
				store.Connections[index] = c
			}
		}
	case "select":
		if req.ID != "" && (index < 0 || !store.Connections[index].Enabled || !agentConnectionConfigured(store.Connections[index])) {
			code, message = "agent_connection_unavailable", "Choose a saved, enabled connection with a compatible endpoint and model."
		} else {
			store.ActiveConnectionID = req.ID
		}
	case "delete":
		if index < 0 {
			code, message = "agent_connection_missing", "That saved agent connection no longer exists."
		} else {
			if store.ActiveConnectionID == req.ID {
				// Explicitly deleting the active selection opts back into legacy.
				store.ActiveConnectionID = ""
			}
			store.Connections = slices.Delete(store.Connections, index, index+1)
		}
	default:
		code, message = "invalid_agent_connection_action", "Choose save, select or delete."
	}
	if code != "" {
		s.mu.Unlock()
		problem(w, 409, code, message)
		return
	}
	store.Revision++
	data, err := json.Marshal(store)
	if err == nil && len(data) > agentConnectionLimit {
		err = errors.New("private connection storage limit")
	}
	if err == nil {
		err = writePrivateAgentJSON(file, store)
	}
	if err != nil {
		s.mu.Unlock()
		problem(w, 500, "agent_connection_save_failed", "Could not save the private agent connection. Check access to Luci's state folder.")
		return
	}
	s.agentConnections = store
	s.mu.Unlock()
	s.getAgentConnections(w, r)
}

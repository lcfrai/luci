package product

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

const workspaceAdminLimit = 1 << 20

var workspaceAdminID = regexp.MustCompile(`^[A-Za-z0-9_-]{1,96}$`)

// This is an allowlisted management bridge, not a general URL/path proxy. Both
// the local trusted actor and the selected backing's own identity must permit
// the operation. No local actor, policy, browser cookie or bearer is forwarded.
func (s *Server) registerWorkspaceAdmin(mux Registrar) {
	for _, route := range []struct{ method, suffix, remote, kind string }{
		{"GET", "/me", "/v1/auth/me", "me"},
		{"GET", "/users", "/v1/admin/users", "users"},
		{"POST", "/users", "/v1/admin/users", "create-user"},
		{"GET", "/invitations", "/v1/admin/invitations", "invitations"},
		{"POST", "/invitations", "/v1/admin/invitations", "create-invitation"},
		{"DELETE", "/invitations/{id}", "/v1/admin/invitations/{id}", "revoke-invitation"},
		{"PATCH", "/users/{id}", "/v1/admin/users/{id}", "update-user"},
		{"GET", "/policy", "/v1/admin/policy", "policy"},
		{"PATCH", "/policy", "/v1/admin/policy", "policy"},
		{"GET", "/audit", "/v1/admin/audit", "audit"},
		{"GET", "/users/{id}/runtime", "/v1/admin/users/{id}/runtime", "runtime"},
		{"POST", "/users/{id}/runtime/stop", "/v1/admin/users/{id}/runtime/stop", "runtime"},
	} {
		mux.HandleFunc(route.method+" /v1/setup/workspace/admin"+route.suffix, func(w http.ResponseWriter, r *http.Request) {
			s.workspaceAdmin(w, r, route.remote, route.kind)
		})
	}
}

func (s *Server) workspaceAdmin(w http.ResponseWriter, r *http.Request, remotePath, kind string) {
	actor, _, trusted := management.ActorFromContext(r.Context())
	if !trusted || actor.Disabled || (actor.Role != "admin" && actor.Role != "super-admin") {
		problem(w, 403, "permission_denied", "An administrator manages backing accounts and policy.")
		return
	}
	if r.URL.RawQuery != "" || r.URL.RawPath != "" || r.URL.Fragment != "" {
		problem(w, 400, "invalid_request", "Use the selected backing and a canonical administration path without query parameters.")
		return
	}
	if strings.Contains(remotePath, "{id}") {
		id := r.PathValue("id")
		if !workspaceAdminID.MatchString(id) {
			problem(w, 400, "invalid_user", "Choose a valid backing user.")
			return
		}
		remotePath = strings.Replace(remotePath, "{id}", id, 1)
	}
	s.mu.RLock()
	ws := s.workspace
	t := s.tunnels[ws.ConnectionID]
	address, token := "", ""
	if ws.Mode == "cloud" && ws.Ready && t != nil && t.Status == "connected" && t.KnownHost != "" {
		address = t.APIURL
		token = s.workspaceAuth[workspaceCredentialKey(*t)].Token
	}
	s.mu.RUnlock()
	if ws.Mode != "cloud" {
		problem(w, 409, "cloud_workspace_required", "Select a cloud backing before managing its accounts.")
		return
	}
	if address == "" {
		problem(w, 503, "workspace_disconnected", "Reconnect the selected backing through verified SSH.")
		return
	}
	if token == "" {
		problem(w, 409, "backing_sign_in_required", "Sign in to the selected backing in Settings.")
		return
	}
	body, password, ok := workspaceAdminBody(w, r, kind)
	if !ok {
		return
	}
	base, err := url.Parse(address)
	if err != nil || base.Scheme != "http" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		problem(w, 503, "workspace_disconnected", "The backing has no valid private API forward.")
		return
	}
	host, port, err := net.SplitHostPort(base.Host)
	portNumber, portErr := strconv.Atoi(port)
	if err != nil || portErr != nil || portNumber < 1 || portNumber > 65535 || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() {
		problem(w, 503, "workspace_disconnected", "The backing API must use its loopback SSH forward.")
		return
	}
	base.Path, base.RawPath = remotePath, ""
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	method := r.Method
	if method == http.MethodHead {
		method = http.MethodGet // Validate the same public DTO; HTTP suppresses the returned body.
	}
	req, err := http.NewRequestWithContext(ctx, method, base.String(), bytes.NewReader(body))
	if err != nil {
		problem(w, 503, "workspace_disconnected", "The backing administration request could not be prepared.")
		return
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if len(body) > 0 {
		req.Header.Set("Content-Type", "application/json")
	}
	client := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	response, err := client.Do(req)
	if err != nil {
		problem(w, 503, "workspace_disconnected", "The backing administration API did not answer over SSH.")
		return
	}
	defer response.Body.Close()
	if response.StatusCode == 401 {
		problem(w, 409, "backing_sign_in_required", "Sign in to the selected backing in Settings.")
		return
	}
	contents, err := io.ReadAll(io.LimitReader(response.Body, workspaceAdminLimit+1))
	if err != nil || len(contents) > workspaceAdminLimit {
		problem(w, 502, "invalid_backing_response", "The backing administration response exceeded its limit.")
		return
	}
	var output any
	if response.StatusCode >= 400 {
		output = &struct {
			Error   string `json:"error"`
			Message string `json:"message"`
		}{}
	} else if response.StatusCode < 200 || response.StatusCode >= 300 {
		problem(w, 502, "invalid_backing_response", "The backing must answer directly without redirects.")
		return
	} else {
		switch kind {
		case "me":
			output = &struct {
				User        management.User        `json:"user"`
				Permissions management.Permissions `json:"permissions"`
				Policy      management.Policy      `json:"policy"`
			}{}
		case "users":
			output = &struct {
				Users []management.User `json:"users"`
			}{}
		case "create-user", "update-user":
			output = &management.User{}
		case "invitations":
			output = &struct {
				Invitations []management.Invitation `json:"invitations"`
			}{}
		case "create-invitation":
			output = &struct {
				Invitation management.Invitation `json:"invitation"`
				InviteCode string                `json:"inviteCode"`
			}{}
		case "revoke-invitation":
			output = &struct {
				Revoked bool `json:"revoked"`
			}{}
		case "policy":
			output = &management.Policy{}
		case "audit":
			output = &struct {
				Events []management.AuditEvent `json:"events"`
			}{}
		case "runtime":
			output = &management.RuntimeStatus{}
		}
	}
	if len(contents) == 0 || bytes.Equal(bytes.TrimSpace(contents), []byte("null")) || json.Unmarshal(contents, output) != nil {
		problem(w, 502, "invalid_backing_response", "The backing returned invalid administration data.")
		return
	}
	// Only the public DTO fields are returned. Also redact an exact request
	// password/session token if an unexpected remote response reflects it.
	data, _ := json.Marshal(output)
	var public any
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	_ = decoder.Decode(&public)
	data, _ = json.Marshal(redactWorkspaceAdminStrings(public, []string{token, password}))
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(response.StatusCode)
	_, _ = w.Write(append(data, '\n'))
}

func redactWorkspaceAdminStrings(value any, secrets []string) any {
	switch value := value.(type) {
	case string:
		for _, secret := range secrets {
			if secret != "" {
				value = strings.ReplaceAll(value, secret, "[redacted]")
			}
		}
		return value
	case map[string]any:
		for key, child := range value {
			value[key] = redactWorkspaceAdminStrings(child, secrets)
		}
	case []any:
		for index, child := range value {
			value[index] = redactWorkspaceAdminStrings(child, secrets)
		}
	}
	return value
}

func workspaceAdminBody(w http.ResponseWriter, r *http.Request, kind string) ([]byte, string, bool) {
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, workspaceAdminLimit))
	if err != nil {
		problem(w, 413, "request_too_large", "Backing administration requests must be at most 1 MiB.")
		return nil, "", false
	}
	if r.Method == "GET" || r.Method == "HEAD" {
		if len(data) != 0 {
			problem(w, 400, "invalid_request", "Administration reads do not accept a request body.")
			return nil, "", false
		}
		return nil, "", true
	}
	if len(data) == 0 && (kind == "runtime" || kind == "revoke-invitation") {
		data = []byte("{}")
	}
	var fields map[string]json.RawMessage
	if json.Unmarshal(data, &fields) != nil || fields == nil {
		problem(w, 400, "invalid_json", "Send one JSON object with the administration fields.")
		return nil, "", false
	}
	allowed := map[string]bool{}
	switch kind {
	case "create-invitation":
		allowed["role"] = true
	case "create-user":
		for _, key := range []string{"username", "displayName", "password", "role"} {
			allowed[key] = true
		}
	case "update-user":
		for _, key := range []string{"role", "disabled", "password"} {
			allowed[key] = true
		}
	case "policy":
		policy, _ := json.Marshal(management.Policy{})
		var keys map[string]json.RawMessage
		_ = json.Unmarshal(policy, &keys)
		for key := range keys {
			allowed[key] = true
		}
	}
	for key := range fields {
		if !allowed[key] {
			problem(w, 400, "invalid_request", "The request contains unsupported administration fields.")
			return nil, "", false
		}
	}
	password := ""
	if raw, exists := fields["password"]; exists && json.Unmarshal(raw, &password) != nil {
		problem(w, 400, "invalid_request", "The password must be a string.")
		return nil, "", false
	}
	return data, password, true
}

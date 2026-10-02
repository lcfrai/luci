package product

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
)

type workspaceCredential struct {
	Token string          `json:"token"`
	User  management.User `json:"user"`
}
type remoteControlStatus struct {
	Product           string `json:"product"`
	Version           string `json:"version"`
	BootstrapRequired bool   `json:"bootstrapRequired"`
}

func workspaceCredentialKey(t Tunnel) string {
	sum := sha256.Sum256([]byte(t.Request.Host + "\x00" + t.Request.User + "\x00" + t.KnownHost))
	return hex.EncodeToString(sum[:])
}
func (s *Server) initWorkspaceAuth() error {
	s.workspaceAuth = map[string]workspaceCredential{}
	data, err := os.ReadFile(filepath.Join(s.stateDir, "workspace-auth.json"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return errors.New("Cloud backing credentials exceed their limit.")
	}
	if err = json.Unmarshal(data, &s.workspaceAuth); err != nil {
		return err
	}
	if s.workspaceAuth == nil {
		s.workspaceAuth = map[string]workspaceCredential{}
	}
	return nil
}
func (s *Server) registerWorkspaceAuth(mux Registrar) {
	mux.HandleFunc("GET /v1/setup/workspace/auth", s.getWorkspaceAuth)
	mux.HandleFunc("POST /v1/setup/workspace/auth", s.loginWorkspace)
	mux.HandleFunc("DELETE /v1/setup/workspace/auth", s.logoutWorkspace)
}
func (s *Server) connectedWorkspace(id string) (Tunnel, workspaceCredential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t := s.tunnels[id]
	if t == nil || t.Status != "connected" {
		return Tunnel{}, workspaceCredential{}, errors.New("Connect the backing through verified SSH first.")
	}
	return *t, s.workspaceAuth[workspaceCredentialKey(*t)], nil
}

// Remote API addresses are generated private SSH forwards. Never send credentials to a browser-supplied URL or follow redirects.
func remoteWorkspaceAPI(ctx context.Context, address, method, path, token string, body any, out any) error {
	base, err := url.Parse(address)
	if err != nil || base.Scheme != "http" || base.User != nil || base.RawQuery != "" || base.Fragment != "" || (base.Path != "" && base.Path != "/") {
		return errors.New("The backing has no valid private API forward.")
	}
	host, port, err := net.SplitHostPort(base.Host)
	if err != nil || net.ParseIP(host) == nil || !net.ParseIP(host).IsLoopback() || port == "" {
		return errors.New("The backing API must use a loopback SSH forward.")
	}
	var data []byte
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, address+path, bytes.NewReader(data))
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	client := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	res, err := client.Do(req)
	if err != nil {
		return errors.New("The cloud backing API did not answer over SSH.")
	}
	defer res.Body.Close()
	contents, err := io.ReadAll(io.LimitReader(res.Body, (1<<20)+1))
	if err != nil || len(contents) > 1<<20 {
		return errors.New("The backing response exceeded its limit.")
	}
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		var p struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(contents, &p)
		if res.StatusCode == 401 {
			return errors.New("Sign in to this backing again.")
		}
		if p.Message == "" {
			p.Message = "The backing refused this operation."
		}
		return errors.New(p.Message)
	}
	if out != nil && json.Unmarshal(contents, out) != nil {
		return errors.New("The backing returned an invalid response.")
	}
	return nil
}
func readRemoteControlStatus(ctx context.Context, address string) (remoteControlStatus, error) {
	var status remoteControlStatus
	err := remoteWorkspaceAPI(ctx, address, "GET", "/v1/control-plane/status", "", nil, &status)
	if err == nil && (status.Product != "Luci" || status.Version != Version) {
		err = errors.New("Install the matching Luci control plane on this backing.")
	}
	return status, err
}
func (s *Server) getWorkspaceAuth(w http.ResponseWriter, r *http.Request) {
	t, credential, err := s.connectedWorkspace(r.URL.Query().Get("connectionId"))
	if err != nil {
		problem(w, 409, "connection_required", err.Error())
		return
	}
	status, err := readRemoteControlStatus(r.Context(), t.APIURL)
	if err != nil {
		problem(w, 409, "backing_unavailable", err.Error())
		return
	}
	result := map[string]any{"authenticated": false, "bootstrapRequired": status.BootstrapRequired, "bootstrapTokenAvailable": status.BootstrapRequired && t.Request.User != "", "message": "Sign in to this backing before activating it."}
	if status.BootstrapRequired {
		result["message"] = "Create the first Super Admin on this backing. The bootstrap code can be read through your verified SSH connection."
	}
	if credential.Token != "" {
		var me struct {
			User management.User `json:"user"`
		}
		if remoteWorkspaceAPI(r.Context(), t.APIURL, "GET", "/v1/auth/me", credential.Token, nil, &me) == nil {
			result["authenticated"] = true
			result["user"] = me.User
			result["message"] = "Signed in to this backing."
		}
	}
	writeJSON(w, 200, result)
}
func (s *Server) loginWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConnectionID   string `json:"connectionId"`
		Username       string `json:"username"`
		Password       string `json:"password"`
		BootstrapToken string `json:"bootstrapToken"`
		DisplayName    string `json:"displayName"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	t, _, err := s.connectedWorkspace(req.ConnectionID)
	if err != nil {
		problem(w, 409, "connection_required", err.Error())
		return
	}
	status, err := readRemoteControlStatus(r.Context(), t.APIURL)
	if err != nil {
		problem(w, 409, "backing_unavailable", err.Error())
		return
	}
	endpoint := "/v1/auth/login"
	input := map[string]string{"username": req.Username, "password": req.Password}
	if status.BootstrapRequired {
		if req.BootstrapToken == "" {
			// Only the fixed installer-owned state path is read. No remote status filepath becomes a shell command.
			if !sshUserPattern.MatchString(t.Request.User) {
				problem(w, 400, "invalid_ssh_user", "The SSH identity is invalid.")
				return
			}
			args := append(sshArguments(t, false), t.Request.User+"@"+t.Request.Host, "cat /home/"+t.Request.User+"/.local/state/luci/control-plane/bootstrap-token")
			ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
			code, e := s.runner(ctx, "ssh", args, "")
			cancel()
			if e != nil || len(strings.TrimSpace(code)) < 40 || len(code) > 128 {
				problem(w, 409, "bootstrap_code_unavailable", "The installed backing's bootstrap code could not be read through SSH. Enter its code explicitly.")
				return
			}
			req.BootstrapToken = strings.TrimSpace(code)
		}
		endpoint = "/v1/auth/bootstrap"
		input["bootstrapToken"] = req.BootstrapToken
		input["displayName"] = req.DisplayName
	}
	var response struct {
		Token string          `json:"token"`
		User  management.User `json:"user"`
	}
	if err = remoteWorkspaceAPI(r.Context(), t.APIURL, "POST", endpoint, "", input, &response); err != nil {
		problem(w, 409, "backing_sign_in_failed", err.Error())
		return
	}
	if len(response.Token) < 40 || response.User.ID == "" {
		problem(w, 502, "invalid_backing_identity", "The backing returned an invalid identity.")
		return
	}
	s.mu.Lock()
	key := workspaceCredentialKey(t)
	before, existed := s.workspaceAuth[key]
	s.workspaceAuth[key] = workspaceCredential{Token: response.Token, User: response.User}
	err = atomicJSON(filepath.Join(s.stateDir, "workspace-auth.json"), s.workspaceAuth)
	if err != nil {
		if existed {
			s.workspaceAuth[key] = before
		} else {
			delete(s.workspaceAuth, key)
		}
	}
	s.mu.Unlock()
	if err != nil {
		problem(w, 500, "save_failed", "Backing sign-in succeeded but its private session could not be saved.")
		return
	}
	writeJSON(w, 200, map[string]any{"authenticated": true, "user": response.User, "bootstrapRequired": false, "message": "Signed in to this backing. You can activate it now."})
}
func (s *Server) logoutWorkspace(w http.ResponseWriter, r *http.Request) {
	t, credential, err := s.connectedWorkspace(r.URL.Query().Get("connectionId"))
	if err != nil {
		problem(w, 409, "connection_required", err.Error())
		return
	}
	if credential.Token != "" {
		_ = remoteWorkspaceAPI(r.Context(), t.APIURL, "POST", "/v1/auth/logout", credential.Token, map[string]any{}, nil)
	}
	s.mu.Lock()
	delete(s.workspaceAuth, workspaceCredentialKey(t))
	err = atomicJSON(filepath.Join(s.stateDir, "workspace-auth.json"), s.workspaceAuth)
	s.mu.Unlock()
	if err != nil {
		problem(w, 500, "save_failed", "The private backing session could not be removed.")
		return
	}
	writeJSON(w, 200, map[string]any{"authenticated": false, "message": "Backing session removed."})
}

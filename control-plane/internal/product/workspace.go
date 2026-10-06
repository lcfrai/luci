package product

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/previewgateway"
)

type Workspace struct {
	Mode         string `json:"mode"`
	ConnectionID string `json:"connectionId"`
	Host         string `json:"host"`
	Ready        bool   `json:"ready"`
}
type savedWorkspace struct {
	Workspace Workspace  `json:"workspace"`
	Request   SSHRequest `json:"request"`
	KnownHost string     `json:"knownHost"`
}
type WorkspaceInstall struct {
	ID           string    `json:"id"`
	ConnectionID string    `json:"connectionId"`
	Status       string    `json:"status"`
	Message      string    `json:"message"`
	CreatedAt    time.Time `json:"createdAt"`
	UpdatedAt    time.Time `json:"updatedAt"`
}
type workspacePreview struct {
	projectID, releaseID                                       string
	remoteUserID, clientKind, clientResourceID, publishableKey string
	server                                                     *http.Server
	url                                                        string
	connectionID                                               string
}

func (s *Server) initWorkspace() error {
	if err := s.initWorkspaceAuth(); err != nil {
		return err
	}
	s.workspace = Workspace{Mode: "local", Ready: true}
	s.workspaceInstalls = map[string]WorkspaceInstall{}
	s.previews = map[string]*workspacePreview{}
	s.workspaceContext, s.workspaceCancel = context.WithCancel(context.Background())
	if data, err := os.ReadFile(filepath.Join(s.stateDir, "workspace-installs.json")); err == nil {
		if len(data) > 1<<20 {
			return errors.New("workspace install history exceeds its limit")
		}
		if err = json.Unmarshal(data, &s.workspaceInstalls); err != nil {
			return err
		}
		if s.workspaceInstalls == nil {
			s.workspaceInstalls = map[string]WorkspaceInstall{}
		}
		for id, j := range s.workspaceInstalls {
			if j.Status == "installing" {
				j.Status = "unknown"
				j.Message = "Luci restarted during installation. Check the selected machine before retrying."
				s.workspaceInstalls[id] = j
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return err
	}
	data, err := os.ReadFile(filepath.Join(s.stateDir, "workspace.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) > 32<<10 {
		return errors.New("workspace settings exceed their limit")
	}
	var saved savedWorkspace
	if err = json.Unmarshal(data, &saved); err != nil {
		return err
	}
	if saved.Workspace.Mode != "cloud" {
		return nil
	}
	if err = validateSSH(saved.Request); err != nil {
		return err
	}
	fields := strings.Fields(saved.KnownHost)
	host := saved.Request.Host
	if saved.Request.Port != 22 {
		host = fmt.Sprintf("[%s]:%d", host, saved.Request.Port)
	}
	if len(fields) != 3 || fields[0] != host {
		return errors.New("saved SSH host key is invalid")
	}
	key, err := base64.StdEncoding.DecodeString(fields[2])
	if err != nil || len(key) < 32 || len(key) > 16384 {
		return errors.New("saved SSH host key is invalid")
	}
	s.workspace = saved.Workspace
	s.workspace.Ready = false
	// Startup may reconnect a previously accepted key. A replacement key is never accepted automatically.
	go s.reconnectWorkspace(saved)
	return nil
}
func (s *Server) reconnectWorkspace(saved savedWorkspace) {
	select {
	case <-s.workspaceContext.Done():
		return
	default:
	}
	id := newID("ssh-")
	s.mu.Lock()
	s.scans[id] = SSHScan{ID: id, Host: saved.Request.Host, Request: saved.Request, CreatedAt: time.Now(), Keys: map[string]string{"saved": saved.KnownHost}}
	s.mu.Unlock()
	data, _ := json.Marshal(map[string]string{"scanId": id, "acceptedFingerprint": "saved"})
	// Reuse the exact public connection validation and lifecycle, including strict known-host checking.
	w := httptest.NewRecorder()
	s.connectSSH(w, httptest.NewRequest("POST", "/v1/setup/ssh/connect", bytes.NewReader(data)).WithContext(s.workspaceContext))
	if w.Code != 202 {
		return
	}
	var t Tunnel
	if json.Unmarshal(w.Body.Bytes(), &t) != nil {
		return
	}
	s.mu.Lock()
	if s.workspace.Mode != "cloud" || s.workspace.Host != saved.Workspace.Host {
		s.mu.Unlock()
		return
	}
	s.workspace.ConnectionID = t.ID
	s.mu.Unlock()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for i := 0; i < 30; i++ {
		select {
		case <-s.workspaceContext.Done():
			return
		case <-ticker.C:
		}
		if workspaceReady(s.workspaceContext, t.APIURL) == nil {
			s.mu.Lock()
			if s.workspace.Mode == "cloud" && s.workspace.ConnectionID == t.ID {
				s.workspace.Ready = true
			}
			s.mu.Unlock()
			return
		}
	}
}
func (s *Server) registerWorkspace(mux Registrar) {
	s.registerWorkspaceAuth(mux)
	s.registerWorkspaceAdmin(mux)
	mux.HandleFunc("POST /v1/setup/ssh/connections/{connectionId}/install-luci", s.installCloudWorkspace)
	mux.HandleFunc("GET /v1/setup/workspace/install", func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		jobs := []WorkspaceInstall{}
		for _, j := range s.workspaceInstalls {
			jobs = append(jobs, j)
		}
		s.mu.RUnlock()
		writeJSON(w, 200, map[string]any{"jobs": jobs})
	})
	mux.HandleFunc("POST /v1/setup/workspace/activate", s.activateWorkspace)
	mux.HandleFunc("POST /v1/setup/workspace/local", s.activateLocalWorkspace)
	mux.HandleFunc("POST /v1/setup/workspace/preview", s.prepareWorkspacePreview)
	mux.HandleFunc("POST /v1/setup/workspace/platform/browser", s.prepareWorkspacePlatformBrowser)
}
func remoteWorkspaceStatus(ctx context.Context, address, method string, tokens ...string) (Status, error) {
	timeout := 45 * time.Second
	if method == "GET" {
		timeout = 5 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	path := "/v1/setup/status"
	if method == "POST" {
		path = "/v1/setup/check"
	}
	req, err := http.NewRequestWithContext(ctx, method, address+path, nil)
	if err != nil {
		return Status{}, err
	}
	if len(tokens) > 0 && tokens[0] != "" {
		req.Header.Set("Authorization", "Bearer "+tokens[0])
	}
	client := &http.Client{Timeout: 45 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(req)
	if err != nil {
		return Status{}, err
	}
	defer res.Body.Close()
	var status Status
	if res.StatusCode != 200 || json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&status) != nil || status.Product != "Luci" || status.Version != Version {
		return Status{}, errors.New("the remote Luci service is not ready or has a different version")
	}
	return status, nil
}
func workspaceReady(ctx context.Context, address string) error {
	_, err := readRemoteControlStatus(ctx, address)
	return err
}
func (s *Server) workspaceStatus(ctx context.Context, method string) Status {
	local := s.statusForActor(ctx)
	if local.Workspace.Mode != "cloud" {
		return local
	}
	s.mu.RLock()
	t := s.tunnels[local.Workspace.ConnectionID]
	address := ""
	if t != nil && t.Status == "connected" {
		address = t.APIURL
	}
	cached := s.remoteStatus
	token := ""
	if t != nil {
		token = s.workspaceAuth[workspaceCredentialKey(*t)].Token
	}
	s.mu.RUnlock()
	remote, err := remoteWorkspaceStatus(ctx, address, method, token)
	if address != "" && err == nil {
		s.mu.Lock()
		s.remoteStatus = &remote
		s.mu.Unlock()
		local.Workspace.Ready = true
	} else {
		local.Workspace.Ready = false
		if cached != nil {
			remote = *cached
			remote.Checks = append([]Check{}, cached.Checks...)
		} else {
			remote.Config = Config{ProjectsDir: "/home/luci/projects", Mode: "local", MonthlyBudgetUSD: 30}
			remote.Checks = defaultChecks()
		}
		for i := range remote.Checks {
			if remote.Checks[i].ID != "aws" && remote.Checks[i].ID != "azure" {
				remote.Checks[i].Status = "missing"
				remote.Checks[i].Message = "Cloud workspace is disconnected."
			}
		}
	}
	local.Config = remote.Config
	local.Configured = remote.Configured
	local.Access = remote.Access
	local.Platform = remote.Platform
	local.Checks = append([]Check{}, remote.Checks...)
	for _, probe := range s.status().Checks {
		if probe.ID == "aws" || probe.ID == "azure" {
			found := false
			for i := range local.Checks {
				if local.Checks[i].ID == probe.ID {
					local.Checks[i] = probe
					found = true
				}
			}
			if !found {
				local.Checks = append(local.Checks, probe)
			}
		}
	}
	return local
}
func (s *Server) activateWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ConnectionID string `json:"connectionId"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	s.mu.RLock()
	t, ok := s.tunnels[req.ConnectionID]
	var address, host, key string
	var ssh SSHRequest
	if ok && t.Status == "connected" {
		address = t.APIURL
		host = t.Host
		key = t.KnownHost
		ssh = t.Request
	}
	s.mu.RUnlock()
	if !ok || address == "" {
		problem(w, 409, "connection_required", "Connect this machine through SSH first.")
		return
	}
	if err := workspaceReady(r.Context(), address); err != nil {
		problem(w, 409, "workspace_not_ready", "Install the matching Luci runtime on this machine, then activate it.")
		return
	}
	_, credential, authErr := s.connectedWorkspace(req.ConnectionID)
	var remoteIdentity struct {
		User management.User `json:"user"`
	}
	if authErr != nil || credential.Token == "" || remoteWorkspaceAPI(r.Context(), address, "GET", "/v1/auth/me", credential.Token, nil, &remoteIdentity) != nil {
		problem(w, 409, "backing_sign_in_required", "Sign in to this backing before activating it.")
		return
	}
	ws := Workspace{Mode: "cloud", ConnectionID: req.ConnectionID, Host: host, Ready: true}
	if err := atomicJSON(filepath.Join(s.stateDir, "workspace.json"), savedWorkspace{ws, ssh, key}); err != nil {
		problem(w, 500, "save_failed", "Could not save this workspace.")
		return
	}
	s.closePreviews()
	s.mu.Lock()
	s.workspace = ws
	s.remoteStatus = nil
	s.mu.Unlock()
	writeJSON(w, 200, s.workspaceStatus(r.Context(), "GET"))
}
func (s *Server) activateLocalWorkspace(w http.ResponseWriter, r *http.Request) {
	ws := Workspace{Mode: "local", Ready: true}
	if err := atomicJSON(filepath.Join(s.stateDir, "workspace.json"), savedWorkspace{Workspace: ws}); err != nil {
		problem(w, 500, "save_failed", "Could not save this workspace.")
		return
	}
	s.closePreviews()
	s.mu.Lock()
	s.workspace = ws
	s.remoteStatus = nil
	s.mu.Unlock()
	writeJSON(w, 200, s.statusForActor(r.Context()))
}
func (s *Server) installCloudWorkspace(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AcknowledgeInstall bool `json:"acknowledgeInstall"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !req.AcknowledgeInstall {
		problem(w, 400, "install_acknowledgement_required", "Accept installation of Luci and its system service on your selected machine.")
		return
	}
	s.mu.Lock()
	t, ok := s.tunnels[r.PathValue("connectionId")]
	if !ok || t.Status != "connected" {
		s.mu.Unlock()
		problem(w, 409, "connection_required", "Connect the server through SSH first.")
		return
	}
	for _, j := range s.workspaceInstalls {
		if j.ConnectionID == t.ID && j.Status == "installing" {
			s.mu.Unlock()
			writeJSON(w, 200, j)
			return
		}
	}
	info, err := os.Lstat(s.cloudBundle)
	if err != nil || !info.Mode().IsRegular() || info.Size() > 300<<20 {
		s.mu.Unlock()
		problem(w, 503, "bundle_unavailable", "This build does not contain the cloud runtime bundle. Install an official packaged Luci build.")
		return
	}
	job := WorkspaceInstall{ID: newID("install-"), ConnectionID: t.ID, Status: "installing", Message: "Transferring the bundled runtime through verified SSH.", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	s.workspaceInstalls[job.ID] = job
	if err := atomicJSON(filepath.Join(s.stateDir, "workspace-installs.json"), s.workspaceInstalls); err != nil {
		delete(s.workspaceInstalls, job.ID)
		s.mu.Unlock()
		problem(w, 500, "save_failed", "Could not record this installation. Nothing was transferred.")
		return
	}
	copyTunnel := *t
	s.mu.Unlock()
	go s.installWorkspace(job, copyTunnel)
	writeJSON(w, 202, job)
}
func (s *Server) finishWorkspaceInstall(job WorkspaceInstall, status, message string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job.Status = status
	job.Message = message
	job.UpdatedAt = time.Now().UTC()
	s.workspaceInstalls[job.ID] = job
	_ = atomicJSON(filepath.Join(s.stateDir, "workspace-installs.json"), s.workspaceInstalls)
}
func (s *Server) installWorkspace(job WorkspaceInstall, t Tunnel) {
	ctx, cancel := context.WithTimeout(s.workspaceContext, 15*time.Minute)
	defer cancel()
	file, err := os.Open(s.cloudBundle)
	if err != nil {
		s.finishWorkspaceInstall(job, "failed", "Could not read the cloud runtime bundle.")
		return
	}
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	file.Close()
	if err != nil {
		s.finishWorkspaceInstall(job, "failed", "Could not verify the runtime bundle.")
		return
	}
	checksum := hex.EncodeToString(hash.Sum(nil))
	destination := "/home/" + t.Request.User + "/" + job.ID + ".tar.gz"
	args := sshArguments(t, true)
	host := t.Request.Host
	if net.ParseIP(host) != nil && strings.Contains(host, ":") {
		host = "[" + host + "]"
	}
	args = append(args, s.cloudBundle, t.Request.User+"@"+host+":"+destination)
	if _, err = s.runner(ctx, "scp", args, ""); err != nil {
		s.finishWorkspaceInstall(job, "failed", "Transfer failed. Check the SSH connection and available server disk space.")
		return
	}
	script := fmt.Sprintf(`set -euo pipefail
printf '%%s  %%s\n' '%s' '%s' | sha256sum --check --status
install -d -m 755 /opt/luci /opt/luci/releases /opt/luci/releases/%s
tar --extract --gzip --file '%s' --directory /opt/luci/releases/%s --no-same-owner
chmod 755 /opt/luci/releases/%s/luci
ln -sfn /opt/luci/releases/%s /opt/luci/current
cat > /etc/systemd/system/luci.service <<'UNIT'
[Unit]
Description=Luci cloud workspace
After=docker.service network-online.target
[Service]
User=%s
SupplementaryGroups=docker
Environment=HOME=/home/%s
Environment=LUCI_STATE_DIR=/home/%s/.local/state/luci
Environment=LUCI_PROJECTS_DIR=/home/%s/projects
Environment=LUCI_ADDR=127.0.0.1:8730
Environment=LUCI_SESSION_CPU_MILLIS=1000
Environment=LUCI_SESSION_MEMORY_MIB=1536
ExecStart=/opt/luci/current/luci
Restart=on-failure
UMask=0077
[Install]
WantedBy=multi-user.target
UNIT
install -d -m 700 -o %s /home/%s/projects /home/%s/.local/state/luci
systemctl daemon-reload
systemctl enable luci
systemctl restart luci
rm -f -- '%s'
`, checksum, destination, job.ID, destination, job.ID, job.ID, job.ID, t.Request.User, t.Request.User, t.Request.User, t.Request.User, t.Request.User, t.Request.User, t.Request.User, destination)
	args = sshArguments(t, false)
	args = append(args, t.Request.User+"@"+t.Request.Host, "sudo -n /usr/bin/bash -c "+shellQuote(script))
	if _, err = s.runner(ctx, "ssh", args, ""); err != nil {
		s.finishWorkspaceInstall(job, "failed", "Installation failed. The SSH account needs sudo access; inspect the selected server before retrying.")
		return
	}
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for i := 0; i < 20; i++ {
		if workspaceReady(ctx, t.APIURL) == nil {
			s.finishWorkspaceInstall(job, "ready", "Luci is ready on your machine. Activate the cloud workspace, install its agent runtime and connect your agent account.")
			return
		}
		select {
		case <-ctx.Done():
			s.finishWorkspaceInstall(job, "unknown", "Installation stopped before readiness could be confirmed. Check the selected machine.")
			return
		case <-ticker.C:
		}
	}
	s.finishWorkspaceInstall(job, "unknown", "Installation ran, but the remote service has not answered. Check server logs before retrying.")
}
func shellQuote(v string) string { return "'" + strings.ReplaceAll(v, "'", "'\"'\"'") + "'" }
func sshArguments(t Tunnel, copy bool) []string {
	portFlag := "-p"
	if copy {
		portFlag = "-P"
	}
	args := []string{portFlag, fmt.Sprint(t.Request.Port), "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=" + t.KnownHostsPath, "-o", "BatchMode=yes", "-o", "ConnectTimeout=10"}
	if t.Request.PrivateKeyPath != "" {
		args = append(args, "-i", t.Request.PrivateKeyPath)
	}
	return args
}

func projectRequest(path string) bool {
	for _, prefix := range []string{"/v1/platform", "/v1/product/projects", "/v1/projects", "/v1/sessions", "/v1/harnesses", "/v1/machines", "/v1/setup/agents", "/v1/setup/agent-connections", "/v1/setup/github", "/v1/setup/git/identity", "/v1/setup/runtime/install", "/v1/setup/targets", "/v1/setup/config"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}

// ProxyWorkspace moves project APIs as a unit. Account provisioning stays on the client machine.
func (s *Server) ProxyWorkspace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/api")
		if (strings.Contains(path, "/releases/") && strings.Contains(path, "/preview/")) || (strings.HasPrefix(path, "/v1/platform/projects/") && strings.Contains(path, "/browser/")) {
			if r.Header.Get("X-Luci-Preview-Bridge") == "1" && r.Header.Get("Sec-Fetch-Site") == "" && r.Header.Get("Sec-Fetch-Mode") == "" && r.Header.Get("Sec-Fetch-Dest") == "" {
				s.mu.RLock()
				local := s.workspace.Mode != "cloud"
				s.mu.RUnlock()
				if local {
					next.ServeHTTP(w, r)
					return
				}
			}
			problem(w, 403, "isolated_preview_required", "Open this application through its separate preview address.")
			return
		}
		if !projectRequest(path) {
			next.ServeHTTP(w, r)
			return
		}
		s.mu.RLock()
		ws := s.workspace
		t := s.tunnels[ws.ConnectionID]
		address, management := "", ""
		if t != nil && t.Status == "connected" {
			address = t.APIURL
			management = t.LocalURL
		}
		token := ""
		if t != nil {
			token = s.workspaceAuth[workspaceCredentialKey(*t)].Token
		}
		s.mu.RUnlock()
		if ws.Mode != "cloud" {
			next.ServeHTTP(w, r)
			return
		}
		if address == "" {
			problem(w, 503, "workspace_disconnected", "Reconnect your cloud workspace in Settings, or return to this machine.")
			return
		}
		if token == "" {
			problem(w, 409, "backing_sign_in_required", "Sign in to the selected backing in Settings.")
			return
		}
		if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
			var ticket struct {
				Ticket string `json:"ticket"`
			}
			if err := remoteWorkspaceAPI(r.Context(), address, "POST", "/v1/auth/socket-ticket", token, map[string]string{"path": path}, &ticket); err != nil || ticket.Ticket == "" {
				problem(w, 409, "backing_sign_in_required", "Reconnect and sign in to this backing before opening a session.")
				return
			}
			clone := r.Clone(r.Context())
			u := *r.URL
			q := u.Query()
			q.Set("ticket", ticket.Ticket)
			u.RawQuery = q.Encode()
			clone.URL = &u
			r = clone
		}
		target, err := url.Parse(address)
		if err != nil || target.Host == "" {
			problem(w, 503, "workspace_disconnected", "The selected workspace has no valid connection.")
			return
		}
		var clientDataResource, clientDataOrigin string
		if r.Method == "POST" && strings.HasPrefix(path, "/v1/platform/projects/") && strings.HasSuffix(path, "/deployments") {
			projectID := strings.TrimSuffix(strings.TrimPrefix(path, "/v1/platform/projects/"), "/deployments")
			if !coolifyID.MatchString(projectID) {
				problem(w, 400, "browser_request_invalid", "Choose a valid project deployment.")
				return
			}
			meta, origin, err := s.cloudDeploymentBrowserData(r, ws.ConnectionID, projectID)
			if err != nil {
				problem(w, 409, "data_browser_connection_required", "Verify and connect this project's data services before deploying its application.")
				return
			}
			clientDataResource, clientDataOrigin = meta.ResourceID, origin
		}
		proxy := httputil.NewSingleHostReverseProxy(target)
		original := proxy.Director
		proxy.Director = func(req *http.Request) {
			original(req)
			req.Host = target.Host
			req.Header.Del("Origin")
			req.Header.Del("Cookie")
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Del("X-Luci-Actor")
			req.Header.Del("X-Luci-Policy")
			req.Header.Del("Forwarded")
			req.Header.Del("X-Forwarded-Host")
			for name := range req.Header {
				if strings.HasPrefix(strings.ToLower(name), "x-luci-") {
					req.Header.Del(name)
				}
			}
			if clientDataOrigin != "" {
				for name := range req.Header {
					if strings.HasPrefix(strings.ToLower(name), "sec-fetch-") {
						req.Header.Del(name)
					}
				}
				req.Header.Set("X-Luci-Preview-Bridge", "1")
				req.Header.Set("X-Luci-Workspace-Data-Origin", clientDataOrigin)
				req.Header.Set("X-Luci-Workspace-Data-Resource", clientDataResource)
			}
			req.URL.Path = path
		}
		if path == "/v1/setup/targets" && r.Method == "POST" {
			data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
			if err != nil {
				problem(w, 400, "bad_request", "Connection settings are too large.")
				return
			}
			var fields map[string]any
			if json.Unmarshal(data, &fields) != nil {
				problem(w, 400, "invalid_json", "Check the connection settings.")
				return
			}
			if fields["baseUrl"] == management {
				fields["baseUrl"] = "http://127.0.0.1:8000"
			}
			data, _ = json.Marshal(fields)
			r.Body = io.NopCloser(bytes.NewReader(data))
			r.ContentLength = int64(len(data))
		}
		proxy.ModifyResponse = func(res *http.Response) error {
			// The desktop still has a valid local session. A remote account
			// expiry must leave Settings reachable rather than logging it out.
			if res.StatusCode == http.StatusUnauthorized {
				res.Body.Close()
				data, _ := json.Marshal(map[string]string{"error": "backing_sign_in_required", "message": "Sign in to the selected backing in Settings."})
				res.StatusCode = http.StatusConflict
				res.Status = "409 Conflict"
				res.Body = io.NopCloser(bytes.NewReader(data))
				res.ContentLength = int64(len(data))
				res.Header.Set("Content-Length", fmt.Sprint(len(data)))
				res.Header.Set("Content-Type", "application/json")
				res.Header.Set("Cache-Control", "no-store")
				res.Header.Del("WWW-Authenticate")
				return nil
			}
			if res.StatusCode >= 400 {
				return nil
			}
			platformResponse := strings.HasPrefix(path, "/v1/platform/") && strings.HasPrefix(res.Header.Get("Content-Type"), "application/json")
			if path != "/v1/setup/config" && path != "/v1/setup/targets" && !((strings.Contains(path, "/releases") || strings.HasSuffix(path, "/prototype-releases")) && res.Header.Get("Content-Type") != "") && !platformResponse {
				return nil
			}
			data, err := io.ReadAll(io.LimitReader(res.Body, 8<<20+1))
			res.Body.Close()
			if err != nil || len(data) > 8<<20 {
				return errors.New("workspace response exceeds its limit")
			}
			var value any
			if err = json.Unmarshal(data, &value); err != nil {
				return err
			}
			if path == "/v1/setup/config" {
				var remote Status
				if json.Unmarshal(data, &remote) != nil {
					return errors.New("invalid workspace settings")
				}
				snapshot := remote
				snapshot.Checks = append([]Check{}, remote.Checks...)
				s.mu.Lock()
				s.remoteStatus = &snapshot
				s.mu.Unlock()
				local := s.status()
				remote.Workspace = local.Workspace
				remote.CloudProviders = local.CloudProviders
				for _, probe := range local.Checks {
					for i := range remote.Checks {
						if (probe.ID == "aws" || probe.ID == "azure") && remote.Checks[i].ID == probe.ID {
							remote.Checks[i] = probe
						}
					}
				}
				value = remote
			} else {
				s.rewriteWorkspaceResponse(value, ws.ConnectionID, management)
				if platformResponse {
					s.rewriteWorkspacePlatformResponse(value, ws.ConnectionID)
				}
			}
			data, err = json.Marshal(value)
			if err != nil {
				return err
			}
			res.Body = io.NopCloser(bytes.NewReader(data))
			res.ContentLength = int64(len(data))
			res.Header.Set("Content-Length", fmt.Sprint(len(data)))
			return nil
		}
		proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
			problem(w, 503, "workspace_disconnected", "The cloud workspace is not answering. Reconnect it in Settings.")
		}
		proxy.ServeHTTP(w, r)
	})
}
func (s *Server) rewriteWorkspaceResponse(value any, connectionID, management string) {
	switch v := value.(type) {
	case []any:
		for _, row := range v {
			s.rewriteWorkspaceResponse(row, connectionID, management)
		}
	case map[string]any:
		if v["baseUrl"] == "http://127.0.0.1:8000" {
			v["baseUrl"] = management
		}
		if v["target"] == "local" && v["status"] == "ready" {
			project, _ := v["projectId"].(string)
			release, _ := v["id"].(string)
			s.mu.RLock()
			bridge := s.previews[project+"/"+release]
			s.mu.RUnlock()
			if bridge != nil && bridge.connectionID == connectionID {
				v["url"] = bridge.url
			} else {
				v["url"] = ""
				v["message"] = "Open this preview through Luci to connect its protected address."
			}
		}
		for _, row := range v {
			s.rewriteWorkspaceResponse(row, connectionID, management)
		}
	}
}
func (s *Server) prepareWorkspacePreview(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID string `json:"projectId"`
		ReleaseID string `json:"releaseId"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	s.mu.RLock()
	ws := s.workspace
	t := s.tunnels[ws.ConnectionID]
	address := ""
	if t != nil && t.Status == "connected" {
		address = t.APIURL
	}
	token := ""
	if t != nil {
		token = s.workspaceAuth[workspaceCredentialKey(*t)].Token
	}
	s.mu.RUnlock()
	if ws.Mode != "cloud" || address == "" {
		problem(w, 409, "workspace_required", "Activate your connected cloud workspace first.")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	history, err := http.NewRequestWithContext(ctx, "GET", address+"/v1/product/projects/"+url.PathEscape(req.ProjectID)+"/releases", nil)
	if err != nil {
		problem(w, 400, "bad_request", "Choose a valid release.")
		return
	}
	history.Header.Set("Authorization", "Bearer "+token)
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	res, err := client.Do(history)
	if err != nil {
		problem(w, 503, "workspace_disconnected", "The workspace is not answering.")
		return
	}
	defer res.Body.Close()
	var list struct {
		Releases []Release `json:"releases"`
	}
	if res.StatusCode != 200 || json.NewDecoder(io.LimitReader(res.Body, 8<<20)).Decode(&list) != nil {
		problem(w, 409, "preview_not_ready", "Could not read the workspace release history.")
		return
	}
	found := false
	for _, row := range list.Releases {
		if row.ID == req.ReleaseID && row.ProjectID == req.ProjectID && row.Target == "local" && row.Status == "ready" {
			if _, err := recordedPreviewOrigin(row.URL); err == nil {
				found = true
			}
		}
	}
	if !found {
		problem(w, 409, "preview_not_ready", "Choose a running preview in this project.")
		return
	}
	remoteUserID := ""
	if t != nil {
		s.mu.RLock()
		remoteUserID = s.workspaceAuth[workspaceCredentialKey(*t)].User.ID
		s.mu.RUnlock()
	}
	key := req.ProjectID + "/" + req.ReleaseID
	s.mu.Lock()
	if s.workspace.ConnectionID != ws.ConnectionID || s.workspace.Mode != "cloud" {
		s.mu.Unlock()
		problem(w, 409, "workspace_changed", "The selected workspace changed. Open the preview again.")
		return
	}
	if bridge := s.previews[key]; bridge != nil && bridge.connectionID == ws.ConnectionID {
		s.mu.Unlock()
		writeJSON(w, 200, map[string]string{"url": bridge.url})
		return
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		s.mu.Unlock()
		problem(w, 500, "preview_unavailable", "Could not open a protected local preview address.")
		return
	}
	origin := "http://" + ln.Addr().String()
	upstream, _ := url.Parse(address)
	prefix := "/v1/product/projects/" + url.PathEscape(req.ProjectID) + "/releases/" + url.PathEscape(req.ReleaseID) + "/preview"
	proxy := httputil.NewSingleHostReverseProxy(upstream)
	proxy.Director = func(out *http.Request) {
		appAuthorization := out.Header.Get("Authorization")
		out.URL.Scheme = upstream.Scheme
		out.URL.Host = upstream.Host
		out.Host = upstream.Host
		out.URL.Path = prefix + out.URL.Path
		if out.URL.RawPath != "" {
			out.URL.RawPath = prefix + out.URL.RawPath
		}
		for name := range out.Header {
			lower := strings.ToLower(name)
			if lower == "origin" || lower == "authorization" || lower == "referer" || lower == "forwarded" || strings.HasPrefix(lower, "sec-fetch-") || strings.HasPrefix(lower, "x-luci-") || strings.HasPrefix(lower, "x-forwarded-") {
				out.Header.Del(name)
			}
		}
		out.Header.Set("X-Luci-Preview-Bridge", "1")
		if len(appAuthorization) <= 8192 && appAuthorization != "" {
			out.Header.Set("X-Luci-Preview-App-Authorization", appAuthorization)
		}
		out.Header.Set("Authorization", "Bearer "+token)
	}
	proxy.ModifyResponse = func(res *http.Response) error {
		location := res.Header.Get("Location")
		if strings.HasPrefix(location, "http://127.0.0.1:") {
			u, err := url.Parse(location)
			if err == nil {
				res.Header.Set("Location", origin+u.RequestURI())
			}
		}
		return nil
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		problem(w, 503, "preview_unavailable", "The workspace preview is not answering.")
	}
	bridgeKey := ""
	if s.browserRegistry != nil {
		bridgeKey, err = previewgateway.NewBridgeKey()
		if err != nil {
			_ = ln.Close()
			s.mu.Unlock()
			problem(w, 500, "preview_unavailable", "The private bridge could not be secured.")
			return
		}
	}
	server := &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if "http://"+r.Host != origin {
			problem(w, 403, "host_rejected", "Use the preview address shown by Luci.")
			return
		}
		if o := r.Header.Get("Origin"); o != "" && o != origin {
			problem(w, 403, "origin_rejected", "This preview accepts requests only from its own page.")
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" && r.Header.Get("Sec-Fetch-Site") == "cross-site" {
			problem(w, 403, "origin_rejected", "Open this preview before changing the application.")
			return
		}
		s.mu.RLock()
		active := s.workspace.Mode == "cloud" && s.workspace.ConnectionID == ws.ConnectionID
		connection := s.tunnels[ws.ConnectionID]
		connected := connection != nil && connection.Status == "connected"
		s.mu.RUnlock()
		if !active || !connected {
			problem(w, 503, "workspace_disconnected", "Reconnect this workspace in Luci.")
			return
		}
		if r.Method == "GET" && r.URL.Path == previewgateway.BridgeProbePath && bridgeKey != "" {
			proof := previewgateway.BridgeProof(bridgeKey, r.Header.Get(previewgateway.BridgeNonceHeader))
			if proof == "" {
				problem(w, 403, "bridge_probe_rejected", "The bridge probe is invalid.")
				return
			}
			w.Header().Set(previewgateway.BridgeProofHeader, proof)
			w.WriteHeader(204)
			return
		}
		proxy.ServeHTTP(w, r)
	})}
	browserOrigin := origin
	if s.browserRegistry != nil {
		browserOrigin, err = s.browserRegistry.Publish(previewgateway.Target{Kind: "bridge", ProjectID: req.ProjectID, ResourceID: req.ReleaseID, Upstream: origin, BridgeKey: bridgeKey})
		if err != nil {
			_ = ln.Close()
			s.mu.Unlock()
			problem(w, 500, "preview_unavailable", "The private preview browser address could not be registered.")
			return
		}
	}
	s.previews[key] = &workspacePreview{server: server, url: browserOrigin, connectionID: ws.ConnectionID, projectID: req.ProjectID, releaseID: req.ReleaseID, remoteUserID: remoteUserID}
	s.mu.Unlock()
	go server.Serve(ln)
	writeJSON(w, 200, map[string]string{"url": browserOrigin})
}
func (s *Server) closePreviews() {
	s.mu.Lock()
	previews := s.previews
	s.previews = map[string]*workspacePreview{}
	s.mu.Unlock()
	for _, preview := range previews {
		_ = preview.server.Close()
		if s.browserRegistry != nil {
			_ = s.browserRegistry.Remove("bridge", preview.projectID, preview.releaseID)
		}
	}
}
func (s *Server) closeWorkspace() {
	if s.workspaceCancel != nil {
		s.workspaceCancel()
	}
	s.closePreviews()
}

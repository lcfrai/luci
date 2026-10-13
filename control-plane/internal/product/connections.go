package product

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type DeploymentTarget struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	BaseURL    string `json:"baseUrl"`
	Token      string `json:"-"`
	ServerUUID string `json:"serverUuid"`
}
type storedTarget struct {
	Target DeploymentTarget `json:"target"`
	Token  string           `json:"token"`
}
type SSHRequest struct {
	Host           string `json:"host"`
	Port           int    `json:"port"`
	User           string `json:"user"`
	PrivateKeyPath string `json:"privateKeyPath"`
}
type SSHScan struct {
	ID           string            `json:"id"`
	Host         string            `json:"host"`
	Fingerprints []string          `json:"fingerprints"`
	CreatedAt    time.Time         `json:"createdAt"`
	Request      SSHRequest        `json:"-"`
	Keys         map[string]string `json:"-"`
}
type Tunnel struct {
	ID             string     `json:"id"`
	Host           string     `json:"host"`
	LocalURL       string     `json:"localUrl"`
	APIURL         string     `json:"apiUrl"`
	Request        SSHRequest `json:"-"`
	KnownHost      string     `json:"-"`
	KnownHostsPath string     `json:"-"`
	Status         string     `json:"status"`
	Message        string     `json:"message"`
	cmd            *exec.Cmd
}

var sshUserPattern = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)

func (s *Server) initConnections() error {
	s.targets = map[string]DeploymentTarget{}
	s.scans = map[string]SSHScan{}
	s.tunnels = map[string]*Tunnel{}
	data, err := os.ReadFile(filepath.Join(s.stateDir, "deployment-targets.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	if len(data) > 1<<20 {
		return errors.New("deployment target state exceeds its limit")
	}
	var records []storedTarget
	if err = json.Unmarshal(data, &records); err != nil {
		return err
	}
	for _, r := range records {
		r.Target.Token = r.Token
		if _, err = NewCoolifyClient(r.Target.BaseURL, r.Token); err != nil {
			return err
		}
		s.targets[r.Target.ID] = r.Target
	}
	return nil
}
func (s *Server) deploymentTarget(id string) (DeploymentTarget, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.targets[id]
	if !ok {
		return t, errors.New("Connect a deployment server in Settings first.")
	}
	return t, nil
}
func (s *Server) registerConnections(mux Registrar) {
	mux.HandleFunc("GET /v1/setup/targets", s.listTargets)
	mux.HandleFunc("POST /v1/setup/targets", s.connectTarget)
	mux.HandleFunc("POST /v1/setup/ssh/inspect", s.inspectSSH)
	mux.HandleFunc("POST /v1/setup/ssh/connect", s.connectSSH)
	mux.HandleFunc("GET /v1/setup/ssh/connections", s.listTunnels)
	mux.HandleFunc("POST /v1/setup/ssh/connections/{connectionId}/stop", s.stopTunnel)
}
func (s *Server) listTargets(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	targets := []DeploymentTarget{}
	for _, t := range s.targets {
		targets = append(targets, t)
	}
	s.mu.RUnlock()
	writeJSON(w, 200, map[string]any{"targets": targets})
}
func (s *Server) connectTarget(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ID         string `json:"id"`
		Name       string `json:"name"`
		BaseURL    string `json:"baseUrl"`
		Token      string `json:"token"`
		ServerUUID string `json:"serverUuid"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if strings.TrimSpace(req.Name) == "" || len(req.Name) > 100 {
		problem(w, 400, "bad_request", "Name this deployment connection.")
		return
	}
	client, err := NewCoolifyClient(req.BaseURL, req.Token)
	if err != nil {
		problem(w, 400, "bad_request", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 15*time.Second)
	defer cancel()
	servers, err := client.ListServers(ctx)
	if err != nil {
		problem(w, 502, "connection_failed", "Could not connect. Check the server address, SSH connection and token permissions.")
		return
	}
	found := false
	for _, server := range servers {
		if server.UUID == req.ServerUUID {
			found = true
		}
	}
	if req.ServerUUID == "" && len(servers) == 1 {
		req.ServerUUID = servers[0].UUID
		found = true
	}
	if !found {
		writeJSON(w, 409, map[string]any{"error": "choose_server", "message": "Select the server that should run your applications.", "servers": servers})
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := req.ID
	if id == "" {
		id = newID("target-")
	} else if _, ok := s.targets[id]; !ok {
		problem(w, 404, "target_not_found", "Connection not found.")
		return
	}
	t := DeploymentTarget{ID: id, Name: req.Name, BaseURL: strings.TrimRight(req.BaseURL, "/"), Token: req.Token, ServerUUID: req.ServerUUID}
	previous, existed := s.targets[id]
	s.targets[id] = t
	records := []storedTarget{}
	for _, v := range s.targets {
		records = append(records, storedTarget{v, v.Token})
	}
	if err = atomicJSON(filepath.Join(s.stateDir, "deployment-targets.json"), records); err != nil {
		if existed {
			s.targets[id] = previous
		} else {
			delete(s.targets, id)
		}
		problem(w, 500, "save_failed", "Could not save this connection.")
		return
	}
	writeJSON(w, 201, t)
}
func validateSSH(r SSHRequest) error {
	if !(validDomain(r.Host) || net.ParseIP(r.Host) != nil) || strings.ContainsAny(r.Host, "\r\n\x00") {
		return errors.New("Use a server hostname or IP address without a URL scheme.")
	}
	if r.Port < 1 || r.Port > 65535 {
		return errors.New("SSH port must be between 1 and 65535.")
	}
	if !sshUserPattern.MatchString(r.User) {
		return errors.New("Use the Linux account name created by setup, normally luci.")
	}
	if r.PrivateKeyPath != "" {
		if !filepath.IsAbs(r.PrivateKeyPath) {
			return errors.New("Choose the absolute path of your SSH private key.")
		}
		info, err := os.Lstat(r.PrivateKeyPath)
		if err != nil || !info.Mode().IsRegular() {
			return errors.New("The SSH key must be an existing regular file.")
		}
	}
	return nil
}
func (s *Server) inspectSSH(w http.ResponseWriter, r *http.Request) {
	var req SSHRequest
	if !readJSON(w, r, &req) {
		return
	}
	if req.Port == 0 {
		req.Port = 22
	}
	if req.User == "" {
		req.User = "luci"
	}
	if err := validateSSH(req); err != nil {
		problem(w, 400, "bad_request", err.Error())
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer cancel()
	raw, err := s.runner(ctx, "ssh-keyscan", []string{"-T", "5", "-p", strconv.Itoa(req.Port), req.Host}, "")
	if err != nil {
		problem(w, 502, "ssh_unavailable", "The server did not answer SSH. Wait for installation, check the firewall and try again.")
		return
	}
	scan := SSHScan{ID: newID("ssh-"), Host: req.Host, Request: req, Keys: map[string]string{}, Fingerprints: []string{}, CreatedAt: time.Now().UTC()}
	host := req.Host
	if req.Port != 22 {
		host = fmt.Sprintf("[%s]:%d", req.Host, req.Port)
	}
	for _, line := range strings.Split(raw, "\n") {
		fields := strings.Fields(line)
		if len(fields) != 3 || fields[0] != host {
			continue
		}
		data, err := base64.StdEncoding.DecodeString(fields[2])
		if err != nil || len(data) < 32 || len(data) > 16384 {
			continue
		}
		sum := sha256.Sum256(data)
		fingerprint := "SHA256:" + base64.RawStdEncoding.EncodeToString(sum[:])
		scan.Keys[fingerprint] = strings.Join(fields, " ")
		scan.Fingerprints = append(scan.Fingerprints, fingerprint)
	}
	if len(scan.Keys) == 0 {
		problem(w, 502, "ssh_unavailable", "No valid SSH host keys were returned.")
		return
	}
	s.mu.Lock()
	s.scans[scan.ID] = scan
	s.mu.Unlock()
	writeJSON(w, 200, scan)
}
func (s *Server) connectSSH(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ScanID              string `json:"scanId"`
		AcceptedFingerprint string `json:"acceptedFingerprint"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	s.mu.RLock()
	scan, ok := s.scans[req.ScanID]
	s.mu.RUnlock()
	key, verified := scan.Keys[req.AcceptedFingerprint]
	if !ok || !verified || time.Since(scan.CreatedAt) > 10*time.Minute {
		problem(w, 409, "host_verification_required", "Inspect the host, compare its fingerprint with your cloud console, then accept that exact fingerprint.")
		return
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		problem(w, 500, "connection_failed", "Could not reserve a local connection port.")
		return
	}
	port := ln.Addr().(*net.TCPAddr).Port
	_ = ln.Close()
	apiListener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		problem(w, 500, "connection_failed", "Could not reserve the workspace connection port.")
		return
	}
	apiPort := apiListener.Addr().(*net.TCPAddr).Port
	_ = apiListener.Close()
	id := newID("tunnel-")
	dir := filepath.Join(s.stateDir, "ssh")
	if err = os.MkdirAll(dir, 0700); err != nil {
		problem(w, 500, "save_failed", "Could not save the verified host key.")
		return
	}
	known := filepath.Join(dir, id+".known_hosts")
	if err = os.WriteFile(known, []byte(key+"\n"), 0600); err != nil {
		problem(w, 500, "save_failed", "Could not save the verified host key.")
		return
	}
	args := []string{"-N", "-L", fmt.Sprintf("127.0.0.1:%d:127.0.0.1:8000", port), "-L", fmt.Sprintf("127.0.0.1:%d:127.0.0.1:8730", apiPort), "-p", strconv.Itoa(scan.Request.Port), "-o", "StrictHostKeyChecking=yes", "-o", "UserKnownHostsFile=" + known, "-o", "BatchMode=yes", "-o", "ExitOnForwardFailure=yes", "-o", "ConnectTimeout=10", "-o", "ServerAliveInterval=30"}
	if scan.Request.PrivateKeyPath != "" {
		args = append(args, "-i", scan.Request.PrivateKeyPath)
	}
	args = append(args, scan.Request.User+"@"+scan.Request.Host)
	cmd := exec.Command("ssh", args...)
	hideProcess(cmd)
	cmd.Env = append(os.Environ(), "SSH_ASKPASS_REQUIRE=never")
	output := &limitedBuffer{limit: 64 << 10}
	cmd.Stderr = output
	if err = cmd.Start(); err != nil {
		problem(w, 502, "ssh_unavailable", "Could not start SSH. Install OpenSSH and check your private key or SSH agent.")
		return
	}
	t := &Tunnel{ID: id, Host: scan.Host, LocalURL: fmt.Sprintf("http://127.0.0.1:%d", port), APIURL: fmt.Sprintf("http://127.0.0.1:%d", apiPort), Request: scan.Request, KnownHost: key, KnownHostsPath: known, Status: "connecting", Message: "Connecting to the deployment server.", cmd: cmd}
	s.mu.Lock()
	s.tunnels[id] = t
	delete(s.scans, scan.ID)
	response := Tunnel{ID: t.ID, Host: t.Host, LocalURL: t.LocalURL, APIURL: t.APIURL, Status: t.Status, Message: t.Message}
	s.mu.Unlock()
	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		defer s.mu.Unlock()
		if t.Status != "stopped" {
			t.Status = "failed"
			t.Message = "SSH disconnected. Check the key or SSH agent, server status and accepted host fingerprint."
			if err == nil {
				t.Status = "stopped"
				t.Message = "Connection closed."
			}
		}
	}()
	go func() {
		for i := 0; i < 15; i++ {
			time.Sleep(time.Second)
			s.mu.RLock()
			status := t.Status
			s.mu.RUnlock()
			if status != "connecting" {
				return
			}
			if message := tunnelServiceReady(t.APIURL, t.LocalURL); message != "" {
				s.mu.Lock()
				if t.Status == "connecting" {
					t.Status = "connected"
					t.Message = message
				}
				s.mu.Unlock()
				return
			}
		}
		s.mu.Lock()
		if t.Status == "connecting" {
			t.Message = "SSH is starting. The deployment service may still be installing; reconnect when it is ready."
		}
		s.mu.Unlock()
	}()
	writeJSON(w, 202, response)
}

// Both addresses are private forwards created after the SSH host fingerprint is
// accepted. An existing Luci workspace does not need a Coolify management server.
func tunnelServiceReady(apiURL, managementURL string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	_, err := readRemoteControlStatus(ctx, apiURL)
	cancel()
	if err == nil {
		return "Connected to Luci. Sign in to this workspace through the verified SSH connection."
	}
	client := &http.Client{Timeout: 2 * time.Second, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }}
	defer client.CloseIdleConnections()
	res, err := client.Get(managementURL)
	if err == nil {
		defer res.Body.Close()
		if res.StatusCode < 500 {
			return "Connected. Complete first-account setup and create an API token through this protected management connection."
		}
	}
	return ""
}
func (s *Server) listTunnels(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	connections := []Tunnel{}
	for _, t := range s.tunnels {
		connections = append(connections, Tunnel{ID: t.ID, Host: t.Host, LocalURL: t.LocalURL, APIURL: t.APIURL, Status: t.Status, Message: t.Message})
	}
	s.mu.RUnlock()
	writeJSON(w, 200, map[string]any{"connections": connections})
}
func (s *Server) stopTunnel(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	t, ok := s.tunnels[r.PathValue("connectionId")]
	if !ok {
		s.mu.Unlock()
		problem(w, 404, "connection_not_found", "SSH connection not found.")
		return
	}
	t.Status = "stopped"
	t.Message = "Connection closed."
	cmd := t.cmd
	s.mu.Unlock()
	if cmd != nil && cmd.Process != nil {
		_ = cmd.Process.Kill()
	}
	writeJSON(w, 200, map[string]string{"status": "stopped"})
}
func (s *Server) Close() {
	s.closeCoordinator()
	s.StopCloudTaskObserver()
	previewCtx, previewCancel := context.WithTimeout(context.Background(), 5*time.Second)
	s.closeLocalPreviews(previewCtx)
	previewCancel()
	s.closeRuntimeInstaller()
	s.closeProjectChecks()
	s.closeWorkspace()
	s.closeProviderLogins()
	s.mu.Lock()
	commands := []*exec.Cmd{}
	for _, t := range s.tunnels {
		t.Status = "stopped"
		commands = append(commands, t.cmd)
	}
	s.mu.Unlock()
	for _, cmd := range commands {
		if cmd != nil && cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
	}
}

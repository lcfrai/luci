package management

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

type RuntimeStatus struct {
	Status    string `json:"status"`
	StartedAt string `json:"startedAt,omitempty"`
	Message   string `json:"message"`
}
type userRuntime struct {
	mu              sync.Mutex
	user            User
	teamID          string
	policy          Policy
	secret, address string
	status          RuntimeStatus
	ready, done     chan struct{}
	stopOnce        sync.Once
	kill            func()
}

func (rt *userRuntime) snapshot() RuntimeStatus { rt.mu.Lock(); defer rt.mu.Unlock(); return rt.status }
func (rt *userRuntime) stop() {
	rt.stopOnce.Do(func() {
		rt.mu.Lock()
		kill, address, secret, u, p := rt.kill, rt.address, rt.secret, rt.user, rt.policy
		rt.status.Status = "stopping"
		rt.status.Message = "Stopping this user's runtime."
		rt.mu.Unlock()
		if address != "" {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			req, _ := http.NewRequestWithContext(ctx, "POST", address+"/v1/setup/shutdown", strings.NewReader("{}"))
			req.Header.Set("Content-Type", "application/json")
			setWorkerHeaders(req, u, p, secret)
			if rt.teamID != "" {
				req.Header.Set(teamWorkerHeader, rt.teamID)
			}
			client := &http.Client{Timeout: 2 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			res, err := client.Do(req)
			if err == nil {
				res.Body.Close()
			}
			cancel()
			select {
			case <-rt.done:
				return
			case <-time.After(500 * time.Millisecond):
			}
		}
		if kill != nil {
			kill()
		}
	})
}
func setWorkerHeaders(r *http.Request, u User, p Policy, secret string) {
	user, _ := json.Marshal(u)
	policy, _ := json.Marshal(p)
	r.Header.Set("Authorization", "Bearer "+secret)
	r.Header.Set("X-Luci-Actor", base64.RawURLEncoding.EncodeToString(user))
	r.Header.Set("X-Luci-Policy", base64.RawURLEncoding.EncodeToString(policy))
	r.Header.Del(teamWorkerHeader)
	r.Header.Del(teamWorkerAccessHeader)
	if !isPrivatePreviewRequest(r) {
		r.Header.Del("Cookie")
	}
	r.Header.Del("Origin")
	r.Header.Del("Forwarded")
	r.Header.Del("X-Forwarded-Host")
}

func WorkerHandler(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		provided := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if len(token) < 40 || subtle.ConstantTimeCompare([]byte(provided), []byte(token)) != 1 {
			fail(w, 401, "worker_auth_required", "This runtime accepts only its control plane.")
			return
		}
		var u User
		var p Policy
		decodeHeader := func(name string, dst any) bool {
			value := r.Header.Get(name)
			if len(value) > 16384 {
				return false
			}
			data, err := base64.RawURLEncoding.DecodeString(value)
			return err == nil && json.Unmarshal(data, dst) == nil
		}
		if !decodeHeader("X-Luci-Actor", &u) || !decodeHeader("X-Luci-Policy", &p) || !validRole(u.Role) || u.Disabled || validatePolicy(p) != nil || !validWorkerIdentity(r, u) {
			fail(w, 403, "worker_identity_mismatch", "This runtime belongs to a different identity.")
			return
		}
		r.Header.Del("Authorization")
		r.Header.Del("X-Luci-Actor")
		r.Header.Del("X-Luci-Policy")
		teamID := r.Header.Get(teamWorkerHeader)
		ctx := withTeamRuntime(withActor(r.Context(), u, p), teamID)
		if teamID != "" {
			p.AllowHostCommands = false
			ctx = withTeamRuntime(withActor(r.Context(), u, p), teamID)
			var access TeamAccess
			if r.Header.Get(teamWorkerAccessHeader) != "" {
				if !decodeHeader(teamWorkerAccessHeader, &access) || access.Team.ID != teamID || access.Member.UserID != u.ID || access.Member.Revision < 1 || !validTeamRole(access.Member.Role) || (access.ProjectID != "" && !teamProjectIDPattern.MatchString(access.ProjectID)) || IntersectTeamPermissions(PermissionsForTeam(access.Member.Role, access.Member.Grants), access.Permissions) != access.Permissions {
					fail(w, 403, "worker_team_scope_mismatch", "This runtime requires a matching authorized team membership.")
					return
				}
				u.Role = "member"
				if access.Permissions.Admin {
					u.Role = "admin"
				}
				ctx = WithTeamAccess(withActor(ctx, u, p), access)
			} else if !(r.Method == http.MethodGet && (r.URL.Path == "/v1/setup/status" || r.URL.Path == "/v1/healthz")) && !(r.Method == http.MethodPost && r.URL.Path == "/v1/setup/shutdown") {
				fail(w, 403, "worker_team_scope_required", "This runtime requires an authorized team membership scope.")
				return
			}
		}
		r.Header.Del(teamWorkerHeader)
		r.Header.Del(teamWorkerAccessHeader)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
func WriteWorkerAddress(path, address string) error {
	if !strings.Contains(address, "://") {
		address = "http://" + address
	}
	if !validWorkerAddress(address) || !filepath.IsAbs(path) {
		return errors.New("worker address must be literal loopback and address file absolute")
	}
	data, _ := json.Marshal(map[string]string{"address": address})
	return atomicPrivate(path, data)
}
func validWorkerAddress(address string) bool {
	u, err := url.Parse(address)
	if err != nil || u.Scheme != "http" || u.Hostname() != "127.0.0.1" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || u.Path != "" {
		return false
	}
	port, err := strconv.Atoi(u.Port())
	return err == nil && port > 0 && port < 65536
}

func isolatedEnvironment(base []string, root, secret, addressFile string, u User) []string {
	// Explicit allowlist: never inherit model/API keys, cloud credentials,
	// Git configuration injection or another user's provider profile paths.
	// Keep the controller's explicit engine selection for worker Config as well
	// as Docker CLI discovery; both must reach the same private Docker daemon.
	allowed := map[string]bool{"PATH": true, "SYSTEMROOT": true, "WINDIR": true, "PATHEXT": true, "COMSPEC": true, "PROGRAMFILES": true, "PROGRAMFILES(X86)": true, "LANG": true, "LC_ALL": true, "TZ": true, "DOCKER_HOST": true, "LUCI_DOCKER_HOST": true, "LUCI_RUNTIME_DIR": true, "LUCI_SANDBOX_IMAGE": true, "LUCI_CLOUD_BUNDLE": true, "LUCI_BROWSER_PREVIEW_PORT": true}
	env := []string{}
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok && allowed[strings.ToUpper(key)] {
			env = append(env, entry)
		}
	}
	home := filepath.Join(root, "home")
	values := map[string]string{"HOME": home, "USERPROFILE": home, "APPDATA": filepath.Join(home, "AppData", "Roaming"), "LOCALAPPDATA": filepath.Join(home, "AppData", "Local"), "TMP": filepath.Join(root, "tmp"), "TEMP": filepath.Join(root, "tmp"), "TMPDIR": filepath.Join(root, "tmp"), "XDG_CONFIG_HOME": filepath.Join(home, ".config"), "XDG_DATA_HOME": filepath.Join(home, ".local", "share"), "XDG_CACHE_HOME": filepath.Join(home, ".cache"), "GH_CONFIG_DIR": filepath.Join(home, ".config", "gh"), "AWS_SHARED_CREDENTIALS_FILE": filepath.Join(home, ".aws", "credentials"), "AWS_CONFIG_FILE": filepath.Join(home, ".aws", "config"), "AWS_EC2_METADATA_DISABLED": "true", "AZURE_CONFIG_DIR": filepath.Join(home, ".azure"), "CODEX_HOME": filepath.Join(home, ".codex"), "CLAUDE_CONFIG_DIR": filepath.Join(home, ".claude"), "DOCKER_CONFIG": filepath.Join(home, ".docker"), "GIT_CONFIG_GLOBAL": filepath.Join(home, ".gitconfig"), "GIT_CONFIG_NOSYSTEM": "1", "GIT_TERMINAL_PROMPT": "0", "LUCI_WORKER": "1", "LUCI_MANAGED_WORKER": "1", "LUCI_ACTOR_ID": u.ID, "LUCI_ACTOR_ROLE": u.Role, "LUCI_WORKER_TOKEN": secret, "LUCI_WORKER_ADDRESS_FILE": addressFile, "LUCI_ADDR": "127.0.0.1:0", "LUCI_STATE_DIR": filepath.Join(root, "state"), "LUCI_PROJECTS_DIR": filepath.Join(root, "projects"), "LUCI_BACKEND": "docker"}
	for key, value := range values {
		env = append(env, key+"="+value)
	}
	return env
}
func (g *Gateway) ensureRuntime(ctx context.Context, u User, p Policy) (*userRuntime, error) {
	if g.managedTeamID != "" {
		return nil, errors.New("personal execution is disabled on this managed team installation")
	}
	g.mu.Lock()
	if g.closed {
		g.mu.Unlock()
		return nil, errors.New("control plane is closing")
	}
	current, exists := g.state.Users[u.ID]
	if !exists || current.User.Disabled || current.User.Role != u.Role {
		g.mu.Unlock()
		return nil, errors.New("identity changed")
	}
	rt := g.runtimes[u.ID]
	if rt != nil {
		select {
		case <-rt.done:
			delete(g.runtimes, u.ID)
			rt = nil
		default:
		}
	}
	if rt == nil {
		rt = &userRuntime{user: u, policy: p, secret: randomToken(), status: RuntimeStatus{Status: "starting", Message: "Starting the user's private workspace."}, ready: make(chan struct{}), done: make(chan struct{})}
		g.runtimes[u.ID] = rt
		go g.startRuntime(rt)
	}
	g.mu.Unlock()
	select {
	case <-rt.ready:
		rt.mu.Lock()
		ready := rt.status.Status == "running"
		rt.mu.Unlock()
		if !ready {
			return nil, errors.New("private workspace could not start")
		}
		return rt, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
func (g *Gateway) startRuntime(rt *userRuntime) {
	readyClosed := false
	finish := func(message string) {
		rt.mu.Lock()
		rt.status = RuntimeStatus{Status: "stopped", Message: message}
		rt.mu.Unlock()
		if !readyClosed {
			close(rt.ready)
			readyClosed = true
		}
		close(rt.done)
	}
	root := filepath.Join(g.dir, "users", rt.user.ID, "runtime")
	conversationOwner := rt.user.ID
	if rt.teamID != "" {
		root = filepath.Join(g.dir, "teams", rt.teamID, "runtime")
		conversationOwner = "team:" + rt.teamID
	}
	for _, dir := range []string{root, filepath.Join(root, "state"), filepath.Join(root, "projects"), filepath.Join(root, "home"), filepath.Join(root, "home", "AppData", "Roaming"), filepath.Join(root, "home", "AppData", "Local"), filepath.Join(root, "tmp")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			finish("Private workspace directories could not be prepared.")
			return
		}
		if err := privatePath(dir); err != nil {
			finish("Private workspace permissions could not be secured.")
			return
		}
	}
	addressFile := filepath.Join(root, "worker-address.json")
	conversations, err := prepareWorkerConversations(g.dir, root, conversationOwner)
	legacyStorage := false
	if err != nil {
		if errors.Is(err, errConversationsActive) {
			// A persisted running status may survive a host crash. Keep the
			// original homes available to the ordinary worker, whose backend
			// recovery proves which sandboxes still exist and lets the user stop
			// live sessions. Never copy a possibly active CLI home to upgrade it.
			conversations = filepath.Join(root, "state", "conversations")
			legacyStorage = true
		} else {
			finish("Private conversation storage could not be prepared. Existing data has been preserved.")
			return
		}
	}
	if err := os.Remove(addressFile); err != nil && !os.IsNotExist(err) {
		finish("Previous workspace address could not be removed.")
		return
	}
	cmd := exec.Command(g.opts.Executable, g.opts.WorkerArgs...)
	cmd.Dir = root
	cmd.Env = isolatedEnvironment(g.opts.BaseEnvironment, root, rt.secret, addressFile, rt.user)
	if rt.teamID != "" {
		cmd.Env = isolatedTeamEnvironment(g.opts.BaseEnvironment, root, rt.secret, addressFile, rt.teamID, rt.user)
	}
	azureConfig, azureErr := prepareAzureWorkerConfig(g.opts.BaseEnvironment, root)
	if azureErr != nil {
		finish("Protected Azure worker configuration could not be prepared. Existing state has been preserved.")
		return
	}
	if azureConfig != "" {
		cmd.Env = append(cmd.Env, "LUCI_E2B_CONFIG="+azureConfig)
	}
	cmd.Env = append(cmd.Env, "LUCI_CONVERSATIONS_DIR="+conversations)
	// The worker rereads the controller's atomically persisted authority before
	// every deferred allocation. Only this trusted launcher supplies its scope.
	billingConfigID := ""
	if g.billing != nil {
		billingConfigID = g.billing.configID
	}
	cmd.Env = append(cmd.Env, "LUCI_EXECUTION_AUTHORITY_FILE="+filepath.Join(g.dir, "identity.json"), "LUCI_EXECUTION_MANAGED_TEAM="+g.managedTeamID, "LUCI_EXECUTION_BILLING_CONFIG="+billingConfigID, "LUCI_EXECUTION_STATE_DIR="+g.opts.ExecutionStateDir)

	cmd.Stdout, cmd.Stderr = io.Discard, io.Discard
	kill, err := startOwnedProcess(cmd)
	if err != nil {
		finish("The private workspace process could not start.")
		return
	}
	rt.mu.Lock()
	rt.kill = kill
	stopping := rt.status.Status == "stopping"
	rt.mu.Unlock()
	if stopping {
		kill()
	}
	exited := make(chan error, 1)
	go func() { exited <- cmd.Wait() }()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(30 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case <-exited:
			kill()
			finish("The private workspace process has stopped.")
			return
		case <-deadline.C:
			kill()
			<-exited
			finish("The private workspace did not become ready within thirty seconds.")
			return
		case <-ticker.C:
			data, err := os.ReadFile(addressFile)
			if err != nil || len(data) > 4096 {
				continue
			}
			var endpoint struct {
				Address string `json:"address"`
			}
			if json.Unmarshal(data, &endpoint) != nil || !validWorkerAddress(endpoint.Address) {
				continue
			}
			req, _ := http.NewRequest("GET", endpoint.Address+"/v1/setup/status", nil)
			setWorkerHeaders(req, rt.user, rt.policy, rt.secret)
			if rt.teamID != "" {
				req.Header.Set(teamWorkerHeader, rt.teamID)
			}
			client := &http.Client{Timeout: 500 * time.Millisecond, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			res, err := client.Do(req)
			if err != nil {
				continue
			}
			res.Body.Close()
			if res.StatusCode != 200 {
				continue
			}
			rt.mu.Lock()
			if rt.status.Status == "stopping" {
				rt.mu.Unlock()
				kill()
				continue
			}
			rt.address = endpoint.Address
			message := "Private user workspace is running."
			if rt.teamID != "" {
				message = "Shared team workspace is running on this host with Docker execution."
			}
			if legacyStorage {
				message = "Private workspace is running with its existing agent homes. Stop active sessions, then restart this workspace to complete the private storage upgrade."
			}
			rt.status = RuntimeStatus{Status: "running", StartedAt: time.Now().UTC().Format(time.RFC3339Nano), Message: message}
			rt.mu.Unlock()
			close(rt.ready)
			readyClosed = true
			<-exited
			kill()
			finish("The private workspace process has stopped.")
			return
		}
	}
}
func (g *Gateway) executionHandler(ctx context.Context, u User, p Policy) (http.Handler, error) {
	if g.managedTeamID != "" {
		return nil, errors.New("personal execution is disabled on this managed team installation")
	}
	g.mu.Lock()
	owner := g.state.OwnerID
	g.mu.Unlock()
	if u.ID == owner {
		return g.opts.InitialHandler, nil
	}
	rt, err := g.ensureRuntime(ctx, u, p)
	if err != nil {
		return nil, err
	}
	rt.mu.Lock()
	address, secret := rt.address, rt.secret
	rt.mu.Unlock()
	target, _ := url.Parse(address)
	proxy := httputil.NewSingleHostReverseProxy(target)
	original := proxy.Director
	proxy.Director = func(req *http.Request) { original(req); req.Host = target.Host; setWorkerHeaders(req, u, p, secret) }
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		fail(w, 503, "runtime_unavailable", "Your private workspace is not answering. Retry or ask an administrator to stop it.")
	}
	return proxy, nil
}
func (g *Gateway) runtimeStatus(w http.ResponseWriter, r *http.Request) {
	if _, ok := requireAdmin(w, r, false); !ok {
		return
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	id := r.PathValue("id")
	if _, ok := g.state.Users[id]; !ok {
		fail(w, 404, "unknown_user", "This user does not exist.")
		return
	}
	if g.managedTeamID != "" {
		jsonReply(w, 200, RuntimeStatus{Status: "disabled", Message: "Personal workspaces are disabled. Use this installation's assigned team workspace."})
		return
	}
	if id == g.state.OwnerID {
		jsonReply(w, 200, RuntimeStatus{Status: "running", Message: "Initial owner's existing workspace runs with the main service."})
		return
	}
	if rt := g.runtimes[id]; rt != nil {
		jsonReply(w, 200, rt.snapshot())
		return
	}
	jsonReply(w, 200, RuntimeStatus{Status: "stopped", Message: "This user's workspace starts on their first project request."})
}
func (g *Gateway) stopRuntime(w http.ResponseWriter, r *http.Request) {
	actor, ok := requireAdmin(w, r, false)
	if !ok {
		return
	}
	g.mu.Lock()
	id := r.PathValue("id")
	row, exists := g.state.Users[id]
	if !exists {
		g.mu.Unlock()
		fail(w, 404, "unknown_user", "This user does not exist.")
		return
	}
	if actor.Role != "super-admin" && row.User.Role != "member" {
		g.mu.Unlock()
		fail(w, 403, "permission_denied", "Only a Super Admin can stop another administrator's runtime.")
		return
	}
	if id == g.state.OwnerID {
		g.mu.Unlock()
		fail(w, 409, "main_runtime", "The initial owner's workspace runs with the main service. Stop Luci to stop that runtime.")
		return
	}
	rt := g.runtimes[id]
	g.auditLocked(actor.ID, "runtime.stop", id, "requested")
	err := g.persistLocked()
	g.mu.Unlock()
	if err != nil {
		fail(w, 500, "save_failed", "The stop request could not be saved.")
		return
	}
	if rt != nil {
		rt.stop()
		jsonReply(w, 200, rt.snapshot())
		return
	}
	jsonReply(w, 200, RuntimeStatus{Status: "stopped", Message: "This user's runtime is already stopped."})
}

type capturedResponse struct {
	header http.Header
	status int
	body   bytes.Buffer
}

func (c *capturedResponse) Header() http.Header    { return c.header }
func (c *capturedResponse) WriteHeader(status int) { c.status = status }
func (c *capturedResponse) Write(data []byte) (int, error) {
	if c.status == 0 {
		c.status = 200
	}
	if c.body.Len()+len(data) > 8<<20 {
		return 0, errors.New("response limit")
	}
	return c.body.Write(data)
}

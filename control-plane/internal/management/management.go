// Package management authenticates the control plane and routes each identity
// to its own execution workspace. Worker credentials never reach the browser.
package management

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

type User struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	Disabled    bool   `json:"disabled"`
}
type Policy struct {
	AllowHostCommands          bool     `json:"allowHostCommands"`
	AllowCloudProvisioning     bool     `json:"allowCloudProvisioning"`
	AllowPublicPublishing      bool     `json:"allowPublicPublishing"`
	MaxSessionsPerUser         int      `json:"maxSessionsPerUser"`
	MaxConcurrentChecksPerUser int      `json:"maxConcurrentChecksPerUser"`
	SessionMemoryMiB           int      `json:"sessionMemoryMiB"`
	SessionCPUMillis           int      `json:"sessionCPUMillis"`
	CheckMemoryMiB             int      `json:"checkMemoryMiB"`
	CheckCPUMillis             int      `json:"checkCPUMillis"`
	MonthlyBudgetUSD           float64  `json:"monthlyBudgetUsd"`
	AllowedHarnesses           []string `json:"allowedHarnesses"`
	BackingMode                string   `json:"backingMode"`
	InstanceName               string   `json:"instanceName"`
}
type Permissions struct {
	ManageUsers     bool `json:"manageUsers"`
	ManageBacking   bool `json:"manageBacking"`
	ManagePolicy    bool `json:"managePolicy"`
	InspectAudit    bool `json:"inspectAudit"`
	RunHostCommands bool `json:"runHostCommands"`
}

func PermissionsFor(u User, p Policy) Permissions {
	admin := u.Role == "admin" || u.Role == "super-admin"
	return Permissions{ManageUsers: admin, ManageBacking: admin, ManagePolicy: u.Role == "super-admin", InspectAudit: admin, RunHostCommands: u.Role == "super-admin" && p.AllowHostCommands}
}

type actorContext struct {
	User   User
	Policy Policy
}
type actorKey struct{}

func ActorFromContext(ctx context.Context) (User, Policy, bool) {
	v, ok := ctx.Value(actorKey{}).(actorContext)
	return v.User, v.Policy, ok
}
func withActor(ctx context.Context, u User, p Policy) context.Context {
	return context.WithValue(ctx, actorKey{}, actorContext{u, p})
}

type Options struct {
	StateDir, InstanceName, Backing, Executable, Version string
	LocalAccessToken                                     string
	PortableOwnerAccess                                  bool
	WorkerArgs, BaseEnvironment                          []string
	InitialHandler                                       http.Handler
}
type userRecord struct {
	User         User   `json:"user"`
	PasswordHash string `json:"passwordHash"`
}
type sessionRecord struct {
	UserID    string    `json:"userId"`
	ExpiresAt time.Time `json:"expiresAt"`
}
type AuditEvent struct {
	ID      string `json:"id"`
	At      string `json:"at"`
	ActorID string `json:"actorId"`
	Action  string `json:"action"`
	Target  string `json:"target"`
	Outcome string `json:"outcome"`
}
type storedState struct {
	Version     int                         `json:"version"`
	OwnerID     string                      `json:"ownerId"`
	Users       map[string]userRecord       `json:"users"`
	Sessions    map[string]sessionRecord    `json:"sessions"`
	Policy      Policy                      `json:"policy"`
	Events      []AuditEvent                `json:"events"`
	Invitations map[string]invitationRecord `json:"invitations,omitempty"`
}
type socketTicket struct {
	UserID, Path, SessionHash string
	ExpiresAt                 time.Time
}
type loginAttempt struct {
	Count int
	Until time.Time
}
type Gateway struct {
	mu                  sync.Mutex
	opts                Options
	dir, bootstrapToken string
	state               storedState
	tickets             map[string]socketTicket
	attempts            map[string]loginAttempt
	passwordSlots       chan struct{}
	runtimes            map[string]*userRuntime
	admissions          map[string]*sync.Mutex
	checks              map[string]int
	sockets             map[string]map[net.Conn]string
	closed              bool
	dependencies        *dependencyManager
}

func New(opts Options) (*Gateway, error) {
	if opts.PortableOwnerAccess && opts.LocalAccessToken != "" {
		return nil, errors.New("portable and native local access capabilities are mutually exclusive")
	}
	if opts.LocalAccessToken != "" && !validLocalAccessToken(opts.LocalAccessToken) {
		return nil, errors.New("invalid local access capability")
	}
	if !filepath.IsAbs(opts.StateDir) {
		return nil, errors.New("management state directory must be absolute")
	}
	if opts.InitialHandler == nil {
		return nil, errors.New("initial owner handler is required")
	}
	if opts.InstanceName == "" {
		opts.InstanceName = "Luci"
	}
	if opts.Backing == "" {
		opts.Backing = "docker"
	}
	if opts.Backing != "docker" && opts.Backing != "aws" && opts.Backing != "azure" {
		return nil, errors.New("choose docker, aws or azure backing")
	}
	if opts.Executable == "" {
		opts.Executable, _ = os.Executable()
	}
	if opts.BaseEnvironment == nil {
		opts.BaseEnvironment = os.Environ()
	}
	g := &Gateway{opts: opts, dir: filepath.Join(opts.StateDir, "control-plane"), tickets: map[string]socketTicket{}, attempts: map[string]loginAttempt{}, passwordSlots: make(chan struct{}, 2), runtimes: map[string]*userRuntime{}, admissions: map[string]*sync.Mutex{}, checks: map[string]int{}, sockets: map[string]map[net.Conn]string{}}
	if err := os.MkdirAll(g.dir, 0700); err != nil {
		return nil, err
	}
	if err := privatePath(g.dir); err != nil {
		return nil, err
	}
	g.state = storedState{Version: 1, Users: map[string]userRecord{}, Sessions: map[string]sessionRecord{}, Events: []AuditEvent{}, Policy: Policy{AllowCloudProvisioning: true, AllowPublicPublishing: true, MaxSessionsPerUser: 1, MaxConcurrentChecksPerUser: 1, SessionMemoryMiB: 1024, SessionCPUMillis: 1000, CheckMemoryMiB: 1024, CheckCPUMillis: 1000, MonthlyBudgetUSD: 30, AllowedHarnesses: []string{"claude-code", "codex", "opencode", "shell"}, BackingMode: opts.Backing, InstanceName: opts.InstanceName}}
	data, err := os.ReadFile(filepath.Join(g.dir, "identity.json"))
	if err == nil {
		if len(data) > 8<<20 || json.Unmarshal(data, &g.state) != nil || g.state.Version != 1 || g.state.Users == nil || g.state.Sessions == nil || validatePolicy(g.state.Policy) != nil {
			return nil, errors.New("invalid management identity state")
		}
		if len(g.state.Users) > 0 {
			owner, ok := g.state.Users[g.state.OwnerID]
			if !ok || owner.User.ID != g.state.OwnerID {
				return nil, errors.New("initial owner identity is missing")
			}
		}
	} else if !os.IsNotExist(err) {
		return nil, err
	}
	if len(g.state.Users) == 0 {
		path := filepath.Join(g.dir, "bootstrap-token")
		data, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			g.bootstrapToken = randomToken()
			if err = atomicPrivate(path, []byte(g.bootstrapToken+"\n")); err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		} else {
			g.bootstrapToken = strings.TrimSpace(string(data))
			if len(g.bootstrapToken) < 40 {
				return nil, errors.New("invalid bootstrap token")
			}
			if err = privatePath(path); err != nil {
				return nil, err
			}
		}
	}
	if g.state.Invitations == nil {
		g.state.Invitations = map[string]invitationRecord{}
	}
	// The caller has bound its loopback listener before constructing the gateway.
	// A portable launcher can prove ownership by reading this private, per-run
	// capability. Native launch capabilities are never written to disk.
	if opts.PortableOwnerAccess {
		g.opts.LocalAccessToken = randomToken()
		if err := atomicPrivate(filepath.Join(g.dir, "owner-access-token"), []byte(g.opts.LocalAccessToken+"\n")); err != nil {
			return nil, err
		}
	}
	g.dependencies, err = newDependencyManager(g, dependencySystemRunner{environment: opts.BaseEnvironment, dir: g.dir})
	if err != nil {
		g.removePortableOwnerAccess()
		return nil, err
	}
	return g, nil
}
func randomToken() string {
	var data [32]byte
	if _, err := rand.Read(data[:]); err != nil {
		panic("operating system randomness unavailable")
	}
	return base64.RawURLEncoding.EncodeToString(data[:])
}
func opaqueID(prefix string) string { return prefix + randomToken()[:22] }
func hashToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}
func atomicPrivate(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), ".identity-")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = privatePath(f.Name()); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
func (g *Gateway) persistLocked() error {
	data, err := json.Marshal(g.state)
	if err != nil {
		return err
	}
	return atomicPrivate(filepath.Join(g.dir, "identity.json"), data)
}
func (g *Gateway) auditLocked(actor, action, target, outcome string) {
	g.state.Events = append(g.state.Events, AuditEvent{ID: opaqueID("event-"), At: time.Now().UTC().Format(time.RFC3339Nano), ActorID: actor, Action: action, Target: target, Outcome: outcome})
	if len(g.state.Events) > 1000 {
		g.state.Events = append([]AuditEvent{}, g.state.Events[len(g.state.Events)-1000:]...)
	}
}
func jsonReply(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
func fail(w http.ResponseWriter, status int, code, message string) {
	jsonReply(w, status, map[string]string{"error": code, "message": message})
}
func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	d := json.NewDecoder(r.Body)
	d.DisallowUnknownFields()
	if err := d.Decode(v); err != nil {
		fail(w, 400, "invalid_request", "Check the submitted fields.")
		return false
	}
	if !errors.Is(d.Decode(new(any)), io.EOF) {
		fail(w, 400, "invalid_request", "Send one JSON object.")
		return false
	}
	return true
}
func validRole(role string) bool { return role == "member" || role == "admin" || role == "super-admin" }

var usernamePattern = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]{2,63}$`)

func validIdentity(username, display, password string) bool {
	return usernamePattern.MatchString(username) && len(strings.TrimSpace(display)) > 0 && len(display) <= 120 && len(password) >= 12 && len(password) <= 1024
}
func (g *Gateway) userListLocked() []User {
	rows := []User{}
	for _, row := range g.state.Users {
		rows = append(rows, row.User)
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i].Username < rows[j].Username })
	return rows
}
func (g *Gateway) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /v1/control-plane/status", g.status)
	mux.HandleFunc("POST /v1/auth/bootstrap", g.bootstrap)
	mux.HandleFunc("POST /v1/auth/login", g.login)
	mux.HandleFunc("GET /v1/auth/local-owner", g.localOwner)
	mux.HandleFunc("POST /v1/auth/local-account", g.localAccount)
	mux.HandleFunc("POST /v1/auth/local-owner/recover", g.recoverLocalOwner)
	mux.HandleFunc("POST /v1/auth/invitations/accept", g.acceptInvitation)
	mux.HandleFunc("GET /v1/admin/invitations", g.listInvitations)
	mux.HandleFunc("POST /v1/admin/invitations", g.createInvitation)
	mux.HandleFunc("DELETE /v1/admin/invitations/{id}", g.revokeInvitation)
	mux.HandleFunc("GET /v1/auth/me", g.me)
	mux.HandleFunc("POST /v1/auth/logout", g.logout)
	mux.HandleFunc("POST /v1/auth/password", g.changePassword)
	mux.HandleFunc("POST /v1/auth/socket-ticket", g.createSocketTicket)
	mux.HandleFunc("GET /v1/admin/users", g.listUsers)
	mux.HandleFunc("POST /v1/admin/users", g.createUser)
	mux.HandleFunc("PATCH /v1/admin/users/{id}", g.updateUser)
	mux.HandleFunc("GET /v1/admin/policy", g.getPolicy)
	mux.HandleFunc("PATCH /v1/admin/policy", g.patchPolicy)
	mux.HandleFunc("GET /v1/admin/audit", g.audit)
	mux.HandleFunc("GET /v1/admin/users/{id}/runtime", g.runtimeStatus)
	mux.HandleFunc("POST /v1/admin/users/{id}/runtime/stop", g.stopRuntime)
	mux.HandleFunc("GET /v1/setup/dependencies", g.dependencyStatus)
	mux.HandleFunc("POST /v1/setup/dependencies/check", g.checkDependencies)
	mux.HandleFunc("POST /v1/setup/dependencies/{id}/install", g.installDependency)
	mux.HandleFunc("/", g.route)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/v1/") {
			clone := r.Clone(r.Context())
			u := *r.URL
			u.Path = strings.TrimPrefix(u.Path, "/api")
			if u.RawPath != "" {
				u.RawPath = strings.TrimPrefix(u.RawPath, "/api")
			}
			clone.URL = &u
			r = clone
		}
		if (r.URL.RawPath != "" && !privatePreviewPath(r)) || strings.Contains(r.URL.Path, "//") || strings.Contains(r.URL.Path, "/../") {
			fail(w, 400, "invalid_path", "Use a canonical API path.")
			return
		}
		for _, header := range []string{"X-Luci-Actor", "X-Luci-Policy"} {
			r.Header.Del(header)
		}
		path := r.URL.Path
		localAccess := (r.Method == "GET" && path == "/v1/auth/local-owner") || (r.Method == "POST" && (path == "/v1/auth/local-account" || path == "/v1/auth/local-owner/recover"))
		if !localAccess {
			r.Header.Del(localAccessHeader)
		}
		public := localAccess || (r.Method == "GET" && path == "/v1/control-plane/status") || (r.Method == "POST" && (path == "/v1/auth/login" || path == "/v1/auth/bootstrap" || path == "/v1/auth/invitations/accept"))
		if !public {
			u, p, sessionHash, ok := g.authenticate(r)
			if !ok {
				fail(w, 401, "sign_in_required", "Sign in to Luci to continue.")
				return
			}
			r = r.WithContext(withActor(r.Context(), u, p))
			if strings.EqualFold(r.Header.Get("Upgrade"), "websocket") {
				w = &socketResponseWriter{ResponseWriter: w, gateway: g, userID: u.ID, sessionHash: sessionHash}
			}
		}
		mux.ServeHTTP(w, r)
	})
}

var previewPathPattern = regexp.MustCompile(`^/v1/product/projects/[A-Za-z0-9_-]+/releases/[A-Za-z0-9_-]+/preview/`)

func privatePreviewPath(r *http.Request) bool {
	if !isPrivatePreviewRequest(r) || !previewPathPattern.MatchString(r.URL.RawPath) {
		return false
	}
	return true
}
func isPrivatePreviewRequest(r *http.Request) bool {
	if r.Header.Get("X-Luci-Preview-Bridge") != "1" || !previewPathPattern.MatchString(r.URL.Path) {
		return false
	}
	for _, name := range []string{"Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest", "Sec-Fetch-User"} {
		if r.Header.Get(name) != "" {
			return false
		}
	}
	return true
}
func (g *Gateway) status(w http.ResponseWriter, r *http.Request) {
	g.mu.Lock()
	defer g.mu.Unlock()
	result := map[string]any{"product": "Luci", "version": g.opts.Version, "configured": len(g.state.Users) > 0, "instanceName": g.state.Policy.InstanceName, "backing": g.state.Policy.BackingMode, "bootstrapRequired": len(g.state.Users) == 0}
	if len(g.state.Users) == 0 {
		result["bootstrapTokenPath"] = filepath.Join(g.dir, "bootstrap-token")
	}
	jsonReply(w, 200, result)
}
func (g *Gateway) Close(ctx context.Context) error {
	defer g.removePortableOwnerAccess()
	g.mu.Lock()
	g.closed = true
	for id := range g.sockets {
		g.closeSocketsLocked(id, "")
	}
	list := []*userRuntime{}
	for _, rt := range g.runtimes {
		list = append(list, rt)
	}
	g.mu.Unlock()
	for _, rt := range list {
		rt.stop()
	}
	if err := g.dependencies.close(ctx); err != nil {
		return err
	}
	for _, rt := range list {
		select {
		case <-rt.done:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return nil
}

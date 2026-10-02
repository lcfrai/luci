package product

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/management"
	"github.com/lcfrai/luci/control-plane/internal/project"
)

func TestManagedWorkspaceStartsReadyAndRetainsAssignedFolder(t *testing.T) {
	root := t.TempDir()
	opts := Options{StateDir: filepath.Join(root, "state"), ProjectsDir: filepath.Join(root, "assigned"), ManagedWorkspace: true}
	s, err := New(opts)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mux := http.NewServeMux()
	s.Register(mux)
	for _, role := range []string{"member", "admin", "super-admin"} {
		w := journeyRequest(t, workspaceAdminActor(mux, role), "GET", "/v1/setup/status", nil, 200)
		var status Status
		if json.Unmarshal(w.Body.Bytes(), &status) != nil || !status.Configured || status.Config.ProjectsDir != opts.ProjectsDir || status.Access == nil || !status.Access.ProjectsDirManaged || status.Access.User.Role != role || status.Access.ManageBacking != (role != "member") {
			t.Fatalf("assigned %s workspace status: %s", role, w.Body.String())
		}
	}
	if _, err := os.Stat(filepath.Join(opts.StateDir, "luci.json")); !os.IsNotExist(err) {
		t.Fatal("readiness should not need or write first-use configuration")
	}
	escape := filepath.Join(root, "outside")
	config := s.Config()
	config.ProjectsDir = escape
	journeyRequest(t, workspaceAdminActor(mux, "super-admin"), "POST", "/v1/setup/config", config, 403)
	if _, err := os.Stat(escape); !os.IsNotExist(err) {
		t.Fatal("rejected folder was created")
	}
	if err := atomicJSON(filepath.Join(opts.StateDir, "luci.json"), config); err != nil {
		t.Fatal(err)
	}
	if _, err := New(opts); err == nil {
		t.Fatal("restart accepted configuration outside the assigned folder")
	}
	if _, err := New(Options{StateDir: t.TempDir(), ManagedWorkspace: true}); err == nil {
		t.Fatal("managed readiness accepted no assigned folder")
	}
}

func TestSetupAccessIgnoresUntrustedHeadersAndCloudUsesRemoteActor(t *testing.T) {
	s, mux := journeyFixture(t)
	mux.HandleFunc("GET /v1/setup/status", s.handleStatus)
	request := httptest.NewRequest("GET", "/v1/setup/status", nil)
	request.Header.Set("X-Luci-Actor", `{"username":"forged","role":"super-admin"}`)
	w := httptest.NewRecorder()
	mux.ServeHTTP(w, request)
	var local Status
	if json.Unmarshal(w.Body.Bytes(), &local) != nil || local.Access != nil {
		t.Fatal("public headers became authoritative setup access")
	}
	remoteRoot := t.TempDir()
	remote, err := New(Options{StateDir: filepath.Join(remoteRoot, "state"), ProjectsDir: filepath.Join(remoteRoot, "assigned"), ManagedWorkspace: true})
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	remoteMux := http.NewServeMux()
	remote.Register(remoteMux)
	endpoint := httptest.NewServer(workspaceAdminActor(remoteMux, "member"))
	defer endpoint.Close()
	tunnel := workspaceTunnel(s, endpoint.URL)
	s.workspace = Workspace{Mode: "cloud", ConnectionID: tunnel.ID, Host: "remote", Ready: true}
	w = journeyRequest(t, workspaceAdminActor(s.ProxyWorkspace(mux), "super-admin"), "GET", "/v1/setup/status", nil, 200)
	var status Status
	if json.Unmarshal(w.Body.Bytes(), &status) != nil || !status.Configured || status.Workspace.Mode != "cloud" || status.Access == nil || status.Access.User.Role != "member" || status.Access.ManageBacking || !status.Access.ProjectsDirManaged || status.Config.ProjectsDir != remote.Config().ProjectsDir {
		t.Fatal("local administrator replaced active remote member authority", w.Body.String())
	}
}

// A real child product service behind the production worker capability boundary.
// No Docker, provider account or model call is needed to use assigned projects.
func TestManagedProductWorkerProcess(t *testing.T) {
	if os.Getenv("LUCI_WORKER") != "1" || os.Getenv("LUCI_WORKER_TOKEN") == "" {
		return
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	s, err := New(Options{StateDir: filepath.Join(os.Getenv("LUCI_STATE_DIR"), "sessions"), ProjectsDir: os.Getenv("LUCI_PROJECTS_DIR"), ManagedWorkspace: true, Shutdown: func() { _ = listener.Close() }})
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	mux := http.NewServeMux()
	s.Register(mux)
	if err := management.WriteWorkerAddress(os.Getenv("LUCI_WORKER_ADDRESS_FILE"), listener.Addr().String()); err != nil {
		t.Fatal(err)
	}
	if err := http.Serve(listener, management.WorkerHandler(os.Getenv("LUCI_WORKER_TOKEN"), mux)); !errors.Is(err, net.ErrClosed) {
		t.Fatal(err)
	}
}

func TestInvitedMemberImmediatelyCreatesPrivateProjectWithoutBackingSetup(t *testing.T) {
	root := t.TempDir()
	owner, err := New(Options{StateDir: filepath.Join(root, "owner-state"), ProjectsDir: filepath.Join(root, "owner-projects")})
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	mux := http.NewServeMux()
	owner.Register(mux)
	gatewayRoot := filepath.Join(root, "gateway")
	g, err := management.New(management.Options{StateDir: gatewayRoot, Version: Version, InitialHandler: mux, WorkerArgs: []string{"-test.run=^TestManagedProductWorkerProcess$"}})
	if err != nil {
		t.Fatal(err)
	}
	defer g.Close(context.Background())
	h := g.Handler()
	call := func(token, method, path string, body any, want int) *httptest.ResponseRecorder {
		return journeyRequest(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Set("Authorization", "Bearer "+token)
			r.Header.Set("X-Luci-Actor", "forged-admin")
			h.ServeHTTP(w, r)
		}), method, path, body, want)
	}
	var ownerAuth, memberAuth struct {
		Token string          `json:"token"`
		User  management.User `json:"user"`
	}
	code, err := os.ReadFile(filepath.Join(gatewayRoot, "control-plane", "bootstrap-token"))
	if err != nil {
		t.Fatal(err)
	}
	json.Unmarshal(call("", "POST", "/v1/auth/bootstrap", map[string]string{"bootstrapToken": strings.TrimSpace(string(code)), "username": "owner", "displayName": "Owner", "password": "owner-test-password"}, 201).Body.Bytes(), &ownerAuth)
	var ownerProject, memberProject project.Project
	json.Unmarshal(call(ownerAuth.Token, "POST", "/v1/product/projects", map[string]string{"name": "owner-work"}, 201).Body.Bytes(), &ownerProject)
	call(ownerAuth.Token, "POST", "/v1/product/projects/"+ownerProject.ID+"/brief", map[string]any{"revision": 0, "purpose": "Owner private work"}, 200)
	var invitation struct {
		InviteCode string `json:"inviteCode"`
	}
	json.Unmarshal(call(ownerAuth.Token, "POST", "/v1/admin/invitations", map[string]string{"role": "member"}, 201).Body.Bytes(), &invitation)
	json.Unmarshal(call("", "POST", "/v1/auth/invitations/accept", map[string]string{"inviteCode": invitation.InviteCode, "username": "invited", "displayName": "Invited Member", "password": "member-test-password"}, 201).Body.Bytes(), &memberAuth)
	w := call(memberAuth.Token, "GET", "/v1/setup/status", nil, 200)
	var status Status
	json.Unmarshal(w.Body.Bytes(), &status)
	assigned := filepath.Join(gatewayRoot, "control-plane", "users", memberAuth.User.ID, "runtime", "projects")
	if !status.Configured || status.Config.ProjectsDir != assigned || status.Access == nil || status.Access.User.Username != "invited" || status.Access.User.Role != "member" || status.Access.ManageBacking || !status.Access.ProjectsDirManaged {
		t.Fatal("invited account was not ready with its own authority", w.Body.String())
	}
	json.Unmarshal(call(memberAuth.Token, "POST", "/v1/product/projects", map[string]string{"name": "member-work"}, 201).Body.Bytes(), &memberProject)
	if memberProject.Path != filepath.Join(assigned, "member-work") {
		t.Fatal("member project escaped its assigned folder", memberProject.Path)
	}
	call(memberAuth.Token, "POST", "/v1/product/projects/"+memberProject.ID+"/brief", map[string]any{"revision": 0, "purpose": "Invited member private work"}, 200)
	data, err := os.ReadFile(filepath.Join(memberProject.Path, "LUCI.md"))
	if err != nil || !strings.Contains(string(data), "Invited member private work") {
		t.Fatal("member brief was not written in its private project", err)
	}
	call(ownerAuth.Token, "GET", "/v1/product/projects/"+memberProject.ID+"/brief", nil, 404)
	call(memberAuth.Token, "GET", "/v1/product/projects/"+ownerProject.ID+"/brief", nil, 404)
	config := status.Config
	config.ProjectsDir = ownerProject.Path
	call(memberAuth.Token, "POST", "/v1/setup/config", config, 403)
	call(memberAuth.Token, "POST", "/v1/setup/runtime/install", map[string]any{}, 403)
	call(memberAuth.Token, "POST", "/v1/setup/cloud/plan", map[string]any{}, 403)
}

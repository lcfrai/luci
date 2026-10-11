package management

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/hostpath"
)

func (g *Gateway) route(w http.ResponseWriter, r *http.Request) {
	u, p, ok := ActorFromContext(r.Context())
	if !ok {
		fail(w, 401, "sign_in_required", "Sign in to continue.")
		return
	}
	teamAccess, teamScoped := TeamAccessFromContext(r.Context())
	if g.managedTeamID != "" && (!teamScoped || teamAccess.Team.ID != g.managedTeamID) {
		fail(w, 403, "managed_team_required", "This installation serves its assigned team. Select that team workspace; personal workspaces are disabled.")
		return
	}
	if teamScoped {
		// Team authority controls this worker; an instance administrator does
		// not acquire team capabilities merely through their global role.
		u.Role = "member"
		if teamAccess.Permissions.Admin {
			u.Role = "admin"
		}
		p.AllowHostCommands = false
		r = r.WithContext(withActor(r.Context(), u, p))
	}
	path := r.URL.Path
	if !strings.HasPrefix(path, "/v1/") {
		fail(w, 404, "not_found", "This control-plane API route does not exist.")
		return
	}
	mutating := r.Method != "GET" && r.Method != "HEAD" && r.Method != "OPTIONS"
	admin := u.Role == "admin" || u.Role == "super-admin"
	if mutating && (strings.HasPrefix(path, "/v1/setup/cloud/") || strings.HasPrefix(path, "/v1/setup/ssh/") || path == "/v1/setup/targets" || path == "/v1/setup/workspace/activate" || path == "/v1/setup/workspace/login" || path == "/v1/setup/workspace/bootstrap" || path == "/v1/setup/runtime/install") {
		if !admin {
			fail(w, 403, "permission_denied", "Backing infrastructure is managed by an administrator.")
			return
		}
		if strings.HasPrefix(path, "/v1/setup/cloud/") && !p.AllowCloudProvisioning {
			fail(w, 403, "cloud_disabled", "Cloud provisioning is disabled by instance policy.")
			return
		}
	}
	if path == "/v1/setup/shutdown" && u.Role != "super-admin" {
		fail(w, 403, "permission_denied", "Only a Super Admin can stop the main service.")
		return
	}
	var fields map[string]json.RawMessage
	inspectBody := mutating && (path == "/v1/setup/config" || path == "/v1/setup/cloud/plan" || path == "/v1/sessions" || strings.HasSuffix(path, "/git/github") || strings.HasPrefix(path, "/v1/setup/ssh/") || strings.HasSuffix(path, "/checks") || (strings.HasPrefix(path, "/v1/product/projects/") && (strings.HasSuffix(path, "/assist/repository") || strings.HasSuffix(path, "/assist"))))
	if inspectBody {
		data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 2<<20))
		if err != nil || json.Unmarshal(data, &fields) != nil || fields == nil {
			fail(w, 400, "invalid_request", "Send a bounded JSON request.")
			return
		}
		r.Body = io.NopCloser(bytes.NewReader(data))
	}
	if path == "/v1/setup/config" && mutating {
		var projectsDir string
		_ = json.Unmarshal(fields["projectsDir"], &projectsDir)
		g.mu.Lock()
		owner := g.state.OwnerID
		g.mu.Unlock()
		if teamScoped {
			expected := filepath.Join(g.dir, "teams", teamAccess.Team.ID, "runtime", "projects")
			if projectsDir != "" && filepath.Clean(projectsDir) != expected {
				fail(w, 403, "workspace_path_fixed", "Team projects stay inside the team's dedicated workspace.")
				return
			}
		} else if u.ID != owner {
			expected := filepath.Join(g.dir, "users", u.ID, "runtime", "projects")
			if projectsDir != "" && filepath.Clean(projectsDir) != expected {
				fail(w, 403, "workspace_path_fixed", "Managed users keep their projects inside their assigned workspace.")
				return
			}
		}
		if !teamScoped && u.ID == owner && u.Role != "super-admin" {
			fail(w, 403, "permission_denied", "Only a Super Admin can reconfigure the initial host workspace.")
			return
		}
	}
	if path == "/v1/setup/cloud/plan" && mutating {
		var budget float64
		if json.Unmarshal(fields["monthlyBudgetUsd"], &budget) != nil || budget > p.MonthlyBudgetUSD {
			fail(w, 403, "budget_policy", "The reviewed monthly budget exceeds instance policy. Ask a Super Admin to update the policy if required.")
			return
		}
	}
	if strings.HasPrefix(path, "/v1/setup/ssh/") && mutating {
		var privateKeyPath string
		_ = json.Unmarshal(fields["privateKeyPath"], &privateKeyPath)
		if privateKeyPath != "" && !PermissionsFor(u, p).RunHostCommands {
			g.mu.Lock()
			owner := g.state.OwnerID
			g.mu.Unlock()
			root := filepath.Join(g.dir, "users", u.ID, "runtime")
			if teamScoped {
				root = filepath.Join(g.dir, "teams", teamAccess.Team.ID, "runtime")
			} else if u.ID == owner {
				root = filepath.Join(g.opts.StateDir, "sessions", "ssh")
			}
			_, err := hostpath.ConfineToMounts([]string{root}, privateKeyPath)
			if err != nil {
				fail(w, 403, "private_key_path", "Use the dedicated key in your own workspace. Other host keys require an enabled Super Admin host-command policy.")
				return
			}
		}
	}
	if strings.HasSuffix(path, "/git/github") && mutating {
		var visibility string
		_ = json.Unmarshal(fields["visibility"], &visibility)
		if visibility == "public" && !p.AllowPublicPublishing {
			fail(w, 403, "public_publishing_disabled", "Public repository creation is disabled by instance policy.")
			return
		}
	}
	if strings.HasSuffix(path, "/checks") && mutating {
		var execution string
		_ = json.Unmarshal(fields["execution"], &execution)
		if execution == "host" && !PermissionsFor(u, p).RunHostCommands {
			fail(w, 403, "host_commands_disabled", "Host commands require a Super Admin and an enabled policy.")
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 35*time.Second)
	var handler http.Handler
	var err error
	if teamScoped {
		handler, err = g.teamExecutionHandler(ctx, teamAccess.Team.ID, u, p)
	} else {
		handler, err = g.executionHandler(ctx, u, p)
	}
	cancel()
	if err != nil {
		fail(w, 503, "runtime_unavailable", "Your private workspace could not start. Ask an administrator to inspect its runtime status.")
		return
	}
	creatingCodexUtility := r.Method == "POST" && path == "/v1/setup/agents/codex/sign-in"
	creatingInterview := r.Method == "POST" && strings.HasPrefix(path, "/v1/product/projects/") && strings.HasSuffix(path, "/assist")
	creatingRepositoryAnalysis := r.Method == "POST" && strings.HasPrefix(path, "/v1/product/projects/") && strings.HasSuffix(path, "/assist/repository")
	creatingSession := creatingCodexUtility || creatingInterview || creatingRepositoryAnalysis || (r.Method == "POST" && (path == "/v1/sessions" || strings.HasSuffix(path, "/restart") || strings.HasSuffix(path, "/fork")) && strings.HasPrefix(path, "/v1/sessions"))
	if creatingSession {
		if creatingRepositoryAnalysis || creatingInterview {
			var chosen string
			if creatingInterview && len(fields["harness"]) == 0 {
				chosen = "codex"
				fields["harness"] = json.RawMessage(`"codex"`)
			} else if json.Unmarshal(fields["harness"], &chosen) != nil {
				chosen = ""
			}
			if chosen != "codex" && chosen != "claude-code" && chosen != "opencode" {
				fail(w, 400, "invalid_analysis_provider", "Choose Codex, Claude Code or OpenCode for the assistant.")
				return
			}
		}
		if creatingCodexUtility {
			// Official Codex account sign-in cannot select another provider.
			fields = map[string]json.RawMessage{"harness": json.RawMessage(`"codex"`)}
		}
		admissionKey := u.ID
		if teamScoped {
			admissionKey = "team:" + teamAccess.Team.ID
		}
		g.mu.Lock()
		lock := g.admissions[admissionKey]
		if lock == nil {
			lock = &sync.Mutex{}
			g.admissions[admissionKey] = lock
		}
		g.mu.Unlock()
		lock.Lock()
		defer lock.Unlock()
		if !g.sessionAdmission(w, r, handler, u, p, fields) {
			return
		}
	}
	if teamScoped && r.Context().Err() != nil {
		fail(w, 403, "team_access_changed", "Team access changed before the operation could start.")
		return
	}
	// The initial inline engine is trusted through context, never headers.
	r.Header.Del("Authorization")
	if !isPrivatePreviewRequest(r) {
		r.Header.Del("Cookie")
	}
	handler.ServeHTTP(w, r)
}

func (g *Gateway) sessionAdmission(w http.ResponseWriter, r *http.Request, handler http.Handler, u User, p Policy, fields map[string]json.RawMessage) bool {
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()
	check, _ := http.NewRequestWithContext(withActor(ctx, u, p), "GET", "http://127.0.0.1/v1/sessions", nil)
	capture := &capturedResponse{header: make(http.Header)}
	handler.ServeHTTP(capture, check)
	var data struct {
		Sessions []struct {
			ID                    string `json:"id"`
			Status                string `json:"status"`
			Harness               string `json:"harness"`
			ProjectRunID          string `json:"projectRunId"`
			ProjectRunOperationID string `json:"projectRunOperationId"`
		} `json:"sessions"`
	}
	if capture.status != 200 || json.Unmarshal(capture.body.Bytes(), &data) != nil {
		fail(w, 503, "session_state_unavailable", "Current session usage could not be checked.")
		return false
	}
	var harness string
	_ = json.Unmarshal(fields["harness"], &harness)
	var runID, operationID string
	_ = json.Unmarshal(fields["projectRunId"], &runID)
	_ = json.Unmarshal(fields["projectRunOperationId"], &operationID)
	runID, operationID = strings.TrimSpace(runID), strings.TrimSpace(operationID)
	replays := 0
	active := 0
	for _, session := range data.Sessions {
		// This worker-owned row establishes that the request can only reuse
		// an existing reserved child. The session API still verifies the
		// private reservation and immutable inputs before returning it.
		if r.Method == http.MethodPost && r.URL.Path == "/v1/sessions" && runID != "" && operationID != "" && session.ProjectRunID == runID && session.ProjectRunOperationID == operationID {
			replays++
		}
		if session.Status == "starting" || session.Status == "running" || session.Status == "idle" {
			active++
		}
		if strings.Contains(r.URL.Path, "/sessions/"+session.ID+"/") {
			harness = session.Harness
			if strings.HasSuffix(r.URL.Path, "/restart") && session.Status != "exited" {
				active--
			}
		}
	}
	if harness == "" {
		harness = "claude-code"
	}
	allowed := false
	for _, id := range p.AllowedHarnesses {
		if id == harness {
			allowed = true
			break
		}
	}
	if !allowed {
		fail(w, 403, "harness_disabled", "This agent is disabled by instance policy.")
		return false
	}
	if active >= p.MaxSessionsPerUser && replays != 1 {
		fail(w, 409, "session_limit", "Stop a running session before starting another; your instance session limit has been reached.")
		return false
	}
	return true
}

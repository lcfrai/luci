package management

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

func teamWorkspacePrefix(teamID string) string { return "/v1/teams/" + teamID + "/workspace" }

const teamResponseLimit = 8 << 20

// Bound before buffering, including malformed or unexpectedly large private
// worker responses. An error after an unbounded ReadAll is too late to cap memory.
type teamCapturedResponse struct {
	header   http.Header
	status   int
	body     bytes.Buffer
	overflow bool
}

func (c *teamCapturedResponse) Header() http.Header { return c.header }
func (c *teamCapturedResponse) WriteHeader(status int) {
	if c.status == 0 {
		c.status = status
	}
}
func (c *teamCapturedResponse) Write(p []byte) (int, error) {
	if c.status == 0 {
		c.status = http.StatusOK
	}
	remaining := teamResponseLimit - c.body.Len()
	if len(p) > remaining {
		c.overflow = true
		_, _ = c.body.Write(p[:remaining])
	} else {
		_, _ = c.body.Write(p)
	}
	return len(p), nil
}

func newTeamCapture() *teamCapturedResponse { return &teamCapturedResponse{header: make(http.Header)} }

// Worker observations are bounded private reads. A browser-supplied project or
// session identity is never used as proof that it belongs to the selected team.
func (g *Gateway) teamRead(ctx context.Context, access TeamAccess, actor User, policy Policy, path string, out any) error {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	ctx = WithTeamAccess(withActor(ctx, scopedTeamActor(actor, access), teamPolicy(policy)), access)
	handler, err := g.teamExecutionHandler(ctx, access.Team.ID, scopedTeamActor(actor, access), teamPolicy(policy))
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "GET", "http://127.0.0.1"+path, nil)
	if err != nil {
		return err
	}
	capture := newTeamCapture()
	handler.ServeHTTP(capture, req)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if capture.status != 200 || capture.overflow || json.Unmarshal(capture.body.Bytes(), out) != nil {
		return errors.New("the team's owned workspace state could not be read")
	}
	return nil
}

type teamProjectSelector struct {
	ID string `json:"id"`
}

func (g *Gateway) teamProjects(ctx context.Context, access TeamAccess, actor User, policy Policy) ([]teamProjectSelector, error) {
	var list struct {
		Projects []teamProjectSelector `json:"projects"`
	}
	if err := g.teamRead(ctx, access, actor, policy, "/v1/projects", &list); err != nil {
		return nil, err
	}
	if len(list.Projects) > 256 {
		return nil, errors.New("the team project catalogue exceeds its supported limit")
	}
	seen := map[string]bool{}
	for _, p := range list.Projects {
		if !teamProjectIDPattern.MatchString(p.ID) || seen[p.ID] {
			return nil, errors.New("ambiguous team project identity")
		}
		seen[p.ID] = true
	}
	return list.Projects, nil
}
func (g *Gateway) teamProjectExists(ctx context.Context, access TeamAccess, projectID string) error {
	actor, policy, ok := ActorFromContext(ctx)
	if !ok {
		return errTeamAccess
	}
	list, err := g.teamProjects(ctx, access, actor, policy)
	if err != nil {
		return err
	}
	for _, p := range list {
		if p.ID == projectID {
			return nil
		}
	}
	return errTeamAccess
}

type teamSessionSelector struct {
	ID         string `json:"id"`
	ProjectID  string `json:"projectId"`
	WorktreeID string `json:"worktreeId"`
	Harness    string `json:"harness"`
	AuthOnly   bool   `json:"authOnly"`
}

func (g *Gateway) resolveTeamSession(ctx context.Context, access TeamAccess, actor User, policy Policy, sessionID string) (teamSessionSelector, error) {
	var session teamSessionSelector
	if !teamProjectIDPattern.MatchString(sessionID) {
		return session, errTeamAccess
	}
	if err := g.teamRead(ctx, access, actor, policy, "/v1/sessions/"+url.PathEscape(sessionID), &session); err != nil {
		return session, err
	}
	if session.ID != sessionID || (!teamProjectIDPattern.MatchString(session.ProjectID) && !teamSignInSession(session)) {
		return session, errTeamAccess
	}
	return session, nil
}

// Codex device sign-in is the one supported session that deliberately mounts no
// project. Its account-wide credential utility remains visible only to the team
// administrators who can explicitly manage and execute provider sign-in.
func teamSignInSession(session teamSessionSelector) bool {
	return session.ProjectID == "" && session.AuthOnly && session.Harness == "codex" && session.WorktreeID == "setup-codex"
}

func (g *Gateway) teamSessionAccess(teamID, userID string, session teamSessionSelector) (TeamAccess, error) {
	if session.ProjectID == "" && !teamSignInSession(session) {
		return TeamAccess{}, errTeamAccess
	}
	access, err := g.resolveTeamAccess(teamID, userID, session.ProjectID)
	if err != nil {
		return TeamAccess{}, err
	}
	if session.AuthOnly && (!access.Permissions.Edit || !access.Permissions.Admin || !access.Permissions.Execute || !access.Permissions.Secrets) {
		return TeamAccess{}, errTeamAccess
	}
	return access, nil
}
func (g *Gateway) resolveTeamWorktree(ctx context.Context, access TeamAccess, actor User, policy Policy, worktreeID string) (string, error) {
	if !teamProjectIDPattern.MatchString(worktreeID) {
		return "", errTeamAccess
	}
	projects, err := g.teamProjects(ctx, access, actor, policy)
	if err != nil {
		return "", err
	}
	found := ""
	for _, p := range projects {
		// A member cannot enumerate worktrees of a project outside their ACL.
		if _, err := g.resolveTeamAccess(access.Team.ID, actor.ID, p.ID); err != nil {
			continue
		}
		var data struct {
			Worktrees []struct {
				ID        string `json:"id"`
				ProjectID string `json:"projectId"`
			} `json:"worktrees"`
		}
		if err := g.teamRead(ctx, access, actor, policy, "/v1/projects/"+url.PathEscape(p.ID)+"/worktrees", &data); err != nil {
			return "", err
		}
		if len(data.Worktrees) > 1024 {
			return "", errors.New("the team worktree catalogue exceeds its supported limit")
		}
		for _, wt := range data.Worktrees {
			if wt.ID == worktreeID {
				if wt.ProjectID != p.ID || found != "" {
					return "", errTeamAccess
				}
				found = p.ID
			}
		}
	}
	if found == "" {
		return "", errTeamAccess
	}
	return found, nil
}
func (g *Gateway) resolveTeamSocketAccess(ctx context.Context, user User, policy Policy, teamID, sessionID string) (TeamAccess, error) {
	g.mu.Lock()
	serviceActive := g.teamServiceAllowsExecutionLocked(teamID)
	g.mu.Unlock()
	if !serviceActive {
		return TeamAccess{}, errTeamAccess
	}
	access, err := g.resolveTeamAccess(teamID, user.ID, "")
	if err != nil {
		return TeamAccess{}, err
	}
	if !access.Permissions.Edit || !access.Permissions.Execute {
		return TeamAccess{}, errTeamAccess
	}
	session, err := g.resolveTeamSession(ctx, access, user, policy, sessionID)
	if err != nil {
		return TeamAccess{}, err
	}
	access, err = g.teamSessionAccess(teamID, user.ID, session)
	if err != nil || !access.Permissions.Edit || !access.Permissions.Execute {
		return TeamAccess{}, errTeamAccess
	}
	// Provider terminals can expose the injected team credential. Secret use is
	// an explicit capability, independent from running a credential-free shell.
	if session.Harness != "shell" && !access.Permissions.Secrets {
		return TeamAccess{}, errTeamAccess
	}
	return access, nil
}

func (g *Gateway) routeTeamWorkspace(w http.ResponseWriter, r *http.Request) {
	actor, policy, ok := ActorFromContext(r.Context())
	if !ok {
		fail(w, 401, "sign_in_required", "Sign in to continue.")
		return
	}
	teamID := r.PathValue("teamId")
	prefix := teamWorkspacePrefix(teamID)
	if !teamIDPattern.MatchString(teamID) || !strings.HasPrefix(r.URL.Path, prefix+"/v1/") || (r.URL.RawPath != "" && !canonicalStorageObjectPath(r)) {
		fail(w, 400, "invalid_team_path", "Use a canonical team workspace API path.")
		return
	}
	inner := r.Clone(r.Context())
	u := *r.URL
	u.Path = strings.TrimPrefix(u.Path, prefix)
	if u.RawPath != "" {
		u.RawPath = strings.TrimPrefix(u.RawPath, prefix)
	}
	inner.URL = &u
	scope, known := ClassifyTeamRoute(inner)
	if !known {
		fail(w, 403, "team_route_unavailable", "This operation is not available in a team workspace.")
		return
	}
	access, err := g.resolveTeamAccess(teamID, actor.ID, scope.ProjectID)
	if err != nil {
		fail(w, 403, "team_access_denied", errTeamAccess.Error())
		return
	}
	if !authorizeTeamRoute(w, inner, access.Permissions) {
		return
	}
	if !g.authorizeTeamService(w, inner, teamID) {
		return
	}
	if scope.CreatesProject && !access.Permissions.Admin {
		fail(w, 403, "team_admin_required", "A team administrator creates or imports projects, then grants project access.")
		return
	}
	if scope.WorktreeFromBody {
		body, readErr := io.ReadAll(http.MaxBytesReader(w, inner.Body, 2<<20))
		if readErr != nil {
			fail(w, 400, "invalid_request", "Send a bounded session request.")
			return
		}
		var fields struct {
			WorktreeID string `json:"worktreeId"`
			Resume     string `json:"resume"`
			Harness    string `json:"harness"`
			AuthOnly   bool   `json:"authOnly"`
		}
		if json.Unmarshal(body, &fields) != nil {
			fail(w, 400, "invalid_request", "Send a valid session request.")
			return
		}
		inner.Body = io.NopCloser(bytes.NewReader(body))
		projectID, err := g.resolveTeamWorktree(inner.Context(), access, actor, policy, fields.WorktreeID)
		if err != nil {
			fail(w, 403, "team_project_unavailable", errTeamAccess.Error())
			return
		}
		access, err = g.resolveTeamAccess(teamID, actor.ID, projectID)
		if err != nil || !authorizeTeamRoute(w, inner, access.Permissions) {
			if err != nil {
				fail(w, 403, "team_project_unavailable", errTeamAccess.Error())
			}
			return
		}
		if (fields.Harness != "shell" || fields.AuthOnly) && !access.Permissions.Secrets {
			fail(w, 403, "team_secret_use_required", "This coding agent uses the team's provider connection. Ask for explicit secret-use permission.")
			return
		}
		if fields.AuthOnly && !access.Permissions.Admin {
			fail(w, 403, "team_admin_required", "A team administrator manages provider sign-in.")
			return
		}
		if fields.Resume != "" {
			source, err := g.resolveTeamSession(inner.Context(), access, actor, policy, fields.Resume)
			if err != nil {
				fail(w, 403, "team_project_unavailable", errTeamAccess.Error())
				return
			}
			from, err := g.teamSessionAccess(teamID, actor.ID, source)
			if err != nil || !from.Permissions.Edit || !from.Permissions.Execute || (source.Harness != "shell" && !from.Permissions.Secrets) {
				fail(w, 403, "team_project_unavailable", errTeamAccess.Error())
				return
			}
			// Resuming can read another project's private agent history. Keep
			// that source grant live through admission, as well as the target
			// grant registered below; revoking either cancels the request.
			var finishSource func()
			inner, finishSource, err = g.RegisterTeamRequest(w, inner, teamID, from.ProjectID, from.Member.Revision, from.ProjectRevision)
			if err != nil {
				fail(w, 403, "team_access_changed", errTeamAccess.Error())
				return
			}
			defer finishSource()
		}
	}
	if scope.SessionID != "" {
		session, lookupErr := g.resolveTeamSession(inner.Context(), access, actor, policy, scope.SessionID)
		if lookupErr != nil {
			fail(w, 403, "team_project_unavailable", errTeamAccess.Error())
			return
		}
		access, err = g.teamSessionAccess(teamID, actor.ID, session)
		if err != nil {
			fail(w, 403, "team_project_unavailable", errTeamAccess.Error())
			return
		}
		if !authorizeTeamRoute(w, inner, access.Permissions) {
			return
		}
		if (strings.HasSuffix(inner.URL.Path, "/attach") || strings.HasSuffix(inner.URL.Path, "/restart")) && session.Harness != "shell" && !access.Permissions.Secrets {
			fail(w, 403, "team_secret_use_required", "This agent requires explicit permission to use team credentials.")
			return
		}
	}
	if scope.WorktreeID != "" {
		owner, lookupErr := g.resolveTeamWorktree(inner.Context(), access, actor, policy, scope.WorktreeID)
		if lookupErr != nil || owner != scope.ProjectID {
			fail(w, 403, "team_project_unavailable", errTeamAccess.Error())
			return
		}
	}
	if scope.CreatesProject {
		// Count and dispatch are one admission operation. Otherwise concurrent
		// creates/imports can both consume the final slot and strand a team
		// beyond the catalogue's supported limit. Never hold g.mu across I/O.
		g.mu.Lock()
		key := "team-projects:" + teamID
		admission := g.admissions[key]
		if admission == nil {
			admission = &sync.Mutex{}
			g.admissions[key] = admission
		}
		g.mu.Unlock()
		admission.Lock()
		defer admission.Unlock()
		projects, lookupErr := g.teamProjects(inner.Context(), access, actor, policy)
		if lookupErr != nil {
			fail(w, 503, "team_catalogue_unavailable", "The current team projects could not be checked.")
			return
		}
		if len(projects) >= 256 {
			fail(w, 409, "team_project_limit", "This team workspace has reached its 256-project limit.")
			return
		}
	}
	inner = inner.WithContext(WithTeamAccess(withActor(inner.Context(), scopedTeamActor(actor, access), teamPolicy(policy)), access))
	inner, finish, err := g.RegisterTeamRequest(w, inner, teamID, access.ProjectID, access.Member.Revision, access.ProjectRevision)
	if err != nil {
		fail(w, 403, "team_access_changed", errTeamAccess.Error())
		return
	}
	defer finish()
	if scope.Collection == "projects" || scope.Collection == "sessions" {
		capture := newTeamCapture()
		g.route(capture, inner)
		if inner.Context().Err() != nil {
			fail(w, 403, "team_access_changed", errTeamAccess.Error())
			return
		}
		if capture.status != 200 || capture.overflow {
			fail(w, 503, "team_catalogue_unavailable", "The team catalogue could not be read.")
			return
		}
		var envelope map[string]json.RawMessage
		var rows []json.RawMessage
		if json.Unmarshal(capture.body.Bytes(), &envelope) != nil || json.Unmarshal(envelope[scope.Collection], &rows) != nil || len(rows) > 10000 {
			fail(w, 503, "team_catalogue_unavailable", "The team catalogue could not be verified.")
			return
		}
		visible := make([]json.RawMessage, 0, len(rows))
		seen := map[string]bool{}
		for _, raw := range rows {
			var selector teamSessionSelector
			if json.Unmarshal(raw, &selector) != nil || !teamProjectIDPattern.MatchString(selector.ID) || seen[selector.ID] {
				fail(w, 503, "team_catalogue_unavailable", "The team catalogue could not be verified.")
				return
			}
			seen[selector.ID] = true
			projectID := selector.ProjectID
			if scope.Collection == "projects" {
				projectID = selector.ID
			}
			var grant TeamAccess
			if scope.Collection == "sessions" {
				grant, err = g.teamSessionAccess(teamID, actor.ID, selector)
			} else {
				grant, err = g.resolveTeamAccess(teamID, actor.ID, projectID)
			}
			if err != nil || !grant.Permissions.Read {
				continue
			}
			if scope.Collection == "sessions" {
				raw, err = teamSessionProjection(raw, prefix)
				if err != nil {
					fail(w, 503, "team_catalogue_unavailable", "The team session identity could not be verified.")
					return
				}
			}
			visible = append(visible, raw)
		}
		journey, _ := json.Marshal(visible)
		// Do not echo unrelated envelope fields that may summarize hidden rows.
		jsonReply(w, 200, map[string]json.RawMessage{scope.Collection: journey})
		return
	}
	// Every session response carries its scoped attach path, preventing a
	// browser from asking the personal workspace to attach a team session.
	if scope.SessionID != "" && !strings.HasSuffix(inner.URL.Path, "/attach") || scope.WorktreeFromBody || inner.URL.Path == "/v1/setup/agents/codex/sign-in" {
		capture := newTeamCapture()
		g.route(capture, inner)
		writeTeamSessionResponse(w, inner.Context(), capture, prefix)
		return
	}
	g.route(w, inner)
}

// The worker response remains private until projection finishes and the pinned
// request is checked again. Large captured documents can take meaningful time
// to decode; revocation during that work must not release the buffered bytes.
func writeTeamSessionResponse(w http.ResponseWriter, ctx context.Context, capture *teamCapturedResponse, prefix string) {
	if ctx.Err() != nil {
		fail(w, 403, "team_access_changed", errTeamAccess.Error())
		return
	}
	if capture.overflow {
		fail(w, 503, "team_session_unavailable", "The team session response exceeded its supported limit.")
		return
	}
	if capture.status >= 200 && capture.status < 300 && capture.body.Len() > 0 {
		projected, err := teamSessionProjection(capture.body.Bytes(), prefix)
		if err != nil {
			fail(w, 503, "team_session_unavailable", "The team session result could not be verified. Refresh before retrying.")
			return
		}
		if ctx.Err() != nil {
			fail(w, 403, "team_access_changed", errTeamAccess.Error())
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(capture.status)
		_, _ = w.Write(projected)
		return
	}
	copyTeamResponse(w, capture)
}

func teamSessionProjection(raw []byte, prefix string) (json.RawMessage, error) {
	var fields map[string]json.RawMessage
	if json.Unmarshal(raw, &fields) != nil {
		return nil, errors.New("invalid session response")
	}
	var id string
	_ = json.Unmarshal(fields["id"], &id)
	// DELETE can return an operation acknowledgement rather than a session.
	if id == "" {
		return raw, nil
	}
	if !teamProjectIDPattern.MatchString(id) {
		return nil, errors.New("invalid session identity")
	}
	path, _ := json.Marshal(prefix + "/v1/sessions/" + id + "/attach")
	fields["attachPath"] = path
	return json.Marshal(fields)
}
func copyTeamResponse(w http.ResponseWriter, c *teamCapturedResponse) {
	for name, values := range c.header {
		for _, value := range values {
			w.Header().Add(name, value)
		}
	}
	status := c.status
	if status == 0 {
		status = 200
	}
	w.WriteHeader(status)
	_, _ = w.Write(c.body.Bytes())
}

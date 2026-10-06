package management

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"strings"
	"sync"
)

const teamWorkerHeader = "X-Luci-Team"
const teamWorkerAccessHeader = "X-Luci-Team-Access"

type teamRuntimeKey struct{}

type teamSocketScope struct {
	TeamID, UserID, ProjectID       string
	MemberRevision, ProjectRevision int
}

type teamRequest struct {
	teamSocketScope
	SessionHash string
}

// RegisterTeamRequest pins the membership and project grant used for admission.
// Revocation cancels this request and closes its upgraded socket after the
// membership change is durably saved. Running shared children remain explicit
// stop operations; removing a collaborator never deletes team work.
func (g *Gateway) RegisterTeamRequest(w http.ResponseWriter, r *http.Request, teamID, projectID string, memberRevision, projectRevision int) (*http.Request, func(), error) {
	actor, _, ok := ActorFromContext(r.Context())
	if !ok {
		return nil, nil, errors.New("sign in before accessing a team workspace")
	}
	scope := teamSocketScope{TeamID: teamID, UserID: actor.ID, ProjectID: projectID, MemberRevision: memberRevision, ProjectRevision: projectRevision}
	sessionHash := ""
	if bearer := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "); bearer != "" && bearer != r.Header.Get("Authorization") {
		sessionHash = hashToken(bearer)
	}
	for writer := w; writer != nil; {
		if socket, ok := writer.(*socketResponseWriter); ok {
			socket.teamScope = &scope
			sessionHash = socket.sessionHash
			break
		}
		unwrapper, ok := writer.(interface{ Unwrap() http.ResponseWriter })
		if !ok {
			break
		}
		writer = unwrapper.Unwrap()
	}
	ctx, cancel := context.WithCancel(r.Context())
	request := &teamRequest{teamSocketScope: scope, SessionHash: sessionHash}
	g.mu.Lock()
	if !g.teamAccessCurrentLocked(teamID, actor.ID, projectID, memberRevision, projectRevision) || !g.teamServiceRequestAllowedLocked(teamID, r) {
		g.mu.Unlock()
		cancel()
		return nil, nil, errors.New("team workspace access changed before admission")
	}
	if g.teamRequests == nil {
		g.teamRequests = map[*teamRequest]context.CancelFunc{}
	}
	g.teamRequests[request] = cancel
	g.mu.Unlock()
	var once sync.Once
	finish := func() {
		once.Do(func() {
			cancel()
			g.mu.Lock()
			delete(g.teamRequests, request)
			g.mu.Unlock()
		})
	}
	return r.WithContext(ctx), finish, nil
}

// Call under g.mu only after persisting the new membership/grants. Empty scope
// components conservatively match every live request in that dimension.
func (g *Gateway) revokeTeamAccessLocked(teamID, userID, projectID string) {
	matches := func(scope teamSocketScope) bool {
		return (teamID == "" || scope.TeamID == teamID) && (userID == "" || scope.UserID == userID) && (projectID == "" || scope.ProjectID == projectID)
	}
	for request, cancel := range g.teamRequests {
		if matches(request.teamSocketScope) {
			cancel()
			delete(g.teamRequests, request)
		}
	}
	for conn, scope := range g.teamSockets {
		if matches(scope) {
			_ = conn.Close()
			delete(g.teamSockets, conn)
			delete(g.sockets[scope.UserID], conn)
		}
	}
	for key, ticket := range g.tickets {
		if ticket.TeamID != "" && matches(teamSocketScope{TeamID: ticket.TeamID, UserID: ticket.UserID, ProjectID: ticket.ProjectID}) {
			delete(g.tickets, key)
		}
	}
}

func (g *Gateway) forgetSocketLocked(userID string, conn net.Conn) {
	delete(g.sockets[userID], conn)
	delete(g.teamSockets, conn)
}

func withTeamRuntime(ctx context.Context, teamID string) context.Context {
	if teamID == "" {
		return ctx
	}
	return context.WithValue(ctx, teamRuntimeKey{}, teamID)
}

// TeamRuntimeIDFromContext is populated only after the worker token and its
// fixed team identity have been verified, or by the trusted gateway fixture.
func TeamRuntimeIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(teamRuntimeKey{}).(string)
	return id
}

func validWorkerIdentity(r *http.Request, u User) bool {
	teamID := os.Getenv("LUCI_TEAM_ID")
	if teamID != "" {
		return teamIDPattern.MatchString(teamID) && r.Header.Get(teamWorkerHeader) == teamID && u.ID != ""
	}
	return r.Header.Get(teamWorkerHeader) == "" && r.Header.Get(teamWorkerAccessHeader) == "" &&
		(os.Getenv("LUCI_ACTOR_ID") == "" || u.ID == os.Getenv("LUCI_ACTOR_ID")) &&
		(os.Getenv("LUCI_ACTOR_ROLE") == "" || u.Role == os.Getenv("LUCI_ACTOR_ROLE"))
}

// Team state and credentials belong to the dedicated worker. Every request
// retains its human actor; no member inherits another person's HOME or tokens.
func isolatedTeamEnvironment(base []string, root, secret, addressFile, teamID string, u User) []string {
	personal := isolatedEnvironment(base, root, secret, addressFile, u)
	env := make([]string, 0, len(personal)+1)
	for _, entry := range personal {
		key, _, _ := strings.Cut(entry, "=")
		if key != "LUCI_ACTOR_ID" && key != "LUCI_ACTOR_ROLE" && key != "LUCI_TEAM_ID" {
			env = append(env, entry)
		}
	}
	// Only a team worker receives the controller's fixed public preview domain;
	// personal workers never become publicly addressable through this setting.
	for _, entry := range base {
		key, _, ok := strings.Cut(entry, "=")
		if ok && key == "LUCI_PREVIEW_BASE_DOMAIN" {
			env = append(env, entry)
		}
	}
	return append(env, "LUCI_TEAM_ID="+teamID)
}

func (g *Gateway) ensureTeamRuntime(ctx context.Context, teamID string, actor User, policy Policy) (*userRuntime, error) {
	if !teamIDPattern.MatchString(teamID) || g.managedTeamID != "" && teamID != g.managedTeamID {
		return nil, errors.New("invalid team workspace identity")
	}
	g.mu.Lock()
	team, exists := g.state.Teams[teamID]
	_, member := team.Members[actor.ID]
	current, active := g.state.Users[actor.ID]
	if g.closed || !exists || !member || !active || current.User.Disabled {
		g.mu.Unlock()
		return nil, errors.New("team workspace access changed")
	}
	key := "team:" + teamID
	rt := g.runtimes[key]
	if rt != nil {
		select {
		case <-rt.done:
			delete(g.runtimes, key)
			rt = nil
		default:
		}
	}
	if rt == nil {
		policy.AllowHostCommands = false
		rt = &userRuntime{user: actor, teamID: teamID, policy: policy, secret: randomToken(), status: RuntimeStatus{Status: "starting", Message: "Starting this team's dedicated workspace on the current host."}, ready: make(chan struct{}), done: make(chan struct{})}
		g.runtimes[key] = rt
		go g.startRuntime(rt)
	}
	g.mu.Unlock()
	select {
	case <-rt.ready:
		if rt.snapshot().Status != "running" {
			return nil, errors.New("the team workspace could not start")
		}
		return rt, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (g *Gateway) teamExecutionHandler(ctx context.Context, teamID string, actor User, policy Policy) (http.Handler, error) {
	access, scoped := TeamAccessFromContext(ctx)
	if !scoped || access.Team.ID != teamID || access.Member.UserID != actor.ID {
		return nil, errors.New("team worker requires an authorized membership scope")
	}
	actor.Role = "member"
	if access.Permissions.Admin {
		actor.Role = "admin"
	}
	policy.AllowHostCommands = false
	if g.opts.TeamHandler != nil {
		handler, err := g.opts.TeamHandler(ctx, teamID)
		if err != nil {
			return nil, err
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			r.Header.Del(teamWorkerHeader)
			r.Header.Del(teamWorkerAccessHeader)
			handler.ServeHTTP(w, r.WithContext(WithTeamAccess(withTeamRuntime(withActor(r.Context(), actor, policy), teamID), access)))
		}), nil
	}
	rt, err := g.ensureTeamRuntime(ctx, teamID, actor, policy)
	if err != nil {
		return nil, err
	}
	rt.mu.Lock()
	address, secret := rt.address, rt.secret
	rt.mu.Unlock()
	if !validWorkerAddress(address) {
		return nil, errors.New("the team workspace has no valid private address")
	}
	target, _ := url.Parse(address)
	proxy := httputil.NewSingleHostReverseProxy(target)
	original := proxy.Director
	proxy.Director = func(req *http.Request) {
		original(req)
		req.Host = target.Host
		setWorkerHeaders(req, actor, policy, secret)
		req.Header.Set(teamWorkerHeader, teamID)
		encoded, _ := json.Marshal(access)
		req.Header.Set(teamWorkerAccessHeader, base64.RawURLEncoding.EncodeToString(encoded))
	}
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		fail(w, http.StatusServiceUnavailable, "team_runtime_unavailable", "The team's workspace is not answering. Retry or ask its owner to inspect the runtime.")
	}
	return proxy, nil
}

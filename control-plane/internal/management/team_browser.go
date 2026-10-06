package management

import (
	"context"
	"encoding/json"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/previewgateway"
)

const teamBrowserHandoffPath = "/.well-known/luci-team-access"
const teamBrowserCookie = previewgateway.TeamAccessCookieName

type teamBrowserGrant struct {
	teamSocketScope
	SessionHash, Origin, Destination string
	ExpiresAt                        time.Time
	DataHandoffs                     []string
	ParentOrigin                     string
}

type teamBrowserAccess struct {
	gateway  *previewgateway.Gateway
	tickets  map[string]teamBrowserGrant
	sessions map[string]teamBrowserGrant
}

// SetBrowserGateway connects isolated browser origins to the same durable
// membership authority as the control API. It runs before either listener.
func (g *Gateway) SetBrowserGateway(browser *previewgateway.Gateway) {
	g.mu.Lock()
	g.teamBrowser = &teamBrowserAccess{gateway: browser, tickets: map[string]teamBrowserGrant{}, sessions: map[string]teamBrowserGrant{}}
	g.mu.Unlock()
	browser.SetTeamAccessHandler(g.TeamBrowserAccess)
}

func (g *Gateway) registerTeamBrowser(mux *http.ServeMux) {
	mux.HandleFunc("POST /v1/teams/{teamId}/projects/{projectId}/browser-access", g.createTeamBrowserAccess)
}

func teamBrowserAllowed(access TeamAccess, target previewgateway.TeamBrowserTarget) bool {
	p := access.Permissions
	return p.Read && p.Edit && p.Execute && ((target.Kind != "data" && target.ClientKind != "data") || (p.Secrets && p.Deploy))
}

func (g *Gateway) createTeamBrowserAccess(w http.ResponseWriter, r *http.Request) {
	var req struct {
		URL string `json:"url"`
	}
	if !decode(w, r, &req) {
		return
	}
	u, err := url.Parse(req.URL)
	if err != nil || len(req.URL) > 8192 || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Opaque != "" || strings.HasPrefix(u.Path, "//") || strings.Contains(u.Path, "\\") || strings.HasPrefix(u.Path, teamBrowserHandoffPath) {
		fail(w, 400, "invalid_browser_url", "Choose this project's isolated browser address.")
		return
	}
	actor, _, _ := ActorFromContext(r.Context())
	teamID, projectID := r.PathValue("teamId"), r.PathValue("projectId")
	access, err := g.resolveTeamAccess(teamID, actor.ID, projectID)
	if err != nil {
		fail(w, 403, "team_access_denied", errTeamAccess.Error())
		return
	}
	g.mu.Lock()
	browser := g.teamBrowser
	g.mu.Unlock()
	if browser == nil {
		fail(w, 409, "browser_gateway_unavailable", "Start Luci with its isolated browser gateway to open a team preview.")
		return
	}
	origin := u.Scheme + "://" + u.Host
	target, err := browser.gateway.ResolveTeamTarget(teamID, projectID, origin)
	if err != nil || !teamBrowserAllowed(access, target) {
		fail(w, 403, "team_browser_denied", "Opening this project's app requires editing and execution access; its data browser also requires deployment and secret access.")
		return
	}
	var dataTargets []previewgateway.TeamBrowserTarget
	if target.Kind != "data" && target.ClientKind != "data" && access.Permissions.Secrets && access.Permissions.Deploy {
		dataTargets, err = browser.gateway.TeamDataTargets(teamID, projectID)
		if err != nil {
			fail(w, 409, "browser_data_unavailable", "This project's browser data registrations are ambiguous. Refresh its data services.")
			return
		}
	}
	sessionHash := hashToken(strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer "))
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.teamServiceAllowsExecutionLocked(teamID) {
		fail(w, 403, "team_service_suspended", "This team's execution service is suspended. Saved project context remains available.")
		return
	}
	if !g.teamAccessCurrentLocked(teamID, actor.ID, projectID, access.Member.Revision, access.ProjectRevision) {
		fail(w, 409, "team_access_changed", "Project access changed. Refresh before opening its browser.")
		return
	}
	session, ok := g.state.Sessions[sessionHash]
	now := time.Now()
	if !ok || session.UserID != actor.ID || !now.Before(session.ExpiresAt) {
		fail(w, 401, "sign_in_required", "Sign in again before opening this preview.")
		return
	}
	for key, grant := range browser.tickets {
		if !now.Before(grant.ExpiresAt) {
			delete(browser.tickets, key)
		}
	}
	for key, grant := range browser.sessions {
		if !now.Before(grant.ExpiresAt) {
			delete(browser.sessions, key)
		}
	}
	if len(browser.tickets)+1+len(dataTargets) > 1024 || len(browser.sessions)+1+len(dataTargets) > 4096 {
		fail(w, 429, "browser_session_limit", "Too many browser connections. Close expired sessions and try again.")
		return
	}
	expires := now.Add(time.Minute)
	if session.ExpiresAt.Before(expires) {
		expires = session.ExpiresAt
	}
	destination := u.RequestURI()
	if u.Fragment != "" {
		destination += "#" + u.EscapedFragment()
	}
	ticket := randomToken()
	grant := teamBrowserGrant{teamSocketScope: teamSocketScope{TeamID: teamID, UserID: actor.ID, ProjectID: projectID, MemberRevision: access.Member.Revision, ProjectRevision: access.ProjectRevision}, SessionHash: sessionHash, Origin: origin, Destination: destination, ExpiresAt: expires}
	for _, data := range dataTargets {
		childTicket := randomToken()
		child := grant
		child.Origin, child.ParentOrigin, child.Destination = data.Origin, origin, ""
		child.DataHandoffs = nil
		browser.tickets[hashToken(childTicket)] = child
		grant.DataHandoffs = append(grant.DataHandoffs, data.Origin+teamBrowserHandoffPath+"?ticket="+url.QueryEscape(childTicket))
	}
	browser.tickets[hashToken(ticket)] = grant
	w.Header().Set("Cache-Control", "no-store")
	jsonReply(w, 200, map[string]any{"url": origin + teamBrowserHandoffPath + "?ticket=" + url.QueryEscape(ticket), "expiresAt": expires})
}

func (g *Gateway) browserGrantCurrentLocked(grant teamBrowserGrant, target previewgateway.TeamBrowserTarget) (TeamAccess, User, Policy, bool) {
	session, ok := g.state.Sessions[grant.SessionHash]
	if !ok || session.UserID != grant.UserID || !time.Now().Before(session.ExpiresAt) || !time.Now().Before(grant.ExpiresAt) || grant.Origin != target.Origin || grant.TeamID != target.TeamID || grant.ProjectID != target.ProjectID || !g.teamAccessCurrentLocked(grant.TeamID, grant.UserID, grant.ProjectID, grant.MemberRevision, grant.ProjectRevision) || !g.teamServiceAllowsExecutionLocked(grant.TeamID) {
		return TeamAccess{}, User{}, Policy{}, false
	}
	access, err := g.resolveTeamAccessLocked(grant.TeamID, grant.UserID, grant.ProjectID)
	if err != nil || !teamBrowserAllowed(access, target) {
		return TeamAccess{}, User{}, Policy{}, false
	}
	return access, g.state.Users[grant.UserID].User, g.state.Policy, true
}

// TeamBrowserAccess never uses the app's Authorization header as Luci identity.
// Only a single-use authenticated handoff can set the host-only HttpOnly cookie.
// Removing membership, its project grant or the login revokes live HTTP and WS.
func (g *Gateway) TeamBrowserAccess(w http.ResponseWriter, r *http.Request, target previewgateway.TeamBrowserTarget, next http.Handler) {
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	// Browsers intentionally omit credentials on preflight. The isolated
	// gateway validates the same-project Origin and returns only CORS metadata;
	// the subsequent data request must present its own browser grant.
	if r.Method == "OPTIONS" && (target.Kind == "data" || target.ClientKind == "data") {
		next.ServeHTTP(w, r)
		return
	}
	if r.URL.Path == teamBrowserHandoffPath {
		if r.Method != "GET" || len(r.URL.Query()["ticket"]) != 1 {
			http.Error(w, "Open a fresh preview from Luci.", 403)
			return
		}
		g.mu.Lock()
		browser := g.teamBrowser
		var grant teamBrowserGrant
		var exists bool
		if browser != nil {
			grant, exists = browser.tickets[hashToken(r.URL.Query().Get("ticket"))]
		}
		_, _, _, valid := g.browserGrantCurrentLocked(grant, target)
		if !exists || !valid {
			g.mu.Unlock()
			http.Error(w, "This browser invitation expired or access changed. Open a fresh preview from Luci.", 403)
			return
		}
		delete(browser.tickets, hashToken(r.URL.Query().Get("ticket")))
		cookie := randomToken()
		grant.ExpiresAt = g.state.Sessions[grant.SessionHash].ExpiresAt
		browser.sessions[hashToken(cookie)] = grant
		g.mu.Unlock()
		http.SetCookie(w, &http.Cookie{Name: teamBrowserCookie, Value: cookie, Path: "/", HttpOnly: true, Secure: true, SameSite: http.SameSiteNoneMode, Partitioned: true, MaxAge: int(time.Until(grant.ExpiresAt).Seconds())})
		// A script navigation originates at this trusted isolated origin. An
		// HTTP redirect would retain the cross-site initiator and force us to
		// admit indistinguishable app-authored iframe/image GETs later.
		serveTeamBrowserBootstrap(w, grant)
		return
	}
	if r.Header.Get("Origin") == "" {
		site := r.Header.Get("Sec-Fetch-Site")
		if site == "cross-site" || site == "same-site" {
			http.Error(w, "Open this project through its own authorized browser invitation.", 403)
			return
		}
	}
	cookie, err := r.Cookie(teamBrowserCookie)
	if err != nil {
		http.Error(w, "Open this team preview from Luci to verify your project access.", 401)
		return
	}
	g.mu.Lock()
	var grant teamBrowserGrant
	if g.teamBrowser != nil {
		grant = g.teamBrowser.sessions[hashToken(cookie.Value)]
	}
	access, actor, policy, valid := g.browserGrantCurrentLocked(grant, target)
	g.mu.Unlock()
	if !valid {
		http.Error(w, "Team browser access expired or was revoked. Open the project in Luci again.", 403)
		return
	}
	ctx, cancel := context.WithDeadline(r.Context(), grant.ExpiresAt)
	defer cancel()
	r = r.Clone(WithTeamAccess(withActor(ctx, scopedTeamActor(actor, access), teamPolicy(policy)), access))
	// The credential never reaches application code, redirects or logs.
	appCookies := r.Cookies()
	r.Header.Del("Cookie")
	for _, appCookie := range appCookies {
		if appCookie.Name != teamBrowserCookie {
			r.AddCookie(appCookie)
		}
	}
	socketWriter := &socketResponseWriter{ResponseWriter: w, gateway: g, userID: actor.ID, sessionHash: grant.SessionHash}
	r, finish, err := g.RegisterTeamRequest(socketWriter, r, access.Team.ID, access.ProjectID, access.Member.Revision, access.ProjectRevision)
	if err != nil {
		http.Error(w, "Team browser access changed before admission.", 403)
		return
	}
	defer finish()
	next.ServeHTTP(socketWriter, r)
}

var teamBrowserBootstrap = template.Must(template.New("team-browser").Parse(`<!doctype html><html><head><meta charset="utf-8"><title>Connecting project browser</title></head><body><p id="status" role="status">Connecting your authorized project services…</p><script nonce="{{.Nonce}}">
const config={{.Config}};
if(config.parent){ window.parent.postMessage({type:'luci-team-browser-ready'}, config.parent); }
else if(!config.data?.length){ location.replace(config.destination); }
else {
 const pending=new Map();
 function receive(event){const frame=pending.get(event.origin);if(frame && event.source===frame.contentWindow && event.data?.type==='luci-team-browser-ready'){pending.delete(event.origin);if(!pending.size){window.removeEventListener('message',receive);location.replace(config.destination);}}}
 window.addEventListener('message',receive);
 for(const source of config.data){const frame=document.createElement('iframe');frame.hidden=true;frame.title='Connect project data';pending.set(new URL(source).origin,frame);frame.src=source;document.body.append(frame);}
 setTimeout(()=>{if(pending.size){document.getElementById('status').textContent='Project data access could not be connected. Return to Luci, refresh your permissions and data services, then open the preview again.';}},15000);
}
</script></body></html>`))

func serveTeamBrowserBootstrap(w http.ResponseWriter, grant teamBrowserGrant) {
	config, _ := json.Marshal(map[string]any{"parent": grant.ParentOrigin, "data": grant.DataHandoffs, "destination": grant.Destination})
	nonce := randomToken()
	frames := []string{}
	for _, raw := range grant.DataHandoffs {
		u, _ := url.Parse(raw)
		frames = append(frames, u.Scheme+"://"+u.Host)
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Content-Security-Policy", "default-src 'none'; script-src 'nonce-"+nonce+"'; frame-src "+strings.Join(frames, " ")+"; base-uri 'none'; form-action 'none'")
	_ = teamBrowserBootstrap.Execute(w, struct {
		Nonce  string
		Config template.JS
	}{nonce, template.JS(config)})
}

package previewgateway

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
	"path/filepath"
	"strings"
	"time"
)

type Verify func(context.Context, Target) error
type Gateway struct {
	stateDir      string
	port          int
	verify        Verify
	transport     *http.Transport
	teamAccess    TeamAccessHandler
	domain        string
	managedTeamID string
}

func New(stateDir string, port int, verify Verify) (*Gateway, error) {
	if !filepath.IsAbs(stateDir) || port < 1 || port > 65535 || verify == nil {
		return nil, errors.New("invalid browser gateway configuration")
	}
	return &Gateway{stateDir: stateDir, port: port, verify: verify, transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, ResponseHeaderTimeout: 30 * time.Second, IdleConnTimeout: 30 * time.Second, MaxIdleConns: 64}}, nil
}

// NewWithPublicIngress admits only the installation's immutable team binding.
// A public hostname never enables legacy personal registrations.
func NewWithPublicIngress(stateDir string, port int, verify Verify, domain, teamID string) (*Gateway, error) {
	if err := ValidatePreviewBaseDomain(domain); err != nil {
		return nil, err
	}
	if teamID == "" || !name.MatchString(teamID) {
		return nil, errors.New("public browser ingress requires the installation's managed team binding")
	}
	g, err := New(stateDir, port, verify)
	if err != nil {
		return nil, err
	}
	g.domain, g.managedTeamID = domain, teamID
	return g, nil
}

type found struct {
	record
	owner  string
	teamID string
}

// TeamBrowserTarget is derived only from a confined team registration. A
// browser-supplied URL never selects an upstream or proves team ownership.
type TeamBrowserTarget struct {
	Target
	TeamID string
	Origin string
}

// TeamAccessHandler authenticates the isolated browser origin before invoking
// next. It owns membership/session expiry and tracks streaming connections.
type TeamAccessHandler func(http.ResponseWriter, *http.Request, TeamBrowserTarget, http.Handler)

const TeamAccessCookieName = "__Host-luci-team-browser"

// SetTeamAccessHandler is a startup-only composition hook. Without it team
// registrations remain inaccessible, including when their URL is known.
func (g *Gateway) SetTeamAccessHandler(handler TeamAccessHandler) { g.teamAccess = handler }

func (g *Gateway) ResolveTeamTarget(teamID, projectID, origin string) (TeamBrowserTarget, error) {
	var result TeamBrowserTarget
	key := browserOriginKey(origin, g.port, g.domain)
	if key == "" {
		return result, errors.New("use an owned isolated browser origin")
	}
	count := 0
	for _, row := range g.records() {
		if row.Key == key {
			count++
			result = TeamBrowserTarget{Target: row.Target, TeamID: row.teamID, Origin: origin}
		}
	}
	if key == "" || count != 1 || teamID == "" || result.TeamID != teamID || result.ProjectID != projectID {
		return TeamBrowserTarget{}, errors.New("this browser origin does not belong to the team's project")
	}
	return result, nil
}

// TeamDataTargets selects only the same private team/project registry. Browser
// bootstrapping can grant these sibling data origins without trusting a URL
// supplied by application content.
func (g *Gateway) TeamDataTargets(teamID, projectID string) ([]TeamBrowserTarget, error) {
	rows := []TeamBrowserTarget{}
	seen := map[string]bool{}
	for _, row := range g.records() {
		if row.teamID != teamID || row.ProjectID != projectID || (row.Kind != "data" && row.ClientKind != "data") {
			continue
		}
		origin := browserOrigin(row.Key, g.port, g.domain)
		if seen[origin] || len(rows) >= 8 {
			return nil, errors.New("ambiguous or oversized project browser data catalogue")
		}
		seen[origin] = true
		rows = append(rows, TeamBrowserTarget{Target: row.Target, TeamID: teamID, Origin: origin})
	}
	return rows, nil
}

func (g *Gateway) records() []found {
	dirs := []string{filepath.Join(g.stateDir, "sessions", "browser-previews")}
	workers, _ := filepath.Glob(filepath.Join(g.stateDir, "control-plane", "users", "*", "runtime", "state", "sessions", "browser-previews"))
	dirs = append(dirs, workers...)
	teams, _ := filepath.Glob(filepath.Join(g.stateDir, "control-plane", "teams", "*", "runtime", "state", "sessions", "browser-previews"))
	dirs = append(dirs, teams...)
	all := []found{}
	for _, dir := range dirs {
		if _, err := os.Lstat(dir); err != nil {
			continue
		}
		// Confine the entire directory chain as well as every registration. An
		// agent workspace may never supply a symlink to another user's records.
		if _, err := safeDirectory(g.stateDir, dir); err != nil {
			continue
		}
		files, _ := filepath.Glob(filepath.Join(dir, "*.json"))
		for _, file := range files {
			if len(all) >= 4096 {
				return all
			}
			if v, err := readRecord(dir, file, g.port); err == nil {
				teamID := ""
				if relative, err := filepath.Rel(filepath.Join(g.stateDir, "control-plane", "teams"), dir); err == nil {
					parts := strings.Split(relative, string(filepath.Separator))
					if len(parts) == 5 && parts[0] != ".." && parts[1] == "runtime" && parts[2] == "state" && parts[3] == "sessions" && parts[4] == "browser-previews" {
						teamID = parts[0]
					}
				}
				if g.domain != "" && teamID != g.managedTeamID {
					continue
				}
				all = append(all, found{record: v, owner: dir, teamID: teamID})
			}
		}
	}
	return all
}
func safeDirectory(root, dir string) (string, error) { return confine(root, dir) }
func jwtClient(value string) bool {
	if len(value) < 32 || len(value) > 4096 {
		return false
	}
	p := strings.Split(value, ".")
	if len(p) != 3 {
		return false
	}
	for _, part := range p {
		if part == "" {
			return false
		}
		if _, err := base64.RawURLEncoding.DecodeString(part); err != nil {
			return false
		}
	}
	data, err := base64.RawURLEncoding.DecodeString(p[1])
	if err != nil {
		return false
	}
	var v struct {
		Role string `json:"role"`
	}
	if json.Unmarshal(data, &v) != nil {
		return false
	}
	return v.Role == "anon" || v.Role == "authenticated"
}
func dataPath(path string) bool {
	for _, prefix := range []string{"/auth/v1", "/rest/v1", "/storage/v1", "/realtime/v1", "/functions/v1", "/graphql/v1"} {
		if path == prefix || strings.HasPrefix(path, prefix+"/") {
			return true
		}
	}
	return false
}
func stripControl(h http.Header) {
	for k := range h {
		low := strings.ToLower(k)
		if strings.HasPrefix(low, "x-luci-") || strings.HasPrefix(low, "x-forwarded-") || low == "forwarded" || low == "proxy-authorization" || low == "referer" {
			h.Del(k)
		}
	}
}
func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if r.Method == "GET" && r.URL.Path == "/healthz" && (r.Host == "127.0.0.1:"+portString(g.port) || r.Host == "localhost:"+portString(g.port)) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"status":"ready"}`))
		return
	}
	key := browserRequestKey(r.Host, g.port, g.domain)
	if key == "" {
		http.NotFound(w, r)
		return
	}
	all := g.records()
	var target *found
	for i := range all {
		if all[i].Key == key {
			if target != nil {
				http.Error(w, "Duplicate browser origin", 503)
				return
			}
			target = &all[i]
		}
	}
	if target == nil {
		http.NotFound(w, r)
		return
	}
	if target.teamID != "" {
		if g.teamAccess == nil {
			http.Error(w, "Team browser access is unavailable", 403)
			return
		}
		g.teamAccess(w, r, TeamBrowserTarget{Target: target.Target, TeamID: target.teamID, Origin: browserOrigin(target.Key, g.port, g.domain)}, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { g.serveTarget(w, r, target, all) }))
		return
	}
	g.serveTarget(w, r, target, all)
}

func (g *Gateway) serveTarget(w http.ResponseWriter, r *http.Request, target *found, all []found) {
	up, err := validate(target.Target, g.port)
	if err != nil {
		http.Error(w, "Invalid browser target", 503)
		return
	}
	origin := browserOrigin(target.Key, g.port, g.domain)
	dataTarget := target.Kind == "data" || target.ClientKind == "data"
	requestOrigin := r.Header.Get("Origin")
	allowedOrigin := requestOrigin == "" || requestOrigin == origin
	if dataTarget && requestOrigin != "" && requestOrigin != origin {
		for _, other := range all {
			if other.owner == target.owner && other.ProjectID == target.ProjectID && other.Kind != "data" && other.ClientKind != "data" && requestOrigin == browserOrigin(other.Key, g.port, g.domain) {
				allowedOrigin = true
				break
			}
		}
	}
	if !allowedOrigin || (requestOrigin == "" && r.Header.Get("Sec-Fetch-Site") == "cross-site" && r.Method != "GET" && r.Method != "HEAD") {
		http.Error(w, "Browser origin is not approved for this project", 403)
		return
	}
	if dataTarget {
		if !AllowDataClient(w, r, target.PublishableKey) {
			return
		}
		if requestOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", requestOrigin)
			if target.teamID != "" {
				w.Header().Set("Access-Control-Allow-Credentials", "true")
			}
			w.Header().Add("Vary", "Origin")
		}
		if r.Method == "OPTIONS" {
			w.Header().Set("Access-Control-Allow-Methods", "GET, HEAD, POST, PUT, PATCH, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "authorization, apikey, content-type, x-client-info, prefer, range, range-unit, x-upsert, tus-resumable, upload-offset, upload-length, upload-metadata")
			w.Header().Set("Access-Control-Max-Age", "300")
			w.WriteHeader(204)
			return
		}
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	err = g.verify(ctx, target.Target)
	cancel()
	if err != nil {
		http.Error(w, "Owned application is unavailable. Refresh it in Luci.", 503)
		return
	}
	p := &httputil.ReverseProxy{Transport: g.transport, Rewrite: func(pr *httputil.ProxyRequest) {
		pr.SetURL(up)
		pr.Out.Host = up.Host
		stripControl(pr.Out.Header)
		if requestOrigin != "" {
			pr.Out.Header.Set("Origin", up.String())
		}
	}, ModifyResponse: func(res *http.Response) error {
		stripControl(res.Header)
		// An app cannot issue a Domain=.localhost cookie that crosses sibling
		// project origins. Retain all other cookie attributes verbatim.
		cookies := res.Header.Values("Set-Cookie")
		res.Header.Del("Set-Cookie")
		for _, raw := range cookies {
			cookieName, _, _ := strings.Cut(raw, "=")
			if target.teamID != "" && strings.TrimSpace(cookieName) == TeamAccessCookieName {
				continue
			}
			parts := strings.Split(raw, ";")
			safe := []string{parts[0]}
			for _, p := range parts[1:] {
				attribute, _, _ := strings.Cut(p, "=")
				if !strings.EqualFold(strings.TrimSpace(attribute), "domain") {
					safe = append(safe, p)
				}
			}
			res.Header.Add("Set-Cookie", strings.Join(safe, ";"))
		}
		if raw := res.Header.Get("Location"); raw != "" {
			if loc, e := url.Parse(raw); e == nil && loc.Scheme == up.Scheme && loc.Host == up.Host {
				if g.domain != "" {
					loc.Scheme = "https"
				} else {
					loc.Scheme = "http"
				}
				loc.Host = r.Host
				res.Header.Set("Location", loc.String())
			}
		}
		if dataTarget {
			// The response writer already owns the single approved CORS
			// origin, including error responses and preflight. ReverseProxy
			// appends upstream headers, so repeating it here creates two
			// Allow-Origin fields and browsers reject the successful response.
			res.Header.Del("Access-Control-Allow-Origin")
			res.Header.Del("Access-Control-Allow-Credentials")
		}
		res.Header.Set("Referrer-Policy", "no-referrer")
		return nil
	}, ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "Application connection is unavailable", 502)
	}}
	p.ServeHTTP(w, r)
}

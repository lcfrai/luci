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
	stateDir  string
	port      int
	verify    Verify
	transport *http.Transport
}

func New(stateDir string, port int, verify Verify) (*Gateway, error) {
	if !filepath.IsAbs(stateDir) || port < 1 || port > 65535 || verify == nil {
		return nil, errors.New("invalid browser gateway configuration")
	}
	return &Gateway{stateDir: stateDir, port: port, verify: verify, transport: &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, ResponseHeaderTimeout: 30 * time.Second, IdleConnTimeout: 30 * time.Second, MaxIdleConns: 64}}, nil
}

type found struct {
	record
	owner string
}

func (g *Gateway) records() []found {
	dirs := []string{filepath.Join(g.stateDir, "sessions", "browser-previews")}
	workers, _ := filepath.Glob(filepath.Join(g.stateDir, "control-plane", "users", "*", "runtime", "state", "sessions", "browser-previews"))
	dirs = append(dirs, workers...)
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
				all = append(all, found{record: v, owner: dir})
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
	key := requestKey(r.Host, g.port)
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
	up, err := validate(target.Target, g.port)
	if err != nil {
		http.Error(w, "Invalid browser target", 503)
		return
	}
	origin := "http://" + r.Host
	requestOrigin := r.Header.Get("Origin")
	allowedOrigin := requestOrigin == "" || requestOrigin == origin
	if target.Kind == "data" && requestOrigin != "" && requestOrigin != origin {
		for _, other := range all {
			if other.owner == target.owner && other.ProjectID == target.ProjectID && other.Kind != "data" && requestOrigin == "http://"+other.Key+".localhost:"+portString(g.port) {
				allowedOrigin = true
				break
			}
		}
	}
	if !allowedOrigin || (requestOrigin == "" && r.Header.Get("Sec-Fetch-Site") == "cross-site" && r.Method != "GET" && r.Method != "HEAD") {
		http.Error(w, "Browser origin is not approved for this project", 403)
		return
	}
	if target.Kind == "data" {
		if !dataPath(r.URL.Path) {
			http.NotFound(w, r)
			return
		}
		auth := r.Header.Get("Authorization")
		if auth != "" && (!strings.HasPrefix(auth, "Bearer ") || !jwtClient(strings.TrimPrefix(auth, "Bearer "))) {
			http.Error(w, "Use a Supabase client user token", 401)
			return
		}
		if key := r.Header.Get("apikey"); key != "" && key != target.PublishableKey {
			http.Error(w, "Use this project's publishable key", 401)
			return
		}
		q := r.URL.Query()
		if key := q.Get("apikey"); key != "" && key != target.PublishableKey {
			http.Error(w, "Use this project's publishable key", 401)
			return
		}
		if token := q.Get("access_token"); token != "" && !jwtClient(token) {
			http.Error(w, "Use a Supabase client user token", 401)
			return
		}
		if requestOrigin != "" {
			w.Header().Set("Access-Control-Allow-Origin", requestOrigin)
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
			parts := strings.Split(raw, ";")
			safe := []string{parts[0]}
			for _, p := range parts[1:] {
				if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(p)), "domain=") {
					safe = append(safe, p)
				}
			}
			res.Header.Add("Set-Cookie", strings.Join(safe, ";"))
		}
		if raw := res.Header.Get("Location"); raw != "" {
			if loc, e := url.Parse(raw); e == nil && loc.Scheme == up.Scheme && loc.Host == up.Host {
				loc.Scheme = "http"
				loc.Host = r.Host
				res.Header.Set("Location", loc.String())
			}
		}
		if target.Kind == "data" {
			res.Header.Del("Access-Control-Allow-Origin")
			res.Header.Del("Access-Control-Allow-Credentials")
			if requestOrigin != "" {
				res.Header.Set("Access-Control-Allow-Origin", requestOrigin)
				res.Header.Add("Vary", "Origin")
			}
		}
		res.Header.Set("Referrer-Policy", "no-referrer")
		return nil
	}, ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
		http.Error(w, "Application connection is unavailable", 502)
	}}
	p.ServeHTTP(w, r)
}

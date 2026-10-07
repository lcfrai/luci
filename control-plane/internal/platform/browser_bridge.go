package platform

import (
	"context"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/previewgateway"
)

// BrowserTarget is client-safe metadata. The browser never supplies an upstream
// URL or learns the private container port or administrative data credentials.
type BrowserTarget struct {
	Kind           string `json:"kind"`
	ProjectID      string `json:"projectId"`
	ResourceID     string `json:"resourceId"`
	PublishableKey string `json:"publishableKey,omitempty"`
}

// Cloud deployment transport supplies a verified desktop-side data bridge. It
// changes only the application's client endpoint; private service bindings stay
// on the selected Docker network. Browser-authored headers cannot use this path.
func (s *Service) deploymentBrowserData(r *http.Request, projectID string, values map[string]string) error {
	origin, resource := r.Header.Get("X-Luci-Workspace-Data-Origin"), r.Header.Get("X-Luci-Workspace-Data-Resource")
	if origin == "" && resource == "" {
		return nil
	}
	if !identifier.MatchString(resource) {
		return errors.New("active data resource binding required")
	}
	if r.Header.Get("X-Luci-Preview-Bridge") != "1" {
		return errors.New("private browser data bridge required")
	}
	for _, name := range []string{"Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest", "Sec-Fetch-User"} {
		if r.Header.Get(name) != "" {
			return errors.New("browser headers cannot select a data bridge")
		}
	}
	u, err := url.Parse(origin)
	_, controlPort, _ := net.SplitHostPort(r.Host)
	port, portErr := strconv.Atoi(uPort(u))
	capabilityHost := false
	if u != nil {
		capabilityHost = previewgateway.IsCapabilityHostname(u.Hostname())
	}
	if err != nil || u == nil || u.Scheme != "http" || u.User != nil || u.Path != "" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || portErr != nil || port < 1 || port > 65535 || u.Port() == controlPort || u.Port() == "8730" || (!capabilityHost && u.Hostname() != "127.0.0.1") {
		return errors.New("data bridge requires a separate protected browser origin")
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	meta, _, err := s.resolveBrowserTarget(ctx, projectID, "data", resource)
	if err != nil || values["SUPABASE_ANON_KEY"] != meta.PublishableKey {
		return errors.New("active data binding changed before deployment")
	}
	values["SUPABASE_PUBLIC_URL"] = origin
	return nil
}

func uPort(u *url.URL) string {
	if u == nil {
		return ""
	}
	return u.Port()
}

func (s *Service) registerBrowserBridge(mux *http.ServeMux) {
	mux.HandleFunc("GET /v1/platform/projects/{id}/browser-target", s.browserTargetMetadata)
	for _, method := range []string{"GET", "HEAD", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		mux.HandleFunc(method+" /v1/platform/projects/{id}/browser/{kind}/{resourceId}/{path...}", s.proxyBrowserTarget)
	}
}

func (s *Service) resolveBrowserTarget(ctx context.Context, projectID, kind, resourceID string) (BrowserTarget, string, error) {
	meta := BrowserTarget{Kind: kind, ProjectID: projectID, ResourceID: resourceID}
	if kind == "application" {
		if !identifier.MatchString(resourceID) {
			return meta, "", errors.New("invalid application identity")
		}
		s.mu.Lock()
		state, err := s.load(projectID)
		s.mu.Unlock()
		if err != nil {
			return meta, "", err
		}
		for _, deployment := range state.Deployments {
			if deployment.ID != resourceID || deployment.Provider != "docker" || deployment.Status != "ready" {
				continue
			}
			info, err := s.inspect(ctx, projectID, deployment.ID, deployment.Container)
			if err != nil || !info.State.Running || (deployment.Image != "" && deployment.Image != info.Image) {
				return meta, "", errors.New("owned application is unavailable")
			}
			address, err := loopbackURL(info, deployment.Port)
			return meta, address, err
		}
		return meta, "", errors.New("owned running deployment required")
	}
	if kind != "data" {
		return meta, "", errors.New("invalid browser resource kind")
	}
	s.mu.Lock()
	var install dataInstallation
	err := s.read(projectID+"-data-installation", &install)
	state, stateErr := s.loadData(projectID)
	s.mu.Unlock()
	if err != nil || stateErr != nil || install.ProjectID != projectID || state.Status != "ready" || (state.Operation != nil && state.Operation.Status == "running") {
		return meta, "", errors.New("owned ready data services required")
	}
	meta.ResourceID = install.storageTenant()
	if !identifier.MatchString(meta.ResourceID) || (resourceID != "" && resourceID != meta.ResourceID) {
		return meta, "", errors.New("data identity changed")
	}
	secret, err := s.dataCredentials(install)
	if err != nil {
		return meta, "", err
	}
	info, err := s.inspect(ctx, projectID, "data-"+install.StackID+"-api-gw", s.dataName(install, "api-gw"))
	if err != nil || !info.State.Running || info.State.Health.Status == "unhealthy" || info.State.Health.Status == "starting" {
		return meta, "", errors.New("owned data gateway is unavailable")
	}
	address, err := loopbackURL(info, 8000)
	if err != nil || address != dataURL(install) {
		return meta, "", errors.New("owned data port changed")
	}
	meta.PublishableKey = secret.Anon
	return meta, address, nil
}

func (s *Service) browserTargetMetadata(w http.ResponseWriter, r *http.Request) {
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	meta, _, err := s.resolveBrowserTarget(ctx, p.ID, r.URL.Query().Get("kind"), r.URL.Query().Get("resourceId"))
	if err != nil {
		problem(w, 409, "browser_resource_unavailable", "Verify the owned running application or data services before connecting its browser address.")
		return
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, 200, meta)
}

func stripBrowserControl(h http.Header) {
	for name := range h {
		lower := strings.ToLower(name)
		if lower == "authorization" || lower == "proxy-authorization" || lower == "origin" || lower == "referer" || lower == "forwarded" || strings.HasPrefix(lower, "x-luci-") || strings.HasPrefix(lower, "x-forwarded-") || strings.HasPrefix(lower, "sec-fetch-") {
			h.Del(name)
		}
	}
}

func (s *Service) proxyBrowserTarget(w http.ResponseWriter, r *http.Request) {
	if r.Header.Get("X-Luci-Preview-Bridge") != "1" {
		problem(w, 403, "browser_bridge_required", "Open this service through Luci's separate browser address.")
		return
	}
	for _, name := range []string{"Sec-Fetch-Site", "Sec-Fetch-Mode", "Sec-Fetch-Dest", "Sec-Fetch-User"} {
		if r.Header.Get(name) != "" {
			problem(w, 403, "browser_bridge_required", "Browser content cannot use the control origin.")
			return
		}
	}
	p, ok := s.project(w, r)
	if !ok {
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	meta, address, err := s.resolveBrowserTarget(ctx, p.ID, r.PathValue("kind"), r.PathValue("resourceId"))
	cancel()
	if err != nil {
		problem(w, 409, "browser_resource_unavailable", "The owned browser resource has stopped or changed. Refresh it in Luci.")
		return
	}
	upstream, err := url.Parse(address)
	if err != nil || upstream.Port() == "8730" || upstream.Port() == "8731" {
		problem(w, 409, "browser_resource_unavailable", "A browser resource cannot use a control port.")
		return
	}
	appPath := "/" + r.PathValue("path")
	rawPath := ""
	prefix := "/browser/" + meta.Kind + "/" + url.PathEscape(meta.ResourceID) + "/"
	if offset := strings.Index(r.URL.EscapedPath(), prefix); offset >= 0 {
		candidate := "/" + r.URL.EscapedPath()[offset+len(prefix):]
		if decoded, e := url.PathUnescape(candidate); e == nil && decoded == appPath {
			rawPath = candidate
		}
	}
	clientAuth := r.Header.Get("X-Luci-Preview-App-Authorization")
	if len(clientAuth) > 8192 {
		problem(w, 400, "browser_request_invalid", "The application authorization header is too large.")
		return
	}
	if meta.Kind == "data" {
		client := r.Clone(r.Context())
		u := *r.URL
		u.Path = appPath
		u.RawPath = rawPath
		client.URL = &u
		client.Header = r.Header.Clone()
		client.Header.Del("Authorization")
		if clientAuth != "" {
			client.Header.Set("Authorization", clientAuth)
		}
		if !previewgateway.AllowDataClient(w, client, meta.PublishableKey) {
			return
		}
	}
	transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, ResponseHeaderTimeout: 30 * time.Second}
	defer transport.CloseIdleConnections()
	proxy := &httputil.ReverseProxy{Transport: transport, FlushInterval: -1, ErrorLog: log.New(io.Discard, "", 0), Rewrite: func(pr *httputil.ProxyRequest) {
		pr.SetURL(upstream)
		pr.Out.URL.Path = appPath
		pr.Out.URL.RawPath = rawPath
		pr.Out.URL.RawQuery = pr.In.URL.RawQuery
		if meta.Kind == "data" {
			q := pr.Out.URL.Query()
			q.Del("ticket")
			pr.Out.URL.RawQuery = q.Encode()
		}
		pr.Out.Host = upstream.Host
		stripBrowserControl(pr.Out.Header)
		if meta.Kind == "data" {
			pr.Out.Header.Del("Cookie")
		}
		if clientAuth != "" {
			pr.Out.Header.Set("Authorization", clientAuth)
		}
	}, ModifyResponse: func(res *http.Response) error {
		stripBrowserControl(res.Header)
		if location, e := url.Parse(res.Header.Get("Location")); e == nil && location.Host == upstream.Host && (location.Scheme == "" || location.Scheme == upstream.Scheme) {
			location.Host = ""
			location.Scheme = ""
			res.Header.Set("Location", location.String())
		}
		return nil
	}, ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
		problem(w, 502, "browser_resource_unavailable", "The owned service did not answer.")
	}}
	proxy.ServeHTTP(w, r)
}

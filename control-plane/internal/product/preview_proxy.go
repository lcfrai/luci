package product

import (
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
)

// registerPreviewProxy is consumed through a separate browser origin by the
// workspace bridge. An application must never share the Luci control origin.
func (s *Server) registerPreviewProxy(mux Registrar) {
	for _, method := range []string{http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions} {
		mux.HandleFunc(method+" /v1/product/projects/{id}/releases/{releaseId}/preview/{path...}", s.proxyReleasePreview)
	}
}

func recordedPreviewOrigin(raw string) (*url.URL, error) {
	u, err := url.Parse(raw)
	if err != nil || u.Scheme != "http" || u.User != nil || u.Opaque != "" || u.Hostname() != "127.0.0.1" || u.Path != "" || u.RawPath != "" || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" {
		return nil, errors.New("release does not contain a local preview origin")
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil || port < 1 || port > 65535 || u.Host != net.JoinHostPort("127.0.0.1", strconv.Itoa(port)) {
		return nil, errors.New("release does not contain a valid local preview port")
	}
	return u, nil
}

func (s *Server) proxyReleasePreview(w http.ResponseWriter, r *http.Request) {
	// A fixed marker identifies the server-to-server bridge, not an auth
	// secret. Browser navigations cannot attach it; browser fetch metadata
	// must never reach this endpoint through the control-origin API proxy.
	if r.Header.Get("X-Luci-Preview-Bridge") != "1" || r.Header.Get("Sec-Fetch-Site") != "" || r.Header.Get("Sec-Fetch-Mode") != "" || r.Header.Get("Sec-Fetch-Dest") != "" {
		journeyError(w, http.StatusForbidden, "preview_bridge_required", "Open this preview from its separate preview address.")
		return
	}
	p, ok := s.requestProject(w, r)
	if !ok {
		return
	}
	s.mu.RLock()
	rows, err := s.readReleases(p.ID)
	s.mu.RUnlock()
	if err != nil {
		journeyError(w, http.StatusInternalServerError, "releases_unavailable", "The saved release history could not be read.")
		return
	}
	var row *storedRelease
	for i := range rows {
		if rows[i].ID == r.PathValue("releaseId") {
			row = &rows[i]
			break
		}
	}
	if row == nil {
		journeyError(w, http.StatusNotFound, "release_missing", "This project has no release with that ID.")
		return
	}
	if row.Target != "local" || row.Status != "ready" {
		journeyError(w, http.StatusConflict, "preview_not_running", "This release has no ready local preview.")
		return
	}
	target, err := recordedPreviewOrigin(row.URL)
	if err != nil {
		journeyError(w, http.StatusConflict, "invalid_preview_origin", "This release has an invalid local preview address.")
		return
	}
	// Refuse a record pointing back at the control listener, including a
	// different loopback spelling. Docker previews use an independent port.
	_, requestPort, _ := net.SplitHostPort(r.Host)
	if target.Port() == requestPort || target.Port() == "8730" {
		journeyError(w, http.StatusConflict, "invalid_preview_origin", "A preview cannot use the Luci control port.")
		return
	}
	path := "/" + r.PathValue("path")
	rawPath := ""
	// Preserve encoded slashes and spaces in application paths. Project and
	// release IDs are canonical route segments; only the wildcard is forwarded.
	marker := "/releases/" + url.PathEscape(row.ID) + "/preview/"
	if offset := strings.Index(r.URL.EscapedPath(), marker); offset >= 0 {
		candidate := "/" + r.URL.EscapedPath()[offset+len(marker):]
		if decoded, decodeErr := url.PathUnescape(candidate); decodeErr == nil && decoded == path {
			rawPath = candidate
		}
	}
	transport := &http.Transport{
		Proxy:                 nil,
		DialContext:           (&net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}).DialContext,
		ResponseHeaderTimeout: 30 * time.Second,
		DisableKeepAlives:     true,
	}
	defer transport.CloseIdleConnections()
	proxy := &httputil.ReverseProxy{
		Transport:     transport,
		FlushInterval: -1,
		ErrorLog:      log.New(io.Discard, "", 0),
		ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			journeyError(w, http.StatusBadGateway, "preview_unavailable", "The recorded preview is not answering. Restart or recreate it in Deploy.")
		},
		ModifyResponse: func(response *http.Response) error {
			// Keep application redirects on the per-release browser origin.
			// Relative and external redirects retain their application meaning.
			location, parseErr := url.Parse(response.Header.Get("Location"))
			if parseErr == nil && location.Host == target.Host && (location.Scheme == "" || location.Scheme == target.Scheme) && location.User == nil {
				location.Host, location.Scheme = "", ""
				if location.Path == "" {
					location.Path = "/"
				}
				response.Header.Set("Location", location.String())
			}
			return nil
		},
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(target)
			pr.Out.URL.Path = path
			pr.Out.URL.RawPath = rawPath
			// Control routing uses only the recorded release and path. Preserve
			// app query encoding, including signed URLs, without interpreting it.
			pr.Out.URL.RawQuery = pr.In.URL.RawQuery
			pr.Out.URL.ForceQuery = pr.In.URL.ForceQuery
			pr.Out.Host = target.Host
			// App cookies and websocket subprotocols belong to the isolated
			// preview origin. Never forward control/bridge authorization.
			for name := range pr.Out.Header {
				lower := strings.ToLower(name)
				if lower == "authorization" || lower == "proxy-authorization" || lower == "referer" || lower == "forwarded" || strings.HasPrefix(lower, "x-forwarded-") || strings.HasPrefix(lower, "x-luci-") {
					pr.Out.Header.Del(name)
				}
			}
			// The authenticated server bridge separates ordinary application
			// authorization from the Luci bearer used to reach the worker.
			if appAuthorization := pr.In.Header.Get("X-Luci-Preview-App-Authorization"); appAuthorization != "" && len(appAuthorization) <= 8192 {
				pr.Out.Header.Set("Authorization", appAuthorization)
			}
			if pr.In.Header.Get("Origin") != "" {
				pr.Out.Header.Set("Origin", target.String())
			}
		},
	}
	proxy.ServeHTTP(w, r)
}

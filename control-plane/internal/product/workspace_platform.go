package product

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"strings"
	"time"

	"github.com/lcfrai/luci/control-plane/internal/platform"
	"github.com/lcfrai/luci/control-plane/internal/previewgateway"
)

func platformPreviewKey(kind, projectID, resourceID string) string {
	if kind == "data" {
		resourceID = "active"
	}
	return "platform/" + kind + "/" + projectID + "/" + resourceID
}

// Preparing data transport is part of the user's explicit deployment POST.
// A read never starts a listener, and no browser-supplied URL becomes an input.
func (s *Server) cloudDeploymentBrowserData(r *http.Request, connectionID, projectID string) (platform.BrowserTarget, string, error) {
	tunnel, credential, err := s.platformWorkspaceConnection(connectionID, "")
	if err != nil {
		return platform.BrowserTarget{}, "", err
	}
	var data platform.DataStatus
	if err = remoteWorkspaceAPI(r.Context(), tunnel.APIURL, "GET", "/v1/platform/projects/"+url.PathEscape(projectID)+"/data", credential.Token, nil, &data); err != nil {
		return platform.BrowserTarget{}, "", err
	}
	if data.ProjectID != projectID {
		return platform.BrowserTarget{}, "", errors.New("data project binding mismatch")
	}
	if data.Status == "not-installed" {
		return platform.BrowserTarget{}, "", nil
	}
	if data.Status != "ready" {
		return platform.BrowserTarget{}, "", errors.New("verify the active data services before deploying a connected app")
	}
	body, _ := json.Marshal(map[string]string{"projectId": projectID, "kind": "data"})
	request := httptest.NewRequest("POST", "/v1/setup/workspace/platform/browser", bytes.NewReader(body)).WithContext(r.Context())
	recorder := httptest.NewRecorder()
	s.prepareWorkspacePlatformBrowser(recorder, request)
	var opened struct {
		URL        string `json:"url"`
		ProjectID  string `json:"projectId"`
		Kind       string `json:"kind"`
		ResourceID string `json:"resourceId"`
	}
	if recorder.Code != 200 || json.Unmarshal(recorder.Body.Bytes(), &opened) != nil || opened.URL == "" || opened.ProjectID != projectID || opened.Kind != "data" {
		return platform.BrowserTarget{}, "", errors.New("could not prepare the owned data browser connection")
	}
	return platform.BrowserTarget{ProjectID: projectID, Kind: "data", ResourceID: opened.ResourceID}, opened.URL, nil
}

func (s *Server) platformWorkspaceConnection(connectionID, userID string) (Tunnel, workspaceCredential, error) {
	tunnel, credential, err := s.connectedWorkspace(connectionID)
	if err != nil || tunnel.KnownHost == "" || credential.Token == "" || credential.User.ID == "" || (userID != "" && credential.User.ID != userID) {
		return tunnel, credential, errors.New("reconnect and sign in to this backing")
	}
	s.mu.RLock()
	active := s.workspace.Mode == "cloud" && s.workspace.ConnectionID == connectionID
	s.mu.RUnlock()
	if !active {
		return tunnel, credential, errors.New("workspace changed")
	}
	return tunnel, credential, nil
}

// Listener allocation is an explicit POST. Metadata reads and DTO rewriting
// never open sockets or present a remote machine's loopback as a usable URL.
func (s *Server) prepareWorkspacePlatformBrowser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		ProjectID  string `json:"projectId"`
		Kind       string `json:"kind"`
		ResourceID string `json:"resourceId,omitempty"`
	}
	if !readJSON(w, r, &req) {
		return
	}
	if !coolifyID.MatchString(req.ProjectID) || (req.Kind != "application" && req.Kind != "data") || (req.Kind == "application" && !coolifyID.MatchString(req.ResourceID)) || (req.Kind == "data" && req.ResourceID != "") {
		problem(w, 400, "browser_request_invalid", "Choose an owned application or the active data services.")
		return
	}
	s.mu.RLock()
	ws := s.workspace
	s.mu.RUnlock()
	tunnel, credential, err := s.platformWorkspaceConnection(ws.ConnectionID, "")
	if err != nil {
		problem(w, 409, "backing_sign_in_required", "Reconnect and sign in to your selected cloud workspace.")
		return
	}
	var meta platform.BrowserTarget
	query := url.Values{"kind": {req.Kind}, "resourceId": {req.ResourceID}}
	if err = remoteWorkspaceAPI(r.Context(), tunnel.APIURL, "GET", "/v1/platform/projects/"+url.PathEscape(req.ProjectID)+"/browser-target?"+query.Encode(), credential.Token, nil, &meta); err != nil {
		problem(w, 409, "browser_resource_unavailable", "Verify the owned application or data services, then connect its browser address again.")
		return
	}
	if meta.Kind != req.Kind || meta.ProjectID != req.ProjectID || !coolifyID.MatchString(meta.ResourceID) || (req.Kind == "application" && meta.ResourceID != req.ResourceID) || (req.Kind == "data" && (len(meta.PublishableKey) < 32 || len(meta.PublishableKey) > 4096)) {
		problem(w, 409, "browser_resource_unavailable", "The backing returned an invalid browser resource binding.")
		return
	}
	key := platformPreviewKey(meta.Kind, meta.ProjectID, meta.ResourceID)
	s.mu.Lock()
	if s.workspace.Mode != "cloud" || s.workspace.ConnectionID != ws.ConnectionID || s.workspaceAuth[workspaceCredentialKey(tunnel)].User.ID != credential.User.ID {
		s.mu.Unlock()
		problem(w, 409, "workspace_changed", "The backing changed. Connect this service again.")
		return
	}
	if old := s.previews[key]; old != nil && old.connectionID == ws.ConnectionID && old.remoteUserID == credential.User.ID && old.clientResourceID == meta.ResourceID && old.publishableKey == meta.PublishableKey {
		s.mu.Unlock()
		writeJSON(w, 200, map[string]string{"url": old.url, "projectId": meta.ProjectID, "kind": meta.Kind, "resourceId": meta.ResourceID})
		return
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		s.mu.Unlock()
		problem(w, 500, "browser_unavailable", "Could not open the protected browser connection.")
		return
	}
	origin := "http://" + ln.Addr().String()
	bridgeKey, err := previewgateway.NewBridgeKey()
	if err != nil {
		_ = ln.Close()
		s.mu.Unlock()
		problem(w, 500, "browser_unavailable", "Could not secure the private browser bridge.")
		return
	}
	identity := sha256.Sum256([]byte(workspaceCredentialKey(tunnel) + "\x00" + credential.User.ID + "\x00" + meta.Kind + "\x00" + meta.ProjectID + "\x00" + meta.ResourceID))
	registryResource := "platform-" + hex.EncodeToString(identity[:16])
	bridge := &workspacePreview{projectID: meta.ProjectID, releaseID: registryResource, connectionID: ws.ConnectionID, remoteUserID: credential.User.ID, clientKind: meta.Kind, clientResourceID: meta.ResourceID, publishableKey: meta.PublishableKey, url: origin}
	bridge.server = &http.Server{ReadHeaderTimeout: 10 * time.Second, Handler: s.platformBridgeHandler(bridge, origin, bridgeKey)}
	if s.browserRegistry != nil {
		bridge.url, err = s.browserRegistry.Publish(previewgateway.Target{Kind: "bridge", ClientKind: meta.Kind, ProjectID: meta.ProjectID, ResourceID: registryResource, Upstream: origin, BridgeKey: bridgeKey, PublishableKey: meta.PublishableKey})
		if err != nil {
			_ = ln.Close()
			s.mu.Unlock()
			problem(w, 500, "browser_unavailable", "Could not register the separate browser address.")
			return
		}
	}
	old := s.previews[key]
	s.previews[key] = bridge
	s.mu.Unlock()
	if old != nil {
		_ = old.server.Close()
	}
	go bridge.server.Serve(ln)
	writeJSON(w, 200, map[string]string{"url": bridge.url, "projectId": meta.ProjectID, "kind": meta.Kind, "resourceId": meta.ResourceID})
}

func (s *Server) platformBridgeHandler(bridge *workspacePreview, origin, bridgeKey string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if "http://"+r.Host != origin {
			problem(w, 403, "host_rejected", "Use the separate browser address shown by Luci.")
			return
		}
		tunnel, credential, err := s.platformWorkspaceConnection(bridge.connectionID, bridge.remoteUserID)
		if err != nil {
			problem(w, 503, "workspace_disconnected", "Reconnect and sign in to this workspace in Luci.")
			return
		}
		if r.Method == "GET" && r.URL.Path == previewgateway.BridgeProbePath {
			proof := previewgateway.BridgeProof(bridgeKey, r.Header.Get(previewgateway.BridgeNonceHeader))
			if proof == "" {
				problem(w, 403, "bridge_probe_rejected", "Invalid bridge challenge.")
				return
			}
			w.Header().Set(previewgateway.BridgeProofHeader, proof)
			w.WriteHeader(204)
			return
		}
		requestOrigin := r.Header.Get("Origin")
		allowedOrigin := requestOrigin == "" || requestOrigin == origin
		if bridge.clientKind == "data" && requestOrigin != "" && !allowedOrigin {
			s.mu.RLock()
			for _, other := range s.previews {
				if other.connectionID == bridge.connectionID && other.projectID == bridge.projectID && other.remoteUserID == bridge.remoteUserID && other.clientKind != "data" && other.url == requestOrigin {
					allowedOrigin = true
					break
				}
			}
			s.mu.RUnlock()
		}
		if !allowedOrigin || (requestOrigin == "" && r.Header.Get("Sec-Fetch-Site") == "cross-site" && r.Method != "GET" && r.Method != "HEAD") {
			problem(w, 403, "origin_rejected", "Only this project's separate browser origins can use the service.")
			return
		}
		if bridge.clientKind == "data" {
			if !previewgateway.AllowDataClient(w, r, bridge.publishableKey) {
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
		upstream, parseErr := url.Parse(tunnel.APIURL)
		if parseErr != nil || upstream.Scheme != "http" || upstream.Hostname() != "127.0.0.1" || upstream.User != nil || upstream.Path != "" || upstream.RawQuery != "" || upstream.Fragment != "" {
			problem(w, 503, "workspace_disconnected", "The verified SSH API connection is unavailable.")
			return
		}
		prefix := "/v1/platform/projects/" + url.PathEscape(bridge.projectID) + "/browser/" + bridge.clientKind + "/" + url.PathEscape(bridge.clientResourceID)
		transport := &http.Transport{Proxy: nil, DialContext: (&net.Dialer{Timeout: 5 * time.Second}).DialContext, ResponseHeaderTimeout: 30 * time.Second}
		defer transport.CloseIdleConnections()
		proxy := &httputil.ReverseProxy{Transport: transport, FlushInterval: -1, ErrorLog: log.New(io.Discard, "", 0), Rewrite: func(pr *httputil.ProxyRequest) {
			appAuth := pr.In.Header.Get("Authorization")
			pr.SetURL(upstream)
			pr.Out.URL.Path = prefix + pr.In.URL.Path
			if pr.In.URL.RawPath != "" {
				pr.Out.URL.RawPath = prefix + pr.In.URL.RawPath
			}
			pr.Out.Host = upstream.Host
			for name := range pr.Out.Header {
				lower := strings.ToLower(name)
				if lower == "authorization" || lower == "proxy-authorization" || lower == "origin" || lower == "referer" || lower == "forwarded" || strings.HasPrefix(lower, "x-luci-") || strings.HasPrefix(lower, "x-forwarded-") || strings.HasPrefix(lower, "sec-fetch-") {
					pr.Out.Header.Del(name)
				}
			}
			pr.Out.Header.Set("Authorization", "Bearer "+credential.Token)
			if bridge.clientKind == "data" {
				pr.Out.Header.Del("Cookie")
			}
			pr.Out.Header.Set("X-Luci-Preview-Bridge", "1")
			if appAuth != "" && len(appAuth) <= 8192 {
				pr.Out.Header.Set("X-Luci-Preview-App-Authorization", appAuth)
			}
		}, ModifyResponse: func(res *http.Response) error {
			for name := range res.Header {
				lower := strings.ToLower(name)
				if strings.HasPrefix(lower, "x-luci-") || strings.HasPrefix(lower, "x-forwarded-") || lower == "forwarded" {
					res.Header.Del(name)
				}
			}
			res.Header.Set("Referrer-Policy", "no-referrer")
			if bridge.clientKind == "data" {
				res.Header.Del("Access-Control-Allow-Origin")
				res.Header.Del("Access-Control-Allow-Credentials")
				if requestOrigin != "" {
					res.Header.Set("Access-Control-Allow-Origin", requestOrigin)
					res.Header.Add("Vary", "Origin")
				}
			}
			cookies := res.Header.Values("Set-Cookie")
			res.Header.Del("Set-Cookie")
			for _, raw := range cookies {
				parts := strings.Split(raw, ";")
				safe := []string{parts[0]}
				for _, part := range parts[1:] {
					if !strings.HasPrefix(strings.ToLower(strings.TrimSpace(part)), "domain=") {
						safe = append(safe, part)
					}
				}
				res.Header.Add("Set-Cookie", strings.Join(safe, ";"))
			}
			return nil
		}, ErrorHandler: func(w http.ResponseWriter, _ *http.Request, _ error) {
			problem(w, 503, "browser_unavailable", "The selected workspace service did not answer.")
		}}
		proxy.ServeHTTP(w, r)
	})
}

func (s *Server) rewriteWorkspacePlatformResponse(value any, connectionID string) {
	switch v := value.(type) {
	case []any:
		for _, child := range v {
			s.rewriteWorkspacePlatformResponse(child, connectionID)
		}
	case map[string]any:
		projectID, _ := v["projectId"].(string)
		kind, resourceID := "", ""
		if v["provider"] == "docker" && projectID != "" {
			kind = "application"
			resourceID, _ = v["id"].(string)
		}
		if v["provider"] == "supabase" && projectID != "" && v["endpointScope"] != nil {
			kind = "data"
		}
		if kind != "" {
			v["url"] = ""
			s.mu.RLock()
			bridge := s.previews[platformPreviewKey(kind, projectID, resourceID)]
			tunnel := s.tunnels[connectionID]
			userID := ""
			if tunnel != nil {
				userID = s.workspaceAuth[workspaceCredentialKey(*tunnel)].User.ID
			}
			s.mu.RUnlock()
			usable := bridge != nil && bridge.connectionID == connectionID && bridge.remoteUserID == userID && v["status"] == "ready" && (kind != "data" || v["publishableKey"] == bridge.publishableKey)
			if usable {
				v["url"] = bridge.url
			}
			if kind == "application" {
				v["browserConnectionRequired"] = !usable && v["status"] == "ready"
			} else if usable {
				v["endpointScope"] = "browser-gateway"
			} else {
				v["endpointScope"] = "workspace-browser-pending"
			}
		}
		for _, child := range v {
			s.rewriteWorkspacePlatformResponse(child, connectionID)
		}
	}
}

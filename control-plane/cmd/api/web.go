package main

import (
	"github.com/lcfrai/luci/control-plane/internal/httpx"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// A portable download serves the same UI as the native application.
func withWeb(api http.Handler, cfg Config) http.Handler {
	root := cfg.WebDir
	if root == "" {
		exe, err := os.Executable()
		if err != nil {
			return api
		}
		root = filepath.Join(filepath.Dir(exe), "web")
	}
	if info, err := os.Stat(filepath.Join(root, "index.html")); err != nil || !info.Mode().IsRegular() {
		return api
	}
	files := http.FileServer(http.Dir(root))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/v1/") || strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		publicHost := ""
		if cfg.PublicAppOrigin != "" {
			parsed, _ := url.Parse(cfg.PublicAppOrigin)
			publicHost = parsed.Host
		}
		if !httpx.LoopbackHost(r.Host) && (publicHost == "" || r.Host != publicHost) {
			http.Error(w, "Luci's local UI requires a loopback host", 403)
			return
		}
		if r.Method != "GET" && r.Method != "HEAD" {
			http.Error(w, "method not allowed", 405)
			return
		}
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", webCSP(cfg))
		w.Header().Set("Cache-Control", "no-cache")
		files.ServeHTTP(w, r)
	})
}

func webCSP(cfg Config) string {
	policy := "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data: blob:; connect-src 'self' http://127.0.0.1:* http://localhost:* ws://127.0.0.1:* ws://localhost:*; worker-src 'self' blob:; frame-src http://127.0.0.1:* http://localhost:* https:; object-src 'none'; base-uri 'self'; form-action 'self'"
	if cfg.PublicAppOrigin != "" {
		app, _ := url.Parse(cfg.PublicAppOrigin)
		policy = "default-src 'self'; script-src 'self' 'wasm-unsafe-eval'; style-src 'self' 'unsafe-inline'; img-src 'self' data: blob:; font-src 'self' data: blob:; connect-src 'self' wss://" + app.Host + "; worker-src 'self' blob:; frame-src https:; object-src 'none'; base-uri 'self'; form-action 'self'; frame-ancestors 'none'"
		return policy
	}
	if cfg.BrowserPreviewPort > 0 {
		port := strconv.Itoa(cfg.BrowserPreviewPort)
		policy = strings.Replace(policy, "ws://localhost:*;", "ws://localhost:* http://*.localhost:"+port+" ws://*.localhost:"+port+";", 1)
		policy = strings.Replace(policy, "https:; object-src", "http://*.localhost:"+port+" https:; object-src", 1)
	}
	return policy
}

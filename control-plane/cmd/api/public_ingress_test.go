package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicIngressConfigurationAndWebBoundary(t *testing.T) {
	base := map[string]string{"LUCI_STATE_DIR": t.TempDir(), "LUCI_PUBLIC_APP_ORIGIN": "https://app.example.test", "LUCI_PREVIEW_BASE_DOMAIN": "preview.projects.test", "LUCI_BROWSER_PREVIEW_ADDR": "127.0.0.1:8731", "LUCI_BROWSER_PREVIEW_PORT": "8731"}
	cfg, err := loadConfig(env(base))
	if err != nil {
		t.Fatal(err)
	}
	for key, value := range map[string]string{"LUCI_PUBLIC_APP_ORIGIN": "http://app.example.test", "LUCI_PREVIEW_BASE_DOMAIN": "app.example.test"} {
		m := map[string]string{}
		for k, v := range base {
			m[k] = v
		}
		m[key] = value
		if _, err := loadConfig(env(m)); err == nil {
			t.Fatal("invalid ingress accepted", key)
		}
	}
	cfg.WebDir = t.TempDir()
	if err := os.WriteFile(filepath.Join(cfg.WebDir, "index.html"), []byte("Luci web"), 0600); err != nil {
		t.Fatal(err)
	}
	h := withWeb(http.NotFoundHandler(), cfg)
	for _, tc := range []struct {
		host   string
		status int
	}{{"app.example.test", 200}, {"127.0.0.1:8730", 200}, {"p-any.preview.example.test", 403}, {"attacker.test", 403}} {
		r := httptest.NewRequest("GET", "http://"+tc.host+"/", nil)
		r.Header.Set("X-Forwarded-Host", "app.example.test")
		w := httptest.NewRecorder()
		h.ServeHTTP(w, r)
		if w.Code != tc.status {
			t.Fatal(tc, w.Code)
		}
		if w.Code == 200 && (strings.Contains(w.Header().Get("Content-Security-Policy"), "127.0.0.1") || !strings.Contains(w.Header().Get("Content-Security-Policy"), "frame-ancestors 'none'") || !strings.Contains(w.Header().Get("Content-Security-Policy"), "wss://app.example.test")) {
			t.Fatal("public UI retained local connection authority")
		}
	}
}

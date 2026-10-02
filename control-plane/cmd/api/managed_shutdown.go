package main

import (
	"crypto/subtle"
	"encoding/json"
	"net"
	"net/http"
)

// A native launcher owns its service with a private lifecycle token held only in memory.
// The application's signed-in management API remains a separate authorization surface.
func withManagedShutdown(next http.Handler, token string, shutdown func()) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/control-plane/shutdown" {
			r.Header.Del("X-Luci-Lifecycle-Token")
			next.ServeHTTP(w, r)
			return
		}
		host, _, err := net.SplitHostPort(r.RemoteAddr)
		ip := net.ParseIP(host)
		if r.Method != "POST" || err != nil || ip == nil || !ip.IsLoopback() || len(token) < 32 || subtle.ConstantTimeCompare([]byte(r.Header.Get("X-Luci-Lifecycle-Token")), []byte(token)) != 1 {
			http.Error(w, "owned service authorization required", http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "stopping"})
		go shutdown()
	})
}

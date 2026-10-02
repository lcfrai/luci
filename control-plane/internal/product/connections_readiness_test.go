package product

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestTunnelServiceReadyAcceptsExistingLuciWithoutCoolify(t *testing.T) {
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/control-plane/status" || r.Header.Get("Authorization") != "" {
			t.Errorf("unexpected readiness request: %s", r.URL.Path)
		}
		fmt.Fprintf(w, `{"product":"Luci","version":%q,"bootstrapRequired":false}`, Version)
	}))
	defer api.Close()
	if message := tunnelServiceReady(api.URL, "http://127.0.0.1:1"); !strings.Contains(message, "Connected to Luci") {
		t.Fatalf("existing authenticated workspace needs no Coolify service: %q", message)
	}
}

func TestTunnelServiceReadyRejectsWrongAPIIdentityAndRedirects(t *testing.T) {
	for _, body := range []string{`{}`, `{"product":"Other","version":"` + Version + `"}`, `{"product":"Luci","version":"old"}`, `{"product":"Luci","version":false}`, strings.Repeat("x", (1<<20)+1)} {
		t.Run(fmt.Sprintf("body-%d-%s", len(body), body[:min(len(body), 24)]), func(t *testing.T) {
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer api.Close()
			if got := tunnelServiceReady(api.URL, "http://127.0.0.1:1"); got != "" {
				t.Fatalf("invalid API marked ready: %q", got)
			}
		})
	}
	redirected := false
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/redirected" {
			redirected = true
			fmt.Fprintf(w, `{"product":"Luci","version":%q}`, Version)
			return
		}
		http.Redirect(w, r, "/redirected", http.StatusFound)
	}))
	defer api.Close()
	if got := tunnelServiceReady(api.URL, "http://127.0.0.1:1"); got != "" || redirected {
		t.Fatalf("API redirect accepted or followed: %q / %v", got, redirected)
	}
}

func TestTunnelServiceReadyRetainsManagementOnlyServers(t *testing.T) {
	management := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(http.StatusOK) }))
	defer management.Close()
	if got := tunnelServiceReady("http://127.0.0.1:1", management.URL); !strings.Contains(got, "API token") {
		t.Fatalf("management setup no longer works before Luci installation: %q", got)
	}
}

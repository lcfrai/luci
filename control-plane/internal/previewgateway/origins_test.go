package previewgateway

import (
	"context"
	"encoding/hex"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPublicBrowserOriginRoundTripsFullCapabilityAndRejectsAmbiguity(t *testing.T) {
	domain := "preview.example.test"
	for last := 0; last < 256; last++ {
		b := make([]byte, 32)
		b[0] = 128
		b[31] = byte(last)
		key := hex.EncodeToString(b)
		host := PublicCapabilityHostname(key, domain)
		if len(strings.Split(host, ".")[0]) != 54 || publicRequestKey(host, domain) != key {
			t.Fatal("capability lost entropy", last, host)
		}
		for _, bad := range []string{host + ":443", host + ".", strings.ToUpper(host), "extra." + host, host + ".attacker.test", strings.Replace(host, domain, "elsewhere.test", 1)} {
			if publicRequestKey(bad, domain) != "" {
				t.Fatal("accepted alternate host", bad)
			}
		}
	}
	for _, origin := range []string{"http://app.example.test", "https://app.example.test/", "https://APP.example.test", "https://user@app.example.test", "https://127.0.0.1", "https://app.localhost", "https://app.example.test?", "https://app.example.test#bad", "https://app.example.test:0", "https://app.example.test:443", "https://app.example.test:0443", "https://app.example.test:443", "https://bad_.example.test"} {
		if ValidatePublicAppOrigin(origin) == nil {
			t.Fatal("accepted untrusted app origin", origin)
		}
	}
	if ValidatePublicAppOrigin("https://app.example.test") != nil {
		t.Fatal("valid app rejected")
	}
}

func TestPublicGatewayProjectsOnlyBoundTeamAndReprojectsExistingRegistration(t *testing.T) {
	root := t.TempDir()
	domain := "preview.example.test"
	team := "team-example"
	seen := 0
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen++
		w.Header().Set("Location", "http://"+r.Host+"/next")
		w.Header().Add("Set-Cookie", "app=ok; Domain=.example.test; Path=/")
		w.WriteHeader(302)
	}))
	defer upstream.Close()
	state := filepath.Join(root, "control-plane", "teams", team, "runtime", "state", "sessions")
	if err := os.MkdirAll(state, 0700); err != nil {
		t.Fatal(err)
	}
	local, err := NewRegistry(state, 8731)
	if err != nil {
		t.Fatal(err)
	}
	previous, err := local.Publish(appTarget(upstream.URL))
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(local.filename(appTarget(upstream.URL)))
	if err != nil {
		t.Fatal(err)
	}
	public, err := NewRegistryWithDomain(state, 8731, domain)
	if err != nil {
		t.Fatal(err)
	}
	origin, err := public.Publish(appTarget(upstream.URL))
	if err != nil {
		t.Fatal(err)
	}
	after, _ := os.ReadFile(local.filename(appTarget(upstream.URL)))
	if string(before) != string(after) {
		t.Fatal("reprojection changed capability or receipt")
	}
	if origin == previous || !public.AcceptsOrigin(origin) || public.AcceptsOrigin(previous) {
		t.Fatal("origin policy not current")
	}
	gateway, err := NewWithPublicIngress(root, 8731, func(context.Context, Target) error { return nil }, domain, team)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := NewWithPublicIngress(root, 8731, func(context.Context, Target) error { return nil }, domain, ""); err == nil {
		t.Fatal("unbound ingress")
	}
	if _, err := gateway.ResolveTeamTarget(team, "project-1", origin); err != nil {
		t.Fatal(err)
	}
	if _, err := gateway.ResolveTeamTarget(team, "project-1", previous); err == nil {
		t.Fatal("stale localhost origin accepted")
	}
	if w := request(gateway, "GET", origin, "/", nil); w.Code != 403 || seen != 0 {
		t.Fatal("public registry bypassed management grant", w.Code)
	}
	calls := 0
	gateway.SetTeamAccessHandler(func(w http.ResponseWriter, r *http.Request, target TeamBrowserTarget, next http.Handler) {
		calls++
		if target.TeamID != team || target.Origin != origin {
			t.Fatal("wrong scope")
		}
		next.ServeHTTP(w, r)
	})
	w := request(gateway, "GET", origin, "/", map[string]string{"Origin": origin, "Forwarded": "host=attacker.test;proto=http"})
	if w.Code != 302 || calls != 1 || seen != 1 || w.Header().Get("Location") != origin+"/next" || strings.Contains(w.Header().Get("Set-Cookie"), "Domain=") {
		t.Fatal("public proxy boundary", w.Code, w.Header())
	}
	for _, dir := range []string{filepath.Join(root, "sessions"), filepath.Join(root, "control-plane", "teams", "team-other", "runtime", "state", "sessions")} {
		if err := os.MkdirAll(dir, 0700); err != nil {
			t.Fatal(err)
		}
		r, err := NewRegistryWithDomain(dir, 8731, domain)
		if err != nil {
			t.Fatal(err)
		}
		address, err := r.Publish(appTarget(upstream.URL))
		if err != nil {
			t.Fatal(err)
		}
		if w := request(gateway, "GET", address, "/", nil); w.Code != 404 {
			t.Fatal("foreign registration exposed", w.Code)
		}
	}
	u, _ := url.Parse(origin)
	r := httptest.NewRequest("GET", "http://attacker.test/", nil)
	r.Header.Set("X-Forwarded-Host", u.Host)
	w = httptest.NewRecorder()
	gateway.ServeHTTP(w, r)
	if w.Code != 404 {
		t.Fatal("forwarded host granted public origin")
	}
}

func TestPublicIngressRequiresSeparateRegistrableSites(t *testing.T) {
	for _, domain := range []string{"preview.luci.test", "luci.test", "previews.app.luci.test"} {
		if ValidatePublicIngress("https://app.luci.test", domain) == nil {
			t.Fatal("same browser partition admitted", domain)
		}
	}
	if err := ValidatePublicIngress("https://app.luci.test", "preview.projects.test"); err != nil {
		t.Fatal(err)
	}
}

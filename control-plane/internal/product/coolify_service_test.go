package product

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in real interpreter/proxy evaluation. It uses only labelled disposable
// containers, an owned network, public fixtures and read-only config mounts.
// This proves the ingress policy, not real provider DNS/ACME or Supabase RLS.
func TestDataIngressActualEnvoyClientBoundary(t *testing.T) {
	if os.Getenv("LUCI_DATA_INGRESS_DOCKER_EVAL") != "1" {
		t.Skip("set LUCI_DATA_INGRESS_DOCKER_EVAL=1 for the pinned Envoy evaluation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 150*time.Second)
	defer cancel()
	docker := func(args ...string) string {
		t.Helper()
		b, e := exec.CommandContext(ctx, "docker", args...).CombinedOutput()
		if e != nil {
			t.Fatalf("owned Docker evaluation %v: %v: %s", args, e, b)
		}
		return strings.TrimSpace(string(b))
	}
	if _, e := exec.LookPath("docker"); e != nil {
		t.Fatal("Docker is required for the requested evaluation")
	}
	if _, e := exec.CommandContext(ctx, "docker", "image", "inspect", dataIngressImage).Output(); e != nil {
		docker("pull", dataIngressImage)
	}
	id := newID("luci-data-ingress-eval-")
	network, upstream, ingress := id+"-net", id+"-up", id+"-client"
	label := "ai.lcfr.luci.eval=" + id
	docker("network", "create", "--label", label, network)
	t.Cleanup(func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 25*time.Second)
		defer cancel()
		for _, name := range []string{ingress, upstream} {
			b, e := exec.CommandContext(cleanupCtx, "docker", "inspect", name, "--format", `{{index .Config.Labels "ai.lcfr.luci.eval"}}`).Output()
			if e == nil && strings.TrimSpace(string(b)) == id {
				if b, e := exec.CommandContext(cleanupCtx, "docker", "rm", "-f", name).CombinedOutput(); e != nil {
					t.Errorf("owned container cleanup: %v %s", e, b)
				}
			}
		}
		b, e := exec.CommandContext(cleanupCtx, "docker", "network", "inspect", network, "--format", `{{index .Labels "ai.lcfr.luci.eval"}}`).Output()
		if e == nil && strings.TrimSpace(string(b)) == id {
			if b, e := exec.CommandContext(cleanupCtx, "docker", "network", "rm", network).CombinedOutput(); e != nil {
				t.Errorf("owned network cleanup: %v %s", e, b)
			}
		}
	})
	key := dataDomainAnon("anon")
	v := dataDomainRecord{PublicDataDomainBinding: PublicDataDomainBinding{ProjectID: "eval-project", StableTenant: "eval-tenant", ApplicationNetwork: network, Domain: "https://api.example.com", AppOrigin: "https://app.example.com", ProbePath: "/__luci/data-health", ConfigurationDigest: strings.Repeat("c", 64)}, Owner: id}
	raw, e := dataIngressCompose(v, key)
	if e != nil {
		t.Fatal(e)
	}
	var compose struct {
		Configs map[string]struct {
			Content string `json:"content"`
		} `json:"configs"`
	}
	if e = json.Unmarshal([]byte(raw), &compose); e != nil {
		t.Fatal(e)
	}
	dir := t.TempDir()
	clientFile := filepath.Join(dir, "client.json")
	upstreamFile := filepath.Join(dir, "upstream.json")
	// Fixture-only diagnostics never run in the deployed client gateway.
	var evalConfig map[string]any
	_ = json.Unmarshal([]byte(compose.Configs["luci-data-client"].Content), &evalConfig)
	hcm := evalConfig["static_resources"].(map[string]any)["listeners"].([]any)[0].(map[string]any)["filter_chains"].([]any)[0].(map[string]any)["filters"].([]any)[0].(map[string]any)["typed_config"].(map[string]any)
	hcm["access_log"] = []any{map[string]any{"name": "envoy.access_loggers.stdout", "typed_config": map[string]any{"@type": "type.googleapis.com/envoy.extensions.access_loggers.stream.v3.StdoutAccessLog", "log_format": map[string]any{"text_format_source": map[string]any{"inline_string": "%RESPONSE_CODE% %RESPONSE_CODE_DETAILS% %RESPONSE_FLAGS%\n"}}}}}
	evalBytes, _ := json.Marshal(evalConfig)
	if e = os.WriteFile(clientFile, evalBytes, 0644); e != nil {
		t.Fatal(e)
	}
	backend := `{"static_resources":{"listeners":[{"name":"fixture","address":{"socket_address":{"address":"0.0.0.0","port_value":8000}},"filter_chains":[{"filters":[{"name":"envoy.filters.network.http_connection_manager","typed_config":{"@type":"type.googleapis.com/envoy.extensions.filters.network.http_connection_manager.v3.HttpConnectionManager","stat_prefix":"fixture","route_config":{"name":"fixture","virtual_hosts":[{"name":"fixture","domains":["*"],"routes":[{"match":{"prefix":"/"},"direct_response":{"status":200,"body":{"inline_string":"owned upstream fixture"}}}]}]},"http_filters":[{"name":"envoy.filters.http.router","typed_config":{"@type":"type.googleapis.com/envoy.extensions.filters.http.router.v3.Router"}}]}}]}]}]}}`
	var backendConfig map[string]any
	_ = json.Unmarshal([]byte(backend), &backendConfig)
	backendHCM := backendConfig["static_resources"].(map[string]any)["listeners"].([]any)[0].(map[string]any)["filter_chains"].([]any)[0].(map[string]any)["filters"].([]any)[0].(map[string]any)["typed_config"].(map[string]any)
	backendHCM["access_log"] = hcm["access_log"]
	backendBytes, _ := json.Marshal(backendConfig)
	if e = os.WriteFile(upstreamFile, backendBytes, 0644); e != nil {
		t.Fatal(e)
	}
	docker("run", "--rm", "--network", "none", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--user", "101:101", "--mount", "type=bind,source="+clientFile+",target=/etc/luci.json,readonly", "--entrypoint", "envoy", dataIngressImage, "--mode", "validate", "-c", "/etc/luci.json")
	start := func(name, file string, up bool) {
		args := []string{"run", "-d", "--name", name, "--label", label, "--network", network, "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--user", "101:101", "--memory", "128m", "--pids-limit", "256", "--mount", "type=bind,source=" + file + ",target=/etc/luci.json,readonly", "--entrypoint", "envoy"}
		if up {
			args = append(args, "--network-alias", "supabase-api")
		} else {
			args = append(args, "--publish", "127.0.0.1::8080")
		}
		args = append(args, dataIngressImage, "-c", "/etc/luci.json", "--disable-hot-restart", "--concurrency", "1")
		docker(args...)
	}
	start(upstream, upstreamFile, true)
	start(ingress, clientFile, false)
	address := docker("port", ingress, "8080/tcp")
	if !strings.HasPrefix(address, "127.0.0.1:") || strings.ContainsAny(address, "\r\n") {
		t.Fatal("unexpected owned listener", address)
	}
	client := &http.Client{Timeout: 5 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	base := "http://" + address
	ready := false
	for i := 0; i < 40; i++ {
		r, e := client.Get(base + v.ProbePath)
		if e == nil {
			io.Copy(io.Discard, r.Body)
			r.Body.Close()
			if r.StatusCode == 200 {
				ready = true
				break
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	if !ready {
		t.Fatal("actual Envoy did not start")
	}
	jwtPayload := func(payload string) string {
		return base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256"}`)) + "." + base64.RawURLEncoding.EncodeToString([]byte(payload)) + "." + base64.RawURLEncoding.EncodeToString([]byte("fixture-signature"))
	}
	cases := []struct {
		name, path, method, origin, token, apiKey string
		want                                      int
	}{
		{"health", v.ProbePath, "GET", "", "", "", 200},
		{"rest", "/rest/v1/notes", "GET", v.AppOrigin, "Bearer " + dataDomainAnon("authenticated"), key, 200},
		{"nested-role", "/rest/v1/notes", "GET", v.AppOrigin, "Bearer " + jwtPayload(`{"role":"authenticated","user_metadata":{"role":"owner"}}`), key, 200},
		{"storage-name", "/storage/v1/object/public/files/my%20file.txt", "GET", v.AppOrigin, "", key, 200},
		{"preflight", "/rest/v1/notes", "OPTIONS", v.AppOrigin, "", "", 200},
		{"studio", "/", "GET", "", "", "", 404},
		{"metadata", "/pg/", "GET", "", "", "", 404},
		{"auth-admin", "/auth/v1/admin/users", "GET", v.AppOrigin, "", key, 404},
		{"encoded-admin", "/auth/v1/%61dmin/users", "GET", v.AppOrigin, "", key, 404},
		{"double-encoded-admin", "/auth/v1/%2561dmin/users", "GET", v.AppOrigin, "", key, 404},
		{"realtime-admin", "/realtime/v1/api/tenants", "GET", v.AppOrigin, "", key, 404},
		{"opaque-control-token", "/rest/v1/notes", "GET", v.AppOrigin, "Bearer luci-control-private-token", "", 401},
		{"service-role", "/rest/v1/notes", "GET", v.AppOrigin, "Bearer " + dataDomainAnon("service_role"), key, 401},
		{"duplicate-role", "/rest/v1/notes", "GET", v.AppOrigin, "Bearer " + jwtPayload(`{"role":"anon","role":"service_role"}`), key, 401},
		{"escaped-service-role", "/rest/v1/notes", "GET", v.AppOrigin, "Bearer " + jwtPayload(`{"role":"service\u005frole"}`), key, 401},
		{"foreign-key", "/rest/v1/notes", "GET", v.AppOrigin, "", "foreign-project", 401},
		{"query-service-role", "/realtime/v1/websocket?access_token=" + dataDomainAnon("service_role"), "GET", v.AppOrigin, "", key, 401},
		{"foreign-origin", "/rest/v1/notes", "POST", "https://foreign.example.com", "", key, 403},
		{"api-origin", "/auth/v1/health", "GET", v.Domain, "", key, 200},
		{"encoded-probe-bypass", "/__luci/%64ata-health", "GET", "", "", "", 200},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			req, e := http.NewRequest(c.method, base+c.path, nil)
			if e != nil {
				t.Fatal(e)
			}
			if c.origin != "" {
				req.Header.Set("Origin", c.origin)
			}
			if c.token != "" {
				req.Header.Set("Authorization", c.token)
			}
			if c.apiKey != "" {
				req.Header.Set("apikey", c.apiKey)
			}
			if c.method == "OPTIONS" {
				req.Header.Set("Access-Control-Request-Method", "POST")
				req.Header.Set("Access-Control-Request-Headers", "authorization,apikey")
			}
			res, e := client.Do(req)
			if e != nil {
				t.Fatal(e)
			}
			b, _ := io.ReadAll(res.Body)
			res.Body.Close()
			if res.StatusCode != c.want {
				time.Sleep(500 * time.Millisecond)
				logs, _ := exec.CommandContext(ctx, "docker", "logs", "--tail", "5", ingress).CombinedOutput()
				t.Logf("fixture-only ingress diagnostics: %s", logs)
				logs, _ = exec.CommandContext(ctx, "docker", "logs", "--tail", "5", upstream).CombinedOutput()
				t.Logf("fixture-only upstream diagnostics: %s; headers: %v", logs, res.Header)
				t.Fatalf("actual policy returned %d want %d: %s", res.StatusCode, c.want, b)
			}
			if c.want == 200 && c.origin != "" && res.Header.Get("Access-Control-Allow-Origin") != c.origin {
				t.Fatal("approved exact origin lost", res.Header)
			}
			if (c.name == "health" || c.name == "encoded-probe-bypass") && string(b) != v.ConfigurationDigest {
				t.Fatal("running configuration receipt differs")
			}
			if c.name == "foreign-origin" && res.Header.Get("Access-Control-Allow-Origin") != "" {
				t.Fatal("foreign origin received CORS permission")
			}
		})
	}
	if t.Failed() {
		time.Sleep(2 * time.Second)
		for _, name := range []string{ingress, upstream} {
			logs, _ := exec.CommandContext(ctx, "docker", "logs", "--tail", "60", name).CombinedOutput()
			t.Logf("final fixture-only diagnostics %s: %s", name, logs)
		}
	}
}

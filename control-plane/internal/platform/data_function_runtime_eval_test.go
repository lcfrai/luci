package platform

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// Opt-in actual pinned-runtime evaluation. This never reads an operator's
// credentials, touches installed stacks, or publishes beyond literal loopback.
func TestDataFunctionPinnedRuntimeEvaluation(t *testing.T) {
	if os.Getenv("LUCI_FUNCTION_RUNTIME_EVAL") != "1" {
		t.Skip("explicit isolated Docker runtime evaluation")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	run := func(args ...string) []byte {
		t.Helper()
		cmd := exec.CommandContext(ctx, "docker", args...)
		out, e := cmd.CombinedOutput()
		if e != nil {
			t.Fatalf("isolated Docker %s failed (%v); raw output withheld", args[0], e)
		}
		return out
	}
	m, e := reviewedDataAssets()
	if e != nil {
		t.Fatal(e)
	}
	image := m.Images["supabase/edge-runtime:v1.76.2"]
	name := randomID("luci-functions-eval-")
	label := "ai.lcfr.luci.functions.eval=" + name
	started := time.Now().UTC()
	complete := false
	receipt := os.Getenv("LUCI_FUNCTION_RUNTIME_EVAL_RECEIPT")
	if receipt != "" && !filepath.IsAbs(receipt) {
		t.Fatal("evaluation receipt path must be absolute")
	}
	root := t.TempDir()
	if strings.Contains(root, ",") {
		t.Fatal("fixture path cannot contain commas")
	}
	put := func(relative, text string) {
		t.Helper()
		target := filepath.Join(root, filepath.FromSlash(relative))
		if e = os.MkdirAll(filepath.Dir(target), 0755); e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(target, []byte(text), 0644); e != nil {
			t.Fatal(e)
		}
		if e = os.Chmod(target, 0644); e != nil {
			t.Fatal(e)
		}
	}
	e = fs.WalkDir(dataAssets, "supabase_assets/volumes/functions/_vendor", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			return nil
		}
		b, e := dataAssets.ReadFile(name)
		if e != nil {
			return e
		}
		put(strings.TrimPrefix(name, "supabase_assets/volumes/functions/"), string(b))
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	fixtureMain := strings.Replace(dataFunctionMain(), "catch { return Response.json({error:'function_boot_failed'},{status:503}); }", "catch(e) { console.error('luci-fixture-compile', String(e)); return Response.json({error:'function_boot_failed'},{status:503}); }", 1)
	put("main/index.ts", fixtureMain)
	put("hello/index.ts", dataFunctionHello)
	put("deno.jsonc", `{"imports":{}}`)
	first, second, bad := randomID("function-release-"), randomID("function-release-"), randomID("function-release-")
	source := `import {version} from './value.ts';
let calls=0;
export default { fetch: async (req:Request) => {
 const blocked:Record<string,boolean>={};
 for(const key of ['JWT_SECRET','SUPABASE_SERVICE_ROLE_KEY','SUPABASE_DB_URL','LUCI_FUNCTION_PROBE_TOKEN']) {try{blocked[key]=!Deno.env.get(key)}catch{blocked[key]=true}}
 try {await Deno.readTextFile('/etc/passwd');blocked.read=false}catch{blocked.read=true}
 try {await Deno.writeTextFile('/tmp/luci-fixture-denied','x');blocked.write=false}catch{blocked.write=true}
 try {await new Deno.Command('echo',{args:['x']}).output();blocked.run=false}catch{blocked.run=true}
 try {await fetch('http://127.0.0.1:9000');blocked.network=false}catch{blocked.network=true}
 return Response.json({version,calls:++calls,blocked,payload:await req.json()});
}};`
	for _, id := range []string{first, second} {
		put("_luci/releases/"+id+"/index.ts", source)
	}
	put("_luci/releases/"+first+"/value.ts", `export const version=1;`)
	put("_luci/releases/"+second+"/value.ts", `export const version=2;`)
	put("_luci/releases/"+bad+"/index.ts", `export default { fetch: ( => broken`)
	routeA := functionRoute{Name: "example", ReleaseID: first, SourceDigest: strings.Repeat("a", 64), AuthMode: "project-key"}
	routeB := functionRoute{Name: "example", ReleaseID: second, SourceDigest: strings.Repeat("b", 64), AuthMode: "authenticated"}
	badRoute := functionRoute{Name: "example", ReleaseID: bad, SourceDigest: strings.Repeat("c", 64), AuthMode: "authenticated"}
	manifest := functionManifest{Version: 1, Routes: map[string]functionRoute{"example": routeA}, Candidates: map[string]functionRoute{first: routeA, second: routeB, bad: badRoute}}
	writeManifest := func() {
		t.Helper()
		b, _ := json.Marshal(manifest)
		if e = writeFunctionPublicFile(root, filepath.Join(root, "_luci", "routes.json"), b); e != nil {
			t.Fatal(e)
		}
	}
	put("_luci/routes.json", "{}")
	writeManifest()
	jwtSecret := "isolated-eval-signing-secret-at-least-32-bytes"
	probe := randomID("fixture-nonce-")
	token := func(role string, expired bool) string {
		h := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
		expiry := time.Now().Add(time.Hour)
		if expired {
			expiry = time.Now().Add(-time.Hour)
		}
		claims, _ := json.Marshal(map[string]any{"role": role, "sub": "fixture-user", "exp": expiry.Unix()})
		p := h + "." + base64.RawURLEncoding.EncodeToString(claims)
		mac := hmac.New(sha256.New, []byte(jwtSecret))
		mac.Write([]byte(p))
		return p + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
	}
	// Pull only the existing approved immutable runtime, if absent.
	check := exec.CommandContext(ctx, "docker", "image", "inspect", image)
	if check.Run() != nil {
		run("pull", image)
	}
	run("run", "-d", "--name", name, "--label", label, "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--memory", "512m", "--cpus", "1", "--pids-limit", "256", "--mount", "type=bind,src="+root+",dst=/home/deno/functions,readonly", "-p", "127.0.0.1::9000", "-e", "JWT_SECRET="+jwtSecret, "-e", "SUPABASE_SERVICE_ROLE_KEY=fixture-private-service-role", "-e", "SUPABASE_DB_URL=fixture-private-db-url", "-e", "LUCI_FUNCTION_PROBE_TOKEN="+probe, "-e", "SUPABASE_ANON_KEY="+token("anon", false), "-e", "SUPABASE_URL=http://api-gw:8000", image, "start", "--main-service", "/home/deno/functions/main")
	t.Cleanup(func() {
		clean, c := context.WithTimeout(context.Background(), 20*time.Second)
		defer c()
		cmd := exec.CommandContext(clean, "docker", "inspect", name, "--format", "{{index .Config.Labels \"ai.lcfr.luci.functions.eval\"}}")
		b, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(b)) == name {
			err = exec.CommandContext(clean, "docker", "rm", "-f", name).Run()
			if err != nil {
				t.Error("owned evaluation container cleanup failed")
			}
			if complete && err == nil && receipt != "" {
				b, _ = json.MarshalIndent(map[string]any{
					"status": "passed", "startedAt": started.Format(time.RFC3339), "completedAt": time.Now().UTC().Format(time.RFC3339),
					"image": image, "dispatcherSHA256": dataSum([]byte(dataFunctionMain())), "container": name, "ownedContainerRemoved": true,
					"checks":      []string{"compile-only probe", "local module import", "eight private environment and operation denials", "native JWT verification", "syntax failure preserves route", "immutable promotion", "rollback", "restart", "remove retains source", "built-in hello"},
					"limitations": []string{"isolated runtime evaluation, not installed whole-app UAT", "does not prove native Supabase RLS or public-domain ingress"},
				}, "", "  ")
				if err = os.WriteFile(receipt, b, 0600); err != nil {
					t.Error("evaluation receipt could not be saved")
				}
			}
		}
	})
	var containers []struct {
		Config          struct{ Image string }
		NetworkSettings struct {
			Ports map[string][]struct{ HostIP, HostPort string }
		}
	}
	if json.Unmarshal(run("inspect", name), &containers) != nil || len(containers) != 1 || containers[0].Config.Image != image {
		t.Fatal("runtime receipt unavailable")
	}
	ports := containers[0].NetworkSettings.Ports["9000/tcp"]
	if len(ports) != 1 || ports[0].HostIP != "127.0.0.1" {
		t.Fatal("fixture must stay loopback")
	}
	address := "http://127.0.0.1:" + ports[0].HostPort
	client := &http.Client{Timeout: 10 * time.Second}
	call := func(path, auth string, input any, withProbe bool) (int, map[string]any) {
		t.Helper()
		b, _ := json.Marshal(input)
		req, _ := http.NewRequestWithContext(ctx, "POST", address+path, strings.NewReader(string(b)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+auth)
		if withProbe {
			req.Header.Set("X-Luci-Function-Probe", probe)
		}
		res, err := client.Do(req)
		if err != nil {
			return 0, nil
		}
		defer res.Body.Close()
		out, _ := io.ReadAll(io.LimitReader(res.Body, 8192))
		var v map[string]any
		_ = json.Unmarshal(out, &v)
		return res.StatusCode, v
	}
	wait := func(stage string) {
		t.Helper()
		deadline := time.Now().Add(35 * time.Second)
		for time.Now().Before(deadline) {
			if code, v := call("/_luci-health", "", map[string]any{}, true); code == 200 && v["runtimeVersion"] == dataFunctionRuntimeVersion {
				return
			}
			time.Sleep(300 * time.Millisecond)
		}
		for _, line := range strings.Split(string(run("logs", "--tail", "12", name)), "\n") {
			if strings.Contains(line, "error") || strings.Contains(line, "Error") || strings.Contains(line, "failed") {
				t.Log(strings.ReplaceAll(strings.ReplaceAll(line, jwtSecret, "[private]"), probe, "[private]"))
			}
		}
		t.Fatal("actual pinned runtime did not become ready", stage)
	}
	wait("initial boot")
	if code, _ := call("/_luci-probe", "", map[string]string{"releaseId": first, "sourceDigest": routeA.SourceDigest}, false); code != 404 {
		t.Fatal("untrusted compilation probe admitted", code)
	}
	if code, _ := call("/_luci-probe", "", map[string]string{"releaseId": first, "sourceDigest": routeA.SourceDigest}, true); code != 200 {
		logs := run("logs", "--tail", "15", name)
		for _, line := range strings.Split(string(logs), "\n") {
			if strings.Contains(line, "luci-fixture-compile") {
				t.Log(strings.ReplaceAll(strings.ReplaceAll(line, jwtSecret, "[private]"), probe, "[private]"))
			}
		}
		t.Fatal("actual local-module compilation failed", code)
	}
	if code, v := call("/example", token("anon", false), map[string]string{"message": "real invocation"}, false); code != 200 || v["version"] != float64(1) || v["calls"] != float64(1) {
		t.Fatal("probe invoked customer fetch or committed handler failed", code)
	} else {
		blocked, ok := v["blocked"].(map[string]any)
		if !ok || len(blocked) != 8 {
			t.Fatal("permission proof incomplete")
		}
		for key, value := range blocked {
			if value != true {
				t.Fatal("private environment or denied operation exposed", key)
			}
		}
	}
	for _, auth := range []string{"luci-control-token", token("service_role", false), token("authenticated", true), token("authenticated", false) + "broken"} {
		if code, _ := call("/example", auth, map[string]any{}, false); code != 401 {
			t.Fatal("unapproved caller executed custom function", code)
		}
	}
	if code, _ := call("/_luci-probe", "", map[string]string{"releaseId": bad, "sourceDigest": badRoute.SourceDigest}, true); code != 503 {
		t.Fatal("invalid syntax accepted", code)
	}
	if code, v := call("/example", token("anon", false), map[string]any{}, false); code != 200 || v["version"] != float64(1) {
		t.Fatal("failed candidate disturbed prior route", code)
	}
	if code, _ := call("/_luci-probe", "", map[string]string{"releaseId": second, "sourceDigest": routeB.SourceDigest}, true); code != 200 {
		t.Fatal("second compilation failed", code)
	}
	manifest.Routes["example"] = routeB
	writeManifest()
	if code, _ := call("/example", token("anon", false), map[string]any{}, false); code != 401 {
		t.Fatal("authenticated policy admitted anon", code)
	}
	if code, v := call("/example", token("authenticated", false), map[string]any{}, false); code != 200 || v["version"] != float64(2) || v["calls"] != float64(1) {
		t.Fatal("promotion reused stale worker or probe ran fetch", code)
	}
	manifest.Routes["example"] = routeA
	writeManifest()
	if code, v := call("/example", token("anon", false), map[string]any{}, false); code != 200 || v["version"] != float64(1) {
		t.Fatal("rollback failed", code)
	}
	run("restart", name)
	// Docker may allocate another random host port after restart. Production
	// stacks retain their explicit API port; this isolated fixture uses no port
	// reservation and must read back its own current mapping.
	if json.Unmarshal(run("inspect", name), &containers) != nil || len(containers) != 1 {
		t.Fatal("restarted runtime receipt unavailable")
	}
	ports = containers[0].NetworkSettings.Ports["9000/tcp"]
	if len(ports) != 1 || ports[0].HostIP != "127.0.0.1" {
		t.Fatal("restarted fixture must stay loopback")
	}
	address = "http://127.0.0.1:" + ports[0].HostPort
	wait("retained route restart")
	if code, v := call("/example", token("anon", false), map[string]any{}, false); code != 200 || v["version"] != float64(1) {
		t.Fatal("restart lost immutable route", code)
	}
	delete(manifest.Routes, "example")
	writeManifest()
	if code, _ := call("/example", token("anon", false), map[string]any{}, false); code != 404 {
		t.Fatal("removed endpoint remained active", code)
	}
	if code, v := call("/hello", token("anon", false), map[string]any{}, false); code != 200 || v["message"] != "Hello from Luci Functions!" {
		t.Fatal("built-in hello compatibility failed", code)
	}
	complete = true
	t.Logf("Actual pinned runtime PASS: compilation without fetch, local modules, eight denial checks, caller verification, syntax failure, promotion, rollback, restart, removal and hello. Image %s. No native Supabase RLS/public-domain proof.", image)
}

package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/lcfrai/luci/control-plane/internal/assistance"
)

// Exercise the actual generated launcher and answer schema together. The
// loopback endpoints below are fixtures; no model or provider is contacted.
func TestOrchestratorNativeCompleteBranchesReachStubbedModel(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python runtime required for the embedded launcher")
	}
	dir := t.TempDir()
	input, output := filepath.Join(dir, "input"), filepath.Join(dir, "output")
	for _, path := range []string{input, output} {
		if err := os.Mkdir(path, 0700); err != nil {
			t.Fatal(err)
		}
	}
	schema, err := orchestratorResponseSchema(true)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"schema.json":    schema,
		"prompt.txt":     []byte("Create a complete Orchard task list proposal."),
		"selection.json": []byte(`{"harness":"opencode","model":"luci-local/fixture","authMode":"configured"}`),
	} {
		if err := os.WriteFile(filepath.Join(input, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	answer := `{"summary":"A saved task list.","question":"","options":[],"rationale":"The brief is complete.","blocking":false,"impact":[],"proposal":null,"nextAction":"review"}`
	var nativeCalls int
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/generate":
			json.NewEncoder(w).Encode(map[string]any{"model": "fixture", "prompt_eval_count": 250, "eval_count": 1, "done": true})
		case "/api/ps":
			json.NewEncoder(w).Encode(map[string]any{"models": []any{map[string]any{"name": "fixture", "context_length": 8192, "digest": "sha256:fixture"}}})
		case "/api/chat":
			var request struct {
				Format json.RawMessage `json:"format"`
				Model  string          `json:"model"`
			}
			if r.Method != "POST" || json.NewDecoder(http.MaxBytesReader(w, r.Body, 128<<10)).Decode(&request) != nil || request.Model != "fixture" || !jsonEqual(request.Format, schema) {
				http.Error(w, "invalid native fixture request", 400)
				return
			}
			nativeCalls++
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"model": "fixture", "done": true, "done_reason": "stop", "message": map[string]string{"role": "assistant", "content": answer}})
		default:
			http.NotFound(w, r)
		}
	}))
	defer model.Close()
	u, err := url.Parse(model.URL)
	if err != nil {
		t.Fatal(err)
	}
	modelConfig, _ := json.Marshal(map[string]any{"model": "luci-local/fixture", "provider": map[string]any{"luci-local": map[string]any{"options": map[string]string{"baseURL": model.URL + "/v1"}, "models": map[string]any{"fixture": map[string]any{"limit": map[string]int{"context": 8192, "output": 2048}}}}}})
	fake := filepath.Join(dir, "opencode.py")
	// This stand-in owns only the private OpenCode HTTP session. Its selected
	// model call goes through the launcher's bounded native gate above.
	fakeSource := `import base64,http.server,json,os,sys,urllib.request
class Handler(http.server.BaseHTTPRequestHandler):
 def log_message(self,*args): pass
 def do_POST(self):
  if base64.b64decode(self.headers.get('Authorization','')[6:]).decode()!='opencode:'+os.environ['OPENCODE_SERVER_PASSWORD']:
   self.send_error(401); return
  size=int(self.headers.get('Content-Length','0'))
  if size<1 or size>524288:
   self.send_error(413); return
  body=json.loads(self.rfile.read(size))
  if self.path=='/session': result={'id':'ses_fixture12345678'}
  elif self.path=='/session/ses_fixture12345678/message':
   if 'format' in body or body.get('model')!={'providerID':'luci-local','modelID':'fixture'}:
    self.send_error(400); return
   base=json.loads(os.environ['OPENCODE_CONFIG_CONTENT'])['provider']['luci-local']['options']['baseURL']
   request=urllib.request.Request(base+'/chat/completions',data=json.dumps({'model':'fixture','messages':[{'role':'user','content':'fixture'}]}).encode(),headers={'Content-Type':'application/json'},method='POST')
   with urllib.request.urlopen(request,timeout=5) as upstream:
    events=[json.loads(line[6:]) for line in upstream.read().decode().splitlines() if line.startswith('data: {')]
   content=''.join(event['choices'][0]['delta'].get('content','') for event in events)
   result={'info':{},'parts':[{'type':'text','text':content}]}
  else:
   self.send_error(404); return
  encoded=json.dumps(result).encode()
  self.send_response(200);self.send_header('Content-Type','application/json');self.send_header('Content-Length',str(len(encoded)));self.end_headers();self.wfile.write(encoded)
server=http.server.HTTPServer(('127.0.0.1',0),Handler)
print('opencode server listening on http://127.0.0.1:'+str(server.server_port),flush=True)
server.serve_forever()
`
	if err := os.WriteFile(fake, []byte(fakeSource), 0600); err != nil {
		t.Fatal(err)
	}
	launcher := strings.ReplaceAll(assistance.RepositoryLauncher, "/luci-input", filepath.ToSlash(input))
	launcher = strings.ReplaceAll(launcher, "/luci-output", filepath.ToSlash(output))
	launcher = strings.Replace(launcher, "base.port != 11434", "base.port != "+u.Port(), 1)
	quotedPython, _ := json.Marshal(filepath.ToSlash(python))
	quotedFake, _ := json.Marshal(filepath.ToSlash(fake))
	launcher = strings.ReplaceAll(launcher, "['opencode',", "["+string(quotedPython)+","+string(quotedFake)+",")
	entry := filepath.Join(dir, "run.py")
	if err := os.WriteFile(entry, []byte(launcher), 0600); err != nil {
		t.Fatal(err)
	}
	run := func(native bool) (int, string) {
		t.Helper()
		os.Remove(filepath.Join(output, "result.json"))
		cmd := exec.Command(python, entry)
		cmd.Env = append(os.Environ(), "OPENCODE_LUCI_OLLAMA_PREFLIGHT=1", "OPENCODE_CONFIG_CONTENT="+string(modelConfig), "OPENCODE_LUCI_INPUT_BYTES=14000")
		if native {
			cmd.Env = append(cmd.Env, "OPENCODE_LUCI_NATIVE_JSON=1")
		}
		logs, runErr := cmd.CombinedOutput()
		if len(logs) != 0 {
			t.Fatalf("launcher leaked fixture/provider output: %q", logs)
		}
		var receipt struct {
			ExitCode    int    `json:"exitCode"`
			FailureCode string `json:"failureCode"`
		}
		data, err := os.ReadFile(filepath.Join(output, "exit.json"))
		if err != nil || json.Unmarshal(data, &receipt) != nil {
			t.Fatal("missing bounded exit receipt", err)
		}
		if (runErr == nil) != (receipt.ExitCode == 0) {
			t.Fatal("launcher exit and receipt disagree", runErr, receipt)
		}
		return receipt.ExitCode, receipt.FailureCode
	}
	if code, cause := run(true); code != 0 || cause != "" || nativeCalls != 1 {
		t.Fatal("complete native branch failed to reach exact stubbed model", code, cause, nativeCalls)
	}
	data, err := os.ReadFile(filepath.Join(output, "result.json"))
	if err != nil || !jsonEqual(data, []byte(answer)) {
		t.Fatal("native result did not survive the full launcher", err)
	}
	if code, cause := run(false); code == 0 || cause != "local_model_response_schema_invalid" || nativeCalls != 1 {
		t.Fatal("nonnative route accepted a root union", code, cause, nativeCalls)
	}
	// A malformed native alternative must fail before any transport effect.
	var invalid map[string]any
	if json.Unmarshal(schema, &invalid) != nil {
		t.Fatal("invalid generated schema")
	}
	delete(invalid["anyOf"].([]any)[0].(map[string]any)["properties"].(map[string]any), "summary")
	broken, _ := json.Marshal(invalid)
	if err := os.WriteFile(filepath.Join(input, "schema.json"), broken, 0600); err != nil {
		t.Fatal(err)
	}
	if code, cause := run(true); code == 0 || cause != "local_model_response_schema_invalid" || nativeCalls != 1 {
		t.Fatal("incomplete native branch reached the provider", code, cause, nativeCalls)
	}
}

func jsonEqual(left, right []byte) bool {
	var a, b any
	return json.Unmarshal(left, &a) == nil && json.Unmarshal(right, &b) == nil && reflect.DeepEqual(a, b)
}

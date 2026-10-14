package assistance

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRepositoryProviderSelectionAndOpenCodeConfigurationAreExplicitReadOnly(t *testing.T) {
	for _, choice := range [][3]string{{"codex", "gpt-selected", "chatgpt"}, {"codex", "gpt-selected", "api"}, {"claude-code", "sonnet", "api"}, {"opencode", "luci-local/model", "configured"}} {
		if ValidateSelection(choice[0], choice[1], choice[2]) != nil {
			t.Fatal("valid explicit selection rejected")
		}
	}
	for _, choice := range [][3]string{{"codex", "default", "chatgpt"}, {"codex", "--flag", "api"}, {"codex", "model\nflag", "api"}, {"opencode", "model", "api"}, {"claude-code", "sonnet", "chatgpt"}} {
		if ValidateSelection(choice[0], choice[1], choice[2]) == nil {
			t.Fatal("implicit/foreign provider mode accepted")
		}
	}
	env := map[string]string{"OPENCODE_CONFIG_CONTENT": `{"model":"luci-local/model","provider":{"luci-local":{"options":{"baseURL":"http://localhost:11434/v1","apiKey":"{env:OPENCODE_LOCAL_API_KEY}"}}},"instructions":["LUCI.md"],"plugin":["untrusted"]}`, "OPENCODE_LOCAL_API_KEY": "selected-secret"}
	if ReadOnlyOpenCode(env, "other/model") == nil {
		t.Fatal("saved model mismatch accepted")
	}
	if err := ReadOnlyOpenCode(env, "luci-local/model"); err != nil {
		t.Fatal(err)
	}
	var config map[string]any
	json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config)
	if len(config["instructions"].([]any)) != 0 || len(config["plugin"].([]any)) != 0 || config["tools"].(map[string]any)["*"] != false || config["permission"].(map[string]any)["*"] != "deny" || env["OPENCODE_LOCAL_API_KEY"] != "selected-secret" {
		t.Fatal("read-only config changed provider or enabled tools")
	}
	model := config["provider"].(map[string]any)["luci-local"].(map[string]any)["models"].(map[string]any)["model"].(map[string]any)
	limit := model["limit"].(map[string]any)
	if limit["context"] != float64(8192) || limit["output"] != float64(2048) || env["OPENCODE_LUCI_INPUT_BYTES"] != "14000" || env["OPENCODE_LUCI_OLLAMA_PREFLIGHT"] != "1" {
		t.Fatal("local Ollama analysis did not reserve a bounded output and input window")
	}
	if !strings.Contains(env["OPENCODE_CONFIG_CONTENT"], "You are Luci's read-only analysis model") {
		t.Fatal("large upstream coding prompt was not replaced for read-only analysis")
	}
	for _, required := range []string{"'--model', model", "'--tools', ''", "'--pure'", "'--sandbox', 'read-only'", "stderr=subprocess.DEVNULL", "source.read(LIMIT+1)", "overflow[0]", "env.pop('OPENAI_API_KEY', None)"} {
		if !strings.Contains(RepositoryLauncher, required) {
			t.Fatalf("verified batch adapter boundary missing %s", required)
		}
	}
}

func TestRepositoryBatchAdaptersExecuteWithoutRawOutputAndRejectOversizedProviderResponses(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		python, err = exec.LookPath("python")
	}
	if err != nil {
		t.Skip("Python runtime required for the actual batch launcher")
	}
	for _, test := range []struct {
		provider, auth string
		bad            bool
	}{{"codex", "chatgpt", false}, {"codex", "api", false}, {"claude-code", "api", false}, {"opencode", "configured", false}, {"claude-code", "api", true}, {"opencode", "configured", true}} {
		t.Run(test.provider+"-"+test.auth+map[bool]string{true: "-oversized", false: ""}[test.bad], func(t *testing.T) {
			dir := t.TempDir()
			input, output := filepath.Join(dir, "input"), filepath.Join(dir, "output")
			os.Mkdir(input, 0700)
			os.Mkdir(output, 0700)
			selection, _ := json.Marshal(map[string]string{"harness": test.provider, "model": "chosen-fixture-model", "authMode": test.auth})
			os.WriteFile(filepath.Join(input, "selection.json"), selection, 0600)
			os.WriteFile(filepath.Join(input, "prompt.txt"), []byte("Actual bounded source snapshot"), 0600)
			os.WriteFile(filepath.Join(input, "schema.json"), []byte(`{"type":"object"}`), 0600)
			fake := filepath.Join(dir, "provider.py")
			script := `import json,os,sys,pathlib
args=sys.argv[2:]
provider=sys.argv[1]
model=args[args.index('--model')+1]
prompt=sys.stdin.read()
receipt={'selectedModel':model=='chosen-fixture-model','sourceSeen':prompt.endswith('Actual bounded source snapshot'),'schemaSeen':provider!='opencode' or ('Response schema:' in prompt and '"type":"object"' in prompt and '--file' not in args),'rawSecretInArgs':any('fixture-private-key' in arg for arg in args),'apiPresent':bool(os.environ.get('OPENAI_API_KEY')),'toolsEmpty':provider!='claude-code' or args[args.index('--tools')+1]==''}
pathlib.Path(os.environ['FIXTURE_RECEIPT']).write_text(json.dumps(receipt))
result={'question':'Who uses it?','options':[],'rationale':'A bounded source sample.','proposal':None,'findings':[{'kind':'inference','summary':'A product question remains.','paths':[]}]}
sys.stderr.write('fixture-private-key-must-not-be-copied')
if os.environ.get('FIXTURE_OVERSIZE')=='true':
 sys.stdout.write('x'*600000)
elif provider=='codex':
 pathlib.Path(args[args.index('--output-last-message')+1]).write_text(json.dumps(result))
 sys.stdout.write('fixture-private-key-must-not-be-copied')
elif provider=='claude-code':
 print(json.dumps({'structured_output':result,'result':'fixture-private-key-must-not-be-copied'}))
else:
 print(json.dumps({'type':'step_start','part':{}}))
 print(json.dumps({'type':'text','part':{'text':json.dumps(result)}}))
`
			os.WriteFile(fake, []byte(script), 0600)
			quote := func(value string) string { b, _ := json.Marshal(filepath.ToSlash(value)); return string(b) }
			launcher := strings.ReplaceAll(RepositoryLauncher, "/luci-input", filepath.ToSlash(input))
			launcher = strings.ReplaceAll(launcher, "/luci-output", filepath.ToSlash(output))
			for _, binary := range []string{"codex", "claude", "opencode"} {
				launcher = strings.ReplaceAll(launcher, "['"+binary+"',", "["+quote(python)+","+quote(fake)+",'"+map[string]string{"codex": "codex", "claude": "claude-code", "opencode": "opencode"}[binary]+"',")
			}
			entry := filepath.Join(dir, "run.py")
			os.WriteFile(entry, []byte(launcher), 0600)
			cmd := exec.Command(python, entry)
			for _, value := range os.Environ() {
				if !strings.HasPrefix(value, "OPENAI_API_KEY=") && !strings.HasPrefix(value, "CODEX_API_KEY=") && !strings.HasPrefix(value, "ANTHROPIC_API_KEY=") {
					cmd.Env = append(cmd.Env, value)
				}
			}
			cmd.Env = append(cmd.Env, "OPENAI_API_KEY=fixture-private-key", "CODEX_API_KEY=fixture-private-key", "ANTHROPIC_API_KEY=fixture-private-key", "FIXTURE_RECEIPT="+filepath.Join(dir, "receipt.json"), "FIXTURE_OVERSIZE="+map[bool]string{true: "true", false: "false"}[test.bad])
			logs, runErr := cmd.CombinedOutput()
			if len(logs) != 0 {
				t.Fatal("raw provider output escaped launcher")
			}
			var receipt struct{ SelectedModel, SourceSeen, SchemaSeen, RawSecretInArgs, APIPresent, ToolsEmpty bool }
			data, _ := os.ReadFile(filepath.Join(dir, "receipt.json"))
			if json.Unmarshal(data, &receipt) != nil || !receipt.SelectedModel || !receipt.SourceSeen || !receipt.SchemaSeen || receipt.RawSecretInArgs || !receipt.ToolsEmpty {
				t.Fatal("argv/stdin/provider isolation failed")
			}
			if test.provider == "codex" && test.auth == "chatgpt" && receipt.APIPresent {
				t.Fatal("subscription command inherited API credentials")
			}
			data, _ = os.ReadFile(filepath.Join(output, "exit.json"))
			var exit struct {
				ExitCode int `json:"exitCode"`
			}
			if json.Unmarshal(data, &exit) != nil {
				t.Fatal("numeric completion receipt missing")
			}
			if test.bad {
				if runErr == nil || exit.ExitCode == 0 {
					t.Fatal("oversized provider output accepted")
				}
				return
			}
			if runErr != nil || exit.ExitCode != 0 {
				t.Fatal("valid adapter output failed", runErr)
			}
			data, _ = os.ReadFile(filepath.Join(output, "result.json"))
			result, err := DecodeResult(data)
			if err != nil || result.Question != "Who uses it?" || strings.Contains(string(data), "fixture-private-key") {
				t.Fatal("structured result not safely extracted", err)
			}
		})
	}
}

func TestOpenCodeReceivesLongContextTailAndExactSchemaThroughStdin(t *testing.T) {
	python, err := exec.LookPath("python3")
	if err != nil {
		t.Skip("Python runtime required for the actual launcher")
	}
	dir := t.TempDir()
	input, output := filepath.Join(dir, "input"), filepath.Join(dir, "output")
	if err := os.Mkdir(input, 0700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(output, 0700); err != nil {
		t.Fatal(err)
	}
	currentRequest := "TAIL_CURRENT_REQUEST_WHO_APPROVES_THE_RELEASE"
	payload, _ := json.Marshal(map[string]string{"currentSpecification": strings.Repeat("long-context-", 300), "userMessage": currentRequest})
	prompt := []byte("Review the complete saved project context and current request.\n" + string(payload))
	if len(payload) <= 3000 || !strings.HasSuffix(string(prompt), `"}`) || !strings.Contains(string(prompt), currentRequest) {
		t.Fatal("fixture did not retain a long line and its tail")
	}
	fields := map[string]any{}
	for i := 0; i < 200; i++ {
		fields["field-"+strings.Repeat("a", i%12)+string(rune('A'+i%26))+string(rune('a'+i/26))] = map[string]string{"type": "string"}
	}
	schema, err := json.Marshal(map[string]any{"type": "object", "properties": fields})
	if err != nil {
		t.Fatal(err)
	}
	if len(schema) < 5000 {
		t.Fatal("fixture must exercise the full response contract")
	}
	for name, data := range map[string][]byte{"prompt.txt": prompt, "schema.json": schema, "selection.json": []byte(`{"harness":"opencode","model":"luci-local/fixture","authMode":"configured"}`)} {
		if err := os.WriteFile(filepath.Join(input, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	fake := filepath.Join(dir, "opencode.py")
	code := `import json,os,pathlib,sys,urllib.request
args=sys.argv[1:]
if os.environ.get('FIXTURE_INVOCATION'):
 pathlib.Path(os.environ['FIXTURE_INVOCATION']).write_text('started')
if os.environ.get('FIXTURE_CHANGE'):
 urllib.request.urlopen(os.environ['FIXTURE_CHANGE'],timeout=2).close()
received=sys.stdin.read()
expected=pathlib.Path(os.environ['FIXTURE_EXPECTED']).read_text()
schema_text=received.split('Response schema:\n',1)[1].split('\nComplete project request:\n',1)[0]
expected_schema=json.loads(pathlib.Path(os.environ['FIXTURE_SCHEMA']).read_text())
if '--file' in args or not received.endswith(expected) or 'TAIL_CURRENT_REQUEST_WHO_APPROVES_THE_RELEASE' not in received or json.loads(schema_text)!=expected_schema:
 sys.exit(9)
if os.environ.get('FIXTURE_FENCED') == '1':
 reply='''` + "```json\n" + `{"question":"Who approves?"}` + "\n```" + `'''
else:
 reply=json.dumps({'question':'Who approves?','options':[],'rationale':'Ask the owner.','proposal':None})
print(json.dumps({'type':'text','part':{'text':reply}}))
`
	if err := os.WriteFile(fake, []byte(code), 0600); err != nil {
		t.Fatal(err)
	}
	var servingContext, evaluatedTokens atomic.Int64
	servingContext.Store(8192)
	evaluatedTokens.Store(1800)
	modelServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/generate":
			var request struct {
				Model   string `json:"model"`
				Prompt  string `json:"prompt"`
				Raw     bool   `json:"raw"`
				Stream  bool   `json:"stream"`
				Options struct {
					NumPredict int `json:"num_predict"`
				} `json:"options"`
			}
			if r.Method != "POST" || json.NewDecoder(http.MaxBytesReader(w, r.Body, 64<<10)).Decode(&request) != nil || request.Model != "fixture" || !request.Raw || request.Stream || request.Options.NumPredict != 1 || !strings.Contains(request.Prompt, currentRequest) {
				http.Error(w, "bad exact-model preflight", 400)
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"model": "fixture", "prompt_eval_count": evaluatedTokens.Load(), "eval_count": 1, "done": true})
		case "/api/ps":
			json.NewEncoder(w).Encode(map[string]any{"models": []map[string]any{{"name": "fixture", "context_length": servingContext.Load(), "digest": "sha256:fixture-exact-model"}}})
		case "/test/change":
			servingContext.Store(4096)
			w.WriteHeader(204)
		default:
			http.NotFound(w, r)
		}
	}))
	defer modelServer.Close()
	u, err := url.Parse(modelServer.URL)
	if err != nil {
		t.Fatal(err)
	}
	port, err := strconv.Atoi(u.Port())
	if err != nil {
		t.Fatal(err)
	}
	modelConfig, _ := json.Marshal(map[string]any{"model": "luci-local/fixture", "provider": map[string]any{"luci-local": map[string]any{"options": map[string]string{"baseURL": modelServer.URL + "/v1"}}}})
	launcher := strings.ReplaceAll(RepositoryLauncher, "/luci-input", filepath.ToSlash(input))
	launcher = strings.ReplaceAll(launcher, "/luci-output", filepath.ToSlash(output))
	launcher = strings.Replace(launcher, "base.port != 11434", "base.port != "+strconv.Itoa(port), 1)
	quoted, _ := json.Marshal(filepath.ToSlash(python))
	quotedFake, _ := json.Marshal(filepath.ToSlash(fake))
	launcher = strings.Replace(launcher, "['opencode',", "["+string(quoted)+","+string(quotedFake)+",", 1)
	entry := filepath.Join(dir, "run.py")
	if err := os.WriteFile(entry, []byte(launcher), 0600); err != nil {
		t.Fatal(err)
	}
	for _, fenced := range []bool{false, true} {
		os.Remove(filepath.Join(output, "result.json"))
		cmd := exec.Command(python, entry)
		cmd.Env = append(os.Environ(), "FIXTURE_EXPECTED="+filepath.Join(input, "prompt.txt"), "FIXTURE_SCHEMA="+filepath.Join(input, "schema.json"), "FIXTURE_FENCED="+map[bool]string{true: "1", false: "0"}[fenced], "OPENCODE_LUCI_INPUT_BYTES=14000", "OPENCODE_LUCI_OLLAMA_PREFLIGHT=1", "OPENCODE_CONFIG_CONTENT="+string(modelConfig))
		logs, runErr := cmd.CombinedOutput()
		if len(logs) != 0 {
			t.Fatal("provider output escaped launcher")
		}
		data, err := os.ReadFile(filepath.Join(output, "exit.json"))
		if err != nil {
			t.Fatal(err)
		}
		var exit struct {
			ExitCode int `json:"exitCode"`
		}
		if json.Unmarshal(data, &exit) != nil {
			t.Fatal("invalid exit receipt")
		}
		if fenced {
			if runErr == nil || exit.ExitCode == 0 {
				t.Fatal("fenced echo was accepted")
			}
			if _, err := os.Stat(filepath.Join(output, "result.json")); !os.IsNotExist(err) {
				t.Fatal("fenced output was published")
			}
		} else {
			if runErr != nil || exit.ExitCode != 0 {
				t.Fatal("complete stdin request failed", runErr)
			}
			result, err := os.ReadFile(filepath.Join(output, "result.json"))
			if err != nil || !strings.Contains(string(result), `"question": "Who approves?"`) {
				t.Fatal("valid structured answer missing", err)
			}
		}
	}
	os.Remove(filepath.Join(output, "result.json"))
	cmd := exec.Command(python, entry)
	cmd.Env = append(os.Environ(), "OPENCODE_LUCI_INPUT_BYTES=14000", "OPENCODE_LUCI_OLLAMA_PREFLIGHT=1", "OPENCODE_CONFIG_CONTENT="+string(modelConfig), "FIXTURE_EXPECTED="+filepath.Join(input, "prompt.txt"), "FIXTURE_SCHEMA="+filepath.Join(input, "schema.json"), "FIXTURE_CHANGE="+modelServer.URL+"/test/change")
	if logs, err := cmd.CombinedOutput(); err == nil || len(logs) != 0 {
		t.Fatal("changed loaded model was accepted", err)
	}
	var changed struct {
		ExitCode    int    `json:"exitCode"`
		FailureCode string `json:"failureCode"`
	}
	changedData, err := os.ReadFile(filepath.Join(output, "exit.json"))
	if err != nil || json.Unmarshal(changedData, &changed) != nil || changed.FailureCode != "local_model_context_unverified" || changed.ExitCode != 2 {
		t.Fatal("changed model did not retain an explicit uncertain failure", err, string(changedData))
	}
	if _, err := os.Stat(filepath.Join(output, "result.json")); !os.IsNotExist(err) {
		t.Fatal("response from changed model was published")
	}
	servingContext.Store(4096)
	os.Remove(filepath.Join(output, "result.json"))
	cmd = exec.Command(python, entry)
	invocation := filepath.Join(dir, "inadequate-context-provider-invocation")
	cmd.Env = append(os.Environ(), "OPENCODE_LUCI_INPUT_BYTES=14000", "OPENCODE_LUCI_OLLAMA_PREFLIGHT=1", "OPENCODE_CONFIG_CONTENT="+string(modelConfig), "FIXTURE_EXPECTED="+filepath.Join(input, "prompt.txt"), "FIXTURE_SCHEMA="+filepath.Join(input, "schema.json"), "FIXTURE_INVOCATION="+invocation)
	if logs, err := cmd.CombinedOutput(); err == nil || len(logs) != 0 {
		t.Fatal("unverified model context reached provider", err)
	}
	var refused struct {
		ExitCode    int    `json:"exitCode"`
		FailureCode string `json:"failureCode"`
	}
	refusedData, err := os.ReadFile(filepath.Join(output, "exit.json"))
	if err != nil || json.Unmarshal(refusedData, &refused) != nil || refused.FailureCode != "local_model_context_limit" || refused.ExitCode != 2 {
		t.Fatal("small actual serving context was not refused", err, string(refusedData))
	}
	if _, err := os.Stat(invocation); !os.IsNotExist(err) {
		t.Fatal("small context started provider")
	}
	// A high-token request can remain below the UTF-8 byte cap. Ollama's own
	// token count must still prevent OpenCode from receiving a truncated input.
	servingContext.Store(8192)
	evaluatedTokens.Store(5000)
	os.Remove(filepath.Join(output, "exit.json"))
	cmd = exec.Command(python, entry)
	cmd.Env = append(os.Environ(), "OPENCODE_LUCI_INPUT_BYTES=14000", "OPENCODE_LUCI_OLLAMA_PREFLIGHT=1", "OPENCODE_CONFIG_CONTENT="+string(modelConfig), "FIXTURE_INVOCATION="+invocation)
	if logs, err := cmd.CombinedOutput(); err == nil || len(logs) != 0 {
		t.Fatal("high-token prompt reached the model", err)
	}
	refusedData, err = os.ReadFile(filepath.Join(output, "exit.json"))
	if err != nil || json.Unmarshal(refusedData, &refused) != nil || refused.FailureCode != "local_model_context_limit" || refused.ExitCode != 2 {
		t.Fatal("high-token request was not refused", err, string(refusedData))
	}
	// The local route refuses an input that cannot fit its conservative serving
	// window before invoking OpenCode. The full current request remains intact.
	tooLarge := append([]byte(strings.Repeat("x", 14000)), prompt...)
	if err := os.WriteFile(filepath.Join(input, "prompt.txt"), tooLarge, 0600); err != nil {
		t.Fatal(err)
	}
	os.Remove(filepath.Join(output, "result.json"))
	cmd = exec.Command(python, entry)
	invocation = filepath.Join(dir, "oversized-provider-invocation")
	cmd.Env = append(os.Environ(), "OPENCODE_LUCI_INPUT_BYTES=14000", "FIXTURE_EXPECTED="+filepath.Join(input, "prompt.txt"), "FIXTURE_SCHEMA="+filepath.Join(input, "schema.json"), "FIXTURE_INVOCATION="+invocation)
	if logs, err := cmd.CombinedOutput(); err == nil || len(logs) != 0 {
		t.Fatal("oversized local input was not refused privately")
	}
	var refusal struct {
		ExitCode    int    `json:"exitCode"`
		FailureCode string `json:"failureCode"`
	}
	data, err := os.ReadFile(filepath.Join(output, "exit.json"))
	if err != nil || json.Unmarshal(data, &refusal) != nil || refusal.ExitCode != 2 || refusal.FailureCode != "local_model_context_limit" {
		t.Fatal("missing bounded local model refusal", err, string(data))
	}
	if _, err := os.Stat(invocation); !os.IsNotExist(err) {
		t.Fatal("oversized prompt started the provider")
	}
}

func TestRepositoryResultFindingsRemainBoundedAndSchemaStrict(t *testing.T) {
	type proposal struct {
		Revision int `json:"revision"`
	}
	b, err := RepositorySchema(proposal{})
	if err != nil {
		t.Fatal(err)
	}
	var schema map[string]any
	json.Unmarshal(b, &schema)
	if len(schema["required"].([]any)) != 5 {
		t.Fatal("repository fields not required")
	}
	valid := `{"question":"Who uses it?","options":[],"rationale":"Partial sample.","proposal":null,"findings":[{"kind":"inference","summary":"Delivery users may be customers.","paths":[]}]}`
	if _, err := DecodeResult([]byte(valid)); err != nil {
		t.Fatal(err)
	}
	for _, invalid := range []string{strings.Replace(valid, "inference", "tested", 1), strings.Replace(valid, "Delivery users may be customers.", "", 1)} {
		if _, err := DecodeResult([]byte(invalid)); err == nil {
			t.Fatal("unbounded/untyped finding accepted")
		}
	}
}

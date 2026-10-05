package assistance

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
prompt=sys.stdin.read() if provider != 'opencode' else pathlib.Path(args[args.index('--file')+1]).read_text()
receipt={'selectedModel':model=='chosen-fixture-model','sourceSeen':prompt=='Actual bounded source snapshot','rawSecretInArgs':any('fixture-private-key' in arg for arg in args),'apiPresent':bool(os.environ.get('OPENAI_API_KEY')),'toolsEmpty':provider!='claude-code' or args[args.index('--tools')+1]==''}
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
			var receipt struct{ SelectedModel, SourceSeen, RawSecretInArgs, APIPresent, ToolsEmpty bool }
			data, _ := os.ReadFile(filepath.Join(dir, "receipt.json"))
			if json.Unmarshal(data, &receipt) != nil || !receipt.SelectedModel || !receipt.SourceSeen || receipt.RawSecretInArgs || !receipt.ToolsEmpty {
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

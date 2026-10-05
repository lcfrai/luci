package assistance

import (
	"encoding/json"
	"errors"
	"strings"
)

// ReadOnlyOpenCode retains only Luci's configured provider, removes instruction
// files/plugins/MCP, and disables every tool on the selected analysis agent.
// The pinned 1.18.34 CLI debug config/agent commands validate this contract.
func ReadOnlyOpenCode(env map[string]string, model string) error {
	var config map[string]any
	if json.Unmarshal([]byte(env["OPENCODE_CONFIG_CONTENT"]), &config) != nil || config["model"] != model {
		return errors.New("Choose the actual saved OpenCode model.")
	}
	config["instructions"] = []string{}
	config["plugin"] = []string{}
	config["mcp"] = map[string]any{}
	config["permission"] = map[string]string{"*": "deny"}
	config["tools"] = map[string]bool{"*": false}
	config["agent"] = map[string]any{"luci-analysis": map[string]any{"mode": "primary", "description": "Analyse supplied source data only", "tools": map[string]bool{"*": false}, "permission": map[string]string{"*": "deny"}}}
	b, err := json.Marshal(config)
	if err != nil {
		return err
	}
	env["OPENCODE_CONFIG_CONTENT"] = string(b)
	env["OPENCODE_PERMISSION"] = `{"*":"deny"}`
	return nil
}

func ValidateSelection(harness, model, authMode string) error {
	if strings.TrimSpace(model) != model || model == "" || model == "default" || len(model) > 200 || strings.HasPrefix(model, "-") || strings.ContainsAny(model, "\x00\r\n\t ") {
		return errors.New("Choose an explicit model name for repository analysis.")
	}
	switch harness {
	case "codex":
		if authMode == "chatgpt" || authMode == "api" {
			return nil
		}
	case "claude-code":
		if authMode == "api" {
			return nil
		}
	case "opencode":
		if authMode == "configured" {
			return nil
		}
	}
	return errors.New("Choose a supported provider and its explicit connection mode.")
}

// RepositoryLauncher runs verified non-interactive CLIs in an empty utility
// workspace. No source mounts or shell interpolation exist. Raw provider output
// is bounded in memory and discarded, never copied to a terminal, log or DTO.
const RepositoryLauncher = `import json, os, subprocess, sys, threading
LIMIT = 262144
with open('/luci-input/prompt.txt', 'rb') as source:
    prompt = source.read(LIMIT+1)
if len(prompt) > LIMIT:
    sys.exit(2)
with open('/luci-input/selection.json') as source:
    selection = json.load(source)
provider, model, auth = selection['harness'], selection['model'], selection['authMode']
env = dict(os.environ)
for name in ('CODEX_API_KEY',):
    env.pop(name, None)
if provider != 'codex':
    env.pop('OPENAI_API_KEY', None)
if provider != 'claude-code':
    env.pop('ANTHROPIC_API_KEY', None)
if provider != 'opencode':
    env.pop('OPENCODE_LOCAL_API_KEY', None)
    env.pop('OPENCODE_CONFIG_CONTENT', None)
if provider == 'codex':
    command = ['codex', '--no-daemon', '-c', 'cli_auth_credentials_store="file"', '-c', 'features.shell_tool=false', '-c', 'features.shell_snapshot=false', '-c', 'features.apps=false', '-c', 'features.remote_plugin=false', '-c', 'features.multi_agent=false', '-c', 'features.hooks=false', '-c', 'features.goals=false', '-c', 'web_search="disabled"']
    if auth == 'chatgpt':
        env.pop('OPENAI_API_KEY', None)
        command += ['-c', 'forced_login_method="chatgpt"', '-c', 'model_provider="openai"']
    elif auth == 'api' and env.get('OPENAI_API_KEY'):
        command += ['-c', 'model_provider="luci-openai"', '-c', 'model_providers.luci-openai={name="OpenAI API",base_url="https://api.openai.com/v1",env_key="OPENAI_API_KEY",wire_api="responses"}']
    else:
        sys.exit(2)
    command += ['--ask-for-approval', 'never', 'exec', '--skip-git-repo-check', '--sandbox', 'read-only', '--model', model, '--output-schema', '/luci-input/schema.json', '--output-last-message', '/luci-output/result.json', '-']
elif provider == 'claude-code' and auth == 'api' and env.get('ANTHROPIC_API_KEY'):
    with open('/luci-input/schema.json') as source:
        schema = json.dumps(json.load(source),separators=(',',':'))
    command = ['claude', '--restricted', '--strict-mcp-config', '--mcp-config', '{"mcpServers":{}}', '--tools', '', '--permission-mode', 'plan', '--print', '--output-format', 'json', '--json-schema', schema, '--model', model]
elif provider == 'opencode' and auth == 'configured':
    command = ['opencode', 'run', '--pure', '--format', 'json', '--agent', 'luci-analysis', '--title', 'Repository analysis', '--model', model, '--file', '/luci-input/prompt.txt', '--', 'Return only the requested JSON object from the attached source analysis request.']
else:
    sys.exit(2)
captured = bytearray()
overflow = [False]
process = subprocess.Popen(command,stdin=subprocess.PIPE,stdout=subprocess.PIPE if provider != 'codex' else subprocess.DEVNULL,stderr=subprocess.DEVNULL,env=env)
def drain():
    while True:
        chunk = process.stdout.read(8192)
        if not chunk:
            return
        if len(captured)+len(chunk)>LIMIT*2:
            overflow[0]=True
        if not overflow[0]:
            captured.extend(chunk)
reader = None
if provider != 'codex':
    reader = threading.Thread(target=drain,daemon=True)
    reader.start()
try:
    process.stdin.write(prompt if provider != 'opencode' else b'')
    process.stdin.close()
    code = process.wait()
    if reader:
        reader.join(2)
    if overflow[0] or (reader and reader.is_alive()):
        code = 2
    if code == 0 and provider != 'codex':
        if provider == 'claude-code':
            envelope = json.loads(captured)
            if envelope.get('is_error'):
                raise ValueError('provider failed')
            result = envelope.get('structured_output')
            if not isinstance(result,dict):
                result = json.loads(envelope['result'])
        else:
            texts=[]
            for line in captured.splitlines():
                event=json.loads(line)
                if event.get('type') == 'error':
                    raise ValueError('provider failed')
                if event.get('type') == 'text':
                    texts.append(event['part']['text'])
            result=json.loads(''.join(texts))
        encoded=json.dumps(result).encode('utf-8')
        if len(encoded)>LIMIT:
            raise ValueError('result exceeds limit')
        with open('/luci-output/result.json.tmp','wb') as target:
            target.write(encoded)
        os.replace('/luci-output/result.json.tmp','/luci-output/result.json')
except Exception:
    code = 2
    if process.poll() is None:
        process.kill()
        process.wait()
with open('/luci-output/exit.json.tmp','w') as target:
    json.dump({'exitCode':code},target)
os.replace('/luci-output/exit.json.tmp','/luci-output/exit.json')
sys.exit(code)
`

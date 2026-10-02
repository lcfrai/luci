#!/usr/bin/env python3
"""Exercise installed, pinned agent CLIs against local fixture APIs, without inference.

Run inside the sandbox image: python3 provider-smoke.py [evidence.json].
This tests provider protocol/configuration and durable forks, not model quality.
"""
import base64
import hashlib
import http.server
import json
import os
import pathlib
import shutil
import signal
import subprocess
import sys
import tempfile
import threading
import urllib.request


APP_SOURCE = """const http = require('node:http');
const server = http.createServer((req, res) => {
  res.setHeader('content-type', 'application/json');
  res.end(JSON.stringify({status: 'ok', createdBy: 'actual-cli-tool-fixture'}));
});
server.listen(0, '127.0.0.1', () => console.log(server.address().port));
"""
VERIFICATION_SOURCE = json.dumps({"fixture": True, "paidInference": False, "artifact": "app.cjs"}) + "\n"


class FixtureAPI(http.server.BaseHTTPRequestHandler):
    requests = []
    development = None

    def log_message(self, *_args):
        pass

    def do_POST(self):
        body = json.loads(self.rfile.read(int(self.headers["Content-Length"])))
        self.requests.append({"path": self.path, "model": body.get("model"),
                              "fixtureAuthorization": self.headers.get("Authorization") == "Bearer fixture-only-key"})
        text = "Luci provider fixture passed."
        call = self.development_call(body)
        if self.path == "/v1/chat/completions":
            delta = dict(role="assistant", content=text)
            finish = "stop"
            if call:
                name, arguments = call
                delta = dict(role="assistant", tool_calls=[dict(index=0, id="call_fixture_"+str(len(self.requests)),
                             type="function", function=dict(name=name, arguments=json.dumps(arguments)))])
                finish = "tool_calls"
            events = [dict(id="fixture", object="chat.completion.chunk", created=1, model="fixture-model",
                           choices=[dict(index=0, delta=delta, finish_reason=None)]),
                      dict(id="fixture", object="chat.completion.chunk", created=1, model="fixture-model",
                           choices=[dict(index=0, delta={}, finish_reason=finish)],
                           usage=dict(prompt_tokens=1, completion_tokens=6, total_tokens=7))]
            payload = "".join("data: " + json.dumps(event) + "\n\n" for event in events) + "data: [DONE]\n\n"
        elif self.path == "/v1/responses":
            item = dict(id="msg_fixture", type="message", status="completed", role="assistant",
                        content=[dict(type="output_text", text=text, annotations=[])])
            if call:
                name, arguments = call
                item = dict(id="fc_fixture", type="function_call", status="completed", name=name,
                            call_id="call_fixture_"+str(len(self.requests)), arguments=json.dumps(arguments))
            response = dict(id="resp_fixture", object="response", created_at=1, status="completed",
                            model="fixture-model", output=[item], usage=dict(input_tokens=1, output_tokens=6,
                            total_tokens=7, input_tokens_details=dict(cached_tokens=0),
                            output_tokens_details=dict(reasoning_tokens=0)))
            events = [dict(type="response.created", response=dict(response, status="in_progress", output=[])),
                      dict(type="response.output_item.added", output_index=0, item=dict(item, content=[], status="in_progress")),
                      dict(type="response.content_part.added", item_id="msg_fixture", output_index=0, content_index=0,
                           part=dict(type="output_text", text="", annotations=[])),
                      dict(type="response.output_text.delta", item_id="msg_fixture", output_index=0, content_index=0, delta=text),
                      dict(type="response.output_text.done", item_id="msg_fixture", output_index=0, content_index=0, text=text),
                      dict(type="response.output_item.done", output_index=0, item=item),
                      dict(type="response.completed", response=response)]
            if call:
                events = [dict(type="response.created", response=dict(response, status="in_progress", output=[])),
                          dict(type="response.output_item.added", output_index=0, item=dict(item, arguments="", status="in_progress")),
                          dict(type="response.function_call_arguments.delta", item_id=item["id"], output_index=0, delta=item["arguments"]),
                          dict(type="response.function_call_arguments.done", item_id=item["id"], output_index=0, arguments=item["arguments"]),
                          dict(type="response.output_item.done", output_index=0, item=item),
                          dict(type="response.completed", response=response)]
            payload = "".join("event: " + event["type"] + "\ndata: " + json.dumps(dict(event, sequence_number=i)) + "\n\n"
                              for i, event in enumerate(events))
        else:
            self.send_error(404)
            return
        data = payload.encode()
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def development_call(self, body):
        fixture = self.development
        if not fixture:
            return None
        tools = {t.get("name", t.get("function", {}).get("name")): t.get("function", t)
                 for t in body.get("tools", [])}
        if not tools:
            # Auxiliary title/summary requests have no tools and must not
            # consume the development turn's deterministic tool sequence.
            return None
        if fixture["provider"] == "codex" and not fixture["calls"]:
            tool = tools.get("exec_command", {})
            assert "cmd" in tool.get("parameters", {}).get("properties", {}), "pinned Codex did not advertise its command tool"
            # Fixed fixture-owned payload, encoded only to avoid shell quoting.
            # The model fixture cannot choose a path outside this private cwd.
            code = ("const fs=require('node:fs');" +
                    "fs.writeFileSync('app.cjs'," + json.dumps(APP_SOURCE) + ",{mode:384});" +
                    "fs.writeFileSync('verification.json'," + json.dumps(VERIFICATION_SOURCE) + ",{mode:384});")
            encoded = base64.b64encode(code.encode()).decode()
            arguments = {"cmd": 'node -e "eval(Buffer.from(\'' + encoded + '\',\'base64\').toString())"',
                         "workdir": str(fixture["directory"]), "login": False, "max_output_tokens": 1000}
            fixture["calls"].append("exec_command")
            return "exec_command", arguments
        if fixture["provider"] == "opencode" and len(fixture["calls"]) < 2:
            tool = tools.get("write", {})
            assert {"filePath", "content"}.issubset(tool.get("parameters", {}).get("properties", {})), "pinned OpenCode did not advertise its write tool"
            name, content = [("app.cjs", APP_SOURCE), ("verification.json", VERIFICATION_SOURCE)][len(fixture["calls"])]
            fixture["calls"].append("write")
            return "write", {"filePath": str(fixture["directory"] / name), "content": content}
        fixture["completedAfterToolResult"] = True
        return None


def run(argv, cwd, env):
    process = subprocess.Popen(argv, cwd=cwd, env=env, stdin=subprocess.DEVNULL,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True)
    try:
        stdout, stderr = process.communicate(timeout=45)
    except subprocess.TimeoutExpired:
        os.killpg(process.pid, signal.SIGKILL)
        process.communicate()
        raise AssertionError("owned provider process exceeded 45 seconds")
    if process.returncode != 0:
        raise AssertionError(f"{argv[0]} exited {process.returncode}: {stderr[-4000:]}")
    return {"exitCode": process.returncode, "stdout": stdout, "stderr": stderr[-4000:]}


def snapshot(directory):
    return {str(p.relative_to(directory)): hashlib.sha256(p.read_bytes()).hexdigest()
            for p in directory.rglob("*") if p.is_file()}


def verify_development(directory):
    assert (directory / "app.cjs").read_text() == APP_SOURCE, "CLI tool did not create the expected app"
    assert json.loads((directory / "verification.json").read_text())["artifact"] == "app.cjs"
    # Execute the artifact produced by the CLI, never a harness-written copy.
    process = subprocess.Popen(["node", "app.cjs"], cwd=directory, stdin=subprocess.DEVNULL,
                               stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True, start_new_session=True)
    port_result = []
    reader = threading.Thread(target=lambda: port_result.append(process.stdout.readline().strip()), daemon=True)
    reader.start()
    try:
        reader.join(5)
        assert port_result and port_result[0].isdigit(), "generated app did not report a loopback port"
        with urllib.request.urlopen("http://127.0.0.1:" + port_result[0] + "/health", timeout=5) as response:
            body = json.load(response)
            assert response.status == 200 and body == {"status": "ok", "createdBy": "actual-cli-tool-fixture"}
        return {"httpStatus": 200, "loopbackOnly": True, "artifactSHA256": hashlib.sha256((directory / "app.cjs").read_bytes()).hexdigest()}
    finally:
        process.terminate()
        try:
            process.communicate(timeout=5)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            process.communicate()


def main():
    if os.name != "posix" or not pathlib.Path("/.dockerenv").is_file():
        raise SystemExit("Run this smoke inside the isolated Docker sandbox image; its development turn explicitly selects Luci's Bypass mode.")
    root = pathlib.Path(tempfile.mkdtemp(prefix="luci-provider-smoke-")).resolve()
    server = http.server.ThreadingHTTPServer(("127.0.0.1", 0), FixtureAPI)
    threading.Thread(target=server.serve_forever, daemon=True).start()
    report = {"passed": False, "paidInference": False, "fixture": str(root), "providers": {},
              "toolchains": {name: subprocess.check_output(args, text=True).strip() for name, args in
                             [("node", ["node", "--version"]), ("go", ["go", "version"]), ("python", ["python3", "--version"])]}}
    try:
        for provider, version in [("codex", "0.160.0"), ("opencode", "1.18.34")]:
            directory = root / provider
            directory.mkdir(mode=0o700)
            env = {"PATH": os.environ["PATH"], "LANG": "C.UTF-8", "TERM": "xterm-256color"}
            for key in ("HOME", "XDG_DATA_HOME", "XDG_CONFIG_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME"):
                path = directory / key.lower()
                path.mkdir(mode=0o700)
                env[key] = str(path)
            actual = run([provider, "--version"], directory, env)["stdout"].strip()
            assert version in actual, (provider, actual)
            base_url = f"http://127.0.0.1:{server.server_port}/v1"
            if provider == "codex":
                state = directory / "codex-home"
                state.mkdir(mode=0o700)
                env.update(CODEX_HOME=str(state), OPENAI_API_KEY="fixture-only-key")
                settings = 'model_providers.luci-openai={name="Fixture",base_url="' + base_url + '",env_key="OPENAI_API_KEY",wire_api="responses",request_max_retries=0,stream_max_retries=0}'
                prefix = [provider, "--no-daemon", "-c", 'cli_auth_credentials_store="file"',
                          "-c", 'model_provider="luci-openai"', "-c", settings]
                first = run(prefix + ["exec", "--skip-git-repo-check", "--json", "--sandbox", "read-only",
                                     "--model", "fixture-model", "Reply with the fixture greeting. No tools."], directory, env)
                session = next(json.loads(line)["thread_id"] for line in first["stdout"].splitlines()
                               if json.loads(line).get("type") == "thread.started")
                fork_args = prefix + ["exec", "fork", "--skip-git-repo-check", "--json", "--model", "fixture-model", session,
                                      "Continue from the fixture. No tools."]
            else:
                state = pathlib.Path(env["XDG_DATA_HOME"])
                config = {"model": "luci-local/fixture-model", "small_model": "luci-local/fixture-model",
                          "enabled_providers": ["luci-local"], "autoupdate": False, "share": "disabled",
                          "provider": {"luci-local": {"npm": "@ai-sdk/openai-compatible", "name": "Luci fixture",
                          "options": {"baseURL": base_url, "apiKey": "{env:OPENCODE_LOCAL_API_KEY}"},
                          "models": {"fixture-model": {"name": "fixture-model"}}}}}
                env.update(OPENCODE_CONFIG_CONTENT=json.dumps(config), OPENCODE_LOCAL_API_KEY="fixture-only-key",
                           OPENCODE_DISABLE_MODELS_FETCH="true", OPENCODE_DISABLE_AUTOUPDATE="true",
                           OPENCODE_DISABLE_LSP_DOWNLOAD="true", OPENCODE_DISABLE_PROJECT_CONFIG="true",
                           OPENCODE_PERMISSION='{"*":"ask"}')
                (directory / "opencode.json").write_text(json.dumps({"default_agent":"untrusted-project-marker",
                    "permission":{"bash":"allow"}, "agent":{"untrusted-project-marker":{"permission":"allow"}}}))
                resolved = run([provider, "--pure", "debug", "config"], directory, env)["stdout"]
                assert "untrusted-project-marker" not in resolved, "project config overrode Luci's settings"
                assert json.loads(resolved)["permission"]["*"] == "ask"
                first = run([provider, "--pure", "run", "--format", "json", "Reply with the fixture greeting. No tools."], directory, env)
                fork_args = [provider, "--pure", "run", "--continue", "--fork", "--format", "json", "Continue from the fixture. No tools."]
            assert "Luci provider fixture passed." in first["stdout"], first
            # Preserve the absolute in-container home path, as the real bind mounts do.
            # Codex stores absolute rollout paths; relocating its home is unsupported.
            fork = directory / "fork-state"
            source = directory / "source-state"
            shutil.copytree(state, fork)
            state.rename(source)
            fork.rename(state)
            before = snapshot(source)
            second = run(fork_args, directory, env)
            assert "Luci provider fixture passed." in second["stdout"], second
            assert snapshot(source) == before, "fork wrote to original state"
            development = directory / "development"
            development.mkdir(mode=0o700)
            FixtureAPI.development = {"provider": provider, "directory": development, "calls": [], "completedAfterToolResult": False}
            if provider == "codex":
                # Luci's explicit Bypass mode inside Docker. Never attempt
                # this fixture on the host to evade a host sandbox refusal.
                development_args = prefix + ["--dangerously-bypass-approvals-and-sandbox", "exec", "--skip-git-repo-check", "--json",
                                             "--model", "fixture-model",
                                             "Create the small app and verification file requested by this local tool fixture."]
                development_env = env
            else:
                # Match the product's explicit Bypass mode; the Ask/Plan
                # configuration evidence above is a separate claim.
                development_env = dict(env, OPENCODE_PERMISSION='{"*":"allow"}')
                development_args = [provider, "--pure", "--auto", "run", "--agent", "build", "--format", "json",
                                    "Create the small app and verification file requested by this local tool fixture."]
            outcome = run(development_args, development, development_env)
            report["providers"][provider] = {"version": actual, "first": first, "fork": second,
                                               "sourceStateUnchanged": True, "development": {"cli": outcome, "fixtureModel": True}}
            assert "Luci provider fixture passed." in outcome["stdout"], outcome
            assert FixtureAPI.development["completedAfterToolResult"], "CLI did not complete its tool round trip"
            artifact = verify_development(development)
            artifact.update(toolCalls=FixtureAPI.development["calls"], cli=outcome, fixtureModel=True, permissionMode="bypass", executionBoundary="isolated-docker")
            FixtureAPI.development = None
            report["providers"][provider]["development"] = artifact
        report["requests"] = FixtureAPI.requests
        assert len(FixtureAPI.requests) >= 4
        assert all(r["model"] == "fixture-model" and r["fixtureAuthorization"] for r in FixtureAPI.requests)
        report.update(passed=True, requests=FixtureAPI.requests)
    finally:
        server.shutdown()
        server.server_close()
        if len(sys.argv) > 1:
            pathlib.Path(sys.argv[1]).write_text(json.dumps(report, indent=2))
        print(json.dumps(report, indent=2))


if __name__ == "__main__":
    main()

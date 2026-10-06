// Installed connection acceptance with deterministic local APIs, never inference.
// Inert without --run and an exact secondary-instance/package grant. This does
// not satisfy the separate real-subscription or live lcfr.ai gateway benchmark.
import assert from "node:assert/strict";
import { createHash, randomBytes, randomUUID } from "node:crypto";
import { spawn } from "node:child_process";
import { mkdir, readFile, writeFile, rename } from "node:fs/promises";
import path from "node:path";
import { setTimeout as delay } from "node:timers/promises";

const APP = `const http = require('node:http');
const server = http.createServer((req, res) => {
  res.setHeader('content-type', 'application/json');
  res.end(JSON.stringify({status: 'ok', createdBy: 'actual-cli-tool-fixture'}));
});
server.listen(0, '127.0.0.1', () => console.log(server.address().port));
`;
const VERIFICATION = JSON.stringify({ fixture: true, paidInference: false, artifact: "app.cjs" }) + "\n";
const QUESTION = { question: "Which user should this fixture application serve first?", options: ["A project owner", "A collaborator"], rationale: "Deterministic connection protocol fixture; no inference.", proposal: null };
const required = ["pins", "routeControls", "codexToolWrite", "codexDefine", "codexRepository", "shellPersistence", "accountIsolation", "secretFreeRecords", "cleanup"];
const hash = value => createHash("sha256").update(value).digest("hex");
const privateValues = new Set();
function secret() { const value = randomBytes(32).toString("hex"); privateValues.add(value); return value; }
function assertPublic(value) { const text = JSON.stringify(value); for (const item of privateValues) assert(!text.includes(item), "Private credential escaped a public record"); return value; }
function redact(error) { let text = String(error?.message || "Acceptance failed"); for (const item of privateValues) text = text.replaceAll(item, "[private]"); return text.slice(0, 600); }
function complete(value) { return value.status !== "failed" && required.every(key => value.proofs?.[key]?.passed === true) && value.fixture === true && value.paidInference === false; }
function secondaryOrigin(value) {
  const u = new URL(value);
  assert(u.origin === "http://127.0.0.1:8750" && u.pathname === "/" && !u.username && !u.password && !u.search && !u.hash, "Only the explicitly approved secondary origin http://127.0.0.1:8750 is permitted");
  return u.origin;
}
const args = process.argv.slice(2);
const option = (name, fallback = "") => { const i = args.indexOf(name); return i < 0 ? fallback : args[i + 1]; };
const plan = {
  scope: "Installed Docker fixture acceptance; not model quality, paid inference, subscription acceptance or a live LiteLLM deployment",
  target: "http://127.0.0.1:8750 only; never 8740",
  mutations: ["Create two uniquely named member accounts using an explicitly supplied administrator login", "Create two empty projects only in the first new account", "Save/select/edit/disable/remove fixture-only connections", "Create, attach, restart and stop only recorded fixture sessions", "Start one labelled private-engine fixture container with no host mounts or published ports", "Disable only the new accounts after recording results; preserve their projects and history"],
  proofs: required,
  protocols: "Codex Responses tool execution and Define/repository are mandatory. --protocols codex,claude-code,opencode additionally runs the actual read-only helper CLIs through Messages/ChatCompletions. It does not claim Claude/OpenCode interactive coding tool execution.",
  gates: ["--run --authorize-installed-secondary", "--manifest <matching distribution-manifest.json>", "--expected-source <full clean source commit> --expected-version 0.4.17", "--controller <exact container> --engine <exact container>", "--controller-image sha256:... --engine-image sha256:... --sandbox-image sha256:...", "--evidence <new dedicated directory>", "LUCI_EVAL_ADMIN_USERNAME and LUCI_EVAL_ADMIN_PASSWORD in the process environment only"],
  command: "node scripts/connection-journey-eval.mjs --run --authorize-installed-secondary --manifest <path> --expected-source <commit> --expected-version 0.4.17 --controller <name> --engine <name> --controller-image <sha256> --engine-image <sha256> --sandbox-image <sha256> --evidence <new-dir> [--protocols codex,claude-code,opencode]",
};

// Based on scripts/provider-smoke.py's pinned Responses/ChatCompletions fixtures.
// Messages SSE follows https://platform.claude.com/docs/en/build-with-claude/streaming.
// The entire program runs in one disposable nonroot private-engine container.
function fixtureProgram() {
  const http = require("node:http");
  let input = "";
  process.stdin.on("data", bytes => { input += bytes; if (input.length > 32768) process.exit(2); });
  process.stdin.on("end", () => {
    const config = JSON.parse(input); input = "";
    let state = { phase: "idle", model: "fixture-codex", key: config.key, calls: 0, toolResult: false };
    const records = [];
    const json = (res, status, value) => { const bytes = JSON.stringify(value); res.writeHead(status, { "content-type": "application/json" }); res.end(bytes); };
    const sse = (res, events, named = true, done = false) => {
      res.writeHead(200, { "content-type": "text/event-stream" });
      events.forEach((event, i) => res.write((named ? `event: ${event.type}\n` : "") + "data: " + JSON.stringify(named && event.type.startsWith("response.") ? { ...event, sequence_number: i } : event) + "\n\n"));
      res.end(done ? "data: [DONE]\n\n" : "");
    };
    const server = http.createServer(async (req, res) => {
      try {
        const route = new URL(req.url, "http://fixture").pathname;
        let raw = ""; for await (const chunk of req) { raw += chunk; if (raw.length > 2 * 1024 * 1024) throw Error("bounded body"); }
        const body = raw ? JSON.parse(raw) : {};
        if (route === "/__control") {
          if (req.headers.authorization !== "Bearer " + config.control) return json(res, 403, {});
          if (req.method === "POST") state = { ...state, ...body, calls: 0, toolResult: false };
          return json(res, 200, { state: { phase: state.phase, calls: state.calls, toolResult: state.toolResult }, records });
        }
        const allowed = ["/v1/responses", "/v1/messages", "/v1/messages/count_tokens", "/v1/chat/completions"];
        const authorized = req.headers.authorization === "Bearer " + state.key || req.headers["x-api-key"] === state.key;
        records.push({ phase: state.phase, route: allowed.includes(route) ? route : "unsupported", model: typeof body.model === "string" ? body.model.slice(0, 200) : "", authorized, sourcePresent: raw.includes("app.cjs") && raw.includes("actual-cli-tool-fixture") });
        if (records.length > 100) return json(res, 429, { error: "fixture request budget" });
        if (!authorized) return json(res, 401, { error: "fixture credential mismatch" });
        if (!allowed.includes(route) || req.method !== "POST") return json(res, 404, { error: "unsupported fixture protocol" });
        if (route.endsWith("count_tokens")) return json(res, 200, { input_tokens: 1 });
        if (body.model !== state.model) return json(res, 400, { error: "fixture model mismatch" });
        let text = "Luci connection fixture completed.";
        if (state.phase.includes("define")) text = JSON.stringify(config.question);
        if (state.phase.includes("repository")) text = JSON.stringify({ ...config.question, findings: [{ kind: "observed", summary: "The supplied fixture snapshot includes app.cjs.", paths: ["app.cjs"] }] });
        let call;
        if (state.phase === "codex-code") {
          const tools = Object.fromEntries((body.tools || []).map(t => [t.name || t.function?.name, t.function || t]));
          if (!state.calls && Object.keys(tools).length) {
            if (!tools.exec_command?.parameters?.properties?.cmd) return json(res, 400, { error: "pinned exec_command unavailable" });
            const code = "const fs=require('node:fs');fs.writeFileSync('app.cjs'," + JSON.stringify(config.app) + ",{mode:384});fs.writeFileSync('verification.json'," + JSON.stringify(config.verification) + ",{mode:384});";
            call = { name: "exec_command", arguments: JSON.stringify({ cmd: `node -e "eval(Buffer.from('${Buffer.from(code).toString("base64")}','base64').toString())"`, workdir: "/home/dev/work", login: false, max_output_tokens: 1000 }) };
            state.calls++;
          } else if (Array.isArray(body.input) && body.input.some(item => item.type === "function_call_output")) state.toolResult = true;
        }
        if (route === "/v1/responses") {
          const id = "fixture_" + records.length;
          const item = call ? { id: "fc_" + id, type: "function_call", status: "completed", name: call.name, call_id: "call_" + id, arguments: call.arguments } : { id: "msg_" + id, type: "message", status: "completed", role: "assistant", content: [{ type: "output_text", text, annotations: [] }] };
          const response = { id: "resp_" + id, object: "response", created_at: 1, status: "completed", model: body.model, output: [item], usage: { input_tokens: 1, output_tokens: 6, total_tokens: 7, input_tokens_details: { cached_tokens: 0 }, output_tokens_details: { reasoning_tokens: 0 } } };
          if (!body.stream) return json(res, 200, response);
          const events = [{ type: "response.created", response: { ...response, status: "in_progress", output: [] } }, { type: "response.output_item.added", output_index: 0, item: { ...item, status: "in_progress", ...(call ? { arguments: "" } : { content: [] }) } }];
          if (call) events.push({ type: "response.function_call_arguments.delta", item_id: item.id, output_index: 0, delta: item.arguments }, { type: "response.function_call_arguments.done", item_id: item.id, output_index: 0, arguments: item.arguments });
          else events.push({ type: "response.content_part.added", item_id: item.id, output_index: 0, content_index: 0, part: { type: "output_text", text: "", annotations: [] } }, { type: "response.output_text.delta", item_id: item.id, output_index: 0, content_index: 0, delta: text }, { type: "response.output_text.done", item_id: item.id, output_index: 0, content_index: 0, text });
          events.push({ type: "response.output_item.done", output_index: 0, item }, { type: "response.completed", response }); return sse(res, events);
        }
        if (route === "/v1/messages") {
          const message = { id: "msg_fixture", type: "message", role: "assistant", content: [{ type: "text", text }], model: body.model, stop_reason: "end_turn", stop_sequence: null, usage: { input_tokens: 1, output_tokens: 6 } };
          if (!body.stream) return json(res, 200, message);
          return sse(res, [{ type: "message_start", message: { ...message, content: [], stop_reason: null } }, { type: "content_block_start", index: 0, content_block: { type: "text", text: "" } }, { type: "content_block_delta", index: 0, delta: { type: "text_delta", text } }, { type: "content_block_stop", index: 0 }, { type: "message_delta", delta: { stop_reason: "end_turn", stop_sequence: null }, usage: { output_tokens: 6 } }, { type: "message_stop" }]);
        }
        const completion = { id: "fixture", object: "chat.completion", created: 1, model: body.model, choices: [{ index: 0, message: { role: "assistant", content: text }, finish_reason: "stop" }], usage: { prompt_tokens: 1, completion_tokens: 6, total_tokens: 7 } };
        if (!body.stream) return json(res, 200, completion);
        return sse(res, [{ ...completion, object: "chat.completion.chunk", choices: [{ index: 0, delta: { role: "assistant", content: text }, finish_reason: null }] }, { ...completion, object: "chat.completion.chunk", choices: [{ index: 0, delta: {}, finish_reason: "stop" }] }], false, true);
      } catch { if (!res.headersSent) json(res, 500, { error: "fixture protocol failure" }); else res.end(); }
    });
    server.listen(config.listenPort ?? 4100, config.listenHost || "0.0.0.0", () => console.log(JSON.stringify({ fixturePort: server.address().port })));
  });
}

if (args.includes("--self-test")) {
  assert.throws(() => secondaryOrigin("http://127.0.0.1:8740"));
  assert.throws(() => secondaryOrigin("http://example.com:8750"));
  assert.equal(secondaryOrigin("http://127.0.0.1:8750"), "http://127.0.0.1:8750");
  const receipt = { fixture: true, paidInference: false, proofs: Object.fromEntries(required.map(key => [key, { passed: true }])) };
  assert(complete(receipt)); for (const key of required) { const copy = structuredClone(receipt); delete copy.proofs[key]; assert(!complete(copy)); }
  assert(!complete({ ...receipt, fixture: false })); assert(!complete({ ...receipt, paidInference: true }));
  assert(!complete({ ...receipt, status: "failed" }));
  const privateMarker = secret(); assert.throws(() => assertPublic({ nested: { unexpected: privateMarker } }));
  assert.equal(redact(new Error(privateMarker)), "[private]");
  // Exercise the actual protocol producer on a random loopback port. It only
  // emits deterministic replies/tool-call data: no CLI or command is executed.
  const key = secret(), control = secret();
  const fixture = spawn(process.execPath, ["-e", `(${fixtureProgram.toString()})()`], { windowsHide: true, stdio: ["pipe", "pipe", "ignore"] });
  const started = new Promise((resolve, reject) => {
    let output = ""; const timer = setTimeout(() => reject(new Error("Loopback fixture self-test timed out")), 5000);
    fixture.stdout.on("data", bytes => { output += bytes; if (output.includes("\n")) { clearTimeout(timer); resolve(JSON.parse(output.split("\n")[0]).fixturePort); } });
    fixture.once("error", () => { clearTimeout(timer); reject(new Error("Could not start loopback fixture self-test")); });
  });
  fixture.stdin.end(JSON.stringify({ key, control, app: APP, verification: VERIFICATION, question: QUESTION, listenHost: "127.0.0.1", listenPort: 0 }));
  try {
    const url = "http://127.0.0.1:" + await started;
    const request = async (route, body, credential = key, expected = 200) => { const r = await fetch(url + route, { method: "POST", headers: { Authorization: "Bearer " + credential, "Content-Type": "application/json" }, body: JSON.stringify(body), signal: AbortSignal.timeout(5000) }); assert.equal(r.status, expected); return r.text(); };
    await request("/__control", { phase: "codex-code", model: "fixture-codex" }, control);
    const call = await request("/v1/responses", { model: "fixture-codex", stream: true, tools: [{ type: "function", name: "exec_command", parameters: { properties: { cmd: { type: "string" } } } }] });
    assert(call.includes('"type":"function_call"') && call.includes('"name":"exec_command"') && call.includes("response.completed"));
    await request("/v1/responses", { model: "fixture-codex", stream: true, input: [{ type: "function_call_output", call_id: "fixture", output: "fixture result" }] });
    const controlRead = await fetch(url + "/__control", { headers: { Authorization: "Bearer " + control }, signal: AbortSignal.timeout(5000) });
    const observed = await controlRead.json(); assert(observed.state.toolResult && observed.state.calls === 1);
    await request("/__control", { phase: "codex-define" }, control);
    const response = JSON.parse(await request("/v1/responses", { model: "fixture-codex", stream: false })); assert.equal(JSON.parse(response.output[0].content[0].text).question, QUESTION.question);
    await request("/__control", { phase: "claude-code-define", model: "fixture-claude" }, control);
    const messages = await request("/v1/messages", { model: "fixture-claude", stream: true }); assert(messages.includes("event: message_start") && messages.includes("event: message_stop"));
    await request("/__control", { phase: "opencode-define", model: "fixture-chat" }, control);
    const chat = await request("/v1/chat/completions", { model: "fixture-chat", stream: true }); assert(chat.includes('"object":"chat.completion.chunk"') && chat.endsWith("data: [DONE]\n\n"));
    await request("/v1/chat/completions", { model: "fixture-chat" }, "wrong-fixture-key", 401);
    await request("/v1/chat/completions", { model: "wrong-model" }, key, 400);
  } finally { fixture.kill(); }
  console.log("Runner guards and loopback Responses/Messages/ChatCompletions fixture checks passed. No Docker, account, CLI, model or installed acceptance run occurred.");
  process.exit(0);
}
if (!args.includes("--run") || args.includes("--help") || args.includes("--plan")) { console.log(JSON.stringify(plan, null, 2)); process.exit(0); }

assert(args.includes("--authorize-installed-secondary"), "An explicit matching-secondary run grant is required");
const origin = secondaryOrigin(option("--url", "http://127.0.0.1:8750"));
const controller = option("--controller"), engine = option("--engine"), sandboxImage = option("--sandbox-image");
const expectedSource = option("--expected-source"), expectedVersion = option("--expected-version");
assert.match(expectedSource, /^[a-f0-9]{40}(?:[a-f0-9]{24})?$/); assert.equal(expectedVersion, "0.4.17");
for (const name of [controller, engine]) assert.match(name, /^[a-zA-Z0-9][a-zA-Z0-9_.-]{0,127}$/);
for (const pin of [option("--controller-image"), option("--engine-image"), sandboxImage]) assert.match(pin, /^sha256:[a-f0-9]{64}$/);
const protocols = option("--protocols", "codex").split(",");
assert(protocols[0] === "codex" && new Set(protocols).size === protocols.length && protocols.every(v => ["codex", "claude-code", "opencode"].includes(v)), "Choose codex[,claude-code,opencode]");
assert(typeof WebSocket === "function", "Use Node 22 or later with native WebSocket support");
const evidence = path.resolve(option("--evidence") || ""); assert(option("--evidence"), "Provide a new dedicated evidence directory");
await mkdir(evidence, { mode: 0o700 }); // exclusive: never adopt an old or unrelated receipt
const nonce = randomUUID(), label = "ai.lcfr.luci.connection-eval", fixtureName = "luci-connection-fixture-" + nonce;
const result = { schema: 1, fixture: true, paidInference: false, passed: false, status: "running", origin, nonce, sourceCommit: expectedSource, version: expectedVersion, startedAt: new Date().toISOString(), requestedProtocols: protocols, proofs: {}, steps: [], accounts: [], projects: [], sessions: [], limits: [plan.scope, plan.protocols, "Only a matching installed run can populate this receipt; --self-test is not acceptance.", "Owned projects/history remain for inspection; only owned fixture container and created sessions are stopped, and only created accounts are disabled."] };
let adminToken, accountToken, observerToken, fixtureProcess, fixtureCreated = false;
const passwords = [], accountIDs = [], live = new Set();
const fixtureKey = secret(), controlKey = secret();
let activeKey = fixtureKey;
async function save() { assertPublic(result); const tmp = path.join(evidence, "results.json.tmp"); await writeFile(tmp, JSON.stringify(result, null, 2) + "\n", { mode: 0o600 }); await rename(tmp, path.join(evidence, "results.json")); }
async function proof(name, fields = {}) { result.proofs[name] = { passed: true, ...assertPublic(fields) }; await save(); }
async function step(name) { result.steps.push({ name, at: new Date().toISOString() }); await save(); }
async function command(argv, input = "", timeout = 30000) {
  return new Promise((resolve, reject) => {
    const child = spawn("docker", argv, { windowsHide: true, stdio: ["pipe", "pipe", "pipe"] });
    let output = "", size = 0; const timer = setTimeout(() => { child.kill(); reject(new Error("Bounded Docker operation timed out")); }, timeout);
    child.stdout.on("data", data => { size += data.length; if (size > 2 * 1024 * 1024) { child.kill(); reject(new Error("Bounded Docker response exceeded limit")); } else output += data; });
    child.stderr.on("data", () => {}); // never persist raw Docker/CLI diagnostics
    child.on("error", () => { clearTimeout(timer); reject(new Error("Docker could not execute the bounded operation")); });
    child.on("close", code => { clearTimeout(timer); code === 0 ? resolve(output.trim()) : reject(new Error(`Docker ${argv[0]} failed with exit ${code}`)); });
    child.stdin.end(input);
  });
}
const inner = (argv, input = "", timeout) => command(["exec", "-i", engine, "docker", "--host=unix:///run/luci-engine/docker.sock", ...argv], input, timeout);
async function api(method, route, body, expected = 200, token = accountToken) {
  const response = await fetch(origin + route, { method, headers: { ...(token ? { Authorization: "Bearer " + token } : {}), ...(body === undefined ? {} : { "Content-Type": "application/json" }) }, body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(45000) });
  assert([expected].flat().includes(response.status), `${method} ${route.split("?")[0]} returned HTTP ${response.status}`);
  const text = await response.text(); assert(text.length < 2 * 1024 * 1024, "Bounded API response exceeded limit");
  const data = text ? JSON.parse(text) : null;
  if (route !== "/v1/auth/login" && route !== "/v1/auth/socket-ticket") assertPublic(data);
  return data;
}
async function until(read, name, timeout = 90000) { const end = Date.now() + timeout; do { const value = await read(); if (value) return value; await delay(500); } while (Date.now() < end); throw new Error(name + " timed out; no success inferred"); }
async function fixtureControl(update) {
  const code = `let s='';process.stdin.on('data',b=>s+=b);process.stdin.on('end',async()=>{const v=JSON.parse(s);const r=await fetch('http://127.0.0.1:4100/__control',{method:v.update?'POST':'GET',headers:{Authorization:'Bearer '+v.control,'Content-Type':'application/json'},body:v.update?JSON.stringify(v.update):undefined});if(!r.ok)process.exit(2);process.stdout.write(await r.text())});`;
  return assertPublic(JSON.parse(await inner(["exec", "--user", "1001:1001", "-i", fixtureName, "node", "-e", code], JSON.stringify({ control: controlKey, update }))));
}
async function sessionContainer(id) {
  assert(result.sessions.includes(id), "Only this run's recorded sessions can be inspected");
  const names = (await inner(["ps", "--all", "--filter", `label=luci.session-id=${id}`, "--format", "{{.ID}}"])).split(/\s+/).filter(Boolean);
  assert.equal(names.length, 1, "Expected exactly one owned session container");
  const info = JSON.parse(await inner(["inspect", names[0]]))[0];
  assert.equal(info.Config.Labels["luci.session-id"], id); assert.equal(info.Config.Labels["luci.managed"], "true"); assert.equal(info.Image, sandboxImage);
  for (const entry of info.Config.Env || []) if (entry.startsWith("LUCI_ATTACH_TOKEN=")) privateValues.add(entry.slice("LUCI_ATTACH_TOKEN=".length));
  assert(!info.HostConfig.Privileged && info.HostConfig.CapDrop.includes("ALL") && info.HostConfig.SecurityOpt.includes("no-new-privileges:true"), "Session lost its Docker boundary");
  assert(!info.HostConfig.SecurityOpt.some(value => value.includes("unconfined")), "Default seccomp must remain enabled");
  assert(!info.Mounts.some(mount => mount.Destination.includes("docker.sock")), "A session received the engine socket");
  const ports = Object.values(info.NetworkSettings.Ports || {}).flatMap(v => v || []);
  assert(ports.length && ports.every(v => v.HostIp === "127.0.0.1"), "Session port escaped loopback");
  return info;
}
async function startSession(worktree, harness, extra = {}) {
  const value = await api("POST", "/v1/sessions", { worktreeId: worktree, harness, ...extra }, 201);
  result.sessions.push(value.id); live.add(value.id); await save();
  assert.equal(value.backend, "docker");
  await until(async () => { const current = await api("GET", `/v1/sessions/${value.id}`); assert(current.status !== "exited", "Created CLI exited before acceptance"); return current.status !== "starting" && current; }, "Session readiness");
  return value;
}
async function stopSession(id) { if (!live.has(id)) return; await api("DELETE", `/v1/sessions/${id}`); live.delete(id); }
async function attach(id, input, observed = () => false, timeout = 90000, poll) {
  const route = `/v1/sessions/${id}/attach`, { ticket } = await api("POST", "/v1/auth/socket-ticket", { path: route }); privateValues.add(ticket);
  const socket = new WebSocket(origin.replace("http:", "ws:") + route + "?ticket=" + encodeURIComponent(ticket)); socket.binaryType = "arraybuffer";
  let output = "", trusted = false, pollTimer;
  const send = text => socket.send(Buffer.concat([Buffer.from([1]), Buffer.from(text)]));
  try {
    await new Promise((resolve, reject) => {
      const timer = setTimeout(() => reject(new Error("Owned CLI terminal timed out; inspect Luci without exposing its transcript")), timeout);
      const done = fn => value => { clearTimeout(timer); fn(value); };
      socket.addEventListener("error", done(() => reject(new Error("Owned CLI attach failed"))));
      socket.addEventListener("open", () => {
        socket.send(Uint8Array.from([2, 0, 120, 0, 40])); if (input) send(input);
        if (poll) {
          let checking = false;
          pollTimer = setInterval(async () => {
            if (checking) return; checking = true;
            try { if (await poll()) done(resolve)(); } catch { done(reject)(new Error("Owned fixture observation failed")); } finally { checking = false; }
          }, 1000);
        }
      });
      socket.addEventListener("message", event => {
        const bytes = Buffer.from(event.data); if (bytes[0] !== 0) return;
        output = (output + bytes.subarray(1).toString()).slice(-65536);
        // Only the explicit trust prompt of our new disposable project is
        // acknowledged. Tool approvals, provider logins and terms are not automated.
        if (!input && !trusted && /Do you trust (?:the files in )?this (?:directory|folder|workspace)|Trust this (?:directory|folder|workspace)/i.test(output)) { trusted = true; send("\r"); }
        if (observed(output)) done(resolve)();
      });
      socket.addEventListener("close", done(() => reject(new Error("Owned CLI exited before terminal observation"))));
    });
  } finally { clearInterval(pollTimer); output = ""; socket.close(); }
}
async function shellCommand(id, code) {
  const suffix = randomBytes(12).toString("hex"), marker = "LUCI_DONE_" + suffix;
  const command = `node -e "eval(Buffer.from('${Buffer.from(code).toString("base64")}','base64').toString())" && printf '\\nLUCI_DONE_%s\\n' '${suffix}'\r`;
  await attach(id, command, output => output.includes(marker), 45000);
}
async function publicSession(id) { const s = await api("GET", `/v1/sessions/${id}`); return { id: s.id, harness: s.harness, model: s.model, permissionMode: s.permissionMode, authMode: s.authMode, agentConnectionId: s.agentConnectionId, agentConnectionRevision: s.agentConnectionRevision, agentConnectionKind: s.agentConnectionKind, agentConnectionProtocol: s.agentConnectionProtocol, backend: s.backend }; }

try {
  await step("pin matching installed secondary before any mutation");
  const manifestBytes = await readFile(option("--manifest")); const manifest = JSON.parse(manifestBytes);
  assert.equal(manifest.kind, "luci-docker-distribution"); assert.equal(manifest.sourceCommit, expectedSource); assert.equal(manifest.version, expectedVersion); assert.equal(manifest.sourceDirty, false);
  const [controllerInfo, engineInfo] = await Promise.all([command(["inspect", controller]).then(v => JSON.parse(v)[0]), command(["inspect", engine]).then(v => JSON.parse(v)[0])]);
  assert.equal(controllerInfo.Image, option("--controller-image")); assert.equal(engineInfo.Image, option("--engine-image"));
  assert.equal(controllerInfo.Config.Labels["ai.lcfr.luci.distribution"], "controller"); assert.equal(engineInfo.Config.Labels["ai.lcfr.luci.distribution"], "private-docker-engine");
  assert.equal(controllerInfo.HostConfig.NetworkMode, "container:" + engineInfo.Id);
  assert(engineInfo.NetworkSettings.Ports["8730/tcp"].some(v => v.HostIp === "127.0.0.1" && v.HostPort === "8750"), "Named engine is not the secondary API endpoint");
  const filePins = [];
  for (const relative of ["luci", "web/index.html", "runtime/images/sandbox/Dockerfile", "runtime/images/sandbox/entrypoint.sh"]) {
    const pin = manifest.files.find(v => v.path === "bundle/" + relative); assert(pin && /^[a-f0-9]{64}$/.test(pin.sha256), "Package lacks a required runtime hash");
    const actual = (await command(["exec", controller, "sha256sum", "/opt/luci/" + relative])).split(/\s+/)[0]; assert.equal(actual, pin.sha256, "Installed package file differs from matching release"); filePins.push({ path: relative, sha256: actual });
  }
  const status = await api("GET", "/v1/control-plane/status", undefined, 200, null); assert.equal(status.version, expectedVersion); assert(!status.bootstrapRequired, "Secondary instance must already have an authorized administrator");
  const imageInfo = JSON.parse(await inner(["image", "inspect", sandboxImage]))[0]; assert.equal(imageInfo.Id, sandboxImage);
  await proof("pins", { manifestSha256: hash(manifestBytes), portableArchiveSha256: manifest.portableArchiveSha256, controllerImage: controllerInfo.Image, engineImage: engineInfo.Image, sandboxImage, files: filePins, controlPlaneVersion: status.version });
  const username = process.env.LUCI_EVAL_ADMIN_USERNAME, password = process.env.LUCI_EVAL_ADMIN_PASSWORD; assert(username && password, "Explicit secondary administrator credentials are required in environment variables"); privateValues.add(password);
  const login = await api("POST", "/v1/auth/login", { username, password }, 200, null); adminToken = login.token; privateValues.add(adminToken); assert(["admin", "super-admin"].includes(login.user.role));
  const beforeUsers = (await api("GET", "/v1/admin/users", undefined, 200, adminToken)).users; result.before = { existingUserIds: beforeUsers.map(v => v.id).sort(), existingUsersDigest: hash(JSON.stringify(beforeUsers)) };
  for (let i = 0; i < 2; i++) {
    const name = "connection-eval-" + nonce.slice(0, 8) + "-" + i, pw = secret(); passwords.push(pw);
    const user = await api("POST", "/v1/admin/users", { username: name, displayName: "Disposable connection acceptance " + i, password: pw, role: "member" }, 201, adminToken);
    accountIDs.push(user.id); result.accounts.push({ id: user.id, username: name }); await save();
    const signed = await api("POST", "/v1/auth/login", { username: name, password: pw }, 200, null); privateValues.add(signed.token); if (!i) accountToken = signed.token; else observerToken = signed.token;
  }
  const setup = await api("GET", "/v1/setup/status"); assert.equal(setup.version, expectedVersion); assert(setup.access?.projectsDirManaged, "Fixture account must have its own managed project directory");
  assert((await api("GET", "/v1/projects")).projects.length === 0 && (await api("GET", "/v1/sessions")).sessions.length === 0, "Never adopt an account with existing work");
  let routes = await api("GET", "/v1/setup/agent-connections"); assert.equal(routes.connections.length, 0);
  const machine = (await api("GET", "/v1/machines")).machines.find(v => v.default); assert(machine?.backend === "docker" && machine.state === "running", "Ready Docker backing required; this runner never installs or repairs it");
  for (let i = 0; i < 2; i++) {
    const project = await api("POST", "/v1/product/projects", { name: "connection-eval-" + nonce.slice(0, 8) + "-" + i }, 201);
    const worktrees = (await api("GET", `/v1/projects/${project.id}/worktrees`)).worktrees; assert.equal(worktrees.length, 1);
    assert(project.path.startsWith(setup.config.projectsDir + "/"), "Fixture project escaped its new account root");
    const initialFiles = (await api("GET", `/v1/product/projects/${project.id}/files`)).entries.map(v => v.path);
    assert(!initialFiles.includes("app.cjs") && !initialFiles.includes("verification.json"), "New fixture project already has implementation artifacts");
    result.projects.push({ id: project.id, name: project.name, worktreeId: worktrees[0].id, initialFiles }); await save();
  }
  const project = result.projects[0], otherProject = result.projects[1];
  await step("start owned private protocol fixture");
  const fixtureArgs = ["exec", "-i", engine, "docker", "--host=unix:///run/luci-engine/docker.sock", "run", "--name", fixtureName, "--label", `${label}=${nonce}`, "--pull", "never", "--read-only", "--cap-drop", "ALL", "--security-opt", "no-new-privileges:true", "--user", "1001:1001", "--pids-limit", "64", "--memory", "192m", "--cpus", "0.5", "--network", "bridge", "-i", "--entrypoint", "node", sandboxImage, "-e", `(${fixtureProgram.toString()})()`];
  fixtureProcess = spawn("docker", fixtureArgs, { windowsHide: true, stdio: ["pipe", "ignore", "ignore"] }); fixtureProcess.on("error", () => {});
  fixtureProcess.stdin.end(JSON.stringify({ key: fixtureKey, control: controlKey, app: APP, verification: VERIFICATION, question: QUESTION }));
  const fixtureInfo = await until(async () => { try { const v = JSON.parse(await inner(["inspect", fixtureName]))[0]; if (v.Config.Labels[label] !== nonce) throw Error("fixture ownership mismatch"); fixtureCreated = true; return v.State.Running && v; } catch { return null; } }, "Fixture startup", 15000);
  assert.equal(fixtureInfo.Image, sandboxImage); assert.equal(fixtureInfo.Mounts.length, 0); assert(!Object.keys(fixtureInfo.HostConfig.PortBindings || {}).length);
  const fixtureIP = fixtureInfo.NetworkSettings.Networks.bridge.IPAddress; assert.match(fixtureIP, /^172\.(1[6-9]|2\d|3[01])\.\d{1,3}\.\d{1,3}$/);
  await until(async () => { try { return await fixtureControl(); } catch { return null; } }, "Fixture protocol readiness", 15000);
  const base = `http://${fixtureIP}:4100/v1`;
  async function connection(action, fields = {}) { routes = await api("POST", "/v1/setup/agent-connections", { action, expectedRevision: routes.revision, ...fields }); return routes; }
  await connection("save", { name: "Controlled fixture only", kind: "litellm", enabled: true, endpoints: { responses: base, anthropicMessages: base, chatCompletions: base }, models: { codex: "fixture-codex", claudeCode: "fixture-claude", opencode: "fixture-chat" }, apiKey: activeKey });
  let route = routes.connections[0]; await connection("select", { id: route.id });
  await api("POST", "/v1/setup/agent-connections", { action: "select", expectedRevision: routes.revision - 1, id: route.id }, 409);
  await api("POST", "/v1/sessions", { worktreeId: project.worktreeId, harness: "shell", agentConnectionId: route.id }, 409);
  await connection("save", { id: route.id, name: route.name, kind: route.kind, endpoints: route.endpoints, models: route.models, enabled: false });
  await api("POST", "/v1/sessions", { worktreeId: project.worktreeId, harness: "codex", agentConnectionId: route.id }, 409);
  await connection("save", { id: route.id, name: route.name, kind: route.kind, enabled: true, endpoints: { responses: base }, models: { codex: "fixture-codex" } });
  await api("POST", "/v1/sessions", { worktreeId: project.worktreeId, harness: "claude-code", agentConnectionId: route.id }, 409);
  await connection("select", { id: "" });
  await api("POST", `/v1/product/projects/${project.id}/assist`, { expectedRevision: 0, message: "Fixture gateway off must not fall back", harness: "codex", model: "fixture-codex" }, 409);
  await connection("save", { id: route.id, name: route.name, kind: route.kind, enabled: true, endpoints: { responses: base, anthropicMessages: base, chatCompletions: base }, models: { codex: "fixture-codex", claudeCode: "fixture-claude", opencode: "fixture-chat" } });
  route = routes.connections.find(v => v.id === route.id); await connection("select", { id: route.id });
  assert.equal((await fixtureControl()).records.length, 0, "Rejected route operations contacted a provider");
  await proof("routeControls", { save: true, select: true, edit: true, disabledRejected: true, protocolMismatchRejected: true, shellCredentialOptInRejected: true, staleRevisionRejected: true, offAssistantRejectedWithoutFallback: true });

  await step("real Codex default Ask tool round trip");
  await fixtureControl({ phase: "codex-code", model: "fixture-codex", key: activeKey });
  const coding = await startSession(project.worktreeId, "codex", { openingPrompt: "Execute the controlled local fixture tool call to create app.cjs and verification.json. This is a disposable connection acceptance project." });
  const codingInfo = await sessionContainer(coding.id);
  const codexArgs = JSON.parse(codingInfo.Config.Env.find(v => v.startsWith("LUCI_ARGS_JSON=")).slice("LUCI_ARGS_JSON=".length));
  assertPublic(codexArgs);
  assert(codexArgs.includes("danger-full-access") && codexArgs.includes("on-request") && !codexArgs.includes("--dangerously-bypass-approvals-and-sandbox"), "Default Ask argv lost the approved Docker policy");
  const codingReceipt = await publicSession(coding.id); assert.equal(codingReceipt.permissionMode, "ask"); assert.equal(codingReceipt.model, "fixture-codex"); assert.equal(codingReceipt.agentConnectionId, route.id);
  await attach(coding.id, null, () => false, 90000, async () => (await fixtureControl()).state.toolResult);
  const traffic = await fixtureControl(); assert.equal(traffic.state.calls, 1); assert(traffic.state.toolResult, "Actual CLI did not return a tool result");
  result.producedFiles = [];
  for (const [name, expected] of [["app.cjs", APP], ["verification.json", VERIFICATION]]) {
    const file = await api("GET", `/v1/product/projects/${project.id}/files?path=${name}`, undefined, [200, 403, 404]);
    const present = typeof file?.content === "string", matches = present && !file.truncated && file.content === expected;
    result.producedFiles.push({ name, present, exactBytesMatch: matches, ...(present ? { bytes: Buffer.byteLength(file.content), sha256: hash(file.content) } : {}) }); await save();
    assert(matches, "Actual CLI did not create the exact expected artifact bytes");
  }
  const verify = `const fs=require('fs'),cp=require('child_process'),crypto=require('crypto');const app=fs.readFileSync('app.cjs','utf8'),verification=fs.readFileSync('verification.json','utf8');if(app!==${JSON.stringify(APP)}||verification!==${JSON.stringify(VERIFICATION)})throw Error('artifact bytes mismatch');const p=cp.spawn('node',['app.cjs'],{stdio:['ignore','pipe','ignore']});const timer=setTimeout(()=>{p.kill();process.exit(2)},8000);p.stdout.once('data',async b=>{try{const port=Number(String(b).trim());if(!Number.isInteger(port))throw Error();const r=await fetch('http://127.0.0.1:'+port+'/health');const body=await r.text();if(r.status!==200||body!==JSON.stringify({status:'ok',createdBy:'actual-cli-tool-fixture'}))throw Error();const status=fs.readFileSync('/proc/self/status','utf8');let denied=false;try{fs.writeFileSync('/etc/luci-fixture-denied','x')}catch{denied=true}const proof={appBase64:Buffer.from(app).toString('base64'),verificationBase64:Buffer.from(verification).toString('base64'),appSha256:crypto.createHash('sha256').update(app).digest('hex'),httpStatus:r.status,httpBody:body,uid:process.getuid(),nonroot:process.getuid()!==0,noNewPrivileges:/NoNewPrivs:\\s+1/.test(status),capabilitiesDropped:/CapEff:\\s+0+\\n/.test(status),systemWriteDenied:denied,dockerSocketAbsent:!fs.existsSync('/var/run/docker.sock')&&!fs.existsSync('/run/luci-engine/docker.sock')};if(!proof.nonroot||!proof.noNewPrivileges||!proof.capabilitiesDropped||!denied||!proof.dockerSocketAbsent)throw Error();console.log(JSON.stringify(proof));}catch{process.exitCode=2}finally{clearTimeout(timer);p.kill()}});`;
  const runtimeGuard = `{const fs=require('fs');const s=fs.readFileSync('/proc/1/status','utf8');if(!/^Uid:\\s+1001\\s+1001\\s+1001\\s+1001$/m.test(s)||!/^NoNewPrivs:\\s+1$/m.test(s)||!/^CapEff:\\s+0+$/m.test(s))throw Error('runtime process boundary');}`;
  const artifact = assertPublic(JSON.parse(await inner(["exec", "--user", "1001:1001", "--workdir", "/home/dev/work", codingInfo.Id, "node", "-e", runtimeGuard + verify])));
  const cliVersions = {};
  for (const [binary, expected] of [["codex", "0.160.0"], ["claude", "2.1.274"], ["opencode", "1.18.34"]]) {
    const actual = await inner(["exec", "--user", "1001:1001", codingInfo.Id, binary, "--version"]); assert(actual.includes(expected), "Installed CLI version differs from pinned fixture contract"); cliVersions[binary] = actual;
  }
  await proof("codexToolWrite", { session: codingReceipt, boundary: artifact, cliVersions, fixtureToolCalls: traffic.state.calls, actualToolResult: true, permissionMode: "ask", cliExitIsNotSuccessEvidence: true });
  await stopSession(coding.id);

  await step("rotate the saved key and retain exact route on restart");
  activeKey = secret(); await connection("save", { id: route.id, name: route.name, kind: route.kind, enabled: true, endpoints: route.endpoints, models: { ...route.models, codex: "fixture-codex" }, apiKey: activeKey });
  route = routes.connections.find(v => v.id === route.id);
  await fixtureControl({ phase: "codex-restart", model: "fixture-codex", key: activeKey });
  await api("POST", `/v1/sessions/${coding.id}/restart`, {}, 200); live.add(coding.id);
  await attach(coding.id, null, () => false, 90000, async () => (await fixtureControl()).records.some(v => v.phase === "codex-restart" && v.authorized));
  const restarted = await publicSession(coding.id); assert.equal(restarted.agentConnectionId, route.id); assert.equal(restarted.agentConnectionRevision, route.revision);
  await sessionContainer(coding.id); await stopSession(coding.id); result.proofs.routeControls.rotatedKeyAndRestartReceipt = true;

  async function assistant(provider, kind) {
    const model = { codex: "fixture-codex", "claude-code": "fixture-claude", opencode: "fixture-chat" }[provider];
    await fixtureControl({ phase: provider + "-" + kind, model, key: activeKey });
    const spec = await api("GET", `/v1/product/projects/${project.id}/specification`);
    const body = { expectedRevision: spec.revision, message: "Return the deterministic fixture question using this saved connection.", harness: provider, model };
    if (kind === "repository") Object.assign(body, { expectedBriefRevision: spec.briefRevision, sourceMode: "worktree", acknowledgeSourceRead: true });
    const job = await api("POST", `/v1/product/projects/${project.id}/assist` + (kind === "repository" ? "/repository" : ""), body, [200, 202]);
    result.sessions.push(job.sessionId); live.add(job.sessionId); await save();
    const ended = await until(async () => { const value = await api("GET", `/v1/product/projects/${project.id}/assist/${job.id}`); return ["succeeded", "failed", "cancelled"].includes(value.status) && value; }, provider + " " + kind, 180000);
    assert.equal(ended.status, "succeeded", "Read-only helper failed; fixture cannot stand in for its result"); assert.equal(ended.result.question, QUESTION.question); assert.equal(ended.agentConnectionId, route.id); assert.equal(ended.agentConnectionRevision, route.revision);
    assert.equal((await api("GET", `/v1/product/projects/${project.id}/specification`)).revision, spec.revision, "Assistant applied a draft without user action");
    if (kind === "repository") assert((await fixtureControl()).records.some(v => v.phase === provider + "-repository" && v.sourcePresent), "Repository helper did not send the actual CLI-authored source snapshot");
    await stopSession(job.sessionId);
    return { jobId: job.id, harness: provider, model: ended.model, authMode: ended.authMode, agentConnectionId: ended.agentConnectionId, agentConnectionRevision: ended.agentConnectionRevision, protocol: ended.agentConnectionProtocol, status: ended.status, sourceDigest: ended.source?.digest, resultApplied: false };
  }
  await proof("codexDefine", await assistant("codex", "define")); await proof("codexRepository", await assistant("codex", "repository"));
  result.additionalProtocols = {};
  for (const provider of protocols.slice(1)) result.additionalProtocols[provider] = { define: await assistant(provider, "define"), repository: await assistant(provider, "repository"), scope: "Read-only helper protocol path, not interactive coding-tool proof" };

  await step("real terminal file editing, private tool installation and restart isolation");
  const firstShell = await startSession(project.worktreeId, "shell");
  await shellCommand(firstShell.id, `const fs=require('fs'),os=require('os');if(process.env.OPENAI_API_KEY||process.env.ANTHROPIC_API_KEY||process.env.ANTHROPIC_AUTH_TOKEN||process.env.LUCI_GATEWAY_API_KEY)throw Error('unexpected model credential');if(os.homedir()!=='/home/dev/.luci-shell')throw Error('private home missing');fs.writeFileSync(os.homedir()+'/.local/bin/fixture-tool','#!/bin/sh\\nprintf retained-tool-bytes',{mode:448});fs.writeFileSync('shell-created.txt','first');fs.writeFileSync('shell-created.txt','edited');fs.writeFileSync('shell-deleted.txt','remove');fs.unlinkSync('shell-deleted.txt');`);
  const firstShellInfo = await sessionContainer(firstShell.id), firstMount = firstShellInfo.Mounts.find(v => v.Destination === "/home/dev/.luci-shell"); assert(firstMount?.RW);
  await stopSession(firstShell.id);
  const otherShell = await startSession(otherProject.worktreeId, "shell");
  await shellCommand(otherShell.id, `const fs=require('fs'),os=require('os');if(fs.existsSync(os.homedir()+'/.local/bin/fixture-tool')||fs.existsSync('shell-created.txt')||fs.existsSync('app.cjs'))throw Error('project/session state leaked');`);
  const otherInfo = await sessionContainer(otherShell.id); assert.notEqual(otherInfo.Mounts.find(v => v.Destination === "/home/dev/.luci-shell")?.Source, firstMount.Source); await stopSession(otherShell.id);
  await api("POST", `/v1/sessions/${firstShell.id}/restart`, {}); live.add(firstShell.id);
  await shellCommand(firstShell.id, `const fs=require('fs'),cp=require('child_process');if(cp.execFileSync('fixture-tool',{encoding:'utf8'})!=='retained-tool-bytes'||fs.readFileSync('shell-created.txt','utf8')!=='edited'||fs.existsSync('shell-deleted.txt'))throw Error('terminal restart lost bytes');`);
  const lastShellInfo = await sessionContainer(firstShell.id); assert.equal(lastShellInfo.Mounts.find(v => v.Destination === "/home/dev/.luci-shell")?.Source, firstMount.Source);
  await stopSession(firstShell.id); await proof("shellPersistence", { restartRetainedToolBytes: true, installedToolOnPATH: true, createEditDelete: true, otherProjectAndSessionIsolated: true, noProviderCredentials: true, killRetains: true });

  await api("GET", `/v1/sessions/${coding.id}`, undefined, 404, observerToken);
  const observerConnections = await api("GET", "/v1/setup/agent-connections", undefined, 200, observerToken); assert.equal(observerConnections.connections.length, 0);
  await api("POST", "/v1/setup/agent-connections", { action: "select", expectedRevision: observerConnections.revision, id: route.id }, 409, observerToken);
  await api("POST", `/v1/product/projects/${project.id}/assist`, { expectedRevision: 0, harness: "codex", model: "fixture-codex", agentConnectionId: route.id, message: "Foreign route must fail" }, 404, observerToken);
  await proof("accountIsolation", { foreignSessionNotVisible: true, foreignProjectNotVisible: true, connectionStoreSeparate: true, foreignConnectionSelectionRejected: true });
  const requests = (await fixtureControl()).records; assert(requests.length > 0 && requests.every(v => v.authorized), "A fixture request used a removed/wrong key"); result.fixtureRequests = requests;
  const publicRecords = await Promise.all([api("GET", "/v1/sessions"), api("GET", "/v1/setup/agent-connections"), api("GET", `/v1/product/projects/${project.id}/assist`), api("GET", "/v1/admin/audit", undefined, 200, adminToken)]); publicRecords.forEach(assertPublic);
  await proof("secretFreeRecords", { sessionDTOs: true, connectionDTOs: true, helperJobDTOs: true, audit: true, receipt: true, requests: requests.length });
  await connection("delete", { id: route.id }); assert.equal(routes.connections.length, 0); assert.equal(routes.activeConnectionId, "");
  result.status = "checks-passed-cleanup-pending";
} catch (error) {
  result.status = "failed"; result.failure = redact(error);
} finally {
  const cleanup = { stoppedOwnedSessions: [], disabledOwnedAccounts: [], fixtureRemoved: false, errors: [] };
  for (const id of [...live]) { try { await stopSession(id); cleanup.stoppedOwnedSessions.push(id); } catch { cleanup.errors.push("could not stop an owned session"); } }
  if (fixtureProcess) {
    try { const info = JSON.parse(await inner(["inspect", fixtureName]))[0]; assert.equal(info.Config.Labels[label], nonce); assert.equal(info.Image, sandboxImage); try { result.fixtureRequests = (await fixtureControl()).records; } catch { result.fixtureRequestsUnavailable = true; } await inner(["rm", "--force", info.Id]); cleanup.fixtureRemoved = true; } catch { cleanup.errors.push("owned fixture cleanup could not be confirmed"); }
  }
  fixtureProcess?.kill();
  for (const id of accountIDs) { try { await api("PATCH", `/v1/admin/users/${id}`, { disabled: true }, 200, adminToken); cleanup.disabledOwnedAccounts.push(id); } catch { cleanup.errors.push("could not disable an owned fixture account"); } }
  if (adminToken && result.before) {
    try { const users = (await api("GET", "/v1/admin/users", undefined, 200, adminToken)).users.filter(v => !accountIDs.includes(v.id)); result.after = { existingUserIds: users.map(v => v.id).sort(), existingUsersDigest: hash(JSON.stringify(users)) }; assert.equal(result.after.existingUsersDigest, result.before.existingUsersDigest, "Existing user records changed"); } catch { cleanup.errors.push("existing account preservation was not confirmed"); }
  }
  if (adminToken) { try { await api("POST", "/v1/auth/logout", {}, 200, adminToken); } catch { cleanup.errors.push("administrator eval session logout failed"); } }
  result.proofs.cleanup = { passed: cleanup.errors.length === 0 && fixtureCreated && cleanup.fixtureRemoved && cleanup.disabledOwnedAccounts.length === 2, ...cleanup };
  result.passed = complete(result); if (result.passed) result.status = "passed"; else if (result.status !== "failed") result.status = "incomplete";
  result.finishedAt = new Date().toISOString(); await save();
}
console.log(JSON.stringify({ status: result.status, passed: result.passed, fixture: true, paidInference: false, evidence: path.join(evidence, "results.json"), failure: result.failure }));
process.exitCode = result.passed ? 0 : 1;

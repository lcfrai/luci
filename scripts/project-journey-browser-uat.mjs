#!/usr/bin/env node
// Browser continuity + actual CLI/Docker execution against a local deterministic
// provider. The fixture supplies implementation, so this is NOT model-quality,
// novice-usability, live-provider, public-deployment, or production acceptance.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { createServer as httpServer } from 'node:http';
import { createServer } from 'node:net';
import { createWriteStream, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, readlinkSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '../desktop/node_modules/playwright/index.mjs';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..'), args = process.argv.slice(2);
const option = (name, fallback) => { const i = args.indexOf(name); if (i < 0) return fallback; assert(args[i + 1] && !args[i + 1].startsWith('--'), 'Missing value for ' + name); return args[i + 1]; };
if (args.includes('--help')) {
  console.log('node scripts/project-journey-browser-uat.mjs --binary PATH --web PATH --image EXISTING_TRUSTED_IMAGE [--release-image EXISTING_NODE_IMAGE] [--chromium PATH] [--report PATH] [--keep-fixture]\nOne fresh browser journey with actual Codex/Docker execution and a deterministic provider: review, broken implementation, failed acceptance, agent repair, checks, preview, exact-run acceptance. No paid inference, cloud work or image pull. Requires an exclusive runtime window. Does not prove model quality or full production readiness.');
  process.exit(0);
}
const binary = resolve(option('--binary', 'artifacts/luci')), web = resolve(option('--web', 'desktop/dist'));
const image = option('--image'), releaseImage = option('--release-image', image);
assert(image, 'Supply an existing trusted --image'); assert.equal(process.platform, 'linux'); assert.notEqual(process.getuid(), 0);
assert(existsSync(binary)); assert(existsSync(join(web, 'index.html')));
const docker = (...values) => execFileSync('docker', values, { encoding: 'utf8', timeout: 30_000 }).trim();
const imageInfo = JSON.parse(docker('image', 'inspect', image))[0], releaseImageInfo = JSON.parse(docker('image', 'inspect', releaseImage))[0];
assert.match(imageInfo.Id, /^sha256:[a-f0-9]{64}$/); assert.match(releaseImageInfo.Id, /^sha256:[a-f0-9]{64}$/);
const bridge = docker('network', 'inspect', 'bridge', '--format', '{{(index .IPAM.Config 0).Gateway}}');
assert.match(bridge, /^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.)/);
const report = resolve(option('--report', 'artifacts/uat/project-journey-' + Date.now() + '/result.json')), evidence = dirname(report);
mkdirSync(evidence, { recursive: true, mode: 0o700 });
const fixture = mkdtempSync('/tmp/luci-project-journey-');
for (const folder of ['home', 'projects', 'state', 'docker']) mkdirSync(join(fixture, folder), { mode: 0o700 });
const sha = bytes => createHash('sha256').update(bytes).digest('hex'), key = 'fixture-only-' + randomUUID(), lifecycle = randomUUID() + randomUUID(), password = randomUUID() + randomUUID();
const sensitive = [key, lifecycle, password], scrub = value => sensitive.reduce((text, secret) => text.replaceAll(secret, '[redacted]'), String(value));
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repo, encoding: 'utf8' }).trim(),
  sourceDirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: repo, encoding: 'utf8' }).trim(),
  binarySHA256: sha(readFileSync(binary)), webIndexSHA256: sha(readFileSync(join(web, 'index.html'))), driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  image: { reference: image, id: imageInfo.Id }, releaseImage: { reference: releaseImage, id: releaseImageInfo.Id },
  scope: 'UI continuity and real isolated execution only. Local deterministic provider supplies an intentionally broken app then repairs app behavior without changing its acceptance test.',
  limitations: ['Existing trusted Docker images; installation is not exercised.', 'No live model-quality, public deployment, novice walkthrough, application authentication, durable application data, cloud, or paid-service claim.'],
  externalModelCalls: 0, cloudResourcesCreated: 0, checks: [], browserMutations: [], cleanup: [],
};
const check = (name, details = {}) => { receipt.checks.push({ name, passed: true, ...details }); console.log('PASS ' + name); };
const sessions = new Set(), releases = new Set(), browserErrors = [];
let service, browser, page, base, token, project, projectRoute, run, immutableInput, stage = 'startup', spawnError, providerError;
let providerCalls = 0, toolRound = 0, completedRounds = 0, expectedContext = {}, expectedPrompt = [];
const terminalTextBySession = new Map();
const pageHTML = '<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Garden notes</title><style>body{font:18px system-ui;max-width:42rem;margin:3rem auto;padding:1rem}input,button{font:inherit;padding:.6rem}label{display:block}li{padding:.5rem}</style></head><body><h1>Garden notes</h1><p>Save an observation for this running preview.</p><label for="note">Garden note</label><input id="note"><button id="save">Save note</button><p role="status" id="status"></p><ul id="notes"></ul><script>async function refresh(){const r=await fetch("/notes");const notes=await r.json();document.querySelector("#notes").replaceChildren(...notes.map(text=>{const li=document.createElement("li");li.textContent=text;return li;}));}document.querySelector("#save").onclick=async()=>{const input=document.querySelector("#note");const r=await fetch("/notes",{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({note:input.value})});if(!r.ok){document.querySelector("#status").textContent="Could not save the note";return;}input.value="";await refresh();document.querySelector("#status").textContent="Saved";};refresh();</script></body></html>';
const appSource = [
  "const http = require('node:http');", 'const saveEnabled = false;', 'const html = ' + JSON.stringify(pageHTML) + ';',
  'function createApp() { const notes = []; return http.createServer(async (req, res) => {',
  "if (req.url === '/health') { res.writeHead(200, {'content-type':'application/json'}); res.end(JSON.stringify({status:'ok'})); return; }",
  "if (req.url === '/notes') { if (req.method === 'POST') { let raw=''; for await (const part of req) raw += part; const {note}=JSON.parse(raw); if (saveEnabled && typeof note === 'string' && note.trim()) notes.push(note.trim()); } res.writeHead(200, {'content-type':'application/json'}); res.end(JSON.stringify(notes)); return; }",
  "res.writeHead(200, {'content-type':'text/html'}); res.end(html); }); }",
  "if (require.main === module) createApp().listen(3000, '0.0.0.0');", 'module.exports = { createApp };', '',
].join('\n');
const testSource = [
  "const { test } = require('node:test'); const assert = require('node:assert/strict'); const { once } = require('node:events'); const { createApp } = require('./app.cjs');",
  "test('Saving a garden note returns it on the next read', async () => { const server=createApp().listen(0,'127.0.0.1'); await once(server,'listening'); const base='http://127.0.0.1:'+server.address().port;",
  "try { const saved=await fetch(base+'/notes',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({note:'Inspect the new seedlings'})}); assert.equal(saved.status,200); const list=await fetch(base+'/notes').then(r=>r.json()); assert.deepEqual(list,['Inspect the new seedlings'],'The saved note must be readable; a healthy process alone is insufficient'); } finally { await new Promise(done=>server.close(done)); } });", '',
].join('\n');
function sendResponse(res, item) {
  const response = { id: 'resp_journey_' + providerCalls, object: 'response', created_at: 1, status: 'completed', model: 'journey-fixture', output: [item], usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2, input_tokens_details: { cached_tokens: 0 }, output_tokens_details: { reasoning_tokens: 0 } } };
  const events = [{ type: 'response.created', response: { ...response, status: 'in_progress', output: [] } }, { type: 'response.output_item.added', output_index: 0, item: { ...item, status: 'in_progress', ...(item.type === 'function_call' ? { arguments: '' } : { content: [] }) } }];
  if (item.type === 'function_call') events.push({ type: 'response.function_call_arguments.delta', item_id: item.id, output_index: 0, delta: item.arguments }, { type: 'response.function_call_arguments.done', item_id: item.id, output_index: 0, arguments: item.arguments });
  else events.push({ type: 'response.content_part.added', item_id: item.id, output_index: 0, content_index: 0, part: { type: 'output_text', text: '', annotations: [] } }, { type: 'response.output_text.delta', item_id: item.id, output_index: 0, content_index: 0, delta: item.content[0].text }, { type: 'response.output_text.done', item_id: item.id, output_index: 0, content_index: 0, text: item.content[0].text });
  events.push({ type: 'response.output_item.done', output_index: 0, item }, { type: 'response.completed', response });
  res.writeHead(200, { 'content-type': 'text/event-stream' }); res.end(events.map((event, sequence_number) => 'event: ' + event.type + '\ndata: ' + JSON.stringify({ ...event, sequence_number }) + '\n\n').join(''));
}
const provider = httpServer(async (req, res) => {
  try {
    assert.equal(req.method, 'POST'); assert.equal(req.url, '/v1/responses'); assert.equal(req.headers.authorization, 'Bearer ' + key);
    const parts = []; let size = 0; for await (const part of req) { size += part.length; assert(size < 4 * 1024 * 1024); parts.push(part); }
    const body = JSON.parse(Buffer.concat(parts).toString()); providerCalls++;
    assert.equal(body.model, 'journey-fixture');
    const serialized = JSON.stringify(body), tools = (body.tools ?? []).map(item => item.name ?? item.function?.name ?? item.type);
    const coding = tools.includes('exec_command'), hasToolResult = serialized.includes('function_call_output');
    if (coding) for (const text of expectedPrompt) assert(serialized.includes(text), 'CLI omitted reviewed task context: ' + text);
    if (coding && !hasToolResult) {
      assert(toolRound < 2, 'Unexpected third implementation attempt'); toolRound++;
      const prefix = "const fs=require('node:fs'),crypto=require('node:crypto'),assert=require('node:assert/strict');const expected=" + JSON.stringify(expectedContext) + ";for(const [p,d] of Object.entries(expected))assert.equal(crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex'),d,'Reviewed context drift: '+p);";
      const payload = toolRound === 1 ? prefix
        + 'fs.writeFileSync("app.cjs",' + JSON.stringify(appSource) + ');fs.writeFileSync("acceptance.cjs",' + JSON.stringify(testSource) + ');'
        + 'fs.writeFileSync("package.json",' + JSON.stringify(JSON.stringify({ name: 'garden-notes-continuity', private: true, scripts: { test: 'node --test acceptance.cjs' } })) + ');'
        + 'fs.writeFileSync("Dockerfile",' + JSON.stringify('FROM ' + releaseImage + '\nWORKDIR /app\nCOPY app.cjs /app/app.cjs\nUSER 1000:1000\nEXPOSE 3000\nENTRYPOINT ["node","/app/app.cjs"]\n') + ');'
        : prefix + 'assert.equal(crypto.createHash("sha256").update(fs.readFileSync("acceptance.cjs")).digest("hex"),' + JSON.stringify(sha(testSource)) + ');const source=fs.readFileSync("app.cjs","utf8");assert(source.includes("const saveEnabled = false;"));fs.writeFileSync("app.cjs",source.replace("const saveEnabled = false;","const saveEnabled = true;"));';
      sendResponse(res, { id: 'fc_journey_' + toolRound, type: 'function_call', status: 'completed', name: 'exec_command', call_id: 'call_journey_' + toolRound, arguments: JSON.stringify({ cmd: 'node -e "eval(Buffer.from(\'' + Buffer.from(payload).toString('base64') + '\',\'base64\').toString())"', workdir: '/home/dev/work', login: false, max_output_tokens: 1000 }) });
    } else {
      if (coding) { assert(hasToolResult); completedRounds = toolRound; }
      sendResponse(res, { id: 'msg_journey_' + providerCalls, type: 'message', status: 'completed', role: 'assistant', content: [{ type: 'output_text', text: toolRound === 2 ? 'Repaired saving notes. The acceptance test is unchanged. Run Luci verification again.' : 'Created the local continuity fixture. Run independent verification before accepting its behavior.', annotations: [] }] });
    }
  } catch (error) { providerError = error; if (!res.headersSent) res.writeHead(500); res.end('Local fixture contract failed'); }
});
async function freePort() { const listener = createServer(); await new Promise((done, reject) => listener.once('error', reject).listen(0, '127.0.0.1', done)); const port = listener.address().port; await new Promise(done => listener.close(done)); return port; }
function processIdentity(pid) {
  try {
    const stat = readFileSync(`/proc/${pid}/stat`, 'utf8');
    const fields = stat.slice(stat.lastIndexOf(')') + 2).trim().split(/\s+/);
    return { pid: Number(pid), start: fields[19], group: Number(fields[2]), status: fields[0] };
  } catch (error) { if (['ENOENT', 'ESRCH'].includes(error.code)) return null; throw error; }
}
function stillOwned(identity) { const now = processIdentity(identity.pid); return now && now.start === identity.start && now.status !== 'Z'; }
function ownedProcesses() {
  const found = [], executable = realpathSync(binary), state = join(fixture, 'state');
  for (const pid of readdirSync('/proc').filter(value => /^\d+$/.test(value))) {
    const identity = processIdentity(pid); if (!identity || identity.status === 'Z') continue;
    try {
      if (readlinkSync(`/proc/${pid}/exe`) !== executable) continue;
      const values = Object.fromEntries(readFileSync(`/proc/${pid}/environ`, 'utf8').split('\0').filter(Boolean).map(value => { const i = value.indexOf('='); return [value.slice(0, i), value.slice(i + 1)]; }));
      if ((values.LUCI_STATE_DIR === state || values.LUCI_STATE_DIR?.startsWith(state + '/control-plane/')) && processIdentity(pid)?.start === identity.start) found.push(identity);
    } catch (error) { if (!['ENOENT', 'ESRCH', 'EACCES', 'EPERM'].includes(error.code)) throw error; }
  }
  return found;
}
async function drainOwnedService() {
  if (!service?.pid) return true;
  const tracked = ownedProcesses();
  if (service.exitCode === null) {
    assert(tracked.some(row => row.pid === service.pid), 'Cannot verify fixture gateway ownership');
    service.kill('SIGTERM');
  }
  // Workers use separate process groups; the gateway group alone is not proof
  // that private state has no remaining writer.
  const deadline = Date.now() + 20_000;
  while (Date.now() < deadline && (tracked.some(stillOwned) || ownedProcesses().length)) await delay(100);
  const remaining = [...tracked.filter(stillOwned), ...ownedProcesses()];
  if (remaining.length) {
    receipt.cleanup.push('Owned gateway or worker failed to drain; force-stopped verified fixture processes'); receipt.passed = false; process.exitCode = 1;
    for (const row of remaining) if (stillOwned(row)) {
      try { process.kill(row.group === row.pid ? -row.pid : row.pid, 'SIGKILL'); } catch (error) { if (error.code !== 'ESRCH') throw error; }
    }
    const killedDeadline = Date.now() + 5_000;
    while (Date.now() < killedDeadline && remaining.some(stillOwned)) await delay(100);
  }
  return !tracked.some(stillOwned) && ownedProcesses().length === 0;
}
async function until(label, fn, timeout = 120_000) {
  const end = Date.now() + timeout; let last;
  while (Date.now() < end) { if (spawnError) throw spawnError; if (providerError) throw providerError; if (service && service.exitCode !== null) throw Error('Service exited ' + service.exitCode); try { const value = await fn(); if (value) return value; } catch (error) { if (error.fatal || error.code === 'ERR_ASSERTION' || error.message?.includes('strict mode violation')) throw error; last = error; } await delay(250); }
  throw Error(label + ' timed out' + (last ? ': ' + scrub(last.message) : ''));
}
// Observations only. Browser UI owns all user-facing mutations.
async function observe(path) { const response = await fetch(base + path, { headers: token ? { Authorization: 'Bearer ' + token } : {}, redirect: 'error', signal: AbortSignal.timeout(15_000) }); assert.equal(response.status, 200, 'Read-only observation ' + path); return response.json(); }
async function mutation(action, suffix, method = 'POST', timeout = 120_000) {
  const pending = page.waitForResponse(response => new URL(response.url()).origin === base && new URL(response.url()).pathname.endsWith(suffix) && response.request().method() === method, { timeout });
  await action(); const response = await pending; const data = await response.json();
  assert(response.ok(), 'Visible action failed: ' + suffix + ' ' + response.status() + ' ' + scrub(data.message ?? data.error ?? '')); return data;
}
const runPanel = () => page.getByRole('region', { name: 'Project runs', exact: true });
async function screenshot(name) { await page.screenshot({ path: join(evidence, name + '.png'), fullPage: true, mask: [page.locator('.xterm-screen'), page.locator('input[type=password]')] }); }
async function lens(name) { await page.getByRole('button', { name, exact: true }).click(); }
async function commit(message) {
  await lens('Git'); await page.getByLabel('Commit message', { exact: true }).fill(message);
  await mutation(() => page.getByRole('button', { name: 'Commit changes', exact: true }).click(), projectRoute + '/git/commit');
  const git = await observe(projectRoute + '/git'); assert(git.clean); return git.log[0].hash;
}
async function reconcile() { return (await mutation(() => runPanel().getByRole('button', { name: 'Reconcile run', exact: true }).click(), '/runs/' + run.id + '/reconcile')).run; }
async function codingAttempt(label, round) {
  stage = round === 1 ? 'initial-coding' : 'repair-coding';
  const session = await mutation(() => runPanel().getByRole('button', { name: label, exact: true }).click(), '/v1/sessions', 'POST', 180_000);
  sessions.add(session.id); assert.equal(session.projectRunId, run.id); assert.equal(session.agentConnectionId, run.input.route.connection.id); assert.equal(session.model, 'journey-fixture');
  await runPanel().getByRole('button', { name: 'Open coding session', exact: true }).click();
  const pane = page.locator('section.pane[data-session-id=' + JSON.stringify(session.id) + ']');
  await pane.waitFor(); assert.equal(await pane.count(), 1);
  await until('Created coding session receives focus', async () => (await pane.getAttribute('class'))?.split(/\s+/).includes('pane--focused'), 5_000);
  assert.equal(await pane.locator('.xterm-helper-textarea').count(), 1);
  const terminalBounds = await until('Open coding session provides a readable terminal', async () => {
    const bounds = await pane.locator('.pane-body').boundingBox();
    return bounds && bounds.height >= 240 && bounds.width >= 360 && bounds;
  }, 5_000);
  (receipt.codingPanes ??= []).push({ round, sessionID: session.id, terminalBounds });
  let trustAccepted = false;
  await until('Actual fixture coding round ' + round, async () => {
    const terminalText = terminalTextBySession.get(session.id) ?? '';
    if (!trustAccepted && terminalText.includes('Trust and continue')) {
      assert(terminalText.includes('/home/dev/work'), 'Unexpected CLI trust target');
      assert((await pane.getAttribute('class')).split(/\s+/).includes('pane--focused'), 'Another session received focus before folder approval');
      const rendered = await pane.innerText();
      if (!['Trust this folder?', '/home/dev/work', 'Trust and continue'].every(text => rendered.includes(text))) return false;
      const terminal = pane.locator('.xterm-helper-textarea'); await terminal.focus();
      assert(await terminal.evaluate(element => document.activeElement === element), 'The current session must own keyboard focus');
      await terminal.press('Enter'); trustAccepted = true;
      (receipt.folderTrustApprovals ??= []).push({ round, sessionID: session.id, path: '/home/dev/work', visiblePrompt: true });
    }
    return completedRounds >= round;
  }, 180_000);
  assert.equal(sha(readFileSync(join(project.path, 'acceptance.cjs'))), sha(testSource), 'Agent changed independent fixture assertion');
  await pane.getByRole('button', { name: /^Session menu for / }).click();
  await mutation(() => page.getByRole('menuitem', { name: 'Kill', exact: true }).click(), '/v1/sessions/' + session.id, 'DELETE');
  assert.equal(docker('ps', '-a', '--filter', 'label=luci.session-id=' + session.id, '--format', '{{.ID}}'), '', 'Visible Kill action must remove this sandbox before another attempt');
  await lens('Develop'); run = await reconcile(); assert.deepEqual(run.input, immutableInput);
  return session;
}
async function verification(expectedStatus) {
  stage = 'verification-' + expectedStatus;
  await runPanel().getByRole('button', { name: 'Verify this build', exact: true }).click();
  const panel = page.locator('.project-checks');
  await panel.getByRole('heading', { name: 'Verify this build', exact: true }).waitFor();
  // Select the exact reviewed criterion explicitly, then review the command.
  const requirements = panel.locator('.check-requirements');
  if (await requirements.count() && !await requirements.evaluate(element => element.open)) await requirements.locator('summary').click();
  for (const box of await requirements.getByRole('checkbox').all()) await box.check();
  const advanced = panel.locator('details').filter({ has: page.locator('summary').filter({ hasText: 'Advanced check commands' }) });
  if (await advanced.count() && !await advanced.evaluate(element => element.open)) await advanced.locator('summary').click();
  for (const remove of await panel.getByRole('button', { name: /^Remove / }).all()) await remove.click();
  await panel.getByLabel('Executable', { exact: true }).fill('node');
  await panel.getByLabel('Arguments · JSON array', { exact: true }).fill('["--test","acceptance.cjs"]');
  await panel.getByRole('button', { name: 'Add check command', exact: true }).click();
  await panel.getByRole('checkbox', { name: /I trust this project's code and these Docker commands/ }).check();
  const record = await mutation(() => panel.getByRole('button', { name: 'Run selected requirement checks', exact: true }).click(), projectRoute + '/checks');
  const final = await until('Docker check ' + expectedStatus, async () => { const value = await observe(projectRoute + '/checks/' + record.id); return !['queued', 'running'].includes(value.status) && value; }, 180_000);
  assert.equal(final.status, expectedStatus); assert.equal(final.execution, 'docker'); assert.equal(final.dirty, false); assert.equal(final.sourceChanged, false);
  assert.deepEqual(final.requirementIds, run.input.requirements.map(row => row.id));
  await panel.getByText(expectedStatus === 'failed' ? 'Commands failed' : 'Commands passed', { exact: true }).first().waitFor();
  return { panel, record: final };
}

try {
  await new Promise((done, reject) => provider.once('error', reject).listen(0, bridge, done));
  const providerBase = 'http://host.docker.internal:' + provider.address().port + '/v1';
  const port = await freePort(), previewPort = await freePort(); base = 'http://127.0.0.1:' + port;
  writeFileSync(join(fixture, 'docker/config.json'), JSON.stringify({ cliPluginsExtraDirs: process.env.DOCKER_CONFIG ? [join(process.env.DOCKER_CONFIG, 'cli-plugins')] : [] }), { mode: 0o600 });
  const env = Object.fromEntries(['PATH', 'LANG', 'LC_ALL', 'TZ', 'DOCKER_HOST'].filter(name => process.env[name]).map(name => [name, process.env[name]]));
  Object.assign(env, { HOME: join(fixture, 'home'), DOCKER_CONFIG: join(fixture, 'docker'), LUCI_ADDR: '127.0.0.1:' + port, LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'), LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: repo, LUCI_SANDBOX_IMAGE: image, LUCI_BACKEND: 'docker', LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_LOG_LEVEL: 'error', LUCI_BROWSER_PREVIEW_ADDR: '127.0.0.1:' + previewPort, LUCI_BROWSER_PREVIEW_PORT: String(previewPort), GIT_CONFIG_GLOBAL: join(fixture, 'home/.gitconfig'), GIT_CONFIG_NOSYSTEM: '1' });
  const log = createWriteStream(join(fixture, 'service.log'), { mode: 0o600 });
  service = spawn(binary, [], { cwd: fixture, env, detached: true, stdio: ['ignore', 'pipe', 'pipe'] }); service.on('error', error => { spawnError = error; }); service.stdout.pipe(log); service.stderr.pipe(log);
  await until('Fresh service ready', () => observe('/v1/control-plane/status'), 45_000);
  browser = await chromium.launch({ headless: true, ...(option('--chromium') ? { executablePath: option('--chromium') } : {}) });
  receipt.browserVersion = await browser.version(); page = await browser.newPage({ viewport: { width: 1440, height: 1080 } }); page.setDefaultTimeout(30_000);
  page.on('pageerror', error => browserErrors.push(scrub(error.message)));
  page.on('response', response => { const url = new URL(response.url()); if (url.origin === base && ['POST', 'PUT', 'PATCH', 'DELETE'].includes(response.request().method())) receipt.browserMutations.push({ method: response.request().method(), path: url.pathname, status: response.status() }); });
  page.on('websocket', socket => {
    const match = new URL(socket.url()).pathname.match(/\/sessions\/(s-[a-f0-9]{8})\/attach$/); if (!match) return;
    const sessionID = match[1];
    socket.on('framereceived', ({ payload }) => {
      const bytes = Buffer.isBuffer(payload) ? payload : Buffer.from(payload);
      if (bytes[0] === 0) terminalTextBySession.set(sessionID, ((terminalTextBySession.get(sessionID) ?? '') + bytes.subarray(1).toString()).slice(-128 * 1024).replace(/\x1b\[[0-9;?]*[A-Za-z]/g, ''));
    });
  });
  stage = 'account-and-setup'; await page.goto(base);
  const bootstrap = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(); sensitive.push(bootstrap);
  await page.getByLabel(/^Bootstrap token/).fill(bootstrap); await page.getByLabel('Display name', { exact: true }).fill('Journey Reviewer');
  await page.getByLabel('Username', { exact: true }).fill('journey-owner'); await page.getByLabel('Password', { exact: true }).fill(password);
  if (await page.getByLabel('Confirm password', { exact: true }).count()) await page.getByLabel('Confirm password', { exact: true }).fill(password);
  const owner = await mutation(() => page.getByRole('button', { name: /Create (Super Admin )?account/, exact: true }).click(), '/v1/auth/bootstrap');
  token = owner.token; assert(token); sensitive.push(token); receipt.ownerID = owner.user.id;
  await mutation(() => page.getByRole('button', { name: 'On this computer', exact: true }).click(), '/v1/setup/config');
  await page.getByLabel('Commit author name', { exact: true }).fill('Journey Reviewer'); await page.getByLabel('Commit author email', { exact: true }).fill('journey@example.invalid');
  await mutation(() => page.getByRole('button', { name: 'Save Git identity', exact: true }).click(), '/v1/setup/git/identity');
  await page.getByRole('button', { name: 'Continue to projects', exact: true }).click();
  await page.getByRole('button', { name: 'Settings', exact: true }).click();
  await page.getByRole('navigation', { name: 'Settings sections', exact: true }).getByRole('button', { name: 'Agent connection', exact: true }).click();
  const connection = page.getByRole('region', { name: 'Saved agent connections', exact: true });
  await connection.getByText('Add or edit a connection', { exact: true }).click();
  await connection.getByLabel('Connection name', { exact: true }).fill('Local journey fixture');
  await connection.getByRole('combobox', { name: /^Connection type/ }).selectOption('compatible-api');
  await connection.getByLabel('Codex · Responses base URL', { exact: true }).fill(providerBase);
  await connection.getByLabel('Codex model', { exact: true }).fill('journey-fixture');
  await connection.getByLabel('Connection API key', { exact: true }).fill(key);
  await mutation(() => connection.getByRole('button', { name: 'Save connection', exact: true }).click(), '/v1/setup/agent-connections');
  await mutation(() => connection.getByRole('combobox', { name: /^Active connection/ }).selectOption({ label: 'Local journey fixture' }), '/v1/setup/agent-connections');
  await page.getByRole('button', { name: 'Close settings', exact: true }).click();
  check('Fresh account, local workspace, Git identity and saved provider connection created through UI');

  stage = 'define-design-plan'; await page.getByRole('button', { name: 'New project', exact: true }).click();
  const create = page.getByRole('dialog', { name: 'Create a project', exact: true }); await create.getByLabel('Project name', { exact: true }).fill('garden-notes-journey');
  project = await mutation(() => create.getByRole('button', { name: 'Create project', exact: true }).click(), '/v1/product/projects');
  assert(resolve(project.path).startsWith(fixture + sep)); projectRoute = '/v1/product/projects/' + project.id; receipt.projectID = project.id;
  const answers = [
    ['What should this solution do?', 'A garden notebook that lets me save a note and immediately read it back.'],
    ['Who will use it?', 'One gardener reviewing a local application preview.'],
    ['What does it need to respect?', 'Label the note field and Save note button clearly. Keep this fixture local; no public publishing or external services.'],
    ['What should we leave out?', 'Accounts and durable data are outside this scoped continuity fixture.'],
  ];
  for (const [question, answer] of answers) { await page.getByRole('textbox', { name: question, exact: true }).fill(answer); await page.getByRole('button', { name: 'Next question', exact: true }).click(); }
  await page.getByRole('button', { name: 'Add an outcome', exact: true }).click();
  await page.getByRole('textbox', { name: 'Outcome 1', exact: true }).fill('Save and read a garden note');
  await page.getByRole('textbox', { name: 'How to check outcome 1', exact: true }).fill('Saving a garden note returns that note on the next read.');
  await page.getByRole('button', { name: 'Review specification', exact: true }).click();
  await mutation(() => page.getByRole('button', { name: 'Save definition', exact: true }).click(), projectRoute + '/specification');
  const specification = await mutation(() => page.getByRole('button', { name: 'Approve definition', exact: true }).click(), projectRoute + '/specification/approve');
  assert.equal(specification.requirements.length, 1);
  await lens('Design'); await page.getByRole('button', { name: 'Describe the first screen', exact: true }).click();
  await page.getByRole('textbox', { name: 'Screen name', exact: true }).fill('Garden notes');
  await page.getByRole('textbox', { name: 'Route', exact: true }).fill('/');
  await page.getByRole('textbox', { name: 'What this screen helps someone do', exact: true }).fill('A gardener using the local preview enters a Garden note, activates Save note and reads the saved observation in the list. Every preview visitor has the same actions; there are no application accounts or role checks. Notes survive a page reload while this application process runs, but are held only in server memory and disappear when it restarts.');
  await page.getByRole('textbox', { name: 'States and recovery', exact: true }).fill([
    'First visit: show the Garden note field, Save note button and an empty notes list.',
    'While saving: retain the entered note until the server responds; do not show Saved before a successful response.',
    'Successful save: clear the note field, show Saved and refresh the list so the next read contains the trimmed note.',
    'Rejected save: show Could not save the note, keep the entered note and let the gardener retry Save note.',
    'Application restart: the notes list starts empty; this fixture does not claim durable storage.',
  ].join('\n'));
  await page.getByRole('textbox', { name: 'Accessibility and interaction', exact: true }).fill('Keep the Garden note label visible and programmatically associated with its input. Tab reaches the native Save note button; Enter or Space on that button saves. Announce Saved or a rejected save through the status region, use a semantic list, and keep the form readable at both planned widths.');
  await page.getByRole('textbox', { name: /^Responsive widths/ }).fill('390, 1440');
  const noteOutcome = page.getByRole('checkbox', { name: `${specification.requirements[0].id} · ${specification.requirements[0].title}`, exact: true });
  assert.equal(await noteOutcome.isChecked(), false, 'The unfinished screen must not claim requirement coverage automatically'); await noteOutcome.check();
  await mutation(() => page.getByRole('button', { name: 'Save design plan', exact: true }).click(), projectRoute + '/design-plan');
  await page.getByRole('tab', { name: 'Implementation handoff', exact: true }).click();
  await page.getByRole('textbox', { name: 'Implementation notes', exact: true }).fill('One labelled note input, a Save note button and a list of saved observations. Saving must actually update the next read.');
  await mutation(() => page.getByRole('button', { name: 'Save design plan', exact: true }).click(), projectRoute + '/design-plan');
  const design = await mutation(() => page.getByRole('button', { name: 'Approve design handoff', exact: true }).click(), projectRoute + '/design-plan/approve');
  await lens('Develop'); await page.getByRole('button', { name: 'Add approved requirements as tasks', exact: true }).click();
  const understandingReads = [], planSaveStarted = Date.now();
  const observeUnderstanding = response => {
    if (response.request().method() === 'GET' && new URL(response.url()).pathname === projectRoute + '/orchestrator' && response.ok()) {
      void response.json().then(value => understandingReads.push({ value, receivedAt: Date.now() })).catch(() => {});
    }
  };
  page.on('response', observeUnderstanding);
  const plan = await mutation(() => page.getByRole('button', { name: 'Save implementation plan', exact: true }).click(), projectRoute + '/plan');
  let refreshedUnderstanding;
  try {
    refreshedUnderstanding = await until('Immediate saved-plan overview refresh before periodic polling', () => understandingReads.find(row => row.value.context?.plan?.revision === plan.revision && row.value.context.plan.tasks.length === plan.tasks.length), 9_000);
  } finally { page.off('response', observeUnderstanding); }
  assert.deepEqual(refreshedUnderstanding.value.context.plan.tasks, plan.tasks);
  assert(refreshedUnderstanding.receivedAt - planSaveStarted < 10_000, 'Overview waited for its periodic poll after a successful plan save');
  await page.getByRole('region', { name: 'Current project understanding', exact: true }).getByRole('button', { name: 'Prepare implementation', exact: true }).waitFor();
  receipt.planOverviewRefreshMilliseconds = refreshedUnderstanding.receivedAt - planSaveStarted;
  await screenshot('01-reviewed-context'); check('Interview, approved definition, screen handoff and linked task created through UI', { specificationRevision: specification.revision, designRevision: design.revision });

  stage = 'prepare-run';
  const location = page.locator('.project-plan').getByRole('combobox', { name: 'Build location', exact: true });
  await until('Visible main checkout choice', async () => (await location.getByRole('option').allTextContents()).some(label => label.endsWith(' · main checkout')));
  const mainChoices = (await location.getByRole('option').allTextContents()).filter(label => label.endsWith(' · main checkout'));
  assert.equal(mainChoices.length, 1, 'Fresh project must offer one named main checkout');
  await location.selectOption({ label: mainChoices[0] });
  await page.getByRole('button', { name: 'Prepare project run', exact: true }).click();
  const picker = page.getByRole('dialog', { name: 'Create project run', exact: true });
  await picker.getByRole('combobox', { name: 'Harness', exact: true }).selectOption('codex');
  await picker.getByRole('combobox', { name: 'Agent connection', exact: true }).selectOption({ label: 'Local journey fixture' });
  await picker.getByRole('combobox', { name: 'Permission mode', exact: true }).selectOption('ask');
  run = (await mutation(() => picker.getByRole('button', { name: 'Create project run', exact: true }).click(), projectRoute + '/runs')).run;
  immutableInput = structuredClone(run.input); receipt.runID = run.id;
  // The CLI prompt names immutable reviewed tasks, criteria and document
  // digests. The full acceptance text lives in the mounted context documents,
  // whose exact bytes the actual tool verifies before changing source.
  expectedPrompt = [...run.input.implementation.taskIds, ...run.input.implementation.taskTitles,
    ...run.input.requirements.map(row => row.id), run.input.implementation.planDigest,
    run.input.implementation.specificationDigest, run.input.implementation.designPlanDigest];
  expectedContext = Object.fromEntries(['LUCI.md', '.luci/plan.json', '.luci/specification.json', '.luci/design-handoff.json', '.luci/design-handoff.md'].map(name => [name, sha(readFileSync(join(project.path, name)))]));
  const first = await codingAttempt('Start coding', 1);
  const brokenCommit = await commit('Implement the first garden notebook candidate'); await lens('Develop'); run = await reconcile();
  const failed = await verification('failed');
  assert(failed.record.commands.some(command => command.exitCode === 1 && command.output.includes('The saved note must be readable')), 'The failure must be the application behavior assertion, not a missing tool or runtime error'); assert.equal(failed.record.commit, brokenCommit);
  assert.equal(await failed.panel.getByRole('button', { name: 'Use these results for this build', exact: true }).count(), 0, 'Failed behavior offered as eligible evidence');
  assert.equal(sha(readFileSync(join(project.path, 'acceptance.cjs'))), sha(testSource));
  await screenshot('02-real-behavior-failure'); check('Actual first CLI implementation fails the unchanged note-saving assertion in isolated checks', { sessionID: first.id, checkID: failed.record.id, commit: brokenCommit });
  await failed.panel.getByRole('button', { name: 'Return to build results', exact: true }).click();
  const repair = await codingAttempt('Start another coding attempt', 2);
  const repairedCommit = await commit('Repair saving notes without changing its acceptance test'); assert.notEqual(repairedCommit, brokenCommit);
  await lens('Develop'); run = await reconcile();
  const verified = await verification('passed'); assert.equal(verified.record.commit, repairedCommit); assert.equal(sha(readFileSync(join(project.path, 'acceptance.cjs'))), sha(testSource));
  await mutation(() => verified.panel.getByRole('button', { name: 'Use these results for this build', exact: true }).click(), '/runs/' + run.id + '/checks');
  run = (await observe(projectRoute + '/runs/' + run.id)).run; assert.deepEqual(run.input, immutableInput); assert.equal(run.state, 'awaiting-release');
  await screenshot('03-verified-repair'); check('Visible second coding attempt repairs application behavior, identical checks pass, and scoped evidence joins the original run', { sessionID: repair.id, checkID: verified.record.id, commit: repairedCommit });

  stage = 'preview-and-accept'; await runPanel().getByRole('button', { name: 'Preview this build', exact: true }).click();
  const deployment = page.getByRole('region', { name: 'Checked build deployment', exact: true });
  await deployment.waitFor(); await deployment.getByText(repairedCommit, { exact: false }).waitFor();
  const release = await mutation(() => page.getByRole('button', { name: 'Build local preview', exact: true }).click(), projectRoute + '/releases', 'POST', 240_000);
  releases.add(release.id); assert.equal(release.commit, repairedCommit); assert.equal(release.status, 'ready');
  await page.getByRole('button', { name: 'Open preview', exact: true }).click();
  const preview = page.frameLocator('iframe[title="Project preview"]');
  await preview.getByRole('heading', { name: 'Garden notes', exact: true }).waitFor();
  await preview.getByLabel('Garden note', { exact: true }).fill('Checked through the actual Luci preview');
  await preview.getByRole('button', { name: 'Save note', exact: true }).click();
  await preview.getByText('Checked through the actual Luci preview', { exact: true }).waitFor();
  await screenshot('04-working-preview');
  await mutation(() => page.getByRole('button', { name: 'Attach release to build results', exact: true }).click(), '/runs/' + run.id + '/releases');
  await runPanel().waitFor();
  run = (await observe(projectRoute + '/runs/' + run.id)).run; assert.equal(run.releases.at(-1).releaseId ?? run.releases.at(-1).id, release.id); assert.deepEqual(run.input, immutableInput);
  await runPanel().getByRole('button', { name: /^(Review and accept outcome|Accept verified outcome)$/, exact: true }).click();
  const acceptance = page.getByRole('dialog').filter({ has: page.getByRole('checkbox', { name: /I reviewed the selected workspace preview/ }) });
  await acceptance.getByRole('checkbox').check();
  const accepted = (await mutation(() => acceptance.getByRole('button', { name: 'Accept reviewed outcome', exact: true }).click(), '/runs/' + run.id + '/accept')).run;
  assert.equal(accepted.state, 'complete'); assert.equal(accepted.acceptance.releaseId, release.id); assert.deepEqual(accepted.input, immutableInput);
  await screenshot('05-accepted-original-run'); check('Checked source built through scoped Deploy action, actual preview behavior reviewed, and explicit acceptance returned to the exact original run', { runID: accepted.id, releaseID: release.id, checkID: verified.record.id, commit: repairedCommit });
  assert.deepEqual(browserErrors, []); assert.equal(toolRound, 2); assert.equal(completedRounds, 2);
  receipt.providerCalls = providerCalls; receipt.acceptanceTestSHA256 = sha(testSource); receipt.passed = true;
} catch (error) {
  receipt.failedStage = stage; receipt.failure = scrub(error.stack ?? error); process.exitCode = 1; console.error(receipt.failure);
  if (page) { await screenshot('failure').catch(() => {}); writeFileSync(join(fixture, 'failure-dom.txt'), scrub(await page.locator('body').innerText().catch(() => 'Unavailable')), { mode: 0o600 }); }
  writeFileSync(join(fixture, 'terminal-diagnostic.json'), scrub(JSON.stringify(Object.fromEntries(terminalTextBySession), null, 2)), { mode: 0o600 });
} finally {
  if (browser) await browser.close().catch(() => {});
  // Stop the private control service and its supervised workers before resource
  // inspection. A lost browser response never authorizes deleting an unrelated resource.
  let stopped = false;
  try { stopped = await drainOwnedService(); } catch (error) { receipt.cleanup.push(scrub(error.message)); }
  if (!stopped) { receipt.cleanup.push('Fixture writers remain or cannot be verified; state and containers retained'); receipt.passed = false; process.exitCode = 1; }
  if (stopped && project && receipt.ownerID) {
    // The bootstrap Super Admin uses the main private runtime; an isolated
    // personal worker uses its account directory. Both roots are fixture-owned.
    for (const state of [join(fixture, 'state/sessions'), join(fixture, 'state/control-plane/users', receipt.ownerID, 'runtime/state/sessions')]) {
      try { const registry = JSON.parse(readFileSync(join(state, 'sessions.json'), 'utf8')); for (const session of registry.sessions ?? []) if (session.projectId === project.id) { assert.match(session.id, /^s-[a-f0-9]{8}$/); sessions.add(session.id); } } catch (error) { if (error.code !== 'ENOENT') { receipt.cleanup.push(scrub(error.message)); receipt.passed = false; process.exitCode = 1; } }
      try { for (const release of JSON.parse(readFileSync(join(state, 'releases', project.id + '.json'), 'utf8'))) { assert.equal(release.projectId, project.id); assert.match(release.id, /^rel-[a-f0-9]{24}$/); assert.equal(release.image, 'luci/preview:' + release.id); releases.add(release.id); } } catch (error) { if (error.code !== 'ENOENT') { receipt.cleanup.push(scrub(error.message)); receipt.passed = false; process.exitCode = 1; } }
    }
    for (const [label, ids] of [['luci.session-id', sessions], ['luci.release-id', releases]]) for (const id of ids) {
      try {
        for (const containerID of docker('ps', '-a', '--filter', 'label=' + label + '=' + id, '--format', '{{.ID}}').split(/\s+/).filter(Boolean)) {
          const container = JSON.parse(docker('inspect', containerID))[0]; assert.equal(container.Config.Labels[label], id);
          if (label === 'luci.session-id') assert(container.Mounts.some(mount => mount.Destination === '/home/dev/work' && resolve(mount.Source) === resolve(project.path)), 'Foreign session mount');
          else { assert.equal(container.Config.Labels['luci.project-id'], project.id); assert.equal(container.Config.Labels['luci.preview'], 'true'); assert.equal(container.Config.Image, 'luci/preview:' + id); }
          docker('rm', '-f', containerID);
        }
      } catch (error) { receipt.cleanup.push(scrub(error.message)); receipt.passed = false; process.exitCode = 1; }
    }
    for (const id of releases) { try { docker('image', 'rm', 'luci/preview:' + id); } catch (error) { if (!String(error.stderr ?? '').includes('No such image')) { receipt.cleanup.push('Owned preview image cleanup failed'); receipt.passed = false; process.exitCode = 1; } } }
  }
  provider.closeAllConnections(); if (provider.listening) await new Promise(done => provider.close(done));
  receipt.finishedAt = new Date().toISOString();
  if (!receipt.passed || args.includes('--keep-fixture')) receipt.privateFixture = fixture; else rmSync(fixture, { recursive: true, force: true });
  writeFileSync(report, JSON.stringify(receipt, null, 2) + '\n', { mode: 0o600 }); console.log((receipt.passed ? 'PASS ' : 'FAIL ') + report);
}

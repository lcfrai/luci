#!/usr/bin/env node
// Two real browser identities amend agreed behavior while the original coding
// session remains active. Local deterministic coding is execution scaffolding,
// not model-quality, assistant-reasoning or full production acceptance.
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
import { exerciseDeploymentRecovery } from './deployment-recovery-uat-steps.mjs';
import { verifyPrototypeReview } from './prototype-review-uat-steps.mjs';
import { exerciseOrchestratorRecovery } from './orchestrator-recovery-uat-steps.mjs';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..'), args = process.argv.slice(2);
const option = (name, fallback) => { const i = args.indexOf(name); if (i < 0) return fallback; assert(args[i + 1] && !args[i + 1].startsWith('--'), 'Missing value for ' + name); return args[i + 1]; };
if (args.includes('--help')) {
  console.log('node scripts/team-amendment-browser-uat.mjs --binary PATH --web PATH --image EXISTING_TRUSTED_IMAGE [--release-image EXISTING_NODE_IMAGE] [--chromium PATH] [--report PATH] [--keep-fixture]\nTwo-browser direct-edit amendment while real coding remains active, explicit conflict recovery preserving both contributors, renewed reviews, new coding/check/preview and immutable original history. Account/team/provider isolation setup uses API; all claimed project actions use UI. One sandbox at a time. No paid inference, cloud work or image pull. Requires an exclusive runtime window. Not model reasoning or full production acceptance.');
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
const report = resolve(option('--report', 'artifacts/uat/team-amendment-' + Date.now() + '/result.json')), evidence = dirname(report);
mkdirSync(evidence, { recursive: true, mode: 0o700 });
const fixture = mkdtempSync('/tmp/luci-team-amendment-');
for (const folder of ['home', 'projects', 'state', 'docker']) mkdirSync(join(fixture, folder), { mode: 0o700 });
const sha = bytes => createHash('sha256').update(bytes).digest('hex'), key = 'fixture-only-' + randomUUID(), lifecycle = randomUUID() + randomUUID(), password = randomUUID() + randomUUID();
const sensitive = [key, lifecycle, password], scrub = value => sensitive.reduce((text, secret) => text.replaceAll(secret, '[redacted]'), String(value));
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repo, encoding: 'utf8' }).trim(),
  sourceDirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: repo, encoding: 'utf8' }).trim(),
  binarySHA256: sha(readFileSync(binary)), webIndexSHA256: sha(readFileSync(join(web, 'index.html'))), driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  helperSHA256: Object.fromEntries(['deployment-recovery-uat-steps.mjs', 'prototype-review-uat-steps.mjs', 'orchestrator-recovery-uat-steps.mjs'].map(name => [name, sha(readFileSync(new URL(name, import.meta.url)))])),
  image: { reference: image, id: imageInfo.Id }, releaseImage: { reference: releaseImage, id: releaseImageInfo.Id },
  scope: 'Two-person direct-edit amendment, conflict recovery and real coding/check/preview continuity. Local deterministic provider implements the exact fixture; no assistant reasoning is claimed.',
  limitations: ['Existing trusted Docker images; installation is not exercised.', 'No live model-quality, public deployment, novice walkthrough, application authentication, durable application data, cloud, or paid-service claim.'],
  externalModelCalls: 0, cloudResourcesCreated: 0, setupScaffolding: ['Two accounts, team membership and project grants', 'Team Git identity and named local fixture provider route'], checks: [], browserMutations: [], cleanup: [],
};
const check = (name, details = {}) => { receipt.checks.push({ name, passed: true, ...details }); console.log('PASS ' + name); };
const sessions = new Set(), releases = new Set(), browserErrors = [];
let service, browser, page, alice, bob, base, token, team, workspace, project, projectRoute, run, immutableInput, originalRun, stage = 'startup', spawnError, providerError, releaseHeldModel;
const actorNames = new Map();
let providerCalls = 0, toolRound = 0, codingPhase = 0, completedRounds = 0, expectedContext = {}, expectedPrompt = [];
const terminalTextBySession = new Map();
const oldTitle = 'Save and read a garden note', oldAcceptance = 'Saving a garden note returns that note on the next read.';
const newTitle = 'Save and read a garden note with its garden area', newAcceptance = 'Save a note for the Greenhouse and confirm the next read and the notes list show both the note and its garden area; notes without an area remain readable.';
const originalConstraints = 'Label the note field and Save note button clearly. Keep this fixture local; no public publishing or external services.';
const aliceConstraint = originalConstraints + ' Keep the garden area and note controls keyboard accessible with visible labels.';
const pageHTML = '<!doctype html><html><head><meta name="viewport" content="width=device-width,initial-scale=1"><title>Garden notes</title><style>body{font:18px system-ui;max-width:42rem;margin:3rem auto;padding:1rem}input,button{font:inherit;padding:.6rem}label{display:block}li{padding:.5rem}</style></head><body><h1>Garden notes</h1><p>Save an observation for this running preview.</p><label for="note">Garden note</label><input id="note"><button id="save">Save note</button><p role="status" id="status"></p><ul id="notes"></ul><script>async function refresh(){const r=await fetch("/notes");const notes=await r.json();document.querySelector("#notes").replaceChildren(...notes.map(text=>{const li=document.createElement("li");li.textContent=text;return li;}));}document.querySelector("#save").onclick=async()=>{const input=document.querySelector("#note");const r=await fetch("/notes",{method:"POST",headers:{"content-type":"application/json"},body:JSON.stringify({note:input.value})});if(!r.ok){document.querySelector("#status").textContent="Could not save the note";return;}input.value="";await refresh();document.querySelector("#status").textContent="Saved";};refresh();</script></body></html>';
const appSource = [
  "const http = require('node:http');", 'const saveEnabled = true;', 'const html = ' + JSON.stringify(pageHTML) + ';',
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
const amendedHTML = pageHTML.replace('<label for="note">', '<label for="area">Garden area</label><input id="area"><label for="note">')
  .replace('JSON.stringify({note:input.value})', 'JSON.stringify({note:input.value,area:document.querySelector("#area").value})')
  .replace('li.textContent=text;', 'li.textContent=typeof text==="string"?text:text.area+" · "+text.note;');
const amendedAppSource = appSource.replace(JSON.stringify(pageHTML), JSON.stringify(amendedHTML))
  .replace('const {note}=JSON.parse(raw);', 'const {note,area}=JSON.parse(raw);')
  .replace('notes.push(note.trim());', "notes.push(typeof area==='string' && area.trim() ? {note:note.trim(),area:area.trim()} : note.trim());");
const amendedTestSource = testSource + [
  "test('An amended note retains its garden area on the next read', async () => { const server=createApp().listen(0,'127.0.0.1'); await once(server,'listening'); const base='http://127.0.0.1:'+server.address().port;",
  "try { const saved=await fetch(base+'/notes',{method:'POST',headers:{'content-type':'application/json'},body:JSON.stringify({note:'Inspect the seedlings',area:'Greenhouse'})}); assert.equal(saved.status,200); const list=await fetch(base+'/notes').then(r=>r.json()); assert.deepEqual(list,[{note:'Inspect the seedlings',area:'Greenhouse'}],'The amended behavior must retain both note and garden area'); } finally { await new Promise(done=>server.close(done)); } });", '',
].join('\n');
function sendResponse(res, item) {
  const response = { id: 'resp_journey_' + providerCalls, object: 'response', created_at: 1, status: 'completed', model: 'amendment-fixture', output: [item], usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2, input_tokens_details: { cached_tokens: 0 }, output_tokens_details: { reasoning_tokens: 0 } } };
  const events = [{ type: 'response.created', response: { ...response, status: 'in_progress', output: [] } }, { type: 'response.output_item.added', output_index: 0, item: { ...item, status: 'in_progress', ...(item.type === 'function_call' ? { arguments: '' } : { content: [] }) } }];
  if (item.type === 'function_call') events.push({ type: 'response.function_call_arguments.delta', item_id: item.id, output_index: 0, delta: item.arguments }, { type: 'response.function_call_arguments.done', item_id: item.id, output_index: 0, arguments: item.arguments });
  else events.push({ type: 'response.content_part.added', item_id: item.id, output_index: 0, content_index: 0, part: { type: 'output_text', text: '', annotations: [] } }, { type: 'response.output_text.delta', item_id: item.id, output_index: 0, content_index: 0, delta: item.content[0].text }, { type: 'response.output_text.done', item_id: item.id, output_index: 0, content_index: 0, text: item.content[0].text });
  events.push({ type: 'response.output_item.done', output_index: 0, item }, { type: 'response.completed', response });
  if (!res.headersSent) res.writeHead(200, { 'content-type': 'text/event-stream' });
  res.end(events.map((event, sequence_number) => 'event: ' + event.type + '\ndata: ' + JSON.stringify({ ...event, sequence_number }) + '\n\n').join(''));
}
const provider = httpServer(async (req, res) => {
  try {
    assert.equal(req.method, 'POST'); assert.equal(req.url, '/v1/responses'); assert.equal(req.headers.authorization, 'Bearer ' + key);
    const parts = []; let size = 0; for await (const part of req) { size += part.length; assert(size < 4 * 1024 * 1024); parts.push(part); }
    const body = JSON.parse(Buffer.concat(parts).toString()); providerCalls++;
    assert.equal(body.model, 'amendment-fixture');
    const serialized = JSON.stringify(body), tools = (body.tools ?? []).map(item => item.name ?? item.function?.name ?? item.type);
    const coding = tools.includes('exec_command'), hasToolResult = serialized.includes('function_call_output');
    if (coding) for (const text of expectedPrompt) assert(serialized.includes(text), 'CLI omitted reviewed task context: ' + text);
    if (coding && !hasToolResult) {
      assert(toolRound < 2, 'Unexpected third implementation attempt');
      assert.equal(codingPhase, toolRound + 1, 'A repeated model request cannot become a new reviewed implementation phase'); toolRound++;
      (receipt.providerInstructions ??= []).push({ attempt: toolRound, model: body.model, runID: run.id, taskTitles: [...run.input.implementation.taskTitles], requirements: structuredClone(run.input.requirements), mountedContextSHA256: { ...expectedContext }, promptMatched: true });
      const prefix = "const fs=require('node:fs'),crypto=require('node:crypto'),assert=require('node:assert/strict');const expected=" + JSON.stringify(expectedContext) + ";for(const [p,d] of Object.entries(expected))assert.equal(crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex'),d,'Reviewed context drift: '+p);";
      const payload = toolRound === 1 ? prefix
        + 'fs.writeFileSync("app.cjs",' + JSON.stringify(appSource) + ');fs.writeFileSync("acceptance.cjs",' + JSON.stringify(testSource) + ');'
        + 'fs.writeFileSync("package.json",' + JSON.stringify(JSON.stringify({ name: 'garden-notes-continuity', private: true, scripts: { test: 'node --test acceptance.cjs' } })) + ');'
        + 'fs.writeFileSync("Dockerfile",' + JSON.stringify('FROM ' + releaseImage + '\nWORKDIR /app\nCOPY app.cjs /app/app.cjs\nUSER 1000:1000\nEXPOSE 3000\nENTRYPOINT ["node","/app/app.cjs"]\n') + ');'
        : prefix + 'assert.equal(crypto.createHash("sha256").update(fs.readFileSync("acceptance.cjs")).digest("hex"),' + JSON.stringify(sha(testSource)) + ');assert.equal(crypto.createHash("sha256").update(fs.readFileSync("app.cjs")).digest("hex"),' + JSON.stringify(sha(appSource)) + ');fs.writeFileSync("app.cjs",' + JSON.stringify(amendedAppSource) + ');fs.writeFileSync("acceptance.cjs",' + JSON.stringify(amendedTestSource) + ');';
      sendResponse(res, { id: 'fc_journey_' + toolRound, type: 'function_call', status: 'completed', name: 'exec_command', call_id: 'call_journey_' + toolRound, arguments: JSON.stringify({ cmd: 'node -e "eval(Buffer.from(\'' + Buffer.from(payload).toString('base64') + '\',\'base64\').toString())"', workdir: '/home/dev/work', login: false, max_output_tokens: 1000 }) });
    } else {
      if (coding) {
        assert(hasToolResult); completedRounds = toolRound;
        if (toolRound === 1) {
          // Keep the actual old coding child active after its real tool wrote
          // the original scope. Only the visible stop action ends that child.
          // SSE comments keep the deliberately pending fixture response alive
          // while two people review the amendment; they contain no model text.
          res.writeHead(200, { 'content-type': 'text/event-stream' });
          res.write(': fixture response awaiting explicit session stop\n\n');
          const heartbeat = setInterval(() => { if (!res.destroyed) res.write(': fixture pending\n\n'); }, 10_000);
          try { await new Promise(done => { releaseHeldModel = done; res.once('close', done); }); }
          finally { clearInterval(heartbeat); }
          if (res.destroyed) return;
        }
      }
      sendResponse(res, { id: 'msg_journey_' + providerCalls, type: 'message', status: 'completed', role: 'assistant', content: [{ type: 'output_text', text: toolRound === 2 ? 'Implemented the reviewed garden-area amendment and retained the original plain-note acceptance test. Run independent verification.' : 'Original plain-note implementation completed; no amended scope was supplied to this session.', annotations: [] }] });
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
async function setupRequest(method, path, body, expected = 200) {
  assert(path === '/v1/auth/bootstrap' || path === '/v1/admin/users' || path === '/v1/teams' || /^\/v1\/teams\/[^/]+\/(?:members(?:\/[^/]+)?|projects\/[^/]+\/members\/[^/]+)$/.test(path) || path.endsWith('/v1/setup/git/identity') || path.endsWith('/v1/setup/agent-connections'), 'Setup API cannot mutate project work: ' + path);
  const response = await fetch(base + path, { method, headers: { Origin: base, 'Content-Type': 'application/json', ...(token ? { Authorization: 'Bearer ' + token } : {}) }, body: JSON.stringify(body), signal: AbortSignal.timeout(30_000) });
  assert.equal(response.status, expected, 'Isolation setup ' + path); return response.json();
}
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
async function codingAttempt(label, round, keepRunning = false) {
  stage = round === 1 ? 'original-coding' : 'amended-coding';
  const session = await mutation(() => runPanel().getByRole('button', { name: label, exact: true }).click(), '/v1/sessions', 'POST', 180_000);
  sessions.add(session.id); assert.equal(session.projectRunId, run.id); assert.equal(session.agentConnectionId, run.input.route.connection.id); assert.equal(session.model, 'amendment-fixture');
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
  assert.equal(sha(readFileSync(join(project.path, 'acceptance.cjs'))), sha(round === 1 ? testSource : amendedTestSource), 'Agent changed fixture acceptance contract');
  if (keepRunning) return session;
  await stopCoding(session);
  return session;
}
async function stopCoding(session) {
  const pane = page.locator('section.pane[data-session-id=' + JSON.stringify(session.id) + ']');
  await pane.getByRole('button', { name: /^Session menu for / }).click();
  await mutation(() => page.getByRole('menuitem', { name: 'Kill', exact: true }).click(), '/v1/sessions/' + session.id, 'DELETE');
  assert.equal(docker('ps', '-a', '--filter', 'label=luci.session-id=' + session.id, '--format', '{{.ID}}'), '', 'Visible Kill action must remove this sandbox before another attempt');
  await lens('Develop'); run = await reconcile(); assert.deepEqual(run.input, immutableInput);
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
  receipt.browserVersion = await browser.version();
  async function actorPage(name) {
    const target = await browser.newPage({ viewport: { width: 1440, height: 1080 } }); target.setDefaultTimeout(30_000); actorNames.set(target, name);
    target.on('pageerror', error => browserErrors.push(name + ': ' + scrub(error.message)));
    target.on('response', response => { const url = new URL(response.url()); if (url.origin === base && ['POST', 'PUT', 'PATCH', 'DELETE'].includes(response.request().method())) receipt.browserMutations.push({ actor: name, method: response.request().method(), path: url.pathname, status: response.status() }); });
    target.on('websocket', socket => {
      const match = new URL(socket.url()).pathname.match(/\/sessions\/(s-[a-f0-9]{8})\/attach$/); if (!match) return;
      socket.on('framereceived', ({ payload }) => { const bytes = Buffer.isBuffer(payload) ? payload : Buffer.from(payload); if (bytes[0] === 0) terminalTextBySession.set(match[1], ((terminalTextBySession.get(match[1]) ?? '') + bytes.subarray(1).toString()).slice(-128 * 1024).replace(/\x1b\[[0-9;?]*[A-Za-z]/g, '')); });
    });
    return target;
  }
  async function login(target, username, userID) {
    page = target; await page.goto(base); await page.getByLabel('Username', { exact: true }).fill(username); await page.getByLabel('Password', { exact: true }).fill(password);
    const signed = await mutation(() => page.getByRole('button', { name: 'Sign in', exact: true }).click(), '/v1/auth/login');
    assert.equal(signed.user.id, userID); sensitive.push(signed.token);
    const firstVisit = page.getByRole('button', { name: 'Continue to projects', exact: true });
    const workspacePicker = page.getByLabel('Workspace', { exact: true });
    await until('Workspace entry after sign in', async () => await firstVisit.isVisible() || await workspacePicker.isVisible(), 30_000);
    if (await firstVisit.isVisible()) {
      await firstVisit.click();
      (receipt.firstVisitChoices ??= []).push({ actor: actorNames.get(target), action: 'Continue to projects', personalProjectCreated: false });
    }
    await workspacePicker.selectOption({ label: team.name });
    await page.getByRole('button', { name: 'Team access', exact: true }).waitFor();
  }
  async function prepareRun() {
    const location = page.locator('.project-plan').getByRole('combobox', { name: 'Build location', exact: true });
    await until('Named main checkout', async () => (await location.getByRole('option').allTextContents()).some(label => label.endsWith(' · main checkout')));
    const choices = (await location.getByRole('option').allTextContents()).filter(label => label.endsWith(' · main checkout')); assert.equal(choices.length, 1);
    await location.selectOption({ label: choices[0] });
    await page.getByRole('button', { name: 'Prepare project run', exact: true }).click();
    const picker = page.getByRole('dialog', { name: 'Create project run', exact: true });
    await picker.getByRole('combobox', { name: 'Harness', exact: true }).selectOption('codex');
    await picker.getByRole('combobox', { name: 'Agent connection', exact: true }).selectOption({ label: 'Local amendment fixture' });
    await picker.getByRole('combobox', { name: 'Permission mode', exact: true }).selectOption('ask');
    await picker.getByRole('combobox', { name: 'Intended outcome', exact: true }).selectOption('preview');
    run = (await mutation(() => picker.getByRole('button', { name: 'Create project run', exact: true }).click(), projectRoute + '/runs')).run;
    codingPhase++;
    immutableInput = structuredClone(run.input);
    expectedPrompt = [...run.input.implementation.taskIds, ...run.input.implementation.taskTitles, ...run.input.requirements.map(row => row.id), run.input.implementation.planDigest, run.input.implementation.specificationDigest, run.input.implementation.designPlanDigest];
    expectedContext = Object.fromEntries(['LUCI.md', '.luci/plan.json', '.luci/specification.json', '.luci/design-handoff.json', '.luci/design-handoff.md'].map(name => [name, sha(readFileSync(join(project.path, name)))]));
  }
  async function understand(target, currentTitle, currentAcceptance, approved) {
    page = target;
    const panel = page.getByRole('complementary', { name: 'Project understanding', exact: true });
    const pending = page.waitForResponse(response => response.request().method() === 'GET' && new URL(response.url()).pathname === projectRoute + '/orchestrator');
    await panel.getByRole('button', { name: 'Refresh project understanding', exact: true }).click(); const response = await pending; assert(response.ok()); const view = await response.json();
    assert(view.context.activeRuns.some(row => row.id === originalRun.id), 'Old real coding activity disappeared after a context amendment');
    const active = panel.getByRole('region', { name: 'Active and unconfirmed work', exact: true });
    const scope = active.getByRole('region', { name: 'In this build', exact: true });
    await scope.getByText(oldTitle, { exact: true }).first().waitFor();
    const outcomes = scope.locator('details').filter({ has: page.locator('summary').filter({ hasText: 'Outcomes this build must prove' }) });
    if (!await outcomes.evaluate(element => element.open)) await outcomes.locator('summary').click();
    await scope.getByText(oldAcceptance, { exact: true }).waitFor();
    await scope.getByText(/Later edits do not silently change work already started/).waitFor();
    const current = panel.getByRole('region', { name: 'Agreed outcomes', exact: true });
    await current.getByText(currentTitle, { exact: true }).waitFor(); await current.getByText(currentAcceptance, { exact: true }).waitFor();
    if (!approved) await current.getByText('These outcomes are proposed until the definition is reviewed.', { exact: true }).waitFor();
    else await panel.getByText(/Reviewed definition.*Revision/).waitFor();
    const preserved = (await observe(projectRoute + '/runs/' + originalRun.id)).run; assert.deepEqual(preserved.input, originalRun.input); assert(!preserved.acceptance);
    const snapshot = { actor: actorNames.get(target), approved, activeScope: await scope.innerText(), currentOutcomes: await current.innerText(), originalRequirement: preserved.input.requirements[0], currentRequirement: view.context.specification.requirements[0] };
    (receipt.understanding ??= []).push(snapshot); return view;
  }

  stage = 'isolation-setup';
  const bootstrap = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(); sensitive.push(bootstrap);
  const owner = await setupRequest('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'alice', displayName: 'Alice Reviewer', password }, 201);
  token = owner.token; sensitive.push(token); receipt.ownerID = owner.user.id;
  const colleague = await setupRequest('POST', '/v1/admin/users', { username: 'bob', displayName: 'Bob Builder', password }, 201);
  team = (await setupRequest('POST', '/v1/teams', { name: 'Garden collaboration' }, 201)).team; receipt.teamID = team.id; workspace = '/v1/teams/' + team.id + '/workspace';
  const grants = { execute: true, secrets: true, deploy: true, admin: false };
  await setupRequest('POST', '/v1/teams/' + team.id + '/members', { expectedRevision: team.revision, username: 'bob', role: 'editor', grants });
  await setupRequest('POST', workspace + '/v1/setup/git/identity', { name: 'Garden Collaboration', email: 'garden-collaboration@example.invalid' });
  await setupRequest('POST', workspace + '/v1/setup/agent-connections', { action: 'save', expectedRevision: 0, name: 'Local amendment fixture', kind: 'compatible-api', enabled: true, endpoints: { responses: providerBase }, models: { codex: 'amendment-fixture' }, apiKey: key });
  alice = await actorPage('Alice Reviewer'); bob = await actorPage('Bob Builder');
  await login(alice, 'alice', owner.user.id);

  stage = 'original-shared-context';
  await page.getByRole('button', { name: 'New project', exact: true }).click();
  const create = page.getByRole('dialog', { name: 'Create a project', exact: true }); await create.getByLabel('Project name', { exact: true }).fill('garden-amendment');
  project = await mutation(() => create.getByRole('button', { name: 'Create project', exact: true }).click(), workspace + '/v1/product/projects');
  assert(resolve(project.path).startsWith(fixture + sep)); projectRoute = workspace + '/v1/product/projects/' + project.id; receipt.projectID = project.id;
  const access = await observe('/v1/teams/' + team.id);
  await setupRequest('PUT', '/v1/teams/' + team.id + '/projects/' + project.id + '/members/' + colleague.id, { expectedRevision: access.team.revision, role: 'editor', grants });
  const answers = [['What should this solution do?', 'A shared garden notebook for saving observations.'], ['Who will use it?', 'Two gardeners reviewing a local application preview.'], ['What does it need to respect?', originalConstraints], ['What should we leave out?', 'Application accounts and durable application data are outside this scoped collaboration fixture.']];
  for (const [question, answer] of answers) { await page.getByRole('textbox', { name: question, exact: true }).fill(answer); await page.getByRole('button', { name: 'Next question', exact: true }).click(); }
  await page.getByRole('button', { name: 'Add an outcome', exact: true }).click(); await page.getByRole('textbox', { name: 'Outcome 1', exact: true }).fill(oldTitle); await page.getByRole('textbox', { name: 'How to check outcome 1', exact: true }).fill(oldAcceptance); await page.getByRole('button', { name: 'Review specification', exact: true }).click();
  await mutation(() => page.getByRole('button', { name: 'Save definition', exact: true }).click(), projectRoute + '/specification');
  const spec = await mutation(() => page.getByRole('button', { name: 'Approve definition', exact: true }).click(), projectRoute + '/specification/approve');
  await lens('Design'); await page.getByRole('button', { name: 'Start a screen plan from my definition', exact: true }).click(); await page.getByRole('textbox', { name: 'Screen name', exact: true }).fill('Garden notes'); await page.getByRole('textbox', { name: /^Responsive widths/ }).fill('390, 1440');
  await mutation(() => page.getByRole('button', { name: 'Save design plan', exact: true }).click(), projectRoute + '/design-plan');
  await page.getByRole('tab', { name: 'Implementation handoff', exact: true }).click(); await page.getByRole('textbox', { name: 'Implementation notes', exact: true }).fill('A labelled Garden note input, Save note button and a list showing the saved observations.');
  await mutation(() => page.getByRole('button', { name: 'Save design plan', exact: true }).click(), projectRoute + '/design-plan');
  await mutation(() => page.getByRole('button', { name: 'Approve design handoff', exact: true }).click(), projectRoute + '/design-plan/approve');
  await lens('Develop'); await page.getByRole('button', { name: 'Add approved requirements as tasks', exact: true }).click(); await mutation(() => page.getByRole('button', { name: 'Save implementation plan', exact: true }).click(), projectRoute + '/plan');
  await prepareRun(); originalRun = structuredClone(run); receipt.originalRunID = run.id;
  const originalSession = await codingAttempt('Start coding', 1, true); run = await reconcile(); assert.equal(run.state, 'coding');
  assert.equal(readFileSync(join(project.path, 'app.cjs'), 'utf8'), appSource);
  check('Alice created and approved the original plain-note context through UI; its actual coding session wrote that candidate and remains active', { runID: run.id, sessionID: originalSession.id, originalTitle: oldTitle, originalAcceptance: oldAcceptance });

  await login(bob, 'bob', colleague.id); await page.getByRole('treeitem', { name: /garden-amendment$/ }).click(); await lens('Define');
  await understand(bob, oldTitle, oldAcceptance, true);
  await screenshot('01-bob-understands-active-original');
  page = alice; await lens('Define'); await page.getByRole('tab', { name: 'Specification', exact: true }).click(); await page.getByRole('textbox', { name: 'Boundaries and constraints', exact: true }).fill(aliceConstraint);
  page = bob; const overview = page.getByRole('complementary', { name: 'Project understanding', exact: true }); await overview.getByRole('button', { name: 'Change the plan', exact: true }).click(); await overview.getByRole('button', { name: 'Edit outcomes', exact: true }).click();
  await page.getByRole('textbox', { name: 'Requirement 1', exact: true }).fill(newTitle); await page.getByRole('textbox', { name: 'Acceptance scenario 1', exact: true }).fill(newAcceptance);
  const bobSaved = await mutation(() => page.getByRole('button', { name: 'Save definition', exact: true }).click(), projectRoute + '/specification');
  assert.equal(bobSaved.requirements[0].id, spec.requirements[0].id); assert.equal(bobSaved.requirements[0].title, newTitle); assert.equal(bobSaved.constraints, originalConstraints);
  await understand(bob, newTitle, newAcceptance, false); await screenshot('02-amended-draft-original-still-active');

  stage = 'two-person-conflict-recovery'; page = alice;
  const conflict = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === projectRoute + '/specification');
  await page.getByRole('button', { name: 'Save definition', exact: true }).click(); assert.equal((await conflict).status(), 409);
  assert.equal(await page.getByRole('textbox', { name: 'Boundaries and constraints', exact: true }).inputValue(), aliceConstraint);
  await page.getByRole('button', { name: 'Reload saved definition', exact: true }).click();
  const recovery = page.getByRole('region', { name: 'Definition draft recovery', exact: true }); await recovery.waitFor();
  // The conflict-aware UI must preserve Bob's disjoint saved outcome and Alice's
  // local constraint. Never re-enter Bob's change to hide a destructive restore.
  await recovery.getByText(/Edits to different fields are combined with the saved version/).waitFor();
  assert.equal(await recovery.getByRole('radio').count(), 0, 'Disjoint edits should combine without asking which contributor to discard');
  await recovery.getByRole('checkbox', { name: 'I reviewed the saved version and these recovery choices.', exact: true }).check();
  await recovery.getByRole('button', { name: 'Restore draft', exact: true }).click();
  assert.equal(await page.getByRole('textbox', { name: 'Requirement 1', exact: true }).inputValue(), newTitle);
  assert.equal(await page.getByRole('textbox', { name: 'Acceptance scenario 1', exact: true }).inputValue(), newAcceptance);
  assert.equal(await page.getByRole('textbox', { name: 'Boundaries and constraints', exact: true }).inputValue(), aliceConstraint);
  const merged = await mutation(() => page.getByRole('button', { name: 'Save definition', exact: true }).click(), projectRoute + '/specification');
  assert.equal(merged.constraints, aliceConstraint); assert.equal(merged.requirements[0].title, newTitle); assert.equal(merged.requirements[0].acceptance, newAcceptance);
  await screenshot('03-both-contributors-preserved');
  const reviewed = await mutation(() => page.getByRole('button', { name: 'Approve definition', exact: true }).click(), projectRoute + '/specification/approve');
  await understand(alice, newTitle, newAcceptance, true); await understand(bob, newTitle, newAcceptance, true);
  check('Bob amended the outcome while coding stayed active; Alice received a real409 and explicitly merged her constraint without overwriting his outcome; both browsers distinguish new reviewed scope from the original active build', { oldDefinitionRevision: spec.revision, newDefinitionRevision: reviewed.revision, bobOutcome: newTitle, aliceConstraint });

  stage = 'renew-review-and-stop-original'; page = bob; await lens('Design');
  await page.getByText('The definition or sketch has changed. Review the screen plan and save a new handoff before approving.', { exact: true }).waitFor();
  await page.getByRole('tab', { name: 'Implementation handoff', exact: true }).click();
  await page.getByRole('textbox', { name: 'Implementation notes', exact: true }).fill('Keep the existing Garden note input and saved-note behavior. Add a labelled, keyboard-accessible Garden area input. Saving a note with an area renders area and note together; old plain notes remain readable.');
  await mutation(() => page.getByRole('button', { name: 'Save design plan', exact: true }).click(), projectRoute + '/design-plan');
  await mutation(() => page.getByRole('button', { name: 'Approve design handoff', exact: true }).click(), projectRoute + '/design-plan/approve');
  await lens('Develop'); await runPanel().getByRole('button', { name: 'Open coding session', exact: true }).click();
  run = (await observe(projectRoute + '/runs/' + originalRun.id)).run; immutableInput = structuredClone(originalRun.input);
  await stopCoding(originalSession); releaseHeldModel?.();
  assert.equal(docker('ps', '-a', '--filter', 'label=luci.session-id=' + originalSession.id, '--format', '{{.ID}}'), '');
  const receipts = runPanel().locator('details.project-run-receipts'); if (!await receipts.evaluate(element => element.open)) await receipts.locator('summary').click();
  await runPanel().getByRole('button', { name: 'Cancel run', exact: true }).click();
  const cancel = page.getByRole('dialog', { name: 'Cancel run', exact: true });
  run = (await mutation(() => cancel.getByRole('button', { name: 'Cancel future dispatch', exact: true }).click(), '/runs/' + originalRun.id + '/cancel')).run;
  assert.equal(run.state, 'cancelled'); assert.deepEqual(run.input, originalRun.input); assert(!run.acceptance);
  const workDetails = page.locator('details.project-work');
  if (!await workDetails.evaluate(element => element.open)) await workDetails.locator('summary').first().click();
  await page.getByRole('button', { name: 'Refresh approved requirements', exact: true }).click();
  await page.getByRole('textbox', { name: 'Task 1', exact: true }).fill(newTitle);
  const links = page.locator('.project-plan details').filter({ has: page.locator('summary').filter({ hasText: /^Requirements for task 1 ·/ }) });
  if (!await links.evaluate(element => element.open)) await links.locator('summary').click();
  await links.getByText(newAcceptance, { exact: false }).waitFor(); assert(await links.getByRole('checkbox').isChecked());
  await mutation(() => page.getByRole('button', { name: 'Save implementation plan', exact: true }).click(), projectRoute + '/plan');
  const preservedCommit = await commit('Preserve original implementation and both reviewed amendments'); await lens('Develop');
  check('Bob renewed the changed design and linked task plan, explicitly stopped the original child and cancelled only its future work; original instructions and candidate source remain recorded', { originalRunID: originalRun.id, originalSessionID: originalSession.id, preservedCommit });

  stage = 'new-reviewed-build';
  const planDetails = page.locator('details.project-work'); if (await planDetails.count() && !await planDetails.evaluate(element => element.open)) await planDetails.locator('summary').click();
  await prepareRun(); assert.notEqual(run.id, originalRun.id); assert.equal(run.input.requirements[0].title, newTitle); assert.equal(run.input.requirements[0].acceptance, newAcceptance); assert.equal(run.input.implementation.specificationRevision, reviewed.revision); receipt.amendedRunID = run.id;
  const amendedSession = await codingAttempt('Start coding', 2);
  assert.equal(readFileSync(join(project.path, 'app.cjs'), 'utf8'), amendedAppSource); assert(readFileSync(join(project.path, 'acceptance.cjs'), 'utf8').startsWith(testSource));
  const amendedCommit = await commit('Implement the reviewed garden-area behavior and retain plain notes'); await lens('Develop'); run = await reconcile();
  const verified = await verification('passed'); assert.equal(verified.record.commit, amendedCommit); assert(verified.record.commands.every(command => command.exitCode === 0));
  await mutation(() => verified.panel.getByRole('button', { name: 'Use these results for this build', exact: true }).click(), '/runs/' + run.id + '/checks');
  run = (await observe(projectRoute + '/runs/' + run.id)).run; assert.equal(run.state, 'awaiting-release'); assert.deepEqual(run.input, immutableInput);
  check('A separately reviewed new run receives the amended scope; Bob starts actual coding, retains the original plain-note assertion, and joins passing amended-scope Docker checks', { newRunID: run.id, sessionID: amendedSession.id, checkID: verified.record.id, commit: amendedCommit });

  stage = 'team-preview-and-accept'; await runPanel().getByRole('button', { name: 'Preview this build', exact: true }).click();
  await page.getByRole('region', { name: 'Checked build deployment', exact: true }).getByText(amendedCommit, { exact: false }).waitFor();
  const release = await mutation(() => page.getByRole('button', { name: 'Build local preview', exact: true }).click(), projectRoute + '/releases', 'POST', 240_000); releases.add(release.id); assert.equal(release.commit, amendedCommit); assert.equal(release.status, 'ready');
  await page.getByRole('button', { name: 'Open preview', exact: true }).click(); const preview = page.frameLocator('iframe[title="Project preview"]');
  await preview.getByRole('heading', { name: 'Garden notes', exact: true }).waitFor(); await preview.getByLabel('Garden area', { exact: true }).fill('Greenhouse'); await preview.getByLabel('Garden note', { exact: true }).fill('Inspect the seedlings'); await preview.getByRole('button', { name: 'Save note', exact: true }).click(); await preview.getByText('Greenhouse · Inspect the seedlings', { exact: true }).waitFor();
  await preview.getByLabel('Garden area', { exact: true }).fill(''); await preview.getByLabel('Garden note', { exact: true }).fill('Plain notes still work'); await preview.getByRole('button', { name: 'Save note', exact: true }).click(); await preview.getByText('Plain notes still work', { exact: true }).waitFor();
  await screenshot('04-amended-preview-preserves-original-behavior');
  await mutation(() => page.getByRole('button', { name: 'Attach release to build results', exact: true }).click(), '/runs/' + run.id + '/releases');
  await runPanel().getByRole('button', { name: 'Review and accept outcome', exact: true }).click(); const acceptance = page.getByRole('dialog', { name: 'Review and accept outcome', exact: true }); await acceptance.getByRole('checkbox').check();
  const accepted = (await mutation(() => acceptance.getByRole('button', { name: 'Accept reviewed outcome', exact: true }).click(), '/runs/' + run.id + '/accept')).run;
  assert.equal(accepted.state, 'complete'); assert.equal(accepted.acceptance.releaseId, release.id); assert.deepEqual(accepted.input, immutableInput);
  const historical = (await observe(projectRoute + '/runs/' + originalRun.id)).run; assert.deepEqual(historical.input, originalRun.input); assert.equal(historical.state, 'cancelled'); assert(!historical.acceptance);
  const history = await observe(projectRoute + '/context-history'); assert(history.events.some(row => row.actor.id === colleague.id && row.action === 'save')); assert(history.events.some(row => row.actor.id === owner.user.id && row.action === 'approve'));
  await screenshot('05-new-outcome-accepted-old-inputs-retained');
  check('Bob verifies both new garden-area behavior and original plain notes in the actual team preview, accepts only the amended run, and the old cancelled run retains its original human-readable instructions without acceptance', { oldRunID: originalRun.id, acceptedRunID: accepted.id, releaseID: release.id, oldOutcome: historical.input.requirements[0], acceptedOutcome: accepted.input.requirements[0], aliceID: owner.user.id, bobID: colleague.id });

  stage = 'day-two-preview-recovery';
  const replacementRelease = await exerciseDeploymentRecovery({ page, projectId: project.id, projectRoute, release, lens, mutation, observe, check, screenshot, onRelease: id => releases.add(id) });
  receipt.replacementReleaseID = replacementRelease.id;
  const acceptedAfterRecovery = (await observe(projectRoute + '/runs/' + accepted.id)).run;
  assert.deepEqual(acceptedAfterRecovery.input, accepted.input);
  assert.deepEqual(acceptedAfterRecovery.acceptance, accepted.acceptance, 'Replacing a preview must not rewrite the earlier acceptance receipt');
  assert.equal(acceptedAfterRecovery.acceptance.releaseId, release.id);

  stage = 'prototype-review-and-future-design-draft';
  const designDraft = await verifyPrototypeReview({ page, projectId: project.id, projectRoute, release: replacementRelease, design: await observe(projectRoute + '/design-plan'), specification: await observe(projectRoute + '/specification'), lens, screenshot, check, mutation });
  const acceptedAfterReview = (await observe(projectRoute + '/runs/' + accepted.id)).run;
  assert.deepEqual(acceptedAfterReview.input, accepted.input);
  assert.deepEqual(acceptedAfterReview.acceptance, accepted.acceptance, 'A later design observation must not change what the earlier run accepted');
  receipt.finalState = { acceptedRunID: accepted.id, historicalAcceptedReleaseID: release.id, currentReplacementReleaseID: replacementRelease.id, designRevision: designDraft.revision, designStatus: designDraft.status, pendingWork: 'The new empty-submission observation is a draft awaiting design review and subsequent implementation; it is not part of the earlier acceptance.' };

  stage = 'composer-recovery-before-admission-fault';
  receipt.orchestratorRecovery = await exerciseOrchestratorRecovery({ page, projectId: project.id, projectRoute, lens, mutation, observe, check, screenshot, modelCalls: () => providerCalls });

  assert.deepEqual(browserErrors, []); assert.equal(toolRound, 2); assert.equal(completedRounds, 2);
  receipt.providerCalls = providerCalls; receipt.originalAcceptanceTestSHA256 = sha(testSource); receipt.amendedAcceptanceTestSHA256 = sha(amendedTestSource); receipt.passed = true;
} catch (error) {
  receipt.failedStage = stage; receipt.failure = scrub(error.stack ?? error); process.exitCode = 1; console.error(receipt.failure);
  for (const [name, target] of [['alice', alice], ['bob', bob]]) if (target) {
    await target.screenshot({ path: join(evidence, name + '-failure.png'), fullPage: true, mask: [target.locator('.xterm-screen'), target.locator('input[type=password]')] }).catch(() => {});
    writeFileSync(join(fixture, name + '-failure-dom.txt'), scrub(await target.locator('body').innerText().catch(() => 'Unavailable')), { mode: 0o600 });
  }
  writeFileSync(join(fixture, 'terminal-diagnostic.json'), scrub(JSON.stringify(Object.fromEntries(terminalTextBySession), null, 2)), { mode: 0o600 });
} finally {
  releaseHeldModel?.();
  if (browser) await browser.close().catch(() => {});
  // Stop the private control service and its supervised workers before resource
  // inspection. A lost browser response never authorizes deleting an unrelated resource.
  let stopped = false;
  try { stopped = await drainOwnedService(); } catch (error) { receipt.cleanup.push(scrub(error.message)); }
  if (!stopped) { receipt.cleanup.push('Fixture writers remain or cannot be verified; state and containers retained'); receipt.passed = false; process.exitCode = 1; }
  if (stopped && project && receipt.ownerID) {
    // Only fixture-owned main, account and this team's private runtime stores.
    const stores = [join(fixture, 'state/sessions'), join(fixture, 'state/control-plane/users', receipt.ownerID, 'runtime/state/sessions')];
    if (team) stores.push(join(fixture, 'state/control-plane/teams', team.id, 'runtime/state/sessions'));
    for (const state of stores) {
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

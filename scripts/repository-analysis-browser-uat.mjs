#!/usr/bin/env node
// Installed UI import → source question → retained human answer → explicit follow-up
// and reviewed draft. Real readonly Codex in Docker uses a deterministic local
// provider. This proves source/clarification/recovery plumbing, not model reasoning.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { createServer as httpServer } from 'node:http';
import { createServer as httpsServer } from 'node:https';
import { createServer as netServer } from 'node:net';
import { existsSync, lstatSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, readlinkSync, realpathSync, rmSync, statfsSync, writeFileSync } from 'node:fs';
import { basename, dirname, join, resolve, sep } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '../desktop/node_modules/playwright/index.mjs';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..'), args = process.argv.slice(2);
const option = (name, fallback) => { const i = args.indexOf(name); if (i < 0) return fallback; assert(args[i + 1] && !args[i + 1].startsWith('--'), 'Missing value for ' + name); return args[i + 1]; };
if (args.includes('--help')) {
  console.log('node scripts/repository-analysis-browser-uat.mjs --binary PATH --web PATH --image EXISTING_TRUSTED_IMAGE --chromium PATH --report PATH --provenance PATH [--keep-fixture]\nLinux-only installed UI repository import, actual readonly Codex analysis, question-only result, answer draft/reload, explicit follow-up with authoritative source/history, dropped accepted response recovery and explicit draft application. Account/Git/provider setup uses API; all claimed project actions use the UI. Private HTTPS Git/Responses fixtures, one sandbox at a time, no external models or charges. Requires an exclusive runtime window. Report directory must be new or empty.');
  process.exit(0);
}
for (const name of ['--binary', '--web', '--image', '--chromium', '--report', '--provenance']) assert(option(name), 'Required: ' + name);
assert.equal(process.platform, 'linux', 'Process ownership checks require Linux /proc');
assert.notEqual(process.getuid(), 0, 'Run acceptance as the ordinary workspace user');
const image = option('--image');
const docker = (...values) => execFileSync('docker', values, { encoding: 'utf8', timeout: 30000 }).trim();
const imageInfo = JSON.parse(docker('image', 'inspect', image))[0];
const bridge = docker('network', 'inspect', 'bridge', '--format', '{{(index .IPAM.Config 0).Gateway}}');
assert.match(bridge, /^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.)/);
assert.equal(docker('ps', '-q'), '', 'This runner requires an exclusive, empty Docker runtime window');
const binary = realpathSync(resolve(option('--binary'))), web = realpathSync(resolve(option('--web'))), executable = realpathSync(resolve(option('--chromium')));
const report = resolve(option('--report')), evidence = dirname(report);
mkdirSync(dirname(evidence), { recursive: true, mode: 0o700 });
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Preserve earlier evidence; use a new or empty report directory');
else mkdirSync(evidence, { mode: 0o700 });
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const binaryHash = sha(readFileSync(binary)), webHash = sha(readFileSync(join(web, 'index.html')));
const provenancePath = resolve(option('--provenance'));
assert(provenancePath, 'Supply matching --provenance or package-provenance.json beside the report directory');
const provenanceBytes = readFileSync(provenancePath), provenance = JSON.parse(provenanceBytes);
assert.equal(provenance.binarySHA256, binaryHash, 'Package provenance binary hash mismatch');
assert.equal(provenance.webIndexSHA256, webHash, 'Package provenance web hash mismatch');
assert.equal(provenance.sourceDirty, false, 'Use a clean packaged candidate');
assert.match(provenance.sourceCommit, /^[a-f0-9]{40}$/);
function webInventory(root) {
  const files = [];
  function visit(directory, prefix = '') { for (const name of readdirSync(directory).sort()) {
    const file = join(directory, name), relative = prefix + name, info = lstatSync(file);
    if (info.isDirectory()) visit(file, relative + '/');
    else { assert(info.isFile() && !info.isSymbolicLink(), 'Web assets must be ordinary files'); files.push({ path: relative, sha256: sha(readFileSync(file)) }); }
  } }
  visit(root); return files;
}
const webFiles = webInventory(web);
assert(provenance.archive && provenance.archiveSHA256, 'Package provenance must identify its verified portable archive');
const archiveMembers = JSON.parse(execFileSync('python3', ['-c', `import hashlib,io,json,pathlib,sys,tarfile
raw=pathlib.Path(sys.argv[1]).read_bytes()
if hashlib.sha256(raw).hexdigest()!=sys.argv[2]: raise ValueError("Package archive changed since provenance was recorded")
result={"web":{},"binary":None}
with tarfile.open(fileobj=io.BytesIO(raw),mode="r:gz") as source:
 for row in source:
  p=pathlib.PurePosixPath(row.name)
  if p.is_absolute() or ".." in p.parts: raise ValueError("Unsafe archive path")
  parts=p.parts
  if not parts or not (parts[0]=="web" or str(p)=="luci"): continue
  if row.isdir(): continue
  if not row.isfile(): raise ValueError("Linked or special package member")
  digest=hashlib.sha256()
  with source.extractfile(row) as stream:
   while block:=stream.read(1024*1024): digest.update(block)
  if str(p)=="luci":
   if result["binary"] is not None: raise ValueError("Duplicate package binary")
   result["binary"]=digest.hexdigest()
  else:
   name=str(pathlib.PurePosixPath(*parts[1:]))
   if name in result["web"]: raise ValueError("Duplicate web member")
   result["web"][name]=digest.hexdigest()
print(json.dumps(result))`, provenance.archive, provenance.archiveSHA256], { encoding: 'utf8', timeout: 60000, maxBuffer: 8 * 1024 * 1024, stdio: ['ignore', 'pipe', 'pipe'] }));
assert.equal(archiveMembers.binary, binaryHash, 'Installed binary differs from the verified package archive');
assert.deepEqual(Object.fromEntries(webFiles.map(file => [file.path, file.sha256])), archiveMembers.web, 'Installed web files differ from the verified package archive');
writeFileSync(join(evidence, 'package-provenance.json'), provenanceBytes, { mode: 0o600, flag: 'wx' });
writeFileSync(join(evidence, 'driver.mjs'), readFileSync(fileURLToPath(import.meta.url)), { mode: 0o600, flag: 'wx' });
const fixture = mkdtempSync(join(tmpdir(), 'luci-repository-analysis-'));
for (const folder of ['home', 'state', 'projects', 'bin', 'remote', 'tls', 'empty-ca', 'docker']) mkdirSync(join(fixture, folder), { mode: 0o700 });
const secrets = new Set(), secret = () => { const value = randomBytes(32).toString('base64url'); secrets.add(value); return value; };
const password = secret(), lifecycle = secret(), key = secret();
const scrub = value => { let text = String(value?.message ?? value); for (const item of secrets) text = text.replaceAll(item, '[redacted]'); return text; };
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit, sourceDirty: false,
  package: { path: dirname(binary), archiveSHA256: provenance.archiveSHA256, provenanceSHA256: sha(provenanceBytes), verifiedArchiveWebFiles: webFiles.length, binaryAndAllWebFilesMatchArchive: true },
  binarySHA256: binaryHash, webIndexSHA256: webHash, webInventorySHA256: sha(JSON.stringify(webFiles)), driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  driverCheckout: { commit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repo, encoding: 'utf8' }).trim(), dirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: repo, encoding: 'utf8' }).trim() },
  scope: 'Installed UI source import, question-only real analysis, retained human clarification, lost accepted-response recovery and explicit reviewed draft application. The local provider is deterministic and does not establish model reasoning quality.',
  limitations: ['Local public HTTPS repository only; no private Git authentication claim.', 'Two readonly provider jobs, no implementation, tests, deployment, cloud resources, external models or charges.', 'Lost-response fault occurs after actual backend 202 acceptance, not before admission; recovery uses original-key GET, never a second send.', 'Successful source/definition CAS application is covered; source-drift rejection remains separately tested.'],
  setupScaffolding: ['Private account, Git author identity, unrelated empty navigation seed and named local provider via API', 'Two-commit Git fixture, private CA and real HTTPS Git launcher; TLS verification stays enabled'],
  image: { reference: image, id: imageInfo.Id },
  checks: [], importRequests: [], analysisRequests: [], recoveryReads: [], browserMutations: [], gitHTTPRequests: [], cleanup: [],
  browserErrors: [], externalRequests: [], externalModelCalls: 0, cloudResourcesCreated: 0,

};
const check = (name, details = {}) => { receipt.checks.push({ name, ...details }); console.log('PASS ' + name); };
const inherited = Object.fromEntries(Object.entries(process.env).filter(([name]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ)$/.test(name)));
const gitEnv = { ...inherited, HOME: join(fixture, 'home'), GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1', GIT_TERMINAL_PROMPT: '0' };
const realGit = realpathSync(execFileSync('which', ['git'], { env: inherited, encoding: 'utf8' }).trim());
const git = (directory, ...values) => execFileSync(realGit, ['-c', 'maintenance.auto=false', '-c', 'gc.auto=0', ...values], { cwd: directory, env: gitEnv, encoding: 'utf8', timeout: 15000, stdio: ['ignore', 'pipe', 'pipe'] }).trim();
const cloneLog = join(fixture, 'clone-events.jsonl');
const cloneEvents = () => existsSync(cloneLog) ? readFileSync(cloneLog, 'utf8').trim().split('\n').filter(Boolean).map(row => JSON.parse(row)) : [];
const starts = () => cloneEvents().filter(row => row.event === 'start');
let service, browser, page, remote, provider, base, token, environment, spawnError, serviceLog = '', stage = 'fixture', interceptError, project, projectRoute, ownerID, connectionID, connectionRevision, panel;
let providerError, providerCalls = 0, releaseProvider, providerWaiting = false, expectedDefinition, firstJob, acceptedReply;
const sessions = new Set(), providerSockets = new Set(), prompts = [];
const remoteSockets = new Set();
async function freePort() { const server = netServer(); await new Promise((done, reject) => server.once('error', reject).listen(0, '127.0.0.1', done)); const port = server.address().port; await new Promise(done => server.close(done)); return port; }
async function api(method, route, body, expected = 200, auth = token) {
  const response = await fetch(base + route, { method, headers: { 'Content-Type': 'application/json', ...(auth ? { Authorization: 'Bearer ' + auth } : {}) }, body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(15000) });
  if (response.status !== expected) {
    const error = await response.json().catch(() => ({}));
    receipt.apiErrors ??= []; receipt.apiErrors.push({ method, route, status: response.status, expected, code: typeof error.error === 'string' ? scrub(error.error) : undefined, message: typeof error.message === 'string' ? scrub(error.message).slice(0, 4000) : undefined });
  }
  assert.equal(response.status, expected, method + ' ' + route);
  const data = await response.json(); if (data.token) secrets.add(data.token); return data;
}
function processIdentity(pid) {
  try { const stat = readFileSync(`/proc/${pid}/stat`, 'utf8'), fields = stat.slice(stat.lastIndexOf(')') + 2).trim().split(/\s+/); return { pid: Number(pid), start: fields[19], group: Number(fields[2]), status: fields[0] }; }
  catch (error) { if (['ENOENT', 'ESRCH'].includes(error.code)) return null; throw error; }
}
const stillOwned = identity => { const current = processIdentity(identity.pid); return current && current.start === identity.start && current.status !== 'Z'; };
function ownedProcesses() {
  const found = [], state = join(fixture, 'state');
  for (const pid of readdirSync('/proc').filter(name => /^\d+$/.test(name))) {
    const identity = processIdentity(pid); if (!identity || identity.status === 'Z') continue;
    try {
      const values = Object.fromEntries(readFileSync(`/proc/${pid}/environ`, 'utf8').split('\0').filter(Boolean).map(entry => { const split = entry.indexOf('='); return [entry.slice(0, split), entry.slice(split + 1)]; }));
      if (!(values.LUCI_STATE_DIR === state || values.LUCI_STATE_DIR?.startsWith(state + '/control-plane/'))) continue;
      const executable = readlinkSync(`/proc/${pid}/exe`);
      // The fresh state identity binds the gateway, separate worker groups and
      // any in-flight Git descendants. Also pin executable and start tick.
      if (processIdentity(pid)?.start === identity.start) found.push({ ...identity, executable });
    } catch (error) { if (!['ENOENT', 'ESRCH', 'EACCES', 'EPERM'].includes(error.code)) throw error; }
  }
  return found;
}
async function stopService() {
  if (!service?.pid) return true;
  const tracked = ownedProcesses();
  if (service.exitCode === null) {
    assert(tracked.some(row => row.pid === service.pid && row.executable === binary), 'Cannot prove ownership of the service to stop');
    try {
      const response = await fetch(base + '/v1/control-plane/shutdown', { method: 'POST', headers: { 'X-Luci-Lifecycle-Token': lifecycle }, signal: AbortSignal.timeout(5000) });
      assert.equal(response.status, 200, 'Fixture shutdown refused');
    } catch (error) {
      receipt.cleanupErrors ??= []; receipt.cleanupErrors.push('Lifecycle shutdown failed: ' + scrub(error));
      const gateway = tracked.find(row => row.pid === service.pid);
      if (gateway && stillOwned(gateway)) service.kill('SIGTERM');
    }
  }
  const deadline = Date.now() + 20000;
  // /proc can report a zombie before libuv delivers ChildProcess's exit event.
  // Wait for both the kernel ownership proof and the actual collected exit code.
  const awaitingExit = () => service.exitCode === null && service.signalCode === null;
  while (Date.now() < deadline && (awaitingExit() || tracked.some(stillOwned) || ownedProcesses().length)) await delay(100);
  const remaining = [...tracked.filter(stillOwned), ...ownedProcesses()];
  if (remaining.length) {
    receipt.cleanupErrors ??= []; receipt.cleanupErrors.push('Service or Git worker needed a forced stop');
    for (const row of remaining) {
      try { if (stillOwned(row) && readlinkSync(`/proc/${row.pid}/exe`) === row.executable) process.kill(row.group === row.pid ? -row.pid : row.pid, 'SIGKILL'); }
      catch (error) { if (!['ENOENT', 'ESRCH'].includes(error.code)) throw error; }
    }
    const forcedDeadline = Date.now() + 5000; while (Date.now() < forcedDeadline && (awaitingExit() || tracked.some(stillOwned) || ownedProcesses().length)) await delay(100);
  }
  const stopped = !tracked.some(stillOwned) && ownedProcesses().length === 0;
  receipt.cleanup.push({ serviceAndGitProcesses: tracked.length, allStopped: stopped, exitCode: service.exitCode });
  if (service.exitCode !== 0) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push('Owned service did not exit normally'); }
  assert(stopped, 'An owned process is still running'); return stopped;
}
const focus = 'Understand how notes are saved and clarify who should be able to read them before proposing a definition.';
const question = 'Should saved garden notes be private to invited collaborators or publicly readable?';
const choice = 'Only invited collaborators should read and add notes.';
const answer = choice + ' Preserve existing plain-text notes and require sign-in for every read and write; never publish them automatically.';
const purpose = 'A private garden journal where signed-in invited collaborators save and read notes while existing plain-text notes remain readable.';
let expectedFiles;
function stringsIn(value, output = []) { if (typeof value === 'string') output.push(value); else if (Array.isArray(value)) value.forEach(item => stringsIn(item, output)); else if (value && typeof value === 'object') Object.values(value).forEach(item => stringsIn(item, output)); return output; }
function readPrompt(body) {
  for (const text of stringsIn(body.input)) {
    const index = text.indexOf('\n\n{"snapshot":');
    if (index < 0) continue;
    const value = JSON.parse(text.slice(index + 2)); if (value.snapshot?.source && typeof value.userMessage === 'string') return value;
  }
  throw new Error('Installed CLI did not receive the canonical repository snapshot prompt');
}
function schemaValue(schema, value) {
  // Fill only schema-required absent fields; this fixture returns an actual
  // complete proposal while preserving the authoritative input provenance.
  if (schema.anyOf) return value === null ? null : schemaValue(schema.anyOf.find(item => item.type !== 'null'), value);
  if (schema.type === 'object') return Object.fromEntries(Object.entries(schema.properties).map(([name, child]) => [name, schemaValue(child, value?.[name])]));
  if (schema.type === 'array') return (value ?? []).map(item => schemaValue(schema.items, item));
  if (value !== undefined) return value;
  return schema.type === 'integer' ? 0 : schema.type === 'boolean' ? false : '';
}
function sendFixtureText(res, text) {
  const item = { id: `msg_repository_${providerCalls}`, type: 'message', status: 'completed', role: 'assistant', content: [{ type: 'output_text', text, annotations: [] }] };
  const response = { id: `resp_repository_${providerCalls}`, object: 'response', created_at: 1, status: 'completed', model: 'repository-fixture', output: [item], usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2, input_tokens_details: { cached_tokens: 0 }, output_tokens_details: { reasoning_tokens: 0 } } };
  const events = [{ type: 'response.created', response: { ...response, status: 'in_progress', output: [] } }, { type: 'response.output_item.added', output_index: 0, item: { ...item, status: 'in_progress', content: [] } }, { type: 'response.content_part.added', item_id: item.id, output_index: 0, content_index: 0, part: { type: 'output_text', text: '', annotations: [] } }, { type: 'response.output_text.delta', item_id: item.id, output_index: 0, content_index: 0, delta: text }, { type: 'response.output_text.done', item_id: item.id, output_index: 0, content_index: 0, text }, { type: 'response.output_item.done', output_index: 0, item }, { type: 'response.completed', response }];
  res.writeHead(200, { 'content-type': 'text/event-stream' }); res.end(events.map((event, sequence_number) => `event: ${event.type}\ndata: ${JSON.stringify({ ...event, sequence_number })}\n\n`).join(''));
}
async function startProvider() {
  provider = httpServer(async (req, res) => {
    try {
      assert.equal(req.method, 'POST'); assert.equal(req.url, '/v1/responses'); assert.equal(req.headers.authorization, 'Bearer ' + key);
      const parts = []; let size = 0; for await (const part of req) { size += part.length; assert(size < 4 * 1024 * 1024); parts.push(part); }
      const body = JSON.parse(Buffer.concat(parts).toString()); providerCalls++; assert(providerCalls <= 2, 'Unexpected duplicate provider inference'); assert.equal(body.model, 'repository-fixture');
      const tools = (body.tools ?? []).map(tool => tool.name ?? tool.function?.name ?? tool.type);
      for (const forbidden of ['exec_command', 'shell', 'apply_patch', 'web_search', 'web_search_preview']) assert(!tools.includes(forbidden), 'Read-only analysis exposed ' + forbidden);
      const prompt = readPrompt(body), snapshot = prompt.snapshot;
      assert.deepEqual(snapshot.specification, expectedDefinition, 'Provider definition differs from the current saved version');
      assert.equal(snapshot.source.mode, 'commit'); assert.equal(snapshot.source.partial, true);
      assert.equal(snapshot.source.commit, receipt.imported.commits.at(-1));
      for (const [path, content] of Object.entries(expectedFiles)) {
        assert.deepEqual(snapshot.files.find(file => file.path === path), { path, content }, 'Selected provider did not receive exact imported source');
        const record = snapshot.source.files.find(file => file.path === path); assert.equal(record.digest, sha(content)); assert.equal(record.bytes, Buffer.byteLength(content));
      }
      assert.equal(snapshot.files.length, Object.keys(expectedFiles).length, 'Unexpected file entered the source fixture');
      assert.equal(prompt.userMessage, providerCalls === 1 ? focus : answer);
      if (providerCalls === 1) assert.equal(snapshot.followUp, undefined);
      else {
        assert(firstJob); const prior = snapshot.followUp;
        assert.equal(prior.analysisId, firstJob.id); assert.deepEqual(prior.source, firstJob.source); assert.deepEqual(prior.result, firstJob.result);
        assert.equal(prior.userMessage, focus); assert.equal(prior.sourceChanged, false); assert.deepEqual(prior.earlier ?? [], []);
        assert.equal(snapshot.source.digest, firstJob.source.digest);
      }
      const result = { question: providerCalls === 1 ? question : '', options: providerCalls === 1 ? [choice, 'Allow anyone to read saved notes.'] : [],
        rationale: providerCalls === 1 ? 'This bounded, partial source sample saves and lists notes, but does not establish the intended sharing boundary.' : 'This bounded, partial source sample lacks access control. The proposed draft records the explicit human answer; it does not claim that behavior already exists.',
        findings: [{ kind: 'observed', summary: 'The existing implementation trims, saves and lists plain-text notes in memory.', paths: ['notes.cjs'] }, { kind: 'gap', summary: 'The source has no sign-in or invitation checks; these are proposed future requirements, not implemented behavior.', paths: ['notes.cjs'] }], proposal: null };
      if (providerCalls === 2) result.proposal = { ...structuredClone(snapshot.specification), status: 'draft', startingPoint: 'existing', purpose, audience: 'Signed-in invited garden collaborators.', constraints: answer,
        requirements: [...snapshot.specification.requirements, { id: 'AC-private-notes', title: 'Read and write notes only as an invited collaborator', priority: 'must', acceptance: 'A signed-in invited collaborator can save and retrieve a note. An uninvited or signed-out person cannot read or write notes. Existing plain-text notes remain readable to invited collaborators.' }] };
      const schema = body.text?.format?.schema; assert(schema, 'Installed CLI omitted its strict result schema');
      const structured = schemaValue(schema, result);
      prompts.push({ call: providerCalls, snapshot, userMessage: prompt.userMessage, result: structured, tools });
      writeFileSync(join(evidence, `provider-contract-${providerCalls}.json`), JSON.stringify(prompts.at(-1), null, 2), { mode: 0o600 });
      providerWaiting = true; await new Promise(done => { releaseProvider = done; }); providerWaiting = false; releaseProvider = undefined;
      sendFixtureText(res, JSON.stringify(structured));
    } catch (error) { providerError = error; if (!res.headersSent) res.writeHead(500); res.end('Local fixture contract failed'); }
  });
  provider.on('connection', socket => { providerSockets.add(socket); socket.once('close', () => providerSockets.delete(socket)); });
  await new Promise((done, reject) => provider.once('error', reject).listen(0, bridge, done));
}
async function until(label, test, timeout = 120000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) {
    if (spawnError) throw spawnError; if (providerError) throw providerError; if (interceptError) throw interceptError;
    if (service && service.exitCode !== null) throw new Error(`Service exited during ${label}: ${service.exitCode}`);
    const value = await test(); if (value) return value; await delay(100);
  }
  throw new Error(label + ' timed out');
}
async function screenshot(name) { await page.screenshot({ path: join(evidence, name + '.png'), fullPage: true }); }
// Capture both the natural post-action viewport and the result after ordinary
// focus/scroll navigation. A non-visible answer after option selection remains
// evidence of discoverability limits; the driver never relabels it as visible.
async function observeAnalysisLayout(name, kind) {
  const original = page.viewportSize();
  const target = kind === 'question' ? panel.getByRole('button', { name: 'Use answer: ' + choice, exact: true }) : panel.getByRole('textbox', { name: 'Your answer', exact: true });
  const read = () => panel.evaluate(element => {
    const rect = node => { const r = node.getBoundingClientRect(); return { x: r.x, y: r.y, width: r.width, height: r.height }; };
    const describe = node => {
      if (!node) return null;
      const box = rect(node); let left = 0, top = 0, right = innerWidth, bottom = innerHeight;
      for (let ancestor = node.parentElement; ancestor; ancestor = ancestor.parentElement) {
        const style = getComputedStyle(ancestor), r = ancestor.getBoundingClientRect();
        if (/(auto|scroll|hidden|clip)/.test(style.overflowX)) { left = Math.max(left, r.left); right = Math.min(right, r.right); }
        if (/(auto|scroll|hidden|clip)/.test(style.overflowY)) { top = Math.max(top, r.top); bottom = Math.min(bottom, r.bottom); }
      }
      return { ...box, intersectsVisibleArea: box.width > 0 && box.height > 0 && box.x < right && box.x + box.width > left && box.y < bottom && box.y + box.height > top,
        fullyInVisibleArea: box.x >= left - 1 && box.x + box.width <= right + 1 && box.y >= top - 1 && box.y + box.height <= bottom + 1 };
    };
    const buttons = [...element.querySelectorAll('button')], label = node => node?.getAttribute('aria-label') || (node?.tagName === 'TEXTAREA' ? 'Your answer' : node?.textContent?.trim()?.slice(0, 100)) || null;
    return { viewport: { width: innerWidth, height: innerHeight }, documentHorizontalOverflow: document.documentElement.scrollWidth - innerWidth,
      question: describe(element.querySelector('[aria-label="Saved analysis question"]')), firstOption: describe(buttons.find(node => node.textContent.startsWith('Use answer:'))),
      answer: describe(element.querySelector('textarea')), send: describe(buttons.find(node => node.textContent === 'Send answer')),
      active: { tag: document.activeElement?.tagName, label: label(document.activeElement), insidePanel: element.contains(document.activeElement) } };
  });
  try {
    for (const width of [1440, 390]) {
      await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
      await page.evaluate(() => new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done))));
      const beforeNavigation = await read();
      await screenshot(name + '-' + width + '-before-navigation');
      await target.scrollIntoViewIfNeeded(); await target.focus();
      const focused = await read();
      assert.equal(focused.active.insidePanel, true, 'Question/answer focus escaped its analysis panel');
      const geometry = kind === 'question' ? focused.firstOption : focused.answer;
      assert(geometry && geometry.intersectsVisibleArea, 'Focused analysis control is not visible');
      await screenshot(name + '-' + width + '-focused');
      await page.keyboard.press('Tab'); const afterTab = await read();
      (receipt.layoutObservations ??= []).push({ state: name, kind, width, beforeNavigation, focused, afterTab, scope: 'Actual rendered geometry and one forward Tab; no automated claim of novice discoverability or complete accessibility conformance.' });
    }
  } finally { await page.setViewportSize(original); }
}
async function openAnalysis() {
  await page.getByRole('treeitem', { name: /garden-repository-review$/ }).click();
  await page.getByRole('button', { name: 'Define', exact: true }).click();
  await page.getByRole('tab', { name: 'Repository analysis', exact: true }).click();
  panel = page.getByRole('region', { name: 'Repository analysis', exact: true });
  await panel.getByRole('heading', { name: 'Understand the existing application', exact: true }).waitFor();
}
async function recoveryRecords() { return page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith('luci.repository-analysis.v1:')).map(([key, value]) => ({ kind: key.endsWith(':pending') ? 'pending' : 'draft', value: JSON.parse(value).value }))); }
async function noDefinitionChange() { assert.deepEqual(await api('GET', projectRoute + '/specification'), expectedDefinition); }
function ownedAssistant(job) {
  assert.equal(job.projectId, project.id); assert.equal(job.kind, 'repository'); assert.equal(job.requestedBy, ownerID); assert.equal(job.ownRequest, true);
  assert.equal(job.harness, 'codex'); assert.equal(job.model, 'repository-fixture'); assert.equal(job.agentConnectionId, connectionID); assert.equal(job.agentConnectionRevision, connectionRevision); assert.equal(job.agentConnectionProtocol, 'responses');
  assert.match(job.sessionId, /^s-[a-f0-9]{8}$/); sessions.add(job.sessionId);
}
async function inspectReadonlyRuntime(job) {
  await until('Selected local provider receives the exact bounded source', () => providerWaiting, 180000);
  const ids = docker('ps', '--filter', `label=luci.session-id=${job.sessionId}`, '--format', '{{.ID}}').split(/\s+/).filter(Boolean); assert.equal(ids.length, 1);
  const container = JSON.parse(docker('inspect', ids[0]))[0]; assert.equal(container.Config.Labels['luci.session-id'], job.sessionId);
  assert(container.Mounts.some(mount => mount.Destination === '/luci-input' && mount.RW === false && realpathSync(mount.Source).startsWith(realpathSync(fixture) + sep)));
  assert(!container.Mounts.some(mount => resolve(mount.Source) === resolve(project.path) || resolve(mount.Source).startsWith(resolve(project.path) + sep)), 'Analysis must never mount editable project source');
  const workloads = docker('top', ids[0], '-eo', 'uid,pid,comm').split('\n').slice(1).map(line => { const [uid, , command] = line.trim().split(/\s+/, 3); return { uid: Number(uid), command }; }).filter(row => /^(sandboxd|python3?|node|codex)$/.test(row.command));
  assert(workloads.some(row => row.command === 'codex')); assert(workloads.every(row => row.uid === process.getuid() && row.uid !== 0));
  (receipt.runtimeChecks ??= []).push({ jobID: job.id, sessionID: job.sessionId, sourceMountAbsent: true, readonlyInput: true, workloads });
}
async function complete(job) {
  releaseProvider();
  const terminal = await until('Actual repository analysis completes', async () => { const value = await api('GET', `${projectRoute}/assist/${job.id}`); return ['succeeded', 'failed', 'cancelled'].includes(value.status) && value; }, 180000);
  assert.equal(terminal.status, 'succeeded', terminal.message); assert.equal(terminal.ownRequest, true);
  await until('UI observes this exact successful result', async () => await panel.getByLabel('Saved analysis', { exact: true }).inputValue() === job.id && !(await recoveryRecords()).some(row => row.kind === 'pending'), 30000);
  await until('Finished readonly sandbox is removed', () => docker('ps', '-a', '--filter', `label=luci.session-id=${job.sessionId}`, '--format', '{{.ID}}') === '', 30000);
  return terminal;
}
function diskGuard() { const disk = statfsSync('/workspace'); assert(disk.bavail * disk.bsize > 5_000_000_000, 'One vfs sandbox requires at least five GB of free workspace'); }
try {
  const tls = join(fixture, 'tls'), source = join(fixture, 'remote/source'); mkdirSync(source, { mode: 0o700 });
  const openssl = values => execFileSync('openssl', values, { cwd: tls, timeout: 15000, stdio: ['ignore', 'pipe', 'pipe'] });
  openssl(['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', 'ca.key', '-out', 'ca.pem', '-days', '1', '-subj', '/CN=Luci Import Fixture CA', '-addext', 'basicConstraints=critical,CA:TRUE', '-addext', 'keyUsage=critical,keyCertSign,cRLSign']);
  openssl(['req', '-newkey', 'rsa:2048', '-nodes', '-keyout', 'server.key', '-out', 'server.csr', '-subj', '/CN=127.0.0.1']);
  writeFileSync(join(tls, 'server.ext'), 'subjectAltName=IP:127.0.0.1\nbasicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\n', { mode: 0o600 });
  openssl(['x509', '-req', '-in', 'server.csr', '-CA', 'ca.pem', '-CAkey', 'ca.key', '-CAcreateserial', '-out', 'server.pem', '-days', '1', '-extfile', 'server.ext']);
  git(source, 'init', '--initial-branch=main');
  const expected = { files: {
    'README.md': '# Garden journal\nA small existing notes application. Notes can be saved and listed. The intended audience and sharing policy have not yet been decided.\n',
    'notes.cjs': "const notes = [];\nexports.save = text => { const clean = text.trim(); if (!clean) throw Error('A note is required'); notes.push(clean); return clean; };\nexports.list = () => [...notes];\n",
  }, history: [] };
  for (const [name, content] of Object.entries(expected.files)) writeFileSync(join(source, name), content);
  git(source, 'add', '.'); git(source, '-c', 'user.name=Analysis fixture', '-c', 'user.email=analysis@example.invalid', 'commit', '-m', 'Add the original garden note implementation');
  expected.files['README.md'] += 'Keep original notes readable when adding team access.\n'; writeFileSync(join(source, 'README.md'), expected.files['README.md']);
  git(source, 'add', '.'); git(source, '-c', 'user.name=Analysis fixture', '-c', 'user.email=analysis@example.invalid', 'commit', '-m', 'Record the compatibility expectation');
  expected.history = git(source, 'rev-list', '--reverse', 'HEAD').split('\n');
  expectedFiles = expected.files;
  git(join(fixture, 'remote'), 'clone', '--bare', source, 'source.git'); git(join(fixture, 'remote/source.git'), 'update-server-info');
  const bare = join(fixture, 'remote/source.git');
  remote = httpsServer({ key: readFileSync(join(tls, 'server.key')), cert: readFileSync(join(tls, 'server.pem')) }, (request, response) => {
    const url = new URL(request.url, 'https://fixture.invalid');
    receipt.gitHTTPRequests.push({ method: request.method, path: url.pathname, smartDiscovery: url.searchParams.get('service') === 'git-upload-pack' });
    if (url.pathname.startsWith('/missing.git/')) { response.writeHead(503); response.end('Owned fixture deliberately unavailable'); return; }
    const relative = url.pathname.slice('/source.git/'.length), file = resolve(bare, relative);
    if (request.method !== 'GET' || !url.pathname.startsWith('/source.git/') || !file.startsWith(bare + sep) || !existsSync(file) || !lstatSync(file).isFile()) { response.writeHead(404); response.end(); return; }
    response.writeHead(200, { 'content-type': 'application/octet-stream', 'cache-control': 'no-store' }); response.end(readFileSync(file));
  });
  remote.on('connection', socket => { remoteSockets.add(socket); socket.once('close', () => remoteSockets.delete(socket)); });
  await new Promise((done, reject) => remote.once('error', reject).listen(0, '127.0.0.1', done));
  const remoteBase = `https://127.0.0.1:${remote.address().port}`, goodURL = remoteBase + '/source.git', badURL = remoteBase + '/missing.git';
  receipt.fixture = { commits: expected.history, files: Object.keys(expected.files), caSHA256: sha(readFileSync(join(tls, 'ca.pem'))), gitVersion: git(source, '--version') };
  // Managed workers deliberately strip inherited Git variables. This owned
  // launcher changes only the child Git environment and keeps real Git/TLS on.
  const launcher = `#!${process.execPath}\nconst {execFileSync}=require('node:child_process');const{appendFileSync}=require('node:fs');const args=process.argv.slice(2),cloning=args.includes('clone');const event=(row)=>appendFileSync(${JSON.stringify(cloneLog)},JSON.stringify(row)+'\\n',{mode:0o600});if(cloning){const url=args.find(arg=>arg.startsWith('https://'));if(!${JSON.stringify([goodURL, badURL])}.includes(url)){event({event:'forbidden-remote'});process.exit(78);}event({event:'start',repository:url,cwd:process.cwd()});}const env={...process.env,GIT_SSL_CAINFO:${JSON.stringify(join(tls, 'ca.pem'))},GIT_SSL_CAPATH:${JSON.stringify(join(fixture, 'empty-ca'))},GIT_CONFIG_NOSYSTEM:'1',GIT_TERMINAL_PROMPT:'0'};delete env.GIT_SSL_NO_VERIFY;try{execFileSync(${JSON.stringify(realGit)},args,{stdio:'inherit',env});if(cloning)event({event:'finish',status:0});}catch(error){if(cloning)event({event:'finish',status:error.status??1});process.exit(error.status??1);}\n`;
  writeFileSync(join(fixture, 'bin/git'), launcher, { mode: 0o700 });
  const port = await freePort(), previewPort = await freePort(); base = `http://127.0.0.1:${port}`;
  await startProvider();
  writeFileSync(join(fixture, 'docker/config.json'), JSON.stringify({ cliPluginsExtraDirs: process.env.DOCKER_CONFIG ? [join(process.env.DOCKER_CONFIG, 'cli-plugins')] : [] }), { mode: 0o600 });
  environment = { ...inherited, PATH: join(fixture, 'bin') + ':' + inherited.PATH, HOME: join(fixture, 'home'), USERPROFILE: join(fixture, 'home'),
    LUCI_ADDR: `127.0.0.1:${port}`, LUCI_BROWSER_PREVIEW_ADDR: `127.0.0.1:${previewPort}`, LUCI_BROWSER_PREVIEW_PORT: String(previewPort), LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'), LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: join(dirname(binary), 'runtime'),
    ...(process.env.DOCKER_HOST ? { DOCKER_HOST: process.env.DOCKER_HOST, LUCI_DOCKER_HOST: process.env.DOCKER_HOST } : {}), DOCKER_CONFIG: join(fixture, 'docker'), LUCI_SANDBOX_IMAGE: image, LUCI_BACKEND: 'docker', LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, AWS_EC2_METADATA_DISABLED: 'true', GIT_CONFIG_GLOBAL: join(fixture, 'home/.gitconfig'), GIT_CONFIG_NOSYSTEM: '1' };
  stage = 'service-start'; service = spawn(binary, [], { cwd: fixture, env: environment, detached: true, stdio: ['ignore', 'pipe', 'pipe'] }); service.once('error', error => { spawnError = error; });
  for (const stream of [service.stdout, service.stderr]) stream.on('data', bytes => { serviceLog = (serviceLog + scrub(bytes.toString())).slice(-65536); });
  const readyBy = Date.now() + 30000; let status;
  while (Date.now() < readyBy) { if (spawnError) throw spawnError; assert.equal(service.exitCode, null, 'Service exited before readiness'); try { status = await api('GET', '/v1/control-plane/status', undefined, 200, ''); break; } catch { await delay(100); } }
  assert(status, 'Service never became ready'); receipt.version = status.version;
  const bootstrap = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(); secrets.add(bootstrap);
  const owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'import-owner', displayName: 'Import owner', password }, 201, ''); token = owner.token; ownerID = owner.user.id; receipt.ownerID = ownerID;
  await api('POST', '/v1/setup/git/identity', { name: 'Import owner', email: 'import@example.invalid' });
  await api('POST', '/v1/product/projects', { name: 'navigation-setup-only' }, 201);
  const catalogue = await api('POST', '/v1/setup/agent-connections', { action: 'save', expectedRevision: 0, name: 'Local repository clarification fixture', kind: 'compatible-api', enabled: true, models: { codex: 'repository-fixture' }, endpoints: { responses: `http://host.docker.internal:${provider.address().port}/v1` }, apiKey: key });
  connectionID = catalogue.connections[0].id; connectionRevision = catalogue.connections[0].revision; assert(Number.isInteger(connectionRevision)); receipt.agentConnectionId = connectionID; assert(!JSON.stringify(catalogue).includes(key));
  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited }); receipt.browserVersion = browser.version();
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, serviceWorkers: 'block' });
  await context.route('**/*', route => { const url = new URL(route.request().url()); if (url.origin !== base) { receipt.externalRequests.push({ method: route.request().method(), origin: url.origin, path: url.pathname }); return route.abort('blockedbyclient'); } return route.continue(); });
  page = await context.newPage(); page.setDefaultTimeout(15000);
  page.on('pageerror', error => receipt.browserErrors.push(scrub(error)));
  page.on('request', request => { const url = new URL(request.url());
    if (url.origin !== base) return;
    if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(request.method())) receipt.browserMutations.push({ method: request.method(), path: url.pathname });
    if (request.method() === 'POST' && url.pathname === '/v1/product/projects') receipt.importRequests.push(request.postDataJSON());
    if (request.method() === 'POST' && url.pathname.endsWith('/assist/repository')) receipt.analysisRequests.push(request.postDataJSON());
    if (request.method() === 'GET' && url.pathname.includes('/assist/repository/requests/')) receipt.recoveryReads.push(url.pathname);
  });
  stage = 'ui-import';
  await page.goto(base); await page.getByLabel('Username', { exact: true }).fill('import-owner'); await page.getByLabel('Password', { exact: true }).fill(password); await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.getByRole('button', { name: 'Create project', exact: true }).click();
  let dialog = page.getByRole('dialog'); await dialog.getByRole('navigation', { name: 'Project source' }).getByRole('button', { name: 'Import repository', exact: true }).click();
  dialog = page.getByRole('dialog', { name: 'Import a repository', exact: true });
  await dialog.getByLabel('Project name', { exact: true }).fill('garden-repository-review'); await dialog.getByRole('textbox', { name: /^Repository URL/ }).fill(goodURL);
  await dialog.getByRole('checkbox', { name: 'I trust the source of this repository.', exact: true }).check();
  const importedResponse = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === '/v1/product/projects');
  await dialog.locator('form').getByRole('button', { name: 'Import repository', exact: true }).click();
  const imported = await importedResponse; assert.equal(imported.status(), 201); project = await imported.json(); projectRoute = '/v1/product/projects/' + project.id;
  await dialog.waitFor({ state: 'hidden' }); assert.equal(basename(project.path), 'garden-repository-review'); assert(realpathSync(project.path).startsWith(realpathSync(fixture) + sep));
  assert.deepEqual(git(project.path, 'rev-list', '--reverse', 'HEAD').split('\n'), expected.history);
  assert.deepEqual(git(project.path, 'ls-files').split('\n'), Object.keys(expected.files).sort());
  for (const [path, content] of Object.entries(expected.files)) assert.equal(readFileSync(join(project.path, path), 'utf8'), content);
  receipt.imported = { projectID: project.id, commits: expected.history, files: Object.entries(expected.files).map(([path, content]) => ({ path, sha256: sha(content) })) };
  await openAnalysis(); expectedDefinition = await api('GET', projectRoute + '/specification');
  await screenshot('01-imported-source-analysis-entry');
  check('UI imports the actual HTTPS repository with both commits and unchanged source, then opens repository analysis', receipt.imported);

  stage = 'first-real-analysis-question';
  await panel.getByLabel('Source to analyse', { exact: true }).selectOption('commit');
  await panel.getByLabel('Analysis provider', { exact: true }).selectOption('codex');
  await panel.getByLabel('Analysis connection', { exact: true }).selectOption({ label: 'Local repository clarification fixture' });
  await until('Selected saved model appears', async () => await panel.getByLabel('Analysis model', { exact: true }).inputValue() === 'repository-fixture', 10000);
  await panel.getByRole('textbox', { name: 'What should the analysis focus on?', exact: true }).fill(focus);
  const consent = () => panel.getByRole('checkbox', { name: 'Send the selected source and definition to this provider for analysis.', exact: true });
  assert.equal(await panel.getByRole('button', { name: 'Analyse repository', exact: true }).isEnabled(), false);
  await consent().check(); diskGuard();
  const initialResponse = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === projectRoute + '/assist/repository');
  await panel.getByRole('button', { name: 'Analyse repository', exact: true }).click(); const started = await initialResponse;
  assert.equal(started.status(), 202); assert.equal(started.request().postDataJSON().expectedAgentConnectionRevision, connectionRevision); let first = await started.json(); ownedAssistant(first);
  await inspectReadonlyRuntime(first); await noDefinitionChange(); firstJob = await complete(first);
  assert.equal(firstJob.result.proposal, null); assert.equal(firstJob.result.question, question); assert.equal(firstJob.userMessage, focus);
  await panel.getByRole('region', { name: 'Saved analysis question', exact: true }).getByText(question, { exact: true }).waitFor();
  assert.equal(await panel.getByRole('button', { name: 'Apply as definition draft', exact: true }).count(), 0);
  await noDefinitionChange(); await screenshot('02-question-only-result-and-source');
  await observeAnalysisLayout('02-question-options', 'question');
  check('Actual selected Codex in readonly Docker receives exact imported source and returns a visible question-only result without applying or inventing an implementation', { jobID: firstJob.id, source: firstJob.source, result: firstJob.result, provider: firstJob.harness, model: firstJob.model, connectionID });

  stage = 'retained-human-answer';
  const requestsBeforeAnswer = receipt.analysisRequests.length, callsBeforeAnswer = providerCalls;
  await panel.getByRole('button', { name: 'Use answer: ' + choice, exact: true }).click();
  assert.equal(await panel.getByRole('textbox', { name: 'Your answer', exact: true }).inputValue(), choice);
  assert.equal(await panel.getByRole('button', { name: 'Send answer', exact: true }).isEnabled(), false);
  await panel.getByRole('textbox', { name: 'Your answer', exact: true }).fill(answer);
  await until('Exact edited answer is durably saved in this browser', async () => (await recoveryRecords()).some(row => row.kind === 'draft' && row.value.message === answer && row.value.replyToAnalysisId === firstJob.id && row.value.agentConnectionId === connectionID));
  assert.equal(await consent().isChecked(), false); await screenshot('03-unsent-edited-answer');
  await observeAnalysisLayout('03-editable-answer', 'answer');
  await page.reload(); await openAnalysis();
  await until('Original question and human answer restored', async () => await panel.getByRole('textbox', { name: 'Your answer', exact: true }).inputValue() === answer && await panel.getByLabel('Saved analysis', { exact: true }).inputValue() === firstJob.id);
  await panel.getByRole('region', { name: 'Saved analysis question', exact: true }).getByText(question, { exact: true }).waitFor();
  assert.equal(await panel.getByLabel('Source to analyse', { exact: true }).inputValue(), 'commit'); assert.equal(await panel.getByLabel('Analysis provider', { exact: true }).inputValue(), 'codex');
  assert.equal(await panel.getByLabel('Analysis connection', { exact: true }).inputValue(), connectionID); assert.equal(await panel.getByLabel('Analysis model', { exact: true }).inputValue(), 'repository-fixture');
  assert.equal(await consent().isChecked(), false); assert.equal(receipt.analysisRequests.length, requestsBeforeAnswer); assert.equal(providerCalls, callsBeforeAnswer); await noDefinitionChange();
  await screenshot('04-answer-and-authoritative-question-after-reload');
  await observeAnalysisLayout('04-reloaded-answer', 'answer');
  check('Choosing an option only fills the draft; the edited human answer and its original question/provider survive actual reload with zero sends or inference', { originalQuestion: question, chosenOption: choice, restoredAnswer: answer, replyToAnalysisID: firstJob.id, providerCallDelta: 0 });

  stage = 'explicit-follow-up-lost-accepted-response';
  let dropCount = 0;
  await page.route(base + projectRoute + '/assist/repository', async route => {
    if (route.request().method() !== 'POST') return route.fallback();
    try {
      dropCount++; assert.equal(dropCount, 1, 'Recovery issued a duplicate analysis POST');
      const submitted = route.request().postDataJSON(); assert.equal(submitted.replyToAnalysisId, firstJob.id); assert.equal(submitted.message, answer);
      assert.equal(submitted.expectedRevision, expectedDefinition.revision); assert.equal(submitted.expectedBriefRevision, expectedDefinition.briefRevision); assert.equal(submitted.agentConnectionId, connectionID); assert.equal(submitted.expectedAgentConnectionRevision, connectionRevision); assert.equal(submitted.model, 'repository-fixture'); assert.equal(submitted.acknowledgeSourceRead, true);
      assert.match(submitted.idempotencyKey, /^analysis-[a-f0-9-]{36}$/);
      const response = await route.fetch({ timeout: 45000, maxRetries: 0, maxRedirects: 0 }); assert.equal(response.status(), 202);
      acceptedReply = await response.json(); ownedAssistant(acceptedReply); receipt.lostResponse = { fault: 'Browser fetched the actual 202 admission response once, then aborted delivery. Provider response is held for recovery; no backend result was fabricated.', actualStatus: response.status(), jobID: acceptedReply.id, idempotencyKey: submitted.idempotencyKey, originalRequest: submitted };
      assert.equal(acceptedReply.idempotencyKey, submitted.idempotencyKey); await route.abort('failed');
    } catch (error) { interceptError = error; await route.abort('failed').catch(() => {}); }
  });
  await consent().check(); diskGuard(); await panel.getByRole('button', { name: 'Send answer', exact: true }).click();
  await until('Actual accepted response was deliberately dropped', () => acceptedReply);
  await panel.getByRole('alert').waitFor();
  const pendingBefore = (await recoveryRecords()).find(row => row.kind === 'pending'); assert.deepEqual(pendingBefore?.value, receipt.lostResponse.originalRequest);
  await inspectReadonlyRuntime(acceptedReply); assert.equal(providerCalls, 2); await noDefinitionChange(); await screenshot('05-accepted-response-lost-original-request-retained');
  const postsAtReload = receipt.analysisRequests.length, callsAtReload = providerCalls;
  const originalPath = projectRoute + '/assist/repository/requests/' + acceptedReply.idempotencyKey;
  const autoRead = page.waitForResponse(response => response.request().method() === 'GET' && new URL(response.url()).pathname === originalPath);
  await page.reload(); await openAnalysis(); const recoveredResponse = await autoRead; assert.equal(recoveredResponse.status(), 200); const recovered = await recoveredResponse.json();
  assert.equal(recovered.id, acceptedReply.id); assert.equal(recovered.ownRequest, true); assert.equal(recovered.requestedBy, ownerID); assert.equal(recovered.replyToAnalysisId, firstJob.id); assert.equal(recovered.userMessage, answer);
  const savedRequest = panel.getByRole('region', { name: 'Saved analysis request', exact: true }); await savedRequest.waitFor();
  const explicitRead = page.waitForResponse(response => response.request().method() === 'GET' && new URL(response.url()).pathname === originalPath);
  await savedRequest.getByRole('button', { name: 'Check original request', exact: true }).click(); const checkedResponse = await explicitRead; assert.equal(checkedResponse.status(), 200); const checked = await checkedResponse.json(); assert.equal(checked.id, acceptedReply.id); assert.equal(checked.ownRequest, true);
  assert.equal(receipt.analysisRequests.length, postsAtReload); assert.equal(providerCalls, callsAtReload); assert.equal(dropCount, 1);
  assert.equal(await panel.getByRole('textbox', { name: 'Your answer', exact: true }).inputValue(), answer);
  assert.deepEqual((await recoveryRecords()).find(row => row.kind === 'pending')?.value, receipt.lostResponse.originalRequest);
  await screenshot('06-original-job-recovered-without-resend');
  check('Lost actual 202 response recovers the original job by its retained key after reload and explicit status check, without a duplicate POST, container or provider call', { originalJobID: acceptedReply.id, originalKey: acceptedReply.idempotencyKey, analysisPOSTs: receipt.analysisRequests.length, providerCalls, duplicateInference: 0 });

  stage = 'review-proposal-and-apply';
  const finished = await complete(acceptedReply); assert.equal(finished.replyToAnalysisId, firstJob.id); assert.equal(finished.userMessage, answer); assert.equal(finished.replySourceChanged ?? false, false);
  assert.deepEqual(prompts[1].snapshot.followUp.result, firstJob.result); assert.equal(prompts[1].userMessage, answer); assert.equal(finished.result.proposal.purpose, purpose);
  await panel.getByRole('heading', { name: 'Proposed definition', exact: true }).waitFor(); await panel.getByText(purpose, { exact: true }).waitFor();
  await panel.getByText('The source has no sign-in or invitation checks; these are proposed future requirements, not implemented behavior.', { exact: true }).waitFor();
  assert.equal(await panel.getByRole('button', { name: 'Apply as definition draft', exact: true }).isEnabled(), false); await noDefinitionChange();
  await screenshot('07-review-source-grounded-proposal');
  check('Explicit follow-up binds server-owned prior question, prior result, human answer and the current exact source; the review separates observed behavior from proposed access controls', { previousAnalysisID: firstJob.id, followUpAnalysisID: finished.id, sourceDigest: finished.source.digest, unchangedSource: true, definitionStillUnchanged: true });
  await panel.getByRole('checkbox', { name: 'I reviewed the proposed definition and its partial source coverage.', exact: true }).check();
  const appliedResponse = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === `${projectRoute}/assist/${finished.id}/apply`);
  await panel.getByRole('button', { name: 'Apply as definition draft', exact: true }).click();
  const applied = await appliedResponse; assert.equal(applied.status(), 200); const saved = await applied.json();
  assert.deepEqual(applied.request().postDataJSON(), { expectedRevision: expectedDefinition.revision, expectedBriefRevision: expectedDefinition.briefRevision, acknowledgePartial: true });
  assert.equal(saved.status, 'draft'); assert.equal(saved.startingPoint, 'existing'); assert.equal(saved.purpose, purpose); assert.equal(saved.constraints, answer); assert(saved.revision > expectedDefinition.revision);
  assert.deepEqual(saved.requirements, finished.result.proposal.requirements);
  assert.deepEqual(await api('GET', projectRoute + '/specification'), saved);
  for (const [path, content] of Object.entries(expected.files)) assert.equal(readFileSync(join(project.path, path), 'utf8'), content, 'Analysis/apply must not modify implementation');
  assert.deepEqual(git(project.path, 'rev-list', '--reverse', 'HEAD').split('\n'), expected.history);
  const jobs = (await api('GET', projectRoute + '/assist')).jobs.filter(job => job.kind === 'repository'); assert.equal(jobs.length, 2); assert.deepEqual(jobs.map(job => job.id).sort(), [firstJob.id, finished.id].sort());
  assert.equal((await api('GET', `${projectRoute}/assist/${finished.id}`)).appliedRevision, saved.revision);
  assert.deepEqual((await api('GET', `${projectRoute}/assist/${firstJob.id}`)).result, firstJob.result);
  await page.getByRole('tab', { name: 'Specification', exact: true }).click(); await page.getByRole('textbox', { name: 'Boundaries and constraints', exact: true }).waitFor();
  assert.equal(await page.getByRole('textbox', { name: 'Boundaries and constraints', exact: true }).inputValue(), answer);
  await screenshot('08-applied-unapproved-definition-retains-human-answer');
  const projectMutations = receipt.browserMutations.filter(row => row.path.startsWith(projectRoute));
  assert.deepEqual(projectMutations.map(row => row.path), [projectRoute + '/assist/repository', projectRoute + '/assist/repository', `${projectRoute}/assist/${finished.id}/apply`]);
  assert.equal(receipt.importRequests.length, 1); assert.equal(starts().length, 1); assert.equal(providerCalls, 2); assert.equal(dropCount, 1);
  assert.deepEqual(receipt.browserErrors, []); assert.deepEqual(receipt.externalRequests, []);
  check('Explicit review applies once through source/definition CAS as an unapproved draft; original source, Git history, question and both job receipts remain intact', { previousRevision: expectedDefinition.revision, appliedRevision: saved.revision, status: saved.status, appliedRequirements: saved.requirements, providerJobs: jobs.length, implementationChanged: false });
  receipt.finalDefinition = saved; receipt.providerCalls = providerCalls; receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error.stack ?? error); receipt.failedStage = stage; console.error(receipt.error);
  if (page) {
    await page.screenshot({ path: join(evidence, 'failure.png'), fullPage: true }).catch(() => {});
    try { writeFileSync(join(evidence, 'failure-dom.txt'), scrub(await page.locator('body').innerText({ timeout: 3000 })), { mode: 0o600 }); } catch { /* Keep original failure. */ }
  }
} finally {
  releaseProvider?.();
  const cleanupFailure = error => { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); };
  let browserStopped = !browser, serviceStopped = !service, listenersStopped = false;
  try { await browser?.close(); browserStopped = true; } catch (error) { cleanupFailure(error); }
  try { serviceStopped = await stopService(); } catch (error) { cleanupFailure(error); }
  if (serviceStopped && project && ownerID) {
    try {
      const registryPath = join(fixture, 'state/control-plane/users', ownerID, 'runtime/state/sessions/sessions.json');
      if (existsSync(registryPath)) for (const session of JSON.parse(readFileSync(registryPath, 'utf8')).sessions ?? []) if (session.projectId === project.id && session.contextRef === 'luci:repository-analysis') { assert.match(session.id, /^s-[a-f0-9]{8}$/); sessions.add(session.id); }
      for (const id of sessions) for (const containerID of docker('ps', '-a', '--filter', 'label=luci.session-id=' + id, '--format', '{{.ID}}').split(/\s+/).filter(Boolean)) {
        const container = JSON.parse(docker('inspect', containerID))[0]; assert.equal(container.Config.Labels['luci.session-id'], id);
        assert(container.Mounts.some(mount => mount.Destination === '/luci-input' && mount.RW === false && realpathSync(mount.Source).startsWith(realpathSync(fixture) + sep)), 'Refusing to remove a foreign or unverified analysis container');
        docker('rm', '-f', containerID);
      }
      receipt.cleanup.push({ analysisSessionsChecked: sessions.size, ownedContainersRemoved: true });
    } catch (error) { cleanupFailure(error); }
  }
  try {
    for (const socket of [...remoteSockets, ...providerSockets]) socket.destroy();
    for (const listener of [remote, provider]) if (listener?.listening) await new Promise((done, reject) => listener.close(error => error ? reject(error) : done()));
    listenersStopped = true;
  } catch (error) { cleanupFailure(error); }
  writeFileSync(join(evidence, 'service.log'), scrub(serviceLog), { mode: 0o600 });
  receipt.providerCalls = providerCalls; receipt.cloneEvents = cloneEvents().map(({ cwd, ...row }) => ({ ...row, ...(cwd ? { privateStagingDirectory: cwd.startsWith(fixture + sep) } : {}) }));
  try { assert.equal(sha(readFileSync(binary)), binaryHash); assert.equal(sha(JSON.stringify(webInventory(web))), receipt.webInventorySHA256); receipt.packageUnchanged = true; } catch (error) { cleanupFailure(error); }
  receipt.passed &&= !receipt.cleanupErrors?.length && browserStopped && serviceStopped && listenersStopped;
  if (receipt.passed && !args.includes('--keep-fixture')) { try { rmSync(fixture, { recursive: true }); receipt.privateFixtureRemoved = true; } catch (error) { cleanupFailure(error); receipt.passed = false; } }
  else receipt.retainedPrivateFixture = fixture;
  receipt.finishedAt = new Date().toISOString(); writeFileSync(report, JSON.stringify(receipt, null, 2), { mode: 0o600, flag: 'wx' });
  console.log(JSON.stringify({ passed: receipt.passed, checks: receipt.checks.length, providerCalls, report, failedStage: receipt.failedStage, cleanupErrors: receipt.cleanupErrors }));
  if (!receipt.passed) process.exitCode = 1;
}

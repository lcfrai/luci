#!/usr/bin/env node
// A completed old catalogue read must not undo a newer explicit connection selection.
// Actual installed APIs and UI; delay original response delivery only. No model execution.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { createServer } from 'node:net';
import { createServer as createHTTP } from 'node:http';
import { existsSync, lstatSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, readlinkSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '../desktop/node_modules/playwright/index.mjs';

const args = process.argv.slice(2);
if (args.includes('--help')) {
  console.log('node scripts/connection-refresh-browser-uat.mjs --binary PATH --web PATH --chromium PATH --provenance PATH --report PATH\nInstalled Settings connection refresh ordering. Supported API setup creates one isolated owner and local settings; UI saves/selects a non-contacted compatible API fixture, then selects Gateway off while an earlier real catalogue response is held. No substituted responses, Docker or provider execution. Use a fresh report directory.');
  process.exit(0);
}
const names = ['--binary', '--web', '--chromium', '--provenance', '--report'], values = new Map();
for (let i = 0; i < args.length; i += 2) {
  assert(names.includes(args[i]) && !values.has(args[i]) && args[i + 1] && !args[i + 1].startsWith('--'), 'Invalid or duplicate argument ' + args[i]);
  values.set(args[i], args[i + 1]);
}
for (const name of names.filter(name => name !== '--chromium')) assert(values.has(name), 'Required: ' + name);
if (!values.has('--chromium')) values.set('--chromium', '/usr/bin/chromium');
assert.equal(process.platform, 'linux', 'Owned process cleanup requires Linux /proc');
const binary = realpathSync(resolve(values.get('--binary'))), web = realpathSync(resolve(values.get('--web'))), executable = realpathSync(resolve(values.get('--chromium')));
const report = resolve(values.get('--report')), evidence = dirname(report);
mkdirSync(dirname(evidence), { recursive: true, mode: 0o700 });
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Use a new evidence directory; never overwrite an earlier receipt');
else mkdirSync(evidence, { mode: 0o700 });
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const provenanceBytes = readFileSync(resolve(values.get('--provenance'))), provenance = JSON.parse(provenanceBytes);
const binarySHA256 = sha(readFileSync(binary)), webIndexSHA256 = sha(readFileSync(join(web, 'index.html')));
assert.equal(provenance.sourceDirty, false, 'Use a clean packaged candidate');
assert.match(provenance.sourceCommit, /^[a-f0-9]{40}$/);
assert.equal(provenance.binarySHA256, binarySHA256); assert.equal(provenance.webIndexSHA256, webIndexSHA256);
assert(provenance.archive && provenance.archiveSHA256, 'Provenance must identify the verified portable archive');
function inventory() {
  const found = {};
  function visit(directory, prefix = '') { for (const name of readdirSync(directory).sort()) {
    const file = join(directory, name), info = lstatSync(file);
    if (info.isDirectory()) visit(file, prefix + name + '/');
    else { assert(info.isFile() && !info.isSymbolicLink(), 'Installed web assets must be ordinary files'); found[prefix + name] = sha(readFileSync(file)); }
  } }
  visit(web); return found;
}
const webFiles = inventory();
const archive = JSON.parse(execFileSync('python3', ['-c', `import hashlib,io,json,pathlib,sys,tarfile
raw=pathlib.Path(sys.argv[1]).read_bytes()
if hashlib.sha256(raw).hexdigest()!=sys.argv[2]: raise ValueError("Archive hash mismatch")
out={"binary":None,"web":{}}
with tarfile.open(fileobj=io.BytesIO(raw),mode="r:gz") as source:
 for row in source:
  p=pathlib.PurePosixPath(row.name)
  if p.is_absolute() or ".." in p.parts: raise ValueError("Unsafe archive path")
  if not p.parts or not (p.parts[0]=="web" or str(p)=="luci") or row.isdir(): continue
  if not row.isfile(): raise ValueError("Linked or special package member")
  digest=hashlib.sha256()
  with source.extractfile(row) as stream:
   while block:=stream.read(1024*1024): digest.update(block)
  if str(p)=="luci":
   if out["binary"] is not None: raise ValueError("Duplicate binary")
   out["binary"]=digest.hexdigest()
  else:
   name=str(pathlib.PurePosixPath(*p.parts[1:]))
   if name in out["web"]: raise ValueError("Duplicate web asset")
   out["web"][name]=digest.hexdigest()
print(json.dumps(out))`, provenance.archive, provenance.archiveSHA256], { encoding: 'utf8', timeout: 60000, maxBuffer: 8 * 1024 * 1024, stdio: ['ignore', 'pipe', 'pipe'] }));
assert.equal(archive.binary, binarySHA256); assert.deepEqual(webFiles, archive.web, 'Installed assets differ from the verified archive');
writeFileSync(join(evidence, 'package-provenance.json'), provenanceBytes, { mode: 0o600, flag: 'wx' });
writeFileSync(join(evidence, 'driver.mjs'), readFileSync(fileURLToPath(import.meta.url)), { mode: 0o600, flag: 'wx' });
const fixture = mkdtempSync(join(tmpdir(), 'luci-connection-refresh-'));
for (const name of ['home', 'projects', 'state']) mkdirSync(join(fixture, name), { mode: 0o700 });
const secrets = new Set(), secret = () => { const value = randomBytes(32).toString('base64url'); secrets.add(value); return value; };
const password = secret(), lifecycle = secret(), fixtureKey = secret();
const scrub = error => { let text = String(error?.message ?? error); for (const value of secrets) text = text.replaceAll(value, '[redacted]'); return text; };
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit,
  package: { archiveSHA256: provenance.archiveSHA256, binarySHA256, webIndexSHA256, allWebFilesMatchArchive: true, webFileCount: Object.keys(webFiles).length },
  driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  scope: 'Actual installed Settings save/select/refresh/off actions and real catalogue response ordering. Compatible API and key are private configuration fixtures only; no provider contact or model quality claim.',
  setupScaffolding: ['Supported APIs bootstrap an isolated owner, configure a local workspace and save its Git identity. Browser sign-in and every tested connection action use rendered UI.'],
  limitations: ['No stale-error response is fabricated. Stale 403/503 behavior remains covered by source regressions.', 'No Docker, model request, cloud provisioning, real billing or session execution is exercised.'],
  runtimeBudgetMS: 180000,
  checks: [], rendering: [], delayedReads: [], browserMutations: [], browserErrors: [], blockedRequests: [], providerAttempts: [], cleanup: [],
};
const inherited = Object.fromEntries(Object.entries(process.env).filter(([name]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ)$/.test(name)));
let service, browser, browserClosing, page, base, token, stage = 'setup', spawnError, serviceLog = '', routeError, denyProxy, heldRead;
const running = new AbortController();
function closeBrowser() {
  if (!browser) return Promise.resolve();
  return browserClosing ??= browser.close();
}
function cancel(reason) {
  if (running.signal.aborted) return;
  receipt.interruption = reason;
  receipt.passed = false; process.exitCode = 1;
  running.abort(new Error(reason));
  heldRead?.markHeld(); heldRead?.release();
  // End pending Playwright actions so the main try reaches its one finally.
  // Process termination remains confined to the existing owned cleanup below.
  void closeBrowser().catch(() => {});
}
const onInterrupt = () => cancel('Runner received SIGINT');
const onTerminate = () => cancel('Runner received SIGTERM');
process.on('SIGINT', onInterrupt); process.on('SIGTERM', onTerminate);
const budget = setTimeout(() => cancel('Runner exceeded its 180-second runtime budget'), receipt.runtimeBudgetMS);
const check = (name, details = {}) => { receipt.checks.push({ name, ...details }); console.log('PASS ' + name); };
async function port() { const server = createServer(); await new Promise((done, reject) => server.once('error', reject).listen(0, '127.0.0.1', done)); const value = server.address().port; await new Promise(done => server.close(done)); return value; }
async function api(method, route, body, expected = 200, auth = token) {
  running.signal.throwIfAborted();
  const response = await fetch(base + route, { method, headers: { 'Content-Type': 'application/json', ...(auth ? { Authorization: 'Bearer ' + auth } : {}) }, body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.any([running.signal, AbortSignal.timeout(15000)]) });
  assert.equal(response.status, expected, method + ' ' + route); if (response.status === 204) return null;
  const value = await response.json(); if (value.token) secrets.add(value.token); return value;
}
function identity(pid) {
  try { const raw = readFileSync(`/proc/${pid}/stat`, 'utf8'), fields = raw.slice(raw.lastIndexOf(')') + 2).trim().split(/\s+/); return { pid: Number(pid), start: fields[19], group: Number(fields[2]), status: fields[0] }; }
  catch (error) { if (['ENOENT', 'ESRCH'].includes(error.code)) return null; throw error; }
}
const alive = row => { const current = identity(row.pid); return current && current.start === row.start && current.status !== 'Z'; };
function owned() {
  const found = [], state = join(fixture, 'state');
  for (const pid of readdirSync('/proc').filter(name => /^\d+$/.test(name))) {
    const row = identity(pid); if (!row || row.status === 'Z') continue;
    try {
      const vars = Object.fromEntries(readFileSync(`/proc/${pid}/environ`, 'utf8').split('\0').filter(Boolean).map(value => { const at = value.indexOf('='); return [value.slice(0, at), value.slice(at + 1)]; }));
      if (vars.LUCI_STATE_DIR !== state && !vars.LUCI_STATE_DIR?.startsWith(state + '/control-plane/')) continue;
      const executable = readlinkSync(`/proc/${pid}/exe`); if (identity(pid)?.start === row.start) found.push({ ...row, executable });
    } catch (error) { if (!['ENOENT', 'ESRCH', 'EACCES', 'EPERM'].includes(error.code)) throw error; }
  }
  return found;
}
async function stop() {
  const initial = owned();
  if (service?.pid && service.exitCode === null && service.signalCode === null) {
    assert(initial.some(row => row.pid === service.pid && row.executable === binary), 'Cannot prove fixture process ownership');
    try { const response = await fetch(base + '/v1/control-plane/shutdown', { method: 'POST', headers: { 'X-Luci-Lifecycle-Token': lifecycle }, signal: AbortSignal.timeout(5000) }); assert.equal(response.status, 200); }
    catch (error) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); if (initial.some(row => row.pid === service.pid && alive(row))) service.kill('SIGTERM'); }
  }
  const deadline = Date.now() + 20000;
  while (Date.now() < deadline && (owned().length || (service?.pid && service.exitCode === null && service.signalCode === null))) await delay(100);
  const remaining = [...initial.filter(alive), ...owned()];
  if (remaining.length) {
    receipt.cleanupErrors ??= []; receipt.cleanupErrors.push('Owned processes needed forced cleanup');
    for (const row of remaining) try { if (alive(row) && readlinkSync(`/proc/${row.pid}/exe`) === row.executable) process.kill(row.group === row.pid ? -row.pid : row.pid, 'SIGKILL'); } catch (error) { if (!['ENOENT', 'ESRCH'].includes(error.code)) throw error; }
    const forcedDeadline = Date.now() + 5000; while (Date.now() < forcedDeadline && owned().length) await delay(100);
  }
  const stopped = owned().length === 0; receipt.cleanup.push({ stopped, initialProcesses: initial.length, exitCode: service?.exitCode });
  assert(stopped, 'Owned fixture processes remain');
  if (service?.pid && service.exitCode !== 0) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push('Service did not exit normally'); }
}

async function start(environment) {
  running.signal.throwIfAborted();
  spawnError = undefined;
  service = spawn(binary, [], { cwd: fixture, env: environment, detached: true, stdio: ['ignore', 'pipe', 'pipe'] });
  service.once('error', error => { spawnError = error; });
  for (const stream of [service.stdout, service.stderr]) stream.on('data', bytes => { serviceLog = (serviceLog + scrub(bytes.toString())).slice(-100000); });
  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    running.signal.throwIfAborted();
    if (spawnError) throw spawnError;
    assert.equal(service.exitCode, null, 'Owned service exited before readiness');
    try { const status = await api('GET', '/v1/control-plane/status', undefined, 200, ''); receipt.version = status.version; return; } catch { running.signal.throwIfAborted(); await delay(100, undefined, { signal: running.signal }); }
  }
  assert.fail('Owned service did not become ready');
}
async function screenshot(name) {
  // Successful outcome captures are unmasked after confirming private inputs are empty.
  // Failure captures protect any unsent password or fixture key.
  await page.screenshot({ path: join(evidence, name + '.png'), fullPage: true, ...(name === 'failure' ? { mask: [page.locator('input[type=password]')] } : {}) });
  writeFileSync(join(evidence, name + '-dom.txt'), scrub(await page.locator('body').innerText()), { mode: 0o600 });
}
async function action(path, activate, expected = 200) {
  const [response] = await Promise.all([
    page.waitForResponse(response => new URL(response.url()).pathname === path && response.request().method() === 'POST'),
    activate(),
  ]);
  assert.equal(response.status(), expected, 'Rendered action ' + path); await response.finished();
  const value = await response.json(); if (value.token) secrets.add(value.token); return value;
}
const endpoint = '/v1/setup/agent-connections';
const connectionName = 'Uncontacted refresh fixture';
async function painted() { await page.evaluate(() => new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done)))); }
function holdNextCatalogue() {
  assert(!heldRead, 'Only one explicit delayed response at a time');
  let markHeld, release, markDone;
  const row = { method: 'GET', path: endpoint, deadlineExceeded: false };
  const gate = { row, claimed: false, request: null,
    held: new Promise(done => { markHeld = done; }), released: new Promise(done => { release = done; }), done: new Promise(done => { markDone = done; }),
  };
  const timer = setTimeout(() => { row.deadlineExceeded = true; markHeld(); release(); }, 10000);
  Object.assign(gate, { markHeld, markDone, release() { clearTimeout(timer); release(); } });
  receipt.delayedReads.push(row); heldRead = gate; return gate;
}
async function observeOutcome(connection, name) {
  const active = connection.getByRole('combobox', { name: 'Active connection', exact: true });
  await active.scrollIntoViewIfNeeded(); await painted();
  assert.equal(await connection.getByLabel('Connection API key', { exact: true }).inputValue(), '');
  await screenshot(name);
  receipt.rendering.push({ screenshot: name + '.png', viewport: page.viewportSize(), active: await active.boundingBox(), region: await connection.boundingBox(), accountExplanation: await connection.getByText('New sessions use the direct accounts configured below. Existing sessions keep the connection they started with.', { exact: true }).boundingBox() });
  writeFileSync(join(evidence, 'progress.json'), JSON.stringify(receipt, null, 2), { mode: 0o600 });
  assert.equal(await active.inputValue(), '');
  assert.equal(await active.locator('option:checked').textContent(), 'Gateway off · own agent settings');
  assert.equal(await connection.getByRole('switch', { name: 'Use gateway', exact: true }).isChecked(), false);
  assert(await connection.getByText('New sessions use the direct accounts configured below. Existing sessions keep the connection they started with.', { exact: true }).isVisible());
  assert.equal(await connection.getByText('New sessions use ' + connectionName + '.', { exact: false }).count(), 0);
  assert(await connection.getByText('Gateway is off. New sessions use your own agent settings.', { exact: true }).isVisible());
  assert.equal(await connection.getByRole('alert').count(), 0);
}
try {
  const controlPort = await port(), preview = await port(); base = `http://127.0.0.1:${controlPort}`;
  denyProxy = createHTTP((request, response) => { receipt.providerAttempts.push({ method: request.method, destination: request.url }); response.writeHead(502); response.end('Fixture forbids provider traffic'); });
  denyProxy.on('connect', (request, socket) => { receipt.providerAttempts.push({ method: 'CONNECT', destination: request.url }); socket.end('HTTP/1.1 502 External traffic forbidden\r\nConnection: close\r\n\r\n'); });
  await new Promise(done => denyProxy.listen(0, '127.0.0.1', done));
  const outbound = `http://127.0.0.1:${denyProxy.address().port}`;
  const environment = { ...inherited, HOME: join(fixture, 'home'), USERPROFILE: join(fixture, 'home'), LUCI_ADDR: `127.0.0.1:${controlPort}`, LUCI_BROWSER_PREVIEW_ADDR: `127.0.0.1:${preview}`, LUCI_BROWSER_PREVIEW_PORT: String(preview), LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'), LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: join(dirname(binary), 'runtime'), LUCI_DOCKER_HOST: 'unix://' + join(fixture, 'absent-docker.sock'), DOCKER_HOST: 'unix://' + join(fixture, 'absent-docker.sock'), LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_BACKEND: 'docker', GIT_CONFIG_GLOBAL: join(fixture, 'home/.gitconfig'), GIT_CONFIG_NOSYSTEM: '1', AWS_EC2_METADATA_DISABLED: 'true', HTTP_PROXY: outbound, HTTPS_PROXY: outbound, NO_PROXY: '127.0.0.1,localhost' };
  await start(environment);
  const bootstrap = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(); secrets.add(bootstrap);
  const owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'refresh-owner', displayName: 'Connection refresh owner', password }, 201, ''); token = owner.token;
  const settings = await api('GET', '/v1/setup/status');
  await api('POST', '/v1/setup/config', { ...settings.config, projectsDir: join(fixture, 'projects'), mode: 'local', cloudProvider: '', region: '', domain: '', monthlyBudgetUsd: 30 });
  await api('POST', '/v1/setup/git/identity', { name: 'Connection refresh fixture', email: 'connection-refresh@example.invalid' });
  assert.deepEqual((await api('GET', '/v1/sessions')).sessions, []);
  const initial = await api('GET', endpoint); assert.equal(initial.activeConnectionId, ''); assert.deepEqual(initial.connections, []);

  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited, args: ['--no-proxy-server'] });
  running.signal.throwIfAborted();
  receipt.browserVersion = browser.version();
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, serviceWorkers: 'block' });
  await context.route('**/*', async route => {
    let original, gate;
    try {
      const request = route.request(), url = new URL(request.url());
      if (url.origin !== base) { receipt.blockedRequests.push({ method: request.method(), origin: url.origin }); return await route.abort('blockedbyclient'); }
      if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method())) {
        const permitted = request.method() === 'POST' && ['/v1/auth/login', endpoint].includes(url.pathname);
        receipt.browserMutations.push({ method: request.method(), path: url.pathname, permitted });
        if (!permitted) { routeError ??= new Error('Unexpected browser mutation ' + url.pathname); return await route.abort('blockedbyclient'); }
      }
      gate = heldRead;
      if (gate && !gate.claimed && request.method() === 'GET' && url.pathname === endpoint) {
        gate.claimed = true; gate.request = request;
        original = await route.fetch({ timeout: 15000 }); const bytes = await original.body(); assert.equal(original.status(), 200);
        const catalogue = JSON.parse(bytes); assert.equal(catalogue.activeConnectionId, receipt.connectionID);
        Object.assign(gate.row, { status: original.status(), bodySHA256: sha(bytes), revision: catalogue.revision, activeConnectionId: catalogue.activeConnectionId, heldAt: new Date().toISOString() });
        gate.markHeld(); await gate.released;
        await route.fulfill({ response: original, body: bytes });
        Object.assign(gate.row, { unchangedOriginalReleased: true, releasedAt: new Date().toISOString() });
      } else { gate = undefined; await route.continue(); }
    } catch (error) { routeError ??= error; gate?.markHeld(); gate?.release(); await route.abort('failed').catch(() => {}); }
    finally { if (gate) { await original?.dispose().catch(() => {}); gate.markDone(); if (heldRead === gate) heldRead = undefined; } }
  });
  page = await context.newPage(); page.setDefaultTimeout(20000); page.on('pageerror', error => receipt.browserErrors.push(scrub(error)));
  stage = 'ui-login-and-settings'; await page.goto(base);
  await page.getByLabel('Username', { exact: true }).fill('refresh-owner'); await page.getByLabel('Password', { exact: true }).fill(password);
  await action('/v1/auth/login', () => page.getByRole('button', { name: 'Sign in', exact: true }).click());
  const enter = page.getByRole('button', { name: 'Continue to projects', exact: true }), settingsButton = page.getByRole('button', { name: 'Settings', exact: true });
  const deadline = Date.now() + 20000;
  while (!(await settingsButton.isVisible()) && !(await enter.isVisible()) && Date.now() < deadline) { running.signal.throwIfAborted(); await delay(50, undefined, { signal: running.signal }); }
  if (await enter.isVisible()) await enter.click();
  await settingsButton.click();
  await page.getByRole('navigation', { name: 'Settings sections', exact: true }).getByRole('button', { name: 'Agent connection', exact: true }).click();
  const connection = page.getByRole('region', { name: 'Saved agent connections', exact: true });
  await connection.getByRole('combobox', { name: 'Active connection', exact: true }).waitFor();
  stage = 'ui-save-and-select'; await connection.getByText('Add or edit a connection', { exact: true }).click();
  await connection.getByLabel('Connection name', { exact: true }).fill(connectionName);
  await connection.getByRole('combobox', { name: 'Connection type', exact: true }).selectOption('compatible-api');
  await connection.getByLabel('Codex · Responses base URL', { exact: true }).fill(outbound + '/v1');
  await connection.getByLabel('Codex model', { exact: true }).fill('uncontacted-fixture');
  await connection.getByLabel('Connection API key', { exact: true }).fill(fixtureKey);
  const saved = await action(endpoint, () => connection.getByRole('button', { name: 'Save connection', exact: true }).click());
  assert.equal(saved.connections.length, 1); const entry = saved.connections[0];
  assert.equal(entry.name, connectionName); assert.equal(entry.kind, 'compatible-api'); assert.equal(entry.configured, true); receipt.connectionID = entry.id;
  const active = connection.getByRole('combobox', { name: 'Active connection', exact: true });
  const [selectedResponse, selected] = await Promise.all([
    page.waitForResponse(async response => response.request().method() === 'GET' && new URL(response.url()).pathname === endpoint && response.status() === 200 && (await response.json()).activeConnectionId === entry.id),
    action(endpoint, () => active.selectOption(entry.id)),
  ]);
  assert.equal(selected.activeConnectionId, entry.id); await selectedResponse.finished();
  assert.equal((await selectedResponse.json()).revision, selected.revision);
  await connection.getByText('New sessions use ' + connectionName + '. Existing sessions keep the connection they started with.', { exact: true }).waitFor();
  assert.equal(await connection.getByLabel('Connection API key', { exact: true }).inputValue(), '');
  await connection.getByText('Add or edit a connection', { exact: true }).click();
  check('Actual Settings UI saves and selects one compatible API fixture without contacting it', { connectionID: entry.id, revision: selected.revision });

  stage = 'delayed-old-read-new-explicit-off';
  // Drain the mutation-triggered GET before admitting the explicit held refresh.
  // Only these real requests are observed; no hook state or response is substituted.
  // selectedResponse above witnessed the actual selection-triggered catalogue response.
  await painted();
  const gate = holdNextCatalogue();
  await connection.getByRole('button', { name: 'Refresh saved connections', exact: true }).click();
  await gate.held; running.signal.throwIfAborted(); assert.equal(routeError, undefined); assert(gate.claimed && !gate.row.deadlineExceeded);
  const [freshResponse, off] = await Promise.all([
    page.waitForResponse(response => response.request() !== gate.request && response.request().method() === 'GET' && new URL(response.url()).pathname === endpoint),
    action(endpoint, () => active.selectOption('')),
  ]);
  assert.equal(off.activeConnectionId, ''); assert(off.revision > gate.row.revision);
  assert.equal(freshResponse.status(), 200); await freshResponse.finished();
  const fresh = await freshResponse.json(); assert.equal(fresh.activeConnectionId, ''); assert.equal(fresh.revision, off.revision);
  receipt.newestRead = { status: freshResponse.status(), bodySHA256: sha(await freshResponse.body()), revision: fresh.revision, activeConnectionId: fresh.activeConnectionId };
  await connection.getByText('Gateway is off. New sessions use your own agent settings.', { exact: true }).waitFor(); await painted();
  await observeOutcome(connection, '01-off-before-old-read');
  const [released] = await Promise.all([
    page.waitForResponse(response => response.request() === gate.request),
    (async () => { gate.release(); await gate.done; })(),
  ]);
  await released.finished(); await painted();
  assert.equal(routeError, undefined); assert(!gate.row.deadlineExceeded); assert.equal(sha(await released.body()), gate.row.bodySHA256);
  await observeOutcome(connection, '02-off-after-old-read-1440');
  check('Old successful catalogue cannot restore the earlier active billing connection after explicit Gateway off', { oldRevision: gate.row.revision, currentRevision: off.revision, oldBodyReleasedUnchanged: true });

  stage = 'phone-outcome'; await page.setViewportSize({ width: 390, height: 844 }); await observeOutcome(connection, '03-off-after-old-read-390');
  const authoritative = await api('GET', endpoint); assert.equal(authoritative.activeConnectionId, ''); assert.equal(authoritative.revision, off.revision); assert.deepEqual(authoritative.connections, off.connections);
  check('Desktop and phone keep Gateway off and truthful own-account copy after unchanged delayed response delivery');
  receipt.sessions = (await api('GET', '/v1/sessions')).sessions; assert.deepEqual(receipt.sessions, []);
  assert.deepEqual(receipt.providerAttempts, []); assert.deepEqual(receipt.blockedRequests, []); assert.deepEqual(receipt.browserErrors, []); assert.equal(routeError, undefined);
  assert.equal(receipt.browserMutations.filter(row => row.path === endpoint).length, 3);
  assert.equal(receipt.browserMutations.filter(row => row.path === '/v1/auth/login').length, 1);
  assert(!existsSync(join(fixture, 'absent-docker.sock'))); assert.equal(sha(readFileSync(binary)), binarySHA256); assert.deepEqual(inventory(), webFiles);
  check('No sessions, provider attempts, extra connection mutations or changed installed assets'); running.signal.throwIfAborted(); receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error); receipt.failedStage = stage; if (routeError) receipt.fixtureError = scrub(routeError);
  if (page) await screenshot('failure').catch(() => {}); process.exitCode = 1;
} finally {
  clearTimeout(budget);
  const pending = heldRead; pending?.release();
  await closeBrowser().catch(error => { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); });
  if (pending?.claimed) {
    let timer;
    try { await Promise.race([pending.done, new Promise((_resolve, reject) => { timer = setTimeout(() => reject(new Error('Held read did not finish during cleanup')), 16000); })]); }
    catch (error) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); }
    finally { clearTimeout(timer); }
  }
  let cleaned = false; try { await stop(); cleaned = true; } catch (error) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); }
  if (denyProxy) { denyProxy.closeAllConnections(); await new Promise(done => denyProxy.close(done)); }
  if (cleaned) rmSync(fixture, { recursive: true, force: true }); else receipt.retainedFixture = fixture;
  if (receipt.cleanupErrors?.length) { receipt.passed = false; process.exitCode = 1; }
  receipt.finishedAt = new Date().toISOString(); writeFileSync(join(evidence, 'service.log'), scrub(serviceLog), { mode: 0o600 });
  writeFileSync(report, JSON.stringify(receipt, null, 2), { mode: 0o600 });
  console.log(JSON.stringify({ passed: receipt.passed, checks: receipt.checks.length, report, failedStage: receipt.failedStage }));
  process.off('SIGINT', onInterrupt); process.off('SIGTERM', onTerminate);
}

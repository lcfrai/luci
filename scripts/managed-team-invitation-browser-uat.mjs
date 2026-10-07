#!/usr/bin/env node
// Existing authenticated accounts join an exactly bound managed team via UI.
// Actual local gateway responses, no response fixtures, Docker or model execution.
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
  console.log('node scripts/managed-team-invitation-browser-uat.mjs --binary PATH --web PATH --chromium PATH --provenance PATH --report PATH\nExisting account accepts a real managed-team invitation through installed UI. API setup creates isolated accounts, one team, invitations and a private project (including native Git initialization); grant/revoke and restart use actual server APIs. No substituted responses, provider credentials, Docker, agent or check execution. Use a fresh report directory. Runtime budget: 180 seconds plus bounded owned cleanup; SIGINT/SIGTERM use the same cleanup.');
  process.exit(0);
}
const names = ['--binary', '--web', '--chromium', '--provenance', '--report'], values = new Map();
for (let i = 0; i < args.length; i += 2) {
  assert(names.includes(args[i]) && !values.has(args[i]) && args[i + 1] && !args[i + 1].startsWith('--'), 'Invalid or duplicate argument ' + args[i]);
  values.set(args[i], args[i + 1]);
}
for (const name of names) assert(values.has(name), 'Required: ' + name);
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
const fixture = mkdtempSync(join(tmpdir(), 'luci-managed-invitation-'));
for (const name of ['home', 'projects', 'state']) mkdirSync(join(fixture, name), { mode: 0o700 });
const secrets = new Set(), secret = () => { const value = randomBytes(32).toString('base64url'); secrets.add(value); return value; };
const password = secret(), lifecycle = secret(), scrub = error => { let text = String(error?.message ?? error); for (const value of secrets) text = text.replaceAll(value, '[redacted]'); return text; };
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit,
  package: { archiveSHA256: provenance.archiveSHA256, binarySHA256, webIndexSHA256, allWebFilesMatchArchive: true, webFileCount: Object.keys(webFiles).length },
  driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  scope: 'Actual installed managed-team existing-account invitation, sign-out, explicit project access, durable membership and later revocation.',
  setupScaffolding: ['Supported APIs create isolated owner and existing member, one team, a private project and two invitation codes; one is revoked.', 'Service restarts with the exact immutable managed-team ID; subsequent project sharing and membership removal use supported owner APIs.'],
  limitations: ['Local loopback acceptance; no public-domain/TLS commissioning claim.', 'This proves invitation entry and authorization continuity, not paid provisioning, billing, model quality or deployed application behavior.'],
  checks: [], browserMutations: [], browserErrors: [], blockedRequests: [], providerAttempts: [], cleanup: [],
  dockerAllocations: 0, agentOrCheckExecutions: 0, runtimeBudgetMS: 180000,
};
const inherited = Object.fromEntries(Object.entries(process.env).filter(([name]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ)$/.test(name)));
let service, browser, browserClosing, page, base, token, team, stage = 'setup', spawnError, serviceLog = '', routeError, denyProxy;
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
  await page.screenshot({ path: join(evidence, name + '.png'), fullPage: true });
  writeFileSync(join(evidence, name + '-dom.txt'), scrub(await page.locator('body').innerText()), { mode: 0o600 });
}
async function action(path, button, expected = 200) {
  const [response] = await Promise.all([
    page.waitForResponse(response => new URL(response.url()).pathname === path && response.request().method() === 'POST'),
    button.click(),
  ]);
  assert.equal(response.status(), expected, 'Rendered action ' + path);
  const value = await response.json(); if (value.token) secrets.add(value.token); return value;
}
async function signIn() {
  await page.getByLabel('Username', { exact: true }).fill('existing-collaborator');
  await page.getByLabel('Password', { exact: true }).fill(password);
  await action('/v1/auth/login', page.getByRole('button', { name: 'Sign in', exact: true }));
  await page.getByRole('heading', { name: 'Workspace unavailable', exact: true }).waitFor();
  await page.getByLabel('Team invitation code', { exact: true }).waitFor();
}
async function assertEntry() {
  assert.equal(await page.getByRole('button', { name: 'Team access', exact: true }).count(), 0);
  assert.equal(await page.getByRole('combobox', { name: 'Workspace', exact: true }).count(), 0);
  assert.equal(await page.getByRole('treeitem', { name: /invitation-private-project/ }).count(), 0);
  assert(await page.getByRole('button', { name: 'Sign out to use another account', exact: true }).isVisible());
}
try {
  const controlPort = await port(), preview = await port(); base = `http://127.0.0.1:${controlPort}`;
  denyProxy = createHTTP((request, response) => { receipt.providerAttempts.push({ method: request.method, destination: request.url }); response.writeHead(502); response.end('Fixture forbids external traffic'); });
  denyProxy.on('connect', (request, socket) => { receipt.providerAttempts.push({ method: 'CONNECT', destination: request.url }); socket.end('HTTP/1.1 502 External traffic forbidden\r\nConnection: close\r\n\r\n'); });
  await new Promise(done => denyProxy.listen(0, '127.0.0.1', done));
  const outbound = `http://127.0.0.1:${denyProxy.address().port}`;
  const environment = { ...inherited, HOME: join(fixture, 'home'), USERPROFILE: join(fixture, 'home'), LUCI_ADDR: `127.0.0.1:${controlPort}`, LUCI_BROWSER_PREVIEW_ADDR: `127.0.0.1:${preview}`, LUCI_BROWSER_PREVIEW_PORT: String(preview), LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'), LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: join(dirname(binary), 'runtime'), LUCI_DOCKER_HOST: 'unix://' + join(fixture, 'absent-docker.sock'), DOCKER_HOST: 'unix://' + join(fixture, 'absent-docker.sock'), LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_BACKEND: 'docker', GIT_CONFIG_GLOBAL: join(fixture, 'home/.gitconfig'), GIT_CONFIG_NOSYSTEM: '1', AWS_EC2_METADATA_DISABLED: 'true', HTTP_PROXY: outbound, HTTPS_PROXY: outbound, NO_PROXY: '127.0.0.1,localhost' };
  await start(environment);
  const bootstrap = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(); secrets.add(bootstrap);
  const owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'invitation-owner', displayName: 'Invitation owner', password }, 201, ''); token = owner.token;
  await api('POST', '/v1/admin/users', { username: 'existing-collaborator', displayName: 'Existing collaborator', password }, 201);
  const existing = await api('POST', '/v1/auth/login', { username: 'existing-collaborator', password }, 200, '');
  team = (await api('POST', '/v1/teams', { name: 'Managed invitation team' }, 201)).team;
  const project = await api('POST', `/v1/teams/${team.id}/workspace/v1/product/projects`, { name: 'invitation-private-project' }, 201);
  team = (await api('GET', `/v1/teams/${team.id}`)).team;
  const grants = { execute: false, secrets: false, deploy: false, admin: false };
  const valid = await api('POST', `/v1/teams/${team.id}/invitations`, { expectedRevision: team.revision, role: 'editor', grants }, 201); secrets.add(valid.inviteCode); team = valid.team;
  const revoked = await api('POST', `/v1/teams/${team.id}/invitations`, { expectedRevision: team.revision, role: 'editor', grants }, 201); secrets.add(revoked.inviteCode); team = revoked.team;
  team = (await api('DELETE', `/v1/teams/${team.id}/invitations/${revoked.invitation.id}`, { expectedRevision: team.revision })).team;
  await stop(); assert.equal(owned().length, 0);
  const managed = { ...environment, LUCI_MANAGED_TEAM_ID: team.id }; await start(managed);
  const before = await api('GET', '/v1/teams', undefined, 200, existing.token);
  assert.equal(before.managedTeamId, team.id); assert.deepEqual(before.teams, []);
  const workspace = `/v1/teams/${team.id}/workspace`;
  await api('GET', workspace + '/v1/projects', undefined, 403, existing.token);

  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited, args: ['--no-proxy-server'] });
  running.signal.throwIfAborted();
  receipt.browserVersion = browser.version();
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, serviceWorkers: 'block' });
  await context.route('**/*', async route => {
    try {
      const request = route.request(), url = new URL(request.url());
      if (url.origin !== base) { receipt.blockedRequests.push({ method: request.method(), origin: url.origin }); return await route.abort('blockedbyclient'); }
      if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method())) {
        const permitted = request.method() === 'POST' && ['/v1/auth/login', '/v1/auth/logout', '/v1/teams/invitations/accept'].includes(url.pathname);
        receipt.browserMutations.push({ method: request.method(), path: url.pathname, permitted });
        if (!permitted) { routeError ??= new Error('Unexpected browser mutation ' + url.pathname); return await route.abort('blockedbyclient'); }
      }
      await route.continue();
    } catch (error) { routeError ??= error; await route.abort('failed').catch(() => {}); }
  });
  page = await context.newPage(); page.setDefaultTimeout(20000); page.on('pageerror', error => receipt.browserErrors.push(scrub(error)));
  stage = 'existing-account-blocked-entry';
  await page.goto(base); await signIn(); await assertEntry();
  await page.getByLabel('Team invitation code', { exact: true }).fill(valid.inviteCode);
  await screenshot('01-existing-account-private-entry');
  await action('/v1/auth/logout', page.getByRole('button', { name: 'Sign out to use another account', exact: true }));
  await signIn(); await assertEntry();
  assert.equal(await page.getByLabel('Team invitation code', { exact: true }).inputValue(), '');
  assert.equal(receipt.browserMutations.filter(row => row.path.endsWith('/invitations/accept')).length, 0);
  stage = 'phone-entry-readability';
  await page.setViewportSize({ width: 390, height: 844 });
  await page.evaluate(() => new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done))));
  await screenshot('01-phone-managed-entry-before-geometry');
  const entryControls = [
    ['invitation', page.getByLabel('Team invitation code', { exact: true })],
    ['join', page.getByRole('button', { name: 'Join team', exact: true })],
    ['signOut', page.getByRole('button', { name: 'Sign out to use another account', exact: true })],
  ];
  const phone = { viewport: await page.evaluate(() => ({ width: innerWidth, height: innerHeight, scrollWidth: document.documentElement.scrollWidth })), controls: [] };
  for (const [name, control] of entryControls) {
    await control.scrollIntoViewIfNeeded();
    phone.controls.push({ name, visible: await control.isVisible(), ...await control.evaluate(element => {
      const box = element.getBoundingClientRect(), hit = document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2);
      return { x: box.x, y: box.y, width: box.width, height: box.height, disabled: element.disabled, unobscured: hit === element || element.contains(hit) };
    }) });
  }
  writeFileSync(join(evidence, 'phone-entry-geometry.json'), JSON.stringify(phone, null, 2), { mode: 0o600 });
  assert(phone.viewport.scrollWidth <= phone.viewport.width, 'Managed entry overflows the phone horizontally');
  for (const control of phone.controls) {
    assert(control.visible && control.width > 0 && control.height >= 32, control.name + ' is not visibly usable');
    assert(control.x >= 0 && control.x + control.width <= 390 && control.y >= 0 && control.y + control.height <= 844, control.name + ' cannot be scrolled into the phone viewport');
    if (!control.disabled) assert(control.unobscured, control.name + ' is obstructed');
  }
  await page.getByLabel('Team invitation code', { exact: true }).focus();
  assert(await page.getByLabel('Team invitation code', { exact: true }).evaluate(element => element === document.activeElement));
  await page.keyboard.press('Tab');
  assert(await page.getByRole('button', { name: 'Sign out to use another account', exact: true }).evaluate(element => element === document.activeElement), 'Keyboard cannot reach sign-out after the empty invitation field');
  await screenshot('01-phone-managed-entry-keyboard-focus');
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.evaluate(() => new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done))));
  check('Existing nonmember sees invitation and sign-out recovery with no accessible private project UI; sign-out discards unsent code without consuming it');
  check('At 390×844 the invitation, disabled empty-code Join action and sign-out are visible without horizontal overflow; native keyboard focus reaches sign-out');

  stage = 'revoked-invitation';
  await page.getByLabel('Team invitation code', { exact: true }).fill(revoked.inviteCode);
  const denied = await action('/v1/teams/invitations/accept', page.getByRole('button', { name: 'Join team', exact: true }), 403);
  await page.getByRole('alert').filter({ hasText: denied.message }).waitFor(); await assertEntry();
  assert.equal(await page.getByLabel('Team invitation code', { exact: true }).inputValue(), revoked.inviteCode);
  assert.deepEqual((await api('GET', '/v1/teams', undefined, 200, existing.token)).teams, []);
  await screenshot('02-revoked-code-remains-recoverable');
  check('Actual backend refusal of revoked invitation remains blocked and retains the code for correction');

  stage = 'accept-exact-managed-team';
  await page.getByLabel('Team invitation code', { exact: true }).fill(valid.inviteCode);
  const accepted = await action('/v1/teams/invitations/accept', page.getByRole('button', { name: 'Join team', exact: true }));
  assert.equal(accepted.team.id, team.id); assert.equal(accepted.membership.userId, existing.user.id);
  assert.equal(accepted.membership.role, 'editor'); assert.deepEqual(accepted.membership.grants, grants);
  await page.getByRole('button', { name: 'Team access', exact: true }).waitFor();
  assert.equal(await page.getByLabel('Team invitation code', { exact: true }).count(), 0);
  assert.equal(await page.getByRole('treeitem', { name: /invitation-private-project/ }).count(), 0);
  const current = await api('GET', '/v1/teams', undefined, 200, existing.token);
  assert.equal(current.managedTeamId, team.id); assert.equal(current.teams.length, 1); assert.equal(current.teams[0].team.id, team.id);
  assert.equal(await page.evaluate(code => JSON.stringify(localStorage).includes(code) || JSON.stringify(sessionStorage).includes(code), valid.inviteCode), false);
  await api('POST', '/v1/teams/invitations/accept', { inviteCode: valid.inviteCode }, 403, existing.token);
  await api('GET', '/v1/admin/users', undefined, 403, existing.token);
  await screenshot('03-exact-managed-team-joined');
  check('Same existing account joins only the bound team with exact editor grants; current membership unlocks workspace, not implicit private project access or administration');

  stage = 'explicit-project-grant-and-restart';
  const detail = await api('GET', `/v1/teams/${team.id}`);
  await api('PUT', `/v1/teams/${team.id}/projects/${project.id}/members/${existing.user.id}`, { expectedRevision: detail.team.revision, role: 'editor', grants });
  await page.reload(); await page.getByRole('treeitem', { name: /invitation-private-project$/ }).waitFor();
  await stop(); await start(managed); await page.reload();
  await page.getByRole('treeitem', { name: /invitation-private-project$/ }).waitFor();
  const persisted = await api('GET', `/v1/teams/${team.id}`, undefined, 200, existing.token);
  assert.equal(persisted.membership.userId, existing.user.id); assert.equal(persisted.membership.role, 'editor'); assert.deepEqual(persisted.membership.grants, grants);
  await screenshot('04-membership-and-explicit-project-survive-restart');
  check('Separate owner project grant makes its project visible and actual gateway/worker restart preserves the existing account membership');

  stage = 'confirmed-removal';
  const latest = await api('GET', `/v1/teams/${team.id}`);
  await api('DELETE', `/v1/teams/${team.id}/members/${existing.user.id}`, { expectedRevision: latest.team.revision });
  await page.reload(); await page.getByLabel('Team invitation code', { exact: true }).waitFor(); await assertEntry();
  await api('GET', workspace + '/v1/projects', undefined, 403, existing.token);
  await screenshot('05-revocation-restores-blocked-entry');
  check('Authoritative membership removal restores blocked entry and denies private projects without switching to personal authority');

  assert.equal(routeError, undefined); assert.deepEqual(receipt.browserErrors, []); assert.deepEqual(receipt.blockedRequests, []); assert.deepEqual(receipt.providerAttempts, []);
  assert.equal(receipt.browserMutations.filter(row => row.path.endsWith('/invitations/accept')).length, 2);
  assert.equal(receipt.browserMutations.filter(row => row.path === '/v1/auth/login').length, 2);
  assert.equal(receipt.browserMutations.filter(row => row.path === '/v1/auth/logout').length, 1);
  assert.deepEqual((await api('GET', workspace + '/v1/sessions')).sessions, []);
  assert.equal(sha(readFileSync(binary)), binarySHA256); assert.deepEqual(inventory(), webFiles); running.signal.throwIfAborted(); receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error); receipt.failedStage = stage; if (routeError) receipt.fixtureError = scrub(routeError);
  if (page) await screenshot('failure').catch(() => {});
  process.exitCode = 1;
} finally {
  clearTimeout(budget);
  await closeBrowser().catch(error => { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); });
  let cleaned = false; try { await stop(); cleaned = true; } catch (error) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); }
  if (denyProxy) { denyProxy.closeAllConnections(); await new Promise(done => denyProxy.close(done)); }
  if (cleaned) rmSync(fixture, { recursive: true, force: true }); else receipt.retainedFixture = fixture;
  if (receipt.cleanupErrors?.length) { receipt.passed = false; process.exitCode = 1; }
  receipt.finishedAt = new Date().toISOString();
  writeFileSync(join(evidence, 'service.log'), scrub(serviceLog), { mode: 0o600 });
  writeFileSync(report, JSON.stringify(receipt, null, 2), { mode: 0o600 });
  console.log(JSON.stringify({ passed: receipt.passed, checks: receipt.checks.length, report, failedStage: receipt.failedStage }));
  process.off('SIGINT', onInterrupt); process.off('SIGTERM', onTerminate);
}

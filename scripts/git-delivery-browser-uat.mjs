#!/usr/bin/env node
// Installed Git-delivery grants and unsent settings with real local Git operations.
// Responses may be delayed unchanged or transport-aborted; no fabricated payloads.
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
  console.log('node scripts/git-delivery-browser-uat.mjs --binary PATH --web PATH --chromium PATH --provenance PATH --report PATH\nInstalled Git-delivery permissions and in-memory draft continuity with actual accounts, team/project grants and local Git commits/origin attachment. Holds genuine GET bytes unchanged; labels one aborted response-delivery fault. Tests a denied GitHub check request; no authorized GitHub operation, external repository access, provider credentials, Docker, agent or check execution. Use a fresh report directory. Runtime budget: 180 seconds plus bounded owned cleanup; SIGINT/SIGTERM use the same cleanup.');
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
const fixture = mkdtempSync(join(tmpdir(), 'luci-git-delivery-'));
for (const name of ['home', 'projects', 'state']) mkdirSync(join(fixture, name), { mode: 0o700 });
const secrets = new Set(), secret = () => { const value = randomBytes(32).toString('base64url'); secrets.add(value); return value; };
const password = secret(), lifecycle = secret(), scrub = error => { let text = String(error?.message ?? error); for (const value of secrets) text = text.replaceAll(value, '[redacted]'); return text; };
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit,
  package: { archiveSHA256: provenance.archiveSHA256, binarySHA256, webIndexSHA256, allWebFilesMatchArchive: true, webFileCount: Object.keys(webFiles).length },
  driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  scope: 'Actual installed Git-delivery grants, in-memory repository drafts, unchanged delayed reads, navigation guard and local origin attachment with explicit failed readback delivery.',
  setupScaffolding: ['Supported owner APIs create private accounts/team/project, configure a fixture Git identity and edit project grants.', 'Supported brief and reviewed-commit APIs make two actual local commits. Browser Connect adds only a reserved .invalid origin to this owned checkout; nothing is pushed.'],
  limitations: ['Local loopback acceptance; no public-domain/TLS commissioning claim.', 'No remote repository is created or verified. Local origin attachment does not prove GitHub OAuth, push, external authorization or model quality.', 'A labelled browser transport abort follows one successful real read; no API response body or status is fabricated.'],
  checks: [], requests: [], permissions: [], interceptedReads: [], browserMutations: [], browserErrors: [], blockedRequests: [], providerAttempts: [], cleanup: [],
  dockerAllocations: 0, agentOrCheckExecutions: 0, runtimeBudgetMS: 180000,
};
const inherited = Object.fromEntries(Object.entries(process.env).filter(([name]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ)$/.test(name)));
let service, browser, browserClosing, page, base, token, team, stage = 'setup', workspace, project, projectRoute, accessPath, member, deliveryPath, spawnError, serviceLog = '', routeError, denyProxy;
const running = new AbortController();
const readQueue = [], heldReads = new Set();
running.signal.addEventListener('abort', () => { for (const held of heldReads) held.release(); });
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

async function until(label, predicate, timeout = 20000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) { running.signal.throwIfAborted(); if (await predicate()) return; await delay(75, undefined, { signal: running.signal }); }
  assert.fail('Timed out: ' + label);
}
function nextRead(mode) {
  let seen, release;
  let cancelSeen;
  const read = { mode, seen: new Promise((resolve, reject) => {
    seen = resolve; cancelSeen = () => reject(running.signal.reason);
    running.signal.addEventListener('abort', cancelSeen, { once: true });
  }), gate: new Promise(resolve => { release = resolve; }), release: () => release(), captured: value => {
    running.signal.removeEventListener('abort', cancelSeen); seen(value);
  } };
  // Cancellation may arrive while the preceding UI click is still pending.
  // Its later await must reject, without an earlier unhandled rejection.
  void read.seen.catch(() => {});
  readQueue.push(read); heldReads.add(read); return read;
}
const remote = () => page.getByLabel('Repository origin · HTTPS', { exact: true });
const repositoryName = () => page.getByLabel('Repository name', { exact: true });
const delivery = () => page.locator('section.project-plan').filter({ has: page.getByRole('heading', { name: 'Publish your reviewed work', exact: true }) });
const button = name => delivery().getByRole('button', { name, exact: true });
async function permissions(role, grants) {
  const latest = await api('GET', `/v1/teams/${team.id}`);
  const changed = await api('PUT', accessPath + '/' + member.user.id, { expectedRevision: latest.team.revision, role, grants: { execute: false, secrets: false, admin: false, ...grants } });
  const actual = await api('GET', accessPath, undefined, 200, member.token);
  assert.equal(actual.projectId, project.id); receipt.permissions.push({ stage, teamRevision: changed.team.revision, permissions: actual.permissions });
  return actual.permissions;
}
async function refreshPermissions() {
  const before = receipt.requests.filter(row => row.path === accessPath).length;
  // Use the application's real focus-refresh listener. No store/permission data
  // is injected; the gateway must answer the new authenticated request.
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await until('new project permission read', () => receipt.requests.filter(row => row.path === accessPath).length > before);
}
async function commitFixture(purpose, message) {
  const brief = await api('GET', projectRoute + '/brief');
  await api('POST', projectRoute + '/brief', { revision: brief.revision, purpose, audience: 'Private acceptance fixture', constraints: 'No external publication', criteria: ['Retain this local context in Git'] });
  const reviewed = await api('GET', projectRoute + '/git');
  assert(!reviewed.clean && reviewed.reviewDigest, 'Fixture needs actual reviewed local changes');
  const result = await api('POST', projectRoute + '/git/commit', { message, expectedReviewDigest: reviewed.reviewDigest });
  assert.match(result.commit, /^[a-f0-9]{40}$/); assert.equal(result.sourceChanged, false);
  return result.commit;
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
  const owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'delivery-owner', displayName: 'Delivery owner', password }, 201, ''); token = owner.token;
  await api('POST', '/v1/admin/users', { username: 'delivery-member', displayName: 'Delivery member', password }, 201);
  member = await api('POST', '/v1/auth/login', { username: 'delivery-member', password }, 200, '');
  team = (await api('POST', '/v1/teams', { name: 'Git delivery acceptance' }, 201)).team;
  team = (await api('POST', `/v1/teams/${team.id}/members`, { expectedRevision: team.revision, username: 'delivery-member', role: 'editor', grants: { execute: false, secrets: false, deploy: true, admin: false } })).team;
  workspace = `/v1/teams/${team.id}/workspace`;
  await api('POST', workspace + '/v1/setup/git/identity', { name: 'Local Git delivery fixture', email: 'delivery@example.invalid' });
  project = await api('POST', workspace + '/v1/product/projects', { name: 'git-delivery-fixture' }, 201);
  assert(realpathSync(project.path).startsWith(realpathSync(fixture) + '/'), 'Project must remain under this owned fixture');
  projectRoute = workspace + `/v1/product/projects/${project.id}`;
  accessPath = `/v1/teams/${team.id}/projects/${project.id}/members`; deliveryPath = projectRoute + '/git/delivery';
  const originalHead = await commitFixture('Original local Git delivery context', 'Original local delivery context');
  stage = 'read-only-project-grant';
  const viewer = await permissions('viewer', { deploy: true }); assert(viewer.read && viewer.deploy && !viewer.edit && !viewer.execute && !viewer.secrets);
  const origin = 'https://example.invalid/luci-fixture/reviewed.git';
  await api('POST', projectRoute + '/git/remote', { url: origin }, 403, member.token);
  await api('POST', workspace + '/v1/setup/github/check', {}, 403, member.token);
  assert.equal((await api('GET', deliveryPath)).origin, undefined);

  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited, args: ['--no-proxy-server'] });
  running.signal.throwIfAborted(); receipt.browserVersion = browser.version();
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, serviceWorkers: 'block' });
  await context.route('**/*', async route => {
    try {
      const request = route.request(), url = new URL(request.url()), path = url.pathname, method = request.method();
      if (url.origin !== base) { receipt.blockedRequests.push({ method, origin: url.origin }); return await route.abort('blockedbyclient'); }
      receipt.requests.push({ stage, method, path });
      if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
        const permitted = method === 'POST' && (path === '/v1/auth/login' || (path === projectRoute + '/git/remote' && stage === 'confirmed-local-attachment'));
        receipt.browserMutations.push({ stage, method, path, permitted });
        assert(permitted, 'Unexpected browser mutation: ' + path);
      }
      if (method === 'GET' && path === deliveryPath && readQueue.length) {
        const queued = readQueue.shift(), original = await route.fetch({ maxRetries: 0 }), bytes = await original.body();
        assert.equal(original.status(), 200, 'Only a genuine authorized successful read can be delayed or transport-aborted');
        const value = JSON.parse(bytes.toString()), observed = { stage, mode: queued.mode, status: original.status(), bytes: bytes.length, sha256: sha(bytes), origin: value.origin ?? '', head: value.head };
        receipt.interceptedReads.push(observed); queued.captured(observed);
        if (queued.mode === 'hold') await queued.gate;
        heldReads.delete(queued);
        if (queued.mode === 'abort-delivery') { observed.delivered = false; return await route.abort('connectionfailed'); }
        await route.fulfill({ response: original, body: bytes }); observed.delivered = true; return;
      }
      await route.continue();
    } catch (error) { if (!running.signal.aborted) routeError ??= error; await route.abort('failed').catch(() => {}); }
  });
  page = await context.newPage(); page.setDefaultTimeout(20000); page.on('pageerror', error => receipt.browserErrors.push(scrub(error)));
  await page.goto(base); await page.getByLabel('Username', { exact: true }).fill('delivery-member'); await page.getByLabel('Password', { exact: true }).fill(password);
  await action('/v1/auth/login', page.getByRole('button', { name: 'Sign in', exact: true }));
  const firstVisit = page.getByRole('button', { name: 'Continue to projects', exact: true }), choice = page.getByLabel('Workspace', { exact: true });
  await until('workspace entry', async () => await firstVisit.isVisible() || await choice.isVisible());
  if (await firstVisit.isVisible()) await firstVisit.click();
  await choice.selectOption(team.id); await page.getByRole('treeitem', { name: /git-delivery-fixture$/ }).click();
  await page.getByRole('button', { name: 'Develop', exact: true }).click();
  await page.getByRole('navigation', { name: 'Develop tools' }).getByRole('button', { name: 'Git', exact: true }).click();
  await button('Refresh repository connection').waitFor();
  await until('initial delivery read completes', () => button('Refresh repository connection').isEnabled());
  await delivery().getByText('Create a GitHub repository', { exact: true }).click();
  for (const name of ['Connect existing repository', 'Create repository', 'Push reviewed commit', 'Check GitHub account', 'Sign in to GitHub']) assert(await button(name).isDisabled(), name + ' enabled for viewer/no secrets');
  assert(await remote().isDisabled() && await repositoryName().isDisabled());
  await screenshot('01-viewer-cannot-publish-or-use-workspace-secrets');
  check('Real project viewer with deploy but no edit cannot mutate delivery; real backend denies origin attachment and workspace GitHub check');

  stage = 'editor-without-deployment';
  const editor = await permissions('editor', { deploy: false }); assert(editor.edit && !editor.deploy);
  await refreshPermissions(); await until('resolved editor grant', () => page.getByLabel('Commit message', { exact: true }).isEnabled());
  assert(await remote().isDisabled() && await button('Connect existing repository').isDisabled());
  await api('POST', projectRoute + '/git/remote', { url: origin }, 403, member.token);
  check('Editing alone does not grant repository publication');

  stage = 'deployer-unsent-draft';
  const deployer = await permissions('editor', { deploy: true }); assert(deployer.edit && deployer.deploy && !deployer.secrets && !deployer.execute);
  await refreshPermissions(); await until('deployment controls enabled', () => remote().isEnabled());
  for (const name of ['Check GitHub account', 'Sign in to GitHub']) assert(await button(name).isDisabled(), 'Project deploy incorrectly granted workspace secrets');
  const heldInitial = nextRead('hold'); await button('Refresh repository connection').click(); await heldInitial.seen;
  await remote().fill(origin); await repositoryName().fill('my-unsent-release-name');
  assert(await button('Connect existing repository').isDisabled(), 'Pending read permitted publication');
  heldInitial.release(); await until('read after typing completes', () => button('Refresh repository connection').isEnabled());
  assert.equal(await remote().inputValue(), origin); assert.equal(await repositoryName().inputValue(), 'my-unsent-release-name');
  const persisted = await page.evaluate(() => JSON.stringify([localStorage, sessionStorage]));
  assert(!persisted.includes(origin) && !persisted.includes('my-unsent-release-name'), 'Repository field draft leaked into browser storage');
  check('An unchanged delayed connection read preserves unsent origin/name; project deployment does not grant GitHub workspace secret access');

  stage = 'head-change-with-old-read';
  const older = nextRead('hold'); await button('Refresh repository connection').click();
  const oldRead = await older.seen; assert.equal(oldRead.head, originalHead); assert.equal(oldRead.origin, '');
  const latestHead = await commitFixture('Amended local Git delivery context', 'Amended local delivery context'); assert.notEqual(latestHead, originalHead);
  await page.getByRole('button', { name: 'Refresh review', exact: true }).click();
  await delivery().getByText(new RegExp('Reviewed commit ' + latestHead.slice(0, 12))).waitFor();
  await until('new head read supersedes held old read', () => button('Refresh repository connection').isEnabled());
  assert.equal(await remote().inputValue(), origin); assert.equal(await repositoryName().inputValue(), 'my-unsent-release-name');
  await page.getByRole('button', { name: 'Define', exact: true }).click();
  const guard = page.getByRole('dialog', { name: 'Keep your changes?', exact: true }); await guard.waitFor();
  assert(await guard.getByText(/Repository settings has unsaved edits/).isVisible());
  assert.equal(await guard.getByRole('button', { name: 'Keep drafts and continue', exact: true }).count(), 0);
  await guard.getByRole('button', { name: 'Keep editing', exact: true }).click();
  assert.equal(await remote().inputValue(), origin); assert.equal(await repositoryName().inputValue(), 'my-unsent-release-name');
  await screenshot('02-local-head-refresh-and-navigation-keep-unsent-fields');
  check('Actual new local commit refreshes reviewed head without replacing origin/name; supported navigation guard keeps this in-memory draft');

  stage = 'confirmed-local-attachment';
  const failedRead = nextRead('abort-delivery');
  const attached = await action(projectRoute + '/git/remote', button('Connect existing repository'));
  assert.equal(attached.repository, origin); assert.equal(attached.status, 'attached'); assert.equal(attached.verified, false);
  await failedRead.seen;
  const readbackError = delivery().getByRole('alert');
  await readbackError.getByText('The latest repository status could not be read. Refresh the repository connection to try again.', { exact: true }).waitFor();
  assert(!(await readbackError.innerText()).includes('Start the service and retry.'), 'A failed status read incorrectly claims the service must be started');
  await delivery().getByText('attached: repository connected. Push your reviewed commit next.', { exact: true }).waitFor();
  assert.equal(await button('Create repository').count(), 0); assert(await button('Push reviewed commit').isDisabled());
  assert(await delivery().getByText('Refresh the repository connection before publishing again. Your draft is kept.', { exact: true }).isVisible());
  // The old successful read predates the real confirmed operation. Releasing
  // its identical bytes must not erase the authoritative attachment receipt.
  older.release(); await until('old unchanged response released', () => receipt.interceptedReads.find(row => row === oldRead)?.delivered === true);
  await page.evaluate(() => new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done))));
  assert.equal(await button('Create repository').count(), 0); assert.equal(await remote().inputValue(), origin);
  assert(await delivery().getByText('attached: repository connected. Push your reviewed commit next.', { exact: true }).isVisible());
  await screenshot('03-confirmed-local-attachment-with-labelled-readback-delivery-failure');
  const actual = await api('GET', deliveryPath); assert.equal(actual.origin, origin); assert.equal(actual.publication.repository, origin); assert.equal(actual.publication.status, 'attached'); assert.equal(actual.publication.verified, false);
  await button('Refresh repository connection').click(); await until('explicit real readback enables available push action', () => button('Push reviewed commit').isEnabled());
  assert.equal(await delivery().getByRole('alert').count(), 0);
  assert.equal(await delivery().getByText('Refresh the repository connection before publishing again. Your draft is kept.', { exact: true }).count(), 0);
  check('Real local origin attachment stays confirmed through labelled readback transport loss and an older unchanged response; explicit refresh reconciles it without replay or push', { origin, remoteVerified: false, followupFault: 'Actual200 GET response delivery aborted, no fabricated status/body' });

  stage = 'explicit-navigation-discard';
  const discardOrigin = 'https://example.invalid/luci-fixture/unsent-discard.git'; await remote().fill(discardOrigin);
  await page.getByRole('button', { name: 'Define', exact: true }).click(); await guard.waitFor();
  await guard.getByRole('button', { name: 'Discard edits and continue', exact: true }).click();
  await page.getByRole('heading', { name: 'Define your project', exact: true }).waitFor();
  await page.getByRole('button', { name: 'Develop', exact: true }).click();
  await page.getByRole('navigation', { name: 'Develop tools' }).getByRole('button', { name: 'Git', exact: true }).click();
  await until('saved origin after explicit discard', async () => await remote().count() > 0 && await remote().inputValue() === origin);
  assert(!(await page.evaluate(() => JSON.stringify([localStorage, sessionStorage]))).includes(discardOrigin));
  check('Explicit discard navigates away and returns to the actual saved origin, without persisting repository fields');

  stage = 'read-grant-revocation';
  const late = nextRead('hold'); await button('Refresh repository connection').click(); const lateRead = await late.seen;
  const latest = await api('GET', `/v1/teams/${team.id}`);
  await api('DELETE', accessPath + '/' + member.user.id, { expectedRevision: latest.team.revision });
  await api('GET', deliveryPath, undefined, 403, member.token);
  await refreshPermissions(); await until('private repository controls hidden after confirmed denial', async () => await remote().count() === 0);
  late.release(); await until('late authorized bytes released', () => lateRead.delivered === true);
  await page.evaluate(() => new Promise(done => requestAnimationFrame(() => requestAnimationFrame(done))));
  assert.equal(await remote().count(), 0); assert.equal(await repositoryName().count(), 0);
  assert.equal(await page.getByText(origin, { exact: true }).count(), 0);
  await screenshot('04-confirmed-read-loss-keeps-late-repository-response-hidden');
  check('Actual project read revocation hides repository fields and a delayed unchanged formerly-authorized response cannot reopen them');

  assert.equal(routeError, undefined); assert.deepEqual(receipt.browserErrors, []); assert.deepEqual(receipt.blockedRequests, []); assert.deepEqual(receipt.providerAttempts, []);
  assert.equal(receipt.browserMutations.length, 2); assert.equal(receipt.browserMutations.filter(row => row.path === projectRoute + '/git/remote').length, 1);
  assert.deepEqual((await api('GET', workspace + '/v1/sessions')).sessions, []);
  assert.equal(sha(readFileSync(binary)), binarySHA256); assert.deepEqual(inventory(), webFiles); running.signal.throwIfAborted(); receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error); receipt.failedStage = stage; if (routeError) receipt.fixtureError = scrub(routeError);
  if (page) await screenshot('failure').catch(() => {}); process.exitCode = 1;
} finally {
  clearTimeout(budget); for (const read of heldReads) read.release();
  await closeBrowser().catch(error => { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); });
  let cleaned = false; try { await stop(); cleaned = true; } catch (error) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); }
  if (denyProxy) { denyProxy.closeAllConnections(); await new Promise(done => denyProxy.close(done)); }
  if (cleaned) rmSync(fixture, { recursive: true, force: true }); else receipt.retainedFixture = fixture;
  if (receipt.cleanupErrors?.length) { receipt.passed = false; process.exitCode = 1; }
  receipt.finishedAt = new Date().toISOString(); writeFileSync(join(evidence, 'service.log'), scrub(serviceLog), { mode: 0o600 });
  writeFileSync(report, JSON.stringify(receipt, null, 2), { mode: 0o600 });
  console.log(JSON.stringify({ passed: receipt.passed, checks: receipt.checks.length, report, failedStage: receipt.failedStage }));
  process.off('SIGINT', onInterrupt); process.off('SIGTERM', onTerminate);
}

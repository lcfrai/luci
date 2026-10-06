#!/usr/bin/env node
// Installed import/recovery acceptance against a disposable HTTPS Git remote.
// Account/setup scaffolding uses API; every claimed import/recovery uses the UI.
// No model calls, Docker, external Git server, credentials or existing state.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { createServer as httpsServer } from 'node:https';
import { createServer as netServer } from 'node:net';
import { existsSync, lstatSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, readlinkSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { basename, dirname, join, resolve, sep } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '../desktop/node_modules/playwright/index.mjs';
import axe from '../desktop/node_modules/axe-core/axe.js';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..'), args = process.argv.slice(2);
const option = (name, fallback) => { const i = args.indexOf(name); if (i < 0) return fallback; assert(args[i + 1] && !args[i + 1].startsWith('--'), 'Missing value for ' + name); return args[i + 1]; };
if (args.includes('--help')) {
  console.log('node scripts/project-import-browser-uat.mjs --binary PATH --web PATH --chromium PATH --report PATH [--provenance PATH]\nLinux-only bounded installed browser proof: failed HTTPS import, corrected same-name clone, lost completed response, read-only status recovery and explicit open. Uses fresh local TLS/Git fixture and private account/setup scaffolding. No Docker/model/cloud or external traffic. Report directory must be new or empty. Package provenance must match binary/web hashes; defaults to package-provenance.json beside the report or its parent.');
  process.exit(0);
}
for (const name of ['--binary', '--web', '--chromium', '--report']) assert(option(name), 'Required: ' + name);
assert.equal(process.platform, 'linux', 'Process ownership checks require Linux /proc');
const binary = realpathSync(resolve(option('--binary'))), web = realpathSync(resolve(option('--web'))), executable = realpathSync(resolve(option('--chromium')));
const report = resolve(option('--report')), evidence = dirname(report);
mkdirSync(dirname(evidence), { recursive: true, mode: 0o700 });
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Preserve earlier evidence; use a new or empty report directory');
else mkdirSync(evidence, { mode: 0o700 });
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const binaryHash = sha(readFileSync(binary)), webHash = sha(readFileSync(join(web, 'index.html')));
const provenancePath = option('--provenance') ?? [join(dirname(evidence), 'package-provenance.json'), join(dirname(binary), 'package-provenance.json')].find(existsSync);
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
const fixture = mkdtempSync(join(tmpdir(), 'luci-import-browser-'));
for (const folder of ['home', 'state', 'projects', 'bin', 'remote', 'tls', 'empty-ca']) mkdirSync(join(fixture, folder), { mode: 0o700 });
const secrets = new Set(), secret = () => { const value = randomBytes(32).toString('base64url'); secrets.add(value); return value; };
const password = secret(), lifecycle = secret();
const scrub = value => { let text = String(value?.message ?? value); for (const item of secrets) text = text.replaceAll(item, '[redacted]'); return text; };
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit, sourceDirty: false,
  package: { path: dirname(binary), archiveSHA256: provenance.archiveSHA256, provenanceSHA256: sha(provenanceBytes), verifiedArchiveWebFiles: webFiles.length, binaryAndAllWebFilesMatchArchive: true },
  binarySHA256: binaryHash, webIndexSHA256: webHash, webInventorySHA256: sha(JSON.stringify(webFiles)), driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  driverCheckout: { commit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repo, encoding: 'utf8' }).trim(), dirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: repo, encoding: 'utf8' }).trim() },
  scope: 'Installed UI import and explicit read-only recovery against real Git over a private local HTTPS fixture. One failed import, one corrected import, one completed import whose browser response is deliberately lost.',
  limitations: ['Local public fixture repositories only; no private repository authentication or external host claim.', 'Axe scans and focus checks are bounded observations, not manual WCAG conformance or novice-usability proof.', 'No drawing, implementation, deployment, provider or paid-service acceptance.'],
  setupScaffolding: ['API bootstrap of one private account and one empty seed project, plus Git author identity', 'Fresh two-commit bare Git remote and CA/server certificate', 'Owned PATH launcher executes real Git with fixture-only CA trust; no TLS verification bypass'],
  checks: [], attempts: [], importRequests: [], statusRequests: [], gitHTTPRequests: [], accessibility: [], keyboard: [], cleanup: [],
  browserErrors: [], externalRequests: [], externalModelCalls: 0, cloudResourcesCreated: 0, dockerAllocations: 0,
};
const check = (name, details = {}) => { receipt.checks.push({ name, ...details }); console.log('PASS ' + name); };
const inherited = Object.fromEntries(Object.entries(process.env).filter(([name]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ)$/.test(name)));
const gitEnv = { ...inherited, HOME: join(fixture, 'home'), GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1', GIT_TERMINAL_PROMPT: '0' };
const realGit = realpathSync(execFileSync('which', ['git'], { env: inherited, encoding: 'utf8' }).trim());
const git = (directory, ...values) => execFileSync(realGit, values, { cwd: directory, env: gitEnv, encoding: 'utf8', timeout: 15000, stdio: ['ignore', 'pipe', 'pipe'] }).trim();
const cloneLog = join(fixture, 'clone-events.jsonl');
const cloneEvents = () => existsSync(cloneLog) ? readFileSync(cloneLog, 'utf8').trim().split('\n').filter(Boolean).map(row => JSON.parse(row)) : [];
const starts = () => cloneEvents().filter(row => row.event === 'start');
let service, browser, page, remote, base, token, environment, spawnError, serviceLog = '', stage = 'fixture', interceptError, droppedProject, expectedFailureID;
const remoteSockets = new Set();
async function freePort() { const server = netServer(); await new Promise((done, reject) => server.once('error', reject).listen(0, '127.0.0.1', done)); const port = server.address().port; await new Promise(done => server.close(done)); return port; }
async function api(method, route, body, expected = 200, auth = token) {
  const response = await fetch(base + route, { method, headers: { 'Content-Type': 'application/json', ...(auth ? { Authorization: 'Bearer ' + auth } : {}) }, body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(15000) });
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
  while (Date.now() < deadline && (tracked.some(stillOwned) || ownedProcesses().length)) await delay(100);
  const remaining = [...tracked.filter(stillOwned), ...ownedProcesses()];
  if (remaining.length) {
    receipt.cleanupErrors ??= []; receipt.cleanupErrors.push('Service or Git worker needed a forced stop');
    for (const row of remaining) {
      try { if (stillOwned(row) && readlinkSync(`/proc/${row.pid}/exe`) === row.executable) process.kill(row.group === row.pid ? -row.pid : row.pid, 'SIGKILL'); }
      catch (error) { if (!['ENOENT', 'ESRCH'].includes(error.code)) throw error; }
    }
    const forcedDeadline = Date.now() + 5000; while (Date.now() < forcedDeadline && (tracked.some(stillOwned) || ownedProcesses().length)) await delay(100);
  }
  const stopped = !tracked.some(stillOwned) && ownedProcesses().length === 0;
  receipt.cleanup.push({ serviceAndGitProcesses: tracked.length, allStopped: stopped, exitCode: service.exitCode });
  if (service.exitCode !== 0) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push('Owned service did not exit normally'); }
  assert(stopped, 'An owned process is still running'); return stopped;
}
async function pendingImport() {
  return page.evaluate(() => Object.entries(localStorage).filter(([key]) => key.startsWith('luci.project-import.v1:')).map(([key, value]) => ({ key, value: JSON.parse(value) })));
}
async function openImport() {
  await page.getByRole('button', { name: 'New project', exact: true }).first().click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('navigation', { name: 'Project source' }).getByRole('button', { name: 'Import repository', exact: true }).click();
  await page.getByRole('dialog', { name: 'Import a repository', exact: true }).waitFor();
  return dialog;
}
async function captureState(name, width, attemptId) {
  await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
  const dialog = page.getByRole('dialog', { name: 'Import a repository', exact: true });
  await dialog.waitFor();
  if (attemptId) {
    const summary = dialog.getByText('Import details', { selector: 'summary', exact: true });
    await summary.focus();
    if (!(await summary.evaluate(node => node.parentElement.open))) await page.keyboard.press('Enter');
    await dialog.getByText(attemptId, { exact: true }).waitFor();
    receipt.keyboard.push({ state: name, importDetailsOpenedByKeyboard: true, visibleAttemptId: attemptId });
  }
  // Test focus wrapping without activation; both directions must stay in the dialog.
  const controls = dialog.locator('button:enabled, input:enabled, select:enabled, a[href]').filter({ visible: true });
  const first = controls.first(), last = controls.last();
  await last.focus(); await page.keyboard.press('Tab'); assert(await first.evaluate(node => document.activeElement === node), 'Tab escaped ' + name);
  await page.keyboard.press('Shift+Tab'); assert(await last.evaluate(node => document.activeElement === node), 'Shift+Tab escaped ' + name);
  receipt.keyboard.push({ state: name, viewport: width, tabWrap: true, shiftTabWrap: true });
  const geometry = await dialog.evaluate(node => ({ viewport: innerWidth, documentWidth: document.documentElement.scrollWidth, dialogWidth: node.getBoundingClientRect().width, scrollWidth: node.scrollWidth, clientWidth: node.clientWidth }));
  assert(geometry.documentWidth <= geometry.viewport + 1 && geometry.scrollWidth <= geometry.clientWidth + 1, 'Horizontal overflow in ' + name);
  await page.evaluate(axe.source);
  const audit = await dialog.evaluate(async node => {
    const result = await window.axe.run(node, { runOnly: ['wcag2a', 'wcag2aa', 'wcag21aa'] });
    const select = rows => rows.map(row => ({ id: row.id, impact: row.impact, description: row.description, help: row.help, nodes: row.nodes.map(node => ({ target: node.target, failureSummary: node.failureSummary })) }));
    return { violations: select(result.violations), incomplete: select(result.incomplete), passes: result.passes.map(row => row.id) };
  });
  writeFileSync(join(evidence, name + '-accessibility.json'), JSON.stringify(audit, null, 2), { mode: 0o600 });
  receipt.accessibility.push({ state: name, viewport: width, violations: audit.violations.map(row => row.id), incomplete: audit.incomplete.map(row => row.id), geometry });
  await page.screenshot({ path: join(evidence, name + '.png'), fullPage: true });
  assert.deepEqual(audit.violations, [], 'Accessibility violation in ' + name);
}
function validateImported(project, name, expected) {
  assert.equal(project.name, name); assert.match(project.id, /^p-/);
  assert(realpathSync(project.path).startsWith(realpathSync(fixture) + sep), 'Imported project escaped the owned fixture');
  assert.equal(basename(project.path), name);
  const history = git(project.path, 'rev-list', '--reverse', 'HEAD').split('\n');
  const files = git(project.path, 'ls-files').split('\n');
  assert.deepEqual(history, expected.history); assert.deepEqual(files, Object.keys(expected.files).sort());
  for (const [file, content] of Object.entries(expected.files)) assert.equal(readFileSync(join(project.path, file), 'utf8'), content);
  return { projectId: project.id, name, commits: history, files: files.map(file => ({ path: file, sha256: sha(readFileSync(join(project.path, file))) })) };
}

try {
  const tls = join(fixture, 'tls'), source = join(fixture, 'remote/source'); mkdirSync(source, { mode: 0o700 });
  const openssl = values => execFileSync('openssl', values, { cwd: tls, timeout: 15000, stdio: ['ignore', 'pipe', 'pipe'] });
  openssl(['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', 'ca.key', '-out', 'ca.pem', '-days', '1', '-subj', '/CN=Luci Import Fixture CA', '-addext', 'basicConstraints=critical,CA:TRUE', '-addext', 'keyUsage=critical,keyCertSign,cRLSign']);
  openssl(['req', '-newkey', 'rsa:2048', '-nodes', '-keyout', 'server.key', '-out', 'server.csr', '-subj', '/CN=127.0.0.1']);
  writeFileSync(join(tls, 'server.ext'), 'subjectAltName=IP:127.0.0.1\nbasicConstraints=critical,CA:FALSE\nkeyUsage=critical,digitalSignature,keyEncipherment\nextendedKeyUsage=serverAuth\n', { mode: 0o600 });
  openssl(['x509', '-req', '-in', 'server.csr', '-CA', 'ca.pem', '-CAkey', 'ca.key', '-CAcreateserial', '-out', 'server.pem', '-days', '1', '-extfile', 'server.ext']);
  git(source, 'init', '--initial-branch=main');
  const expected = { files: { 'README.md': '# Imported garden notes\nPreserve this real repository and its history.\n', 'notes.txt': 'Seedling notes\nSecond committed observation\n' }, history: [] };
  writeFileSync(join(source, 'README.md'), expected.files['README.md']); writeFileSync(join(source, 'notes.txt'), 'Seedling notes\n');
  git(source, 'add', '.'); git(source, '-c', 'user.name=Import fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-m', 'Initial garden notes');
  writeFileSync(join(source, 'notes.txt'), expected.files['notes.txt']); git(source, 'add', '.'); git(source, '-c', 'user.name=Import fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-m', 'Record another observation');
  expected.history = git(source, 'rev-list', '--reverse', 'HEAD').split('\n');
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
  environment = { ...inherited, PATH: join(fixture, 'bin') + ':' + inherited.PATH, HOME: join(fixture, 'home'), USERPROFILE: join(fixture, 'home'),
    LUCI_ADDR: `127.0.0.1:${port}`, LUCI_BROWSER_PREVIEW_ADDR: `127.0.0.1:${previewPort}`, LUCI_BROWSER_PREVIEW_PORT: String(previewPort), LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'), LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: join(dirname(binary), 'runtime'),
    LUCI_DOCKER_HOST: 'unix://' + join(fixture, 'absent-docker.sock'), DOCKER_HOST: 'unix://' + join(fixture, 'absent-docker.sock'), LUCI_BACKEND: 'docker', LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, AWS_EC2_METADATA_DISABLED: 'true', GIT_CONFIG_GLOBAL: '/dev/null', GIT_CONFIG_NOSYSTEM: '1' };
  stage = 'service-start'; service = spawn(binary, [], { cwd: fixture, env: environment, detached: true, stdio: ['ignore', 'pipe', 'pipe'] }); service.once('error', error => { spawnError = error; });
  for (const stream of [service.stdout, service.stderr]) stream.on('data', bytes => { serviceLog = (serviceLog + scrub(bytes.toString())).slice(-65536); });
  const readyBy = Date.now() + 30000; let status;
  while (Date.now() < readyBy) { if (spawnError) throw spawnError; assert.equal(service.exitCode, null, 'Service exited before readiness'); try { status = await api('GET', '/v1/control-plane/status', undefined, 200, ''); break; } catch { await delay(100); } }
  assert(status, 'Service never became ready'); receipt.version = status.version;
  const bootstrap = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(); secrets.add(bootstrap);
  const owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'import-owner', displayName: 'Import owner', password }, 201, ''); token = owner.token;
  await api('POST', '/v1/setup/git/identity', { name: 'Import owner', email: 'import@example.invalid' });
  const seed = await api('POST', '/v1/product/projects', { name: 'import-entry-seed' }, 201);
  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited }); receipt.browserVersion = browser.version();
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, serviceWorkers: 'block' });
  await context.route('**/*', route => { const url = new URL(route.request().url()); if (url.origin !== base) { receipt.externalRequests.push({ method: route.request().method(), origin: url.origin, path: url.pathname }); return route.abort('blockedbyclient'); } return route.continue(); });
  page = await context.newPage(); page.setDefaultTimeout(15000);
  page.on('pageerror', error => receipt.browserErrors.push(scrub(error)));
  page.on('request', request => { const url = new URL(request.url());
    if (url.origin !== base) return;
    if (request.method() === 'POST' && url.pathname === '/v1/product/projects') { const body = request.postDataJSON(); receipt.importRequests.push({ name: body.name, remoteUrl: body.remoteUrl, importAttemptId: body.importAttemptId }); }
    if (request.method() === 'GET' && url.pathname.startsWith('/v1/product/project-imports/')) receipt.statusRequests.push({ path: url.pathname });
  });
  stage = 'login'; await page.goto(base); await page.getByLabel('Username', { exact: true }).fill('import-owner'); await page.getByLabel('Password', { exact: true }).fill(password); await page.getByRole('button', { name: 'Sign in', exact: true }).click(); await page.getByRole('button', { name: 'New project', exact: true }).first().waitFor();
  let dialog = await openImport(); await captureState('import-entry-wide', 1440);
  const retryName = 'garden-import-retry', lostName = 'garden-import-recovered';
  stage = 'failed-import'; await dialog.getByLabel('Project name', { exact: true }).fill(retryName); await dialog.getByRole('textbox', { name: /^Repository URL/ }).fill(badURL); await dialog.getByRole('checkbox', { name: 'I trust the source of this repository.', exact: true }).check();
  const failedResponse = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === '/v1/product/projects');
  await dialog.locator('form').getByRole('button', { name: 'Import repository', exact: true }).click(); const failed = await failedResponse; assert.equal(failed.status(), 422); const failure = await failed.json();
  assert.equal(failure.error, 'git_failed'); assert.equal(failure.attempt.status, 'failed'); expectedFailureID = failure.attempt.id;
  assert.match(expectedFailureID, /^imp-[a-f0-9]{24}$/); assert.equal(receipt.importRequests[0].importAttemptId, expectedFailureID);
  await dialog.getByRole('button', { name: 'Edit import details', exact: true }).waitFor();
  const catalogueAfterFailure = await api('GET', '/v1/projects'); assert.deepEqual(catalogueAfterFailure.projects.map(project => project.name), [seed.name]);
  assert(!existsSync(join(dirname(seed.path), retryName))); assert.equal(await page.getByRole('treeitem', { name: new RegExp(retryName) }).count(), 0);
  assert.equal(typeof failure.attempt.retainedPath, 'string');
  const retained = resolve(dirname(seed.path), failure.attempt.retainedPath);
  assert(retained.startsWith(join(dirname(seed.path), '.li') + sep), 'Failure evidence must identify the private staging tree');
  assert(lstatSync(retained).isDirectory() && !lstatSync(retained).isSymbolicLink(), 'Failed staging directory was not retained');
  const failurePending = await pendingImport(); assert.equal(failurePending.length, 1); assert.equal(failurePending[0].value.id, expectedFailureID);
  receipt.attempts.push({ id: expectedFailureID, name: retryName, status: 'failed', hiddenPartialRetained: true });
  await captureState('import-failed-mobile', 390, expectedFailureID); check('Failed real HTTPS clone leaves no visible or final-name project and retains scoped recovery identity');
  stage = 'corrected-import'; await page.setViewportSize({ width: 1440, height: 1000 }); await dialog.getByRole('button', { name: 'Edit import details', exact: true }).click();
  assert.equal(await dialog.getByLabel('Project name', { exact: true }).inputValue(), retryName); assert.equal(await dialog.getByRole('checkbox').isChecked(), false);
  await dialog.getByRole('textbox', { name: /^Repository URL/ }).fill(goodURL); assert.equal(await dialog.locator('form').getByRole('button', { name: 'Import repository', exact: true }).isEnabled(), false); await dialog.getByRole('checkbox').check();
  const correctedResponse = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === '/v1/product/projects');
  await dialog.locator('form').getByRole('button', { name: 'Import repository', exact: true }).click(); const corrected = await correctedResponse; assert.equal(corrected.status(), 201); const imported = await corrected.json();
  await page.getByRole('dialog').waitFor({ state: 'hidden' }); await page.getByRole('treeitem', { name: new RegExp(retryName) }).waitFor();
  const correctedID = receipt.importRequests[1].importAttemptId; assert.notEqual(correctedID, expectedFailureID); assert.equal((await pendingImport()).length, 0);
  const importedEvidence = validateImported(imported, retryName, expected);
  for (const [file, content] of Object.entries(expected.files)) assert.equal((await api('GET', `/v1/product/projects/${imported.id}/files?path=${encodeURIComponent(file)}`)).content, content);
  receipt.attempts.push({ id: correctedID, status: 'complete', ...importedEvidence });
  await page.screenshot({ path: join(evidence, 'corrected-import-open.png'), fullPage: true }); check('Corrected URL imports under the same intended name with exact original files and both real commits');
  stage = 'lost-response'; dialog = await openImport(); await dialog.getByLabel('Project name', { exact: true }).fill(lostName); await dialog.getByRole('textbox', { name: /^Repository URL/ }).fill(goodURL); await dialog.getByRole('checkbox').check();
  let responseDropped = false;
  await page.route(base + '/v1/product/projects', async route => {
    if (route.request().method() !== 'POST' || route.request().postDataJSON().name !== lostName) return route.fallback();
    try { const response = await route.fetch({ timeout: 30000, maxRetries: 0, maxRedirects: 0 }); assert.equal(response.status(), 201); droppedProject = await response.json(); responseDropped = true; await route.abort('failed'); }
    catch (error) { interceptError = error; await route.abort('failed').catch(() => {}); }
  });
  await dialog.locator('form').getByRole('button', { name: 'Import repository', exact: true }).click();
  await dialog.getByRole('alert').filter({ hasText: 'The import result could not be confirmed.' }).waitFor(); assert.ifError(interceptError); assert(responseDropped && droppedProject);
  const pendingBeforeReload = await pendingImport(); assert.equal(pendingBeforeReload.length, 1); const lostID = pendingBeforeReload[0].value.id;
  assert.equal(lostID, receipt.importRequests[2].importAttemptId); assert.equal(pendingBeforeReload[0].value.name, lostName); assert.equal(await dialog.getByRole('button', { name: 'Retry import', exact: true }).count(), 0);
  const postsBeforeRecovery = receipt.importRequests.length, clonesBeforeRecovery = starts().length;
  await captureState('import-unconfirmed-mobile', 390, lostID); await delay(750);
  assert.equal(receipt.importRequests.length, postsBeforeRecovery); assert.equal(starts().length, clonesBeforeRecovery);
  receipt.lostResponse = { id: lostID, realServerStatus: 201, realServerProjectId: droppedProject.id, fault: 'Browser route fetched actual completed response once, then aborted delivery; response was not fabricated', pendingScopeKeySHA256: sha(pendingBeforeReload[0].key), noAutoRetryObservationMS: 750 };
  check('Lost completed response remains an unknown browser result with saved request and no automatic retry');
  stage = 'status-recovery'; await page.unroute(base + '/v1/product/projects'); await page.setViewportSize({ width: 1440, height: 1000 }); await page.reload(); await page.getByRole('button', { name: 'New project', exact: true }).first().waitFor();
  const automaticRead = page.waitForResponse(response => response.request().method() === 'GET' && new URL(response.url()).pathname === '/v1/product/project-imports/' + lostID);
  dialog = await openImport(); const automatic = await automaticRead; assert.equal(automatic.status(), 200); assert.equal((await automatic.json()).status, 'complete');
  assert.equal((await pendingImport())[0].value.id, lostID); await dialog.getByRole('button', { name: 'Open imported project', exact: true }).waitFor();
  const explicitRead = page.waitForResponse(response => response.request().method() === 'GET' && new URL(response.url()).pathname === '/v1/product/project-imports/' + lostID);
  await dialog.getByRole('button', { name: 'Check import status', exact: true }).focus(); await page.keyboard.press('Enter');
  const checked = await explicitRead; assert.equal(checked.status(), 200); const recovered = await checked.json(); assert.equal(recovered.id, lostID); assert.equal(recovered.status, 'complete'); assert.deepEqual(recovered.project, droppedProject);
  assert.equal(receipt.importRequests.length, postsBeforeRecovery); assert.equal(starts().length, clonesBeforeRecovery); assert.equal(receipt.statusRequests.length, 2);
  await captureState('import-complete-wide', 1440, lostID);
  assert.equal(await dialog.isVisible(), true, 'Status observation should not automatically navigate');
  await dialog.getByRole('button', { name: 'Open imported project', exact: true }).focus(); await page.keyboard.press('Enter');
  await page.getByRole('dialog').waitFor({ state: 'hidden' }); await page.getByRole('treeitem', { name: new RegExp(lostName) }).waitFor();
  assert.equal(await page.getByRole('treeitem', { name: new RegExp(lostName) }).getAttribute('aria-selected'), 'true');
  assert.equal((await pendingImport()).length, 0); assert.equal(receipt.importRequests.length, 3); assert.equal(starts().length, 3);
  receipt.attempts.push({ id: lostID, status: 'complete', ...validateImported(droppedProject, lostName, expected) });
  assert.deepEqual((await api('GET', '/v1/projects')).projects.map(project => project.name).sort(), [seed.name, retryName, lostName].sort());
  await page.screenshot({ path: join(evidence, 'recovered-import-open.png'), fullPage: true });
  check('Reload and explicit keyboard status check use exactly two GETs, no second POST or clone, then require explicit open');
  assert.deepEqual(receipt.browserErrors, []); assert.deepEqual(receipt.externalRequests, []);
  assert.equal(cloneEvents().filter(row => row.event === 'finish' && row.status === 0).length, 2);
  assert.equal(cloneEvents().filter(row => row.event === 'finish' && row.status !== 0).length, 1);
  assert(!cloneEvents().some(row => row.event === 'forbidden-remote'));
  receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error); receipt.failedStage = stage;
  if (page) {
    await page.screenshot({ path: join(evidence, 'failure.png'), fullPage: true }).catch(() => {});
    try { writeFileSync(join(evidence, 'failure-dom.txt'), scrub(await page.locator('body').innerText({ timeout: 3000 })), { mode: 0o600 }); } catch { /* Receipt still written. */ }
  }
} finally {
  let browserStopped = !browser, serviceStopped = !service, remoteStopped = !remote;
  const cleanupFailure = error => { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); };
  try { await browser?.close(); browserStopped = true; receipt.cleanup.push({ browserClosed: true }); } catch (error) { cleanupFailure(error); }
  try { serviceStopped = await stopService(); } catch (error) { cleanupFailure(error); }
  try { if (remote?.listening) { for (const socket of remoteSockets) socket.destroy(); await new Promise((done, reject) => remote.close(error => error ? reject(error) : done())); } remoteStopped = true; receipt.cleanup.push({ fixtureHTTPSServerClosed: true }); } catch (error) { cleanupFailure(error); }
  receipt.cloneEvents = cloneEvents().map(({ cwd, ...event }) => ({ ...event, ...(cwd ? { privateStagingDirectory: cwd.startsWith(fixture + sep) } : {}) }));
  writeFileSync(join(evidence, 'service.log'), scrub(serviceLog), { mode: 0o600 });
  try {
    receipt.binaryUnchanged = sha(readFileSync(binary)) === binaryHash; receipt.webUnchanged = sha(JSON.stringify(webInventory(web))) === receipt.webInventorySHA256;
    assert(receipt.binaryUnchanged && receipt.webUnchanged, 'Measured package changed during execution');
  } catch (error) { cleanupFailure(error); }
  if (browserStopped && serviceStopped && remoteStopped) { try { rmSync(fixture, { recursive: true }); receipt.privateFixtureRemoved = true; } catch (error) { cleanupFailure(error); } }
  else { receipt.privateFixtureRemoved = false; receipt.retainedPrivateFixture = fixture; }
  receipt.passed &&= !receipt.cleanupErrors?.length; receipt.finishedAt = new Date().toISOString();
  writeFileSync(report, JSON.stringify(receipt, null, 2), { mode: 0o600, flag: 'wx' });
  console.log(JSON.stringify({ passed: receipt.passed, checks: receipt.checks.length, importPOSTs: receipt.importRequests.length, statusGETs: receipt.statusRequests.length, report, error: receipt.error, cleanupErrors: receipt.cleanupErrors }));
  if (!receipt.passed) process.exitCode = 1;
}

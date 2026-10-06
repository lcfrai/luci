#!/usr/bin/env node
// Rendered standalone PostgreSQL operations, with real SQL data as a fixture
// witness. Account/project creation and SQL inserts are declared scaffolding;
// no application login, RLS, model quality or public hosting is claimed.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { createWriteStream, existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, readlinkSync, realpathSync, lstatSync, statfsSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:net';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '../desktop/node_modules/playwright/index.mjs';

const repository = resolve(dirname(fileURLToPath(import.meta.url)), '..'), args = process.argv.slice(2);
const option = (name, fallback) => { const index = args.indexOf(name); if (index < 0) return fallback; assert(args[index + 1] && !args[index + 1].startsWith('--'), 'Missing ' + name); return args[index + 1]; };
const postgresImage = 'postgres:16.15-bookworm@sha256:efedf3595f1d6f415c08568ba171029bf54052e754cc9f030e3f2412b21f3d67';
if (args.includes('--help')) {
  console.log('node scripts/database-recovery-browser-uat.mjs --binary PATH --web PATH [--chromium PATH] --report NEW_RESULT_PATH [--provenance PATH]\nRequires exclusive runtime approval, an already installed exact pinned PostgreSQL16.15 image, >=2GiB free on the actual Docker data filesystem, and capacity for the service policy database limit (at least1GiB RAM/1CPU recommended for this tiny fixture). Never pulls images. One PostgreSQL container, one owned volume/network; candidate databases share that retained volume. Browser backup/download/import/stop/restore/rollback and service-restart discovery. SQL fixture witnesses are explicit; no app authentication/RLS/full Supabase or model-quality claim.');
  process.exit(0);
}
assert.equal(process.platform, 'linux'); assert.notEqual(process.getuid(), 0, 'Run the compiled service as its ordinary installation user');
const binary = resolve(option('--binary', 'artifacts/luci')), web = resolve(option('--web', 'desktop/dist'));
assert(existsSync(binary)); assert(existsSync(join(web, 'index.html')));
const docker = (...values) => execFileSync('docker', values, { encoding: 'utf8', timeout: 30_000, maxBuffer: 4 * 1024 * 1024 }).trim();
const image = JSON.parse(docker('image', 'inspect', postgresImage))[0];
assert.match(image.Id, /^sha256:[a-f0-9]{64}$/);
const info = JSON.parse(docker('info', '--format', '{{json .}}'));
assert.equal(info.OSType, 'linux'); assert(info.MemTotal >= 1024 ** 3 && info.NCPU >= 1);
assert(info.DockerRootDir && existsSync(info.DockerRootDir), 'Inspect the actual Docker data filesystem before accepting this fixture');
const capacity = statfsSync(info.DockerRootDir), available = capacity.bavail * capacity.bsize;
assert(available >= 2 * 1024 ** 3, 'Need 2GiB actual free Docker storage for this bounded fixture; no sparse mounts, image substitution or cleanup bypass');
const report = resolve(option('--report', 'artifacts/uat/database-browser-' + Date.now() + '/result.json')), evidence = dirname(report);
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Preserve earlier evidence; use a new or empty report directory');
else mkdirSync(evidence, { recursive: true, mode: 0o700 });
const fixture = mkdtempSync('/tmp/luci-database-browser-');
for (const name of ['state', 'projects']) mkdirSync(join(fixture, name), { mode: 0o700 });
writeFileSync(join(fixture, 'empty-git-config'), '', { mode: 0o600 });
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const provenancePath = option('--provenance') ?? [join(dirname(evidence), 'package-provenance.json'), join(dirname(binary), 'package-provenance.json')].find(existsSync);
assert(provenancePath, 'Supply matching --provenance or package-provenance.json beside the report directory');
const provenanceBytes = readFileSync(provenancePath), provenance = JSON.parse(provenanceBytes);
assert.equal(provenance.binarySHA256, sha(readFileSync(binary)), 'Package provenance binary hash mismatch');
assert.equal(provenance.webIndexSHA256, sha(readFileSync(join(web, 'index.html'))), 'Package provenance web hash mismatch');
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
assert.equal(archiveMembers.binary, sha(readFileSync(binary)), 'Installed binary differs from the verified package archive');
assert.deepEqual(Object.fromEntries(webFiles.map(file => [file.path, file.sha256])), archiveMembers.web, 'Installed web files differ from the verified package archive');
writeFileSync(join(evidence, 'package-provenance.json'), provenanceBytes, { mode: 0o600, flag: 'wx' });
writeFileSync(join(evidence, 'driver.mjs'), readFileSync(fileURLToPath(import.meta.url)), { mode: 0o600, flag: 'wx' });
const password = randomUUID() + randomUUID(), lifecycle = randomUUID() + randomUUID();
const secrets = [password, lifecycle], scrub = value => secrets.reduce((text, secret) => text.replaceAll(secret, '[redacted]'), String(value));
const result = {
  passed: false, startedAt: new Date().toISOString(), fixture, sourceCommit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repository, encoding: 'utf8' }).trim(),
  sourceDirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: repository, encoding: 'utf8' }).trim(),
  packageSourceCommit: provenance.sourceCommit, packageSourceDirty: provenance.sourceDirty, packageArchiveSHA256: provenance.archiveSHA256, packageProvenanceSHA256: sha(provenanceBytes), webFiles,
  binarySHA256: sha(readFileSync(binary)), webIndexSHA256: sha(readFileSync(join(web, 'index.html'))), driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  image: { reference: postgresImage, id: image.Id, logicalBytes: image.Size }, preflight: { dockerRoot: info.DockerRootDir, availableBytes: available, dockerMemoryBytes: info.MemTotal, dockerCPUs: info.NCPU, reserveBytes: 2 * 1024 ** 3 },
  scope: 'Rendered standalone PostgreSQL backup/download/import/restore/rollback with real fixture data, retained original database and installed service restart.',
  scaffolding: ['Isolated owner and project created by authenticated API', 'Fixture records inserted and observed with psql in the verified owned PostgreSQL container'],
  limitations: ['No application login, user-role isolation or row-level-security acceptance', 'No full Supabase stack or storage-file restore', 'No live model, source deployment, public cloud or paid-provider calls', 'Browser UI over the installed service; no native desktop-shell claim'],
  modelCalls: 0, cloudResourcesCreated: 0, preexistingImageRequired: true, checks: [], browserMutations: [], cleanup: [],
};
const check = (name, detail = {}) => { result.checks.push({ name, passed: true, ...detail }); console.log('PASS ' + name); };
const labels = { project: 'ai.lcfr.luci.platform.project', instance: 'ai.lcfr.luci.platform.instance', resource: 'ai.lcfr.luci.platform.resource' };
let service, browser, page, origin, token, project, route, spawnError, ownedInstance, stage = 'startup';
const browserErrors = [], log = createWriteStream(join(evidence, 'control-plane.log'), { mode: 0o600 });
const listener = createServer(); await new Promise(done => listener.listen(0, '127.0.0.1', done));
const address = '127.0.0.1:' + listener.address().port; await new Promise(done => listener.close(done)); origin = 'http://' + address;
const environment = { ...Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('LUCI_'))),
  LUCI_ADDR: address, LUCI_BACKEND: 'docker', LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'),
  LUCI_RUNTIME_DIR: join(dirname(binary), 'runtime'), LUCI_WEB_DIR: web, LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle,
  GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: join(fixture, 'empty-git-config'), OPENAI_API_KEY: '', ANTHROPIC_API_KEY: '',
};
async function request(method, path, body, expected = [200, 201], headers = {}) {
  const response = await fetch(origin + path, { method, redirect: 'error', signal: AbortSignal.timeout(30_000), headers: { ...(token ? { Authorization: 'Bearer ' + token } : {}), ...(body === undefined ? {} : { 'Content-Type': 'application/json' }), ...headers }, body: body === undefined ? undefined : JSON.stringify(body) });
  assert([expected].flat().includes(response.status), `${method} ${path}: ${response.status}`);
  return response.json();
}
async function observe(path) { return request('GET', path, undefined, 200); }
async function until(name, read, timeout = 45_000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) { if (spawnError) throw spawnError; const value = await read(); if (value) return value; await delay(200); }
  throw new Error(name + ' timed out');
}
async function start() {
  spawnError = null; service = spawn(binary, [], { cwd: fixture, env: environment, stdio: ['ignore', 'pipe', 'pipe'] });
  service.on('error', error => { spawnError = error; }); service.stdout.pipe(log, { end: false }); service.stderr.pipe(log, { end: false });
  const status = await until('Installed service ready', async () => { try { return await observe('/v1/control-plane/status'); } catch { return false; } });
  result.version = status.version; assert.equal(status.product, 'Luci');
}
function processIdentity(pid) {
  try { const raw = readFileSync(`/proc/${pid}/stat`, 'utf8'), fields = raw.slice(raw.lastIndexOf(')') + 2).trim().split(/\s+/); return { pid: Number(pid), start: fields[19], status: fields[0] }; }
  catch (error) { if (['ENOENT', 'ESRCH'].includes(error.code)) return null; throw error; }
}
function ownedProcesses() {
  const found = [], state = join(fixture, 'state');
  for (const pid of readdirSync('/proc').filter(name => /^\d+$/.test(name))) {
    const identity = processIdentity(pid); if (!identity || identity.status === 'Z') continue;
    try {
      const values = Object.fromEntries(readFileSync(`/proc/${pid}/environ`, 'utf8').split('\0').filter(Boolean).map(value => { const at = value.indexOf('='); return [value.slice(0, at), value.slice(at + 1)]; }));
      if (values.LUCI_STATE_DIR !== state && !values.LUCI_STATE_DIR?.startsWith(state + '/control-plane/')) continue;
      const executable = readlinkSync(`/proc/${pid}/exe`);
      if (processIdentity(pid)?.start === identity.start) found.push({ ...identity, executable });
    } catch (error) { if (!['ENOENT', 'ESRCH', 'EACCES', 'EPERM'].includes(error.code)) throw error; }
  }
  return found;
}
async function stopService() {
  if (service && service.exitCode === null && service.signalCode === null) {
    const identity = ownedProcesses().find(row => row.pid === service.pid);
    assert(identity && identity.executable === realpathSync(binary), 'Cannot prove installed service ownership');
    await request('POST', '/v1/control-plane/shutdown', undefined, 200, { 'X-Luci-Lifecycle-Token': lifecycle });
  }
  await until('Installed gateway and private worker stopped', () => ownedProcesses().length === 0 && (!service || service.exitCode !== null || service.signalCode !== null));
  if (service) assert.equal(service.exitCode, 0);
}
async function forceStopOwned() {
  const rows = ownedProcesses();
  for (const row of rows) {
    try { if (processIdentity(row.pid)?.start === row.start && readlinkSync(`/proc/${row.pid}/exe`) === row.executable) process.kill(row.pid, 'SIGTERM'); }
    catch (error) { if (!['ENOENT', 'ESRCH'].includes(error.code)) throw error; }
  }
  await until('Owned processes drained after failure', () => ownedProcesses().length === 0, 10_000);
}
function container() {
  assert(project?.id && ownedInstance);
  const ids = docker('ps', '--all', '--filter', `label=${labels.project}=${project.id}`, '--filter', `label=${labels.instance}=${ownedInstance}`, '--filter', `label=${labels.resource}=postgres`, '--format', '{{.ID}}').split(/\s+/).filter(Boolean);
  assert.equal(ids.length, 1, 'Exactly one owned PostgreSQL container');
  const entry = JSON.parse(docker('inspect', ids[0]))[0];
  assert.equal(entry.Config.Labels[labels.project], project.id); assert.equal(entry.Config.Labels[labels.resource], 'postgres'); assert.equal(entry.Image, image.Id);
  assert.equal(entry.Config.Labels[labels.instance], ownedInstance);
  return entry.Id;
}
function sql(database, command) {
  assert.match(database, /^[a-z][a-z0-9_]{0,62}$/);
  return docker('exec', container(), 'psql', '-U', 'luci', '-d', database, '-v', 'ON_ERROR_STOP=1', '-Atqc', command);
}
async function mutation(action, suffix, timeout = 180_000, readBody = true) {
  // Capture ordinary JSON immediately. Blob-transfer receipts are subsequently
  // joined to their saved GET record and visible selection: Chromium may evict
  // a consumed upload response body from its inspector cache.
  const pending = page.waitForResponse(response => response.request().method() === 'POST' && new URL(response.url()).pathname === suffix, { timeout }).then(response => {
    assert(response.ok(), suffix + ': ' + response.status());
    return readBody ? response.json() : { status: response.status() };
  });
  await action(); return pending;
}
async function screenshot(name) { await page.screenshot({ path: join(evidence, name + '.png'), fullPage: true }); }
async function loginAndOpen() {
  await page.goto(origin);
  await page.getByLabel('Username', { exact: true }).fill('database-owner'); await page.getByLabel('Password', { exact: true }).fill(password);
  const signed = await mutation(() => page.getByRole('button', { name: 'Sign in', exact: true }).click(), '/v1/auth/login'); secrets.push(signed.token);
  const firstVisit = page.getByRole('button', { name: 'Continue to projects', exact: true }), tree = page.getByRole('treeitem', { name: /database-recovery$/ });
  await until('Project entry', async () => await firstVisit.isVisible() || await tree.isVisible()); if (await firstVisit.isVisible()) await firstVisit.click();
  await tree.click(); await page.getByRole('button', { name: 'Deploy', exact: true }).click();
  await page.getByRole('tab', { name: 'Application platform', exact: true }).click(); await page.getByRole('navigation', { name: 'Platform sections', exact: true }).getByRole('button', { name: 'Data', exact: true }).click();
  await page.getByText('Standalone PostgreSQL database', { exact: true }).click();
  await page.getByRole('region', { name: 'Standalone PostgreSQL', exact: true }).waitFor();
}
const panel = () => page.getByRole('region', { name: 'Standalone PostgreSQL', exact: true });
async function settle(operation) {
  const current = await until('Verified database recovery', async () => { const value = await observe(route); return !value.busy && value.project.databaseOperation?.id === operation.id && value.project.databaseOperation.status !== 'running' ? value.project : false; }, 180_000);
  assert.equal(current.databaseOperation.status, 'succeeded', current.databaseOperation.message); assert.equal(current.databaseOperation.stage, 'complete');
  await panel().getByText(current.databaseOperation.message, { exact: true }).first().waitFor({ timeout: 15_000 });
  return current;
}
function cleanupOwned() {
  if (!project || !ownedInstance) return;
  assert.equal(ownedProcesses().length, 0, 'Do not delete database resources while any owned gateway or worker is still running');
  const filters = ['--filter', `label=${labels.project}=${project.id}`, '--filter', `label=${labels.instance}=${ownedInstance}`];
  for (const id of docker('ps', '--all', ...filters, '--format', '{{.ID}}').split(/\s+/).filter(Boolean)) {
    const row = JSON.parse(docker('inspect', id))[0]; assert.equal(row.Config.Labels[labels.project], project.id); assert.equal(row.Config.Labels[labels.instance], ownedInstance);
    docker('rm', '--force', id); result.cleanup.push({ kind: 'container', id });
  }
  for (const kind of ['volume', 'network']) for (const name of docker(kind, 'ls', ...filters, '--format', '{{.Name}}').split(/\s+/).filter(Boolean)) {
    const row = JSON.parse(docker(kind, 'inspect', name))[0]; assert.equal(row.Labels[labels.project], project.id); assert.equal(row.Labels[labels.instance], ownedInstance);
    docker(kind, 'rm', name); result.cleanup.push({ kind, name });
  }
}
try {
  await start();
  const bootstrapToken = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim();
  const owner = await request('POST', '/v1/auth/bootstrap', { bootstrapToken, username: 'database-owner', displayName: 'Database recovery fixture', password }); token = owner.token; secrets.push(token);
  await observe('/v1/setup/status'); project = await request('POST', '/v1/product/projects', { name: 'database-recovery' }); route = '/v1/platform/projects/' + project.id; result.projectId = project.id;
  // Bind ownership from the freshly created private worker, never adopt an
  // instance label from a shared daemon's project-ID matches.
  assert.match(owner.user.id, /^user-[A-Za-z0-9_-]{22}$/);
  const workerRoot = join(fixture, 'state/control-plane/users', owner.user.id, 'runtime');
  const localProject = join(fixture, 'projects/database-recovery');
  const localOwner = existsSync(localProject) && realpathSync(project.path) === realpathSync(localProject);
  if (!localOwner) assert.equal(realpathSync(project.path), realpathSync(join(workerRoot, 'projects/database-recovery')));
  const workerState = localOwner ? join(fixture, 'state') : join(workerRoot, 'state');
  const workerInstance = readFileSync(join(workerState, 'instance/instance-id'), 'utf8').trim();
  result.runtimeScope = localOwner ? 'isolated local owner runtime' : 'isolated managed personal worker';
  assert.match(workerInstance, /^[a-f0-9-]{36}$/);
  ownedInstance = sha(join(workerState, 'sessions') + workerInstance).slice(0, 16);
  result.platformInstance = ownedInstance;
  const limits = await observe('/v1/platform/status'); assert(limits.dockerReady); result.serviceLimits = limits.limits; assert(info.MemTotal >= limits.limits.memoryMiB * 1024 ** 2);
  browser = await chromium.launch({ headless: true, ...(option('--chromium') ? { executablePath: option('--chromium') } : {}) }); result.browserVersion = browser.version();
  page = await browser.newPage({ viewport: { width: 1440, height: 1100 }, acceptDownloads: true }); page.setDefaultTimeout(30_000);
  function recordPage() {
    page.on('pageerror', error => browserErrors.push(scrub(error.message)));
    page.on('response', response => { const path = new URL(response.url()).pathname; if (new URL(response.url()).origin === origin && ['POST', 'PUT', 'PATCH', 'DELETE'].includes(response.request().method())) result.browserMutations.push({ path, method: response.request().method(), status: response.status() }); });
  }
  recordPage();
  await loginAndOpen(); stage = 'start-and-backup';
  const started = await mutation(() => panel().getByRole('button', { name: 'Start PostgreSQL', exact: true }).click(), route + '/database'); assert.equal(started.status, 'ready');
  container(); sql('app', "CREATE TABLE recovery_probe (marker text PRIMARY KEY); INSERT INTO recovery_probe VALUES ('saved-before-backup');");
  const backup = await mutation(() => panel().getByRole('button', { name: 'Create database backup', exact: true }).click(), route + '/database');
  const downloadEvent = page.waitForEvent('download'); await panel().getByRole('button', { name: /^Download database backup/ }).click();
  const download = await downloadEvent, backupFile = join(evidence, 'fixture.dump'); await download.saveAs(backupFile); const bytes = readFileSync(backupFile);
  assert.equal(bytes.subarray(0, 5).toString(), 'PGDMP'); assert.equal(bytes.length, backup.bytes); assert.equal(sha(bytes), backup.sha256);
  await screenshot('01-rendered-backup-and-exact-download');
  check('Rendered PostgreSQL start and backup produced exact downloadable private bytes', { backupId: backup.id, bytes: bytes.length, sha256: backup.sha256 });
  stage = 'import'; await panel().getByLabel('PostgreSQL dump file', { exact: true }).setInputFiles(backupFile);
  assert.equal(await panel().getByRole('button', { name: 'Import PostgreSQL backup', exact: true }).isEnabled(), false);
  await panel().getByRole('checkbox', { name: 'I trust this dump and the SQL it can execute during restore.', exact: true }).check();
  await mutation(() => panel().getByRole('button', { name: 'Import PostgreSQL backup', exact: true }).click(), route + '/backups/import', 180_000, false);
  await until('Import receipt selected in the UI', async () => await panel().getByRole('combobox', { name: 'PostgreSQL backup to restore', exact: true }).inputValue());
  const importedID = await panel().getByRole('combobox', { name: 'PostgreSQL backup to restore', exact: true }).inputValue();
  const imported = (await observe(route)).project.backups.find(row => row.id === importedID);
  assert(imported); assert.equal(imported.sha256, backup.sha256); assert.equal(imported.bytes, backup.bytes);
  assert.equal(await panel().getByLabel('PostgreSQL dump file', { exact: true }).inputValue(), '');
  assert.equal(sql('app', 'SELECT marker FROM recovery_probe ORDER BY marker'), 'saved-before-backup');
  check('Explicit trusted UI import retains verified bytes without changing the database', { importedId: imported.id });
  stage = 'restore'; sql('app', "INSERT INTO recovery_probe VALUES ('changed-after-backup');");
  await panel().getByRole('combobox', { name: 'PostgreSQL backup to restore', exact: true }).selectOption(imported.id);
  await panel().getByRole('checkbox', { name: 'Switch to this backup after verification, keeping the previous database.', exact: true }).check();
  assert.equal(await panel().getByRole('button', { name: 'Restore PostgreSQL backup', exact: true }).isEnabled(), false);
  const stopped = await mutation(() => panel().getByRole('button', { name: 'Stop PostgreSQL', exact: true }).click(), route + '/database'); assert.equal(stopped.status, 'stopped');
  const accepted = await mutation(() => panel().getByRole('button', { name: 'Restore PostgreSQL backup', exact: true }).click(), route + '/database'); assert.equal(accepted.status, 'running');
  const restored = await settle(accepted); assert.notEqual(restored.database.database, 'app');
  assert.equal(sql(restored.database.database, 'SELECT marker FROM recovery_probe ORDER BY marker'), 'saved-before-backup');
  assert.equal(sql('app', 'SELECT marker FROM recovery_probe ORDER BY marker'), 'changed-after-backup\nsaved-before-backup');
  await screenshot('02-verified-restore-retains-original');
  check('UI restore after explicit stop promotes the real backup candidate while retaining original changed records', { restoreId: accepted.id, candidate: restored.database.database, previous: 'app' });
  stage = 'rollback'; await mutation(() => panel().getByRole('button', { name: 'Stop PostgreSQL', exact: true }).click(), route + '/database');
  await panel().getByText('Recover the previous database', { exact: true }).click();
  await panel().getByRole('checkbox', { name: 'Switch to the verified previous database, keeping the current database.', exact: true }).check();
  const rolled = await mutation(() => panel().getByRole('button', { name: 'Use previous database', exact: true }).click(), route + '/database');
  const recovered = await settle(rolled); assert.equal(recovered.database.database, 'app');
  assert.equal(sql('app', 'SELECT marker FROM recovery_probe ORDER BY marker'), 'changed-after-backup\nsaved-before-backup');
  assert.equal(sql(restored.database.database, 'SELECT marker FROM recovery_probe ORDER BY marker'), 'saved-before-backup');
  check('Explicit UI previous-database recovery verifies the retained original and preserves the restored generation', { rollbackId: rolled.id });
  stage = 'restart'; await stopService(); await start();
  await browser.close(); browser = await chromium.launch({ headless: true, ...(option('--chromium') ? { executablePath: option('--chromium') } : {}) });
  page = await browser.newPage({ viewport: { width: 1440, height: 1100 } }); page.setDefaultTimeout(30_000); recordPage(); await loginAndOpen();
  const retained = (await observe(route)).project; assert.equal(retained.database.database, 'app'); assert.deepEqual(retained.databaseRestoreHistory.map(row => row.id), [rolled.id, accepted.id]);
  assert(retained.backups.some(row => row.id === imported.id && row.sha256 === imported.sha256));
  assert.equal(sql('app', 'SELECT marker FROM recovery_probe ORDER BY marker'), 'changed-after-backup\nsaved-before-backup');
  await panel().getByText('Restore history', { exact: true }).click(); await panel().getByRole('list', { name: 'PostgreSQL restore history', exact: true }).waitFor();
  await screenshot('03-installed-service-restart-recovery-history');
  check('Real installed service restart and a fresh browser sign-in retain database records, backup and ordered recovery history', { history: retained.databaseRestoreHistory.map(row => row.id) });
  await mutation(() => panel().getByRole('button', { name: 'Stop PostgreSQL', exact: true }).click(), route + '/database');
  assert.equal(browserErrors.length, 0, browserErrors.join('\n')); result.passed = true;
} catch (error) { result.error = scrub(error.stack || error); result.stage = stage; process.exitCode = 1; try { await screenshot('failure-' + stage); } catch {} }
finally {
  try { await browser?.close(); } catch {}
  try { await stopService(); } catch (error) { result.stopError = scrub(error); process.exitCode = 1; try { await forceStopOwned(); } catch (cause) { result.workerStopError = scrub(cause); } }
  try { assert.equal(ownedProcesses().length, 0); cleanupOwned(); result.ownedResourcesRemoved = true; } catch (error) { result.cleanupError = scrub(error); process.exitCode = 1; }
  log.end(); result.finishedAt = new Date().toISOString(); if (process.exitCode) result.passed = false;
  writeFileSync(report, JSON.stringify(result, null, 2) + '\n', { mode: 0o600 }); console.log(JSON.stringify({ passed: result.passed, checks: result.checks.length, stage: result.stage, error: result.error, report, ownedResourcesRemoved: result.ownedResourcesRemoved }));
}

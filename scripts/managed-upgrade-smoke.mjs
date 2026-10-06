#!/usr/bin/env node
// Bounded Linux stopped-state upgrade rehearsal. Owns only a fresh disposable
// fixture; no Docker, real provider, managed-team migration or cloud operation.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { createServer } from 'node:net';
import { chmodSync, existsSync, mkdirSync, mkdtempSync, readdirSync, readFileSync, readlinkSync, renameSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve, basename } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const args = process.argv.slice(2);
const option = (name, fallback) => { const i = args.indexOf(name); assert(i < 0 || args[i + 1], `Supply ${name}`); return i < 0 ? fallback : args[i + 1]; };
if (args.includes('--help')) {
  console.log('node scripts/managed-upgrade-smoke.mjs --old-package PATH (--new-package PATH | --new-binary PATH --new-version VERSION) [--output PATH] [--keep-fixture]\nLinux only. Creates fresh private fixture accounts and stopped backups; never accepts an existing state directory. Packages require their adjacent release-manifest.json and matching SHA-256. No containers or external model/cloud calls.');
  process.exit(0);
}
assert.equal(process.platform, 'linux', 'This stopped-process/backup rehearsal currently supports Linux');
const oldPackage = resolve(option('--old-package', 'artifacts/releases/0.4.18/luci-linux-amd64.tar.gz'));
const newPackage = option('--new-package', ''), newBinary = option('--new-binary', '');
assert(Boolean(newPackage) !== Boolean(newBinary), 'Supply exactly one of --new-package and --new-binary');
const output = resolve(option('--output', `artifacts/uat/managed-upgrade-${Date.now()}/result.json`));
mkdirSync(dirname(output), { recursive: true, mode: 0o700 });
const fixture = mkdtempSync(join(tmpdir(), 'luci-upgrade-'));
const state = join(fixture, 'state'), home = join(fixture, 'home');
for (const path of [state, home, join(fixture, 'projects')]) mkdirSync(path, { mode: 0o700 });
const sha = data => createHash('sha256').update(data).digest('hex');
const secrets = new Set();
const secret = () => { const value = randomBytes(24).toString('base64url'); secrets.add(value); return value; };
const ownerPassword = secret(), memberPassword = secret(), providerKey = secret(), lifecycle = secret();
const redact = error => { let value = String(error?.message || error); for (const item of secrets) value = value.replaceAll(item, '[redacted]'); return value; };
const receipt = { passed: false, startedAt: new Date().toISOString(), platform: process.platform, architecture: process.arch,
  runnerSHA256: sha(readFileSync(fileURLToPath(import.meta.url))), checks: [], externalModelCalls: 0, cloudResourcesCreated: 0, containersCreated: 0,
  scope: 'Actual historical and candidate Linux services, private personal accounts/projects, stopped pre-upgrade backup and restore. Version 0.4.18 predates teams; no team migration, installer, browser, real provider authentication, databases or running-workload upgrade claim.' };
function check(name, details = {}) { receipt.checks.push({ name, passed: true, ...details }); console.log(`PASS ${name}`); }
function packageBinary(archive, target) {
  archive = resolve(archive);
  const manifest = JSON.parse(readFileSync(join(dirname(archive), 'release-manifest.json')));
  assert.equal(manifest.sourceDirty, false, 'Use a package built from clean source');
  const row = manifest.artifacts.find(item => item.name === basename(archive));
  const data = readFileSync(archive);
  assert(row && row.bytes === data.length && row.sha256 === sha(data), 'Package does not match its release manifest');
  mkdirSync(target, { mode: 0o700 });
  // Packages are verified first; still reject links, special entries and paths
  // which could escape this newly owned extraction directory.
  execFileSync('python3', ['-c', `import pathlib,sys,tarfile
with tarfile.open(sys.argv[1], 'r:gz') as source:
 for row in source.getmembers():
  path=pathlib.PurePosixPath(row.name)
  assert not path.is_absolute() and '..' not in path.parts and (row.isfile() or row.isdir()), 'unsafe package entry'
 source.extractall(sys.argv[2], filter='data')`, archive, target], { timeout: 60000, stdio: 'pipe' });
  const binary = join(target, 'luci');
  assert(existsSync(binary));
  return { binary, version: manifest.version, package: archive, packageSHA256: row.sha256, sourceCommit: manifest.sourceCommit, sourceDirty: false, binarySHA256: sha(readFileSync(binary)) };
}
// Reuse the production recovery tool's no-link Tree, private ownership checks,
// stable file inventory and process leases. Its team-bound CLI intentionally
// cannot accept this pre-team state. This fixture-only copy requires both a
// newly owned root and stopped services; it is not a general backup command.
const copyState = `import contextlib,hashlib,importlib.util,json,os,stat,sys
module=importlib.util.spec_from_file_location('recovery',sys.argv[1]); m=importlib.util.module_from_spec(module); module.loader.exec_module(m)
source,destination,fixture=sys.argv[2:]
assert source.startswith(fixture+'/') and destination.startswith(fixture+'/') and not os.path.exists(destination)
with contextlib.closing(m.Tree(source)) as tree:
 entries=tree.inventory(); rows=[]
 os.mkdir(destination,0o700)
 with contextlib.closing(m.Tree(destination)) as target:
  for path,info in sorted(entries.items(),key=lambda x:(len(x[0].split('/')),x[0])):
   if stat.S_ISDIR(info.st_mode):
    target.mkdir(path); rows.append({'path':path,'kind':'directory'}); continue
   digest=hashlib.sha256()
   with os.fdopen(tree.open(path),'rb') as src,os.fdopen(target.open(path,os.O_WRONLY|os.O_CREAT|os.O_EXCL),'wb') as dst:
    m.require(m.same(info,os.fstat(src.fileno())),'source changed before copy')
    while data:=src.read(m.BLOCK): digest.update(data); dst.write(data)
    dst.flush(); os.fchmod(dst.fileno(),stat.S_IMODE(info.st_mode)&0o700); os.fsync(dst.fileno())
    m.require(m.same(info,os.fstat(src.fileno())),'source changed during copy')
   with os.fdopen(target.open(path),'rb') as copied: m.require(m.digest_stream(copied)==digest.hexdigest(),'backup bytes differ')
   rows.append({'path':path,'kind':'file','bytes':info.st_size,'mode':stat.S_IMODE(info.st_mode)&0o700,'sha256':digest.hexdigest()})
  after=tree.inventory(); m.require(set(after)==set(entries) and all(m.same(entries[p],after[p]) for p in entries),'source changed after copy')
  os.fsync(target.fd)
 print(json.dumps({'entries':len(rows),'bytes':sum(x.get('bytes',0) for x in rows),'inventorySHA256':hashlib.sha256(m.json_bytes(rows)).hexdigest()}))`;
function stoppedCopy(source, destination) {
  assert.equal(service, null, 'Stop the owned service and workers before copying private state');
  return JSON.parse(execFileSync('python3', ['-c', copyState, join(repo, 'scripts/managed-host-backup.py'), source, destination, fixture], { encoding: 'utf8', timeout: 60000, stdio: ['ignore', 'pipe', 'pipe'] }));
}
const reservePort = async () => { const server = createServer(); await new Promise((done, fail) => server.once('error', fail).listen(0, '127.0.0.1', done)); const port = server.address().port; await new Promise(done => server.close(done)); return port; };
const port = await reservePort(), previewPort = await reservePort(), base = `http://127.0.0.1:${port}`;
const inherited = Object.fromEntries(Object.entries(process.env).filter(([name]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ|TMPDIR)$/.test(name)));
const environment = { ...inherited, HOME: home, USERPROFILE: home, XDG_CONFIG_HOME: join(home, '.config'), XDG_DATA_HOME: join(home, '.local/share'), XDG_STATE_HOME: join(home, '.local/state'),
  GIT_CONFIG_GLOBAL: join(home, '.gitconfig'), GIT_CONFIG_NOSYSTEM: '1', AWS_EC2_METADATA_DISABLED: 'true',
  LUCI_ADDR: `127.0.0.1:${port}`, LUCI_BROWSER_PREVIEW_ADDR: `127.0.0.1:${previewPort}`, LUCI_BROWSER_PREVIEW_PORT: String(previewPort), LUCI_STATE_DIR: state, LUCI_PROJECTS_DIR: join(fixture, 'projects'),
  LUCI_DOCKER_HOST: `unix://${join(fixture, 'no-docker.sock')}`, LUCI_BACKEND: 'docker', LUCI_BACKING: 'docker', LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_LOG_LEVEL: 'error' };
let service = null, ownerToken = '', memberToken = '', activeVersion = '';
function processIdentity(pid) {
  try {
    const text = readFileSync(`/proc/${pid}/stat`, 'utf8');
    const fields = text.slice(text.lastIndexOf(')') + 2).trim().split(/\s+/);
    return { pid: Number(pid), start: fields[19], group: Number(fields[2]), status: fields[0] };
  } catch (error) { if (error.code === 'ENOENT' || error.code === 'ESRCH') return null; throw error; }
}
function stillOwned(identity) {
  const now = processIdentity(identity.pid);
  return now && now.start === identity.start && now.status !== 'Z';
}
function ownedProcesses(binary) {
  const found = [];
  for (const pid of readdirSync('/proc').filter(value => /^\d+$/.test(value))) {
    const identity = processIdentity(pid); if (!identity || identity.status === 'Z') continue;
    try {
      // Exact executable plus the newly owned fixture state binds a process to
      // this run. Never inherit authority from a PID or broad executable name.
      if (readlinkSync(`/proc/${pid}/exe`) !== binary) continue;
      const values = Object.fromEntries(readFileSync(`/proc/${pid}/environ`, 'utf8').split('\0').filter(Boolean).map(value => { const i = value.indexOf('='); return [value.slice(0, i), value.slice(i + 1)]; }));
      const ownState = values.LUCI_STATE_DIR === state || values.LUCI_STATE_DIR?.startsWith(state + '/control-plane/');
      if (ownState && processIdentity(pid)?.start === identity.start) found.push(identity);
    } catch (error) { if (!['ENOENT', 'ESRCH', 'EACCES', 'EPERM'].includes(error.code)) throw error; }
  }
  return found;
}
async function api(method, route, body, { token = memberToken, status = 200 } = {}) {
  const response = await fetch(base + route, { method, headers: { Origin: base, 'Content-Type': 'application/json', ...(token ? { Authorization: `Bearer ${token}` } : {}) }, body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(45000) });
  assert.equal(response.status, status, `${activeVersion}: ${method} ${route} HTTP status`);
  const value = await response.json(); if (value.token) secrets.add(value.token); return value;
}
async function start(release) {
  assert.equal(service, null);
  activeVersion = release.version;
  const child = spawn(release.binary, [], { cwd: fixture, env: environment, detached: true, stdio: ['ignore', 'pipe', 'pipe'] });
  const record = { child, binary: release.binary, tracked: [], error: null, log: '' }; service = record;
  child.once('error', error => { record.error = error; });
  for (const stream of [child.stdout, child.stderr]) stream.on('data', bytes => { record.log = (record.log + bytes.toString()).slice(-65536); });
  const deadline = Date.now() + 45000;
  while (Date.now() < deadline) {
    if (record.error) throw record.error;
    assert.equal(child.exitCode, null, `Service exited during ${release.version} startup`);
    try { const status = await api('GET', '/v1/control-plane/status', undefined, { token: '' }); assert.equal(status.version, release.version); return status; } catch (error) { if (error.actual && error.expected) throw error; }
    await delay(100);
  }
  throw Error(`Service did not become ready: ${release.version}`);
}
async function stop() {
  if (!service) return;
  const { child, binary } = service;
  assert(child.pid, 'Service was not spawned');
  service.tracked = ownedProcesses(binary);
  if (child.exitCode === null) assert(service.tracked.some(row => row.pid === child.pid), 'Cannot verify the owned gateway identity');
  if (child.exitCode === null) {
    const response = await fetch(base + '/v1/control-plane/shutdown', { method: 'POST', headers: { 'X-Luci-Lifecycle-Token': lifecycle }, signal: AbortSignal.timeout(5000) });
    assert.equal(response.status, 200, 'Owned service refused lifecycle shutdown');
    const deadline = Date.now() + 20000;
    while (child.exitCode === null && Date.now() < deadline) await delay(100);
  }
  assert.equal(child.exitCode, 0, 'Service did not stop cleanly');
  // Each worker has its OWN process group. Track PID+start tick before shutdown
  // and also discover any fixture worker allocated during that short interval.
  // The historical release has no state leases, so this cannot rely on leases.
  let running = service.tracked.filter(stillOwned);
  const deadline = Date.now() + 5000;
  while ((running.length || ownedProcesses(binary).length) && Date.now() < deadline) {
    await delay(100); running = service.tracked.filter(stillOwned);
  }
  assert.equal(running.length + ownedProcesses(binary).length, 0, 'An owned worker survived shutdown');
  (receipt.shutdowns ||= []).push({ version: activeVersion, trackedGatewayAndWorkers: service.tracked.length, allStopped: true });
  service = null;
}
async function login() {
  ownerToken = (await api('POST', '/v1/auth/login', { username: 'upgrade-owner', password: ownerPassword }, { token: '' })).token;
  memberToken = (await api('POST', '/v1/auth/login', { username: 'upgrade-member', password: memberPassword }, { token: '' })).token;
}
const briefWrite = (brief, purpose) => ({ revision: brief.revision, purpose, audience: brief.audience, constraints: brief.constraints, criteria: brief.criteria });
try {
  const old = packageBinary(oldPackage, join(fixture, 'old-package'));
  assert.equal(old.version, '0.4.18', 'This rehearsal fixture targets the historical 0.4.18 contracts');
  const candidate = newPackage ? packageBinary(newPackage, join(fixture, 'new-package')) : { binary: resolve(newBinary), version: option('--new-version', '0.5.0'), binarySHA256: sha(readFileSync(resolve(newBinary))), sourceCommit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repo, encoding: 'utf8' }).trim(), sourceDirty: execFileSync('git', ['status', '--porcelain'], { cwd: repo, encoding: 'utf8' }).trim() !== '', provenance: 'Compiled source candidate; hash identifies binary, Git metadata is observed runner checkout only' };
  assert.equal(candidate.version, '0.5.0', 'This rehearsal targets 0.5.0');
  receipt.old = old; receipt.candidate = candidate;
  assert.equal((await start(old)).bootstrapRequired, true);
  const bootstrapToken = readFileSync(join(state, 'control-plane/bootstrap-token'), 'utf8').trim(); secrets.add(bootstrapToken);
  const owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken, username: 'upgrade-owner', displayName: 'Upgrade owner', password: ownerPassword }, { token: '', status: 201 }); ownerToken = owner.token;
  const member = await api('POST', '/v1/admin/users', { username: 'upgrade-member', displayName: 'Upgrade member', password: memberPassword, role: 'member' }, { token: ownerToken, status: 201 });
  await login();
  await api('POST', '/v1/setup/git/identity', { name: 'Upgrade member', email: 'upgrade@example.invalid' });
  const project = await api('POST', '/v1/product/projects', { name: 'upgrade-context' }, { status: 201 });
  const route = `/v1/product/projects/${project.id}`;
  const runtime = join(state, 'control-plane/users', member.id, 'runtime');
  assert.equal(project.path, join(runtime, 'projects/upgrade-context'));
  let brief = await api('POST', route + '/brief', { revision: 0, purpose: 'Preserve reviewed collaboration context across upgrades', audience: 'Existing self-hosted users', constraints: 'No model calls or deployment', criteria: ['Accounts, settings and historical revisions survive'] });
  brief = await api('POST', route + '/brief', briefWrite(brief, 'Second preserved revision of our project purpose'));
  await api('POST', route + '/plan', { expectedRevision: 0, tasks: [{ id: 'preserve-context', title: 'Verify the upgrade preserves context', status: 'todo' }] });
  await api('POST', route + '/design', { revision: 0, frames: [{ id: 'overview', name: 'Upgrade overview', width: 600, height: 400, background: '#ffffff', elements: [] }] });
  let spec = await api('GET', route + '/specification');
  spec = await api('POST', route + '/specification', { expectedRevision: spec.revision, expectedBriefRevision: spec.briefRevision, specification: { ...spec, requirements: [{ id: 'retention', title: 'Preserve private account context', priority: 'must', acceptance: 'Upgrade and explicit rollback retain the saved project history' }] } });
  spec = await api('POST', route + '/specification/approve', { revision: spec.revision, briefRevision: spec.briefRevision });
  let handoff = await api('GET', route + '/design-plan');
  handoff = await api('POST', route + '/design-plan', { expectedRevision: handoff.revision, expectedBriefRevision: spec.briefRevision, expectedSpecificationRevision: spec.revision, plan: { ...handoff, screens: [{ id: 'overview', name: 'Context overview', route: '/', purpose: 'Review saved context', states: ['Loading', 'Ready', 'Recovery'], widths: [390, 1440], requirementIds: ['retention'], accessibility: 'Keyboard navigation and labeled controls' }] } });
  handoff = await api('POST', route + '/design-plan/approve', { revision: handoff.revision });
  await api('POST', '/v1/setup/agent-connections', { action: 'save', expectedRevision: 0, name: 'Offline upgrade fixture', kind: 'litellm', enabled: true, endpoints: { responses: 'https://provider.example.invalid/v1', anthropicMessages: 'https://provider.example.invalid', chatCompletions: 'https://provider.example.invalid/v1' }, models: { codex: 'retained-model', claudeCode: 'retained-claude', opencode: 'retained-model' }, apiKey: providerKey });
  const committed = await api('POST', route + '/git/commit', { message: 'Preserve reviewed legacy context before upgrade' });
  const snapshot = async () => {
    const values = {};
    for (const key of ['brief', 'plan', 'design', 'specification', 'design-plan']) values[key] = await api('GET', route + '/' + key);
    values.specificationHistory = (await api('GET', route + '/specification/history')).revisions;
    values.providers = await api('GET', '/v1/setup/agent-connections');
    values.gitLog = (await api('GET', route + '/git')).log;
    return values;
  };
  const expected = await snapshot();
  brief = expected.brief; // Saving the specification also updates its brief.
  assert(expected.specificationHistory.length > 0);
  const canonical = join(project.path, '.luci/state.json');
  const legacyBytes = readFileSync(canonical); assert.equal(JSON.parse(legacyBytes).version, 1);
  const privateProvider = join(runtime, 'state/sessions/agent-connections.json');
  const providerHash = sha(readFileSync(privateProvider));
  const beforeAudit = (await api('GET', '/v1/admin/audit', undefined, { token: ownerToken })).events;
  check('Historical packaged 0.4.18 creates real accounts, private provider configuration, approved context, revisions and Git history', { projectId: project.id, commit: committed.commit, legacyContextSHA256: sha(legacyBytes), specificationRevisions: expected.specificationHistory.length });
  await stop();
  const backup = stoppedCopy(state, join(fixture, 'pre-upgrade-backup'));
  receipt.backup = { ...backup, includesPrivateSecrets: true, excludes: ['process lease files', 'fixture home and logs outside state', 'Docker resources and external services'] };
  check('Stopped gateway and all supervised workers; private pre-upgrade copy verified every file and inventory', backup);

  assert.equal((await start(candidate)).bootstrapRequired, false);
  // Existing sessions and new password logins must both survive the upgrade.
  assert.equal((await api('GET', '/v1/auth/me')).user.id, member.id);
  assert.equal((await api('GET', '/v1/auth/me', undefined, { token: ownerToken })).user.id, owner.user.id);
  await login();
  assert.deepEqual(await snapshot(), expected);
  assert.equal(sha(readFileSync(privateProvider)), providerHash);
  assert.deepEqual(readFileSync(canonical), legacyBytes, 'Read-only upgrade rewrote legacy context');
  await api('GET', route + '/brief', undefined, { token: ownerToken, status: 404 });
  const upgradeAudit = (await api('GET', '/v1/admin/audit', undefined, { token: ownerToken })).events;
  assert(beforeAudit.every(row => upgradeAudit.some(current => current.id === row.id)), 'Upgrade discarded audit history');
  check('0.5.0 preserves existing sessions, password login, account isolation, private provider bytes, approved documents and complete legacy history without a read migration');
  const advanced = await api('POST', route + '/brief', briefWrite(brief, 'Explicit candidate-era revision after verified upgrade'));
  assert.equal(advanced.revision, brief.revision + 1);
  const migrated = JSON.parse(readFileSync(canonical)); assert.equal(migrated.version, 2); assert(migrated.contextId);
  assert.equal(sha(readFileSync(privateProvider)), providerHash);
  await stop();
  await start(candidate); await login();
  assert.deepEqual(await api('GET', route + '/brief'), advanced);
  assert.deepEqual((await api('GET', route + '/specification/history')).revisions.slice(0, expected.specificationHistory.length), expected.specificationHistory);
  check('An explicit 0.5.0 edit migrates to format 2 and survives service restart with earlier history and provider configuration intact');
  await stop();
  renameSync(state, join(fixture, 'upgraded-state-retained'));
  const restored = stoppedCopy(join(fixture, 'pre-upgrade-backup'), state);
  assert.deepEqual(restored, backup, 'Rollback restored different private bytes or permissions');
  assert.deepEqual(readFileSync(canonical), legacyBytes);
  assert.equal(JSON.parse(readFileSync(canonical)).version, 1);
  check('Rollback first restores the exact pre-upgrade private snapshot at the original path; upgraded format-2 state is retained separately', restored);
  assert.equal((await start(old)).bootstrapRequired, false);
  await login();
  assert.deepEqual(await snapshot(), expected);
  assert.equal(sha(readFileSync(privateProvider)), providerHash);
  const rollbackAudit = (await api('GET', '/v1/admin/audit', undefined, { token: ownerToken })).events;
  assert(beforeAudit.every(row => rollbackAudit.some(current => current.id === row.id)));
  // Prove the restored old release can write safely, without ever exposing the
  // candidate's format-2 state to its writer.
  const rolled = await api('POST', route + '/brief', briefWrite(brief, 'Old release writes only restored format-1 state'));
  assert.equal(rolled.revision, brief.revision + 1);
  assert.equal(JSON.parse(readFileSync(canonical)).version, 1);
  check('Historical package signs into restored accounts, reads exact provider/context/history and writes only restored format-1 state');
  await stop(); receipt.passed = true;
} catch (error) {
  receipt.failure = redact(error); process.exitCode = 1;
} finally {
  if (service) {
    try { await stop(); } catch (error) {
      receipt.cleanupFailure = redact(error); receipt.passed = false; process.exitCode = 1;
      const owned = [...service.tracked, ...ownedProcesses(service.binary)];
      for (const row of owned) if (stillOwned(row)) {
        try { process.kill(row.group === row.pid ? -row.pid : row.pid, 'SIGKILL'); } catch (failure) { if (failure.code !== 'ESRCH') receipt.cleanupKillFailure = failure.code; }
      }
      const deadline = Date.now() + 5000;
      while (owned.some(stillOwned) && Date.now() < deadline) await delay(100);
      if (owned.some(stillOwned) || ownedProcesses(service.binary).length) receipt.cleanupWritersRemain = true;
    }
  }
  if (args.includes('--keep-fixture') || !receipt.passed) {
    receipt.privateFixture = fixture;
    if (service?.log) writeFileSync(join(fixture, 'service-diagnostic.log'), service.log, { mode: 0o600 });
  } else rmSync(fixture, { recursive: true });
  receipt.completedAt = new Date().toISOString();
  writeFileSync(output, JSON.stringify(receipt, null, 2) + '\n', { mode: 0o600 }); chmodSync(output, 0o600);
  console.log(JSON.stringify(receipt, null, 2));
}

// Proves the private operator archive with real account/team/context APIs.
// node scripts/managed-host-recovery-smoke.mjs <compiled-binary> <fresh-evidence-dir>
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { randomUUID, createHash } from 'node:crypto';
import { once } from 'node:events';
import { createWriteStream, mkdirSync, mkdtempSync, readFileSync, readdirSync, renameSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:net';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
assert.equal(process.platform, 'linux'); assert.equal(process.argv.length, 4);
const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const binary = resolve(process.argv[2]), evidence = resolve(process.argv[3]);
mkdirSync(evidence, { mode: 0o700 });
const fixture = mkdtempSync(join(tmpdir(), 'luci-host-recovery-'));
const state = join(fixture, 'state'), projects = join(fixture, 'projects'), archive = join(fixture, 'private-host.zip');
mkdirSync(state, { mode: 0o700 }); mkdirSync(projects, { mode: 0o700 });
const listener = createServer(); await new Promise(done => listener.listen(0, '127.0.0.1', done));
const address = `127.0.0.1:${listener.address().port}`; await new Promise(done => listener.close(done));
const origin = `http://${address}`, password = randomUUID() + randomUUID(), memberPassword = randomUUID() + randomUUID(), lifecycle = randomUUID() + randomUUID();
const environment = { ...Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('LUCI_') && !key.startsWith('DOCKER_'))), LUCI_ADDR: address, LUCI_BACKEND: 'docker', LUCI_STATE_DIR: state, LUCI_PROJECTS_DIR: projects, LUCI_RUNTIME_DIR: repo, LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_DOCKER_HOST: 'unix:///var/run/docker.sock', ANTHROPIC_API_KEY: '', OPENAI_API_KEY: '', GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: join(fixture, 'empty-git-config') };
const log = createWriteStream(join(evidence, 'control-plane.log'), { mode: 0o600 });
const checks = [], result = { passed: false, binary, binarySHA256: createHash('sha256').update(readFileSync(binary)).digest('hex'), fixture, paidInference: false, cloudResourcesCreated: false, checks };
let service, token, teamId, version, writer, orphan;
async function request(method, path, body, expected = 200) {
 const response = await fetch(origin + path, { method, redirect: 'error', signal: AbortSignal.timeout(60000), headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(body === undefined ? {} : { 'Content-Type': 'application/json' }), ...(path.endsWith('/shutdown') ? { 'X-Luci-Lifecycle-Token': lifecycle } : {}) }, body: body === undefined ? undefined : JSON.stringify(body) });
 const value = await response.json(); assert.equal(response.status, expected, `${method} ${path}: ${response.status} ${response.status === expected ? '' : value.message || value.error || 'Unexpected response'}`); return value;
}
async function start(binding = '') {
 service = spawn(binary, [], { cwd: repo, env: { ...environment, LUCI_MANAGED_TEAM_ID: binding }, stdio: ['ignore', 'pipe', 'pipe'] });
 let spawnError; service.on('error', error => { spawnError = error; }); service.stdout.pipe(log, { end: false }); service.stderr.pipe(log, { end: false });
 const deadline = Date.now() + 30000;
 while (Date.now() < deadline) {
  if (spawnError) throw spawnError;
  assert.equal(service.exitCode, null, 'Service exited before readiness');
  try { version = (await request('GET', '/v1/control-plane/status')).version; return; } catch { await delay(100); }
 }
 throw new Error('Service readiness timed out');
}
async function stop() {
 if (!service || service.exitCode !== null) return;
 await request('POST', '/v1/control-plane/shutdown');
 const deadline = Date.now() + 15000;
 while (service.exitCode === null && Date.now() < deadline) await delay(100);
 assert.equal(service.exitCode, 0, 'Gateway did not stop cleanly');
}
async function login(member = false) { token = (await request('POST', '/v1/auth/login', { username: member ? 'restore-member' : 'restore-owner', password: member ? memberPassword : password })).token; }
const docker = (...args) => execFileSync('docker', ['--host', 'unix:///var/run/docker.sock', ...args], { encoding: 'utf8', timeout: 30000, maxBuffer: 4 * 1024**2 });
function recovery(operation, sha256, archivePath = archive) {
 const args = [join(repo, 'scripts/managed-host-backup.py'), operation, '--state-dir', state, '--team-id', teamId, '--release-version', version, '--archive', archivePath, '--docker-host', 'unix:///var/run/docker.sock', '--acknowledge-private-secrets', '--acknowledge-stopped-workloads', ...(sha256 ? ['--sha256', sha256] : [])];
 return JSON.parse(execFileSync('python3', args, { encoding: 'utf8', timeout: 60000, stdio: ['ignore', 'pipe', 'pipe'] }));
}
function erase(requestId, proof, receipt) {
 const args = [join(repo, 'scripts/managed-host-delete.py'), '--state-dir', state, '--team-id', teamId, '--request-id', requestId, '--release-version', version, '--archive', proof.archive, '--sha256', proof.sha256, '--docker-host', 'unix:///var/run/docker.sock', '--receipt', receipt, '--acknowledge-state-erasure'];
 return JSON.parse(execFileSync('python3', args, { encoding: 'utf8', timeout: 60000, stdio: ['ignore', 'pipe', 'pipe'] }));
}
function refused(reason) {
 assert.throws(() => recovery('backup'), error => error.status === 1 && String(error.stderr).includes(reason));
}
try {
 docker('info', '--format', '{{.ServerVersion}}');
 await start();
 const bootstrapToken = readFileSync(join(state, 'control-plane/bootstrap-token'), 'utf8').trim();
 token = (await request('POST', '/v1/auth/bootstrap', { bootstrapToken, username: 'restore-owner', displayName: 'Restore owner', password }, 201)).token;
 const member = await request('POST', '/v1/admin/users', { username: 'restore-member', displayName: 'Restore member', password: memberPassword, role: 'member' }, 201);
 let team = await request('POST', '/v1/teams', { name: 'Private recovery team' }, 201); teamId = team.team.id; result.teamId = teamId;
 team = await request('POST', `/v1/teams/${teamId}/members`, { expectedRevision: team.team.revision, username: 'restore-member', role: 'viewer', grants: { execute: false, secrets: false, deploy: false, admin: false } });
 const workspace = `/v1/teams/${teamId}/workspace`, project = await request('POST', workspace + '/v1/product/projects', { name: 'recovery-context' }, 201);
 const projectPath = workspace + `/v1/product/projects/${project.id}`;
 team = await request('GET', `/v1/teams/${teamId}`);
 await request('PUT', `/v1/teams/${teamId}/projects/${project.id}/members/${member.id}`, { expectedRevision: team.team.revision, role: 'viewer', grants: { execute: false, secrets: false, deploy: false, admin: false } });
 let brief = await request('GET', projectPath + '/brief');
 brief = await request('POST', projectPath + '/brief', { revision: brief.revision, purpose: 'Shared context survives a private host restore', audience: 'Owner and viewer', constraints: 'No inference', criteria: ['Account grants remain private and effective'] });
 const history = await request('GET', projectPath + '/context-history');
 await stop(); token = undefined; await start(teamId); await login();
 const beforeTeam = await request('GET', `/v1/teams/${teamId}`), beforeService = await request('GET', `/v1/teams/${teamId}/service`);
 refused('still running'); checks.push('live gateway refuses offline backup');
 await stop(); token = undefined;
 const workerLease = join(state, 'control-plane/teams', teamId, 'runtime/state/.luci-state.lock');
 orphan = spawn('python3', ['-c', 'import fcntl,sys,time; f=open(sys.argv[1],"r"); fcntl.flock(f,fcntl.LOCK_EX); print("ready",flush=True); time.sleep(60)', workerLease], { stdio: ['ignore', 'pipe', 'pipe'] });
 await once(orphan.stdout, 'data'); refused('still running'); orphan.kill('SIGTERM'); await once(orphan, 'exit'); orphan = undefined;
 checks.push('orphaned worker lease refuses snapshot after gateway stopped');
 writer = docker('run', '--detach', '--rm', '--pull=never', '--network', 'none', '--memory', '32m', '--cpus', '0.1', '--pids-limit', '16', '--label', `ai.lcfr.luci.host-recovery-eval=${randomUUID()}`, '--mount', `type=bind,src=${state},dst=/fixture`, '--entrypoint', '/bin/sh', 'luci/sandbox:dev', '-c', 'sleep 120').trim();
 refused('writable mount'); docker('rm', '--force', writer); writer = undefined;
 checks.push('real running Docker bind writer refuses snapshot');
 const backup = recovery('backup'); result.archive = backup.archive; result.archiveSHA256 = backup.sha256;
 const retained = state + '.retained-original'; renameSync(state, retained); mkdirSync(state, { mode: 0o700 });
 const restored = recovery('restore', backup.sha256); assert.equal(restored.serviceStarted, false);
 for (const file of ['control-plane/identity.json', 'control-plane/managed-team.json']) assert.deepEqual(readFileSync(join(state, file)), readFileSync(join(retained, file)));
 checks.push('private identity and binding restore byte-for-byte into the original empty path without starting service');
 await start(); await login();
 assert.equal((await request('GET', '/v1/teams')).managedTeamId, teamId);
 assert.deepEqual(await request('GET', `/v1/teams/${teamId}`), beforeTeam);
 assert.deepEqual(await request('GET', `/v1/teams/${teamId}/service`), beforeService);
 assert.deepEqual(await request('GET', projectPath + '/brief'), brief);
 assert.deepEqual(await request('GET', projectPath + '/context-history'), history);
 await request('GET', '/v1/projects', undefined, 403);
 await login(true);
 const viewer = await request('GET', `/v1/teams/${teamId}/projects/${project.id}/members`);
 assert.equal(viewer.permissions.read, true); assert.equal(viewer.permissions.edit, false);
 assert.deepEqual(await request('GET', projectPath + '/brief'), brief);
 await request('POST', projectPath + '/brief', { revision: brief.revision, purpose: 'forbidden' }, 403);
 checks.push('normal owner/member passwords, private membership grants, service state, shared context and attribution remain effective after restart');
 await login();
 const lifecyclePath = `/v1/teams/${teamId}/data-lifecycle`;
 let lifecycleState = await request('GET', lifecyclePath);
 lifecycleState = await request('POST', lifecyclePath + '/request', { expectedRevision: lifecycleState.team.revision, acknowledgeDeletion: true });
 const firstRequest = lifecycleState.dataLifecycle.requestId;
 assert.equal(lifecycleState.dataLifecycle.status, 'requested');
 assert.deepEqual(await request('GET', projectPath + '/brief'), brief, 'Request must not erase shared context');
 await stop(); token = undefined;
 const cancelledProof = recovery('backup', undefined, join(fixture, 'cancelled-request-proof.zip'));
 await start(); await login();
 lifecycleState = await request('GET', lifecyclePath);
 lifecycleState = await request('POST', lifecyclePath + '/cancel', { expectedRevision: lifecycleState.team.revision, requestId: firstRequest });
 assert.equal(lifecycleState.dataLifecycle.status, 'cancelled');
 await stop(); token = undefined;
 assert.throws(() => erase(firstRequest, cancelledProof, join(fixture, 'must-not-create-receipt.json')), error => error.status === 1 && String(error.stderr).includes('pending owner deletion request'));
 assert.ok(readFileSync(join(state, 'control-plane/identity.json')).length);
 checks.push('a real owner cancellation defeats a previously verified pending-request backup without erasing state');
 await start(); await login();
 lifecycleState = await request('GET', lifecyclePath);
 lifecycleState = await request('POST', lifecyclePath + '/request', { expectedRevision: lifecycleState.team.revision, acknowledgeDeletion: true });
 const currentRequest = lifecycleState.dataLifecycle.requestId;
 assert.notEqual(currentRequest, firstRequest);
 await stop(); token = undefined;
 const currentProof = recovery('backup', undefined, join(fixture, 'current-request-proof.zip'));
 const erased = erase(currentRequest, currentProof, join(fixture, 'private-state-erasure-receipt.json'));
 assert.equal(erased.status, 'state-erased'); assert.equal(erased.resourceCleanupComplete, false);
 assert.deepEqual(new Set(readdirSync(state)), new Set(['.luci-state.lock', '.luci-host-deleted']));
 assert.equal(createHash('sha256').update(readFileSync(currentProof.archive)).digest('hex'), currentProof.sha256);
 assert.ok(readFileSync(join(retained, 'control-plane/identity.json')).length, 'Retained recovery copy must not be erased');
 const blocked = spawn(binary, [], { cwd: repo, env: environment, stdio: ['ignore', 'pipe', 'pipe'] });
 let refusal = ''; blocked.stdout.on('data', chunk => { refusal += chunk; }); blocked.stderr.on('data', chunk => { refusal += chunk; });
 const refusalDeadline = setTimeout(() => blocked.kill('SIGTERM'), 10000);
 const [exitCode] = await once(blocked, 'exit'); clearTimeout(refusalDeadline);
 assert.equal(exitCode, 1); assert.match(refusal, /marked for erasure/);
 result.erasureRequestId = currentRequest; result.erasureReceipt = join(fixture, 'private-state-erasure-receipt.json');
 checks.push('current real request plus exact current private backup permits bounded state erasure, retains external proofs and blocks automatic fresh bootstrap');
 result.passed = true;
} catch (error) { result.error = String(error); console.error(error.stack); process.exitCode = 1; }
finally {
 if (orphan) orphan.kill('SIGTERM');
 if (writer) { try { docker('rm', '--force', writer); } catch { result.writerCleanupFailed = true; process.exitCode = 1; } }
 try { await stop(); } catch (error) { result.stopError = String(error); service?.kill('SIGTERM'); process.exitCode = 1; }
 log.end(); if (process.exitCode) result.passed = false;
 writeFileSync(join(evidence, 'result.json'), JSON.stringify(result, null, 2) + '\n', { mode: 0o600 });
 console.log(JSON.stringify(result));
}

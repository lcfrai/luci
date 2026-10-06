#!/usr/bin/env node
// Actual compiled-service acceptance. No Docker, provider account or model key
// is required. Each run owns its temporary state and hidden child processes.
import assert from 'node:assert/strict';
import { spawn, execFile } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { promises as fs } from 'node:fs';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { promisify } from 'node:util';
const execFileAsync = promisify(execFile);

const args = process.argv.slice(2);
const option = (name, fallback) => { const i = args.indexOf(name); return i < 0 ? fallback : args[i + 1]; };
const binary = path.resolve(option('--binary', process.platform === 'win32' ? 'artifacts/goal-build/luci.exe' : 'artifacts/goal-build/luci'));
const output = path.resolve(option('--output', 'artifacts/control-plane-smoke.json'));
const root = await fs.mkdtemp(path.join(os.tmpdir(), 'luci-control-proof-'));
const state = path.join(root, 'state'), projects = path.join(root, 'legacy-projects');
await fs.mkdir(projects, { recursive: true });
const reserve = net.createServer(); await new Promise((resolve) => reserve.listen(0, '127.0.0.1', resolve));
const port = reserve.address().port; await new Promise((resolve) => reserve.close(resolve));
const base = `http://127.0.0.1:${port}`;
const password = `Proof-${randomBytes(18).toString('hex')}`;
const replacementPassword = `Changed-${randomBytes(18).toString('hex')}`;
const marker = `fixture-${randomBytes(12).toString('hex')}`;
const evidence = { version: 1, completedAt: null, status: 'running', binarySHA256: createHash('sha256').update(await fs.readFile(binary)).digest('hex'), outcomes: [], limitations: ['No Docker/container, model inference, real GitHub, cloud account or DNS/TLS call is made.'] };
const env = { ...process.env, LUCI_ADDR: `127.0.0.1:${port}`, LUCI_STATE_DIR: state, LUCI_PROJECTS_DIR: projects, LUCI_BACKEND: 'docker', LUCI_BACKING: 'docker', LUCI_GIT_NAME: '', LUCI_GIT_EMAIL: '', GIT_CONFIG_GLOBAL: path.join(root, 'owner.gitconfig'), GIT_CONFIG_NOSYSTEM: '1', GH_TOKEN: marker, OPENAI_API_KEY: marker, AWS_ACCESS_KEY_ID: marker, LUCI_WEB_DIR: '', LUCI_WORKER: '', LUCI_WORKER_TOKEN: '', LUCI_WORKER_ADDRESS_FILE: '', LUCI_MANAGED_SHUTDOWN_TOKEN: '' };
const child = spawn(binary, [], { cwd: root, env, windowsHide: true, stdio: ['ignore', 'pipe', 'pipe'] });
let log = '', exit = null; child.stdout.on('data', (x) => { log = (log + x).slice(-16384); }); child.stderr.on('data', (x) => { log = (log + x).slice(-16384); });
child.on('error', (error) => { exit = error.message; }); const exited = new Promise((resolve) => child.once('exit', (code) => { exit = code; resolve(); }));
let ownerToken = '', memberToken = '';
const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
async function api(method, resource, token, body, expected = 200) {
  const headers = {}; if (token) headers.Authorization = `Bearer ${token}`;
  if (body !== undefined) headers['Content-Type'] = Buffer.isBuffer(body) ? 'application/zip' : 'application/json';
  const response = await fetch(base + resource, { method, headers, body: body === undefined ? undefined : Buffer.isBuffer(body) ? body : JSON.stringify(body), signal: AbortSignal.timeout(45000) });
  const data = Buffer.from(await response.arrayBuffer());
  assert.ok([expected].flat().includes(response.status), `${method} ${resource}: HTTP ${response.status}; ${data.toString().slice(0, 700)}`);
  return response.headers.get('content-type')?.includes('application/json') ? JSON.parse(data) : data;
}
function pass(name, detail) { evidence.outcomes.push({ name, status: 'passed', detail }); }
try {
  let status; const deadline = Date.now() + 30000;
  while (Date.now() < deadline) { if (exit !== null) throw new Error(`Service exited during startup (${exit}).`); try { status = await api('GET', '/v1/control-plane/status'); break; } catch { await pause(200); } }
  assert.ok(status?.bootstrapRequired, 'Fresh service did not offer real bootstrap.');
  await api('GET', '/v1/projects', '', undefined, 401);
  const bootstrapToken = (await fs.readFile(path.join(state, 'control-plane', 'bootstrap-token'), 'utf8')).trim();
  const owner = await api('POST', '/v1/auth/bootstrap', '', { bootstrapToken, username: 'owner', displayName: 'Fixture Owner', password }, 201); ownerToken = owner.token;
  assert.equal(owner.user.role, 'super-admin');
  const member = await api('POST', '/v1/admin/users', ownerToken, { username: 'member', displayName: 'Fixture Member', password, role: 'member' }, 201);
  const signedIn = await api('POST', '/v1/auth/login', '', { username: 'member', password }); memberToken = signedIn.token;
  assert.equal((await api('GET', '/v1/auth/me', memberToken)).permissions.manageUsers, false);
  pass('real_identity_bootstrap_login', 'Fresh bootstrap creates Super Admin; member sign-in uses persisted Argon2id identity and opaque bearer sessions.');

  await api('POST', '/v1/setup/git/identity', memberToken, { name: 'Fixture Member', email: 'member@example.invalid' });
  const project = await api('POST', '/v1/product/projects', memberToken, { name: 'member-app' }, 201);
  const runtimeRoot = path.join(state, 'control-plane', 'users', member.id, 'runtime');
  assert.equal(path.resolve(project.path), path.join(runtimeRoot, 'projects', 'member-app'));
  const projectBase = `/v1/product/projects/${project.id}`;
  const brief = await api('POST', projectBase + '/brief', memberToken, { revision: 0, purpose: 'A private member project', audience: 'Fixture users', constraints: 'No cloud or inference', criteria: ['Preserve private ownership'] });
  const plan = await api('POST', projectBase + '/plan', memberToken, { expectedRevision: 0, tasks: [{ id: 'build', title: 'Build the reviewed feature', status: 'todo' }] });
  const design = await api('POST', projectBase + '/design', memberToken, { revision: 0, frames: [{ id: 'home', name: 'Member design', width: 600, height: 400, background: '#ffffff', elements: [] }] });
  assert.equal(brief.revision, 1); assert.equal(plan.revision, 1); assert.equal(design.revision, 1);
  const definition = await api('GET', projectBase + '/specification', memberToken);
  definition.requirements = [{ id: 'private-ownership', title: 'Private member workspace', priority: 'must', acceptance: 'Only this member can read the project through the control API, including after its worker restarts.' }];
  const savedDefinition = await api('POST', projectBase + '/specification', memberToken, { expectedRevision: definition.revision, expectedBriefRevision: definition.briefRevision, specification: definition });
  const approvedDefinition = await api('POST', projectBase + '/specification/approve', memberToken, { revision: savedDefinition.revision, briefRevision: savedDefinition.briefRevision });
  assert.equal(approvedDefinition.status, 'approved'); assert.match(approvedDefinition.digest, /^[a-f0-9]{64}$/);
  const handoff = await api('GET', projectBase + '/design-plan', memberToken);
  handoff.screens = [{ id: 'member-home', name: 'Private project', route: '/', purpose: 'Read the owned project and recover from failure', states: ['Loading', 'Owned project', 'Access denied', 'Retry'], widths: [390, 1440], requirementIds: ['private-ownership'], accessibility: 'Labeled controls and keyboard access' }];
  const savedHandoff = await api('POST', projectBase + '/design-plan', memberToken, { expectedRevision: handoff.revision, expectedBriefRevision: approvedDefinition.briefRevision, expectedSpecificationRevision: approvedDefinition.revision, plan: handoff });
  const approvedHandoff = await api('POST', projectBase + '/design-plan/approve', memberToken, { revision: savedHandoff.revision });
  assert.equal(approvedHandoff.status, 'approved'); assert.match(approvedHandoff.digest, /^[a-f0-9]{64}$/);
  const commit = await api('POST', projectBase + '/git/commit', memberToken, { message: 'Save member brief, plan and design' });
  assert.match(commit.commit, /^[0-9a-f]{40,64}$/);
  const ownIdentity = await fs.readFile(path.join(runtimeRoot, 'home', '.gitconfig'), 'utf8');
  assert.match(ownIdentity, /Fixture Member/);
  await assert.rejects(fs.readFile(path.join(root, 'owner.gitconfig')), { code: 'ENOENT' });
  await api('GET', projectBase + '/brief', ownerToken, undefined, [403, 404]);
  const ownerProjects = await api('GET', '/v1/projects', ownerToken); assert.ok(!ownerProjects.projects.some((x) => x.id === project.id));
  pass('isolated_compiled_worker_journey', 'Actual supervised child owns separate projects, Git identity and saved brief/plan/design; immutable commit created; owner API cannot resolve member project ID.');

  const outside = path.join(root, 'outside-private'); await fs.mkdir(outside); await fs.writeFile(path.join(outside, 'LUCI.md'), `outside-private-${marker}`);
  for (const [key, value] of [['core.worktree', outside], ['core.bare', 'true']]) await execFileAsync('git', ['-C', project.path, 'config', key, value], { env, windowsHide: true });
  const guardedGit = await api('GET', projectBase + '/git', memberToken);
  assert.ok(!JSON.stringify(guardedGit).includes(marker), 'Repository configuration escaped into private host files.'); assert.equal(guardedGit.clean, true);
  pass('repository_config_confinement', 'A real member repository with hostile core.worktree and core.bare still reads only its assigned project; outside private marker never enters the API.');

  for (const [method, route, body] of [
    ['GET', '/v1/admin/users', undefined], ['PATCH', '/v1/admin/policy', { allowHostCommands: true }],
    ['POST', '/v1/setup/cloud/create', {}], ['POST', '/v1/setup/config', { projectsDir: projects }],
    ['POST', projectBase + '/checks', { execution: 'host' }],
  ]) await api(method, route, memberToken, body, 403);
  await api('PATCH', '/v1/admin/users/' + owner.user.id, ownerToken, { disabled: true }, 409);
  await api('PATCH', '/v1/admin/policy', ownerToken, { maxSessionsPerUser: 2, maxConcurrentChecksPerUser: 1, allowPublicPublishing: false });
  await api('POST', projectBase + '/git/github', memberToken, { name: 'not-created', visibility: 'public', acknowledgePublic: true }, 403);
  pass('management_and_execution_policy', 'Member cannot administer, provision, change its workspace root or execute host checks; last Super Admin cannot be disabled; public creation policy enforced before GitHub calls.');

  await fs.writeFile(path.join(project.path, '.env'), `SECRET=${marker}\n`, { mode: 0o600 });
  const archive = await api('POST', projectBase + '/backups', memberToken, {});
  assert.ok(Buffer.isBuffer(archive) && archive.subarray(0, 2).toString() === 'PK');
  const restored = await api('POST', '/v1/product/projects/restore?name=restored-app', memberToken, archive, 201);
  assert.notEqual(restored.id, project.id); await assert.rejects(fs.readFile(path.join(restored.path, '.env')), { code: 'ENOENT' });
  const restoredBase = `/v1/product/projects/${restored.id}`;
  assert.equal((await api('GET', restoredBase + '/brief', memberToken)).projectId, restored.id);
  const restoredPlan = await api('GET', restoredBase + '/plan', memberToken); assert.equal(restoredPlan.projectId, restored.id); assert.equal(restoredPlan.tasks[0].title, plan.tasks[0].title);
  const restoredDesign = await api('GET', restoredBase + '/design', memberToken); assert.equal(restoredDesign.projectId, restored.id); assert.equal(restoredDesign.frames[0].name, design.frames[0].name);
  const restoredDefinition = await api('GET', restoredBase + '/specification', memberToken);
  const restoredHandoff = await api('GET', restoredBase + '/design-plan', memberToken);
  assert.notEqual(restoredDefinition.status, 'approved'); assert.notEqual(restoredHandoff.status, 'approved');
  assert.equal(restoredDefinition.requirements[0].id, 'private-ownership');
  const restoredGit = await api('GET', restoredBase + '/git', memberToken); assert.equal(restoredGit.log[0].hash, commit.commit);
  await api('POST', '/v1/product/projects/restore?name=restored-app', memberToken, archive, 409);
  pass('actual_zip_backup_restore', `Downloaded ${archive.length} byte ZIP, restored history/brief/plan/design with new identity, omitted current secret file, refused overwrite.`);

  const before = await api('GET', `/v1/admin/users/${member.id}/runtime`, ownerToken); assert.equal(before.status, 'running');
  await api('POST', `/v1/admin/users/${member.id}/runtime/stop`, ownerToken, {});
  let stopped; for (let n = 0; n < 50; n++) { stopped = await api('GET', `/v1/admin/users/${member.id}/runtime`, ownerToken); if (stopped.status === 'stopped') break; await pause(100); } assert.equal(stopped.status, 'stopped');
  assert.equal((await api('GET', projectBase + '/brief', memberToken)).purpose, brief.purpose);
  const retainedDefinition = await api('GET', projectBase + '/specification', memberToken);
  const retainedHandoff = await api('GET', projectBase + '/design-plan', memberToken);
  assert.equal(retainedDefinition.status, 'approved'); assert.equal(retainedDefinition.digest, approvedDefinition.digest);
  assert.equal(retainedHandoff.status, 'approved'); assert.equal(retainedHandoff.digest, approvedHandoff.digest);
  const restarted = await api('GET', `/v1/admin/users/${member.id}/runtime`, ownerToken); assert.equal(restarted.status, 'running'); assert.notEqual(restarted.startedAt, before.startedAt);
  pass('owned_runtime_stop_restart', 'Administrator stopped the actual child; next authenticated member request started a new child with durable project data and exact private-backed Define/Design approvals. Imported content retains its requirements without importing approval authority.');

  await api('POST', '/v1/auth/password', memberToken, { currentPassword: password, newPassword: replacementPassword });
  await api('GET', '/v1/auth/me', memberToken, undefined, 401);
  const changed = await api('POST', '/v1/auth/login', '', { username: 'member', password: replacementPassword }); memberToken = changed.token;
  await api('PATCH', '/v1/admin/users/' + member.id, ownerToken, { disabled: true });
  await api('GET', '/v1/auth/me', memberToken, undefined, 401);
  const audit = await api('GET', '/v1/admin/audit', ownerToken);
  assert.ok(audit.events.some((x) => x.action === 'runtime.stop')); assert.ok(audit.events.some((x) => x.action === 'auth.password'));
  const auditText = JSON.stringify(audit); for (const secret of [password, replacementPassword, ownerToken, memberToken, marker]) assert.ok(!auditText.includes(secret));
  const identityText = await fs.readFile(path.join(state, 'control-plane', 'identity.json'), 'utf8'); for (const secret of [password, replacementPassword, ownerToken, memberToken]) assert.ok(!identityText.includes(secret));
  pass('revocation_and_secret_free_audit', 'Password changes and disable revoke sessions; audit attributes actions without passwords or bearer tokens; persisted sessions are hashed.');
  evidence.status = 'passed';
} catch (error) {
  evidence.status = 'failed'; evidence.failure = String(error.message).replaceAll(password, '[redacted]').replaceAll(replacementPassword, '[redacted]').replaceAll(marker, '[redacted]');
  process.exitCode = 1;
} finally {
  if (ownerToken && exit === null) { try { await api('POST', '/v1/setup/shutdown', ownerToken, {}); } catch {} }
  await Promise.race([exited, pause(8000)]); if (exit === null) { child.kill(); await Promise.race([exited, pause(3000)]); }
  evidence.completedAt = new Date().toISOString(); await fs.mkdir(path.dirname(output), { recursive: true }); await fs.writeFile(output, JSON.stringify(evidence, null, 2) + '\n');
  // All files are owned by this test's mkdtemp; no user project is removed.
  if (path.dirname(root) === os.tmpdir() && path.basename(root).startsWith('luci-control-proof-')) await fs.rm(root, { recursive: true, force: true, maxRetries: 4, retryDelay: 100 });
}
console.log(JSON.stringify({ status: evidence.status, outcomes: evidence.outcomes.length, evidence: output, ...(evidence.failure ? { failure: evidence.failure } : {}) }));

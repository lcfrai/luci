#!/usr/bin/env node
// Real existing-workspace acceptance: two compiled services, OpenSSH, Chromium.
// Run as an ordinary unlocked Linux user. No provider accounts, Docker, hosted
// mock API or paid inference are used. /run/sshd must exist; port8730 must be free.
import assert from 'node:assert/strict';
import { spawn, execFile } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { promises as fs } from 'node:fs';
import { createRequire } from 'node:module';
import net from 'node:net';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { promisify } from 'node:util';

const args = process.argv.slice(2);
if (args.includes('--help')) {
  console.log('Usage: node scripts/workspace-browser-eval.mjs --binary <luci> --desktop <desktop directory with dist and node_modules> --output <evidence directory> [--sshd /usr/sbin/sshd]');
  process.exit(0);
}
const option = (name, fallback) => { const i = args.indexOf(name); return i < 0 ? fallback : args[i + 1]; };
const repo = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const binary = path.resolve(option('--binary', path.join(repo, 'artifacts/goal-build/luci')));
const desktop = path.resolve(option('--desktop', path.join(repo, 'desktop')));
const output = path.resolve(option('--output', path.join(repo, 'artifacts/workspace-browser-eval')));
const sshdBinary = path.resolve(option('--sshd', '/usr/sbin/sshd'));
assert.equal(process.platform, 'linux', 'This real SSH acceptance runs on Linux.');
assert.notEqual(process.getuid(), 0, 'Run this acceptance as an ordinary user.');
const { chromium } = createRequire(path.join(desktop, 'package.json'))('playwright');
const exec = promisify(execFile);
const pause = (ms) => new Promise((resolve) => setTimeout(resolve, ms));
// OpenSSH StrictModes checks every parent of AuthorizedKeysFile until the
// account home. Keep generated keys beneath that owned home, never world-writable
// /tmp; do not weaken host/user-key validation to accommodate the test fixture.
const root = await fs.mkdtemp(path.join(os.homedir(), '.luci-workspace-proof-'));
await fs.chmod(root, 0o700);
const username = os.userInfo().username;
assert.match(username, /^[a-z_][a-z0-9_-]{0,31}$/);
const passwordLocal = `Local-${randomBytes(18).toString('hex')}`;
const passwordRemote = `Remote-${randomBytes(18).toString('hex')}`;
const secrets = [passwordLocal, passwordRemote];
const processes = [];
const evidence = { version: 1, startedAt: new Date().toISOString(), status: 'running', binarySHA256: createHash('sha256').update(await fs.readFile(binary)).digest('hex'), uid: process.getuid(), outcomes: [], limitations: ['Loopback SSH proves the real existing-workspace connection and account route. AWS/Azure purchases, public DNS/TLS, Docker execution and model inference are outside this test.'] };
let browser, page, local, remote, stage = 'initialize';
function pass(name, detail) { evidence.outcomes.push({ name, status: 'passed', detail }); }
function sanitize(value) { let result = String(value); for (const secret of secrets) if (secret) result = result.replaceAll(secret, '[redacted]'); return result; }
async function freePort(required = 0) {
  const server = net.createServer();
  await new Promise((resolve, reject) => { server.once('error', reject); server.listen(required, '127.0.0.1', resolve); });
  const port = server.address().port;
  await new Promise((resolve) => server.close(resolve));
  return port;
}
function start(executable, argv, env, label) {
  const child = spawn(executable, argv, { cwd: root, env, detached: true, stdio: ['ignore', 'pipe', 'pipe'] });
  const proc = { child, label, log: '', exited: false, code: null };
  processes.push(proc);
  for (const stream of [child.stdout, child.stderr]) stream.on('data', (chunk) => { proc.log = (proc.log + chunk).slice(-16384); });
  child.once('exit', (code) => { proc.exited = true; proc.code = code; });
  child.once('error', (error) => { proc.exited = true; proc.code = error.message; });
  return proc;
}
async function until(check, message, timeout = 30000) {
  const deadline = Date.now() + timeout;
  while (Date.now() < deadline) { if (await check()) return; await pause(150); }
  throw new Error(message);
}
async function api(service, method, resource, body, expected = 200, token = service.token) {
  const response = await fetch(service.base + resource, { method, redirect: 'error', headers: { ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}) }, body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(30000) });
  // Do not include arbitrary response bodies: authentication and provider errors
  // must not put credentials in evidence or the CI log.
  assert.ok([expected].flat().includes(response.status), `${method} ${resource}: expected${expected}, got${response.status}`);
  return response.json();
}
async function startService(name, port, account, password) {
  const directory = path.join(root, name), state = path.join(directory, 'state'), projects = path.join(directory, 'projects'), home = path.join(directory, 'home');
  for (const folder of [state, projects, home]) await fs.mkdir(folder, { recursive: true, mode: 0o700 });
  // Deliberate minimal environment: no host provider credentials or configuration.
  const env = { PATH: process.env.PATH, HOME: home, USER: username, LOGNAME: username, LANG: 'C.UTF-8', TMPDIR: root, LUCI_ADDR: `127.0.0.1:${port}`, LUCI_STATE_DIR: state, LUCI_PROJECTS_DIR: projects, LUCI_BACKEND: 'docker', LUCI_BACKING: 'docker', LUCI_WEB_DIR: path.join(desktop, 'dist'), GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: path.join(home, '.gitconfig'), GH_CONFIG_DIR: path.join(home, 'gh'), AWS_CONFIG_FILE: path.join(home, 'aws-config'), AWS_SHARED_CREDENTIALS_FILE: path.join(home, 'aws-credentials'), AZURE_CONFIG_DIR: path.join(home, 'azure') };
  const proc = start(binary, [], env, name);
  const service = { base: `http://127.0.0.1:${port}`, state, projects, proc, token: '' };
  await until(async () => {
    if (proc.exited) throw new Error(`${name} exited during startup: ${proc.code}`);
    try { return (await api(service, 'GET', '/v1/control-plane/status')).bootstrapRequired; } catch { return false; }
  }, `${name} did not start`);
  const bootstrapToken = (await fs.readFile(path.join(state, 'control-plane/bootstrap-token'), 'utf8')).trim(); secrets.push(bootstrapToken);
  const result = await api(service, 'POST', '/v1/auth/bootstrap', { bootstrapToken, username: account, displayName: name === 'local' ? 'Local Workspace Owner' : 'Remote Workspace Owner', password }, 201);
  service.token = result.token; secrets.push(result.token); service.user = result.user;
  return service;
}
async function clickResponse(button, resource, expected = 200, readBody = true, method = 'POST') {
  const received = page.waitForResponse((response) => new URL(response.url()).pathname === resource && response.request().method() === method);
  // Handle both promises immediately. If a locator click fails, retain that
  // error instead of an unhandled response wait masking it during cleanup.
  const [response] = await Promise.all([received, button.click()]);
  assert.equal(response.status(), expected, `Browser ${resource} returned${response.status()}`);
  return readBody ? response.json() : undefined;
}
async function switchWorkspace(button, resource) {
  // Activation deliberately reloads the document. Its response body is not used;
  // Chromium may discard that body before CDP can read it after the navigation.
  const navigation = page.waitForEvent('framenavigated', { predicate: (frame) => frame === page.mainFrame() });
  await Promise.all([navigation, clickResponse(button, resource, 200, false)]);
}
async function saveStructuredDefinition(projectId, purpose, audience, acceptance) {
  await page.getByRole('heading', { name: 'Define your project', exact: true }).waitFor();
  await page.getByRole('button', { name: 'Specification', exact: true }).click();
  await page.getByRole('textbox', { name: 'The idea', exact: true }).fill(purpose);
  await page.getByRole('textbox', { name: 'Who it is for', exact: true }).fill(audience);
  await page.getByRole('textbox', { name: 'Boundaries and constraints', exact: true }).fill('Keep project data private to this workspace identity. Use the already verified SSH connection.');
  await page.getByRole('button', { name: 'Add requirement', exact: true }).click();
  await page.getByRole('textbox', { name: 'Requirement 1', exact: true }).fill('Private durable project');
  await page.getByRole('textbox', { name: 'Acceptance scenario 1', exact: true }).fill(acceptance);
  const saved = await clickResponse(page.getByRole('button', { name: 'Save definition', exact: true }), `/v1/product/projects/${projectId}/specification`);
  assert.equal(saved.revision, 1);
  assert.equal(saved.briefRevision, 1);
  assert.equal(saved.requirements.length, 1);
  assert.equal(saved.requirements[0].acceptance, acceptance);
  const approved = await clickResponse(page.getByRole('button', { name: 'Approve definition', exact: true }), `/v1/product/projects/${projectId}/specification/approve`);
  assert.equal(approved.status, 'approved');
  assert.equal(approved.revision, 2);
  assert.equal(approved.briefRevision, saved.briefRevision);
  assert.equal(approved.requirements[0].id, saved.requirements[0].id);
  assert.match(approved.digest, /^[a-f0-9]{64}$/);
  return approved;
}
try {
  await fs.mkdir(output, { recursive: true });
  await fs.access(path.join(desktop, 'dist/index.html'));
  await freePort(8730);
  await freePort(8000); // Do not let an unrelated management service mask readiness.
  stage = 'start-real-services';
  remote = await startService('remote', 8730, 'remote-owner', passwordRemote);
  local = await startService('local', await freePort(), 'local-owner', passwordLocal);
  const remoteSettings = await api(remote, 'GET', '/v1/setup/status');
  await api(remote, 'POST', '/v1/setup/config', { ...remoteSettings.config, projectsDir: remote.projects });
  const sentinel = await api(local, 'POST', '/v1/product/projects', { name: 'local-only-project' }, 201);
  assert.notEqual(local.user.id, remote.user.id);
  assert.equal((await api(local, 'GET', '/v1/setup/status')).configured, false);

  stage = 'start-user-owned-sshd';
  const key = path.join(root, 'client-key'), hostKey = path.join(root, 'host-key');
  for (const filename of [key, hostKey]) {
    await exec('ssh-keygen', ['-q', '-t', 'ed25519', '-N', '', '-f', filename]);
    await fs.chmod(filename, 0o600);
    const metadata = await fs.stat(filename);
    assert.equal(metadata.uid, process.getuid(), 'Generated SSH key must belong to the ordinary test user');
    assert.equal(metadata.mode & 0o777, 0o600, 'Generated SSH private key must be owner-only');
  }
  const publicKey = (await fs.readFile(hostKey + '.pub', 'utf8')).trim().split(/\s+/)[1];
  const trustedFingerprint = 'SHA256:' + createHash('sha256').update(Buffer.from(publicKey, 'base64')).digest('base64').replace(/=+$/, '');
  const authorized = path.join(root, 'authorized_keys');
  await fs.copyFile(key + '.pub', authorized); await fs.chmod(authorized, 0o600);
  const sshPort = await freePort(), config = path.join(root, 'sshd_config');
  await fs.writeFile(config, [`Port ${sshPort}`, 'ListenAddress 127.0.0.1', `HostKey ${hostKey}`, `PidFile ${path.join(root, 'sshd.pid')}`, `AuthorizedKeysFile ${authorized}`, 'PubkeyAuthentication yes', 'PasswordAuthentication no', 'KbdInteractiveAuthentication no', 'UsePAM no', 'StrictModes yes', 'PermitRootLogin no', `AllowUsers ${username}`, 'AllowTcpForwarding local', 'PermitOpen 127.0.0.1:8730 127.0.0.1:8000', 'AllowAgentForwarding no', 'X11Forwarding no', 'PermitTunnel no', 'GatewayPorts no', 'LogLevel VERBOSE', ''].join('\n'), { mode: 0o600 });
  await exec(sshdBinary, ['-t', '-f', config]);
  const sshd = start(sshdBinary, ['-D', '-e', '-f', config], { PATH: process.env.PATH, HOME: os.homedir(), USER: username, LOGNAME: username, LANG: 'C.UTF-8' }, 'sshd');
  await until(async () => {
    if (sshd.exited) throw new Error(`Owned sshd exited: ${sshd.code}`);
    return new Promise((resolve) => { const socket = net.connect(sshPort, '127.0.0.1'); socket.setTimeout(500); socket.once('connect', () => { socket.destroy(); resolve(true); }); socket.once('error', () => resolve(false)); socket.once('timeout', () => { socket.destroy(); resolve(false); }); });
  }, 'Owned sshd did not listen');
  const preflightKnownHosts = path.join(root, 'preflight-known-hosts');
  await fs.writeFile(preflightKnownHosts, `[127.0.0.1]:${sshPort} ssh-ed25519 ${publicKey}\n`, { mode: 0o600 });
  await exec('ssh', ['-p', String(sshPort), '-i', key, '-o', 'IdentitiesOnly=yes', '-o', 'StrictHostKeyChecking=yes', '-o', `UserKnownHostsFile=${preflightKnownHosts}`, '-o', 'BatchMode=yes', '-o', 'ConnectTimeout=10', `${username}@127.0.0.1`, 'true'], { timeout: 15000 });
  pass('separate_real_services_and_ssh', 'Separate compiled control services, state, identities and projects; generated Ed25519 host/user keys; ordinary-user sshd listens only on loopback. No management service is running on8000.');

  stage = 'browser-first-run-existing-choice';
  browser = await chromium.launch({ headless: true });
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 } });
  page = await context.newPage(); page.setDefaultTimeout(20000);
  const errors = []; page.on('pageerror', (error) => errors.push(error.message));
  await page.goto(local.base);
  await page.getByLabel('Username', { exact: true }).fill('local-owner');
  await page.getByLabel('Password', { exact: true }).fill(passwordLocal);
  await clickResponse(page.getByRole('button', { name: 'Sign in', exact: true }), '/v1/auth/login');
  await page.getByRole('heading', { name: 'Choose your workspace', exact: true }).waitFor();
  await page.getByRole('button', { name: 'Connect to an existing workspace', exact: true }).click();
  await page.getByLabel('Server address', { exact: true }).fill('127.0.0.1');
  await page.getByLabel('SSH port', { exact: true }).fill(String(sshPort));
  await page.getByLabel('SSH user', { exact: true }).fill(username);
  await page.getByLabel('Private key path', { exact: true }).fill(key);
  const scan = await clickResponse(page.getByRole('button', { name: 'Inspect server fingerprint' }), '/v1/setup/ssh/inspect');
  assert.ok(scan.fingerprints.includes(trustedFingerprint), 'Browser scan differs from independently hashed generated SSH host key');
  const trust = page.getByRole('button', { name: 'Trust and connect' });
  assert.equal(await trust.isDisabled(), true, 'SSH trust must require an explicit verified fingerprint');
  await page.getByRole('combobox', { name: 'Server fingerprint', exact: true }).selectOption(trustedFingerprint);
  await page.getByLabel('I compared this fingerprint with my server’s trusted console.').check();
  const tunnel = await clickResponse(trust, '/v1/setup/ssh/connect', 202);
  await until(async () => {
    const current = (await api(local, 'GET', '/v1/setup/ssh/connections')).connections.find((item) => item.id === tunnel.id);
    evidence.tunnel = current ? { status: current.status, message: current.message } : null;
    if (current?.status === 'failed' || current?.status === 'stopped') throw new Error(`Real SSH tunnel ${current.status}: ${current.message}`);
    return current?.status === 'connected';
  }, 'Real SSH tunnel did not become connected', 45000);
  await page.getByLabel('Remote username', { exact: true }).waitFor();
  assert.equal((await api(local, 'GET', '/v1/setup/ssh/connections')).connections.find((item) => item.id === tunnel.id)?.status, 'connected');
  pass('real_browser_host_verification', 'Visible onboarding choices lead to actual ssh-keyscan, independent host fingerprint comparison, explicit trust and a real pinned OpenSSH API tunnel; Luci readiness does not require Coolify.');

  stage = 'remote-account-error-and-retry';
  await page.getByLabel('Remote username', { exact: true }).fill('remote-owner');
  await page.getByLabel('Remote password', { exact: true }).fill('deliberately-wrong-password');
  await clickResponse(page.getByRole('button', { name: 'Sign in to workspace', exact: true }), '/v1/setup/workspace/auth', 409);
  await page.getByRole('alert').filter({ hasText: /sign in|password|credentials/i }).waitFor();
  assert.equal(await page.getByRole('button', { name: 'Use this cloud workspace' }).isDisabled(), true);
  assert.equal(await page.getByRole('button', { name: 'Sign in', exact: true }).count(), 0, 'Remote password error must not sign out the local browser');
  await page.getByLabel('Remote password', { exact: true }).fill(passwordRemote);
  const auth = await clickResponse(page.getByRole('button', { name: 'Sign in to workspace', exact: true }), '/v1/setup/workspace/auth');
  assert.equal(auth.authenticated, true); assert.equal(auth.user.id, remote.user.id);
  await until(() => page.getByRole('button', { name: 'Use this cloud workspace' }).isEnabled(), 'Authenticated workspace did not enable activation');
  await switchWorkspace(page.getByRole('button', { name: 'Use this cloud workspace' }), '/v1/setup/workspace/activate');
  await page.getByRole('button', { name: 'Local Workspace Owner', exact: true }).waitFor();
  assert.equal((await api(local, 'GET', '/v1/auth/me')).user.id, local.user.id);
  assert.equal((await api(local, 'GET', '/v1/setup/status')).workspace.mode, 'cloud');
  assert.ok(!(await api(local, 'GET', '/v1/projects')).projects.some((item) => item.id === sentinel.id));
  pass('separate_account_error_recovery_and_activation', 'Wrong remote password keeps the browser signed in locally and activation disabled; real remote sign-in succeeds on retry, then the selected cloud workspace replaces the project view while local identity stays intact.');

  stage = 'create-and-save-remote-project-in-browser';
  await page.getByRole('heading', { name: 'Your projects', exact: true }).waitFor();
  await page.getByRole('button', { name: 'New project', exact: true }).click();
  const dialog = page.getByRole('dialog', { name: 'Create a project', exact: true });
  await dialog.getByLabel('Project name', { exact: true }).fill('ssh-browser-app');
  const project = await clickResponse(dialog.getByRole('button', { name: 'Create project', exact: true }), '/v1/product/projects', 201);
  assert.equal(path.resolve(project.path), path.join(remote.projects, 'ssh-browser-app'));
  const purpose = 'A shared garden journal saved through a real SSH workspace.';
  const definition = await saveStructuredDefinition(project.id, purpose, 'Garden volunteers', 'The definition and canonical brief persist only in the remote workspace.');
  evidence.ownerDefinition = { projectId: project.id, revision: definition.revision, briefRevision: definition.briefRevision, digest: definition.digest, requirementIds: definition.requirements.map(item => item.id) };
  assert.equal((await api(remote, 'GET', `/v1/product/projects/${project.id}/specification`)).digest, definition.digest);
  assert.equal((await api(remote, 'GET', `/v1/product/projects/${project.id}/brief`)).purpose, purpose);
  assert.equal((await api(remote, 'GET', `/v1/product/projects/${project.id}/brief`)).revision, definition.briefRevision);
  assert.ok((await fs.readFile(path.join(project.path, 'LUCI.md'), 'utf8')).includes(purpose));
  assert.equal(JSON.parse(await fs.readFile(path.join(project.path, '.luci', 'specification.json'), 'utf8')).digest, definition.digest);
  await assert.rejects(fs.access(path.join(local.projects, 'ssh-browser-app')), { code: 'ENOENT' });
  await page.screenshot({ path: path.join(output, 'remote-saved-brief.png'), fullPage: true });
  pass('browser_to_ssh_to_remote_durable_project', 'The dedicated project dialog and structured Define UI create an approved revision2 definition with stable requirements and canonical brief1. Remote API, specification.json and LUCI.md agree; no equivalent project exists locally.');

  stage = 'selected-remote-admin-invitation';
  await page.getByRole('button', { name: 'Admin', exact: true }).click();
  const admin = page.getByRole('dialog', { name: 'Workspace administration', exact: true });
  await until(async () => (await admin.getByRole('combobox', { name: 'Control plane', exact: true }).inputValue()) === 'cloud', 'Admin did not select the active remote control plane');
  await admin.getByRole('button', { name: 'Manage invitations', exact: true }).click();
  await admin.getByRole('combobox', { name: 'Invitation role', exact: true }).selectOption('member');
  const invitation = await clickResponse(admin.getByRole('button', { name: 'Create invitation', exact: true }), '/v1/setup/workspace/admin/invitations', 201);
  assert(invitation.inviteCode && invitation.invitation.id);
  secrets.push(invitation.inviteCode);
  assert.equal(await admin.getByLabel('New invitation code', { exact: true }).innerText(), invitation.inviteCode);
  await admin.getByRole('button', { name: 'Hide invitation code', exact: true }).click();
  assert((await api(remote, 'GET', '/v1/admin/invitations')).invitations.some(item => item.id === invitation.invitation.id));
  assert(!(await api(local, 'GET', '/v1/admin/invitations')).invitations.some(item => item.id === invitation.invitation.id));
  await admin.getByRole('button', { name: 'Close', exact: true }).click();
  pass('remote_admin_invitation_boundary', 'The selected remote Admin UI issued a real single-use member invitation on the remote control plane, never in the local account realm.');

  stage = 'join-remote-workspace-with-invitation';
  await page.getByRole('button', { name: 'Settings', exact: true }).click();
  await page.getByRole('button', { name: 'Connection and account', exact: true }).click();
  await clickResponse(page.getByRole('button', { name: 'Disconnect remote sign-in', exact: true }), '/v1/setup/workspace/auth', 200, true, 'DELETE');
  await page.getByRole('button', { name: 'Join with an invitation', exact: true }).click();
  const joinedPassword = 'Invited-' + randomBytes(18).toString('hex'); secrets.push(joinedPassword);
  await page.getByLabel(/^Remote invitation code/).fill(invitation.inviteCode);
  await page.getByLabel('Remote display name', { exact: true }).fill('Invited Remote Member');
  await page.getByLabel('Remote username', { exact: true }).fill('invited-remote-member');
  await page.getByLabel('Remote password', { exact: true }).fill(joinedPassword);
  await page.getByLabel('Confirm remote password', { exact: true }).fill(joinedPassword);
  const joined = await clickResponse(page.getByRole('button', { name: 'Create workspace account', exact: true }), '/v1/setup/workspace/auth');
  assert.equal(joined.authenticated, true);
  assert.equal(joined.user.role, 'member');
  assert.notEqual(joined.user.id, remote.user.id);
  assert.equal((await api(local, 'GET', '/v1/auth/me')).user.id, local.user.id);
  await until(() => page.getByRole('button', { name: 'Use this cloud workspace', exact: true }).isEnabled(), 'Invited account did not enable remote activation');
  await switchWorkspace(page.getByRole('button', { name: 'Use this cloud workspace', exact: true }), '/v1/setup/workspace/activate');
  await page.getByRole('button', { name: 'Local Workspace Owner', exact: true }).waitFor();
  assert(!(await api(local, 'GET', '/v1/projects')).projects.some(item => item.id === project.id), 'New remote member must not inherit owner project access');
  assert.equal((await api(remote, 'GET', `/v1/product/projects/${project.id}/brief`)).purpose, purpose);
  const accepted = (await api(remote, 'GET', '/v1/admin/invitations')).invitations.find(item => item.id === invitation.invitation.id);
  assert.equal(accepted.acceptedBy, joined.user.id);
  assert(accepted.acceptedAt);
  pass('remote_invitation_account_and_isolation', 'WorkspaceSignIn redeemed the actual remote invitation through pinned SSH, selected a distinct member identity and preserved both the local identity and remote owner data.');

  stage = 'invited-member-first-project';
  const memberStatus = await api(local, 'GET', '/v1/setup/status');
  assert.equal(memberStatus.configured, true, 'The managed member must be ready in the server-assigned workspace');
  assert.equal(memberStatus.workspace.mode, 'cloud');
  assert.equal(memberStatus.access.user.username, 'invited-remote-member');
  assert.equal(memberStatus.access.user.role, 'member');
  assert.equal(memberStatus.access.manageBacking, false);
  assert.equal(memberStatus.access.projectsDirManaged, true);
  const assignedProjects = path.join(remote.state, 'control-plane', 'users', joined.user.id, 'runtime', 'projects');
  assert.equal(path.resolve(memberStatus.config.projectsDir), assignedProjects);
  await page.getByRole('button', { name: 'Settings', exact: true }).click();
  await page.getByRole('heading', { name: 'Your cloud workspace', exact: true }).waitFor();
  assert.equal(await page.getByRole('button', { name: 'On this computer', exact: true }).count(), 0);
  assert.equal(await page.getByRole('button', { name: 'AWS', exact: true }).count(), 0);
  assert.equal(await page.getByRole('button', { name: 'Azure', exact: true }).count(), 0);
  assert.equal(await page.getByRole('button', { name: 'Save settings', exact: true }).count(), 0, 'Local administrator identity must not expose remote member folder writes');
  await page.getByRole('button', { name: 'Create your first project', exact: true }).click();
  const memberDialog = page.getByRole('dialog', { name: 'Create a project', exact: true });
  await memberDialog.getByLabel('Project name', { exact: true }).fill('ssh-member-app');
  const memberProject = await clickResponse(memberDialog.getByRole('button', { name: 'Create project', exact: true }), '/v1/product/projects', 201);
  assert.equal(path.resolve(memberProject.path), path.join(assignedProjects, 'ssh-member-app'));
  const memberPurpose = 'An invited member creates and saves their own garden journal in the assigned cloud workspace.';
  const memberDefinition = await saveStructuredDefinition(memberProject.id, memberPurpose, 'This invited workspace member', 'The member can save and approve work without changing server settings or seeing another account project.');
  evidence.memberDefinition = { projectId: memberProject.id, revision: memberDefinition.revision, briefRevision: memberDefinition.briefRevision, digest: memberDefinition.digest, requirementIds: memberDefinition.requirements.map(item => item.id) };
  assert.equal((await api(local, 'GET', `/v1/product/projects/${memberProject.id}/specification`)).digest, memberDefinition.digest);
  assert.equal((await api(local, 'GET', `/v1/product/projects/${memberProject.id}/brief`)).purpose, memberPurpose);
  assert.equal((await api(local, 'GET', `/v1/product/projects/${memberProject.id}/brief`)).revision, memberDefinition.briefRevision);
  assert.ok((await fs.readFile(path.join(memberProject.path, 'LUCI.md'), 'utf8')).includes(memberPurpose));
  assert.equal(JSON.parse(await fs.readFile(path.join(memberProject.path, '.luci', 'specification.json'), 'utf8')).digest, memberDefinition.digest);
  assert(!(await api(local, 'GET', '/v1/projects')).projects.some(item => item.id === project.id));
  await api(local, 'GET', `/v1/product/projects/${project.id}/brief`, undefined, 404);
  await api(remote, 'GET', `/v1/product/projects/${memberProject.id}/brief`, undefined, 404);
  await api(local, 'GET', `/v1/product/projects/${project.id}/specification`, undefined, 404);
  await api(remote, 'GET', `/v1/product/projects/${memberProject.id}/specification`, undefined, 404);
  assert.equal((await api(remote, 'GET', `/v1/product/projects/${project.id}/brief`)).purpose, purpose);
  await assert.rejects(fs.access(path.join(local.projects, 'ssh-member-app')), { code: 'ENOENT' });
  await page.screenshot({ path: path.join(output, 'member-saved-brief.png'), fullPage: true });
  pass('remote_member_first_project', 'The invited member used the dedicated project dialog and structured Define UI to save and approve a project in the assigned per-user folder without server setup. Both canonical brief and full definition reject member/owner cross-account reads.');

  stage = 'return-to-original-local-workspace';
  await page.getByRole('button', { name: 'Settings', exact: true }).click();
  await page.getByRole('heading', { name: 'Your cloud workspace', exact: true }).waitFor();
  await switchWorkspace(page.getByRole('button', { name: 'Return to this machine', exact: true }), '/v1/setup/workspace/local');
  await page.getByRole('heading', { name: 'Choose your workspace', exact: true }).waitFor();
  assert.equal(await page.getByRole('button', { name: 'Close settings', exact: true }).count(), 0);
  await page.getByRole('button', { name: 'Continue to projects', exact: true }).click();
  await page.getByRole('heading', { name: 'Your projects', exact: true }).waitFor();
  await page.getByRole('treeitem').filter({ hasText: 'local-only-project' }).first().waitFor();
  const localProjects = (await api(local, 'GET', '/v1/projects')).projects;
  assert.ok(localProjects.some((item) => item.id === sentinel.id)); assert.ok(!localProjects.some((item) => item.id === project.id));
  assert.equal((await api(local, 'GET', '/v1/auth/me')).user.id, local.user.id);
  assert.equal((await api(remote, 'GET', `/v1/product/projects/${project.id}/brief`)).purpose, purpose);
  assert.equal((await api(remote, 'GET', `/v1/product/projects/${project.id}/specification`)).digest, definition.digest);
  assert.ok(!localProjects.some(item => item.id === memberProject.id));
  assert.ok((await fs.readFile(path.join(memberProject.path, 'LUCI.md'), 'utf8')).includes(memberPurpose));
  await page.screenshot({ path: path.join(output, 'returned-local-workspace.png'), fullPage: true });
  assert.deepEqual(errors, [], 'Browser raised uncaught JavaScript errors');
  pass('return_local_preserves_both_realms', 'The visible return control restores original local projects/account, leaves the saved remote project durable and produces no uncaught browser error.');
  evidence.status = 'passed';
} catch (error) {
  evidence.status = 'failed'; evidence.failure = { stage, message: sanitize(error.message) }; process.exitCode = 1;
  if (page) {
    try {
      await page.screenshot({ path: path.join(output, 'failure.png'), fullPage: true });
      await fs.writeFile(path.join(output, 'failure-visible-text.txt'), sanitize(await page.locator('body').innerText()).slice(0, 12000));
    } catch {}
  }
} finally {
  if (browser) await browser.close().catch(() => {});
  for (const service of [local, remote]) if (service?.token && !service.proc.exited) { try { await api(service, 'POST', '/v1/setup/shutdown', {}); } catch {} }
  await pause(500);
  for (const proc of processes.slice().reverse()) if (proc.child.pid) {
    // A process group created by this script may contain supervised workers/SSH.
    try { process.kill(-proc.child.pid, 'SIGTERM'); } catch {}
  }
  await pause(500);
  for (const proc of processes.slice().reverse()) if (proc.child.pid) { try { process.kill(-proc.child.pid, 'SIGKILL'); } catch {} }
  evidence.completedAt = new Date().toISOString();
  if (evidence.status !== 'passed') evidence.processDiagnostics = processes.map((proc) => ({ label: proc.label, exitCode: proc.code, tail: sanitize(proc.log).slice(-6000) }));
  await fs.mkdir(output, { recursive: true });
  await fs.writeFile(path.join(output, 'summary.json'), JSON.stringify(evidence, null, 2) + '\n');
  if (path.dirname(root) === os.homedir() && path.basename(root).startsWith('.luci-workspace-proof-')) await fs.rm(root, { recursive: true, force: true, maxRetries: 4, retryDelay: 100 });
}
console.log(JSON.stringify({ status: evidence.status, outcomes: evidence.outcomes.length, evidence: path.join(output, 'summary.json'), ...(evidence.failure ? { failure: evidence.failure } : {}) }));

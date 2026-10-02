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
const root = await fs.mkdtemp(path.join(os.tmpdir(), 'luci-workspace-proof-'));
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
async function clickResponse(button, resource, expected = 200) {
  const received = page.waitForResponse((response) => new URL(response.url()).pathname === resource && response.request().method() === 'POST');
  await button.click(); const response = await received;
  assert.equal(response.status(), expected, `Browser ${resource} returned${response.status()}`);
  return response.json();
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
  for (const filename of [key, hostKey]) await exec('ssh-keygen', ['-q', '-t', 'ed25519', '-N', '', '-f', filename]);
  const publicKey = (await fs.readFile(hostKey + '.pub', 'utf8')).trim().split(/\s+/)[1];
  const trustedFingerprint = 'SHA256:' + createHash('sha256').update(Buffer.from(publicKey, 'base64')).digest('base64').replace(/=+$/, '');
  const authorized = path.join(root, 'authorized_keys');
  await fs.copyFile(key + '.pub', authorized); await fs.chmod(authorized, 0o600);
  const sshPort = await freePort(), config = path.join(root, 'sshd_config');
  await fs.writeFile(config, [`Port ${sshPort}`, 'ListenAddress 127.0.0.1', `HostKey ${hostKey}`, `PidFile ${path.join(root, 'sshd.pid')}`, `AuthorizedKeysFile ${authorized}`, 'PubkeyAuthentication yes', 'PasswordAuthentication no', 'KbdInteractiveAuthentication no', 'UsePAM no', 'StrictModes yes', 'PermitRootLogin no', `AllowUsers ${username}`, 'AllowTcpForwarding local', 'PermitOpen 127.0.0.1:8730 127.0.0.1:8000', 'AllowAgentForwarding no', 'X11Forwarding no', 'PermitTunnel no', 'GatewayPorts no', 'LogLevel ERROR', ''].join('\n'), { mode: 0o600 });
  await exec(sshdBinary, ['-t', '-f', config]);
  const sshd = start(sshdBinary, ['-D', '-e', '-f', config], { PATH: process.env.PATH, HOME: os.homedir(), USER: username, LOGNAME: username, LANG: 'C.UTF-8' }, 'sshd');
  await until(async () => {
    if (sshd.exited) throw new Error(`Owned sshd exited: ${sshd.code}`);
    return new Promise((resolve) => { const socket = net.connect(sshPort, '127.0.0.1'); socket.setTimeout(500); socket.once('connect', () => { socket.destroy(); resolve(true); }); socket.once('error', () => resolve(false)); socket.once('timeout', () => { socket.destroy(); resolve(false); }); });
  }, 'Owned sshd did not listen');
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
  await page.getByLabel('Server fingerprint', { exact: true }).selectOption(trustedFingerprint);
  await page.getByLabel('I compared this fingerprint with my server’s trusted console.').check();
  const tunnel = await clickResponse(trust, '/v1/setup/ssh/connect', 202);
  await page.getByLabel('Remote username', { exact: true }).waitFor({ timeout: 45000 });
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
  await clickResponse(page.getByRole('button', { name: 'Use this cloud workspace' }), '/v1/setup/workspace/activate');
  await page.getByRole('button', { name: 'Local Workspace Owner', exact: true }).waitFor();
  assert.equal((await api(local, 'GET', '/v1/auth/me')).user.id, local.user.id);
  assert.equal((await api(local, 'GET', '/v1/setup/status')).workspace.mode, 'cloud');
  assert.ok(!(await api(local, 'GET', '/v1/projects')).projects.some((item) => item.id === sentinel.id));
  pass('separate_account_error_recovery_and_activation', 'Wrong remote password keeps the browser signed in locally and activation disabled; real remote sign-in succeeds on retry, then the selected cloud workspace replaces the project view while local identity stays intact.');

  stage = 'create-and-save-remote-project-in-browser';
  await page.getByRole('button', { name: 'Settings', exact: true }).click();
  const dialog = page.getByRole('dialog');
  await dialog.getByRole('button', { name: 'New project', exact: true }).click();
  await dialog.getByLabel('Project name', { exact: true }).fill('ssh-browser-app');
  const project = await clickResponse(dialog.getByRole('button', { name: 'Create project', exact: true }), '/v1/product/projects', 201);
  assert.equal(path.resolve(project.path), path.join(remote.projects, 'ssh-browser-app'));
  const purpose = 'A shared garden journal saved through a real SSH workspace.';
  await page.getByLabel('The idea', { exact: true }).fill(purpose);
  await page.getByLabel('Who it is for', { exact: true }).fill('Garden volunteers');
  await page.getByLabel('What good looks like').fill('The brief persists only in the remote workspace.');
  const brief = await clickResponse(page.getByRole('button', { name: 'Save brief', exact: true }), `/v1/product/projects/${project.id}/brief`);
  assert.equal(brief.revision, 1);
  assert.equal((await api(remote, 'GET', `/v1/product/projects/${project.id}/brief`)).purpose, purpose);
  assert.ok((await fs.readFile(path.join(project.path, 'LUCI.md'), 'utf8')).includes(purpose));
  await assert.rejects(fs.access(path.join(local.projects, 'ssh-browser-app')), { code: 'ENOENT' });
  await page.screenshot({ path: path.join(output, 'remote-saved-brief.png'), fullPage: true });
  pass('browser_to_ssh_to_remote_durable_project', 'Browser-created project and saved revision1 brief are verified through the remote service and actual remote LUCI.md; no equivalent project exists in local storage.');

  stage = 'return-to-original-local-workspace';
  await page.getByRole('button', { name: 'Settings', exact: true }).click();
  await clickResponse(page.getByRole('button', { name: 'Return to this machine', exact: true }), '/v1/setup/workspace/local');
  await page.getByRole('heading', { name: 'Choose your workspace', exact: true }).waitFor();
  await page.getByRole('button', { name: 'Close settings', exact: true }).click();
  await page.getByRole('treeitem').filter({ hasText: 'local-only-project' }).first().waitFor();
  const localProjects = (await api(local, 'GET', '/v1/projects')).projects;
  assert.ok(localProjects.some((item) => item.id === sentinel.id)); assert.ok(!localProjects.some((item) => item.id === project.id));
  assert.equal((await api(local, 'GET', '/v1/auth/me')).user.id, local.user.id);
  assert.equal((await api(remote, 'GET', `/v1/product/projects/${project.id}/brief`)).purpose, purpose);
  await page.screenshot({ path: path.join(output, 'returned-local-workspace.png'), fullPage: true });
  assert.deepEqual(errors, [], 'Browser raised uncaught JavaScript errors');
  pass('return_local_preserves_both_realms', 'The visible return control restores original local projects/account, leaves the saved remote project durable and produces no uncaught browser error.');
  evidence.status = 'passed';
} catch (error) {
  evidence.status = 'failed'; evidence.failure = { stage, message: sanitize(error.message) }; process.exitCode = 1;
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
  if (path.dirname(root) === os.tmpdir() && path.basename(root).startsWith('luci-workspace-proof-')) await fs.rm(root, { recursive: true, force: true, maxRetries: 4, retryDelay: 100 });
}
console.log(JSON.stringify({ status: evidence.status, outcomes: evidence.outcomes.length, evidence: path.join(output, 'summary.json'), ...(evidence.failure ? { failure: evidence.failure } : {}) }));

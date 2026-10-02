// Run the connected browser journey against a supplied compiled/shipped service.
// Fresh, private fixture directories are mandatory. Nothing uses a real profile.
// node scripts/run-ui-uat.mjs <binary> <web-directory> <runtime-directory> [evidence]
import assert from 'node:assert/strict';
import { spawn } from 'node:child_process';
import { randomUUID } from 'node:crypto';
import { mkdir, writeFile, readFile } from 'node:fs/promises';
import { createServer } from 'node:net';
import path from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '../desktop/node_modules/playwright/index.mjs';

const repo = path.resolve(import.meta.dirname, '..');
assert.ok(process.argv[2] && process.argv[3] && process.argv[4], 'Provide the binary, shipped web directory and runtime directory');
const binary = path.resolve(process.argv[2]);
const web = path.resolve(process.argv[3]);
const runtime = path.resolve(process.argv[4]);
const fixture = path.resolve(process.argv[5] || path.join(repo, 'artifacts/uat', randomUUID()));
assert.match(fixture.replaceAll('\\', '/'), /\/artifacts\/uat\/[^/]+$/);
await mkdir(path.dirname(fixture), { recursive: true });
await mkdir(fixture); // Refuse to reuse an identity or existing projects.
const state = path.join(fixture, 'state'), projects = path.join(fixture, 'projects'), evidence = path.join(fixture, 'evidence');
for (const folder of [state, projects, evidence]) await mkdir(folder, { mode: 0o700 });
const listener = createServer();
await new Promise((done, fail) => listener.once('error', fail).listen(0, '127.0.0.1', done));
const address = `127.0.0.1:${listener.address().port}`;
await new Promise(done => listener.close(done));
const base = `http://${address}`;
const lifecycle = randomUUID() + randomUUID();
const username = 'uat-owner', password = randomUUID() + randomUUID();
const inherited = Object.fromEntries(Object.entries(process.env).filter(([key]) => !key.startsWith('LUCI_')));
const environment = {
  ...inherited, LUCI_ADDR: address, LUCI_STATE_DIR: state, LUCI_PROJECTS_DIR: projects,
  LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: runtime, LUCI_BACKEND: 'docker',
  LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, OPENAI_API_KEY: '', ANTHROPIC_API_KEY: '',
  GIT_CONFIG_NOSYSTEM: '1', GIT_CONFIG_GLOBAL: path.join(fixture, 'empty-git-config'),
};
let service, spawnError;
const output = [];
const receipt = { passed: false, binary, web, runtime, startedAt: new Date().toISOString(), checks: [] };
async function start() {
  service = spawn(binary, [], { cwd: repo, env: environment, windowsHide: true });
  service.once('error', error => { spawnError = error; });
  for (const stream of [service.stdout, service.stderr]) stream.on('data', data => {
    if (output.join('').length < 256 * 1024) output.push(data.toString());
  });
  const deadline = Date.now() + 45000;
  while (Date.now() < deadline) {
    if (spawnError) throw spawnError;
    assert.equal(service.exitCode, null, 'The owned control service exited before readiness');
    try {
      const response = await fetch(base + '/v1/control-plane/status', { signal: AbortSignal.timeout(1000) });
      if (response.ok) {
        const control = await response.json();
        assert.equal(control.product, 'Luci');
        receipt.version = control.version;
        return;
      }
    } catch {}
    await delay(150);
  }
  throw new Error('The compiled control service did not become ready');
}
async function stop() {
  if (!service || service.exitCode !== null) return;
  const response = await fetch(base + '/v1/control-plane/shutdown', {
    method: 'POST', headers: { 'X-Luci-Lifecycle-Token': lifecycle }, signal: AbortSignal.timeout(5000),
  });
  assert.equal(response.status, 200, 'App-owned lifecycle shutdown was refused');
  const deadline = Date.now() + 15000;
  while (service.exitCode === null && Date.now() < deadline) await delay(100);
  assert.equal(service.exitCode, 0, 'Owned service failed to exit cleanly');
}
async function journey() {
  const child = spawn(process.execPath, ['scripts/verify-uat.mjs'], {
    cwd: path.join(repo, 'desktop'), windowsHide: true,
    env: { ...inherited, LUCI_EVAL_URL: base, LUCI_EVAL_CSP: '1', LUCI_UAT_FIXTURE: fixture, LUCI_UAT_EVIDENCE: evidence, LUCI_UAT_USERNAME: username, LUCI_UAT_PASSWORD: password },
    stdio: ['ignore', 'inherit', 'inherit'],
  });
  const code = await new Promise((done, fail) => { child.once('error', fail); child.once('close', done); });
  assert.equal(code, 0, 'The complete browser journey failed; inspect its outcome evidence');
}
try {
  await start();
  receipt.checks.push('Fresh isolated shipped service served its actual web assets');
  await journey();
  await stop();
  await start();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 900 } });
    const result = JSON.parse(await readFile(path.join(evidence, 'results.json'), 'utf8'));
    await page.goto(base);
    await page.getByLabel('Username', { exact: true }).fill(username);
    await page.getByLabel('Password', { exact: true }).fill(password);
    await page.getByRole('button', { name: 'Sign in', exact: true }).click();
    await page.getByRole('treeitem', { name: new RegExp(`${result.restoredName}$`) }).click();
    await page.getByRole('button', { name: 'Define', exact: true }).click();
    await page.getByLabel('The idea', { exact: true }).waitFor();
    assert.match(await page.getByLabel('The idea', { exact: true }).inputValue(), /garden journal/);
    await page.getByRole('button', { name: 'Develop', exact: true }).click();
    assert.equal(await page.getByLabel(/^Progress for task 1/).inputValue(), 'doing');
    await page.getByRole('button', { name: 'Design', exact: true }).click();
    await page.getByText(/Design 1 · Brief 1/).waitFor();
    await page.screenshot({ path: path.join(evidence, '11-process-restart.png'), fullPage: true });
    receipt.checks.push('Signed in after a real process restart; restored project, brief, design and task state recovered in the UI');
  } finally { await browser.close(); }
  await stop();
  receipt.checks.push('Both owned service instances exited with code zero through the app lifecycle API');
  receipt.passed = true;
} catch (error) {
  receipt.error = String(error);
  throw error;
} finally {
  try { await stop(); } catch (error) {
    receipt.cleanupError = String(error);
    if (service?.exitCode === null) service.kill();
  }
  await writeFile(path.join(evidence, 'service.log'), output.join(''), { mode: 0o600 });
  await writeFile(path.join(evidence, 'lifecycle.json'), JSON.stringify(receipt, null, 2));
}
console.log(JSON.stringify(receipt));

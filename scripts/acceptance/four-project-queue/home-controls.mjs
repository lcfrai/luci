#!/usr/bin/env node
// Installed UI + real local API queue controls. No containers, provider or cloud execution.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { createServer as httpServer } from 'node:http';
import { createServer } from 'node:net';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '../../../desktop/node_modules/playwright/index.mjs';

const here = dirname(fileURLToPath(import.meta.url)), repo = resolve(here, '../../..'), args = process.argv.slice(2);
const option = (name, fallback) => { const i = args.indexOf(name); if (i < 0) return fallback; assert(args[i + 1] && !args[i + 1].startsWith('--'), 'Missing ' + name); return args[i + 1]; };
if (args.includes('--help')) {
  console.log('node scripts/acceptance/four-project-queue/home-controls.mjs --package DIR --provenance JSON --expected-source SHA --report FRESH_JSON [--fixture-root /workspace/.s] [--chromium /usr/bin/chromium]\nLinux installed browser witness. API seeds labelled projects/reviews/runs, accounts, grants and paused policy. Real UI mutates real queue versions. Docker uses an absent private socket; a loopback provider trap must receive zero requests. No model, container, cloud or Windows acceptance claim.');
  process.exit(0);
}
assert.equal(process.platform, 'linux'); assert.notEqual(process.getuid(), 0);
const packageDir = realpathSync(resolve(option('--package'))), binary = join(packageDir, 'luci'), web = join(packageDir, 'web');
const provenancePath = resolve(option('--provenance')), expectedSource = option('--expected-source');
const provenance = JSON.parse(readFileSync(provenancePath, 'utf8'));
const integrity = JSON.parse(execFileSync('python3', [join(here, 'verify-package.py'), packageDir, provenancePath, expectedSource], { encoding: 'utf8', timeout: 60_000 }));
const report = resolve(option('--report')), evidence = dirname(report);
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Choose a fresh evidence directory');
mkdirSync(evidence, { recursive: true, mode: 0o700 });
const fixtureParent = resolve(option('--fixture-root', '/workspace/.s')); mkdirSync(fixtureParent, { recursive: true, mode: 0o700 });
const fixture = mkdtempSync(join(fixtureParent, 'hq-'));
for (const name of ['state', 'projects', 'home', 'tmp', 'docker']) mkdirSync(join(fixture, name), { mode: 0o700 });
const absentSocket = join(fixture, 'absent-docker.sock'); assert(!existsSync(absentSocket));
const password = randomUUID() + randomUUID(), lifecycle = randomUUID() + randomUUID(), providerKey = 'fixture-only-' + randomUUID();
const secrets = [password, lifecycle, providerKey];
const scrub = input => secrets.reduce((text, secret) => text.replaceAll(secret, '[redacted]'), String(input)).replaceAll(fixture, '[private-fixture]');
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const receipt = { passed: false, startedAt: new Date().toISOString(), sourceCommit: expectedSource, packageIntegrity: integrity,
  harnessSHA256: sha(readFileSync(fileURLToPath(import.meta.url))), verifierSHA256: sha(readFileSync(join(here, 'verify-package.py'))),
  scope: 'Actual installed Linux web app and local backend: four-project queue intervention, current permission checks, partial results and exact-run navigation. Separately labelled intercepted overview errors check packaged UI state handling only.',
  setupScaffolding: ['API-seeded accounts, team/project grants, four approved project contexts and unstarted runs', 'API-created queue entries and globally paused dispatch', 'A grant is explicitly revoked through the owner API after browser selection'],
  limitations: ['No agents are executed; this is not live-model, swarm execution, speed, Windows, installer or cloud acceptance.', 'Browser Chromium renders the installed package; this is not a native desktop WebView test.', 'Controlled overview HTTP409/403/500 responses do not establish actual server source-lock or permission-revocation behavior; actual queue requests still reach the backend.'],
  checks: [], apiScaffolding: [], browserMutations: [], queueTransitions: [], browserErrors: [], cleanup: [] };
writeFileSync(join(evidence, 'package-provenance.json'), JSON.stringify(provenance, null, 2) + '\n', { mode: 0o600, flag: 'wx' });
for (const name of ['home-controls.mjs', 'verify-package.py']) writeFileSync(join(evidence, name), readFileSync(join(here, name)), { mode: 0o600, flag: 'wx' });
let service, browser, provider, base, token, team, workspace, editor, page, stage = 'startup', providerRequests = 0, failure;
const owners = [], deadline = Date.now() + 10 * 60_000;
const check = (name, details = {}) => { receipt.checks.push({ name, ...details }); console.log('PASS ' + name); };
const ensure = () => { if (failure) throw failure; assert(Date.now() < deadline, 'Ten minute deadline exceeded'); if (service) assert.equal(service.exitCode, null, 'Private service stopped unexpectedly'); };
async function until(label, predicate, ms = 30_000) { const end = Date.now() + ms; while (Date.now() < end) { ensure(); const value = await predicate(); if (value) return value; await delay(100); } throw Error(label + ' timed out'); }
async function freePort() { const server = createServer(); await new Promise((done, reject) => server.once('error', reject).listen(0, '127.0.0.1', done)); const port = server.address().port; await new Promise(done => server.close(done)); return port; }
async function api(method, path, body, expected = 200, auth = token) {
  ensure(); const response = await fetch(base + path, { method, redirect: 'error', headers: { Origin: base, ...(auth ? { Authorization: 'Bearer ' + auth } : {}), ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) }, body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(30_000) });
  receipt.apiScaffolding.push({ stage, method, path, status: response.status });
  const value = await response.json(); assert.equal(response.status, expected, `${method} ${path}: ${response.status} ${scrub(value.message || value.error || '')}`); return value;
}
async function queues() { return Promise.all(owners.map(async owner => ({ owner, queue: await api('GET', owner.route + '/swarm-queue') }))); }
function processes() {
  const result = [];
  for (const pid of readdirSync('/proc').filter(name => /^\d+$/.test(name))) try {
    const env = readFileSync(`/proc/${pid}/environ`, 'utf8').split('\0');
    if (!env.includes('LUCI_HOME_QUEUE_FIXTURE=' + fixture)) continue;
    const raw = readFileSync(`/proc/${pid}/stat`, 'utf8'), fields = raw.slice(raw.lastIndexOf(')') + 2).trim().split(/\s+/);
    if (fields[0] !== 'Z') result.push({ pid: Number(pid), start: fields[19] });
  } catch (error) { if (!['ENOENT', 'ESRCH', 'EACCES', 'EPERM'].includes(error.code)) throw error; }
  return result;
}
async function stopOwned() {
  if (browser) await browser.close();
  if (service?.exitCode === null) service.kill('SIGTERM');
  const end = Date.now() + 20_000; while (Date.now() < end && processes().length) await delay(100);
  const remaining = processes();
  if (remaining.length) { receipt.cleanup.push('Owned processes required force termination'); receipt.passed = false; for (const identity of remaining) if (processes().some(row => row.pid === identity.pid && row.start === identity.start)) process.kill(identity.pid, 'SIGKILL'); await delay(250); }
  assert.equal(processes().length, 0, 'Private processes remain');
  receipt.cleanup.push('No private service, worker or browser processes remain');
}
async function openHome() {
  await page.goto(base);
  const username = page.getByLabel('Username', { exact: true }), enter = page.getByRole('button', { name: 'Continue to projects', exact: true }), picker = page.getByLabel('Workspace', { exact: true });
  await until('Account entry', async () => await username.isVisible() || await enter.isVisible() || await picker.isVisible());
  if (await username.isVisible()) { await username.fill('queueeditor'); await page.getByLabel('Password', { exact: true }).fill(password); await page.getByRole('button', { name: 'Sign in', exact: true }).click(); }
  await until('Workspace entry', async () => await enter.isVisible() || await picker.isVisible()); if (await enter.isVisible()) await enter.click();
  await picker.selectOption(team.id); await page.getByRole('button', { name: 'All projects', exact: true }).click();
  await page.getByRole('heading', { name: 'Across your projects', exact: true }).waitFor();
  await until('Four project cards', async () => await page.locator('li.overview-card').count() === 4);
}
const card = owner => page.locator('li.overview-card').filter({ has: page.getByRole('button', { name: 'Open project ' + owner.project.name, exact: true }) });
async function selectRows(selected) {
  for (const owner of selected) {
    const block = card(owner), manage = block.getByRole('button', { name: 'Manage queue', exact: true }); if (await manage.isVisible()) await manage.click();
    await block.getByRole('checkbox', { name: `Select ${owner.task.title} in ${owner.project.name}`, exact: true }).check();
  }
}
async function bulk(action, selected, button, expectedState, priority) {
  stage = 'browser-bulk-' + action; const before = await queues(); await selectRows(selected);
  if (priority) await page.getByRole('combobox', { name: 'Selected task priority', exact: true }).selectOption(priority);
  await page.getByRole('button', { name: button, exact: true }).click();
  if (action === 'cancel') { const dialog = page.getByRole('dialog', { name: 'Cancel selected waiting tasks?', exact: true }); await dialog.getByText(selected[0].project.name + ' · ' + selected[0].task.title, { exact: true }).waitFor(); await dialog.getByRole('button', { name: `Cancel ${selected.length} waiting task${selected.length === 1 ? '' : 's'}`, exact: true }).click(); }
  await page.getByText(`${selected.length} changed · 0 failed · 0 unconfirmed · 0 skipped`, { exact: true }).waitFor();
  const after = await queues();
  for (const owner of selected) { const a = before.find(row => row.owner === owner).queue.items[0], b = after.find(row => row.owner === owner).queue.items[0]; assert.equal(b.id, a.id); assert(b.revision > a.revision); assert.equal(b.state, expectedState); if (priority) assert.equal(b.priority, priority); receipt.queueTransitions.push({ action, projectId: owner.project.id, itemId: b.id, beforeRevision: a.revision, afterRevision: b.revision, state: b.state, priority: b.priority }); }
  check('Real browser ' + action + ' changes actual queue versions', { projects: selected.length });
}
// This section deliberately injects only one project's overview GET failures.
// It proves packaged UI handling of those responses, not a real source lock.
// Queue reads/mutations and permission reads continue to the installed backend.
async function controlledOverviewWitness(owner) {
  stage = 'controlled-overview-real-baseline';
  const endpoint = owner.route + '/overview';
  const matches = url => url.origin === base && url.pathname === endpoint;
  const realRead = () => page.waitForResponse(response => matches(new URL(response.url())) && response.request().method() === 'GET' && response.status() === 200);
  const baselineRead = realRead();
  await page.getByRole('button', { name: 'All projects', exact: true }).click();
  const baseline = await (await baselineRead).json();
  assert.equal(baseline.projectId, owner.project.id); assert(baseline.goal?.text); assert(baseline.phase?.label);
  const block = card(owner), refresh = page.getByRole('button', { name: 'Refresh overview', exact: true });
  await block.getByText(baseline.goal.text, { exact: true }).waitFor();
  await block.getByRole('button', { name: baseline.nextAction.label, exact: true }).waitFor();
  const controlled = receipt.controlledOverviewResponses = {
    scope: 'Packaged UI state handling with intercepted overview GET responses; not a server source-lock witness.',
    actualBaseline: { projectId: owner.project.id, observedAt: baseline.observedAt, goal: baseline.goal.text, phase: baseline.phase.label, nextAction: baseline.nextAction.label },
    serverLockWitness: false, queueRequestsIntercepted: false, intercepted: [], checks: [],
  };
  let injected = { status: 409, error: 'branch_busy', message: 'Controlled fixture: source preparation is busy.' };
  const handler = async route => {
    assert(matches(new URL(route.request().url())));
    if (route.request().method() !== 'GET') { await route.fallback(); return; }
    controlled.intercepted.push({ stage, method: 'GET', path: endpoint, status: injected.status, error: injected.error });
    await route.fulfill({ status: injected.status, contentType: 'application/json', body: JSON.stringify({ error: injected.error, message: injected.message }) });
  };
  const assertCleared = async message => {
    await block.getByText('Summary unavailable. ' + message, { exact: true }).waitFor();
    assert.equal(await block.locator('.overview-goal').count(), 0);
    assert.equal(await block.getByText('Last recorded: ' + baseline.phase.label, { exact: true }).count(), 0);
    assert.equal(await block.getByRole('button', { name: baseline.nextAction.label, exact: true }).count(), 0);
    assert.equal(await block.getByLabel('Recorded swarm queue activity', { exact: true }).count(), 0);
    assert.equal(await block.locator('.overview-details, .overview-questions').count(), 0);
    assert.equal(await block.getByText(/(?:Latest|Last recorded) run:/).count(), 0);
  };
  await page.route(matches, handler);
  try {
    stage = 'controlled-overview-409'; await refresh.click();
    await block.getByText('Last recorded: ' + baseline.phase.label, { exact: true }).waitFor();
    await block.getByText(baseline.goal.text, { exact: true }).waitFor();
    await block.getByText('Source is being prepared; showing last recorded context. Queue controls remain available. Activity counts are recorded, not live.', { exact: true }).waitFor();
    assert.equal(await block.getByRole('button', { name: baseline.nextAction.label, exact: true }).count(), 0);
    assert(await block.getByRole('button', { name: 'Manage queue', exact: true }).isEnabled());
    controlled.checks.push('HTTP409 branch_busy retains the actual previously read goal and stage, labels counts as recorded, and removes the stale next action.');
    // The queue request is not intercepted: its real durable revision must change.
    const transitionsBefore = receipt.queueTransitions.length;
    await bulk('priority', [owner], 'Set priority', 'paused', 'normal');
    controlled.realQueueTransition = receipt.queueTransitions[transitionsBefore];
    assert.equal(controlled.realQueueTransition.projectId, owner.project.id);
    await block.getByText('Last recorded: ' + baseline.phase.label, { exact: true }).waitFor();
    assert.equal(await block.getByRole('button', { name: baseline.nextAction.label, exact: true }).count(), 0);
    controlled.checks.push('While overview HTTP409 persists, a real browser priority control changes the actual backend queue revision without starting work.');
    await page.setViewportSize({ width: 390, height: 844 });
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth));
    await page.screenshot({ path: join(evidence, 'controlled-overview-busy-phone.png'), fullPage: true });
    await page.setViewportSize({ width: 1440, height: 1080 });
    stage = 'controlled-overview-403'; injected = { status: 403, error: 'forbidden', message: 'Controlled fixture: project summary access denied.' };
    await refresh.click(); await assertCleared(injected.message);
    controlled.checks.push('A confirmed HTTP403 response clears the retained goal, stage, counters, context details and next action.');
    await page.screenshot({ path: join(evidence, 'controlled-overview-denied-desktop.png'), fullPage: true });
  } finally { await page.unroute(matches, handler); }
  stage = 'controlled-overview-real-recovery';
  const recoveryRead = realRead(); await refresh.click(); await recoveryRead;
  await block.getByText(baseline.goal.text, { exact: true }).waitFor();
  await block.getByRole('button', { name: baseline.nextAction.label, exact: true }).waitFor();
  controlled.checks.push('A fresh successful backend overview read restores current context and its next action.');
  injected = { status: 500, error: 'internal', message: 'Controlled fixture: summary read failed.' };
  await page.route(matches, handler);
  try {
    stage = 'controlled-overview-500'; await refresh.click(); await assertCleared(injected.message);
    controlled.checks.push('A confirmed HTTP500 response also clears private summary data rather than retaining it as current.');
  } finally { await page.unroute(matches, handler); }
  stage = 'controlled-overview-final-recovery';
  const finalRead = realRead(); await refresh.click(); await finalRead;
  await block.getByText(baseline.goal.text, { exact: true }).waitFor();
  assert(controlled.intercepted.some(item => item.status === 409));
  assert(controlled.intercepted.some(item => item.status === 403));
  assert(controlled.intercepted.some(item => item.status === 500));
  check('Controlled overview error witness: labelled retained context, real queue control, private-data clearing and recovery', { scope: controlled.scope });
}
async function grant(owner, execute) {
  const route = `/v1/teams/${team.id}/projects/${owner.project.id}/members`, access = await api('GET', route);
  await api('PUT', route + '/' + editor.id, { expectedRevision: access.team.revision, role: 'editor', grants: { execute, secrets: false, deploy: false, admin: false } });
}
try {
  provider = httpServer((_request, response) => { providerRequests++; response.writeHead(503); response.end('Acceptance forbids model execution'); failure = Error('Unexpected provider request'); });
  await new Promise(done => provider.listen(0, '127.0.0.1', done));
  const port = await freePort(), preview = await freePort(); base = 'http://127.0.0.1:' + port;
  const env = Object.fromEntries(['PATH', 'LANG', 'LC_ALL', 'TZ'].filter(name => process.env[name]).map(name => [name, process.env[name]]));
  Object.assign(env, { HOME: join(fixture, 'home'), TMPDIR: join(fixture, 'tmp'), DOCKER_CONFIG: join(fixture, 'docker'), DOCKER_HOST: 'unix://' + absentSocket, LUCI_DOCKER_HOST: 'unix://' + absentSocket,
    LUCI_HOME_QUEUE_FIXTURE: fixture, LUCI_ADDR: '127.0.0.1:' + port, LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'), LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: join(packageDir, 'runtime'), LUCI_BACKEND: 'docker', LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_LOG_LEVEL: 'error', LUCI_BROWSER_PREVIEW_ADDR: '127.0.0.1:' + preview, LUCI_BROWSER_PREVIEW_PORT: String(preview), GIT_CONFIG_GLOBAL: join(fixture, 'home/.gitconfig'), GIT_CONFIG_NOSYSTEM: '1' });
  service = spawn(binary, [], { cwd: fixture, env, stdio: 'ignore' }); service.on('error', error => { failure = error; });
  await until('Installed service startup', async () => { try { const response = await fetch(base + '/v1/control-plane/status', { signal: AbortSignal.timeout(1000) }); if (!response.ok) return false; const value = await response.json(); return value.product === 'Luci' && value.version === '0.5.0'; } catch { return false; } }, 45_000);
  const bootstrap = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(); secrets.push(bootstrap);
  token = (await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'queueowner', displayName: 'Queue fixture owner', password }, 201)).token; secrets.push(token);
  editor = await api('POST', '/v1/admin/users', { username: 'queueeditor', displayName: 'Queue fixture editor', password }, 201);
  team = (await api('POST', '/v1/teams', { name: 'Queue control acceptance fixture' }, 201)).team; workspace = `/v1/teams/${team.id}/workspace`;
  // Project grants narrow team authority: base editor/execute is required so
  // each project can independently withhold or later grant execution.
  await api('POST', `/v1/teams/${team.id}/members`, { expectedRevision: team.revision, username: 'queueeditor', role: 'editor', grants: { execute: true, secrets: false, deploy: false, admin: false } });
  const settings = await api('GET', '/v1/product/swarm-scheduler'); await api('PUT', '/v1/product/swarm-scheduler', { expectedRevision: settings.revision, policy: settings.policy, dispatchPaused: true });
  await api('POST', workspace + '/v1/setup/git/identity', { name: 'Queue fixture', email: 'queue@example.invalid' });
  const connection = (await api('POST', workspace + '/v1/setup/agent-connections', { action: 'save', expectedRevision: 0, name: 'Never execute fixture', kind: 'compatible-api', enabled: true, endpoints: { responses: `http://127.0.0.1:${provider.address().port}/v1` }, models: { codex: 'queue-no-execution' }, apiKey: providerKey })).connections[0];
  for (const letter of 'abcd') {
    stage = 'api-fixture-project-' + letter;
    const project = await api('POST', workspace + '/v1/product/projects', { name: 'fixture-queue-' + letter }, 201), route = workspace + '/v1/product/projects/' + project.id;
    assert(realpathSync(project.path).startsWith(fixture + '/'));
    await api('POST', route + '/brief', { revision: 0, purpose: 'Fixture ' + letter.toUpperCase() + ': review waiting work across projects without starting agents.', audience: 'Local operators', constraints: 'Acceptance fixture only. No model, container or cloud execution.', criteria: ['Queue interventions change the exact selected work with clear receipts.'] });
    let spec = await api('GET', route + '/specification'); spec = await api('POST', route + '/specification', { expectedRevision: spec.revision, expectedBriefRevision: spec.briefRevision, specification: { ...spec, requirements: [{ id: 'AC-queue', title: 'Review queue changes', priority: 'must', acceptance: 'Only explicitly selected queued tasks change state and all results remain inspectable.' }] } });
    spec = await api('POST', route + '/specification/approve', { revision: spec.revision, briefRevision: spec.briefRevision });
    let design = await api('GET', route + '/design-plan'); design = await api('POST', route + '/design-plan', { expectedRevision: design.revision, expectedBriefRevision: spec.briefRevision, expectedSpecificationRevision: spec.revision, plan: { ...design, screens: [{ id: 'queue', name: 'Queue', route: '/', purpose: 'Inspect waiting work', states: ['Queued', 'Paused', 'Cancelled'], widths: [390, 1440], requirementIds: ['AC-queue'] }] } });
    design = await api('POST', route + '/design-plan/approve', { revision: design.revision });
    const task = { id: 'work-' + letter, title: 'Fixture work ' + letter.toUpperCase(), status: 'todo', requirementIds: ['AC-queue'], writePaths: ['src/' + letter + '.ts'], acceptance: 'Record explicit queue intervention; never execute this fixture task.' };
    const plan = await api('POST', route + '/plan', { expectedRevision: 0, expectedSpecificationRevision: spec.revision, tasks: [task] });
    const review = await api('GET', route + '/git'); await api('POST', route + '/git/commit', { message: 'Fixture seed: reviewed queue input', expectedReviewDigest: review.reviewDigest });
    const branch = (await api('GET', workspace + '/v1/projects/' + project.id + '/branches')).branches[0];
    const worktree = (await api('GET', workspace + '/v1/projects/' + project.id + '/worktrees?branchId=' + encodeURIComponent(branch.id))).worktrees.find(item => item.isPrimary);
    assert(worktree);
    // Explicit fixture-only unattended permission. Dispatch remains globally
    // paused and Docker is absent; this never changes a user's saved route.
    const input = { worktreeId: worktree.id, harness: 'codex', model: 'queue-no-execution', permissionMode: 'bypass', agentConnectionId: connection.id, outcome: 'preview', implementationContext: { projectId: project.id, briefRevision: spec.briefRevision, specificationRevision: spec.revision, designPlanRevision: design.revision, planRevision: plan.revision, taskIds: [task.id] } };
    let older;
    if (letter === 'd') { older = (await api('POST', route + '/runs', { ...input, idempotencyKey: 'installed-home-fixture-older' }, 201)).run; older = (await api('POST', route + '/runs/' + older.id + '/cancel', { expectedRevision: older.revision })).run; }
    let run = (await api('POST', route + '/runs', { ...input, idempotencyKey: 'installed-home-fixture-current' }, 201)).run;
    run = (await api('POST', route + '/runs/' + run.id + '/swarm', { expectedRevision: run.revision, maxParallel: 1 }, 201)).run;
    const queued = await api('POST', route + '/runs/' + run.id + '/swarm/queue', { expectedRevision: run.revision, taskIds: [task.id], idempotencyKey: 'installed-home-fixture-queue', priority: 'normal' }, 201);
    assert.equal(queued.queue.dispatchPaused, true); assert.equal(queued.items[0].state, 'queued');
    const owner = { project, route, task, run, older }; owners.push(owner); await grant(owner, false);
  }
  const editorToken = (await api('POST', '/v1/auth/login', { username: 'queueeditor', password })).token; secrets.push(editorToken);
  for (const owner of owners) {
    const access = await api('GET', `/v1/teams/${team.id}/projects/${owner.project.id}/members`, undefined, 200, editorToken);
    assert.equal(access.permissions.edit, true); assert.equal(access.permissions.execute, false);
  }
  check('Four reviewed fixture projects queued through labelled API scaffolding; editor can edit but cannot execute in any project; dispatch remains globally paused');
  browser = await chromium.launch({ headless: true, executablePath: option('--chromium', '/usr/bin/chromium'), env: { PATH: process.env.PATH, HOME: join(fixture, 'home'), TMPDIR: join(fixture, 'tmp'), LUCI_HOME_QUEUE_FIXTURE: fixture }, args: ['--no-proxy-server'] });
  receipt.browserVersion = browser.version(); const context = await browser.newContext({ viewport: { width: 1440, height: 1080 }, serviceWorkers: 'block' });
  await context.route('**/*', async route => { const request = route.request(), url = new URL(request.url()); if (url.origin !== base) { failure = Error('Browser external request refused: ' + url.origin); await route.abort('blockedbyclient'); return; } if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method())) receipt.browserMutations.push({ stage, method: request.method(), path: url.pathname }); await route.continue(); });
  page = await context.newPage(); page.setDefaultTimeout(20_000); page.on('pageerror', error => { receipt.browserErrors.push(scrub(error)); failure = Error('Browser script error'); });
  await openHome(); await bulk('pause', owners, 'Pause waiting', 'paused'); await bulk('priority', owners, 'Set priority', 'paused', 'high'); await bulk('cancel', [owners[3]], 'Cancel waiting…', 'cancelled');
  stage = 'api-grant-before-resume'; await grant(owners[0], true); await grant(owners[1], true); await openHome(); await selectRows(owners.slice(0, 2));
  const beforeResume = await queues(); stage = 'api-revoke-after-browser-selection'; await grant(owners[1], false);
  stage = 'browser-partial-resume'; await page.getByRole('button', { name: 'Resume paused', exact: true }).click(); await page.getByText('1 changed · 1 failed · 0 unconfirmed · 0 skipped', { exact: true }).waitFor();
  await page.getByText('Failed: Execution access is required to resume this project’s queued work.', { exact: true }).waitFor();
  const afterResume = await queues(); assert.equal(afterResume[0].queue.items[0].state, 'queued'); assert(afterResume[0].queue.items[0].revision > beforeResume[0].queue.items[0].revision); assert.deepEqual(afterResume[1].queue.items, beforeResume[1].queue.items);
  const resumeMutations = receipt.browserMutations.filter(item => item.stage === stage && item.path.endsWith('/resume')); assert.equal(resumeMutations.length, 1); assert(resumeMutations[0].path.includes(owners[0].project.id));
  check('Fresh permission revocation yields one actual resume and one visible failure, without replay or mutation of revoked work');
  await page.screenshot({ path: join(evidence, 'bulk-partial-desktop.png'), fullPage: true });
  await page.setViewportSize({ width: 390, height: 844 }); assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)); await page.screenshot({ path: join(evidence, 'bulk-partial-phone.png'), fullPage: true });
  await page.setViewportSize({ width: 1440, height: 1080 });
  // The cancelled queue has no live queue attention overriding its latest-run
  // action. Other projects intentionally keep their queue-focused next step.
  stage = 'browser-exact-run'; const first = owners[3]; await card(first).getByRole('button', { name: 'Open project', exact: true }).click(); await page.getByRole('button', { name: 'Develop', exact: true }).click();
  const history = page.locator('.project-run-history'); await history.locator('summary').click(); const old = history.getByRole('button', { name: /^Cancelled/ }); await old.click(); assert.equal(await old.getAttribute('aria-pressed'), 'true');
  await page.getByRole('button', { name: 'All projects', exact: true }).click(); const overview = await api('GET', first.route + '/overview'); assert.equal(overview.nextAction.runId, first.run.id);
  const exactRead = page.waitForResponse(response => new URL(response.url()).pathname === first.route + '/runs/' + first.run.id && response.request().method() === 'GET');
  await card(first).getByRole('button', { name: overview.nextAction.label, exact: true }).click(); const response = await exactRead; assert.equal(response.status(), 200); assert.equal((await response.json()).run.id, first.run.id);
  await page.getByRole('region', { name: 'Agent team', exact: true }).waitFor();
  check('Overview navigation reads and opens the exact latest run after a different historical run was selected', { inspectedRunId: first.run.id, previousRunId: first.older.id });
  await page.screenshot({ path: join(evidence, 'exact-run-desktop.png'), fullPage: true });
  await controlledOverviewWitness(owners[2]);
  stage = 'final-evidence'; const final = await queues();
  for (const { owner, queue } of final) { assert.equal(queue.dispatchPaused, true); assert(queue.items.every(item => !item.operationId && !item.childId)); assert.equal((await api('GET', owner.route + '/runs/' + owner.run.id)).run.operations.length, 0); }
  assert.equal((await api('GET', '/v1/product/swarm-scheduler')).dispatchPaused, true); assert.equal(providerRequests, 0); assert(!existsSync(absentSocket)); assert.equal(receipt.browserErrors.length, 0);
  receipt.finalQueues = final.map(({ owner, queue }) => ({ projectId: owner.project.id, items: queue.items, dispatchPaused: queue.dispatchPaused }));
  receipt.noExecution = { absentDockerEndpoint: true, providerTrapRequests: providerRequests, childIds: 0, reservedOperations: 0, dispatchPaused: true };
  receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error.stack || error).slice(0, 8000); process.exitCode = 1;
  if (page) try { await page.screenshot({ path: join(evidence, 'failed-stage.png'), fullPage: true }); } catch {}
} finally {
  receipt.stage = stage;
  try { await stopOwned(); } catch (error) { receipt.cleanup.push(scrub(error)); receipt.passed = false; process.exitCode = 1; }
  if (provider) { provider.closeAllConnections(); await new Promise(done => provider.close(done)); }
  receipt.providerTrapRequests = providerRequests;
  if (failure) { receipt.backgroundFailure = scrub(failure); receipt.passed = false; process.exitCode = 1; }
  try { assert(!existsSync(absentSocket)); assert.equal(sha(readFileSync(binary)), provenance.binarySHA256); assert.equal(sha(readFileSync(join(web, 'index.html'))), provenance.webIndexSHA256); assert.equal(realpathSync(fixture), fixture); assert.equal(dirname(fixture), fixtureParent); if (!processes().length) { rmSync(fixture, { recursive: true }); receipt.fixtureRemoved = true; } } catch (error) { receipt.cleanup.push(scrub(error)); receipt.passed = false; process.exitCode = 1; }
  receipt.finishedAt = new Date().toISOString(); writeFileSync(report, JSON.stringify(receipt, null, 2) + '\n', { mode: 0o600, flag: 'wx' }); console.log((receipt.passed ? 'PASS ' : 'FAIL ') + report);
}

#!/usr/bin/env node
// Installed overview handoff, actual plan amendment and occupied composer choices.
// Seeded historical check/question records are setup only; no execution is claimed.
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
  console.log('node scripts/overview-composer-browser-uat.mjs --binary PATH --web PATH --chromium PATH --provenance PATH --report PATH\nInstalled exact-run handoff, actual plan-only amendment, and occupied message Keep/Replace at desktop/phone. Explicitly labelled private historical check/question scaffolding, written only while the owned service is stopped; a real supported check join changes the mounted run. No check/agent/model execution, Docker, remote publication or fabricated HTTP responses. Fresh report directory required. 180-second runtime budget plus bounded owned cleanup.');
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
const fixture = mkdtempSync(join(tmpdir(), 'luci-overview-composer-'));
for (const name of ['home', 'projects', 'state']) mkdirSync(join(fixture, name), { mode: 0o700 });
const secrets = new Set(), secret = () => { const value = randomBytes(32).toString('base64url'); secrets.add(value); return value; };
const password = secret(), lifecycle = secret(), scrub = error => { let text = String(error?.message ?? error); for (const value of secrets) text = text.replaceAll(value, '[redacted]'); return text; };
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit,
  package: { archiveSHA256: provenance.archiveSHA256, binarySHA256, webIndexSHA256, allWebFilesMatchArchive: true, webFileCount: Object.keys(webFiles).length },
  driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  scope: 'Installed exact-run handoff, real plan-only amendment, historical-scope clarity and unsent composer choice preservation at 1440 and 390 pixels.',
  setupScaffolding: ['Supported private owner APIs save and approve definition/design/plan A, configure a deny-only named connection, commit local context and prepare one real run.', 'Only while every owned process is stopped, the private fixture run is marked awaiting-checks and one historical passed-check record plus two nonblocking question turns are seeded. No check or inference occurred.', 'After the browser caches awaiting-checks A, the real authenticated join-check API validates and attaches that seeded historical record. No new command is started.'],
  limitations: ['This is not proof of model quality, passed application checks, deployment, acceptance or public hosting.', 'Historical status/command outputs and questions are explicitly labelled setup; subsequent GETs, check join, plan save, freshness and browser draft actions are real.', 'No provider endpoint, Docker socket or external publication is used. A local deny-only fixture records any attempted provider/external request.'],
  checks: [], requests: [], setupRequests: [], browserMutations: [], browserErrors: [], blockedRequests: [], providerAttempts: [], cleanup: [], geometry: [],
  dockerAllocations: 0, agentOrCheckExecutions: 0, runtimeBudgetMS: 180000,
};
const inherited = Object.fromEntries(Object.entries(process.env).filter(([name]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ)$/.test(name)));
let service, browser, browserClosing, page, base, token, stage = 'setup', project, projectRoute, spawnError, serviceLog = '', routeError, denyProxy;
const running = new AbortController();
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
  receipt.setupRequests.push({ stage, method, path: route });
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
const taskA = 'Save a private garden observation for its author';
const taskB = 'Save a private garden observation with an editable observation date';
const firstQuestion = 'Which observation details should the first screen request?';
const secondQuestion = 'How should the journal order saved observations?';
const suggestion = 'Show newest observations first, with the recorded date visible.';
const panel = () => page.getByRole('complementary', { name: 'Project understanding', exact: true });
const overview = () => panel().getByRole('region', { name: 'Current project understanding', exact: true });
const nextStep = () => overview().getByRole('region', { name: 'Next step', exact: true });
const runs = () => page.getByRole('region', { name: 'Project runs', exact: true });
const composer = () => panel().getByLabel('Message Luci', { exact: true });
async function openOverview() {
  const show = panel().getByRole('button', { name: 'Show project overview', exact: true });
  if (await show.isVisible()) await show.click();
  await overview().waitFor();
}
async function openDetails(details) { if (!await details.evaluate(element => element.open)) await details.locator(':scope > summary').click(); }
async function inspectRun() {
  await runs().waitFor();
  const details = runs().locator('details.project-run-receipts'); await openDetails(details);
  await details.getByText(taskA, { exact: true }).waitFor();
  assert.equal(await details.getByText(taskB, { exact: true }).count(), 0, 'Historical run scope was replaced by current task text');
  return details;
}
async function sample(name, target) {
  await target.scrollIntoViewIfNeeded();
  await screenshot(name); // Keep the rendered evidence even if the geometry fails.
  const shape = await target.evaluate(element => {
    const rect = element.getBoundingClientRect(), x = Math.max(0, Math.min(innerWidth - 1, rect.x + rect.width / 2)), y = Math.max(0, Math.min(innerHeight - 1, rect.y + rect.height / 2));
    const hit = document.elementFromPoint(x, y);
    return { x: rect.x, y: rect.y, width: rect.width, height: rect.height, viewport: [innerWidth, innerHeight], scrollWidth: document.documentElement.scrollWidth, hit: !!hit && (element.contains(hit) || hit.contains(element)) };
  });
  receipt.geometry.push({ name, ...shape });
  assert(shape.width > 20 && shape.height > 16 && shape.x >= -1 && shape.x + shape.width <= shape.viewport[0] + 1, 'Target does not fit the viewport: ' + name);
  assert(shape.y >= -1 && shape.y + shape.height <= shape.viewport[1] + 1 && shape.hit, 'Target is not reachable: ' + name);
  assert(shape.scrollWidth <= shape.viewport[0] + 1, 'Horizontal page overflow: ' + name);
}
function ownedRunFile(runID) {
  const matches = []; let entries = 0;
  function visit(directory) { for (const name of readdirSync(directory)) {
    assert(++entries < 10000, 'Unexpectedly large owned fixture state');
    const path = join(directory, name), info = lstatSync(path);
    assert(!info.isSymbolicLink(), 'Refuse linked private fixture state');
    if (info.isDirectory()) visit(path);
    else if (path.endsWith(`/project-runs/${project.id}/${runID}.json`)) { assert(info.isFile() && info.nlink === 1); matches.push(path); }
  } }
  visit(join(fixture, 'state')); assert.equal(matches.length, 1, 'Expected one owned API-created run'); return matches[0];
}
async function browserDraft(message, replyTo) {
  const rows = await page.evaluate(() => Object.keys(localStorage).filter(key => key.startsWith('luci.orchestrator-composer.v1:')).map(key => ({ key, value: JSON.parse(localStorage.getItem(key)) })));
  assert.equal(rows.filter(row => row.key.includes(':send:')).length, 0, 'A choice reserved an assistant request');
  const drafts = rows.filter(row => row.value.message !== undefined);
  assert.equal(drafts.length, 1); assert.equal(drafts[0].value.message, message); assert.equal(drafts[0].value.replyTo, replyTo);
  assert(decodeURIComponent(drafts[0].key).includes(project.id), 'Draft lacks project scope');
  return drafts[0];
}
try {
  const controlPort = await port(), preview = await port(); base = `http://127.0.0.1:${controlPort}`;
  denyProxy = createHTTP((request, response) => { receipt.providerAttempts.push({ method: request.method, destination: request.url }); response.writeHead(502); response.end('No provider or external operation is permitted in this fixture'); });
  denyProxy.on('connect', (request, socket) => { receipt.providerAttempts.push({ method: 'CONNECT', destination: request.url }); socket.end('HTTP/1.1 502 Forbidden\r\nConnection: close\r\n\r\n'); });
  await new Promise(done => denyProxy.listen(0, '127.0.0.1', done)); const outbound = `http://127.0.0.1:${denyProxy.address().port}`;
  const environment = { ...inherited, HOME: join(fixture, 'home'), USERPROFILE: join(fixture, 'home'), LUCI_ADDR: `127.0.0.1:${controlPort}`, LUCI_BROWSER_PREVIEW_ADDR: `127.0.0.1:${preview}`, LUCI_BROWSER_PREVIEW_PORT: String(preview), LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'), LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: join(dirname(binary), 'runtime'), LUCI_DOCKER_HOST: 'unix://' + join(fixture, 'absent-docker.sock'), DOCKER_HOST: 'unix://' + join(fixture, 'absent-docker.sock'), LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_BACKEND: 'docker', GIT_CONFIG_GLOBAL: join(fixture, 'home/.gitconfig'), GIT_CONFIG_NOSYSTEM: '1', AWS_EC2_METADATA_DISABLED: 'true', HTTP_PROXY: outbound, HTTPS_PROXY: outbound, NO_PROXY: '127.0.0.1,localhost' };
  await start(environment);
  const bootstrap = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(); secrets.add(bootstrap);
  const owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'overview-owner', displayName: 'Overview fixture owner', password }, 201, ''); token = owner.token;
  await api('POST', '/v1/setup/git/identity', { name: 'Overview fixture', email: 'overview@example.invalid' });
  project = await api('POST', '/v1/product/projects', { name: 'garden-journal-overview' }, 201); projectRoute = `/v1/product/projects/${project.id}`;
  assert(realpathSync(project.path).startsWith(realpathSync(fixture) + '/'), 'Project escaped the owned fixture');
  await api('POST', projectRoute + '/brief', { revision: 0, purpose: 'A garden journal for private observations with clear author ownership.', audience: 'Gardeners recording their own plants and observations.', constraints: 'Private by default; never publish without an explicit decision.', criteria: ['An author can save and retrieve a private observation.'] });
  let spec = await api('GET', projectRoute + '/specification');
  spec = await api('POST', projectRoute + '/specification', { expectedRevision: spec.revision, expectedBriefRevision: spec.briefRevision, specification: { ...spec, requirements: [{ id: 'AC-observation', title: 'Save a private observation', priority: 'must', acceptance: 'The signed-in author can save and retrieve an observation; another account cannot read it.' }] } });
  spec = await api('POST', projectRoute + '/specification/approve', { revision: spec.revision, briefRevision: spec.briefRevision });
  let design = await api('GET', projectRoute + '/design-plan');
  design = await api('POST', projectRoute + '/design-plan', { expectedRevision: design.revision, expectedBriefRevision: spec.briefRevision, expectedSpecificationRevision: spec.revision, plan: { ...design, screens: [{ id: 'journal', name: 'Private journal', route: '/', purpose: 'The author records an observation and can review their saved notes.', states: ['Empty journal prompts the author to add a note', 'Saving prevents duplicate submission', 'Saved note appears in the author journal', 'Access denied shows a sign-in remedy without private content'], widths: [390, 1440], requirementIds: ['AC-observation'], accessibility: 'Label note fields, expose save progress and failure to assistive technology, and preserve keyboard focus.' }], implementationNotes: 'Retain author ownership for every data read and write.' } });
  design = await api('POST', projectRoute + '/design-plan/approve', { revision: design.revision });
  const plan = await api('POST', projectRoute + '/plan', { expectedRevision: 0, expectedSpecificationRevision: spec.revision, tasks: [{ id: 'observation', title: taskA, status: 'todo', requirementIds: ['AC-observation'] }] });
  const committed = await api('POST', projectRoute + '/git/commit', { message: 'Save reviewed garden journal context', expectedReviewDigest: (await api('GET', projectRoute + '/git')).reviewDigest });
  assert.match(committed.commit, /^[a-f0-9]{40}$/); assert.equal(committed.sourceChanged, false);
  const connections = await api('POST', '/v1/setup/agent-connections', { action: 'save', expectedRevision: 0, name: 'No inference acceptance fixture', kind: 'compatible-api', enabled: true, models: { codex: 'overview-fixture' }, endpoints: { responses: outbound }, apiKey: secret() });
  const branches = (await api('GET', `/v1/projects/${project.id}/branches`)).branches, branch = branches.find(row => row.name === 'main') ?? branches[0];
  const worktree = (await api('GET', `/v1/projects/${project.id}/worktrees?branchId=${encodeURIComponent(branch.id)}`)).worktrees[0];
  const prepared = (await api('POST', projectRoute + '/runs', { idempotencyKey: randomBytes(24).toString('hex'), worktreeId: worktree.id, harness: 'codex', model: 'overview-fixture', permissionMode: 'ask', agentConnectionId: connections.connections[0].id, outcome: 'preview', implementationContext: { projectId: project.id, briefRevision: spec.briefRevision, specificationRevision: spec.revision, designPlanRevision: design.revision, planRevision: plan.revision, taskIds: ['observation'] } }, 201)).run;
  assert.equal(prepared.state, 'ready'); assert.deepEqual(prepared.operations, []);
  const snapshot = (await api('GET', projectRoute + '/orchestrator')).context, immutableInput = structuredClone(prepared.input), checkID = 'check-labelled-historical-fixture';
  await stop(); assert.equal(owned().length, 0, 'Private fixture history may only be seeded with all owned processes stopped');
  const runPath = ownedRunFile(prepared.id), beforeBytes = readFileSync(runPath), stored = JSON.parse(beforeBytes), privateRoot = dirname(dirname(dirname(runPath))), now = new Date().toISOString();
  assert.deepEqual(stored.run.input, immutableInput); assert.equal(stored.run.projectId, project.id); assert.equal(stored.run.state, 'ready'); assert.deepEqual(stored.run.checks, []);
  stored.run.state = 'awaiting-checks'; stored.run.reason = 'Labelled historical fixture only: no coding or check execution took place.'; stored.run.revision++; stored.run.updatedAt = now;
  // Seed only recorded history. Actual HTTP read models, check joins and later
  // context amendments still pass their normal authorization/validation paths.
  const history = [{ id: checkID, projectId: project.id, commit: committed.commit, dirty: false, execution: 'host', memoryMiB: 0, cpuMillis: 0, sourceDigest: immutableInput.implementation.sourceDigest, sourceChanged: false, status: 'passed', commands: [{ label: 'Historical fixture only; no command executed', executable: 'node', args: ['fixture-not-executed.js'], status: 'passed', exitCode: 0, output: 'Seeded history only; this command was never executed.', truncated: false }], createdAt: now, updatedAt: now, message: 'Explicit historical acceptance fixture, not an actual verification result.', requirementIds: immutableInput.requirements.map(row => row.id), requirementSnapshots: immutableInput.requirements, specificationRevision: spec.revision, specificationDigest: spec.digest, designPlanRevision: design.revision, designPlanDigest: design.digest, approvalSelection: immutableInput.approvals }];
  const questions = [firstQuestion, secondQuestion].map((question, index) => ({ id: `turn-fixture-question-${index + 1}`, projectId: project.id, sequence: index + 1, actor: { id: owner.user.id, displayName: 'Labelled historical fixture' }, message: 'Historical question setup only; no assistant request or inference occurred.', jobId: `assist-fixture-question-${index + 1}`, contextDigest: snapshot.digest, context: snapshot, status: 'succeeded', createdAt: now, updatedAt: now, answer: { summary: 'Seeded question for unsent-message interaction acceptance.', question, options: index ? [suggestion] : ['Ask for the plant name and a short note.'], rationale: 'This nonblocking question is seeded fixture history, not a model response.', blocking: false, impact: [], proposal: null, nextAction: 'Discuss without sending automatically' }, keyDigest: sha(`fixture-key-${index}`), requestDigest: sha(`fixture-request-${index}`) }));
  const ledger = { version: 1, accountScope: stored.accountScope, projectScope: stored.projectScope, projectId: project.id, revision: 2, turns: questions };
  const seeded = [[runPath, stored], [join(privateRoot, 'project-checks', project.id + '.json'), history], [join(privateRoot, 'orchestrator', project.id + '.json'), ledger]];
  for (const [path, value] of seeded) { assert(path.startsWith(realpathSync(fixture) + '/')); mkdirSync(dirname(path), { recursive: true, mode: 0o700 }); if (path !== runPath) assert(!existsSync(path), 'Refuse to overwrite pre-existing private history'); writeFileSync(path, JSON.stringify(value), { mode: 0o600, flag: path === runPath ? 'w' : 'wx' }); }
  receipt.fixtureHistory = { projectId: project.id, runId: prepared.id, checkId: checkID, beforeSHA256: sha(beforeBytes), afterSHA256: sha(readFileSync(runPath)), immutableInputSHA256: sha(JSON.stringify(immutableInput)), inputDigest: stored.inputDigest, questionTurnIds: questions.map(row => row.id), allOwnedProcessesStoppedAtSeed: true, sourceCommit: committed.commit, checkExecutionClaim: 'None; explicitly seeded historical record.' };
  await start(environment);
  const initial = await api('GET', projectRoute + '/orchestrator'); assert.equal(initial.context.currentRun.state, 'awaiting-checks'); assert.equal(initial.context.currentRun.contextFreshness.status, 'current'); assert.equal(initial.blockingQuestions.length, 0); assert.equal(initial.turns.length, 2);
  assert.equal((await api('GET', projectRoute + '/checks')).runs[0].evidenceStatus, 'linked-test-evidence', 'Seeded record must satisfy the actual evidence binding before a supported join');

  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited, args: ['--no-proxy-server'] }); running.signal.throwIfAborted(); receipt.browserVersion = browser.version();
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, serviceWorkers: 'block' });
  await context.route('**/*', async route => {
    try {
      const request = route.request(), url = new URL(request.url()), path = url.pathname, method = request.method();
      if (url.origin !== base) { receipt.blockedRequests.push({ method, origin: url.origin }); return await route.abort('blockedbyclient'); }
      receipt.requests.push({ stage, method, path });
      if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) { const permitted = method === 'POST' && (path === '/v1/auth/login' || (path === projectRoute + '/plan' && stage === 'plan-amendment')); receipt.browserMutations.push({ stage, method, path, permitted }); assert(permitted, 'Unexpected browser mutation: ' + path); }
      await route.continue();
    } catch (error) { if (!running.signal.aborted) routeError ??= error; await route.abort('failed').catch(() => {}); }
  });
  page = await context.newPage(); page.setDefaultTimeout(15000); page.on('pageerror', error => receipt.browserErrors.push(scrub(error)));
  await page.goto(base); await page.getByLabel('Username', { exact: true }).fill('overview-owner'); await page.getByLabel('Password', { exact: true }).fill(password); await action('/v1/auth/login', page.getByRole('button', { name: 'Sign in', exact: true }));
  const firstVisit = page.getByRole('button', { name: 'Continue to projects', exact: true });
  await until('project entry', async () => await firstVisit.isVisible() || await page.getByRole('treeitem', { name: /garden-journal-overview$/ }).isVisible()); if (await firstVisit.isVisible()) await firstVisit.click();
  await page.getByRole('treeitem', { name: /garden-journal-overview$/ }).click(); await openOverview();
  stage = 'mounted-run-refresh';
  await overview().getByRole('button', { name: 'Inspect build and checks', exact: true }).click();
  const receipts = await inspectRun(); await runs().getByText('Awaiting verification', { exact: true }).waitFor();
  assert.equal(await receipts.getByText(new RegExp(checkID)).count(), 0, 'Run should be cached before its historical check is joined');
  const joined = (await api('POST', `${projectRoute}/runs/${prepared.id}/checks`, { expectedRevision: stored.run.revision, checkId: checkID })).run;
  assert.equal(joined.state, 'awaiting-release'); assert.equal(joined.checks[0].checkId, checkID); assert.deepEqual(joined.input, immutableInput); assert.deepEqual(joined.operations, []);
  await panel().getByRole('button', { name: 'Refresh project understanding', exact: true }).click();
  await nextStep().getByRole('button', { name: 'Review verified build', exact: true }).waitFor();
  assert.equal(await receipts.getByText(new RegExp(checkID)).count(), 0, 'Overview refresh unexpectedly updated cached run; this would not exercise the stale mounted handoff');
  const runGETs = () => receipt.requests.filter(row => row.method === 'GET' && row.path === `${projectRoute}/runs/${prepared.id}`).length;
  const beforeInspect = runGETs(); await nextStep().getByRole('button', { name: 'Review verified build', exact: true }).click();
  await until('explicit exact-run refresh', () => runGETs() > beforeInspect);
  await (await inspectRun()).getByText(new RegExp(checkID)).waitFor();
  assert.equal((await api('GET', projectRoute + '/orchestrator')).nextAction.runId, prepared.id);
  check('Real supported historical-check join updates overview, then Review verified build refreshes the exact already-mounted run and its newly joined evidence');
  stage = 'checked-deployment-intent';
  await runs().getByRole('button', { name: 'Preview this build', exact: true }).click();
  const intent = page.getByRole('region', { name: 'Checked build deployment', exact: true });
  await until('checked intent source validation', async () => await intent.getByRole('button', { name: 'Refresh checked source', exact: true }).isEnabled());
  assert.equal(await intent.getByRole('alert').count(), 0); const intentText = await intent.innerText();
  for (const value of [prepared.id, checkID, committed.commit]) assert(intentText.includes(value), 'Checked deployment lost identity ' + value);
  assert(await page.getByRole('button', { name: 'Build local preview', exact: true }).isEnabled());
  for (const width of [1440, 390]) { await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 }); await sample(`01-checked-build-intent-${width}`, intent.getByRole('button', { name: 'Return to build results', exact: true })); }
  await intent.getByRole('button', { name: 'Return to build results', exact: true }).click(); await inspectRun(); await page.setViewportSize({ width: 1440, height: 1000 });
  check('Explicit Preview this build carries exact run, check and source to the deployment view, with usable desktop/phone return and no release started');

  stage = 'plan-amendment';
  const details = page.locator('details.project-work'); await openDetails(details);
  const planRegion = page.getByRole('region', { name: 'Implementation plan', exact: true });
  await planRegion.getByLabel('Task 1', { exact: true }).fill(taskB);
  const amended = await action(projectRoute + '/plan', planRegion.getByRole('button', { name: 'Save implementation plan', exact: true }));
  assert.equal(amended.revision, plan.revision + 1); assert.equal(amended.tasks[0].title, taskB);
  assert.deepEqual(await api('GET', projectRoute + '/specification'), spec); assert.deepEqual(await api('GET', projectRoute + '/design-plan'), design);
  await openOverview(); await nextStep().getByRole('button', { name: 'Review current plan', exact: true }).waitFor();
  const after = await api('GET', projectRoute + '/orchestrator');
  assert.equal(after.nextAction.id, 'plan'); assert.equal(after.context.currentRun.id, prepared.id); assert.equal(after.context.currentRun.contextFreshness.status, 'changed'); assert.equal(after.context.plan.tasks[0].title, taskB); assert.equal(after.context.currentRun.tasks[0].title, taskA);
  const original = (await api('GET', `${projectRoute}/runs/${prepared.id}`)).run; assert.deepEqual(original.input, immutableInput); assert.deepEqual(original.checks, joined.checks); assert.deepEqual(original.operations, []); assert.equal(original.state, 'awaiting-release');
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 }); await openOverview();
    const scope = overview().getByRole('region', { name: 'In this build', exact: true });
    await scope.getByText('Context changed since this build', { exact: true }).waitFor(); await scope.getByText(taskA, { exact: true }).waitFor(); assert.equal(await scope.getByText(taskB, { exact: true }).count(), 0);
    await sample(`02-current-plan-remedy-${width}`, nextStep().getByRole('button', { name: 'Review current plan', exact: true }));
    await sample(`03-original-build-scope-${width}`, scope.getByRole('button', { name: 'Inspect build and checks', exact: true }));
    if (await details.evaluate(element => element.open)) await details.locator(':scope > summary').click();
    await nextStep().getByRole('button', { name: 'Review current plan', exact: true }).click();
    await until('plan focus after overview handoff', () => planRegion.evaluate(element => document.activeElement === element));
    assert(await details.evaluate(element => element.open)); assert.equal(await planRegion.getByLabel('Task 1', { exact: true }).inputValue(), taskB);
    await openOverview(); // Compact plan handoff deliberately collapses the overview.
    await scope.getByRole('button', { name: 'Inspect build and checks', exact: true }).click(); await inspectRun();
  }
  receipt.amendment = { oldPlanRevision: plan.revision, newPlanRevision: amended.revision, taskA, taskB, definitionRevision: spec.revision, designRevision: design.revision, runId: prepared.id, currentFreshness: after.context.currentRun.contextFreshness };
  check('Actual UI plan-only save makes current-plan review explicit while original build scope/evidence stay immutable; exact plan and run handoffs work at desktop and phone');

  stage = 'occupied-composer';
  await openOverview(); const discussion = panel().locator('details.orchestrator-discussion'); await openDetails(discussion);
  const q1 = panel().getByRole('region', { name: 'Luci question', exact: true }).filter({ hasText: firstQuestion }), q2 = panel().getByRole('region', { name: 'Luci question', exact: true }).filter({ hasText: secondQuestion });
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1000 });
    await q1.getByRole('button', { name: 'Reply to this question', exact: true }).click();
    const unsent = `For the first screen, keep my unfinished ${width}px note exactly:\nPlant name, observation date, and a note — ask before adding extra fields.`;
    await composer().fill(unsent); const saved = await browserDraft(unsent, questions[0].id);
    await panel().getByText('Replying to: ' + firstQuestion, { exact: true }).waitFor();
    const option = q2.getByRole('button', { name: suggestion, exact: true }); await option.click();
    let dialog = page.getByRole('dialog', { name: 'Replace your unsent message?', exact: true }); await dialog.waitFor(); await dialog.getByText(secondQuestion, { exact: true }).waitFor();
    await dialog.getByText('Current unsent message', { exact: true }).click(); await dialog.getByText(unsent, { exact: true }).waitFor();
    assert.equal(await composer().inputValue(), unsent); assert.deepEqual(await browserDraft(unsent, questions[0].id), saved);
    await sample(`04-choice-keeps-original-draft-${width}`, dialog.getByRole('button', { name: 'Keep current message', exact: true }));
    await dialog.getByRole('button', { name: 'Keep current message', exact: true }).click();
    await until('Keep returns option focus', () => option.evaluate(element => document.activeElement === element));
    assert.equal(await composer().inputValue(), unsent); assert.deepEqual(await browserDraft(unsent, questions[0].id), saved); await panel().getByText('Replying to: ' + firstQuestion, { exact: true }).waitFor();
    await option.click(); dialog = page.getByRole('dialog', { name: 'Replace your unsent message?', exact: true }); await dialog.getByRole('button', { name: 'Replace with suggested answer', exact: true }).click();
    await until('Replace focuses composer', () => composer().evaluate(element => document.activeElement === element));
    assert.equal(await composer().inputValue(), suggestion); await browserDraft(suggestion, questions[1].id); await panel().getByText('Replying to: ' + secondQuestion, { exact: true }).waitFor();
    await sample(`05-explicit-replacement-unsent-${width}`, composer());
  }
  check('Occupied cross-question suggestions preserve exact text, reply identity and durable draft on Keep; explicit Replace changes only the draft and focuses its composer at both widths');
  assert.equal(routeError, undefined); assert.deepEqual(receipt.browserErrors, []); assert.deepEqual(receipt.blockedRequests, []); assert.deepEqual(receipt.providerAttempts, []);
  assert.deepEqual(receipt.browserMutations.map(row => row.path), ['/v1/auth/login', projectRoute + '/plan']);
  assert.equal(receipt.requests.filter(row => row.method !== 'GET' && /\/assist(?:\/|$)/.test(row.path)).length, 0);
  assert.deepEqual((await api('GET', '/v1/sessions')).sessions, []); assert.deepEqual((await api('GET', projectRoute + '/releases')).releases, []);
  const finalRun = (await api('GET', `${projectRoute}/runs/${prepared.id}`)).run; assert.deepEqual(finalRun.input, immutableInput); assert.deepEqual(finalRun.operations, []); assert.deepEqual(finalRun.releases, []); assert.equal(finalRun.acceptance, undefined);
  const finalView = await api('GET', projectRoute + '/orchestrator'); assert.equal(finalView.revision, 2); assert.equal(finalView.turns.length, 2);
  assert.equal(sha(readFileSync(binary)), binarySHA256); assert.deepEqual(inventory(), webFiles); running.signal.throwIfAborted(); receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error); receipt.failedStage = stage; if (routeError) receipt.fixtureError = scrub(routeError);
  if (page) await screenshot('failure').catch(() => {}); process.exitCode = 1;
} finally {
  clearTimeout(budget);
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

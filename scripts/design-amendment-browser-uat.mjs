#!/usr/bin/env node
// Installed Design amendment recovery. Baseline account/context is API scaffolding;
// removal, width typing, draft recovery, saving and approval use the real UI.
// No Docker, provider, source code or release allocation.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { createServer } from 'node:net';
import { existsSync, lstatSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, readlinkSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '../desktop/node_modules/playwright/index.mjs';

const args = process.argv.slice(2);
if (args.includes('--help')) {
  console.log('node scripts/design-amendment-browser-uat.mjs --binary PATH --web PATH --chromium PATH --provenance PATH --report PATH\nInstalled browser proof of requirement removal, explicit design unlinking, ordinary per-key responsive width entry, invalid draft reload/recovery and explicit design review. Account and initial reviewed context are labelled API scaffolding. All claimed amendment steps use UI; no Docker or model calls. Report directory must be new or empty.');
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
const fixture = mkdtempSync(join(tmpdir(), 'luci-design-amendment-'));
for (const name of ['home', 'projects', 'state']) mkdirSync(join(fixture, name), { mode: 0o700 });
const secrets = new Set(), secret = () => { const value = randomBytes(32).toString('base64url'); secrets.add(value); return value; };
const password = secret(), lifecycle = secret(), scrub = error => { let text = String(error?.message ?? error); for (const value of secrets) text = text.replaceAll(value, '[redacted]'); return text; };
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit,
  package: { archiveSHA256: provenance.archiveSHA256, binarySHA256, webIndexSHA256, allWebFilesMatchArchive: true, webFileCount: Object.keys(webFiles).length },
  driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  driverCheckout: { commit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: dirname(fileURLToPath(import.meta.url)), encoding: 'utf8' }).trim(), dirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: dirname(fileURLToPath(import.meta.url)), encoding: 'utf8' }).trim() },
  scope: 'Installed UI recovery of a removed requirement link and exact unfinished responsive widths; saving and approval use the actual service.',
  limitations: ['Initial account, project, two-outcome definition and reviewed one-screen design are API scaffolding, not a new-project UI journey.', 'No Docker, agent/provider, source implementation, checks, release or model-quality claim.', 'Concurrent width conflict choices are covered separately by component tests; this installed proof covers a single editing account.'],
  setupScaffolding: ['Private account and Git author identity', 'One project with a reviewed two-outcome definition and one reviewed screen linked to both outcomes'],
  checks: [], browserMutations: [], blockedRequests: [], browserErrors: [], cleanup: [],
  externalModelCalls: 0, cloudResourcesCreated: 0, dockerAllocations: 0,

};
const inherited = Object.fromEntries(Object.entries(process.env).filter(([name]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ)$/.test(name)));
let service, browser, page, base, token, project, projectRoute, stage = 'setup', spawnError, serviceLog = '', routeError;
const check = (name, details = {}) => { receipt.checks.push({ name, ...details }); console.log('PASS ' + name); };
async function port() { const server = createServer(); await new Promise((done, reject) => server.once('error', reject).listen(0, '127.0.0.1', done)); const value = server.address().port; await new Promise(done => server.close(done)); return value; }
async function api(method, route, body, expected = 200, auth = token) {
  assert(stage === 'setup' || method === 'GET', 'Only UI may change project context after setup');
  const response = await fetch(base + route, { method, headers: { 'Content-Type': 'application/json', ...(auth ? { Authorization: 'Bearer ' + auth } : {}) }, body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(15000) });
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
async function until(label, probe, timeout = 20000) {
  const end = Date.now() + timeout;
  while (Date.now() < end) { const value = await probe(); if (value) return value; if (routeError) throw routeError; await delay(50); }
  throw new Error('Timed out: ' + label);
}
async function screenshot(name) { await page.screenshot({ path: join(evidence, name + '.png'), fullPage: true }); }
async function mutation(action, path) {
  const response = page.waitForResponse(value => value.request().method() === 'POST' && new URL(value.url()).pathname === path);
  await action(); const actual = await response;
  assert.equal(actual.status(), 200, 'UI mutation refused: ' + path);
  return actual.json();
}
async function openProject(lens) {
  await page.getByRole('treeitem', { name: /design-amendment$/ }).click();
  await page.getByRole('button', { name: lens, exact: true }).click();
}
const input = () => page.getByRole('textbox', { name: 'Responsive widths', exact: true });
const saveButton = () => page.getByRole('button', { name: 'Save design plan', exact: true });
const removeLink = () => page.getByRole('checkbox', { name: 'Remove unavailable outcome AC-export from screen 1', exact: true });
async function makePage() {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, serviceWorkers: 'block' });
  await context.route('**/*', async route => {
    try {
      const request = route.request(), url = new URL(request.url()), path = url.pathname, method = request.method();
      if (url.origin !== base) { receipt.blockedRequests.push({ method, origin: url.origin, path }); throw new Error('Unexpected external request'); }
      if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
        assert.equal(method, 'POST');
        assert(['/v1/auth/login', projectRoute + '/specification', projectRoute + '/specification/approve', projectRoute + '/design-plan', projectRoute + '/design-plan/approve'].includes(path), 'Unexpected browser mutation: ' + path);
        receipt.browserMutations.push({ stage, method, path, ...(path === '/v1/auth/login' ? {} : { body: request.postDataJSON() }) });
      }
      await route.continue();
    } catch (error) { routeError ??= error; await route.abort('failed').catch(() => {}); }
  });
  const current = await context.newPage(); current.setDefaultTimeout(20000);
  current.on('pageerror', error => receipt.browserErrors.push(scrub(error)));
  current.on('dialog', dialog => dialog.type() === 'beforeunload' ? dialog.accept() : dialog.dismiss());
  return current;
}
try {
  const address = `127.0.0.1:${await port()}`, preview = await port(); base = 'http://' + address;
  const environment = { ...inherited, HOME: join(fixture, 'home'), USERPROFILE: join(fixture, 'home'), LUCI_ADDR: address, LUCI_BROWSER_PREVIEW_ADDR: `127.0.0.1:${preview}`, LUCI_BROWSER_PREVIEW_PORT: String(preview), LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'), LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: join(dirname(binary), 'runtime'), LUCI_DOCKER_HOST: 'unix://' + join(fixture, 'absent-docker.sock'), DOCKER_HOST: 'unix://' + join(fixture, 'absent-docker.sock'), LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_BACKEND: 'docker', GIT_CONFIG_GLOBAL: join(fixture, 'home/.gitconfig'), GIT_CONFIG_NOSYSTEM: '1', AWS_EC2_METADATA_DISABLED: 'true' };
  service = spawn(binary, [], { cwd: fixture, env: environment, detached: true, stdio: ['ignore', 'pipe', 'pipe'] }); service.once('error', error => { spawnError = error; });
  for (const stream of [service.stdout, service.stderr]) stream.on('data', bytes => { serviceLog = (serviceLog + scrub(bytes.toString())).slice(-65536); });
  const deadline = Date.now() + 30000; let status;
  while (Date.now() < deadline) { if (spawnError) throw spawnError; assert.equal(service.exitCode, null, 'Service exited before readiness'); try { status = await api('GET', '/v1/control-plane/status', undefined, 200, ''); break; } catch { await delay(100); } }
  assert(status, 'Service not ready'); receipt.version = status.version;
  const bootstrap = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(); secrets.add(bootstrap);
  token = (await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'design-owner', displayName: 'Design review owner', password }, 201, '')).token;
  await api('POST', '/v1/setup/git/identity', { name: 'Design review', email: 'design-review@example.invalid' });
  project = await api('POST', '/v1/product/projects', { name: 'design-amendment' }, 201); projectRoute = '/v1/product/projects/' + project.id;
  const emptyDefinition = await api('GET', projectRoute + '/specification');
  let definition = await api('POST', projectRoute + '/specification', { expectedRevision: emptyDefinition.revision, expectedBriefRevision: emptyDefinition.briefRevision, specification: { ...emptyDefinition, purpose: 'A private garden notebook.', audience: 'Invited gardeners.', requirements: [
    { id: 'AC-save', title: 'Save a garden observation', priority: 'must', acceptance: 'A saved observation remains readable after reopening.' },
    { id: 'AC-export', title: 'Export garden observations', priority: 'should', acceptance: 'An invited gardener can export their saved observations.' },
  ] } });
  definition = await api('POST', projectRoute + '/specification/approve', { revision: definition.revision, briefRevision: definition.briefRevision });
  const emptyPlan = await api('GET', projectRoute + '/design-plan');
  let original = await api('POST', projectRoute + '/design-plan', { expectedRevision: emptyPlan.revision, expectedBriefRevision: definition.briefRevision, expectedSpecificationRevision: definition.revision, plan: { ...emptyPlan, screens: [{ id: 'screen-notes', name: 'Garden notes', route: '/notes', purpose: 'Save, revisit and export observations.', states: ['First visit without observations', 'Saved observation', 'Save error with retry'], widths: [390, 1440], requirementIds: ['AC-save', 'AC-export'], accessibility: 'Labelled fields, keyboard operation and visible focus.' }], implementationNotes: 'Retain source observations and keyboard behavior during amendments.' } });
  original = await api('POST', projectRoute + '/design-plan/approve', { revision: original.revision });
  receipt.baseline = { projectID: project.id, definition, design: original };
  const originalHistory = await api('GET', projectRoute + '/design-plan/history');
  stage = 'remove-outcome';
  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited }); receipt.browserVersion = browser.version(); page = await makePage();
  await page.goto(base); await page.getByLabel('Username', { exact: true }).fill('design-owner'); await page.getByLabel('Password', { exact: true }).fill(password); await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  const firstVisit = page.getByRole('button', { name: 'Continue to projects', exact: true }), projectItem = page.getByRole('treeitem', { name: /design-amendment$/ });
  await until('project entry', async () => await firstVisit.isVisible() || await projectItem.isVisible());
  if (await firstVisit.isVisible()) await firstVisit.click();
  await openProject('Define'); await page.getByRole('tab', { name: 'Specification', exact: true }).click();
  await page.getByRole('button', { name: 'Remove requirement 2', exact: true }).click();
  const draftDefinition = await mutation(() => page.getByRole('button', { name: 'Save definition', exact: true }).click(), projectRoute + '/specification');
  assert.deepEqual(draftDefinition.requirements, [definition.requirements[0]]); assert.equal(draftDefinition.status, 'draft');
  const amended = await mutation(() => page.getByRole('button', { name: 'Approve definition', exact: true }).click(), projectRoute + '/specification/approve');
  assert.equal(amended.status, 'approved'); assert.deepEqual(amended.requirements, draftDefinition.requirements);
  await page.getByRole('button', { name: 'Design', exact: true }).click(); await removeLink().waitFor();
  assert(await removeLink().isChecked()); assert(await saveButton().isDisabled());
  const unchanged = await api('GET', projectRoute + '/design-plan'); assert.deepEqual(unchanged.screens, original.screens); assert.equal(unchanged.status, 'outdated');
  await screenshot('01-removed-outcome-visible-with-screen-preserved');
  check('Actual Define removal and renewed review preserve the screen and surface its unavailable link; Design cannot silently save it');

  stage = 'explicit-unlink-and-typing';
  // This explicit unlink removes its own checkbox. A normal click followed by
  // absence verifies the action; uncheck() waits for a node that no longer exists.
  await removeLink().click(); assert.equal(await removeLink().count(), 0);
  const widthField = input(); await widthField.click(); await page.keyboard.press('End'); await page.keyboard.type(', ');
  assert.equal(await widthField.inputValue(), '390, 1440, '); assert.equal(await widthField.getAttribute('aria-invalid'), 'true'); assert(await saveButton().isDisabled());
  await page.keyboard.press('Tab'); assert.equal(await widthField.inputValue(), '390, 1440, ');
  await page.getByRole('tab', { name: 'Implementation handoff', exact: true }).click();
  assert.equal(await page.getByRole('textbox', { name: 'Implementation notes', exact: true }).inputValue(), original.implementationNotes);
  assert(await saveButton().isDisabled()); await page.getByRole('tab', { name: 'Screens and states', exact: true }).click();
  assert.equal(await input().inputValue(), '390, 1440, ');
  await screenshot('02-unfinished-widths-retained');
  assert.deepEqual(await api('GET', projectRoute + '/design-plan'), unchanged);
  assert.equal(receipt.browserMutations.filter(row => row.path === projectRoute + '/design-plan').length, 0);
  check('Ordinary keyboard comma entry survives blur and tab navigation; invalid exact text and explicit unlink remain an unsaved draft');

  stage = 'recover-draft';
  await page.reload(); await openProject('Design');
  const recovery = page.getByRole('region', { name: 'Design plan draft recovery', exact: true }); await recovery.waitFor();
  await screenshot('03-reload-offers-explicit-draft-recovery');
  await recovery.getByRole('button', { name: 'Restore draft', exact: true }).click();
  assert.equal(await input().inputValue(), '390, 1440, '); assert.equal(await removeLink().count(), 0); assert(await saveButton().isDisabled());
  assert.deepEqual(await api('GET', projectRoute + '/design-plan'), unchanged);
  check('Actual reload and explicit Restore recover unfinished widths and the chosen unlink without changing the saved design');

  stage = 'valid-save';
  await input().click(); await page.keyboard.press('End'); await page.keyboard.type('768');
  assert.equal(await input().inputValue(), '390, 1440, 768'); assert.equal(await input().getAttribute('aria-invalid'), 'false');
  const saved = await mutation(() => saveButton().click(), projectRoute + '/design-plan');
  await page.getByText(`Design plan ${saved.revision} saved. Implementation context updated.`, { exact: true }).waitFor();
  const expectedScreen = { ...original.screens[0], widths: [390, 1440, 768], requirementIds: ['AC-save'] };
  assert.deepEqual(saved.screens, [expectedScreen]); assert.equal(saved.implementationNotes, original.implementationNotes); assert.equal(saved.status, 'draft'); assert.equal(saved.specificationRevision, amended.revision);
  assert.equal(receipt.browserMutations.filter(row => row.path === projectRoute + '/design-plan/approve').length, 0);
  assert.deepEqual(receipt.browserMutations.find(row => row.path === projectRoute + '/design-plan').body.plan.screens, [expectedScreen]);
  await page.reload(); await openProject('Design'); assert.equal(await input().inputValue(), '390, 1440, 768');
  assert.equal(await removeLink().count(), 0); assert.deepEqual(await api('GET', projectRoute + '/design-plan'), saved);
  await screenshot('04-saved-screen-draft-awaits-review');
  check('Valid save and reload persist numeric widths and only the explicit unlink, preserving every other screen field and requiring fresh approval');

  stage = 'explicit-review';
  const approved = await mutation(() => page.getByRole('button', { name: 'Approve design handoff', exact: true }).click(), projectRoute + '/design-plan/approve');
  assert.equal(approved.status, 'approved'); assert.equal(approved.revision, saved.revision + 1); assert.deepEqual(approved.screens, saved.screens);
  await page.getByRole('button', { name: 'Continue to Develop', exact: true }).waitFor();
  const finalHistory = await api('GET', projectRoute + '/design-plan/history'); assert.deepEqual(finalHistory.revisions.slice(0, originalHistory.revisions.length), originalHistory.revisions);
  assert.equal(finalHistory.revisions.length, originalHistory.revisions.length + 2);
  receipt.final = { saved, approved, originalHistoryPreserved: true };
  await screenshot('05-explicit-review-complete');
  check('Explicit review approves the exact amended revision; immutable earlier design history is preserved');
  assert.equal(routeError, undefined); assert.deepEqual(receipt.browserErrors, []); assert.deepEqual(receipt.blockedRequests, []);
  assert.equal(sha(readFileSync(binary)), binarySHA256); assert.deepEqual(inventory(), webFiles);
  receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error); receipt.failedStage = stage; if (routeError) receipt.fixtureError = scrub(routeError);
  if (page) { await screenshot('failure').catch(() => {}); await page.locator('body').innerText().then(body => writeFileSync(join(evidence, 'failure-dom.txt'), scrub(body), { mode: 0o600 })).catch(() => {}); }
  process.exitCode = 1;
} finally {
  await browser?.close().catch(error => { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); });
  let cleaned = false; try { await stop(); cleaned = true; } catch (error) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); }
  if (cleaned && receipt.passed) rmSync(fixture, { recursive: true, force: true }); else receipt.retainedFixture = fixture;
  if (receipt.cleanupErrors?.length) { receipt.passed = false; process.exitCode = 1; }
  receipt.finishedAt = new Date().toISOString();
  writeFileSync(join(evidence, 'service.log'), scrub(serviceLog), { mode: 0o600 }); writeFileSync(report, JSON.stringify(receipt, null, 2), { mode: 0o600 });
  console.log(JSON.stringify({ passed: receipt.passed, checks: receipt.checks.length, report, failedStage: receipt.failedStage }));
}

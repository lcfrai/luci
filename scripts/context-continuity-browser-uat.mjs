#!/usr/bin/env node
// Installed context navigation and verification-draft continuity. Baseline documents
// are labelled API scaffolding. Claimed answer, review, navigation and recovery
// steps use the UI. Docker is unavailable; all execution endpoints are blocked.
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
import axe from '../desktop/node_modules/axe-core/axe.js';

const args = process.argv.slice(2);
if (args.includes('--help')) {
  console.log('node scripts/context-continuity-browser-uat.mjs --binary PATH --web PATH --chromium PATH --provenance PATH --report PATH\nInstalled manual-question navigation, dirty-draft cancellation, saved versus reviewed answers, and verification command recovery across unchanged and amended context. Account/project/initial documents/host-option policy are API scaffolding. All claimed amendments and recovery use the UI. No Docker, provider, commands or model dispatch. New or empty evidence directory required.');
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
const fixture = mkdtempSync(join(tmpdir(), 'luci-context-continuity-'));
for (const name of ['home', 'projects', 'state']) mkdirSync(join(fixture, name), { mode: 0o700 });
const secrets = new Set(), secret = () => { const value = randomBytes(32).toString('base64url'); secrets.add(value); return value; };
const password = secret(), lifecycle = secret(), scrub = error => { let text = String(error?.message ?? error); for (const value of secrets) text = text.replaceAll(value, '[redacted]'); return text; };
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit,
  package: { archiveSHA256: provenance.archiveSHA256, binarySHA256, webIndexSHA256, allWebFilesMatchArchive: true, webFileCount: Object.keys(webFiles).length },
  driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  driverCheckout: { commit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: dirname(fileURLToPath(import.meta.url)), encoding: 'utf8' }).trim(), dirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: dirname(fileURLToPath(import.meta.url)), encoding: 'utf8' }).trim() },
  scope: 'Installed manual-question navigation and verification setup recovery, truthful reviewed handoff facts, available no-session planning viewport, and unsent help with explicit review and renewed execution consent.',
  limitations: ['Account/project/initial definition and authored design draft are API scaffolding; claimed answer, review, navigation and recovery steps use the UI.', 'Instance host-command policy is enabled only to demonstrate that a previously selected host mode and consent never recover. No command is dispatched; execution routes are blocked and Docker points to an absent socket.', 'No provider, model reasoning, source implementation, checks, release or full-journey quality claim. Composer remains independently blocked by missing provider/model; this UI slice proves editable unsent help and its disclosure, not dispatch readiness.', 'Bounded axe and rendered focus/reflow observations are not accessibility certification; incomplete findings remain separate for manual review.'],
  setupScaffolding: ['Private account and Git identity', 'One project with an initial draft definition: first question answered, second question unanswered', 'An authored one-screen design draft', 'Host-command selector enabled by setup policy; no host commands allowed by browser interceptor'],
  checks: [], accessibility: [], browserMutations: [], blockedRequests: [], browserErrors: [], cleanup: [],
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
async function auditScope(name, selector) {
  await page.evaluate(axe.source);
  const workspace = page.locator(selector);
  const result = await workspace.evaluate(async node => {
    const rect = element => { const r = element.getBoundingClientRect(); return { x: r.x, y: r.y, width: r.width, height: r.height }; };
    const active = document.activeElement;
    const audit = await window.axe.run(node, { runOnly: ['wcag2a', 'wcag2aa', 'wcag21aa'] });
    const findings = rows => rows.map(row => ({ id: row.id, impact: row.impact, help: row.help, nodes: row.nodes.map(item => ({ target: item.target, failureSummary: item.failureSummary, review: [...item.any, ...item.all, ...item.none].map(check => ({ id: check.id, message: check.message, data: check.data })) })) }));
    return {
      viewport: { width: innerWidth, height: innerHeight }, documentScrollWidth: document.documentElement.scrollWidth,
      workspace: rect(node),
      focus: { insideScope: node.contains(active), tag: active?.tagName, label: active?.getAttribute('aria-label') || active?.closest('label')?.textContent || (active?.tagName === 'BUTTON' ? active.textContent : ''), bounds: active ? rect(active) : null },
      violations: findings(audit.violations), incomplete: findings(audit.incomplete), passes: audit.passes.map(row => row.id),
    };
  });
  writeFileSync(join(evidence, name + '-accessibility.json'), JSON.stringify(result, null, 2), { mode: 0o600 });
  receipt.accessibility.push({ state: name, viewport: result.viewport, documentScrollWidth: result.documentScrollWidth, focus: result.focus, violations: result.violations.map(row => row.id), incomplete: result.incomplete.map(row => row.id) });
  assert(result.documentScrollWidth <= result.viewport.width + 1, name + ' has horizontal document overflow');
  assert.deepEqual(result.violations, [], name + ' has scoped automated accessibility violations');
}
// Read actual clipping/scroll ownership after an ordinary browser scroll. The
// no-session planning surface should consume its workspace, not retain nested
// check panes above an empty terminal reserve. No exact pixel size is prescribed.
async function planningViewport(name, target, { scroll = true } = {}) {
  if (scroll) await target.scrollIntoViewIfNeeded();
  const measure = () => target.evaluate(node => {
    const workspace = node.closest('.develop-workspace');
    if (!workspace) throw new Error('Planning control is outside Develop');
    const rect = item => { const r = item.getBoundingClientRect(); return { x: r.x, y: r.y, width: r.width, height: r.height, top: r.top, right: r.right, bottom: r.bottom, left: r.left }; };
    const scrollOwners = [];
    for (let parent = node.parentElement; parent; parent = parent.parentElement) {
      const style = getComputedStyle(parent);
      if (/(auto|scroll)/.test(style.overflowY) && parent.scrollHeight > parent.clientHeight + 1) scrollOwners.push({ isWorkspace: parent === workspace, tag: parent.tagName, classes: parent.className, bounds: rect(parent), clientHeight: parent.clientHeight, scrollHeight: parent.scrollHeight });
      if (parent === workspace) break;
    }
    const r = rect(node), w = rect(workspace), x = r.left + r.width / 2, y = r.top + r.height / 2;
    const hit = document.elementFromPoint(x, y), active = document.activeElement;
    return { viewport: { width: innerWidth, height: innerHeight }, workspace: w, target: r, scrollOwners,
      contained: r.top >= Math.max(0, w.top) - 1 && r.bottom <= Math.min(innerHeight, w.bottom) + 1 && r.left >= Math.max(0, w.left) - 1 && r.right <= Math.min(innerWidth, w.right) + 1,
      hitTarget: !!hit && (hit === node || node.contains(hit)),
      focus: { tag: active?.tagName, label: active?.getAttribute('aria-label') || active?.closest('label')?.textContent || (active?.tagName === 'BUTTON' ? active.textContent : ''), bounds: active ? rect(active) : null } };
  });
  let value;
  try { await until('normal scroll exposes ' + name, async () => { value = await measure(); return value.contained && value.hitTarget; }, 5000); }
  finally { receipt.planningLayout ??= []; receipt.planningLayout.push({ name, ...value }); }
  assert.equal(value.scrollOwners.length, 1, name + ' must use one natural content scroll area');
  assert.equal(value.scrollOwners[0].isWorkspace, true, name + ' must use the available Develop workspace, not a nested restricted editor');
  await screenshot(name);
}
async function revealPlanFromOverview() {
  const work = page.locator('details.project-work');
  const plan = page.getByRole('region', { name: 'Implementation plan', exact: true });
  receipt.planNavigation ??= [];
  for (const viewport of [{ width: 1440, height: 1000 }, { width: 390, height: 844 }]) {
    await page.setViewportSize(viewport);
    assert(await work.evaluate(node => node.open));
    await work.locator('summary').first().click();
    assert.equal(await work.evaluate(node => node.open), false);
    const understanding = await overview();
    await understanding.getByRole('region', { name: 'Next step', exact: true }).getByRole('button', { name: 'Review implementation tasks', exact: true }).click();
    await until('overview opens and focuses implementation plan', () => plan.evaluate(node => node === document.activeElement));
    assert.equal(await work.evaluate(node => node.open), true);
    // Do not help the product scroll to its destination. The normal Next step
    // action must itself reveal the focused region and its visible heading.
    await planningViewport('04c-overview-reveals-plan-' + viewport.width, plan.getByRole('heading', { name: 'Implementation plan', exact: true }), { scroll: false });
    assert(await plan.evaluate(node => node === document.activeElement));
    receipt.planNavigation.push({ viewport, disclosureOpened: true, exactRegionFocused: true, userScrollAfterNextAction: false });
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
}
async function unsentCheckHelp(panel, unfinished) {
  await overview();
  const assistant = page.getByRole('complementary', { name: 'Project understanding', exact: true });
  await disclosure(assistant, 'Discuss or request a change');
  const note = 'Your verification setup is a browser draft and is not included automatically. Paste the relevant commands or unfinished arguments into your message to ask for help. Sending does not execute them.';
  await assistant.getByText(note, { exact: true }).waitFor();
  const message = assistant.getByRole('textbox', { name: 'Message Luci', exact: true });
  const question = 'How should I finish the arguments for the retention check? Please explain before I run it.';
  assert(await message.isEnabled()); await message.fill(question);
  assert.equal(await message.inputValue(), question);
  assert(!(await assistant.innerText()).includes('Verification setup has unsaved edits.'), 'Local setup must not masquerade as unsaved shared context');
  const routingRemedy = 'Choose a saved connection, or enter the exact model for your own agent settings.';
  await assistant.getByText(routingRemedy, { exact: true }).waitFor();
  assert(await assistant.getByRole('button', { name: 'Send to Luci', exact: true }).isDisabled(), 'No configured provider/model is present in this fixture');
  assert.equal(await panel.getByRole('textbox', { name: 'Arguments · JSON array', exact: true }).inputValue(), unfinished);
  await message.scrollIntoViewIfNeeded(); await screenshot('04a-local-check-draft-unsent-help');
  receipt.localSetupHelp = { explicitBoundary: note, message: question, configuredProvider: false, sendDisabledReason: routingRemedy, sent: false, unfinishedArgumentsPreserved: true };
  // Clear only the unsent message through its editable UI; do not leave a new
  // composer recovery prompt to obscure the later verification-only reload.
  await message.fill(''); await disclosure(assistant, 'Discuss or request a change', false);
}
async function mutation(action, path) {
  const response = page.waitForResponse(value => value.request().method() === 'POST' && new URL(value.url()).pathname === path);
  // Observe failure immediately if the action fails before this waiter is awaited.
  // The await below still rejects; this only keeps cleanup and evidence reachable.
  void response.catch(() => {});
  await action(); const actual = await response;
  assert.equal(actual.status(), 200, 'UI mutation refused: ' + path);
  return actual.json();
}
async function openProject(lens) {
  await page.getByRole('treeitem', { name: /context-continuity$/ }).click();
  await page.getByRole('button', { name: lens, exact: true }).click();
}
async function disclosure(scope, name, open = true) {
  const summary = scope.getByText(name, { exact: true });
  await summary.waitFor();
  if (await summary.evaluate(node => node.closest('details').open) !== open) await summary.click();
}
async function overview() {
  const show = page.getByRole('button', { name: 'Show project overview', exact: true });
  if (await show.isVisible()) await show.click();
  return page.getByRole('region', { name: 'Current project understanding', exact: true });
}
async function checksPanel() {
  const work = page.locator('details.project-work');
  await work.waitFor();
  if (!await work.evaluate(node => node.open)) await work.locator('summary').first().click();
  const panel = page.getByRole('region', { name: 'Project checks', exact: true });
  await panel.waitFor();
  await panel.getByRole('textbox', { name: 'Executable', exact: true }).waitFor();
  return panel;
}
const trust = panel => panel.getByRole('checkbox', { name: /^I trust/ });
async function freshConsent(panel) {
  await until('Docker mode after recovery', async () => await panel.getByRole('combobox', { name: 'Check execution', exact: true }).inputValue() === 'docker');
  assert.equal(await trust(panel).isChecked(), false);
  assert(await panel.getByRole('button', { name: /^(Run selected requirement checks|Run project checks)$/ }).isDisabled());
}
async function reviewDesign() {
  await page.getByRole('button', { name: 'Design', exact: true }).click();
  await page.getByRole('button', { name: 'Save design plan', exact: true }).waitFor();
  const saved = await mutation(() => page.getByRole('button', { name: 'Save design plan', exact: true }).click(), projectRoute + '/design-plan');
  const approved = await mutation(() => page.getByRole('button', { name: 'Approve design handoff', exact: true }).click(), projectRoute + '/design-plan/approve');
  assert.equal(approved.status, 'approved'); assert.deepEqual(approved.screens, saved.screens);
  return approved;
}
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
  token = (await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'continuity-owner', displayName: 'Continuity review owner', password }, 201, '')).token;
  await api('PATCH', '/v1/admin/policy', { allowHostCommands: true });
  await api('POST', '/v1/setup/git/identity', { name: 'Continuity review', email: 'continuity-review@example.invalid' });
  project = await api('POST', '/v1/product/projects', { name: 'context-continuity' }, 201); projectRoute = '/v1/product/projects/' + project.id;
  const empty = await api('GET', projectRoute + '/specification');
  const requirements = [{ id: 'AC-notes', title: 'Keep garden observations', priority: 'must', acceptance: 'A member saves an observation and can read the same text after reopening the garden journal.' }];
  const questions = [
    { id: 'question-sharing', question: 'Who can read a saved observation?', answer: 'Only members of this garden workspace.' },
    { id: 'question-retention', question: 'What happens to an observation after a member leaves?', answer: '' },
  ];
  let definition = await api('POST', projectRoute + '/specification', { expectedRevision: empty.revision, expectedBriefRevision: empty.briefRevision, specification: { ...empty, purpose: 'Keep a shared garden journal with clear retention decisions.', audience: 'Garden members and coordinators.', requirements, questions } });
  const emptyPlan = await api('GET', projectRoute + '/design-plan');
  const originalNotes = 'Retain member observations and show save failures without losing entered text.';
  const originalDesign = await api('POST', projectRoute + '/design-plan', { expectedRevision: emptyPlan.revision, expectedBriefRevision: definition.briefRevision, expectedSpecificationRevision: definition.revision, plan: { ...emptyPlan, screens: [{ id: 'screen-journal', name: 'Garden journal', route: '/journal', purpose: 'Members save and reread garden observations; coordinators can review the shared journal.', states: ['Empty journal invites a member to add an observation.', 'Saved observation remains visible after reopening.', 'Save failure retains the entered text and offers retry.'], widths: [390, 1440], requirementIds: ['AC-notes'], accessibility: 'Label the note field, preserve keyboard focus and announce save status.' }], implementationNotes: originalNotes } });
  receipt.baseline = { projectID: project.id, definition, design: originalDesign };
  stage = 'question-navigation-and-cancel';
  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited }); receipt.browserVersion = browser.version(); page = await makePage();
  await page.goto(base); await page.getByLabel('Username', { exact: true }).fill('continuity-owner'); await page.getByLabel('Password', { exact: true }).fill(password); await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  const firstVisit = page.getByRole('button', { name: 'Continue to projects', exact: true }), projectItem = page.getByRole('treeitem', { name: /context-continuity$/ });
  await until('project entry', async () => await firstVisit.isVisible() || await projectItem.isVisible()); if (await firstVisit.isVisible()) await firstVisit.click();
  await openProject('Design'); await page.getByRole('tab', { name: 'Implementation handoff', exact: true }).click();
  const notes = page.getByRole('textbox', { name: 'Implementation notes', exact: true });
  const unsavedNotes = originalNotes + ' Unsaved reminder: ask the coordinator about exports.';
  await notes.fill(unsavedNotes);
  let understanding = await overview();
  const questionItem = () => understanding.getByRole('listitem').filter({ has: page.getByText(questions[1].question, { exact: true }) });
  await questionItem().getByRole('button', { name: 'Answer in definition', exact: true }).click();
  let guard = page.getByRole('dialog', { name: 'Keep your changes?', exact: true }); await guard.waitFor();
  assert((await guard.innerText()).includes('Design')); await screenshot('00-question-navigation-guard');
  await guard.getByRole('button', { name: 'Keep editing', exact: true }).click();
  assert.equal(await notes.inputValue(), unsavedNotes);
  assert.equal(await page.getByRole('tab', { name: 'Implementation handoff', exact: true }).getAttribute('aria-selected'), 'true');
  assert.deepEqual(await api('GET', projectRoute + '/design-plan'), originalDesign);
  await questionItem().getByRole('button', { name: 'Answer in definition', exact: true }).click();
  await guard.getByRole('button', { name: 'Discard edits and continue', exact: true }).click();
  const question = page.getByRole('group', { name: 'Open question 2', exact: true });
  const answer = question.getByRole('textbox', { name: 'Answer', exact: true });
  await until('exact second answer receives product focus', () => answer.evaluate(node => node === document.activeElement));
  assert.equal(await question.getByRole('textbox', { name: 'Question', exact: true }).inputValue(), questions[1].question);
  assert.equal(await page.getByRole('group', { name: 'Open question 1', exact: true }).getByRole('textbox', { name: 'Answer', exact: true }).inputValue(), questions[0].answer);
  assert.equal(await page.getByRole('tab', { name: 'Specification', exact: true }).getAttribute('aria-selected'), 'true');
  assert.equal(await page.getByText('Open questions', { exact: true }).evaluate(node => node.closest('details').open), true);
  await screenshot('01-exact-answer-focused-1440'); await auditScope('01-exact-answer-focused-1440', '.definition-workspace');
  check('Manual question navigation respects dirty Design cancellation, then explicit discard focuses the exact unanswered record without changing saved context');

  stage = 'saved-versus-reviewed-answer';
  const retentionAnswer = 'Keep existing observations for the garden; remove the departed member’s access immediately.';
  await answer.fill(retentionAnswer);
  definition = await mutation(() => page.getByRole('button', { name: 'Save definition', exact: true }).click(), projectRoute + '/specification');
  assert.equal(definition.status, 'draft'); assert.equal(definition.questions[1].answer, retentionAnswer); assert.equal(definition.questions[0].answer, questions[0].answer);
  understanding = await overview(); await disclosure(understanding, 'Saved answers');
  await understanding.getByText('These saved answers need review with the current definition.', { exact: true }).waitFor();
  assert((await understanding.innerText()).includes(retentionAnswer)); await screenshot('02-saved-answer-needs-review');
  await page.reload(); await openProject('Define'); understanding = await overview(); await disclosure(understanding, 'Saved answers');
  await understanding.getByText('These saved answers need review with the current definition.', { exact: true }).waitFor();
  const savedAnswer = understanding.locator('details').filter({ has: page.getByText('Saved answers', { exact: true }) }).locator('div').filter({ has: page.getByRole('heading', { name: questions[1].question, exact: true }) });
  await page.setViewportSize({ width: 390, height: 844 });
  await savedAnswer.getByRole('button', { name: 'Edit answer in definition', exact: true }).click();
  await until('phone exact answer receives product focus', () => answer.evaluate(node => node === document.activeElement));
  assert.equal(await answer.inputValue(), retentionAnswer);
  const phoneAnswer = await answer.evaluate(node => { const r = node.getBoundingClientRect(); return { x: r.x, right: r.right, visibleHeight: Math.min(r.bottom, innerHeight) - Math.max(r.top, 0), viewportWidth: innerWidth }; });
  assert(phoneAnswer.x >= 0 && phoneAnswer.right <= phoneAnswer.viewportWidth + 1 && phoneAnswer.visibleHeight >= 24, 'Focused phone answer must be visible and contained');
  receipt.phoneAnswerFocus = phoneAnswer;
  await screenshot('03-saved-answer-edit-focused-390'); await auditScope('03-saved-answer-edit-focused-390', '.definition-workspace');
  await page.setViewportSize({ width: 1440, height: 1000 });
  definition = await mutation(() => page.getByRole('button', { name: 'Approve definition', exact: true }).click(), projectRoute + '/specification/approve');
  understanding = await overview(); await disclosure(understanding, 'Saved answers');
  await understanding.getByText(`Part of reviewed definition ${definition.revision}.`, { exact: true }).waitFor();
  assert.equal(definition.status, 'approved'); await screenshot('04-explicitly-reviewed-answer');
  const reviewedDesign = await reviewDesign(); assert.deepEqual(reviewedDesign.screens, originalDesign.screens); assert.equal(reviewedDesign.implementationNotes, originalNotes);
  check('Saved answers survive reload, remain labelled for review until explicit approval, and exact Answer editing is focused at 390px');

  stage = 'unchanged-verification-recovery';
  await page.getByRole('button', { name: 'Develop', exact: true }).click();
  let checks = await checksPanel();
  await revealPlanFromOverview();
  const facts = page.locator('.project-work-facts');
  await facts.getByText(`Definition revision ${definition.revision} · Approved.`, { exact: true }).waitFor();
  await facts.getByText(`Design handoff revision ${reviewedDesign.revision} · Approved.`, { exact: true }).waitFor();
  await facts.getByText(`1 planned screen · 3 documented states · based on definition ${definition.revision}.`, { exact: true }).waitFor();
  await facts.getByText('No optional sketch saved.', { exact: true }).waitFor();
  assert(!(await facts.innerText()).includes('No saved design yet'));
  await planningViewport('04b-canonical-reviewed-handoff-facts', facts.getByText(`Design handoff revision ${reviewedDesign.revision} · Approved.`, { exact: true }));
  receipt.handoffFacts = { definitionRevision: definition.revision, designRevision: reviewedDesign.revision, screens: reviewedDesign.screens.length, states: reviewedDesign.screens.flatMap(screen => screen.states).length, optionalSketchRevision: reviewedDesign.sketchRevision };
  const program = checks.getByRole('textbox', { name: 'Executable', exact: true }), argumentsField = checks.getByRole('textbox', { name: 'Arguments · JSON array', exact: true });
  await program.fill('node'); await argumentsField.fill('["--test","tests/retention.test.mjs"]'); await checks.getByRole('button', { name: 'Add check command', exact: true }).click();
  const unfinished = '["--test", "tests/unsaved';
  await program.fill('node'); await argumentsField.fill(unfinished);
  await disclosure(checks, 'Link checks to approved requirements · 0 selected');
  let criterion = checks.getByRole('checkbox', { name: 'Check AC-notes · Keep garden observations', exact: true });
  await until('reviewed criterion available', () => criterion.isEnabled()); await criterion.check();
  await checks.getByRole('combobox', { name: 'Check execution', exact: true }).selectOption('host'); await trust(checks).check();
  assert(await trust(checks).isChecked());
  await unsentCheckHelp(checks, unfinished);
  assert(await trust(checks).isChecked(), 'Opening help does not execute or change check consent');
  const unchangedCoverage = (await api('GET', projectRoute + '/checks')).coverage;
  await page.getByRole('button', { name: 'Design', exact: true }).click();
  guard = page.getByRole('dialog', { name: 'Keep your changes?', exact: true }); await guard.getByRole('button', { name: 'Keep editing', exact: true }).click();
  assert.equal(await argumentsField.inputValue(), unfinished); assert(await trust(checks).isChecked());
  await page.getByRole('button', { name: 'Design', exact: true }).click(); await guard.getByRole('button', { name: 'Keep drafts and continue', exact: true }).click();
  await page.reload(); await openProject('Develop'); checks = await checksPanel();
  let recovery = checks.getByRole('region', { name: 'Verification setup draft recovery', exact: true }); await recovery.waitFor();
  await disclosure(recovery, 'Compare draft with current check setup');
  // The comparison renders lazily after its native disclosure toggle. Await
  // the exact visible unfinished text before reading the comparison body.
  await recovery.getByText(unfinished, { exact: true }).waitFor();
  assert((await recovery.innerText()).includes(unfinished)); assert((await recovery.innerText()).includes('tests/retention.test.mjs'));
  await planningViewport('05-readable-check-draft-recovery-1440', checks.getByRole('heading', { name: 'Verify the implementation', exact: true }));
  await auditScope('05-readable-check-draft-recovery-1440', '.project-checks');
  await auditScope('05a-visible-check-heading-1440', '.project-checks > h2');
  await planningViewport('05b-visible-recovery-actions-1440', recovery.getByRole('button', { name: 'Restore draft', exact: true }));
  await auditScope('05b-visible-recovery-actions-1440', '.draft-recovery-panel > .journey-actions');
  assert.deepEqual((await api('GET', projectRoute + '/checks')).coverage, unchangedCoverage);
  await recovery.getByRole('button', { name: 'Restore draft', exact: true }).click();
  await until('restored executable', async () => await program.inputValue() === 'node'); assert.equal(await argumentsField.inputValue(), unfinished);
  assert.equal(await checks.getByRole('button', { name: 'Remove node', exact: true }).count(), 1);
  // Reload returns the native coverage disclosure to its closed default. Open
  // it normally before inspecting the restored visible criterion control.
  await disclosure(checks, 'Link checks to approved requirements · 1 selected');
  await until('matching-context coverage restored', () => criterion.isChecked()); await freshConsent(checks);
  await screenshot('06-restored-checks-with-fresh-consent-required');
  check('Custom command and unfinished argument text survive cancelled navigation, kept draft and reload; explicit restore retains matching coverage but resets host mode and execution consent');

  stage = 'amended-verification-recovery';
  await page.getByRole('button', { name: 'Define', exact: true }).click(); await guard.getByRole('button', { name: 'Keep drafts and continue', exact: true }).click();
  await page.getByRole('tab', { name: 'Specification', exact: true }).click();
  const amendedCriterion = 'A member saves an observation and can read the same text after reopening; departing members immediately lose access to retained observations.';
  await page.getByRole('textbox', { name: 'Acceptance scenario 1', exact: true }).fill(amendedCriterion);
  const amendedDraft = await mutation(() => page.getByRole('button', { name: 'Save definition', exact: true }).click(), projectRoute + '/specification');
  const amendedDefinition = await mutation(() => page.getByRole('button', { name: 'Approve definition', exact: true }).click(), projectRoute + '/specification/approve');
  assert.equal(amendedDefinition.requirements[0].acceptance, amendedCriterion); assert.equal(amendedDraft.status, 'draft');
  const amendedDesign = await reviewDesign(); assert.deepEqual(amendedDesign.screens, originalDesign.screens);
  await page.getByRole('button', { name: 'Develop', exact: true }).click(); checks = await checksPanel();
  recovery = checks.getByRole('region', { name: 'Verification setup draft recovery', exact: true }); await recovery.waitFor();
  await recovery.getByText('The source, reviewed context or selected build differs from this draft. Review the setup; outdated requirement selections will be cleared.', { exact: true }).waitFor();
  await disclosure(recovery, 'Compare draft with current check setup');
  await page.setViewportSize({ width: 390, height: 844 });
  await planningViewport('07-changed-context-recovery-review-390', recovery.getByText('Recover your verification setup draft', { exact: true }));
  await auditScope('07-changed-context-recovery-review-390', '.project-checks');
  await planningViewport('07a-visible-recovery-actions-390', recovery.getByRole('button', { name: 'Restore draft', exact: true }));
  await recovery.getByRole('checkbox', { name: 'I reviewed the current check setup and want to continue with this draft.', exact: true }).check();
  await recovery.getByRole('button', { name: 'Restore draft', exact: true }).click();
  await until('changed-context restore warning', async () => (await checks.innerText()).includes('review and select the criteria again.'));
  assert.equal(await argumentsField.inputValue(), unfinished); assert.equal(await program.inputValue(), 'node');
  await disclosure(checks, 'Link checks to approved requirements · 0 selected');
  criterion = checks.getByRole('checkbox', { name: 'Check AC-notes · Keep garden observations', exact: true }); assert.equal(await criterion.isChecked(), false);
  assert((await checks.innerText()).includes(amendedCriterion)); await freshConsent(checks);
  await planningViewport('08-amended-criteria-not-preselected-390', criterion);
  await checks.getByRole('button', { name: 'Discard verification setup', exact: true }).click();
  assert.equal(await argumentsField.inputValue(), '[]'); assert.equal(await program.inputValue(), '');
  assert.equal(await checks.getByRole('button', { name: 'Remove node', exact: true }).count(), 0);
  await page.setViewportSize({ width: 1440, height: 1000 });
  await page.reload(); await openProject('Develop'); checks = await checksPanel();
  assert.equal(await checks.getByRole('region', { name: 'Verification setup draft recovery', exact: true }).count(), 0);
  const checkHistory = await api('GET', projectRoute + '/checks'); assert.deepEqual(checkHistory.runs, []);
  const conversation = await api('GET', projectRoute + '/orchestrator'); assert.deepEqual(conversation.turns, []);
  receipt.final = { definition: amendedDefinition, design: amendedDesign, checkHistory, conversationTurns: conversation.turns };
  check('A real reviewed context amendment clears old criterion selections while retaining command text; explicit discard removes the browser draft and no execution or model job occurs');
  assert.equal(receipt.checks.length, 4); assert.equal(routeError, undefined); assert.deepEqual(receipt.browserErrors, []); assert.deepEqual(receipt.blockedRequests, []);
  assert(!receipt.browserMutations.some(row => /\/(checks|assist|runs)(\/|$)/.test(row.path)), 'No execution or model dispatch may occur');
  assert.equal(sha(readFileSync(binary)), binarySHA256); assert.deepEqual(inventory(), webFiles);
  receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error); receipt.failedStage = stage; if (routeError) receipt.fixtureError = scrub(routeError);
  if (page) { await screenshot('failure').catch(() => {}); await page.locator('body').innerText().then(body => writeFileSync(join(evidence, 'failure-dom.txt'), scrub(body), { mode: 0o600 })).catch(() => {}); }
  process.exitCode = 1;
} finally {
  await browser?.close().catch(error => { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); });
  let cleaned = false; try { await stop(); cleaned = true; } catch (error) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); }
  if (cleaned && receipt.passed) {
    try { rmSync(fixture, { recursive: true, force: true }); }
    catch (error) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); receipt.retainedFixture = fixture; }
  } else receipt.retainedFixture = fixture;
  if (receipt.cleanupErrors?.length) { receipt.passed = false; process.exitCode = 1; }
  receipt.finishedAt = new Date().toISOString();
  writeFileSync(join(evidence, 'service.log'), scrub(serviceLog), { mode: 0o600 }); writeFileSync(report, JSON.stringify(receipt, null, 2), { mode: 0o600 });
  console.log(JSON.stringify({ passed: receipt.passed, checks: receipt.checks.length, report, failedStage: receipt.failedStage }));
}

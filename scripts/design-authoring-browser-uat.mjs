#!/usr/bin/env node
// Installed fresh Design authoring. Baseline account/definition is API scaffolding;
// guidance, unsent help, screen authoring, saving and approval use the real UI.
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
import axe from '../desktop/node_modules/axe-core/axe.js';

const args = process.argv.slice(2);
if (args.includes('--help')) {
  console.log('node scripts/design-authoring-browser-uat.mjs --binary PATH --web PATH --chromium PATH --provenance PATH --report PATH\nInstalled fresh Design authoring with saved role/permission/acceptance guidance, existing unsent Luci help, an intentionally incomplete starter, a real draft save, concrete user-authored screen behavior and separate explicit approval. Account/project/reviewed definition are API scaffolding. No Docker, provider, model dispatch or invented design-quality scoring. New or empty evidence directory required.');
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
const fixture = mkdtempSync(join(tmpdir(), 'luci-design-authoring-'));
for (const name of ['home', 'projects', 'state']) mkdirSync(join(fixture, name), { mode: 0o700 });
const secrets = new Set(), secret = () => { const value = randomBytes(32).toString('base64url'); secrets.add(value); return value; };
const password = secret(), lifecycle = secret(), scrub = error => { let text = String(error?.message ?? error); for (const value of secrets) text = text.replaceAll(value, '[redacted]'); return text; };
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit,
  package: { archiveSHA256: provenance.archiveSHA256, binarySHA256, webIndexSHA256, allWebFilesMatchArchive: true, webFileCount: Object.keys(webFiles).length },
  driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  driverCheckout: { commit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: dirname(fileURLToPath(import.meta.url)), encoding: 'utf8' }).trim(), dirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: dirname(fileURLToPath(import.meta.url)), encoding: 'utf8' }).trim() },
  scope: 'Installed fresh Design authoring: understand saved people/outcomes, open existing unsent planning help, author screen behavior and separately approve the exact saved revision.',
  limitations: ['Account/project/reviewed role-and-outcome definition are API scaffolding; the fresh Design flow alone is claimed as UI-driven.', 'No Docker, provider/model response, source implementation, checks, release or model-quality claim.', 'Authored text is a deterministic human fixture; the product does not claim to grade its semantic quality or prescribe a screen architecture.', 'Bounded axe and rendered focus/reflow observations are not accessibility certification; incomplete findings remain separate for manual review.'],
  setupScaffolding: ['Private account and Git identity', 'One project with a reviewed three-outcome definition for garden coordinators and members, without any design screens'],
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
async function auditDesign(name) {
  await page.evaluate(axe.source);
  const workspace = page.locator('.design-workspace');
  const result = await workspace.evaluate(async node => {
    const rect = element => { const r = element.getBoundingClientRect(); return { x: r.x, y: r.y, width: r.width, height: r.height }; };
    const active = document.activeElement;
    const audit = await window.axe.run(node, { runOnly: ['wcag2a', 'wcag2aa', 'wcag21aa'] });
    const findings = rows => rows.map(row => ({ id: row.id, impact: row.impact, help: row.help, nodes: row.nodes.map(item => ({ target: item.target, failureSummary: item.failureSummary, review: [...item.any, ...item.all, ...item.none].map(check => ({ id: check.id, message: check.message, data: check.data })) })) }));
    return {
      viewport: { width: innerWidth, height: innerHeight }, documentScrollWidth: document.documentElement.scrollWidth,
      workspace: rect(node),
      focus: { insideDesign: node.contains(active), tag: active?.tagName, label: active?.getAttribute('aria-label') || active?.closest('label')?.textContent || (active?.tagName === 'BUTTON' ? active.textContent : ''), bounds: active ? rect(active) : null },
      violations: findings(audit.violations), incomplete: findings(audit.incomplete), passes: audit.passes.map(row => row.id),
    };
  });
  writeFileSync(join(evidence, name + '-accessibility.json'), JSON.stringify(result, null, 2), { mode: 0o600 });
  receipt.accessibility.push({ state: name, viewport: result.viewport, documentScrollWidth: result.documentScrollWidth, focus: result.focus, violations: result.violations.map(row => row.id), incomplete: result.incomplete.map(row => row.id) });
  assert(result.documentScrollWidth <= result.viewport.width + 1, name + ' has horizontal document overflow');
  assert.deepEqual(result.violations, [], name + ' has scoped automated accessibility violations');
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
  await page.getByRole('treeitem', { name: /design-authoring$/ }).click();
  await page.getByRole('button', { name: lens, exact: true }).click();
}
const saveButton = () => page.getByRole('button', { name: 'Save design plan', exact: true });
async function makePage() {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, serviceWorkers: 'block' });
  await context.route('**/*', async route => {
    try {
      const request = route.request(), url = new URL(request.url()), path = url.pathname, method = request.method();
      if (url.origin !== base) { receipt.blockedRequests.push({ method, origin: url.origin, path }); throw new Error('Unexpected external request'); }
      if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
        assert.equal(method, 'POST');
        assert(['/v1/auth/login', projectRoute + '/design-plan', projectRoute + '/design-plan/approve'].includes(path), 'Unexpected browser mutation: ' + path);
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
  project = await api('POST', '/v1/product/projects', { name: 'design-authoring' }, 201); projectRoute = '/v1/product/projects/' + project.id;
  const emptyDefinition = await api('GET', projectRoute + '/specification');
  const requirements = [
    { id: 'AC-assign', title: 'Coordinators assign garden tasks', priority: 'must', acceptance: 'A coordinator assigns a task to a member; a member cannot assign tasks to others.' },
    { id: 'AC-complete', title: 'Members complete their own tasks', priority: 'must', acceptance: 'A member sees their assigned tasks and records completion without editing another member\'s tasks.' },
    { id: 'AC-persist', title: 'Keep task changes after reopening', priority: 'must', acceptance: 'Assignments and completion remain when a coordinator or member returns; a failed save retains entered changes for retry.' },
  ];
  const roles = [
    { id: 'role-coordinator', name: 'Coordinator', canDo: 'View the shared garden task list and assign tasks to members.', cannotDo: 'Read private member notes.' },
    { id: 'role-member', name: 'Member', canDo: 'View and complete their own assigned tasks.', cannotDo: 'Assign tasks or change another member\'s task completion.' },
  ];
  let definition = await api('POST', projectRoute + '/specification', { expectedRevision: emptyDefinition.revision, expectedBriefRevision: emptyDefinition.briefRevision, specification: { ...emptyDefinition, purpose: 'Help a community garden coordinate durable shared tasks with clear permissions.', audience: 'Garden coordinators and invited members.', requirements, roles } });
  definition = await api('POST', projectRoute + '/specification/approve', { revision: definition.revision, briefRevision: definition.briefRevision });
  const initial = await api('GET', projectRoute + '/design-plan'); assert.equal(initial.screens.length, 0); assert.equal(initial.revision, 0);
  receipt.baseline = { projectID: project.id, definition, design: initial };
  stage = 'read-saved-guidance';
  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited }); receipt.browserVersion = browser.version(); page = await makePage();
  await page.goto(base); await page.getByLabel('Username', { exact: true }).fill('design-owner'); await page.getByLabel('Password', { exact: true }).fill(password); await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  const firstVisit = page.getByRole('button', { name: 'Continue to projects', exact: true }), projectItem = page.getByRole('treeitem', { name: /design-authoring$/ });
  await until('project entry', async () => await firstVisit.isVisible() || await projectItem.isVisible()); if (await firstVisit.isVisible()) await firstVisit.click();
  await openProject('Design');
  const guide = page.getByRole('region', { name: 'Plan the experience', exact: true });
  await guide.waitFor(); await auditDesign('00-manual-or-assisted-entry-1440'); await screenshot('00-manual-or-assisted-entry-1440');
  const contextSummary = guide.getByText('People, permissions and outcomes from your definition', { exact: true });
  await contextSummary.click();
  for (const text of [definition.audience, ...roles.flatMap(role => [role.name, role.canDo, role.cannotDo]), ...requirements.flatMap(row => [row.title, row.acceptance])]) assert((await guide.innerText()).includes(text));
  await screenshot('01-saved-people-and-outcomes-1440');
  await page.setViewportSize({ width: 390, height: 844 }); await guide.scrollIntoViewIfNeeded();
  assert(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth + 1), 'Phone layout overflows horizontally');
  await screenshot('02-saved-guidance-390'); await contextSummary.click();
  const help = guide.getByRole('button', { name: 'Ask Luci to help plan the experience', exact: true }); await help.scrollIntoViewIfNeeded();
  await auditDesign('03-manual-or-assisted-entry-390'); await screenshot('03-manual-or-assisted-entry-390'); await page.setViewportSize({ width: 1440, height: 1000 });
  check('Fresh Design exposes actual saved audience, role boundaries and acceptance at 1440/390, with a compact disclosure and manual or assisted next step');

  stage = 'unsent-existing-help';
  await help.click();
  const message = page.getByRole('textbox', { name: 'Message Luci', exact: true });
  await until('unsent design help request', async () => (await message.inputValue()).includes('each person or role'));
  assert((await message.inputValue()).includes('design-plan draft for my review'));
  assert(await page.getByRole('button', { name: 'Send to Luci', exact: true }).isDisabled(), 'A fixture with no provider must not appear ready to send');
  const ownMessage = 'Keep my existing question: should members also be able to uncomplete their own tasks?';
  await message.fill(ownMessage); await help.click(); assert.equal(await message.inputValue(), ownMessage);
  await page.getByText('Your unsent message is still here. Send or clear it before starting another question.', { exact: true }).waitFor();
  assert.deepEqual(await api('GET', projectRoute + '/design-plan'), initial);
  assert.equal(receipt.browserMutations.filter(row => row.path !== '/v1/auth/login').length, 0);
  await screenshot('04-existing-unsent-message-preserved');
  await page.getByRole('button', { name: 'Hide project overview', exact: true }).click();
  check('Planning help opens the existing editable composer, preserves a prior message and sends nothing; missing provider remains an explicit boundary');

  stage = 'unfinished-starter';
  await page.getByRole('button', { name: 'Describe the first screen', exact: true }).click();
  const name = page.getByRole('textbox', { name: 'Screen name', exact: true });
  assert(await name.evaluate(node => document.activeElement === node), 'Manual authoring should focus the new screen name');
  for (const label of ['Screen name', 'Route', 'What this screen helps someone do', 'States and recovery', 'Accessibility and interaction']) assert.equal(await page.getByRole('textbox', { name: label, exact: true }).inputValue(), '');
  for (const requirement of requirements) assert.equal(await page.getByRole('checkbox', { name: `${requirement.id} · ${requirement.title}`, exact: true }).isChecked(), false);
  assert(await saveButton().isDisabled()); assert.equal(await page.getByRole('button', { name: 'Continue to Develop', exact: true }).count(), 0);
  assert.deepEqual(await api('GET', projectRoute + '/design-plan'), initial);
  await screenshot('05-unfinished-screen-needs-authored-decisions');
  await name.fill('Garden tasks');
  const partial = await mutation(() => saveButton().click(), projectRoute + '/design-plan');
  await page.getByText(`Design plan ${partial.revision} saved. Implementation context updated.`, { exact: true }).waitFor();
  assert.equal(partial.status, 'draft'); assert.equal(partial.screens[0].purpose, ''); assert.deepEqual(partial.screens[0].states, []); assert.deepEqual(partial.screens[0].requirementIds, []);
  assert(await page.getByRole('button', { name: 'Approve design handoff', exact: true }).isDisabled());
  assert((await page.getByRole('region', { name: 'Design decisions still needed', exact: true }).innerText()).includes('what people do here and how their access differs'));
  assert.equal((await api('GET', projectRoute + '/orchestrator')).nextAction.lens, 'design');
  await auditDesign('06-named-incomplete-draft-1440'); await screenshot('06-named-draft-is-not-an-approved-handoff');
  await page.setViewportSize({ width: 390, height: 844 }); await name.click();
  assert(await name.evaluate(node => document.activeElement === node), 'Named draft remains normally focusable on a phone viewport');
  await auditDesign('06-named-incomplete-draft-390'); await screenshot('06-named-incomplete-draft-390'); await page.setViewportSize({ width: 1440, height: 1000 });
  check('Untouched starter invents no screen behavior or coverage; a named unfinished draft can save but cannot approve or advance the project to Develop');

  stage = 'author-role-behavior';
  const purpose = 'Coordinators see the shared garden task list and assign a task to a member. Members see and complete only their assigned tasks, without assignment controls or other members\' private notes. Saved assignments and completion remain when either role returns.';
  const states = [
    'First visit: a coordinator sees an empty task list and an Assign task action; a member without assignments sees who to contact.',
    'Loading: keep the list labelled as loading and do not present missing tasks as an empty result.',
    'Saved assignment: the coordinator sees the assignee; that member sees the task after reopening.',
    'Saved completion: the member sees the task completed and the coordinator sees its shared completion state after returning.',
    'Save failure: retain the entered assignment or completion change, explain that it is not saved and offer retry.',
    'Permission denied: a member opening the assignment route sees an access explanation and can return to their own tasks.',
  ];
  const accessibility = 'Label the assignee and task fields; keyboard users can assign or complete permitted tasks and receive visible focus and save/error status.';
  await page.getByRole('textbox', { name: 'Route', exact: true }).fill('/tasks');
  await page.getByRole('textbox', { name: 'What this screen helps someone do', exact: true }).fill(purpose);
  await page.getByRole('textbox', { name: 'States and recovery', exact: true }).fill(states.join('\n'));
  await page.getByRole('textbox', { name: 'Accessibility and interaction', exact: true }).fill(accessibility);
  for (const requirement of requirements) await page.getByRole('checkbox', { name: `${requirement.id} · ${requirement.title}`, exact: true }).check();
  const saved = await mutation(() => saveButton().click(), projectRoute + '/design-plan');
  await page.getByText(`Design plan ${saved.revision} saved. Implementation context updated.`, { exact: true }).waitFor();
  const expectedScreen = { ...partial.screens[0], route: '/tasks', purpose, states, accessibility, requirementIds: requirements.map(row => row.id) };
  assert.deepEqual(saved.screens, [expectedScreen]); assert.equal(saved.status, 'draft');
  assert.equal(receipt.browserMutations.filter(row => row.path === projectRoute + '/design-plan/approve').length, 0);
  await page.reload(); await openProject('Design');
  assert.equal(await page.getByRole('textbox', { name: 'What this screen helps someone do', exact: true }).inputValue(), purpose);
  assert.equal(await page.getByRole('textbox', { name: 'States and recovery', exact: true }).inputValue(), states.join('\n'));
  assert.deepEqual(await api('GET', projectRoute + '/design-plan'), saved);
  await screenshot('07-authored-role-behavior-saved-awaiting-review');
  const approved = await mutation(() => page.getByRole('button', { name: 'Approve design handoff', exact: true }).click(), projectRoute + '/design-plan/approve');
  assert.equal(approved.status, 'approved'); assert.deepEqual(approved.screens, saved.screens); assert.equal(approved.revision, saved.revision + 1);
  await page.getByRole('button', { name: 'Continue to Develop', exact: true }).waitFor();
  assert.equal((await api('GET', projectRoute + '/orchestrator')).nextAction.lens, 'develop');
  receipt.final = { partial, saved, approved };
  check('Authored coordinator/member behavior, persistence, permission denial and recovery survive reload; only separate explicit review advances to Develop');
  assert.deepEqual(receipt.browserMutations.map(row => row.path), ['/v1/auth/login', projectRoute + '/design-plan', projectRoute + '/design-plan', projectRoute + '/design-plan/approve']);
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
  if (cleaned && receipt.passed) {
    try { rmSync(fixture, { recursive: true, force: true }); }
    catch (error) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); receipt.retainedFixture = fixture; }
  } else receipt.retainedFixture = fixture;
  if (receipt.cleanupErrors?.length) { receipt.passed = false; process.exitCode = 1; }
  receipt.finishedAt = new Date().toISOString();
  writeFileSync(join(evidence, 'service.log'), scrub(serviceLog), { mode: 0o600 }); writeFileSync(report, JSON.stringify(receipt, null, 2), { mode: 0o600 });
  console.log(JSON.stringify({ passed: receipt.passed, checks: receipt.checks.length, report, failedStage: receipt.failedStage }));
}

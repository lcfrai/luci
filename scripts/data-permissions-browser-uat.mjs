#!/usr/bin/env node
// Installed data-permission rendering with real team grants. Display-only status
// fixtures are labelled and authorized first. No database, Docker or model work.
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
  console.log('node scripts/data-permissions-browser-uat.mjs --binary PATH --web PATH --chromium PATH --provenance PATH --report PATH\nInstalled UI permission acceptance with real local accounts, team/project grants and permission refreshes. Only already-authorized GET data/platform responses receive labelled display fixtures. Holds and fails permission response delivery to prove draft retention without cached authority. Blocks all workspace mutations, uses an absent Docker socket, and never allocates a database. No claim of actual database restore, Supabase auth/RLS, app data or model quality. Report directory must be new or empty.');
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
const fixture = mkdtempSync(join(tmpdir(), 'luci-data-permissions-'));
for (const name of ['home', 'projects', 'state']) mkdirSync(join(fixture, name), { mode: 0o700 });
const secrets = new Set(), secret = () => { const value = randomBytes(32).toString('base64url'); secrets.add(value); return value; };
const password = secret(), lifecycle = secret(), scrub = error => { let text = String(error?.message ?? error); for (const value of secrets) text = text.replaceAll(value, '[redacted]'); return text; };
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit,
  package: { archiveSHA256: provenance.archiveSHA256, binarySHA256, webIndexSHA256, allWebFilesMatchArchive: true, webFileCount: Object.keys(webFiles).length },
  driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  scope: 'Installed data-permission rendering with real authentication, team membership and project grant changes; database/status results are labelled browser-local display fixtures, not operational receipts.',
  limitations: ['No Docker, database, restore, rollback or resource allocation occurs.', 'No actual application users, row-level security, durable app data, Supabase, external model or cloud claim.', 'Database recovery navigation is exercised using a labelled succeeded display fixture; actual PostgreSQL backup/restore/restart has separate installed 7ff66d3b evidence.', 'Temporary pending and failed permission responses are explicit browser transport fault injection after successful real server authorization.', 'No-new-private-read checks observe bounded 250ms windows; they are not a long-duration polling guarantee.'],
  setupScaffolding: ['API-created isolated owner/member accounts, one team, empty personal entry projects and one shared project.', 'Team membership permits editing/deployment/secrets while narrower real project grants move through denied, private viewer, editor and revoked-secret states.', 'Platform Docker/status, saved database recovery and empty application tables are display-only fixtures substituted after real endpoint authorization.'],
  checks: [], permissions: [], fieldObservations: [], fixtureResponses: [], requests: [], browserMutations: [], blockedRequests: [], browserErrors: [], cleanup: [],
  externalModelCalls: 0, cloudResourcesCreated: 0, dockerAllocations: 0,
};
const inherited = Object.fromEntries(Object.entries(process.env).filter(([name]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ)$/.test(name)));
let service, browser, page, base, token, member, team, project, workspace, accessPath, platformPath, stage = 'startup', spawnError, serviceLog = '', routeError;
let permissionMode = 'pass', releaseHeld = () => {}, heldPermission;
const check = (name, details = {}) => { receipt.checks.push({ name, ...details }); console.log('PASS ' + name); };
async function port() { const server = createServer(); await new Promise((done, reject) => server.once('error', reject).listen(0, '127.0.0.1', done)); const value = server.address().port; await new Promise(done => server.close(done)); return value; }
async function api(method, route, body, expected = 200, auth = token) {
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
const privateRequests = () => receipt.requests.filter(row => row.path === platformPath + '/data' || row.path.startsWith(platformPath + '/data/'));
const writes = () => receipt.browserMutations.filter(row => row.path !== '/v1/auth/login');
const dataPanel = () => page.getByRole('region', { name: 'Project data platform', exact: true });
const postgresPanel = () => page.getByRole('region', { name: 'Standalone PostgreSQL', exact: true });
const platformTabs = () => page.getByRole('navigation', { name: 'Platform sections', exact: true });
const missingAccess = 'Ask a team owner for project read, deployment and secrets access to inspect application data and backups.';
const missingDeployment = 'Deployment access is required to build, stop, restart or check deployment health. Ask a team owner for editing and deployment access.';
const missingLogs = 'Application logs require deployment and secrets access. Ask a team owner for those permissions.';
const missingPreview = 'Opening private application previews requires execution access.';
async function noConfirmedMissingGrants() {
  for (const message of [missingDeployment, missingLogs, missingPreview]) assert.equal(await page.getByText(message, { exact: true }).count(), 0, 'Uncertain permissions were presented as a confirmed missing grant: ' + message);
}
const readOnly = 'Read-only data access. Ask a team owner for editing access to change data services, application accounts, records and backups.';
const fixtureMessage = 'LOCAL DISPLAY FIXTURE: no data services or database have been created.';
const recoveryMessage = 'LOCAL DISPLAY FIXTURE: simulated verified recovery for navigation only; no database was restored.';
async function screenshot(name) { await page.screenshot({ path: join(evidence, name + '.png'), fullPage: true }); }
async function captureTableFields(tableName, originalInput) {
  const columnType = page.getByRole('combobox', { name: 'Column 1 type', exact: true });
  const tableSection = page.getByRole('region', { name: 'Application tables', exact: true });
  const originalViewport = page.viewportSize();
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: width === 390 ? 844 : 1080 });
    // Use the actual keyboard sequence from the disclosure into its fields.
    await tableSection.getByText('Create a table', { exact: true }).focus(); await page.keyboard.press('Tab');
    assert(await tableName.evaluate(node => node === document.activeElement), 'Table name is not reachable by keyboard');
    for (const [name, field] of [['table-name', tableName], ['column-type', columnType]]) {
      if (name === 'column-type') { await page.keyboard.press('Tab'); await page.keyboard.press('Tab'); }
      await field.scrollIntoViewIfNeeded();
      const measured = await field.evaluate(node => {
        const box = node.getBoundingClientRect(), style = getComputedStyle(node), label = node.closest('label');
        const texts = [...label.childNodes].filter(child => child.nodeType === Node.TEXT_NODE && child.textContent.trim());
        const range = document.createRange(); range.setStart(texts[0], 0); range.setEnd(texts.at(-1), texts.at(-1).textContent.length); const labelText = range.getBoundingClientRect();
        const rect = value => ({ x: value.x, y: value.y, width: value.width, height: value.height, right: value.right, bottom: value.bottom });
        const clip = { left: 0, top: 0, right: innerWidth, bottom: innerHeight };
        for (let parent = node.parentElement; parent; parent = parent.parentElement) {
          const bounds = parent.getBoundingClientRect(), css = getComputedStyle(parent);
          if (/(auto|scroll|hidden|clip)/.test(css.overflowX)) { clip.left = Math.max(clip.left, bounds.left + parent.clientLeft); clip.right = Math.min(clip.right, bounds.left + parent.clientLeft + parent.clientWidth); }
          if (/(auto|scroll|hidden|clip)/.test(css.overflowY)) { clip.top = Math.max(clip.top, bounds.top + parent.clientTop); clip.bottom = Math.min(clip.bottom, bounds.top + parent.clientTop + parent.clientHeight); }
        }
        const hit = document.elementFromPoint(box.x + box.width / 2, box.y + box.height / 2);
        return { box: rect(box), labelText: rect(labelText), clip, unobscured: hit === node || node.contains(hit), viewport: { width: innerWidth, height: innerHeight }, focused: document.activeElement === node,
          boxSizing: style.boxSizing, fontSize: parseFloat(style.fontSize), minSize: parseFloat(style.getPropertyValue('--field-min-size')),
          paddingTop: parseFloat(style.paddingTop), paddingBottom: parseFloat(style.paddingBottom), shadow: style.boxShadow,
          focusVisible: node.matches(':focus-visible'), outlineStyle: style.outlineStyle, outlineWidth: parseFloat(style.outlineWidth), outlineOffset: parseFloat(style.outlineOffset),
          formWidth: node.closest('form').clientWidth, formScrollWidth: node.closest('form').scrollWidth, documentWidth: document.documentElement.scrollWidth };
      });
      receipt.fieldObservations.push({ width, field: name, ...measured, scope: 'Rendered existing table form after locator scrolling; no schema submission or database operation.' });
      assert(measured.focused && measured.focusVisible && measured.outlineStyle !== 'none' && measured.outlineWidth >= 2, name + ' lacks visible keyboard focus');
      assert.equal(measured.boxSizing, 'border-box'); assert(measured.box.height >= Math.max(36, measured.minSize), name + ' is shorter than the selected Luci field size');
      assert(measured.fontSize >= 14 && measured.paddingTop >= 7 && measured.paddingBottom >= 7 && measured.shadow !== 'none', name + ' lacks readable field styling');
      assert(measured.labelText.bottom + 4 <= measured.box.y, name + ' label is not clearly separated from its control');
      const ring = measured.outlineWidth + measured.outlineOffset;
      assert(measured.unobscured && measured.box.x - ring >= measured.clip.left - 1 && measured.box.right + ring <= measured.clip.right + 1 && measured.box.y - ring >= measured.clip.top - 1 && measured.box.bottom + ring <= measured.clip.bottom + 1, name + ' or its focus ring is obscured by the viewport or a scroll container');
      assert(measured.labelText.x >= measured.clip.left - 1 && measured.labelText.right <= measured.clip.right + 1 && measured.labelText.y >= measured.clip.top - 1, name + ' label is clipped');
      assert(measured.formScrollWidth <= measured.formWidth + 1 && measured.documentWidth <= width + 1, 'Table fields overflow at ' + width);
      assert(await tableName.evaluate((node, original) => node === original, originalInput)); assert.equal(await tableName.inputValue(), 'pending_private_table');
      await screenshot('02-table-fields-' + width + '-' + name + '-display-fixture');
    }
    const checkbox = await tableSection.getByRole('checkbox', { name: 'Optional', exact: true }).boundingBox();
    assert(checkbox && checkbox.width === 18 && checkbox.height === 18, 'Field styling changed the optional-column checkbox geometry');
  }
  await page.setViewportSize(originalViewport);
}
async function grant(role, grants) {
  const latest = await api('GET', `/v1/teams/${team.id}`);
  const updated = await api('PUT', accessPath + '/' + member.user.id, { expectedRevision: latest.team.revision, role, grants: { execute: false, admin: false, ...grants } });
  const actual = await api('GET', accessPath, undefined, 200, member.token);
  receipt.permissions.push({ stage, revision: updated.team.revision, permissions: actual.permissions });
  assert.equal(actual.projectId, project.id);
  return actual.permissions;
}
async function refreshPermissions() {
  const count = receipt.requests.filter(row => row.path === accessPath).length;
  // Exercise the real provider's focus refresh without changing application state.
  await page.evaluate(() => window.dispatchEvent(new Event('focus')));
  await until('a fresh real permission request', () => receipt.requests.filter(row => row.path === accessPath).length > count);
}
function dataFixture(original) {
  return { ...original, projectId: project.id, provider: 'supabase', version: 'display-fixture', revision: 1, status: 'ready', message: fixtureMessage,
    services: [], backups: [], capabilities: {}, requirements: { memoryMiB: 4096, cpuMillis: 2000, diskGiB: 40 }, priorStackRetained: false, initializationState: 'complete' };
}
async function makePage() {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1080 }, serviceWorkers: 'block' });
  await context.route('**/*', async route => {
    try {
      const request = route.request(), url = new URL(request.url()), path = url.pathname, method = request.method();
      if (url.origin !== base) { receipt.blockedRequests.push({ method, origin: url.origin, path }); return await route.abort('blockedbyclient'); }
      receipt.requests.push({ stage, method, path });
      if (!['GET', 'HEAD', 'OPTIONS'].includes(method)) {
        receipt.browserMutations.push({ stage, method, path });
        // A bug in a supposedly read-only flow must not allocate resources.
        assert.equal(path, '/v1/auth/login', 'Unexpected browser mutation: ' + path);
        return await route.continue();
      }
      const fixtureRoute = path === accessPath || path === workspace + '/v1/platform/status' || path === platformPath || path === platformPath + '/data' || path === platformPath + '/data/tables';
      if (!fixtureRoute) return await route.continue();
      const original = await route.fetch(), backendStatus = original.status();
      if (path === accessPath) {
        const mode = permissionMode;
        receipt.permissions.push({ stage, request: 'browser', backendStatus, delivery: mode });
        if (mode === 'hold') { assert.equal(backendStatus, 200); await heldPermission; }
        if (mode === 'fail') { assert.equal(backendStatus, 200); return await route.fulfill({ status: 503, json: { error: 'labelled_transport_fixture', message: 'Temporary permission transport failure (browser fixture).' } }); }
        return await route.fulfill({ response: original });
      }
      // Never turn authorization failures into an allowed display result.
      if ([401, 403, 404].includes(backendStatus)) return await route.fulfill({ response: original });
      let value;
      if (path === platformPath + '/data/tables') {
        assert.equal(backendStatus, 409); assert.equal((await original.json()).error, 'data_not_ready'); value = { tables: [] };
      } else {
        assert.equal(backendStatus, 200); value = await original.json();
        if (path === workspace + '/v1/platform/status') value = { ...value, dockerReady: true, scope: fixtureMessage };
        else if (path === platformPath + '/data') { assert.equal(value.status, 'not-installed'); value = dataFixture(value); }
        else {
          assert.equal(value.project.projectId, project.id); assert.equal(value.project.deployments.length, 0);
          value = { ...value, busy: false, project: { ...value.project, database: { status: 'ready', engine: 'PostgreSQL display fixture', message: fixtureMessage },
            databaseOperation: { id: 'display-only-recovery', action: 'restore', status: 'succeeded', stage: 'complete', message: recoveryMessage, createdAt: '2026-10-06T00:00:00Z', updatedAt: '2026-10-06T00:00:00Z' } } };
        }
      }
      receipt.fixtureResponses.push({ stage, method, path, backendStatus, label: 'Authorized browser-local display only; no resource operation' });
      return await route.fulfill({ response: original, status: 200, json: value });
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
  const owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'data-owner', displayName: 'Data permission owner', password }, 201, ''); token = owner.token;
  await api('POST', '/v1/admin/users', { username: 'data-member', displayName: 'Data permission member', password }, 201);
  member = await api('POST', '/v1/auth/login', { username: 'data-member', password }, 200, '');
  await api('POST', '/v1/product/projects', { name: 'owner-entry-seed' }, 201);
  await api('POST', '/v1/product/projects', { name: 'member-entry-seed' }, 201, member.token);
  team = (await api('POST', '/v1/teams', { name: 'Data permissions fixture' }, 201)).team;
  team = (await api('POST', `/v1/teams/${team.id}/members`, { expectedRevision: team.revision, username: 'data-member', role: 'editor', grants: { execute: false, secrets: true, deploy: true, admin: false } })).team;
  workspace = `/v1/teams/${team.id}/workspace`;
  project = await api('POST', workspace + '/v1/product/projects', { name: 'data-permissions' }, 201);
  accessPath = `/v1/teams/${team.id}/projects/${project.id}/members`; platformPath = workspace + `/v1/platform/projects/${project.id}`;
  stage = 'private-read-denied';
  assert.deepEqual(await grant('viewer', { secrets: false, deploy: false }), { read: true, edit: false, execute: false, secrets: false, deploy: false, admin: false });
  await api('GET', platformPath + '/data', undefined, 403, member.token);
  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited }); receipt.browserVersion = browser.version(); page = await makePage();
  await page.goto(base); await page.getByLabel('Username', { exact: true }).fill('data-member'); await page.getByLabel('Password', { exact: true }).fill(password); await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  const firstVisit = page.getByRole('button', { name: 'Continue to projects', exact: true }), workspaceChoice = page.getByLabel('Workspace', { exact: true });
  await until('project entry or workspace choice', async () => await firstVisit.isVisible() || await workspaceChoice.isVisible());
  if (await firstVisit.isVisible()) await firstVisit.click();
  await workspaceChoice.selectOption(team.id); await page.getByRole('treeitem', { name: /data-permissions$/ }).click();
  await page.getByRole('button', { name: 'Deploy', exact: true }).click(); await page.getByRole('tab', { name: 'Application platform', exact: true }).click();
  await platformTabs().getByRole('button', { name: 'Data', exact: true }).click(); await page.getByText('Standalone PostgreSQL database', { exact: true }).click();
  await dataPanel().getByText(missingAccess, { exact: true }).waitFor(); await postgresPanel().getByText(missingAccess, { exact: true }).waitFor();
  for (const message of [missingDeployment, missingLogs, missingPreview]) await page.getByText(message, { exact: true }).waitFor();
  assert.equal(await page.getByText('Checking platform permissions…', { exact: true }).count(), 0);
  assert.equal(await page.getByText(recoveryMessage, { exact: true }).count(), 0); assert.equal(privateRequests().length, 0); assert.equal(writes().length, 0);
  await screenshot('01-private-data-denied'); check('Real private-read denial hides sensitive status and sends no private data requests', { backendDataGET: 403 });

  stage = 'private-viewer'; const viewer = await grant('viewer', { secrets: true, deploy: true }); assert(viewer.read && viewer.secrets && viewer.deploy && !viewer.edit);
  await refreshPermissions(); await dataPanel().getByText(readOnly, { exact: true }).waitFor(); await dataPanel().getByText(fixtureMessage, { exact: true }).waitFor();
  await postgresPanel().getByText(recoveryMessage, { exact: true }).waitFor();
  for (const name of ['Start data services', 'Stop data services', 'Check service health']) assert(await dataPanel().getByRole('button', { name, exact: true }).isDisabled());
  assert(await postgresPanel().getByRole('button', { name: 'Stop PostgreSQL', exact: true }).isDisabled());
  await dataPanel().getByRole('button', { name: 'Backups', exact: true }).click(); assert(await dataPanel().getByRole('button', { name: 'Create data backup', exact: true }).isDisabled());
  const beforeRead = privateRequests().length; await dataPanel().getByRole('button', { name: 'Refresh saved data status', exact: true }).click();
  await until('read-only saved status refresh', () => privateRequests().length > beforeRead);
  await until('saved status refresh completed', () => dataPanel().getByRole('button', { name: 'Refresh saved data status', exact: true }).isEnabled());
  assert.equal(writes().length, 0); await screenshot('02-private-viewer-display-fixtures'); check('Real private viewer can read saved data but cannot mutate services, health or backups', { realAuthorizedPrivateReads: privateRequests().length });

  stage = 'recovery-navigation'; const beforeNavigation = writes().length;
  await postgresPanel().getByRole('button', { name: 'Open application deployments', exact: true }).click();
  assert.equal(await platformTabs().getByRole('button', { name: 'Deployments', exact: true }).getAttribute('aria-pressed'), 'true'); assert.equal(writes().length, beforeNavigation);
  check('Labelled completed recovery offers an actual read-only route to application deployments without deployment or restore', { displayFixtureOnly: true });
  await platformTabs().getByRole('button', { name: 'Data', exact: true }).click();
  await dataPanel().getByRole('button', { name: 'Tables', exact: true }).click(); await page.getByText('Create a table', { exact: true }).click();
  const tableName = page.getByRole('textbox', { name: 'Table name', exact: true }), columnName = page.getByRole('textbox', { name: 'Column 1 name', exact: true }), createTable = page.getByRole('button', { name: 'Create table with RLS', exact: true });
  assert(await tableName.isDisabled()); assert(await createTable.isDisabled());

  stage = 'editor-draft'; const editor = await grant('editor', { secrets: true, deploy: true }); assert(editor.edit && editor.secrets && editor.deploy && !editor.execute);
  await refreshPermissions(); await until('editing enabled by the actual project grant', () => tableName.isEnabled());
  await tableName.fill('pending_private_table'); await columnName.fill('private_draft_note'); assert(await createTable.isEnabled());
  const originalInput = await tableName.elementHandle(); assert(originalInput);
  await captureTableFields(tableName, originalInput);
  check('Real editing grant enables schema controls while execution remains ungranted; no table is submitted');

  stage = 'permission-refresh-pending'; permissionMode = 'hold'; heldPermission = new Promise(resolve => { releaseHeld = resolve; });
  await refreshPermissions(); await dataPanel().getByText('Checking data permissions…', { exact: true }).first().waitFor();
  await page.getByText('Checking platform permissions…', { exact: true }).waitFor(); await noConfirmedMissingGrants();
  await until('writes disabled during permission refresh', () => createTable.isDisabled());
  assert(await tableName.evaluate((node, original) => node === original, originalInput)); assert.equal(await tableName.inputValue(), 'pending_private_table'); assert.equal(await columnName.inputValue(), 'private_draft_note');
  assert(await page.getByRole('button', { name: 'Refresh tables', exact: true }).isDisabled());
  const refreshReads = privateRequests().length; await delay(250); assert.equal(privateRequests().length, refreshReads, 'Private reads continued under pending authority'); assert.equal(writes().length, 0);
  await screenshot('03-pending-permissions-retain-draft-display-fixture'); permissionMode = 'pass'; releaseHeld();
  await until('fresh real permission response restores editing', () => createTable.isEnabled()); assert(await tableName.evaluate((node, original) => node === original, originalInput)); assert.equal(await tableName.inputValue(), 'pending_private_table');
  check('Pending real permission refresh retains the same draft DOM while writes and new private reads remain locked');

  stage = 'permission-refresh-error'; permissionMode = 'fail'; await refreshPermissions();
  await dataPanel().getByText(/Data access unavailable:/).first().waitFor(); await until('writes disabled after temporary permission response failure', () => createTable.isDisabled());
  const uncertainty = page.getByText(/^Platform permissions could not be confirmed\./);
  await uncertainty.waitFor(); assert((await uncertainty.textContent()).includes('Temporary permission transport failure (browser fixture).'));
  assert.equal(await page.getByText('Checking platform permissions…', { exact: true }).count(), 0); await noConfirmedMissingGrants();
  assert(await tableName.evaluate((node, original) => node === original, originalInput)); assert.equal(await tableName.inputValue(), 'pending_private_table'); assert.equal(await columnName.inputValue(), 'private_draft_note');
  assert(await page.getByRole('button', { name: 'Refresh tables', exact: true }).isDisabled());
  const failedReads = privateRequests().length; await delay(250); assert.equal(privateRequests().length, failedReads); assert.equal(writes().length, 0);
  await screenshot('04-transient-error-retains-draft-display-fixture'); permissionMode = 'pass'; await refreshPermissions();
  await until('permission retry restores explicit editing authority', () => createTable.isEnabled()); assert(await tableName.evaluate((node, original) => node === original, originalInput)); assert.equal(await tableName.inputValue(), 'pending_private_table');
  check('Labelled temporary permission transport failure retains the same unsaved draft but no authority; real refresh recovers it');

  stage = 'confirmed-revocation'; const revoked = await grant('editor', { secrets: false, deploy: true }); assert(revoked.read && revoked.edit && revoked.deploy && !revoked.secrets);
  await refreshPermissions(); await dataPanel().getByText(missingAccess, { exact: true }).waitFor();
  await page.getByText(missingLogs, { exact: true }).waitFor(); await page.getByText(missingPreview, { exact: true }).waitFor();
  assert.equal(await page.getByText(missingDeployment, { exact: true }).count(), 0, 'Revoking secrets must not imply deployment editing was revoked');
  assert.equal(await page.getByText(/^Platform permissions could not be confirmed\./).count(), 0);
  assert.equal(await tableName.count(), 0); assert.equal(await columnName.count(), 0); assert.equal(await page.getByText(recoveryMessage, { exact: true }).count(), 0);
  const revokedReads = privateRequests().length; await delay(250); assert.equal(privateRequests().length, revokedReads);
  await api('GET', platformPath + '/data', undefined, 403, member.token); await screenshot('05-confirmed-revocation-hides-private-state');
  check('Confirmed real secrets revocation clears private schema draft and recovery state; backend denies access and UI makes no new private reads', { backendDataGET: 403 });
  assert.equal(routeError, undefined); assert.deepEqual(receipt.browserErrors, []); assert.deepEqual(receipt.blockedRequests, []); assert.deepEqual(writes(), []);
  const actual = await api('GET', platformPath); assert.equal(actual.project.database.status, 'not-created'); assert.equal(actual.project.deployments.length, 0); assert.equal(actual.project.backups.length, 0); assert(!actual.project.databaseOperation);
  const actualData = await api('GET', platformPath + '/data'); assert.equal(actualData.status, 'not-installed');
  assert.equal(sha(readFileSync(binary)), binarySHA256); assert.deepEqual(inventory(), webFiles);
  receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error); receipt.failedStage = stage; if (routeError) receipt.fixtureError = scrub(routeError);
  if (page) { await screenshot('failure').catch(() => {}); await page.locator('body').innerText().then(body => writeFileSync(join(evidence, 'failure-dom.txt'), scrub(body), { mode: 0o600 })).catch(() => {}); }
  process.exitCode = 1;
} finally {
  permissionMode = 'pass'; releaseHeld();
  await browser?.close().catch(error => { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); });
  let cleaned = false; try { await stop(); cleaned = true; } catch (error) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); }
  if (cleaned) rmSync(fixture, { recursive: true, force: true }); else receipt.retainedFixture = fixture;
  if (receipt.cleanupErrors?.length) { receipt.passed = false; process.exitCode = 1; }
  receipt.finishedAt = new Date().toISOString();
  writeFileSync(join(evidence, 'service.log'), scrub(serviceLog), { mode: 0o600 }); writeFileSync(report, JSON.stringify(receipt, null, 2), { mode: 0o600 });
  console.log(JSON.stringify({ passed: receipt.passed, checks: receipt.checks.length, report, failedStage: receipt.failedStage }));
}

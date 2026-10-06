#!/usr/bin/env node
// Installed billing-return navigation with real accounts/team permissions and
// explicitly local browser fixtures for billing presentation. Never runs Stripe.
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

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..'), args = process.argv.slice(2);
if (args.includes('--help')) {
  console.log('node scripts/billing-return-browser-uat.mjs --binary PATH --web PATH --chromium PATH --provenance PATH --report PATH\nPrepare/run against a clean installed package. Verifies binary and every web asset against its archive. Real local accounts, membership, sign-in and workspace navigation; labelled browser-local billing/service display fixtures only. Covers return/cancel/portal, explicit verification/resume requests, alternate/forged/unauthorized teams, retained drafts, keyboard focus and 390/1440px dialogs. No Stripe, charge, entitlement, native shell, Docker, model or cloud claim. Report directory must be new or empty.');
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
const fixture = mkdtempSync(join(tmpdir(), 'luci-billing-return-'));
for (const name of ['home', 'projects', 'state']) mkdirSync(join(fixture, name), { mode: 0o700 });
const secrets = new Set(), secret = () => { const value = randomBytes(32).toString('base64url'); secrets.add(value); return value; };
const password = secret(), lifecycle = secret(), scrub = error => { let text = String(error?.message ?? error); for (const value of secrets) text = text.replaceAll(value, '[redacted]'); return text; };
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit,
  package: { archiveSHA256: provenance.archiveSHA256, binarySHA256, webIndexSHA256, allWebFilesMatchArchive: true, webFileCount: Object.keys(webFiles).length },
  driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  scope: 'Installed rendered return navigation using real local authentication and team membership; browser-local billing presentation fixtures are not subscription receipts.',
  limitations: ['No live Stripe API, checkout, payment or entitlement verification.', 'No managed-team binding or native browser handoff runtime claim; server compatibility and native dispatch have separate coverage.', 'Draft guard is exercised through the actual workspace selector after return, not a mounted draft surviving a full-page Stripe redirect.', 'Bounded axe/focus/mobile observations are not full accessibility or novice-usability certification.'],
  setupScaffolding: ['API-created owner and second account, two real teams and empty projects', 'Only authorized service/billing responses are replaced by labelled browser-local display data; explicit billing actions reach an unconfigured backend and are refused before a local response is displayed'],
  checks: [], billingRequests: [], browserMutations: [], blockedRequests: [], fixtureDestinations: [], accessibility: [], browserErrors: [], cleanup: [],
  externalStripeRequests: 0, externalModelCalls: 0, cloudResourcesCreated: 0, dockerAllocations: 0,
};
const inherited = Object.fromEntries(Object.entries(process.env).filter(([name]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ)$/.test(name)));
let service, browser, page, other, base, token, teamA, teamB, projectA, stage = 'startup', spawnError, serviceLog = '', routeError, scenario = 'unsubscribed';
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
const returnURL = (kind, team) => base + '/?' + new URLSearchParams({ billing: kind, billingTeam: team }).toString();
function billingState() {
  return { enabled: true, owner: true, executionAllowed: false, revision: 1, status: 'unsubscribed', separateCosts: 'Local browser fixture only. No Stripe account, charge or verified subscription is connected.',
    ...(scenario === 'reserved' ? { checkoutState: 'reserved', checkoutRecoverable: true, quote: { id: 'fixture-reserved-quote', expiresAt: 2000000000, termsUrl: base + '/terms', privacyUrl: base + '/privacy', price: { priceId: 'price_browser_fixture', unitAmount: 2400, currency: 'usd', interval: 'month', intervalCount: 1, taxBehavior: 'exclusive' } } } : {}),
  };
}
async function makePage() {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, serviceWorkers: 'block' });
  await context.route('**/*', async route => {
    try {
      const request = route.request(), url = new URL(request.url());
      if (url.href === 'https://checkout.stripe.com/c/pay/local-navigation-fixture') {
        receipt.fixtureDestinations.push({ label: 'Browser-local checkout navigation shell; no Stripe request', method: request.method() });
        return await route.fulfill({ contentType: 'text/html', body: `<!doctype html><html lang="en"><head><title>Local navigation fixture</title></head><body><h1>Local navigation fixture</h1><p>This is a browser-local page. No Stripe request or charge occurred.</p><a href="${returnURL('cancel', teamA.id).replaceAll('&', '&amp;')}">Return to Luci without checkout</a></body></html>` });
      }
      if (url.origin !== base) { receipt.blockedRequests.push({ method: request.method(), origin: url.origin, path: url.pathname }); return await route.abort('blockedbyclient'); }
      const match = /^\/v1\/teams\/(team-[A-Za-z0-9_-]{22})\/(service|billing)(?:\/(refresh|checkout))?$/.exec(url.pathname);
      if (!match) return await route.continue();
      const original = await route.fetch();
      if (match[2] === 'service') {
        if (original.status() !== 200) return await route.fulfill({ response: original });
        assert.equal(request.method(), 'GET');
        return await route.fulfill({ response: original, json: { ...await original.json(), billingEnabled: true, reason: 'Local browser fixture: subscription display only; real backend billing is disabled.' } });
      }
      receipt.billingRequests.push({ teamId: match[1], method: request.method(), action: match[3] ?? 'read', backendStatus: original.status(), source: 'browser-local fixture' });
      if (request.method() === 'GET') { assert.equal(original.status(), 200); return await route.fulfill({ response: original, json: billingState() }); }
      assert.equal(request.method(), 'POST'); assert.equal(original.status(), 409, 'Fixture backend must refuse real billing work');
      assert.equal((await original.json()).error, 'billing_disabled');
      if (match[3] === 'checkout') {
        assert.equal(scenario, 'reserved');
        assert.deepEqual(request.postDataJSON(), { quoteId: 'fixture-reserved-quote', expectedRevision: 1, acknowledgePrice: true, acknowledgeTerms: true });
        return await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify({ url: 'https://checkout.stripe.com/c/pay/local-navigation-fixture' }) });
      }
      assert.equal(match[3], 'refresh'); return await route.fulfill({ status: 200, contentType: 'application/json', body: JSON.stringify(billingState()) });
    } catch (error) { routeError ??= error; await route.abort('failed').catch(() => {}); }
  });
  const result = await context.newPage(); result.setDefaultTimeout(20000);
  result.on('pageerror', error => receipt.browserErrors.push(scrub(error)));
  result.on('dialog', dialog => dialog.type() === 'beforeunload' ? dialog.accept() : dialog.dismiss());
  result.on('request', request => { const url = new URL(request.url()); if (url.origin === base && !['GET', 'HEAD', 'OPTIONS'].includes(request.method())) receipt.browserMutations.push({ method: request.method(), path: url.pathname }); });
  return result;
}
async function login(current, username, destination) {
  await current.goto(destination); await current.getByLabel('Username', { exact: true }).fill(username); await current.getByLabel('Password', { exact: true }).fill(password); await current.getByRole('button', { name: 'Sign in', exact: true }).click();
}
const billingMutations = () => receipt.billingRequests.filter(row => row.method !== 'GET');
async function billingReturn(current, kind, team, navigate = true) {
  const count = billingMutations().length;
  if (navigate) await current.goto(returnURL(kind, team));
  const heading = kind === 'cancel' ? 'Checkout closed' : kind === 'portal' ? 'Back from subscription management' : 'Back from checkout';
  await current.getByRole('heading', { name: heading, exact: true }).waitFor();
  await current.getByText('Local browser fixture only. No Stripe account, charge or verified subscription is connected.', { exact: true }).waitFor();
  assert.equal(await current.getByRole('tab', { name: 'Service and data', exact: true }).getAttribute('aria-selected'), 'true');
  assert.equal(await current.getByLabel('Workspace', { exact: true }).inputValue(), team);
  assert.equal(new URL(current.url()).search, '');
  assert.equal(billingMutations().length, count, 'A return query initiated verification or mutation');
  assert(await current.getByText(/New team execution is unavailable/).isVisible());
}
async function capture(name, width) {
  await page.setViewportSize({ width, height: width < 600 ? 844 : 1000 });
  const dialog = page.getByRole('dialog'); await dialog.waitFor();
  await dialog.evaluate(async node => { for (const animation of node.getAnimations({ subtree: true })) await animation.finished.catch(() => {}); });
  const geometry = await dialog.evaluate(node => { const box = node.getBoundingClientRect(); return { x: box.x, y: box.y, width: box.width, height: box.height, clientWidth: node.clientWidth, scrollWidth: node.scrollWidth, viewport: { width: innerWidth, height: innerHeight } }; });
  assert(geometry.x >= -1 && geometry.x + geometry.width <= width + 1 && geometry.y >= -1 && geometry.y + geometry.height <= geometry.viewport.height + 1, 'Dialog does not fit ' + name);
  assert(geometry.scrollWidth <= geometry.clientWidth + 1, 'Dialog has horizontal overflow: ' + name);
  // Exercise native keyboard focus, including trap wrapping; no DOM focus edits.
  for (const key of ['Tab', ...Array(18).fill('Tab'), ...Array(18).fill('Shift+Tab')]) {
    await page.keyboard.press(key);
    assert(await dialog.evaluate(node => node.contains(document.activeElement)), 'Keyboard focus escaped billing dialog');
  }
  await page.evaluate(axe.source);
  const audit = await dialog.evaluate(async node => window.axe.run(node, { runOnly: ['wcag2a', 'wcag2aa', 'wcag21aa'] }));
  writeFileSync(join(evidence, name + '-accessibility.json'), JSON.stringify(audit, null, 2), { mode: 0o600 });
  receipt.accessibility.push({ name, geometry, focusStayedInDialog: true, violations: audit.violations.map(row => row.id), incomplete: audit.incomplete.map(row => row.id) });
  await page.screenshot({ path: join(evidence, name + '.png'), fullPage: true });
  assert.deepEqual(audit.violations, [], 'Accessibility violation: ' + name);
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
  const owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'billing-owner', displayName: 'Billing return owner', password }, 201, ''); token = owner.token;
  await api('POST', '/v1/admin/users', { username: 'billing-member', displayName: 'Other team member', password }, 201);
  const member = await api('POST', '/v1/auth/login', { username: 'billing-member', password }, 200, '');
  await api('POST', '/v1/product/projects', { name: 'owner-entry-seed' }, 201);
  await api('POST', '/v1/product/projects', { name: 'member-entry-seed' }, 201, member.token);
  teamA = (await api('POST', '/v1/teams', { name: 'Garden billing fixture' }, 201)).team;
  teamB = (await api('POST', '/v1/teams', { name: 'Orchard billing fixture' }, 201)).team;
  teamA = (await api('POST', `/v1/teams/${teamA.id}/members`, { expectedRevision: teamA.revision, username: 'billing-member', role: 'viewer', grants: { execute: false, secrets: false, deploy: false, admin: false } })).team;
  projectA = await api('POST', `/v1/teams/${teamA.id}/workspace/v1/product/projects`, { name: 'billing-draft-garden' }, 201);
  await api('POST', `/v1/teams/${teamB.id}/workspace/v1/product/projects`, { name: 'billing-draft-orchard' }, 201);
  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited }); receipt.browserVersion = browser.version(); page = await makePage();

  stage = 'return-through-sign-in';
  await login(page, 'billing-owner', returnURL('return', teamA.id));
  const returning = page.getByRole('dialog', { name: 'Billing return', exact: true }); await returning.waitFor();
  assert.equal(await page.getByLabel('Workspace', { exact: true }).inputValue(), ''); assert.equal(billingMutations().length, 0);
  await returning.getByRole('button', { name: 'Open Garden billing fixture billing', exact: true }).focus(); await page.keyboard.press('Enter');
  await billingReturn(page, 'return', teamA.id, false);
  await capture('checkout-return-wide', 1440); await capture('checkout-return-mobile', 390);
  check('Incoming return survives real sign-in and requires explicit keyboard team selection before opening authorized Service and data; URL grants no access');

  stage = 'explicit-local-verification';
  const beforeVerify = billingMutations().length;
  await page.getByRole('button', { name: 'Verify subscription with Stripe', exact: true }).click();
  await page.getByRole('button', { name: 'Verify subscription with Stripe', exact: true }).waitFor({ state: 'visible' });
  await page.waitForFunction(() => ![...document.querySelectorAll('button')].find(node => node.textContent === 'Verify subscription with Stripe')?.disabled);
  assert.equal(billingMutations().length, beforeVerify + 1); assert.equal(billingMutations().at(-1).action, 'refresh');
  assert(await page.getByText(/New team execution is unavailable/).isVisible());
  check('Only explicit Verify creates one local fixture request; real backend refuses billing and no entitlement changes');

  stage = 'cancel-resume'; scenario = 'reserved'; await page.setViewportSize({ width: 1440, height: 1000 }); await billingReturn(page, 'cancel', teamA.id);
  assert(await page.getByRole('button', { name: 'Resume reserved checkout', exact: true }).isDisabled());
  await page.getByRole('checkbox', { name: /I accept the terms and this recurring team price/ }).check();
  const beforeResume = billingMutations().length; await page.getByRole('button', { name: 'Resume reserved checkout', exact: true }).click();
  const checkout = page.getByRole('link', { name: 'Continue to Stripe checkout', exact: true }); await checkout.waitFor();
  assert.equal(billingMutations().length, beforeResume + 1); assert.equal(await checkout.getAttribute('target'), null);
  const pages = page.context().pages().length; await checkout.click(); await page.getByRole('heading', { name: 'Local navigation fixture', exact: true }).waitFor();
  assert.equal(page.context().pages().length, pages); await page.getByRole('link', { name: 'Return to Luci without checkout', exact: true }).click(); await billingReturn(page, 'cancel', teamA.id, false);
  check('Cancellation explains preserved subscription state; acknowledged existing checkout resumes once and a labelled local checkout shell returns in the same tab');

  stage = 'portal-return'; scenario = 'unsubscribed'; await billingReturn(page, 'portal', teamA.id); await capture('portal-return-mobile', 390);
  check('Portal return has distinct guidance and opens saved billing state without automatic verification');
  stage = 'forged-return'; await page.setViewportSize({ width: 1440, height: 1000 });
  const requestsBeforeForged = receipt.billingRequests.length;
  await page.goto(base + '/?billing=return&billing=cancel&billingTeam=' + teamA.id + '&status=active');
  await page.getByRole('dialog', { name: 'Billing return', exact: true }).waitFor();
  await page.getByText(/could not be matched to a team available/).waitFor();
  assert.equal(receipt.billingRequests.length, requestsBeforeForged); assert.equal(await page.getByLabel('Workspace', { exact: true }).inputValue(), teamA.id);
  await capture('invalid-return-wide', 1440); await page.getByRole('button', { name: 'Back to my workspace', exact: true }).click();
  check('Duplicate/forged return parameters show safe guidance with no billing read, verification or workspace switch');

  stage = 'alternate-team'; const beforeAlternate = billingMutations().length; await page.goto(returnURL('return', teamB.id));
  await page.getByRole('dialog', { name: 'Billing return', exact: true }).waitFor();
  assert.equal(await page.getByLabel('Workspace', { exact: true }).inputValue(), teamA.id);
  const confirming = page.waitForResponse(response => response.request().method() === 'GET' && new URL(response.url()).pathname === `/v1/teams/${teamB.id}`);
  await page.getByRole('button', { name: 'Open Orchard billing fixture billing', exact: true }).click(); assert.equal((await confirming).status(), 200);
  await billingReturn(page, 'return', teamB.id, false); assert.equal(billingMutations().length, beforeAlternate);
  check('Another available team requires explicit choice and a fresh real membership response before switching');

  stage = 'unavailable-team'; other = await makePage(); await login(other, 'billing-member', returnURL('portal', teamB.id));
  await other.getByText(/could not be matched to a team available/).waitFor();
  assert.equal(await other.getByRole('button', { name: 'Open Orchard billing fixture billing', exact: true }).count(), 0);
  assert.equal(await other.getByLabel('Workspace', { exact: true }).inputValue(), '');
  await api('GET', `/v1/teams/${teamB.id}`, undefined, 404, member.token);
  check('A separate authenticated nonmember receives unavailable-team guidance; real team detail remains denied');

  stage = 'retained-draft-guard'; await page.getByRole('button', { name: 'Close', exact: true }).click();
  await page.getByLabel('Workspace', { exact: true }).selectOption(teamA.id);
  await page.getByRole('treeitem', { name: /billing-draft-garden$/ }).click(); await page.getByRole('button', { name: 'Define', exact: true }).click();
  const draftText = 'Retain this garden idea while I review team billing.', idea = page.getByRole('textbox', { name: 'What should this solution do?', exact: true });
  await idea.fill(draftText); await page.getByText(/Unsaved changes · kept in this browser/).waitFor();
  await page.getByLabel('Workspace', { exact: true }).selectOption(teamB.id); await page.getByRole('dialog', { name: 'Switch workspace?', exact: true }).waitFor();
  await page.getByRole('button', { name: 'Stay here', exact: true }).click(); assert.equal(await idea.inputValue(), draftText); assert.equal(await page.getByLabel('Workspace', { exact: true }).inputValue(), teamA.id);
  await page.getByLabel('Workspace', { exact: true }).selectOption(teamB.id); await page.getByRole('button', { name: 'Keep drafts and switch', exact: true }).click();
  await page.getByLabel('Workspace', { exact: true }).selectOption(teamA.id); await page.getByRole('treeitem', { name: /billing-draft-garden$/ }).click(); await page.getByRole('button', { name: 'Define', exact: true }).click();
  const recovery = page.getByRole('region', { name: 'Definition draft recovery', exact: true }); await recovery.waitFor(); await recovery.getByRole('button', { name: 'Restore draft', exact: true }).click();
  assert.equal(await idea.inputValue(), draftText);
  const saved = await api('GET', `/v1/teams/${teamA.id}/workspace/v1/product/projects/${projectA.id}/specification`); assert.equal(saved.purpose, '');
  await page.screenshot({ path: join(evidence, 'retained-definition.png'), fullPage: true });
  check('After returning, actual workspace switch cancellation preserves edits and keep-drafts/switch/back restores the unsaved text without changing the saved project');

  assert.equal(routeError, undefined); assert.deepEqual(receipt.browserErrors, []); assert.deepEqual(receipt.blockedRequests, []);
  assert.deepEqual(receipt.browserMutations.filter(row => row.path !== '/v1/auth/login' && !/^\/v1\/teams\/team-[A-Za-z0-9_-]{22}\/billing\/(refresh|checkout)$/.test(row.path)), [], 'Unexpected browser mutation or execution');
  assert.equal(billingMutations().length, 2); assert.equal(receipt.fixtureDestinations.length, 1);
  for (const team of [teamA, teamB]) assert.equal((await api('GET', `/v1/teams/${team.id}/billing`)).enabled, false, 'Fixture must not configure actual billing');
  assert.equal(sha(readFileSync(binary)), binarySHA256); assert.deepEqual(inventory(), webFiles, 'Installed package changed during acceptance');
  receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error); receipt.failedStage = stage; if (routeError) receipt.fixtureError = scrub(routeError);
  for (const [name, current] of [['owner', page], ['nonmember', other]]) if (current) {
    await current.screenshot({ path: join(evidence, name + '-failure.png'), fullPage: true }).catch(() => {});
    await current.locator('body').innerText().then(body => writeFileSync(join(evidence, name + '-failure-dom.txt'), scrub(body), { mode: 0o600 })).catch(() => {});
  }
  process.exitCode = 1;
} finally {
  await browser?.close().catch(error => { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); });
  let cleaned = false; try { await stop(); cleaned = true; } catch (error) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); }
  if (cleaned) rmSync(fixture, { recursive: true, force: true });
  else receipt.retainedFixture = fixture;
  if (receipt.cleanupErrors?.length) { receipt.passed = false; process.exitCode = 1; }
  receipt.finishedAt = new Date().toISOString();
  writeFileSync(join(evidence, 'service.log'), scrub(serviceLog), { mode: 0o600 });
  writeFileSync(report, JSON.stringify(receipt, null, 2), { mode: 0o600 });
  console.log(JSON.stringify({ passed: receipt.passed, checks: receipt.checks.length, report, failedStage: receipt.failedStage }));
}

#!/usr/bin/env node
// Installed subscription-denial guidance with real UI requests and a labelled
// private saved billing observation. No response substitution or Stripe traffic.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomBytes } from 'node:crypto';
import { createServer, createConnection } from 'node:net';
import { createServer as createHTTP, request as httpRequest } from 'node:http';
import { createServer as createHTTPS } from 'node:https';
import { existsSync, lstatSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, readlinkSync, realpathSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve } from 'node:path';
import { tmpdir } from 'node:os';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '../desktop/node_modules/playwright/index.mjs';

const args = process.argv.slice(2);
if (args.includes('--help')) {
  console.log('node scripts/billing-guidance-browser-uat.mjs --binary PATH --web PATH --chromium PATH --provenance PATH --report PATH\nActual installed owner/member subscription-denial UI, Service and data destination, and operator-suspension precedence. Real local accounts/grants; stopped private state is seeded with a labelled billing observation. Exact local TLS key trust, no public-domain claim. A deny-only outbound proxy prevents Stripe requests. No Docker, command execution, payment or entitlement verification. Use a new report directory.');
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
const fixture = mkdtempSync(join(tmpdir(), 'luci-billing-guidance-'));
for (const name of ['home', 'projects', 'state']) mkdirSync(join(fixture, name), { mode: 0o700 });
const secrets = new Set(), secret = () => { const value = randomBytes(32).toString('base64url'); secrets.add(value); return value; };
const password = secret(), lifecycle = secret(), scrub = error => { let text = String(error?.message ?? error); for (const value of secrets) text = text.replaceAll(value, '[redacted]'); return text; };
const receipt = {
  passed: false, startedAt: new Date().toISOString(), sourceCommit: provenance.sourceCommit,
  package: { archiveSHA256: provenance.archiveSHA256, binarySHA256, webIndexSHA256, allWebFilesMatchArchive: true, webFileCount: Object.keys(webFiles).length },
  driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  scope: 'Actual installed UI and backend subscription-denial recovery using a labelled saved billing observation and real isolated accounts/project grants.',
  limitations: ['Private billing status is explicitly seeded while the owned service is stopped; this is not a verified provider receipt.', 'No live Stripe account, checkout, charge, subscription renewal or entitlement verification.', 'Local self-signed TLS is trusted by this exact fixture certificate key; no public-domain or certificate commissioning claim.', 'Source tests separately cover the complete state and cleanup matrix; these few rendered cases do not claim whole-product acceptance.'],
  setupScaffolding: ['API-created owner/member, one team and empty project', 'Stopped private identity state receives a labelled past_due observation bound to fake merchant configuration; no browser responses are replaced'],
  checks: [], browserMutations: [], denials: [], blockedRequests: [], providerAttempts: [], browserErrors: [], cleanup: [],
  stripeCalls: 0, commandExecutions: 0, dockerAllocations: 0,
};
const inherited = Object.fromEntries(Object.entries(process.env).filter(([name]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ)$/.test(name)));
let service, browser, page, other, base, token, teamA, projectA, stage = 'startup', spawnError, serviceLog = '', routeError, tlsServer, denyProxy, publicOrigin, merchantEnvironment;
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

async function start(environment) {
  spawnError = undefined;
  service = spawn(binary, [], { cwd: fixture, env: environment, detached: true, stdio: ['ignore', 'pipe', 'pipe'] });
  service.once('error', error => { spawnError = error; });
  for (const stream of [service.stdout, service.stderr]) stream.on('data', bytes => { serviceLog = (serviceLog + scrub(bytes.toString())).slice(-100000); });
  const deadline = Date.now() + 30000;
  while (Date.now() < deadline) {
    if (spawnError) throw spawnError;
    assert.equal(service.exitCode, null, 'Owned service exited before readiness');
    try { const status = await api('GET', '/v1/control-plane/status', undefined, 200, ''); receipt.version = status.version; return; } catch { await delay(100); }
  }
  assert.fail('Owned service did not become ready');
}
async function makePage(username) {
  const context = await browser.newContext({ viewport: { width: 1440, height: 1000 }, serviceWorkers: 'block' });
  await context.route('**/*', async route => {
    try {
      const request = route.request(), url = new URL(request.url());
      if (url.origin !== publicOrigin) { receipt.blockedRequests.push({ method: request.method(), origin: url.origin }); return await route.abort('blockedbyclient'); }
      // Never replace responses. Only the actual expected check request and
      // real sign-in are browser mutations in this bounded acceptance.
      if (!['GET', 'HEAD', 'OPTIONS'].includes(request.method())) {
        const permitted = request.method() === 'POST' && (url.pathname === '/v1/auth/login' || url.pathname === `/v1/teams/${teamA.id}/workspace/v1/product/projects/${projectA.id}/checks`);
        receipt.browserMutations.push({ actor: username, method: request.method(), path: url.pathname, permitted });
        if (!permitted) { routeError ??= new Error('Unexpected browser mutation ' + url.pathname); return await route.abort('blockedbyclient'); }
      }
      await route.continue();
    } catch (error) { routeError ??= error; await route.abort('failed').catch(() => {}); }
  });
  const current = await context.newPage(); current.setDefaultTimeout(20000);
  current.on('pageerror', error => receipt.browserErrors.push(scrub(error)));
  current.on('dialog', dialog => dialog.type() === 'beforeunload' ? dialog.accept() : dialog.dismiss());
  await current.goto(publicOrigin);
  await current.getByLabel('Username', { exact: true }).fill(username);
  await current.getByLabel('Password', { exact: true }).fill(password);
  await current.getByRole('button', { name: 'Sign in', exact: true }).click();
  await current.getByRole('treeitem', { name: /subscription-guidance-project$/ }).click();
  await current.getByRole('button', { name: 'Develop', exact: true }).click();
  const details = current.locator('details.project-work');
  if (!await details.getAttribute('open').then(value => value !== null)) await details.locator(':scope > summary').click();
  const executableField = current.getByRole('textbox', { name: 'Executable', exact: true });
  await executableField.fill('node');
  await current.getByRole('textbox', { name: 'Arguments · JSON array', exact: true }).fill(JSON.stringify(['-e', 'throw new Error("must never execute")']));
  await current.getByRole('button', { name: 'Add check command', exact: true }).click();
  await current.getByRole('button', { name: 'Remove node', exact: true }).waitFor();
  return current;
}
async function denyChecks(current, label, expected, forbidden) {
  const path = `/v1/teams/${teamA.id}/workspace/v1/product/projects/${projectA.id}/checks`;
  await current.getByRole('checkbox', { name: "I trust this project's code and these Docker commands to write project files and use the network.", exact: true }).check();
  const runButton = current.getByRole('button', { name: 'Run project checks', exact: true });
  const before = await runButton.evaluate(button => { const box = button.getBoundingClientRect(); return { disabled: button.disabled, rect: { x: box.x, y: box.y, width: box.width, height: box.height }, viewport: { width: innerWidth, height: innerHeight } }; });
  writeFileSync(join(evidence, label + '-before-click.json'), JSON.stringify(before, null, 2), { mode: 0o600 });
  writeFileSync(join(evidence, label + '-before-click-dom.txt'), scrub(await current.locator('body').innerText()), { mode: 0o600 });
  await current.screenshot({ path: join(evidence, label + '-before-click.png'), fullPage: true });
  const [response] = await Promise.all([
    current.waitForResponse(response => new URL(response.url()).pathname === path && response.request().method() === 'POST'),
    runButton.click(),
  ]);
  const value = await response.json();
  assert.equal(response.status(), 403); assert.equal(value.error, 'team_service_suspended');
  assert(value.message.includes(expected), label + ' did not receive expected backend guidance');
  if (forbidden) assert(!value.message.includes(forbidden), label + ' received the wrong recovery authority');
  const alert = current.getByRole('alert').filter({ hasText: expected }); await alert.waitFor();
  assert.equal(await alert.innerText(), value.message, 'Rendered guidance is not the actual backend response');
  await current.screenshot({ path: join(evidence, label + '.png'), fullPage: true });
  receipt.denials.push({ label, status: response.status(), error: value.error, message: value.message });
  assert.deepEqual((await api('GET', path)).runs, [], 'Denied checks created an execution record');
  assert.deepEqual((await api('GET', `/v1/teams/${teamA.id}/workspace/v1/sessions`)).sessions, [], 'Denied checks created a session');
}
async function openService(current, owner, label) {
  await current.getByRole('button', { name: 'Team access', exact: true }).click();
  const tab = current.getByRole('tab', { name: 'Service and data', exact: true }); await tab.click();
  await current.getByText('Operator execution access is active', { exact: true }).waitFor();
  await current.getByText(/Subscription: past due\./).waitFor();
  if (owner) {
    assert(await current.getByRole('button', { name: 'Verify subscription with Stripe', exact: true }).isEnabled());
    assert(await current.getByRole('button', { name: 'Manage or cancel subscription', exact: true }).isEnabled());
  } else {
    assert(await current.getByText('A team owner can review pricing, subscribe or cancel.', { exact: true }).isVisible());
    assert.equal(await current.getByRole('button', { name: 'Verify subscription with Stripe', exact: true }).count(), 0);
  }
  await current.screenshot({ path: join(evidence, label + '.png'), fullPage: true });
  await current.getByRole('button', { name: 'Close', exact: true }).click();
}

try {
  const controlPort = await port(), tlsPort = await port(), preview = await port();
  base = `http://127.0.0.1:${controlPort}`; publicOrigin = `https://billing-guidance.test:${tlsPort}`;
  denyProxy = createHTTP((req, res) => { receipt.providerAttempts.push({ method: req.method, destination: req.url }); res.writeHead(502); res.end('Fixture forbids provider traffic'); });
  denyProxy.on('connect', (req, socket) => { receipt.providerAttempts.push({ method: 'CONNECT', destination: req.url }); socket.end('HTTP/1.1 502 Provider traffic forbidden\r\nConnection: close\r\n\r\n'); });
  await new Promise(done => denyProxy.listen(0, '127.0.0.1', done));
  const outbound = `http://127.0.0.1:${denyProxy.address().port}`;
  const environment = { ...inherited, HOME: join(fixture, 'home'), USERPROFILE: join(fixture, 'home'), LUCI_ADDR: `127.0.0.1:${controlPort}`, LUCI_BROWSER_PREVIEW_ADDR: `127.0.0.1:${preview}`, LUCI_BROWSER_PREVIEW_PORT: String(preview), LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'), LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: join(dirname(binary), 'runtime'), LUCI_DOCKER_HOST: 'unix://' + join(fixture, 'absent-docker.sock'), DOCKER_HOST: 'unix://' + join(fixture, 'absent-docker.sock'), LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_BACKEND: 'docker', GIT_CONFIG_GLOBAL: join(fixture, 'home/.gitconfig'), GIT_CONFIG_NOSYSTEM: '1', AWS_EC2_METADATA_DISABLED: 'true', HTTP_PROXY: outbound, HTTPS_PROXY: outbound, NO_PROXY: '127.0.0.1,localhost' };
  await start(environment);
  const bootstrap = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(); secrets.add(bootstrap);
  const owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken: bootstrap, username: 'billing-owner', displayName: 'Billing guidance owner', password }, 201, ''); token = owner.token;
  await api('POST', '/v1/admin/users', { username: 'billing-member', displayName: 'Billing guidance member', password }, 201);
  teamA = (await api('POST', '/v1/teams', { name: 'Subscription guidance fixture' }, 201)).team;
  teamA = (await api('POST', `/v1/teams/${teamA.id}/members`, { expectedRevision: teamA.revision, username: 'billing-member', role: 'editor', grants: { execute: true, secrets: false, deploy: true, admin: false } })).team;
  const detail = await api('GET', `/v1/teams/${teamA.id}`), memberID = detail.members.find(row => row.user.username === 'billing-member').userId;
  projectA = await api('POST', `/v1/teams/${teamA.id}/workspace/v1/product/projects`, { name: 'subscription-guidance-project' }, 201);
  const current = await api('GET', `/v1/teams/${teamA.id}`);
  await api('PUT', `/v1/teams/${teamA.id}/projects/${projectA.id}/members/${memberID}`, { expectedRevision: current.team.revision, role: 'editor', grants: { execute: true, secrets: false, deploy: true, admin: false } });
  await stop(); assert.equal(owned().length, 0);
  const configID = sha(JSON.stringify(['acct_local_fixture', false, 'price_local_fixture', publicOrigin, publicOrigin + '/terms', publicOrigin + '/privacy']));
  const identityPath = join(fixture, 'state/control-plane/identity.json'), identityData = JSON.parse(readFileSync(identityPath, 'utf8'));
  const billingObservation = { configId: configID, revision: 1, status: 'past_due', customerId: 'cus_local_fixture', subscriptionId: 'sub_local_fixture' };
  identityData.teams[teamA.id].billing = billingObservation;
  writeFileSync(identityPath, JSON.stringify(identityData), { mode: 0o600 });
  receipt.seededObservation = { status: 'past_due', revision: 1, label: 'Private stopped-service fixture input, not a Stripe receipt' };
  const fakeKey = 'sk_test_local_fixture_only', fakeWebhook = 'whsec_local_fixture_only'; secrets.add(fakeKey); secrets.add(fakeWebhook);
  merchantEnvironment = { ...environment, LUCI_MANAGED_TEAM_ID: teamA.id, LUCI_PUBLIC_APP_ORIGIN: publicOrigin, LUCI_STRIPE_API_KEY: fakeKey, LUCI_STRIPE_WEBHOOK_SECRET: fakeWebhook, LUCI_STRIPE_ACCOUNT_ID: 'acct_local_fixture', LUCI_STRIPE_PRICE_ID: 'price_local_fixture', LUCI_BILLING_TERMS_URL: publicOrigin + '/terms', LUCI_BILLING_PRIVACY_URL: publicOrigin + '/privacy' };
  const keyFile = join(fixture, 'tls-key.pem'), certFile = join(fixture, 'tls-cert.pem');
  execFileSync('openssl', ['req', '-x509', '-newkey', 'rsa:2048', '-nodes', '-keyout', keyFile, '-out', certFile, '-days', '1', '-subj', '/CN=billing-guidance.test', '-addext', 'subjectAltName=DNS:billing-guidance.test'], { stdio: 'ignore', timeout: 15000 });
  const publicKey = execFileSync('openssl', ['x509', '-in', certFile, '-pubkey', '-noout']);
  const spki = createHash('sha256').update(execFileSync('openssl', ['pkey', '-pubin', '-outform', 'DER'], { input: publicKey })).digest('base64');
  tlsServer = createHTTPS({ key: readFileSync(keyFile), cert: readFileSync(certFile) }, (req, res) => {
    if (req.headers.host !== new URL(publicOrigin).host) { res.writeHead(421); res.end(); return; }
    const upstream = httpRequest({ host: '127.0.0.1', port: controlPort, path: req.url, method: req.method, headers: req.headers }, response => { res.writeHead(response.statusCode, response.headers); response.pipe(res); });
    upstream.on('error', () => { res.writeHead(502); res.end(); }); req.pipe(upstream);
  });
  tlsServer.on('upgrade', (req, socket, head) => {
    if (req.headers.host !== new URL(publicOrigin).host) { socket.destroy(); return; }
    const upstream = createConnection({ host: '127.0.0.1', port: controlPort }, () => { upstream.write(`${req.method} ${req.url} HTTP/${req.httpVersion}\r\n` + req.rawHeaders.reduce((lines, value, index, all) => index % 2 ? lines : lines + value + ': ' + all[index + 1] + '\r\n', '') + '\r\n'); if (head.length) upstream.write(head); upstream.pipe(socket); socket.pipe(upstream); });
    upstream.on('error', () => socket.destroy()); socket.on('error', () => upstream.destroy()); socket.on('close', () => upstream.destroy());
  });
  await new Promise(done => tlsServer.listen(tlsPort, '127.0.0.1', done));
  await start(merchantEnvironment);
  assert.deepEqual((await api('GET', `/v1/teams/${teamA.id}/billing`)).status, 'past_due');
  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited, args: ['--no-proxy-server', '--host-resolver-rules=MAP billing-guidance.test 127.0.0.1', '--ignore-certificate-errors-spki-list=' + spki] });
  receipt.browserVersion = browser.version();

  stage = 'owner-guidance'; page = await makePage('billing-owner');
  await denyChecks(page, '01-owner-subscription-denial', "Open the team's Service and data settings", 'Ask the service operator');
  await openService(page, true, '02-owner-subscription-recovery');
  check('Owner sees actual subscription403 then reaches existing verification/management controls; no provider request or execution');

  stage = 'member-guidance'; other = await makePage('billing-member');
  await denyChecks(other, '03-member-subscription-denial', 'Ask a team owner to open Service and data', 'Ask the service operator');
  await openService(other, false, '04-member-subscription-status');
  check('Member sees actual owner-directed recovery and read-only subscription status without billing actions');

  stage = 'operator-precedence';
  const serviceState = await api('GET', `/v1/teams/${teamA.id}/service`);
  await api('PUT', `/v1/admin/team-services/${teamA.id}`, { expectedRevision: serviceState.revision, mode: 'managed', status: 'suspended', reason: 'Explicit acceptance fixture suspension; no merchant operation' });
  await denyChecks(page, '05-operator-suspension-precedence', 'Ask the service operator', 'verify it with Stripe');
  check('Explicit operator suspension takes precedence over the same saved billing failure; paying is not presented as an override');

  assert.equal(routeError, undefined); assert.deepEqual(receipt.browserErrors, []); assert.deepEqual(receipt.blockedRequests, []); assert.deepEqual(receipt.providerAttempts, [], 'Unexpected outbound provider request');
  assert.equal(receipt.browserMutations.filter(row => row.path.endsWith('/checks')).length, 3);
  assert.equal(receipt.browserMutations.filter(row => row.path.includes('/billing/')).length, 0);
  assert.deepEqual(JSON.parse(readFileSync(identityPath, 'utf8')).teams[teamA.id].billing, billingObservation, 'UI guidance changed saved billing authority');
  assert.equal(sha(readFileSync(binary)), binarySHA256); assert.deepEqual(inventory(), webFiles);
  receipt.passed = true;
} catch (error) {
  receipt.error = scrub(error); receipt.failedStage = stage; if (routeError) receipt.fixtureError = scrub(routeError);
  for (const [name, current] of [['owner', page], ['member', other]]) if (current) {
    await current.screenshot({ path: join(evidence, name + '-failure.png'), fullPage: true }).catch(() => {});
    await current.locator('body').innerText().then(body => writeFileSync(join(evidence, name + '-failure-dom.txt'), scrub(body), { mode: 0o600 })).catch(() => {});
  }
  process.exitCode = 1;
} finally {
  await browser?.close().catch(error => { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); });
  if (tlsServer) { tlsServer.closeAllConnections(); await new Promise(done => tlsServer.close(done)); }
  let cleaned = false; try { await stop(); cleaned = true; } catch (error) { receipt.cleanupErrors ??= []; receipt.cleanupErrors.push(scrub(error)); }
  if (denyProxy) { denyProxy.closeAllConnections(); await new Promise(done => denyProxy.close(done)); }
  if (cleaned) rmSync(fixture, { recursive: true, force: true }); else receipt.retainedFixture = fixture;
  if (receipt.cleanupErrors?.length) { receipt.passed = false; process.exitCode = 1; }
  receipt.finishedAt = new Date().toISOString();
  writeFileSync(join(evidence, 'service.log'), scrub(serviceLog), { mode: 0o600 });
  writeFileSync(report, JSON.stringify(receipt, null, 2), { mode: 0o600 });
  console.log(JSON.stringify({ passed: receipt.passed, checks: receipt.checks.length, report, failedStage: receipt.failedStage }));
}

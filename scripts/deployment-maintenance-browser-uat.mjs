#!/usr/bin/env node
// Intended destination: scripts/deployment-maintenance-browser-uat.mjs.
// Fixture accounts, approved context and exact A/B/C source commits are setup,
// not agent-generated implementation. All deployment operations below use UI.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { existsSync, lstatSync, mkdirSync, mkdtempSync, readFileSync, readdirSync, readlinkSync, realpathSync, rmSync, statfsSync, writeFileSync } from 'node:fs';
import { createServer } from 'node:net';
import { dirname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '../desktop/node_modules/playwright/index.mjs';
import axe from '../desktop/node_modules/axe-core/axe.js';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..'), args = process.argv.slice(2);
const option = name => { const at = args.indexOf(name); if (at < 0) return undefined; assert(args[at + 1] && !args[at + 1].startsWith('--'), 'Missing ' + name); return args[at + 1]; };
if (args.includes('--help')) {
  console.log('node scripts/deployment-maintenance-browser-uat.mjs --binary PATH --web PATH --chromium PATH --provenance PATH --report NEW_RESULT_PATH\nRun only in an exclusive Docker window. Requires the retained exact Node22 Alpine image; never pulls. Two running app containers maximum, 256MiB/250m CPU each, >=2GiB actual Docker-storage headroom. Real UI deploy A, unhealthy B while A remains reachable, logs, installed service restart, explicit stop B, reviewed repair C, stop/restart A. Six-minute journey budget plus bounded cleanup. No provider, cloud, database, traffic-cutover or live-model claim.');
  process.exit(0);
}
for (const name of ['--binary', '--web', '--chromium', '--provenance', '--report']) assert(option(name), 'Required ' + name);
assert.equal(process.platform, 'linux'); assert.notEqual(process.getuid(), 0, 'Use the ordinary installation user');
const binary = realpathSync(option('--binary')), web = realpathSync(option('--web')), executable = realpathSync(option('--chromium'));
const report = resolve(option('--report')), evidence = dirname(report), sha = bytes => createHash('sha256').update(bytes).digest('hex');
if (existsSync(evidence)) assert.equal(readdirSync(evidence).length, 0, 'Preserve evidence: choose a new or empty report directory');
else mkdirSync(evidence, { recursive: true, mode: 0o700 });
const provenanceBytes = readFileSync(option('--provenance')), provenance = JSON.parse(provenanceBytes);
const binaryHash = sha(readFileSync(binary));
assert.equal(provenance.sourceDirty, false); assert.match(provenance.sourceCommit, /^[a-f0-9]{40}$/);
assert.equal(provenance.binarySHA256, binaryHash); assert.equal(provenance.webIndexSHA256, sha(readFileSync(join(web, 'index.html'))));
function webInventory(root) {
  const rows = [];
  function visit(path, prefix = '') { for (const name of readdirSync(path).sort()) {
    const file = join(path, name), stat = lstatSync(file); assert(!stat.isSymbolicLink(), 'Linked package member');
    if (stat.isDirectory()) visit(file, prefix + name + '/');
    else { assert(stat.isFile()); rows.push({ path: prefix + name, sha256: sha(readFileSync(file)) }); }
  } }
  visit(root); return rows;
}
const webFiles = webInventory(web);
assert(provenance.archive && provenance.archiveSHA256, 'An archive-verified package is required');
// Same archive/member proof used by the installed import/data drivers: index.html
// alone does not bind the JS/CSS assets actually rendered by the browser.
const members = JSON.parse(execFileSync('python3', ['-c', `import hashlib,io,json,pathlib,sys,tarfile
raw=pathlib.Path(sys.argv[1]).read_bytes()
if hashlib.sha256(raw).hexdigest()!=sys.argv[2]: raise ValueError('Package archive changed')
result={'web':{},'binary':None}
with tarfile.open(fileobj=io.BytesIO(raw),mode='r:gz') as archive:
 for row in archive:
  p=pathlib.PurePosixPath(row.name)
  if p.is_absolute() or '..' in p.parts: raise ValueError('Unsafe package path')
  if not p.parts or not (p.parts[0]=='web' or str(p)=='luci'): continue
  if row.isdir(): continue
  if not row.isfile(): raise ValueError('Special package member')
  h=hashlib.sha256()
  with archive.extractfile(row) as stream:
   while block:=stream.read(1048576): h.update(block)
  if str(p)=='luci':
   if result['binary'] is not None: raise ValueError('Duplicate binary')
   result['binary']=h.hexdigest()
  else:
   name=str(pathlib.PurePosixPath(*p.parts[1:]))
   if name in result['web']: raise ValueError('Duplicate web member')
   result['web'][name]=h.hexdigest()
print(json.dumps(result))`, provenance.archive, provenance.archiveSHA256], { encoding: 'utf8', timeout: 60000, maxBuffer: 8 * 1024 * 1024 }));
assert.equal(members.binary, binaryHash); assert.deepEqual(Object.fromEntries(webFiles.map(row => [row.path, row.sha256])), members.web);
writeFileSync(join(evidence, 'package-provenance.json'), provenanceBytes, { mode: 0o600, flag: 'wx' });
writeFileSync(join(evidence, 'driver.mjs'), readFileSync(fileURLToPath(import.meta.url)), { mode: 0o600, flag: 'wx' });
const dockerHost = process.env.DOCKER_HOST || 'unix:///var/run/docker.sock';
assert(dockerHost.startsWith('unix:///') && lstatSync(dockerHost.slice('unix://'.length)).isSocket(), 'Use the explicit local Unix Docker socket; remote engines are outside this proof');
const docker = (...values) => execFileSync('docker', ['--host', dockerHost, ...values], { encoding: 'utf8', timeout: 30000, maxBuffer: 4 * 1024 * 1024, env: { ...Object.fromEntries(Object.entries(process.env).filter(([key]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ)$/.test(key))), DOCKER_HOST: dockerHost, DOCKER_CONTEXT: '' } }).trim();
const baseImage = 'node:22-alpine', expectedBase = 'sha256:7c3b093add7c43400ee83b815ab2cda98794a10045bcf76ce9bb2f89b97cbc5c';
const image = JSON.parse(docker('image', 'inspect', baseImage))[0]; assert.equal(image.Id, expectedBase, 'Retained trusted Node image changed; never substitute or pull');
const dockerInfo = JSON.parse(docker('info', '--format', '{{json .}}')); assert.equal(dockerInfo.OSType, 'linux');
assert(dockerInfo.MemTotal >= 1024 ** 3 && dockerInfo.NCPU >= 1); assert(existsSync(dockerInfo.DockerRootDir));
const disk = statfsSync(dockerInfo.DockerRootDir), freeBytes = disk.bavail * disk.bsize; assert(freeBytes >= 2 * 1024 ** 3, 'Require 2GiB actual Docker-filesystem headroom');
assert.equal(docker('ps', '--format', '{{.ID}}'), '', 'This driver requires an exclusive, idle Docker window');
const preexisting = Object.fromEntries(['container', 'image', 'network', 'volume'].map(kind => [kind, docker(kind, 'ls', ...(kind === 'container' ? ['--all'] : []), '--quiet').split(/\s+/).filter(Boolean)]));
const fixture = mkdtempSync('/tmp/luci-deployment-maintenance-'), fixtureID = randomUUID();
for (const name of ['state', 'projects', 'home', 'docker-config']) mkdirSync(join(fixture, name), { mode: 0o700 });
writeFileSync(join(fixture, 'docker-config/config.json'), '{}', { mode: 0o600 });
const secrets = new Set(), secret = () => { const value = randomUUID() + randomUUID(); secrets.add(value); return value; };
const password = secret(), lifecycle = secret(), scrub = value => { let result = String(value?.message ?? value); for (const item of secrets) result = result.replaceAll(item, '[redacted]'); return result; };
const result = {
  passed: false, startedAt: new Date().toISOString(), packageSourceCommit: provenance.sourceCommit, packageSourceDirty: false,
  packageArchiveSHA256: provenance.archiveSHA256, binarySHA256: binaryHash, webIndexSHA256: provenance.webIndexSHA256,
  webInventorySHA256: sha(JSON.stringify(webFiles)), verifiedWebFiles: webFiles.length, allPackageMembersMatched: true,
  driverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))),
  driverCheckout: { commit: execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repo, encoding: 'utf8' }).trim(), dirty: !!execFileSync('git', ['status', '--porcelain'], { cwd: repo, encoding: 'utf8' }).trim() },
  image: { reference: baseImage, id: image.Id, repoDigests: image.RepoDigests ?? [] },
  preflight: { dockerHost, dockerRoot: dockerInfo.DockerRootDir, freeBytes, memoryBytes: dockerInfo.MemTotal, cpus: dockerInfo.NCPU },
  scope: 'Installed persistent Docker application maintenance: A remains reachable during real unhealthy B; retained records survive installed service restart; explicit stop and reviewed C repair; owned A stop/restart preserves its exact source and image.',
  scaffolding: ['API bootstrap/account/project, Git identity and 256MiB/250m check-resource policy', 'API-approved fixture definition/design; fixture-written real clean A/B/C Git commits'],
  limitations: ['Fixture source, not AI-generated implementation or a novice usability study', 'No database, authentication/RLS, traffic cutover, zero-downtime routing, cloud/public hosting, model or payment claim', 'Axe/focus/reflow observations are bounded, not full WCAG conformance'],
  checks: [], commits: [], deployments: [], browserMutations: [], accessibility: [], visualObservations: [], appObservations: [], cleanup: [], browserErrors: [], externalRequests: [],
  modelCalls: 0, cloudResourcesCreated: 0, maxRunningAppContainers: 2, memoryMiBPerApp: 256, cpuMillisPerApp: 250,
};
const check = (name, details = {}) => { result.checks.push({ name, ...details }); console.log('PASS ' + name); };
const labels = { project: 'ai.lcfr.luci.platform.project', instance: 'ai.lcfr.luci.platform.instance', resource: 'ai.lcfr.luci.platform.resource', fixture: 'ai.lcfr.luci.acceptance.fixture' };
const inherited = Object.fromEntries(Object.entries(process.env).filter(([key]) => /^(PATH|LANG|LC_ALL|LC_CTYPE|TZ|DOCKER_HOST)$/.test(key)));
let service, browser, context, page, appPage, token, project, projectRoute, route, ownedInstance, environment, origin, previewPort, serviceLog = '', stage = 'startup', spawnError;
const known = new Map(), allowedAppOrigins = new Set();
const journeyDeadline = Date.now() + 360000;
function remaining(max = 15000) { const time = journeyDeadline - Date.now(); assert(time > 0, 'Six-minute journey deadline exceeded'); return Math.min(max, time); }
async function until(name, read, timeout = 30000, cleanup = false) {
  const end = Date.now() + (cleanup ? timeout : remaining(timeout));
  while (Date.now() < end) { if (spawnError) throw spawnError; const value = await read(); if (value) return value; await delay(200); }
  throw new Error(name + ' timed out');
}
async function api(method, path, body, expected = [200, 201], extra = {}, cleanup = false) {
  const response = await fetch(origin + path, { method, redirect: 'error', signal: AbortSignal.timeout(cleanup ? 5000 : remaining()), headers: { ...(token ? { Authorization: 'Bearer ' + token } : {}), ...(body === undefined ? {} : { 'Content-Type': 'application/json' }), ...extra }, body: body === undefined ? undefined : JSON.stringify(body) });
  assert([expected].flat().includes(response.status), method + ' ' + path + ': ' + response.status);
  const value = await response.json(); if (value.token) secrets.add(value.token); return value;
}
const observe = path => api('GET', path, undefined, 200);
async function freePort() { const s = createServer(); await new Promise((done, reject) => s.once('error', reject).listen(0, '127.0.0.1', done)); const port = s.address().port; await new Promise(done => s.close(done)); return port; }
async function start() {
  spawnError = null; service = spawn(binary, [], { cwd: fixture, env: environment, detached: true, stdio: ['ignore', 'pipe', 'pipe'] }); service.once('error', error => { spawnError = error; });
  for (const stream of [service.stdout, service.stderr]) stream.on('data', bytes => { serviceLog = (serviceLog + bytes.toString()).slice(-128 * 1024); });
  const status = await until('Installed service ready', async () => { try { return await observe('/v1/control-plane/status'); } catch { assert(service.exitCode === null && service.signalCode === null, 'Service exited'); return false; } });
  assert.equal(status.product, 'Luci'); result.version = status.version;
}
function identity(pid) { try { const stat = readFileSync(`/proc/${pid}/stat`, 'utf8'), fields = stat.slice(stat.lastIndexOf(')') + 2).trim().split(/\s+/); return { pid: Number(pid), start: fields[19], group: Number(fields[2]), status: fields[0] }; } catch (error) { if (['ENOENT', 'ESRCH'].includes(error.code)) return null; throw error; } }
const alive = row => { const current = identity(row.pid); return current && current.start === row.start && current.status !== 'Z'; };
function ownedProcesses() {
  const found = [], state = join(fixture, 'state');
  for (const pid of readdirSync('/proc').filter(value => /^\d+$/.test(value))) { const row = identity(pid); if (!row || row.status === 'Z') continue;
    try { const env = Object.fromEntries(readFileSync(`/proc/${pid}/environ`, 'utf8').split('\0').filter(Boolean).map(value => { const i = value.indexOf('='); return [value.slice(0, i), value.slice(i + 1)]; }));
      if (env.LUCI_STATE_DIR !== state && !env.LUCI_STATE_DIR?.startsWith(state + '/control-plane/')) continue;
      const executable = readlinkSync(`/proc/${pid}/exe`); if (identity(pid)?.start === row.start) found.push({ ...row, executable });
    } catch (error) { if (!['ENOENT', 'ESRCH', 'EACCES', 'EPERM'].includes(error.code)) throw error; }
  } return found;
}
async function stopService() {
  if (!service) return;
  const tracked = ownedProcesses();
  if (service.exitCode === null && service.signalCode === null) { assert(tracked.some(row => row.pid === service.pid && row.executable === binary), 'Cannot establish service ownership'); await api('POST', '/v1/control-plane/shutdown', undefined, 200, { 'X-Luci-Lifecycle-Token': lifecycle }, true); }
  await until('Gateway and workers stopped', () => !tracked.some(alive) && !ownedProcesses().length && (service.exitCode !== null || service.signalCode !== null), 15000, true);
  assert.equal(service.exitCode, 0); result.cleanup.push({ trackedProcesses: tracked.length, allStopped: true, serviceExitCode: service.exitCode });
}
async function forceStopOwned() {
  for (const signal of ['SIGTERM', 'SIGKILL']) {
    for (const row of ownedProcesses()) { try { if (alive(row) && readlinkSync(`/proc/${row.pid}/exe`) === row.executable) process.kill(row.group === row.pid ? -row.pid : row.pid, signal); } catch (error) { if (!['ENOENT', 'ESRCH'].includes(error.code)) throw error; } }
    try { await until('Owned forced cleanup', () => !ownedProcesses().length, 5000, true); return; } catch { /* Escalate only the same owned identities. */ }
  } assert.equal(ownedProcesses().length, 0);
}
function ownContainer(deploymentID) {
  assert(project?.id && ownedInstance && known.has(deploymentID));
  const ids = docker('ps', '--all', '--filter', `label=${labels.project}=${project.id}`, '--filter', `label=${labels.instance}=${ownedInstance}`, '--filter', `label=${labels.resource}=${deploymentID}`, '--format', '{{.ID}}').split(/\s+/).filter(Boolean);
  assert.equal(ids.length, 1, 'Expected exactly one recorded owned deployment container');
  const value = JSON.parse(docker('inspect', ids[0]))[0];
  assert.equal(value.Config.Labels[labels.project], project.id); assert.equal(value.Config.Labels[labels.instance], ownedInstance); assert.equal(value.Config.Labels[labels.resource], deploymentID); assert.equal(value.Config.Labels[labels.fixture], fixtureID);
  assert.equal(value.HostConfig.Memory, 256 * 1024 ** 2); assert.equal(value.HostConfig.NanoCpus, 250000000);
  let address;
  if (value.State.Running) {
    const binding = value.NetworkSettings.Ports['3000/tcp']; assert.equal(binding.length, 1); assert.equal(binding[0].HostIp, '127.0.0.1');
    address = 'http://127.0.0.1:' + binding[0].HostPort; allowedAppOrigins.add(address);
  }
  const all = docker('ps', '--filter', `label=${labels.instance}=${ownedInstance}`, '--filter', `label=${labels.project}=${project.id}`, '--format', '{{.ID}}').split(/\s+/).filter(Boolean);
  assert(all.length <= 2, 'Exceeded two running application containers');
  return { ...value, address };
}
async function witness(deploymentID, version, label, expectedHealth = 200) {
  const container = ownContainer(deploymentID); assert(container.State.Running);
  const health = await fetch(container.address + '/health', { signal: AbortSignal.timeout(remaining(3000)), redirect: 'error' }); assert.equal(health.status, expectedHealth);
  const response = await fetch(container.address + '/version', { signal: AbortSignal.timeout(remaining(3000)), redirect: 'error' }); assert.equal(response.status, 200); assert.deepEqual(await response.json(), { version, fixture: fixtureID });
  result.appObservations.push({ label, deploymentID, version, healthStatus: health.status, containerID: container.Id, imageID: container.Image, at: new Date().toISOString() });
  return container;
}
function retain(deployment) { assert.equal(deployment.projectId, project.id); assert.equal(deployment.provider, 'docker'); assert.match(deployment.id, /^deploy-[a-f0-9]+$/); known.set(deployment.id, deployment); return deployment; }
async function mutation(action, path, method = 'POST', expected = 200) {
  const pending = page.waitForResponse(response => response.request().method() === method && new URL(response.url()).origin === origin && new URL(response.url()).pathname === path, { timeout: remaining(20000) }).then(async response => { assert.equal(response.status(), expected, method + ' ' + path); const value = await response.json(); if (value.token) secrets.add(value.token); if (value.provider === 'docker' && value.id?.startsWith('deploy-')) retain(value); return value; });
  void pending.catch(() => {}); // If clicking fails, the still-pending waiter must not abort cleanup later.
  await action(); return pending;
}
const row = id => page.getByRole('article', { name: 'Deployment ' + id, exact: true });
async function loginAndOpen() {
  await page.goto(origin); await page.getByLabel('Username', { exact: true }).fill('maintenance-owner'); await page.getByLabel('Password', { exact: true }).fill(password);
  await mutation(() => page.getByRole('button', { name: 'Sign in', exact: true }).click(), '/v1/auth/login');
  const firstVisit = page.getByRole('button', { name: 'Continue to projects', exact: true }), tree = page.getByRole('treeitem', { name: /deployment-maintenance$/ });
  await until('Project navigation', async () => await firstVisit.isVisible() || await tree.isVisible()); if (await firstVisit.isVisible()) await firstVisit.click();
  await tree.click(); await openDeployments();
}
async function openDeployments() { await page.getByRole('button', { name: 'Deploy', exact: true }).click(); await page.getByRole('tab', { name: 'Application platform', exact: true }).click(); await page.getByRole('navigation', { name: 'Platform sections', exact: true }).getByRole('button', { name: 'Deployments', exact: true }).click(); await page.getByRole('region', { name: 'Persistent deployment history', exact: true }).waitFor(); }
async function settle(deployment, expected, preserve) {
  let lastWitness = 0;
  const saved = await until('Deployment ' + expected, async () => {
    if (preserve && Date.now() - lastWitness >= 2000) { await witness(preserve.id, 'A', 'A remains reachable during B admission'); lastWitness = Date.now(); }
    const state = await observe(route), item = state.project.deployments.find(value => value.id === deployment.id);
    assert(item && item.commit === deployment.commit && item.projectId === project.id);
    return !state.busy && !['building', 'deploying'].includes(item.status) && item;
  }, 100000);
  assert.equal(saved.status, expected, saved.message); retain(saved);
  await row(saved.id).getByText(expected, { exact: true }).waitFor({ timeout: remaining() });
  return saved;
}
async function visibleGeometry(locator) {
  return locator.evaluate(node => {
    const box = node.getBoundingClientRect(), style = getComputedStyle(node);
    const rect = value => ({ x: value.x, y: value.y, width: value.width, height: value.height });
    const clip = { left: 0, top: 0, right: innerWidth, bottom: innerHeight };
    for (let parent = node.parentElement; parent; parent = parent.parentElement) {
      const bounds = parent.getBoundingClientRect(), css = getComputedStyle(parent);
      if (/(auto|scroll|hidden|clip)/.test(css.overflowX)) { clip.left = Math.max(clip.left, bounds.left + parent.clientLeft); clip.right = Math.min(clip.right, bounds.left + parent.clientLeft + parent.clientWidth); }
      if (/(auto|scroll|hidden|clip)/.test(css.overflowY)) { clip.top = Math.max(clip.top, bounds.top + parent.clientTop); clip.bottom = Math.min(clip.bottom, bounds.top + parent.clientTop + parent.clientHeight); }
    }
    const full = box.width > 0 && box.height > 0 && clip.left <= box.left + 1 && clip.top <= box.top + 1 && clip.right >= box.right - 1 && clip.bottom >= box.bottom - 1;
    const outlineWidth = parseFloat(style.outlineWidth) || 0, outlineOffset = parseFloat(style.outlineOffset) || 0;
    const outlineOutset = Math.max(0, outlineWidth + outlineOffset);
    const outline = { left: box.left - outlineOutset, top: box.top - outlineOutset, right: box.right + outlineOutset, bottom: box.bottom + outlineOutset };
    const outlineUnclipped = style.outlineStyle !== 'none' && outlineWidth > 0 && clip.left <= outline.left && clip.top <= outline.top && clip.right >= outline.right && clip.bottom >= outline.bottom;
    const samples = [[0.5, 0.5], [0.2, 0.2], [0.8, 0.8]].map(([x, y]) => {
      const hit = document.elementFromPoint(box.left + box.width * x, box.top + box.height * y);
      return !!hit && (hit === node || node.contains(hit));
    });
    return { rect: rect(box), clip, fullyVisible: full && samples.every(Boolean), hitTests: samples, outline: { ...outline, outset: outlineOutset, unclipped: outlineUnclipped },
      active: document.activeElement === node, focusVisible: node.matches(':focus-visible'),
      focusStyle: { outlineStyle: style.outlineStyle, outlineWidth: style.outlineWidth, outlineOffset: style.outlineOffset, outlineColor: style.outlineColor, boxShadow: style.boxShadow },
      labels: [...(node.labels ?? [])].map(label => ({ text: label.textContent.trim(), rect: rect(label.getBoundingClientRect()) })) };
  });
}
async function snapshot(name, width = 1440, review, prepare) {
  await page.setViewportSize({ width, height: width === 390 ? 844 : 1050 });
  const history = review ?? page.getByRole('region', { name: 'Persistent deployment history', exact: true });
  if (prepare) await prepare(); else await history.scrollIntoViewIfNeeded();
  const dimensions = await page.evaluate(() => ({ viewport: innerWidth, document: document.documentElement.scrollWidth })); assert(dimensions.document <= dimensions.viewport + 1, 'Horizontal overflow at ' + name);
  await page.evaluate(axe.source);
  const audit = await history.evaluate(async node => { const audit = await window.axe.run(node, { runOnly: ['wcag2a', 'wcag2aa', 'wcag21aa'] }); const select = rows => rows.map(row => ({ id: row.id, impact: row.impact, nodes: row.nodes.map(node => ({ target: node.target, failureSummary: node.failureSummary })) })); return { violations: select(audit.violations), incomplete: select(audit.incomplete) }; });
  writeFileSync(join(evidence, name + '-accessibility.json'), JSON.stringify(audit, null, 2), { mode: 0o600 }); result.accessibility.push({ state: name, width, dimensions, violations: audit.violations.map(row => row.id), incomplete: audit.incomplete.map(row => row.id) });
  await page.screenshot({ path: join(evidence, name + '.png'), fullPage: true }); assert.deepEqual(audit.violations, [], 'Automated accessibility violation');
}
async function repairFieldSnapshot(form, width) {
  const name = '04-repair-fields-' + (width === 390 ? 'mobile' : 'desktop');
  await snapshot(name, width, form, async () => {
    const port = form.getByLabel('Application port', { exact: true }), health = form.getByLabel('Health check path', { exact: true });
    await port.scrollIntoViewIfNeeded(); await port.focus(); await page.keyboard.press('Tab');
    const observations = [];
    for (const [label, control] of [['Application port', port], ['Health check path', health]]) {
      const input = await visibleGeometry(control), wrapper = await visibleGeometry(control.locator('..'));
      assert(input.fullyVisible && wrapper.fullyVisible, label + ' and its label must be visible at ' + width);
      assert(input.rect.width >= 24 && input.rect.height >= 24, label + ' must have a usable target');
      assert.deepEqual(input.labels.map(value => value.text), [label]);
      observations.push({ label, input, wrapper });
    }
    const focused = observations[1].input;
    assert(focused.active && focused.focusVisible, 'Native Tab must focus the visible Health check path field');
    assert(focused.outline.unclipped, 'Focused field must expose its complete focus ring within the viewport and scroll clips');
    assert.equal(await port.inputValue(), '3000'); assert.equal(await health.inputValue(), '/health');
    result.visualObservations.push({ state: name, width, keyboard: 'Application port → Tab → Health check path', fields: observations });
  });
}
function appSource(version) {
  return `const http=require('node:http');const version=${JSON.stringify(version)},fixture=${JSON.stringify(fixtureID)};console.log('FIXTURE '+version+' '+(version==='B'?'intentional unhealthy rollout':'healthy application'));http.createServer((req,res)=>{if(req.url==='/health'){res.writeHead(version==='B'?503:200,{'content-type':'application/json'});res.end(JSON.stringify({version,healthy:version!=='B'}));return;}if(req.url==='/version'){res.writeHead(200,{'content-type':'application/json'});res.end(JSON.stringify({version,fixture}));return;}res.writeHead(200,{'content-type':'text/html; charset=utf-8'});res.end('<!doctype html><html lang="en"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Maintenance fixture '+version+'</title><main><h1>Application version '+version+'</h1><p>This is the owned deployment maintenance fixture.</p><button id="check">Check application version</button><p id="result" role="status"></p></main><script>document.getElementById("check").onclick=async()=>{const r=await fetch("/version");const v=await r.json();document.getElementById("result").textContent="Serving fixture version "+v.version;};</script></html>');}).listen(3000,'0.0.0.0');\n`;
}
async function seedCommit(version) {
  assert(realpathSync(project.path).startsWith(realpathSync(fixture) + sep));
  writeFileSync(join(project.path, 'app.cjs'), appSource(version), { mode: 0o644 });
  writeFileSync(join(project.path, 'Dockerfile'), `FROM ${baseImage}\nLABEL ${labels.fixture}=${JSON.stringify(fixtureID)}\nWORKDIR /app\nCOPY app.cjs /app/app.cjs\nUSER 1000:1000\nEXPOSE 3000\nENTRYPOINT ["node","/app/app.cjs"]\n`, { mode: 0o600 });
  await api('POST', projectRoute + '/git/commit', { message: 'Fixture version ' + version + ' for explicit deployment maintenance acceptance' });
  const [git, delivery] = await Promise.all([observe(projectRoute + '/git'), observe(projectRoute + '/git/delivery')]);
  assert(git.clean); assert.match(delivery.head, /^[a-f0-9]{40}$/);
  result.commits.push({ version, commit: delivery.head, clean: true, appSHA256: sha(readFileSync(join(project.path, 'app.cjs'))), dockerfileSHA256: sha(readFileSync(join(project.path, 'Dockerfile'))) });
  return delivery.head;
}
async function browserApp(deployment, version) {
  await witness(deployment.id, version, 'Before UI open');
  const opened = context.waitForEvent('page', { timeout: remaining() });
  void opened.catch(() => {});
  const button = row(deployment.id).getByRole('button', { name: 'Open deployed application', exact: true });
  if (await button.count()) await button.click(); else await row(deployment.id).getByRole('link', { name: 'Open deployed application', exact: true }).click();
  appPage = await opened; appPage.setDefaultTimeout(remaining());
  await appPage.getByRole('heading', { name: 'Application version ' + version, exact: true }).waitFor();
  await appPage.getByRole('button', { name: 'Check application version', exact: true }).focus(); await appPage.keyboard.press('Enter');
  await appPage.getByRole('status').filter({ hasText: 'Serving fixture version ' + version }).waitFor();
  await appPage.screenshot({ path: join(evidence, 'application-' + version + '.png'), fullPage: true }); await appPage.close(); appPage = null;
}
async function cleanupResources() {
  if (!project || !ownedInstance) return;
  assert.equal(ownedProcesses().length, 0, 'Private writers must stop before Docker cleanup');
  const filters = ['--filter', `label=${labels.project}=${project.id}`, '--filter', `label=${labels.instance}=${ownedInstance}`];
  for (const id of docker('ps', '--all', ...filters, '--format', '{{.ID}}').split(/\s+/).filter(Boolean)) {
    const entry = JSON.parse(docker('inspect', id))[0]; assert.equal(entry.Config.Labels[labels.fixture], fixtureID); assert.equal(entry.Config.Labels[labels.instance], ownedInstance); assert.equal(entry.Config.Labels[labels.project], project.id); assert.match(entry.Config.Labels[labels.resource], /^deploy-[a-f0-9]+$/);
    docker('rm', '--force', id); result.cleanup.push({ container: entry.Id });
  }
  assert.equal(docker('volume', 'ls', ...filters, '--quiet'), '', 'This fixture must create no volumes');
  for (const id of docker('network', 'ls', ...filters, '--quiet').split(/\s+/).filter(Boolean)) { const entry = JSON.parse(docker('network', 'inspect', id))[0]; assert.equal(entry.Labels[labels.instance], ownedInstance); assert.equal(entry.Labels[labels.project], project.id); assert.equal(Object.keys(entry.Containers ?? {}).length, 0); docker('network', 'rm', id); result.cleanup.push({ network: entry.Id }); }
  for (const id of [...new Set(docker('image', 'ls', '--filter', `label=${labels.fixture}=${fixtureID}`, '--quiet', '--no-trunc').split(/\s+/).filter(Boolean))]) {
    const entry = JSON.parse(docker('image', 'inspect', id))[0]; assert.equal(entry.Config.Labels[labels.fixture], fixtureID); assert.notEqual(entry.Id, image.Id);
    for (const tag of entry.RepoTags ?? []) assert(tag.startsWith('luci-platform/' + ownedInstance + ':deploy-'), 'Do not remove an image adopted elsewhere');
    docker('image', 'rm', id); result.cleanup.push({ image: entry.Id });
  }
  assert.equal(docker('ps', '--all', ...filters, '--format', '{{.ID}}'), ''); assert.equal(docker('network', 'ls', ...filters, '--quiet'), '');
  for (const [kind, ids] of Object.entries(preexisting)) for (const id of ids) docker(kind, 'inspect', id);
  assert.equal(JSON.parse(docker('image', 'inspect', baseImage))[0].Id, image.Id);
  result.cleanup.push({ ownedResourcesRemoved: true, preexistingResourcesRetained: true });
}

try {
  const port = await freePort(); previewPort = await freePort(); origin = 'http://127.0.0.1:' + port;
  environment = { ...inherited, HOME: join(fixture, 'home'), USERPROFILE: join(fixture, 'home'), DOCKER_CONFIG: join(fixture, 'docker-config'), DOCKER_HOST: dockerHost, LUCI_DOCKER_HOST: dockerHost, LUCI_ADDR: '127.0.0.1:' + port, LUCI_BROWSER_PREVIEW_ADDR: '127.0.0.1:' + previewPort, LUCI_BROWSER_PREVIEW_PORT: String(previewPort), LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'), LUCI_BACKEND: 'docker', LUCI_RUNTIME_DIR: join(dirname(binary), 'runtime'), LUCI_WEB_DIR: web, LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, GIT_CONFIG_GLOBAL: join(fixture, 'home/.gitconfig'), GIT_CONFIG_NOSYSTEM: '1', AWS_EC2_METADATA_DISABLED: 'true' };
  await start(); const bootstrapToken = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(); secrets.add(bootstrapToken);
  const owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken, username: 'maintenance-owner', displayName: 'Maintenance fixture', password }); token = owner.token;
  await api('PATCH', '/v1/admin/policy', { checkMemoryMiB: 256, checkCPUMillis: 250 });
  await api('POST', '/v1/setup/git/identity', { name: 'Maintenance fixture', email: 'maintenance@example.invalid' });
  project = await api('POST', '/v1/product/projects', { name: 'deployment-maintenance' }); projectRoute = '/v1/product/projects/' + project.id; route = '/v1/platform/projects/' + project.id; result.projectID = project.id;
  const workerRoot = join(fixture, 'state/control-plane/users', owner.user.id, 'runtime'), localPath = join(fixture, 'projects/deployment-maintenance');
  const local = existsSync(localPath) && realpathSync(project.path) === realpathSync(localPath); if (!local) assert.equal(realpathSync(project.path), realpathSync(join(workerRoot, 'projects/deployment-maintenance')));
  const workerState = local ? join(fixture, 'state') : join(workerRoot, 'state'), instanceID = readFileSync(join(workerState, 'instance/instance-id'), 'utf8').trim(); assert.match(instanceID, /^[a-f0-9-]{36}$/);
  ownedInstance = sha(join(workerState, 'sessions') + instanceID).slice(0, 16); result.platformInstance = ownedInstance;
  await api('POST', projectRoute + '/brief', { revision: 0, purpose: 'Inspect and recover an owned deployment without replacing a healthy service accidentally.', audience: 'Application maintainers', constraints: 'Local fixture only. Preserve healthy A while intentionally unhealthy B is diagnosed. No public hosting or data claim.', criteria: ['The application reports its exact deployed version and a working health endpoint.'] });
  let specification = await observe(projectRoute + '/specification'); specification = await api('POST', projectRoute + '/specification', { expectedRevision: specification.revision, expectedBriefRevision: specification.briefRevision, specification: { ...specification, requirements: [{ id: 'AC-version', title: 'Inspect deployed application version', priority: 'must', acceptance: 'The visible application identifies its deployed version and Check application version retrieves that same version.' }] } });
  specification = await api('POST', projectRoute + '/specification/approve', { revision: specification.revision, briefRevision: specification.briefRevision });
  let design = await observe(projectRoute + '/design-plan'); design = await api('POST', projectRoute + '/design-plan', { expectedRevision: design.revision, expectedBriefRevision: specification.briefRevision, expectedSpecificationRevision: specification.revision, plan: { ...design, screens: [{ id: 'version', name: 'Application version', route: '/', purpose: 'Identify the actual running fixture', states: ['Healthy', 'Unhealthy'], widths: [390, 1440], requirementIds: ['AC-version'] }] } });
  await api('POST', projectRoute + '/design-plan/approve', { revision: design.revision });
  const commitA = await seedCommit('A');
  const platformStatus = await observe('/v1/platform/status'); assert(platformStatus.dockerReady); assert.deepEqual(platformStatus.limits, { memoryMiB: 256, cpuMillis: 250 });
  browser = await chromium.launch({ headless: true, executablePath: executable, env: inherited }); result.browserVersion = browser.version(); context = await browser.newContext({ viewport: { width: 1440, height: 1050 }, serviceWorkers: 'block' });
  await context.route('**/*', intercepted => { const url = new URL(intercepted.request().url());
    const ownedGateway = url.protocol === 'http:' && url.port === String(previewPort) && /^[a-f0-9]{32}\.[a-f0-9]{32}\.localhost$/.test(url.hostname);
    if (url.origin === origin || allowedAppOrigins.has(url.origin) || ownedGateway) return intercepted.continue();
    result.externalRequests.push({ origin: url.origin, path: url.pathname }); return intercepted.abort('blockedbyclient');
  });
  context.on('page', tab => tab.on('pageerror', error => result.browserErrors.push(scrub(error))));
  page = await context.newPage(); page.setDefaultTimeout(15000);
  page.on('request', req => { const url = new URL(req.url()); if (url.origin === origin && url.pathname.startsWith(route + '/') && ['POST', 'PATCH', 'PUT', 'DELETE'].includes(req.method())) result.browserMutations.push({ path: url.pathname, method: req.method(), body: req.postDataJSON() }); });
  await loginAndOpen(); stage = 'healthy-A'; await page.getByLabel('Health check path', { exact: true }).fill('/health');
  let A = await mutation(() => page.getByRole('button', { name: 'Start persistent deployment', exact: true }).click(), route + '/deployments', 'POST', 202); assert.equal(A.commit, commitA); A = await settle(A, 'ready'); await browserApp(A, 'A');
  const originalA = await witness(A.id, 'A', 'Healthy A baseline'); await snapshot('01-healthy-A');
  check('Exact committed A deploys through UI and its actual application identifies version A', { deploymentID: A.id, commit: A.commit, containerID: originalA.Id, imageID: originalA.Image });

  stage = 'unhealthy-B'; const commitB = await seedCommit('B'); assert.notEqual(commitA, commitB); await page.reload(); await openDeployments(); await page.getByLabel('Health check path', { exact: true }).fill('/health');
  let B = await mutation(() => page.getByRole('button', { name: 'Start persistent deployment', exact: true }).click(), route + '/deployments', 'POST', 202); assert.equal(B.commit, commitB); B = await settle(B, 'failed', A);
  assert(!B.url); assert.equal(await row(B.id).getByRole('link', { name: 'Open deployed application', exact: true }).count(), 0); assert.equal(await row(B.id).getByRole('button', { name: 'Open deployed application', exact: true }).count(), 0);
  await witness(B.id, 'B', 'Real unhealthy B', 503); await witness(A.id, 'A', 'A after B failed');
  const logs = await mutation(() => row(B.id).getByRole('button', { name: 'Read application logs', exact: true }).click(), route + '/deployments/' + B.id + '/logs', 'GET'); assert(logs.text.includes('FIXTURE B intentional unhealthy rollout')); await row(B.id).getByText(/FIXTURE B intentional unhealthy rollout/).waitFor();
  await snapshot('02-failed-B-retains-A-mobile', 390, row(B.id), async () => {
    const status = row(B.id).getByText('failed', { exact: true }), stop = row(B.id).getByRole('button', { name: 'Stop deployment', exact: true });
    await status.scrollIntoViewIfNeeded(); await stop.focus(); await page.keyboard.press('Tab'); await page.keyboard.press('Shift+Tab');
    const statusGeometry = await visibleGeometry(status), control = await visibleGeometry(stop), article = await visibleGeometry(row(B.id));
    assert(statusGeometry.fullyVisible && control.fullyVisible, 'Failed B status and Stop deployment must be visible in the mobile capture');
    assert(control.active && control.focusVisible, 'Failed B stop control must retain visible keyboard focus');
    assert(control.outline.unclipped, 'Failed B stop control must expose its complete focus ring within the viewport and scroll clips');
    result.visualObservations.push({ state: '02-failed-B-retains-A-mobile', width: 390, deploymentID: B.id, article, status: statusGeometry, focusedControl: control });
  }); await page.setViewportSize({ width: 1440, height: 1050 });
  check('Real committed B fails health, exposes bounded application logs and has no ready link while A stays reachable', { deploymentID: B.id, commit: B.commit, retainedA: A.id, originalAHealthySamples: result.appObservations.filter(row => row.label === 'A remains reachable during B admission').length });

  stage = 'service-restart'; await stopService(); await witness(A.id, 'A', 'A during stopped control service'); await witness(B.id, 'B', 'B during stopped control service', 503);
  await context.close(); await start(); context = await browser.newContext({ viewport: { width: 1440, height: 1050 }, serviceWorkers: 'block' });
  // Reuse the same owned-origin rule for the new browser account session.
  await context.route('**/*', intercepted => { const url = new URL(intercepted.request().url()); if (url.origin === origin || allowedAppOrigins.has(url.origin) || (url.protocol === 'http:' && url.port === String(previewPort) && /^[a-f0-9]{32}\.[a-f0-9]{32}\.localhost$/.test(url.hostname))) return intercepted.continue(); result.externalRequests.push({ origin: url.origin, path: url.pathname }); return intercepted.abort('blockedbyclient'); });
  context.on('page', tab => tab.on('pageerror', error => result.browserErrors.push(scrub(error)))); page = await context.newPage(); page.setDefaultTimeout(15000);
  page.on('request', req => { const url = new URL(req.url()); if (url.origin === origin && url.pathname.startsWith(route + '/') && ['POST', 'PATCH', 'PUT', 'DELETE'].includes(req.method())) result.browserMutations.push({ path: url.pathname, method: req.method(), body: req.postDataJSON() }); });
  const mutationCount = result.browserMutations.length; await loginAndOpen(); await row(A.id).getByText('ready', { exact: true }).waitFor(); await row(B.id).getByText('failed', { exact: true }).waitFor(); assert.equal(result.browserMutations.length, mutationCount, 'Returning navigation must not deploy/restart anything');
  const afterRestart = (await observe(route)).project.deployments; assert.equal(afterRestart.find(row => row.id === A.id).commit, commitA); assert.equal(afterRestart.find(row => row.id === B.id).commit, commitB);
  const reobservedA = await witness(A.id, 'A', 'A after control restart'); assert.equal(reobservedA.Id, originalA.Id); assert.equal(reobservedA.Image, originalA.Image); await snapshot('03-service-restart-retains-both');
  check('Installed service restart and fresh browser sign-in preserve exact healthy and failed deployment records without resource mutations');

  stage = 'explicit-repair-C'; const stoppedB = await mutation(() => row(B.id).getByRole('button', { name: 'Stop deployment', exact: true }).click(), route + '/deployments/' + B.id + '/stop'); assert.equal(stoppedB.status, 'stopped'); assert.equal(ownContainer(B.id).State.Running, false);
  const commitC = await seedCommit('C'); assert(![commitA, commitB].includes(commitC)); await page.reload(); await openDeployments();
  const beforeReview = result.browserMutations.length; const prepare = row(B.id).getByRole('button', { name: 'Prepare replacement deployment', exact: true }); await prepare.focus(); await page.keyboard.press('Enter');
  const form = page.locator('form').filter({ has: page.getByRole('heading', { name: 'Review replacement deployment', exact: true }) }); await form.waitFor();
  assert((await form.innerText()).includes(commitB) && (await form.innerText()).includes(commitC)); assert((await form.innerText()).includes('not a rollback')); assert.equal(await form.evaluate(node => document.activeElement === node), true);
  const repair = form.getByRole('button', { name: 'Start replacement deployment', exact: true }); assert.equal(await repair.isEnabled(), false); assert.equal(result.browserMutations.length, beforeReview); await page.getByLabel('Health check path', { exact: true }).fill('/health'); await snapshot('04-explicit-repair-review', 1440, form);
  await repairFieldSnapshot(form, 1440); await repairFieldSnapshot(form, 390); await page.setViewportSize({ width: 1440, height: 1050 });
  assert.equal(await repair.isEnabled(), false); assert.equal(result.browserMutations.length, beforeReview, 'Inspecting field layout must not mutate deployments');
  await form.getByRole('checkbox', { name: 'Create this replacement and retain the earlier deployment.', exact: true }).check();
  let C = await mutation(() => repair.click(), route + '/deployments', 'POST', 202); assert.equal(C.commit, commitC); C = await settle(C, 'ready'); await browserApp(C, 'C'); await witness(A.id, 'A', 'A retained after repaired C');
  const builds = result.browserMutations.filter(row => row.path === route + '/deployments'); assert.deepEqual(builds.map(row => row.body.commit), [commitA, commitB, commitC]);
  check('Explicit stop of B and source-reviewed C replacement create one repaired deployment while retaining A and B', { stoppedB: B.id, repairedC: C.id, commit: commitC, deploymentPOSTs: builds.length });

  stage = 'owned-A-stop-restart'; const stoppedA = await mutation(() => row(A.id).getByRole('button', { name: 'Stop deployment', exact: true }).click(), route + '/deployments/' + A.id + '/stop'); assert.equal(stoppedA.status, 'stopped'); assert.equal(ownContainer(A.id).State.Running, false);
  const restartedA = await mutation(() => row(A.id).getByRole('button', { name: 'Restart deployment', exact: true }).click(), route + '/deployments/' + A.id + '/restart'); assert.equal(restartedA.status, 'ready'); assert.equal(restartedA.commit, commitA);
  const finalA = await witness(A.id, 'A', 'A explicitly restarted'); assert.equal(finalA.Id, originalA.Id); assert.equal(finalA.Image, originalA.Image); await witness(C.id, 'C', 'C remains healthy during A restart'); await snapshot('05-final-maintenance-history');
  check('Explicit A stop/restart retains its original container/image/source while repaired C remains healthy');
  const final = (await observe(route)).project.deployments; assert.equal(final.length, 3); assert.deepEqual(Object.fromEntries(final.map(row => [row.commit, row.status])), { [commitA]: 'ready', [commitB]: 'stopped', [commitC]: 'ready' }); result.deployments = final.map(({ url, ...record }) => record);
  assert.deepEqual(result.browserErrors, []); assert.deepEqual(result.externalRequests, []); result.passed = true;
} catch (error) {
  result.error = scrub(error); result.failedStage = stage;
  if (page && !page.isClosed()) { await page.screenshot({ path: join(evidence, 'failure.png'), fullPage: true }).catch(() => {}); try { writeFileSync(join(evidence, 'failure-dom.txt'), scrub(await page.locator('body').innerText({ timeout: 3000 })), { mode: 0o600 }); } catch { /* Keep receipt even after a browser fault. */ } }
} finally {
  const failedCleanup = error => { (result.cleanupErrors ??= []).push(scrub(error)); };
  try { await browser?.close(); result.cleanup.push({ browserClosed: true }); } catch (error) { failedCleanup(error); }
  try { await stopService(); } catch (error) { failedCleanup(error); try { await forceStopOwned(); } catch (cause) { failedCleanup(cause); } }
  try { await cleanupResources(); } catch (error) { failedCleanup(error); }
  try { assert.equal(sha(readFileSync(binary)), binaryHash); assert.deepEqual(webInventory(web), webFiles); result.packageUnchanged = true; } catch (error) { failedCleanup(error); }
  writeFileSync(join(evidence, 'control-plane.log'), scrub(serviceLog), { mode: 0o600 });
  if (!result.cleanupErrors?.length) { try { rmSync(fixture, { recursive: true }); result.privateFixtureRemoved = true; } catch (error) { failedCleanup(error); } } else result.retainedPrivateFixture = fixture;
  result.passed &&= !result.cleanupErrors?.length; result.finishedAt = new Date().toISOString(); writeFileSync(report, JSON.stringify(result, null, 2), { mode: 0o600, flag: 'wx' });
  console.log(JSON.stringify({ passed: result.passed, checks: result.checks.length, report, error: result.error, cleanupErrors: result.cleanupErrors })); if (!result.passed) process.exitCode = 1;
}

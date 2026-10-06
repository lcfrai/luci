#!/usr/bin/env node
// Actual team worker + pinned Codex + Docker checks/release; local fixture only.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { createServer as httpServer } from 'node:http';
import { createServer } from 'node:net';
import { createWriteStream, existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { dirname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
const repository = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const args = process.argv.slice(2);
if (args.includes('--help')) { console.log('node scripts/team-run-smoke.mjs --binary PATH --image EXISTING_TRUSTED_IMAGE [--release-image EXISTING_NODE_IMAGE] [--report PATH] [--keep-fixture]\nCreates a fresh private instance and owned local containers. No paid inference, cloud resources, runtime build or image pull.'); process.exit(0); }
function arg(name, fallback) { const i = args.indexOf(name); return i < 0 ? fallback : args[i + 1]; }
const binary = resolve(arg('--binary', process.env.LUCI_TEAM_SMOKE_BINARY ?? 'artifacts/luci'));
const image = arg('--image', process.env.LUCI_RUNTIME_SMOKE_IMAGE);
const releaseImage = arg('--release-image', process.env.LUCI_RELEASE_SMOKE_IMAGE ?? image);
assert(image, 'Supply --image or LUCI_RUNTIME_SMOKE_IMAGE'); assert.equal(process.platform, 'linux'); assert.notEqual(process.getuid(), 0); assert(existsSync(binary));
const report = resolve(arg('--report', `artifacts/uat/team-run-${Date.now()}/results.json`));
mkdirSync(dirname(report), { recursive: true, mode: 0o700 });
const fixture = mkdtempSync(join(dirname(report), '.private-team-run-'));
for (const name of ['home', 'projects', 'state']) mkdirSync(join(fixture, name), { mode: 0o700 });
const docker = (...argv) => execFileSync('docker', argv, { encoding: 'utf8', timeout: 30_000 }).trim();
const imageInfo = JSON.parse(docker('image', 'inspect', image))[0]; assert.match(imageInfo.Id, /^sha256:[a-f0-9]{64}$/);
const releaseImageInfo = JSON.parse(docker('image', 'inspect', releaseImage))[0]; assert.match(releaseImageInfo.Id, /^sha256:[a-f0-9]{64}$/);
const bridge = docker('network', 'inspect', 'bridge', '--format', '{{(index .IPAM.Config 0).Gateway}}'); assert.match(bridge, /^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.)/);
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const result = { passed: false, startedAt: new Date().toISOString(), binarySHA256: sha(readFileSync(binary)), imageId: imageInfo.Id, releaseImage: { reference: releaseImage, id: releaseImageInfo.Id, repoDigests: releaseImageInfo.RepoDigests ?? [] }, externalModelCalls: 0, cloudResourcesCreated: 0, checks: [], scope: 'Actual main service, dedicated team worker, two accounts, reviewed run, installed Codex CLI against local fixture, Docker checks and local release. Verifies protocol and execution, not model quality or public deployment.' };
const checked = (name, details = {}) => { result.checks.push({ name, passed: true, ...details }); console.log(`PASS ${name}`); };
const key = `fixture-only-${randomUUID()}`;
let expectedPrompt = [], expectedContext = {}, toolSent = false, providerFinished = false, fixtureError, calls = 0;
const appSource = "const http=require('node:http');const server=http.createServer((req,res)=>{res.writeHead(200,{'content-type':'application/json'});res.end(JSON.stringify({status:'ok',createdBy:'actual-team-codex-tool',modelKeyPresent:Boolean(process.env.OPENAI_API_KEY)}));});server.listen(3000,'0.0.0.0');module.exports=server;\n";
const testSource = "const assert=require('node:assert/strict');const server=require('./app.cjs');server.on('listening',async()=>{try{const r=await fetch('http://127.0.0.1:3000/health');assert.equal(r.status,200);assert.deepEqual(await r.json(),{status:'ok',createdBy:'actual-team-codex-tool',modelKeyPresent:false});console.log('team fixture HTTP acceptance passed');}catch(e){console.error(e);process.exitCode=1;}finally{server.close();}});\n";
const provider = httpServer(async (req, res) => {
  try {
    assert.equal(req.url, '/v1/responses'); assert.equal(req.method, 'POST'); assert.equal(req.headers.authorization, `Bearer ${key}`);
    const chunks = []; let size = 0; for await (const c of req) { chunks.push(c); size += c.length; assert(size < 4 * 1024 * 1024); }
    const body = JSON.parse(Buffer.concat(chunks).toString()), serialized = JSON.stringify(body); calls++; writeFileSync(join(fixture, `provider-request-${calls}.json`), JSON.stringify(body), { mode: 0o600 });
    const tools = Object.fromEntries((body.tools ?? []).map(t => [t.name ?? t.function?.name, t])); let item;
    const auxiliary = Object.keys(tools).length === 0;
    if (!auxiliary) for (const text of expectedPrompt) assert(serialized.includes(text), `CLI omitted bound context: ${text}`);
    if (!toolSent && tools.exec_command) {
      assert(tools.exec_command.parameters.properties.cmd);
      const payload = `const fs=require('node:fs'),crypto=require('node:crypto');const expected=${JSON.stringify(expectedContext)};const files={};for(const [p,digest] of Object.entries(expected)){const hash=crypto.createHash('sha256').update(fs.readFileSync(p)).digest('hex');if(hash!==digest)throw Error('Reviewed bytes changed: '+p);files[p]=hash;}fs.writeFileSync('app.cjs',${JSON.stringify(appSource)});fs.writeFileSync('acceptance.cjs',${JSON.stringify(testSource)});fs.writeFileSync('Dockerfile',${JSON.stringify(`FROM ${releaseImage}\nWORKDIR /app\nCOPY app.cjs /app/app.cjs\nUSER 1000:1000\nEXPOSE 3000\nENTRYPOINT ["node","/app/app.cjs"]\n`)});fs.writeFileSync('implementation-proof.json',JSON.stringify({uid:process.getuid(),cwd:process.cwd(),files,status:fs.readFileSync('/proc/self/status','utf8')}));`;
      item = { id: 'fc_team', type: 'function_call', status: 'completed', name: 'exec_command', call_id: 'call_team_1', arguments: JSON.stringify({ cmd: `node -e "eval(Buffer.from('${Buffer.from(payload).toString('base64')}','base64').toString())"`, workdir: '/home/dev/work', login: false, max_output_tokens: 1000 }) }; toolSent = true;
    } else {
      if (!auxiliary) { assert(toolSent, 'Pinned CLI did not advertise exec_command'); assert(serialized.includes('function_call_output'), 'CLI omitted tool result'); providerFinished = true; }
      item = { id: 'msg_team', type: 'message', status: 'completed', role: 'assistant', content: [{ type: 'output_text', text: 'Created application and acceptance check. Run independent Luci checks before acceptance.', annotations: [] }] };
    }
    const response = { id: `resp_team_${calls}`, object: 'response', created_at: 1, status: 'completed', model: 'fixture-model', output: [item], usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2, input_tokens_details: { cached_tokens: 0 }, output_tokens_details: { reasoning_tokens: 0 } } };
    const events = [{ type: 'response.created', response: { ...response, status: 'in_progress', output: [] } }, { type: 'response.output_item.added', output_index: 0, item: { ...item, status: 'in_progress', ...(item.type === 'function_call' ? { arguments: '' } : { content: [] }) } }];
    if (item.type === 'function_call') events.push({ type: 'response.function_call_arguments.delta', item_id: item.id, output_index: 0, delta: item.arguments }, { type: 'response.function_call_arguments.done', item_id: item.id, output_index: 0, arguments: item.arguments });
    else events.push({ type: 'response.content_part.added', item_id: item.id, output_index: 0, content_index: 0, part: { type: 'output_text', text: '', annotations: [] } }, { type: 'response.output_text.delta', item_id: item.id, output_index: 0, content_index: 0, delta: item.content[0].text }, { type: 'response.output_text.done', item_id: item.id, output_index: 0, content_index: 0, text: item.content[0].text });
    events.push({ type: 'response.output_item.done', output_index: 0, item }, { type: 'response.completed', response });
    res.writeHead(200, { 'content-type': 'text/event-stream' }); res.end(events.map((e, i) => `event: ${e.type}\ndata: ${JSON.stringify({ ...e, sequence_number: i })}\n\n`).join(''));
  } catch (error) { fixtureError = error; res.writeHead(500); res.end('Fixture contract failed'); }
});
await new Promise((done, reject) => provider.once('error', reject).listen(0, bridge, done));
const providerBase = `http://host.docker.internal:${provider.address().port}/v1`;
const listener = createServer(); await new Promise(done => listener.listen(0, '127.0.0.1', done)); const port = listener.address().port; await new Promise(done => listener.close(done));
const origin = `http://127.0.0.1:${port}`;
// Never inherit caller model/cloud/SSH/GitHub credentials or managed host binding.
const env = Object.fromEntries(['PATH', 'LANG', 'LC_ALL', 'TZ', 'DOCKER_HOST'].filter(k => process.env[k]).map(k => [k, process.env[k]]));
const dockerConfig = join(fixture, 'docker-config'); mkdirSync(dockerConfig, { mode: 0o700 });
writeFileSync(join(dockerConfig, 'config.json'), JSON.stringify({ cliPluginsExtraDirs: process.env.DOCKER_CONFIG ? [join(process.env.DOCKER_CONFIG, 'cli-plugins')] : [] }), { mode: 0o600 });
Object.assign(env, { DOCKER_CONFIG: dockerConfig, HOME: join(fixture, 'home'), LUCI_ADDR: `127.0.0.1:${port}`, LUCI_BACKEND: 'docker', LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'), LUCI_SANDBOX_IMAGE: image, LUCI_RUNTIME_DIR: repository, LUCI_LOG_LEVEL: 'error', LUCI_GIT_NAME: 'Team Acceptance', LUCI_GIT_EMAIL: 'team-acceptance@example.invalid', GIT_CONFIG_GLOBAL: join(fixture, 'home/.gitconfig'), GIT_CONFIG_NOSYSTEM: '1' });
const log = createWriteStream(join(fixture, 'service.log'), { mode: 0o600 });
const child = spawn(binary, [], { cwd: fixture, env, detached: true, stdio: ['ignore', 'pipe', 'pipe'] }); child.stdout.pipe(log, { end: false }); child.stderr.pipe(log, { end: false });
let owner, editor, team, project, session, release, workspace, projectRoute, socket, closed, spawnFailure;
child.once('error', error => { spawnFailure = error; });
const sessionIDs = [], releaseIDs = [];
async function api(method, path, body, token = owner?.token, expected, timeout = 35_000) {
  const response = await fetch(origin + path, { method, redirect: 'error', signal: AbortSignal.timeout(timeout), headers: { Origin: origin, ...(token ? { Authorization: `Bearer ${token}` } : {}), ...(body !== undefined ? { 'Content-Type': 'application/json' } : {}) }, body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await response.text(); let data; try { data = text ? JSON.parse(text) : {}; } catch { throw Error(`${method} ${path}: invalid response ${response.status}`); }
  if (expected ? response.status !== expected : !response.ok) throw Error(`${method} ${path}: ${response.status} ${data.error ?? ''} ${data.message ?? ''}`); return data;
}
async function until(label, fn, timeout = 60_000) { const end = Date.now() + timeout; let last; while (Date.now() < end) { if (spawnFailure) throw spawnFailure; if (fixtureError) throw fixtureError; if (child.exitCode !== null) throw Error(`Service exited ${child.exitCode}`); try { const v = await fn(); if (v) return v; } catch (e) { if (e.fatal) throw e; last = e; } await delay(250); } throw Error(`${label} timed out${last ? ': ' + last.message : ''}`); }
function containers(label, value) { return docker('ps', '-a', '--filter', `label=${label}=${value}`, '--format', '{{.ID}}').split(/\s+/).filter(Boolean); }
function owned(id, label, value) { const data = JSON.parse(docker('inspect', id))[0]; assert.equal(data.Config.Labels[label], value); if (label === 'luci.session-id') { assert(project?.path); assert(data.Mounts.some(m => m.Type === 'bind' && m.Destination === '/home/dev/work' && resolve(m.Source) === resolve(project.path)), 'Session container does not mount this exact fixture project'); } return data; }
async function attach(id, token, trustFixture = false) {
  const path = `${workspace}/v1/sessions/${id}/attach`; const { ticket } = await api('POST', '/v1/auth/socket-ticket', { path }, token);
  const ws = new WebSocket(origin.replace('http:', 'ws:') + path + `?ticket=${encodeURIComponent(ticket)}`); ws.binaryType = 'arraybuffer';
  const done = new Promise(finish => ws.addEventListener('close', finish, { once: true })); let terminal = '', trusted = false;
  ws.addEventListener('message', event => { const bytes = Buffer.from(event.data); if (bytes[0] === 0) { terminal = (terminal + bytes.subarray(1).toString()).slice(-128 * 1024); writeFileSync(join(fixture, 'terminal.txt'), terminal, { mode: 0o600 }); const plain = terminal.replace(/\x1b\[[0-9;?]*[A-Za-z]/g, ''); if (trustFixture && !trusted && plain.includes('/home/dev/work') && plain.includes('Trust and continue')) { trusted = true; console.log('Acknowledging the isolated fixture folder trust prompt'); setTimeout(() => { if (ws.readyState === WebSocket.OPEN) ws.send(Buffer.from([1, 13])); }, 300); } } });
  await new Promise((doneOpen, reject) => { const timer = setTimeout(() => reject(Error('Attach timed out')), 10_000); ws.addEventListener('open', () => { clearTimeout(timer); ws.send(Uint8Array.from([2, 0, 120, 0, 40])); doneOpen(); }, { once: true }); ws.addEventListener('error', () => { clearTimeout(timer); reject(Error('Attach failed')); }, { once: true }); }); return { ws, done };
}
try {
  await until('Service readiness', () => api('GET', '/v1/control-plane/status', undefined, undefined, undefined, 2000));
  owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken: readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(), username: 'teamrunowner', displayName: 'Team Run Owner', password: randomUUID() + randomUUID() });
  const password = randomUUID() + randomUUID(); const editorUser = await api('POST', '/v1/admin/users', { username: 'teamruneditor', displayName: 'Team Run Editor', password }); editor = await api('POST', '/v1/auth/login', { username: 'teamruneditor', password });
  team = await api('POST', '/v1/teams', { name: 'Reviewed run acceptance' }); const teamPath = `/v1/teams/${team.team.id}`; workspace = `${teamPath}/workspace`;
  const grants = { execute: true, secrets: true, deploy: true, admin: false };
  team = await api('POST', `${teamPath}/members`, { expectedRevision: team.team.revision, username: 'teamruneditor', role: 'editor', grants });
  await api('POST', `${workspace}/v1/setup/git/identity`, { name: 'Team Acceptance', email: 'team-acceptance@example.invalid' });
  const readiness = await api('POST', `${workspace}/v1/setup/check`, {}); for (const id of ['docker', 'sandbox-image']) assert.equal(readiness.checks.find(x => x.id === id)?.status, 'ready');
  project = await api('POST', `${workspace}/v1/product/projects`, { name: 'reviewed-team-run' }); assert(resolve(project.path).startsWith(join(fixture, 'state/control-plane/teams', team.team.id, 'runtime/projects') + sep)); projectRoute = `${workspace}/v1/product/projects/${project.id}`;
  team = await api('PUT', `${teamPath}/projects/${project.id}/members/${editorUser.id}`, { expectedRevision: team.team.revision, role: 'editor', grants }); checked('dedicated team worker and explicit project grant');
  await api('POST', `${projectRoute}/brief`, { revision: 0, purpose: 'Build a verified collaborative app', audience: 'Two collaborating reviewers', constraints: 'Only local fixture inference; preserve reviewed context', criteria: ['The application returns a healthy JSON response'] }, editor.token);
  let spec = await api('GET', `${projectRoute}/specification`); spec = await api('POST', `${projectRoute}/specification`, { expectedRevision: spec.revision, expectedBriefRevision: spec.briefRevision, specification: { ...spec, requirements: [{ id: 'AC-health', title: 'Healthy application', priority: 'must', acceptance: 'GET /health returns 200 with status ok and no model credential.' }] } }, editor.token);
  spec = await api('POST', `${projectRoute}/specification/approve`, { revision: spec.revision, briefRevision: spec.briefRevision });
  let design = await api('GET', `${projectRoute}/design-plan`); design = await api('POST', `${projectRoute}/design-plan`, { expectedRevision: design.revision, expectedBriefRevision: spec.briefRevision, expectedSpecificationRevision: spec.revision, plan: { ...design, screens: [{ id: 'health', name: 'Application health', route: '/', purpose: 'Show verified status', states: ['Ready', 'Error'], widths: [390, 1440], requirementIds: ['AC-health'] }] } });
  design = await api('POST', `${projectRoute}/design-plan/approve`, { revision: design.revision }, editor.token);
  const plan = await api('POST', `${projectRoute}/plan`, { expectedRevision: 0, expectedSpecificationRevision: spec.revision, tasks: [{ id: 'implement-health', title: 'Implement the reviewed healthy application', status: 'todo', requirementIds: ['AC-health'] }] }, editor.token);
  const history = (await api('GET', `${projectRoute}/context-history`)).events; assert(history.some(e => e.kind === 'specification' && e.action === 'approve' && e.actor.id === owner.user.id)); assert(history.some(e => e.kind === 'design' && e.action === 'approve' && e.actor.id === editorUser.id)); checked('reviewed definition and design attributed to two actual accounts', { ownerId: owner.user.id, editorId: editorUser.id, historyEntries: history.length });
  await api('POST', `${projectRoute}/git/commit`, { message: 'Review shared implementation inputs', expectedReviewDigest: (await api('GET', `${projectRoute}/git`)).reviewDigest });
  expectedContext = Object.fromEntries(['LUCI.md', '.luci/plan.json', '.luci/specification.json', '.luci/design-handoff.json', '.luci/design-handoff.md'].map(p => [p, sha(readFileSync(join(project.path, p)))]));
  const connections = await api('POST', `${workspace}/v1/setup/agent-connections`, { action: 'save', expectedRevision: 0, name: 'Local acceptance fixture', kind: 'compatible-api', enabled: true, models: { codex: 'fixture-model' }, endpoints: { responses: providerBase }, apiKey: key }); const connectionID = connections.connections[0].id; assert(!JSON.stringify(connections).includes(key));
  const branches = (await api('GET', `${workspace}/v1/projects/${project.id}/branches`, undefined, editor.token)).branches; const branch = branches.find(x => x.name === 'main') ?? branches[0]; const worktree = (await api('GET', `${workspace}/v1/projects/${project.id}/worktrees?branchId=${encodeURIComponent(branch.id)}`, undefined, editor.token)).worktrees[0];
  const selection = { projectId: project.id, briefRevision: spec.briefRevision, specificationRevision: spec.revision, designPlanRevision: design.revision, planRevision: plan.revision, taskIds: ['implement-health'] };
  let run = (await api('POST', `${projectRoute}/runs`, { idempotencyKey: randomUUID(), worktreeId: worktree.id, harness: 'codex', model: 'fixture-model', permissionMode: 'ask', agentConnectionId: connectionID, outcome: 'preview', implementationContext: selection }, editor.token)).run; const immutable = structuredClone(run.input);
  expectedPrompt = ['Implement the reviewed healthy application', 'AC-health', run.input.implementation.planDigest, run.input.implementation.specificationDigest, run.input.implementation.designPlanDigest];
  const reserved = await api('POST', `${projectRoute}/runs/${run.id}/operations`, { expectedRevision: run.revision, idempotencyKey: randomUUID(), kind: 'session' }, editor.token);
  sessionIDs.push(reserved.operation.childId); // Reservation predates allocation, including failed/ambiguous responses.
  const sessionRequest = { worktreeId: worktree.id, harness: 'codex', model: 'fixture-model', permissionMode: 'ask', agentConnectionId: connectionID, implementationContext: selection, projectRunId: run.id, projectRunOperationId: reserved.operation.id };
  session = await api('POST', `${workspace}/v1/sessions`, sessionRequest, editor.token, undefined, 90_000); sessionIDs.push(session.id); assert.equal(session.id, reserved.operation.childId); assert.equal(session.projectRunId, run.id);
  assert.equal((await api('POST', `${workspace}/v1/sessions`, sessionRequest, editor.token)).id, session.id); assert.equal(containers('luci.session-id', session.id).length, 1);
  const runtimeContainer = owned(containers('luci.session-id', session.id)[0], 'luci.session-id', session.id);
  const cliVersion = docker('exec', '--user', `${process.getuid()}:${process.getgid()}`, runtimeContainer.Id, 'codex', '--version'); assert.match(cliVersion, /0\.160\.0/); result.codexVersion = cliVersion;
  ({ ws: socket, done: closed } = await attach(session.id, editor.token, true));
  await until('Real CLI fixture implementation', () => providerFinished && existsSync(join(project.path, 'implementation-proof.json')), 90_000);
  const proof = JSON.parse(readFileSync(join(project.path, 'implementation-proof.json'), 'utf8')); assert.deepEqual(proof.files, expectedContext); assert.equal(proof.uid, process.getuid()); assert.equal(proof.cwd, '/home/dev/work'); assert.match(proof.status, /^CapEff:\s*0+$/m); assert.match(proof.status, /^NoNewPrivs:\s*1$/m); assert.equal(readFileSync(join(project.path, 'app.cjs'), 'utf8'), appSource);
  checked('real Codex receives immutable prompt and reads exact reviewed bytes in assigned nonroot worktree', { providerCalls: calls, sessionId: session.id, operationId: reserved.operation.id, contextSHA256: expectedContext });
  team = await api('GET', teamPath);
  await api('DELETE', `${teamPath}/members/${editorUser.id}`, { expectedRevision: team.team.revision });
  await Promise.race([closed, delay(5000, undefined, { ref: false }).then(() => { throw Error('Revocation did not close the active coding socket'); })]);
  await api('GET', `${projectRoute}/runs/${run.id}`, undefined, editor.token, 403);
  assert(owned(runtimeContainer.Id, 'luci.session-id', session.id).State.Running, 'Membership removal silently stopped shared coding work');
  checked('removing coding collaborator revokes socket and run access while preserving shared child');
  await api('DELETE', `${workspace}/v1/sessions/${session.id}`); await closed;
  await api('POST', `${projectRoute}/git/commit`, { message: 'Implement verified team application', expectedReviewDigest: (await api('GET', `${projectRoute}/git`, undefined, owner.token)).reviewDigest }, owner.token); const git = await api('GET', `${projectRoute}/git`); assert.equal(git.clean, true); const candidate = git.log[0].hash; assert.notEqual(candidate, immutable.implementation.sourceCommit);
  const check = await api('POST', `${projectRoute}/checks`, { execution: 'docker', acknowledgeHostExecution: true, commands: [{ label: 'Actual application HTTP acceptance', executable: 'node', args: ['acceptance.cjs'] }], requirementIds: ['AC-health'], expectedSpecificationRevision: spec.revision, expectedDesignPlanRevision: design.revision }, owner.token);
  const checkedRun = await until('Independent Docker checks', async () => { const row = await api('GET', `${projectRoute}/checks/${check.id}`); if (['failed', 'cancelled'].includes(row.status)) throw Object.assign(Error(`Independent check ${row.status}: ${JSON.stringify(row.commands)}`), { fatal: true }); return row.status === 'passed' && row; }, 120_000); assert.equal(checkedRun.commit, candidate); assert.equal(checkedRun.dirty, false); assert.equal(checkedRun.sourceChanged, false);
  run = (await api('GET', `${projectRoute}/runs/${run.id}`)).run; run = (await api('POST', `${projectRoute}/runs/${run.id}/reconcile`, { expectedRevision: run.revision }, owner.token)).run; run = (await api('POST', `${projectRoute}/runs/${run.id}/checks`, { expectedRevision: run.revision, checkId: check.id }, owner.token)).run; checked('independent Docker check joined exact clean candidate and full requirement coverage', { checkId: check.id, commit: candidate });
  release = await api('POST', `${projectRoute}/releases`, { target: 'local', port: 3000, dockerfile: 'Dockerfile' }, owner.token, undefined, 240_000); releaseIDs.push(release.id); assert.equal(release.status, 'ready'); assert.equal(release.commit, candidate);
  const container = owned(containers('luci.release-id', release.id)[0], 'luci.release-id', release.id); assert.equal(container.Config.Labels['luci.project-id'], project.id); const bindings = Object.values(container.NetworkSettings.Ports).flatMap(x => x ?? []); assert(bindings.every(x => x.HostIp === '127.0.0.1'));
  const http = await fetch(`http://127.0.0.1:${bindings[0].HostPort}/health`); assert.equal(http.status, 200); assert.deepEqual(await http.json(), { status: 'ok', createdBy: 'actual-team-codex-tool', modelKeyPresent: false });
  run = (await api('POST', `${projectRoute}/runs/${run.id}/releases`, { expectedRevision: run.revision, releaseId: release.id, target: 'local' }, owner.token)).run; run = (await api('POST', `${projectRoute}/runs/${run.id}/accept`, { expectedRevision: run.revision }, owner.token)).run; assert.equal(run.state, 'complete'); assert.deepEqual(run.input, immutable); assert.equal(run.acceptance.releaseId, release.id); checked('checked commit deployed healthy and explicitly accepted with immutable starting inputs', { runId: run.id, releaseId: release.id, commit: candidate });
  assert.equal((await api('POST', `${projectRoute}/releases/${release.id}/stop`)).status, 'stopped'); assert.equal(containers('luci.release-id', release.id).length, 0); checked('remaining owner finishes shared run and explicitly stops its release'); result.passed = true;
} catch (error) { result.failure = error.message; process.exitCode = 1; console.error(error.stack); }
finally {
  socket?.close();
  // Drain the owned worker before enumerating resources from an ambiguous response.
  if (child.exitCode === null && child.pid) { child.kill('SIGTERM'); await Promise.race([new Promise(done => child.once('close', done)), delay(20_000, undefined, { ref: false })]); if (child.exitCode === null) { process.kill(-child.pid, 'SIGKILL'); result.cleanupError = 'Service did not drain workers'; result.passed = false; process.exitCode = 1; } }
  // Discover only this fresh fixture's private recorded children after a lost response.
  if (team?.team?.id && project?.id) {
    try { const state = JSON.parse(readFileSync(join(fixture, 'state/control-plane/teams', team.team.id, 'runtime/state/sessions/sessions.json'), 'utf8')); for (const row of state.sessions ?? []) if (row.projectId === project.id && /^s-[a-f0-9]{8}$/.test(row.id)) sessionIDs.push(row.id); } catch (error) { if (error.code !== 'ENOENT') { result.cleanupError = 'Could not enumerate owned private session records'; result.passed = false; process.exitCode = 1; } }
  }
  if (team?.team?.id && project?.id) {
    try { const rows = JSON.parse(readFileSync(join(fixture, 'state/control-plane/teams', team.team.id, 'runtime/state/sessions/releases', project.id + '.json'), 'utf8')); for (const row of rows) { assert.equal(row.projectId, project.id); assert.match(row.id, /^rel-[a-f0-9]{24}$/); if (row.image) assert.equal(row.image, `luci/preview:${row.id}`); releaseIDs.push(row.id); } } catch (error) { if (error.code !== 'ENOENT') { result.cleanupError = 'Could not enumerate owned private release records'; result.passed = false; process.exitCode = 1; } }
  }
  for (const [label, ids] of [['luci.session-id', sessionIDs], ['luci.release-id', releaseIDs]]) for (const id of new Set(ids)) { try { for (const cid of containers(label, id)) { const record = owned(cid, label, id); if (label === 'luci.release-id') { assert.equal(record.Config.Labels['luci.project-id'], project.id); assert.equal(record.Config.Labels['luci.preview'], 'true'); } docker('rm', '-f', cid); } } catch (error) { result.cleanupError = error.message; process.exitCode = 1; result.passed = false; } }
  // Release tags are unique server-generated IDs recorded by this fixture.
  for (const id of new Set(releaseIDs)) { try { const tag = `luci/preview:${id}`; const rows = JSON.parse(docker('image', 'inspect', tag)); assert.equal(rows[0].Config.Labels['luci.preview'], 'true'); docker('image', 'rm', tag); } catch (error) { if (!String(error.stderr ?? '').includes('No such image')) { result.cleanupError = 'Could not remove owned preview image'; result.passed = false; process.exitCode = 1; } } }
  provider.closeAllConnections(); await new Promise(done => provider.close(done)); log.end(); result.finishedAt = new Date().toISOString();
  if (!result.passed || args.includes('--keep-fixture')) result.privateFixture = fixture; else rmSync(fixture, { recursive: true, force: true }); writeFileSync(report, JSON.stringify(result, null, 2) + '\n', { mode: 0o600 }); console.log(`${result.passed ? 'PASS' : 'FAIL'} ${report}`);
}

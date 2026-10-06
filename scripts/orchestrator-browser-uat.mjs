#!/usr/bin/env node
// Actual compiled Luci + browser + installed Codex CLI with local deterministic
// Responses fixture. Proves orchestration boundaries, not live model quality.
import assert from 'node:assert/strict';
import { spawn, execFileSync } from 'node:child_process';
import { createHash, randomUUID } from 'node:crypto';
import { createServer as httpServer } from 'node:http';
import { createServer } from 'node:net';
import { createWriteStream, existsSync, mkdirSync, mkdtempSync, readFileSync, writeFileSync, statfsSync } from 'node:fs';
import { dirname, join, resolve, sep } from 'node:path';
import { fileURLToPath } from 'node:url';
import { setTimeout as delay } from 'node:timers/promises';
import { chromium } from '../desktop/node_modules/playwright/index.mjs';
import axe from '../desktop/node_modules/axe-core/axe.js';
const repo = resolve(dirname(fileURLToPath(import.meta.url)), '..');
const args = process.argv.slice(2), option = (name, fallback) => { const i = args.indexOf(name); return i < 0 ? fallback : args[i + 1]; };
if (args.includes('--help')) { console.log('node scripts/orchestrator-browser-uat.mjs --binary PATH --web PATH --image EXISTING_IMAGE [--chromium PATH] [--report PATH]\nRuns isolated compiled/browser/Docker acceptance using only a local provider fixture. No image build/pull, live model credentials, cloud resources or public deployment.'); process.exit(0); }
const binary = resolve(option('--binary', 'artifacts/luci')), web = resolve(option('--web', 'desktop/dist')), image = option('--image', 'luci/sandbox:dev');
const report = resolve(option('--report', `artifacts/uat/orchestrator-${Date.now()}/result.json`));
assert(existsSync(binary)); assert(existsSync(join(web, 'index.html'))); assert.equal(process.platform, 'linux'); assert.notEqual(process.getuid(), 0);
mkdirSync(dirname(report), { recursive: true, mode: 0o700 });
const fixture = mkdtempSync('/tmp/luci-orchestrator-'), evidence = dirname(report);
for (const folder of ['home', 'projects', 'state', 'docker']) mkdirSync(join(fixture, folder), { mode: 0o700 });
const sha = bytes => createHash('sha256').update(bytes).digest('hex');
const docker = (...argv) => execFileSync('docker', argv, { encoding: 'utf8', timeout: 30_000 }).trim();
const imageInfo = JSON.parse(docker('image', 'inspect', image))[0];
const bridge = docker('network', 'inspect', 'bridge', '--format', '{{(index .IPAM.Config 0).Gateway}}'); assert.match(bridge, /^(10\.|172\.(1[6-9]|2\d|3[01])\.|192\.168\.)/);
const sourceRevision = execFileSync('git', ['rev-parse', 'HEAD'], { cwd: repo, encoding: 'utf8' }).trim();
const sourcePaths = execFileSync('git', ['ls-files', '-co', '--exclude-standard', '-z'], { cwd: repo, encoding: 'utf8' }).split('\0').filter(name => /^(control-plane\/|sandboxd\/|desktop\/src\/|desktop\/package(?:-lock)?\.json$|images\/|go\.work$)/.test(name));
const sourceManifest = [...new Set(sourcePaths)].sort().map(name => [name, sha(readFileSync(join(repo, name)))]);
writeFileSync(join(evidence, 'source-manifest.json'), JSON.stringify(sourceManifest, null, 2) + '\n', { mode: 0o600 });
const receipt = { passed: false, startedAt: new Date().toISOString(), source: { revision: sourceRevision, dirty: execFileSync('git', ['status', '--porcelain'], { cwd: repo, encoding: 'utf8' }).trim() !== '', capturedSourceManifestSHA256: sha(JSON.stringify(sourceManifest)), acceptanceDriverSHA256: sha(readFileSync(fileURLToPath(import.meta.url))) }, binarySHA256: sha(readFileSync(binary)), webIndexSHA256: sha(readFileSync(join(web, 'index.html'))), image: { reference: image, id: imageInfo.Id }, provider: 'Local deterministic Responses fixture through installed Codex CLI', externalModelCalls: 0, cloudResourcesCreated: 0, checks: [], privateFixture: fixture };
const check = (name, details = {}) => { receipt.checks.push({ name, passed: true, ...details }); console.log(`PASS ${name}`); };
const key = `fixture-only-${randomUUID()}`, lifecycle = randomUUID() + randomUUID(), password = randomUUID() + randomUUID();
const sensitive = [key, lifecycle, password], scrub = value => sensitive.reduce((text, secret) => text.replaceAll(secret, '[redacted]'), String(value));
let mode = 'question', expectedContext, expectedHistory = [], expectedReply = '', providerError, calls = 0, releaseProvider, providerWaiting = false;
const observedPrompts = [], responses = [], sessions = new Set(), errors = [], browserMutations = [];
const question = 'Should the shared journal be private to its invited team or publicly readable?';
const decision = 'Keep the journal private to explicitly invited collaborators.';
const amendedPurpose = 'A private shared journal for explicitly invited collaborators, with deliberate publication only after separate review.';
function stringsIn(value, output = []) { if (typeof value === 'string') output.push(value); else if (Array.isArray(value)) value.forEach(v => stringsIn(v, output)); else if (value && typeof value === 'object') Object.values(value).forEach(v => stringsIn(v, output)); return output; }
function promptFrom(body) {
  for (const text of stringsIn(body.input)) {
    const prefix = "You are Luci's project orchestrator";
    if (!text.includes(prefix)) continue;
    const start = text.indexOf('{', text.indexOf(prefix));
    if (start >= 0) { const result = JSON.parse(text.slice(start)); if (result.context && result.userMessage) return result; }
  }
  throw Error('Selected provider did not receive the orchestration prompt');
}
const provider = httpServer(async (req, res) => {
  try {
    assert.equal(req.method, 'POST'); assert.equal(req.url, '/v1/responses'); assert.equal(req.headers.authorization, `Bearer ${key}`);
    const parts = []; let size = 0; for await (const p of req) { parts.push(p); size += p.length; assert(size < 4 * 1024 * 1024); }
    const body = JSON.parse(Buffer.concat(parts).toString()), prompt = promptFrom(body); calls++;
    assert.equal(body.model, 'orchestrator-fixture');
    const tools = (body.tools ?? []).map(t => t.name ?? t.function?.name ?? t.type);
    for (const tool of ['exec_command', 'shell', 'apply_patch', 'web_search', 'web_search_preview']) assert(!tools.includes(tool), `Read-only assistant exposed tool ${tool}`);
    assert.deepEqual(prompt.context, expectedContext, 'Provider context differs from exact saved server snapshot');
    for (const turn of expectedHistory) { const prior = prompt.previousTurns.find(x => x.id === turn.id); assert(prior, `Missing prior turn ${turn.id}`); assert.equal(prior.message, turn.message); assert.deepEqual(prior.answer, turn.answer ? { ...turn.answer, proposal: null } : null); assert.equal(prior.replyToTurnId, turn.replyToTurnId ?? ''); }
    assert.equal(prompt.replyToTurnId, expectedReply); assert.equal(prompt.previousTurnsOmitted ?? 0, 0, 'Short conversation was unexpectedly omitted');
    observedPrompts.push(prompt); writeFileSync(join(fixture, `provider-prompt-${calls}.json`), JSON.stringify(prompt, null, 2), { mode: 0o600 });
    if (mode === 'failure') { res.writeHead(401, { 'content-type': 'application/json' }); res.end(JSON.stringify({ error: { type: 'authentication_error', code: 'invalid_api_key', message: 'Deliberate local fixture rejection' } })); return; }
    if (mode === 'question') { providerWaiting = true; await new Promise(done => { releaseProvider = done; }); providerWaiting = false; }
    const answer = { summary: mode === 'question' ? 'One consequential sharing decision is unresolved.' : 'The recorded decision is private collaboration. Review the proposed definition change before saving it.', question: mode === 'question' ? question : '', options: mode === 'question' ? [decision, 'Allow public reading of the journal.'] : [], rationale: mode === 'question' ? 'Visibility changes access controls, privacy and acceptance criteria.' : 'The proposed amendment reflects your explicit answer and preserves the saved requirements.', blocking: mode === 'question', impact: ['Access', 'Privacy', 'Acceptance'], proposal: null, nextAction: mode === 'question' ? 'Answer the sharing question.' : 'Review the definition amendment.' };
    if (mode !== 'question') answer.proposal = { target: 'specification', summary: 'Record private team visibility in the definition.', specification: { ...structuredClone(prompt.context.specification), status: 'draft', digest: '', purpose: amendedPurpose }, designPlan: null, plan: null };
    responses.push(answer);
    const item = { id: `msg_orchestrator_${calls}`, type: 'message', status: 'completed', role: 'assistant', content: [{ type: 'output_text', text: JSON.stringify(answer), annotations: [] }] };
    const response = { id: `resp_orchestrator_${calls}`, object: 'response', created_at: 1, status: 'completed', model: 'orchestrator-fixture', output: [item], usage: { input_tokens: 1, output_tokens: 1, total_tokens: 2, input_tokens_details: { cached_tokens: 0 }, output_tokens_details: { reasoning_tokens: 0 } } };
    const events = [{ type: 'response.created', response: { ...response, status: 'in_progress', output: [] } }, { type: 'response.output_item.added', output_index: 0, item: { ...item, status: 'in_progress', content: [] } }, { type: 'response.content_part.added', item_id: item.id, output_index: 0, content_index: 0, part: { type: 'output_text', text: '', annotations: [] } }, { type: 'response.output_text.delta', item_id: item.id, output_index: 0, content_index: 0, delta: item.content[0].text }, { type: 'response.output_text.done', item_id: item.id, output_index: 0, content_index: 0, text: item.content[0].text }, { type: 'response.output_item.done', output_index: 0, item }, { type: 'response.completed', response }];
    res.writeHead(200, { 'content-type': 'text/event-stream' }); res.end(events.map((e, i) => `event: ${e.type}\ndata: ${JSON.stringify({ ...e, sequence_number: i })}\n\n`).join(''));
  } catch (error) { providerError = error; res.writeHead(500); res.end('Local fixture contract failed'); }
});
await new Promise((done, reject) => provider.once('error', reject).listen(0, bridge, done));
const providerBase = `http://host.docker.internal:${provider.address().port}/v1`;
const listener = createServer(); await new Promise(done => listener.listen(0, '127.0.0.1', done)); const port = listener.address().port; await new Promise(done => listener.close(done));
const base = `http://127.0.0.1:${port}`;
writeFileSync(join(fixture, 'docker/config.json'), JSON.stringify({ cliPluginsExtraDirs: process.env.DOCKER_CONFIG ? [join(process.env.DOCKER_CONFIG, 'cli-plugins')] : [] }), { mode: 0o600 });
const env = Object.fromEntries(['PATH', 'LANG', 'LC_ALL', 'TZ', 'DOCKER_HOST'].filter(k => process.env[k]).map(k => [k, process.env[k]]));
Object.assign(env, { HOME: join(fixture, 'home'), DOCKER_CONFIG: join(fixture, 'docker'), LUCI_ADDR: `127.0.0.1:${port}`, LUCI_STATE_DIR: join(fixture, 'state'), LUCI_PROJECTS_DIR: join(fixture, 'projects'), LUCI_WEB_DIR: web, LUCI_RUNTIME_DIR: repo, LUCI_SANDBOX_IMAGE: image, LUCI_BACKEND: 'docker', LUCI_MANAGED_SHUTDOWN_TOKEN: lifecycle, LUCI_LOG_LEVEL: 'error', GIT_CONFIG_GLOBAL: join(fixture, 'home/empty-git-config'), GIT_CONFIG_NOSYSTEM: '1' });
const log = createWriteStream(join(fixture, 'service.log'), { mode: 0o600 });
let service, browser, page, panel, owner, project, projectRoute, spawnError;
async function api(method, path, body, status = 200) {
  const response = await fetch(base + path, { method, redirect: 'error', signal: AbortSignal.timeout(35_000), headers: { Origin: base, ...(owner?.token ? { Authorization: `Bearer ${owner.token}` } : {}), ...(body === undefined ? {} : { 'Content-Type': 'application/json' }) }, body: body === undefined ? undefined : JSON.stringify(body) });
  const text = await response.text(); let data; try { data = text ? JSON.parse(text) : {}; } catch { throw Error(`${method} ${path}: invalid response ${response.status}`); }
  assert.equal(response.status, status, `${method} ${path}: ${data.message ?? data.error ?? ''}`); return data;
}
async function until(label, fn, timeout = 120_000) {
  const end = Date.now() + timeout; let last;
  while (Date.now() < end) { if (spawnError) throw spawnError; if (providerError) throw providerError; if (service?.exitCode !== null) throw Error(`Service exited ${service?.exitCode}`); try { const result = await fn(); if (result) return result; } catch (error) { if (error.fatal) throw error; last = error; } await delay(200); }
  throw Error(`${label} timed out${last ? `: ${last.message}` : ''}`);
}
async function start() {
  service = spawn(binary, [], { cwd: fixture, env, detached: true, stdio: ['ignore', 'pipe', 'pipe'] }); service.once('error', error => { spawnError = error; });
  service.stdout.pipe(log, { end: false }); service.stderr.pipe(log, { end: false });
  const status = await until('Service readiness', () => api('GET', '/v1/control-plane/status'), 45_000); receipt.version = status.version;
}
async function stop() {
  if (!service || service.exitCode !== null) return;
  const response = await fetch(base + '/v1/control-plane/shutdown', { method: 'POST', headers: { 'X-Luci-Lifecycle-Token': lifecycle }, signal: AbortSignal.timeout(5000) }); assert.equal(response.status, 200);
  const end = Date.now() + 20_000; while (service.exitCode === null && Date.now() < end) await delay(100); assert.equal(service.exitCode, 0, 'Owned service did not stop cleanly');
}
const snapshot = () => api('GET', `${projectRoute}/orchestrator`);
async function noContextChange(before) {
  const after = await snapshot(); assert.deepEqual(after.context.specification, before.specification); assert.deepEqual(after.context.designPlan, before.designPlan); assert.deepEqual(after.context.plan, before.plan); return after;
}
async function openPanel() {
  panel = page.getByRole('complementary', { name: 'Project understanding', exact: true });
  const show = page.getByRole('button', { name: 'Show project overview', exact: true });
  if (await show.isVisible()) await show.click();
  if (!await panel.getByRole('textbox', { name: 'Message Luci', exact: true }).isVisible()) await panel.getByText('Discuss or request a change', { exact: true }).click();
  await panel.getByRole('textbox', { name: 'Message Luci', exact: true }).waitFor();
}
async function assertProjectUnderstanding(context) {
  const overview = panel.getByRole('region', { name: 'Current project understanding', exact: true });
  const idea = overview.getByRole('region', { name: 'What we’re building', exact: true });
  await idea.getByText(context.specification.purpose || context.brief.purpose, { exact: true }).waitFor();
  const outcomes = overview.getByRole('region', { name: 'Agreed outcomes', exact: true });
  for (const requirement of context.specification.requirements) { await outcomes.getByText(requirement.title, { exact: true }).waitFor(); await outcomes.getByText(requirement.acceptance, { exact: true }).waitFor(); }
  const view = await snapshot(), stage = { discover: 'Define', design: 'Design', develop: 'Develop', deploy: 'Deploy' }[view.nextAction.lens];
  await until('Project stage reflects the current saved context', async () => await overview.getByRole('navigation', { name: 'Project stages', exact: true }).getByRole('button', { name: `Open ${stage} stage`, exact: true }).getAttribute('aria-current') === 'step', 15000);
  await overview.getByRole('region', { name: 'Next step', exact: true }).getByRole('button', { name: view.nextAction.label, exact: true }).waitFor();
  if (context.currentRun) { const building = overview.getByRole('region', { name: 'In this build', exact: true }); await building.getByText('Implement reviewed observation access', { exact: true }).waitFor(); assert.equal(context.currentRun.id, view.context.currentRun.id); }
}
async function send(message, responseStatus = 202) {
  const disk = statfsSync('/workspace'); assert(disk.bavail * disk.bsize > 5_000_000_000, 'Docker vfs needs five GB of free workspace for one isolated assistant');
  const view = await snapshot(); expectedContext = structuredClone(view.context); expectedHistory = view.turns; expectedReply = view.blockingQuestions.at(-1)?.turnId ?? '';
  await panel.getByRole('textbox', { name: 'Message Luci', exact: true }).fill(message);
  const responding = page.waitForResponse(r => r.request().method() === 'POST' && r.url().endsWith('/assist/orchestrator'));
  await panel.getByRole('button', { name: 'Send to Luci', exact: true }).click();
  const response = await responding; assert.equal(response.status(), responseStatus, await response.text()); const job = await response.json(); if (job.sessionId) sessions.add(job.sessionId);
  if (responseStatus !== 202) return job;
  assert.equal(job.kind, 'orchestrator'); assert.equal(job.model, 'orchestrator-fixture'); assert.equal(job.agentConnectionId, receipt.agentConnectionId); assert.equal(job.agentConnectionProtocol, 'responses');
  if (mode === 'question') {
    await until('Selected provider receives saved context', () => providerWaiting, 90_000);
    const running = await api('GET', `${projectRoute}/assist/${job.id}`); assert.equal(running.status, 'running');
    const owned = docker('ps', '--filter', `label=luci.session-id=${job.sessionId}`, '--format', '{{.ID}}').split(/\s+/).filter(Boolean); assert.equal(owned.length, 1);
    const container = JSON.parse(docker('inspect', owned[0]))[0]; assert.equal(container.Config.Labels['luci.session-id'], job.sessionId); assert.equal(container.HostConfig.PidsLimit, 256);
    assert(container.Config.User && !['root', '0', '0:0'].includes(container.Config.User), 'Assistant process was configured as root');
    assert(container.Mounts.some(m => m.Destination === '/luci-input' && m.RW === false));
    assert(!container.Mounts.some(m => resolve(m.Source) === resolve(project.path) || resolve(m.Source).startsWith(resolve(project.path) + sep)), 'Read-only orchestration mounted editable project source');
    await assertProjectUnderstanding(expectedContext);
    check('Project stage, saved outcomes and selected build scope stay visible while the actual CLI waits for its provider response');
    releaseProvider(); releaseProvider = undefined;
  }
  const finished = await until('Orchestrator provider completion', async () => { const value = await api('GET', `${projectRoute}/assist/${job.id}`); if (value.sessionId) sessions.add(value.sessionId); return ['succeeded', 'failed', 'cancelled'].includes(value.status) && value; }, 180_000);
  assert.equal(finished.status, mode === 'failure' ? 'failed' : 'succeeded', finished.message);
  await panel.getByRole('button', { name: 'Refresh conversation', exact: true }).click();
  const after = await until('Persisted conversation completion', async () => { const value = await snapshot(); return value.turns.some(t => t.jobId === job.id && t.status === finished.status) && value; });
  assert.equal(docker('ps', '-a', '--filter', `label=luci.session-id=${job.sessionId}`, '--format', '{{.ID}}'), '', 'Assistant left its container behind');
  return { job: finished, view: after, turn: after.turns.find(t => t.jobId === job.id) };
}
try {
  await start(); const bootstrapToken = readFileSync(join(fixture, 'state/control-plane/bootstrap-token'), 'utf8').trim(); sensitive.push(bootstrapToken);
  owner = await api('POST', '/v1/auth/bootstrap', { bootstrapToken, username: 'orchestrator-owner', displayName: 'Orchestrator Reviewer', password }, 201); sensitive.push(owner.token);
  await api('POST', '/v1/setup/git/identity', { name: 'Orchestrator Acceptance', email: 'orchestrator@example.invalid' });
  project = await api('POST', '/v1/product/projects', { name: 'continuous-project-context' }, 201); projectRoute = `/v1/product/projects/${project.id}`;
  const otherProject = await api('POST', '/v1/product/projects', { name: 'separate-project-context' }, 201);
  await api('POST', `${projectRoute}/brief`, { revision: 0, purpose: 'A shared garden journal with reviewed access decisions.', audience: 'Invited gardeners and reviewers.', constraints: 'Use the selected fixture only; never infer permission to publish.', criteria: ['Authorized gardeners can save a private observation.'] });
  let spec = await api('GET', `${projectRoute}/specification`);
  spec = await api('POST', `${projectRoute}/specification`, { expectedRevision: spec.revision, expectedBriefRevision: spec.briefRevision, specification: { ...spec, requirements: [{ id: 'AC-private-observation', title: 'Save a garden observation', priority: 'must', acceptance: 'An authorized gardener can save and retrieve a private observation.' }] } });
  spec = await api('POST', `${projectRoute}/specification/approve`, { revision: spec.revision, briefRevision: spec.briefRevision });
  let design = await api('GET', `${projectRoute}/design-plan`);
  design = await api('POST', `${projectRoute}/design-plan`, { expectedRevision: design.revision, expectedBriefRevision: spec.briefRevision, expectedSpecificationRevision: spec.revision, plan: { ...design, screens: [{ id: 'observations', name: 'Private observations', route: '/', purpose: 'Create and review authorized observations.', states: ['Empty', 'Saved', 'Access denied'], widths: [390, 1440], requirementIds: ['AC-private-observation'] }], implementationNotes: 'Retain the reviewed privacy boundary and explicit authorization.' } });
  design = await api('POST', `${projectRoute}/design-plan/approve`, { revision: design.revision });
  const plan = await api('POST', `${projectRoute}/plan`, { expectedRevision: 0, expectedSpecificationRevision: spec.revision, tasks: [{ id: 'private-observations', title: 'Implement reviewed observation access', status: 'todo', requirementIds: ['AC-private-observation'] }] });
  await api('POST', `${projectRoute}/git/commit`, { message: 'Review complete orchestration context' });
  const initial = await snapshot(); assert.equal(initial.context.specification.status, 'approved'); assert.equal(initial.context.designPlan.status, 'approved'); assert.deepEqual(initial.context.plan, plan);
  // Missing access is a failed durable attempt, never a synthesized answer.
  const missingConnection = await api('POST', `${projectRoute}/assist/orchestrator`, { expectedRevision: initial.revision, expectedContextDigest: initial.context.digest, idempotencyKey: randomUUID(), message: 'Please inspect the saved plan without a connected provider.', harness: 'codex', model: 'orchestrator-fixture', authMode: 'api', agentConnectionId: '' }, 409);
  assert.equal(missingConnection.error, 'assistant_connection_required', `Missing connection returned the wrong admission failure: ${missingConnection.message}`);
  const missing = await until('Missing connection failure recorded', async () => { const value = await snapshot(); return value.turns.some(t => t.status === 'failed') && value; }); assert.equal(calls, 0); assert.equal(missing.turns.at(-1).answer, undefined); await noContextChange(initial.context);
  browser = await chromium.launch({ headless: true, ...(option('--chromium') ? { executablePath: option('--chromium') } : {}) }); receipt.browserVersion = await browser.version();
  page = await browser.newPage({ viewport: { width: 1440, height: 1000 } }); page.setDefaultTimeout(25000);
  page.on('pageerror', error => errors.push(scrub(error.message)));
  page.on('request', r => { if (['POST', 'PUT', 'PATCH', 'DELETE'].includes(r.method()) && new URL(r.url()).origin === base) browserMutations.push({ method: r.method(), path: new URL(r.url()).pathname }); });
  await page.goto(base); await page.getByLabel('Username', { exact: true }).fill('orchestrator-owner'); await page.getByLabel('Password', { exact: true }).fill(password); await page.getByRole('button', { name: 'Sign in', exact: true }).click();
  await page.getByRole('treeitem', { name: /continuous-project-context$/ }).click(); await openPanel();
  await panel.getByText(/failed|could not start/i).first().waitFor();
  check('Missing selected provider creates a truthful failed private turn without inference, synthetic answer or context mutation');
  const catalogue = await api('POST', '/v1/setup/agent-connections', { action: 'save', expectedRevision: 0, name: 'Local orchestrator acceptance fixture', kind: 'compatible-api', enabled: true, models: { codex: 'orchestrator-fixture' }, endpoints: { responses: providerBase }, apiKey: key });
  const connectionID = catalogue.connections[0].id; receipt.agentConnectionId = connectionID; assert(!JSON.stringify(catalogue).includes(key));
  const branches = (await api('GET', `/v1/projects/${project.id}/branches`)).branches;
  const branch = branches.find(row => row.name === 'main') ?? branches[0];
  const worktree = (await api('GET', `/v1/projects/${project.id}/worktrees?branchId=${encodeURIComponent(branch.id)}`)).worktrees[0];
  const build = (await api('POST', `${projectRoute}/runs`, { idempotencyKey: randomUUID(), worktreeId: worktree.id, harness: 'codex', model: 'orchestrator-fixture', permissionMode: 'ask', agentConnectionId: connectionID, outcome: 'preview', implementationContext: { projectId: project.id, briefRevision: spec.briefRevision, specificationRevision: spec.revision, designPlanRevision: design.revision, planRevision: plan.revision, taskIds: ['private-observations'] } }, 201)).run;
  assert.deepEqual(build.operations, [], 'Preparing run unexpectedly started coding'); assert.equal(build.state, 'ready');

  await page.reload(); await openPanel(); await panel.getByText('Agent and model', { exact: true }).click();
  await panel.getByLabel('Luci agent', { exact: true }).selectOption('codex'); await panel.getByLabel('Luci connection', { exact: true }).selectOption(connectionID); assert.equal(await panel.getByLabel('Luci model', { exact: true }).inputValue(), 'orchestrator-fixture');
  const asked = await send('Inspect the complete saved project context and ask the material visibility question before recommending implementation.');
  assert(asked.turn.answer.blocking); assert.equal(asked.view.blockingQuestions.length, 1); assert.equal(asked.view.nextAction.id, 'answer-question'); await assertProjectUnderstanding(asked.view.context); await panel.getByText(question, { exact: true }).first().waitFor(); await noContextChange(initial.context);
  check('Real selected Codex CLI receives exact saved definition/design/plan and records its material question without changing documents', { turnId: asked.turn.id, contextDigest: asked.turn.contextDigest, providerCalls: calls });
  await api('POST', `${projectRoute}/runs/${build.id}/operations`, { expectedRevision: build.revision, idempotencyKey: randomUUID(), kind: 'session' }, 409);
  assert.equal((await api('GET', `${projectRoute}/runs/${build.id}`)).run.operations.length, 0, 'A blocking decision allocated coding work');
  const beforeNavigation = calls;
  for (const lens of ['Design', 'Develop', 'Deploy', 'Define']) { await panel.getByRole('navigation', { name: 'Project stages', exact: true }).getByRole('button', { name: `Open ${lens} stage`, exact: true }).click(); await openPanel(); await panel.getByText(question, { exact: true }).first().waitFor(); }
  assert.equal(calls, beforeNavigation);
  await stop(); await start(); await page.reload(); await openPanel(); await panel.getByText(question, { exact: true }).first().waitFor(); assert.equal(calls, beforeNavigation);
  const recovered = await snapshot(); assert.equal(recovered.blockingQuestions[0].turnId, asked.turn.id); assert.equal(recovered.turns.find(t => t.id === asked.turn.id).jobId, asked.job.id); await noContextChange(initial.context);
  check('The same private question and conversation recover across all four phases, reload and process restart without redispatch');
  await page.screenshot({ path: join(evidence, '01-persistent-question.png'), fullPage: true });
  await panel.getByRole('button', { name: 'Change the plan', exact: true }).click(); assert.equal(calls, beforeNavigation);
  await panel.getByRole('region', { name: 'Next step', exact: true }).getByRole('button', { name: recovered.nextAction.label, exact: true }).click();
  assert.equal(calls, beforeNavigation, 'Primary question action dispatched before an explicit send');
  // Type the answer directly: an offered-option button also binds the reply
  // and would hide a broken primary Next step action.
  await panel.getByRole('textbox', { name: 'Message Luci', exact: true }).fill(decision);
  assert.equal(calls, beforeNavigation);
  mode = 'proposal'; const proposed = await send(decision); assert.equal(proposed.turn.replyToTurnId, asked.turn.id); assert.equal(proposed.view.blockingQuestions.length, 0); await noContextChange(initial.context);
  const prompt = observedPrompts.at(-1); assert.equal(prompt.userMessage, decision); assert.equal(prompt.previousTurns.find(t => t.id === asked.turn.id).answer.question, question);
  check('The primary next-step question action binds the explicit human reply, which reaches the selected provider with the prior question and complete unchanged saved context; proposal remains unapplied');
  await panel.getByRole('button', { name: 'Review amendment', exact: true }).last().click();
  let dialog = page.getByRole('dialog', { name: 'Review definition amendment', exact: true }); await dialog.waitFor();
  assert.equal(await dialog.getByRole('button', { name: 'Apply reviewed amendment', exact: true }).isEnabled(), false);
  await dialog.getByRole('heading', { name: 'Saved when asked', exact: true }).waitFor(); await dialog.getByRole('heading', { name: 'Proposed draft', exact: true }).waitFor();
  await dialog.getByText(spec.purpose, { exact: true }).waitFor(); await dialog.getByText(amendedPurpose, { exact: true }).waitFor(); await dialog.getByText('Affected work', { exact: true }).waitFor();
  await noContextChange(initial.context);
  // A real concurrent write after review opened must invalidate the proposal.
  let concurrent;
  // Hold only request timing. Both the concurrent write and the original,
  // unmodified reviewed request reach the real compiled server; no response is mocked.
  const applyPattern = `**/orchestrator/turns/${proposed.turn.id}/apply`;
  await page.route(applyPattern, async route => {
    concurrent = await api('POST', `${projectRoute}/specification`, { expectedRevision: spec.revision, expectedBriefRevision: spec.briefRevision, specification: { ...spec, constraints: `${spec.constraints} Concurrent reviewer retained a new accessibility constraint.` } });
    await route.continue();
  });
  await dialog.getByRole('checkbox', { name: 'I reviewed these changes. Save them as a draft and clear any affected approvals.', exact: true }).check();
  const conflicting = page.waitForResponse(r => r.request().method() === 'POST' && r.url().endsWith(`/orchestrator/turns/${proposed.turn.id}/apply`));
  await dialog.getByRole('button', { name: 'Apply reviewed amendment', exact: true }).click(); assert.equal((await conflicting).status(), 409); await page.unroute(applyPattern); assert.deepEqual(await api('GET', `${projectRoute}/specification`), concurrent);
  await page.getByRole('alert').filter({ hasText: /changed|reload|context/i }).first().waitFor();
  await dialog.getByRole('button', { name: 'Keep current context', exact: true }).click();
  check('Opening a proposal does not save it; real concurrent context change rejects reviewed apply with409 and preserves the newer revision');
  await panel.getByRole('button', { name: 'Refresh conversation', exact: true }).click();
  const rebased = await send('Keep my recorded private-collaboration decision and propose it against the current concurrent revision without discarding its new constraint.');
  assert(observedPrompts.at(-1).previousTurns.some(t => t.replyToTurnId === asked.turn.id && t.message === decision));
  assert.equal(rebased.turn.answer.proposal.specification.constraints, concurrent.constraints);
  const beforeApply = await api('GET', `${projectRoute}/specification`); assert.equal(beforeApply.purpose, concurrent.purpose);
  await panel.getByRole('button', { name: 'Review amendment', exact: true }).last().click(); dialog = page.getByRole('dialog', { name: 'Review definition amendment', exact: true });
  await dialog.getByRole('checkbox', { name: 'I reviewed these changes. Save them as a draft and clear any affected approvals.', exact: true }).check();
  const applying = page.waitForResponse(r => r.request().method() === 'POST' && r.url().endsWith(`/orchestrator/turns/${rebased.turn.id}/apply`));
  await dialog.getByRole('button', { name: 'Apply reviewed amendment', exact: true }).click(); assert.equal((await applying).status(), 200);
  const applied = await api('GET', `${projectRoute}/specification`); assert.equal(applied.revision, beforeApply.revision + 1); assert.equal(applied.purpose, amendedPurpose); assert.equal(applied.constraints, concurrent.constraints); assert.equal(applied.status, 'draft');
  assert.notEqual((await api('GET', `${projectRoute}/design-plan`)).status, 'approved');
  const afterApply = await snapshot(); assert.equal(afterApply.nextAction.id, 'review-definition'); await assertProjectUnderstanding(afterApply.context); await panel.getByText(new RegExp(`Draft definition.*Revision ${applied.revision}`)).waitFor(); assert.equal(afterApply.context.currentRun.specificationRevision, spec.revision); assert.equal(afterApply.turns.find(t => t.id === rebased.turn.id).appliedRevision, applied.revision);
  check('A freshly reviewed amendment applies once by CAS, preserves the concurrent constraint and clears affected approvals without implementation', { appliedRevision: applied.revision });
  await page.screenshot({ path: join(evidence, '02-reviewed-amendment.png'), fullPage: true });
  await panel.getByText('Discuss or request a change', { exact: true }).click();
  receipt.accessibility = [];
  for (const [surface, viewport] of [['wide', { width: 1440, height: 1000 }], ['mobile', { width: 390, height: 844 }]]) {
    await page.setViewportSize(viewport);
    await panel.locator('.orchestrator-body').evaluate(element => { element.scrollTop = 0; });
    await panel.getByRole('region', { name: 'Current project understanding', exact: true }).waitFor();
    assert(await page.evaluate(() => document.documentElement.scrollWidth <= window.innerWidth), `${surface} layout has horizontal document overflow`);
    await page.screenshot({ path: join(evidence, `03-overview-${surface}.png`), fullPage: true });
    await page.evaluate(axe.source);
    const audit = await page.evaluate(async () => { const result = await window.axe.run('.orchestrator', { runOnly: { type: 'tag', values: ['wcag2a', 'wcag2aa', 'wcag21aa', 'wcag22aa', 'best-practice'] } }); return { violations: result.violations, incomplete: result.incomplete, passes: result.passes.length }; });
    receipt.accessibility.push({ surface, ...audit });
    writeFileSync(join(evidence, 'overview-accessibility.json'), JSON.stringify(receipt.accessibility, null, 2) + '\n', { mode: 0o600 });
    assert.deepEqual(audit.violations, [], `${surface} project overview has automated accessibility violations`);
  }
  await page.setViewportSize({ width: 1440, height: 1000 });
  check('Wide and mobile project overview render without horizontal document overflow and pass scoped automated WCAG A/AA checks', { limitation: 'Automated checks are not accessibility certification; incomplete checks remain in the evidence.' });

  await page.getByRole('treeitem', { name: /separate-project-context$/ }).click(); await openPanel(); assert.equal(await panel.getByText(question, { exact: true }).count(), 0); assert.equal((await api('GET', `/v1/product/projects/${otherProject.id}/orchestrator`)).turns.length, 0);
  await page.getByRole('treeitem', { name: /continuous-project-context$/ }).click(); await openPanel();
  mode = 'failure'; const beforeFailure = await snapshot(); const failureMessage = 'Verify that a rejected provider request remains an explicit failure, without substituting another model.'; const failed = await send(failureMessage); assert.equal(failed.turn.answer, undefined); await noContextChange(beforeFailure.context);
  await panel.getByText(failureMessage, { exact: true }).locator('..').getByText('Request failed', { exact: true }).waitFor();
  const failedCalls = calls; await page.reload(); await openPanel(); assert.equal(calls, failedCalls); await noContextChange(beforeFailure.context);
  check('Project conversations stay isolated; a real provider401 remains failed after reload with no fallback, fabricated answer or document mutation');
  assert.deepEqual(browserMutations, [
    { method: 'POST', path: '/v1/auth/login' },
    ...Array.from({ length: 2 }, () => ({ method: 'POST', path: `${projectRoute}/assist/orchestrator` })),
    { method: 'POST', path: `${projectRoute}/orchestrator/turns/${proposed.turn.id}/apply` },
    { method: 'POST', path: `${projectRoute}/assist/orchestrator` },
    { method: 'POST', path: `${projectRoute}/orchestrator/turns/${rebased.turn.id}/apply` },
    { method: 'POST', path: `${projectRoute}/assist/orchestrator` },
  ], 'Browser must send exactly the explicit login, four messages and two reviewed applies in their expected order');
  const allSessions = await api('GET', '/v1/sessions'); const rows = Array.isArray(allSessions) ? allSessions : allSessions.sessions;
  assert(rows.every(row => row.projectId !== project.id || row.contextRef === 'luci:orchestrator'), 'Conversation started implementation work');
  assert.deepEqual(errors, []); receipt.providerCalls = calls; receipt.promptSHA256 = observedPrompts.map(value => sha(JSON.stringify(value))); receipt.browserMutations = browserMutations;
  check('Every browser mutation was explicit conversation send or reviewed apply; no approval, coding, check, deployment or cloud request was triggered'); receipt.passed = true;
} catch (error) {
  receipt.failure = scrub(error.stack ?? error); process.exitCode = 1; console.error(receipt.failure);
  if (page) { await page.screenshot({ path: join(evidence, 'failure.png'), fullPage: true }).catch(() => {}); writeFileSync(join(evidence, 'failure-dom.txt'), scrub(await page.locator('body').innerText().catch(() => 'unavailable')), { mode: 0o600 }); }
 } finally {
  const cleanupFailure = (step, error) => { (receipt.cleanupErrors ??= []).push({ step, error: scrub(error) }); receipt.passed = false; process.exitCode = 1; };
  try { releaseProvider?.(); releaseProvider = undefined; } catch (error) { cleanupFailure('release local provider', error); }
  try { await browser?.close(); } catch (error) { cleanupFailure('close browser', error); }
  try { await stop(); } catch (error) {
    cleanupFailure('stop service', error);
    try { if (service?.pid && service.exitCode === null) process.kill(-service.pid, 'SIGKILL'); } catch (killError) { if (killError.code !== 'ESRCH') cleanupFailure('terminate owned service', killError); }
  }
  // Discover only this isolated instance's durable utility sessions after an
  // interrupted HTTP response, then require exact owned input mounts to remove.
  try { const registry = JSON.parse(readFileSync(join(fixture, 'state/sessions/sessions.json'), 'utf8')); for (const row of registry.sessions ?? []) if (row.projectId === project?.id && row.contextRef === 'luci:orchestrator') sessions.add(row.id); } catch (error) { if (error.code !== 'ENOENT') cleanupFailure('enumerate private fixture sessions', error); }
  for (const id of sessions) {
    try { assert.match(id, /^s-[a-f0-9]{8}$/); for (const cid of docker('ps', '-a', '--filter', `label=luci.session-id=${id}`, '--format', '{{.ID}}').split(/\s+/).filter(Boolean)) { const c = JSON.parse(docker('inspect', cid))[0]; assert.equal(c.Config.Labels['luci.session-id'], id); assert(c.Mounts.some(m => m.Destination === '/luci-input' && resolve(m.Source).startsWith(fixture + sep))); docker('rm', '-f', cid); } }
    catch (error) { cleanupFailure(`remove owned session ${id}`, error); }
  }
  try { provider.closeAllConnections(); await new Promise((done, reject) => provider.close(error => error ? reject(error) : done())); } catch (error) { cleanupFailure('close local provider', error); }
  try { log.end(); } catch (error) { cleanupFailure('close service log', error); }
  receipt.finishedAt = new Date().toISOString(); writeFileSync(report, JSON.stringify(receipt, null, 2) + '\n', { mode: 0o600 }); console.log(`${receipt.passed ? 'PASS' : 'FAIL'} ${report}`);
}

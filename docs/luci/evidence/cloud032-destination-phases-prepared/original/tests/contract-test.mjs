// Offline contracts only: no browser, service, build, product mutation or benchmark.
import assert from 'node:assert/strict';
import vm from 'node:vm';
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import { createRequire } from 'node:module';
import { fileURLToPath } from 'node:url';
import { resolve } from 'node:path';

const root = fileURLToPath(new URL('../', import.meta.url));
const require = createRequire('/workspace/luci/desktop/package.json');
const ts = require('typescript');
assert.equal(ts.version, '6.0.3', 'Use the existing pinned TypeScript compiler');
const sourcePath = resolve(root, 'overlay/desktop/src/diagnostics/destinationPhaseTiming.ts');
const source = readFileSync(sourcePath, 'utf8');
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.CommonJS, target: ts.ScriptTarget.ES2020 },
  reportDiagnostics: true,
});
assert.equal(compiled.diagnostics.length, 0, 'Probe must transpile without diagnostics');
const { destinationValueWitness, validateDestinationCapture } = await import('../actual/destination-diagnostics.mjs');
const { interactionRunner } = await import('../actual/interaction-metrics.mjs');
const { browserLatencyInit, browserFrameWitness, nodeStageRecorder } = await import('../actual/latency-diagnostics.mjs');
const hash = bytes => createHash('sha256').update(bytes).digest('hex');
const clone = value => JSON.parse(JSON.stringify(value));
const tests = [];
const test = (name, fn) => tests.push({ name, fn });
const PRIVATE_PROJECT = 'contract-private-project-271828';
const PRIVATE_VALUE = 'contract-private-purpose-314159';

function fresh({ enabled = true, armed = true, noWindow = false } = {}) {
  let clock = 0;
  const frames = [];
  const context = {
    exports: {},
    performance: { timeOrigin: 123456, now: () => ++clock / 10 },
    requestAnimationFrame: callback => { frames.push(callback); return frames.length; },
    document: { visibilityState: 'visible', activeElement: null },
    location: { href: 'http://diagnostic.invalid/', origin: 'http://diagnostic.invalid' },
    URL,
    PerformanceObserver: class { static supportedEntryTypes = []; },
  };
  if (!noWindow) context.window = {
    ...(enabled ? { __luciDestinationPhaseRequested: 1 } : {}),
    addEventListener() {},
  };
  vm.createContext(context);
  vm.runInContext(compiled.outputText, context, { timeout: 1000 });
  const probe = context.window?.__luciDestinationPhase;
  if (probe && armed) probe.arm();
  const element = (value = PRIVATE_VALUE) => ({ tagName: 'TEXTAREA', isConnected: true, value });
  return {
    context, exports: context.exports, probe, element, frames,
    advance(milliseconds) { clock += milliseconds * 10; },
    flushFrame(before = () => {}) {
      assert(frames.length, 'Expected a scheduled animation frame');
      before(); clock += 160;
      frames.shift()(clock / 10);
    },
    invoke(fn, ...args) {
      context.__args = args;
      return vm.runInContext(`(${fn.toString()})(...__args)`, context, { timeout: 1000 });
    },
  };
}

function begin(c, { project = PRIVATE_PROJECT, generation = 1 } = {}) {
  c.exports.destinationNavigate(null, 'discover');
  c.exports.destinationNavigate(project, 'discover');
  const token = c.exports.destinationReadBegin(project, generation);
  const spec = { projectId: project, purpose: PRIVATE_VALUE, revision: 7, briefRevision: 11 };
  const element = c.element();
  return { project, generation, token, spec, element };
}
function accepted(c, state) { c.exports.destinationReadAccepted(state.token, state.spec); }
function commit(c, state, overrides = {}) {
  const s = { ...state, ...overrides };
  c.exports.destinationLayoutCommit(s.token, s.project, s.generation, s.spec, s.element);
}
function evaluate(c, state, ordinal = 1) {
  return c.probe.evaluationEnter(ordinal, state.project, PRIVATE_VALUE, state.element);
}
function witness(c, state, ordinal = 1) {
  const evaluation = evaluate(c, state, ordinal);
  c.probe.evaluationSample(evaluation, state.element, true);
  c.probe.evaluationWitness(evaluation, state.element);
}
function complete(c, ordinal = 1) {
  const state = begin(c, { generation: ordinal });
  accepted(c, state); commit(c, state); witness(c, state, ordinal);
  return state;
}
function receiptFor(snapshot, actions = 1) {
  const stages = ['verify-start', 'waitFor-start', 'waitFor-end', 'evaluate-start', 'evaluate-end',
    'inputValue-start', 'inputValue-end', 'verify-end'];
  return {
    browserEventWindows: [{ destinationPhase: clone(snapshot) }],
    interactions: Array.from({ length: actions }, () => ({ kind: 'project-switch', success: true,
      diagnostics: { nodeStages: stages.map((stage, index) => ({ stage, offsetMs: index })) } })),
  };
}
function assertClean(snapshot) {
  for (const key of ['dropped', 'errors', 'overlaps', 'mismatches', 'active', 'pendingReads', 'pendingEvaluations']) {
    assert.equal(snapshot[key], 0, key);
  }
}

test('disabled and server contexts leave hooks inert', () => {
  for (const c of [fresh({ enabled: false }), fresh({ noWindow: true })]) {
    assert.equal(c.probe, undefined);
    const state = begin(c);
    assert.equal(state.token, null);
    assert.doesNotThrow(() => { accepted(c, state); commit(c, state); });
  }
});

test('enabled but unarmed hooks retain no observation', () => {
  const c = fresh({ armed: false }), state = begin(c);
  accepted(c, state); commit(c, state);
  assert.equal(state.token, null);
  const snapshot = c.probe.snapshot();
  assert.equal(snapshot.armed, 0); assert.equal(snapshot.observed, 0);
  assert.equal(snapshot.actions, 0); assert.equal(snapshot.rows.length, 0);
});

test('matching target, accepted object, revision and textarea produce one complete action', () => {
  const c = fresh(), state = complete(c), snapshot = c.probe.snapshot();
  assert.deepEqual(Object.keys(state.token), [], 'Opaque read token carries no private data');
  assertClean(snapshot);
  assert.equal(snapshot.actions, 1); assert.equal(snapshot.reads, 1);
  assert.equal(snapshot.accepted, 1); assert.equal(snapshot.commits, 1);
  assert.equal(snapshot.evaluations, 1); assert.equal(snapshot.samples, 1); assert.equal(snapshot.witnesses, 1);
  assert.deepEqual(clone(snapshot.rows.map(row => row.kind)), [1, 2, 3, 4, 5, 6, 7, 8]);
  assert.doesNotThrow(() => validateDestinationCapture(receiptFor(snapshot), 1));
});

test('entry and negative first sample can precede acceptance and first commit', () => {
  const c = fresh(), state = begin(c);
  state.element.value = 'stale-local-value';
  const evaluation = evaluate(c, state);
  assert(evaluation, 'Entry must not depend on future product readiness');
  c.probe.evaluationSample(evaluation, state.element, false);
  accepted(c, state); state.element.value = PRIVATE_VALUE; commit(c, state);
  c.probe.evaluationSample(evaluation, state.element, true);
  c.probe.evaluationWitness(evaluation, state.element);
  const snapshot = c.probe.snapshot(); assertClean(snapshot);
  assert.equal(snapshot.samples, 2);
  const sample = snapshot.rows.find(row => row.kind === 7);
  assert.equal(sample.match, 0);
  assert(snapshot.rows.find(row => row.kind === 6).sequence < snapshot.rows.find(row => row.kind === 5).sequence);
  assert.doesNotThrow(() => validateDestinationCapture(receiptFor(snapshot), 1));
});

test('evaluation can begin before the read and joins the eventual accepted response', () => {
  const c = fresh();
  c.exports.destinationNavigate(null, 'discover');
  c.exports.destinationNavigate(PRIVATE_PROJECT, 'discover');
  const element = c.element('not-ready');
  const evaluation = c.probe.evaluationEnter(1, PRIVATE_PROJECT, PRIVATE_VALUE, element);
  c.probe.evaluationSample(evaluation, element, false);
  const state = { project: PRIVATE_PROJECT, generation: 1,
    token: c.exports.destinationReadBegin(PRIVATE_PROJECT, 1),
    spec: { projectId: PRIVATE_PROJECT, purpose: PRIVATE_VALUE, revision: 7, briefRevision: 11 }, element };
  accepted(c, state); element.value = PRIVATE_VALUE; commit(c, state);
  c.probe.evaluationSample(evaluation, element, true); c.probe.evaluationWitness(evaluation, element);
  const snapshot = c.probe.snapshot(); assertClean(snapshot);
  assert.equal(snapshot.rows.find(row => row.kind === 6).read, 0);
  assert.equal(snapshot.rows.find(row => row.kind === 7).read, 0);
  assert.equal(snapshot.rows.find(row => row.kind === 8).read, 1);
  assert.doesNotThrow(() => validateDestinationCapture(receiptFor(snapshot), 1));
});

test('first valid commit is preserved and post-witness reset reads are ignored', () => {
  const c = fresh(), state = begin(c); accepted(c, state); commit(c, state);
  commit(c, state); commit(c, state);
  witness(c, state);
  // Lens-only switches do not invoke the project navigation hook.
  assert.equal(c.exports.destinationReadBegin(state.project, 2), null);
  const snapshot = c.probe.snapshot(); assertClean(snapshot);
  assert.equal(snapshot.commits, 1); assert.equal(snapshot.reads, 1); assert.equal(snapshot.rows.length, 8);
});

test('stale and fabricated read tokens cannot accept a response', () => {
  const c = fresh(), old = complete(c);
  begin(c, { generation: 2 });
  c.exports.destinationReadAccepted(old.token, old.spec);
  c.exports.destinationReadAccepted({}, old.spec);
  const snapshot = c.probe.snapshot();
  assert.equal(snapshot.accepted, 1); assert(snapshot.errors > 0);
});

test('replaced target cannot commit an earlier accepted response', () => {
  const c = fresh(), state = begin(c); accepted(c, state);
  c.exports.destinationNavigate(null, 'discover');
  c.exports.destinationNavigate('different-private-project', 'discover');
  commit(c, state);
  const snapshot = c.probe.snapshot();
  assert.equal(snapshot.commits, 0); assert(snapshot.errors > 0);
});

test('aborted or unaccepted reads cannot commit', () => {
  const c = fresh(), state = begin(c);
  commit(c, state);
  const snapshot = c.probe.snapshot();
  assert.equal(snapshot.accepted, 0); assert.equal(snapshot.commits, 0);
  assert(snapshot.errors > 0);
});

test('commit rejects stale generation, cloned response, changed revisions, value or ref', () => {
  const changes = [
    state => ({ generation: state.generation + 1 }),
    state => ({ spec: { ...state.spec } }),
    state => { state.spec.revision++; return {}; },
    state => { state.spec.briefRevision++; return {}; },
    state => { state.spec.purpose = 'changed-private-purpose'; return {}; },
    state => { state.element.value = 'wrong-private-value'; return {}; },
    state => { state.element.isConnected = false; return {}; },
    state => { state.element.tagName = 'INPUT'; return {}; },
  ];
  for (const change of changes) {
    const c = fresh(), state = begin(c); accepted(c, state);
    commit(c, state, change(state));
    const snapshot = c.probe.snapshot();
    assert.equal(snapshot.commits, 0); assert(snapshot.errors > 0);
  }
});

test('response target and invalid numeric revisions are refused', () => {
  for (const patch of [{ projectId: 'other-private-project' }, { revision: -1 }, { revision: NaN }, { briefRevision: 1.5 }, { purpose: null }]) {
    const c = fresh(), state = begin(c); Object.assign(state.spec, patch); accepted(c, state);
    const snapshot = c.probe.snapshot();
    assert.equal(snapshot.accepted, 0); assert(snapshot.errors > 0);
  }
});

test('evaluation refuses wrong action, target, value and replaced element at witness', () => {
  for (const scenario of ['action', 'target', 'value', 'element']) {
    const c = fresh(), state = begin(c); accepted(c, state); commit(c, state);
    const evaluation = c.probe.evaluationEnter(scenario === 'action' ? 2 : 1,
      scenario === 'target' ? 'wrong-private-project' : state.project,
      scenario === 'value' ? 'wrong-private-purpose' : PRIVATE_VALUE, state.element);
    const element = scenario === 'element' ? c.element() : state.element;
    c.probe.evaluationSample(evaluation, element, true);
    c.probe.evaluationWitness(evaluation, element);
    const snapshot = c.probe.snapshot();
    assert.equal(snapshot.witnesses, 0); assert(snapshot.errors > 0);
  }
});

test('a caller cannot claim a positive sample while detached or showing a wrong value', () => {
  for (const disconnected of [false, true]) {
    const c = fresh(), state = begin(c); accepted(c, state); commit(c, state);
    const evaluation = evaluate(c, state);
    if (disconnected) state.element.isConnected = false; else state.element.value = 'wrong-private-purpose';
    c.probe.evaluationSample(evaluation, state.element, true);
    c.probe.evaluationWitness(evaluation, state.element);
    const snapshot = c.probe.snapshot();
    assert.equal(snapshot.witnesses, 0); assert(snapshot.errors > 0);
  }
});

test('snapshot is numeric, drains rows and never serializes identities or values', () => {
  const c = fresh(); complete(c); const snapshot = c.probe.snapshot();
  for (const [key, value] of Object.entries(snapshot)) if (key !== 'rows') assert(Number.isFinite(value), key);
  for (const row of snapshot.rows) for (const [key, value] of Object.entries(row)) assert(Number.isFinite(value), key);
  const encoded = JSON.stringify(snapshot);
  for (const privateValue of [PRIVATE_PROJECT, PRIVATE_VALUE]) assert(!encoded.includes(privateValue));
  const next = c.probe.snapshot();
  assert.equal(next.rows.length, 0); assert.equal(next.observed, snapshot.observed);
  assert.equal(next.actions, 1); assert.equal(next.witnesses, 1);
});

test('100 actions retain a contiguous sequence; the 101st exceeds the declared bound', () => {
  const c = fresh();
  for (let action = 1; action <= 100; action++) complete(c, action);
  const snapshot = c.probe.snapshot(); assertClean(snapshot);
  assert.equal(snapshot.actions, 100); assert.equal(snapshot.rows.length, 800);
  assert.deepEqual(clone(snapshot.rows.map(row => row.sequence)), Array.from({ length: 800 }, (_, index) => index + 1));
  const receipt = receiptFor(snapshot, 100); receipt.homePhaseCapture = { homes: 100 };
  assert.doesNotThrow(() => validateDestinationCapture(receipt, 100));
  c.exports.destinationNavigate(null, 'discover');
  const overflow = c.probe.snapshot();
  assert(overflow.dropped > 0); assert(overflow.errors > 0);
  assert(overflow.actions <= 100, 'Action counter must remain bounded');
});

test('sample observation is bounded and overflow refuses capture', () => {
  const c = fresh(), state = begin(c); accepted(c, state); commit(c, state);
  const evaluation = evaluate(c, state); state.element.value = 'not-ready';
  for (let index = 0; index < 2050; index++) c.probe.evaluationSample(evaluation, state.element, false);
  const snapshot = c.probe.snapshot();
  assert(snapshot.samples <= 2048); assert(snapshot.rows.length <= 2048);
  assert(snapshot.errors > 0); assert(snapshot.dropped > 0);
  assert.throws(() => validateDestinationCapture(receiptFor(snapshot), 1));
});

test('capture validation refuses missing phases, duplicated rows, nonnumeric rows and counters', () => {
  const c = fresh(); complete(c); const valid = c.probe.snapshot();
  const mutations = [
    value => { value.rows.splice(value.rows.findIndex(row => row.kind === 5), 1); },
    value => { value.rows[1].sequence = value.rows[0].sequence; },
    value => { value.rows[0].privateValue = PRIVATE_VALUE; },
    value => { value.rows[0].at = NaN; },
    value => { value.observed++; },
    value => { value.errors++; },
    value => { value.dropped++; },
    value => { value.pendingReads++; },
    value => { value.pendingEvaluations++; },
  ];
  for (const mutate of mutations) {
    const snapshot = clone(valid); mutate(snapshot);
    assert.throws(() => validateDestinationCapture(receiptFor(snapshot), 1));
  }
});

test('capture validation refuses missing actions and a witness before its product commit', () => {
  const c = fresh(); complete(c); const valid = c.probe.snapshot();
  assert.throws(() => validateDestinationCapture(receiptFor(valid), 2));
  const outOfOrder = clone(valid), commitRow = outOfOrder.rows.find(row => row.kind === 5);
  const witnessRow = outOfOrder.rows.find(row => row.kind === 8);
  [commitRow.kind, witnessRow.kind] = [witnessRow.kind, commitRow.kind];
  assert.throws(() => validateDestinationCapture(receiptFor(outOfOrder), 1));
});

test('capture validation enforces monotonic windows and preserves drained rows', () => {
  const c = fresh(); complete(c); const first = c.probe.snapshot();
  complete(c, 2); const second = c.probe.snapshot();
  const receipt = receiptFor(first, 2);
  receipt.browserEventWindows.push({ destinationPhase: clone(second) });
  assert.doesNotThrow(() => validateDestinationCapture(receipt, 2));
  assert.equal(receipt.destinationPhaseCapture.rows, 16);
  for (const name of ['observed', 'samples', 'actions']) {
    const broken = clone(receipt); broken.browserEventWindows[1].destinationPhase[name] = 0;
    assert.throws(() => validateDestinationCapture(broken, 2));
  }
});

test('capture validation requires unchanged successful switches and ordered await markers', () => {
  const c = fresh(); complete(c); const snapshot = c.probe.snapshot();
  const changes = [
    receipt => { receipt.interactions[0].success = false; },
    receipt => { receipt.interactions[0].diagnostics.nodeStages.splice(2, 1); },
    receipt => { receipt.interactions[0].diagnostics.nodeStages[2].stage = 'evaluate-start'; },
    receipt => { receipt.interactions[0].diagnostics.nodeStages[2].offsetMs = -1; },
    receipt => { receipt.homePhaseCapture = { homes: 2 }; },
  ];
  for (const change of changes) {
    const receipt = receiptFor(snapshot); change(receipt);
    assert.throws(() => validateDestinationCapture(receipt, 1));
  }
});

test('immediate value witness preserves the original oracle and schedules no retry frame', async () => {
  const c = fresh(); c.invoke(browserLatencyInit);
  const state = begin(c); accepted(c, state); commit(c, state);
  await c.invoke(destinationValueWitness, state.element, { value: PRIVATE_VALUE, action: 1, projectId: state.project });
  assert.equal(c.frames.length, 0);
  const snapshot = c.probe.snapshot(); assertClean(snapshot);
  assert.equal(snapshot.evaluations, 1); assert.equal(snapshot.samples, 1); assert.equal(snapshot.witnesses, 1);
  assert.equal(c.context.window.__luciAcceptanceLatency.snapshot().rows.filter(row => row.kind === 'expected-dom').length, 1);
});

test('later rAF value witness records entry before readiness and keeps disconnected/value checks', async () => {
  const c = fresh(); c.invoke(browserLatencyInit);
  const state = begin(c); state.element.value = 'not-ready';
  const pending = c.invoke(destinationValueWitness, state.element, { value: PRIVATE_VALUE, action: 1, projectId: state.project });
  assert.equal(c.frames.length, 1);
  c.flushFrame(() => { state.element.value = PRIVATE_VALUE; state.element.isConnected = false; });
  assert.equal(c.frames.length, 1, 'Matching text while detached cannot complete');
  c.flushFrame(() => { state.element.isConnected = true; accepted(c, state); commit(c, state); });
  await pending;
  const snapshot = c.probe.snapshot(); assertClean(snapshot);
  assert.equal(snapshot.samples, 3); assert.equal(snapshot.witnesses, 1);
  assert.equal(snapshot.rows.find(row => row.kind === 7).match, 0);
});

test('diagnostic binding refusal does not replace the original connected-value oracle', async () => {
  const c = fresh(); c.invoke(browserLatencyInit);
  // No navigation/read binding exists, so diagnostics must refuse this capture.
  // The existing connected/value completion still succeeds independently.
  await c.invoke(destinationValueWitness, c.element(), { value: PRIVATE_VALUE, action: 1, projectId: PRIVATE_PROJECT });
  const snapshot = c.probe.snapshot();
  assert(snapshot.errors > 0); assert.equal(snapshot.witnesses, 0);
  assert.equal(c.context.window.__luciAcceptanceLatency.snapshot().rows.filter(row => row.kind === 'expected-dom').length, 1);
});

test('value witness retains its original ten-second failure', async () => {
  const c = fresh({ enabled: false }); c.invoke(browserLatencyInit);
  const element = c.element('never-ready');
  const pending = c.invoke(destinationValueWitness, element, { value: PRIVATE_VALUE, action: 1, projectId: PRIVATE_PROJECT });
  c.advance(10001); c.flushFrame();
  await assert.rejects(pending, /Expected project context did not render/);
  assert.equal(c.context.window.__luciAcceptanceLatency.snapshot().rows.filter(row => row.kind === 'expected-dom').length, 0);
});

function mockedRunner({ incorrectInputValue = false } = {}) {
  const c = fresh({ enabled: false }); c.invoke(browserLatencyInit);
  const log = [], element = c.element(), receipt = {};
  const owner = { owner: { id: PRIVATE_PROJECT, name: 'Contract project' }, goal: PRIVATE_VALUE };
  const settle = async pending => {
    for (let count = 0; c.frames.length && count < 10; count++) c.flushFrame();
    assert.equal(c.frames.length, 0, 'Mock frame work must remain bounded');
    return await pending;
  };
  const field = {
    async waitFor() { log.push('field.waitFor'); },
    async evaluate(fn, args) { log.push('field.evaluate'); return await settle(c.invoke(fn, element, args)); },
    async inputValue() { log.push('field.inputValue'); return incorrectInputValue ? 'wrong-value' : element.value; },
    async focus() { log.push('field.focus'); c.context.document.activeElement = element; },
    async press(key) {
      log.push(`field.press:${key}`);
      if (key === 'x') element.value += 'x';
      if (key === 'Backspace') element.value = element.value.slice(0, -1);
    },
  };
  const locator = name => ({
    async click() { log.push(`click:${name}`); },
    async waitFor() { log.push(`waitFor:${name}`); },
    getByRole(_role, options) { return locator(options.name); },
  });
  const page = {
    getByRole(role, options) { return role === 'textbox' ? field : locator(options.name); },
    async evaluate(fn) {
      assert.equal(fn, browserFrameWitness, 'Existing frame RPC callback must be preserved');
      log.push('page.evaluate:two-frames');
      const before = c.context.performance.now();
      const value = await settle(c.invoke(fn));
      assert(value.requestedAt >= before);
      assert(value.requestedAt < value.firstFrameAt && value.firstFrameAt < value.secondFrameAt);
      return value;
    },
    async waitForFunction(fn, value, options) {
      log.push('page.waitForFunction'); assert.equal(options.timeout, 10000);
      assert.equal(c.invoke(fn, value), true);
    },
  };
  const runner = interactionRunner({ page, owners: [owner], receipt, phase: () => 'contract',
    assertHealthy: () => log.push('assertHealthy'), observe: async () => log.push('observe') });
  return { runner, receipt, log };
}

test('mocked interactionRunner preserves await order, assertions and all four two-frame calls', async () => {
  const { runner, receipt, log } = mockedRunner();
  await runner.calibrate(1);
  assert.deepEqual(log, [
    'assertHealthy', 'click:All projects', 'waitFor:Across your projects', 'click:Open project Contract project',
    'field.waitFor', 'field.evaluate', 'field.inputValue', 'page.evaluate:two-frames',
    'assertHealthy', 'field.focus', 'field.press:End', 'field.press:x', 'page.waitForFunction', 'page.evaluate:two-frames',
    'field.press:Backspace', 'page.waitForFunction', 'page.evaluate:two-frames',
    'click:Design', 'click:Discard edits and continue', 'click:Discover', 'field.inputValue', 'page.evaluate:two-frames', 'observe',
  ]);
  assert.deepEqual(receipt.interactions.map(row => row.kind), ['project-switch', 'input', 'input', 'draft-reset']);
  assert(receipt.interactions.every(row => row.success));
  const switchStages = receipt.interactions[0].diagnostics.nodeStages.map(row => row.stage);
  assert.deepEqual(switchStages, [
    'action-start', 'all-projects-click-end', 'overview-heading-ready', 'open-project-click-end', 'action-end', 'verify-start',
    'waitFor-start', 'waitFor-end', 'evaluate-start', 'evaluate-end', 'inputValue-start', 'inputValue-end',
    'verify-end', 'frames-start', 'frames-end',
  ]);
  assert.equal(receipt.interactions[0].diagnostics.nodeClockReads, 15);
  for (const row of receipt.interactions.slice(1)) assert.equal(row.diagnostics.nodeStages.length, 6);
});

test('mocked interactionRunner retains inputValue assertion and does not continue after failure', async () => {
  const { runner, receipt, log } = mockedRunner({ incorrectInputValue: true });
  await assert.rejects(runner.calibrate(1), /Expected values to be strictly equal/);
  assert.equal(receipt.interactions.length, 1); assert.equal(receipt.interactions[0].success, false);
  assert.equal(log.at(-1), 'field.inputValue');
  assert(!log.includes('page.evaluate:two-frames')); assert(!log.includes('field.focus'));
});

test('Node recorder keeps diagnostics bounded and refuses unknown stages', () => {
  const item = {}, recorder = nodeStageRecorder(item, 0, () => 1);
  assert.throws(() => recorder.mark('unapproved-private-marker'));
  for (let index = 0; index < 18; index++) recorder.mark('waitFor-start');
  assert.throws(() => recorder.mark('waitFor-end'));
  recorder.close(); recorder.mark('waitFor-end');
  assert.equal(item.diagnostics.nodeStages.length, 18);
});

let passed = 0;
const suiteDeadline = setTimeout(() => {
  console.error('FAIL offline contract suite exceeded its 20-second bound');
  process.exit(1);
}, 20000);
try {
  for (const { name, fn } of tests) {
    await fn(); passed++;
    console.log(`PASS ${name}`);
  }
} finally {
  clearTimeout(suiteDeadline);
}
console.log(JSON.stringify({
  tests: tests.length, passed, probeSHA256: hash(source),
  helperSHA256: Object.fromEntries(['destination-diagnostics.mjs', 'interaction-metrics.mjs', 'latency-diagnostics.mjs']
    .map(name => [name, hash(readFileSync(resolve(root, 'actual', name)))])),
  typescriptVersion: ts.version, typescriptSHA256: hash(readFileSync(require.resolve('typescript'))),
  transpileOnly: true, browser: false, webBuild: false, services: false,
}));

// Controlled scenario orchestration tests only. Every API, browser, Docker,
// ledger and source observation below is a synthetic in-process DTO. These
// tests are not evidence of an installed product recovery or a real fence.
import test from 'node:test';
import assert from 'node:assert/strict';
import {runInstalledRecoveryScenario, pins, command} from './recovery-scenario.mjs';

const clone = structuredClone;
const output = 'RECOVERY_CHECK_OK\n';
const originalID = 'd'.repeat(64), markerID = 'f'.repeat(64);
const when = '2026-01-01T00:00:00.000Z';
function identity(digit, projectId) {
  return {protocol: 1, engineId: 'synthetic-engine', instanceId: 'synthetic-instance',
    allocationId: 'checkexec-' + digit.repeat(24), checkId: 'check-' + digit.repeat(24),
    commandIndex: 0, scopeDigest: 'a'.repeat(64), projectId, commandDigest: digit.repeat(64)};
}
function lease(id, state = 'creating') {
  return {id: id.allocationId, scope: id.scopeDigest, identity: clone(id), state,
    actor: {local: false, userId: 'synthetic-owner', teamId: 'synthetic-team', projectId: id.projectId},
    resources: {CPUMillis: 1000, MemoryMiB: 1024}, ref: '', createDiskBytes: 2 * 1024 ** 3,
    createdAt: when, updatedAt: when};
}
function check(id, status = 'unknown') {
  return {id: id.checkId, projectId: id.projectId, revision: 7, status,
    reasonCode: status === 'unknown' ? 'check_create_unconfirmed' : '', resourcesHeld: true,
    evidenceStatus: 'unverified', commit: 'b'.repeat(40), sourceDigest: 'c'.repeat(64),
    commands: [{allocationId: id.allocationId, ...clone(command), status,
      cleanupConfirmed: false, output: '', truncated: false, exitCode: null}]};
}
function passing(row) {
  row.status = 'passed'; row.resourcesHeld = false;
  Object.assign(row.commands[0], {status: 'passed', cleanupConfirmed: true, output, exitCode: 0});
  return row;
}

function controlledContext({chargedWitness = false, hooks = {}} = {}) {
  const targetProject = {id: 'synthetic-target'}, otherProject = {id: 'synthetic-unrelated'};
  const ids = {target: identity('1', targetProject.id), witness: identity('2', otherProject.id), history: identity('3', otherProject.id)};
  const state = {calls: [], records: [], checks: new Map(), gates: new Map(),
    ledger: {version: 1, leases: [], checks: [], fences: [], checkFences: []},
    files: {[targetProject.id]: {'verify.mjs': 'target-file-digest'}, [otherProject.id]: {'verify.mjs': 'unrelated-file-digest'}},
    recovered: false, released: false, fenceVerified: false, markerPresent: true};
  const note = name => state.calls.push(name);
  const hook = async name => {note(name); if (hooks[name]) await hooks[name](state, ctx);};
  const observations = {counts: {create: 0, start: 0, check: 0, fence: 0, worker: 0}, requests: [], dropped: 0, failures: []};
  const request = (method, path, details = {}) => {
    const row = {method, path, requestSequence: observations.requests.length + 1, ...details};
    observations.requests.push(row); return row;
  };
  const checkCreate = (id, containerID) => {
    observations.counts.create++; observations.counts.check++;
    return request('POST', '/v1.45/containers/create', {kind: 'check', id: containerID, status: 201,
      labels: {'luci.check-project': id.projectId, 'luci.check-id': id.checkId, 'luci.check-allocation': id.allocationId}});
  };
  const start = containerID => {observations.counts.start++; request('POST', '/v1.45/containers/' + containerID + '/start', {ref: containerID});};
  const read = id => clone(state.checks.get(id));
  const ctx = {windowGranted: true, pins: clone(pins), freshFixture: true, externalModelCalls: 0,
    projects: [targetProject, otherProject], chargedWitness, expectedOutput: output,
    record(row) {state.records.push(clone(row));},
    proxy: {observations,
      armGate(gate) {
        note('arm:' + gate.id); assert(!state.gates.has(gate.id));
        assert.equal(gate.method, 'POST'); assert(gate.pathPattern.test('/v1.45/containers/create'));
        if (gate.id === 'target-create-ack-lost') {
          assert.equal(gate.phase, 'after-response'); assert.equal(gate.decision, 'lose-response');
        } else {
          assert.equal(gate.id, 'unrelated-create-held'); assert.equal(gate.phase, 'before-forward'); assert.equal(gate.decision, 'hold');
        }
        state.gates.set(gate.id, gate);
      },
      async waitForGate(name, options) {
        note('wait:' + name); assert.equal(options.timeoutMs, 120000); assert(state.gates.has(name));
        const id = name === 'target-create-ack-lost' ? ids.target : ids.witness;
        const row = observations.requests.find(row => row.labels?.['luci.check-id'] === id.checkId);
        assert(row); assert(state.gates.get(name).match(row)); return {request: clone(row)};
      },
      async releaseGate(name) {
        note('release:' + name); assert.equal(name, 'unrelated-create-held');
        assert(state.recovered && state.fenceVerified, 'Witness request remains held until recovery proof');
        assert.equal(state.released, false); state.released = true;
        passing(state.checks.get(ids.witness.checkId));
        state.ledger.checks.find(row => row.id === ids.witness.allocationId).state = 'finished';
        start('e'.repeat(64)); await hook('afterWitnessRelease');
      },
    },
    async startCheckInBrowser(project, cmd) {
      assert.deepEqual(cmd, command);
      const role = project.id === targetProject.id ? 'target' : state.checks.has(ids.history.checkId) ? 'witness' : 'history';
      note('start:' + role); const id = ids[role];
      if (role === 'target') assert(state.gates.has('target-create-ack-lost'));
      if (role === 'witness') assert(state.gates.has('unrelated-create-held'));
      const row = role === 'history' ? passing(check(id, 'passed')) : check(id, role === 'target' ? 'unknown' : 'running');
      state.checks.set(row.id, row); state.ledger.checks.push(lease(id, role === 'history' ? 'finished' : 'creating'));
      checkCreate(id, role === 'target' ? originalID : role === 'history' ? 'c'.repeat(64) : 'e'.repeat(64));
      if (role === 'history') start('c'.repeat(64));
      await hook('afterStart:' + role); return {id: row.id};
    },
    async waitCheckTerminal(project, checkId) {note('terminal:' + checkId); return read(checkId);},
    async readCheck(project, checkId) {return read(checkId);},
    async readLedger() {return clone(state.ledger);},
    async snapshotProjectFiles() {return clone(state.files);},
    async assertRealOriginalCreatedAndNeverStarted(row, allocation) {
      note('original-inspect'); assert.equal(row.id, originalID); assert.equal(allocation.id, ids.target.allocationId);
      assert(!observations.requests.some(row => row.path.endsWith('/start') && row.ref === originalID));
      await hook('afterOriginalInspect');
    },
    async admitAdditionalChargedWitness() {note('admit-witness'); assert(chargedWitness);},
    async assertCurrentReservationTotals(expected) {
      const held = state.ledger.checks.filter(row => row.state !== 'finished');
      note('reservations:' + held.length);
      assert.deepEqual(expected, {count: held.length, memoryMiB: held.length * 1024, cpuMillis: held.length * 1000});
    },
    async readRecovery(project, checkId) {
      note('read-review'); const row = state.checks.get(checkId);
      return {projectId: project.id, checkId, revision: row.revision,
        state: state.recovered ? 'complete' : 'available', method: state.recovered ? null : 'fence-launch',
        commandIndex: 0, recoveryDigest: state.recovered ? null : '9'.repeat(64), reasonCode: '', message: 'Synthetic review'};
    },
    async openAndDismissReviewInBrowser(project, checkId, review) {
      assert.equal(review.checkId, checkId); await hook('afterDismiss');
    },
    async probeNoEditAuthority(probe) {assert.equal(probe.expectedStatus, 403); await hook('afterNoEdit');},
    async probeStaleReview(probe) {assert.equal(probe.expectedStatus, 409); await hook('afterStale');},
    async recoverInBrowser(project, checkId, review) {
      note('recover'); assert.equal(state.recovered, false); state.recovered = true;
      const row = state.checks.get(checkId), original = state.ledger.checks.find(row => row.id === ids.target.allocationId);
      assert.equal(review.revision, row.revision);
      Object.assign(row, {revision: row.revision + 1, status: 'cancelled', reasonCode: 'check_stopped_by_review', resourcesHeld: false});
      Object.assign(row.commands[0], {status: 'cancelled', cleanupConfirmed: true});
      state.ledger.checks = state.ledger.checks.filter(row => row.id !== original.id);
      const fence = {scope: original.scope, identity: clone(original.identity), actor: clone(original.actor),
        intentId: '8'.repeat(32), state: 'retired', createdAt: when, updatedAt: when,
        proof: {identity: clone(original.identity), intentId: '8'.repeat(32), fenceRef: markerID,
          configDigest: '7'.repeat(64), acknowledgedAt: when, observedAt: when}};
      state.ledger.checkFences.push(fence);
      request('DELETE', '/v1.45/containers/' + originalID, {ref: originalID, status: 204});
      observations.counts.create++; observations.counts.fence++;
      request('POST', '/v1.45/containers/create', {kind: 'fence', id: markerID, status: 201, labels: {}});
      const result = {httpStatus: 200, request: {expectedRevision: review.revision, recoveryDigest: review.recoveryDigest, acknowledgeStop: true}, postCount: 1};
      state.posted = result; await hook('afterRecovery'); return clone(result);
    },
    async assertAcknowledgedPermanentFence(evidence) {
      note('verify-fence'); assert.equal(evidence.originalCreate.id, originalID);
      assert.equal(evidence.fence.proof.fenceRef, markerID); assert(state.markerPresent);
      state.fenceVerified = true; await hook('afterFenceProof');
    },
    async assertFenceStillPresent(fence) {
      note('marker-still-present'); assert.equal(fence.proof.fenceRef, markerID); assert(state.markerPresent);
      await hook('afterFinalMarkerInspect');
    },
  };
  state.ids = ids; state.request = request;
  return {ctx, state};
}

test('synthetic default flow proves preserved completed history and one-to-zero charge release, then awaits adapter closure', async () => {
  const {ctx, state} = controlledContext();
  const result = await runInstalledRecoveryScenario(ctx);
  assert.equal(result.passed, false); assert.equal(result.cleanupPending, true);
  assert.equal(result.witness, null); assert.equal(result.target.status, 'cancelled');
  assert.equal(result.target.commands[0].output, '');
  assert.deepEqual(result.retainedMarker, {id: markerID, allocationId: state.ids.target.allocationId});
  assert.equal(state.checks.get(state.ids.history.checkId).commands[0].output, output);
  assert.equal(state.calls.includes('admit-witness'), false); assert.equal(state.released, false);
  assert.deepEqual(state.records.map(row => row.phase), ['real-history-created', 'real-create-ack-lost', 'exact-target-charge-retired', 'scenario-complete-marker-retained']);
  const retirement = state.records[2];
  assert.deepEqual(retirement.beforeCharge, {memoryMiB: 1024, cpuMillis: 1000});
  assert.deepEqual(retirement.afterCharge, {memoryMiB: 0, cpuMillis: 0});
  assert.equal(retirement.concurrentChargedWitness, 'not-exercised-single-allocation-scope');
  assert(state.calls.indexOf('afterNoEdit') < state.calls.indexOf('afterStale'));
  assert(state.calls.indexOf('afterStale') < state.calls.indexOf('recover'));
  assert.equal(state.calls.filter(call => call === 'recover').length, 1);
  assert.equal(state.calls.filter(call => call === 'verify-fence').length, 1);
  assert.equal(state.calls.filter(call => call === 'marker-still-present').length, 1);
});

test('synthetic charged-witness flow holds the unrelated request through exact target retirement and releases it once', async () => {
  const {ctx, state} = controlledContext({chargedWitness: true});
  const result = await runInstalledRecoveryScenario(ctx);
  assert.equal(result.passed, false); assert.equal(result.cleanupPending, true);
  assert.equal(result.witness.status, 'passed'); assert.equal(result.witness.commands[0].output, output);
  assert.equal(state.calls.filter(call => call === 'admit-witness').length, 1);
  assert.equal(state.calls.filter(call => call === 'release:unrelated-create-held').length, 1);
  assert(state.calls.indexOf('verify-fence') < state.calls.indexOf('release:unrelated-create-held'));
  assert.deepEqual(state.records[2].beforeCharge, {memoryMiB: 2048, cpuMillis: 2000});
  assert.deepEqual(state.records[2].afterCharge, {memoryMiB: 1024, cpuMillis: 1000});
  assert.equal(state.records[2].concurrentChargedWitness, 'verified');
});

for (const [phase, label] of [['afterDismiss', 'Read and dismiss'], ['afterNoEdit', 'No current Edit authority'], ['afterStale', 'Stale reviewed revision']]) {
  for (const [kind, mutation] of [
    ['missing history', state => state.checks.delete(state.ids.history.checkId)],
    ['changed history output', state => {state.checks.get(state.ids.history.checkId).commands[0].output = 'REWRITTEN\n';}],
    ['changed source', state => {state.files['synthetic-target']['verify.mjs'] = 'changed-digest';}],
    ['unexpected Docker mutation', state => {state.request('DELETE', '/v1.45/containers/foreign', {ref: 'foreign'});}],
  ]) test(`synthetic flow rejects ${kind} after ${label}`, async () => {
    const {ctx, state} = controlledContext({hooks: {[phase]: mutation}});
    await assert.rejects(runInstalledRecoveryScenario(ctx), error => error.code === 'ERR_ASSERTION' && error.message.includes(label));
    assert.equal(state.calls.includes('recover'), false);
    assert.equal(state.ledger.checks.find(row => row.id === state.ids.target.allocationId).state, 'creating');
  });
}

for (const [name, mutation, pattern] of [
  ['a second target Create', (state, ctx) => {
    const original = ctx.proxy.observations.requests.find(row => row.id === originalID);
    state.request('POST', '/v1.45/containers/create', {...clone(original), requestSequence: 99});
  }, /must not replay target Create/],
  ['a target Start', (state, ctx) => {state.request('POST', '/v1.45/containers/' + originalID + '/start', {ref: originalID});}, /Expected values to be strictly equal/],
  ['history deletion during successful recovery', state => {state.checks.delete(state.ids.history.checkId);}, /Recorded output\/history/],
  ['source modification during successful recovery', state => {state.files['synthetic-unrelated']['verify.mjs'] = 'rewritten';}, /Expected values to be strictly deep-equal/],
  ['multiple browser recovery posts', state => {state.posted.postCount = 2;}, /Exactly one successful explicit browser recovery POST/],
  ['a mismatched reviewed digest', state => {state.posted.request.recoveryDigest = '0'.repeat(64);}, /Expected values to be strictly deep-equal/],
]) test('synthetic flow rejects ' + name, async () => {
  const {ctx, state} = controlledContext({hooks: {afterRecovery: mutation}});
  await assert.rejects(runInstalledRecoveryScenario(ctx), pattern);
  assert.equal(state.records.some(row => row.phase === 'scenario-complete-marker-retained'), false);
});

test('synthetic charged-witness flow rejects changing the other active charge during target recovery', async () => {
  const {ctx, state} = controlledContext({chargedWitness: true, hooks: {afterRecovery: state => {
    state.ledger.checks.find(row => row.id === state.ids.witness.allocationId).resources.MemoryMiB = 512;
  }}});
  await assert.rejects(runInstalledRecoveryScenario(ctx));
  assert.equal(state.released, false);
});

test('synthetic charged-witness flow rechecks history after the unrelated request finishes', async () => {
  const {ctx, state} = controlledContext({chargedWitness: true, hooks: {afterWitnessRelease: state => {
    state.checks.get(state.ids.history.checkId).commands[0].output = 'REWRITTEN AFTER RETIREMENT\n';
  }}});
  await assert.rejects(runInstalledRecoveryScenario(ctx));
  assert.equal(state.released, true);
  assert.equal(state.records.some(row => row.phase === 'scenario-complete-marker-retained'), false);
});

test('synthetic flow refuses completion if the permanent marker disappears after retirement proof', async () => {
  const {ctx, state} = controlledContext({hooks: {afterFenceProof: state => {state.markerPresent = false;}}});
  await assert.rejects(runInstalledRecoveryScenario(ctx));
  assert.equal(state.calls.includes('marker-still-present'), true);
  assert.equal(state.records.some(row => row.phase === 'scenario-complete-marker-retained'), false);
});

test('synthetic flow requires the explicit runtime-window gate before any adapter action', async () => {
  const {ctx, state} = controlledContext(); ctx.windowGranted = false;
  await assert.rejects(runInstalledRecoveryScenario(ctx), /Root must explicitly release the runtime window/);
  assert.deepEqual(state.calls, []); assert.deepEqual(ctx.proxy.observations.requests, []);
});

test('synthetic flow accepts an explicitly selected exact product source while retaining helper and image pins',async()=>{
  const {ctx}=controlledContext();ctx.expectedSource='9'.repeat(40);ctx.pins={...ctx.pins,product:ctx.expectedSource};
  const result=await runInstalledRecoveryScenario(ctx);assert.equal(result.passed,false);assert.equal(result.cleanupPending,true);
});
for(const [label,mutate] of [
  ['unselected product',ctx=>{ctx.pins={...ctx.pins,product:'9'.repeat(40)};}],
  ['mismatched selected product',ctx=>{ctx.expectedSource='9'.repeat(40);}],
  ['changed runtime image',ctx=>{ctx.pins={...ctx.pins,image:'sha256:'+'9'.repeat(64)};}],
  ['changed helper baseline',ctx=>{ctx.pins={...ctx.pins,helper:'9'.repeat(40)};}],
])test('synthetic flow refuses '+label+' before adapter actions',async()=>{
  const {ctx,state}=controlledContext();mutate(ctx);await assert.rejects(runInstalledRecoveryScenario(ctx));assert.deepEqual(state.calls,[]);
});

// Pure proof/admission tests using controlled DTOs only; no installed runtime.
import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {exactLease, assertUnknown, assertReview, assertRetired} from './recovery-scenario.mjs';
import {validateFence, assertCapacity, assertComputeEnvelope, identityLabels} from './fixture-guards.mjs';

const clone = structuredClone, MiB = 1024 ** 2, GiB = 1024 ** 3;
const hash = text => createHash('sha256').update(text).digest('hex');
function makeIdentity(digit, projectId = 'project-' + digit) {
  return {protocol: 1, engineId: 'fixture-engine', instanceId: 'fixture-instance',
    allocationId: 'checkexec-' + digit.repeat(24), checkId: 'check-' + digit.repeat(24),
    commandIndex: 0, scopeDigest: 'a'.repeat(64), projectId, commandDigest: digit.repeat(64)};
}
function makeLease(identity, state = 'creating') {
  return {id: identity.allocationId, scope: identity.scopeDigest, identity: clone(identity), state,
    actor: {local: false, userId: 'fixture-owner', teamId: 'fixture-team', projectId: identity.projectId},
    resources: {CPUMillis: 1000, MemoryMiB: 1024}, ref: '', createDiskBytes: 2 * GiB,
    createdAt: '2026-01-01T00:00:00.000Z', updatedAt: '2026-01-01T00:00:01.000Z'};
}
function makeCheck(identity, status = 'unknown') {
  return {id: identity.checkId, projectId: identity.projectId, revision: 7, status,
    reasonCode: status === 'unknown' ? 'check_create_unconfirmed' : '', resourcesHeld: true,
    evidenceStatus: 'unverified', commit: 'b'.repeat(40), sourceDigest: 'c'.repeat(64),
    commands: [{allocationId: identity.allocationId, label: 'node', executable: 'node', args: ['verify.mjs', 'final'],
      status, cleanupConfirmed: false, output: '', truncated: false, exitCode: null}]};
}
function retirement() {
  const targetIdentity = makeIdentity('1'), witnessIdentity = makeIdentity('2'), historicalIdentity = makeIdentity('3');
  const target = makeLease(targetIdentity), witness = makeLease(witnessIdentity), historyLease = makeLease(historicalIdentity, 'finished');
  const targetBefore = makeCheck(targetIdentity), witnessBefore = makeCheck(witnessIdentity, 'running');
  const historyBefore = makeCheck(historicalIdentity, 'passed');
  historyBefore.resourcesHeld = false; historyBefore.commands[0].cleanupConfirmed = true;
  historyBefore.commands[0].output = 'RECOVERY_CHECK_OK final\n'; historyBefore.commands[0].exitCode = 0;
  const before = {version: 1, leases: [], checks: [target, witness, historyLease], fences: [], checkFences: []};
  const targetAfter = clone(targetBefore);
  Object.assign(targetAfter, {revision: 10, status: 'cancelled', reasonCode: 'check_stopped_by_review', resourcesHeld: false});
  Object.assign(targetAfter.commands[0], {status: 'cancelled', cleanupConfirmed: true});
  const record = {scope: target.scope, identity: clone(targetIdentity), actor: clone(target.actor),
    intentId: 'd'.repeat(32), state: 'retired', createdAt: '2026-01-01T00:00:02.000Z', updatedAt: '2026-01-01T00:00:04.000Z'};
  record.proof = {identity: clone(record.identity), intentId: record.intentId,
    fenceRef: 'e'.repeat(64), configDigest: 'f'.repeat(64),
    acknowledgedAt: '2026-01-01T00:00:03.000Z', observedAt: '2026-01-01T00:00:04.000Z'};
  const after = clone(before); after.checks = after.checks.filter(row => row.id !== target.id); after.checkFences.push(record);
  return {before, after, targetBefore, targetAfter, witnessBefore, witnessAfter: clone(witnessBefore),
    historyBefore, historyAfter: clone(historyBefore)};
}

test('exactLease binds one real-shaped active allocation and the exact resource charge', () => {
  const f = retirement(); assert.equal(exactLease(f.before, f.targetBefore), f.before.checks[0]);
  assertUnknown(f.targetBefore, f.before.checks[0]);
});
for (const [name, mutate] of [
  ['missing allocation', f => {f.before.checks.shift();}],
  ['duplicate allocation', f => {f.before.checks.push(clone(f.before.checks[0]));}],
  ['finished allocation', f => {f.before.checks[0].state = 'finished';}],
  ['wrong memory charge', f => {f.before.checks[0].resources.MemoryMiB = 512;}],
  ['wrong CPU charge', f => {f.before.checks[0].resources.CPUMillis = 500;}],
  ['wrong outer scope', f => {f.before.checks[0].scope = '9'.repeat(64);}],
  ['wrong allocation identity', f => {f.before.checks[0].identity.allocationId = 'checkexec-' + '9'.repeat(24);}],
  ['wrong project identity', f => {f.before.checks[0].identity.projectId = 'other-project';}],
  ['wrong command index', f => {f.before.checks[0].identity.commandIndex = 1;}],
  ['multiple commands', f => {f.targetBefore.commands.push(clone(f.targetBefore.commands[0]));}],
]) test('exactLease rejects ' + name, () => {const f = retirement(); mutate(f); assert.throws(() => exactLease(f.before, f.targetBefore));});

for (const [name, mutate] of [
  ['invented output', f => {f.targetBefore.commands[0].output = 'unexecuted';}],
  ['product Create acknowledgement', f => {f.before.checks[0].ref = 'a'.repeat(64);}],
  ['released capacity', f => {f.targetBefore.resourcesHeld = false;}],
  ['confirmed cleanup', f => {f.targetBefore.commands[0].cleanupConfirmed = true;}],
  ['invented never-started command success', f => {f.targetBefore.commands[0].status = 'passed';}],
  ['invented never-started exit code', f => {f.targetBefore.commands[0].exitCode = 0;}],
]) test('assertUnknown rejects ' + name, () => {const f = retirement(); mutate(f); assert.throws(() => assertUnknown(f.targetBefore, f.before.checks[0]));});

function review(row) {
  return {projectId: row.projectId, checkId: row.id, revision: row.revision, state: 'available', method: 'fence-launch',
    commandIndex: 0, recoveryDigest: 'a'.repeat(64), reasonCode: 'check_stop_review_required', message: 'Review exact retained check'};
}
test('assertReview accepts an exact current command review', () => {const f = retirement(); assertReview(review(f.targetBefore), f.targetBefore);});
for (const [name, mutate] of [
  ['stale revision', v => {v.revision--;}], ['wrong check', v => {v.checkId = 'another-check';}],
  ['wrong project', v => {v.projectId = 'another-project';}], ['wrong command', v => {v.commandIndex = 1;}],
  ['wrong method', v => {v.method = 'remove-exact';}], ['missing digest', v => {v.recoveryDigest = null;}],
  ['malformed digest', v => {v.recoveryDigest = 'z'.repeat(64);}], ['unexpected property', v => {v.authorized = true;}],
]) test('assertReview rejects ' + name, () => {const f = retirement(), v = review(f.targetBefore); mutate(v); assert.throws(() => assertReview(v, f.targetBefore));});

test('assertRetired proves precisely one charge release while preserving complete unrelated rows', () => {
  const f = retirement(), result = assertRetired(f);
  assert.equal(result.original.id, f.targetBefore.commands[0].allocationId);
  assert.equal(result.witness.id, f.witnessBefore.commands[0].allocationId);
  assert.equal(result.fence.state, 'retired');
});
test('assertRetired proves one-to-zero release with unrelated completed history and no charged-witness claim', () => {
  const f = retirement(), witnessID = f.witnessBefore.commands[0].allocationId;
  f.before.checks = f.before.checks.filter(row => row.id !== witnessID);
  f.after.checks = f.after.checks.filter(row => row.id !== witnessID);
  f.witnessBefore = null; f.witnessAfter = null;
  assert.notEqual(f.historyBefore.projectId, f.targetBefore.projectId);
  const result = assertRetired(f);
  assert.equal(result.witness, null); assert.equal(result.fence.state, 'retired');
  assert.equal(f.before.checks.filter(row => row.state !== 'finished').length, 1);
  assert.equal(f.after.checks.filter(row => row.state !== 'finished').length, 0);
  assert.deepEqual(f.historyAfter, f.historyBefore);
});
test('assertRetired also permits the exact finished target pending product receipt acknowledgement', () => {
  const f = retirement(), finished = clone(f.before.checks[0]); finished.state = 'finished'; f.after.checks.push(finished); assertRetired(f);
});
for (const [name, mutate] of [
  ['unrelated active charge release', f => {f.after.checks = f.after.checks.filter(row => row.id !== f.witnessBefore.commands[0].allocationId);}],
  ['unrelated charge resize', f => {f.after.checks[0].resources.MemoryMiB = 512;}],
  ['unrelated receipt revision', f => {f.witnessAfter.revision++;}],
  ['historical output rewrite', f => {f.historyAfter.commands[0].output = '';}],
  ['historical receipt deletion', f => {f.historyAfter = undefined;}],
  ['historical finished lease deletion', f => {f.after.checks = f.after.checks.filter(row => row.state !== 'finished');}],
  ['agent lease insertion', f => {f.after.leases.push({id: 'new-agent-allocation'});}],
  ['agent fence insertion', f => {f.after.fences.push({intentId: 'new-agent-fence'});}],
  ['foreign check fence insertion', f => {const other = clone(f.after.checkFences[0]); other.identity.allocationId = 'checkexec-' + '8'.repeat(24); f.after.checkFences.push(other);}],
  ['unknown ledger field change', f => {f.after.extraAuthority = true;}],
  ['target output invented', f => {f.targetAfter.commands[0].output = 'replayed';}],
  ['target arguments changed', f => {f.targetAfter.commands[0].args = ['another-command'];}],
  ['target command falsely passed', f => {f.targetAfter.commands[0].status = 'passed';}],
  ['target command invented exit code', f => {f.targetAfter.commands[0].exitCode = 0;}],
  ['target command added', f => {f.targetAfter.commands.push(clone(f.targetAfter.commands[0]));}],
  ['target project changed', f => {f.targetAfter.projectId = 'other-project';}],
  ['marker not retired', f => {f.after.checkFences[0].state = 'proved';}],
  ['marker scope changed', f => {f.after.checkFences[0].scope = '0'.repeat(64);}],
  ['marker actor changed', f => {f.after.checkFences[0].actor.userId = 'other-user';}],
  ['marker identity changed', f => {f.after.checkFences[0].identity.engineId = 'other-engine';}],
  ['proof identity changed', f => {f.after.checkFences[0].proof.identity.instanceId = 'other-instance';}],
  ['replayed different intent proof', f => {f.after.checkFences[0].proof.intentId = '9'.repeat(32);}],
  ['invalid marker ID', f => {f.after.checkFences[0].proof.fenceRef = 'short';}],
  ['invalid configuration digest', f => {f.after.checkFences[0].proof.configDigest = 'invalid';}],
  ['missing proof', f => {delete f.after.checkFences[0].proof;}],
  ['proof acknowledgement before intent', f => {f.after.checkFences[0].proof.acknowledgedAt = '2025-01-01T00:00:00Z';}],
  ['proof observed before acknowledgement', f => {f.after.checkFences[0].proof.observedAt = '2026-01-01T00:00:02Z';}],
  ['proof in future', f => {f.after.checkFences[0].proof.observedAt = '2999-01-01T00:00:00Z';}],
  ['duplicate finished target allocation', f => {const finished = clone(f.before.checks[0]); finished.state = 'finished'; f.after.checks.push(finished, clone(finished));}],
]) test('assertRetired rejects ' + name, () => {const f = retirement(); mutate(f); assert.throws(() => assertRetired(f));});

function fenceFixture() {
  const record = retirement().after.checkFences[0], name = 'luci-check-' + record.identity.allocationId;
  const binding = hash(JSON.stringify({Identity: record.identity, IntentID: record.intentId, ExpectedOriginalRef: ''}));
  const cfg = {Image: '', Cmd: ['/'], NetworkDisabled: true,
    Labels: {...identityLabels(record.identity), 'luci.launch-fence': '1', 'luci.fence-intent': record.intentId, 'luci.fence-binding': binding}};
  const host = {ReadonlyRootfs: true, NetworkMode: 'none', IpcMode: 'none', RestartPolicy: {Name: 'no'},
    CapDrop: ['ALL'], SecurityOpt: ['no-new-privileges:true'], LogConfig: {Type: 'none'}, Memory: 6 * MiB, NanoCpus: 100000000, PidsLimit: 1};
  const create = {...clone(cfg), HostConfig: clone(host)};
  const inspect = {Id: record.proof.fenceRef, Name: '/' + name, Image: '', Path: '/', Args: [], Mounts: [],
    State: {Status: 'created', Running: false, Pid: 0}, Config: clone(cfg), HostConfig: clone(host)};
  return {record, name, create, inspect};
}
test('validateFence accepts exact image-free inert create and matching fresh inspect shape', () => {
  const f = fenceFixture();
  assert.deepEqual(validateFence(f.create, f), f.create.Labels);
  assert.deepEqual(validateFence(f.inspect, {...f, inspected: true}), f.create.Labels);
});
for (const [name, mutate] of [
  ['wrong name', f => {f.name += '-foreign';}],
  ['wrong binding digest', f => {f.create.Labels['luci.fence-binding'] = '0'.repeat(64);}],
  ['wrong intent', f => {f.create.Labels['luci.fence-intent'] = '0'.repeat(32);}],
  ['foreign instance', f => {f.create.Labels['luci.instance'] = 'other-instance';}],
  ['unexpected label', f => {f.create.Labels.extra = 'value';}],
  ['image', f => {f.create.Image = 'sha256:' + '1'.repeat(64);}],
  ['executable command', f => {f.create.Cmd = ['sh'];}],
  ['environment', f => {f.create.Env = ['FAKE=value'];}],
  ['entrypoint', f => {f.create.Entrypoint = ['sh'];}],
  ['network enabled', f => {f.create.NetworkDisabled = false;}],
  ['writable rootfs', f => {f.create.HostConfig.ReadonlyRootfs = false;}],
  ['bind mount', f => {f.create.HostConfig.Binds = ['/tmp:/tmp'];}],
  ['privileged mode', f => {f.create.HostConfig.Privileged = true;}],
  ['automatic removal', f => {f.create.HostConfig.AutoRemove = true;}],
  ['restart policy', f => {f.create.HostConfig.RestartPolicy.Name = 'always';}],
  ['host IPC', f => {f.create.HostConfig.IpcMode = 'host';}],
  ['added capability', f => {f.create.HostConfig.CapAdd = ['SYS_ADMIN'];}],
  ['memory changed', f => {f.create.HostConfig.Memory = 1024 * MiB;}],
  ['CPU changed', f => {f.create.HostConfig.NanoCpus = 1000000000;}],
  ['PID limit changed', f => {f.create.HostConfig.PidsLimit = 2;}],
]) test('validateFence rejects ' + name, () => {const f = fenceFixture(); mutate(f); assert.throws(() => validateFence(f.create, f));});
for (const [name, mutate] of [
  ['different immutable ID', f => {f.inspect.Id = '1'.repeat(64);}],
  ['renamed marker', f => {f.inspect.Name += '-renamed';}],
  ['runnable state', f => {f.inspect.State.Status = 'running'; f.inspect.State.Running = true;}],
  ['nonzero PID', f => {f.inspect.State.Pid = 99;}],
  ['actual image', f => {f.inspect.Image = 'sha256:' + '1'.repeat(64);}],
  ['actual mount', f => {f.inspect.Mounts = [{Type: 'bind'}];}],
  ['different executable path', f => {f.inspect.Path = '/bin/sh';}],
]) test('validateFence inspect rejects ' + name, () => {const f = fenceFixture(); mutate(f); assert.throws(() => validateFence(f.inspect, {...f, inspected: true}));});

function capacityFixture() {
  const now = Date.now(), observedAt = new Date(now - 100).toISOString(), ledger = retirement().before;
  const view = {policy: {headroomPercent: 25, minHeadroomMiB: 2048, diskReserveMiB: 1024},
    resources: {host: {status: 'ready', observedAt, memory: {effectiveTotalBytes: 16 * GiB, effectiveAvailableBytes: 12 * GiB}, cpu: {effectiveMillis: 8000}},
      execution: {status: 'ready', observedAt, totalMemoryBytes: 16 * GiB, cpuMillis: 8000},
      reservations: {observedAt, count: 2, memoryMiB: 2048, cpuMillis: 2000}}};
  return {view, options: {ledger, now, engine: {MemoryLimit: true, SwapLimit: true, CpuCfsPeriod: true, CpuCfsQuota: true, PidsLimit: true, Driver: 'vfs'},
    rootFreeBytes: 16 * GiB, imageSize: GiB, requireCreate: true}};
}

for(const cached of [null,{count:0,memoryMiB:0,cpuMillis:0,observedAt:'2025-01-01T00:00:00Z'}])test('diagnostic-only owner cache cannot replace exact shared team charges '+JSON.stringify(cached),()=>{
  const f=capacityFixture();f.view.resources.reservations=cached;f.options.requireReservationMatch=false;const unchanged=clone(f);
  assert.deepEqual(assertCapacity(f.view,f.options),{count:2,memoryMiB:2048,cpuMillis:2000});assert.deepEqual(f,unchanged);
});
test('diagnostic-only reservations still enforce fresh host capacity and full ledger charges',()=>{
  const f=capacityFixture();f.options.requireReservationMatch=false;f.view.resources.reservations=null;
  f.view.resources.host.observedAt='2025-01-01T00:00:00Z';assert.throws(()=>assertCapacity(f.view,f.options));
  const bad=capacityFixture();bad.options.requireReservationMatch=false;bad.options.ledger.checks[0].resources.MemoryMiB=512;assert.throws(()=>assertCapacity(bad.view,bad.options));
});
function prospectiveFixture() {
  const f = capacityFixture();
  // Keep the real-shaped current full ledger at one charge. The prospective
  // demand is a separate arithmetic input; no synthetic second lease is made.
  f.options.ledger.checks = f.options.ledger.checks.filter(row => row.identity.checkId !== 'check-' + '2'.repeat(24));
  Object.assign(f.view.resources.reservations, {count: 1, memoryMiB: 1024, cpuMillis: 1000});
  return f;
}
for (const [name, mutate] of [
  ['host RAM', f => {f.view.resources.host.memory.effectiveAvailableBytes = 5 * GiB;}],
  ['execution RAM', f => {f.view.resources.execution.totalMemoryBytes = 3 * GiB;}],
  ['host CPU', f => {f.view.resources.host.cpu.effectiveMillis = 2000;}],
  ['execution CPU', f => {f.view.resources.execution.cpuMillis = 2000;}],
]) test('assertComputeEnvelope refuses prospective second-check demand when only current ' + name + ' fits', () => {
  const f = prospectiveFixture(); mutate(f);
  const unchanged = clone(f), current = assertCapacity(f.view, f.options);
  assert.deepEqual(current, {count: 1, memoryMiB: 1024, cpuMillis: 1000});
  assert.throws(() => assertComputeEnvelope(f.view, {memoryMiB: current.memoryMiB + 1024, cpuMillis: current.cpuMillis + 1000}));
  assert.deepEqual(f, unchanged, 'Prospective admission must not manufacture or filter a current ledger claim');
});
test('assertComputeEnvelope admits prospective second check at the exact combined RAM boundary', () => {
  const f = prospectiveFixture();
  f.view.resources.host.memory.effectiveAvailableBytes = 6 * GiB; // 4 GiB headroom plus two 1 GiB checks.
  f.view.resources.execution.totalMemoryBytes = 4 * GiB; // 2 GiB minimum headroom plus two checks.
  const current = assertCapacity(f.view, f.options), prospective = {memoryMiB: current.memoryMiB + 1024, cpuMillis: current.cpuMillis + 1000};
  assert.doesNotThrow(() => assertComputeEnvelope(f.view, prospective));
  f.view.resources.host.memory.effectiveAvailableBytes--;
  assert.throws(() => assertComputeEnvelope(f.view, prospective));
});
test('assertComputeEnvelope preserves the minimum 2 GiB headroom on a smaller host', () => {
  const f = prospectiveFixture();
  f.view.resources.host.memory.effectiveTotalBytes = 4 * GiB;
  f.view.resources.host.memory.effectiveAvailableBytes = 3 * GiB;
  const current = assertCapacity(f.view, f.options);
  assert.throws(() => assertComputeEnvelope(f.view, {memoryMiB: current.memoryMiB + 1024, cpuMillis: current.cpuMillis + 1000}));
});
test('assertComputeEnvelope keeps the exact CPU headroom boundary', () => {
  const f = prospectiveFixture(); f.view.resources.host.cpu.effectiveMillis = 4000;
  assert.doesNotThrow(() => assertComputeEnvelope(f.view, {memoryMiB: 1024, cpuMillis: 3000}));
  assert.throws(() => assertComputeEnvelope(f.view, {memoryMiB: 1024, cpuMillis: 3001}));
});
for (const demand of [
  {memoryMiB: -1, cpuMillis: 1000}, {memoryMiB: 1024, cpuMillis: -1},
  {memoryMiB: 1024.5, cpuMillis: 1000}, {memoryMiB: 1024, cpuMillis: Infinity},
]) test('assertComputeEnvelope rejects invalid demand ' + JSON.stringify(demand), () => {
  const f = prospectiveFixture(); assert.throws(() => assertComputeEnvelope(f.view, demand));
});
test('assertCapacity counts both live leases and excludes completed lease history', () => {
  const f = capacityFixture(); assert.deepEqual(assertCapacity(f.view, f.options), {count: 2, memoryMiB: 2048, cpuMillis: 2000});
});
test('assertCapacity reflects precisely one charge after retirement', () => {
  const f = capacityFixture(); f.options.ledger = retirement().after;
  Object.assign(f.view.resources.reservations, {count: 1, memoryMiB: 1024, cpuMillis: 1000});
  assert.deepEqual(assertCapacity(f.view, f.options), {count: 1, memoryMiB: 1024, cpuMillis: 1000});
});
test('assertCapacity accepts an old unchanged reservation snapshot from the last ledger mutation', () => {
  const f = capacityFixture();
  // The API samples host/execution freshness separately. Reservation observedAt
  // is the ledger mutation time, so a held creating claim does not age out.
  f.view.resources.reservations.observedAt = new Date(f.options.now - 6000).toISOString();
  assert.deepEqual(assertCapacity(f.view, f.options), {count: 2, memoryMiB: 2048, cpuMillis: 2000});
});
for (const [name, mutate] of [
  ['filtered one-lease reservation totals', f => {Object.assign(f.view.resources.reservations, {count: 1, memoryMiB: 1024, cpuMillis: 1000});}],
  ['host memory sufficient for only one lease', f => {f.view.resources.host.memory.effectiveAvailableBytes = 5 * GiB;}],
  ['execution memory sufficient for only one lease', f => {f.view.resources.execution.totalMemoryBytes = 3 * GiB;}],
  ['CPU sufficient for only one lease', f => {f.view.resources.host.cpu.effectiveMillis = 2000;}],
  ['disk sufficient for only one pending clone', f => {f.options.ledger.checks.filter(row => row.state !== 'finished').forEach(row => {row.createDiskBytes = 3 * GiB;}); f.options.rootFreeBytes = 6 * GiB;}],
  ['stale host observation', f => {f.view.resources.host.observedAt = new Date(f.options.now - 6000).toISOString();}],
  ['stale execution observation', f => {f.view.resources.execution.observedAt = new Date(f.options.now - 6000).toISOString();}],
  ['fresh totals that predate the current allocation', f => {f.options.ledger.checks[0].updatedAt = new Date(f.options.now - 50).toISOString();}],
  ['unavailable host observation', f => {f.view.resources.host.status = 'unavailable';}],
  ['weakened headroom', f => {f.view.policy.headroomPercent = 0;}],
  ['missing engine resource control', f => {f.options.engine.PidsLimit = false;}],
  ['unknown storage driver', f => {f.options.engine.Driver = 'unknown';}],
  ['missing image size', f => {f.options.imageSize = 0;}],
  ['unaccounted worker lease', f => {f.options.ledger.leases.push({id: 'worker-allocation'});}],
]) test('assertCapacity rejects ' + name, () => {const f = capacityFixture(); mutate(f); assert.throws(() => assertCapacity(f.view, f.options));});

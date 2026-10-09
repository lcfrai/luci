// Pure synthetic guards only. No Docker, services, browser, models, or compiler.
import test from 'node:test';
import assert from 'node:assert/strict';
import {createHash} from 'node:crypto';
import {mkdtempSync, mkdirSync, writeFileSync, rmSync, symlinkSync} from 'node:fs';
import {tmpdir} from 'node:os';
import {join} from 'node:path';
import {checkCommandDigest, validateCheckCreate} from './check-create-guard.mjs';
import {assertCapacity} from './fixture-guards.mjs';

const GiB = 1024 ** 3;
const sha = value => createHash('sha256').update(value).digest('hex');
const command = {label: 'node', executable: 'node', args: ['verify.mjs', 'final']};

function fixture(t, peer = false) {
  const root = mkdtempSync(join(tmpdir(), 'recovery-check-guard-'));
  t.after(() => rmSync(root, {recursive: true, force: true}));
  const now = Date.now();
  const scopeDigest = sha(join(root, 'state', 'control-plane', 'conversations'));
  const imageID = 'sha256:' + 'f'.repeat(64);
  const engineID = 'engine-fixture';
  const instance = '11111111-1111-4111-8111-111111111111';
  function project(suffix, hex) {
    const projectRoot = join(root, 'projects', suffix);
    const primary = join(projectRoot, 'primary');
    mkdirSync(primary, {recursive: true});
    writeFileSync(join(primary, 'verify.mjs'), "process.stdout.write('RECOVERY_CHECK_OK final\\n');\n");
    const definition = {id: 'p-' + suffix, spec: {revision: 3, digest: 'a'.repeat(64)}, design: {revision: 4, digest: 'b'.repeat(64)}};
    const check = {id: 'check-' + hex.repeat(24), projectId: definition.id, execution: 'docker',
      memoryMiB: 1024, cpuMillis: 1000, commit: 'd'.repeat(40), sourceDigest: 'e'.repeat(64),
      dirty: false, sourceChanged: false, requirementIds: ['AC-recovery'],
      requirementSnapshots: [{id: 'AC-recovery', title: 'Preserve exact recovery', acceptance: 'Retain the unrelated allocation'}],
      specificationRevision: 3, specificationDigest: definition.spec.digest,
      designPlanRevision: 4, designPlanDigest: definition.design.digest,
      status: 'running', revision: 4, resourcesHeld: true,
      commands: [{...structuredClone(command), allocationId: 'checkexec-' + hex.repeat(24),
        status: 'running', output: '', truncated: false, cleanupConfirmed: false}]};
    const actor = {userId: 'fixture-owner', projectId: definition.id};
    const options = {fixture: root, projectRoot, primary, project: definition, check, actor,
      commit: check.commit, sourceDigest: check.sourceDigest, imageID, uid: 1000, gid: 1000,
      engineID, instance, scopeDigest, imageVirtualSize: 2 * GiB, storageDriver: 'vfs', now,
      name: 'luci-check-' + check.commands[0].allocationId};
    const identity = {protocol: 1, engineId: engineID, instanceId: instance,
      allocationId: check.commands[0].allocationId, checkId: check.id, commandIndex: 0,
      scopeDigest, projectId: definition.id, commandDigest: ''};
    identity.commandDigest = checkCommandDigest({...options, identity});
    const timestamp = new Date(now - 100).toISOString();
    const lease = {id: identity.allocationId, scope: scopeDigest, identity,
      actor: structuredClone(actor), resources: {CPUMillis: 1000, MemoryMiB: 1024},
      createDiskBytes: 4 * GiB, state: 'creating', createdAt: timestamp, updatedAt: timestamp};
    const input = {Image: imageID, Healthcheck: {Test: ['NONE']}, User: '1000:1000',
      WorkingDir: '/workspace', Entrypoint: ['node'], Cmd: ['verify.mjs', 'final'],
      AttachStdout: true, AttachStderr: true,
      Labels: {'luci.check': 'true', 'luci.check-protocol': '1', 'luci.instance': instance,
        'luci.check-engine': engineID, 'luci.check-allocation': identity.allocationId,
        'luci.check-id': check.id, 'luci.check-command-index': '0', 'luci.check-scope': scopeDigest,
        'luci.check-project': definition.id, 'luci.check-command-digest': identity.commandDigest,
        'luci.check-source-digest': check.sourceDigest, 'luci.check-image': imageID},
      Env: ['HOME=/tmp', 'GOCACHE=/tmp/go-cache', 'GOMODCACHE=/tmp/go-mod', 'npm_config_cache=/tmp/npm-cache'],
      HostConfig: {AutoRemove: false, Init: true, ReadonlyRootfs: true,
        Memory: GiB, MemorySwap: GiB, NanoCpus: 1e9, PidsLimit: 256,
        CapDrop: ['ALL'], SecurityOpt: ['no-new-privileges:true'],
        Tmpfs: {'/tmp': 'rw,nosuid,nodev,size=1g'}, RestartPolicy: {Name: 'no'},
        Mounts: [{Type: 'bind', Source: primary, Target: '/workspace'}]}};
    return {input, options, lease};
  }
  const target = project('target', '1');
  const witness = peer ? project('witness', '2') : undefined;
  const ledger = {version: 1, leases: [], checks: [target.lease, ...(witness ? [witness.lease] : [])]};
  const authorizedChecks = new Set(ledger.checks.map(row => row.identity.checkId));
  for (const row of [target, witness].filter(Boolean)) Object.assign(row.options, {ledger, authorizedChecks});
  return {...target, witness};
}

function capacity(f) {
  const observedAt = new Date(f.options.now - 100).toISOString();
  const count = f.options.ledger.checks.filter(row => row.state !== 'finished').length;
  return {view: {policy: {headroomPercent: 25, minHeadroomMiB: 2048, diskReserveMiB: 1024},
    resources: {host: {status: 'ready', observedAt,
      memory: {effectiveTotalBytes: 16 * GiB, effectiveAvailableBytes: (4 + count) * GiB},
      cpu: {effectiveMillis: 4000, utilizationPercent: 20}},
      execution: {status: 'ready', observedAt, totalMemoryBytes: 16 * GiB, cpuMillis: 4000},
      reservations: {observedAt, count, memoryMiB: count * 1024, cpuMillis: count * 1000}}},
    options: {ledger: f.options.ledger, now: f.options.now, rootFreeBytes: Math.max(6, 4 * count + 1) * GiB,
      imageSize: f.options.imageVirtualSize, requireCreate: true,
      engine: {ID: f.options.engineID, Driver: 'vfs', MemoryLimit: true, SwapLimit: true,
        CpuCfsPeriod: true, CpuCfsQuota: true, PidsLimit: true}}};
}

test('command digest retains the pinned Go contract and HTML escaping golden', () => {
  const identity = {allocationId: 'checkexec-' + '9'.repeat(24), checkId: 'check-' + 'c'.repeat(24),
    scopeDigest: 'a'.repeat(64), projectId: 'p-fixture'};
  assert.equal(checkCommandDigest({identity, primary: '/fixture/projects/p<&>',
    sourceDigest: 'e'.repeat(64), uid: 1000, gid: 1000}),
  '2706d0b5030dea1d931a631cdd695e9501f2ac7cc022cbea6d5ad07de8d28b29');
});

test('admits the exact check and leaves its input and full ledger unchanged', t => {
  const f = fixture(t);
  const before = structuredClone({input: f.input, options: f.options});
  const admitted = validateCheckCreate(f.input, f.options);
  assert.equal(admitted.checkId, f.options.check.id);
  assert.equal(admitted.allocationId, f.lease.id);
  assert.equal(admitted.createDiskBytes, 4 * GiB);
  assert.deepEqual({input: f.input, options: f.options}, before);
});

test('both separately consented projects remain in the full ledger and consume capacity once each', t => {
  const f = fixture(t, true);
  validateCheckCreate(f.input, f.options);
  validateCheckCreate(f.witness.input, f.witness.options);
  const c = capacity(f);
  assert.deepEqual(assertCapacity(c.view, c.options), {count: 2, memoryMiB: 2048, cpuMillis: 2000});
  assert.equal(c.options.rootFreeBytes, 9 * GiB);
  const filtered = {...c.options, ledger: {...c.options.ledger, checks: [f.lease]}};
  assert.throws(() => assertCapacity(c.view, filtered), 'Omitting the witness must disagree with the real reservation view');
});

for (const [name, mutate] of Object.entries({
  'wrong Docker memory': f => { f.input.HostConfig.Memory = 2 * GiB; },
  'wrong Docker CPU': f => { f.input.HostConfig.NanoCpus = 2e9; },
  'unlimited swap': f => { f.input.HostConfig.MemorySwap = -1; },
  'wrong receipt memory': f => { f.options.check.memoryMiB = 2048; },
  'wrong ledger resources': f => { f.lease.resources.MemoryMiB = 2048; },
  'wrong allocation owner': f => { f.lease.actor.userId = 'foreign-user'; },
  'wrong owner project': f => { f.lease.actor.projectId = 'p-foreign'; },
  'wrong outer scope': f => { f.lease.scope = '0'.repeat(64); },
  'wrong identity scope': f => { f.lease.identity.scopeDigest = '0'.repeat(64); },
  'wrong engine': f => { f.lease.identity.engineId = 'foreign-engine'; },
  'wrong instance': f => { f.lease.identity.instanceId = '22222222-2222-4222-8222-222222222222'; },
  'wrong check identity': f => { f.lease.identity.checkId = 'check-' + '3'.repeat(24); },
  'wrong allocation identity': f => { f.lease.identity.allocationId = 'checkexec-' + '3'.repeat(24); },
  'wrong command index': f => { f.lease.identity.commandIndex = 1; },
  'wrong command digest': f => { f.lease.identity.commandDigest = '0'.repeat(64); },
  'wrong label identity': f => { f.input.Labels['luci.check-id'] = 'check-' + '3'.repeat(24); },
  'missing product consent': f => { f.options.authorizedChecks.clear(); },
  'receipt without held resources': f => { f.options.check.resourcesHeld = false; },
  'duplicate selected allocation': f => { f.options.ledger.checks.push(structuredClone(f.lease)); },
  'extra worker claim': f => { f.options.ledger.leases.push({id: 'worker-unexpected'}); },
  'third active check': f => { f.options.ledger.checks.push(structuredClone(f.lease), structuredClone(f.lease)); },
  'unacknowledged disk demand removed': f => { f.lease.createDiskBytes = 0; },
  'unverified storage driver': f => { f.options.storageDriver = 'unknown'; },
  'created lease reused for Create': f => { f.lease.state = 'created'; f.lease.ref = '8'.repeat(64); },
  'future ledger timestamp': f => { f.lease.updatedAt = new Date(f.options.now + 1).toISOString(); },
  'extra command': f => { f.input.Cmd.push('unreviewed'); },
  'extra environment': f => { f.input.Env.push('LD_PRELOAD=/tmp/foreign'); },
  'extra writable mount': f => { f.input.HostConfig.Mounts.push({Type: 'bind', Source: '/tmp', Target: '/extra'}); },
  'host network': f => { f.input.HostConfig.NetworkMode = 'host'; },
  'privileged container': f => { f.input.HostConfig.Privileged = true; },
  'wrong source commit': f => { f.options.check.commit = '0'.repeat(40); },
})) test('refuses ' + name, t => {
  const f = fixture(t); mutate(f);
  assert.throws(() => validateCheckCreate(f.input, f.options));
});

test('same-target symlink source is refused after its digest is rebound', t => {
  const f = fixture(t);
  const alias = join(f.options.projectRoot, 'alias');
  symlinkSync(f.options.primary, alias);
  f.options.primary = alias;
  f.input.HostConfig.Mounts[0].Source = alias;
  f.lease.identity.commandDigest = checkCommandDigest({...f.options, identity: f.lease.identity});
  f.input.Labels['luci.check-command-digest'] = f.lease.identity.commandDigest;
  assert.throws(() => validateCheckCreate(f.input, f.options));
});

for (const [name, mutate] of Object.entries({
  'foreign actor': f => { f.witness.lease.actor.userId = 'foreign-user'; },
  'actor project mismatch': f => { f.witness.lease.actor.projectId = 'p-foreign'; },
  'identity scope mismatch': f => { f.witness.lease.identity.scopeDigest = '0'.repeat(64); },
  'allocation ID mismatch': f => { f.witness.lease.identity.allocationId = 'checkexec-' + '3'.repeat(24); },
  'unsupported protocol': f => { f.witness.lease.identity.protocol = 2; },
  'unexpected command index': f => { f.witness.lease.identity.commandIndex = 1; },
  'wrong resources': f => { f.witness.lease.resources.CPUMillis = 2000; },
  'missing pending image-copy demand': f => { delete f.witness.lease.createDiskBytes; },
  'zero pending image-copy demand': f => { f.witness.lease.createDiskBytes = 0; },
  'negative pending image-copy demand': f => { f.witness.lease.createDiskBytes = -4 * GiB; },
  'unconsented identity': f => { f.witness.lease.identity.checkId = 'check-' + '3'.repeat(24); },
})) test('full ledger refuses a witness with ' + name, t => {
  const f = fixture(t, true); mutate(f);
  assert.throws(() => validateCheckCreate(f.input, f.options));
});

for (const [name, mutate] of Object.entries({
  'stale host sample': c => { c.view.resources.host.observedAt = new Date(c.options.now - 5001).toISOString(); },
  'stale execution sample': c => { c.view.resources.execution.observedAt = new Date(c.options.now - 5001).toISOString(); },
  'future reservation sample': c => { c.view.resources.reservations.observedAt = new Date(c.options.now + 1).toISOString(); },
  'missing witness count': c => { c.view.resources.reservations.count = 1; },
  'missing witness memory': c => { c.view.resources.reservations.memoryMiB = 1024; },
  'missing witness CPU': c => { c.view.resources.reservations.cpuMillis = 1000; },
  'one byte below shared host headroom': c => { c.view.resources.host.memory.effectiveAvailableBytes--; },
  'insufficient shared CPU': c => { c.view.resources.host.cpu.effectiveMillis = 2000; },
  'one byte below both pending image copies plus disk reserve': c => { c.options.rootFreeBytes--; },
  'unenforced memory limits': c => { c.options.engine.MemoryLimit = false; },
  'unenforced swap limits': c => { c.options.engine.SwapLimit = false; },
  'unexpected worker charge': c => { c.options.ledger.leases.push({id: 'worker-unexpected'}); },
})) test('capacity guard refuses ' + name, t => {
  const f = fixture(t, true), c = capacity(f); mutate(c);
  assert.throws(() => assertCapacity(c.view, c.options));
});

test('unchanged reservation snapshot retains its old last-mutation timestamp with fresh host observations', t => {
  const f = fixture(t, true), c = capacity(f);
  const observedAt = new Date(c.options.now - 60000).toISOString();
  c.view.resources.reservations.observedAt = observedAt;
  for (const lease of c.options.ledger.checks) {
    lease.createdAt = new Date(c.options.now - 61000).toISOString();
    lease.updatedAt = observedAt;
  }
  assert.deepEqual(assertCapacity(c.view, c.options), {count: 2, memoryMiB: 2048, cpuMillis: 2000});
});

test('current-public visibility accepts exact fresh charged and released totals without mutation', async () => {
  const {assertPublicReservationTotals}=await import('./fixture-guards.mjs');const now=Date.now();
  for(const count of [0,1,2]){const expected={count,memoryMiB:count*1024,cpuMillis:count*1000};const row={status:'ready',observedAt:new Date(now).toISOString(),...expected};const before=structuredClone(row);
    assert.deepEqual(assertPublicReservationTotals(row,expected,{now}),row);assert.deepEqual(row,before);}
});
for(const [name,change] of Object.entries({
  absent:()=>null,unavailable:()=>({status:'unavailable'}),legacy:r=>{delete r.status;return r;},'false-zero':r=>({...r,count:0,memoryMiB:0,cpuMillis:0}),
  stale:r=>({...r,observedAt:new Date(1000).toISOString()}),future:r=>({...r,observedAt:new Date(Date.now()+60000).toISOString()}),
  'invalid-time':r=>({...r,observedAt:'invalid'}),'missing-cpu':r=>{delete r.cpuMillis;return r;},'negative-memory':r=>({...r,memoryMiB:-1024}),
  'fractional-count':r=>({...r,count:1.5}),'string-count':r=>({...r,count:'1'}),
}))test('current-public visibility refuses '+name,async()=>{
  const {assertPublicReservationTotals}=await import('./fixture-guards.mjs');const expected={count:1,memoryMiB:1024,cpuMillis:1000};const row=change({status:'ready',observedAt:new Date().toISOString(),...expected});
  assert.throws(()=>assertPublicReservationTotals(row,expected));
});
test('installed CLI accepts explicit public visibility flag and preserves baseline help without runtime',async()=>{
  const {main}=await import('./installed-recovery.mjs');const original=console.log;const logs=[];console.log=value=>logs.push(value);
  try{await main(['--help']);await main(['--help','--require-public-reservations']);await assert.rejects(main(['--help','--require-public-reservations','--require-public-reservations']),/Duplicate adapter argument/);}finally{console.log=original;}
  assert.equal(logs.length,2);for(const text of logs)assert.match(text,/require-public-reservations/);
});

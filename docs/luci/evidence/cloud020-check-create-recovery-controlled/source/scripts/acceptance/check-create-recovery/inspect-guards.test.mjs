// Controlled DTO tests only. No filesystem fixture, Docker, network, or process.
// Prepared during the exclusive performance window; execution remains UNRUN.
import test from 'node:test';
import assert from 'node:assert/strict';
import {assertOriginalInspect} from './fixture-guards.mjs';

function fixture() {
  const id = '1'.repeat(64), imageID = 'sha256:' + '2'.repeat(64);
  const allocationId = 'checkexec-' + '3'.repeat(24), name = 'luci-check-' + allocationId;
  const source = '/synthetic/private-recovery/projects/p-target/primary';
  const labels = {'luci.check': 'true', 'luci.check-protocol': '1',
    'luci.instance': '11111111-1111-4111-8111-111111111111', 'luci.check-engine': 'fixture-engine',
    'luci.check-allocation': allocationId, 'luci.check-id': 'check-' + '4'.repeat(24),
    'luci.check-command-index': '0', 'luci.check-scope': '5'.repeat(64),
    'luci.check-project': 'p-target', 'luci.check-command-digest': '6'.repeat(64),
    'luci.check-source-digest': '7'.repeat(64), 'luci.check-image': imageID};
  const guardedCreate = {Image: imageID, User: '1000:1000', WorkingDir: '/workspace',
    Entrypoint: ['node'], Cmd: ['verify.mjs', 'final'], AttachStdout: true, AttachStderr: true,
    Healthcheck: {Test: ['NONE']}, Labels: labels,
    Env: ['HOME=/tmp', 'GOCACHE=/tmp/go-cache', 'GOMODCACHE=/tmp/go-mod', 'npm_config_cache=/tmp/npm-cache'],
    HostConfig: {AutoRemove: false, Init: true, ReadonlyRootfs: true,
      Memory: 1024 ** 3, MemorySwap: 1024 ** 3, NanoCpus: 1e9, PidsLimit: 256,
      CapDrop: ['ALL'], SecurityOpt: ['no-new-privileges:true'],
      Tmpfs: {'/tmp': 'rw,nosuid,nodev,size=1g'}, RestartPolicy: {Name: 'no'},
      Mounts: [{Type: 'bind', Source: source, Target: '/workspace'}]}};
  // Docker inherits image variables, then overrides matching keys from Create.
  const imageEnv = ['PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin',
    'HOME=/home/luci', 'NODE_VERSION=22.16.0', 'LANG=C.UTF-8', 'FIXTURE_TOKENIZATION=a=b'];
  const config = structuredClone(guardedCreate);
  delete config.HostConfig;
  config.Env = ['PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin',
    'HOME=/tmp', 'NODE_VERSION=22.16.0', 'LANG=C.UTF-8', 'FIXTURE_TOKENIZATION=a=b',
    'GOCACHE=/tmp/go-cache', 'GOMODCACHE=/tmp/go-mod', 'npm_config_cache=/tmp/npm-cache'];
  const input = {Id: id, Name: '/' + name, Image: imageID, Config: config,
    HostConfig: {...structuredClone(guardedCreate.HostConfig), NetworkMode: 'default', IpcMode: 'private',
      Runtime: 'runc', CgroupnsMode: 'private', LogConfig: {Type: 'json-file', Config: {}}},
    Mounts: [{Type: 'bind', Source: source, Destination: '/workspace', Mode: '', RW: true, Propagation: 'rprivate'}],
    State: {Status: 'created', Running: false, Paused: false, Restarting: false, Dead: false,
      Pid: 0, ExitCode: 0, Error: '', StartedAt: '0001-01-01T00:00:00Z', FinishedAt: '0001-01-01T00:00:00Z'},
    Path: 'node', Args: ['verify.mjs', 'final']};
  return {input, options: {id, name, imageID, guardedCreate, imageEnv}};
}

test('accepts exact unstarted original with daemon defaults and merged image/Create environment', () => {
  const f = fixture(), before = structuredClone(f);
  assertOriginalInspect(f.input, f.options);
  assert.deepEqual(f, before, 'Inspection validation must not rewrite its evidence');
});

test('accepts equivalent environment ordering while retaining embedded equals signs', () => {
  const f = fixture();
  f.input.Config.Env.reverse();
  assertOriginalInspect(f.input, f.options);
});

for (const [network, ipc] of [['bridge', 'private'], ['', '']]) {
  test('accepts source-supported daemon modes ' + JSON.stringify({network, ipc}), () => {
    const f = fixture(); f.input.HostConfig.NetworkMode = network; f.input.HostConfig.IpcMode = ipc;
    assertOriginalInspect(f.input, f.options);
  });
}

for (const [name, mutate] of Object.entries({
  'different immutable container': f => { f.input.Id = '9'.repeat(64); },
  'different logical name': f => { f.input.Name += '-other'; },
  'different image bytes': f => { f.input.Image = 'sha256:' + '9'.repeat(64); },
  'mutable configured image tag': f => { f.input.Config.Image = 'fixture:latest'; },
  'foreign check identity': f => { f.input.Config.Labels['luci.check-id'] = 'check-' + '9'.repeat(24); },
  'changed source digest': f => { f.input.Config.Labels['luci.check-source-digest'] = '9'.repeat(64); },
  'different user': f => { f.input.Config.User = '0:0'; },
  'different working directory': f => { f.input.Config.WorkingDir = '/'; },
  'shell entrypoint': f => { f.input.Config.Entrypoint = ['sh', '-c']; },
  'different command arguments': f => { f.input.Config.Cmd = ['verify.mjs', 'other']; },
  'enabled healthcheck': f => { f.input.Config.Healthcheck = {Test: ['CMD', 'touch', '/workspace/extra']}; },
  'interactive input': f => { f.input.Config.OpenStdin = true; },
  'missing captured output': f => { f.input.Config.AttachStdout = false; },
  'writable root filesystem': f => { f.input.HostConfig.ReadonlyRootfs = false; },
  'privileged mode': f => { f.input.HostConfig.Privileged = true; },
  'added capability': f => { f.input.HostConfig.CapAdd = ['SYS_ADMIN']; },
  'missing capability drop': f => { f.input.HostConfig.CapDrop = []; },
  'missing no-new-privileges': f => { f.input.HostConfig.SecurityOpt = []; },
  'host network': f => { f.input.HostConfig.NetworkMode = 'host'; },
  'host IPC': f => { f.input.HostConfig.IpcMode = 'host'; },
  'host process namespace': f => { f.input.HostConfig.PidMode = 'host'; },
  'extra device': f => { f.input.HostConfig.Devices = [{PathOnHost: '/dev/fixture', PathInContainer: '/dev/fixture', CgroupPermissions: 'rwm'}]; },
  'automatic removal': f => { f.input.HostConfig.AutoRemove = true; },
  'automatic restart': f => { f.input.HostConfig.RestartPolicy.Name = 'always'; },
  'published port': f => { f.input.HostConfig.PortBindings = {'8080/tcp': [{HostPort: '8080'}]}; },
  'excess memory': f => { f.input.HostConfig.Memory = 2 * 1024 ** 3; },
  'unlimited swap': f => { f.input.HostConfig.MemorySwap = -1; },
  'excess CPU': f => { f.input.HostConfig.NanoCpus = 2e9; },
  'excess process limit': f => { f.input.HostConfig.PidsLimit = 512; },
  'unbounded temporary filesystem': f => { f.input.HostConfig.Tmpfs['/tmp'] = 'rw'; },
  'extra legacy source bind': f => { f.input.HostConfig.Binds = ['/foreign:/extra']; },
  'different configured source mount': f => { f.input.HostConfig.Mounts[0].Source = '/foreign'; },
  'different observed source mount': f => { f.input.Mounts[0].Source = '/foreign'; },
  'different observed mount target': f => { f.input.Mounts[0].Destination = '/other'; },
  'volume in place of source bind': f => { f.input.Mounts[0].Type = 'volume'; },
  'read-only project mount': f => { f.input.Mounts[0].RW = false; },
  'shared mount propagation': f => { f.input.Mounts[0].Propagation = 'rshared'; },
  'extra observed source mount': f => { f.input.Mounts.push({Type: 'bind', Source: '/foreign', Destination: '/extra', RW: true, Propagation: 'rprivate'}); },
  'unexpected environment key': f => { f.input.Config.Env.push('LD_PRELOAD=/tmp/foreign.so'); },
  'unexpected prototype-named environment key': f => { f.input.Config.Env.push('__proto__=foreign'); },
  'missing inherited image variable': f => { f.input.Config.Env = f.input.Config.Env.filter(value => !value.startsWith('NODE_VERSION=')); },
  'changed inherited image PATH': f => { f.input.Config.Env = f.input.Config.Env.map(value => value.startsWith('PATH=') ? 'PATH=/foreign' : value); },
  'image HOME overriding guarded HOME': f => { f.input.Config.Env = f.input.Config.Env.map(value => value === 'HOME=/tmp' ? 'HOME=/home/luci' : value); },
  'missing guarded cache variable': f => { f.input.Config.Env = f.input.Config.Env.filter(value => !value.startsWith('GOCACHE=')); },
  'duplicate environment key': f => { f.input.Config.Env.push('HOME=/tmp'); },
  'malformed environment entry': f => { f.input.Config.Env.push('MALFORMED'); },
  'running original': f => { f.input.State.Status = 'running'; f.input.State.Running = true; f.input.State.Pid = 1234; },
  'hidden nonzero process': f => { f.input.State.Pid = 1234; },
  'restarting original': f => { f.input.State.Restarting = true; },
  'different inspected executable': f => { f.input.Path = 'sh'; },
  'different inspected arguments': f => { f.input.Args = ['verify.mjs', 'extra']; },
})) test('refuses original inspect with ' + name, () => {
  const f = fixture(); mutate(f);
  assert.throws(() => assertOriginalInspect(f.input, f.options));
});

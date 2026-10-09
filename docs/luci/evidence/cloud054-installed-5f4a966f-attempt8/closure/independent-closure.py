"""Read-only, explicitly pinned post-completion observation; never performs cleanup."""
from pathlib import Path
import argparse, datetime, hashlib, json, os, re, stat, subprocess

p = argparse.ArgumentParser()
for key in ('base', 'fixture-parent', 'repo'):
    p.add_argument('--' + key, type=Path, required=True)
for key in ('expected-source', 'expected-image'):
    p.add_argument('--' + key, required=True)
a = p.parse_args()
assert re.fullmatch(r'[a-f0-9]{40}', a.expected_source)
assert re.fullmatch(r'sha256:[a-f0-9]{64}', a.expected_image)
base = a.base.resolve(strict=True)
out = base / 'independent-closure.json'
assert not out.exists()

def sha(path):
    with path.open('rb') as f:
        return hashlib.file_digest(f, 'sha256').hexdigest()

def snapshot(directory):
    files, links = {}, []
    for path in sorted(directory.rglob('*')):
        info = path.lstat()
        rel = str(path.relative_to(directory))
        if stat.S_ISLNK(info.st_mode):
            links.append({'relative': rel, 'targetSHA256': hashlib.sha256(os.readlink(path).encode()).hexdigest()})
        elif stat.S_ISREG(info.st_mode):
            files[rel] = {'sha256': sha(path), 'bytes': info.st_size}
        else:
            assert stat.S_ISDIR(info.st_mode), 'Unexpected special file'
    return files, links

def manifest_hash(value):
    return hashlib.sha256(json.dumps(value, sort_keys=True).encode()).hexdigest()

def load(path):
    return json.loads(path.read_text())

# Missing durable completion receipts refuse observation of an active fixture.
wrapper = load(base / 'wrapper-result.json')
driver = load(base / 'driver/result.json')
before_launch = load(base / 'before-launch.json')
assert wrapper.get('finishedAt') and wrapper.get('receiptDurable') is True
assert driver.get('endedAt')
assert driver['sourceCommit'] == driver['helperSourceCommit'] == a.expected_source
assert driver['daemon']['imageID'] == a.expected_image
assert driver['package']['sourceCommit'] == a.expected_source
assert before_launch['dockerEndpoint'] == 'unix:///var/run/docker.sock'
assert wrapper['sourceBefore'] == wrapper['sourceAfter'] == before_launch['sources']
original = snapshot(base)
script_hash = sha(Path(__file__))
source_bindings = before_launch['sources']

def check_sources():
    for raw, expected in source_bindings.items():
        path = Path(raw)
        assert sha(path) == expected['sha256'] and path.stat().st_size == expected['bytes']
    for row in driver['helperBindings']:
        assert row['commit'] == a.expected_source
        path = base / 'driver/accepted-harness' / row['path'].removeprefix('scripts/acceptance/')
        assert sha(path) == row['sha256'] and path.stat().st_size == row['bytes']
        committed = subprocess.check_output(['git', 'show', a.expected_source + ':' + row['path']], cwd=a.repo, timeout=10)
        assert hashlib.sha256(committed).hexdigest() == row['sha256']
    for name, field in [('launch.mjs', 'launcherSHA256'), ('scenario.mjs', 'scenarioSHA256')]:
        assert sha(base / 'driver/accepted-harness' / name) == driver[field]
    command = before_launch['command']
    for flag, field, suffix in [('--binary', 'binarySHA256', None), ('--web', 'webIndexSHA256', 'index.html')]:
        path = Path(command[command.index(flag) + 1])
        if suffix:
            path = path / suffix
        assert sha(path) == driver['package'][field]
    assert sha(Path(__file__)) == script_hash

check_sources()
fixture = Path((base / 'driver/private-fixture-path').read_text().strip())
assert fixture.parent == a.fixture_parent.resolve(strict=True)
assert fixture.name.startswith('luci-c054-') and fixture.resolve(strict=True) == fixture
fixture_before = snapshot(fixture)

known = {}
for row in wrapper['identities']:
    expected = row['expected']
    known[(expected['pid'], str(expected['startTicks']))] = expected['family']
for family, rows in driver.get('final', {}).get('knownProcessIdentities', {}).items():
    for row in rows:
        key = (row['pid'], str(row['start']))
        assert key not in known or known[key] == family
        known[key] = family
identities = []
for (pid, start), family in sorted(known.items()):
    try:
        fields = (Path('/proc') / str(pid) / 'stat').read_text().rsplit(') ', 1)[1].split()
        now = {'status': 'observed' if fields[19] == start else 'original-absent-pid-reused',
               'startTicks': fields[19], 'state': fields[0], 'parentPID': int(fields[1])}
    except FileNotFoundError:
        now = {'status': 'absent'}
    except PermissionError:
        now = {'status': 'unavailable'}
    identities.append({'pid': pid, 'expectedStartTicks': start, 'family': family, 'now': now})

env = {key: os.environ[key] for key in ['PATH', 'LANG', 'LC_ALL', 'TZ'] if key in os.environ}
env['HOME'] = str(base / 'launcher-home')
prefix = ['docker', '--host', 'unix:///var/run/docker.sock', '--config', str(base / 'launcher-home')]

def docker(*args):
    return subprocess.check_output(prefix + list(args), env=env, text=True, timeout=10).strip()

engine = docker('info', '--format', '{{.ID}}')
assert engine == driver['daemon']['id']
image = json.loads(docker('image', 'inspect', '--format', '{{json .}}', a.expected_image))
assert image['Id'] == a.expected_image
containers = docker('ps', '-aq', '--no-trunc').split()
assert all(re.fullmatch(r'[a-f0-9]{64}', x) for x in containers)
volumes = docker('volume', 'ls', '-q').split()
known_containers = set()
for phase in driver['phases']:
    for key in ('containers', 'knownAllocations'):
        for row in phase.get(key, []):
            if isinstance(row, dict) and re.fullmatch(r'[a-f0-9]{64}', row.get('id', '')):
                assert row['imageID'] == a.expected_image
                known_containers.add(row['id'])
container_observations = []
for container_id in sorted(known_containers):
    observed = subprocess.run(prefix + ['inspect', '--type', 'container', '--format', '{{.Id}}', container_id],
                              env=env, text=True, capture_output=True, timeout=10)
    if observed.returncode == 0:
        assert observed.stdout.strip() == container_id
        status = 'present'
    elif container_id not in containers and re.search(r'No such (?:object|container):\s*' + container_id, observed.stderr):
        status = 'confirmed-absent-at-observation'
    else:
        status = 'unavailable'
    container_observations.append({'id': container_id, 'status': status, 'exitCode': observed.returncode})

ledger = fixture / 'state/control-plane/execution/leases.json'
value = load(ledger)
assert isinstance(value, dict) and isinstance(value.get('leases'), list)
counts = {}
for key in ['leases', 'checks', 'fences', 'checkFences']:
    assert isinstance(value.get(key, []), list)
    counts[key] = len(value.get(key, []))
ledger_observation = {'sha256': sha(ledger), 'bytes': ledger.stat().st_size, 'counts': counts,
                      'matchesDriverFinal': sha(ledger) == driver.get('final', {}).get('ledger', {}).get('sha256')}
assert docker('info', '--format', '{{.ID}}') == engine
assert docker('ps', '-aq', '--no-trunc').split() == containers
assert docker('volume', 'ls', '-q').split() == volumes
check_sources()
assert snapshot(base) == original and snapshot(fixture) == fixture_before

zombies = [x for x in identities if x['now']['status'] == 'observed' and x['now']['state'] == 'Z']
live = [x for x in identities if x['now']['status'] == 'observed' and x['now']['state'] != 'Z']
unknown = [x for x in identities if x['now']['status'] == 'unavailable']
result = {'at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'sourceCommit': a.expected_source, 'expectedImageID': a.expected_image,
    'passedWorkflow': driver['passed'], 'originalFailedStage': driver.get('failedStage'),
    'originalResultSHA256': sha(base / 'driver/result.json'), 'originalWrapperSHA256': sha(base / 'wrapper-result.json'),
    'identities': identities, 'knownLiveProcessCount': len(live), 'knownZombieCount': len(zombies),
    'knownUnavailableProcessCount': len(unknown),
    'unreadableEnvironmentObservations': {'wrapper': wrapper.get('unreadableEnvironmentObservations'),
        'driver': driver.get('final', {}).get('unreadableEnvironmentObservations'),
        'freshEnvironmentCensusPerformed': False, 'wholeMachineClosureEstablished': False},
    'containers': containers, 'knownContainers': container_observations, 'volumes': volumes,
    'sameEngine': True, 'immutableImageVerified': {'id': image['Id'], 'os': image['Os'], 'architecture': image['Architecture']},
    'ledger': ledger_observation, 'fixtureFilesCount': len(fixture_before[0]),
    'fixtureManifestSHA256': manifest_hash(fixture_before[0]), 'fixtureLinks': fixture_before[1],
    'originalFilesUnchanged': True, 'originalManifestSHA256': manifest_hash(original[0]),
    'fixtureFilesUnchanged': True, 'sourceBindingsUnchanged': True,
    'sourceBindings': source_bindings, 'scriptSHA256': script_hash,
    'scope': 'Read-only completed-attempt observation of exact recorded PID/start identities, immutable container IDs, selected daemon inventories and parsed private ledger counts. No signals, cleanup, runtime mutation, process-environment values, whole-machine census, historical charge release, admission reconstruction or Docker wire counts. Zombies and prior unreadable-environment observations remain explicit. Invariance covers the observation interval; prior run source binding is separately retained.'}
with out.open('x') as f:
    json.dump(result, f, indent=2)
    f.write('\n')
os.chmod(out, 0o600)
print(json.dumps({'receipt': str(out), 'sha256': sha(out), 'sourceCommit': a.expected_source,
    'passedWorkflow': driver['passed'], 'originalFailedStage': driver.get('failedStage'),
    'knownLiveProcessCount': len(live), 'knownZombieCount': len(zombies), 'knownUnavailableProcessCount': len(unknown),
    'knownContainers': container_observations, 'containers': containers, 'volumes': volumes,
    'ledgerCounts': counts, 'sourceBindingsUnchanged': True, 'originalFilesUnchanged': True,
    'fixtureFilesUnchanged': True, 'wholeMachineClosureEstablished': False}))

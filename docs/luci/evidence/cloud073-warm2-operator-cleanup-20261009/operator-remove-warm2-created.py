"""One-off operator removal; never product recovery or ledger repair."""
from pathlib import Path
from datetime import datetime, timezone
import hashlib, json, os, stat, subprocess, time

ROOT = Path('/workspace/.luci-tools/cloud073-installed-78665270')
BASE = ROOT / 'evidence/warm-attempt2'
OUT = ROOT / 'operator-warm2-created-removal'
CID = '78a90473d0b49f1934e5fe5942d0a3167c47a7bd68db8cd1e4385cf8e59dfcaf'
assert not OUT.exists(), 'Do not overwrite or repeat this operation'
OUT.mkdir(mode=0o700)
def sha(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()
def snap(directory):
    rows = {}
    for path in sorted(directory.rglob('*')):
        info = path.lstat()
        assert not stat.S_ISLNK(info.st_mode), 'Unexpected linked evidence'
        if stat.S_ISREG(info.st_mode):
            rows[str(path.relative_to(directory))] = {'sha256': sha(path), 'bytes': info.st_size}
        else:
            assert stat.S_ISDIR(info.st_mode)
    return rows
def save(name, data):
    with (OUT / name).open('x') as stream:
        json.dump(data, stream, indent=2); stream.write('\n'); stream.flush(); os.fsync(stream.fileno())
    os.chmod(OUT / name, 0o600)
def stamp(): return datetime.now(timezone.utc).isoformat()
def available():
    v = os.statvfs('/workspace'); return v.f_bavail * v.f_frsize
env = {k: os.environ[k] for k in ('PATH', 'LANG', 'LC_ALL', 'TZ') if k in os.environ}
env['HOME'] = str(BASE / 'launcher-home')
def docker(*args, timeout=10):
    return subprocess.run(['docker', '--host', 'unix:///var/run/docker.sock', '--config', str(BASE / 'launcher-home'), *args], env=env, capture_output=True, text=True, timeout=timeout)
def read(*args):
    result = docker(*args); assert result.returncode == 0, 'Read-only Docker probe failed'
    return result.stdout.strip()
def identities(rows):
    result = []
    for row in rows:
        expected = row['expected']; path = Path('/proc') / str(expected['pid']) / 'stat'
        try:
            fields = path.read_text().rsplit(') ', 1)[1].split()
            observed = {'startTicks': fields[19], 'state': fields[0], 'parentPID': int(fields[1])}
            absent = fields[19] != expected['startTicks']
        except FileNotFoundError:
            observed = {'state': 'absent'}; absent = True
        assert absent or (expected['family'] == 'browser' and observed['state'] == 'Z'), 'Known live process or uncertain identity'
        result.append({'expected': expected, 'observed': observed, 'originalAbsent': absent})
    return result
receipt = {'startedAt': stamp(), 'scope': 'Explicit operator removal of one owned Created allocation. Original pending Restart, registry and charged lease remain untouched. This is not automatic cleanup, Create acknowledgement, transport settlement, capacity release or successful recovery.', 'scriptSHA256': sha(Path(__file__)), 'mutationAttempted': False}
try:
    projection = json.loads((BASE / 'retained-allocation-projection.json').read_text())
    original = json.loads((BASE / 'independent-closure.json').read_text())
    driver = json.loads((BASE / 'driver/result.json').read_text())
    wrapper = json.loads((BASE / 'wrapper-result.json').read_text())
    fixture = Path((BASE / 'driver/private-fixture-path').read_text().strip())
    assert fixture == ROOT / 'fixtures/<PRIVATE_FIXTURE>' and fixture.resolve() == fixture
    fixture_before = snap(fixture); evidence_before = snap(BASE)
    assert len(fixture_before) == original['fixtureFilesCount'] == 31
    manifest_digest = hashlib.sha256(json.dumps(fixture_before, sort_keys=True).encode()).hexdigest()
    assert manifest_digest == original['fixtureManifestSHA256']
    assert sha(BASE / 'driver/result.json') == original['originalResultSHA256'] == projection['originalResultSHA256']
    ledger = fixture / 'state/control-plane/execution/leases.json'
    assert sha(ledger) == original['ledger']['sha256'] == projection['privateFileBindings']['ledger']['sha256']
    assert read('info', '--format', '{{.ID}}') == driver['daemon']['id']
    receipt['identitiesBefore'] = identities(wrapper['identities'])
    template = '{"id":{{json .Id}},"name":{{json .Name}},"status":{{json .State.Status}},"running":{{json .State.Running}},"imageID":{{json .Image}},"memoryBytes":{{json .HostConfig.Memory}},"nanoCPUs":{{json .HostConfig.NanoCpus}},"managed":{{json (index .Config.Labels "luci.managed")}},"instance":{{json (index .Config.Labels "luci.instance")}},"sandboxId":{{json (index .Config.Labels "luci.sandbox-id")}},"sessionId":{{json (index .Config.Labels "luci.session-id")}}}'
    current = json.loads(read('inspect', '--type', 'container', '--format', template, CID))
    assert current == projection['container'] and current['id'] == CID and current['status'] == 'created' and current['running'] is False
    assert snap(fixture) == fixture_before and snap(BASE) == evidence_before
    receipt.update({'sourceCommit': driver['sourceCommit'], 'originalResultSHA256': original['originalResultSHA256'], 'containerBefore': current, 'ledgerSHA256Before': sha(ledger), 'fixtureFileCount': len(fixture_before), 'fixtureManifestSHA256': manifest_digest, 'originalEvidenceFileCount': len(evidence_before), 'originalEvidenceManifestSHA256': hashlib.sha256(json.dumps(evidence_before, sort_keys=True).encode()).hexdigest(), 'rootAvailableBytesBefore': available(), 'sameEngine': True, 'knownProcessesQuiescentBefore': True})
    save('before.json', receipt)
    receipt['mutationAttempted'] = True; receipt['mutationStartedAt'] = stamp()
    began = time.monotonic()
    try:
        removal = docker('rm', CID, timeout=45)
        receipt['removal'] = {'exitCode': removal.returncode, 'stdoutIsExactID': removal.stdout.strip() == CID, 'stdoutBytes': len(removal.stdout.encode()), 'stderrBytes': len(removal.stderr.encode()), 'stderrSHA256': hashlib.sha256(removal.stderr.encode()).hexdigest(), 'elapsedSeconds': time.monotonic() - began}
        assert removal.returncode == 0 and removal.stdout.strip() == CID, 'Nonforce removal failed; no retry'
        ids = read('ps', '-aq', '--no-trunc').split()
        receipt['exactIDAbsentFromDaemonInventory'] = CID not in ids
        assert CID not in ids
        absence = docker('inspect', '--type', 'container', '--format', '{{.Id}}', CID)
        receipt['inspectAfter'] = {'exitCode': absence.returncode, 'stdoutBytes': len(absence.stdout.encode()), 'stderrSHA256': hashlib.sha256(absence.stderr.encode()).hexdigest(), 'explicitNoSuchObject': ('No such object: ' + CID) in absence.stderr or ('No such container: ' + CID) in absence.stderr}
        assert absence.returncode != 0 and receipt['inspectAfter']['explicitNoSuchObject']
        receipt['removalObserved'] = True
    finally:
        receipt['rootAvailableBytesAfter'] = available()
        receipt['fixtureFilesUnchanged'] = snap(fixture) == fixture_before
        receipt['originalEvidenceFilesUnchanged'] = snap(BASE) == evidence_before
        receipt['ledgerSHA256After'] = sha(ledger)
        receipt['identitiesAfter'] = identities(wrapper['identities'])
        receipt['ledgerCountsAfter'] = {k: len(json.loads(ledger.read_text()).get(k, [])) for k in ('leases', 'checks', 'fences', 'checkFences')}
        assert receipt['fixtureFilesUnchanged'] and receipt['originalEvidenceFilesUnchanged'] and receipt['ledgerSHA256After'] == receipt['ledgerSHA256Before']
except Exception as error:
    receipt['operationErrorType'] = type(error).__name__
    raise
finally:
    receipt['finishedAt'] = stamp()
    save('result.json', receipt)
    print(json.dumps({'receipt': str(OUT / 'result.json'), 'sha256': sha(OUT / 'result.json'), 'mutationAttempted': receipt['mutationAttempted'], 'removalObserved': receipt.get('removalObserved', False), 'originalEvidenceFilesUnchanged': receipt.get('originalEvidenceFilesUnchanged'), 'fixtureFilesUnchanged': receipt.get('fixtureFilesUnchanged'), 'remainingLeaseCount': receipt.get('ledgerCountsAfter', {}).get('leases')}))

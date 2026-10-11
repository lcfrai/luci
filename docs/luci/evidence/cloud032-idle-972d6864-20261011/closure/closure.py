from pathlib import Path
import datetime, hashlib, json, os, subprocess

BASE = Path('/workspace/luci/artifacts/quality-swarm/idle-972d6864-plain-attempt1')
OUT = Path(__file__).resolve().parent
REPO = Path('/workspace/luci')
PRE = Path('/workspace/.luci-tools/idle972d-prelaunch-processes.json')
ORIGINAL = Path('/workspace/.luci-tools/cloud032-idle-d20ebd29-analysis/closure.py')
EXPECTED_RESULT = '5f25447b1f6dc7eca50e220f358a4a029bd311e7d368e15ec85d693198569a00'

def sha(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()

def artifacts():
    return {str(p.relative_to(BASE)): {'sha256': sha(p), 'bytes': p.stat().st_size}
            for p in sorted(BASE.rglob('*')) if p.is_file()}

def identity(pid):
    raw = Path('/proc', str(pid), 'stat').read_text()
    fields = raw.rsplit(') ', 1)[1].split()
    return {'pid': int(pid), 'comm': raw.split('(', 1)[1].rsplit(') ', 1)[0],
            'state': fields[0], 'parent': int(fields[1]), 'startTicks': int(fields[19])}

before = artifacts()
assert before['driver/result.json']['sha256'] == EXPECTED_RESULT
pre_sha = sha(PRE)
pre = json.loads(PRE.read_text())
d = json.loads((BASE/'driver/result.json').read_text())
wrap = json.loads((BASE/'wrapper-result.json').read_text())
journal = [json.loads(line) for line in (BASE/'driver/diagnostic-journal.jsonl').read_text().splitlines()]
known = [{'family': 'driver', **wrap['driverIdentity']}] + [
    {'family': 'controller', **row} for row in journal if row['event'] == 'controller-start']
observed = []
for row in known:
    expected = {'family': row['family'], 'pid': row['pid'], 'startTicks': row['startTicks']}
    try:
        current = identity(row['pid'])
        same = current['startTicks'] == int(row['startTicks'])
        state = 'same-identity-zombie' if same and current['state'] == 'Z' else 'same-identity-live' if same else 'different-identity'
    except FileNotFoundError:
        state, current = 'absent', None
    except (PermissionError, OSError, ValueError, IndexError) as error:
        state, current = 'unknown', {'errorType': type(error).__name__}
    observed.append({'expected': expected, 'observation': state, 'current': current})

names = sorted({row['comm'] for row in pre['selectedNames']})
survey, unreadable = [], []
for path in Path('/proc').iterdir():
    if not path.name.isdigit():
        continue
    try:
        row = identity(path.name)
        if row['comm'] in names:
            survey.append(row)
    except FileNotFoundError:
        pass
    except (PermissionError, OSError, ValueError, IndexError) as error:
        unreadable.append({'pid': int(path.name), 'errorType': type(error).__name__})
survey.sort(key=lambda row: row['pid'])
pre_keys = {(row['pid'], row['startTicks']) for row in pre['selectedNames']}
new_rows = [row for row in survey if (row['pid'], row['startTicks']) not in pre_keys]

def cmd(args, env=None):
    return subprocess.check_output(args, cwd=REPO, text=True, timeout=30, env=env).strip()

docker_env = {k: os.environ[k] for k in ('PATH', 'LANG', 'LC_ALL', 'TZ') if k in os.environ}
docker_cmd = ['/usr/local/bin/docker', '--host', 'unix:///var/run/docker.sock', '--config', '/workspace/.luci-tools/c076-c078-runtime-972d6864-attempt1']
containers = cmd(docker_cmd + ['ps', '-aq', '--no-trunc'], docker_env).splitlines()
volumes = cmd(docker_cmd + ['volume', 'ls', '-q'], docker_env).splitlines()
image = json.loads(cmd(docker_cmd + ['image', 'inspect', d['image']['id']], docker_env))[0]['Id']
engine_hash = hashlib.sha256(cmd(docker_cmd + ['info', '--format', '{{.ID}}'], docker_env).encode()).hexdigest()
after = artifacts()
assert before == after and sha(PRE) == pre_sha
receipt = {
    'observedAt': datetime.datetime.now(datetime.timezone.utc).isoformat(),
    'scope': 'Independent read-only known driver/controller identities, selected comm-name survey and selected exclusive daemon inventory. No cleanup mutations, signals or fixture edits.',
    'adaptedFrom': {'path': str(ORIGINAL), 'sha256': sha(ORIGINAL)},
    'closureScriptSHA256': sha(Path(__file__)),
    'sourceHEAD': cmd(['git', 'rev-parse', 'HEAD']), 'sourceStatus': cmd(['git', 'status', '--porcelain']).splitlines(),
    'originalResultSHA256': EXPECTED_RESULT, 'allOriginalArtifactsStable': before == after,
    'originalArtifactsBefore': before, 'originalArtifactsAfter': after,
    'identities': observed, 'dockerEndpoint': 'unix:///var/run/docker.sock',
    'containers': containers, 'volumes': volumes, 'imageID': image,
    'imageMatchesDriver': image == d['image']['id'], 'engineIdentitySHA256': engine_hash,
    'engineMatchesDriver': engine_hash == d['engineIdentitySHA256'],
    'driverCleanupAssertions': {k: d[k] for k in ['controllersStopped', 'dockerRequestsDrained', 'dockerMutationStateCertain', 'fixtureRemoved', 'cleanup']},
    'processSurvey': {'prelaunchPath': str(PRE), 'prelaunchSHA256': pre_sha, 'prelaunchAt': pre['at'],
        'selectedCommNames': names, 'selectionBasis': 'Comm names represented in recorded prelaunch rows; original selection predicate was not recorded.',
        'prelaunchCount': len(pre['selectedNames']), 'currentCount': len(survey), 'currentRows': survey,
        'unreadablePids': unreadable, 'newIdentityRows': new_rows,
        'newZombies': [row for row in new_rows if row['state'] == 'Z'],
        'currentNonZombies': [row for row in survey if row['state'] != 'Z'],
        'ownership': 'Survey differences are not owned-browser PID/start bindings and are not attributed to this run.'},
    'rootFreeBytes': os.statvfs('/').f_bavail * os.statvfs('/').f_frsize,
    'limits': ['Known driver/controller PID/start observations are independent. No browser PID/start census or owned-process resource sampler was captured.',
        'Selected comm-name survey and empty Docker inventory do not establish browser absence or whole-machine closure; new zombies remain unattributed.',
        'Fixture removal is a retained driver assertion. No retained failed fixture or charged ledger was read or modified. No counter adjunct ran in this plain attempt.']}
with (OUT/'independent-closure.json').open('x') as output:
    json.dump(receipt, output, indent=2)
    output.write('\n')
print(json.dumps({k: receipt[k] for k in ['sourceHEAD', 'sourceStatus', 'allOriginalArtifactsStable', 'identities', 'containers', 'volumes', 'imageMatchesDriver', 'engineMatchesDriver', 'rootFreeBytes']} | {
    'artifactCount': len(before), 'surveyBeforeCount': len(pre['selectedNames']), 'surveyAfterCount': len(survey),
    'newZombies': receipt['processSurvey']['newZombies'], 'currentNonZombies': receipt['processSurvey']['currentNonZombies'],
    'unreadablePids': unreadable, 'receipt': str(OUT/'independent-closure.json')}, indent=2))

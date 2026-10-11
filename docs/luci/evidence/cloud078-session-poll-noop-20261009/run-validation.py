import datetime
import hashlib
import json
import os
import pathlib
import subprocess
import sys
import time

BASE = pathlib.Path(__file__).parent
REPO = pathlib.Path('/workspace/luci')
arm = sys.argv[1]
assert arm in ('before', 'after')
assert not (BASE / ('validation-' + arm + '-receipt.json')).exists(), 'Do not overwrite a receipt'
binding = json.loads((BASE / 'source-binding.json').read_text())
inputs = json.loads((BASE / ('validation-inputs-' + arm + '.json')).read_text())

def snapshot():
    rows = []
    for row in binding['files']:
        path = REPO / row['path']
        sha = hashlib.sha256(path.read_bytes()).hexdigest()
        assert sha == row['sha256'], 'Selected working source changed: ' + row['path']
        committed = subprocess.check_output(['git', '-C', str(REPO), 'show',
                    binding['sourceCommit'] + ':' + row['path']])
        assert hashlib.sha256(committed).hexdigest() == sha, 'Declared source binding mismatch'
        rows.append({'path': row['path'], 'sha256': sha})
    for row in inputs['files']:
        assert hashlib.sha256(pathlib.Path(row['path']).read_bytes()).hexdigest() == row['sha256']
    return rows

before = snapshot()
command = ['node', str(REPO / 'desktop/node_modules/vitest/vitest.mjs'), 'run',
           '--config', str(BASE / ('vitest-' + arm + '.config.mjs')),
           '--configLoader', 'runner', '--maxWorkers=1', '--no-file-parallelism']
record = {'arm': arm, 'sourceCommit': binding['sourceCommit'],
          'observedHEAD': subprocess.check_output(['git', '-C', str(REPO), 'rev-parse', 'HEAD'], text=True).strip(),
          'command': command, 'cwd': str(REPO / 'desktop'),
          'environmentOverrides': {'CI': '1', 'GOMAXPROCS': '2'},
          'startedAt': datetime.datetime.now(datetime.timezone.utc).isoformat(),
          'timeoutSeconds': 90, 'before': before, 'overlayAndSharedTestInputs': inputs['files']}
(BASE / ('validation-' + arm + '-invocation.json')).write_text(json.dumps(record, indent=2) + '\n')
start = time.monotonic()
with (BASE / ('validation-' + arm + '.stdout.log')).open('x') as out, (BASE / ('validation-' + arm + '.stderr.log')).open('x') as err:
    try:
        result = subprocess.run(command, cwd=REPO / 'desktop', env={**os.environ, 'CI': '1', 'GOMAXPROCS': '2'},
                                stdout=out, stderr=err, timeout=90)
        record['exitCode'] = result.returncode
    except subprocess.TimeoutExpired:
        record['exitCode'] = 124
        record['timedOut'] = True
record['elapsedSeconds'] = time.monotonic() - start
record['completedAt'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
record['after'] = snapshot()
record['selectedSourcesAndOverlayStable'] = record['before'] == record['after']
for suffix in ('.stdout.log', '.stderr.log', '.json'):
    path = BASE / ('validation-' + arm + suffix)
    if path.exists():
        record.setdefault('outputs', []).append({'path': path.name, 'bytes': path.stat().st_size,
                                                 'sha256': hashlib.sha256(path.read_bytes()).hexdigest()})
path = BASE / ('validation-' + arm + '.json')
if path.exists():
    result = json.loads(path.read_text())
    record['outcomes'] = {key: result.get(key) for key in
                         ('numTotalTests', 'numPassedTests', 'numFailedTests', 'numPendingTests', 'numTodoTests', 'success')}
(BASE / ('validation-' + arm + '-receipt.json')).write_text(json.dumps(record, indent=2) + '\n')
print(json.dumps({key: record.get(key) for key in ('arm', 'exitCode', 'elapsedSeconds', 'outcomes', 'selectedSourcesAndOverlayStable')}))
sys.exit(record['exitCode'])

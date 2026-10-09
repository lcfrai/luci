"""Capture one selected Go overlay invocation and unchanged source bindings."""
from pathlib import Path
import datetime
import hashlib
import json
import os
import subprocess
import sys
import time

ROOT = Path('/workspace/luci')
BASE = Path('/workspace/.luci-tools/cloud054-terminal-close/backend')
mode = sys.argv[1]
assert mode in ('before', 'after', 'affected')
dest = BASE / 'receipts' / mode
dest.mkdir(exist_ok=False)
env = dict(os.environ, GOMAXPROCS='2')
def digest(path):
    raw = path.read_bytes()
    return {'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}
paths = [ROOT / 'control-plane/internal/proxy/proxy.go', ROOT / 'control-plane/internal/proxy/proxy_test.go',
         ROOT / 'control-plane/internal/api/attach.go', ROOT / 'control-plane/internal/api/attach_test.go',
         ROOT / 'control-plane/go.mod', ROOT / 'control-plane/go.sum',
         BASE / 'before/proxy.go', BASE / 'after/proxy.go', BASE / 'tests/proxy_test.go', BASE / 'tests/attach_test.go',
         BASE / 'before-overlay.json', BASE / 'after-overlay.json', Path(__file__)]
before = {str(p): digest(p) for p in paths}
(dest / 'inputs-before.json').write_text(json.dumps(before, indent=2) + '\n')
goenv = json.loads(subprocess.check_output(['go', 'env', '-json', 'GOVERSION', 'GOOS', 'GOARCH', 'GOROOT', 'GOMODCACHE', 'GOWORK', 'CGO_ENABLED'], cwd=ROOT, env=env))
cmd = ['go', 'test', '-p', '1', '-count=1', '-timeout=2m', '-json']
if mode != 'before': cmd.append('-race')
cmd += ['-overlay=' + str(BASE / ('before-overlay.json' if mode == 'before' else 'after-overlay.json')),
        './control-plane/internal/proxy', './control-plane/internal/api']
if mode != 'affected':
    cmd += ['-run', '^(TestRelayNormal|TestRelayAbnormal|TestRelayParentCancellation|TestAttachNaturalExit)']
else:
    cmd += ['-run', '^(TestRelay|TestAttach|TestADroppedSocketDestroysNothing)']
started = datetime.datetime.now(datetime.timezone.utc).isoformat()
begin = time.monotonic()
with (dest / 'stdout.jsonl').open('wb') as stdout, (dest / 'stderr.log').open('wb') as stderr:
    result = subprocess.run(cmd, cwd=ROOT, env=env, stdout=stdout, stderr=stderr, timeout=180)
after = {str(p): digest(p) for p in paths}
(dest / 'inputs-after.json').write_text(json.dumps(after, indent=2) + '\n')
outcomes = []
for line in (dest / 'stdout.jsonl').read_text().splitlines():
    try: row = json.loads(line)
    except ValueError: continue
    if row.get('Test') and row.get('Action') in ('pass', 'fail', 'skip'):
        outcomes.append({k: row[k] for k in ('Package', 'Test', 'Action', 'Elapsed') if k in row})
record = {'command': cmd, 'cwd': str(ROOT), 'environment': {'GOMAXPROCS': '2', **goenv},
          'startedAt': started, 'finishedAt': datetime.datetime.now(datetime.timezone.utc).isoformat(),
          'elapsedSeconds': time.monotonic() - begin, 'exitCode': result.returncode,
          'sourceInputsStable': before == after, 'sourceScope': 'Selected exact source/overlay files, module locks and capture helper; not a complete compiled dependency provenance claim.',
          'outcomes': outcomes, 'stdout': digest(dest / 'stdout.jsonl'), 'stderr': digest(dest / 'stderr.log')}
(dest / 'result.json').write_text(json.dumps(record, indent=2) + '\n')
assert before == after
print(json.dumps({'mode': mode, 'exitCode': result.returncode, 'outcomes': outcomes, 'sourceInputsStable': before == after}))

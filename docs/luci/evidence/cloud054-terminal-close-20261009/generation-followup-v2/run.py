from pathlib import Path
import datetime, hashlib, json, os, subprocess, time
root = Path('/workspace/luci')
base = Path(__file__).parent
dto = base.parent / 'frontend/backend/control-plane/internal/api'
out = base / 'receipts'
out.mkdir(exist_ok=False)
mapping = {str(root / 'control-plane/internal/api' / name): str(dto / name) for name in ('sessions.go', 'session_execution_generation.go', 'session_execution_generation_test.go')}
mapping[str(root / 'control-plane/internal/api/session_execution_generation_http_test.go')] = str(base / 'session_execution_generation_http_test.go')
mapping[str(root / 'control-plane/internal/management/team_session_execution_generation_test.go')] = str(base / 'team_session_execution_generation_test.go')
(out / 'overlay.json').write_text(json.dumps({'Replace': mapping}, indent=2) + '\n')
def digest(path):
    raw = path.read_bytes()
    return {'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}
inputs = [Path(p) for p in mapping.values()] + [Path(__file__), out / 'overlay.json', root / 'control-plane/go.mod', root / 'control-plane/go.sum']
before = {str(p): digest(p) for p in inputs}
cmd = ['go', 'test', '-race', '-p', '1', '-count=1', '-timeout=2m', '-json', '-overlay=' + str(out / 'overlay.json'),
       './control-plane/internal/management', '-run',
       '^TestTeamOpaqueExecutionGenerationRemainsProjectScoped$']
started = datetime.datetime.now(datetime.timezone.utc).isoformat()
begin = time.monotonic()
with (out / 'stdout.jsonl').open('wb') as stdout, (out / 'stderr.log').open('wb') as stderr:
    completed = subprocess.run(cmd, cwd=root, env=dict(os.environ, GOMAXPROCS='2'), stdout=stdout, stderr=stderr, timeout=180)
after = {str(p): digest(p) for p in inputs}
outcomes = []
for line in (out / 'stdout.jsonl').read_text().splitlines():
    try: row = json.loads(line)
    except ValueError: continue
    if row.get('Test') and row.get('Action') in ('pass', 'fail', 'skip'):
        outcomes.append({k: row[k] for k in ('Package', 'Test', 'Action', 'Elapsed') if k in row})
result = {'command': cmd, 'startedAt': started, 'elapsedSeconds': time.monotonic() - begin,
          'exitCode': completed.returncode, 'sourceInputsStable': before == after, 'inputsBefore': before, 'inputsAfter': after,
          'outcomes': outcomes, 'stdout': digest(out / 'stdout.jsonl'), 'stderr': digest(out / 'stderr.log'),
          'scope': 'Corrected synthetic team-field fixture only. Distinct markers for authorized provider session and hidden project; actual HMAC/HTTP restart tests remain in prior unchanged API receipt. No Docker/native/provider or complete compiled-input provenance claim.'}
(out / 'result.json').write_text(json.dumps(result, indent=2) + '\n')
assert before == after
print(json.dumps({'exitCode': completed.returncode, 'outcomes': outcomes, 'stable': before == after}))

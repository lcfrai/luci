#!/usr/bin/env python3
"""Bounded affected-file regression; load the recorded test source via Vite."""
from pathlib import Path
import datetime, hashlib, json, os, shutil, signal, subprocess, sys, time

R = Path('/workspace/luci')
D = R / 'desktop'
B = Path(__file__).resolve().parent
REL = 'desktop/src/components/common/ProjectDraftRecovery.test.tsx'
phase = sys.argv[1]
assert phase in ('before', 'fixed')
os.umask(0o077)
out = B / 'runs' / phase
out.mkdir(parents=True, exist_ok=False)
source = B / phase / REL
node = shutil.which('node')
assert node
env = {k: os.environ[k] for k in ('PATH', 'HOME', 'LANG', 'LC_ALL', 'TZ', 'TMPDIR', 'TEMP', 'TMP', 'SystemRoot') if k in os.environ}
env.update(CI='1', NO_COLOR='1')
assert subprocess.check_output([node, '--version'], env=env, text=True, timeout=10).strip() == 'v24.19.0'

def metadata(p):
    raw = p.read_bytes()
    return {'bytes': len(raw), 'sha256': hashlib.sha256(raw).hexdigest()}

def write(name, value):
    (out / name).write_text(json.dumps(value, indent=2) + '\n')

def snapshot():
    paths = sorted(p for p in (D / 'src').rglob('*') if p.is_file())
    paths += sorted(p for p in D.iterdir() if p.is_file() and p.suffix in ('.ts', '.tsx', '.json', '.js', '.mjs', '.cjs', '.html', '.css'))
    rows = {str(p.relative_to(R)): metadata(p) for p in paths}
    rows[REL] = {**metadata(source), 'effectiveSourcePath': str(source), 'repositoryFile': metadata(R / REL)}
    return {'scope': 'All desktop src and root source/config/lock files; explicit test-only load override. No private fixtures or application state.', 'files': rows}

config = out / 'vitest.config.mts'
config.write_text('''import base from "/workspace/luci/desktop/vitest.config.ts";
import { readFileSync, writeFileSync } from "node:fs";
import { createHash } from "node:crypto";
export default {
  ...base,
  cacheDir: CACHE,
  plugins: [{
    name: "recorded-draft-recovery-test-source",
    enforce: "pre",
    load(id) {
      if (id.split("?")[0] !== TARGET) return;
      const code = readFileSync(SOURCE, "utf8");
      writeFileSync(BINDING, JSON.stringify({ target: TARGET, source: SOURCE,
        bytes: Buffer.byteLength(code), sha256: createHash("sha256").update(code).digest("hex") }, null, 2) + "\\n");
      return { code, map: null };
    },
  }, ...(base.plugins ?? [])],
};
'''.replace('CACHE', json.dumps(str(out / 'vite-cache'))).replace('TARGET', json.dumps(str(R / REL))).replace('SOURCE', json.dumps(str(source))).replace('BINDING', json.dumps(str(out / 'loaded-test-source.json'))))
cmd = [node, str(D / 'node_modules/vitest/vitest.mjs'), 'run', 'src/components/common/ProjectDraftRecovery.test.tsx', '--config=' + str(config), '--maxWorkers=1', '--no-file-parallelism', '--reporter=json', '--outputFile=' + str(out / 'vitest.json')]
before = snapshot()
write('source-before.json', before)
dependencies = [D / 'node_modules/.package-lock.json', D / 'node_modules/vitest/vitest.mjs']
dependencies += [D / 'node_modules' / x / 'package.json' for x in ('vitest', 'vite', 'react', 'react-dom', 'jsdom', '@vitejs/plugin-react', '@testing-library/react', '@testing-library/user-event')]
write('dependencies.json', {'node': {'path': node, 'version': 'v24.19.0', **metadata(Path(node).resolve())}, 'selectedInstalledFiles': {str(p.relative_to(D)): metadata(p) for p in dependencies}, 'activation': metadata(Path('/workspace/.luci-tools/activate.sh')), 'scope': 'Actual Node executable, installed lock, Vitest entry and selected package manifests; not a transitive byte census.'})
record = {'phase': phase, 'command': cmd, 'cwd': str(D), 'timeoutSeconds': 120, 'maxLogBytesPerStream': 4 * 1024 * 1024, 'startedAt': datetime.datetime.now(datetime.timezone.utc).isoformat(), 'head': subprocess.check_output(['git', '-C', str(R), 'rev-parse', 'HEAD'], text=True, timeout=10).strip(), 'config': metadata(config), 'runner': metadata(Path(__file__)), 'environmentNames': sorted(env), 'explicitEnvironment': {'CI': '1', 'NO_COLOR': '1'}}
write('invocation.json', record)
proc = None
start = time.monotonic()
try:
    with (out / 'stdout.log').open('xb') as stdout, (out / 'stderr.log').open('xb') as stderr:
        proc = subprocess.Popen(cmd, cwd=D, env=env, stdout=stdout, stderr=stderr, start_new_session=True)
        record['pid'] = proc.pid
        while proc.poll() is None:
            if time.monotonic() - start > record['timeoutSeconds']:
                raise TimeoutError('120 second regression bound')
            if any((out / name).stat().st_size > record['maxLogBytesPerStream'] for name in ('stdout.log', 'stderr.log')):
                raise RuntimeError('Log size bound')
            time.sleep(0.25)
        record['exitCode'] = proc.returncode
except BaseException as error:
    record['runnerFailure'] = {'type': type(error).__name__, 'message': str(error)}
finally:
    if proc is not None and proc.poll() is None:
        os.killpg(proc.pid, signal.SIGTERM)
        try:
            proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            os.killpg(proc.pid, signal.SIGKILL)
            proc.wait(timeout=10)
    after = snapshot()
    write('source-after.json', after)
    record['sourceStable'] = before == after
    record['sourceChanges'] = [k for k in sorted(set(before['files']) | set(after['files'])) if before['files'].get(k) != after['files'].get(k)]
    record['elapsedSeconds'] = time.monotonic() - start
    record['completedAt'] = datetime.datetime.now(datetime.timezone.utc).isoformat()
    if (out / 'vitest.json').exists():
        data = json.loads((out / 'vitest.json').read_text())
        record['vitest'] = {k: data.get(k) for k in ('numTotalTests', 'numPassedTests', 'numFailedTests', 'numPendingTests', 'numTodoTests', 'success')}
        record['failedTests'] = [{'file': f['name'], 'name': t['fullName'], 'failureMessages': t['failureMessages']} for f in data.get('testResults', []) for t in f.get('assertionResults', []) if t['status'] == 'failed']
    record['loadBindingMatches'] = (out / 'loaded-test-source.json').exists() and json.loads((out / 'loaded-test-source.json').read_text())['sha256'] == metadata(source)['sha256']
    record['passed'] = record.get('exitCode') == 0 and record['sourceStable'] and record['loadBindingMatches'] and record.get('vitest', {}).get('success') is True
    write('result.json', record)
    print(json.dumps({k: record.get(k) for k in ('phase', 'exitCode', 'sourceStable', 'loadBindingMatches', 'elapsedSeconds', 'vitest', 'failedTests', 'runnerFailure', 'passed')}, indent=2))
sys.exit(0 if record['passed'] else 1)

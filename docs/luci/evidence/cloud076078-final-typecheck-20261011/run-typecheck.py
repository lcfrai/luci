#!/usr/bin/env python3
"""One bounded full desktop typecheck with unchanged effective input checks."""
from pathlib import Path
import datetime, importlib.util, json, os, shutil, signal, subprocess, sys

B = Path(__file__).resolve().parent
R = Path('/workspace/luci')
D = R / 'desktop'
os.umask(0o077)
out = B / 'run'
out.mkdir(exist_ok=False)
spec = importlib.util.spec_from_file_location('binding', B / 'binding-and-execution.py')
binding = importlib.util.module_from_spec(spec)
spec.loader.exec_module(binding)
node = shutil.which('node')
assert node
env = {k: os.environ[k] for k in ('PATH', 'HOME', 'LANG', 'LC_ALL', 'TZ', 'TMPDIR', 'TEMP', 'TMP', 'SystemRoot') if k in os.environ}
env.update(CI='1', NO_COLOR='1')
version = subprocess.check_output([node, '--version'], env=env, text=True, timeout=10).strip()
assert version == 'v24.19.0', version
record = {
    'version': 1, 'startedAt': binding.now(), 'nodeVersion': version,
    'headBefore': binding.git('rev-parse', 'HEAD'),
    'worktreeStatusBefore': binding.git('status', '--porcelain=v1', '--untracked-files=all'),
    'effectiveSource': 'Actual integrated working tree, including staged and unstaged changes; no source overlay.',
    'cleanHEADClaim': False,
    'environmentNames': sorted(env), 'explicitEnvironment': {'CI': '1', 'NO_COLOR': '1'},
    'runner': binding.filemeta(Path(__file__)),
    'helper': binding.filemeta(B / 'binding-and-execution.py'),
    'commands': [],
}
binding.write(out / 'invocation.json', record)

def cancel(signum, frame):
    raise InterruptedError('Runner received signal ' + str(signum))

signal.signal(signal.SIGTERM, cancel)
try:
    before = binding.inputs()
    deps_before = binding.deps(node)
    binding.write(out / 'source-before.json', before)
    binding.write(out / 'dependencies-before.json', deps_before)
    record['commands'].append(binding.execute('typecheck', [node, str(D / 'node_modules/typescript/bin/tsc'), '--noEmit'], 180, out, env))
    after = binding.inputs()
    deps_after = binding.deps(node)
    binding.write(out / 'source-after.json', after)
    binding.write(out / 'dependencies-after.json', deps_after)
    record.update({
        'inputCount': before['count'],
        'sourceStable': before == after,
        'dependenciesStable': deps_before == deps_after,
        'sourceChanges': [k for k in sorted(set(before['files']) | set(after['files'])) if before['files'].get(k) != after['files'].get(k)],
        'sourceBefore': binding.filemeta(out / 'source-before.json'),
        'sourceAfter': binding.filemeta(out / 'source-after.json'),
    })
    record['passed'] = all(c['exitCode'] == 0 and not c.get('failure') for c in record['commands']) and record['sourceStable'] and record['dependenciesStable']
except BaseException as error:
    record['passed'] = False
    record['runnerFailure'] = {'type': type(error).__name__, 'message': str(error)}
finally:
    proc = binding.ACTIVE
    if proc is not None and proc.poll() is None:
        signal.signal(signal.SIGTERM, signal.SIG_IGN)
        try:
            os.killpg(proc.pid, signal.SIGTERM)
        except ProcessLookupError:
            pass
        try:
            proc.wait(timeout=10)
        except subprocess.TimeoutExpired:
            os.killpg(proc.pid, signal.SIGKILL)
            proc.wait(timeout=10)
        record['interruptedChildCleanup'] = {'pid': proc.pid, 'exitCode': proc.returncode, 'scope': 'Owned process group only, 20 second cleanup bound.'}
    record.update({'headAfter': binding.git('rev-parse', 'HEAD'), 'worktreeStatusAfter': binding.git('status', '--porcelain=v1', '--untracked-files=all'), 'completedAt': binding.now()})
    binding.write(out / 'result.json', record)
    print(json.dumps({k: record.get(k) for k in ('passed', 'inputCount', 'sourceStable', 'dependenciesStable', 'sourceChanges', 'runnerFailure')}, indent=2))
sys.exit(0 if record['passed'] else 1)

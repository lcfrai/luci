#!/usr/bin/env python3
"""Run only the frozen Node/VM contracts, with durable bounded receipts. No browser/build."""
from pathlib import Path
import argparse, datetime, hashlib, json, os, shutil, signal, subprocess, sys, time

B = Path(__file__).resolve().parent
R = Path('/workspace/luci')
def sha(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()
def write(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')
    path.chmod(0o600)
def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()
def verify(prepared, dependencies):
    for row in prepared['files']:
        path = B / row['path']
        assert path.is_file() and not path.is_symlink() and path.stat().st_size == row['bytes'] and sha(path) == row['sha256'], row['path']
    for name, expected in dependencies.items():
        assert sha(R / 'desktop/node_modules' / name) == expected, name

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--output', type=Path, default=B / 'contract-check')
    args = parser.parse_args()
    prepared = json.loads((B / 'prepared-files.json').read_text())
    dependencies = json.loads((B / 'dependency-bindings.json').read_text())['files']
    verify(prepared, dependencies)
    version = subprocess.check_output(['node', '--version'], text=True, timeout=5).strip()
    assert version == 'v24.19.0', version
    output = args.output.resolve()
    assert not output.exists(), 'Use a fresh output path; retain failed checks'
    output.mkdir(mode=0o700, parents=True)
    # Retain the exact attempted source before execution, including failures.
    for row in prepared['files']:
        target = output / 'frozen-inputs' / row['path']
        target.parent.mkdir(mode=0o700, parents=True, exist_ok=True)
        shutil.copyfile(B / row['path'], target)
        target.chmod(0o600)
        assert sha(target) == row['sha256']
    shutil.copyfile(B / 'prepared-files.json', output / 'frozen-inputs/prepared-files.json')
    command = ['node', str(B / 'tests/contract-test.mjs')]
    binding = sha(B / 'prepared-files.json')
    invocation = {'version': 1, 'argv': command, 'cwd': str(B), 'nodeVersion': version,
                  'startedAt': now(), 'wallTimeoutSeconds': 20, 'termGraceSeconds': 5, 'killWaitSeconds': 5,
                  'preparedManifestSHA256': binding, 'dependencyBindingsSHA256': sha(B / 'dependency-bindings.json'),
                  'browserExecuted': False, 'webBuildExecuted': False, 'packageExecuted': False}
    write(output / 'invocation.json', invocation)
    started = time.monotonic()
    timed_out = False
    settlement_unknown = False
    environment = dict(os.environ, NODE_OPTIONS='--max-old-space-size=512')
    with (output / 'stdout.log').open('wb') as stdout, (output / 'stderr.log').open('wb') as stderr:
        child = subprocess.Popen(command, cwd=B, env=environment, stdout=stdout, stderr=stderr, start_new_session=True)
        try:
            code = child.wait(timeout=20)
        except subprocess.TimeoutExpired:
            timed_out = True
            child.send_signal(signal.SIGTERM)
            try:
                code = child.wait(timeout=5)
            except subprocess.TimeoutExpired:
                child.kill()
                try:
                    code = child.wait(timeout=5)
                except subprocess.TimeoutExpired:
                    code = None
                    settlement_unknown = True
    unchanged = True
    try:
        assert sha(B / 'prepared-files.json') == binding
        verify(prepared, dependencies)
    except (AssertionError, OSError):
        unchanged = False
    result = {'version': 1, 'finishedAt': now(), 'elapsedSeconds': time.monotonic() - started,
              'exitCode': code, 'timedOut': timed_out, 'processSettlementUnknown': settlement_unknown,
              'inputsUnchanged': unchanged, 'preparedManifestSHA256': binding,
              'frozenInputFiles': len(prepared['files']) + 1,
              'stdoutSHA256': sha(output / 'stdout.log'), 'stderrSHA256': sha(output / 'stderr.log'),
              'browserExecuted': False, 'webBuildExecuted': False, 'packageExecuted': False,
              'scope': 'Controlled fake DOM/clock contracts only; no actual React/browser commit or performance proof'}
    write(output / 'result.json', result)
    print(json.dumps(result), flush=True)
    return 124 if timed_out or settlement_unknown else 1 if not unchanged else code

if __name__ == '__main__':
    sys.exit(main())

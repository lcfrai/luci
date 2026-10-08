#!/usr/bin/env python3
"""Bounded controlled validation; only run in a root-assigned test window."""
import argparse
import ast
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import signal
import subprocess
import time

REPO = Path('/workspace/luci')
PREFIX = 'scripts/acceptance/check-create-recovery/'
TESTS = [PREFIX + name for name in ['proxy-recovery.test.mjs', 'recovery-proofs.test.mjs',
         'check-guard.test.mjs', 'inspect-guards.test.mjs', 'scenario-flow.test.mjs',
         'cleanup.test.mjs', 'provenance-binding.test.mjs']]
TESTS += ['scripts/acceptance/launch-recovery/acceptance-lifecycle.test.mjs',
          'scripts/acceptance/launch-recovery/acceptance-startup.test.mjs']
RUNTIME = [PREFIX + name for name in ['installed-recovery.mjs', 'recovery-scenario.mjs',
           'fixture-guards.mjs', 'check-create-guard.mjs', 'provenance.mjs', 'run-with-observer.py']]
RUNTIME += ['scripts/acceptance/launch-recovery/' + name for name in ['acceptance-lifecycle.mjs',
            'acceptance-startup.mjs', 'durable-acceptance-journal.mjs', 'fenced-docker-proxy.mjs']]
RUNTIME += ['scripts/acceptance/four-project-queue/sample-linux-owned.py']
SOURCES = sorted(set(TESTS + RUNTIME + [PREFIX + 'wrapper.test.py', PREFIX + 'freeze-source-binding.py']))

def stamp():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()

def sha(data):
    return hashlib.sha256(data).hexdigest()

def snapshot():
    return {name: sha((REPO / name).read_bytes()) for name in SOURCES}

def run(out, name, command, timeout=45):
    record = {'scope': 'controlled local sockets/DTOs/temp-files/tiny Git fixture/mocked wrapper only; no real Docker/browser/installed service/model',
              'command': command, 'cwd': str(REPO), 'timeoutSeconds': timeout, 'startedAt': stamp(), 'sourceSHA256Before': snapshot()}
    began = time.monotonic()
    process = subprocess.Popen(command, cwd=REPO, stdout=subprocess.PIPE, stderr=subprocess.PIPE, start_new_session=True)
    record['processPID'] = process.pid
    record['processStartTicks'] = Path('/proc', str(process.pid), 'stat').read_text().rsplit(') ', 1)[1].split()[19]
    timed_out = False
    try:
        stdout, stderr = process.communicate(timeout=timeout)
    except subprocess.TimeoutExpired:
        timed_out = True
        os.killpg(process.pid, signal.SIGTERM)
        try:
            stdout, stderr = process.communicate(timeout=3)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            stdout, stderr = process.communicate(timeout=3)
    try:
        os.killpg(process.pid, 0)
        group_absent = False
    except ProcessLookupError:
        group_absent = True
    record.update(exitCode=process.returncode, timedOut=timed_out, processReaped=process.returncode is not None,
                  processGroupAbsent=group_absent, wallSeconds=time.monotonic()-began, finishedAt=stamp(), sourceSHA256After=snapshot())
    record['sourceFilesStableDuringRun'] = record['sourceSHA256Before'] == record['sourceSHA256After']
    for stream, data in [('stdout', stdout), ('stderr', stderr)]:
        path = out / (name + '.' + stream + '.log'); path.write_bytes(data)
        record[stream] = {'path': str(path), 'bytes': len(data), 'sha256': sha(data)}
    text = stdout.decode(errors='replace') + '\n' + stderr.decode(errors='replace')
    record['counts'] = {key: int(value) for key, value in re.findall(r'^# (tests|pass|fail|cancelled|skipped|todo) (\d+)$', text, re.M)}
    if name == 'wrapper':
        match = re.search(r'Ran (\d+) tests?', text); record['counts'] = {'tests': int(match.group(1)) if match else None}
    record['failingTests'] = re.findall(r'^(?:not ok|FAIL:|ERROR:)[^\n]*', text, re.M)
    record['passed'] = process.returncode == 0 and not timed_out and group_absent and record['sourceFilesStableDuringRun']
    (out / (name + '.json')).write_text(json.dumps(record, indent=2) + '\n')
    print(json.dumps({key: record[key] for key in ['command', 'passed', 'exitCode', 'timedOut', 'counts', 'failingTests', 'processGroupAbsent']}), flush=True)
    return record['passed']

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root-assigned-test-window', action='store_true', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    out = args.output.absolute(); out.mkdir(parents=True, exist_ok=False)
    (out / 'runner-source.py').write_bytes(Path(__file__).read_bytes())
    overall = run(out, 'node', ['node', '--test', '--test-concurrency=1', '--test-timeout=30000', '--test-reporter=tap', *TESTS])
    overall = run(out, 'wrapper', ['python3', '-B', PREFIX + 'wrapper.test.py']) and overall
    syntax = {'scope': 'parse-only; no module startup', 'startedAt': stamp(), 'sourceSHA256Before': snapshot(), 'checks': []}
    for name in RUNTIME + [PREFIX + 'freeze-source-binding.py', PREFIX + 'wrapper.test.py']:
        if name.endswith('.mjs'):
            command = ['node', '--check', name]
            result = subprocess.run(command, cwd=REPO, capture_output=True, timeout=10)
            syntax['checks'].append({'path': name, 'command': command, 'exitCode': result.returncode,
                                      'stdout': result.stdout.decode(), 'stderr': result.stderr.decode(), 'passed': result.returncode == 0})
        else:
            try:
                ast.parse((REPO / name).read_text(), filename=name)
                syntax['checks'].append({'path': name, 'method': 'Python ast.parse, no import', 'passed': True})
            except SyntaxError as error:
                syntax['checks'].append({'path': name, 'method': 'Python ast.parse, no import', 'passed': False,
                                          'errorType': type(error).__name__, 'line': error.lineno, 'offset': error.offset})
    syntax['sourceSHA256After'] = snapshot(); syntax['finishedAt'] = stamp()
    syntax['passed'] = all(row['passed'] for row in syntax['checks']) and syntax['sourceSHA256Before'] == syntax['sourceSHA256After']
    (out / 'syntax.json').write_text(json.dumps(syntax, indent=2) + '\n')
    overall = overall and syntax['passed']
    print(json.dumps({'allPassed': overall, 'syntaxChecks': len(syntax['checks']), 'output': str(out)}), flush=True)
    raise SystemExit(0 if overall else 1)

if __name__ == '__main__':
    main()

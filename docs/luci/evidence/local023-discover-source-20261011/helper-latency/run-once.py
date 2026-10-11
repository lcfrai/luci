#!/usr/bin/env python3
"""Single authorized helper test; capture only, no repository mutations."""
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import shlex
import shutil
import signal
import subprocess
import time

OUT = Path(__file__).resolve().parent
REPO = Path('/workspace/luci')
TARGET = REPO / 'scripts/acceptance/sustained-four-project/latency-diagnostics.test.mjs'
EXPECTED_HEAD = '6f00501d7f370789ed45b018a7d958fd25cc1e2d'

def stamp():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()

def digest(data):
    return hashlib.sha256(data).hexdigest()

def write_json(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')

def command(argv):
    p = subprocess.run(argv, cwd=REPO, capture_output=True, check=True)
    return p.stdout

def graph():
    pending, seen, edges = [TARGET], set(), []
    pattern = re.compile(r'''(?:\bfrom\s*|\bimport\s*\(\s*|\brequire\s*\(\s*|\bimport\s*)['"]([^'"\n]+)['"]''')
    while pending:
        path = pending.pop().resolve()
        if path in seen:
            continue
        seen.add(path)
        for spec in pattern.findall(path.read_text()):
            edge = {'from': str(path), 'specifier': spec}
            if spec.startswith('.'):
                resolved = (path.parent / spec).resolve()
                assert resolved.is_file(), resolved
                edge['resolved'] = str(resolved)
                pending.append(resolved)
            else:
                edge['kind'] = 'node-builtin' if spec.startswith('node:') else 'external'
                assert spec.startswith('node:'), 'Unexpected external import: ' + spec
            edges.append(edge)
    return sorted(seen), edges

def snapshot(label):
    destination = OUT / label
    destination.mkdir()
    inputs, edges = graph()
    ancestors = [TARGET.parent, *TARGET.parent.parents]
    config_names = ['package.json', 'package-lock.json', 'npm-shrinkwrap.json', '.nvmrc', '.node-version', '.npmrc', 'node.config.json', 'node.config.mjs', '.node-test.json']
    configs, config_search = [], []
    for ancestor in ancestors:
        found = [ancestor / name for name in config_names if (ancestor / name).is_file()]
        configs.extend(found)
        config_search.append({'directory': str(ancestor), 'checked_names': config_names, 'found': [str(p) for p in found]})
    supplemental = [REPO / 'AGENTS.md', Path('/workspace/.luci-tools/activate.sh')]
    files = []
    for path in sorted(set(inputs + configs + supplemental)):
        data = path.read_bytes()
        copy = destination / 'files' / str(path).lstrip('/')
        copy.parent.mkdir(parents=True, exist_ok=True)
        copy.write_bytes(data)
        files.append({'path': str(path), 'bytes': len(data), 'sha256': digest(data), 'copy': str(copy.relative_to(OUT))})
    node = Path(shutil.which('node')).resolve()
    node_data = node.read_bytes()
    node_identity = json.loads(command(['node', '-p', 'JSON.stringify({execPath:process.execPath,version:process.version,versions:process.versions,platform:process.platform,arch:process.arch})']))
    node_identity.update({'resolvedExecutable': str(node), 'sha256': digest(node_data), 'bytes': len(node_data)})
    assert node_identity['version'] == 'v24.19.0', node_identity['version']
    head = command(['git', 'rev-parse', 'HEAD']).decode().strip()
    assert head == EXPECTED_HEAD, head
    status = command(['git', 'status', '--porcelain=v1'])
    diff = command(['git', 'diff', 'HEAD', '--binary', '--no-ext-diff'])
    index_diff = command(['git', 'diff', '--cached', '--binary', '--no-ext-diff'])
    (destination / 'git-status.txt').write_bytes(status)
    receipt = {'capturedAt': stamp(), 'head': head, 'rootDiffSHA256': digest(diff), 'rootDiffBytes': len(diff), 'cachedDiffSHA256': digest(index_diff), 'cachedDiffBytes': len(index_diff), 'gitStatusSHA256': digest(status), 'node': node_identity, 'files': files, 'importGraph': edges, 'configSearch': config_search, 'environmentPresence': {name: name in os.environ for name in ['NODE_OPTIONS', 'NODE_PATH', 'NODE_V8_COVERAGE']}, 'configScope': 'Direct .mjs Node test; no desktop/Vitest configuration is selected. Ancestor package/runtime configuration presence is recorded.'}
    write_json(destination / 'receipt.json', receipt)
    return receipt

lock = OUT / 'RUN_STARTED'
with lock.open('x') as file:
    file.write(stamp() + '\n')
before = snapshot('before')
argv = ['node', '--test', '--test-concurrency=1', '--test-reporter=tap', str(TARGET)]
invocation = {'argv': argv, 'shellDisplay': shlex.join(argv), 'cwd': str(REPO), 'activation': '. /workspace/.luci-tools/activate.sh', 'startUTC': stamp(), 'timeoutSeconds': 115, 'terminationGraceSeconds': 3, 'scope': 'Exactly one controlled helper test; no browser, Docker, services, network, or repository edits requested by this harness.'}
write_json(OUT / 'invocation.json', invocation)
start = time.monotonic()
timed_out = False
with (OUT / 'stdout.tap').open('wb') as stdout, (OUT / 'stderr.txt').open('wb') as stderr:
    process = subprocess.Popen(argv, cwd=REPO, stdout=stdout, stderr=stderr, start_new_session=True)
    invocation['pid'] = process.pid
    write_json(OUT / 'invocation.json', invocation)
    try:
        exit_code = process.wait(timeout=115)
    except subprocess.TimeoutExpired:
        timed_out = True
        os.killpg(process.pid, signal.SIGTERM)
        try:
            exit_code = process.wait(timeout=3)
        except subprocess.TimeoutExpired:
            os.killpg(process.pid, signal.SIGKILL)
            exit_code = process.wait(timeout=1)
end = stamp()
elapsed = time.monotonic() - start
after = snapshot('after')
tap = (OUT / 'stdout.tap').read_text()
counts = {name: int(value) for name, value in re.findall(r'^# (tests|suites|pass|fail|cancelled|skipped|todo) (\d+)\s*$', tap, flags=re.M)}
before_hashes = {v['path']: v['sha256'] for v in before['files']}
after_hashes = {v['path']: v['sha256'] for v in after['files']}
result = {'startUTC': invocation['startUTC'], 'endUTC': end, 'elapsedSeconds': elapsed, 'exitCode': exit_code, 'timedOut': timed_out, 'counts': counts, 'failedTAPLines': re.findall(r'^not ok.*$', tap, flags=re.M), 'sourceAndConfigurationStable': before_hashes == after_hashes, 'nodeIdentityStable': before['node'] == after['node'], 'rootDiffStable': before['rootDiffSHA256'] == after['rootDiffSHA256'], 'gitStatusStable': before['gitStatusSHA256'] == after['gitStatusSHA256'], 'headStable': before['head'] == after['head'], 'stdoutBytes': (OUT / 'stdout.tap').stat().st_size, 'stderrBytes': (OUT / 'stderr.txt').stat().st_size}
write_json(OUT / 'result.json', result)
(OUT / 'RUN_COMPLETE').write_text(stamp() + '\n')
print(json.dumps(result, indent=2))

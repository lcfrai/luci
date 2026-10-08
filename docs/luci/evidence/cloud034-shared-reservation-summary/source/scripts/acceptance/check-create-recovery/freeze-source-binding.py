#!/usr/bin/env python3
"""Freeze exact committed helper bytes before an installed recovery run.

Read-only Git/filesystem validation plus one fresh JSON output. No product,
Docker, browser, model, compiler, test, or service startup.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess

PATHS = [
    'check-create-recovery/installed-recovery.mjs', 'check-create-recovery/recovery-scenario.mjs',
    'check-create-recovery/fixture-guards.mjs', 'check-create-recovery/check-create-guard.mjs',
    'check-create-recovery/provenance.mjs', 'check-create-recovery/run-with-observer.py',
    'launch-recovery/acceptance-lifecycle.mjs', 'launch-recovery/acceptance-startup.mjs',
    'launch-recovery/durable-acceptance-journal.mjs', 'launch-recovery/fenced-docker-proxy.mjs',
    'four-project-queue/sample-linux-owned.py',
]
BASE = '81f2ba5b194a81a4f2ad167e466793f59162b387'
OBSERVER = '5d306b6dae3cc9d0a4d9cbf95f8142c37958964f'

def sha(data):
    return hashlib.sha256(data).hexdigest()

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--repo', type=Path, required=True)
    parser.add_argument('--helper-commit', required=True)
    parser.add_argument('--output', type=Path, required=True)
    args = parser.parse_args()
    if not re.fullmatch('[a-f0-9]{40}', args.helper_commit):
        parser.error('--helper-commit must be an exact commit hash')
    repo = args.repo.resolve(strict=True)
    root = repo / 'scripts/acceptance'
    if root.resolve(strict=True) != root:
        parser.error('Linked acceptance root is refused')
    output = args.output.absolute()
    if output.exists() or output.is_symlink():
        parser.error('--output must be fresh')
    def committed(commit, path):
        return subprocess.check_output(['git', 'show', commit + ':' + path], cwd=repo, timeout=10)
    source = committed(args.helper_commit, 'scripts/acceptance/check-create-recovery/recovery-scenario.mjs').decode()
    pins = {}
    for key, pattern in [('product', '[a-f0-9]{40}'), ('helper', '[a-f0-9]{40}'), ('image', 'sha256:[a-f0-9]{64}')]:
        match = re.search(key + ": '(" + pattern + ")'", source)
        if not match:
            raise ValueError('Committed scenario lacks an exact pin')
        pins[key] = match.group(1)
    rows = []
    for path in sorted(PATHS):
        local = root / path
        stat = local.lstat()
        if local.is_symlink() or local.resolve(strict=True) != local or not local.is_file() or stat.st_nlink != 1:
            raise ValueError('Nonordinary executed helper')
        data = local.read_bytes()
        repository_path = 'scripts/acceptance/' + path
        if data != committed(args.helper_commit, repository_path):
            raise ValueError('Executed helper differs from selected commit: ' + path)
        row = {'path': path, 'repositoryPath': repository_path, 'bytes': len(data), 'sha256': sha(data),
               'origin': {'commit': args.helper_commit, 'path': repository_path}}
        prior = BASE if path.startswith('launch-recovery/') else OBSERVER if path.startswith('four-project-queue/') else None
        if prior:
            before = committed(prior, repository_path)
            row['historicalOrigin'] = {'commit': prior, 'path': repository_path, 'sha256': sha(before),
                                       'relation': 'exact-bytes' if before == data else 'modified-from'}
        rows.append(row)
    binding = {'version': 1, 'sourceRoot': 'scripts/acceptance', 'helperSourceCommit': args.helper_commit,
               'pins': pins, 'status': 'committed-source-binding-not-runtime-evidence', 'executedFiles': rows}
    descriptor = os.open(output, os.O_WRONLY | os.O_CREAT | os.O_EXCL | os.O_NOFOLLOW, 0o600)
    with os.fdopen(descriptor, 'w') as stream:
        json.dump(binding, stream, indent=2); stream.write('\n'); stream.flush(); os.fsync(stream.fileno())
    print(json.dumps({'helperSourceCommit': args.helper_commit, 'files': len(rows), 'bindingSHA256': sha(output.read_bytes())}))

if __name__ == '__main__':
    main()

#!/usr/bin/env python3
"""Apply only the reviewed helper overlay after root assigns integration."""
import argparse
import hashlib
import json
from pathlib import Path
import subprocess

HERE = Path(__file__).resolve().parent
REPO = Path('/workspace/luci')
NAMES = ['README.md', 'adapter-contract.md', 'check-create-guard.mjs', 'check-guard.test.mjs',
         'cleanup.test.mjs', 'fixture-guards.mjs', 'freeze-source-binding.py', 'inspect-guards.test.mjs',
         'installed-recovery.mjs', 'provenance-binding.test.mjs', 'provenance.mjs', 'proxy-recovery.test.mjs',
         'recovery-proofs.test.mjs', 'recovery-scenario.mjs', 'run-with-observer.py', 'scenario-flow.test.mjs',
         'wrapper.test.py']
BASE_PROXY_SHA = '48e97d6d5c915cf80c99f0c79a8fb16261eb219616f878fa1ce103ee2c59303d'

def sha(data):
    return hashlib.sha256(data).hexdigest()

def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--root-assigned-integration', action='store_true', required=True)
    args = parser.parse_args()
    del args
    overlay = HERE / 'integration-overlay/scripts/acceptance'
    source = overlay / 'check-create-recovery'
    target = REPO / 'scripts/acceptance/check-create-recovery'
    proxy = REPO / 'scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs'
    status = subprocess.check_output(['git', 'status', '--porcelain=v1'], cwd=REPO, timeout=10)
    if status:
        raise ValueError('Root checkout must be clean before the assigned integration')
    if target.exists() or target.is_symlink():
        raise ValueError('Integration destination already exists')
    if sorted(path.name for path in source.iterdir()) != sorted(NAMES):
        raise ValueError('The reviewed integration set is exactly17 files')
    if sha(proxy.read_bytes()) != BASE_PROXY_SHA:
        raise ValueError('Shared proxy baseline changed; review rather than overwrite')
    # Common helpers remain in place, and must match the reference bytes tested.
    for name in ['launch-recovery/acceptance-lifecycle.mjs', 'launch-recovery/acceptance-startup.mjs',
                 'launch-recovery/durable-acceptance-journal.mjs', 'four-project-queue/sample-linux-owned.py']:
        if (overlay / name).read_bytes() != (REPO / 'scripts/acceptance' / name).read_bytes():
            raise ValueError('Shared source changed since preparation: ' + name)
    content = {name: (source / name).read_bytes() for name in NAMES}
    patched = (overlay / 'launch-recovery/fenced-docker-proxy.mjs').read_bytes()
    before = subprocess.check_output(['git', 'rev-parse', 'HEAD'], cwd=REPO, timeout=10).decode().strip()
    target.mkdir()
    for name, data in content.items():
        with (target / name).open('xb') as output:
            output.write(data)
    proxy.write_bytes(patched)
    receipt = {'scope': '17 new acceptance helper files and one existing opt-in proxy; no product edits or commit',
               'headBefore': before, 'proxyBeforeSHA256': BASE_PROXY_SHA, 'proxyAfterSHA256': sha(patched),
               'newFiles': [{'path': 'scripts/acceptance/check-create-recovery/' + name, 'bytes': len(data), 'sha256': sha(data)}
                            for name, data in sorted(content.items())]}
    with (HERE / 'integration-applied.json').open('x') as output:
        json.dump(receipt, output, indent=2); output.write('\n')
    print(json.dumps({'newFiles': len(content), 'modifiedExistingFiles': 1, 'committed': False}))

if __name__ == '__main__':
    main()

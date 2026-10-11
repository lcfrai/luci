#!/usr/bin/env python3
"""Explicitly invoked, bounded official build at a published immutable pin.

This external wrapper never changes the original checkout, starts Luci, or
touches Docker. Product admission floors are unrelated and remain unchanged.
"""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import re
import runpy
import shutil
import signal
import subprocess
import sys
import time

BASE = Path(__file__).resolve().parent
SOURCE = BASE / 'source'
RECEIPTS = BASE / 'receipts'
ORIGINAL = Path('/workspace/luci')
ROOT_FLOOR = 1536 << 20
TMP_FLOOR = 384 << 20
NOW = lambda: datetime.datetime.now(datetime.timezone.utc).isoformat()


def save(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n')


def digest(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def git(*args, cwd=SOURCE):
    return subprocess.check_output(['git', '-C', str(cwd), *args], timeout=120)


def manifest(root):
    paths = git('ls-files', '-z', cwd=root).split(b'\0')
    result = []
    for raw in paths:
        if not raw:
            continue
        relative = os.fsdecode(raw)
        path = root / relative
        if path.is_symlink() or not path.is_file():
            raise ValueError('Nonordinary tracked source: ' + relative)
        result.append({'path': relative, 'bytes': path.stat().st_size, 'sha256': digest(path)})
    return result


def disk():
    return {'workspace_free_bytes': shutil.disk_usage('/workspace').free,
            'tmp_free_bytes': shutil.disk_usage('/tmp').free}


def require_space():
    row = disk()
    if row['workspace_free_bytes'] < ROOT_FLOOR or row['tmp_free_bytes'] < TMP_FLOOR:
        raise ValueError('Packaging free-space floor unavailable: ' + json.dumps(row))
    return row


def existing_archives():
    values = []
    for release in (ORIGINAL / 'artifacts/releases/0.5.0',
                    Path('/workspace/.luci-tools/package-80462898/source/artifacts/releases/0.5.0')):
        for path in sorted(release.glob('*.tar.gz')):
            values.append({'path': str(path), 'bytes': path.stat().st_size, 'sha256': digest(path)})
    return values


def captured_command(name, argv, cwd=BASE, timeout=180):
    with (RECEIPTS / (name + '.stdout')).open('wb') as out, (RECEIPTS / (name + '.stderr')).open('wb') as err:
        result = subprocess.run(argv, cwd=cwd, stdout=out, stderr=err, timeout=timeout)
    save(RECEIPTS / (name + '.json'), {'argv': argv, 'cwd': str(cwd), 'exitCode': result.returncode})
    if result.returncode:
        raise RuntimeError(name + ' failed with exit ' + str(result.returncode))


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--commit', required=True)
    args = parser.parse_args()
    if not re.fullmatch(r'[0-9a-f]{40}', args.commit):
        raise ValueError('Expected an exact 40-character commit')
    if SOURCE.exists() or RECEIPTS.exists():
        raise ValueError('Fresh invocation required; preserve prior receipts/source unchanged')
    RECEIPTS.mkdir(mode=0o700)
    save(RECEIPTS / 'preflight.json', {'startedUTC': NOW(), 'sourceCommit': args.commit,
          'disk': require_space(), 'workspaceFloorBytes': ROOT_FLOOR, 'tmpFloorBytes': TMP_FLOOR,
          'runtimeProductFloorsChanged': False, 'serviceExecuted': False, 'dockerInvoked': False})
    old = existing_archives()
    save(RECEIPTS / 'previous-release-before.json', old)
    captured_command('clone', ['git', 'clone', '--shared', '--no-checkout', str(ORIGINAL), str(SOURCE)])
    captured_command('checkout', ['git', '-C', str(SOURCE), 'checkout', '--detach', args.commit])
    captured_command('official-origin', ['git', '-C', str(SOURCE), 'remote', 'set-url', 'origin',
                                        'https://github.com/lcfrai/luci.git'])
    captured_command('tracking-fetch', ['git', '-C', str(SOURCE), 'fetch', '--no-tags', 'origin',
        '+refs/heads/codex/luci-production-runs:refs/remotes/origin/codex/luci-production-runs'])
    if git('rev-parse', 'HEAD').decode().strip() != args.commit:
        raise ValueError('Checkout pin mismatch')
    parity = []
    for name in ('package.json', 'package-lock.json'):
        source, original = SOURCE / 'desktop' / name, ORIGINAL / 'desktop' / name
        source_hash, original_hash = digest(source), digest(original)
        if source_hash != original_hash:
            raise ValueError('Reused desktop dependency metadata mismatch: ' + name)
        parity.append({'path': 'desktop/' + name, 'sha256': source_hash, 'bytes': source.stat().st_size})
    (SOURCE / 'desktop/node_modules').symlink_to(ORIGINAL / 'desktop/node_modules', target_is_directory=True)
    save(RECEIPTS / 'dependency-reuse.json', {'ignoredSymlink': str(SOURCE / 'desktop/node_modules'),
         'target': str(ORIGINAL / 'desktop/node_modules'), 'matchingRepositoryInputs': parity,
         'completeInstalledByteInventoryClaimed': False, 'dependencyInstallationRun': False})
    (BASE / 'go.work').write_text('go 1.25.0\n\nuse (\n\t' + str(SOURCE / 'control-plane') + '\n\t' + str(SOURCE / 'sandboxd') + '\n)\n')
    shutil.copyfile('/workspace/.luci-tools/go.work.sum', BASE / 'go.work.sum')
    before = manifest(SOURCE)
    save(RECEIPTS / 'tracked-inputs-before.json', before)
    if git('status', '--porcelain=v1', '--untracked-files=all').strip():
        raise ValueError('Checkout is dirty before build')
    staging = runpy.run_path(str(SOURCE / 'scripts/stage-release-docs.py'))
    association = staging['official_association'](SOURCE, args.commit)
    save(RECEIPTS / 'documentation-preflight.json', {'officialCachedAssociation': association})
    if not association:
        raise ValueError('Compact documentation association unavailable; no full fallback build attempted')
    for name in ('go-tmp', 'tmp'):
        (BASE / name).mkdir(mode=0o700)
    overrides = {
        'PATH': '/workspace/.luci-tools/bin:/workspace/.luci-tools/go1.26.3/bin:' + os.environ['PATH'],
        'GOMAXPROCS': '2', 'GOFLAGS': '-mod=readonly -p=1', 'GOTOOLCHAIN': 'local',
        'GOPROXY': 'off', 'GOWORK': str(BASE / 'go.work'),
        'GOCACHE': '/tmp/luci-go-build-cache', 'GOTMPDIR': str(BASE / 'go-tmp'),
        'TMPDIR': str(BASE / 'tmp'), 'npm_config_cache': '/tmp/luci-npm-cache-recovery',
        'GOMODCACHE': '/tmp/luci-retained-tools/go-mod', 'GOPATH': '/tmp/luci-gopath',
        'PYTHONDONTWRITEBYTECODE': '1',
    }
    argv = ['bash', 'scripts/build-release.sh', '0.5.0', 'linux-amd64']
    require_space()
    start = time.monotonic()
    save(RECEIPTS / 'build-invocation.json', {'argv': argv, 'cwd': str(SOURCE),
        'environmentOverrides': overrides, 'startedUTC': NOW(), 'sourceCommit': args.commit,
        'workspaceFloorBytes': ROOT_FLOOR, 'tmpFloorBytes': TMP_FLOOR,
        'wallTimeoutSeconds': 1800, 'terminationGraceSeconds': 15})
    samples, reason, forced = [], None, False
    with (RECEIPTS / 'build.stdout').open('wb') as out, (RECEIPTS / 'build.stderr').open('wb') as err:
        process = subprocess.Popen(argv, cwd=SOURCE, env={**os.environ, **overrides},
                                   stdout=out, stderr=err, start_new_session=True)
        save(RECEIPTS / 'build-process.json', {'pid': process.pid, 'processGroup': process.pid,
             'procStat': Path('/proc/' + str(process.pid) + '/stat').read_text()})
        while process.poll() is None:
            row = {**disk(), 'elapsedSeconds': round(time.monotonic() - start, 3)}
            samples.append(row)
            save(RECEIPTS / 'build-progress.json', row)
            if row['workspace_free_bytes'] < ROOT_FLOOR or row['tmp_free_bytes'] < TMP_FLOOR:
                reason = 'free-space floor reached'
            elif row['elapsedSeconds'] > 1800:
                reason = 'wall timeout reached'
            if reason:
                os.killpg(process.pid, signal.SIGTERM)
                try:
                    process.wait(timeout=15)
                except subprocess.TimeoutExpired:
                    forced = True
                    os.killpg(process.pid, signal.SIGKILL)
                    process.wait(timeout=15)
                break
            time.sleep(5)
        code = process.wait()
    save(RECEIPTS / 'build-disk-samples.json', samples)
    result = {'exitCode': code, 'stopReason': reason, 'forced': forced, 'finishedUTC': NOW(),
              'elapsedSeconds': round(time.monotonic() - start, 3), 'sourceCommit': args.commit,
              'minimumWorkspaceFreeBytes': min(x['workspace_free_bytes'] for x in samples),
              'minimumTmpFreeBytes': min(x['tmp_free_bytes'] for x in samples)}
    save(RECEIPTS / 'build-result.json', result)
    after = manifest(SOURCE)
    save(RECEIPTS / 'tracked-inputs-after.json', after)
    old_after = existing_archives()
    save(RECEIPTS / 'previous-release-after.json', old_after)
    source_clean = not git('status', '--porcelain=v1', '--untracked-files=all').strip()
    preserved = {'pinnedCommit': args.commit, 'trackedFileCount': len(before),
                 'allTrackedHashesUnchanged': before == after, 'sourceClean': source_clean,
                 'original577and8046ArchivesUnchanged': old == old_after}
    save(RECEIPTS / 'source-preservation.json', preserved)
    print(json.dumps({'build': result, 'preservation': preserved}), flush=True)
    if code or reason or before != after or not source_clean or old != old_after:
        return 1
    save(BASE / 'verify-config.json', {'source': str(SOURCE), 'commit': args.commit})
    return 0


if __name__ == '__main__':
    try:
        sys.exit(main())
    except Exception as error:
        if RECEIPTS.exists():
            save(RECEIPTS / 'wrapper-error.json', {'errorType': type(error).__name__, 'error': str(error),
                                                 'recordedUTC': NOW()})
        raise

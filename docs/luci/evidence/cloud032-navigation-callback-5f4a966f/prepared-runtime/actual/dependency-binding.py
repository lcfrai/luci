#!/usr/bin/env python3
"""Capture or verify only the two installed Playwright package trees.

No package code is imported. Capture uses Node's built-in resolver only, then
copies ordinary files. Verify reads no snapshot files and copies no files.
"""
import argparse
import datetime
import hashlib
import json
import os
from pathlib import Path
import stat
import subprocess
import sys

ROOTS = {
    'playwright': '/workspace/luci/desktop/node_modules/playwright',
    'playwright-core': '/workspace/luci/desktop/node_modules/playwright-core',
}
LIMITS = {'files': 10000, 'bytes': 128 * 1024 * 1024,
          'directories': 10000, 'manifestBytes': 16 * 1024 * 1024}
SCHEMA = 'luci.playwright-dependency-binding.v1'


def now():
    return datetime.datetime.now(datetime.timezone.utc).isoformat()


def digest(data):
    return hashlib.sha256(data).hexdigest()


def canonical(value):
    return json.dumps(value, sort_keys=True, separators=(',', ':')).encode()


def write_new(path, value):
    path = Path(path)
    with path.open('x', encoding='utf-8') as handle:
        json.dump(value, handle, indent=2, sort_keys=True)
        handle.write('\n')


def require_external_destination(path):
    target = Path(path).resolve(strict=False)
    for configured in ROOTS.values():
        for root in (Path(configured), Path(configured).resolve(strict=False)):
            if target == root or root in target.parents:
                raise ValueError('Output inside installed dependency tree refused')
    return target


def signature(value):
    return [value.st_dev, value.st_ino, value.st_mode, value.st_size,
            value.st_mtime_ns, value.st_ctime_ns]


def read_json(path):
    with Path(path).open('rb') as handle:
        data = handle.read(LIMITS['manifestBytes'] + 1)
    if len(data) > LIMITS['manifestBytes']:
        raise ValueError('Manifest exceeds declared byte limit')
    return json.loads(data), digest(data)


def resolve_packages():
    # resolve() inspects package metadata; it does not execute package modules.
    code = """const {createRequire}=require('node:module');
const req=createRequire('/workspace/luci/desktop/package.json');
const pw=req.resolve('playwright/package.json');
const fromPW=createRequire(pw);
process.stdout.write(JSON.stringify({nodeVersion:process.version,
playwright:{packageJSON:pw,requireEntrypoint:req.resolve('playwright')},
'playwright-core':{packageJSON:fromPW.resolve('playwright-core/package.json'),
requireEntrypoint:fromPW.resolve('playwright-core')}}));"""
    completed = subprocess.run(['node', '-e', code], check=True,
                               capture_output=True, text=True, timeout=10)
    result = json.loads(completed.stdout)
    for name, configured in ROOTS.items():
        root = Path(configured).resolve(strict=True)
        if Path(result[name]['packageJSON']).resolve().parent != root:
            raise ValueError('Resolved package root differs: ' + name)
        result[name]['root'] = str(root)
    return result


def scan(roots, snapshot=None, configured_roots=ROOTS):
    files, directories, states, root_records = {}, {}, {}, {}
    totals = {'files': 0, 'bytes': 0, 'directories': 0}
    package_json = {}

    def visit(fd, package, relative, destination):
        start = os.fstat(fd)
        key = package + ('/' + relative if relative else '')
        directories[key] = {'mode': stat.S_IMODE(start.st_mode)}
        states[key + '/'] = signature(start)
        totals['directories'] += 1
        if totals['directories'] > LIMITS['directories']:
            raise ValueError('Directory limit exceeded')
        names = sorted(os.listdir(fd))
        for name in names:
            if name in ('.', '..') or '/' in name or '\x00' in name:
                raise ValueError('Invalid directory member')
            rel = relative + '/' + name if relative else name
            member = package + '/' + rel
            before = os.stat(name, dir_fd=fd, follow_symlinks=False)
            if stat.S_ISLNK(before.st_mode):
                raise ValueError('Inner symlink refused: ' + member)
            if stat.S_ISDIR(before.st_mode):
                child = os.open(name, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW,
                                dir_fd=fd)
                try:
                    if signature(os.fstat(child)) != signature(before):
                        raise ValueError('Directory changed while opening: ' + member)
                    target = destination / name if destination is not None else None
                    if target is not None:
                        target.mkdir(mode=0o700)
                    visit(child, package, rel, target)
                    if target is not None:
                        target.chmod(stat.S_IMODE(before.st_mode))
                finally:
                    os.close(child)
            elif stat.S_ISREG(before.st_mode):
                totals['files'] += 1
                totals['bytes'] += before.st_size
                if totals['files'] > LIMITS['files'] or totals['bytes'] > LIMITS['bytes']:
                    raise ValueError('Ordinary-file inventory limit exceeded')
                source_fd = os.open(name, os.O_RDONLY | os.O_NOFOLLOW, dir_fd=fd)
                output = None
                try:
                    if signature(os.fstat(source_fd)) != signature(before):
                        raise ValueError('File changed while opening: ' + member)
                    if destination is not None:
                        output = (destination / name).open('xb')
                    sha = hashlib.sha256()
                    size = 0
                    metadata = bytearray() if rel == 'package.json' else None
                    while True:
                        chunk = os.read(source_fd, 1024 * 1024)
                        if not chunk:
                            break
                        size += len(chunk)
                        if size > before.st_size:
                            raise ValueError('File grew while reading: ' + member)
                        sha.update(chunk)
                        if output is not None:
                            output.write(chunk)
                        if metadata is not None:
                            metadata.extend(chunk)
                    if size != before.st_size or signature(os.fstat(source_fd)) != signature(before):
                        raise ValueError('File changed while reading: ' + member)
                    if signature(os.stat(name, dir_fd=fd, follow_symlinks=False)) != signature(before):
                        raise ValueError('File replaced while reading: ' + member)
                    files[member] = {'bytes': size, 'sha256': sha.hexdigest(),
                                     'mode': stat.S_IMODE(before.st_mode)}
                    states[member] = signature(before)
                    if metadata is not None:
                        package_json[package] = json.loads(metadata)
                finally:
                    if output is not None:
                        output.close()
                        (destination / name).chmod(stat.S_IMODE(before.st_mode))
                    os.close(source_fd)
            else:
                raise ValueError('Non-ordinary member refused: ' + member)
        if sorted(os.listdir(fd)) != names or signature(os.fstat(fd)) != signature(start):
            raise ValueError('Directory changed during traversal: ' + key)

    for package, configured in configured_roots.items():
        resolved = str(Path(configured).resolve(strict=True))
        if roots[package] != resolved:
            raise ValueError('Configured root resolved elsewhere: ' + package)
        before = os.stat(resolved, follow_symlinks=False)
        if not stat.S_ISDIR(before.st_mode):
            raise ValueError('Package root is not a directory: ' + package)
        fd = os.open(resolved, os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW)
        try:
            if signature(os.fstat(fd)) != signature(before):
                raise ValueError('Package root changed while opening: ' + package)
            destination = Path(snapshot) / package if snapshot else None
            if destination is not None:
                destination.mkdir(mode=0o700)
            visit(fd, package, '', destination)
            if destination is not None:
                destination.chmod(stat.S_IMODE(before.st_mode))
        finally:
            os.close(fd)
        if str(Path(configured).resolve(strict=True)) != resolved or signature(os.stat(resolved)) != signature(before):
            raise ValueError('Package root changed during traversal: ' + package)
        root_records[package] = {'configuredPath': configured, 'resolvedRoot': resolved,
                                 'stat': signature(before)}
    return {'files': files, 'directories': directories, 'states': states,
            'roots': root_records, 'totals': totals, 'packageJSON': package_json}


def inventory(value):
    return {'files': value['files'], 'directories': value['directories']}


def stable_scan(roots, snapshot=None):
    first = scan(roots, snapshot)
    second = scan(roots)
    if first != second:
        raise ValueError('Installed dependencies changed between complete capture/read passes')
    return first


def capture(destination):
    destination = require_external_destination(destination)
    destination.mkdir(mode=0o700)
    started = now()
    resolved = resolve_packages()
    roots = {name: resolved[name]['root'] for name in ROOTS}
    snapshot = destination / 'snapshot'
    snapshot.mkdir(mode=0o700)
    value = stable_scan(roots, snapshot)
    snapshot_roots = {name: str((snapshot / name).resolve(strict=True)) for name in ROOTS}
    captured = scan(snapshot_roots, configured_roots=snapshot_roots)
    if inventory(captured) != inventory(value):
        raise ValueError('Preserved snapshot differs from installed inventory')
    packages = {}
    for name in ROOTS:
        metadata = value['packageJSON'][name]
        if metadata['name'] != name:
            raise ValueError('Package name mismatch: ' + name)
        exports = metadata['exports']['.']
        import_relative = exports['import']
        if import_relative.startswith('./'):
            import_relative = import_relative[2:]
        if name + '/' + import_relative not in value['files']:
            raise ValueError('Import entrypoint missing from inventory: ' + name)
        packages[name] = {**value['roots'][name], 'version': metadata['version'],
                          'license': metadata.get('license'),
                          'packageJSON': resolved[name]['packageJSON'],
                          'requireEntrypoint': resolved[name]['requireEntrypoint'],
                          'importEntrypoint': str(Path(roots[name]) / import_relative),
                          'snapshotRoot': 'snapshot/' + name}
    manifest = {'schema': SCHEMA, 'startedAt': started, 'finishedAt': now(),
                'limits': LIMITS, 'packages': packages, **inventory(value),
                'totals': value['totals'], 'nodeResolverVersion': resolved['nodeVersion'],
                'sourceStateAtCapture': value['states'],
                'snapshotIndependentlyRehashed': True,
                'inventorySHA256': digest(canonical(inventory(value))),
                'scope': 'Current installed package bytes only; no retrospective historic-run claim. Browser binaries and unrelated dependencies are excluded.',
                'capture': 'Two complete equal content/metadata passes; descriptor-relative ordinary-file reads; inner symlinks and special files refused.'}
    path = destination / 'manifest.json'
    write_new(path, manifest)
    manifest_sha = digest(path.read_bytes())
    print(json.dumps({'manifest': str(path), 'manifestSHA256': manifest_sha,
                      'inventorySHA256': manifest['inventorySHA256'], **value['totals']}))


def compare_members(expected, observed):
    missing = sorted(set(expected) - set(observed))
    extra = sorted(set(observed) - set(expected))
    changed = [{'path': path, 'expected': expected[path], 'observed': observed[path]}
               for path in sorted(set(expected) & set(observed)) if expected[path] != observed[path]]
    return {'missing': missing, 'extra': extra, 'changed': changed}


def verify(manifest_path, output):
    output = require_external_destination(output)
    if Path(output).exists():
        raise ValueError('Verification output must be a fresh path')
    report = {'schema': SCHEMA + '.verification', 'startedAt': now(),
              'passed': False, 'scope': 'Read-only installed dependency verification; no file copies or dependency mutations.'}
    try:
        manifest, sha = read_json(manifest_path)
        report['manifestSHA256'] = sha
        if manifest.get('schema') != SCHEMA or manifest.get('limits') != LIMITS:
            raise ValueError('Unsupported manifest schema or limits')
        if set(manifest['packages']) != set(ROOTS):
            raise ValueError('Manifest package allowlist differs')
        for name, configured in ROOTS.items():
            if manifest['packages'][name]['configuredPath'] != configured:
                raise ValueError('Manifest configured root differs: ' + name)
        expected = inventory(manifest)
        if len(expected['files']) > LIMITS['files'] or sum(row['bytes'] for row in expected['files'].values()) > LIMITS['bytes']:
            raise ValueError('Manifest inventory exceeds limits')
        if digest(canonical(expected)) != manifest['inventorySHA256']:
            raise ValueError('Manifest inventory digest inconsistent')
        roots = {name: manifest['packages'][name]['resolvedRoot'] for name in ROOTS}
        value = stable_scan(roots)
        observed = inventory(value)
        report['expectedInventorySHA256'] = manifest['inventorySHA256']
        report['observedInventorySHA256'] = digest(canonical(observed))
        report['observedTotals'] = value['totals']
        report['packages'] = {name: {'configuredPath': ROOTS[name], 'resolvedRoot': roots[name],
                                     'version': value['packageJSON'][name].get('version')}
                              for name in ROOTS}
        report['files'] = compare_members(expected['files'], observed['files'])
        report['directories'] = compare_members(expected['directories'], observed['directories'])
        report['passed'] = expected == observed
        report['stableAcrossTwoReadPasses'] = True
    except Exception as error:
        report['error'] = str(error)
    report['finishedAt'] = now()
    write_new(output, report)
    print(json.dumps({'passed': report['passed'], 'output': str(output),
                      'manifestSHA256': report.get('manifestSHA256')}))
    return 0 if report['passed'] else 1


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='command', required=True)
    command = commands.add_parser('capture')
    command.add_argument('destination')
    command = commands.add_parser('verify')
    command.add_argument('manifest')
    command.add_argument('fresh_output')
    args = parser.parse_args()
    if args.command == 'capture':
        capture(args.destination)
        return 0
    return verify(args.manifest, args.fresh_output)


if __name__ == '__main__':
    try:
        sys.exit(main())
    except Exception as error:
        print(json.dumps({'error': str(error), 'passed': False}), file=sys.stderr)
        sys.exit(1)

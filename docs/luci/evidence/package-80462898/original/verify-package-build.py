#!/usr/bin/env python3
"""Statically verify the pinned Linux release; never start its service binary."""

import argparse
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import runpy
import shutil
import subprocess
import sys
import tarfile
import tempfile


SOURCE = Path('/workspace/.luci-tools/package-80462898/source')
COMMIT = '80462898735a191cceaa4cb33e486b42b0a35f3c'
VERSION = '0.5.0'
CLOUD = 'luci-cloud-linux-amd64.tar.gz'
DESKTOP = 'luci-linux-amd64.tar.gz'
CAPTURE = {'licenses/manifest.json': 16 << 20,
           'licenses/MISSING-LICENSES.txt': 1 << 20,
           'docs/PACKAGE-VALIDATION.md': 1 << 20,
           'package-identity': 128}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def ordinary_chain(path):
    path = Path(os.path.abspath(path))
    for part in (path, *path.parents):
        if part.exists() or part.is_symlink():
            require(not part.is_symlink(), f'Linked path refused: {part}')
    return path


def digest_file(path):
    with path.open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def safe_name(value):
    require(isinstance(value, str) and value and '\\' not in value and
            not any(ord(c) < 32 for c in value), 'Unsafe archive or inventory path')
    normalized = value[:-1] if value.endswith('/') else value
    path = PurePosixPath(normalized)
    require(not path.is_absolute() and path.parts and
            all(part not in ('.', '..') for part in path.parts) and
            path.as_posix() == normalized, f'Noncanonical package path: {value}')
    return normalized


def read_archive(path, expected_sha, binary_to=None, extraction=None):
    require(digest_file(path) == expected_sha, f'Archive changed before reading: {path.name}')
    records, captured, parent_paths = {}, {}, set()
    with tarfile.open(path, 'r|gz') as archive:
        for member in archive:
            name = safe_name(member.name)
            require(name not in records, f'Duplicate archive member: {name}')
            require(member.type in (tarfile.REGTYPE, tarfile.AREGTYPE, tarfile.DIRTYPE)
                    and not member.sparse, f'Linked, sparse or special member: {name}')
            directory = member.isdir()
            require(member.size >= 0 and (not directory or member.size == 0),
                    f'Invalid member size: {name}')
            expected_mode = 0o755 if directory or name in ('luci', 'start-luci.sh') or name.endswith('.sh') else 0o644
            require(member.mode == expected_mode and member.uid == member.gid == 0 and
                    member.uname == member.gname == '', f'Unexpected package modes/owner: {name}')
            for parent in PurePosixPath(name).parents:
                key = parent.as_posix()
                if key in records:
                    require(records[key]['directory'], f'File used as directory: {key}')
                parent_paths.add(key)
            if not directory:
                require(name not in parent_paths,
                        f'File replaces an archive directory: {name}')
            row = {'directory': directory, 'bytes': member.size, 'mode': oct(member.mode)}
            records[name] = row
            destination = extraction.joinpath(*PurePosixPath(name).parts) if extraction else None
            if directory:
                if destination:
                    destination.mkdir(parents=True, exist_ok=True)
                    destination.chmod(expected_mode)
                continue
            if name in CAPTURE:
                require(member.size <= CAPTURE[name], f'Oversized metadata: {name}')
            save_to = destination or (binary_to if name == 'luci' else None)
            writer = None
            if save_to:
                save_to.parent.mkdir(parents=True, exist_ok=True)
                writer = save_to.open('xb')
            content = bytearray() if name in CAPTURE else None
            hashed, total = hashlib.sha256(), 0
            try:
                with archive.extractfile(member) as stream:
                    while block := stream.read(1 << 20):
                        total += len(block)
                        require(total <= member.size, f'Oversized member data: {name}')
                        hashed.update(block)
                        if content is not None:
                            content.extend(block)
                        if writer:
                            writer.write(block)
            finally:
                if writer:
                    writer.close()
            require(total == member.size, f'Truncated member: {name}')
            row['sha256'] = hashed.hexdigest()
            if content is not None:
                captured[name] = bytes(content)
            if destination:
                destination.chmod(expected_mode)
    require(digest_file(path) == expected_sha, f'Archive changed while reading: {path.name}')
    require(set(CAPTURE) <= set(captured), f'Package metadata missing: {path.name}')
    require('luci' in records and not records['luci']['directory'] and
            'web/index.html' in records and not records['web/index.html']['directory'],
            'Service binary or complete web assets missing')
    identity = hashlib.sha256(b'luci-portable-package-v1\n')
    # package.py sorts Path objects, which order components, not slash strings.
    ordered = ['luci'] + [str(p) for p in sorted(PurePosixPath(name) for name, row in records.items()
                                                if name.startswith('web/') and not row['directory'])]
    for name in ordered:
        identity.update(name.encode('utf-8') + b'\0' + bytes.fromhex(records[name]['sha256']))
    package_identity = captured['package-identity'].decode('ascii')
    require(package_identity == identity.hexdigest() + '\n', 'Package identity differs from binary/web bytes')
    licenses = json.loads(captured['licenses/manifest.json'])
    require(licenses.get('schemaVersion') == 1 and licenses.get('components') and
            licenses.get('missingLicenses') == [], 'License inventory is incomplete')
    require(captured['licenses/MISSING-LICENSES.txt'] == b'', 'Missing-license notice is not empty')
    license_files = 0
    for component in licenses['components']:
        require(component.get('missingLicense') is False and component.get('files'),
                'Component lacks retained license evidence')
        items = list(component['files'])
        if component.get('sourceArchive'):
            require(component['sourceArchive'].get('modified') is False, 'Unexpected modified retained source')
            items.append(component['sourceArchive'])
        for item in items:
            name = 'licenses/' + safe_name(item['path'])
            require(re.fullmatch(r'[0-9a-f]{64}', item.get('sha256', '')) is not None and
                    name in records and not records[name]['directory'] and
                    records[name]['sha256'] == item['sha256'], f'License/source hash mismatch: {name}')
            license_files += 1
    guide = captured['docs/PACKAGE-VALIDATION.md'].decode('utf-8')
    compact = 'Runtime packages omit the validation archive to limit download and installation size.' in guide
    full = 'The complete documentation and validation tree is retained (' in guide
    require(compact != full, 'Unrecognized documentation staging mode')
    evidence = [name for name in records if name.startswith('docs/luci/evidence/')]
    if compact:
        require(f'This package was built from `{COMMIT}`' in guide and
                f'https://github.com/lcfrai/luci/tree/{COMMIT}/docs/luci/evidence' in guide and
                not evidence, 'Compact documentation source or evidence boundary differs')
    else:
        require(evidence and 'No compact documentation transformation is claimed' in guide,
                'Full documentation fallback lacks its retained evidence')
    summary = {'name': path.name, 'sha256': expected_sha, 'bytes': path.stat().st_size,
               'members': len(records), 'unpackedFileBytes': sum(r['bytes'] for r in records.values()),
               'binarySHA256': records['luci']['sha256'], 'webIndexSHA256': records['web/index.html']['sha256'],
               'packageIdentity': identity.hexdigest(), 'licenseComponents': len(licenses['components']),
               'verifiedLicenseAndSourceFiles': license_files, 'missingLicenses': [],
               'documentationMode': 'compact' if compact else 'full', 'evidenceMembers': len(evidence)}
    return summary, records, captured


def save_json(path, value):
    path.write_text(json.dumps(value, indent=2) + '\n', encoding='utf-8')


def verify(args, receipt):
    release = ordinary_chain(args.release_dir)
    source = ordinary_chain(SOURCE)
    revision = subprocess.check_output(['git', '-C', str(source), 'rev-parse', 'HEAD'], text=True).strip()
    require(revision == COMMIT, 'Verifier source checkout is not the pinned commit')
    status = subprocess.check_output(['git', '-C', str(source), 'status', '--porcelain=v1', '--untracked-files=all'], text=True)
    require(status == '', 'Verifier source checkout changed after packaging')
    official = runpy.run_path(str(source / 'scripts/release-integrity.py'))
    measured = official['verify_group'](release, 'linux-amd64', VERSION, COMMIT)
    save_json(receipt / 'official-release-integrity.json', measured)
    manifest = json.loads((release / 'release-manifest.json').read_text())
    (receipt / 'release-manifest.json').write_bytes((release / 'release-manifest.json').read_bytes())
    (receipt / 'SHA256SUMS').write_bytes((release / 'SHA256SUMS').read_bytes())
    expected = {row['name']: row for row in manifest['artifacts']}
    require(set(expected) == {CLOUD, DESKTOP}, 'Linux invocation has unexpected archive subjects')
    summaries, inventories, captures = {}, {}, {}
    for name in (CLOUD, DESKTOP):
        summary, records, captured = read_archive(release / name, expected[name]['sha256'],
                                                  binary_to=receipt / 'luci' if name == CLOUD else None)
        summaries[name], inventories[name], captures[name] = summary, records, captured
        save_json(receipt / (name + '.members.json'), records)
        (receipt / (name + '.PACKAGE-VALIDATION.md')).write_bytes(captured['docs/PACKAGE-VALIDATION.md'])
        (receipt / (name + '.licenses.json')).write_bytes(captured['licenses/manifest.json'])
    cloud, desktop = summaries[CLOUD], summaries[DESKTOP]
    require(cloud['binarySHA256'] == desktop['binarySHA256'], 'Cloud and desktop binaries differ')
    require(cloud['packageIdentity'] == desktop['packageIdentity'], 'Cloud and desktop runtime identities differ')
    require(captures[CLOUD] == captures[DESKTOP], 'Cloud and desktop metadata differs')
    nested = inventories[DESKTOP].get('cloud/' + CLOUD, {})
    require(nested.get('sha256') == cloud['sha256'] and nested.get('bytes') == cloud['bytes'],
            'Nested cloud archive differs from standalone archive')
    license_counts = manifest['portableLicenseComponents']
    require(license_counts == [{'target': 'luci-linux-amd64', 'count': desktop['licenseComponents'], 'missing': 0}],
            'Release license counts differ from shipped inventory')
    go = shutil.which('go')
    require(go is not None, 'Go executable is required to inspect static binary build information')
    command = [go, 'version', '-m', str(receipt / 'luci')]
    buildinfo = subprocess.run(command, check=False, capture_output=True, text=True, timeout=30)
    (receipt / 'binary-buildinfo.stdout.txt').write_text(buildinfo.stdout)
    (receipt / 'binary-buildinfo.stderr.txt').write_text(buildinfo.stderr)
    save_json(receipt / 'binary-buildinfo.invocation.json', {'argv': command, 'exitCode': buildinfo.returncode})
    require(buildinfo.returncode == 0, 'Static binary build-information command failed')
    settings = {}
    for line in buildinfo.stdout.splitlines():
        fields = line.strip().split('\t')
        if len(fields) == 2 and fields[0] == 'build' and '=' in fields[1]:
            key, value = fields[1].split('=', 1)
            require(key not in settings, f'Duplicate build setting: {key}')
            settings[key] = value
    for key, value in {'GOOS': 'linux', 'GOARCH': 'amd64', 'CGO_ENABLED': '0'}.items():
        require(settings.get(key) == value, f'Binary build information differs: {key}')
    for key, expected in {'vcs.revision': COMMIT, 'vcs.modified': 'false'}.items():
        require(key not in settings or settings[key] == expected, f'Conflicting embedded VCS metadata: {key}')
    embedded_vcs = {'present': 'vcs.revision' in settings, 'sourceBinding': 'embedded Go VCS metadata' if 'vcs.revision' in settings else 'Official build invocation, clean pinned checkout/source bindings, release manifest and archive/binary hashes; official binary contains no VCS fields'}
    extracted = None
    if args.extract_to:
        require(desktop['documentationMode'] == 'compact', 'Optional installation requires the compact archive')
        target = ordinary_chain(args.extract_to)
        require(not target.exists() and not target.is_symlink(), 'Installation destination already exists')
        require(target.parent.is_dir(), 'Installation parent must already exist')
        staging = Path(tempfile.mkdtemp(prefix='.' + target.name + '-verify-', dir=target.parent))
        try:
            again, records, captured = read_archive(release / DESKTOP, desktop['sha256'], extraction=staging)
            require(records == inventories[DESKTOP] and captured == captures[DESKTOP],
                    'Archive changed before installation')
            require(not target.exists(), 'Installation destination appeared during extraction')
            staging.rename(target)
            extracted = str(target)
        finally:
            if staging.exists():
                shutil.rmtree(staging)
    return {'ok': True, 'sourceCommit': COMMIT, 'sourceDirty': False, 'version': VERSION,
            'sourceRoot': str(source), 'releaseDirectory': str(release), 'archives': summaries,
            'packageIdentity': desktop['packageIdentity'], 'documentationMode': desktop['documentationMode'],
            'binaryBuildSettings': settings, 'embeddedVCS': embedded_vcs, 'installationDirectory': extracted,
            'checks': {'officialReleaseIntegrity': True, 'safePathsAndModes': True,
                       'packageIdentity': True, 'licenseFilesAndMissingEntries': True,
                       'documentationMode': True, 'nestedCloudArchive': True,
                       'matchingBinaries': True, 'binaryBuildInformation': True,
                       'serviceExecuted': False}}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('release_dir', type=Path)
    parser.add_argument('receipt_dir', type=Path)
    parser.add_argument('--extract-to', type=Path)
    args = parser.parse_args()
    receipt = ordinary_chain(args.receipt_dir)
    require(not receipt.exists(), 'Use a new receipt directory for each verification run')
    receipt.mkdir(parents=True)
    try:
        result = verify(args, receipt)
    except Exception as error:
        result = {'ok': False, 'sourceCommit': COMMIT, 'errorType': type(error).__name__,
                  'error': str(error), 'serviceExecuted': False}
        save_json(receipt / 'verification.json', result)
        print(json.dumps(result), file=sys.stderr)
        return 1
    save_json(receipt / 'verification.json', result)
    print(json.dumps(result, indent=2))
    return 0


if __name__ == '__main__':
    sys.exit(main())

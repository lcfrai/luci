"""Additive checks for an already extracted, source-bound compact Linux package.

This does not replace verify-local-package.py or alter packages/provenance.
Guide reproduction deliberately uses the exact committed staging implementation;
Git inventories, unchanged bytes, new hosted links and licence checks are separate.
"""
import argparse
from collections import Counter
import datetime
import hashlib
import json
import os
from pathlib import Path, PurePosixPath
import re
import shutil
import stat
import subprocess
import sys
import tarfile
import tempfile
import types
from urllib.parse import unquote, urlsplit


FIXED = {'LICENSE', 'NOTICE', 'README.md', 'DESIGN.md', 'docs/UAT.md', 'docs/RUNBOOK.md'}
INDEX = 'docs/PACKAGE-VALIDATION.md'
HELPER = 'scripts/stage-release-docs.py'
HOSTED = re.compile(r'https://(?:github\.com/lcfrai/luci/|raw\.githubusercontent\.com/lcfrai/luci/)[^\s<>"\')`]+')


def require(value, message):
    if not value:
        raise ValueError(message)


def digest(path):
    with Path(path).open('rb') as stream:
        return hashlib.file_digest(stream, 'sha256').hexdigest()


def regular(path, directory=False):
    info = Path(path).lstat()
    require(stat.S_ISDIR(info.st_mode) if directory else stat.S_ISREG(info.st_mode),
            f'Expected ordinary {"directory" if directory else "file"}: {path}')


def safe_name(name):
    path = PurePosixPath(name)
    require(bool(name) and not path.is_absolute() and path.as_posix() == name
            and '..' not in path.parts and '\\' not in name,
            f'Unsafe or noncanonical member: {name}')
    return name


def tree_inventory(root, include_modes=False):
    root = Path(root)
    regular(root, True)
    result = {}
    for parent, directories, files in os.walk(root, followlinks=False):
        for name in directories:
            regular(Path(parent) / name, True)
        for name in files:
            path = Path(parent) / name
            regular(path)
            relative = safe_name(path.relative_to(root).as_posix())
            result[relative] = {'bytes': path.stat().st_size, 'sha256': digest(path)}
            if include_modes:
                result[relative]['mode'] = stat.S_IMODE(path.stat().st_mode)
    return result


def archive_inventory(path, include_modes=False):
    regular(path)
    result, seen = {}, set()
    with tarfile.open(path, 'r:gz') as archive:
        for member in archive:
            name = safe_name(member.name.rstrip('/') if member.isdir() else member.name)
            require(name not in seen, f'Duplicate archive member: {name}')
            seen.add(name)
            require(member.isdir() or member.isfile(), f'Linked/special archive member: {name}')
            if member.isfile():
                with archive.extractfile(member) as stream:
                    result[name] = {'bytes': member.size,
                                    'sha256': hashlib.file_digest(stream, 'sha256').hexdigest()}
                if include_modes:
                    result[name]['mode'] = stat.S_IMODE(member.mode)
    return result


def git(repo, *args):
    environment = {key: value for key, value in os.environ.items() if not key.startswith('GIT_')}
    return subprocess.run(['git', '-C', str(repo), *args], check=True,
                          stdout=subprocess.PIPE, stderr=subprocess.PIPE,
                          env=environment, timeout=60).stdout


def git_inventory(repo, commit):
    result = {}
    for entry in git(repo, 'ls-tree', '-r', '-t', '-z', commit).split(b'\0'):
        if entry:
            metadata, name = entry.split(b'\t', 1)
            mode, kind, oid = metadata.decode().split()
            result[safe_name(name.decode())] = {'mode': mode, 'kind': kind, 'oid': oid}
    return result


def source_blob(repo, source, path):
    item = source.get(path)
    require(item is not None and item['kind'] == 'blob' and item['mode'] in ('100644', '100755'),
            f'Source input is not an ordinary committed file: {path}')
    return git(repo, 'cat-file', 'blob', item['oid'])


def source_record(content):
    return {'bytes': len(content), 'sha256': hashlib.sha256(content).hexdigest()}


def guide_names(source):
    names = FIXED | {path for path, item in source.items() if item['kind'] == 'blob'
                     and path.startswith('docs/luci/')
                     and not path.startswith('docs/luci/evidence/')}
    require(FIXED.issubset(source), 'Source is missing a mandatory guide/licence')
    return names


def runtime_names(source):
    return {path for path, item in source.items() if item['kind'] == 'blob' and (
        path in ('sandboxd/go.mod', 'sandboxd/go.sum') or
        path.startswith(('sandboxd/cmd/', 'sandboxd/internal/', 'sandboxd/vendor/', 'images/sandbox/')))}


def guide_records(inventory):
    return {name: row for name, row in inventory.items() if name in FIXED or name.startswith('docs/')}


def verify_licenses(package, inventory, expected_count):
    manifest = json.loads((package / 'licenses/manifest.json').read_text())
    require(manifest.get('missingLicenses') == [], 'Missing licence declarations')
    require((package / 'licenses/MISSING-LICENSES.txt').read_bytes() == b'', 'Nonempty missing-licence report')
    components = manifest.get('components')
    require(isinstance(components, list) and len(components) == expected_count and expected_count > 0,
            'Licence component count differs from release manifest')
    wanted = {'licenses/manifest.json', 'licenses/README.txt', 'licenses/MISSING-LICENSES.txt'}
    for item in components:
        require(item.get('missingLicense') is False and bool(item.get('files')), 'Component lacks licence files')
        for file in item['files']:
            name = 'licenses/' + safe_name(file['path'])
            require(name in inventory and inventory[name]['bytes'] > 0
                    and inventory[name]['sha256'] == file['sha256'], f'Licence content mismatch: {name}')
            wanted.add(name)
    require({name for name in inventory if name.startswith('licenses/')} == wanted,
            'Licence file inventory is incomplete or contains unlisted files')
    return len(components)


def verify_package_identity(package, inventory):
    value = hashlib.sha256(b'luci-portable-package-v1\n')
    for name in ['luci', *sorted(name for name in inventory if name.startswith('web/'))]:
        value.update(name.encode() + b'\0' + bytes.fromhex(inventory[name]['sha256']))
    identity = value.hexdigest()
    require((package / 'package-identity').read_text() == identity + '\n', 'Portable runtime identity mismatch')
    return identity


def verify_added_urls(original, changed, commit, source):
    added = Counter(HOSTED.findall(changed)) - Counter(HOSTED.findall(original))
    checked = 0
    for address, count in added.items():
        url = urlsplit(address)
        parts = url.path.lstrip('/').split('/')
        require(parts[:2] == ['lcfrai', 'luci'], 'Wrong source repository in added URL')
        if url.hostname == 'raw.githubusercontent.com':
            require(len(parts) >= 4 and parts[2] == commit, 'Added raw URL uses another revision')
            kind, path = 'blob', '/'.join(parts[3:])
        else:
            require(len(parts) >= 5 and parts[2] in ('blob', 'tree') and parts[3] == commit,
                    'Added source URL uses another revision')
            kind, path = parts[2], '/'.join(parts[4:])
        path = safe_name(unquote(path, errors='strict'))
        require(path in source and source[path]['kind'] == kind, f'Added URL target/type absent at commit: {path}')
        checked += count
    return checked


def verify(repo, release, package, commit, provenance):
    repo, release, package, provenance = map(Path, (repo, release, package, provenance))
    require(re.fullmatch('[0-9a-f]{40}', commit), 'Expected a full source commit')
    require(git(repo, 'rev-parse', 'HEAD').decode().strip() == commit, 'Checkout moved from expected source')
    require(not git(repo, 'status', '--porcelain=v1', '--untracked-files=all').strip(), 'Source checkout is dirty')
    original_provenance = provenance.read_bytes()
    proof = json.loads(original_provenance)
    require(proof['sourceCommit'] == commit and proof['sourceDirty'] is False, 'Product provenance source mismatch')
    require(Path(proof['packageDir']).resolve() == package.resolve(), 'Wrong extracted package')
    manifest = json.loads((release / 'release-manifest.json').read_text())
    require(manifest['sourceCommit'] == commit and manifest['sourceDirty'] is False, 'Release source mismatch')
    artifacts = manifest['artifacts']
    require(len(artifacts) == 2 and {item['name'] for item in artifacts} ==
            {'luci-linux-amd64.tar.gz', 'luci-cloud-linux-amd64.tar.gz'}, 'Unexpected Linux release artifacts')
    artifacts = {item['name']: item for item in artifacts}
    for name, row in artifacts.items():
        path = release / name
        # Release archive symlinks may deliberately preserve a verified relocation.
        regular(path.resolve())
        require(path.stat().st_size == row['bytes'] and digest(path) == row['sha256'], f'Release archive changed: {name}')
    portable = release / 'luci-linux-amd64.tar.gz'
    require(Path(proof['archive']).resolve() == portable.resolve()
            and proof['archiveSHA256'] == artifacts[portable.name]['sha256'], 'Archive provenance mismatch')
    wanted_sums = {f"{row['sha256']}  {name}" for name, row in artifacts.items()}
    require(set((release / 'SHA256SUMS').read_text().splitlines()) == wanted_sums, 'SHA256SUMS mismatch')
    installed_modes = tree_inventory(package, include_modes=True)
    require(installed_modes == archive_inventory(portable.resolve(), include_modes=True),
            'Extracted package differs from complete archive inventory or file modes')
    require(all(row['mode'] == (0o755 if name in ('luci', 'start-luci.sh') or name.endswith('.sh') else 0o644)
                for name, row in installed_modes.items()), 'Portable file modes differ from normalized release rules')
    installed = {name: {key: value for key, value in row.items() if key != 'mode'} for name, row in installed_modes.items()}
    require(installed['luci']['sha256'] == proof['binarySHA256']
            and installed['web/index.html']['sha256'] == proof['webIndexSHA256'], 'Binary/web provenance mismatch')
    package_identity = verify_package_identity(package, installed)
    cloud_path = (release / 'luci-cloud-linux-amd64.tar.gz').resolve()
    require(installed['cloud/luci-cloud-linux-amd64.tar.gz'] ==
            {'bytes': cloud_path.stat().st_size, 'sha256': digest(cloud_path)}, 'Embedded cloud archive differs')
    cloud_modes = archive_inventory(cloud_path, include_modes=True)
    require({name: row for name, row in installed_modes.items() if not name.startswith('cloud/')} == cloud_modes,
            'Standalone cloud contents differ from portable core')
    cloud = {name: {key: value for key, value in row.items() if key != 'mode'} for name, row in cloud_modes.items()}
    source = git_inventory(repo, commit)
    guides = guide_names(source)
    require(set(guide_records(installed)) == guides | {INDEX}, 'Guide completeness/evidence omission mismatch')
    require(not any(name.startswith('docs/luci/evidence/') for name in installed), 'Validation payload still bundled')
    expected_runtime = {'runtime/' + name: source_record(source_blob(repo, source, name)) for name in runtime_names(source)}
    require(expected_runtime and {name: row for name, row in installed.items() if name.startswith('runtime/')} == expected_runtime,
            'Runtime source inventory differs from exact commit')
    checked_urls, unchanged = 0, 0
    for name in guides:
        content = source_blob(repo, source, name)
        if name.endswith('.md'):
            checked_urls += verify_added_urls(content.decode(), (package / name).read_text(), commit, source)
        else:
            require(installed[name] == source_record(content), f'Unchanged guide/licence bytes differ: {name}')
            unchanged += 1
    checked_urls += verify_added_urls('', (package / INDEX).read_text(), commit, source)
    license_rows = [row for row in manifest['portableLicenseComponents'] if row['target'] == 'luci-linux-amd64']
    require(len(license_rows) == 1 and license_rows[0]['missing'] == 0, 'Missing Linux licence summary')
    license_count = verify_licenses(package, installed, license_rows[0]['count'])
    helper = source_blob(repo, source, HELPER)
    module_name = '_luci_exact_committed_docs_verification'
    module = types.ModuleType(module_name)
    sys.modules[module_name] = module
    temporary = Path(tempfile.mkdtemp(prefix='luci-compact-doc-verifier-', dir='/tmp'))
    try:
        exec(compile(helper, f'{commit}:{HELPER}', 'exec'), module.__dict__)
        staged = temporary / 'expected'
        staging = module.stage(repo, staged, commit, 'false')
        require(staging['compact'] is True, 'Source guide staging did not establish compact eligibility')
        require(tree_inventory(staged) == guide_records(installed), 'Guides differ from exact committed staging reproduction')
    finally:
        sys.modules.pop(module_name, None)
        require(temporary.parent == Path('/tmp') and temporary.name.startswith('luci-compact-doc-verifier-')
                and temporary.resolve() == temporary and not temporary.is_symlink()
                and temporary.stat().st_uid == os.getuid(), 'Refusing unsafe temporary cleanup')
        shutil.rmtree(temporary)
    require(provenance.read_bytes() == original_provenance, 'Original provenance changed')
    require(git(repo, 'rev-parse', 'HEAD').decode().strip() == commit
            and not git(repo, 'status', '--porcelain=v1', '--untracked-files=all').strip(), 'Source changed during verification')
    return {'status': 'passed', 'at': datetime.datetime.now(datetime.timezone.utc).isoformat(),
            'sourceCommit': commit, 'sourceDirty': False, 'packageDir': str(package),
            'originalProvenanceSHA256': hashlib.sha256(original_provenance).hexdigest(),
            'releaseManifestSHA256': digest(release / 'release-manifest.json'),
            'packageIdentity': package_identity,
            'artifacts': list(artifacts.values()), 'verifiedPackageFiles': len(installed),
            'verifiedCloudFiles': len(cloud), 'verifiedRuntimeSourceFiles': len(expected_runtime),
            'verifiedGuideFiles': len(guides) + 1, 'unchangedNonMarkdownGuides': unchanged,
            'newHostedURLsBoundToExactGitObjects': checked_urls, 'licenseComponents': license_count,
            'stagingHelperSHA256': hashlib.sha256(helper).hexdigest(), 'stagingResult': staging,
            'stagingReproduction': 'same committed implementation; not independent Markdown rewrite proof',
            'independentChecks': ['Git guide/runtime file inventories', 'unchanged non-Markdown guide/licence bytes',
                                  'new hosted URL commit/object/type', 'licence manifest/file hashes',
                                  'full archive/extraction and embedded cloud equality including file modes'],
            'ownedTemporaryStageRemoved': not temporary.exists(), 'originalProvenanceUnchanged': True}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ('repo', 'release', 'package', 'commit', 'provenance', 'report'):
        parser.add_argument('--' + name, required=True)
    args = parser.parse_args()
    report = Path(args.report)
    require(not report.exists() and not report.is_symlink(), 'Verification report must be fresh')
    result = verify(args.repo, args.release, args.package, args.commit, args.provenance)
    with report.open('x') as stream:
        json.dump(result, stream, indent=2)
        stream.write('\n')
    os.chmod(report, 0o600)
    print(json.dumps({'status': result['status'], 'report': str(report), 'reportSHA256': digest(report),
                      'verifiedGuideFiles': result['verifiedGuideFiles'], 'newHostedURLs': result['newHostedURLsBoundToExactGitObjects']}))


if __name__ == '__main__':
    main()

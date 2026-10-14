"""Offline v2 release staging. Consumers need only the packaged Luci binary."""
import argparse
import copy
import gzip
import hashlib
from pathlib import Path
import re
import shutil
import tarfile
import tempfile
import time

import archive
import bootstrap
from canonical_inputs import inventory_digest, stage
from common import plain_directory, read_json, schema_validate, write_json
import payload as legacy
import producer
import pull_metadata
import registry_export
import registry_store

MANIFEST = 'runtime-oci-pull-manifest-v2.json'
PULL_RECEIPT = 'runtime-pull-producer-receipt-v2.json'
CONVERSION = registry_store.RECEIPT_NAME
FILES = (MANIFEST, PULL_RECEIPT, legacy.RECEIPT, CONVERSION, bootstrap.RECEIPT,
         registry_store.STORE_NAME, bootstrap.NAME, legacy.SOURCE)
REUSE_FILES = (legacy.REUSE, legacy.PRIOR_MANIFEST, legacy.PRIOR_RECEIPT)


def canonical_receipt(root, name, digest):
    archive.regular(root / name)
    if not 1 <= (root / name).stat().st_size <= archive.MAX_METADATA:
        raise ValueError('v2 receipt exceeds metadata bound: ' + name)
    raw = (root / name).read_bytes() if archive.file_record(root / name)['sha256'] == digest else None
    if raw is None:
        raise ValueError('v2 receipt hash changed: ' + name)
    value = archive.json_bytes(raw)
    if raw != pull_metadata.encoded(value):
        raise ValueError('v2 receipt is not exact Go canonical JSON: ' + name)
    return value


def valid_args(value):
    return isinstance(value, list) and 1 <= len(value) <= 32 and all(
        isinstance(x, str) and 1 <= len(x.encode('utf-8')) <= 256 and not any(c in x for c in '\0\r\n') for x in value)


def verify_receipts(root, m):
    original = schema_validate(read_json(root / legacy.RECEIPT), 'runtime-oci-build-receipt-v1.schema.json')
    p, image = m['producer'], m['image']
    if (archive.file_record(root / legacy.RECEIPT)['sha256'] != p['originalBuildReceiptSHA256'] or
            original['canonicalInputDigest'] != m['canonicalInputDigest'] or original['imageID'] != image['id'] or
            original['canonicalInputAlgorithm'] != m['canonicalInputAlgorithm'] or original['builder']['exitCode'] != 0 or
            {x['reference'] for x in original['baseImages']} != {'golang:1.26-bookworm', 'node:24-bookworm-slim'} or
            {x['tool']: x['reportedVersion'] for x in original['probes']} != dict(archive.RUNTIME['harnesses'], sandboxd='0.5.0') or
            any(x['exitCode'] != 0 for x in original['probes'])):
        raise ValueError('v2 original source/build/probe receipt differs')
    conversion = canonical_receipt(root, CONVERSION, p['conversionReceiptSHA256'])
    wanted = {'version': 2, 'kind': 'luci-runtime-registry-conversion', 'selectedImageID': image['id'],
        'registryManifestDigest': image['manifestDigest'], 'config': {'digest': image['id'], 'bytes': image['configBytes']},
        'layers': [{k: x[k] for k in ('compressedDigest', 'compressedBytes', 'diffID', 'uncompressedBytes')} for x in image['layers']],
        'registryStore': {'sha256': m['registryStore']['sha256'], 'bytes': m['registryStore']['bytes'], 'memberCount': len(m['registryStore']['members'])},
        'verification': {'compressedDigestsVerified': True, 'diffIDsVerified': True, 'configVerified': True}}
    if conversion != wanted:
        raise ValueError('v2 conversion receipt differs')
    proof = canonical_receipt(root, PULL_RECEIPT, p['pullProducerReceiptSHA256'])
    push = proof.get('push', {})
    wanted = {'version': 2, 'kind': 'luci-runtime-pull-producer', 'sourceCommit': m['sourceCommit'],
        'canonicalInputDigest': m['canonicalInputDigest'], 'selectedImageID': image['id'],
        'originalBuildReceiptSHA256': p['originalBuildReceiptSHA256'], 'conversionReceiptSHA256': p['conversionReceiptSHA256'],
        'registryStoreSHA256': m['registryStore']['sha256'], 'registryManifestDigest': image['manifestDigest'], 'push': push}
    if proof != wanted or set(push) != {'session', 'repository', 'engineID', 'instanceID', 'imageID', 'commandExitStatus',
            'terminalStreamSHA256', 'manifestReadbackDigest', 'helperTerminalReceiptSHA256', 'helperRetirementReceiptSHA256'}:
        raise ValueError('v2 producer receipt fields differ')
    if (not re.fullmatch('[a-f0-9]{32}', push['session']) or push['repository'] != 'luci/runtime-export-' + push['session'] or
            push['imageID'] != image['id'] or type(push['commandExitStatus']) is not int or push['commandExitStatus'] != 0 or
            push['manifestReadbackDigest'] != image['manifestDigest'] or any(not isinstance(push[x], str) or
            not re.fullmatch(r'[\x21-\x7e]{1,160}', push[x]) for x in ('engineID', 'instanceID')) or
            any(not re.fullmatch('[a-f0-9]{64}', push[x]) for x in ('terminalStreamSHA256', 'helperTerminalReceiptSHA256', 'helperRetirementReceiptSHA256'))):
        raise ValueError('v2 producer positive terminal identity invalid')
    b = m['bootstrap']
    receipt = canonical_receipt(root, bootstrap.RECEIPT, b['buildReceiptSHA256'])
    wanted = {'version': 1, 'kind': 'luci-runtime-registry-bootstrap', 'buildSourceCommit': receipt.get('buildSourceCommit'),
        'gateSourceSHA256': b['gateSourceSHA256'], 'buildRecipeSHA256': b['buildRecipeSHA256'],
        'distribution': {'version': b['distributionVersion'], 'archiveSHA256': b['distributionArchiveSHA256'],
                         'binarySHA256': b['distributionBinarySHA256'], 'licenseSHA256': b['distributionLicenseSHA256']},
        'image': {'id': b['imageID'], 'configSHA256': b['configSHA256'], 'configBytes': b['configBytes'], 'layers': b['layers'],
                  'archiveSHA256': b['sha256'], 'archiveBytes': b['bytes']}, 'builder': receipt.get('builder'), 'probe': receipt.get('probe')}
    if receipt != wanted or not re.fullmatch('[a-f0-9]{40}', receipt['buildSourceCommit']):
        raise ValueError('v2 bootstrap receipt differs')
    for phase in ('builder', 'probe'):
        fact = receipt[phase]
        keys = {'argv', 'exitStatus', 'stdoutSHA256', 'stderrSHA256'} | ({'version', 'memoryMiB', 'cpuMillis'} if phase == 'builder' else set())
        if (not isinstance(fact, dict) or set(fact) != keys or not valid_args(fact['argv']) or
                type(fact['exitStatus']) is not int or fact['exitStatus'] != 0 or
                any(not re.fullmatch('[a-f0-9]{64}', fact[x]) for x in ('stdoutSHA256', 'stderrSHA256'))):
            raise ValueError('v2 bootstrap execution facts invalid')
    build = receipt['builder']
    if not isinstance(build['version'], str) or not build['version'] or type(build['memoryMiB']) is not int or not 128 <= build['memoryMiB'] <= 2048 or type(build['cpuMillis']) is not int or not 100 <= build['cpuMillis'] <= 4000:
        raise ValueError('v2 bootstrap execution allocation invalid')
    if receipt['probe']['stdoutSHA256'] != hashlib.sha256(b'luci-registry-gate 1\n').hexdigest() or receipt['probe']['stderrSHA256'] != hashlib.sha256(b'').hexdigest():
        raise ValueError('v2 actual bootstrap version probe changed')
    return original


def verify_store(root, m):
    store, image = m['registryStore'], m['image']
    record = archive.file_record(root / registry_store.STORE_NAME)
    if any(record[k] != store[k] for k in ('sha256', 'bytes')) or record['bytes'] > registry_store.MAX_STORE:
        raise ValueError('v2 store bytes changed')
    expected = {row['path']: row for row in store['members']}
    if len(expected) != len(store['members']) or list(expected) != sorted(expected) or sum(x['bytes'] for x in expected.values()) > registry_store.MAX_STORE:
        raise ValueError('v2 store member order/aggregate invalid')
    prefix = registry_store.PREFIX
    def blob(digest):
        return prefix + 'blobs/sha256/' + digest[7:9] + '/' + digest[7:] + '/data'
    repository = prefix + 'repositories/luci/runtime-installed/'
    links = {repository + '_layers/sha256/' + image['id'][7:] + '/link': image['id'],
             repository + '_manifests/revisions/sha256/' + image['manifestDigest'][7:] + '/link': image['manifestDigest']}
    for layer in image['layers']:
        links[repository + '_layers/sha256/' + layer['compressedDigest'][7:] + '/link'] = layer['compressedDigest']
    names = {blob(image['id']), blob(image['manifestDigest']), *(blob(x['compressedDigest']) for x in image['layers']), *links}
    if set(expected) != names:
        raise ValueError('v2 store extra/missing/foreign member')
    deadline = time.monotonic() + 1800
    with tarfile.open(root / registry_store.STORE_NAME, 'r:') as tar:
        seen = []
        for file in tar:
            row = expected.get(file.name)
            if (row is None or not file.isfile() or file.linkname or file.pax_headers or file.issparse() or
                    file.size != row['bytes'] or file.name in seen or time.monotonic() > deadline):
                raise ValueError('v2 store linked/extra/changed member')
            seen.append(file.name)
            with tar.extractfile(file) as content:
                digest, _ = archive.stream_hash(content, file.size)
            if digest != row['sha256']:
                raise ValueError('v2 store member hash changed')
        if seen != list(expected):
            raise ValueError('v2 store inventory changed')
        with (root / registry_store.STORE_NAME).open('rb') as tail:
            tail.seek(tar.offset)
            if any(block.strip(b'\0') for block in iter(lambda: tail.read(1 << 20), b'')):
                raise ValueError('v2 store concealed trailing data')
        for name, value in links.items():
            if tar.extractfile(name).read() != value.encode('ascii'):
                raise ValueError('v2 store repository link changed')
        raw = tar.extractfile(blob(image['id'])).read(archive.MAX_METADATA + 1)
        if len(raw) != image['configBytes'] or hashlib.sha256(raw).hexdigest() != image['configSHA256'] or archive.config_policy(archive.json_bytes(raw)) != [x['diffID'] for x in image['layers']]:
            raise ValueError('v2 store original config changed')
        raw = tar.extractfile(blob(image['manifestDigest'])).read(archive.MAX_METADATA + 1)
        wanted = {'schemaVersion': 2, 'mediaType': registry_export.DOCKER_MANIFEST,
                  'config': {'mediaType': registry_export.DOCKER_CONFIG, 'size': image['configBytes'], 'digest': image['id']},
                  'layers': [{'mediaType': x['mediaType'], 'size': x['compressedBytes'], 'digest': x['compressedDigest']} for x in image['layers']]}
        if hashlib.sha256(raw).hexdigest() != image['manifestDigest'][7:] or archive.json_bytes(raw) != wanted:
            raise ValueError('v2 store distribution manifest changed')
        for row in image['layers']:
            file = tar.getmember(blob(row['compressedDigest']))
            if file.size != row['compressedBytes'] or expected[file.name]['sha256'] != row['compressedDigest'][7:]:
                raise ValueError('v2 store compressed layer identity changed')
            layer_deadline = min(deadline, time.monotonic() + 600)
            total, digest = 0, hashlib.sha256()
            with tar.extractfile(file) as content, gzip.GzipFile(fileobj=content) as decoded:
                while True:
                    if time.monotonic() > layer_deadline:
                        raise ValueError('v2 store decompression deadline exceeded')
                    block = decoded.read(1 << 20)
                    if not block:
                        break
                    total += len(block)
                    if total > row['uncompressedBytes']:
                        raise ValueError('v2 store expanded layer exceeds bound')
                    digest.update(block)
            if total != row['uncompressedBytes'] or digest.hexdigest() != row['diffID'][7:]:
                raise ValueError('v2 store diffID changed')


def verify(root, package=None, canonical=None, source_commit=None):
    root = plain_directory(root)
    package = plain_directory(package or root)
    canonical = plain_directory(canonical or root / 'canonical')
    m = schema_validate(read_json(root / MANIFEST), 'runtime-oci-pull-manifest-v2.schema.json')
    legacy.source_receipt(read_json(root / legacy.SOURCE), m['sourceCommit'])
    if source_commit is not None and m['sourceCommit'] != source_commit:
        raise ValueError('v2 installed source differs')
    if (root / legacy.PAYLOAD).exists() or (root / legacy.MANIFEST).exists():
        raise ValueError('v2 package contains duplicate v1 payload/manifest')
    rows = legacy.source_inventory(canonical)
    if rows != m['sourceFiles'] or inventory_digest(rows) != m['canonicalInputDigest'] or m['runtime'] != archive.RUNTIME:
        raise ValueError('v2 canonical source/runtime differs')
    image = m['image']
    if (image['id'][7:] != image['configSHA256'] or len({x['compressedDigest'] for x in image['layers']}) != len(image['layers']) or
            sum(x['compressedBytes'] for x in image['layers']) > registry_store.MAX_STORE or
            sum(x['uncompressedBytes'] for x in image['layers']) > registry_store.MAX_EXPANDED or
            sum(x['uncompressedBytes'] for x in m['bootstrap']['layers']) > bootstrap.MAX_EXPANDED):
        raise ValueError('v2 image aggregate/config identity invalid')
    original = verify_receipts(root, m)
    reuse = m['producer'].get('reuseEvidenceSHA256')
    if bool(reuse) != (original['buildSourceCommit'] != m['sourceCommit']):
        raise ValueError('v2 original source requires exact reuse evidence')
    if reuse:
        prior = schema_validate(read_json(root / legacy.PRIOR_MANIFEST), 'runtime-oci-manifest-v1.schema.json')
        wanted = reuse_record(root, m, prior, original)
        if read_json(root / legacy.REUSE) != wanted or archive.file_record(root / legacy.REUSE)['sha256'] != reuse:
            raise ValueError('v2 equivalent-input reuse receipt differs')
        if (prior['sourceCommit'] != original['buildSourceCommit'] or prior['producer']['buildReceiptSHA256'] != m['producer']['originalBuildReceiptSHA256'] or
                prior['canonicalInputDigest'] != m['canonicalInputDigest'] or prior['sourceFiles'] != m['sourceFiles'] or
                prior['image']['id'] != image['id'] or prior['image']['diffIDs'] != [x['diffID'] for x in image['layers']] or
                prior['runtime'] != m['runtime'] or prior['licenses'] != [x for x in m['licenses'] if x['path'].startswith('licenses/runtime-image/')] or
                (root / legacy.PRIOR_RECEIPT).read_bytes() != (root / legacy.RECEIPT).read_bytes()):
            raise ValueError('v2 original raw reuse evidence differs')
    names = [x['path'] for x in m['licenses']]
    if names != sorted(set(names)) or sum(x['bytes'] for x in m['licenses']) > 64 << 20:
        raise ValueError('v2 licence aggregate/order invalid')
    found = []
    for tree in ('runtime-image', 'runtime-bootstrap'):
        base = plain_directory(package / 'licenses' / tree)
        for file in base.rglob('*'):
            if file.is_dir():
                plain_directory(file)
            else:
                archive.regular(file)
                found.append(file.relative_to(package).as_posix())
    if sorted(found) != names:
        raise ValueError('v2 unmanifested/missing licence')
    for row in m['licenses']:
        actual = archive.file_record(package / archive.safe_path(row['path']))
        if any(actual[x] != row[x] for x in ('sha256', 'bytes')):
            raise ValueError('v2 licence bytes changed')
    if not any(x['path'].startswith('licenses/runtime-bootstrap/') and x['sha256'] == m['bootstrap']['distributionLicenseSHA256'] for x in m['licenses']):
        raise ValueError('v2 original Distribution grant missing')
    verify_store(root, m)
    b = m['bootstrap']
    with tempfile.TemporaryDirectory(prefix='luci-bootstrap-verify-') as temp:
        normalized = bootstrap.normalize(root / bootstrap.NAME, Path(temp) / bootstrap.NAME, b['imageID'], b['gateBinarySHA256'], b['distributionBinarySHA256'])
    if any(normalized[key] != b[key] for key in normalized):
        raise ValueError('v2 bootstrap bytes/configuration changed')
    return m


def reuse_record(root, manifest, prior, original):
    return {'version': 1, 'kind': 'luci-runtime-equivalent-input-reuse',
        'priorManifestSHA256': archive.file_record(root / legacy.PRIOR_MANIFEST)['sha256'],
        'priorBuildReceiptSHA256': archive.file_record(root / legacy.PRIOR_RECEIPT)['sha256'],
        'priorBuildSourceCommit': original['buildSourceCommit'], 'installedSourceCommit': manifest['sourceCommit'],
        'installedSourceReceiptSHA256': archive.file_record(root / legacy.SOURCE)['sha256'],
        'canonicalInputAlgorithm': manifest['canonicalInputAlgorithm'], 'canonicalInputDigest': manifest['canonicalInputDigest'],
        'imageID': manifest['image']['id'], 'archiveSHA256': prior['archive']['sha256'], 'archiveBytes': prior['archive']['bytes']}


def stage_payload(source, repo, revision, package):
    source, repo, package = Path(source).absolute(), Path(repo).absolute(), plain_directory(package)
    producer.clean_source(repo, revision)
    original = verify(source)
    runtime = package / 'runtime'
    runtime.mkdir(exist_ok=True)
    plain_directory(runtime)
    if any((runtime / name).exists() or (runtime / name).is_symlink() for name in (*FILES, *REUSE_FILES, legacy.PAYLOAD, legacy.MANIFEST)):
        raise ValueError('v2 package runtime destination already populated')
    with tempfile.TemporaryDirectory(prefix='luci-v2-inputs-', dir=package.parent) as temp:
        inputs = stage(repo, revision, Path(temp) / 'canonical')
        if inputs['sourceFiles'] != original['sourceFiles'] or inputs['canonicalInputDigest'] != original['canonicalInputDigest']:
            raise ValueError('v2 new package changes original runtime inputs')
        for row in inputs['sourceFiles']:
            shutil.copyfile(Path(temp) / 'canonical' / row['path'], legacy.destination_file(runtime, row['path']))
    for name in FILES:
        if name not in (MANIFEST, legacy.SOURCE, PULL_RECEIPT):
            shutil.copyfile(source / name, legacy.destination_file(runtime, name))
    for row in original['licenses']:
        shutil.copyfile(source / row['path'], legacy.destination_file(package, row['path']))
    manifest = copy.deepcopy(original)
    manifest['sourceCommit'] = revision
    write_json(runtime / legacy.SOURCE, {'version': 1, 'sourceCommit': revision, 'sourceDirty': False})
    original_receipt = read_json(source / legacy.RECEIPT)
    if original_receipt['buildSourceCommit'] != revision:
        # The producer always retains the original v1 siblings, even for a
        # same-source v2 output, to support later equivalent-input packages.
        for name in (legacy.PRIOR_MANIFEST, legacy.PRIOR_RECEIPT):
            shutil.copyfile(source / name, runtime / name)
        prior = read_json(runtime / legacy.PRIOR_MANIFEST)
        write_json(runtime / legacy.REUSE, reuse_record(runtime, manifest, prior, original_receipt))
        manifest['producer']['reuseEvidenceSHA256'] = archive.file_record(runtime / legacy.REUSE)['sha256']
    else:
        manifest['producer'].pop('reuseEvidenceSHA256', None)
    proof = read_json(source / PULL_RECEIPT)
    proof['sourceCommit'] = revision
    pull_metadata.write(runtime / PULL_RECEIPT, proof)
    manifest['producer']['pullProducerReceiptSHA256'] = archive.file_record(runtime / PULL_RECEIPT)['sha256']
    pull_metadata.write(runtime / MANIFEST, manifest)
    verify(runtime, package, runtime, revision)
    producer.clean_source(repo, revision)
    return manifest


def companion_record(runtime, manifest, manifest_hash):
    return {'version': 2, 'kind': 'luci-runtime-pull-companion-required', 'sourceCommit': manifest['sourceCommit'],
        'manifestSHA256': manifest_hash, 'registryStoreSHA256': manifest['registryStore']['sha256'],
        'registryStoreBytes': manifest['registryStore']['bytes'], 'bootstrapSHA256': manifest['bootstrap']['sha256'],
        'bootstrapBytes': manifest['bootstrap']['bytes'], 'ordinaryLocalInstall': 'requires-verified-companion',
        'automaticSourceBuildFallback': False}


def verify_companion(runtime):
    runtime = plain_directory(runtime)
    marker = read_json(runtime / legacy.COMPANION)
    keys = {'version', 'kind', 'sourceCommit', 'manifestSHA256', 'registryStoreSHA256', 'registryStoreBytes',
            'bootstrapSHA256', 'bootstrapBytes', 'ordinaryLocalInstall', 'automaticSourceBuildFallback'}
    if (not isinstance(marker, dict) or set(marker) != keys or type(marker['version']) is not int or marker['version'] != 2 or
            marker['kind'] != 'luci-runtime-pull-companion-required' or marker['ordinaryLocalInstall'] != 'requires-verified-companion' or
            marker['automaticSourceBuildFallback'] is not False or any(not re.fullmatch('[a-f0-9]{64}', marker[x]) for x in ('manifestSHA256', 'registryStoreSHA256', 'bootstrapSHA256')) or
            type(marker['registryStoreBytes']) is not int or not 1024 <= marker['registryStoreBytes'] <= registry_store.MAX_STORE or
            type(marker['bootstrapBytes']) is not int or not 1024 <= marker['bootstrapBytes'] <= bootstrap.MAX_BYTES or
            any((runtime / name).exists() for name in (MANIFEST, legacy.MANIFEST, legacy.PAYLOAD, registry_store.STORE_NAME, bootstrap.NAME))):
        raise ValueError('v2 core companion identity/duplication invalid')
    legacy.source_receipt(read_json(runtime / legacy.SOURCE), marker['sourceCommit'])
    return marker


def core_from_package(package, destination):
    package, destination = plain_directory(package), Path(destination).absolute()
    plain_directory(destination.parent)
    manifest = verify(package / 'runtime', package, package / 'runtime')
    if destination.exists() or destination.is_symlink() or destination.is_relative_to(package):
        raise ValueError('v2 core requires fresh separate destination')
    excluded = {'runtime/' + name for name in (*FILES, *REUSE_FILES) if name != legacy.SOURCE}
    destination.mkdir(mode=0o700)
    for source in sorted(package.rglob('*')):
        relative = source.relative_to(package).as_posix()
        if (relative in excluded or relative == 'package-identity' or relative == 'cloud' or relative.startswith('cloud/') or
                any(relative == tree or relative.startswith(tree + '/') for tree in ('licenses/runtime-image', 'licenses/runtime-bootstrap'))):
            continue
        if source.is_dir():
            plain_directory(source)
            (destination / relative).mkdir(parents=True, exist_ok=True)
        else:
            archive.regular(source)
            shutil.copyfile(source, legacy.destination_file(destination, relative))
    marker = companion_record(destination / 'runtime', manifest, archive.file_record(package / 'runtime' / MANIFEST)['sha256'])
    pull_metadata.write(destination / 'runtime' / legacy.COMPANION, marker)
    verify_companion(destination / 'runtime')
    return marker


def distribution_record(package):
    root = plain_directory(Path(package) / 'runtime')
    m = schema_validate(read_json(root / MANIFEST), 'runtime-oci-pull-manifest-v2.schema.json')
    legacy.source_receipt(read_json(root / legacy.SOURCE), m['sourceCommit'])
    return {'version': 2, 'sourceCommit': m['sourceCommit'], 'buildSourceCommit': read_json(root / legacy.RECEIPT)['buildSourceCommit'],
        'manifestSHA256': archive.file_record(root / MANIFEST)['sha256'], 'imageID': m['image']['id'],
        'canonicalInputDigest': m['canonicalInputDigest'], 'registryStoreSHA256': m['registryStore']['sha256'],
        'registryStoreBytes': m['registryStore']['bytes'], 'bootstrapSHA256': m['bootstrap']['sha256'], 'bootstrapBytes': m['bootstrap']['bytes'],
        'payloadRole': 'one-per-outer-package', 'embeddedManagementCore': 'requires-verified-companion', 'automaticSourceBuildFallback': False}


if __name__ == '__main__':
    import json
    parser = argparse.ArgumentParser(description=__doc__)
    commands = parser.add_subparsers(dest='action', required=True)
    check = commands.add_parser('verify'); check.add_argument('--payload', type=Path, required=True)
    install = commands.add_parser('stage')
    for key in ('payload', 'repo', 'package'):
        install.add_argument('--' + key, type=Path, required=True)
    install.add_argument('--revision', required=True)
    core = commands.add_parser('core-from-package')
    core.add_argument('--package', type=Path, required=True); core.add_argument('--output', type=Path, required=True)
    describe = commands.add_parser('describe-package'); describe.add_argument('--package', type=Path, required=True)
    args = parser.parse_args()
    try:
        if args.action == 'verify': result = verify(args.payload)
        elif args.action == 'stage': result = stage_payload(args.payload, args.repo, args.revision, args.package)
        elif args.action == 'core-from-package': result = core_from_package(args.package, args.output)
        else: result = distribution_record(args.package)
    except (ValueError, OSError, producer.subprocess.SubprocessError) as error:
        parser.exit(1, 'Compressed runtime payload refusal: ' + str(error) + '\n')
    print(json.dumps(result, sort_keys=True))

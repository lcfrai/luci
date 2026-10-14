"""Build the fixed, immutable v2 registry store from verified producer bytes.

No Docker, networking, image mutation or host extraction occurs here. The
original registry/config bytes and compressed layers are retained unchanged.
"""
from __future__ import annotations

import gzip
import hashlib
import io
from pathlib import Path
import tarfile
import time

import archive
from common import plain_directory
import registry_export
import pull_metadata

STORE_NAME = 'runtime-registry-store-v2.tar'
RECEIPT_NAME = 'runtime-registry-conversion-receipt-v2.json'
INSTALLED_REPOSITORY = 'luci/runtime-installed'
MAX_STORE = 4 << 30
MAX_EXPANDED = 8 << 30
MAX_MEMBERS = 4096
PREFIX = 'docker/registry/v2/'


def digest_bytes(value):
    return 'sha256:' + hashlib.sha256(value).hexdigest()


def checked_file(path, digest, limit):
    registry_export._identity(digest)
    record = archive.file_record(path)
    if not 0 < record['bytes'] <= limit or record['sha256'] != digest[7:]:
        raise ValueError('registry store source digest or length changed')
    return record


def expanded_record(path, diffid, limit, total_deadline):
    """Finite streamed digest verification; layer tar members stay opaque."""
    registry_export._identity(diffid)
    deadline = min(total_deadline, time.monotonic() + 600)
    digest, size = hashlib.sha256(), 0
    archive.regular(path)
    with path.open('rb') as raw, gzip.GzipFile(fileobj=raw, mode='rb') as decoded:
        while True:
            if time.monotonic() > deadline:
                raise ValueError('registry store verification deadline exceeded')
            block = decoded.read(1 << 20)
            if not block:
                break
            size += len(block)
            if size > limit:
                raise ValueError('registry store aggregate expanded bytes exceed bound')
            digest.update(block)
    if size == 0 or digest.hexdigest() != diffid[7:]:
        raise ValueError('registry store compressed bytes differ from original diffID')
    return size


def create(conversion_directory, conversion, image_id, diffids, output):
    root, output = plain_directory(conversion_directory), Path(output).absolute()
    plain_directory(output.parent)
    if output.exists() or output.is_symlink():
        raise ValueError('registry store requires a fresh output')
    if ((conversion.get('version'), conversion.get('state')) not in (
            (1, 'registry_readback_normalized_original_image_verified'),
            (2, 'registry_readback_compressed_original_image_verified')) or
            conversion.get('imageID') != image_id or conversion.get('image', {}).get('diffIDs') != diffids or
            conversion.get('pushAndRegistryOwnershipVerified') is not False):
        raise ValueError('registry conversion/original image identity differs')
    registry_export._identity(image_id)
    manifest_digest = registry_export._identity(conversion.get('registryManifestDigest'))
    source = plain_directory(root / 'registry')
    manifest_record = checked_file(source / 'manifest.json', manifest_digest, archive.MAX_METADATA)
    manifest = archive.json_bytes((source / 'manifest.json').read_bytes())
    config = manifest.get('config', {})
    layers = manifest.get('layers')
    if (set(manifest) != {'schemaVersion', 'mediaType', 'config', 'layers'} or manifest['schemaVersion'] != 2 or
            manifest['mediaType'] != registry_export.DOCKER_MANIFEST or
            set(config) != {'mediaType', 'size', 'digest'} or config['mediaType'] != registry_export.DOCKER_CONFIG or
            config['digest'] != image_id or not isinstance(layers, list) or len(layers) != len(diffids) or not 1 <= len(layers) <= 2048):
        raise ValueError('registry store manifest is not exact original Docker schema2')
    config_record = checked_file(source / 'config.json', image_id, archive.MAX_METADATA)
    if type(config['size']) is not int or config['size'] != config_record['bytes']:
        raise ValueError('registry store original config size differs')
    if archive.config_policy(archive.json_bytes((source / 'config.json').read_bytes())) != diffids:
        raise ValueError('registry store original configuration/diffIDs differ')
    entries = {}

    def add(name, raw=None, file=None, record=None):
        if len(name) > 512 or not name.startswith(PREFIX) or '..' in name.split('/') or name in entries:
            raise ValueError('registry store duplicate or unsafe path')
        if raw is not None:
            record = {'sha256': hashlib.sha256(raw).hexdigest(), 'bytes': len(raw)}
        entries[name] = (raw, file, record)

    def blob(digest, path, record):
        key = digest[7:]
        add(PREFIX + 'blobs/sha256/' + key[:2] + '/' + key + '/data', file=path, record=record)

    repository = PREFIX + 'repositories/' + INSTALLED_REPOSITORY + '/'
    blob(image_id, source / 'config.json', config_record)
    blob(manifest_digest, source / 'manifest.json', manifest_record)
    add(repository + '_layers/sha256/' + image_id[7:] + '/link', raw=image_id.encode('ascii'))
    add(repository + '_manifests/revisions/sha256/' + manifest_digest[7:] + '/link', raw=manifest_digest.encode('ascii'))
    result_layers, compressed, expanded = [], 0, 0
    deadline = time.monotonic() + 1800
    for index, (layer, diffid) in enumerate(zip(layers, diffids)):
        if (not isinstance(layer, dict) or set(layer) != {'mediaType', 'size', 'digest'} or
                layer['mediaType'] != registry_export.DOCKER_LAYER or type(layer['size']) is not int or not 0 < layer['size'] <= MAX_STORE):
            raise ValueError('registry store foreign or oversized compressed descriptor')
        digest = registry_export._identity(layer['digest'])
        path = source / (str(index) + '.layer.gz')
        record = checked_file(path, digest, MAX_STORE)
        if record['bytes'] != layer['size']:
            raise ValueError('registry store compressed descriptor length differs')
        compressed += record['bytes']
        if compressed > MAX_STORE:
            raise ValueError('registry store aggregate compressed bytes exceed bound')
        size = expanded_record(path, diffid, MAX_EXPANDED - expanded, deadline)
        expanded += size
        blob(digest, path, record)
        add(repository + '_layers/sha256/' + digest[7:] + '/link', raw=digest.encode('ascii'))
        result_layers.append({'compressedDigest': digest, 'compressedBytes': record['bytes'], 'diffID': diffid, 'uncompressedBytes': size})
    if not 4 <= len(entries) <= MAX_MEMBERS or sum(row[2]['bytes'] for row in entries.values()) > MAX_STORE:
        raise ValueError('registry store aggregate members exceed bound')
    # Refuse the padded tar size before writing, including record padding.
    required = 1024 + sum(512 + ((row[2]['bytes'] + 511) // 512) * 512 for row in entries.values())
    if ((required + 10239) // 10240) * 10240 > MAX_STORE:
        raise ValueError('registry store tar exceeds bound')
    output.mkdir(mode=0o700)
    destination = output / STORE_NAME
    records = []
    with tarfile.open(destination, 'w', format=tarfile.USTAR_FORMAT) as target:
        for name in sorted(entries, key=lambda value: value.encode('ascii')):
            raw, file, record = entries[name]
            entry = tarfile.TarInfo(name)
            entry.mode, entry.mtime, entry.uid, entry.gid, entry.size = 0o644, 0, 0, 0, record['bytes']
            with (io.BytesIO(raw) if raw is not None else file.open('rb')) as stream:
                target.addfile(entry, stream)
            if raw is None and archive.file_record(file) != dict(record, path=file.name):
                raise ValueError('registry store source changed during publication')
            records.append({'path': name, 'sha256': record['sha256'], 'bytes': record['bytes']})
    stored = archive.file_record(destination)
    receipt = {'version': 2, 'kind': 'luci-runtime-registry-conversion', 'selectedImageID': image_id,
               'registryManifestDigest': manifest_digest, 'config': {'digest': image_id, 'bytes': config_record['bytes']},
               'layers': result_layers, 'registryStore': {'sha256': stored['sha256'], 'bytes': stored['bytes'], 'memberCount': len(records)},
               'verification': {'compressedDigestsVerified': True, 'diffIDsVerified': True, 'configVerified': True}}
    pull_metadata.write(output / RECEIPT_NAME, receipt)
    return {'registryStore': {'fileName': STORE_NAME, 'sha256': stored['sha256'], 'bytes': stored['bytes'],
                             'repository': INSTALLED_REPOSITORY, 'members': records}, 'receipt': receipt}

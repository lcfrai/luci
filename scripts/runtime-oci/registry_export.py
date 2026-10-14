"""Bounded, read-only conversion of an owned loopback registry image to v1 tar.

The caller owns the Distribution registry, its serial upload gate and the
exact Docker push lifecycle. This module performs no push, tag or cleanup.
It retains original registry bytes alongside the normalized 763 archive.
"""

from __future__ import annotations

import gzip
import hashlib
import http.client
import io
from pathlib import Path
import re
import shutil
import tarfile
import time
from urllib.parse import urlsplit

import archive
from common import plain_directory, write_json


HEX = re.compile(r'[0-9a-f]{64}\Z')
REPOSITORY = re.compile(r'luci/runtime-export-[0-9a-f]{32}\Z')
MAX_BLOB = archive.MAX_ARCHIVE
BLOCK = 1 << 20
MIN_FREE = 3 * archive.MAX_ARCHIVE + (2 << 30)
LAYER_SECONDS = 600
DOCKER_MANIFEST = 'application/vnd.docker.distribution.manifest.v2+json'
DOCKER_CONFIG = 'application/vnd.docker.container.image.v1+json'
DOCKER_LAYER = 'application/vnd.docker.image.rootfs.diff.tar.gzip'


def _identity(value):
    if not isinstance(value, str) or not value.startswith('sha256:') or not HEX.fullmatch(value[7:]):
        raise ValueError('registry identity is not a sha256 digest')
    return value


def _endpoint(value):
    parsed = urlsplit(value)
    if (parsed.scheme != 'http' or parsed.hostname not in ('127.0.0.1', 'localhost') or
            parsed.username or parsed.password or parsed.query or parsed.fragment or parsed.path not in ('', '/') or
            parsed.port is None or not 1024 <= parsed.port <= 65535):
        raise ValueError('registry endpoint must be an owned loopback port')
    return parsed.port


class LoopbackRegistry:
    """One bounded response at a time; redirects and foreign hosts are refused."""

    def __init__(self, endpoint, repository):
        self.port = _endpoint(endpoint)
        if not isinstance(repository, str) or not REPOSITORY.fullmatch(repository):
            raise ValueError('registry repository is not the owned export name')
        self.repository = repository

    def read(self, path, destination, expected_digest, limit, accept):
        _identity(expected_digest)
        if not path.startswith('/v2/' + self.repository + '/') or '?' in path or '#' in path:
            raise ValueError('registry read escaped the selected repository')
        connection = http.client.HTTPConnection('127.0.0.1', self.port, timeout=30)
        try:
            connection.request('GET', path, headers={'Accept': accept, 'Connection': 'close'})
            response = connection.getresponse()
            if response.status != 200 or response.getheader('Location') or response.getheader('Docker-Content-Digest') not in (None, expected_digest):
                raise ValueError('registry response changed or redirected')
            length = response.getheader('Content-Length')
            if length is not None and (not length.isdecimal() or int(length) > limit):
                raise ValueError('registry response exceeds its bound')
            digest, total = hashlib.sha256(), 0
            with destination.open('xb') as output:
                while True:
                    data = response.read(BLOCK)
                    if not data:
                        break
                    total += len(data)
                    if total > limit:
                        raise ValueError('registry response exceeds its bound')
                    digest.update(data)
                    output.write(data)
            if total == 0 or length is not None and int(length) != total or digest.hexdigest() != expected_digest[7:]:
                raise ValueError('registry content differs from its immutable digest')
            return total
        finally:
            connection.close()


def _private_output(output):
    output = Path(output).absolute()
    plain_directory(output.parent)
    if output.exists() or output.is_symlink():
        raise ValueError('registry export must retain a fresh private output')
    if shutil.disk_usage(output.parent).free < MIN_FREE:
        raise ValueError('registry conversion needs bounded E: scratch headroom')
    output.mkdir(mode=0o700)
    return output


def _layer(source, destination, expected_diffid, limit):
    """Decode one gzip layer, bounding memory and disk before the next layer."""
    _identity(expected_diffid)
    deadline = time.monotonic() + LAYER_SECONDS
    with source.open('rb') as compressed:
        with gzip.GzipFile(fileobj=compressed, mode='rb') as decoded, destination.open('xb') as output:
            digest, total = hashlib.sha256(), 0
            while True:
                if time.monotonic() > deadline:
                    raise ValueError('registry layer conversion exceeded its time bound')
                block = decoded.read(BLOCK)
                if not block:
                    break
                total += len(block)
                if total > limit:
                    raise ValueError('registry layer expands beyond archive bound')
                digest.update(block)
                output.write(block)
    if not total or digest.hexdigest() != expected_diffid[7:]:
        raise ValueError('registry layer differs from original uncompressed diffID')
    return total


def convert(endpoint, repository, manifest_digest, image_id, expected_diffids, output, *, reader=None):
    """Return exact transport and normalized-archive evidence for one image.

    `reader` may be a fixture with the same read method; production uses only
    the fixed loopback HTTP client. Caller must bind a positive, owned terminal
    push/serial-gate receipt before this read-only conversion is accepted.
    """
    _identity(manifest_digest)
    _identity(image_id)
    if (not isinstance(expected_diffids, list) or not 1 <= len(expected_diffids) <= 2048 or
            len(set(expected_diffids)) != len(expected_diffids)):
        raise ValueError('original image diffID inventory is invalid')
    for item in expected_diffids:
        _identity(item)
    _endpoint(endpoint)
    if not isinstance(repository, str) or not REPOSITORY.fullmatch(repository):
        raise ValueError('registry repository is not the owned export name')
    registry = reader if reader is not None else LoopbackRegistry(endpoint, repository)
    work = _private_output(output)
    registry_root = work / 'registry'
    registry_root.mkdir(mode=0o700)
    manifest_path = registry_root / 'manifest.json'
    registry.read('/v2/' + repository + '/manifests/' + manifest_digest, manifest_path,
                  manifest_digest, archive.MAX_METADATA, DOCKER_MANIFEST)
    with manifest_path.open('rb') as stream:
        manifest = archive.json_bytes(stream.read(archive.MAX_METADATA + 1))
    if (not isinstance(manifest, dict) or manifest.get('schemaVersion') != 2 or
            manifest.get('mediaType') != DOCKER_MANIFEST or
            set(manifest) - {'schemaVersion', 'mediaType', 'config', 'layers'}):
        raise ValueError('registry manifest is not the exact Docker schema2 image')
    config, layers = manifest.get('config'), manifest.get('layers')
    if (not isinstance(config, dict) or set(config) != {'mediaType', 'size', 'digest'} or
            config['mediaType'] != DOCKER_CONFIG or config['digest'] != image_id or
            not isinstance(config['size'], int) or isinstance(config['size'], bool) or
            not 0 < config['size'] <= archive.MAX_METADATA or not isinstance(layers, list) or
            len(layers) != len(expected_diffids)):
        raise ValueError('registry image config or layer count differs')
    config_path = registry_root / 'config.json'
    actual = registry.read('/v2/' + repository + '/blobs/' + image_id, config_path,
                           image_id, archive.MAX_METADATA, DOCKER_CONFIG)
    if actual != config['size']:
        raise ValueError('registry image config size changed')
    with config_path.open('rb') as stream:
        raw_config = stream.read(archive.MAX_METADATA + 1)
    image_config = archive.json_bytes(raw_config)
    if archive.config_policy(image_config) != expected_diffids:
        raise ValueError('registry image execution config or diffIDs changed')
    compressed_records, layer_records = [], []
    total = 0
    for index, (row, diffid) in enumerate(zip(layers, expected_diffids)):
        if (not isinstance(row, dict) or set(row) != {'mediaType', 'size', 'digest'} or
                row['mediaType'] != DOCKER_LAYER or not isinstance(row['size'], int) or
                isinstance(row['size'], bool) or not 0 < row['size'] <= MAX_BLOB):
            raise ValueError('registry layer uses unsupported media/size/foreign URLs')
        digest = _identity(row['digest'])
        compressed = registry_root / (str(index) + '.layer.gz')
        size = registry.read('/v2/' + repository + '/blobs/' + digest, compressed,
                             digest, min(MAX_BLOB, archive.MAX_ARCHIVE - total), DOCKER_LAYER)
        if size != row['size']:
            raise ValueError('registry compressed layer length changed')
        total += size
        decoded = registry_root / (str(index) + '.layer.tar')
        expanded = _layer(compressed, decoded, diffid, archive.MAX_ARCHIVE - sum(x['bytes'] for x in layer_records))
        layer_records.append({'path': diffid[7:] + '/layer.tar', 'sha256': diffid[7:], 'bytes': expanded})
        compressed_records.append({'digest': digest, 'bytes': size, 'diffID': diffid})
    members = {image_id[7:] + '.json': (raw_config, None),
               'manifest.json': (archive.encoded([{'Config': image_id[7:] + '.json', 'RepoTags': [],
                   'Layers': [row['path'] for row in layer_records]}]), None)}
    for index, row in enumerate(layer_records):
        members[row['path']] = (None, registry_root / (str(index) + '.layer.tar'))
    tar_bytes = 1024 + sum(512 + 512 * (((len(raw) if raw is not None else file.stat().st_size) + 511) // 512)
                           for raw, file in members.values())
    if ((tar_bytes + 10239) // 10240) * 10240 > archive.MAX_ARCHIVE:
        raise ValueError('normalized registry archive exceeds 8 GiB')
    destination = work / 'runtime-image-linux-amd64.tar'
    records = []
    with tarfile.open(destination, 'w', format=tarfile.USTAR_FORMAT) as target:
        for name in sorted(members):
            raw, file = members[name]
            length = len(raw) if raw is not None else file.stat().st_size
            entry = tarfile.TarInfo(name)
            entry.size, entry.mode, entry.mtime = length, 0o644, 0
            with (io.BytesIO(raw) if raw is not None else file.open('rb')) as source:
                target.addfile(entry, source)
            records.append({'path': name, 'sha256': hashlib.sha256(raw).hexdigest() if raw is not None else name.split('/')[0], 'bytes': length})
    if destination.stat().st_size > archive.MAX_ARCHIVE:
        raise ValueError('normalized registry archive exceeds 8 GiB')
    image = {'id': image_id, 'configPath': image_id[7:] + '.json',
             'layerPaths': [row['path'] for row in layer_records], 'diffIDs': expected_diffids, 'repoTags': []}
    archive.verify(destination, image, records)
    receipt = {'version': 1, 'state': 'registry_readback_normalized_original_image_verified',
               'registryManifestDigest': manifest_digest, 'imageID': image_id,
               'compressedLayers': compressed_records, 'archive': archive.file_record(destination),
               'members': records, 'image': image, 'pushAndRegistryOwnershipVerified': False}
    write_json(work / 'registry-export-receipt.json', receipt)
    return receipt

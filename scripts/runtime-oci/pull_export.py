"""Compressed-only immutable readback for v2. Never writes a v1 large tar."""
from pathlib import Path
import shutil
import time

import archive
from common import plain_directory
import registry_export as transport
import registry_store
import pull_metadata


def readback(endpoint, repository, digest, image_id, diffids, output, reader=None):
    transport._endpoint(endpoint)
    if not isinstance(repository, str) or not transport.REPOSITORY.fullmatch(repository) or not isinstance(diffids, list):
        raise ValueError('v2 readback repository/diffID inventory invalid')
    for identity in [digest, image_id, *diffids]:
        transport._identity(identity)
    if not 1 <= len(diffids) <= 2048 or len(set(diffids)) != len(diffids):
        raise ValueError('v2 readback original diffID inventory invalid')
    registry = transport.LoopbackRegistry(endpoint, repository) if reader is None else reader
    output = Path(output).absolute()
    plain_directory(output.parent)
    if output.exists() or output.is_symlink() or shutil.disk_usage(output.parent).free < (10 << 30):
        raise ValueError('v2 readback requires fresh output and 10 GiB scratch reserve')
    (output / 'registry').mkdir(parents=True, mode=0o700)
    root = output / 'registry'
    registry.read('/v2/' + repository + '/manifests/' + digest, root / 'manifest.json', digest,
                  archive.MAX_METADATA, transport.DOCKER_MANIFEST)
    manifest = archive.json_bytes((root / 'manifest.json').read_bytes())
    if (not isinstance(manifest, dict) or set(manifest) != {'schemaVersion', 'mediaType', 'config', 'layers'} or
            manifest['schemaVersion'] != 2 or manifest['mediaType'] != transport.DOCKER_MANIFEST):
        raise ValueError('v2 readback requires exact Docker schema2 manifest')
    config, layers = manifest['config'], manifest['layers']
    if (not isinstance(config, dict) or set(config) != {'mediaType', 'size', 'digest'} or
            config['mediaType'] != transport.DOCKER_CONFIG or config['digest'] != image_id or
            type(config['size']) is not int or not 0 < config['size'] <= archive.MAX_METADATA or
            not isinstance(layers, list) or len(layers) != len(diffids)):
        raise ValueError('v2 readback config or layer count changed')
    length = registry.read('/v2/' + repository + '/blobs/' + image_id, root / 'config.json', image_id,
                           archive.MAX_METADATA, transport.DOCKER_CONFIG)
    if length != config['size'] or archive.config_policy(archive.json_bytes((root / 'config.json').read_bytes())) != diffids:
        raise ValueError('v2 readback original config changed')
    compressed, expanded, seen = 0, 0, set()
    deadline = time.monotonic() + 1800
    for index, (layer, diffid) in enumerate(zip(layers, diffids)):
        if (not isinstance(layer, dict) or set(layer) != {'mediaType', 'size', 'digest'} or
                layer['mediaType'] != transport.DOCKER_LAYER or type(layer['size']) is not int or
                not 0 < layer['size'] <= registry_store.MAX_STORE - compressed or layer['digest'] in seen):
            raise ValueError('v2 readback compressed layer descriptor invalid')
        seen.add(transport._identity(layer['digest']))
        file = root / (str(index) + '.layer.gz')
        length = registry.read('/v2/' + repository + '/blobs/' + layer['digest'], file, layer['digest'],
                               layer['size'], transport.DOCKER_LAYER)
        if length != layer['size']:
            raise ValueError('v2 readback compressed length changed')
        compressed += length
        expanded += registry_store.expanded_record(file, diffid, registry_store.MAX_EXPANDED - expanded, deadline)
    value = {'version': 2, 'state': 'registry_readback_compressed_original_image_verified',
             'imageID': image_id, 'image': {'diffIDs': diffids}, 'registryManifestDigest': digest,
             'pushAndRegistryOwnershipVerified': False}
    pull_metadata.write(output / 'registry-readback-v2.json', value)
    return value

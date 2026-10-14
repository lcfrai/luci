"""Inert registry bytes; no Docker daemon, registry, or network is started."""

import gzip
import hashlib
import io
from pathlib import Path
import tarfile
import tempfile
import unittest
from unittest.mock import patch

import archive
import registry_export


def identity(data):
    return 'sha256:' + hashlib.sha256(data).hexdigest()


class BytesReader:
    def __init__(self, rows):
        self.rows = rows
        self.reads = []

    def read(self, path, destination, expected_digest, limit, accept):
        self.reads.append((path, accept))
        data = self.rows[path]
        if not 0 < len(data) <= limit or identity(data) != expected_digest:
            raise ValueError('immutable registry blob changed')
        destination.write_bytes(data)
        return len(data)


class RegistryConversionTests(unittest.TestCase):
    def setUp(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        self.root = Path(temp.name)
        self.repository = 'luci/runtime-export-' + 'a' * 32
        self.endpoint = 'http://127.0.0.1:37891'
        stream = io.BytesIO()
        with tarfile.open(fileobj=stream, mode='w') as tar:
            entry = tarfile.TarInfo('hello')
            entry.size = 5
            tar.addfile(entry, io.BytesIO(b'hello'))
        self.layer = stream.getvalue()
        self.diffid = identity(self.layer)
        self.compressed = gzip.compress(self.layer, mtime=0)
        self.compressed_id = identity(self.compressed)
        config = {'os': 'linux', 'architecture': 'amd64', 'config': {
            'User': 'dev', 'WorkingDir': '/home/dev',
            'Entrypoint': ['/usr/local/bin/entrypoint.sh'],
            'ExposedPorts': {'8080/tcp': {}}, 'Labels': {'luci.runtime-payload.source': 'fixture'}},
            'rootfs': {'type': 'layers', 'diff_ids': [self.diffid]}}
        self.config = archive.encoded(config)
        self.image_id = identity(self.config)
        self.manifest = {
            'schemaVersion': 2, 'mediaType': registry_export.DOCKER_MANIFEST,
            'config': {'mediaType': registry_export.DOCKER_CONFIG, 'size': len(self.config), 'digest': self.image_id},
            'layers': [{'mediaType': registry_export.DOCKER_LAYER, 'size': len(self.compressed), 'digest': self.compressed_id}]}
        self.prepare()

    def prepare(self):
        self.manifest_bytes = archive.encoded(self.manifest)
        self.manifest_id = identity(self.manifest_bytes)
        prefix = '/v2/' + self.repository
        self.reader = BytesReader({prefix + '/manifests/' + self.manifest_id: self.manifest_bytes,
                                   prefix + '/blobs/' + self.image_id: self.config,
                                   prefix + '/blobs/' + self.compressed_id: self.compressed})

    def convert(self, name='out'):
        with patch.object(registry_export.shutil, 'disk_usage') as usage:
            usage.return_value.free = registry_export.MIN_FREE
            return registry_export.convert(self.endpoint, self.repository, self.manifest_id,
                                           self.image_id, [self.diffid], self.root / name, reader=self.reader)

    def test_original_config_and_diffid_become_tagless_verified_archive(self):
        result = self.convert()
        self.assertEqual(result['image']['id'], self.image_id)
        self.assertEqual(result['image']['repoTags'], [])
        self.assertEqual(result['compressedLayers'][0]['digest'], self.compressed_id)
        self.assertFalse(result['pushAndRegistryOwnershipVerified'])
        self.assertTrue(archive.verify(self.root / 'out/runtime-image-linux-amd64.tar',
                                       result['image'], result['members']))
        self.assertEqual(len(self.reader.reads), 3)
        with self.assertRaisesRegex(ValueError, 'fresh private output'):
            self.convert()

    def test_manifest_urls_foreign_layer_and_config_drift_refused(self):
        self.manifest['layers'][0]['urls'] = ['https://foreign.example/layer']
        self.prepare()
        with self.assertRaisesRegex(ValueError, 'foreign URLs'):
            self.convert('foreign')
        self.manifest['layers'][0].pop('urls')
        self.manifest['config']['digest'] = 'sha256:' + '0' * 64
        self.prepare()
        with self.assertRaisesRegex(ValueError, 'config'):
            self.convert('config')

    def test_wrong_uncompressed_diffid_is_refused_even_with_valid_registry_digests(self):
        with self.assertRaisesRegex(ValueError, 'diffID'):
            with patch.object(registry_export.shutil, 'disk_usage') as usage:
                usage.return_value.free = registry_export.MIN_FREE
                registry_export.convert(self.endpoint, self.repository, self.manifest_id, self.image_id,
                                        ['sha256:' + '0' * 64], self.root / 'wrong', reader=self.reader)

    def test_invalid_endpoint_repository_and_disk_refused_before_read(self):
        for endpoint, repository in [('http://10.0.0.1:37891', self.repository),
                                     ('http://127.0.0.1:37891@evil.test', self.repository),
                                     (self.endpoint, 'foreign/repository')]:
            with self.subTest(endpoint=endpoint, repository=repository):
                with self.assertRaisesRegex(ValueError, 'loopback|repository'):
                    registry_export.convert(endpoint, repository, self.manifest_id, self.image_id,
                                            [self.diffid], self.root / 'invalid', reader=self.reader)
        with patch.object(registry_export.shutil, 'disk_usage') as usage:
            usage.return_value.free = registry_export.MIN_FREE - 1
            with self.assertRaisesRegex(ValueError, 'headroom'):
                registry_export.convert(self.endpoint, self.repository, self.manifest_id, self.image_id,
                                        [self.diffid], self.root / 'no-disk', reader=self.reader)
        self.assertFalse(self.reader.reads)

    def test_oversized_compressed_descriptor_and_digest_drift_refused(self):
        self.manifest['layers'][0]['size'] = registry_export.MAX_BLOB + 1
        self.prepare()
        with self.assertRaisesRegex(ValueError, 'size'):
            self.convert('oversized')
        self.manifest['layers'][0]['size'] = len(self.compressed)
        self.prepare()
        prefix = '/v2/' + self.repository
        self.reader.rows[prefix + '/blobs/' + self.compressed_id] = b'changed'
        with self.assertRaisesRegex(ValueError, 'changed'):
            self.convert('drift')

    def test_expansion_is_bounded_before_an_extra_layer_is_accepted(self):
        source = self.root / 'bomb.gz'
        source.write_bytes(gzip.compress(b'a' * 10000, mtime=0))
        with self.assertRaisesRegex(ValueError, 'expands'):
            registry_export._layer(source, self.root / 'bomb.tar', identity(b'a' * 10000), 9)


if __name__ == '__main__':
    unittest.main()

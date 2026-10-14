"""Offline immutable-store tests; no real registry/daemon/model acceptance."""
import copy
import hashlib
import tarfile
import unittest
from unittest.mock import patch

import archive
import registry_store
import test_registry_export as fixtures


class RegistryStoreTests(unittest.TestCase):
    def setUp(self):
        self.fixture = fixtures.RegistryConversionTests()
        self.fixture.setUp()
        self.addCleanup(self.fixture.doCleanups)
        self.converted = self.fixture.convert()

    def create(self, name='store'):
        return registry_store.create(self.fixture.root / 'out', self.converted, self.fixture.image_id,
                                     [self.fixture.diffid], self.fixture.root / name)

    def test_fixed_repository_exact_blobs_regular_sorted_and_deterministic(self):
        first, second = self.create(), self.create('second')
        self.assertEqual(first, second)
        self.assertEqual(first['registryStore']['repository'], 'luci/runtime-installed')
        self.assertEqual(first['receipt']['selectedImageID'], self.fixture.image_id)
        self.assertEqual(first['receipt']['layers'][0]['uncompressedBytes'], len(self.fixture.layer))
        with tarfile.open(self.fixture.root / 'store' / registry_store.STORE_NAME) as tar:
            members = tar.getmembers()
            self.assertEqual([m.name for m in members], sorted(m.name for m in members))
            self.assertEqual(len(members), first['receipt']['registryStore']['memberCount'])
            for member, expected in zip(members, first['registryStore']['members']):
                self.assertTrue(member.isfile())
                self.assertEqual((member.uid, member.gid, member.mtime, member.mode), (0, 0, 0, 0o644))
                data = tar.extractfile(member).read()
                self.assertEqual(hashlib.sha256(data).hexdigest(), expected['sha256'])
                self.assertEqual(len(data), expected['bytes'])
                self.assertNotIn('runtime-export-', member.name)
                self.assertNotIn('/tags/', member.name)
            config = next(m for m in members if m.name.endswith(self.fixture.image_id[7:] + '/data'))
            self.assertEqual(tar.extractfile(config).read(), self.fixture.config)
        with self.assertRaisesRegex(ValueError, 'fresh output'):
            self.create()

    def test_original_identity_compressed_bytes_and_diffid_must_match(self):
        with self.assertRaisesRegex(ValueError, 'identity differs'):
            registry_store.create(self.fixture.root / 'out', self.converted, 'sha256:' + '0' * 64,
                                  [self.fixture.diffid], self.fixture.root / 'foreign')
        layer = self.fixture.root / 'out/registry/0.layer.gz'
        layer.write_bytes(b'changed')
        with self.assertRaisesRegex(ValueError, 'digest or length changed'):
            self.create()
        self.assertFalse((self.fixture.root / 'store').exists())

    def test_expanded_and_store_aggregate_bounds_refuse_before_publication(self):
        with patch.object(registry_store, 'MAX_EXPANDED', len(self.fixture.layer) - 1):
            with self.assertRaisesRegex(ValueError, 'expanded bytes'):
                self.create()
        with patch.object(registry_store, 'MAX_STORE', 512):
            with self.assertRaisesRegex(ValueError, 'members|tar|compressed'):
                self.create()
        self.assertFalse((self.fixture.root / 'store').exists())

    def test_foreign_descriptor_and_manifest_hash_drift_refused(self):
        bad = copy.deepcopy(self.converted)
        bad['registryManifestDigest'] = 'sha256:' + '0' * 64
        with self.assertRaisesRegex(ValueError, 'digest or length'):
            registry_store.create(self.fixture.root / 'out', bad, self.fixture.image_id,
                                  [self.fixture.diffid], self.fixture.root / 'changed')
        layer = self.fixture.root / 'out/registry/0.layer.gz'
        saved = layer.read_bytes()
        target = layer.with_suffix('.original')
        layer.rename(target)
        try:
            layer.symlink_to(target)
        except OSError:
            self.skipTest('symbolic link creation unavailable for this Windows account')
        with self.assertRaisesRegex(ValueError, 'linked runtime'):
            self.create()
        self.assertEqual(target.read_bytes(), saved)


if __name__ == '__main__':
    unittest.main()

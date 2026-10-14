"""Tiny inert v2 bytes exercise producer/export/package trust joins offline."""
import copy
import gzip
import hashlib
import io
from pathlib import Path
import shutil
import tarfile
import unittest
from unittest.mock import patch

import archive
import bootstrap
from common import read_json, write_json
import payload as legacy
import producer
import pull_export
import pull_metadata
import pull_payload
import pull_producer
import registry_export
import registry_store
import test_bootstrap
import test_payload
from test_registry_export import BytesReader, identity


class PullPayloadTests(unittest.TestCase):
    def setUp(self):
        self.v1 = test_payload.PayloadTests()
        self.v1.setUp()
        self.addCleanup(self.v1.doCleanups)
        self.root, self.repo, self.revision = self.v1.root, self.v1.repo, self.v1.revision
        prior = read_json(self.v1.output / legacy.MANIFEST)
        image = prior['image']['id']
        with tarfile.open(self.v1.output / legacy.PAYLOAD) as saved:
            config = saved.extractfile(prior['image']['configPath']).read()
            layer = saved.extractfile(prior['image']['layerPaths'][0]).read()
        compressed = gzip.compress(layer, mtime=0)
        manifest = {'schemaVersion': 2, 'mediaType': registry_export.DOCKER_MANIFEST,
            'config': {'mediaType': registry_export.DOCKER_CONFIG, 'size': len(config), 'digest': image},
            'layers': [{'mediaType': registry_export.DOCKER_LAYER, 'size': len(compressed), 'digest': identity(compressed)}]}
        raw = archive.encoded(manifest)
        digest = identity(raw)
        self.repository = 'luci/runtime-export-' + 'a' * 32
        prefix = '/v2/' + self.repository
        reader = BytesReader({prefix + '/manifests/' + digest: raw, prefix + '/blobs/' + image: config,
                              prefix + '/blobs/' + identity(compressed): compressed})
        with patch.object(pull_export.shutil, 'disk_usage') as usage:
            usage.return_value.free = 10 << 30
            converted = pull_export.readback('http://127.0.0.1:37891', self.repository, digest, image,
                prior['image']['diffIDs'], self.root / 'compressed', reader)
        self.assertFalse((self.root / 'compressed' / legacy.PAYLOAD).exists())
        self.assertFalse(list((self.root / 'compressed').rglob('*.layer.tar')))
        self.target = self.root / 'v2'
        store = registry_store.create(self.root / 'compressed', converted, image, prior['image']['diffIDs'], self.target)
        shutil.copytree(self.v1.output / 'canonical', self.target / 'canonical')
        shutil.copytree(self.v1.output / 'licenses', self.target / 'licenses')
        for src, dst in ((legacy.MANIFEST, legacy.PRIOR_MANIFEST), (legacy.RECEIPT, legacy.RECEIPT), (legacy.RECEIPT, legacy.PRIOR_RECEIPT)):
            shutil.copyfile(self.v1.output / src, self.target / dst)
        tiny = test_bootstrap.BootstrapTests()
        raw, image_id, gate, distribution = tiny.fixture(self.root)
        b = bootstrap.normalize(raw, self.target / bootstrap.NAME, image_id, gate, distribution)
        notice = self.target / 'licenses/runtime-bootstrap/distribution/LICENSE'
        notice.parent.mkdir(parents=True)
        notice.write_bytes(b'original fixture Distribution grant')
        notice_row = dict(archive.file_record(notice, notice.relative_to(self.target).as_posix()),
            component='distribution/distribution', version='3.0.0', grant='Apache-2.0', provenance='Fixture only')
        b.update(distributionVersion='3.0.0', distributionArchiveSHA256='61c9a2c0d5981a78482025b6b69728521fbc78506d68b223d4a2eb825de5ca3d',
            distributionBinarySHA256=distribution, distributionLicenseSHA256=notice_row['sha256'],
            gateSourceSHA256='b' * 64, gateBinarySHA256=gate, buildRecipeSHA256='c' * 64)
        receipt = {'version': 1, 'kind': 'luci-runtime-registry-bootstrap', 'buildSourceCommit': self.revision,
            'gateSourceSHA256': b['gateSourceSHA256'], 'buildRecipeSHA256': b['buildRecipeSHA256'],
            'distribution': {'version': '3.0.0', 'archiveSHA256': b['distributionArchiveSHA256'],
                             'binarySHA256': distribution, 'licenseSHA256': notice_row['sha256']},
            'builder': {'argv': ['docker', 'build', '--network=none', '.'], 'version': '28.5.1', 'memoryMiB': 256,
                        'cpuMillis': 1000, 'exitStatus': 0, 'stdoutSHA256': 'd' * 64, 'stderrSHA256': 'e' * 64},
            'image': {'id': image_id, 'configSHA256': b['configSHA256'], 'configBytes': b['configBytes'], 'layers': b['layers'],
                      'archiveSHA256': b['sha256'], 'archiveBytes': b['bytes']},
            'probe': {'argv': ['docker', 'run', '--version'], 'exitStatus': 0,
                      'stdoutSHA256': hashlib.sha256(b'luci-registry-gate 1\n').hexdigest(), 'stderrSHA256': hashlib.sha256(b'').hexdigest()}}
        pull_metadata.write(self.target / bootstrap.RECEIPT, receipt)
        b['buildReceiptSHA256'] = archive.file_record(self.target / bootstrap.RECEIPT)['sha256']
        proof = {'version': 2, 'kind': 'luci-runtime-pull-producer', 'sourceCommit': self.revision,
            'canonicalInputDigest': prior['canonicalInputDigest'], 'selectedImageID': image,
            'originalBuildReceiptSHA256': prior['producer']['buildReceiptSHA256'],
            'conversionReceiptSHA256': archive.file_record(self.target / registry_store.RECEIPT_NAME)['sha256'],
            'registryStoreSHA256': store['registryStore']['sha256'], 'registryManifestDigest': digest,
            'push': {'session': 'a' * 32, 'repository': self.repository, 'engineID': 'engine-fixture', 'instanceID': 'instance-fixture',
                     'imageID': image, 'commandExitStatus': 0, 'terminalStreamSHA256': 'f' * 64, 'manifestReadbackDigest': digest,
                     'helperTerminalReceiptSHA256': 'a' * 64, 'helperRetirementReceiptSHA256': 'b' * 64}}
        pull_metadata.write(self.target / pull_payload.PULL_RECEIPT, proof)
        self.manifest = {'version': 2, 'kind': 'luci-runtime-registry-pull', 'sourceCommit': self.revision, 'target': prior['target'],
            'canonicalInputAlgorithm': prior['canonicalInputAlgorithm'], 'canonicalInputDigest': prior['canonicalInputDigest'],
            'sourceFiles': prior['sourceFiles'], 'runtime': prior['runtime'], 'registryStore': store['registryStore'], 'bootstrap': b,
            'image': {'id': image, 'configSHA256': image[7:], 'configBytes': len(config), 'manifestDigest': digest,
                      'manifestMediaType': registry_export.DOCKER_MANIFEST,
                      'layers': [dict(row, mediaType=registry_export.DOCKER_LAYER) for row in store['receipt']['layers']]},
            'licenses': sorted([*prior['licenses'], notice_row], key=lambda row: row['path']),
            'producer': {'originalBuildReceiptSHA256': proof['originalBuildReceiptSHA256'], 'conversionReceiptSHA256': proof['conversionReceiptSHA256'],
                         'pullProducerReceiptSHA256': archive.file_record(self.target / pull_payload.PULL_RECEIPT)['sha256']}}
        write_json(self.target / legacy.SOURCE, {'version': 1, 'sourceCommit': self.revision, 'sourceDirty': False})
        pull_metadata.write(self.target / pull_payload.MANIFEST, self.manifest)
        pull_payload.verify(self.target)

    def test_staging_core_and_identity_have_one_payload_and_original_evidence(self):
        package = self.v1.package()
        result = pull_payload.stage_payload(self.target, self.repo, self.revision, package)
        self.assertEqual(result, self.manifest)
        self.assertEqual((package / 'runtime' / legacy.RECEIPT).read_bytes(), (self.v1.output / legacy.RECEIPT).read_bytes())
        self.assertFalse((package / 'runtime' / legacy.PAYLOAD).exists())
        identity_before = test_payload.PACKAGE.package_identity(package, 'linux')
        moved = self.root / 'another-machine'
        shutil.copytree(package, moved)
        self.assertEqual(test_payload.PACKAGE.package_identity(moved, 'linux'), identity_before)
        core = self.root / 'core'
        marker = pull_payload.core_from_package(package, core)
        self.assertEqual(marker['version'], 2)
        self.assertFalse((core / 'runtime' / registry_store.STORE_NAME).exists())
        self.assertFalse((core / 'runtime' / bootstrap.NAME).exists())
        self.assertTrue((core / 'licenses/app-LICENSE').exists())
        test_payload.PACKAGE.package_identity(core, 'linux')

    def test_later_source_reuse_is_exact_and_raw_original_receipts_retained(self):
        (self.repo / 'README.md').write_bytes(b'Documentation change only\n')
        self.v1.git('add', '.')
        self.v1.git('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'docs')
        revision = self.v1.git('rev-parse', 'HEAD').strip()
        package = self.v1.package()
        result = pull_payload.stage_payload(self.target, self.repo, revision, package)
        self.assertNotEqual(result['sourceCommit'], self.revision)
        self.assertIn('reuseEvidenceSHA256', result['producer'])
        self.assertEqual((package / 'runtime' / legacy.PRIOR_MANIFEST).read_bytes(), (self.v1.output / legacy.MANIFEST).read_bytes())
        self.assertEqual(read_json(package / 'runtime' / pull_payload.PULL_RECEIPT)['sourceCommit'], revision)
        reuse = read_json(package / 'runtime' / legacy.REUSE)
        reuse['archiveBytes'] += 1
        write_json(package / 'runtime' / legacy.REUSE, reuse)
        with self.assertRaisesRegex(ValueError, 'reuse receipt'):
            pull_payload.verify(package / 'runtime', package, package / 'runtime')

    def test_canonical_receipt_newline_and_extra_grants_refuse(self):
        file = self.target / pull_payload.CONVERSION
        file.write_bytes(file.read_bytes() + b'\n')
        self.manifest['producer']['conversionReceiptSHA256'] = archive.file_record(file)['sha256']
        pull_metadata.write(self.target / pull_payload.MANIFEST, self.manifest)
        with self.assertRaisesRegex(ValueError, 'canonical JSON'):
            pull_payload.verify(self.target)

    def test_source_and_aggregate_mismatch_refuse_before_effects(self):
        (self.repo / 'sandboxd/go.mod').write_bytes(b'changed\n')
        with patch.object(producer, 'command', wraps=producer.command) as effect:
            with self.assertRaisesRegex(ValueError, 'clean selected HEAD'):
                pull_payload.stage_payload(self.target, self.repo, self.revision, self.v1.package())
            self.assertTrue(effect.call_args_list)
            self.assertTrue(all(call.args[0][0] == 'git' for call in effect.call_args_list))

    def test_fresh_transport_tag_refuses_unrelated_image_and_real_daemon_errors(self):
        with patch.object(producer, 'command', return_value=b'[]'):
            with self.assertRaisesRegex(ValueError, 'already exists'):
                pull_producer.fresh_tag('owned:fixture')
        with patch.object(producer, 'command', side_effect=OSError('daemon unavailable')):
            with self.assertRaises(OSError):
                pull_producer.fresh_tag('owned:fixture')


if __name__ == '__main__': unittest.main()

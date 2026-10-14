"""Small real-byte archive/package tests. These are not Docker acceptance."""
import copy
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import subprocess
import tempfile
import unittest
from unittest.mock import patch

import archive
from canonical_inputs import inventory_digest, stage
from common import read_json, write_json
import payload
import producer
import test_archive

SCRIPTS = Path(__file__).resolve().parents[1]
SPEC = importlib.util.spec_from_file_location('package_tool', SCRIPTS / 'package.py')
PACKAGE = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(PACKAGE)


class PayloadTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo = self.root / 'source'
        self.repo.mkdir()
        for name in ('sandboxd/cmd/main.go', 'sandboxd/internal/server.go', 'sandboxd/vendor/modules.txt',
                     'sandboxd/go.mod', 'sandboxd/go.sum', 'images/sandbox/entrypoint.sh'):
            file = self.repo / name
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_bytes(b'canonical fixture\n')
        (self.repo / 'images/sandbox/Dockerfile').write_text('FROM golang:1.26-bookworm AS build\nFROM node:24-bookworm-slim\n', encoding='utf-8')
        self.git('init', '-q')
        self.git('add', '.')
        self.git('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'fixture')
        self.revision = self.git('rev-parse', 'HEAD').strip()
        self.output = self.root / 'producer'
        self.output.mkdir()
        inputs = stage(self.repo, self.revision, self.output / 'canonical')
        helper = test_archive.ArchiveTests()
        helper.setUp()
        self.addCleanup(helper.doCleanups)
        raw, image_id = helper.raw()
        normalized = archive.normalize(raw, self.output / payload.PAYLOAD, image_id)
        fixtures = SCRIPTS.parent / 'docs/luci/fixtures/runtime-oci-v1'
        receipt = read_json(fixtures / 'valid-build-receipt-structural.json')
        receipt.update(buildSourceCommit=self.revision, canonicalInputDigest=inputs['canonicalInputDigest'],
                       imageID=image_id, archive={key: normalized['archive'][key] for key in ('sha256', 'bytes')})
        write_json(self.output / payload.RECEIPT, receipt)
        notice = self.output / 'licenses/runtime-image/fixture/LICENSE'
        notice.parent.mkdir(parents=True)
        notice.write_bytes(b'Fixture grant retained exactly\n')
        license_row = dict(archive.file_record(notice, notice.relative_to(self.output).as_posix()),
                           component='fixture', version='1', grant='Fixture', provenance='Test-only fixture')
        manifest = read_json(fixtures / 'valid-structural.json')
        manifest.update(sourceCommit=self.revision, canonicalInputDigest=inputs['canonicalInputDigest'],
                        sourceFiles=inputs['sourceFiles'], image=normalized['image'], runtime=archive.RUNTIME,
                        licenses=[license_row], archive=dict(normalized['archive'], fileName=payload.PAYLOAD, members=normalized['members']))
        manifest['archive'].pop('path')
        manifest['producer'] = {'buildSourceCommit': self.revision,
                                'buildReceiptSHA256': archive.file_record(self.output / payload.RECEIPT)['sha256'],
                                'dockerfileSHA256': next(row['sha256'] for row in inputs['sourceFiles'] if row['path'] == 'images/sandbox/Dockerfile')}
        write_json(self.output / payload.MANIFEST, manifest)
        payload.verify(self.output)

    def git(self, *args):
        return subprocess.check_output(['git', '-C', str(self.repo), *args], text=True, stderr=subprocess.DEVNULL)

    def package(self, name='package'):
        package = self.root / name
        (package / 'web').mkdir(parents=True)
        (package / 'web/index.html').write_bytes(b'ui')
        (package / 'luci').write_bytes(b'binary')
        # Existing nonruntime licences must be retained, without joining the
        # exhaustive runtime-image inventory.
        (package / 'licenses').mkdir()
        (package / 'licenses/app-LICENSE').write_bytes(b'Application grant')
        return package

    def test_stage_and_exact_input_reuse_bind_original_raw_evidence(self):
        package = self.package()
        payload.stage_payload(self.output, self.repo, self.revision, package)
        first = PACKAGE.package_identity(package, 'linux')
        (self.repo / 'README.md').write_bytes(b'Nonruntime change\n')
        self.git('add', '.')
        self.git('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'docs')
        next_revision = self.git('rev-parse', 'HEAD').strip()
        reused = self.package('reused')
        manifest = payload.stage_payload(self.output, self.repo, next_revision, reused)
        self.assertEqual((reused / 'runtime' / payload.PRIOR_MANIFEST).read_bytes(), (self.output / payload.MANIFEST).read_bytes())
        self.assertEqual((reused / 'runtime' / payload.PRIOR_RECEIPT).read_bytes(), (self.output / payload.RECEIPT).read_bytes())
        self.assertNotEqual(first, PACKAGE.package_identity(reused, 'linux'))
        self.assertEqual(manifest['producer']['buildSourceCommit'], self.revision)
        value = read_json(reused / 'runtime' / payload.REUSE)
        value['priorManifestSHA256'] = '0' * 64
        write_json(reused / 'runtime' / payload.REUSE, value)
        with self.assertRaises(ValueError):
            PACKAGE.package_identity(reused, 'linux')

    def test_changed_runtime_dirty_source_and_unmanifested_grants_refused(self):
        file = self.repo / 'sandboxd/go.sum'
        file.write_bytes(b'changed')
        with self.assertRaisesRegex(ValueError, 'clean'):
            payload.stage_payload(self.output, self.repo, self.revision, self.package())
        self.git('add', '.')
        self.git('-c', 'user.name=Fixture', '-c', 'user.email=fixture@example.invalid', 'commit', '-qm', 'runtime change')
        with self.assertRaisesRegex(ValueError, 'changed runtime'):
            payload.stage_payload(self.output, self.repo, self.git('rev-parse', 'HEAD').strip(), self.package('changed'))
        (self.output / 'licenses/runtime-image/extra').write_bytes(b'unaccounted grant')
        with self.assertRaisesRegex(ValueError, 'unmanifested'):
            payload.verify(self.output)

    def test_core_has_one_explicit_companion_and_no_payload_recursion(self):
        package = self.package()
        payload.stage_payload(self.output, self.repo, self.revision, package)
        core = self.root / 'core'
        marker = payload.core_from_package(package, core)
        self.assertFalse((core / 'runtime' / payload.PAYLOAD).exists())
        self.assertFalse((core / 'runtime' / payload.MANIFEST).exists())
        self.assertTrue((core / 'licenses/app-LICENSE').exists())
        self.assertEqual(marker['ordinaryLocalInstall'], 'requires-verified-companion')
        self.assertIs(marker['automaticSourceBuildFallback'], False)
        PACKAGE.build_package(core, 'linux')
        identity = PACKAGE.package_identity(core, 'linux')
        spec = importlib.util.spec_from_file_location('upgrade_evaluator', SCRIPTS / 'portable-upgrade-identity-eval.py')
        evaluator = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(evaluator)
        provenance = self.root / 'provenance.json'
        write_json(provenance, {'sourceDirty': False, 'sourceCommit': self.revision,
                               'binarySHA256': archive.file_record(core / 'luci')['sha256'],
                               'webIndexSHA256': archive.file_record(core / 'web/index.html')['sha256'],
                               'archiveSHA256': archive.file_record(Path(str(core) + '.tar.gz'))['sha256'],
                               'archive': str(core) + '.tar.gz'})
        self.assertEqual(evaluator.verify_package(core, provenance, archive=True)['packageIdentity'], identity)
        marker['archiveSHA256'] = '0' * 64
        write_json(core / 'runtime' / payload.COMPANION, marker)
        self.assertNotEqual(identity, PACKAGE.package_identity(core, 'linux'))

    def test_role_specific_archive_limits_do_not_treat_payload_as_core(self):
        package = self.package()
        payload.stage_payload(self.output, self.repo, self.revision, package)
        core_bytes = sum(file.stat().st_size for file in package.rglob('*') if file.is_file() and file.relative_to(package).as_posix() != 'runtime/' + payload.PAYLOAD)
        payload_bytes = (package / 'runtime' / payload.PAYLOAD).stat().st_size
        with patch.object(PACKAGE, 'CORE_LIMIT', core_bytes + 1), patch.object(PACKAGE, 'PAYLOAD_LIMIT', payload_bytes):
            PACKAGE.build_package(package, 'linux')
        with patch.object(PACKAGE, 'PAYLOAD_LIMIT', payload_bytes - 1):
            with self.assertRaisesRegex(ValueError, 'role-specific'):
                PACKAGE.build_package(package, 'linux')
        with patch.object(PACKAGE, 'CORE_LIMIT', core_bytes - 1):
            with self.assertRaisesRegex(ValueError, 'role-specific'):
                PACKAGE.build_package(package, 'linux')

    def test_no_build_effect_when_source_dirty_or_capacity_unavailable(self):
        (self.repo / 'untracked').write_bytes(b'private')
        with patch.object(producer, 'docker_ids') as docker:
            with self.assertRaisesRegex(ValueError, 'clean'):
                producer.produce(self.repo, self.revision, self.root / 'denied')
            docker.assert_not_called()
        (self.repo / 'untracked').unlink()
        with patch.object(producer, 'clean_source'), patch.object(producer, 'docker_ids', return_value=[]), patch.object(producer, 'available_memory', return_value=producer.RESERVE), patch.object(producer.subprocess, 'Popen') as build:
            with self.assertRaisesRegex(ValueError, 'reserve'):
                producer.produce(self.repo, self.revision, self.root / 'low')
            build.assert_not_called()
            self.assertFalse((self.root / 'low').exists())


if __name__ == '__main__':
    unittest.main()

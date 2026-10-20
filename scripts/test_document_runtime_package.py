"""Generated ordinary-file package fixtures; no archive or native parser calls."""
import hashlib
import importlib.util
import json
from pathlib import Path
import shutil
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).parent))
import document_runtime_package as documents


class DocumentRuntimePackageTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.source = self.root / 'source'
        (self.source / 'manifests').mkdir(parents=True)
        self.package = self.root / 'package'
        (self.package / 'web').mkdir(parents=True)
        (self.package / 'web/index.html').write_bytes(b'owned web fixture')
        (self.package / 'luci').write_bytes(b'owned executable fixture, never executed')
        self.tool = b'{"fixture":"a bounded tool-domain object, not execution proof"}'
        self.payload = b'owned fixture bytes, not an archive'
        (self.source / documents.TOOL_PATH).write_bytes(self.tool)
        (self.source / 'proof-input').write_bytes(self.payload)
        self.profile = {
            'schema': 1, 'id': 'generated-fixture',
            'protocol': 'luci-debian-signature-go-owner-v1',
            'sourceSHA256': 'a' * 64,
            'files': [
                {'role': documents.TOOL_ROLE, 'relativePath': documents.TOOL_PATH, 'bytes': len(self.tool), 'sha256': self.digest(self.tool)},
                {'role': 'fixture', 'relativePath': 'proof-input', 'bytes': len(self.payload), 'sha256': self.digest(self.payload)},
            ],
        }
        self.write_profile()

    @staticmethod
    def digest(data):
        return hashlib.sha256(data).hexdigest()

    def write_profile(self):
        self.raw = json.dumps(self.profile, ensure_ascii=False, separators=(',', ':')).encode()
        (self.source / 'profile.json').write_bytes(self.raw)
        self.profile_pin, self.tool_pin = self.digest(self.raw), self.digest(self.tool)

    def stage(self):
        return documents.stage(self.source, self.package, self.profile_pin, self.tool_pin)

    def test_exact_independent_pins_stage_actual_files_and_verify_after_relocation(self):
        source = documents.verify_source(self.source, self.profile_pin, self.tool_pin)
        self.assertEqual(source['fileCount'], 2)
        result = self.stage()
        self.assertEqual(documents.read_pins(self.package), [self.profile_pin, self.tool_pin])
        self.assertEqual((self.package / documents.PROFILE_ROOT / 'profile.json').read_bytes(), self.raw)
        relocated = self.root / 'relocated'
        shutil.copytree(self.package, relocated)
        self.assertEqual(documents.verify_package(relocated)['profileSHA256'], result['profileSHA256'])
        self.assertEqual((relocated / documents.PROFILE_ROOT / 'proof-input').read_bytes(), self.payload)

    def test_missing_profile_is_explicit_but_orphan_files_or_partial_pins_refuse(self):
        self.assertIsNone(documents.verify_package(self.package))
        for profile, tool in (('', self.tool_pin), (self.profile_pin, ''), ('A' * 64, self.tool_pin)):
            with self.subTest(profile=profile, tool=tool), self.assertRaises(ValueError):
                documents.verify_source(self.source, profile, tool)
        (self.package / documents.PROFILE_ROOT).mkdir(parents=True)
        with self.assertRaises(ValueError):
            documents.verify_package(self.package)

    def test_descriptor_digest_cannot_appoint_itself_or_an_unrelated_tool_domain(self):
        with self.assertRaises(ValueError):
            documents.verify_source(self.source, 'b' * 64, self.tool_pin)
        with self.assertRaises(ValueError):
            documents.verify_source(self.source, self.profile_pin, 'b' * 64)
        (self.source / 'profile.json').write_bytes(self.raw + b'\n')
        with self.assertRaises(ValueError):
            documents.verify_source(self.source, self.digest(self.raw + b'\n'), self.tool_pin)
        self.profile['unexpected'] = 'not part of the owner descriptor'
        self.write_profile()
        with self.assertRaises(ValueError):
            documents.verify_source(self.source, self.profile_pin, self.tool_pin)

    def test_failed_copy_does_not_publish_configuration_or_overwrite_a_stage(self):
        (self.source / 'proof-input').write_bytes(b'changed bytes')
        with self.assertRaises(ValueError):
            self.stage()
        self.assertFalse((self.package / documents.PIN_FILE).exists())
        self.assertFalse((self.package / documents.PROFILE_ROOT / 'profile.json').exists())
        retained = (self.package / documents.PROFILE_ROOT / documents.TOOL_PATH).read_bytes()
        with self.assertRaises(ValueError):
            self.stage()
        self.assertEqual((self.package / documents.PROFILE_ROOT / documents.TOOL_PATH).read_bytes(), retained)

    def test_exact_package_members_pins_and_actual_bytes_are_required(self):
        self.stage()
        pins_file = self.package / documents.PIN_FILE
        pins_file.write_bytes(documents.pins(self.profile_pin, self.tool_pin).replace(b'\n', b'\r\n'))
        with self.assertRaises(ValueError):
            documents.verify_package(self.package)
        pins_file.write_bytes(documents.pins(self.profile_pin, self.tool_pin))
        extra = self.package / documents.PROFILE_ROOT / 'undeclared'
        extra.write_bytes(b'not selected')
        with self.assertRaises(ValueError):
            documents.verify_package(self.package)
        extra.unlink()
        (self.package / documents.PROFILE_ROOT / 'proof-input').write_bytes(b'tampered')
        with self.assertRaises(ValueError):
            documents.verify_package(self.package)

    def test_path_aliases_and_links_are_not_followed(self):
        for name in ('../outside', '/outside', 'manifests/../outside', 'MANIFESTS/../outside', 'a\\b', 'a:b', 'trailing. '):
            with self.subTest(name=name), self.assertRaises(ValueError):
                documents.relative_name(name)
        link = self.package / 'runtime'
        outside = self.root / 'outside'
        outside.mkdir()
        try:
            link.symlink_to(outside, target_is_directory=True)
        except (OSError, NotImplementedError):
            return  # Other assertions still run on hosts without symlink permission.
        with self.assertRaises(ValueError):
            self.stage()
        self.assertEqual(list(outside.iterdir()), [])

    def test_package_identity_includes_selected_profile_and_every_actual_proof_file(self):
        spec = importlib.util.spec_from_file_location('document_fixture_packaging', Path(__file__).with_name('package.py'))
        packaging = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(packaging)
        old_identity = packaging.package_identity(self.package, 'linux')
        self.stage()
        identity = packaging.package_identity(self.package, 'linux')
        self.assertNotEqual(identity, old_identity)
        self.assertEqual(identity, packaging.package_identity(self.package, 'linux'))
        (self.package / documents.PROFILE_ROOT / 'proof-input').write_bytes(b'changed')
        with self.assertRaises(ValueError):
            packaging.package_identity(self.package, 'linux')

    def test_one_fixed_primary_pinned_signature_descriptor_stages_its_exact_union(self):
        child = json.loads(self.raw)
        extra = b'owned generated signature-stage fixture'
        (self.source / 'signature-input').write_bytes(extra)
        child['files'].append({'role': 'signature-input', 'relativePath': 'signature-input', 'bytes': len(extra), 'sha256': self.digest(extra)})
        child_raw = json.dumps(child, separators=(',', ':')).encode()
        (self.source / 'profiles').mkdir()
        (self.source / documents.SIGNATURE_PATH).write_bytes(child_raw)
        self.profile['files'].append({'role': documents.SIGNATURE_ROLE, 'relativePath': documents.SIGNATURE_PATH, 'bytes': len(child_raw), 'sha256': self.digest(child_raw)})
        self.profile['purpose'] = 'binary-bindings'
        self.write_profile()
        self.stage()
        self.assertEqual((self.package / documents.PROFILE_ROOT / 'signature-input').read_bytes(), extra)
        self.assertEqual(len(documents.verify_package(self.package)['files']), 6)
        # Replacing the child descriptor cannot appoint a new trusted child.
        (self.package / documents.PROFILE_ROOT / documents.SIGNATURE_PATH).write_bytes(child_raw + b'\n')
        with self.assertRaises(ValueError):
            documents.verify_package(self.package)

    def test_signature_descriptor_role_is_closed_to_binary_parent_and_fixed_path(self):
        self.profile['files'].append({'role': documents.SIGNATURE_ROLE, 'relativePath': 'other-profile.json', 'bytes': 1, 'sha256': 'b' * 64})
        self.write_profile()
        with self.assertRaises(ValueError):
            documents.verify_source(self.source, self.profile_pin, self.tool_pin)
        self.profile['purpose'] = 'binary-bindings'
        self.write_profile()
        with self.assertRaises(ValueError):
            documents.verify_source(self.source, self.profile_pin, self.tool_pin)

    def image_fixture(self):
        directory = self.root / 'image-source'
        directory.mkdir()
        # An opaque generated transport fixture, deliberately NOT a Docker-save
        # archive or runtime acceptance. These tests do not parse any archive.
        archive = b'owned non-executable image transport fixture\n' * 64
        (directory / documents.IMAGE_ARCHIVE).write_bytes(archive)
        notice = b'Owned generated grant-evidence fixture; not an actual third-party grant.\n'
        (directory / 'notices').mkdir()
        (directory / 'notices/LICENSE-fixture.txt').write_bytes(notice)
        closure = {'schema': 1, 'kind': 'luci-document-notice-source-closure-v1',
                   'inputClosureSha256': 'a' * 64, 'sourceRetentionDigest': 'f' * 64,
                   'notices': [{'path': 'notices/LICENSE-fixture.txt', 'sha256': self.digest(notice), 'bytes': len(notice), 'component': 'generated-fixture', 'version': '1', 'grant': 'fixture-only', 'provenance': 'owned generated test data'}]}
        notice_raw = json.dumps(closure, separators=(',', ':')).encode()
        (directory / documents.IMAGE_NOTICES).write_bytes(notice_raw)
        config = 'b' * 64 + '.json'
        manifest = {
            'version': 1, 'kind': 'luci-document-prebuilt-docker-save', 'toolProtocol': 1,
            'provenance': {'assemblyReceiptSha256': 'a' * 64, 'inputClosureSha256': 'a' * 64,
                           'captureSha256': 'a' * 64, 'captureBytes': 1024,
                           'noticeClosureSha256': self.digest(notice_raw), 'noticeClosureFileName': documents.IMAGE_NOTICES,
                           'noticeClosureBytes': len(notice_raw), 'sourceCommit': '1' * 40,
                           'sourceDigest': 'a' * 64, 'sourceRetentionDigest': 'f' * 64},
            'archive': {'fileName': documents.IMAGE_ARCHIVE, 'sha256': self.digest(archive), 'bytes': len(archive), 'members': [
                {'path': config, 'sha256': 'b' * 64, 'bytes': 20},
                {'path': 'layer/layer.tar', 'sha256': 'c' * 64, 'bytes': 30},
                {'path': 'manifest.json', 'sha256': 'd' * 64, 'bytes': 20},
            ]},
            'image': {'id': 'sha256:' + 'b' * 64, 'configPath': config, 'layerPath': 'layer/layer.tar', 'diffId': 'sha256:' + 'c' * 64},
            'extractorSha256': 'e' * 64,
        }
        raw = json.dumps(manifest, separators=(',', ':')).encode()
        (directory / documents.IMAGE_MANIFEST).write_bytes(raw)
        return directory, archive, manifest, self.digest(raw)

    def test_prebuilt_image_pin_is_separate_from_windows_proof_and_binds_actual_transport_bytes(self):
        directory, archive, _, pin = self.image_fixture()
        documents.stage_image(directory, self.package, pin)
        self.assertIsNone(documents.verify_package(self.package))
        selected = documents.verify_image_package(self.package)
        self.assertEqual(selected['manifestSHA256'], pin)
        self.assertEqual(selected['archiveBytes'], len(archive))
        self.assertEqual((self.package / documents.IMAGE_ROOT / documents.IMAGE_ARCHIVE).read_bytes(), archive)
        (self.package / documents.IMAGE_ROOT / documents.IMAGE_ARCHIVE).write_bytes(archive + b'changed')
        with self.assertRaises(ValueError):
            documents.verify_image_package(self.package)

    def test_image_manifest_never_trusts_itself_or_a_sandbox_or_changed_member_shape(self):
        directory, _, manifest, pin = self.image_fixture()
        with self.assertRaises(ValueError):
            documents.verify_image_source(directory, 'a' * 64)
        for change in ('sandbox', 'duplicate', 'unsafe', 'unknown'):
            modified = json.loads(json.dumps(manifest))
            if change == 'sandbox':
                modified['kind'] = 'luci-runtime-docker-save'
            elif change == 'duplicate':
                modified['archive']['members'][1] = modified['archive']['members'][0]
            elif change == 'unsafe':
                modified['image']['layerPath'] = '../layer.tar'
            else:
                modified['authority'] = 'not a declared field'
            raw = json.dumps(modified, separators=(',', ':')).encode()
            (directory / documents.IMAGE_MANIFEST).write_bytes(raw)
            with self.subTest(change=change), self.assertRaises(ValueError):
                documents.verify_image_source(directory, self.digest(raw))

    def test_image_copy_failure_does_not_publish_a_new_manifest_pin(self):
        directory, archive, _, pin = self.image_fixture()
        (directory / documents.IMAGE_ARCHIVE).write_bytes(archive[:-1])
        with self.assertRaises(ValueError):
            documents.stage_image(directory, self.package, pin)
        self.assertFalse((self.package / documents.IMAGE_PIN_FILE).exists())
        self.assertFalse((self.package / documents.IMAGE_ROOT / documents.IMAGE_MANIFEST).exists())

    def test_notice_sidecars_are_required_and_bound_to_the_selected_provenance(self):
        directory, _, _, pin = self.image_fixture()
        (directory / 'notices/LICENSE-fixture.txt').write_bytes(b'changed grant evidence')
        with self.assertRaises(ValueError):
            documents.verify_image_source(directory, pin)
        with self.assertRaises(ValueError):
            documents.stage_image(directory, self.package, pin)
        self.assertFalse((self.package / documents.IMAGE_PIN_FILE).exists())

    def test_collector_retains_original_notices_and_labels_coverage_without_inventing_sbom(self):
        directory, _, manifest, pin = self.image_fixture()
        spec = importlib.util.spec_from_file_location('document_fixture_licences', Path(__file__).with_name('collect-licenses.py'))
        collector = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(collector)
        output = self.root / 'licences'
        output.mkdir()
        records, selected = collector.collect_document_notices(directory, pin, output)
        self.assertEqual(len(records), 1)
        self.assertEqual(selected['manifestSHA256'], pin)
        self.assertEqual(selected['provenance'], manifest['provenance'])
        self.assertIn('not an independent SBOM', records[0]['evidenceScope'])
        self.assertNotIn('sbomClosed', selected)
        retained = output / records[0]['files'][0]['path']
        self.assertEqual(retained.read_bytes(), (directory / 'notices/LICENSE-fixture.txt').read_bytes())
        self.assertEqual((output / selected['manifestFile']).read_bytes(), (directory / documents.IMAGE_MANIFEST).read_bytes())
        self.assertEqual((output / selected['noticeClosureFile']).read_bytes(), (directory / documents.IMAGE_NOTICES).read_bytes())


if __name__ == '__main__':
    unittest.main()

"""Portable package identity binds executable and every served asset."""
import importlib.util
from pathlib import Path
import shutil
import tarfile
import tempfile
import unittest
import zipfile

spec = importlib.util.spec_from_file_location('packaging', Path(__file__).with_name('package.py'))
packaging = importlib.util.module_from_spec(spec)
spec.loader.exec_module(packaging)


class PackageIdentityTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name) / 'package'
        (self.root / 'web/assets').mkdir(parents=True)
        (self.root / 'luci').write_bytes(b'actual executable fixture')
        (self.root / 'web/index.html').write_bytes(b'<script src="assets/app.js"></script>')
        (self.root / 'web/assets/app.js').write_bytes(b'console.log("first");')

    def test_identity_covers_binary_and_all_assets_and_survives_relocation(self):
        identity = packaging.package_identity(self.root, 'linux')
        self.assertRegex(identity, r'^[0-9a-f]{64}$')
        copied = self.root.with_name('same-package-elsewhere')
        shutil.copytree(self.root, copied)
        (copied / 'web/index.html').chmod(0o600)
        self.assertEqual(packaging.package_identity(copied, 'linux'), identity)
        for name in ('luci', 'web/index.html', 'web/assets/app.js'):
            with self.subTest(name=name):
                original = (copied / name).read_bytes()
                (copied / name).write_bytes(original + b'changed')
                self.assertNotEqual(packaging.package_identity(copied, 'linux'), identity)
                (copied / name).write_bytes(original)
        (copied / 'web/assets/app.js').rename(copied / 'web/assets/renamed.js')
        self.assertNotEqual(packaging.package_identity(copied, 'linux'), identity)

    def test_identity_is_in_both_archive_formats_and_repeated_build_is_stable(self):
        for platform in ('linux', 'windows'):
            if platform == 'windows':
                (self.root / 'luci').rename(self.root / 'luci.exe')
            packaging.build_package(self.root, platform)
            expected = (packaging.package_identity(self.root, platform) + '\n').encode()
            self.assertEqual((self.root / 'package-identity').read_bytes(), expected)
            if platform == 'windows':
                with zipfile.ZipFile(str(self.root) + '.zip') as archive:
                    self.assertEqual(archive.read('package-identity'), expected)
            else:
                with tarfile.open(str(self.root) + '.tar.gz') as archive:
                    self.assertEqual(archive.extractfile('package-identity').read(), expected)
            packaging.build_package(self.root, platform)
            self.assertEqual((self.root / 'package-identity').read_bytes(), expected)

    def test_incomplete_or_linked_runtime_inputs_do_not_receive_identity(self):
        for name in ('luci', 'web/index.html', 'web/assets/app.js'):
            with self.subTest(name=name):
                file = self.root / name
                original = file.read_bytes()
                external = self.root.parent / 'external'
                external.write_bytes(original)
                file.unlink()
                file.symlink_to(external)
                with self.assertRaises(ValueError):
                    packaging.build_package(self.root, 'linux')
                self.assertFalse((self.root / 'package-identity').exists())
                file.unlink()
                file.write_bytes(original)
        (self.root / 'web/index.html').unlink()
        with self.assertRaises(ValueError):
            packaging.build_package(self.root, 'linux')


if __name__ == '__main__':
    unittest.main()

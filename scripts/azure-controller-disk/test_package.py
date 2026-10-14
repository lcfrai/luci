import importlib.util
from pathlib import Path
import tempfile
import unittest
from unittest.mock import patch


spec = importlib.util.spec_from_file_location("luci_azure_controller_package", Path(__file__).with_name("package.py"))
package = importlib.util.module_from_spec(spec)
spec.loader.exec_module(package)


class ControllerPackageTest(unittest.TestCase):
    def test_bound_archive_image_install_and_tamper_refusal(self):
        with tempfile.TemporaryDirectory() as directory:
            root = Path(directory)
            source = root / "source"
            scripts = source / "scripts/azure-controller-disk"
            scripts.mkdir(parents=True)
            host = source / "scripts/azure-embed-host"
            host.mkdir(parents=True)
            (source / "LICENSE").write_text("license")
            for name in package.SOURCES:
                (scripts / name).write_text(name)
            (host / "deliver.py").write_text("deliver")
            api = root / "luci-api"
            api.write_bytes(b"binary")
            revision = "a" * 40
            with patch.object(package, "exact_source") as exact, patch.object(package, "exact_go_binary") as binary:
                files = package.collect(source, revision, api)
                exact.assert_called_once_with(source, revision)
                binary.assert_called_once_with(api, revision)
            archive = root / "controller.tar"
            digest = package.archive(files, archive)
            verified = package.read_archive(archive, digest)
            self.assertEqual(verified, files)
            image = root / "image"
            package.install(verified, image, "/mnt/luci-controller")
            self.assertEqual((image / "opt/luci/azure-controller/bin/luci-api").read_bytes(), b"binary")
            unit = (image / "etc/systemd/system/luci-api.service").read_text()
            self.assertIn("RuntimeDirectoryMode=0700", unit)
            self.assertIn("mount.py --resume /etc/luci/azure-controller-disk-initial.json", unit)
            self.assertIn("RequiresMountsFor=/mnt/luci-controller", unit)
            self.assertIn("EnvironmentFile=/etc/luci/azure-controller.env", unit)
            with self.assertRaisesRegex(ValueError, "already contains"):
                package.install(verified, image, "/mnt/luci-controller")
            with self.assertRaisesRegex(ValueError, "mount path"):
                package.install(verified, root / "unsafe", "/home/owner")
            archive.write_bytes(archive.read_bytes() + b"x")
            with self.assertRaisesRegex(ValueError, "hash changed"):
                package.read_archive(archive, digest)


if __name__ == "__main__":
    unittest.main()

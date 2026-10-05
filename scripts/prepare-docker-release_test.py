"""Outcome checks for the Docker distribution's private build-context boundary."""
import importlib.util
import io
import json
from pathlib import Path
import shutil
import tarfile
import tempfile
import unittest
import zipfile

SOURCE_ROOT = Path(__file__).resolve().parent.parent
spec = importlib.util.spec_from_file_location("docker_release", SOURCE_ROOT / "scripts/prepare-docker-release.py")
packaging = importlib.util.module_from_spec(spec)
spec.loader.exec_module(packaging)


class DockerReleaseBoundary(unittest.TestCase):
    def setUp(self):
        self.temporary = tempfile.TemporaryDirectory()
        self.addCleanup(self.temporary.cleanup)
        self.repo = Path(self.temporary.name)
        self.release = self.repo / "artifacts/releases/0.4.4"
        self.bundle = self.release / "luci-linux-amd64"
        self.bundle.mkdir(parents=True)
        for name in packaging.REQUIRED:
            file = self.bundle / name
            file.parent.mkdir(parents=True, exist_ok=True)
            file.write_bytes(b"release fixture\n")
        executable = bytearray(20)
        executable[:6] = b"\x7fELF\x02\x01"
        executable[18:20] = b"\x3e\x00"
        (self.bundle / "luci").write_bytes(executable)
        (self.bundle / "licenses/manifest.json").write_text('{"missingLicenses": []}', encoding="utf-8")
        (self.bundle / "licenses/MISSING-LICENSES.txt").write_bytes(b"")
        shutil.copytree(SOURCE_ROOT / "distribution/docker", self.repo / "distribution/docker")
        self.archive()

    def archive(self, extra=None):
        archive_path = self.release / "luci-linux-amd64.tar.gz"
        with tarfile.open(archive_path, "w:gz") as archive:
            for source in sorted(self.bundle.iterdir()):
                archive.add(source, arcname=source.name)
            if extra is not None:
                archive.addfile(extra, io.BytesIO(b""))
        self.manifest = {"version": "0.4.4", "sourceDirty": False, "sourceCommit": "a" * 40,
            "artifacts": [{"name": archive_path.name, "bytes": archive_path.stat().st_size, "sha256": packaging.digest(archive_path)}]}
        (self.release / "release-manifest.json").write_text(json.dumps(self.manifest), encoding="utf-8")

    def test_prepares_only_attested_bundle_and_distribution_files(self):
        manifest, files = packaging.verify_release(self.release, "0.4.4")
        output = self.repo / "artifacts/docker/0.4.4"
        result = packaging.prepare(self.repo, self.release, "0.4.4", output, manifest, files)
        self.assertEqual(result["sourceCommit"], "a" * 40)
        self.assertEqual((output / "bundle/luci").read_bytes(), (self.bundle / "luci").read_bytes())
        self.assertEqual({record["path"] for record in result["files"]}, set(packaging.PACKAGING_FILES) | {"bundle/" + name for name in files})
        for record in result["files"]:
            self.assertEqual(packaging.digest(output / record["path"]), record["sha256"])
        self.assertNotIn(b"\r", (output / "entrypoint.sh").read_bytes())
        for name in ("engine-entrypoint.sh", "workspace-init.sh"):
            self.assertEqual((output / name).read_bytes(), (SOURCE_ROOT / "distribution/docker" / name).read_bytes().replace(b"\r\n", b"\n"))
            self.assertNotIn(b"\r", (output / name).read_bytes())
        self.assertEqual((output / "compose.yaml").read_bytes().count(b"${LUCI_VERSION:-0.4.4}"), 2)
        self.assertNotIn(b"${LUCI_VERSION:-0.4.9}", (output / "compose.yaml").read_bytes())
        archive = packaging.archive_distribution(self.repo, output)
        self.assertEqual(archive["sha256"], packaging.digest(output.with_name(output.name + ".zip")))
        with zipfile.ZipFile(output.with_name(output.name + ".zip")) as zipped:
            self.assertEqual(zipped.read("bundle/luci"), (self.bundle / "luci").read_bytes())
            self.assertTrue((zipped.getinfo("start-luci.sh").external_attr >> 16) & 0o111)
            self.assertTrue((zipped.getinfo("workspace-init.sh").external_attr >> 16) & 0o111)

    def test_refuses_private_top_level_and_changed_extracted_files(self):
        private = self.bundle / "state/private-account.json"
        private.parent.mkdir()
        private.write_text('{"password":"must not be copied"}', encoding="utf-8")
        self.archive()
        with self.assertRaisesRegex(ValueError, "Unexpected release bundle"):
            packaging.verify_release(self.release, "0.4.4")
        private.unlink()
        private.parent.rmdir()
        self.archive()
        (self.bundle / "web/index.html").write_text("Changed after packaging", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "differs from its archive"):
            packaging.verify_release(self.release, "0.4.4")

    def test_refuses_archive_links_and_path_escape(self):
        linked = tarfile.TarInfo("web/linked-private-file")
        linked.type = tarfile.SYMTYPE
        linked.linkname = "/private-account.json"
        self.archive(linked)
        with self.assertRaisesRegex(ValueError, "Linked, special"):
            packaging.verify_release(self.release, "0.4.4")
        escaping = tarfile.TarInfo("../private-account.json")
        self.archive(escaping)
        with self.assertRaisesRegex(ValueError, "Private or escaping"):
            packaging.verify_release(self.release, "0.4.4")

    def test_refuses_dirty_release_and_archive_hash_mismatch(self):
        self.manifest["sourceDirty"] = True
        (self.release / "release-manifest.json").write_text(json.dumps(self.manifest), encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "clean-source"):
            packaging.verify_release(self.release, "0.4.4")
        self.archive()
        with (self.release / "luci-linux-amd64.tar.gz").open("ab") as file:
            file.write(b"unattested")
        with self.assertRaisesRegex(ValueError, "does not match"):
            packaging.verify_release(self.release, "0.4.4")

    def test_does_not_overwrite_existing_output_or_allow_normalized_escape(self):
        output = self.repo / "artifacts/docker/existing"
        output.mkdir(parents=True)
        sentinel = output / "preserve.txt"
        sentinel.write_text("owned existing files", encoding="utf-8")
        with self.assertRaisesRegex(ValueError, "already exists"):
            packaging.bounded_output(self.repo, output)
        self.assertEqual(sentinel.read_text(encoding="utf-8"), "owned existing files")
        with self.assertRaisesRegex(ValueError, "beneath artifacts/docker"):
            packaging.bounded_output(self.repo, self.repo / "artifacts/docker/../private-output")


if __name__ == "__main__":
    unittest.main()

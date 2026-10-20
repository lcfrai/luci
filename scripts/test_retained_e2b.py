import shutil
import tempfile
import unittest
import importlib.util
import json
from unittest.mock import patch
from pathlib import Path

from retained_e2b import E2B_ROOT, verify_e2b
from retained_pdf_fonts import PDF_FONTS


class RetainedE2BTests(unittest.TestCase):
    def fixture(self):
        temp = tempfile.TemporaryDirectory()
        self.addCleanup(temp.cleanup)
        root = Path(temp.name)
        shutil.copytree(Path(__file__).resolve().parent.parent / E2B_ROOT, root / E2B_ROOT)
        return root

    def test_exact_generated_bindings_and_license(self):
        self.assertEqual(verify_e2b(self.fixture())["verifiedFiles"], 4)

    def test_actual_collector_includes_original_apache_grant_and_attribution(self):
        root = self.fixture()
        repository = Path(__file__).resolve().parent.parent
        shutil.copytree(repository / PDF_FONTS, root / PDF_FONTS)
        shutil.copytree(repository / "third-party/pdf-fonts", root / "third-party/pdf-fonts")
        (root / "desktop").mkdir()
        (root / "desktop/package-lock.json").write_text('{"packages":{}}')
        goroot = root / "fixture-toolchain"
        goroot.mkdir()
        (goroot / "LICENSE").write_text("Controlled toolchain licence fixture\n")
        spec = importlib.util.spec_from_file_location("collector", Path(__file__).with_name("collect-licenses.py"))
        collector = importlib.util.module_from_spec(spec)
        spec.loader.exec_module(collector)

        def fixture_command(args, cwd, **env):
            if args == ["go", "env", "GOROOT"]:
                return str(goroot)
            if args == ["go", "env", "GOVERSION"]:
                return "fixture-only"
            if args[:3] == ["go", "list", "-deps"]:
                return ""
            self.fail("Unexpected tool invocation: " + repr(args))

        output = root / "out"
        with patch.object(collector, "command", fixture_command):
            _, missing = collector.collect(root, output, False)
        self.assertEqual(missing, [])
        record = next(row for row in json.loads((output / "manifest.json").read_text())["components"] if row["name"] == "e2b-envd-process-bindings")
        self.assertEqual(record["declaredLicense"], "Apache-2.0")
        licence = next(row for row in record["files"] if row["path"].endswith("/LICENSE"))
        self.assertEqual((output / licence["path"]).read_bytes(), (root / E2B_ROOT / "LICENSE").read_bytes())
        self.assertTrue(any(row["path"].endswith("/UPSTREAM.md") for row in record["files"]))

    def test_changed_original_or_license_is_refused(self):
        for name in ("process.pb.go", "process.proto", "LICENSE"):
            with self.subTest(name=name):
                root = self.fixture()
                path = root / E2B_ROOT / name
                path.write_bytes(path.read_bytes() + b"changed\n")
                with self.assertRaisesRegex(ValueError, "pinned original"):
                    verify_e2b(root)

    def test_only_exact_import_relocation_is_permitted(self):
        root = self.fixture()
        path = root / E2B_ROOT / "processconnect/process.connect.go"
        path.write_bytes(path.read_bytes().replace(b"connect.IsAtLeastVersion1_13_0", b"connect.IsAtLeastVersion1_14_0"))
        with self.assertRaisesRegex(ValueError, "pinned original"):
            verify_e2b(root)

    def test_missing_license_is_refused(self):
        root = self.fixture()
        (root / E2B_ROOT / "LICENSE").unlink()
        with self.assertRaisesRegex(ValueError, "Missing"):
            verify_e2b(root)

    def test_windows_line_endings_preserve_exact_source(self):
        root = self.fixture()
        for path in (root / E2B_ROOT).rglob("*"):
            if path.is_file():
                path.write_bytes(path.read_bytes().replace(b"\n", b"\r\n"))
        self.assertEqual(verify_e2b(root)["verifiedFiles"], 4)

    def test_linked_generated_directory_is_refused(self):
        root = self.fixture()
        parent = root / E2B_ROOT
        actual = root / "outside"
        (parent / "processconnect").rename(actual)
        try:
            (parent / "processconnect").symlink_to(actual, target_is_directory=True)
        except OSError as error:
            self.skipTest("Host cannot create the controlled symlink: " + str(error))
        with self.assertRaisesRegex(ValueError, "Linked"):
            verify_e2b(root)


if __name__ == "__main__":
    unittest.main()

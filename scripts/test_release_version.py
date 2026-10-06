import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("release_version", Path(__file__).with_name("release-version.py"))
version = importlib.util.module_from_spec(spec)
spec.loader.exec_module(version)


class ReleaseVersionTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        source = Path(__file__).resolve().parents[1]
        for name in (*version.JSON_SLOTS, *version.TEXT_SLOTS):
            target = self.root / name
            target.parent.mkdir(parents=True, exist_ok=True)
            target.write_bytes((source / name).read_bytes())

    def test_candidate_update_preserves_dependency_version_and_locked_resolution(self):
        before = (self.root / "desktop/src-tauri/Cargo.lock").read_text()
        original_version = json.loads((self.root / "desktop/package.json").read_text())["version"]
        version.set_version(self.root, "0.5.1-rc.1")
        self.assertEqual(version.check(self.root, "0.5.1-rc.1"), "0.5.1-rc.1")
        after = (self.root / "desktop/src-tauri/Cargo.lock").read_text()
        self.assertEqual(after.replace('name = "luci"\nversion = "0.5.1-rc.1"', f'name = "luci"\nversion = "{original_version}"'), before)

    def test_mismatch_and_unsafe_input_fail_without_rewriting_source(self):
        file = self.root / "desktop/package.json"
        before = file.read_bytes()
        with self.assertRaisesRegex(ValueError, "mismatch"):
            version.check(self.root, "9.9.9")
        with self.assertRaises(ValueError):
            version.set_version(self.root, "0.5.0/../../bad")
        self.assertEqual(file.read_bytes(), before)
        manifest = self.root / "desktop/src-tauri/Cargo.lock"
        manifest.write_text(manifest.read_text().replace('name = "luci"', 'name = "renamed"'))
        with self.assertRaises(ValueError):
            version.set_version(self.root, "0.5.1")
        self.assertEqual(file.read_bytes(), before)


if __name__ == "__main__":
    unittest.main()

import hashlib
import importlib.util
import json
from pathlib import Path
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("release_integrity", Path(__file__).with_name("release-integrity.py"))
integrity = importlib.util.module_from_spec(spec)
spec.loader.exec_module(integrity)
COMMIT = "a" * 40


class CandidateIntegrityTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.run = {"id": 123, "repository": {"full_name": "lcfrai/luci"}, "head_repository": {"full_name": "lcfrai/luci"}, "status": "completed", "conclusion": "success", "head_sha": COMMIT, "event": "push", "path": ".github/workflows/luci-ci.yml"}

    def group(self, group="linux-amd64"):
        directory = self.root / group / "releases" / "0.5.0"
        directory.mkdir(parents=True)
        records = []
        for name in integrity.selected_names(group, "0.5.0"):
            data = ("actual fixture bytes for " + name).encode()
            (directory / name).write_bytes(data)
            records.append({"name": name, "bytes": len(data), "sha256": hashlib.sha256(data).hexdigest()})
        manifest = {"version": "0.5.0", "sourceCommit": COMMIT, "sourceDirty": False, "portableLicenseComponents": [{"count": 1, "missing": 0}], "nativeRequested": group == "windows-native", "nativeLicenseComponents": 1 if group == "windows-native" else 0, "artifacts": records}
        (directory / "release-manifest.json").write_text(json.dumps(manifest))
        (directory / "SHA256SUMS").write_text("".join(row["sha256"] + "  " + row["name"] + "\n" for row in records))
        return directory, manifest

    def test_real_bytes_all_platforms_and_nested_ci_download_layout(self):
        for group in integrity.GROUPS:
            self.group(group)
            rows = integrity.verify_group(self.root / group, group, "0.5.0", COMMIT)
            self.assertEqual(len(rows), len(integrity.selected_names(group, "0.5.0")) + 1)
            self.assertTrue(all(row["name"].startswith(group + "/") for row in rows))
        self.assertEqual(integrity.verify_run(self.run, "lcfrai/luci", COMMIT, COMMIT)["id"], 123)

    def test_trusted_build_gates_reject_fork_failure_wrong_source_and_workflow(self):
        for change in ({"conclusion": "failure"}, {"head_sha": "b" * 40}, {"event": "pull_request"}, {"path": "other.yml"}, {"head_repository": {"full_name": "other/luci"}}):
            with self.subTest(change=change), self.assertRaises(ValueError):
                integrity.verify_run(self.run | change, "lcfrai/luci", COMMIT, COMMIT)
        with self.assertRaises(ValueError):
            integrity.verify_run(self.run, "lcfrai/luci", COMMIT, "b" * 40)

    def test_tampered_bytes_dirty_source_duplicate_records_and_links_rejected(self):
        directory, manifest = self.group()
        artifact = directory / "luci-linux-amd64.tar.gz"
        original = artifact.read_bytes()
        artifact.write_bytes(original + b"tampered")
        with self.assertRaises(ValueError):
            integrity.verify_group(self.root, "linux-amd64", "0.5.0", COMMIT)
        artifact.write_bytes(original)
        for change in ({"sourceDirty": True}, {"artifacts": manifest["artifacts"] * 2}, {"portableLicenseComponents": [{"count": 1, "missing": 1}]}):
            (directory / "release-manifest.json").write_text(json.dumps(manifest | change))
            with self.subTest(change=change), self.assertRaises(ValueError):
                integrity.verify_group(self.root, "linux-amd64", "0.5.0", COMMIT)
        (directory / "release-manifest.json").write_text(json.dumps(manifest))
        elsewhere = self.root / "elsewhere"
        elsewhere.write_bytes(original)
        artifact.unlink()
        artifact.symlink_to(elsewhere)
        with self.assertRaises(ValueError):
            integrity.verify_group(self.root, "linux-amd64", "0.5.0", COMMIT)


if __name__ == "__main__":
    unittest.main()

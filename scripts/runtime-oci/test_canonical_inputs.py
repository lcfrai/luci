import hashlib
import json
from pathlib import Path
import tempfile
import unittest
import subprocess

from canonical_inputs import ALGORITHM, MODE, inventory_digest, stage


FIXTURE = Path(__file__).resolve().parents[2] / "docs/luci/fixtures/runtime-oci-v1/canonical-cross-os.json"


class CanonicalInputsTest(unittest.TestCase):
    def test_stage_reads_clean_git_blobs_not_platform_checkout(self):
        with tempfile.TemporaryDirectory() as tmp:
            repo = Path(tmp) / "repo"
            repo.mkdir()
            subprocess.run(["git", "init", "-q", str(repo)], check=True)
            inputs = {
                "images/sandbox/Dockerfile": b"FROM node:24-bookworm-slim\n",
                "images/sandbox/entrypoint.sh": b"#!/bin/sh\n",
                "sandboxd/go.mod": b"module example.test/sandboxd\n",
                "sandboxd/go.sum": b"",
                "sandboxd/cmd/sandboxd/main.go": b"package main\n",
                "sandboxd/vendor/modules.txt": b"# vendored\n",
            }
            for name, content in inputs.items():
                output = repo / name
                output.parent.mkdir(parents=True, exist_ok=True)
                output.write_bytes(content)
            subprocess.run(["git", "-C", str(repo), "add", "."], check=True)
            subprocess.run(["git", "-C", str(repo), "-c", "user.name=Fixture", "-c", "user.email=fixture@example.invalid", "commit", "-qm", "fixture"], check=True)
            revision = subprocess.check_output(["git", "-C", str(repo), "rev-parse", "HEAD"], text=True).strip()
            first = stage(repo, revision, Path(tmp) / "linux")
            for name, content in inputs.items():
                source = repo / name
                source.write_bytes(content.replace(b"\n", b"\r\n"))
                source.chmod(0o666)
            second = stage(repo, revision, Path(tmp) / "windows")
            self.assertEqual(first, second)
            self.assertEqual((Path(tmp) / "windows/images/sandbox/Dockerfile").read_bytes(), inputs["images/sandbox/Dockerfile"])
            self.assertIn({"path": "sandboxd/go.sum", "mode": MODE, "bytes": 0, "sha256": hashlib.sha256(b"").hexdigest()}, second["sourceFiles"])

    def test_cross_os_blobs_ignore_worktree_line_endings_and_modes(self):
        fixture = json.loads(FIXTURE.read_text())
        self.assertEqual(fixture["algorithm"], ALGORITHM)
        rows = []
        with tempfile.TemporaryDirectory() as tmp:
            for item in fixture["files"]:
                raw = item["utf8"].encode("utf-8")
                rows.append({"path": item["path"], "mode": MODE, "bytes": len(raw), "sha256": hashlib.sha256(raw).hexdigest()})
                checkout = Path(tmp) / item["path"]
                checkout.parent.mkdir(parents=True, exist_ok=True)
                checkout.write_bytes(raw.replace(b"\n", b"\r\n"))
                checkout.chmod(0o666)
            self.assertEqual(inventory_digest(rows), fixture["expectedCanonicalInputDigest"])
            self.assertNotEqual(hashlib.sha256((Path(tmp) / "images/sandbox/Dockerfile").read_bytes()).hexdigest(), rows[0]["sha256"])


if __name__ == "__main__":
    unittest.main()

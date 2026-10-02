"""Source-export boundary tests against disposable, synthetic Git repositories."""
import importlib.util
import json
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import zipfile


sys.dont_write_bytecode = True
spec = importlib.util.spec_from_file_location("exporter", Path(__file__).with_name("export-public-source.py"))
exporter = importlib.util.module_from_spec(spec)
spec.loader.exec_module(exporter)


class ExportTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="luci-source-export-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name)
        self.repo = self.root / "source"
        self.repo.mkdir()
        self.run_git("init", "--quiet")
        self.run_git("config", "user.name", "Source export fixture")
        self.run_git("config", "user.email", "fixture@example.invalid")
        self.run_git("config", "core.autocrlf", "false")
        for name in exporter.REQUIRED:
            self.write(name, b"fixture\n")
        self.write("LICENSE", b"MIT License\nPermission is hereby granted\n")
        self.write("desktop/package.json", exporter.json_bytes({"version": "0.4.0", "license": "MIT"}))
        self.write("desktop/package-lock.json", exporter.json_bytes({"version": "0.4.0", "packages": {"": {"version": "0.4.0"}}}))
        self.write("desktop/src-tauri/Cargo.toml", b'[package]\nname = "luci"\nversion = "0.4.0"\nlicense = "MIT"\n')
        self.write("desktop/src-tauri/Cargo.lock", b'[[package]]\nname = "luci"\nversion = "0.4.0"\n')
        self.write("desktop/src-tauri/tauri.conf.json", b'{"version":"0.4.0"}')
        self.write("control-plane/internal/product/server.go", b'const Version = "0.4.0"\n')
        self.write(".github/workflows/luci-ci.yml", b"env:\n  LUCI_VERSION: '0.4.0'\n")
        self.grant = b"Original upstream grant\r\nExact CRLF bytes\r\n"
        self.write("third-party/licenses/fixture/LICENSE", self.grant)
        self.write("third-party/licenses/manifest.json", exporter.json_bytes([
            {"files": [{"path": "fixture/LICENSE", "sha256": exporter.sha(self.grant)}]},
        ]))
        self.write("docs/legacy/private.md", b"excluded archive\n")
        self.write("desktop/.env.local", b"excluded secret state\n")
        self.write("desktop/src/seams.test.ts", b"historical audit\n")
        self.write("desktop/src/active.test.ts", b"retained test\n")
        self.run_git("add", "--all")
        self.run_git("update-index", "--chmod=+x", "scripts/build-release.sh")
        self.commit()

    def run_git(self, *args):
        return subprocess.run(["git", "-C", str(self.repo), *args], check=True,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout

    def write(self, name, data):
        path = self.repo / name
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)

    def commit(self):
        self.run_git("add", "--all")
        self.run_git("commit", "--quiet", "-m", "Synthetic export fixture")
        self.revision = self.run_git("rev-parse", "HEAD").decode().strip()

    def export(self, name="candidate", plan=False):
        return exporter.export(self.repo, self.revision, "0.4.0", self.root / name, plan)

    def test_exact_bytes_modes_selection_and_checksums(self):
        result = self.export()
        output = Path(result["output"])
        self.assertEqual((output / "third-party/licenses/fixture/LICENSE").read_bytes(), self.grant)
        for name in (".git", "docs/legacy", "desktop/.env.local", "desktop/src/seams.test.ts"):
            self.assertFalse((output / name).exists(), name)
        self.assertTrue((output / "desktop/src/active.test.ts").is_file())
        checks = json.loads((output / "EXPORT-CHECKS.json").read_bytes())
        self.assertFalse(checks["fullCleanRebuildRerun"])
        self.assertFalse(checks["publicPublicationAuthorizedOrPerformed"])
        with zipfile.ZipFile(result["archive"]) as bundle:
            self.assertEqual(bundle.getinfo("candidate/scripts/build-release.sh").external_attr >> 16 & 0o777, 0o755)
            self.assertEqual(bundle.read("candidate/third-party/licenses/fixture/LICENSE"), self.grant)
        self.assertEqual(exporter.sha(Path(result["archive"]).read_bytes()), result["archiveSha256"])
        for row in (output / "EXPORT-SHA256SUMS").read_text().splitlines():
            digest, name = row.split("  ", 1)
            self.assertEqual(exporter.sha((output / name).read_bytes()), digest)
        with self.assertRaisesRegex(exporter.ExportError, "already exists"):
            self.export()

    def test_plan_writes_nothing(self):
        self.assertTrue(self.export(plan=True)["planOnly"])
        self.assertFalse((self.root / "candidate").exists())

    def test_dirty_and_wrong_revision_refused(self):
        self.write("new-untracked.txt", b"uncommitted\n")
        with self.assertRaisesRegex(exporter.ExportError, "dirty"):
            self.export()
        old_revision = self.revision
        self.commit()
        self.revision = old_revision
        with self.assertRaisesRegex(exporter.ExportError, "HEAD"):
            self.export()

    def test_changed_grant_refused(self):
        self.write("third-party/licenses/fixture/LICENSE", self.grant.replace(b"\r\n", b"\n"))
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "changed retained third-party grant"):
            self.export()

    def test_secrets_and_version_mismatch_refused(self):
        self.write("desktop/src/key.ts", b"ghp_" + b"A" * 36)
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "Credential signature"):
            self.export()
        self.write("desktop/src/key.ts", b"removed\n")
        self.write("desktop/package.json", b'{"version":"0.5.0","license":"MIT"}')
        self.commit()
        with self.assertRaisesRegex(exporter.ExportError, "version mismatch"):
            self.export()

    def test_unsafe_paths_and_symlinks_refused(self):
        self.assertFalse(exporter.safe_path("desktop/../private"))
        self.assertFalse(exporter.safe_path("desktop/con.txt"))
        # update-index creates a tracked symlink on Windows without requiring OS symlink privileges.
        blob = subprocess.run(["git", "-C", str(self.repo), "hash-object", "-w", "--stdin"],
                              input=b"../outside", check=True, stdout=subprocess.PIPE).stdout.decode().strip()
        self.run_git("update-index", "--add", "--cacheinfo", "120000," + blob + ",desktop/escape")
        self.run_git("commit", "--quiet", "-m", "Symlink fixture")
        revision = self.run_git("rev-parse", "HEAD").decode().strip()
        with self.assertRaisesRegex(exporter.ExportError, "symlink"):
            exporter.read_source(self.repo, revision)


if __name__ == "__main__":
    unittest.main()

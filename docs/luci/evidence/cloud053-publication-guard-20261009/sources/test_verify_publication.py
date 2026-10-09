"""Real Git publication regressions; mutations are confined to owned temp repos."""
import hashlib
import json
import os
from pathlib import Path
import subprocess
import sys
import tempfile
import unittest
import uuid


SCRIPT = Path(__file__).with_name("verify-publication.py")
CAPSULE = "docs/evidence/capsule"


def isolated_environment():
    env = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
    env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull,
               GIT_TERMINAL_PROMPT="0", GIT_NO_REPLACE_OBJECTS="1")
    return env


class PublicationTests(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory(prefix="luci-publication-test-")
        self.addCleanup(self.temp.cleanup)
        self.root = Path(self.temp.name).resolve()
        self.token = uuid.uuid4().hex
        self.marker = self.root / ".owned-publication-test"
        self.marker.write_text(self.token)
        self.repo = self.root / "repo"
        self.repo.mkdir()
        self.git("init", "--quiet")
        self.git("config", "user.name", "Publication fixture")
        self.git("config", "user.email", "publication@example.invalid")
        self.git("config", "core.autocrlf", "false")
        self.git("config", "core.hooksPath", str(self.root / "no-hooks"))
        self.write(".gitignore", b"*.log\n")
        self.write(CAPSULE + "/README.md", b"Retained fixture evidence\n")
        self.write(CAPSULE + "/stdout.log", b"Original output\n")
        self.write(CAPSULE + "/empty.log", b"")
        self.manifest(["README.md", "stdout.log", "empty.log"])
        self.git("add", "--", ".gitignore", CAPSULE)

    def assert_owned(self):
        if (self.root.is_symlink() or not self.root.name.startswith("luci-publication-test-")
                or self.repo != self.root / "repo" or self.repo.is_symlink()
                or self.repo.resolve().parent != self.root
                or not self.marker.is_file() or self.marker.is_symlink()
                or self.marker.read_text() != self.token):
            raise AssertionError("Refusing mutation outside exact owned temporary repository")

    def git(self, *args):
        self.assert_owned()
        return subprocess.run(["git", "-C", str(self.repo), *args],
                              env=isolated_environment(), check=True,
                              stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout

    def write(self, relative, data):
        self.assert_owned()
        path = self.repo / relative
        if path.resolve().is_relative_to(self.repo.resolve()) is False:
            raise AssertionError("Fixture write escapes owned repository")
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(data)

    def manifest(self, paths):
        content = "".join(hashlib.sha256((self.repo / CAPSULE / name).read_bytes()).hexdigest()
                          + "  " + name + "\n" for name in paths)
        self.write(CAPSULE + "/SHA256SUMS", content.encode())

    def add_logs(self):
        self.git("add", "--force", "--", CAPSULE + "/stdout.log", CAPSULE + "/empty.log")

    def commit(self):
        self.git("commit", "--quiet", "-m", "Owned evidence fixture")
        return self.git("rev-parse", "HEAD").decode().strip()

    def verify(self, *options, roots=None, env=None):
        result = subprocess.run([sys.executable, str(SCRIPT), "--repo", str(self.repo),
                                 *options, *(roots or [CAPSULE])],
                                env=env or isolated_environment(), check=False,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30)
        stream = result.stdout if result.returncode == 0 else result.stderr
        return result.returncode, json.loads(stream)

    def test_ignored_logs_missing_index_and_old_commit_then_forced_add_pass(self):
        index_before = (self.repo / ".git/index").read_bytes()
        failed, refusal = self.verify("--index", "--require-complete")
        self.assertEqual(failed, 1)
        self.assertIn("Missing Git blob", refusal["error"])
        self.assertEqual((self.repo / ".git/index").read_bytes(), index_before)
        old = self.commit()
        self.add_logs()
        passed, verified = self.verify("--index", "--require-complete")
        self.assertEqual(passed, 0, verified)
        self.assertEqual(verified["capsules"][0]["verifiedBlobs"], 3)
        new = self.commit()
        # Correct working-tree/index bytes cannot repair the immutable old commit.
        self.assertEqual(self.verify("--ref", old)[0], 1)
        passed, verified = self.verify("--ref", new, "--require-complete")
        self.assertEqual(passed, 0, verified)
        self.assertEqual(verified["commit"], new)

    def test_only_zero_byte_log_omitted_is_still_failure(self):
        self.git("add", "--force", "--", CAPSULE + "/stdout.log")
        code, result = self.verify("--index")
        self.assertEqual(code, 1)
        self.assertIn("empty.log", result["error"])
        self.add_logs()
        self.assertEqual(self.verify("--index")[0], 0)

    def test_wrong_staged_bytes_fail_even_when_working_file_is_correct(self):
        self.add_logs()
        self.write(CAPSULE + "/stdout.log", b"Wrong staged output\n")
        self.git("add", "--force", "--", CAPSULE + "/stdout.log")
        self.write(CAPSULE + "/stdout.log", b"Original output\n")
        code, result = self.verify("--index")
        self.assertEqual(code, 1)
        self.assertIn("SHA256 mismatch", result["error"])
        self.add_logs()
        self.assertEqual(self.verify("--index")[0], 0)
        # Conversely a dirty working file cannot change a valid staged snapshot.
        self.write(CAPSULE + "/stdout.log", b"Unstaged different output\n")
        self.assertEqual(self.verify("--index")[0], 0)

    def test_manifest_comes_from_index_not_working_tree(self):
        self.add_logs()
        self.write(CAPSULE + "/SHA256SUMS", b"Invalid unstaged manifest\n")
        self.assertEqual(self.verify("--index")[0], 0)
        self.git("add", "--", CAPSULE + "/SHA256SUMS")
        self.assertEqual(self.verify("--index")[0], 1)

    def test_unsafe_checksum_paths_fail(self):
        digest = hashlib.sha256(b"").hexdigest()
        for path in ["../outside", "/absolute", "a/../../outside", "C:/drive",
                     "a\\outside", "a//empty", "./relative", "a/.git/config", "tab\tname"]:
            with self.subTest(path=path):
                self.write(CAPSULE + "/SHA256SUMS", (digest + "  " + path + "\n").encode())
                self.git("add", "--", CAPSULE + "/SHA256SUMS")
                code, result = self.verify("--index")
                self.assertEqual(code, 1)
                self.assertIn("Unsafe", result["error"])

    def test_missing_duplicate_empty_and_self_manifest_refused(self):
        digest = hashlib.sha256(b"").hexdigest()
        for data, expected in [(b"", "Empty"), (b"malformed\n", "Invalid"),
                               ((digest + "  empty.log\n") * 2, "Duplicate"),
                               (digest + "  SHA256SUMS\n", "cannot list itself")]:
            with self.subTest(expected=expected):
                self.write(CAPSULE + "/SHA256SUMS", data.encode() if isinstance(data, str) else data)
                self.git("add", "--", CAPSULE + "/SHA256SUMS")
                self.assertIn(expected, self.verify("--index")[1]["error"])
        self.git("rm", "--cached", "--", CAPSULE + "/SHA256SUMS")
        self.assertIn("Missing Git blob", self.verify("--index")[1]["error"])

    def test_symlink_mode_and_unlisted_entries_refused(self):
        self.add_logs()
        blob = self.git("rev-parse", ":" + CAPSULE + "/empty.log").decode().strip()
        self.git("update-index", "--cacheinfo", "120000," + blob + "," + CAPSULE + "/empty.log")
        self.assertIn("Non-regular", self.verify("--index")[1]["error"])
        self.git("update-index", "--cacheinfo", "100644," + blob + "," + CAPSULE + "/empty.log")
        self.write(CAPSULE + "/unlisted.txt", b"Additional retained receipt\n")
        self.git("add", "--", CAPSULE + "/unlisted.txt")
        self.assertEqual(self.verify("--index")[0], 0)
        self.assertIn("Unlisted", self.verify("--index", "--require-complete")[1]["error"])

    def test_selected_root_and_space_names_are_literal(self):
        self.add_logs()
        self.write(CAPSULE + "/name with spaces.txt", b"literal filename\n")
        self.manifest(["README.md", "stdout.log", "empty.log", "name with spaces.txt"])
        self.git("add", "--", CAPSULE)
        self.write("other/SHA256SUMS", b"Deliberately invalid unrelated capsule\n")
        self.git("add", "--", "other")
        self.assertEqual(self.verify("--index", "--require-complete")[0], 0)
        self.assertEqual(self.verify("--index", roots=[CAPSULE, "other"])[0], 1)

    def test_inherited_git_redirection_cannot_select_other_index(self):
        self.add_logs()
        env = isolated_environment()
        env.update(GIT_DIR=str(self.root / "foreign.git"), GIT_WORK_TREE=str(self.root),
                   GIT_COMMON_DIR=str(self.root / "foreign-common"),
                   GIT_INDEX_FILE=str(self.root / "foreign-index"),
                   GIT_OBJECT_DIRECTORY=str(self.root / "foreign-objects"),
                   GIT_ALTERNATE_OBJECT_DIRECTORIES=str(self.root / "foreign-alternates"),
                   GIT_CONFIG_COUNT="1", GIT_CONFIG_KEY_0="core.bare", GIT_CONFIG_VALUE_0="true")
        self.assertEqual(self.verify("--index", env=env)[0], 0)
        self.assertFalse((self.root / "foreign-index").exists())

    def test_partial_clone_missing_objects_do_not_fetch_or_change_object_store(self):
        self.add_logs()
        revision = self.commit()
        self.git("config", "uploadpack.allowFilter", "true")
        partial = self.repo / "partial"
        # file:// stays entirely inside this marked fixture; no network remote.
        self.git("clone", "--quiet", "--no-local", "--filter=blob:none", "--no-checkout",
                 "--", self.repo.as_uri(), str(partial))
        manifest_oid = self.git("rev-parse", revision + ":" + CAPSULE + "/SHA256SUMS").decode().strip()
        missing = subprocess.run(["git", "--no-lazy-fetch", "-C", str(partial),
                                  "cat-file", "-e", manifest_oid], env=isolated_environment(),
                                 stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
        self.assertNotEqual(missing.returncode, 0, "Fixture must actually omit promisor blobs")

        def objects():
            return {p.relative_to(partial / ".git/objects").as_posix(): hashlib.sha256(p.read_bytes()).hexdigest()
                    for p in (partial / ".git/objects").rglob("*") if p.is_file()}

        before = objects()
        result = subprocess.run([sys.executable, str(SCRIPT), "--repo", str(partial),
                                 "--ref", revision, CAPSULE], env=isolated_environment(),
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, timeout=30)
        self.assertEqual(result.returncode, 1, "Missing promisor objects must fail without lazy fetch")
        self.assertIn("Git cat-file failed", json.loads(result.stderr)["error"])
        self.assertEqual(objects(), before, "Guard must not populate the local object store")

    def test_fixture_guard_refuses_non_owned_repository(self):
        owned = self.repo
        self.repo = self.root
        with self.assertRaisesRegex(AssertionError, "owned temporary"):
            self.git("init")
        self.repo = owned
        self.marker.write_text("changed ownership")
        with self.assertRaisesRegex(AssertionError, "owned temporary"):
            self.git("add", "--all")


if __name__ == "__main__":
    unittest.main(verbosity=2)

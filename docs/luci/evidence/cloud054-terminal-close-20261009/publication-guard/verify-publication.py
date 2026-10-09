#!/usr/bin/env python3
"""Verify selected evidence capsules against Git bytes, never working-tree bytes.

Example: python3 scripts/acceptance/verify-publication.py --index \
    --require-complete docs/luci/evidence/my-capsule

Use --ref COMMIT to verify a published revision. SHA256SUMS must also exist in
that index/revision. This is a read-only content check, not an evidence-quality
audit. Complete coverage refers to the selected Git snapshot, not untracked or
ignored working-tree files. No staging, fetching, filters or text conversion.
"""
import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys


REGULAR_MODES = {"100644", "100755"}
MAX_MANIFEST_BYTES = 4 * 1024 * 1024


class VerificationError(Exception):
    pass


def git_environment():
    # --repo and the actual repository index must not be redirected by inherited
    # GIT_DIR/GIT_INDEX_FILE/GIT_WORK_TREE or injected configuration.
    env = {key: value for key, value in os.environ.items() if not key.startswith("GIT_")}
    env.update(GIT_CONFIG_NOSYSTEM="1", GIT_CONFIG_GLOBAL=os.devnull,
               GIT_OPTIONAL_LOCKS="0", GIT_NO_REPLACE_OBJECTS="1",
               GIT_TERMINAL_PROMPT="0", GIT_NO_LAZY_FETCH="1")
    return env


def safe_path(value):
    if (not value or value.startswith("/") or any(ord(c) < 32 or ord(c) == 127 for c in value)
            or "\\" in value or ":" in value
            or any(part in {"", ".", "..", ".git"} for part in value.split("/"))):
        raise VerificationError(f"Unsafe repository-relative path: {value!r}")
    return value


class GitSnapshot:
    def __init__(self, repo, roots, ref):
        self.env = git_environment()
        self.repo = Path(repo).resolve()
        self.command = ["git", "--literal-pathspecs", "-c", "core.fsmonitor=false",
                        "-c", "core.untrackedCache=false", "-C", str(self.repo)]
        actual = Path(self.run("rev-parse", "--show-toplevel").decode().strip()).resolve()
        if actual != self.repo:
            raise VerificationError("--repo must identify the repository root")
        self.ref = None
        self.entries = {}
        if ref is None:
            records = self.run("ls-files", "--stage", "-z", "--", *roots)
        else:
            self.ref = self.run("rev-parse", "--verify", "--end-of-options",
                                ref + "^{commit}").decode().strip()
            records = self.run("ls-tree", "-r", "-z", "--full-tree", self.ref, "--", *roots)
        for record in records.split(b"\0"):
            if not record:
                continue
            header, path = record.split(b"\t", 1)
            path = path.decode("utf-8")
            safe_path(path)
            fields = header.decode("ascii").split()
            if ref is None:
                mode, oid, stage = fields
                if stage != "0":
                    raise VerificationError(f"Unmerged index entry: {path}")
            else:
                mode, kind, oid = fields
                if kind != "blob":
                    raise VerificationError(f"Non-blob Git entry: {path}")
            if path in self.entries:
                raise VerificationError(f"Duplicate Git entry: {path}")
            self.entries[path] = (mode, oid)

    def run(self, *args):
        result = subprocess.run(self.command + list(args), env=self.env,
                                stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=False)
        if result.returncode:
            raise VerificationError(f"Git {args[0]} failed (exit {result.returncode})")
        return result.stdout

    def object_id(self, path):
        if path not in self.entries:
            raise VerificationError(f"Missing Git blob: {path}")
        mode, oid = self.entries[path]
        if mode not in REGULAR_MODES:
            raise VerificationError(f"Non-regular Git mode {mode}: {path}")
        return oid

    def manifest(self, path):
        oid = self.object_id(path)
        size = int(self.run("cat-file", "-s", oid))
        if size > MAX_MANIFEST_BYTES:
            raise VerificationError(f"Checksum manifest exceeds {MAX_MANIFEST_BYTES} bytes: {path}")
        return self.run("cat-file", "blob", oid)

    def digest(self, path):
        oid = self.object_id(path)
        process = subprocess.Popen(self.command + ["cat-file", "blob", oid], env=self.env,
                                   stdout=subprocess.PIPE, stderr=subprocess.PIPE)
        digest = hashlib.sha256()
        try:
            with process.stdout:
                for block in iter(lambda: process.stdout.read(1024 * 1024), b""):
                    digest.update(block)
            with process.stderr:
                process.stderr.read()
            if process.wait():
                raise VerificationError(f"Cannot read Git blob: {path}")
        finally:
            if process.poll() is None:
                process.kill()
                process.wait()
        return digest.hexdigest()


def verify(repo, roots, ref=None, complete=False):
    roots = [safe_path(root) for root in roots]
    if len(set(roots)) != len(roots):
        raise VerificationError("Duplicate capsule root")
    if any(a.startswith(b + "/") for a in roots for b in roots if a != b):
        raise VerificationError("Overlapping capsule roots")
    snapshot = GitSnapshot(repo, roots, ref)
    reports = []
    for root in roots:
        manifest_path = root + "/SHA256SUMS"
        raw = snapshot.manifest(manifest_path)
        expected = {}
        for number, line in enumerate(raw.decode("utf-8").splitlines(), 1):
            match = re.fullmatch(r"([0-9a-fA-F]{64}) [ *](.+)", line)
            if not match:
                raise VerificationError(f"Invalid checksum line {number}: {manifest_path}")
            digest, relative = match.groups()
            safe_path(relative)
            if relative == "SHA256SUMS":
                raise VerificationError(f"Checksum manifest cannot list itself: {manifest_path}")
            if relative in expected:
                raise VerificationError(f"Duplicate checksum path: {root}/{relative}")
            expected[relative] = digest.lower()
        if not expected:
            raise VerificationError(f"Empty checksum manifest: {manifest_path}")
        for relative, digest in expected.items():
            path = root + "/" + relative
            if snapshot.digest(path) != digest:
                raise VerificationError(f"SHA256 mismatch: {path}")
        if complete:
            covered = {root + "/" + name for name in expected} | {manifest_path}
            selected = {path for path in snapshot.entries if path.startswith(root + "/")}
            extras = sorted(selected - covered)
            if extras:
                raise VerificationError(f"Unlisted Git entry: {extras[0]}")
        reports.append({"root": root, "verifiedBlobs": len(expected),
                        "manifestSHA256": hashlib.sha256(raw).hexdigest()})
    selected = [[path, *value] for path, value in sorted(snapshot.entries.items())]
    return {"passed": True, "mode": "ref" if ref is not None else "index",
            "commit": snapshot.ref, "capsules": reports, "completeTrackedCoverage": complete,
            "selectedGitEntriesSHA256": hashlib.sha256(json.dumps(selected).encode()).hexdigest(),
            "scope": "Exact selected Git blobs; no working-tree, provenance, semantic or runtime acceptance claim."}


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo", default=".", help="Repository root (default: current directory)")
    mode = parser.add_mutually_exclusive_group(required=True)
    mode.add_argument("--index", action="store_true", help="Read the current staged Git snapshot")
    mode.add_argument("--ref", help="Resolve this revision once and read its immutable commit blobs")
    parser.add_argument("--require-complete", action="store_true",
                        help="Also reject unlisted entries in selected Git capsule trees")
    parser.add_argument("capsules", nargs="+", help="Repository-relative capsule directories")
    args = parser.parse_args()
    try:
        result = verify(args.repo, args.capsules, args.ref, args.require_complete)
    except (VerificationError, UnicodeError, OSError, ValueError) as error:
        print(json.dumps({"passed": False, "error": str(error)}), file=sys.stderr)
        return 1
    print(json.dumps(result, indent=2))
    return 0


if __name__ == "__main__":
    sys.exit(main())

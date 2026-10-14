#!/usr/bin/env python3
"""Stage the exact clean Git blobs used to produce the packaged Docker runtime.

This is deliberately independent of checkout line endings, executable bits and
filesystem enumeration order. It has no Docker or cloud side effects.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import subprocess
import sys

MAX_FILES = 8192
MAX_BYTES = 64 << 20
MODE = "0644"
ALGORITHM = "luci-git-blobs-v1"
ROOT_FILES = {"sandboxd/go.mod", "sandboxd/go.sum", "images/sandbox/Dockerfile", "images/sandbox/entrypoint.sh"}
TREES = ("sandboxd/cmd/", "sandboxd/internal/", "sandboxd/vendor/")
SAFE = re.compile(r"^[A-Za-z0-9._/-]{1,256}$")


def selected(path: str) -> bool:
    return path in ROOT_FILES or path.startswith(TREES)


def inventory_digest(rows: list[dict]) -> str:
    """SHA256 of sorted path NUL mode NUL size NUL content-sha256 LF rows."""
    digest = hashlib.sha256()
    for row in sorted(rows, key=lambda value: value["path"].encode("ascii")):
        path, size, checksum = row["path"], row["bytes"], row["sha256"]
        if not SAFE.fullmatch(path) or path.startswith("/") or ".." in path.split("/") or "//" in path:
            raise ValueError("unsafe runtime input path")
        if row["mode"] != MODE or not isinstance(size, int) or size < 0 or size > MAX_BYTES or not re.fullmatch(r"[a-f0-9]{64}", checksum):
            raise ValueError("invalid runtime input inventory")
        digest.update(f"{path}\0{MODE}\0{size}\0{checksum}\n".encode("ascii"))
    return digest.hexdigest()


def git(repo: Path, *args: str) -> bytes:
    return subprocess.run(["git", "-C", str(repo), *args], check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE).stdout


def stage(repo: Path, revision: str, destination: Path) -> dict:
    if not re.fullmatch(r"[a-f0-9]{40}", revision) or git(repo, "rev-parse", "--verify", "HEAD").strip().decode() != revision:
        raise ValueError("runtime inputs require the exact selected HEAD")
    if destination.exists():
        raise ValueError("runtime staging destination already exists")
    entries = git(repo, "ls-tree", "-rz", "--full-tree", revision, "sandboxd", "images/sandbox")
    files = []
    total = 0
    for entry in entries.split(b"\0"):
        if not entry:
            continue
        metadata, raw_path = entry.split(b"\t", 1)
        mode, kind, oid = metadata.decode("ascii").split(" ")
        path = raw_path.decode("ascii")
        if not selected(path):
            continue
        if not SAFE.fullmatch(path) or path.startswith("/") or ".." in path.split("/") or "//" in path or kind != "blob" or mode not in ("100644", "100755"):
            raise ValueError("runtime input is not a safe regular Git blob")
        size = int(git(repo, "cat-file", "-s", oid))
        if len(files) >= MAX_FILES or size < 0 or size > MAX_BYTES - total:
            raise ValueError("runtime inputs exceed bounded inventory")
        blob = git(repo, "cat-file", "blob", oid)
        if len(blob) != size:
            raise ValueError("runtime Git blob changed during staging")
        total += size
        files.append((path, blob))
    paths = {path for path, _ in files}
    if not ROOT_FILES <= paths or not any(path.startswith("sandboxd/vendor/") for path in paths):
        raise ValueError("runtime input inventory is incomplete")
    rows = [{"path": path, "mode": MODE, "bytes": len(blob), "sha256": hashlib.sha256(blob).hexdigest()} for path, blob in files]
    digest = inventory_digest(rows)
    destination.mkdir(parents=True, mode=0o700)
    for path, blob in files:
        output = destination / path
        output.parent.mkdir(parents=True, exist_ok=True)
        output.write_bytes(blob)
        os.chmod(output, 0o644)
    for directory in (p for p in destination.rglob("*") if p.is_dir()):
        os.chmod(directory, 0o755)
    return {"algorithm": ALGORITHM, "sourceCommit": revision, "canonicalInputDigest": digest, "sourceFiles": sorted(rows, key=lambda row: row["path"].encode("ascii"))}


if __name__ == "__main__":
    parser = argparse.ArgumentParser()
    parser.add_argument("--repo", type=Path, required=True)
    parser.add_argument("--revision", required=True)
    parser.add_argument("--output", type=Path, required=True)
    args = parser.parse_args()
    try:
        result = stage(args.repo, args.revision, args.output)
    except (ValueError, subprocess.CalledProcessError, UnicodeError) as exc:
        print(f"runtime canonical input refusal: {exc}", file=sys.stderr)
        raise SystemExit(1) from exc
    print(json.dumps(result, sort_keys=True, separators=(",", ":")))

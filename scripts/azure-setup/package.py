#!/usr/bin/env python3
"""Package one clean, exact Luci Azure Develop installer for distribution."""

import argparse
import hashlib
import io
import json
from pathlib import Path
import re
import stat
import subprocess
import tarfile


SHA = re.compile(r"[0-9a-f]{64}\Z")
REV = re.compile(r"[0-9a-f]{40}\Z")
MAX_ARCHIVE = 4 << 20
MODULE = ("compute.tf", "fence.tf", "identity.tf", "network.tf", "outputs.tf", "versions.tf", "variables.tf", ".terraform.lock.hcl")
SCRIPTS = ("azure-setup/installer.py", "azure-setup/README.md", "azure-embed-host/package.py", "azure-embed-host/service.py",
           "azure-embed-host/deliver.py", "azure-controller-disk/package.py", "azure-controller-disk/mount.py",
           "azure-controller-disk/recover.py")


def sha(data):
    return hashlib.sha256(data).hexdigest()


def collect(source, revision):
    source = Path(source)
    if not REV.fullmatch(revision) or subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip() != revision or subprocess.check_output(["git", "-C", str(source), "status", "--porcelain"], text=True).strip():
        raise ValueError("Azure setup bundle requires a clean immutable Luci revision")
    files = {"LICENSE": (source / "LICENSE").read_bytes()}
    for name in MODULE:
        files["terraform/" + name] = (source / "infra/terraform/azure_embed_private" / name).read_bytes()
    for name in SCRIPTS:
        files[name] = (source / "scripts" / name).read_bytes()
    for name in files:
        if name.startswith("terraform/"):
            origin = source / "infra/terraform/azure_embed_private" / Path(name).name
        elif "/" in name:
            origin = source / "scripts" / name
        else:
            origin = source / name
        info = origin.lstat()
        if not stat.S_ISREG(info.st_mode) or info.st_size == 0 or info.st_size > 2 << 20:
            raise ValueError("Azure setup bundle has an unsafe source member")
    files["manifest.json"] = (json.dumps({"version": 1, "luciRevision": revision,
        "files": {name: {"sha256": sha(data), "size": len(data)} for name, data in sorted(files.items())}},
        sort_keys=True, separators=(",", ":")) + "\n").encode()
    return files


def write_archive(files, path):
    with Path(path).open("wb") as sink, tarfile.open(fileobj=sink, mode="w", format=tarfile.USTAR_FORMAT) as tar:
        for name, data in sorted(files.items()):
            row = tarfile.TarInfo(name)
            row.size, row.mode, row.mtime, row.uid, row.gid = len(data), 0o755 if name.endswith(".py") else 0o644, 0, 0, 0
            tar.addfile(row, io.BytesIO(data))
    if Path(path).stat().st_size > MAX_ARCHIVE:
        Path(path).unlink()
        raise ValueError("Azure setup bundle exceeds the size limit")
    return sha(Path(path).read_bytes())


def verify_archive(path, expected):
    path = Path(path)
    if not SHA.fullmatch(expected) or path.is_symlink() or path.stat().st_size > MAX_ARCHIVE or sha(path.read_bytes()) != expected:
        raise ValueError("Azure setup bundle identity changed")
    files = {}
    with tarfile.open(path, mode="r:") as tar:
        for row in tar:
            if not row.isfile() or row.name in files or Path(row.name).is_absolute() or ".." in Path(row.name).parts or row.size == 0 or row.size > 2 << 20:
                raise ValueError("Azure setup bundle entry is unsafe")
            files[row.name] = tar.extractfile(row).read()
    wanted = {"LICENSE", "manifest.json"} | {"terraform/" + name for name in MODULE} | set(SCRIPTS)
    if set(files) != wanted:
        raise ValueError("Azure setup bundle inventory changed")
    manifest = json.loads(files["manifest.json"])
    if manifest.get("version") != 1 or not REV.fullmatch(manifest.get("luciRevision", "")) or set(manifest.get("files", {})) != wanted - {"manifest.json"}:
        raise ValueError("Azure setup bundle source revision is invalid")
    for name, data in files.items():
        if name != "manifest.json" and manifest["files"][name] != {"sha256": sha(data), "size": len(data)}:
            raise ValueError("Azure setup bundle member changed")
    return files


def main():
    parser = argparse.ArgumentParser()
    commands = parser.add_subparsers(dest="action", required=True)
    build = commands.add_parser("build")
    build.add_argument("--source", required=True)
    build.add_argument("--revision", required=True)
    build.add_argument("--output", required=True)
    verify = commands.add_parser("verify")
    verify.add_argument("--archive", required=True)
    verify.add_argument("--sha256", required=True)
    args = parser.parse_args()
    if args.action == "build":
        print(write_archive(collect(args.source, args.revision), args.output))
    else:
        verify_archive(args.archive, args.sha256)
        print(args.sha256)


if __name__ == "__main__":
    main()

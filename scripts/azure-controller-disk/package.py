#!/usr/bin/env python3
"""Build an immutable Azure controller image payload without contacting Azure.

Image construction is separate from account provisioning. The selected
operator template and environment are generated later by the private setup
installer; this payload never embeds credentials or an ARM access token.
"""

import argparse
import hashlib
import io
import json
from pathlib import Path
import re
import shutil
import stat
import subprocess
import tarfile
import tempfile


SHA = re.compile(r"[0-9a-f]{64}\Z")
REVISION = re.compile(r"[0-9a-f]{40}\Z")
MAX_ARCHIVE = 128 << 20
MAX_FILE = 96 << 20
SOURCES = ("mount.py", "recover.py")


def sha(data):
    return hashlib.sha256(data).hexdigest()


def regular(path):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_size < 1 or info.st_size > MAX_FILE:
        raise ValueError("controller package input is not a bounded regular file")
    return path.read_bytes()


def exact_source(source, revision):
    if not REVISION.fullmatch(revision):
        raise ValueError("controller source revision is not immutable")
    head = subprocess.check_output(["git", "-C", str(source), "rev-parse", "HEAD"], text=True).strip()
    dirty = subprocess.check_output(["git", "-C", str(source), "status", "--porcelain"], text=True).strip()
    if head != revision or dirty:
        raise ValueError("controller package source must be exact and clean")


def exact_go_binary(api, revision):
    output = subprocess.check_output(["go", "version", "-m", str(api)], text=True, timeout=10)
    if "\tbuild\tvcs.revision=" + revision + "\n" not in output + "\n" or "\tbuild\tvcs.modified=false" not in output:
        raise ValueError("controller API binary is not bound to the selected clean source")


def collect(source, revision, api):
    exact_source(source, revision)
    exact_go_binary(api, revision)
    script = source / "scripts/azure-controller-disk"
    files = {"bin/luci-api": regular(api), "licences/LUCI-LICENSE": regular(source / "LICENSE")}
    for name in SOURCES:
        files["bin/" + name] = regular(script / name)
    files["bin/deliver.py"] = regular(source / "scripts/azure-embed-host/deliver.py")
    manifest = {"version": 1, "luciRevision": revision,
                "files": {name: {"sha256": sha(data), "size": len(data)} for name, data in sorted(files.items())}}
    files["manifest.json"] = (json.dumps(manifest, sort_keys=True, separators=(",", ":")) + "\n").encode()
    return files


def archive(files, path):
    with Path(path).open("wb") as sink, tarfile.open(fileobj=sink, mode="w", format=tarfile.USTAR_FORMAT) as tar:
        for name, data in sorted(files.items()):
            row = tarfile.TarInfo(name)
            row.size, row.mode, row.mtime, row.uid, row.gid = len(data), 0o755 if name.startswith("bin/") else 0o644, 0, 0, 0
            tar.addfile(row, io.BytesIO(data))
    if Path(path).stat().st_size > MAX_ARCHIVE:
        Path(path).unlink()
        raise ValueError("controller package is too large")
    return sha(Path(path).read_bytes())


def read_archive(path, expected):
    if not SHA.fullmatch(expected) or Path(path).stat().st_size > MAX_ARCHIVE:
        raise ValueError("controller package identity is invalid")
    data = Path(path).read_bytes()
    if sha(data) != expected:
        raise ValueError("controller package hash changed")
    files = {}
    with tarfile.open(fileobj=io.BytesIO(data), mode="r:") as tar:
        for row in tar:
            name = Path(row.name)
            if not row.isfile() or row.name in files or name.is_absolute() or any(part in ("", ".", "..") for part in name.parts) or row.size < 1 or row.size > MAX_FILE:
                raise ValueError("controller package has an unsafe entry")
            files[row.name] = tar.extractfile(row).read()
    required = {"bin/luci-api", "bin/mount.py", "bin/recover.py", "bin/deliver.py", "licences/LUCI-LICENSE", "manifest.json"}
    if set(files) != required:
        raise ValueError("controller package inventory changed")
    manifest = json.loads(files["manifest.json"])
    if manifest.get("version") != 1 or not REVISION.fullmatch(manifest.get("luciRevision", "")) or set(manifest.get("files", {})) != required - {"manifest.json"}:
        raise ValueError("controller package source binding is invalid")
    for name, data in files.items():
        if name != "manifest.json" and manifest["files"][name] != {"sha256": sha(data), "size": len(data)}:
            raise ValueError("controller package member changed")
    return files


API_UNIT = """[Unit]
Description=Luci private Azure controller API
After=network-online.target
RequiresMountsFor=__MOUNT__
[Service]
Type=simple
EnvironmentFile=/etc/luci/azure-controller.env
ExecStart=/opt/luci/azure-controller/bin/luci-api
Restart=no
UMask=0077
RuntimeDirectory=luci
NoNewPrivileges=yes
ProtectSystem=strict
ReadWritePaths=__MOUNT__ /run/luci
[Install]
WantedBy=multi-user.target
"""


def install(files, root, mount):
    if not re.fullmatch(r"/mnt/[a-z0-9-]+", mount):
        raise ValueError("controller mount path is invalid")
    root = Path(root)
    parent = root / "opt/luci"
    destination = parent / "azure-controller"
    unit = root / "etc/systemd/system/luci-api.service"
    if destination.exists() or destination.is_symlink() or unit.exists() or unit.is_symlink():
        raise ValueError("controller image already contains this package")
    parent.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix="azure-controller-", dir=parent))
    try:
        for name, data in files.items():
            path = stage / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
            path.chmod(0o755 if name.startswith("bin/") else 0o644)
        unit.parent.mkdir(parents=True, exist_ok=True)
        unit.write_text(API_UNIT.replace("__MOUNT__", mount))
        unit.chmod(0o644)
        shutil.move(stage, destination)
    finally:
        if stage.exists():
            shutil.rmtree(stage)


def main():
    parser = argparse.ArgumentParser()
    commands = parser.add_subparsers(dest="action", required=True)
    build = commands.add_parser("build")
    for name in ("source", "revision", "api", "output"):
        build.add_argument("--" + name, required=True)
    verify = commands.add_parser("verify")
    verify.add_argument("--archive", required=True)
    verify.add_argument("--sha256", required=True)
    image = commands.add_parser("install-image")
    image.add_argument("--archive", required=True)
    image.add_argument("--sha256", required=True)
    image.add_argument("--root", required=True)
    image.add_argument("--mount", required=True)
    args = parser.parse_args()
    if args.action == "build":
        print(archive(collect(Path(args.source), args.revision, Path(args.api)), args.output))
    else:
        files = read_archive(args.archive, args.sha256)
        if args.action == "install-image":
            install(files, args.root, args.mount)
        else:
            print(args.sha256)


if __name__ == "__main__":
    main()

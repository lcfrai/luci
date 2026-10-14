#!/usr/bin/env python3
"""Build and install an immutable, nonsecret dedicated Azure Embed host package.

The upstream checkout and every image digest are explicit inputs. Nothing here
contacts Azure, pulls images, starts Docker, or reads credentials.
"""

import argparse
import hashlib
import io
import importlib.util
import json
import os
from pathlib import Path
import re
import shutil
import stat
import subprocess
import tarfile
import tempfile

import yaml


UPSTREAM_REVISION = "cf536c284434fdd2f09bbab35ec807a5dacc87e4"
SHA = re.compile(r"[0-9a-f]{64}\Z")
GIT_SHA = re.compile(r"[0-9a-f]{40}\Z")
SERVICE = re.compile(r"[a-z][a-z0-9-]*\Z")
IMAGE = re.compile(r"[a-zA-Z0-9][a-zA-Z0-9._:/-]*@sha256:[0-9a-f]{64}\Z")
MAX_ARCHIVE = 256 << 20
MAX_FILE = 128 << 20


def digest(data):
    return hashlib.sha256(data).hexdigest()


def regular_bytes(path):
    info = path.lstat()
    if not stat.S_ISREG(info.st_mode) or info.st_size > MAX_FILE:
        raise ValueError("package input is not a bounded regular file")
    return path.read_bytes()


def exact_checkout(path, expected):
    head = subprocess.check_output(["git", "-C", str(path), "rev-parse", "HEAD"], text=True).strip()
    dirty = subprocess.check_output(["git", "-C", str(path), "status", "--porcelain"], text=True).strip()
    if head != expected or dirty:
        raise ValueError("package source checkout must be exact and clean")


def collect(args):
    upstream = Path(args.upstream).resolve()
    luci = Path(args.luci).resolve()
    exact_checkout(upstream, UPSTREAM_REVISION)
    exact_checkout(luci, args.luci_revision)
    compose = upstream / "embed" / "compose"
    files = {}
    for path in sorted(compose.rglob("*")):
        if path.is_symlink():
            raise ValueError("upstream Compose package contains a link")
        if path.is_file():
            rel = path.relative_to(compose).as_posix()
            if len(rel) > 180 or rel in (".", "..") or rel.startswith("../") or "/../" in rel:
                raise ValueError("upstream Compose package path is unsafe")
            files["compose/" + rel] = regular_bytes(path)
    if "compose/compose.yaml" not in files or "compose/.env" not in files:
        raise ValueError("pinned upstream Compose tree is incomplete")
    source = yaml.safe_load(files["compose/compose.yaml"])
    services = set(source.get("services", {}))
    locks = json.loads(regular_bytes(Path(args.image_digests)))
    if not isinstance(locks, dict) or set(locks) != services or not all(
        SERVICE.fullmatch(k) and isinstance(v, str) and IMAGE.fullmatch(v)
        for k, v in locks.items()
    ):
        raise ValueError("every upstream service needs an immutable image digest")
    if any("build" in value or "extends" in value for value in source["services"].values()):
        raise ValueError("runtime host must not build or extend service images")
    files["compose/image-lock.yaml"] = yaml.safe_dump(
        {"services": {key: {"image": locks[key]} for key in sorted(locks)}},
        sort_keys=True,
    ).encode()
    # The ordinary upstream Compose default builds e2bdev/base independently.
    # Azure publishes the signed selected guest through a durable controller
    # journal, so its default stack must never run that second build/ready path.
    files["compose/azure-services.yaml"] = yaml.safe_dump({"services": {
        name: {"profiles": ["luci-manual-template-only"]}
        for name in ("base-template", "ready") if name in services}}, sort_keys=True).encode()
    guest_receipt = getattr(args, 'guest_receipt', None)
    if guest_receipt:
        spec = importlib.util.spec_from_file_location('luci_guest_registry', Path(__file__).with_name('guest_registry.py'))
        registry = importlib.util.module_from_spec(spec); spec.loader.exec_module(registry)
        selected = registry.verify(Path(guest_receipt).parent, args.luci_revision)
        if json.loads(regular_bytes(Path(guest_receipt))) != selected:
            raise ValueError('Selected guest registry receipt differs')
        files['guest-source-receipt.json'] = regular_bytes(Path(guest_receipt))
    files["licences/E2B-LICENSE"] = regular_bytes(upstream / "LICENSE")
    files["licences/LUCI-LICENSE"] = regular_bytes(luci / "LICENSE")
    files["licences/ENVOY-LICENSE"] = regular_bytes(Path(args.envoy_license))
    if getattr(args, 'envoy_notice', None):
        files['licences/ENVOY-NOTICE'] = regular_bytes(Path(args.envoy_notice))
    for name, path in {
        "azure-embed-host-bootstrap": args.bootstrap,
        "azure-gateway-config": args.gateway,
        "envoy": args.envoy,
        "service.py": Path(__file__).with_name("service.py"),
        "guest_registry.py": Path(__file__).with_name("guest_registry.py"),
    }.items():
        files["bin/" + name] = regular_bytes(Path(path))
    manifest = {
        "version": 1,
        "luciRevision": args.luci_revision,
        "e2bRevision": UPSTREAM_REVISION,
        "files": {name: {"sha256": digest(data), "size": len(data)} for name, data in sorted(files.items())},
    }
    files["manifest.json"] = (json.dumps(manifest, sort_keys=True, separators=(",", ":")) + "\n").encode()
    return files


def write_archive(files, path):
    path = Path(path)
    with path.open("wb") as sink, tarfile.open(fileobj=sink, mode="w", format=tarfile.USTAR_FORMAT) as out:
        for name, data in sorted(files.items()):
            item = tarfile.TarInfo(name)
            item.size = len(data)
            item.mode = 0o755 if name.startswith("bin/") else 0o644
            item.mtime = 0
            item.uid = item.gid = 0
            out.addfile(item, io.BytesIO(data))
    if path.stat().st_size > MAX_ARCHIVE:
        path.unlink()
        raise ValueError("host package exceeds the bounded size")
    return digest(path.read_bytes())


def read_archive(path, expected):
    path = Path(path)
    if not SHA.fullmatch(expected) or path.stat().st_size > MAX_ARCHIVE:
        raise ValueError("host package identity or size is invalid")
    raw = path.read_bytes()
    if digest(raw) != expected:
        raise ValueError("host package SHA-256 does not match selected image")
    files = {}
    with tarfile.open(fileobj=io.BytesIO(raw), mode="r:") as archive:
        for item in archive:
            if not item.isfile() or item.name in files or item.size > MAX_FILE or len(item.name) > 180:
                raise ValueError("host package entry is unsafe")
            p = Path(item.name)
            if p.is_absolute() or any(part in ("", ".", "..") for part in p.parts):
                raise ValueError("host package path is unsafe")
            files[item.name] = archive.extractfile(item).read()
    if "manifest.json" not in files:
        raise ValueError("host package manifest is missing")
    manifest = json.loads(files.pop("manifest.json"))
    if manifest.get("version") != 1 or manifest.get("e2bRevision") != UPSTREAM_REVISION or not GIT_SHA.fullmatch(manifest.get("luciRevision", "")):
        raise ValueError("host package source binding is invalid")
    if set(manifest.get("files", {})) != set(files):
        raise ValueError("host package file inventory changed")
    for name, data in files.items():
        record = manifest["files"][name]
        if record != {"sha256": digest(data), "size": len(data)}:
            raise ValueError("host package file changed")
    files["manifest.json"] = (json.dumps(manifest, sort_keys=True, separators=(",", ":")) + "\n").encode()
    return files


MOUNT_UNIT = """[Unit]
Description=Private Luci Embed host secret tmpfs
Before=luci-azure-embed-host.service
[Mount]
What=tmpfs
Where=/run/luci_embed
Type=tmpfs
Options=size=16m,mode=0700,nosuid,nodev,noexec
[Install]
WantedBy=multi-user.target
"""
GATEWAY_UNIT = """[Unit]
Description=Private Luci Embed mTLS gateway
After=network-online.target
[Service]
Type=simple
ExecStart=/opt/luci/azure-embed/package/bin/envoy -c /run/luci_embed/active-envoy.yaml
Restart=no
UMask=0077
[Install]
WantedBy=multi-user.target
"""
HOST_UNIT = """[Unit]
Description=Owned Azure Embed host bootstrap and recovery gate
Requires=run-luci_embed.mount docker.service
After=run-luci_embed.mount docker.service network-online.target
[Service]
Type=oneshot
RemainAfterExit=yes
ExecStart=/usr/bin/python3 /opt/luci/azure-embed/package/bin/service.py boot /etc/luci/azure-embed-deployment.json
ExecReload=/usr/bin/python3 /opt/luci/azure-embed/package/bin/service.py rotate /etc/luci/azure-embed-deployment.json
UMask=0077
TimeoutStartSec=180
[Install]
WantedBy=multi-user.target
"""

REGISTRY_DEPENDENCY = """[Unit]
Requires=luci-azure-guest-registry.service
After=luci-azure-guest-registry.service
"""


def install(files, root, expected, archive):
    """Image-build operation: no systemctl, container or firewall effect."""
    root = Path(root)
    parent = root / "opt/luci/azure-embed"
    target = parent / "package"
    if target.exists() or target.is_symlink() or (parent / "package.tar").exists():
        raise ValueError("selected image already contains a host package")
    parent.mkdir(parents=True, exist_ok=True)
    stage = Path(tempfile.mkdtemp(prefix="package-", dir=parent))
    try:
        for name, data in files.items():
            path = stage / name
            path.parent.mkdir(parents=True, exist_ok=True)
            path.write_bytes(data)
            path.chmod(0o755 if name.startswith("bin/") else 0o644)
        units = root / "etc/systemd/system"
        units.mkdir(parents=True, exist_ok=True)
        for name in ("run-luci_embed.mount", "luci-azure-embed-gateway.service", "luci-azure-embed-host.service"):
            if (units / name).exists():
                raise ValueError("host unit already exists")
        shutil.copyfile(archive, parent / "package.tar")
        os.replace(stage, target)
        (parent / "package.sha256").write_text(expected + "\n")
        for name, content in {
            "run-luci_embed.mount": MOUNT_UNIT,
            "luci-azure-embed-gateway.service": GATEWAY_UNIT,
            "luci-azure-embed-host.service": HOST_UNIT,
        }.items():
            (units / name).write_text(content)
        if 'guest-source-receipt.json' in files:
            dropin = units / 'luci-azure-embed-host.service.d'
            dropin.mkdir()
            (dropin / 'guest-registry.conf').write_text(REGISTRY_DEPENDENCY)
    except Exception:
        if stage.exists():
            shutil.rmtree(stage)
        raise


def main():
    parser = argparse.ArgumentParser()
    commands = parser.add_subparsers(dest="command", required=True)
    build = commands.add_parser("build")
    for flag in ("upstream", "luci", "luci-revision", "image-digests", "bootstrap", "gateway", "envoy", "envoy-license", "output"):
        build.add_argument("--" + flag, required=True)
    build.add_argument('--guest-receipt')
    build.add_argument('--envoy-notice')
    verify = commands.add_parser("verify")
    verify.add_argument("--archive", required=True)
    verify.add_argument("--sha256", required=True)
    image = commands.add_parser("install-image")
    image.add_argument("--archive", required=True)
    image.add_argument("--sha256", required=True)
    image.add_argument("--root", required=True)
    args = parser.parse_args()
    if args.command == "build":
        if not GIT_SHA.fullmatch(args.luci_revision):
            raise ValueError("Luci source revision must be immutable")
        print(write_archive(collect(args), args.output))
    else:
        files = read_archive(args.archive, args.sha256)
        if args.command == "install-image":
            install(files, args.root, args.sha256, args.archive)
        print(args.sha256)


if __name__ == "__main__":
    main()

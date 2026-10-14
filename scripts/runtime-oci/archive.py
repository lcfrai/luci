"""Bounded, streaming normalization and verification of a single Docker image.

The outer tar contains ordinary files only. Layer tar contents are opaque image
filesystem data: their uncompressed digests, not host extraction, establish identity.
"""
from __future__ import annotations

import hashlib
import json
from pathlib import Path, PurePosixPath
import re
import stat
import tarfile

MAX_ARCHIVE = 8 << 30
MAX_METADATA = 1 << 20
MAX_MEMBERS = 4096
HEX = re.compile(r"^[a-f0-9]{64}$")
RUNTIME = {"user": "dev", "workingDir": "/home/dev", "entrypoint": ["/usr/local/bin/entrypoint.sh"], "exposedPorts": ["8080/tcp"],
           "harnesses": {"codex": "0.160.0", "claudeCode": "2.1.274", "openCode": "1.18.34"}}


def regular(path: Path) -> None:
    path = Path(path).absolute()
    for entry in [path, *path.parents]:
        info = entry.lstat()
        if entry.is_symlink() or getattr(info, "st_file_attributes", 0) & 1024:
            raise ValueError("linked runtime input refused")
    if not stat.S_ISREG(path.lstat().st_mode):
        raise ValueError("runtime input must be a regular file")


def safe_path(name: str) -> str:
    path = PurePosixPath(name)
    if not name or not path.parts or len(name) > 256 or not re.fullmatch(r"[A-Za-z0-9._/-]+", name) or path.is_absolute() or path.as_posix() != name or any(p in (".", "..") for p in path.parts):
        raise ValueError("unsafe runtime archive path")
    return name


def unique_object(pairs):
    result = {}
    for key, value in pairs:
        if key in result:
            raise ValueError("duplicate JSON field")
        result[key] = value
    return result


def json_bytes(raw: bytes):
    if len(raw) > MAX_METADATA:
        raise ValueError("runtime metadata exceeds 1 MiB")
    def refuse_constant(value):
        raise ValueError("nonfinite JSON number refused")
    return json.loads(raw.decode("utf-8", errors="strict"), object_pairs_hook=unique_object, parse_constant=refuse_constant)


def encoded(value) -> bytes:
    return (json.dumps(value, ensure_ascii=True, sort_keys=True, separators=(",", ":")) + "\n").encode("utf-8")


def stream_hash(stream, expected_size=None):
    digest, size = hashlib.sha256(), 0
    for block in iter(lambda: stream.read(1 << 20), b""):
        size += len(block)
        if size > MAX_ARCHIVE or expected_size is not None and size > expected_size:
            raise ValueError("runtime stream exceeds its bound")
        digest.update(block)
    if expected_size is not None and size != expected_size:
        raise ValueError("runtime stream length changed")
    return digest.hexdigest(), size


def file_record(path: Path, name=None):
    regular(path)
    with path.open("rb") as stream:
        checksum, size = stream_hash(stream, path.stat().st_size)
    return {"path": name or path.name, "sha256": checksum, "bytes": size}


def config_policy(config):
    value = config.get("config")
    rootfs = config.get("rootfs")
    if config.get("os") != "linux" or config.get("architecture") != "amd64" or not isinstance(value, dict) or not isinstance(rootfs, dict) or rootfs.get("type") != "layers":
        raise ValueError("runtime image platform/configuration mismatch")
    if value.get("User") != RUNTIME["user"] or value.get("WorkingDir") != RUNTIME["workingDir"] or value.get("Entrypoint") != RUNTIME["entrypoint"] or sorted(value.get("ExposedPorts", {})) != RUNTIME["exposedPorts"] or value.get("Volumes") or value.get("OnBuild"):
        raise ValueError("runtime image execution configuration mismatch")
    labels = value.get("Labels") or {}
    if not isinstance(labels, dict) or any(key.startswith("luci.") and not key.startswith("luci.runtime-payload.") for key in labels):
        raise ValueError("shared payload has execution ownership labels")
    diff_ids = rootfs.get("diff_ids")
    if not isinstance(diff_ids, list) or not 1 <= len(diff_ids) <= 2048 or len(set(diff_ids)) != len(diff_ids) or any(not isinstance(x, str) or not x.startswith("sha256:") or not HEX.fullmatch(x[7:]) for x in diff_ids):
        raise ValueError("invalid or duplicate runtime layer identity")
    return diff_ids


def scan(archive: tarfile.TarFile, normalized=False):
    members, size = {}, 0
    for member in archive:
        name = safe_path(member.name.rstrip("/") if member.isdir() else member.name)
        if name in members or len(members) >= MAX_MEMBERS or member.size < 0:
            raise ValueError("duplicate/oversized runtime tar inventory")
        if member.isdir() and not normalized:
            members[name] = member
            continue
        if not member.isfile():
            raise ValueError("linked or special runtime archive member")
        size += member.size
        if size > MAX_ARCHIVE:
            raise ValueError("runtime archive content exceeds 8 GiB")
        members[name] = member
    return members


def read_metadata(archive, members, name):
    member = members.get(name)
    if member is None or not member.isfile() or member.size > MAX_METADATA:
        raise ValueError("missing or oversized runtime archive metadata")
    stream = archive.extractfile(member)
    if stream is None:
        raise ValueError("unreadable runtime archive metadata")
    with stream:
        raw = stream.read(MAX_METADATA + 1)
    return raw, json_bytes(raw)


def normalize(source: Path, destination: Path, image_id: str):
    regular(source)
    if source.stat().st_size > MAX_ARCHIVE or not image_id.startswith("sha256:") or not HEX.fullmatch(image_id[7:]) or destination.exists():
        raise ValueError("invalid runtime export input/output")
    with tarfile.open(source, "r:") as archive:
        members = scan(archive)
        _, manifests = read_metadata(archive, members, "manifest.json")
        if not isinstance(manifests, list) or len(manifests) != 1 or not isinstance(manifests[0], dict):
            raise ValueError("runtime export must contain one image")
        manifest = manifests[0]
        if set(manifest) - {"Config", "RepoTags", "Layers", "LayerSources"}:
            raise ValueError("unexpected Docker-save image fields")
        config_name = safe_path(manifest["Config"])
        config_raw, config = read_metadata(archive, members, config_name)
        if hashlib.sha256(config_raw).hexdigest() != image_id[7:]:
            raise ValueError("runtime export config ID mismatch")
        diff_ids = config_policy(config)
        layers = manifest.get("Layers")
        if not isinstance(layers, list) or len(layers) != len(diff_ids) or len(set(layers)) != len(layers):
            raise ValueError("runtime export layer inventory mismatch")
        mappings = {}
        for old, diff_id in zip(layers, diff_ids):
            old = safe_path(old)
            member = members.get(old)
            if member is None or not member.isfile():
                raise ValueError("runtime layer is missing")
            with archive.extractfile(member) as stream:
                checksum, _ = stream_hash(stream, member.size)
            if checksum != diff_id[7:]:
                raise ValueError("Docker-save layer is not the exact uncompressed diffID")
            mappings[diff_id[7:] + "/layer.tar"] = member
        config_path = image_id[7:] + ".json"
        layer_paths = [x[7:] + "/layer.tar" for x in diff_ids]
        manifest_raw = encoded([{"Config": config_path, "RepoTags": [], "Layers": layer_paths}])
        records = []
        import io
        with tarfile.open(destination, "w", format=tarfile.USTAR_FORMAT) as output:
            for name in sorted([config_path, "manifest.json", *mappings]):
                if name == config_path:
                    raw, stream = config_raw, io.BytesIO(config_raw)
                elif name == "manifest.json":
                    raw, stream = manifest_raw, io.BytesIO(manifest_raw)
                else:
                    raw, stream = None, archive.extractfile(mappings[name])
                length = len(raw) if raw is not None else mappings[name].size
                info = tarfile.TarInfo(name)
                info.size, info.mode, info.mtime = length, 0o644, 0
                with stream:
                    output.addfile(info, stream)
                checksum = hashlib.sha256(raw).hexdigest() if raw is not None else name.split("/")[0]
                records.append({"path": name, "sha256": checksum, "bytes": length})
    result = {"archive": file_record(destination), "members": records,
              "image": {"id": image_id, "configPath": config_path, "layerPaths": layer_paths, "diffIDs": diff_ids, "repoTags": []}}
    if result["archive"]["bytes"] > MAX_ARCHIVE:
        raise ValueError("normalized runtime archive exceeds 8 GiB")
    verify(destination, result["image"], records)
    return result


def verify(path: Path, image: dict, expected_members: list):
    regular(path)
    if path.stat().st_size > MAX_ARCHIVE:
        raise ValueError("runtime archive exceeds 8 GiB")
    with tarfile.open(path, "r:") as archive:
        members = scan(archive, normalized=True)
        if list(members) != sorted(members) or list(members) != [x["path"] for x in expected_members]:
            raise ValueError("runtime archive member inventory/order mismatch")
        for row in expected_members:
            with archive.extractfile(members[row["path"]]) as stream:
                checksum, size = stream_hash(stream, row["bytes"])
            if checksum != row["sha256"] or size != row["bytes"]:
                raise ValueError("runtime archive member bytes mismatch")
        _, manifests = read_metadata(archive, members, "manifest.json")
        if manifests != [{"Config": image["configPath"], "RepoTags": [], "Layers": image["layerPaths"]}]:
            raise ValueError("runtime archive has tags or different image paths")
        raw, config = read_metadata(archive, members, image["configPath"])
        if image["configPath"] != image["id"][7:] + ".json" or hashlib.sha256(raw).hexdigest() != image["id"][7:] or config_policy(config) != image["diffIDs"]:
            raise ValueError("runtime archive configuration identity mismatch")
        if image["repoTags"] or len(image["layerPaths"]) != len(image["diffIDs"]):
            raise ValueError("runtime archive layer/tag mismatch")
        for layer, diff_id in zip(image["layerPaths"], image["diffIDs"]):
            if layer != diff_id[7:] + "/layer.tar" or next(x["sha256"] for x in expected_members if x["path"] == layer) != diff_id[7:]:
                raise ValueError("runtime uncompressed layer identity mismatch")
    return True

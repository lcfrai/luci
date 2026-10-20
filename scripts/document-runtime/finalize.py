#!/usr/bin/env python3
"""Convert one positively retained document assembler OCI capture to a candidate Docker save.

This offline maintainer tool has no installer authority. Its input hashes are
supplied by the trusted release owner, not copied from the files being read.
The resulting manifest still needs an independent release pin before startup.
"""
from __future__ import annotations

import argparse
import gzip
import hashlib
import io
import json
import os
from pathlib import Path
import re
import stat
import struct
import tarfile

HEX = re.compile(r"^[0-9a-f]{64}$")
COMMIT = re.compile(r"^[0-9a-f]{40}$")
MAGIC = b"LUCI-DOCUMENT-OCI/1\n"
END = b"\nLUCI-DOCUMENT-OCI-END/1\n"
MAX_CAPTURE = 2 << 30
MAX_META = 1 << 20
MAX_LAYER = 2 << 30
MAX_ARCHIVE = 8 << 30
BLOCK = 64 << 10
ENTRY = ["/usr/bin/python3", "-I", "/opt/luci/document_extractor.py"]


def refuse(reason: str):
    raise ValueError("document finalizer refused: " + reason)


def regular(path: Path, maximum: int) -> int:
    path = path.absolute()
    for item in [path, *path.parents]:
        mode = item.lstat().st_mode
        if stat.S_ISLNK(mode):
            refuse("linked input")
    info = path.stat()
    if not stat.S_ISREG(info.st_mode) or info.st_nlink != 1 or info.st_size < 1 or info.st_size > maximum:
        refuse("input is not a bounded ordinary file")
    return info.st_size


def digest_file(path: Path, maximum: int) -> tuple[str, int]:
    expected = regular(path, maximum)
    h = hashlib.sha256()
    with path.open("rb") as source:
        before = os.fstat(source.fileno())
        count = 0
        while True:
            data = source.read(BLOCK)
            if not data:
                break
            count += len(data)
            if count > expected:
                refuse("input grew")
            h.update(data)
        after = os.fstat(source.fileno())
    if count != expected or (before.st_dev, before.st_ino, before.st_size, before.st_mtime_ns) != (after.st_dev, after.st_ino, after.st_size, after.st_mtime_ns):
        refuse("input changed while reading")
    return h.hexdigest(), count


def json_object(pairs):
    out = {}
    for key, value in pairs:
        if key in out:
            refuse("duplicate JSON field")
        out[key] = value
    return out


def decode(raw: bytes):
    if len(raw) > MAX_META:
        refuse("metadata exceeds bound")
    return json.loads(raw.decode("utf-8", "strict"), object_pairs_hook=json_object)


def encoded(value) -> bytes:
    return (json.dumps(value, sort_keys=True, separators=(",", ":"), ensure_ascii=True) + "\n").encode()


def pinned_json(path: Path, pin: str):
    if not HEX.fullmatch(pin) or regular(path, MAX_META) > MAX_META:
        refuse("metadata pin missing")
    raw = path.read_bytes()
    if hashlib.sha256(raw).hexdigest() != pin:
        refuse("metadata differs from independent pin")
    return decode(raw)


def member_name(name: str) -> bool:
    if not name or len(name) > 256 or name.startswith("/") or "\\" in name or any(ord(c) < 32 or ord(c) == 127 for c in name):
        return False
    return all(part not in ("", ".", "..") for part in name.split("/"))


def layer_name(name: str, directory: bool) -> str:
    if not name or len(name.encode("utf-8")) > 1024 or name.startswith("/") or "\\" in name or any(ord(c) < 32 or ord(c) == 127 for c in name):
        refuse("layer name invalid")
    while name.startswith("./"):
        name = name[2:]
    if directory:
        name = name.removesuffix("/")
    if not name or any(part in ("", ".", "..") for part in name.split("/")):
        refuse("layer name invalid")
    if not directory and name.endswith("/"):
        refuse("file name ends in directory marker")
    return name


class BoundedHashReader:
    def __init__(self, source, limit: int):
        self.source, self.remaining, self.hash = source, limit, hashlib.sha256()
        self.read_bytes = 0

    def read(self, amount=-1):
        if amount < 0:
            amount = BLOCK
        if self.remaining == 0:
            return b""
        data = self.source.read(min(amount, self.remaining, BLOCK))
        self.remaining -= len(data)
        self.read_bytes += len(data)
        self.hash.update(data)
        return data

    def drain(self):
        while self.read(BLOCK):
            pass
        if self.remaining != 0:
            refuse("truncated bounded stream")


def exact_receipt(receipt, capture_pin: str, capture_bytes: int):
    outcome = receipt.get("outcome") if isinstance(receipt, dict) else None
    if not isinstance(outcome, dict) or receipt.get("captureExitVerified") is not True or outcome.get("state") != "assembled_owned_resources_retired_not_registered" or outcome.get("exitCode") != 0 or outcome.get("oomKilled") is not False or outcome.get("readerSettled") is not True or outcome.get("watchdogSettled") is not True or outcome.get("cleanupConfirmed") is not True or outcome.get("resourcesHeld") is not False:
        refuse("assembly result is not positively settled")
    capture, closure, identity = outcome.get("capture"), outcome.get("inputClosureRef"), outcome.get("identity")
    if not isinstance(capture, dict) or capture.get("digest") != capture_pin or capture.get("bytes") != capture_bytes or not isinstance(closure, dict) or closure.get("schema") != 1 or not HEX.fullmatch(str(closure.get("digest", ""))) or not isinstance(identity, dict) or identity.get("protocol") != 1 or identity.get("profile") != "document-flat-oci-v1" or not COMMIT.fullmatch(str(identity.get("sourceCommit", ""))) or any(not HEX.fullmatch(str(identity.get(key, ""))) for key in ("sourceDigest", "inputDigest", "helperDigest", "recipeDigest")):
        refuse("assembly capture/closure identity differs")
    return outcome, closure, identity


def oci_index(metadata):
    if metadata.get("oci-layout") != b'{"imageLayoutVersion":"1.0.0"}':
        refuse("OCI layout changed")
    index = decode(metadata.get("index.json", b""))
    manifests = index.get("manifests") if isinstance(index, dict) else None
    if index.get("schemaVersion") != 2 or not isinstance(manifests, list) or len(manifests) != 1:
        refuse("OCI index must select one image")
    descriptor = manifests[0]
    manifest_blob = metadata.get("blobs/sha256/" + str(descriptor.get("digest", ""))[7:])
    if descriptor.get("mediaType") != "application/vnd.oci.image.manifest.v1+json" or not manifest_blob or descriptor.get("size") != len(manifest_blob) or descriptor.get("digest") != "sha256:" + hashlib.sha256(manifest_blob).hexdigest():
        refuse("OCI image manifest changed")
    manifest = decode(manifest_blob)
    layers = manifest.get("layers") if isinstance(manifest, dict) else None
    if manifest.get("schemaVersion") != 2 or not isinstance(layers, list) or len(layers) != 1 or layers[0].get("mediaType") != "application/vnd.oci.image.layer.v1.tar+gzip":
        refuse("OCI layer profile changed")
    config_desc = manifest.get("config")
    if not isinstance(config_desc, dict):
        refuse("OCI config missing")
    config_blob = metadata.get("blobs/sha256/" + str(config_desc.get("digest", ""))[7:])
    if not config_blob or config_desc.get("size") != len(config_blob) or config_desc.get("digest") != "sha256:" + hashlib.sha256(config_blob).hexdigest():
        refuse("OCI config changed")
    config = decode(config_blob)
    app = config.get("config") if isinstance(config, dict) else None
    rootfs = config.get("rootfs") if isinstance(config, dict) else None
    if config.get("os") != "linux" or config.get("architecture") != "amd64" or not isinstance(app, dict) or not isinstance(rootfs, dict) or app.get("User") != "1001:1001" or app.get("WorkingDir") != "/tmp" or app.get("Entrypoint") != ENTRY or app.get("Volumes") or app.get("ExposedPorts") or rootfs.get("type") != "layers" or not isinstance(rootfs.get("diff_ids"), list) or len(rootfs["diff_ids"]) != 1:
        refuse("OCI document execution profile changed")
    diff = rootfs["diff_ids"][0]
    if not isinstance(diff, str) or not diff.startswith("sha256:") or not HEX.fullmatch(diff[7:]):
        refuse("OCI rootfs digest invalid")
    return layers[0], config_blob, config, diff


def inspect_layer(layer_path: Path, closure, identity, notice_closure):
    expected_notices = notice_closure.get("notices")
    if notice_closure.get("schema") != 1 or notice_closure.get("kind") != "luci-document-notice-source-closure-v1" or notice_closure.get("inputClosureSha256") != closure["digest"] or not HEX.fullmatch(str(notice_closure.get("sourceRetentionDigest", ""))) or not isinstance(expected_notices, list) or not 1 <= len(expected_notices) <= 512:
        refuse("notice closure is absent or mismatched")
    notices = {}
    for item in expected_notices:
        if not isinstance(item, dict) or not isinstance(item.get("path"), str) or not item["path"].startswith("notices/") or not member_name(item["path"]) or not HEX.fullmatch(str(item.get("sha256", ""))) or not 1 <= item.get("bytes", 0) <= 1 << 20 or not all(isinstance(item.get(k), str) and 0 < len(item[k]) <= 512 for k in ("component", "version", "grant", "provenance")):
            refuse("notice closure entry invalid")
        path = "opt/luci/licenses/" + item["path"][len("notices/"):]
        if path in notices:
            refuse("duplicate notice")
        notices[path] = item
    wanted = {"opt/luci/document_extractor.py", "opt/luci/assembly-manifest.json", *notices}
    found = {}
    seen = set()
    names_bytes = 0
    with layer_path.open("rb") as source, tarfile.open(fileobj=source, mode="r|") as archive:
        for member in archive:
            name = layer_name(member.name, member.isdir())
            names_bytes += len(member.name.encode("utf-8"))
            if name in seen or len(seen) >= 100000 or names_bytes > 16 << 20:
                refuse("layer path invalid or duplicated")
            seen.add(name)
            if name in wanted:
                if not member.isfile() or member.size < 1 or member.size > MAX_META:
                    refuse("required layer entry invalid")
                stream = archive.extractfile(member)
                if stream is None:
                    refuse("required layer entry unreadable")
                found[name] = stream.read(MAX_META + 1)
    if set(found) != wanted:
        refuse("required source or notice absent from image")
    manifest = decode(found["opt/luci/assembly-manifest.json"])
    if manifest.get("version") != 1 or manifest.get("profile") != "document-flat-oci-v1" or manifest.get("sourceCommit") != identity["sourceCommit"] or manifest.get("sourceDigest") != identity["sourceDigest"] or manifest.get("sourceRetentionDigest") != notice_closure.get("sourceRetentionDigest") or manifest.get("sbomClosed") is not True or manifest.get("authenticated") is not True or manifest.get("distributionNoticesClosed") is not True:
        refuse("embedded assembly obligations differ")
    declared = {}
    if not isinstance(manifest.get("notices"), list):
        refuse("embedded notice inventory absent")
    for item in manifest["notices"]:
        if not isinstance(item, dict) or not isinstance(item.get("path"), str) or not item["path"].startswith("notices/") or not HEX.fullmatch(str(item.get("digest", ""))) or not isinstance(item.get("bytes"), int):
            refuse("embedded notice inventory invalid")
        path = "opt/luci/licenses/" + item["path"][len("notices/"):]
        if path in declared:
            refuse("embedded notice inventory duplicates a path")
        declared[path] = item["digest"], item["bytes"]
    if set(declared) != set(notices):
        refuse("embedded notice inventory differs")
    for path, item in notices.items():
        data = found[path]
        if declared[path] != (item["sha256"], item["bytes"]) or len(data) != item["bytes"] or hashlib.sha256(data).hexdigest() != item["sha256"]:
            refuse("retained notice bytes differ")
    extractor = found["opt/luci/document_extractor.py"]
    if hashlib.sha256(extractor).hexdigest() != manifest.get("extractor", {}).get("digest"):
        refuse("embedded extractor differs")
    return hashlib.sha256(extractor).hexdigest(), manifest


def finalize(args):
    capture = Path(args.capture)
    capture_hash, capture_bytes = digest_file(capture, MAX_CAPTURE)
    if capture_hash != args.capture_sha256:
        refuse("capture differs from independent pin")
    receipt = pinned_json(Path(args.assembly_receipt), args.assembly_receipt_sha256)
    outcome, closure, identity = exact_receipt(receipt, capture_hash, capture_bytes)
    notice_path = Path(args.notice_closure)
    notice = pinned_json(notice_path, args.notice_closure_sha256)
    target = Path(args.output_dir).absolute()
    if target.exists():
        refuse("output directory already exists")
    target.mkdir(mode=0o700)
    notice_bytes = notice_path.read_bytes()
    if len(notice_bytes) > MAX_META or hashlib.sha256(notice_bytes).hexdigest() != args.notice_closure_sha256:
        refuse("notice closure changed after pin check")
    (target / "document-runtime-notices-v1.json").write_bytes(notice_bytes)
    layer_path = target / "layer.tar"
    metadata = {}
    with capture.open("rb") as source:
        if source.read(len(MAGIC)) != MAGIC:
            refuse("assembler frame magic changed")
        declared = struct.unpack(">Q", source.read(8))[0]
        frame_digest = source.read(32)
        if declared < 1024 or declared > MAX_CAPTURE - len(MAGIC) - 8 - 32 - len(END) or len(frame_digest) != 32:
            refuse("assembler frame length invalid")
        bounded = BoundedHashReader(source, declared)
        with tarfile.open(fileobj=bounded, mode="r|") as archive:
            seen = set()
            layer_descriptor = None
            for member in archive:
                if not member.isfile() or not member_name(member.name) or member.name in seen or member.size < 1:
                    refuse("OCI member invalid or duplicated")
                seen.add(member.name)
                stream = archive.extractfile(member)
                if stream is None:
                    refuse("OCI member unreadable")
                if member.name in ("oci-layout", "index.json") or member.name.startswith("blobs/sha256/") and member.size <= MAX_META:
                    if member.size > MAX_META or len(metadata) >= 4:
                        refuse("OCI metadata bound exceeded")
                    metadata[member.name] = stream.read(MAX_META + 1)
                else:
                    if layer_descriptor is not None or not metadata.get("index.json"):
                        refuse("OCI layer order invalid")
                    layer_descriptor, config_bytes, config, diff = oci_index(metadata)
                    if member.name != "blobs/sha256/" + layer_descriptor["digest"][7:] or member.size != layer_descriptor["size"] or member.size > MAX_CAPTURE:
                        refuse("OCI compressed layer differs")
                    compressed = BoundedHashReader(stream, member.size)
                    with gzip.GzipFile(fileobj=compressed) as expanded, layer_path.open("xb") as layer:
                        expanded_hash = hashlib.sha256()
                        total = 0
                        while True:
                            block = expanded.read(BLOCK)
                            if not block:
                                break
                            total += len(block)
                            if total > MAX_LAYER:
                                refuse("expanded layer exceeds bound")
                            layer.write(block)
                            expanded_hash.update(block)
                    compressed.drain()
                    if compressed.hash.hexdigest() != layer_descriptor["digest"][7:] or expanded_hash.hexdigest() != diff[7:]:
                        refuse("OCI layer digest differs")
        bounded.drain()
        if bounded.hash.digest() != frame_digest or source.read(len(END)) != END or source.read(1) != b"":
            refuse("assembler frame digest, ending or EOF invalid")
    if layer_descriptor is None or len(seen) != 5:
        refuse("OCI inventory incomplete")
    labels = config["config"].get("Labels") or {}
    if labels.get("org.opencontainers.image.revision") != identity["sourceCommit"] or labels.get("luci.document-source") != identity["sourceDigest"] or labels.get("luci.document-input-manifest") != identity["inputDigest"] or labels.get("luci.document-source-retention") != notice.get("sourceRetentionDigest"):
        refuse("OCI config source binding differs")
    extractor_sha, assembly_manifest = inspect_layer(layer_path, closure, identity, notice)
    image_id = "sha256:" + hashlib.sha256(config_bytes).hexdigest()
    layer_name = "layer/layer.tar"
    config_name = image_id[7:] + ".json"
    saved = encoded([{"Config": config_name, "RepoTags": [], "Layers": [layer_name]}])
    files = {config_name: config_bytes, "manifest.json": saved, layer_name: layer_path}
    archive_path = target / "document-image-linux-amd64.tar"
    members = []
    with archive_path.open("xb") as output, tarfile.open(fileobj=output, mode="w", format=tarfile.USTAR_FORMAT) as archive:
        for name in sorted(files):
            value = files[name]
            if isinstance(value, Path):
                sha, size = digest_file(value, MAX_LAYER)
                stream = value.open("rb")
            else:
                sha, size = hashlib.sha256(value).hexdigest(), len(value)
                stream = io.BytesIO(value)
            info = tarfile.TarInfo(name)
            info.size, info.mode, info.mtime, info.uid, info.gid = size, 0o444, 0, 0, 0
            with stream:
                archive.addfile(info, stream)
            members.append({"path": name, "sha256": sha, "bytes": size})
    archive_sha, archive_bytes = digest_file(archive_path, MAX_ARCHIVE)
    result = {"version": 1, "kind": "luci-document-prebuilt-docker-save", "toolProtocol": 1,
              "provenance": {"assemblyReceiptSha256": args.assembly_receipt_sha256, "inputClosureSha256": closure["digest"],
                             "captureSha256": capture_hash, "captureBytes": capture_bytes,
                             "noticeClosureSha256": args.notice_closure_sha256, "noticeClosureFileName": "document-runtime-notices-v1.json",
                             "noticeClosureBytes": len(notice_bytes), "sourceCommit": identity["sourceCommit"],
                             "sourceDigest": identity["sourceDigest"], "sourceRetentionDigest": notice["sourceRetentionDigest"]},
              "archive": {"fileName": archive_path.name, "sha256": archive_sha, "bytes": archive_bytes, "members": members},
              "image": {"id": image_id, "configPath": config_name, "layerPath": layer_name, "diffId": diff},
              "extractorSha256": extractor_sha}
    manifest_bytes = encoded(result)
    (target / "document-runtime-manifest-v1.json").write_bytes(manifest_bytes)
    (target / "document-runtime-finalizer-receipt-v1.json").write_bytes(encoded({
        "version": 1, "kind": "luci-document-finalizer-candidate", "manifestSha256": hashlib.sha256(manifest_bytes).hexdigest(),
        "captureSha256": capture_hash, "assemblyReceiptSha256": args.assembly_receipt_sha256,
        "noticeClosureSha256": args.notice_closure_sha256, "releaseAuthority": "none"}))


def main():
    parser = argparse.ArgumentParser()
    for name in ("capture", "capture-sha256", "assembly-receipt", "assembly-receipt-sha256", "notice-closure", "notice-closure-sha256", "output-dir"):
        parser.add_argument("--" + name, required=True)
    finalize(parser.parse_args())


if __name__ == "__main__":
    main()

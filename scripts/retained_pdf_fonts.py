"""Verify the exact embedded PDF font bundle before collecting release notices."""
from __future__ import annotations

import hashlib
import json
import pathlib
import re

PDF_FONTS = pathlib.PurePosixPath("control-plane/internal/product/distribute_pdf_fonts")
SCHEMA = "luci.distribute.pdf-fonts.v1"


def _regular(path: pathlib.Path) -> None:
    if path.is_symlink() or not path.is_file():
        raise ValueError("PDF font input must be a regular file")
    for parent in path.parents:
        if parent.is_symlink() or (hasattr(parent, "is_junction") and parent.is_junction()):
            raise ValueError("Linked PDF font input ancestry")


def _sha(path: pathlib.Path) -> str:
    value = hashlib.sha256()
    with path.open("rb") as stream:
        for block in iter(lambda: stream.read(65536), b""):
            value.update(block)
    return value.hexdigest()


def verify_pdf_fonts(root: pathlib.Path) -> dict:
    directory = root / PDF_FONTS
    manifest_path = directory / "manifest.json"
    _regular(manifest_path)
    if manifest_path.stat().st_size > 16384:
        raise ValueError("PDF font manifest exceeds its bound")
    raw = manifest_path.read_bytes()
    manifest = json.loads(raw)
    if (set(manifest) != {"schema", "fonts", "notices"} or manifest["schema"] != SCHEMA
            or not isinstance(manifest["fonts"], list)
            or not isinstance(manifest["notices"], list)
            or len(manifest["fonts"]) != 3
            or not 1 <= len(manifest["notices"]) <= 8):
        raise ValueError("Unsupported PDF font manifest")
    names = {"manifest.json"}
    roles = set()
    total = 0
    for font in manifest["fonts"]:
        if set(font) != {"id", "file", "bytes", "sha256", "faceIndex", "outline", "license"}:
            raise ValueError("Unexpected PDF font record")
        role = font["id"]
        if role not in {"latin", "arabic", "cjk"} or role in roles:
            raise ValueError("PDF font roles must be complete and unique")
        roles.add(role)
        if (type(font["faceIndex"]) is not int or not 0 <= font["faceIndex"] <= 9
                or font["outline"] not in {"truetype", "cff1"}
                or font["license"] != "OFL-1.1"):
            raise ValueError("Unsupported embedded font format or grant")
    for record in [*manifest["fonts"], *manifest["notices"]]:
        name = record["file"]
        if (not isinstance(name, str) or not re.fullmatch(r"[A-Za-z0-9][A-Za-z0-9._-]{0,127}", name)
                or name in names):
            raise ValueError("Invalid or duplicate PDF font filename")
        names.add(name)
        if (type(record["bytes"]) is not int or not 0 < record["bytes"] <= 32 << 20
                or not isinstance(record["sha256"], str)
                or not re.fullmatch(r"[a-f0-9]{64}", record["sha256"])):
            raise ValueError("Invalid PDF font input identity")
        path = directory / name
        _regular(path)
        if path.stat().st_size != record["bytes"] or _sha(path) != record["sha256"]:
            raise ValueError("Embedded PDF font or grant bytes changed: " + name)
        total += record["bytes"]
    for notice in manifest["notices"]:
        if set(notice) != {"file", "bytes", "sha256"} or notice["bytes"] > 2 << 20:
            raise ValueError("Invalid PDF font notice")
    if total > 34 << 20 or {path.name for path in directory.iterdir()} != names:
        raise ValueError("Unexpected or oversized PDF font bundle")
    if sum(row["bytes"] for row in manifest["fonts"]) > 32 << 20:
        raise ValueError("PDF font aggregate exceeds its bound")
    return {
        "directory": directory, "manifest": manifest,
        "digest": hashlib.sha256(raw).hexdigest(),
    }

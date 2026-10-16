#!/usr/bin/env python3
"""Read-only, no-network input inventory for the held CFF1 pilot.

Module h1 is deliberately NOT claimed here: Go's offline zip hash check and
exact imported-package closure are separate required gates before a build.
"""

import argparse
import base64
import hashlib
import io
import json
import os
from pathlib import Path
import re
import stat
import zipfile

ROOT = Path(__file__).resolve().parent
STAGE = re.compile(r"pdf-inputs-[0-9a-f]{32}\Z")


def refuse(condition, message):
    if not condition:
        raise ValueError(message)


def regular(root, relative, maximum, expected_size=None, expected_sha=None):
    rel = Path(relative)
    refuse(not rel.is_absolute() and ".." not in rel.parts and len(rel.parts) <= 5, "manifest input path is unsafe")
    path = root
    for part in rel.parts:
        path = path / part
        item = path.lstat()
        refuse(not stat.S_ISLNK(item.st_mode), "input path contains a link")
    digest = hashlib.sha256()
    flags = os.O_RDONLY | getattr(os, "O_NOFOLLOW", 0) | getattr(os, "O_CLOEXEC", 0)
    with os.fdopen(os.open(path, flags), "rb") as handle:
        item = os.fstat(handle.fileno())
        refuse(stat.S_ISREG(item.st_mode) and 0 < item.st_size <= maximum, "input is absent, nonregular or oversized")
        if expected_size is not None:
            refuse(item.st_size == expected_size, "input byte count differs")
        total = 0
        while block := handle.read(min(1 << 20, maximum - total + 1)):
            total += len(block)
            refuse(total <= maximum, "input grew beyond read bound")
            digest.update(block)
        after = os.fstat(handle.fileno())
        refuse(total == item.st_size == after.st_size and (item.st_dev, item.st_ino) == (after.st_dev, after.st_ino), "input changed during read")
    actual = digest.hexdigest()
    if expected_sha is not None:
        refuse(actual == expected_sha, "input original-byte digest differs")
    return {"input": relative, "bytes": total, "sha256": actual}


def module_zip_h1(root, module, maximum, expanded_maximum, entry_maximum, raw_sha):
    """Go dirhash.HashZip wire format, with explicit compressed/expanded caps.

    This validates original module zip contents; it does not resolve imports.
    The caller already verified the zip's first bounded read with regular().
    """
    path = root / module["input"]
    with path.open("rb") as handle:
        raw = handle.read(maximum + 1)
    refuse(0 < len(raw) <= maximum and hashlib.sha256(raw).hexdigest() == raw_sha, "module zip changed after original-byte check")
    prefix = module["name"] + "@" + module["version"] + "/"
    hashes = {}
    expanded = 0
    try:
        with zipfile.ZipFile(io.BytesIO(raw)) as archive:
            entries = archive.infolist()
            refuse(0 < len(entries) <= entry_maximum, "module zip entry count outside bound")
            for entry in entries:
                name = entry.filename
                relative = name[len(prefix):] if name.startswith(prefix) else ""
                parts = relative.split("/")
                mode = entry.external_attr >> 16
                refuse(relative and len(name.encode("utf-8")) <= 512 and not name.endswith("/")
                       and all(part not in ("", ".", "..") for part in parts)
                       and "\\" not in name and "\x00" not in name and name not in hashes,
                       "module zip name is duplicate or outside exact module")
                refuse((stat.S_IFMT(mode) == 0 or stat.S_ISREG(mode)) and not (entry.flag_bits & 1), "module zip contains nonfile or encrypted entry")
                refuse(0 <= entry.file_size <= expanded_maximum - expanded, "module zip expanded bytes exceed bound")
                expanded += entry.file_size
                digest = hashlib.sha256()
                total = 0
                with archive.open(entry) as source:
                    while block := source.read(min(1 << 20, entry.file_size - total + 1)):
                        total += len(block)
                        refuse(total <= entry.file_size, "module zip member grew beyond bound")
                        digest.update(block)
                refuse(total == entry.file_size, "module zip member length changed")
                hashes[name] = digest.hexdigest()
    except (OSError, zipfile.BadZipFile, RuntimeError) as exc:
        raise ValueError("module zip is unavailable or invalid") from exc
    canonical = "".join(f"{hashes[name]}  {name}\n" for name in sorted(hashes)).encode()
    actual = "h1:" + base64.b64encode(hashlib.sha256(canonical).digest()).decode()
    refuse(actual == module["h1"], "module zip Go h1 differs")
    return {"input": module["input"], "h1": actual, "entries": len(hashes), "expandedBytes": expanded}


def module_mod_h1(root, metadata):
    path = root / metadata["input"]
    with path.open("rb") as handle:
        raw = handle.read((1 << 16) + 1)
    refuse(len(raw) == metadata["bytes"] and len(raw) <= 1 << 16
           and hashlib.sha256(raw).hexdigest() == metadata["sha256"],
           "module go.mod changed after original-byte check")
    canonical = f"{hashlib.sha256(raw).hexdigest()}  go.mod\n".encode()
    return "h1:" + base64.b64encode(hashlib.sha256(canonical).digest()).decode()


def preflight(inputs, platform):
    manifest = json.loads((ROOT / "inputs-v1.json").read_text(encoding="utf-8"))
    refuse(manifest["version"] == 1 and platform in manifest["toolchains"], "unknown pilot manifest/platform")
    refuse(inputs.is_absolute() and STAGE.fullmatch(inputs.name) is not None, "input stage identity differs")
    refuse(stat.S_ISDIR(inputs.lstat().st_mode) and not inputs.is_symlink(), "input stage missing or linked")
    observed = []
    go = manifest["toolchains"][platform]
    observed.append(regular(inputs, go["input"], go["bytes"], go["bytes"], go["sha256"]))
    package = manifest["debianPackage"]
    observed.append(regular(inputs, package["input"], package["bytes"], package["bytes"]))
    observed.append(regular(inputs, package["fontInput"], manifest["fontInputMaxBytes"], package["fontBytes"], package["fontSHA256"]))
    observed.append(regular(inputs, package["copyrightInput"], 1 << 20, expected_sha=package["copyrightSHA256"]))
    total = 0
    expanded_total = 0
    modules = set()
    module_hashes = []
    for module in manifest["modules"]:
        key = (module["name"], module["version"])
        refuse(key not in modules and module["url"].startswith("https://proxy.golang.org/"), "duplicate or unpinned module")
        modules.add(key)
        item = regular(inputs, module["input"], manifest["moduleZipMaxBytes"])
        total += item["bytes"]
        observed.append(item)
        verified = module_zip_h1(inputs, module, manifest["moduleZipMaxBytes"],
                                 manifest["moduleZipExpandedMaxBytes"], manifest["moduleZipEntriesMax"], item["sha256"])
        expanded_total += verified["expandedBytes"]
        refuse(expanded_total <= manifest["moduleZipExpandedTotalMaxBytes"], "all module zips expand beyond bound")
        module_hashes.append(verified)
        for key in ("mod", "info"):
            meta = module[key]
            observed.append(regular(inputs, meta["input"], 1 << 16, meta["bytes"], meta["sha256"]))
        verified["modH1"] = module_mod_h1(inputs, module["mod"])
        for grant in module["grants"]:
            observed.append(regular(inputs, grant["input"], 1 << 20, expected_sha=grant["sha256"]))
    refuse(total <= manifest["moduleZipTotalMaxBytes"], "module input total exceeds bound")
    return {
        "version": 1,
        "status": "input-bytes-and-module-h1-verified; package graph and Debian wrapper relation unverified",
        "platform": platform,
        "moduleCount": len(modules),
        "moduleZipBytes": total,
        "moduleZipExpandedBytes": expanded_total,
        "moduleHashes": module_hashes,
        "inputs": observed,
    }


def main():
    parser = argparse.ArgumentParser()
    parser.add_argument("--inputs", type=Path, required=True)
    parser.add_argument("--platform", choices=("linux-amd64", "windows-amd64"), required=True)
    args = parser.parse_args()
    print(json.dumps(preflight(args.inputs, args.platform), sort_keys=True, separators=(",", ":")))


if __name__ == "__main__":
    main()

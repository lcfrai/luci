#!/usr/bin/env python3
"""Read-only, no-network input inventory for the held CFF1 pilot.

Selected module ZIP and go.mod h1 values are checked here. Exact imported-
package closure is a separate required gate before a build.
"""

import argparse
import base64
from contextlib import contextmanager
import hashlib
import io
import json
import os
from pathlib import Path
import re
import stat
import unicodedata
import zipfile

ROOT = Path(__file__).resolve().parent
STAGE = re.compile(r"pdf-inputs-[0-9a-f]{32}\Z")


def refuse(condition, message):
    if not condition:
        raise ValueError(message)


def same_input_stat(before, after):
    return (stat.S_ISREG(after.st_mode) and os.path.samestat(before, after)
            and before.st_size == after.st_size and before.st_mtime_ns == after.st_mtime_ns
            and before.st_ctime_ns == after.st_ctime_ns)


@contextmanager
def bounded_regular(root, relative, maximum):
    """Open one staged ordinary file, retaining exact identity through the read.

    This does not establish ownership of the input stage. The later contained
    controller must supply that separately. Refuse platforms without the
    required no-follow, nonblocking and directory-relative open primitives.
    """
    refuse(all(hasattr(os, name) for name in ("O_NOFOLLOW", "O_NONBLOCK", "O_DIRECTORY"))
           and os.open in os.supports_dir_fd and os.stat in os.supports_dir_fd,
           "safe staged-input open is unavailable on this platform")
    rel = Path(relative)
    refuse(not rel.is_absolute() and rel.parts and ".." not in rel.parts and len(rel.parts) <= 5,
           "manifest input path is unsafe")
    root = Path(root)
    refuse(root.is_absolute() and ".." not in root.parts, "input stage path is unsafe")
    directory_flags = os.O_RDONLY | os.O_DIRECTORY | os.O_NOFOLLOW | os.O_NONBLOCK | getattr(os, "O_CLOEXEC", 0)
    file_flags = os.O_RDONLY | os.O_NOFOLLOW | os.O_NONBLOCK | getattr(os, "O_CLOEXEC", 0)
    descriptors = []
    try:
        directory = os.open(root.anchor, directory_flags)
        descriptors.append(directory)
        for part in (*root.parts[1:], *rel.parts[:-1]):
            named = os.stat(part, dir_fd=directory, follow_symlinks=False)
            refuse(stat.S_ISDIR(named.st_mode), "input path contains a link or nondirectory")
            directory = os.open(part, directory_flags, dir_fd=directory)
            descriptors.append(directory)
            refuse(os.path.samestat(named, os.fstat(directory)), "input directory changed before open")
        leaf = rel.parts[-1]
        named = os.stat(leaf, dir_fd=directory, follow_symlinks=False)
        refuse(stat.S_ISREG(named.st_mode) and 0 < named.st_size <= maximum,
               "input is absent, linked, nonregular or oversized")
        fd = os.open(leaf, file_flags, dir_fd=directory)
        descriptors.append(fd)
        opened = os.fstat(fd)
        refuse(same_input_stat(named, opened), "input changed before bounded read")
        yield fd, opened
        after = os.fstat(fd)
        current = os.stat(leaf, dir_fd=directory, follow_symlinks=False)
        refuse(same_input_stat(opened, after) and same_input_stat(opened, current),
               "input changed during read")
    finally:
        for fd in reversed(descriptors):
            os.close(fd)


def regular(root, relative, maximum, expected_size=None, expected_sha=None):
    digest = hashlib.sha256()
    with bounded_regular(root, relative, maximum) as (fd, item):
        if expected_size is not None:
            refuse(item.st_size == expected_size, "input byte count differs")
        total = 0
        while block := os.read(fd, min(1 << 20, maximum - total + 1)):
            total += len(block)
            refuse(total <= maximum, "input grew beyond read bound")
            digest.update(block)
        refuse(total == item.st_size, "input changed during read")
    actual = digest.hexdigest()
    if expected_sha is not None:
        refuse(actual == expected_sha, "input original-byte digest differs")
    return {"input": relative, "bytes": total, "sha256": actual}


def read_bounded_fd(fd, maximum):
    chunks = []
    total = 0
    while block := os.read(fd, min(1 << 20, maximum - total + 1)):
        total += len(block)
        refuse(total <= maximum, "staged input grew beyond read bound")
        chunks.append(block)
    return b"".join(chunks)


def module_zip_h1(root, module, maximum, expanded_maximum, entry_maximum, raw_sha):
    """Go dirhash.HashZip wire format, with explicit compressed/expanded caps.

    This validates original module zip contents; it does not resolve imports.
    The caller already verified the zip's first bounded read with regular().
    """
    with bounded_regular(root, module["input"], maximum) as (fd, item):
        raw = read_bounded_fd(fd, maximum)
        refuse(len(raw) == item.st_size, "module zip changed during bounded read")
    refuse(0 < len(raw) <= maximum and hashlib.sha256(raw).hexdigest() == raw_sha, "module zip changed after original-byte check")
    prefix = module["name"] + "@" + module["version"] + "/"
    hashes = {}
    expanded = 0
    try:
        with zipfile.ZipFile(io.BytesIO(raw)) as archive:
            entries = archive.infolist()
            refuse(0 < len(entries) <= entry_maximum, "module zip entry count outside bound")
            for entry in entries:
                refuse(entry.orig_filename == entry.filename,
                       "module zip original name differs from decoded filename")
                name = entry.filename
                relative = name[len(prefix):] if name.startswith(prefix) else ""
                parts = relative.split("/")
                mode = entry.external_attr >> 16
                refuse(relative and len(name.encode("utf-8")) <= 512 and not name.endswith("/")
                       and all(part not in ("", ".", "..") for part in parts)
                       and "\\" not in name and not any(unicodedata.category(c) == "Cc" for c in name)
                       and name not in hashes,
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
    with bounded_regular(root, metadata["input"], 1 << 16) as (fd, item):
        raw = read_bounded_fd(fd, 1 << 16)
        refuse(len(raw) == item.st_size, "module go.mod changed during bounded read")
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
    observed.append(regular(inputs, package["input"], package["bytes"], package["bytes"], package["sha256"]))
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

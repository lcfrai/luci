#!/usr/bin/env python3
"""Read-only, no-network input inventory for the held CFF1 pilot.

Module h1 is deliberately NOT claimed here: Go's offline zip hash check and
exact imported-package closure are separate required gates before a build.
"""

import argparse
import hashlib
import json
import os
from pathlib import Path
import re
import stat

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
    item = path.stat()
    refuse(stat.S_ISREG(item.st_mode) and 0 < item.st_size <= maximum, "input is absent, nonregular or oversized")
    if expected_size is not None:
        refuse(item.st_size == expected_size, "input byte count differs")
    digest = hashlib.sha256()
    with path.open("rb") as handle:
        while block := handle.read(1 << 20):
            digest.update(block)
    actual = digest.hexdigest()
    if expected_sha is not None:
        refuse(actual == expected_sha, "input original-byte digest differs")
    return {"input": relative, "bytes": item.st_size, "sha256": actual}


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
    modules = set()
    for module in manifest["modules"]:
        key = (module["name"], module["version"])
        refuse(key not in modules and module["url"].startswith("https://proxy.golang.org/"), "duplicate or unpinned module")
        modules.add(key)
        item = regular(inputs, module["input"], manifest["moduleZipMaxBytes"])
        total += item["bytes"]
        observed.append(item)
        for key in ("mod", "info"):
            meta = module[key]
            observed.append(regular(inputs, meta["input"], 1 << 16, meta["bytes"], meta["sha256"]))
        for grant in module["grants"]:
            observed.append(regular(inputs, grant["input"], 1 << 20, expected_sha=grant["sha256"]))
    refuse(total <= manifest["moduleZipTotalMaxBytes"], "module input total exceeds bound")
    return {
        "version": 1,
        "status": "input-bytes-only; module h1 and package graph unverified",
        "platform": platform,
        "moduleCount": len(modules),
        "moduleZipBytes": total,
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

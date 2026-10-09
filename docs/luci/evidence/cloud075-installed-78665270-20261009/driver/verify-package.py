#!/usr/bin/env python3
"""Verify an installed package against its clean-build receipt and original archive."""
import hashlib
import json
from pathlib import Path, PurePosixPath
import sys
import tarfile


def digest(path):
    h = hashlib.sha256()
    with open(path, "rb") as stream:
        while block := stream.read(1024 * 1024):
            h.update(block)
    return h.hexdigest()


root, receipt_path, expected_source = Path(sys.argv[1]).resolve(), Path(sys.argv[2]), sys.argv[3]
receipt = json.loads(receipt_path.read_text())
assert receipt["sourceDirty"] is False
assert receipt["sourceCommit"] == expected_source
assert len(expected_source) == 40 and all(c in "0123456789abcdef" for c in expected_source)
archive = Path(receipt["archive"])
assert digest(archive) == receipt["archiveSHA256"]
assert digest(root / "luci") == receipt["binarySHA256"]
assert digest(root / "web/index.html") == receipt["webIndexSHA256"]
count = 0
with tarfile.open(archive, "r:gz") as source:
    for row in source:
        name = PurePosixPath(row.name)
        assert not name.is_absolute() and ".." not in name.parts
        if row.isdir():
            continue
        assert row.isfile(), "Only regular package members are accepted"
        target = root.joinpath(*name.parts)
        assert target.is_file() and not target.is_symlink()
        assert target.resolve().is_relative_to(root)
        h = hashlib.sha256()
        with source.extractfile(row) as stream:
            while block := stream.read(1024 * 1024):
                h.update(block)
        assert digest(target) == h.hexdigest(), "Installed package member differs: " + str(name)
        count += 1
print(json.dumps({"sourceCommit": expected_source, "archiveSHA256": receipt["archiveSHA256"],
                  "verifiedMembers": count, "allMembersMatch": True}))

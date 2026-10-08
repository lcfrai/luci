"""Verify Luci's pinned shadcn adaptation and its original MIT/source snapshots."""
from __future__ import annotations

import hashlib
import json
from pathlib import Path
import re

SHADCN_ROOT = "third-party/shadcn-ui/"
SHADCN_MANIFEST = SHADCN_ROOT + "manifest.json"
SHADCN_SOURCE = "desktop/src/components/ui/"
SHADCN_REVISION = "97ddbf4274ed09d02aa7fd34375f2cf1e48cbbf0"
SHADCN_LICENSE_SHA256 = "1564074e13439397221ffd522e2e504d56561994a23d371aa5e3ad43e4f5423f"
SHADCN_UPSTREAM_HASHES = {
    "button": "79dd6f75f8136394442202d6b8b922fb269eaad0a5dba579397c9d5b41f893bb",
    "dialog": "2ade6074e5fad0a4eb171ead44c43d3301a3c83d186d7236fb1fdbea80ee556b",
    "input": "0c9457181f6ddc80969bcf854e92c362903a55b6e889fbef3fd85343ecc4af5b",
    "textarea": "5cafcfeeee29534f449387a16b585f5923d2faab1a0cdc50a874416b175f59ce",
    "badge": "13a9380dfff853ee730c14a52df49f22aecd31f40f2998d7c1e7ec458b6e1650",
}
SHADCN_METADATA = {SHADCN_MANIFEST, SHADCN_ROOT + "LICENSE", SHADCN_ROOT + "README.md",
                   *(SHADCN_ROOT + "upstream/" + name + ".tsx" for name in SHADCN_UPSTREAM_HASHES)}
SHADCN_COMPONENTS = {SHADCN_SOURCE + name + ".tsx" for name in SHADCN_UPSTREAM_HASHES}


def verify_shadcn(files: dict[str, bytes]) -> dict:
    if not (SHADCN_METADATA | SHADCN_COMPONENTS) & files.keys():
        return {"present": False, "adaptedSourceFilesVerified": 0}
    try:
        manifest = json.loads(files[SHADCN_MANIFEST])
        if (not isinstance(manifest, dict) or manifest.get("name") != "shadcn/ui"
                or manifest.get("upstream") != "https://github.com/shadcn-ui/ui"
                or manifest.get("revision") != SHADCN_REVISION or manifest.get("license") != "MIT"
                or manifest.get("licenseSha256") != SHADCN_LICENSE_SHA256
                or not isinstance(manifest.get("files"), list) or len(manifest["files"]) != 5):
            raise ValueError("Invalid retained shadcn provenance")
        if hashlib.sha256(files[SHADCN_ROOT + "LICENSE"]).hexdigest() != SHADCN_LICENSE_SHA256:
            raise ValueError("Missing or changed original shadcn MIT grant")
        seen = set()
        for row in manifest["files"]:
            if not isinstance(row, dict) or row.get("path") not in SHADCN_COMPONENTS or row["path"] in seen:
                raise ValueError("Invalid retained shadcn source entry")
            stem = row["path"].removeprefix(SHADCN_SOURCE).removesuffix(".tsx")
            original = SHADCN_ROOT + "upstream/" + stem + ".tsx"
            if (row.get("sourcePath") != original
                    or row.get("upstreamPath") != "apps/v4/registry/new-york-v4/ui/" + stem + ".tsx"
                    or row.get("upstreamSha256") != SHADCN_UPSTREAM_HASHES[stem]
                    or hashlib.sha256(files[original]).hexdigest() != SHADCN_UPSTREAM_HASHES[stem]
                    or not isinstance(row.get("normalizedSha256"), str)
                    or not re.fullmatch(r"[a-f0-9]{64}", row["normalizedSha256"])):
                raise ValueError("Missing or changed original shadcn source: " + stem)
            adapted = files[row["path"]].decode("utf-8").replace("\r\n", "\n").replace("\r", "\n").encode("utf-8")
            if hashlib.sha256(adapted).hexdigest() != row["normalizedSha256"]:
                raise ValueError("Adapted shadcn source differs from its provenance: " + stem)
            seen.add(row["path"])
    except (KeyError, UnicodeError, TypeError, json.JSONDecodeError) as exc:
        raise ValueError("Retained shadcn source requires its complete manifest, original sources and MIT grant") from exc
    return {"present": True, "revision": SHADCN_REVISION, "adaptedSourceFilesVerified": len(seen),
            "originalSourceFilesVerified": len(seen), "licenseSha256": SHADCN_LICENSE_SHA256}


def read_shadcn(root: Path) -> dict[str, bytes]:
    """Read only exact known files. Refuse linked ancestors before following them."""
    result = {}
    for name in sorted(SHADCN_METADATA | SHADCN_COMPONENTS):
        path = root
        for part in name.split("/"):
            path = path / part
            if path.is_symlink():
                raise ValueError("Linked retained shadcn source: " + name)
        if path.exists():
            if not path.is_file() or path.stat().st_size > 1024 * 1024:
                raise ValueError("Invalid retained shadcn file: " + name)
            result[name] = path.read_bytes()
    return result

"""Verify the narrowly copied E2B wire bindings before packaging their grant."""
from __future__ import annotations

import hashlib
from pathlib import Path

E2B_REVISION = "cf536c284434fdd2f09bbab35ec807a5dacc87e4"
E2B_ROOT = "control-plane/internal/backend/e2bbe/envdprocess"
_UPSTREAM_IMPORT = b"github.com/e2b-dev/infra/packages/shared/pkg/grpc/envd/process"
_LOCAL_IMPORT = b"github.com/lcfrai/luci/control-plane/internal/backend/e2bbe/envdprocess"
_ORIGINALS = {
    "process.pb.go": "f385b69c66e254ecc6d339fbe26bcf20459b60fb249a34465ca8834d39b5ea85",
    "process.proto": "8edd9358c7dbfcad96796b3f0ed8d14c262b8b14d6bc7d5e84d468941511b8e0",
    "processconnect/process.connect.go": "d6e845cf159047d740cd980dcf38c584d4d6e5de840271310a4d5353605f7097",
    "LICENSE": "b4ef1bf811cb4095229fb86b574199e467c78f0ac4078cf8be62189e1fbd0818",
}


def verify_e2b(root: Path) -> dict:
    root = root.resolve()
    base = root / E2B_ROOT
    cursor = root
    for part in Path(E2B_ROOT).parts:
        cursor = cursor / part
        if cursor.is_symlink():
            raise ValueError("Linked retained E2B path: " + str(cursor.relative_to(root)))
    if not base.exists():
        return {"present": False}
    for relative, expected in _ORIGINALS.items():
        path = base / relative
        for ancestor in (path, *path.parents):
            if ancestor == base:
                break
            if ancestor.is_symlink():
                raise ValueError("Linked retained E2B source: " + relative)
        if not path.is_file() or path.stat().st_size > 128 << 10:
            raise ValueError("Missing or oversized retained E2B source: " + relative)
        # Windows Git text checkout may convert LF to CRLF. No other
        # normalization or generated-code rewrite is accepted.
        body = path.read_bytes().replace(b"\r\n", b"\n")
        if relative == "processconnect/process.connect.go":
            if body.count(_LOCAL_IMPORT) != 1 or _UPSTREAM_IMPORT in body:
                raise ValueError("Unexpected E2B generated import relocation")
            body = body.replace(_LOCAL_IMPORT, _UPSTREAM_IMPORT)
        if hashlib.sha256(body).hexdigest() != expected:
            raise ValueError("Retained E2B source differs from pinned original: " + relative)
    return {"present": True, "revision": E2B_REVISION, "verifiedFiles": len(_ORIGINALS)}

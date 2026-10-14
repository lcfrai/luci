# Azure builder source bundle v1

This is an **offline source distribution contract**, not proof that an Azure
builder, microVM, provider connection or deployment has run. It lives inside
the installed `azure-setup.tar` as exactly these new members:

- `azure-setup/source_bundle.py`, the pinned producer/restorer;
- `azure-embed-host/guest_registry.py`, the selected host registry helper;
- `builder-source/source.json`, canonical JSON with a final newline;
- `builder-source/luci.pack` and `builder-source/e2b.pack`, Git pack streams.

`source_bundle.collect(luci_repo, luci_revision, e2b_repo, e2b_revision,
output_dir)` writes the three `builder-source/` members to a new directory.
Both repositories must be clean SHA-1 Git checkouts at their named HEAD. E2B
must be pinned at `cf536c284434fdd2f09bbab35ec807a5dacc87e4`. The normal
installed path uses the embedded packs; it does not contact GitHub or require
the user's credentials or source checkout. An exact-SHA clone remains an
explicit, separately selected fallback.

The manifest has exactly `version: 1`,
`kind: "luci-azure-builder-source"`, and `luci` and `e2b` role objects. Each
role contains:

| Field | Binding |
| --- | --- |
| `revision` | Original 40-character lowercase Git commit ID |
| `tree` | Original root tree ID from those commit bytes |
| `pack.sha256`, `pack.bytes` | Exact compressed pack stream |
| `objectCount`, `expandedBytes` | Exact selected Git object count and total decompressed object bytes |
| `paths[]` | Sorted selected `{path, mode, blob, bytes}` from the original tree |

The pack contains the **original commit object, all original tree objects, and
only the selected original blobs**. It does not relabel, rewrite or synthesize
the commit. Luci selection: `LICENSE`, `NOTICE`, `go.work`, `go.work.sum`, all
of `control-plane/`, `sandboxd/`, `images/microvm/`, `images/sandbox/`,
`scripts/azure-controller-disk/`, `scripts/azure-embed-host/` (including
`guest_registry.py` once committed),
`third-party/licenses/` (all source-bound original grant bytes, including
the current Codex and any approved exact-version npm/OpenCode fallbacks), plus
`scripts/runtime-oci/{image_licenses.py,grant_fallbacks.py,archive.py,common.py}`.
The sandboxd selection retains vendored licence files and `vendor/modules.txt`.
E2B selection: original `LICENSE` and `embed/compose/` at the pinned revision.
The original Node, Go, Debian and Codex licence texts remain intact; the
source pack grants no authority to use cloud credentials or resources.

`source_bundle.restore(bundle_root, target_root)` checks the exact manifest,
pack hashes, pack object set/types/sizes, original commit/root tree and all
selected path/mode/blob bindings. It creates clean sparse `luci/` and `e2b/`
Git checkouts under a previously absent target, with the original HEAD and
unselected index entries marked skip-worktree. It writes selected blobs
directly, with no Git filters, hooks, alternates or implicit fetch. The caller
must separately retain the source manifest/outer bundle digest in its
builder intent and preserve unknown external effects.

Limits: installed outer bundle 8 MiB, each source entry/pack 4 MiB, manifest
2 MiB, at most 2,048 selected paths and 8,192 objects per role, 64 MiB total
expanded object bytes, 16 MiB per selected file, 240 characters per safe path,
120 seconds per Git subprocess. The outer bundle manifest and Go product
reader bind every member's exact SHA256/size and the Luci/E2B revisions. The
guest restorer rejects duplicate, linked, out-of-scope, missing, extra and
drifted objects or paths. These limits are above the measured 237f3a73 Luci
selection (1,215 blobs, 3,590 objects and 3,402,964 packed bytes), while
keeping source restoration bounded.

Azure setup's `package.py`/`installer.py`, guest image staging and portable or
native release staging must add these exact members before the normal BYO
flow uses them. The installer/guest integration and Envoy provenance are owned
separately. This contract alone is not a finished setup route.

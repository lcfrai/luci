# Linux package — edc742e8

The official command `bash scripts/build-release.sh 0.5.0 linux-amd64` passed against clean, detached source **`edc742e8b6ce4216a1726e74cceef892663ce3d5`** in a new isolated plain shared clone. Static archive verification and safe extraction passed. The binary was **not executed** by this packaging task, and no Docker operation ran.

| Distribution | Bytes | SHA256 |
| --- | ---: | --- |
| Linux desktop | 69,588,055 | `d7c1df74a402c781cb19ac422f6448ca8ddc741709cbba33db82cf339cf19ba5` |
| Linux cloud | 34,887,013 | `6645ceb915c11d4bd75d3f0c7e9ada49b74ca76446ca333378fd443df6eb4ecc` |

Both archives contain binary SHA256 `7a370a2a67e836564f053b4991d8ee5b30acedfbeba8ea9f9e5ef41c533408a2`, web index SHA256 `b5522b6f687ae04a8598cc22a14b9e4332577bbe31d38dac1f3bee2d80989a32`, and runtime package identity `778a3f34b2a7a181e7dec7bfdfddd909c5f78b385f501d1510ac53b02b0383c3`. The desktop archive contains the exact cloud archive. The verifier checked every member hash, type, mode, ownership, safe path, license/source reference, package identity and nested archive. **347 license components**, **374 license/source files**, and **zero missing licenses** were verified. Guides use the official compact staging mode with immutable source links; archive member inventories retain every exact shipped byte binding.

[Build invocation](original/receipts/build-invocation.json), [build result](original/receipts/build-result.json), original stdout/stderr and [static verification](original/verification/verification.json) are retained. The official source scripts are frozen here. The wrapper uses `GOMAXPROCS=2`, `-p=1`, the existing activated Go caches, an ignored dependency symlink after package/lockfile parity, and isolated temporary/output directories. It performs no dependency installation. It records a 30-minute build deadline, 15-second termination grace and packaging-only stop floors of 1.5 GiB root / 384 MiB temporary storage. Product runtime admission floors are unchanged. The reused dependency/cache bytes are **not** claimed to have a complete independently verified installed-byte inventory.

All **13,216 tracked input hashes** remained unchanged before and after the official build; source stayed clean. Earlier 577 and 8046 archive hashes/sizes remained unchanged. Source bindings are exact losslessly compressed originals, not selected-file substitutes. The official binary contains embedded `vcs.revision=edc742e8b6ce4216a1726e74cceef892663ce3d5` and `vcs.modified=false`; these exact fields passed static verification. This differs from the older 8046 worktree build that lacked embedded VCS fields. Source identity is additionally bound by the clean pinned invocation, source hash bindings, release manifest and archive/binary identities.

The archive bytes and extracted binary remain in the private execution workspace, not in this public capsule. Source at the pinned revision is retrievable and the official packaging script can rebuild it with the documented toolchains. This is Linux packaging evidence only: Windows, live cancellation, interrupted transport, provider quality and complete product acceptance remain separate.

## Exact evidence retrieval

[artifact-map.json](artifact-map.json) maps each original receipt/helper to its published bytes. Large originals are deterministic gzip objects; decompressing one must reproduce its declared original SHA256 and byte count. Equal originals share a single object. [publication-verification.json](publication-verification.json) records exact copy/decompression checks. [manifest.json](manifest.json) and `SHA256SUMS` cover all published files. Historical execution paths in original receipts are provenance, not commands that should be run on another person's machine.

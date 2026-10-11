# Linux package — 80462898

The official Linux packaging command completed against clean, detached source **`80462898735a191cceaa4cb33e486b42b0a35f3c`**, version **0.5.0**. Static package verification passed. This capsule belongs to the [80462898 test checkpoint](../../TEST-CHECKPOINT-80462898.md); actual preview execution is separate evidence.

The command was `bash scripts/build-release.sh 0.5.0 linux-amd64`, run from the isolated pinned checkout. [Exact first invocation](original/receipts/build-invocation.json) and [compact invocation](original/receipts/compact/build-invocation.json) retain the environment overrides, timestamps and disk floors. The [original wrapper](original/run-build.py), [compact wrapper](original/run-compact-build.py), [official script](official-source/scripts/build-release.sh), and [official source bindings](official-source-bindings.json) are included. Go used `GOMAXPROCS=2`, `-p=1`, task-owned build/temp directories and the existing module cache. The ignored desktop dependency symlink was reused after package/lockfile byte parity; there was no dependency installation or lockfile edit.

## Two preserved build outcomes

| Official invocation | Result | Documentation | Reason |
| --- | --- | --- | --- |
| [First build](original/receipts/build-result.json) | Exit 0 | Full | Cached official branch tracking still referred to the earlier source. The official staging rule conservatively retained full documentation and evidence. |
| [Compact build](original/receipts/compact/build-result.json) | Exit 0 | Compact | After preserving the first outputs unchanged, an authorized fetch refreshed the actual published branch association. The official script ran again against the same pinned source. |

The [fetch receipt](original/receipts/tracking-fetch.json), [documentation decision before](original/receipts/documentation-preflight.json) and [after](original/receipts/documentation-after-fetch.json), complete build stdout/stderr, disk samples and original outcome receipts are retained. The compact build reused compiled outputs. Its shorter duration is not product performance evidence.

| Distribution | Archive bytes | SHA256 |
| --- | ---: | --- |
| Compact cloud | 34,880,559 | `34f9951f1e7b07e1dc866b1a1ba65c0a8630b695cb84b8079dbadc7112563c67` |
| Compact desktop | 69,559,318 | `5307c88d5b1aeec6d1394cffcc8fae2639403878b3fcb43f5519dbe96a928ca1` |
| Preserved full cloud | 150,861,087 | `04b272051418b75c61db65ba6dbd80a0728ca668b2dc0f723c5371570a94c0ab` |
| Preserved full desktop | 301,382,429 | `8db42bf43e3224eac00940fe851bec4bb405be057133be62e4b2623f27729779` |

Binary archives stay in the private execution workspace; they are not part of this capsule. Both distributions contain binary SHA256 `b3a971589a0097ca08eef3c409f016617fc5f2590f79a7c5b673a2654f0a0f3d` and runtime package identity `4dccaf0d141ee12da9523bdb00affc3f2517dcb8e45652625e3502a8bfb2cf0c`.

## Verification and its limit

[Compact verification](original/verification-compact-final/verification.json) and [preserved full verification](original/verification-full-doc-final/verification.json) passed official archive hash/size/manifest checks, safe paths and modes, package identity, matching binaries, nested cloud archive equality, guide mode and license checks. Both retained **347 license components**, **374 verified license/source files**, and **zero missing licenses**. Exact shipped guide receipts and license manifests are retained. Compact archives contain no evidence members; the preserved full archives contain 12,672.

The [compact installation receipt](original/verification-compact-final/verification.json) records extraction into a new directory after a second complete archive read matched the earlier member inventory and captured metadata. [The verifier](original/verify-package-build.py) preserves that extraction check. The installed [package-identity bytes](original/installed/package-identity) and every member's hash, size and mode are bound here. No installed binary was executed by this packaging task.

**The official Go binary contains no embedded VCS revision fields.** The first external verifier mistakenly required those optional fields and failed for both distributions: [compact failure](original/verification-compact/verification.json), [full failure](original/verification-full-doc/verification.json). Its [exact original source](original/verify-package-build-original-vcs-assumption.py), static build-info stdout and all preceding verification outputs remain included. The [narrow correction](verifier-correction.patch) keeps Linux/amd64/CGO checks, rejects conflicting VCS fields if present and explicitly records absent fields. Both distributions then passed static verification. There was no rebuild for this verifier correction. Source attribution comes from the official invocation, clean pinned source bindings, release manifest and archive/binary hashes; it is not an embedded-VCS attestation.

All **12,677 tracked input hashes** remained unchanged before and after both builds. The four exact source-binding documents have identical bytes and share [one compressed object](objects/ef3c726ba25028fef5eeb3559475a49e217d3d4d3b49dfe3a0b8464a754061d1.json.gz). Their original names, sizes and SHA256 are in the artifact map. These bindings cover tracked source, not a complete byte inventory of the reused dependency/module caches. [Source preservation](original/receipts/source-preservation.json), [original 577 archive hashes before](original/receipts/previous-release-before.json) and [after](original/receipts/previous-release-after.json) confirm the earlier release was unchanged. The [task-cache cleanup receipt](original/receipts/build-cache-cleanup.json) records removal of 699,517,073 disposable file bytes; it is an environment record, not a product outcome.

## Exact evidence retrieval

[artifact-map.json](artifact-map.json) maps **95 logical original files** to retained bytes. Small originals remain under `original/`. Large JSON documents use deterministic gzip under `objects/`; equal originals share one object. Decompressing produces the exact original byte sequence, SHA256 and size recorded in the map. The map includes all archive member inventories from the failed and successful verification attempts, complete license manifests and all four before/after source bindings.

For example, from this capsule directory:

```sh
gzip -dc objects/ef3c726ba25028fef5eeb3559475a49e217d3d4d3b49dfe3a0b8464a754061d1.json.gz > /tmp/package-80462898-tracked-inputs.json
sha256sum /tmp/package-80462898-tracked-inputs.json
```

The expected decompressed SHA256 is the object's filename prefix. [manifest.json](manifest.json) declares every published file and byte hash; `SHA256SUMS` covers it and the selected files. [publication-verification.json](publication-verification.json) records lossless copy/decompression checks. Original receipts and historical helpers retain their execution-workspace paths; they are provenance, not portable replay commands. Rebuilding requires the pinned source and its documented dependencies.

No new build, test, Docker operation or runtime capture was performed to publish this capsule. It does not establish cold-image provisioning, Windows behavior, generated application quality or completion of the product gates.

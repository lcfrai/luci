# Runtime OCI distribution contract v1

Status: **producer/importer agreement proposed; no Docker effect is implemented by this document.** Controlling decision: [LOCAL-RUNTIME-DISTRIBUTION-RECONCILIATION-2026-10-14.md](../LOCAL-RUNTIME-DISTRIBUTION-RECONCILIATION-2026-10-14.md). Formal shape: [runtime-oci-manifest-v1.schema.json](runtime-oci-manifest-v1.schema.json). Shared [metadata fixtures](../fixtures/runtime-oci-v1) are structural examples, not real image or source evidence.

The clean release package carries `runtime-oci-manifest-v1.json`, `runtime-image-linux-amd64.tar` and `runtime-build-receipt.json` as separate regular files under its runtime directory. The archive must be a **single-image tagless Docker-save tar**. The source-bound, hash-verified installed package, not an HTTP request or user-selected path, supplies the manifest and archive to the API. A missing/mismatched asset blocks ordinary installation; it never selects the local compilation fallback. The fallback remains an explicit advanced choice under the old limits and journal.

## Required cross-field and byte checks

1. Reject metadata over 1 MiB, duplicate JSON keys, unknown fields, noncanonical digests, invalid UTF-8 and target other than Linux/amd64. Require `sourceCommit` to equal the installed clean source receipt, `runtimeContextDigest` to equal the digest recomputed from the current bundled runtime context, and `sourceFiles` to list **every** opened runtime-context regular file exactly once with matching bytes/hash. `producer.dockerfileSHA256` equals the source-file entry for `images/sandbox/Dockerfile`; `producer.buildReceiptSHA256` equals the bounded sibling receipt. The receipt binds the exact image ID, source revision, input inventory and producer build command/result. Receipt and manifest are provenance claims until their package/producer verification succeeds.
2. Before any Docker mutation, inspect each path as a regular, nonlinked file in the selected installed package root. Stream the archive's exact `bytes` and SHA-256 with a hard 8 GiB cap and no full-buffer read. Scan tar headers with bounded member count and path size; reject links, devices, absolute/traversal paths, duplicate names, nonregular entries, unexpected members or mismatched member lengths/hashes. `archive.members` is the complete sorted member inventory. No `repositories` mapping or any `RepoTags` value other than absent/null/empty is accepted in the tar. The canonical manifest `image.repoTags` is always `[]`.
3. The tar's one `manifest.json` entry names exactly `image.configPath` and `image.layerPaths` in order. The config file's SHA-256 equals the hex part of `image.id`; each layer tar matches its member digest, and each uncompressed layer digest matches the corresponding `image.diffIDs` and config `rootfs.diff_ids`. All paths are distinct safe relative paths. The config OS/architecture, user, working directory, entrypoint, exposed ports, absence of volumes/OnBuild and declared CLI versions match the manifest and pinned release policy. The version fields alone do not prove actual CLI execution; producer and installed harness probes remain required evidence.
4. `licenses` is an exact path/hash/length inventory of the packaged notices; preserve every applicable upstream licence and its source/provenance. No manifest field grants execution authority. The current actor/policy, engine/instance, exact allocation and fresh whole-machine capacity are checked again before each Docker effect.

## Effect ownership

The importer journals **archive import**, **small allocation-derived image creation**, and **tag publication** as separate exact effects. `ImageLoad` receives only the preverified tagless stream. A lost load response may reconcile by exact immutable ID/config readback; no response, missing image or Docker timeout alone settles it. The shared payload is never deleted as an allocation-owned image. A tiny derivative from the immutable imported ID adds the current allocation/job/scope/source labels without a Go, apt or npm RUN. Only that derivative may become the configured runtime tag after reauthorization and exact readback. Cancellation/restart retain uncertain effects and capacity charges until positive settlement or supported engine-retirement evidence. Docker tags have no CAS; refuse a different observed tag and keep the existing engine-wide publication lock.

`image.id` is the immutable **shared payload** configuration ID. The derivative has a different image ID and exact allocation labels. Import demand is separately measured and admitted; do not reuse the former 1 GiB RUN demand or lower the host reserve to pass. A fresh observer monitors actual host capacity throughout and cancels only owned effects. Installed proof must include no local heavy RUN, host/WSL memory and responsiveness, a real upstream Docker task and cleanup; metadata fixtures alone do not close LOCAL-033.

## Fixture expectations

| Fixture | Expected result |
| --- | --- |
| `valid-structural.json` | Schema and cross-field metadata shape pass with synthetic bytes; not a real archive pass. |
| `refuse-repository-tag.json` | Schema refusal: manifest attempts a mutable repository tag. |
| `refuse-wrong-source.json` | Semantic refusal against the installed clean source revision. |
| `refuse-unsafe-member.json` | Semantic refusal: parent-directory tar member path. |
| `refuse-config-mismatch.json` | Semantic refusal: config path does not name the configuration ID. |
| `refuse-oversized-archive.json` | Schema refusal: archive exceeds the hard byte cap. |

The producer and importer owners must ACK this versioned contract or commit a compatible revision **before** effect code and release assets are built. Fixture paths are shared test inputs; later byte-level tests must construct actual small tar streams and exercise loss/cancel/restart without Docker mutation on the development host.

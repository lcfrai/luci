# Clean Linux package at 0dcafb14

**Packaging and compact-documentation verification passed at `0dcafb14da7d1f7e2eedaf70366f1e3cf4f4303f`. This is not an integrated local acceptance handoff.** The run owner reports that sustained attempt 6 subsequently failed while waiting for project B, after three natural worker completions and two integrations; that separate workload's full evidence is outside this package capsule. It does not pass because the package is valid. Windows/native execution, real-provider quality and sustained four-project acceptance remain separate gates.

## Exact artifacts

| Artifact | Bytes | SHA256 |
| --- | ---: | --- |
| `luci-linux-amd64.tar.gz` | 69,000,975 | `501a29e3c920173b99041797f794564b433e9d9b6c45036509165a4092a00b25` |
| `luci-cloud-linux-amd64.tar.gz` | 34,595,815 | `0fd56b73af03bcf074de420c631068f2e1519757bb6d9fced499df0f49a200c6` |

The original [package provenance](receipts/package-provenance.json) binds API binary `684e1837d39aa526dd254c933af1d17cbf48a451aad59d00cc33423bf789ea5e` and web index `c9637481e649b1689961417f3e93293cc3995481bb0d878776e396f2b6acdcc5`. The recomputed portable identity is `78fab8a4b401432ab3ebfdf3c9d4b4cc3258ec766b5818c8a8c7f3435ee9cb79`.

The [build result](receipts/build-result.json) records the documented Linux release command, bounded build concurrency, clean source before/after, unchanged HEAD, exit zero and 228.42 seconds. Its [raw log](receipts/build.log) is retained, including warnings. The initial [invocation](receipts/build-invocation.json) says running because it predates the final result; it is not an unresolved build status.

The [original member verifier](helpers/verify-local-package.py) and [integrity result](receipts/package-integrity.json) record **953 extracted files matching archive members**, **347 licence components**, and zero missing licences. An [additive verifier](helpers/verify-compact-local-package.py) records:

- Complete portable archive/extracted-file equality, including modes, and equality of the embedded cloud archive with the separately hashed archive; **952 cloud files**.
- **104 runtime source files** matched to the exact Git revision, complete guide inventory and validation archive omission.
- **44 guide files**, **203 new hosted URLs** resolving to the correct commit/object/type, and unchanged non-Markdown licence/guide bytes. The staging helper reports **202 rewritten links**; the additional generated validation-index URL accounts for the different URL count.
- Exact licence-manifest/file hashes, original provenance unchanged, and removal of its owned temporary guide stage.

The additive [receipt](receipts/compact-doc-verification.json) explicitly distinguishes independent inventory/hash/URL checks from guide regeneration: regeneration uses the **same committed staging implementation**, so it is not an independent Markdown rewriting proof. Its helper SHA is `cd520022c86ff25695faaaeed5aad512147b91f37342963e1cea2cc3252b8e26`. Cached official remote association is offline provenance, not a live hosted-URL availability check. Heading anchors and native Windows behavior are not established here.

## Preservation and measured size

The [previous-release map](receipts/previous-release-preservation.json) preserves the old 2da release directory, archive target hashes, original provenance and failed fixtures. The [new archive relocation](receipts/archive-relocation.json) records byte-preserving movement to task-owned retained storage while preserving the original provenance paths through links. These are historical operations already performed by the run owner; this publication task did not move or delete anything.

The old 2da portable archive was 134,313,689 bytes; the new 0dc archive is 69,000,975 bytes, a recorded difference of **65,312,714 bytes**. The cloud archive changed from 67,254,189 to 34,595,815 bytes. These are exact sizes for two different revisions, which also include other source changes; they are not a controlled isolated speedup or RAM measurement. The [duplicate web-output retirement receipt](receipts/redundant-dist-retirement.json) preserves the file manifest and verified replacement; it does not assert added host capacity.

## Reproduction and remaining scope

[Reproduction and Windows source-build instructions](REPRODUCE.md) identify the exact retrievable revision and supported commands. No binary/archive payload is copied into Git by this capsule. Rebuilding produces a new artifact identity; archive timestamps and toolchain differences mean the original checksum must not be forced onto a rebuild.

The recorded package verification was performed by the run owner. This publication task copied and hashed its receipts, verifier sources and selected committed packaging inputs; it did **not** rerun package tests, rehash the large archives, execute Luci or perform Windows testing. Both external verifiers are frozen under `helpers/`; selected packaging sources fetched from exact 0dc are under `source/`. This is selected-source provenance, not a complete compiler dependency capture.

[Original bindings](original-bindings.json) bind every copied receipt/log/source byte. Standard task source/artifact paths are intentionally retained for exact provenance and the relocation map; no application state, provider configuration or credential files were read. The [prior source-candidate capsule](../cloud042-package-docs/README.md) remains separate and unchanged. This new receipt advances the Linux packaging scope only; it does not replace failed workload evidence or declare the local milestone finished.

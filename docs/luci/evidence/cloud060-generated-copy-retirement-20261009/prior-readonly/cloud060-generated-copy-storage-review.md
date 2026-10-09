# C060 generated-copy boundaries

Read-only receipt/metadata review, 2026-10-09. No payload hashing, process discovery, private state/log reads, app/test/build/Docker activity or mutation was performed. Details are in `cloud060-generated-copy-storage-review.json`.

All three exact roots below have the same 425 receipt-listed output paths, byte sizes and recorded SHA256 values: `luci` and 424 files under `web/`. Each has 85,812,216 logical payload bytes. Observed files are ordinary, match recorded sizes and have link count one. No output inode is shared among these copies or with `/tmp/luci-retained-tools/package-0.5.0-5f4a966f`; no unlisted non-directory web entries were observed. Allocated-block sums are 86,740,992 / 86,740,992 / 86,745,088 bytes, respectively. These values do not guarantee reclaimed physical space or exclude filesystem sharing/open file references.

- `/workspace/.luci-tools/cloud060-package-reuse-5f4a966f`
- `/workspace/.luci-tools/cloud060-package-reuse-v2-5f4a966f`
- `/workspace/.luci-tools/cloud060-package-reuse-v3-5f4a966f`

The receipts' generic `scope` text says “Actual source build,” but `commands: []` and `reuse.mode: verified-existing-package` expressly describe reuse. No fresh build is inferred. The retained receipt equivalence is not a fresh content hash; root is reviewing those hashes separately.

The proposed candidate boundary is only `luci` and `web` in an exact, confirmed-inactive root above. Preserve `build-receipt.json`, `reuse-summary.json` where present, all cache/home/tmp directories, helper/validation/evidence trees, retained `/tmp` reference package and source release archives. Whole-root deletion would exceed that boundary and risk unrelated generated data or provenance. Do not traverse fixture state/private logs to justify disposal.

V4's documented invocation targets `/workspace/.luci-tools/cloud060-package-reuse-v4-5f4a966f`; this target did not yet exist at the metadata read. That fact is **not** proof that an active v4 launcher is independent of v3: `launch.mjs` obtains its binary and `LUCI_WEB_DIR` from the actual `--artifacts` argument. The live pinned invocation and exact known process identities/open references must determine exclusions. Exclude all v4 helpers, outputs and fixture paths, plus any earlier payload actually used by v4. A path-name/version assumption alone is insufficient.

Payload-only removal leaves historical receipts accurately describing previously captured bytes but makes those original paths unavailable. Record the precise retirement and retained source/package locator instead of rewriting old receipts or calling the run absent. If relocation is selected, preserve file/mode/link metadata and verify copied bytes before switching; refuse unknown destination collisions. A compatibility link must be described as a later operation, not original package provenance. No operation or safety completion is certified here.

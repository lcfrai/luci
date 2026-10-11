# CLOUD-082: installed warm multistage preview on 80462898

The installed Linux fixture passed on product **80462898735a191cceaa4cb33e486b42b0a35f3c**. Through Luci's normal project/review/configuration/preview APIs, it built and served a handwritten Node calculator using a cached immutable Node 22 base. Docker actually executed the first-stage tests and build, the serving allocation returned useful HTTP output, and normal API Stop settled the exact preview reservations and container.

This is **warm-base Linux handwritten-fixture evidence**. It does not verify a cold pull, Windows, live-model output quality, Luci's project-check workflow, the runtime installer, latency, peak resource usage or full product acceptance. The earlier CLOUD-082 source regressions and this installed warm witness have separate scopes.

## Exact identities

- Source: `80462898735a191cceaa4cb33e486b42b0a35f3c`.
- Package identity: `4dccaf0d141ee12da9523bdb00affc3f2517dcb8e45652625e3502a8bfb2cf0c`.
- Installed binary SHA256: `b3a971589a0097ca08eef3c409f016617fc5f2590f79a7c5b673a2654f0a0f3d`.
- Frozen package verification SHA256: `6383014b1869914439cda1710c2f7f0a6fefa55bacc3081a6283ed7c892d666f`, retained exactly in [package-provenance.json](run/package-provenance.json). This capsule does not contain the binary archives.
- Exact helper SHA256: `767db04a94975c16c773c4b4e8cf4bcbf43ac991550bfa9311cd01d5b27f659e`.
- Original private result SHA256: `e12f02022b9e275a54cd1565b5bae73a1520e2c29170d71974caa1ed01af4613`.
- Public path-substituted result SHA256: `5df7654763826b15b646894e037be9905c4907a690eaffebbab40fb27badadc7`.
- Reviewed fixture commit: `1245d3f6463f452e16290089145dea2c5d6faac5`.
- Cached base: `sha256:7c3b093add7c43400ee83b815ab2cda98794a10045bcf76ce9bb2f89b97cbc5c`.
- Built image: `sha256:eb1309aded7dc04bd3f9f5c6ca16fba0e8d8b8f43fc14d1e6e33d04b89dae146`.

## What ran

The wrapper started at 2026-10-11 13:03:48 UTC and exited 0 after 58.267 seconds, with no wrapper timeout or forced helper kill. The fixture's final result is a pass; duration is a run observation, not a throughput or responsiveness comparison.

The exact [Dockerfile](run/reviewed-Dockerfile) uses the same cached immutable base in two stages. The first stage runs ten meaningful decimal arithmetic/input tests, then builds output from the committed source. The second stage copies only that output from the completed prior stage. No host-built app, TAP or output directory entered the source context. A fresh nonce and four source hashes join the reviewed input to the actual [HTTP build proof](run/http-build-proof.json) and its [decoded TAP](derived/build-test-output.tap). Node inside the build was v22.23.3. All ten actual Docker RUN tests passed with no failures, skips or cancellations.

The build proof observed `memory.max=1073741824`, `memory.swap.max=0`, and `cpu.max=100000 100000` inside the actual RUN child. These are one GiB, no swap, and one CPU's quota/period. `pids.max` was `max`: this is preserved literally. The production build uses an `nproc` ulimit, not a build PID cgroup cap; no measured build PID-limit claim is made. Serving inspection separately verifies its 256 PID cgroup limit, one GiB/one CPU, disabled swap, non-root UID 65532, capability drop, no-new-privileges, no mounts and loopback binding.

Normal APIs saved the brief, saved and approved the specification and design, committed the reviewed fixture and applied its project greeting. The wrong expected commit and dirty-source preview requests both returned 409 before any release existed. The exact normal preview POST then returned 201. HTTP returned the calculator form, `12.5 + 7.25 = 19.75`, the configured greeting, UID 65532 and no model key; invalid input returned 400.

[Lease observations](run/build-lease-observations.json) contain 837 read attempts and seven changed snapshots. They show one exact build allocation progressing reserved → building → finished, then the exact serving allocation. At most one active preview row was charged in each captured snapshot. These are 50 ms-target samples, not exhaustive event timestamps or whole-machine accounting. The sampled ledger hashes refer to original private ledger bytes, which are not published here.

The build allocation was `preview-build-ebf92ccfab2358e95121a65f`; serving was `preview-c1e3ac6740cca9891935545a`. Normal project Stop returned 200 with `phase=finished`, `resourcesHeld=false` and `cleanupConfirmed=true`, retaining release history. Exact project lease rows then became empty. No manual Docker removal, ledger repair or global prune occurred. Output image and cache intentionally remain.

## Closure and limits

The helper observed normal controller exit 0. The separate [independent closure](run/independent-closure.json), taken at 13:05:35 UTC, confirms its one known observed controller PID/start identity absent, the exact serving container absent, the same daemon with empty container/volume inventories, and a 25-byte fixture ledger with zero workers/checks/runtime-image/preview entries. The existing base image ID, tags, digests and size remained unchanged. No independent mutation was performed.

**The independent receipt's historical field `nativeBuildTestCount: 10` means Docker RUN test count. It is not native Windows evidence.** The original receipt is kept unchanged. Its process scope is known observed identities only, not a lifetime or whole-machine census. No browser was used in this fixture. Other stopped controller roots and previously retained charged fixtures were not inspected or changed.

The helper has no pull/tag command and used only exact pre-existing image IDs. The receipt does not contain an intercepted Docker request transcript; do not claim independent wire-level zero-ImagePull or Create/Start counts. Actual image/container and lease outcomes are supported by the retained inspections, app proof and selected API receipts. Cgroup limits are not peak CPU/RAM observations or an atomic admission snapshot. The original product source bindings are nine selected anchors, not all compiled dependencies.

## Publication and reproduction

[publication-manifest.json](publication-manifest.json) binds 103 selected original/public files. Files are exact unless the manifest declares only one of two path substitutions: the exact private fixture run directory becomes `PRIVATE_WARM_PREVIEW_RUN`; its evidence-parent prefix becomes `PRIVATE_WARM_PREVIEW_EVIDENCE_PARENT`. Hashes and lengths distinguish original and transformed representations. Original selected files were rehashed after publication and remain unchanged. Auth/bootstrap, account/configuration/session stores, raw controller logs and the entire private directory were excluded; the private tree was not recursively traversed.

The executable [helper](helpers/preview-smoke.mjs), fixture sources, original C080 helper and preparation generator are included. [Actual configuration](helpers/run-config-001.json) preserves the installed identities with the declared fixture evidence-path substitution; it is a historical receipt, not a portable invocation. To reproduce, use a new canonical private evidence directory and a verified matching package, establish the current exact daemon/base identities, and deliberately prepare a new configuration from [the template](helpers/run-config.template.json). Do not overwrite a retained fixture or weaken policy/admission. Follow [the prepared helper instructions](helpers/README.md); their “prepared only” status describes the preparation stage before this executed run.

Preparation tests remain separate: ten host Node 24 fixture tests passed, and nine synthetic exact-helper proof-validator cases passed twice with complete overlap. These are not ten plus ten actual builds or eighteen unique proof cases. The final actual Docker RUN count is ten. The original default spec reporter and subsequent explicit TAP output are retained; no fixture test failure was discarded.

The derived files only validate retained byte/identity joins and extract TAP; no tests or runtime were rerun to publish this capsule. SHA256SUMS covers every capsule file except itself.

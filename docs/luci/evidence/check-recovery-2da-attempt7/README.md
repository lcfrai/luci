# Check recovery attempt 7 — failed before historical command Start

This matching installed Linux attempt used **product/helper `2da1a821b8456c20a858fb86ef87649ea917d9de`**. It **failed** before its initial historical command could start: the daemon acknowledged one Create, then product Start admission refused with `memory_pressure`. Exact cleanup completed, with `resourcesHeld=false`. No target check, lost acknowledgement, recovery review or fence was reached.

Driver timestamps: `2026-10-09T01:41:46.985Z`–`01:42:25.452Z` (**11:41–11:42 a.m. Brisbane**, 9 October). Wrapper exit **1**; failure stage `consent-check-1`, `AssertionError / ERR_ASSERTION`, `recovery-scenario.mjs:131`, which expected the initial historical check to pass. Preserve this failure separately from the later successful [attempt 8](../check-recovery-2da-attempt8/README.md).

## Exact refusal and scope

Check `check-7c853f3f9118e2d4e1638652` has status **blocked / admission / memory_pressure**. Its recorded message says available memory could not preserve configured headroom. Allocation `checkexec-d8e0113096c545b04b93ea90` was actually created in container `d964f78b7753684657f961f91a59c64813d4bb27103f8ba4fcc450537854ca09`, then exactly removed. The terminal command has confirmed cleanup, empty output and no exit code; the check holds no resources.

Transport totals are **1 Create / 0 Starts / 0 fences / 0 workers / 0 external model calls**. The original proxy failure list is empty. Timestamped advisory/phase resources are preserved, but **the exact atomic numeric admission operands are not captured**. Later cgroup or physical-memory readings cannot explain the precise refusal or earlier runs. Guards and thresholds were not relaxed. CLOUD-039's exact terminal-before-gate refusal branch was **not exercised**: the real Create gate had already been captured. No recovery or CLOUD-038 success is credited here.

The [failed-stage screenshot](result/failed-stage.png) captured the earlier running/pending UI, while the terminal API receipt records the later authoritative refusal. This frame is not terminal-state UI verification. The complete [failed report](result/report.json.gz), four phase receipts, journal and all observer rows are retained.

## Package and helpers

The [package receipt](package/package-integrity.json) records **3,587 matching extracted members**, 347 licensed components, zero missing licenses. Archive SHA-256: `078f0689489d51583fb58f46846da9d13156845e6f647e90b744696aa3c7c09e`; binary SHA-256: `e1dd6db97f506b7be2b2efff9bc9a774de79469f707f924f80e717746d92c29b`; image: `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`.

All **11 executed helper copies** match the exact `2da1a821` Git blobs, as recorded in [source-verification.json](source-verification.json). Historical manifest defaults do not supersede the actual invocation. Complete build/integrity/log and image provenance are included; binaries/images are not uploaded, and independent archive retrieval is not claimed.

## Closure

The original driver closed all startup/controller/proxy/browser handles, positively drained the proxy, and recorded an **empty Docker/volume inventory and empty active ledger** at `01:42:25.451Z`. It created no permanent marker. Its raw private ledger-file SHA-256 is `c0022f0995797c072d8097729ea90e74487bcefb911576c91e4af0665403a49e`.

The later root [read-only closure](independent-scoped-closure.json) at `01:53:58.395887Z` records **108 fixture files unchanged**, no active attempt-7 leases, **13 absent recorded identities plus two exited Chromium zombies adopted by PID 1**, and three other unreadable process environments. At that later time, Docker contained **only the unrelated attempt-8 permanent marker**. `unrelatedAttempt8MarkerOnly=true` makes that distinction explicit. The inherited scope phrase about a retained marker describes that later daemon inventory, not an attempt-7 recovery. Original empty-driver receipt and later nonempty daemon inventory are both preserved.

No whole-machine zero-process claim is made. Raw private ledger/inventory bytes remain publisher-held; this publication did not independently inspect the running host or daemon, modify a fixture, signal a process, or remove an object.

## Resources and publication

[53 resource samples](observation/samples.jsonl.gz) span **57.737 seconds**. There are **30 observed control-plane samples / 60 observed cgroup rows**, with **zero recomputed formula mismatches**. Minimum sampled effective available memory is **9,617,809,408 bytes**; minimum physical available is **20,780,109,824 bytes**. They are independent sampled minima, not atomic admission inputs. Check coverage is **2 identity samples / 0 usage samples**.

Reported summed owned-process RSS peaks at **1,180,635,136 bytes** (double-counting of shared pages is possible). Observer interpreter cost averages **0.25005 CPU core**, excluding its children/daemon. No sustained throughput, latency, idle-baseline, whole-computer responsiveness, Windows or model-quality claim follows.

[REPRODUCE.md](REPRODUCE.md) supplies the immutable pin, source-binding command and exact helper invocation. [publication.json](publication.json) binds originals/stored bytes, including deterministic gzip decoded hashes; [SHA256SUMS](SHA256SUMS) covers all capsule files except itself. The privacy receipt records decoded/recursive scans and the visually inspected screenshot. Original failure bytes and earlier failed fixtures remain unchanged.

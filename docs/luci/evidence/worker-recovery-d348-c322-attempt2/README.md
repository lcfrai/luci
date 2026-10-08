# Installed worker recovery attempt 2 — task output reached, recovery failed

This actual installed Linux run used product **`d348f0127a920aab05b54a4192baa8ed3800fe54`**, clean helper **`c322398ce5645236866cfc7b1bad13cd94bdcc99`**, native Codex and a **scripted local provider**. It made **zero external model calls**, with no browser or compiler runs. One worker was created and started and one task result was recorded. The helper then failed; this is not worker recovery, natural session completion or full product acceptance.

The [actual invocation](invocation.json) and [exit capture](invocation-exit.json) retain the launch command and exit **1**. The driver ran `2026-10-08T23:46:59.523Z`–`23:47:33.887Z` (9 October Brisbane). Wrapper elapsed time was **53.657 seconds**, with driver exit **1** and observer exit **0**. Its source-stability and observer-completeness checks passed; the scenario receipt did not.

All **13** captured helper files match the pinned `c322398` Git blobs and source manifest; the before/after source bindings are byte-identical. The captured external wrapper, observer-cost launcher and sampler have their own retained hashes. The launch manifest's `runtimeInvoked: false` belongs to its preparation record; the actual invocation, driver result and wrapper receipt establish the later execution.

The driver verified **2,714 matching installed package members**. Archive SHA-256 is `01512720ca8190b2737fd521b2db39e29329529b211fd830b0ac0a891073d7fd`; binary SHA-256 is `94e59ff60a5ee6a156f55302f0e55000c232a81a90e4d6fa3fd4872947930db3`. The matching [package integrity receipt](../recovery-d348f012-attempt1/package/package-integrity.json) retains 347 licence components and zero missing licences. Image is `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`. These are retained local verification records, without archive upload or independent remote-retrieval claims.

## Reached workflow and exact failure

Docker transport recorded **one worker Create and one Start**, with zero fence Creates. One scripted provider request issued `exec_command`; the native task recorded `build-a`, result commit `84dbad847724e0fc15eb139310fe14e68000fd64`, changed file `build-a-result.txt`, and output SHA-256 `d010792f3cb603f1bfe74359ea707d8ac4e858a02bdf42cc85c22bb929a0c822`. The retained task result also records its read-only context check. This is controlled task output from the installed worker, not live-model quality.

The terminal observer retained **three stdout frames / 7,650 stdout bytes** and **zero exit frames**. `exitObservedAt` is null and exit classification is `not-observed`. Closing its socket does not prove a natural native session exit or successful final task acceptance.

The original failure is **`ReferenceError: canonicalNames is not defined`** in `retained()` at archived [`installed-worker-recovery.mjs`](result/accepted-harness/scripts_acceptance_worker-recovery_installed-worker-recovery.mjs), line **314**. The helper reached actual worker output after the earlier context-export fix, then failed while preparing the retained-state check. Its stage remained `startup`, and `phases` is empty; that stage label must not be interpreted as zero workers. No controller-restart, supported Stop/recovery or second-worker natural-completion scenario was validated.

## Failed-fixture cleanup and retained product charge

Controller, proxy and provider cleanup phases completed; all three acquired startup handles closed. The proxy drained with zero pending requests/handlers/body bytes and no uncertain mutation. The driver separately recorded:

> Unexpected retained worker requires exact failed-fixture cleanup

Its failed-fixture cleanup checked the exact allocation's ownership, stopped it if still running and removed it. The final receipt then recorded zero containers, volumes and owned controllers. This removal was failure cleanup, not natural task completion or a successful product recovery action. The original incomplete cleanup checkpoint is retained alongside the later final failed result.

The [independent read-only closure](independent-scoped-closure.json), at `23:48:41.563134Z`, subsequently observed no owned processes, containers or volumes. The product ledger still contains **`queue-03db3c77af0f4c8767d41489`**, state **`backend-settled`**, charged **1,024 MiB / 1,000 mCPU**. Ledger SHA-256 is `23cc36d21e99b4e5e22ca566727134a1919392a582c8002035b9522621eabc3b`.

**Product recovery, lease release and automatic product cleanup remain unverified.** The independent closure performed no ledger mutation; this capsule exports only the scoped charge fields and original private-receipt hash, not raw ledger contents.

## Observation and publication limits

The observer retained **53 samples over 52.779 seconds**. Its check-specific identity/cgroup counters are zero (`not-observed`); those counters are not worker coverage. No pre-run idle baseline was observed. These observations do not establish sustained resource use, UI latency, causal observer overhead or full recovery acceptance.

[`publication.json`](publication.json) binds the full failed result, journal, observer samples, invocation, accepted sources and derived closure to their original byte counts/hashes. Gzip uses deterministic encoding and retains decoded hashes. The worker result and wrapper are explicitly named sanitized copies: recursive redaction removes exact private fixture roots from every field, including `fixtureRetained.path` and `privateFixtureParent`. Original failed reports remain unchanged. Package host paths are omitted from the sanitized provenance copy.

Exact private roots were derived in memory from the original result, ownership record, wrapper and instance pointer. Every exported JSON string field and decoded text member was checked against those roots. Available retained credential values were inspected in memory and checked across the published result, API/Docker request records and other members. No match remained. Private fixture homes, credential/configuration files, raw ledger, instance pointer and ownership record are excluded. Ephemeral credentials no longer retained cannot be re-read; this is not a universal secret-detection claim.

Publication was prepared entirely outside the checkout using read-only original evidence and Git blobs, with no runtime, browser, Docker command, build or tests. Original failed fixtures were not changed. A future corrected-helper attempt requires a fresh fixture/output, exact product/helper/package/image bindings and unchanged recorded bounds; do not reuse the original invocation's output or fixture paths.

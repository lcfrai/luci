# Installed check recovery attempt 2 — failed before recovery review

This actual installed Linux run used product **`d348f0127a920aab05b54a4192baa8ed3800fe54`**, helper **`fdde0470025e612c631fb97128c6f0a8df8e99b1`**, and **zero external model calls**. It failed at `read-and-dismiss-exact-recovery`; wrapper exit was **1**. The driver ran `2026-10-08T23:36:30.412Z`–`23:37:32.004Z` (9 October Brisbane). This capsule does not establish recovery or full product acceptance.

The [captured invocation](invocation.json) retains the actual launch command, including `--require-public-reservations`. All **11** accepted helper files match the pinned helper Git blobs. The original source manifest retains historical `pins.product=4df94ab9…` / `pins.helper=5d306b6d…`; the runtime report and installed package provenance identify the actual product/helper revisions above. Current helper edits are not substituted for the executed copies.

The driver verified **2,714 matching installed package members**. Archive SHA-256 is `01512720ca8190b2737fd521b2db39e29329529b211fd830b0ac0a891073d7fd`; binary SHA-256 is `94e59ff60a5ee6a156f55302f0e55000c232a81a90e4d6fa3fd4872947930db3`. The matching [package build/integrity receipts](../recovery-d348f012-attempt1/package/) retain 347 licence components and zero missing licences. Image is `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`. These are retained local verification records; no archive upload or independent remote retrieval is claimed.

## Reached behavior and precise failure

The first actual check ran `node verify.mjs final`, exited **0**, emitted `RECOVERY_CHECK_OK`, and recorded `cleanupConfirmed=true` / `resourcesHeld=false`. Public reservations changed **1 → 0**, and the command message confirmed recorded output/exit and exact cleanup. This is scoped installed evidence for CLOUD-033 wording and CLOUD-034 resource visibility.

The second successful Docker Create acknowledgement was deliberately lost. The product retained **`unknown`**, reason **`check_create_unconfirmed`**, with one held **1,024 MiB / 1,000 mCPU** allocation. The helper's corrected original-container inspection returned successfully before [`real-create-ack-lost`](result/phase-07.json) was recorded. Its following [ledger observation](result/phase-08.json) verified the full private charge and matching public totals. This supplies actual scoped evidence that the CLOUD-036 image-inheritance guard correction advanced past the earlier failure.

The next helper action failed an immediate `assert.equal(await buttons.count(), 1)` in archived [`installed-recovery.mjs`](result/accepted-harness/check-create-recovery_installed-recovery.mjs), line **171**. `openProject` waits for the section heading, then `openRecovery` counts the “Review retained check” buttons without waiting for check-history readiness. **The actual count was not exported.** The receipt proves the assertion failed; it does not establish whether the observed count was zero or greater than one. The recovery drawer interaction was not reached, and the attempt supplies no completed recovery acceptance. CLOUD-037 addresses the helper's readiness mechanism separately.

## Cleanup scope and retained charge

The driver's controller, proxy and browser cleanup phases all completed, with an empty cleanup-error array. `stopControllers` completed before the startup scope invoked its controller close again; the repeated-stop path therefore completed in this actual attempt. The proxy drained with zero pending requests/handlers/body bytes and no uncertain mutation. This supplies scoped actual evidence for the CLOUD-036 repeated-stop correction.

**Successful handle teardown did not remove the held check container or release its lease.** The original final report retained container `e7e0fb5f949168405baa74a5e90852c3d83f50f34214e354e3bead37ad1acb01` and one `creating` check charge of **1,024 MiB / 1,000 mCPU**.

The later [independent scoped closure](independent-scoped-closure.json), `23:39:38.559586Z`–`23:39:42.978582Z`, records that exact container as created, not running, PID 0, with zero StartedAt/FinishedAt. The parent operator removed only that immutable ID without force. After-observations contain no containers, volumes or owned fixture processes. All **110** recorded private fixture files retained identical bytes. Their individual paths and metadata are excluded.

The ledger remains bound to SHA-256 `1a04ef3f21e9961c1c50d9cda612ec170b2018c76d8446e3db263fb88b1be15a`, with the same charged `creating` allocation. Product recovery, product lease release and automatic container/lease cleanup remain **unverified**. Manual removal did not repair the product state or turn this run into a pass.

## Observation and publication limits

The observer retained **80 samples over 81.361 seconds**, with **21 check identity samples and zero check cgroup-usage samples** (`identity-only`). No pre-run idle baseline was observed. These observations do not establish check memory peaks, sustained performance, UI latency or causal observer overhead.

[`publication.json`](publication.json) binds every capsule member to its original byte count/hash where applicable. The complete failed report, all eight phase receipts, journal, observer samples/metadata/lifecycle/self-cost, invocation and accepted helper sources are retrievable here. Gzip is deterministic and decoded hashes are retained. Package host paths are omitted in an explicitly sanitized provenance copy. The private independent receipts are represented only by a scoped projection and exact original-record hashes.

Exact private roots were derived in memory from the actual instance pointer and original cleanup records. All exported JSON string fields and decoded text were checked against those roots; available retained credential material was also checked in memory. No match remained. Private fixture homes, credentials, state/source contents, individual file inventories, instance pointer, screenshot and raw wrapper output are excluded. Ephemeral credentials no longer retained cannot be re-read; this is not a universal secret-detection claim.

No browser, workload, Docker command, build or test was run to prepare this capsule. Original failed reports and fixtures were not changed. A future corrected-helper attempt needs a fresh fixture/output, exact source/package/image bindings and the unchanged recorded bounds; the captured command's original output paths must not be reused.

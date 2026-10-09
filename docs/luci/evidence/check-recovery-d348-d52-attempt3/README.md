# Installed check recovery attempt 3 — dialog opened, exact review comparison failed

This actual installed Linux run used product **`d348f0127a920aab05b54a4192baa8ed3800fe54`**, helper **`d52f7fe3d3981342b46391c34ad6517f100ba1be`**, and **zero external model calls**. It failed at `read-and-dismiss-exact-recovery`; wrapper exit was **1**. Driver timestamps are `2026-10-08T23:54:01.835Z`–`23:55:07.681Z` (9 October Brisbane). This is not check recovery or full product acceptance.

The [actual invocation](invocation.json) retains the command, including `--require-public-reservations`. All **11** accepted helper files match the pinned `d52f7fe3` Git blobs. Historical `pins.product=4df94ab9…` / `pins.helper=5d306b6d…` remain unchanged in the source manifest; the runtime report and package verification identify the actual product/helper revisions above. Later source edits are not substituted for the executed copies.

The driver verified **2,714 matching installed package members**. Archive SHA-256 is `01512720ca8190b2737fd521b2db39e29329529b211fd830b0ac0a891073d7fd`; binary SHA-256 is `94e59ff60a5ee6a156f55302f0e55000c232a81a90e4d6fa3fd4872947930db3`. The matching [package integrity receipt](../recovery-d348f012-attempt1/package/package-integrity.json) retains 347 licence components and zero missing licences. Image is `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`. These are retained local verification records; no archive upload or independent remote retrieval is claimed.

## Reached behavior and exact failure

The first actual check ran `node verify.mjs final`, exited **0**, emitted `RECOVERY_CHECK_OK`, and recorded confirmed cleanup with public reservation count **1 → 0**. The second successful Docker Create acknowledgement was deliberately lost. The product recorded **`unknown` / `check_create_unconfirmed`**, while the helper verified the original created container and a held charge of **1,024 MiB / 1,000 mCPU**, matching the public totals. No recovery marker was created.

The helper then waited for a visible recovery control and opened the dialog. The original, privacy-reviewed [failure screenshot](result/failed-stage.png) shows **Review check recovery**, the exact target check, **revision 7**, unchecked consent and the disabled recovery action. [Screenshot provenance](screenshot-review.json) binds its original bytes to this attempt and the archived capture source. No new browser or screenshot was generated, and the image was not edited.

The failure is **`AssertionError` / `ERR_ASSERTION`** at archived [`installed-recovery.mjs`](result/accepted-harness/check-create-recovery_installed-recovery.mjs), line **175**. That line confirms HTTP 200, then executes `assert.deepEqual(await response.json(), review)`. It therefore failed an exact comparison after the dialog opened. **The expected and actual HTTP review bodies were not retained.** Their precise difference cannot be reconstructed from the failure receipt or screenshot. The original assertion and failed outcome are preserved.

The [scoped check comparison](persisted-check-comparison.json) separately compares the archived public check DTO at **revision 6** with the later retained raw check record at **revision 7**. Top-level and command messages differ:

- Earlier public DTO: “Docker check creation is unconfirmed. Review this exact attempt before starting another command.”
- Later persisted record: “Docker creation is unconfirmed. Review this exact retained attempt to stop and recover it.”

`revision`, `updatedAt` and `staleReasons[0]` also differ. The stale-reason wording may reflect the public DTO's derived evidence fields rather than an intervening persisted mutation. This is **not** a pure before/after comparison of two persisted rows, and neither row is claimed to be the missing HTTP operand. The pinned [execution path](diagnosis-source/control-plane/internal/api/check_execution.go) emits the earlier wording, while the [observation path](diagnosis-source/control-plane/internal/api/check_observation.go) assigns the later wording and persists the outcome for a creating lease. This is the source basis for CLOUD-038 message/revision churn; these observations support that investigation without proving the exact failed HTTP difference. No assertion was weakened to prepare this evidence.

## Completed handle teardown, held container and lease

Controller, proxy and browser cleanup phases completed, with an empty cleanup-error array. All five startup resources closed. The proxy drained with zero pending requests, handlers and body bytes and no uncertain mutation. This is ordinary handle teardown, not automatic container or lease cleanup.

The original final report still retained container `b57f6e3b9bb8dde80b21ade4177475f66371d5e2262132f32f6e27a697dabb2c` and one **`creating`** check charge of **1,024 MiB / 1,000 mCPU**. It contains no completed recovery result.

The later [independent scoped closure](independent-scoped-closure.json), `2026-10-09T00:00:03.703400Z`–`00:00:05.930307Z`, records the exact container as **created, not running, PID 0, with zero StartedAt/FinishedAt**. The parent operator removed only that immutable ID without force. After-observations found no owned fixture processes, containers or volumes. All **110** recorded private fixture files retained identical bytes; individual paths and metadata are excluded.

The ledger remains bound to SHA-256 `2ce745fb54a0f45d7b99eb21edc2ee78b9dbded5d1d2c750d765a911c4d7f72a`, with the same charged `creating` allocation. **Product recovery, lease release and automatic container/lease cleanup remain unverified.** Manual removal did not repair state or convert this run into a pass; raw ledger contents are not published.

## Observation and publication limits

The observer retained **83 samples over 85.565 seconds**, with **23 check identity samples and zero check cgroup-usage samples** (`identity-only`). No pre-run idle baseline was observed. These observations do not establish check memory peaks, sustained performance, UI latency or causal observer overhead.

[`publication.json`](publication.json) binds the complete failed report, all eight phase receipts, journal, observer evidence, actual invocation, accepted helper sources, screenshot and derived projections. Gzip uses deterministic encoding with decoded hashes. Package host paths are omitted in an explicitly sanitized provenance copy. Private cleanup records and the later persisted check record are represented only by scoped projections and original hashes; their private paths and complete contents are excluded.

Exact private roots were obtained in memory from the original instance pointer and cleanup records. All exported JSON strings and decoded text were checked against those roots and available retained credential material. The screenshot was visually checked; PNG text metadata was inspected. No private-root or known-credential match remained. Private fixture homes, credentials, raw state/ledger, individual file inventories, instance pointer, ownership and raw wrapper output are excluded. Ephemeral credentials no longer retained cannot be re-read; this is not a universal secret-detection claim.

The capsule was staged outside the checkout using read-only original evidence and Git blobs, with no runtime, browser, Docker command, build or tests. Original failed fixtures and reports were not changed. A future corrected attempt needs fresh fixture/output paths, exact source/package/image bindings and unchanged recorded bounds; do not reuse the captured invocation's original output paths.

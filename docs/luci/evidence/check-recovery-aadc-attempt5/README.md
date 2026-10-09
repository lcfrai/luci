# Check recovery attempt 5 — target blocked by CPU admission before Create

This actual installed Linux attempt used matching product/helper **`aadc1c281aef28aee04046a5c5e58da5fe7094e4`** and made **zero external model calls**. The historical check passed, but the target check was blocked by **`cpu_pressure`** before any target Create. The helper later failed at `consent-check-2` with **`gate-wait-timeout`**, waiting for a Create acknowledgement that never arrived. This is not an unknown-Create or recovery pass.

Driver timestamps are `2026-10-09T00:36:38.981Z`–`00:39:13.719Z`; the [captured wrapper exit](wrapper-exit.json) is **1**. The [actual invocation](invocation.json), all **11** source-bound helper copies and the original failed result are retained. Those copies match the `aadc1c28` Git blobs, not later edited worktree files. Historical source-manifest defaults do not replace the runtime identities above.

The installed package had **3,126 matching members**, archive SHA-256 `e73f44b5dd59d7f8205661f821d556cfcaeb43b0f765755ae7e6b5c81d20d38d`, and binary SHA-256 `5d70f3d2fe83d49ac9daabd207a0251fffde5fdb181f9249ed28b92278f90763`. Image was `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`. These are retained local verification records; no archive upload or independent retrieval is claimed.

## Exact target cause retained in state

The first historical check accounts for the transport's **one Create / one Start**. It ran `node verify.mjs final`, passed, and recorded confirmed cleanup and reservation release. The target check was then submitted, but **no target Docker Create occurred**.

The [allowlisted target diagnosis](target-admission-diagnosis.json) binds the original retained project-check record. Target **`check-91c2af0e6716ba0572f54b4e`**, project **`p-d3416195`**, is revision **5**, **`blocked / admission / cpu_pressure`**, with `resourcesHeld=false`. It was created at `00:37:12.364133405Z` and updated at `00:37:12.485169317Z`. Its recorded message is:

> Current CPU pressure needs to fall before another worker starts.

The command has the same status/reason, no exit code and empty output. Its literal `cleanupConfirmed=false` is preserved: no target container was allocated, and no target resources remain held. That field must not be presented as a leaked target container.

The pinned [capacity evaluator](diagnosis-source/control-plane/internal/api/execution_capacity.go), lines 50–51, returns this reason when fresh host CPU utilization exceeds **`100 − policy.HeadroomPercent`**. This branch follows the memory guard and is distinct from the `cpu_capacity` allocation-limit branch. **The target decision's exact utilization value and policy input were not retained** in the check record or diagnostic journal. No numeric admission sample, external-load cause or memory refusal is invented. Nearby resource observations are not substitutes for the exact decision input.

The original helper failure remains `gate-wait-timeout`; its retained journal does not record the target's later status. Reading the exact target record after closure established the CPU refusal without modifying the fixture. A subsequent helper correction is separate from this failed attempt; no assertion or threshold was weakened here.

## Resources and ordinary cleanup

[Resource observations](resource-observations.json) preserve phase timestamps, physical/effective scopes and raw-sample bindings. All original observer samples are retained: **175 samples over 175.327 seconds**, with **two check identity samples and zero check cgroup-usage samples** (`identity-only`). These include the historical check and the helper's wait; they are not sustained target work or target memory usage. No pre-run idle baseline or exact atomic target-admission snapshot was observed. The values do not establish causal overhead or explain earlier refusals.

Controller/proxy/browser cleanup and all five startup handles completed. The proxy drained with no pending requests, handlers or body bytes and no uncertain mutation. The driver's final inventory and ledger were empty. There was no target recovery marker and no separate manual container removal.

The [independent read-only closure](independent-read-only-closure.json) separately confirmed empty actual Docker/volume inventories, absence of the historical created container and a parsed version-1 ledger with zero queue/check leases. Ledger SHA-256 remained `c0022f0995797c072d8097729ea90e74487bcefb911576c91e4af0665403a49e`, matching the original final receipt. All **109** recorded fixture files remained byte-identical during those checks; the target-record hash also matches that inventory.

Of **14** retained PID/start identities, **12 were absent and two were exited zombies**; none was active. The exact fixture-environment scan found no matches among readable entries, while **5,851** other process environments were unreadable. This does not establish universal process/descendant absence. No process was stopped, Docker object removed or ledger repaired by the independent inspection. Empty allocations do not establish an unknown-Create recovery that was never reached.

## Publication

[`publication.json`](publication.json) binds the complete failed report, four phase receipts, journal, all observer data, invocation, captured helper sources, pinned evaluator and derived diagnosis/closure. Gzip is deterministic with decoded hashes. Package host paths are omitted from the explicitly sanitized provenance. Private source records are represented only by scoped projections and hashes; no raw ledger, fixture path, credential, private file inventory or complete target state is exported.

Exact private-root and retained-credential checks cover exported JSON strings and decoded payloads. No known match remained; ephemeral credentials no longer retained cannot be re-read. Preparation used original artifact/state reads, Git blobs and explicit read-only inventory checks. No worker, browser, build or tests were launched. Original failed reports and fixtures remain unchanged; any later retry needs a fresh fixture/output with unchanged admission rules.

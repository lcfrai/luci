# Installed recovery attempts — both failed

These two actual installed Linux attempts used product **`d348f0127a920aab05b54a4192baa8ed3800fe54`** and made **zero external model calls**. Neither establishes worker/check recovery acceptance, sustained performance, Windows behavior or real-model quality. Preparation of this capsule used retained files and read-only hashing; it did not run another browser, workload, Docker command, compiler or test.

The matching package build exited zero. Its retained verification records **2,714 matching extracted files**, **347 licence components** and **zero missing licences**. Archive SHA-256 is `01512720ca8190b2737fd521b2db39e29329529b211fd830b0ac0a891073d7fd`; executed binary SHA-256 is `94e59ff60a5ee6a156f55302f0e55000c232a81a90e4d6fa3fd4872947930db3`. These are local build/verification receipts, not independent remote retrieval or a release publication. Package bytes are not included in this capsule.

Both attempts used image `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`. The retained [image provenance](../cloud029-installed-4df94ab9-attempt11/result/image-provenance.json) has SHA-256 `5af726e6670fd6f04c0f67cf3b26b3f6072ed89c1e1e3481c652a631ae6d0a3b`. Reusing that image does not change either attempt's product binding.

## Worker recovery: stopped before allocation

At `2026-10-08T23:13:18.182Z`–`23:13:21.035Z`, the installed helper failed during `startup`: its seeding code tried to open the nonexistent `.luci/brief.json` export. The failure is an `ENOENT` helper contract mismatch. No recovery phase, accepted task result, worker Create or Start occurred. The worker result records zero browser and compiler runs.

The wrapper exited **1** after **21.848 seconds**. Its driver failed and its observer-completeness check also failed because the driver never published the expected instance pointer. The observer process itself exited zero and retained **22 samples over 21.174 seconds**, with no observed check identity. Those samples are not recovery or workload measurements. Controller, proxy and provider cleanup phases completed; the retained independent daemon observation found zero containers and volumes.

The worker helper and product both bind to `d348f0127a920aab05b54a4192baa8ed3800fe54`. All **12** captured helper files match that commit byte-for-byte; before/after source bindings are identical. The external wrapper, sampler and observer-cost launcher are separately retained and hashed. The launch manifest's original `runtimeInvoked: false` describes its preparation record; the actual failed result and wrapper receipt establish the later attempted execution.

## Check recovery: first check passed, recovery never reached

At `2026-10-08T23:18:31.403Z`–`23:19:31.946Z`, the check helper used clean helper revision **`6e91c319901ee912dc02b562e3c3e4e449e9ce4e`** against the same installed `d348f012` package. All **11** captured helper files match that helper commit. Its source manifest retains historical `pins.product=4df94ab9…` and `pins.helper=5d306b6d…`; the report's explicit product/helper identities and package verification identify what actually ran. The historical manifest fields have not been rewritten.

The first actual check ran `node verify.mjs final`, exited **0**, emitted `RECOVERY_CHECK_OK`, and recorded `cleanupConfirmed=true` / `resourcesHeld=false`. The public owner resource snapshots show reservation count **1 → 0**. Its command message says, “The command exit and output were recorded; exact container cleanup is confirmed.” This supplies scoped installed evidence for the CLOUD-033 cleanup wording and CLOUD-034 public reservation display. Public totals alone are not authority to release a reservation, and this check does not establish completion of the application's acceptance criteria.

The second check deliberately lost its successful Create acknowledgement. The product recorded **`unknown`**, reason **`check_create_unconfirmed`**, `cleanupConfirmed=false` and `resourcesHeld=true`; the public reservation count remained **1**. Docker transport recorded two Creates but only the first container's Start. Independent later inspection found the second container **created, not running, PID 0, with zero StartedAt/FinishedAt**.

The helper failed at `consent-check-2`, before recovery, because [`assertOriginalInspect`](check/result/accepted-harness/check-create-recovery_fixture-guards.mjs) line 54 requires an empty `Config.ExposedPorts`. The image supplies `8080/tcp`; no host port binding existed. This inherited declaration is not a published port, and the helper's rejection does not establish an unsafe check allocation. The failed report also retains `sdk-check-request-shape-refused` in transport diagnostics; neither failure is erased or upgraded to a passing result.

The observer retained **78 samples over 79.291 seconds**, including **19 check identity samples and zero check cgroup-usage samples** (`identity-only`). There was no pre-run idle baseline. These observations do not establish check memory peaks, UI latency, sustained responsiveness or observer overhead relative to an unobserved run.

## Cleanup remains a separate, scoped observation

The check run's original cleanup reports a controller duplicate-stop assertion at `installed-recovery.mjs:99`, with controller cleanup `failed-or-unsettled`. Browser resources closed and the proxy drained with zero pending requests/handlers/body bytes and no uncertain mutation. Its final receipt still recorded the second container and a charged **`creating`** check lease: **1,024 MiB / 1,000 mCPU**.

The later [independent scoped closure](check/independent-scoped-closure.json), recorded `23:24:45.911614Z`–`23:24:49.795523Z`, verified no live fixture processes and removed only exact container `e47e24aceed8ca8e92ddfd978bf37415442cf9d6f94b8854b595933323b0224b`, without force. Its after-observations found no containers, volumes or active/identified-zombie fixture processes. All **110** recorded private fixture files retained identical bytes; individual paths and metadata are excluded from this capsule.

The ledger remains byte-identical to the failed report's binding, SHA-256 `f2d1001369dd08a30f6eb5a492b5e57d460896eb4d2616ba489d8c0c71b8e743`, with the same charged `creating` check allocation. **Product recovery, product lease release and automatic cleanup are not certified.** Manual container removal did not repair state or convert the failed run into a pass. Earlier attempt 7 and sustained attempt 3 evidence and private fixtures were not changed by this publication.

## Retrievable evidence and limits

[`publication.json`](publication.json) binds each retained member to its original path, byte count and SHA-256. Compressed results, journals and samples use deterministic gzip and include decoded hashes. The full check report, phases, journal and observations are retained byte-for-byte after decoding. The worker result redacts the residual private hierarchy from its `ENOENT` message; its original hash remains bound. Wrapper private-parent and package host-path fields are omitted in explicitly named sanitized copies. Original failed reports remain untouched.

Private cleanup records are represented only by a derived allowlist and exact original-record hashes. Fixture homes, configuration, credential values, private file inventories, state/source contents, instance pointer, ownership marker and screenshot are excluded. Available retained credential material was inspected in memory for validation; no matching value was exported. Ephemeral credentials no longer retained cannot be re-read, and pattern scanning is not a universal secret-detection claim.

Use [`invocation.json`](invocation.json) for command identity and reconstruction limits. A future run needs a corrected helper, the exact pinned package/image and a **fresh** isolated fixture/output path. Preserve the 600-second scenario/180-second cleanup/120-second request bounds, exclusive daemon ownership, 6 GiB worker pre-Create guard, 1 GiB running reserve and declared allocation limits. Do not reuse either failed fixture. Installed worker recovery, full check recovery and the wider product acceptance gates remain open.

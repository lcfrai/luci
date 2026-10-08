# CLOUD-020 external integration evidence

The frozen 47 Go/JSON overlay inputs are listed in `source-integration-manifest.json`, with each original 4e7d5fe4 source hash, the staged hash, and the identical root-applied checkout hash. The last pre-application `go-overlay-preparation.json` is preserved unchanged for provenance; it must not be reused as a current checkout manifest after application. The staging agent kept the checkout unchanged during these checks. Root subsequently applied the sources and owns checkout validation. The separate staged API document is not a compiler input and needs the root owner's documentation merge.

## Preserved checks

| Evidence | Observed result | Scope |
| --- | --- | --- |
| `diagnostic-2/report.json` | 219 pass, 4 fail, 0 skip | Selected Docker adapter, management, API, product and command wiring race checks. Four adapter failures were preserved. |
| `diagnostic-3-adapter/report.json` | 85 pass, 0 fail, 0 skip | Corrected Docker capability-slice comparison, all four original adapter failures, and added negative configuration cases. |
| `retired-reservation-unfixed-complete/report.json` | 0 pass, 1 fail, 0 skip | Deterministic HTTP/controller reproduction: a delayed controller performed one Create and one Start after another owner settled and acknowledged its reserved lease, then overwrote the settled public receipt. |
| `retired-reservation-fixed/report.json` | 13 pass, 0 fail, 0 skip | The resurrection regression and exact-claim matrix after correction. |
| `retired-reservation-check-lifecycle/report.json` | 31 pass, 0 fail, 0 skip | All selected API project check lifecycle tests, including the new regression, after correction. |

Each completed check above retained the same 820 source inputs throughout its run. The later API lifecycle run used docs-only HEAD f9f80e88; earlier runs used 4e7d5fe4. Counts overlap and must not be added together. The initially unactivated `retired-reservation-unfixed` environment attempt was stopped during dependency discovery; it is not a test result.

`diagnostic-2/events.jsonl` records `TestSwarmTerminalCleanupRestartBeforeWaitingProjectAdmission` passing in 5.87 seconds. This LOCAL-012 regression verifies terminal cleanup after reopening occurs before admitting the waiting project. Its source remained unchanged through the subsequent adapter and check-only lifecycle corrections.

## Lifecycle correction and boundary

The new API guard runs immediately after acquiring the exact allocation's lifecycle OS lock and before publishing progress or rechecking admission. It requires the same pending lease identity, actor, resources, engine, instance, project, command and scope. A lost or changed claim returns `ErrProjectCheckSuperseded`; the product controller exits without publishing its cached final run. A created claim is not accepted at this initial ownership boundary. The normal acknowledged-Create recheck later in the same held lifecycle remains supported.

The regression drives the original launch through HTTP, pauses image preparation, gives an independent controller the lifecycle lock, and settles/acknowledges the real pending lease with the production observer publication methods. It then resumes the delayed original controller. The corrected run proves zero Create/Start, no restored capacity claim, and byte-for-byte preservation of the entire settled private project history, including revision and output. Eleven exact-claim cases additionally verify valid reserved ownership and changed/absent claim rejection.

Five original failing source bodies are preserved under `retired-reservation-unfixed-complete/unfixed-sources`. Their hashes were checked against that run's `before.json`; this keeps the reproduction reviewable after the correction.

This race does not claim that a dead Linux process resumes after a normal restart. Normal Linux startup has a whole-state exclusive writer lease. Non-managed Windows/macOS startup lacks that exclusion and can run separate controllers against the same state on distinct configured loopback addresses. Broader same-state document writer exclusion is tracked separately as CLOUD-022. Before a first check reservation exists, observers/recovery have no private allocation to acknowledge; the demonstrated stale-controller window is the existing reservation waiting for lifecycle ownership.

## Coverage map and remaining gates

- Durable pending checks and cross-account worker/check exclusion: `TestProjectCheckHTTPAccountsSharePendingCapacityBeforeCreate`, `TestProjectCheckHTTPPendingCheckBlocksIndependentWorkerAdmission`, and `TestProjectCheckHTTPExistingWorkerBlocksCheckWithoutProviderLease`.
- Fresh authority, host refusal and input scoping: `TestProjectCheckHTTPRechecksRevokedAuthorityBeforeCreate`, `TestProjectCheckHTTPRejectsHostAndBrowserAuthorityInputs`, management check/control-authority selections.
- Explicit unknown-Create fence recovery with current Edit, retained proof and no replay: `TestProjectCheckHTTPUnacknowledgedCreateRequiresReviewedFenceAndFreshEditor`.
- Cleanup/output preservation and final publication acknowledgment: `TestProjectCheckHTTPCleanupFailureKeepsOutputAcrossReopenWithoutReplay`, `TestProjectCheckHTTPFinishedReceiptSurvivesFinalPublicationBoundary`.
- Shared observer deadline and peer progress after reopening: `TestProjectCheckHTTPObservationBudgetPreservesPeerProgressAfterReopen`; cleanup now receives the observer's context instead of a new background deadline.
- Delayed reserved-controller retirement and unchanged publication: `TestProjectCheckHTTPRetiredReservationCannotBeRecreatedByDelayedController`, `TestProjectCheckReservedClaimRequiresExactPendingOwnership`.

These are controlled source/race checks. They do not establish live Docker performance, installed mixed worker/check behavior, browser usability, native Windows behavior, or the complete Create/Start/ACK fault matrix. Full affected package execution and installed acceptance remain separate root-owned gates.

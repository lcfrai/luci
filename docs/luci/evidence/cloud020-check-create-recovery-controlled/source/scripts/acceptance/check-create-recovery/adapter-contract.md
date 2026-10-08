# Installed adapter contract (not yet executed)

`installed-recovery.mjs` implements this contract for `runInstalledRecoveryScenario(ctx)` using actual installed HTTP/browser/Docker observations. Synthetic product DTOs, manufactured leases or mock daemon proofs are invalid installed evidence. Do not invoke it before a granted exclusive runtime window and completed bounded pure validation. Default scope is one unknown allocation plus unrelated completed history; concurrent charged isolation requires the separately capacity-gated `--charged-witness` option.

| Capability | Required binding |
| --- | --- |
| `pins`, `windowGranted`, `freshFixture` | Exact pinned package/archive/web/image integrity, source/helper identity, fresh fixture and output directory, explicit root window. No attempt7 path. |
| `projects` | Two application-created fixture projects, each `{id,...}`; current owner/control identities remain private. General checks do not require synthetic approvals or a DAG/model run. |
| `startCheckInBrowser(project, command)` | Actual installed ProjectChecks UI; clear prefilled commands, add only typed `node ["verify.mjs","final"]`, select approved AC-recovery and explicit trust acknowledgement, one POST `/v1/teams/{team}/workspace/v1/product/projects/{id}/checks`, 202. Return the real queued check. Record the request without headers/tokens. |
| `waitCheckTerminal`, `readCheck`, `readRecovery` | Exact project/check HTTP paths; bounded response size, deadline, real status, no automatic mutation/retry. Do not regard an HTTP202 as allocation/execution. |
| `readLedger()` | Read-only, bounded ordinary-file read of this fresh fixture's `state/control-plane/execution/leases.json`. Require version1, exact instance/engine/scope and unchanged file identity during read. No repair or writes. Full schema retained in memory; publication uses only manually reviewed allowlisted fields/hashes. |
| `snapshotProjectFiles()` | Confined Git/source hash snapshots for both fresh project directories; no credential or unrelated-file traversal. |
| `assertCurrentReservationTotals()` | Derive exact count/memory/CPU from the complete read-only shared ledger, preserving all scopes. Fresh host/execution samples remain mandatory. The owner's public reservation object is an API-process-local cache and may be null or stale for team-child writes; retain it as explicitly non-authoritative diagnostic evidence, never a release proof. This is a separate product visibility finding, not acceptable verified counter behavior. Never infer release solely from a404. |
| `assertRealOriginalCreatedAndNeverStarted()` | Fresh immutable-ID inspect matches captured real201 and exact target identity, configured image/source/mounts/resource/security shape; state `created`, no target Start; ledger has no acknowledged reference. |
| `openAndDismissReviewInBrowser()` | Actual dialog `Review check recovery`, matching check/source/command, unchecked consent, disabled `Recover this exact check`; dismiss without any recovery POST. |
| `probeNoEditAuthority()` | Real editor membership plus explicit project grant, both established through supported team CAS APIs. Editor reads valid recovery view; owner downgrades project grant to viewer; same editor token submits same reviewed body and gets403 team_permission_denied, with no effects. Owner performs the later independent authorized recovery. Do not forge authority headers or edit identity files. |
| `probeStaleReview()` | Submit the true digest with a deliberately wrong positive expected revision through the real authenticated API; expect409. Keep this negative probe distinct from the single successful explicit UI POST. |
| `recoverInBrowser()` | Match GET recovery to displayed current check/revision, method `fence-launch`, index0 and digest; explicit checkbox `/I authorize recovery of this exact check command/`, one click `Recover this exact check`; intercept the exact real HTTP POST to verify body and200. Return `{httpStatus,request,postCount}`. No callback substitution. |
| `assertAcknowledgedPermanentFence()` | Correlate original immutable-ID force Delete acknowledgement, original absence, one real fence201 delivered to product, durable `checkFences` retired proof, exact engine/instance/allocation/check/index/project/scope/command identity and intent. Fresh read-only inspect by immutable ID AND name agrees with proof; original ID absent; exact filtered inventory contains only marker. Check all inert shape constraints from pinned `dockerbe/check_fence.go` and `launch_fence.go`. Never start the marker to test it. |
| `assertFenceStillPresent()` | Repeat fresh exact-ID/name/shape checks after unrelated work settles. Leave it retained. |
| `record()` | Write bounded phase JSON and a durable allowlisted phase marker in the shared journal, without raw Create body, auth headers, fixture configuration or credentials. |

The runner must use `createWorkflowDeadline` and `createStartupScope`, admit every process/port before late startup can outlive cleanup, and use `runBoundedCleanup` in `finally`. Stop owned controllers before closing/draining proxy. Capture failed phase before cleanup but distinguish it from final cleanup. Do not let a finally exception overwrite original outcome. Do not remove an unknown allocation manually; unresolved product state stays charged and retained. Failure cleanup cancels only the unforwarded witness gate, never treats cancellation as daemon absence, and retains any unresolved controller/lease/marker with explicit status.

Default transport options in the staged extension:

```js
{allowCheckTraffic:true, checkTrafficProtocol:'sdk-ledger-v1',
 allowCheckRecovery:true, checkCreateLimit:2,
 maxCreates:3, maxCreatesByKind:{worker:1,check:2,fence:1}, maxStarts:1,
 upstreamTimeoutMs:120000, holdTimeoutMs:120000}
```

The positive `worker:1` is an existing proxy schema constraint, not authorization for a worker. `identifyCreate` rejects every worker and totals require zero. Explicit `--charged-witness` raises checkCreateLimit/check to3, maxCreates to4 and maxStarts to2. It must first verify full current ledger plus prospective RAM/CPU/image disk demand; no speculative second allocation is made on insufficient measured capacity. The guard allows exactly the separately consented index0 checks and one target fence, validating fresh policy, complete ledger, current owner identity and project confinement on identify and before-forward. Never substitute a constructed smaller ledger.

Fence create labels are the ten check identity labels plus `luci.launch-fence`, `luci.fence-intent`, `luci.fence-binding`; no source/image labels. Validate the permanent shape: no image, mounts, env, ports, entrypoint or healthcheck; `Cmd:["/"]`; disabled network; readonly rootfs; network/ipc `none`; no privilege/autoremove/restart; ALL caps dropped; no-new-privileges; log driver `none`; 6MiB/100mCPU/one PID. Compare intent/binding against the actual product-created current fence intent; labels alone never authorize retirement.

Final wrapper pass requires every scenario assertion plus all owned processes closed, proxy drained without uncertainty, and independent inventory exactly equal to the retained marker (no runnable check containers or volumes). The exact product ledger must have no active check charges, retain its retired marker proof, and preserve unrelated history. Archive integrity/publication is a later phase. `runInstalledRecoveryScenario` deliberately returns `passed:false,cleanupPending:true` until the adapter provides those closure receipts.

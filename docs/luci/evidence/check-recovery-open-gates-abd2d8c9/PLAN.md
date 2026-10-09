# Remaining unknown-Create acceptance: focused source proposal

Prepared from clean published source `abd2d8c9314d6b73d0c202c82de72f3c3acc0d9a`. This is a **source proposal**, not implemented helper behavior or runtime evidence. No application, Docker command, provider, compiler or test was launched; no shared source or fixture was edited. Exact inspected source hashes are in `source-audit.json`.

## 1. Concurrent charged witness already has a real execution route

`scripts/acceptance/check-create-recovery/installed-recovery.mjs` already accepts **`--charged-witness`**. It changes only the fresh fixture's supported per-user check concurrency policy from 1 to 2; both real checks still request **1,024 MiB / 1,000 mCPU**. It retains 25%/minimum 2 GiB host headroom, 1 GiB working disk reserve, 6 GiB Create floor, real host/execution/Windows backing-volume checks and live authority. The scheduler `maxActive=1` controls queued workers, not the independent per-user check limit; `execution_check_leases.go:147` independently applies that check policy and all shared resource demand.

The existing scenario creates the nonempty historical check, then loses the target's actual Create acknowledgement. With the flag, a **third real, explicitly UI-consented check** is submitted in the other project. Its actual single Create request is held **before forwarding** through `unrelated-create-held`. Its real product ledger entry is `creating`, charged and has no acknowledged ref. No ledger is seeded or fabricated.

`assertRetired` already requires the other exact lease and check receipt to remain deeply equal through target recovery, and compares all unrelated ledger fields. Current-authority 403, stale-review 409, read/dismiss, source/history and mutation guards all run while it remains charged. After the target's positively acknowledged fence and exact release, the same held request is released **once**, with the existing fresh forwarding guard. Its own product admission/Start/exit/output/exact cleanup must pass. Final totals become **3 ordinary Creates + 1 fence / 2 Starts**, with no target Start/replay. The target remains cancelled/unverified; historical and witness checks pass independently.

The optional flow has controlled positive and corruption-negative coverage in `scenario-flow.test.mjs:199,242,250`. Those are synthetic receipts, not actual concurrent Docker acceptance. **Do not rerun the default attempt-8 path and call the gap closed.**

### Capacity is a real gate

The existing image receipt records size **2,294,629,077 bytes**; VFS creates use a conservative **4,589,258,154-byte** estimate. While the target Create is unacknowledged, its disk claim remains charged even when the helper can inspect its real container. Before admitting the witness, the helper requires measured free disk of at least:

`max(6 GiB, target pending 4,589,258,154 + prospective 4,589,258,154 + 1 GiB reserve)`

= **10,252,258,132 bytes** at that stage, in addition to disk already occupied by the target. This is a necessary guard, not proof of sufficient global capacity. Normal production admission also accounts for every visible allocation and retained lease; fresh memory/CPU, authority, per-user limits and inventory must all pass. The helper's prospective two-charge memory check alone requires at least **10 GiB effective available** on a 32 GiB effective-total host with 25% headroom. Physical availability alone is not sufficient. CPU demand and other allocations remain authoritative.

Root's one-heavy-allocation VFS environment may legitimately be unable to satisfy this. A refusal must be recorded as **not exercised / capacity blocked**, without weakening disk claims/headroom, changing the image into a toy, fabricating a witness, or repairing an old fixture. Use safe retirement of verified disposable artifacts if enough capacity can actually be recovered; otherwise use a spacious verified environment and retain the gate for Windows. Never reuse the original failed fixtures or their charges.

Root's current reported baseline is approximately **6.2 GiB free on the execution backing volume**, with effective cgroup headroom around **15 GiB before one copy / 10 GiB loaded**. These are planning observations, not fresh admission inputs captured by this audit. The required **10,252,258,132 bytes free after target allocation** clearly does not fit that disk baseline. Moving several GiB of files onto RAM-backed `/tmp` could also consume the very cgroup headroom needed for two charges and the image copy; it is not a free capacity increase.

Safe preparation options, in order, without changing current fixtures or workloads:

1. Inventory **owned generated copies only**: obsolete extracted packages, repeat build caches and duplicate cross-compiled binaries. Retire a copy only after its entire member manifest/bytes are verified against a retained original archive and no active process uses it. Existing root retirement receipts indicate much of this has already been done; do not claim several extra GiB without actual inventory. Preserve original failed evidence, private fixture archives, licenses and exact package/image artifacts. Do this outside measurement windows.
2. Move verified generated archives to an **already available spacious durable volume**, with before/copy/after hashes and updated location receipts, if that volume actually exists. Do not confuse another path on the same mount with new capacity. Do not put private evidence onto public storage or assume memory-backed `/tmp` is suitable; both physical storage and cgroup budgets must improve or remain adequate.
3. Use a **separate disposable execution host/daemon with adequate storage**, preserving the pinned runtime image, source/package identity and all normal admission checks. A supported copy-on-write Docker driver (`overlay2`/`overlayfs`) can use its actual product-supported disk estimate, but switching the existing daemon's storage driver or relabelling VFS is not acceptable. A new isolated daemon requires demonstrated host support, sufficient storage for the image and normal kernel/security capabilities; do not promise it works without preflight. Real snapshots must identify the new daemon, driver, image and free physical backing space. No additional hosted resource is provisioned by this proposal.
4. Windows Docker's actual copy-on-write environment is a valid later route once native RAM admission permits it. Carry the host backing-volume proof and normal 2.5 GiB browser-launch gate; the current local RAM hold remains real. Do not launch there merely because the disk demand may be smaller.

The immediate cloud plan is to leave the installed concurrent witness gate **capacity-blocked**, continue independent work, and run it only after a verified environment genuinely meets the complete fresh budget. The existing deterministic source proof and single-allocation installed pass retain their original scope.

### Minimal helper extension proposed: durable witness checkpoints

No new proxy API or product behavior is required. Add narrow scenario evidence and assertions so the public receipt directly demonstrates the missing witness property:

1. **After the witness gate is captured, before target review:** assert exactly one `before-forward` gate for the witness, `state=holding`, exact project/check/allocation identity, no forwarded timestamp/daemon ID and no witness Start. Record `charged-witness-held-before-recovery` with check/project/allocation IDs, expected 1,024/1,000 resources, command/source digests, current check revision/status, captured gate metadata and canonical parsed-lease SHA-256. Include no raw private source paths, auth or Create body.
2. **After target fence proof/target release, before witness gate release:** reuse `assertRetired` and the held-gate assertions; record `charged-witness-preserved-after-target-retirement`. Prove the witness lease/receipt are byte-for-byte semantically unchanged, raw/private ledger total is exactly one witness charge, public totals agree, and target remains cancelled/unverified. Preserve original source/history checks.
3. **Release the same gate once:** retain release timestamp, request sequence, original consent identity, actual forwarded Create201/immutable ID, acknowledged Start204, exit0/nonempty output, exact Delete204/absence and witness charge release. Record `charged-witness-completed-after-single-release` with final exact check result and separate claim totals. Existing product and proxy admission still decide; no resubmission on a refusal.
4. Keep existing finite 120-second gate/request and 600-second workflow budgets. A held request expiring or an ambiguous transport fails, preserving ownership/uncertainty. It does not become a cleanup pass. Retain the marker in the normal final run; terminal retirement stays separate.

Suggested helper interface (inert until wired):

```text
assertHeldChargedWitness({proxyObservations, check, exactLease})
  -> allowlistedWitnessCheckpoint

recordWitnessCompletion({beforeCheckpoint, finalCheck, transportRows, ledger})
  -> allowlistedCompletionCheckpoint
```

The helper must derive identity from the real product-created lease and gate, not trusted caller-supplied IDs alone. Compare the old/new canonical digest locally and include both hashes in the public checkpoint. SHA-256 is a binding, not a replacement for deep identity/resource comparisons.

### Minimum meaningful controlled verification for the extension

- Existing real scenario flow with synthetic callbacks plus new held/preserved/completed checkpoint assertions.
- Negative gate cases: wrong check/project/allocation, not holding, prematurely forwarded, already released, forged daemon ACK/ref or any target/witness Start before allowed point.
- Witness lease or receipt changes during target recovery must fail before releasing the gate; preserve existing resource corruption and history-change negatives.
- Witness forward/Start/refusal/cleanup failure must keep overall acceptance failed; never invent final charge release.
- Exactly one original witness consent/POST and one release; zero target replay. Final expected transport totals remain unchanged.
- Run only the affected scenario/proof/guard/source-binding suites after the extension; no repeated full frontend suite for this helper-only change. Freeze exact product/helper/image/package identities before the fresh `--charged-witness --require-public-reservations` installed run.

## 2. CLOUD-038 first-observer ordering cannot be forced by the current Docker proxy

Source path:

- `project_check_runtime.go:56` starts an independent **2-second** observer ticker, each pass bounded to **3 seconds**.
- `ObserveChecksOnce` selects exact private ledger rows, then `observeCheckCandidate` acquires a per-child lifecycle lock. Lock acquisition is capped at **1 second** (`execution_lifecycle_lock.go:84`).
- For `creating` / `fencing`, `check_observation.go:156` only reads local state, preserves/normalizes the message, and calls `RecordProjectCheckObservation`. **It performs no Docker request in this branch.**
- The original check controller holds that child lock through Create and publication. After it publishes the unknown result and returns, the lock is released without any later proxy-visible boundary (`check_execution.go:99,145`).
- Recovery GET reads the current check plus exact lease and computes the digest. It neither invokes nor pauses the observer (`check_recovery.go:122`).
- The current installed helper deliberately settles/read-checks before opening review (`recovery-scenario.mjs:165`), so attempt 8 does not establish view-before-first-observer ordering.

Consequently, a Docker gate cannot pause the relevant first observer write or prove that the first visit has not occurred. Holding the target's Create response holds its active controller too; reading review before the unknown receipt then tests a **meaningful running→unknown revision change**, which must invalidate review, not CLOUD-038's copy-only normalization. Holding some unrelated Docker Inspect is not enough either: it cannot reliably identify the calling observer, and bounded/fair observer passes can advance beyond that peer. Timing a GET inside the 2-second interval, assuming absent mutations mean no observer ran, blocking every Docker request, pausing the whole process, or externally forging/locking state cannot produce trustworthy deterministic evidence.

**Recommendation:** do not add a proxy-only `--review-before-first-observer` flag that claims to guarantee this order. It would be false assurance. Keep the exact installed-ordering gap explicitly unverified until there is an honest observation/control seam.

### Existing deterministic proof and next useful native check

`TestProjectCheckHTTPUnconfirmedCreateObservationPreservesReview` already uses production HTTP publication/review/recovery with a controlled backend. It captures the review before any explicit observer call in the `creating` case, calls the real observer four times with a reopen, requires unchanged receipt/revision/digest/charge, checks current-authority refusal, then submits the original exact review for recovery. The fencing branch covers meaningful allocation/output changes and stale rejection. The old source fails the same regression; the fixed source passes. This is sound deterministic source evidence, not installed Docker or native UI evidence.

The minimum unperformed Windows addition is that **existing test on a fresh pinned Windows checkout**, when the local tester's resource gate permits it. Do not rerun unrelated passing suites. Example from `control-plane` using the task's configured Go/cache/temp:

```powershell
$env:GOMAXPROCS = '2'
go test -p 1 -count=1 -timeout=8m ./internal/api -run '^TestProjectCheckHTTPUnconfirmedCreateObservationPreservesReview$'
```

Preserve source HEAD/hash manifests and exact output. This would verify native locks/path/store and controlled ordered recovery; it still would **not** prove a packaged Windows observer timer interleaving or actual Docker fence.

### If an installed ordering witness is still mandatory

There are two honest further options, neither implemented here:

1. A dedicated integration harness using production HTTP, real Docker backend and an explicitly controlled observer scheduler. Capture the actual UI review, invoke the real observer, then submit the unchanged reviewed UI request. This needs its own artifact and authentication/source guards. It is a **controlled integration build**, not the unmodified shipping package; keep that label and exact test seam visible.
2. Useful production **read-only observation diagnostics**, with bounded per-allocation generation/attempt/outcome counters and exact-generation identity, could prove the order of a naturally occurring run. They must expose no private data to unauthorized users and never pause/skip observers or mutate admission. A receipt must show the captured review precedes the first exact observer event, and the same digest/revision survives that event. Such diagnostics alone do not force the ordering; if an earlier observer already ran, mark the case unexercised rather than retrying mutations or declaring success. Do not introduce a test-only production pause/debug endpoint merely to manufacture a green acceptance result.

The first option is deterministic but has an explicit test seam; the second can supply honest shipping-package observations but is not guaranteed to hit the order. Combining the current deterministic regression with attempt-8's real packaged authority/UI/fence/release evidence is a legitimate **layered proof with a stated boundary**, not a claim that the exact missing installed ordering has been observed.

## Delivery boundary

Implement only the witness-checkpoint extension after source review; no shared proxy or sustained files need changing. Freeze new helper bindings and run a fresh charged witness only if actual capacity permits. Retain the precise CLOUD-038 native/installed-ordering boundaries. Neither this plan nor a duplicate default recovery pass completes the remaining local acceptance.

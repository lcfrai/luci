# Installed dependency attempt 11 — scoped workflow passed

Exact clean product source: **`4df94ab9dad0e9fedd28a873bbadc56720e1402e`**. Exact clean helper source: **`d6949ac18e4f72fc2dc286627dc5bddc4067e36d`**. This installed Linux Docker/native Codex workflow **passed**, including the actual Luci Docker whole-build check and explicit joining of its evidence. Wrapper exit was **0** after **309.889 seconds**. It used a **scripted provider**, with eight local provider requests and **zero external model calls**. This is a scoped dependency/conflict/check workflow, not complete local-product, sustained four-project, Windows, live-model or deployment acceptance.

Retained provenance identifies Linux archive SHA-256 `0ac05247a194dd3cd8851c2111b1eda5906b399dbb892438484ff2b508c80d1d`, binary SHA-256 `6d6b11dd4382db11161384b23a55f1596e948f68c5a049a7e0aa7e409c724a9c`, 2,021 archive-matched package members, and sandbox image `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`. Those are retained local build/run assertions, not independent remote retrieval of the binary artifacts. All 18 archived helper files bind byte-for-byte to the pinned helper revision.

## Dependencies, conflict and reviewed integration

Four workers—Alpha, original Beta, replacement Beta and dependent join—exited naturally. Each had a clean native exit frame, exact container absence, completed queue state and observed disappearance of its own charged task lease. One coding dispatch per worker was recorded; none was integrated before exit.

Six HTTP 409 probes proved the join remained unallocated before dependency starts, after original exits, after Alpha alone integrated, while original Beta's conflict was retained, after retiring that conflict without a replacement, and after replacement Beta exited without integration. Each retained source/revision/effects and the exact unallocated join identifiers.

Three reviewed integrations succeeded: Alpha, replacement Beta and join. Original Beta remains `unknown` with its source/worktree/output hashes and actual Git conflict preserved. The recorded UI states that integration did not complete and dependencies still require reviewed integration. It does not claim that a retained conflict succeeded.

After Alpha integration, six complete source reads and the visible UI boundary preceded the stale-review request. The exact request returned the review-specific HTTP 409 refusal; revision 17, primary commit/clean status, 2/2/2 Create/Start/Destroy effects and absent integration checkout remained unchanged. The raw diagnostic's `unclassified-response` label is preserved alongside its specific body rather than retrospectively rewritten.

The join only launched after both current dependencies were integrated. Its output was integrated into exact combined commit **`7cd47dd709296df5a90411326b8de875935e81cd`**. The independent fixture command `node verify.mjs final` passed first. That independent command is recorded separately from the following actual Luci check.

## Actual Luci check and explicit evidence join

Check **`check-4d62db8779e121849ce2683d`**, allocation **`checkexec-0aeda4ff1ba6f1fea8c25448`**, ran the real Docker command `node verify.mjs final` on the exact combined commit. It returned exit **0**, output **`DAG_CHECK_OK final`**, `status: passed`, `sourceChanged: false`, **`cleanupConfirmed: true`** and **`resourcesHeld: false`**. Source digest is `3d3afe7ad36efea304cf6ae1110189e5eeea02a6092e6521bc8725067a7c4080`. The full raw command record retains a stale “cleanup is pending” message alongside the authoritative final cleanup/status fields; no raw text was rewritten.

The fifth Create was the check; the other four were workers. Exact check container **`533cd674db7831e73c953315f662707ddca56ef314373398c0393b02f2c004fb`** had these recorded transport outcomes:

| Operation | Acknowledgement time (UTC) | Result |
|---|---|---|
| Create | `22:14:26.075Z`; delivered `22:14:26.077Z` | HTTP 201, exact container ID |
| Start | `22:14:26.287Z` | HTTP 204 |
| Delete | `22:14:27.779Z` | HTTP 204 for that immutable ID |

Ten pre-forward Create-identification diagnostics are retained, including both exact check snapshots. They preserve bounded identity/body-size/hash metadata, not complete request bodies. Their `forwarded: false` means the snapshot was taken before forwarding; later transport records establish actual forwarding and acknowledgements.

A separate explicit join bound this passed check to requirement **`AC-dag`**, specification/design revision **2**, exact approval IDs/digests and the same source commit/digest. The joined record retains the original check. The product run advanced to **`awaiting-release`**, as shown in `DAG-verified.png`. Passing commands and linking evidence do not automatically prove all completion criteria; no preview, release or deployment occurred.

## Capacity observations and precise limits

The helper preserved two fresh production-capacity readbacks: before submission and before forwarding the check Create. Both recorded effective host availability **15,200,002,048 bytes**, host effective total **34,359,738,368 bytes**, and root free space above the unchanged **6,442,450,944-byte** pre-Create threshold. Before forwarding, one exact check reservation was present and `selfReservationCountedOnce: true`. Image virtual size was 2,294,629,077 bytes; the VFS copy estimate was 4,589,258,154 bytes. These are the helper's cross-check receipts, not every internal production admission decision.

The C030 observer captured **305 samples over 309.687 seconds**, with **282** valid exact control-plane PID/start cgroup observations and **23** explicitly unobserved rows. Effective available memory over the whole run ranged from **10,192,949,248** to **15,794,905,088 bytes**; minimum physical available memory was **21,433,262,080 bytes**. Minima can occur at different instants. Effective and physical memory are different scopes and must not be substituted for each other.

During the check workflow interval, 36/36 samples had valid control-plane cgroup observations. At check Create acknowledgement/Start, nearest bracketing samples recorded effective availability approximately **10.313→10.278 GB** and root free space approximately **1.859→1.836 GB**. The lower disk reading is after the copy has occupied space; the unchanged working reserve remains 1 GiB. [Resource observations](analysis/resource-observations.json) retain exact sample values, timestamps, offsets and raw-source bindings.

These approximately 1 Hz sequential counters are **not atomic Create/Start admission snapshots**. They do not identify the cause of older memory-pressure refusals or isolate a performance effect of CLOUD-029. Compared with earlier attempts, environment/cache/storage state changed; no matched causal before/after experiment was performed and no speedup is claimed. Host cgroup counters include unrelated work, retained RAM-backed files and observer activity. Hidden ancestors are not inferred.

Peak summed owned-process RSS was **1,194,475,520 bytes**, including driver/browser and double-counting shared pages. Separately, the maximum sampled worker cgroup memory was **49,467,392 bytes**. Check coverage was **`identity-only`**: two check identity observations and **zero check cgroup-usage samples**. The short check was actually executed, but its memory peak was not measured; zero samples must not become a zero-memory claim. These overlapping memory scopes must not be blindly added.

Observer interpreter cost was **62.691 CPU seconds over 309.754 wall seconds** (about 0.202 core on average), peak RSS **19,267,584 bytes**, excluding spawned Docker CLI/helpers and daemon work. There is no matched no-observer control, idle baseline, UI latency, sustained mixed-project throughput or whole-computer responsiveness result in this workflow.

## Cleanup and publication

All controller/proxy/browser/provider cleanup phases and five startup acquisitions closed, with no cleanup error. The proxy drained with zero pending request/handler/body bytes and no mutation uncertainty. The final driver records zero owned service/browser processes, Docker containers and volumes, and unchanged protected package/image/helper identities. The [independent closure](independent-closure.json), at `22:15:37.765551Z`, separately found zero Docker containers/volumes and the exact version-1 empty ledger, SHA-256 `c0022f0995797c072d8097729ea90e74487bcefb911576c91e4af0665403a49e`. Of 13 sampled PID/start identities, two exited Chromium zombies remained adopted by PID 1 (`2020719` / `2020722`); no active sampled process or unknown check remained. These literal observations are not “zero remaining PIDs” and do not establish absence of never-observed descendants. Independent inspection made no ledger repair or allocation removal.

`publication.json` hashes every other member, with original-path/byte/hash bindings. Journal, observer samples and lifecycle use deterministic gzip and decoded hashes. The complete final/incomplete receipts, 87-row journal, six dependency probes, source-read cohort, stale-review refusal, ten Create diagnostics, preserved conflict/patch, three screenshots, sampler/helpers and package/image provenance remain accessible. No credential-bearing fixture home/configuration, account/session database, source/state tree or conversation was exported. Only explicitly scoped metadata embedded in the independent closure is included.

Reproduction uses the exact pinned package/image/helpers, a fresh isolated fixture/output directory, exclusive safe Docker inventory, and the command/unchanged bounds in `invocation.json` and the pinned `scripts/acceptance/task-dependencies/README.md`. Original paths identify local evidence, not downloaded artifacts. No workload, browser, Docker command, compiler or test suite was rerun to publish this capsule. Prior failed attempts remain failed and immutable; this pass does not repair their retained leases or verify unknown-Create recovery, Windows/native worker behavior, real-provider quality, sustained four-project efficiency, preview or release.

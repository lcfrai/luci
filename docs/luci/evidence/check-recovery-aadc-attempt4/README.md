# Check recovery attempt 4 — historical check blocked before Start

This actual installed Linux attempt used matching product/helper **`aadc1c281aef28aee04046a5c5e58da5fe7094e4`** and made **zero external model calls**. It failed at `consent-check-1`: the initial historical check was blocked by **`memory_pressure`** before Start. The target unknown-Create and recovery stages were not reached. Wrapper exit was **1**; driver timestamps are `2026-10-09T00:29:16.605Z`–`00:29:45.981Z`.

The [actual invocation](invocation.json), all **11** pinned helper copies and the original failed receipt are retained. Source copies match the `aadc1c28` Git blobs, independent of later worktree changes. The manifest's historical product/helper defaults are not the runtime identities above. The driver verified **3,126 matching package members**. Archive SHA-256 is `e73f44b5dd59d7f8205661f821d556cfcaeb43b0f765755ae7e6b5c81d20d38d`; binary SHA-256 is `5d70f3d2fe83d49ac9daabd207a0251fffde5fdb181f9249ed28b92278f90763`. Image is `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`. These are local verification receipts, not an archive upload or independent retrieval claim.

## Recorded refusal and cleanup

Docker transport recorded **one Create / zero Starts**, with no workers or recovery fences. The Create returned 201 at `00:29:43.386Z`. After its acknowledgement was released, the product refused the pre-Start admission check. The check and command are **`blocked / admission / memory_pressure`**, with the message “Available memory cannot preserve the configured headroom for another worker.” The command has no exit code or output, **`cleanupConfirmed=true`**, and the check records **`resourcesHeld=false`**.

The exact created container was deleted with HTTP 204 at `00:29:44.255Z`; its absence and empty ledger are retained in the final receipt. The scenario required a successful historical check, so its assertion failed before any target check or recovery. No threshold or assertion was relaxed.

## Timestamped capacity observations

The original phase snapshots retain separate capture and host-observation timestamps. Values below are **bytes**, with UTC times on 9 October. They are not the exact atomic admission inputs.

| Phase | Capture time | Host observation time | Physical available | Effective available | Public reservations |
|---|---|---|---:|---:|---:|
| 01 — fresh Create admission | 00:29:21.968 | 00:29:21.661494586 | 19,290,828,800 | 12,731,179,008 | 1 |
| 02 — Create ACK held before Start | 00:29:43.399–43.402 | 00:29:42.660834616 | 19,307,147,264 | 8,077,209,600 | 1 |
| 03 — terminal check | 00:29:44.476–44.479 | 00:29:43.661607216 | 19,271,032,832 | 9,178,939,392 | 0 |

Effective total was **34,359,738,368 bytes**, limited by the visible control-plane cgroup; physical total was **35,743,784,960 bytes**. Physical and effective availability describe different scopes. The approximate **12.731 GB → 8.077 GB** effective-availability change is a timestamped observation. It does not identify a cache, process or other cause, reconstruct the exact pre-Start guard input, or explain an older run's refusal. Shared cgroup use is not attributed entirely to Luci. The conservative headroom guard remained intact.

[Resource observations](resource-observations.json) bind the original phase/sample hashes and retain nearby raw cgroup observations. The observer recorded **50 samples**: **28** with cgroup records and **22** without. The **56** cgroup observations cover two process identities sharing one visible cgroup location; they are not 56 independent groups. Check coverage is **one identity sample / zero cgroup-usage samples** (`identity-only`). There is no pre-run idle baseline, sustained-performance result, atomic admission trace or causal observer-overhead measurement. All raw samples are retained, including missing observations.

## Independent closure and its limits

Controller/proxy/browser cleanup and all five startup resources completed. The proxy drained with no pending work or uncertain mutation. The final receipt recorded zero containers, volumes, active checks and fences.

The [independent read-only closure](independent-read-only-closure.json), `00:32:45.978421Z`–`00:32:46.215446Z`, separately checked actual Docker inventory, the exact created container, retained PID/start identities and the parsed ledger. Docker containers and volumes were empty; the created container was absent. The version-1 ledger had zero queue/check leases and matched the final receipt's SHA-256 `c0022f0995797c072d8097729ea90e74487bcefb911576c91e4af0665403a49e`.

Of **14** retained process identities, **12 were absent and two were exited zombies**; none was active. No exact fixture-environment match was found among readable processes, while **5,846** other process environments were unreadable. This is not universal process or descendant-history absence proof. All **108** recorded fixture files remained byte-identical before/after the read-only checks. No process was stopped, Docker object removed or ledger repaired during publication. These closure observations do not establish a recovery scenario that was never reached.

## Publication

[`publication.json`](publication.json) binds the full failed report, all three phase receipts, journal, complete observer evidence, actual invocation, accepted sources and derived observations. Gzip is deterministic and decoded hashes are retained. Package host paths are omitted in the named sanitized copy. Exact private roots and available retained credentials were checked in memory against exported JSON strings and decoded payloads. Raw fixture paths, credentials, per-file inventory and ledger contents are excluded; no known match remained. Ephemeral credentials no longer retained cannot be re-read.

Preparation used original-artifact reads, Git blobs and the explicit read-only closure commands. No new worker, browser, build or tests were run. Original failed reports/fixtures remain unchanged. Future attempts need fresh fixture/output paths and unchanged admission rules; this attempt supplies no target unknown-Create or recovery acceptance.

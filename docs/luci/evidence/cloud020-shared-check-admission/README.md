# Shared admission and durable recovery for project checks

CLOUD-020 is implemented in the source accompanying this receipt. New project checks use the configured Docker SDK backend and the same private cross-process capacity ledger as agent work. A pending check is charged before Create, across personal/team workspaces, with fresh authority and host/guest memory, CPU and storage checks. It does not consume a provider slot. Historical host checks remain readable; new host checks are refused because their limits cannot be enforced.

The backend requires memory, combined memory/swap, CPU quota and PID enforcement where requested. Container memory-plus-swap equals the memory limit; configured limits are not measurements. An acknowledged immutable container identity is persisted before Start. Exit/output, cleanup and final publication are separate durable steps; only confirmed absence releases capacity. An uncertain Create retains its charge and requires an explicitly reviewed permanent name fence before recovery. The app never treats a timeout, missing response or one 404 as permission to replay.

Current Edit authorizes safe stop/recovery independently of Execute, while creating work requires both. A stale controller that loses its reserved allocation exits without creating a new lease or overwriting a settled receipt. The original deterministic HTTP regression reproduced one Create/Start after retirement; the corrected regression preserves the entire settled check-history bytes with no allocation or charge.

## Evidence, including failures

All counts are Go test/subtest outcomes and overlap. They must not be summed.

| Run | Result | Scope |
| --- | --- | --- |
| Staged diagnostic 1 | 142 pass, 3 fail; adapter build failure | Initial controlled integration failures, retained. |
| Staged diagnostic 2 | 219 pass, 4 fail | Adapter ownership predicate failures retained; API/product/authority checks and LOCAL-012 restart-before-admission pass. |
| Adapter correction | 85 pass | Corrected typed capability-slice predicate and negative security/configuration cases. |
| Stale-controller original | 1 fail | Exact reserved-allocation resurrection and publication overwrite reproduced. |
| Stale-controller correction | 13 pass; affected lifecycle 31 pass | Exact claim guards and retained settled receipt, controlled race tests. |
| Full actual-checkout race run | 3,187 pass, 1 fail, 29 skip | All package results retained. API ACK timing assertion failed; product and other packages passed. This is not a clean full-suite result. |
| ACK test synchronization correction | 34 pass, 0 fail/skip | Actual-checkout API/project lifecycle plus exact emitted HTTP contract. Waits for finished noncharging receipt ACK, without forcing cleanup or relaxing assertions. All 802 inputs unchanged. |
| CPU/PID capabilities original | 5 failing outcomes | Before-Create refusal regression against previous implementation. |
| Complete Docker adapter after correction | 221 pass, 3 skip | Controlled SDK fixtures, not an actual daemon workload. |

`publication.json` records exact published and uncompressed hashes. Large raw logs and source snapshots use deterministic gzip. For example, `python3 -m gzip -d full-race/stdout.jsonl.gz` makes a disposable decoded copy; keep the original archive. Each run retains its own command and before/after source hashes. `current-source.json` binds the final control-plane publication, not a retrospective claim that earlier tests used final bytes. Historical overlay paths describe those original runs. Unfixed source bodies remain retrievable for the regressions.

The ACK test originally watched the in-process job map, which can become idle before durable final publication acknowledgement removes a finished, zero-demand receipt. The bounded replacement accepts only that exact finished identity and its matching result digest until it disappears; fault tests and the generic idle helper are unchanged. Full-run failure and original test bytes remain under `ack-synchronization`.

## Four-project restart and remaining boundaries

`TestSwarmTerminalCleanupRestartBeforeWaitingProjectAdmission` now restarts before B/C/D receive their first allocation. A's exact stopped-but-present container retains its charge; the remaining projects pass normal capacity admission, the fifth slot is refused, and only confirmed removal releases A. This passes in staged diagnostic 2 and the actual-checkout full API run. It closes the earlier missing controlled scenario for LOCAL-012; actual Docker and native Windows verification are still separate.

New check allocation calls occur outside the global ledger lock. The inherited shared worker reconciliation can perform exact terminal observations inside that lock with its bounded shared context. This is an explicit remaining contention measurement point, not a claim that no backend call ever occurs under the ledger lock.

These receipts establish controlled source behavior. Matching installed SDK wire compatibility, interrupted Create/Start/ACK recovery, native Windows runtime behavior, measured mixed worker/check resources and sustained four-project responsiveness remain required. Neither scripted providers nor passing fixtures establish live-model quality. Whole-state process exclusion is documented separately under [CLOUD-022](../cloud022-state-single-writer/README.md); the corresponding [interactive check recovery](../cloud020-check-recovery-ui/README.md) has its own source/browser scope.

The later [CLOUD-024 correction](../cloud024-check-observation-order/README.md) preserves later command outcomes when earlier completed receipts are observed out of order. Its final 40 affected race outcomes pass, including the original passed/failed/pre-admission-blocked failures, without replay or weakening held cleanup. This supersedes earlier observation source hashes; the original full run and 34-result ACK follow-up retain their own source identity.

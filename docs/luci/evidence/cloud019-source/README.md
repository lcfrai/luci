# CLOUD-019: retained start refusal and truthful failed-attempt status

Luci now records a bounded `startFailure` stage, reason code and fixed public message when its allocator refuses **before calling Backend.Create**. The exact session and task retain that diagnostic. Their queue entry is terminal `failed`, contributes to `counts.failed`, and cannot silently resume or replay. Home identifies the blocked run. A never-started attempt produces no worker report or verification result; its captured context and checkout remain inspectable.

An abnormal worker outcome is separate from normal process exit awaiting review. Backend Create/acknowledgment errors for bound run children remain **unknown**, because a failed response cannot prove a delayed Create/Start is impossible. The diagnostic changes no lifecycle, backend-absence, fence, resource-admission or permission proof. Failed-attempt capacity can remain held; the hold wording disappears only after the existing release check succeeds.

The original installed `3b8054e6` B1 failure reached the final pre-allocation check without calling Backend.Create. Its specific reason was discarded by that version and remains unknown. These controlled regressions reproduce several possible refusals independently; they do not identify which occurred in that installed attempt. See the separate [installed-failure evidence](../cloud019-installed-failure/).

## Source-bound verification

Counts include subtests. Every run used Go's race detector with `GOMAXPROCS=2`, `-p1`, and `-count=1`; commands, UTC times, package results and raw-log hashes are retained in [verification.json](verification.json).

| Run | Passing outcomes | Failures | Skips | Unchanged Go/module inputs |
| --- | ---: | ---: | ---: | ---: |
| Initial diagnostic | 40 | 0 | 0 | 681 |
| Full affected API/product/registry | 1,747 | 1 | 8 | 681 |
| Final targeted correction | 102 | 0 | 0 | 682 |

The full affected run's one failure was an older test expecting `unknown` after a positively recorded pre-backend authority refusal. Its corrected assertion requires `failed`, the exact safe reason, zero Backend.Create calls, and unchanged diagnostic on repeated dispatch. The full failed report is preserved. Product's 848 outcomes and registry's 81 outcomes passed in that full run; API recorded 818 passes, the old assertion failure and six skips. The final targeted run covers the changed boundaries, rather than claiming another full-suite pass.

The tests exercise actual HTTP queue/manual routes, private Git/context/registry persistence, shared execution admission and reopening with a **controlled backend**. They cover authority revocation, stale observations, memory pressure, retained unconfirmed capacity, manual completion observation, scoped Home counts, restart/no replay, safe error projection, ordinary interactive restart, late backend acknowledgment, and registry observation generation. They invoke no Docker or model provider. Native Windows execution and a matching installed corrected DAG remain separate gates.

The base commit is `3b8054e69340bce37afe3aec17189a33c2fb2991`, with uncommitted changes during verification. The three input manifests identify the exact tested bytes; the base commit alone does not. Final input hashes were rechecked at publication. Private raw logs remain under ignored artifacts and are referenced by SHA-256, not copied into this public receipt.

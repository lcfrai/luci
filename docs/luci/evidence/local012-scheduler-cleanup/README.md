# LOCAL-012: retained cleanup and project scheduling

The controlled four-project regression now passes. A failed task with its exact
stopped Docker object still present keeps its reservation and retained files;
three other projects can use the remaining capacity. An additional allocation
still hits the configured four-slot limit.

The scheduler continues only after verifying the original engine and instance,
immutable object reference, ownership labels, exited registry generation and
settled private launch receipt. Unknown, running, mismatched, legacy or unavailable
objects remain conservative. The classification does not release capacity or
authorize cleanup, replacement or integration.

[The receipt](verification.json) records the exact commands, source hashes and
scope. The tested tree contains changes over `b1d5aeaabc5876755b86b1ecb0f4c1923dfb90b9`;
that base revision alone does not contain this correction.

| Run | Result | Interpretation |
| --- | --- | --- |
| Initial fixture | Failed | Missing fixture runtime callbacks caused `scheduler_unavailable` before the bug. |
| Corrected fixture, original production | Failed | Terminal cleanup stopped the turn before B/C/D could progress. |
| Corrected production, focused race | 35 passed | The identical four-project regression and negative classification checks passed. |
| Affected API/product race | 128 passed | Scheduler, cleanup, uncertain launch, failure diagnostics and completion-observer regressions passed. |

Both successful runs had zero failed/skipped outcomes and unchanged source
manifests. Their scopes overlap; the counts are not separate unique coverage.
The four-project test checks repeated turns, service/store reopen, unchanged
attempts, retained failure guidance, the full 4 GiB charge, a refused fifth slot
and no scheduler removal calls. Negative cases retain refusal for uncertainty,
changed ownership/engine, cancellation and corrupt shared state.

Run the focused checks from the repository root with the pinned Go toolchain:

```sh
GOMAXPROCS=2 go test -race -p 1 -count=1 -timeout=5m \
  ./control-plane/internal/api ./control-plane/internal/product \
  -run 'TestSwarmTerminalCleanupDoesNotStarveOtherProjects|TestSwarmCleanupPending'
```

These are Linux tests with controlled backend states. Native Windows, actual
Docker cleanup behavior, live providers, sustained load and complete product
acceptance remain separate gates. No actual Docker or model execution occurred
in this batch.

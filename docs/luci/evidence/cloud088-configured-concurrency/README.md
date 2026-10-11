# CLOUD-088 — configured concurrency source checkpoint

This slice removes the fixed four-task/per-project and sixteen-workspace ceilings. Run and project limits accept1–200, matching the existing selected-task contract; the workspace accepts1–500, matching the existing durable queue limit. Defaults remain run2, project2, workspace4. A configured ceiling is not a throughput guarantee: actual task readiness, provider/account permissions, subscription1, RAM/CPU/storage admission and exact held reservations remain authoritative. The bounded four-admission scheduling turn and conservative128-object Docker observation budget are unchanged.

The shared Go constants were committed in `8805c8c61e51e533b8224d3d458def2dd2c80ea4` because the coordinator continuation references them. This checkpoint completes preparation/store/scheduler/provider-ledger validation and the two existing UI controls. It is not completion of CLOUD-088: genuine per-task microVM fan-out, durable artifacts, policy-controlled spending and live lifecycle acceptance remain open.

## Evidence

- The same two new regression files fail four top-level cases against five original production files through a Go overlay. The overlay never overwrites the working tree. Exact original copies and their hashes are retained under `before/`.
- Corrected focused run:4top-level+5subtests pass. Final affected race run:29top-level+40subtests pass,0fail/skip. The nine focused outcomes overlap the69race outcomes.
- Real product HTTP, reviewed Git task contracts, durable FileStore and shared admission ledger with controlled backend/telemetry prove six concurrent tasks in one project, seventh waiting, six across four projects with no project starved, exact6GiB/6000mCPU reservations, queue cancellation and restart without replay. Raising the ceiling to24 still refuses the next worker at the fixture's actual CPU capacity. A provider limit24 persists through ledger reopen.
- Existing affected tests cover subscription1, account/team limits, unknown allocations, current policy, absent telemetry, concurrent reservations, dependency integration and pause/cancel behavior.
- Frontend59/59 tests across the actual ProjectSwarm and HostCapacity components pass. Defaults, valid6/24 ceilings, invalid numbers, subscription and existing authorization/CAS/uncertainty paths remain covered. Final combined TypeScript checking is coordinated separately; no typecheck claim is made here.
-13selected source hashes are equal before and after Go runs. This is selected-source evidence, not a full compiler/dependency closure or clean shared-checkout claim. UI raw reporter source bindings were recorded after its run, so no UI pre/post stability claim is implied.

`product-first.jsonl` is a retained harness failure: the first shell wrapper used the wrong directory for copy/binding steps and then ran zero matching tests. That zero-test exit is not credited. Corrected compiled tests and logs follow separately. The original wrapper's missing-path/gofmt/Python errors are described in summary; its tool output was not written to a file.

## Reproduction

Use the pinned repository Go/Node activation. On Linux, from `control-plane`:

```sh
GOMAXPROCS=2 go test -p 1 -count=1 -timeout=4m -json ./internal/product ./internal/api -run '^(TestSwarmConfigured|TestConfiguredConcurrency|TestConfiguredProvider)'
GOMAXPROCS=2 go test -race -p 1 -count=1 -timeout=4m -json ./internal/product ./internal/api -run '^(TestSwarmConfigured|TestConfiguredConcurrency|TestConfiguredProvider|TestSwarmSchedulerShared|TestProjectSwarmHTTPSubscriptionPreparationRequiresSingleSlot|TestProjectSwarmStore|TestExecutionAdmission)'
```

From `desktop`:

```sh
npm test -- --run src/components/journey/ProjectSwarm.test.tsx src/components/projects/HostCapacity.test.tsx --maxWorkers=1
```

To reproduce the original refusal without modifying sources, run `python make-before-overlay.py <absolute-temporary-overlay.json>` from any directory, then add `-overlay <that-file>` to the first Go command. Four original failures are expected; no original product success is claimed.

## Windows handoff

No configuration, migration, runtime image or credential change is required. Use an isolated checkout and the existing task-local supported Go/Node activation and private TEMP/caches. These controlled tests create disposable Git projects and registry state; they require no Docker or provider calls. Run each package separately with `GOMAXPROCS=2`, `-p1`, `-count=1`, a10minute package timeout and the first regex above, retaining output, source hashes and exact HEAD before/after. Linux race evidence does not establish native Windows behavior.

In the actual application, review a run with more than four independent tasks, open Prepare team and choose6; inspect the unchanged subscription restriction when applicable. Admins can review workspace24/project6 through existing capacity settings. Changing a ceiling does not enqueue work; existing explicitly consented queued tasks may become eligible and start. Under a genuine permitted execution backend, verify normal authority/resource refusal and seven-or-more task scheduling, cancellation/reopen and exact reservations; do not assume the configured ceiling overrides machine capacity. Preserve the representative twelve-task/four-project/four-concurrent-microVM contract workload as the live acceptance gate. No latency, full product, Windows or live microVM claim comes from these fixtures.

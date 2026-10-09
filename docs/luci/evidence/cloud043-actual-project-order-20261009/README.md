# CLOUD-043 / LOCAL-021 — observe actual cross-project allocation order

P1 acceptance-harness correction. This capsule does not establish an installed or Windows pass, live-model quality, or sustained performance.

## Original failure and scope

The unchanged sustained attempt5 used product `2da1a821b8456c20a858fb86ef87649ea917d9de` and helper `e2f426bb50267a7cceac1f53eb1e00a0a4cc278e`. A and B each produced an accepted command receipt and naturally exited; A was integrated. The scheduler then actually created and started D, while the helper polled only C. D's native opening prompt therefore waited for a task bundle that the driver never registered. This is the provider's bounded `Controller verifies actual task bundle` failure, not a CLOUD-041 receipt-parser failure.

`retained-selection.json` and `final/fixtures/queue-order-retained.json` contain bounded allowlisted projections: exact project/task/run/queue/session/operation operands, final queued C and running D state, the last six C queue GETs, D's exact opening-task line and its full-text hash, and the exact third Create identity. Queue grouping metadata in this projection is not an added field in the production HTTP contract; the actual HTTP component test deliberately uses the real queue shape without a top-level projectId. Private source hashes are recorded but their full original registry/rollout/fixture bytes remain privately retained. This publication is not independent verification of those excluded bytes.

The parent publishes the complete original failed run and independent closure separately. Its third D charge remained backend-settled at 1024 MiB/1000 mCPU after failed-fixture container cleanup. No original fixture, reservation, provider request, output or history was modified for this correction. Empty Docker inventory is not treated as product capacity release.

## Final helper behavior

- Discover the first worker from all four explicitly prepared projects, then discover every successor from the remaining set. Never infer cross-project order from enqueue array order or a per-project priority mutation.
- Validate exact project, task, run and original enqueue identity, a single running candidate, and the existing session/operation/worktree/context/container guards before registering the bundle or issuing its command.
- Pause only A's exact `cancel-a` entry through the real UI while fresh queue observations still report global dispatch paused. This prevents an eligible extra task starting if build-a or other peers are temporarily deferred. During real loaded execution, cancel that exact paused entry through the UI and retain zero child/operation evidence.
- Raise priority through the UI on a different waiting project. Record the exact scoped high-priority receipt; do not predict that project's next turn.
- Restart the actual first worker once, complete and review all four actual tasks exactly once, and preserve pre-integration peer progress. The reviewed amendment still targets A.

The source-span hashes in `unchanged-guards-final.json` prove the existing worker context block, provider contract, exercise/review functions, loaded minimum 200 switches/400 inputs, 150 seconds per task/600 aggregate fuzz minimum, four initial results, amendment, final five Creates/Starts and natural completions, and latency thresholds remain unchanged. No admission or product scheduling policy changed.

## Validation

- **Original reproduction:** the identical final test file against exact original helper gives **24 expected failures and 1 passing A→B→C→D control** (25 selected tests). This covers the retained D-before-C operand and all 24 startup/completion orders. Raw before output is retained.
- **Final affected pure batch:** **98 tests pass, 0 failures, 0 skips** across seven files. This includes 37 selection/control tests, all 24 complete orders, non-A initial selection, initial canceled-child refusal, wrong/missing/duplicate identity and context, one-worker bounds, target/readback refusals, source/lifecycle/context/receipt guards and generated task-command tests. These execute actual source slices with inert adapters; they do not execute a scheduler, container, native worker or real UI.
- **Actual component/browser:** **8 checks pass**, each of four possible first workers at 1440 and 390 pixels. The complete real SwarmRunQueue and swarmQueueApi run with controlled HTTP. Exact extracted helper action/control functions issue pause with current revision 2, loaded cancel with revision 3, and priority with revision 2 on the correct other project. Real collapsed controls are opened. Source bindings include 425 repository UI/config/helper inputs before and after. This is not full AppShell or installed acceptance. Browser and Vite both closed.
- The preliminary final-candidate 37 tests overlap the final 98 and are not added together. Earlier intermediate candidate results are preserved: 8 passes; 65 passes plus one missing external copied-module test-file load failure, followed by only the affected 4 tests passing after copying the unchanged module. The intermediate candidate still assumed A then B and was rejected in source review; its results are not the final correction. Its original 2 failures/1 control also overlap later evidence.

`all-orders-source-before.json` and `all-orders-source-after.json` match all 25 selected helper/module/fixture inputs. `candidate-bindings.json` binds the three changed repository files and exact patch. Shared lifecycle sources are frozen under `shared/`; unchanged copied modules/fixtures are retained under `final/`. The final browser runner, fixture, source manifest, raw receipt and eight screenshots are retrievable under `ui/`. No performance improvement is inferred from these checks.

## Reproduction

After the parent integrates the three changed files, from the repository using its supported Node runtime:

```sh
node --test scripts/acceptance/sustained-four-project/{helper,lifecycle,queue-selection,receipt-transport,source-binding,task-command,task-context}.test.mjs
LUCI_SUSTAINED_TEST_SOURCE=/absolute/path/to/this/capsule/before/sustained.mjs \
  node --test --test-name-pattern='retained D-before-C|actual startup and completion loop' \
  scripts/acceptance/sustained-four-project/queue-selection.test.mjs
```

The second command intentionally exits 1 with the original failures. Do not relabel it as a failed corrected run.

For the exact controlled browser fixture, use the original Linux layout `/workspace/luci` with its existing locked desktop dependencies and Chromium; copy this capsule's `final/` to `/workspace/.luci-tools/cloud043-queue-order/files/`, and `ui/{source-browser.mjs,fixture.tsx,source-bindings.json}` to the corresponding `ui/`. The fixture's relative imports refer to that checkout. Run only after releasing any exclusive installed/performance witness, with a fresh output directory:

```sh
LUCI_CLOUD043_WINDOW=controlled-component \
LUCI_CLOUD043_OUT=/workspace/.luci-tools/cloud043-queue-order/ui/new-attempt \
  node /workspace/.luci-tools/cloud043-queue-order/ui/source-browser.mjs
```

The captured invocation used an outer 180-second Python timeout. The runner's actions have browser/API limits and browser/server shutdown waits are each bounded at 15 seconds. Exact command receipts and all retained failures are under `runs/`. No Docker daemon or provider is contacted by this fixture.

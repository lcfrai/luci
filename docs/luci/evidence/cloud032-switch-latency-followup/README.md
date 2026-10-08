# CLOUD-032 compatibility and redundant-update follow-up

The integrated frontend run passed **2713/2713 tests**, then TypeScript failed with **TS2550** because the first absent-draft guard used `Object.hasOwn`, which is outside the current configured library target. That failed source and exact compiler output are retained here; the prior [switch-latency capsule](../cloud032-switch-latency/README.md) is unchanged.

The guard now uses `Object.prototype.hasOwnProperty.call` with the same behavior. Compiler targets and configuration are unchanged. An additional small Home change keeps summary/queue-selection maps when membership pruning has nothing to remove. It keeps no state across visits, changes no reads, and retains removal, authority and source invalidation behavior. The added component regression verifies that reordering retains valid selections and removing a project prevents its selected task from reaching the next bulk request.

Focused validation passed **19/19 store tests** and **33/33 overview tests**, in separate single-worker batches. The original integration count is not presented as a rerun after these corrections. The captured store stderr retains jsdom's unimplemented canvas diagnostic; test assertions passed. The final shared TypeScript check passed with **435 source/config inputs unchanged** across the run, after the separately owned C034 frontend files were frozen.

No browser or package measurement was performed for this correction. The earlier actual idle diagnostic remains **367.203417ms switch p95**, above the unchanged **300ms** target. Avoided state updates are source-supported; their latency contribution and the candidate's latency remain unmeasured. [Source review](source-review.md) states the authority requirements and precise timing limits.

From `desktop`, the executed checks were:

```sh
node node_modules/vitest/vitest.mjs run src/store/draftRegistration.test.ts src/store/sourceNavigation.test.ts src/store/projectNavigation.test.ts --maxWorkers=1 --no-file-parallelism --reporter=json --outputFile=<fresh-result>/tests.json
node node_modules/vitest/vitest.mjs run src/components/projects/ProjectOverviewBoard.test.tsx --maxWorkers=1 --no-file-parallelism --reporter=json --outputFile=<fresh-result>/overview-tests.json
npm run typecheck
```

Exact command arrays, timestamps, statuses and raw reports are under `corrected/`. `original-failure/` retains the integrated count, unchanged-input bindings and original failed compiler output; `vitest.json.gz` decompresses to the original report SHA recorded in `captured-binding.json`. `before/`, `source/` and `follow-up.patch` bind the correction. All four final owned source hashes are listed in `corrected/current-source-hashes.json`. The typecheck input list includes other agents' separately owned C034/C033 integration changes; those are not attributed to this narrow UI correction.

This capsule contains controlled test/source artifacts and hashes, with indicator-scan results; it contains no actual browser trace, credential state or fixture account files. The actual diagnostic's raw trace remains private. No threshold, metric wait, journey, sample count, or acceptance criterion was changed.

# CLOUD-078 — unchanged session polls avoid redundant store and preference writes

Source baseline: `786652704fa8bebd1ba1af41cfa1d414d7def100`. This is an external candidate; the repository was not edited. Parent owns integration and issue publication.

Every successful two-second session poll currently reconciles into newly allocated top-level maps, even when all retained rows and per-worktree ordered arrays are unchanged. Zustand persistence runs for every `set`, including a callback returning the current state. The candidate calculates the same reconciliation from `get()` and returns before calling `set` when both maps have exactly the same keys and value identities. Actual row, index, status, allocation-generation and timestamp changes still publish. No polling interval, server request, authority guard, admission rule or rendering requirement changes.

Only `desktop/src/store/slices/dataSlice.ts` changes in production. Its reconciliation and existing `sameSession` comparator remain unchanged. This deliberately preserves conservative reference comparison of nested start-failure objects; it does not claim to suppress every semantically similar response. Object dictionary insertion order is not the server's per-worktree ordering contract; ordered index changes still publish. The guard does not await or invoke user callbacks between reading and committing the snapshot.

The historical quiet-tick test expected one store notification, despite unchanged user-visible data. Its exact original bytes and excerpt remain in `before/` and `old-quiet-tick-expectation.txt`. The new oracle explicitly changes that observable contract to no notification and no preference write, while checking that the fresh network read and good-tick catalogue callback still occur. It does not replace a real state change with silence.

## Controlled evidence

The same new test bodies ran against original and corrected production using a Vite load overlay; no repository file was temporarily replaced. Original: **27 passed, 9 expected failures**, exit 1. Corrected: **36 passed, 0 failed, 0 skipped/todo**, exit 0. Both used one Vitest worker and bounded 90-second invocation limits; elapsed times were approximately 6.33 and 8.20 seconds, respectively. These are test-run durations, not an efficiency comparison.

The 36 assertions comprise 13 existing/updated poller cases, 6 unchanged generation/authority cases, and 17 new store cases. Empty and identical decoded rows, uncovered records, key completeness, server order, additions/removals, first empty indexes, all three timestamp fields, status, allocation generation, pending Kill/Forget and omitted held-row position are covered. Real changes retain exactly one notification and preference write. The fixed tests reach all no-op subscription/storage assertions. Baseline failures occur first at root snapshot identity; later assertions in those failing cases are not separately credited as measured baseline counters.

Raw commands, stdout/stderr, JSON outcomes, selected-source hashes before/after, overlay load logs and exact shared test-body hashes are retained. All selected repository files and overlay/test inputs stayed unchanged in both arms. Neither arm timed out. The complete installed transitive compiler/dependency inventory is not bound; package metadata and the repository lockfile are recorded. There is no full-suite or TypeScript result here.

## Reproduction and integration

Review `CLOUD-078.patch`. It includes one production file, the corrected existing quiet-tick test and one new focused test. Original source files remain in `before/`; proposed source is in `after/`.

`run-validation.py before` and `run-validation.py after` record the exact executed commands and refuse to overwrite their consumed receipts. They invoke the installed Vitest entrypoint with `--configLoader runner --maxWorkers=1 --no-file-parallelism`, an external cache directory and exact selected-source checks. Reproduce only in a separately prepared external directory against the declared source/dependency versions, with fresh path-bound configs and output receipts. Do not rerun consumed output paths or weaken a source mismatch. Configuration replacement is limited to the selected production module and identical proposed poller test; the new store test resolves imports back to the same source tree.

The capsule omits regenerable Vite cache content; it is not part of the evidence member list. No full repository/build copy was made. Failed receipts and originals are preserved. The parent must separately authorize broader validation, integration or installed execution.

This change addresses demonstrably redundant state/persistence work. It does **not** establish the cause of CLOUD-032, physical paint latency or a performance improvement. The original full-action 300 ms switch / 100 ms input targets and every failed runtime receipt remain unchanged.

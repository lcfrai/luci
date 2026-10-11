# Home phase diagnostic — eight rounds

The 11 October 2026 instrumented run captured all **32 actions and 96 numeric Home rows**. Raw capture and numeric completeness checks passed. This is a diagnostic result; it does not establish latency acceptance or a performance gain, and no product optimization is included.

| Observation | Samples | Result |
|---|---:|---:|
| Project switch p95 | 8 | 337.119671 ms |
| Input p95 | 16 | 57.339455 ms |
| Home handler maximum | 8 | 2.1 ms |
| Handler to Home layout-effect maximum | 8 | 9.7 ms |

The measured synchronous Home handler and persistence spans are small here; they do not support a storage cause for the longer switch observation. Switch/input p95 targets remain 300/100 ms and p99 must remain strictly below 1,000 ms. Eight rounds do not replace the required 100-switch acceptance sample. The retained 972d 400.974902 ms failure is unchanged. Nested browser spans overlap and must not be added; Node and browser clocks must not be subtracted. Render-entry to layout effect includes scheduling and commit, not isolated render CPU.

[`run-001/numeric-projection.json`](run-001/numeric-projection.json) is copied byte-for-byte, including every allowlisted action and numeric row; [`summary.json`](summary.json) retains exact summary precision. Projection SHA-256: `656db4445bae4491e49a6e2019662d9d64b7d899342f6080e1b13a154f5980b8`. The original raw result remains private and is bound only by SHA-256 `3b4c7e81784fb67908700e305df0bf8352d4d80d46e1458c322edf0afbb3aec0`; no raw result, credential report or saved-context report is included.

The wrapper exited 0 and observed its driver absent. Independent closure observed the exact driver/controller PIDs absent and an empty Docker container/volume inventory. **No independent browser start-identity census was collected.** These observations are not a whole-machine or zero-zombie claim; the wrapper explicitly does not certify all cleanup. Both exact receipts are retained.

The unmodified installed binary is from `577f7a0fdb039b7f875901a6a567d62fdfee88c8`; frozen lifecycle helpers are from `64829123a5799858b57616b711c84c61b945871a`. All 43 prepared inputs, six numeric overlay files, 17 actual helpers, seven frozen dependencies and their original manifests are under `prepared/`. The original prepared README and static receipt describe the earlier pre-run state and are preserved unchanged. Exact TypeScript/Vite invocation, output and exit-0 receipts and before/after frontend/dependency bindings are under `build/`; no build or browser run was repeated for publication preparation.

This is a review/reproduction-input capsule, not a relocated runnable installation. Frozen helpers retain their original absolute paths, source commits and hash assertions. A reproduction needs those pinned repository objects, Node v24.19.0, the bound dependencies, installed package, separately retained image provenance and exact image, a fresh approved build/output root, and the original exclusive runtime/resource controls. Do not execute the historical paths over the retained run or silently rewrite bindings; use a fresh reviewed reconstruction if paths change. Dependency bindings cover 13 selected files, not the full Node toolchain or node_modules; browser version is absent from the public projection. The capsule does not claim exact cross-machine performance reproduction. The generated web and duplicate accepted-harness copies are omitted: their exact member hashes remain in the web binding, and preparation verified the accepted helpers byte-identical to the retained originals.

`copy-map.json` records every original-to-capsule copy. `verification.json` records this offline integrity check. `manifest.json` inventories the capsule payload; `SHA256SUMS` additionally covers that manifest. Publication remains the root agent's next-checkpoint action.

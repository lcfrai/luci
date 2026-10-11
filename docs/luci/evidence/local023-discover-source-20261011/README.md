# LOCAL-023 and project-name guidance: final source validation

One authorized run against the actual frozen working tree at HEAD `6f00501d7f370789ed45b018a7d958fd25cc1e2d` plus the recorded root changes. No source overlays or repository edits by this validation task; this is not a clean-HEAD execution claim.

Node **24.19.0**, unchanged repository Vitest configuration, **two workers**: **283/283 tests passed across the nine requested files**, exit **0**, **41.374694 seconds**, no failures, skipped/pending or todo cases. Exact file selection and raw JSON/stdout/stderr are retained under `frontend/`.

| File | Passed |
| --- | ---: |
| AppShell | 19 |
| DefinitionWorkspace | 25 |
| OrchestratorPanel | 115 |
| ProjectUnderstanding | 32 |
| ProjectPlan | 26 |
| ProjectWork | 26 |
| SetupPanel | 16 |
| WorkspaceInstaller | 3 |
| ProjectEntry | 21 |

Both added ProjectEntry cases passed: invalid new/import folder names retain the entered spelling, expose guidance, block the API/pending import, then submit a valid trimmed name once. The test's invalid examples include spaces, Windows-reserved `CON` and an overlong name. The batch retains existing component/navigation/approval behavior; it is not a full frontend-suite run.

The separate full desktop **`tsc --noEmit` passed**, exit **0**, **29.376682 seconds**, with empty stdout/stderr. Execution bounds were 300 seconds for Vitest and 180 seconds for TypeScript, with bounded cleanup of the owned child process group. Neither timed out.

All **1,230 effective source/config/raw-import inputs** were identical before execution, after Vitest and after TypeScript. The manifest covers all desktop src/e2e/scripts and root source/config/lock files, literal raw imports and the full external seam input set. The Node executable, installed lock and selected runner/compiler/package entries also remained identical; this is not a complete census of every transitive installed dependency. Exact git metadata retains the dirty source scope without treating the starting HEAD as the entire tested input.

The separate [latency helper](helper-latency/README.md) ran once under Node 24.19.0: **17/17 controlled VM/fake-transport tests passed**, exit **0**, no failures/skips/cancellations/todo. Its three-file import closure, nearest package/config absence, Node identity, HEAD, root diff and git status were stable. No browser, Docker, service or application runtime was started.

`summary.json` indexes source checks. `run-frontend.py`, the unchanged copied binding/execution helper and its provenance describe the actual commands. Raw reporters/logs, real underlying exit receipts, exact timestamps and before/after hashes are preserved. `SHA256SUMS` covers every file except itself, including nested helper checksums and empty logs. No rerun or widened suite was performed; Windows/native, UI runtime, latency, model and product acceptance remain separate.

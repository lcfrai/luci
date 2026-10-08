# Retrievable source verification receipts

Start with [summary.json](summary.json) and [failure-lineage.json](failure-lineage.json).
This directory travels with the clone. Each `*-report.json` links its input hash
map and allowlisted test outcomes. `artifact-manifest.json` hashes the public
files; reports separately retain hashes of the original report and input manifest.
The original full Go and frontend results remain failed.

- `full-go-*`: complete race run at dirty ancestry 23e25c21; all 2,003 recorded
  checkout inputs unchanged, 3,023 pass/29 named skips/one failure.
- `full-frontend-*`: original configured frontend, 2,575 pass/three failures;
  TypeScript/build passed, 433 scoped source inputs unchanged.
- `initial-focused-*`, `checkpoint-lock-failure-*`, `checkpoint-corrected-*`:
  original focused failure history and exact source snapshots. They are not
  overwritten with later passes.
- `affected-api-product-*`: 126 later affected race outcomes pass; 669 inputs.
- `affected-frontend-*`: 169 affected tests pass, nine actual files; TypeScript
  and build pass, 436 inputs. Read the requested-path caveat in the report.
- `controlled-browser-*`, `keyboard-focus-before-*`, `keyboard-focus-after-*`:
  scoped Linux Chromium observations with controlled API callbacks; original focus
  failure retained. Portable reproduction: [task-review-ui](../../../../scripts/acceptance/task-review-ui/README.md).
- `terminal-corrected-*` and `terminal-original-failure.json`: preserved original
  large-stdin failure and 32 affected hub/server race passes. No Docker/image or
  installed Windows execution is implied.
- `affected-input-deltas.json`: changed/added files compared with the full Go
  snapshot, limited to each affected runner's manifest.

Input paths are repository-relative. One external activation `go.work` in the
older focused manifests is renamed `@verification-environment/go.work` while
preserving its content hash and the original manifest hash. Absolute temp/workspace
prefixes in test labels and URL userinfo are replaced with explicit placeholders.
Commands replace only the private JSON-output destination. Assertions, DOM dumps,
private fixture paths, environment contents and authentication logs are omitted.

The hashes describe tested bytes, including uncommitted source. HEAD alone is
not a substitute for the map, and a later clean commit/package requires its own
binding and checks. Do not sum repeated test outcomes, convert skips into passes,
or relabel affected reruns as a new full run. The final controlled source-browser 22-checkpoint pass and original/after-correction
keyboard focus pair are included separately. The CLOUD-016 correction passes 32 affected race outcomes with 88 inputs unchanged;
it requires a matching rebuilt sandbox image and does not replace the failed full run.

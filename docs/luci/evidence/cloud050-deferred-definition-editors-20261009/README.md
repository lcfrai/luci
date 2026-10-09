# CLOUD-050: defer unopened definition editors

The local app no longer constructs every record editor and journey/requirement checkbox matrix when a specification first opens. Each native disclosure retains its title/count and mounts its editor body on first expansion. Once visited, controls stay mounted through collapse/reopen, while the specification remains the owner of edited values. Exact question navigation first mounts the requested section and then uses the existing focus/recovery fence. Missing journey links still reveal usable repair controls immediately. Project/account boundaries, read/edit guards, revision checks and draft storage limits are unchanged. No payload or cross-project cache is added.

This is a context-scale efficiency improvement, **not an established cause or fix for CLOUD-032**. The controlled measurements establish fewer DOM elements and preserve the tested interactions. They do not establish improved installed responsiveness, whole-computer RAM, Windows behavior, live-provider quality or full local acceptance.

## Evidence

- `before/` contains the original product and test files matching the b81d6454 checkpoint (product 99007404). `final/` contains the changed product and regression test. `product.patch` is the exact two-file change.
- `before-fixed-harness.json`: the same four regression cases fail on the original product, with 21 tests filtered. The Vite load overlay in `vitest-before.mts` substitutes only the bound original `DefinitionWorkspace.tsx`; the captured repository manifest otherwise contains the current candidate. `after-affected.json`: 91 assertions pass across four configured definition/draft/repository files (five reporter suites), zero failures. These results overlap the individual earlier runs; counts are not additive.
- `typecheck-exit.json`: TypeScript passed at that captured source. This run precedes the separate C051 changes; the root's later combined checks are separate evidence, not retroactive relabeling of this result.
- `browser-before-final/report.json` and `browser-after-final/report.json`: each round completed the same 42 recorded checkpoints with the identical helper, fixture and 424 captured repository input hashes; zero in-run source changes or page errors. These are overlapping workflow observations, **not 84 distinct tests**. The only served difference is the original DefinitionWorkspace load overlay in the before round.
- Each of four definitions contains 60 requirements, 20 journeys, 25 roles, 50 entities, 20 integrations, 100 glossary terms, 50 decisions and 30 questions. Each project has distinct values and IDs. On both 1440×1080 and 390×844, the eight initial project observations changed from **6,124 to 867 definition DOM elements**, and **2,307 to zero hidden group controls**. These counts are per selected project, not simultaneous worker or throughput measures.
- The actual styled AppShell and its normal navigation/draft controls exercise native summary Space/Enter activation; first open; same DOM control and edited value after collapse/reopen; a 1,200-checkbox journey matrix; four separate retained drafts; explicit Keep drafts and continue; exact question 30 focus only after Restore draft; preserved answers and glossary/journey changes; save callback identity/revision; Edit and Read revocation; and visible missing-link repair/removal. All four retained drafts total **2,445,044 bytes**, below the unchanged 4 MiB limit. API callbacks are controlled in-memory fixtures, not a service or persisted backend.
- `comparison.json` independently checks the eight DOM/hidden-control pairs and retains the captured resource/closure scope. Raw CDP counters and sequential `/proc` RSS readings of CDP-identified Chromium processes exist at 26 checkpoints per round. They include fixture-held documents and development tooling effects; other validation was concurrent. They are **not a causal CPU/RAM comparison**, unique physical-memory accounting, or a sustained whole-owned-process budget. Vite, test/compiler processes, daemons, unobserved processes and normal apps are excluded.
- Both final browser rounds closed their browser/server and removed their owned profile directory. A subsequent exact PID/start check found six original-round identities absent and one zombie adopted by PID 1; all seven candidate-round identities were absent. Port 15150 was free. No force cleanup or zero-PID/whole-machine claim is made. Snapshot visual runs have their own closure receipts and do not replace these records.
- `current-input-relationship.json` retains one later **unserved test-only** change: C051 removes unsupported Testing Library options in `AppShell.test.tsx`, from hash `6d865fae7421601056a0d746fd18bd8a83e1df375f780a891600baffcbdf95c1` to `c2d8aa217b1fd9ebcb8139eaec422de5ee606b54e4cce691e0fd22ded6b9f60b`. The original browser manifests are unchanged. All captured runtime modules still match the integrated source. The newer C051 45-case and root TypeScript checks are separate. Do not assert every captured test input matches the final Git tree.

## Preserved unsuccessful and preliminary runs

All raw reports and stdout/stderr remain unchanged. `comparison-initial.json` retains an initial derived label that called summary counts preserved without stating its source; the corrected comparison explicitly attributes that point to unchanged code, unit assertions and screenshots, not unrecorded numeric browser text. The initial new unit helper incorrectly called unsupported `fireEvent.toggle`; the initial before run has three intended failures plus that helper error, and the initial candidate run has 22 passes/three helper errors. The corrected helper dispatches the native `toggle` event and produces four intended original failures plus the final passing affected batch. `initial-candidate-with-test-helper-error/` retains those exact initial test bytes.

Browser attempt 1 failed before mounting the app because its private temporary directory exceeded Chromium's socket-path length. Attempts 2/3 failed on a label lookup; attempt 3 retained actual summary focus and `open=true` after Space. Attempt 4 queried an inaccessible role after collapsing it; the corrected identity assertion retains the element handle. Attempt 5 waited for an unchecked checkbox after the product correctly removed it; the correction requires exact detachment. Attempt 6 assumed desktop navigation visibility on the phone; the corrected driver opens the actual Projects drawer. These are retained fixture failures, not product failures. Attempt 7 completed all 42 checkpoints while two C051 source files changed; it is preliminary functional evidence only. The final pair reran against the frozen shared runtime.

The historical `question-focus-*.png` filename captures the recovered editor after subsequent glossary/journey disclosures, not an image of the earlier exact focus assertion. That assertion is bound by the helper and successful result. `visual-before/` and `visual-after/` are separate, short screenshot-only captures of the seven collapsed summaries; their two checkpoints each are not an additional full workflow pass.

## Reproduction

Portable source checks from the desktop directory:

```text
npm ci
npx vitest run src/components/definition/DefinitionWorkspace.test.tsx src/components/common/ProjectDraftRecovery.test.tsx src/components/common/DraftRecoveryReview.test.tsx src/components/definition/RepositoryAnalysisPanel.test.tsx --maxWorkers=1
npx tsc --noEmit
```

The exact browser helper uses the pinned desktop Vite/Playwright installation, Linux `/usr/bin/chromium`, repository `/workspace/luci`, an owned short `/workspace/.t` directory and free loopback port 15150. Copy this capsule to `/workspace/.luci-tools/cloud050-context-scale` (or consistently update those fixed fixture/overlay paths in a separately identified reproduction), retain the exact source checkpoint, then run:

```text
node /workspace/.luci-tools/cloud050-context-scale/browser.mjs <fresh-before-output> before
node /workspace/.luci-tools/cloud050-context-scale/browser.mjs <fresh-after-output> after
```

No Docker or provider is needed. External browser requests are blocked. This is a controlled source fixture, not the installed Linux harness and not an instruction to run it against a personal Windows Docker instance. `run-check.py` records the exact unit/TypeScript command and broad pre/post bindings. `verify.py` rechecks the published arithmetic and hashes without launching workloads.

Native Windows disclosure/focus/draft behavior and resource acceptance remain open. Mounted bodies deliberately remain mounted after their first expansion; this change does not promise to reclaim their DOM on every collapse.

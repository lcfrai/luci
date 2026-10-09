# Bounded source review after the await-boundary diagnostic

Product source reviewed through `git show edd0cb121ff6847f681ab6bf41509a497f043d41:<path>` only. The current uncommitted C056 tree was not used as product evidence. This review ran no tests, browser, application, benchmark, network request or cleanup. Run statistics supplied by the parent were not independently recomputed here.

## Finding

There is no source-grounded product optimization justified by the late verification marks. The reported evaluation spans (for example, row 300: 177.827 ms; row 352: 117.729 ms) contain automation lookup, browser/driver scheduling, callback execution and return work. They do not by themselves show that the application was still loading the saved definition or performing an animation.

The frozen helper's `actual/interaction-metrics.mjs:75-77` executes three sequential operations: `field.waitFor()`, `field.evaluate(...)`, then `field.inputValue()` plus a Node assertion. The callback at line 76 immediately samples `el.isConnected && el.value === expected`; on success it marks expected DOM and resolves. Only a failed sample schedules another animation frame. The callback retains its resolved element rather than reacquiring an element on every sample. The prior autonomous observer establishes its exact structural predicate at a sampled instant, not continued readiness or the identity of this later resolved handle.

## Product contracts in edd0

- `desktop/src/components/projects/ProjectOverviewBoard.tsx:53-82,133-136`: overview reads run concurrently with navigation controls; their cleanup aborts on unmount. The Open project button directly calls `selectProject`.
- `desktop/src/store/slices/uiSlice.ts:208-217`: selection synchronously guards unsaved drafts and publishes the navigation state. `:240-243` already skips an absent clean draft removal before the persistence middleware can write.
- `desktop/src/components/AppShell.tsx:148-154` and `desktop/src/components/lens/DiscoverLens.tsx:5-6`: project/source changes remount the lens; Definition and Orchestrator are sibling subtrees.
- `desktop/src/components/definition/DefinitionWorkspace.tsx:126-158`: one cancellable specification read is guarded by read permission, generation, liveness and write ownership. Its completion publishes saved specification, base, ready state and the specification tab. It does not await Orchestrator. The loaded purpose is the controlled textarea value at `:257`.
- `desktop/src/api/definition.ts:52-55` and `desktop/src/api/product.ts:481-544`: the read performs fetch, response text and JSON parsing with cancellation and scope checks. No artificial post-read timer or second application read is inserted before Definition uses the returned value.
- `desktop/src/hooks/useProjectDraft.ts:64-65`: the synchronous storage persistence effect returns while the editor is clean. Removing draft recovery on the strength of switch verification timing would therefore target the wrong condition and lose a user contract.
- `desktop/src/components/orchestrator/OrchestratorPanel.tsx:141-176`: context refresh is independently owned state. It can create concurrent work, but there is no serial dependency from Definition verification to that read.
- `desktop/src/components/journey/ContextStorage.tsx:22-26`: storage/history content, with its reads, is not mounted until its details element opens.
- `desktop/src/components/teams/TeamWorkspaceProvider.tsx:123-149`: personal access returns immediately; team permission checks are separately gated. This source contract alone does not identify which branch or any refresh was active in a slow run row.

These contracts rule out the proposed serial context barrier in the inspected path. They do not rule out concurrent rendering, layout, selector cost, garbage collection or scheduling, and they do not assign the observed milliseconds among them.

## Dependency-source explanation and binding limit

The frozen `actual/sustained.mjs:43` imports `/workspace/luci/desktop/node_modules/playwright/index.mjs`. Present installed `playwright` and `playwright-core` declare 1.63.0. A bounded independent read found:

- `desktop/node_modules/playwright-core/lib/coreBundle.js:58354-58366,58399-58400`: Locator evaluation first resolves a fresh strict attached element with `waitForSelector`, evaluates on its handle, and awaits handle disposal. The explicit preceding `field.waitFor()` does not eliminate this new resolution.
- The same file `:23720` onward: lookup runs navigation/locator-handler prechecks, selector work, visibility calculation, handle extraction/adoption and possible retries, with immediate then 20/50/100/100/500 ms retry intervals. It does not run stable/enabled/hit-target actionability checks.
- `:57797-57801,57828-57830` and `:35596`: evaluation and disposal involve channel calls; Chromium awaits the callback's returned Promise.

The present bundle SHA-256 is `549070af3acabb3efcc4f55bfe6210f9f7c2fcf633cf7eaa59bfe60719969171`. The retained capsule binds helper JavaScript, not the installed Playwright package bytes. This read is an explanation from the currently installed dependency source, not an independent proof of the historic run's exact executed dependency bytes.

## Concrete next step, if another diagnostic is authorized

Use one short control on the same real, already-settled edd0 Definition DOM, outside the acceptance cohort. Keep the original verifier and targets intact. Alternate the same verification triplet in a fixed balanced order, changing only the evaluate operation between its original Locator path and a pre-acquired handle for the same element. Acquire the handle outside measured calls, perform no navigation or edits within a block, and fail the block if selection, exact expected value, connection or handle identity changes. Keep both the preceding wait and following value assertion in both arms. Bind the actual dependency bytes before running.

Both arms must use byte-identical callback instrumentation: return callback-entry and successful-sample browser timestamps plus sample count through the existing evaluation response, using a separate diagnostic record rather than altering the acceptance ledger. Proposed bounds: four destinations, two ABBA blocks per destination (32 evaluations total), five seconds per call and 60 seconds for the diagnostic phase, with no automatic retry/relaunch. Re-establish current selected-project, exact saved-value and enabled-state witnesses at each block; do not reuse a prior run's readiness observation. Acquire one handle only after that block's fresh witness, require the Locator still resolves to that same handle at the boundary, and refuse replacement, detachment, selection/value change or lost editability rather than reacquiring or dropping the affected observation. Report all calls, failures and cap stops separately from the original 400+32 cohort and its unchanged thresholds. Retain only ordinal identities and numeric/boolean metadata, not saved values.

If both paths show large Node elapsed time while the browser callback succeeds on its first sample with negligible browser elapsed time, the excess is aggregate automation/query/scheduling/transport work outside the value predicate. A repeatable Locator-only excess would identify repeated resolution/handle lifecycle as diagnostic overhead. Callback retries would instead establish that the value/connection predicate was not continuously satisfied for that resolved element. This control cannot isolate network transport from browser or driver scheduling, and its cost must not be subtracted from original switch durations. It determines whether a product render investigation is warranted before selecting a product fix.

## Fresh immutable-source hashes

SHA-256 of each complete file read again from the stated edd0 commit for this note:

| Path (under `desktop/src/`) | SHA-256 |
| --- | --- |
| `components/projects/ProjectOverviewBoard.tsx` | `380db2996d677f099da77e2702fabfe5391e4dd226404513db1e58546a59b48f` |
| `store/slices/uiSlice.ts` | `4da15890388c23ac657ffe99311b5a1155001b78400f1b40846061372f1a5b0b` |
| `components/AppShell.tsx` | `b3aaf4ce4302109c2c10b023ecbff249b96655955f4d29bf0bb4834b4fc8fcd4` |
| `components/lens/DiscoverLens.tsx` | `5ffbc5f8e9411836118a4e7dbd12589e61e7dbb5ede9ed98cfb390a5d75e8436` |
| `components/definition/DefinitionWorkspace.tsx` | `fe63aa3dcda3c5896ab13140ff2db74c492eda0e068a415cc560b7ec7777cfc7` |
| `api/definition.ts` | `19824453993b27b745049952340f808475c8424ee01bf32b8a6cd696bad3a19c` |
| `api/product.ts` | `d6d8bda455d20cdf660976f7ca58d38967048780d6fc385ecdd07c7493329240` |
| `hooks/useProjectDraft.ts` | `d9991f749c285641bddb7eb8f565b3c853e61136148a7a32aa98a49f4fb831b2` |
| `components/orchestrator/OrchestratorPanel.tsx` | `a6c174f959daaea7f0c3c38c8a7710602fc2ab2fa718bcc62dbaedeecc28e93a` |
| `components/journey/ContextStorage.tsx` | `6fc91d6862b34dd24b57ec35a6ec9ad25d813c94742e12f2035984b9305c6b63` |
| `components/teams/TeamWorkspaceProvider.tsx` | `e6a3a98612aa506e9b78a656891dd36e8d2207a5f806fba7f8a65f1da7bf11bb` |

# CLOUD-048 recovery guidance — prepared source only

This candidate is external to the repository. No product/helper sources, application state or retained fixture were changed while the installed sustained benchmark ran. **Tests, type checks, builds and runtime checks have not been run.** Root must review/integrate and validate after releasing the measurement window. This is not a passing correction or acceptance claim.

The current UI safely retires a canonical pre-Create failure, but the task continues hiding its direct retry button because the historical `startFailure` remains. The existing background-queue flow can authorize a new attempt; the missing piece is a clear next action.

This three-file candidate adds persistent guidance to a current cancelled operation with a retained start failure, no pending recovery and no integration. It keeps the previous diagnostic visible, labels the task **Attempt retained**, and states that retirement started no new agent. **Go to queue review** scrolls the existing enabled **Queue reviewed tasks** button into view, then focuses it. If that button is disabled, it scrolls the labelled **Background queue review** region to the top and focuses that region so the existing queue and controls remain discoverable. It neither opens consent nor enqueues work.

The user still chooses **Queue reviewed tasks**, reviews the existing saved-task/model/permission consent, and explicitly clicks **Authorize queued tasks**. This adds one deliberate navigation step while avoiding a second consent implementation, changed eligibility rules or a hidden retry. The navigation remains useful when a newer attempt is already queued or execution permission is unavailable; a disabled queue button is not presented as a failed navigation.

Guidance derives from the current task operation, rather than a transient successful-request flag. It therefore survives reopening a retained attempt and disappears when that task points to a newer operation or a different project/run is selected. No historical diagnosis is cleared, no operation/session is replaced, and no admission, capacity, backend, authorization or idempotency behavior changes. The existing Background queue summary action uses the same focus routing.

Prepared regression cases in `ProjectSwarmQueue.test.tsx` cover confirmed retirement with preserved diagnosis, navigation with no reserve/create/enqueue or implicit consent, separate explicit consent at the current revision, an already queued attempt, revoked execution permission, focus fallback, and removal of stale project guidance. They remain unrun. The original related component tests are retained unchanged in `before/`; no test results have been fabricated.

A bounded independent source review of prepared variant1 found no concrete source/test defect in its three before/after pairs. It did not run tests or inspect shared components. The author separately read the existing shared Button adapter and confirmed that it uses `forwardRef<HTMLButtonElement, ButtonProps>` and passes the ref to the underlying button. Native focus behavior still needs the prepared regression to execute after the measurement window.

The initial untested prepared variant is preserved byte-for-byte under `variants/prepared-variant1/`, including its original manifest, patch and checksums. Root then requested a small focus-visibility refinement: scroll the chosen enabled button rather than its potentially tall enclosing region; use `block: start` for the disabled-button region fallback. This is a source refinement, not a claimed reproduced runtime defect. After the measurement window, a browser check with a tall existing queue should verify that the focused review control is inside the viewport and that navigation neither authorizes work nor bypasses consent. DOM-only focus assertions cannot establish that geometry.

`candidate.patch` affects only:

- `desktop/src/components/journey/ProjectSwarm.tsx`
- `desktop/src/components/journey/SwarmRunQueue.tsx`
- `desktop/src/components/journey/ProjectSwarmQueue.test.tsx`

`candidate.json` binds the exact before/after bytes and the unchanged current repository inputs. `SHA256SUMS` covers all capsule members except itself. A later validation capsule should retain the new regression against the original source, then the revised source, plus focused related component tests and TypeScript results. No validation process should start before root releases the benchmark window.

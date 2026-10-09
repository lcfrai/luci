# CLOUD-032: autonomous DOM-readiness witness design

External design only. No implementation, product change, issue assignment, test or measurement is authorized by this document. The existing full browser actions, saved-context checks, measured spans and 300 ms switch target remain unchanged.

## Decision and evidence

Add one bounded, passive browser observer to a separately declared diagnostic helper. Its purpose is to distinguish **a UI state observed ready before the driver notices it** from **a UI state not yet observed ready**, not to subtract driver overhead or establish a faster product.

The measured d20 idle result is bound by SHA256 `da1cd7eb46948775af3d5641389940776a4c75f34c1bc71b42bef9ce42c79a77`, product `d20ebd292f3f1de04d6726bab0998c210987bcb5`. Its original 100 switches have p95 424.672 ms; 15 exceed 300 ms. The worst switch is 728.808 ms, including 517.418 ms in the All-projects command. The existing expected-DOM timestamp is emitted by a driver-triggered evaluation after its locator wait, so it is not an autonomous app-ready timestamp. The separate eight-round trace cannot explain those earlier primary failures.

Source inspection found no avoidable serial fetch barrier: All projects changes selection synchronously; Home's heading and primary destinations do not wait for summaries/capacity; Define's specification read does not wait for orchestrator. Synchronous preference persistence is an unmeasured candidate whose semantics must be retained. No source optimization follows from this design.

## Instrumentation boundary

Install a self-contained observer once in the actual browser document. Installation and cohort enable/disable RPCs occur outside primary action spans. Preserve the original Playwright routing/origin/mutation guards, request bodies, cancellation, click/wait/evaluate calls and two-frame endpoint byte-for-byte. Do not add request interception, app hooks, fetch wrappers, CPU profiling, force-clicks, alternative routes or replay.

Enable the observer only for the fixed idle cohort after warm-up/quiet. The original immutable fixture supplies four project ordinals, their button labels and exact saved Define values to private in-memory comparison state. It also supplies the expected project order. No DOM result chooses a different target or advances the driver.

A capture listener observes original **trusted `click` events** on the existing native All-projects button or the exact expected Open-project button. It neither prevents default nor stops propagation. It records the click's browser-clock timestamp and opens a new observation generation before the product handler runs. A trusted pointerdown may also be recorded as a separate timestamp, but it does not arm or substitute for the click. Programmatic clicks, unrelated controls, keyboard events without the expected resulting trusted click, and wrong targets cannot produce a valid witness.

There is one active stage at a time:

| Stage | DOM predicate, evaluated independently of driver RPCs |
| --- | --- |
| Home destination | The current workspace root contains the exact “Across your projects” heading and expected primary Open-project button. The button is connected and natively enabled, with no `hidden`, `inert` or `aria-hidden=true` ancestor. The definition editor from the previous project is no longer present in this active root. |
| Saved Define value | After the matching trusted Open-project click, a newly connected editor node has the exact “The idea” accessible label and its `.value` equals that project's original saved fixture value. It is natively enabled (including disabled fieldset semantics), not read-only, and has no hidden/inert/aria-hidden ancestor. The active project identity must agree with the expected destination using the existing rendered project-selection context; ambiguity invalidates the witness. |

These are **structural DOM and native-enabled predicates**, not full Playwright actionability or visibility checks. CSS occlusion, pointer hit testing, stability, physical paint and visual completion are intentionally not inferred. Original driver actionability and fresh-context checks continue to determine action success.

Require a fresh editor node belonging to the current click generation. A same-looking retained node, stale value from another project or a witness from the prior document cannot satisfy the next stage. Keep exact-value comparisons private; publish only project ordinal and predicate result.

## Bounded observation without layout or mutation churn

Use one `requestAnimationFrame` sampling chain while a stage is armed. Do **not** install a MutationObserver: React textarea `.value` updates need not produce an attribute mutation, and observing the whole workspace would add unrelated summary-commit churn. Do not intercept property setters.

Each sample reads only fixed, bounded DOM structure, element identity, labels, native disabled/read-only flags and `.value`. It never reads bounding boxes, offsets, scroll dimensions, computed style, layout-dependent visibility or screenshots; it never scrolls or changes focus. Resolve only within the existing active workspace root. Use a capped DOM traversal/control count; a cap hit records unavailable observation instead of widening the scan. Exact selectors and traversal bounds must be checked against the real current DOM during implementation, not guessed from a mock.

Proposed initial caps: one active rAF chain, at most 180 samples or 3 seconds per stage (whichever arrives first), at most 512 visited DOM nodes / 32 candidate controls per sample, 200 stages for the 100-switch cohort, 36,000 samples total, 1,000 result rows / 512 KiB metadata. A cap is a diagnostic censoring/failure, not an app failure or a readiness timestamp. Do not extend a cap to make an attempt pass. If readiness is observed, record its first sample and stop sampling that stage. An earlier ready state between samples may have been missed.

At the existing outside-span observation drain, collect numeric rows and counters; no new per-action RPC belongs inside the measured span. Stop and clear a prior generation before arming the next. If the driver ends/fails/cancels a round, disables the cohort or closes/navigates the page, cancel the active rAF, invalidate pending generations and detach listeners at owned cleanup. No background observer survives the diagnostic. Unknown cleanup remains explicit.

## Project, authority and lifetime rules

This first design is a fixed-authority, fixed-saved-context fixture, not a general app readiness API. The existing harness owns fresh project/actor/workspace/source identity and mutation guards. The observer must not treat a displayed value as permission or source authority.

A different trusted project destination, workspace/document replacement, root/editor disconnection, denied-access state, failed original action or a harness-known actor/team/permission/source generation change cancels the current stage. Every emitted row includes document generation, click generation, expected project ordinal and stage. Late callbacks from a cancelled generation can neither publish a valid row nor arm another stage.

For changes not reliably exposed by the current DOM, use an explicit diagnostic lifecycle invalidation from the harness when its authoritative fixture generation changes, outside primary spans. Do not invent a DOM attribute or infer an unchanged grant from an unchanged value. If an adversarial authority/source transition cannot be observed or invalidated reliably, classify that transition as unsupported and omit its readiness conclusion; the original product/driver guard still decides failure. Do not broaden the observer into production permission machinery.

A readiness witness already recorded before a later revocation remains a historical observation at its timestamp, followed by an invalidation row. It cannot make a later failed action pass. A denied or replaced context must never reuse that witness as current readiness. No write or paid operation is cancelled, repeated or accepted by this observer.

## Clock, privacy and interpretation

Record only document `performance.timeOrigin`, browser `performance.now()` at trusted click / sampling / invalidation, local sequences, stage and project ordinal, predicate booleans, sample count and fixed status/reason enums. Keep labels, saved values, project IDs, URLs, headers, tokens, input keys and response bodies out of exported rows. Private expected values live only in the fixture observer's memory and are cleared at cleanup. Project ordinals map to the original private fixture through a separately bound harness record, not public project names.

All readiness intervals and comparisons to the existing browser expected-DOM/frame marks use the same browser document clock. Never subtract Node command times, CDP Network times or wall-clock timestamps from browser values. The original Node full-span duration remains the sole 300 ms target measurement, unchanged.

Call the value **first observed DOM-ready witness**, not “actual app-ready time.” It is an upper bound on when the observed predicate most recently became true, subject to sampling and possible transient states; it is not necessarily the first time it became true. Browser scheduling delay can postpone the observer itself. Record sample gaps so a long blind interval is visible.

- If a valid autonomous witness precedes the driver's expected-DOM mark by a large interval, the predicate was demonstrably true at that earlier sample; the later driver observation contributed delay within the unchanged full workflow. Do not deduct that difference or label all of it transport cost.
- If the autonomous witness is late, that does **not** prove late app readiness: the browser may have been unable to sample. A late/missing witness with a large sampling gap is inconclusive.
- If it is never observed, report censored/unavailable and preserve the original action result. Do not replace a failed original context witness with observer success, or observer failure with driver success.

No observer time is subtracted. Instrumentation cost and altered scheduling remain limitations. The witnesses do not establish React commit boundaries, physical paint, CPU attribution, server work, performance improvement or a production optimization.

## Meaningful validation before an installed diagnostic

Use a small controlled, styled fixture first; retain the initial expected failure of the old late-only observation design and exact before/after sources/results. No installed run is proposed until root reviews implementation and these cases.

1. **App ready early, driver notices late:** let the trusted click render the correct enabled target, then independently hold the driver's observation RPC while the browser remains responsive. The autonomous witness must precede the late driver mark; the original full elapsed span must still include the hold and fail its target when over 300 ms. This is a deliberately induced diagnostic distinction, not acceptance performance.
2. **Actual DOM predicate delayed:** delay only the fixture's relevant DOM state after the trusted click. While samples continue without a long gap, no witness may precede the real controlled transition. A disabled or wrong-value field does not qualify.
3. **Observer delayed too:** deliberately block the fixture main thread or pause observer scheduling after the click. The result must expose a sample gap and make no assertion that first readiness occurred at the eventual witness. Do not use this as a CPU profile or timing calibration.
4. **Identity and stale callbacks:** A→B→A, same saved text in two projects, a wrong-target click, a replaced editor/root/document and a late prior-generation sample. Require exact project/context binding and no witness reuse. If existing DOM cannot disambiguate identity, mark unavailable rather than pass.
5. **Authority and cancellation:** revoke read/edit, disable the fieldset, show denied access, replace fixture authority generation, abort the original action, unmount and close. Require invalidation, bounded settled cleanup and no changed product mutation/provider behavior. A later grant needs a fresh original click/generation.
6. **No instrumentation effects:** verify no synthetic click, focus, scroll, request, fulfillment, DOM write or layout-query API is invoked by the observer; original full action list/thresholds and route guard remain exact. Test bounds, overflow, no target and cleanup. Treat these checks as behavioral evidence, not proof of zero overhead.

After those cases, a single separately labeled installed diagnostic may collect witnesses beside all original actions on an exact clean package/helper/source/image binding. All rows, failures, limits and invalidations remain. Primary and separately traced follow-up cohorts stay distinct. A production correction is considered only if the new observation identifies a concrete product operation with a semantics-preserving fix; this design alone recommends none.

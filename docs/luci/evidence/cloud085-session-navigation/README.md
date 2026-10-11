# Explicit existing-session navigation

Session-row click/keyboard activation and the branch attention button now open their existing session in Develop. Background `focusSession` remains focus-only. The new `openSession` action validates the retained session, its exact project/branch/checkout and indexed membership; it never creates or starts a session. Changing project, phase or checkout goes through the existing unsaved-draft guard before changing focus. Returning to the same Develop checkout reveals the retained editor without discarding its draft.

A pending destination captures its session creation/allocation identity, catalogue/source generation and project/checkout paths. Confirmation revalidates them before any draft discard. Missing, replaced or changed destinations preserve the edited draft and existing focus. Normal status/output observation does not invalidate navigation.

## Scoped checks

The five-file single-worker Vitest command in `test-exit.json` passed 145/145 tests, exit 0. It covers explicit click/Enter/attention actions, cancel/keep/discard behavior, same-editor retention, changed destination refusals and existing project/source navigation/store behavior. All 21 selected source/test/config bindings were unchanged across the run. These are selected bindings, not an exhaustive compiler/dependency manifest. The raw log preserves jsdom's existing HTMLCanvasElement getContext diagnostic; it did not fail assertions. No earlier failing test run was executed for this slice; the pre-change issue was found by source review.

The resource guard is a read-only pre-test observation, not atomic product admission or performance evidence. No standalone TypeScript, app/browser/native/Docker/provider or latency test ran here. The parent will check the assembled main pane and final TypeScript. No full product acceptance is claimed.

Shared-checkout base at capture: `b35a9dd70688dd47b891b96a0131448ff1c864fd`. Only uiSlice.ts, luciSurface.test.ts, SessionRow.tsx, BranchRow.tsx, Rail.test.tsx and this capsule are owned by this change.

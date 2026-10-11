# CLOUD-086 team coordination UI — controlled source checks

This slice adds a compact shared-intent display and an explicit, actor-owned resolution control to the continuing Luci conversation. It consumes the backend's authoritative projection; reads do not register intent, approve context, dispatch work or announce agreement. The backend automatically records successful proposals. The separate primary conversation integration is owned and validated with the parent component.

The display distinguishes pending application, conflicting intent, unsatisfied prerequisites, saved context and superseded history. It retains exact actors, context versions, causal events and affected tasks. Only the current person's pending intent can be superseded, with an eligible same-document replacement, current revision/context, a reason and fresh consent. Server authority and stale-state checks remain authoritative. The UI never claims that saving context completes implementation or delivery.

Unknown write outcomes retain their exact request in person/account/workspace/project-scoped browser storage. Neither reopening nor permission churn replays it. Readback must confirm the exact actor/intent/replacement/reason before releasing that barrier. Drafts survive reopening; consent does not. A cross-window storage mismatch stops dispatch. Read cancellation is separate from mutations.

## Evidence and limits

- `final-tests.json`: **29 passed, 0 failed, 0 skipped**, two selected files, one worker. API contract validation and actual React component fixtures cover conflicting versus compatible/pending states, causal validation, current grants, own-only resolution, stale context, restart/draft recovery, lost responses, revoke/restore races, read revocation, project/person changes and client-generated scope conflicts.
- `client-conflict-before.json`: **one expected regression failure**, 13 other cases filtered. The same test proves the initial component incorrectly treated a client-generated `ApiError(conflict,409)` as server refusal. `before-error-classification.tsx.txt` retains that source. Final production code accepts only parsed `ProductApiError` refusals for that settlement branch. The final run includes the unchanged regression and its true-server-refusal control.
- `initial-tests.json` (26 passes) and `intermediate-tests.json` (28 passes) are historical, overlapping runs, not additional unique coverage. The initial run predates prerequisite display, replacement filtering and the client-conflict fix. `intermediate-selected.json` binds the earlier selected source state, not the final candidate.
- Final TypeScript command exited **0**; its empty stdout/stderr are retained. `selected-before.json` and `selected-after.json` bind the four owned source/test files, byte-identical around the final component/API run and unchanged through typechecking. They are **selected-source provenance**, not a complete compiler-input manifest. Other owners were implementing concurrently.
- No browser/native app, live model, Docker, microVM, deployment, latency, screenshot, full frontend-suite or actual two-person coordination acceptance was performed by this slice. CLOUD-086 and the full lifecycle remain open.

Commands from `/workspace/luci` using the task-local Node environment:

```sh
source /workspace/.luci-tools/activate.sh
node --max-old-space-size=512 desktop/node_modules/vitest/vitest.mjs run --root desktop src/api/coordinatorIntents.test.ts src/components/orchestrator/CoordinatorTeamIntents.test.tsx --maxWorkers=1 --reporter=json --outputFile=../docs/luci/evidence/cloud086-team-intents-ui/final-tests.json
cd desktop
node --max-old-space-size=768 node_modules/typescript/bin/tsc --noEmit --pretty false
```

The negative command selects the component file with `-t 'client workspace-change'`, using the retained original component and the same final test. Setup/configuration is unchanged. `SHA256SUMS` covers the published capsule artifacts; it excludes itself.

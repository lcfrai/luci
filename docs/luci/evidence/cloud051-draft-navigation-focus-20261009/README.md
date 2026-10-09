# CLOUD-051: restore usable focus after compact draft navigation

Actual styled AppShell reproduction: open Projects at 390px, choose All projects with an unsaved gamma definition, then Keep editing. The drawer closes while the draft decision stays open. The captured former opener is still connected but lives in the now-inert rail; the previous generic close behavior cannot focus it. BODY remains active after two frames and the full fifteen-second observation. The project and exact unsaved text remain preserved. This is a keyboard-focus defect, not a data-isolation failure or established performance cause.

The correction is confined to DraftNavigationDialog's existing WorkflowDialog `getCloseFocusTarget` hook. It keeps a usable original opener (including ordinary desktop navigation), otherwise resolves the current usable Projects trigger after the modal trap is removed. Disabled, inert, hidden and disconnected targets are excluded. Global WorkflowDialog behavior, permissions, navigation decisions, draft retention and consent remain unchanged.

Evidence:

- The identical new seven-case actual AppShell unit regression has two compact failures before the correction; two desktop controls and three disabled/inert/hidden fallback controls pass. Twelve unrelated tests were filtered in that targeted invocation.
- Corrected affected AppShell, ProjectDraftRecovery and WorkflowDialog files: 45 passing tests, zero failures/skips. This supersedes the two changed files in the prior separately recorded 2,787-test full-suite result; it is not another full frontend pass. Final combined TypeScript and packaging belong to the parent publication.
- Root final TypeScript initially rejected six unsupported Testing Library `exact: true` options in the new AppShell tests. The original compiler failure/source/dependency manifests are retained. Removing only those ignored options preserves string-name exact matching; the final affected three-file suite again passes 45/45. These are overlapping runs, not 90 unique tests. Final test SHA256 is `c2d8aa217b1fd9ebcb8139eaec422de5ee606b54e4cce691e0fd22ded6b9f60b`; product bytes remain unchanged. Parent records the subsequent final compiler result separately.
- Same styled browser follow-up: six checks pass before, final focus check fails. After correction all seven pass, including drawer opening focus, Escape and explicit Close restoration, 390-to-1440-to-390 navigation/context, modal focus and usable Projects focus after Keep editing with gamma's exact draft retained. Original and corrected helper bytes match; selected source bindings show the product change explicitly.
- A wrapper attempted cmp/cp with the wrong working directory before an affected run. Those commands failed, leaving original product code; the raw broad original run is retained under `original-broad-cwd-error`, never credited as corrected validation.

Run the controlled source browser scope after installing dependencies on Linux with Chromium:

```sh
node scripts/acceptance/sustained-four-project/phone-overview.browser.mjs --out /absolute/fresh-output --followup-only --port 15149
```

Run affected units from `desktop` with `node node_modules/vitest/vitest.mjs run src/components/AppShell.test.tsx src/components/common/ProjectDraftRecovery.test.tsx src/components/common/WorkflowDialog.test.tsx --maxWorkers=2`.

Source Chromium uses actual components and styles with controlled read-only context, not backend execution. Source bindings are selected inputs, not a full compiler manifest. No installed/Windows/live-provider/performance or complete local acceptance claim. Browser/Vite/profile cleanup completed and the independent narrow closure observation is included; no universal browser/zombie/whole-machine claim.

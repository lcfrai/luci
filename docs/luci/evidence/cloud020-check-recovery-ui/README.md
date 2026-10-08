# CLOUD-020 check recovery and CLOUD-023 keyboard evidence

The final controlled browser witness passed **34 checkpoints**, produced **17 screenshots**, and retained unchanged hashes for 439 application inputs plus the exact helper and fixture. The narrow pending recovery dialog now has a named, visibly focusable region, allowing keyboard scrolling while dismissal and every mutation remain blocked. This directory publishes existing evidence; publication performs no reruns.

The exact [browser helper and reproducible command](../../../../scripts/acceptance/check-recovery-ui/README.md) are published separately. [final-source-manifest.json](final-source-manifest.json) binds the 12 final frontend files and is the explicit manifest input for that unchanged helper. [publication-manifest.json](publication-manifest.json) records every published artifact's byte count and SHA-256. Original absolute invocation paths are retained as historical metadata; [original-artifact-locations.json](original-artifact-locations.json) maps those copies to their original artifacts.

## Separate outcomes and retained failures

| Run | Outcome and retained evidence |
| --- | --- |
| [Initial external diagnostic 1](diagnostic-1/result.json) | 80/82 tests. Obsolete host selector and premature async disclosure lookup failed; TypeScript projection omitted the existing e2e dependency. Raw reports and logs retained. |
| [External diagnostic 2](diagnostic-2/result.json) and [TypeScript-only diagnostic 3](diagnostic-3/result.json) | 90/90 targeted tests. TypeScript then passed after erased test-fixture type imports/casts; the original type failure remains retained. |
| [Full frontend baseline](full-frontend-baseline/report.json) | **2,689 passed, 3 failed**, zero pending/todo; 436 inputs unchanged. Failures were the pre-existing domain-contract mirror mismatch, missing token-layer import in the new CSS and obsolete Execute-only cancellation expectation. [Raw full report](full-frontend-baseline/tests.json) is preserved and is not relabeled as passing. |
| [Applied correction 1](correction-diagnostic-1/result.json) | **170/173 passed**, TypeScript passed; 1,173 inputs unchanged. The three remaining failures came from newly added test assertions: duplicate requirement text and JSDOM retaining the disabled submit's focus. Exact failed test source is retained. ProjectChecks, design-system and seam gates passed. |
| [Applied correction 2](correction-diagnostic-2/result.json) | **24/24 passed** in the two corrected files; full TypeScript passed; 1,173 inputs unchanged. Evidence selection is scoped to the opened requirement details, and the component test starts from the dialog fallback used by the browser. The real browser separately proves natural Tab behavior. |
| [Browser attempt 1](browser-attempt-1/result.json) | No page/checkpoints: Chromium temporary UNIX socket path was too long. The same helper was retried with a shorter private path. |
| [Browser attempt 2](browser-attempt-2/result.json) | 23 checkpoints succeeded; checkpoint 24 found **CLOUD-023**, an Axe `scrollable-region-focusable` failure during the pending 390-pixel recovery. [Failure screenshot](browser-attempt-2/failed-stage.png), helper and fixture retained. |
| [Browser attempt 3](browser-attempt-3/result.json) | 29 checkpoints passed, including corrected pending keyboard scrolling/Axe. A harness assertion then queried a hidden phone rail item without `includeHidden`; [failure screenshot](browser-attempt-3/failed-stage.png) retained. Only that DOM attribute selector changed for attempt 4. |
| [Browser attempt 4](browser-attempt-4/result.json) | **34 checkpoints passed**, **17 screenshots**, zero page errors. 439 application inputs and helper/fixture unchanged; Chromium 151.0.7922.173. Owned browser, Vite and private temporary profile cleaned up. |

Counts overlap and are not added as unique tests. The final whole frontend suite was not rerun after these scoped fixes. The full failed baseline, focused corrections and controlled browser result remain separate claims. Root repaired only the contract §2 TypeScript mirror to match the unchanged domain types; no seam assertion was weakened.

## What the final witness exercised

At both viewport sizes, actual AppShell/Develop/check/dialog source renders retained failed and unknown output, exact check/revision/command review, explicit consent and one recovery callback, and a lost response followed by readback without mutation replay. It checks abort/close during slow reading, blocked dismissal during a held write, current GET403 clearing private check history/coverage and focusing explicit access refresh, retained unsent setup, and an old-project denial arriving after navigation to another project.

CLOUD-023's 390-pixel case uses natural Tab to reach **Check recovery details**, observes a 2-pixel focus outline, and uses Home/ArrowDown to change dialog scrollTop from 0 to 8 while the POST is held. Axe then reports no violations under the selected WCAG tags. The mutation and dismissal controls remain disabled. See [pending recovery, 390](browser-attempt-4/pending-recovery-390.png) and [1440](browser-attempt-4/pending-recovery-1440.png).

The actual five-second polling sequence recorded 12 detail reads with at most four inflight, no overlap, abort/pause under controlled hidden visibility, fairness across six records, and automatic stop after terminal history. It recorded zero command starts and zero mutations. The final receipt contains the individual request/round observations.

Representative final screenshots: [exact review, 390](browser-attempt-4/exact-recovery-review-390.png), [retained failure, 390](browser-attempt-4/retained-failure-output-390.png), [lost-response readback, 390](browser-attempt-4/lost-response-readback-390.png), [read denial, 390](browser-attempt-4/read-access-denied-390.png), [project switch, 390](browser-attempt-4/project-switch-no-private-leak-390.png), and [settled history, 1440](browser-attempt-4/settled-check-history-1440.png). All 17 final screenshots and earlier attempt screenshots are retained.

The shared actual-handler golden SHA-256 is `1dc6700c476d58fb5b6624525bb04150010e0c5aad30c116b887d3e07a6a9a54`. Its backend execution evidence remains separately owned; this frontend witness uses controlled API callbacks.

## Limits and publication contents

The exact successful helper SHA-256 is `f3f3cf648159801858cc81704dd4f38bb471d262bb200b312009edd2d4044c78`; fixture SHA-256 is `dfff76c5eb378031607881a9942bca90548b3cecb067eb54983466243e11347e`. Each browser attempt retains its own original helper/fixture bytes and nonsecret invocation metadata. Source manifests, raw test JSON, concise command/result logs and screenshots are published; browser profiles, cookies, credentials, environment dumps and real user/project state are excluded. Browser state in receipts is explicitly generated fixture data.

This is source-component evidence with controlled account/team/setup providers and API callbacks. Only the unrelated ProjectPlan leaf is omitted; AppShell, access/draft hooks, navigation, check UI, WorkflowDialog and pinned shadcn dialog are actual source. A synthetic POST log is not an HTTP transport or Docker cleanup claim. Controlled DOM visibility is not OS background-window behavior. Automated Axe/geometry checks are scoped and do not establish complete accessibility certification.

No installed package, native Windows, real backend, Docker, cloud, model, deployment or performance acceptance is claimed here.

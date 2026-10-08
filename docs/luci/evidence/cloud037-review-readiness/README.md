# CLOUD-037: wait for the exact check recovery control

Installed check recovery attempt 2 reached `read-and-dismiss-exact-recovery` and recorded an `AssertionError` at `installed-recovery.mjs:171:137`, the immediate recovery-button count assertion. Its [published failure excerpt](actual-failure-excerpt.json) does not retain count operands or a check-history timing trace. The original report remains unchanged.

The source explains a readiness gap: `openProject` waits for the section heading, while `ProjectChecks` initializes an empty run list and loads check history asynchronously (`useEffect` calls `load(true)`, which awaits `productApi.checks` before `setRuns`). A visible heading therefore does not establish that check history has rendered. This is a supported failure mechanism, not a claim that the installed attempt recorded that exact event ordering.

The one-line adapter correction waits for the existing exact recovery-button locator to become visible, using the page's existing 15-second default timeout. It then retains the strict count-one assertion and click. It changes no selector, review response equality, exact GET path/method, dialog check identity, unchecked-confirmation requirement, recovery mutation, or command replay behavior. Multiple controls and unavailable controls still fail acceptance.

The controlled regression executes the actual `openRecovery` body in a VM with deferred history and locator/response doubles. Before history is released, the old function fails with `0 !== 1`; the corrected function waits without clicking. Releasing history permits one review click, an exact matching GET response, the unchanged review and the exact disabled-confirmation dialog. This is controlled asynchronous behavior; no browser or installed service runs.

Evidence:

- [Before regression](before-regression.log): one test, one failure, exit 1.
- [After regression](after-regression.log): identical regression bytes, one pass, exit 0.
- [Targeted tests](targeted-tests.log): five passes, exit 0. Negatives cover duplicate controls, unavailable visibility, foreign project/check GET paths, wrong method/status, changed review revision, wrong dialog check and an enabled confirmation button.
- [Validation manifest](validation.json): exact commands, source hashes, limitations and the product frontend source binding. [Before manifest](before.json), `before/`, `tested-sources/` and [checksums](SHA256SUMS) retain the original and tested bytes.

No browser, runtime, compiler, model call or original private fixture was used by this correction. The installed recovery acceptance remains pending a new clean helper commit and a fresh exact-source-bound attempt.

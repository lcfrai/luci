# CLOUD-021: dependency refusal before source admission

The same controlled HTTP regression fails on the original production source and
passes with the correction. With an ordinary source reader held open, a task
whose dependencies remain unintegrated now receives the exact dependency refusal;
a stale revision receives the existing reload response. Neither case resolves a
provider, allocates a checkout, or changes private/source state.

The fourteen-line production change checks private revision/task readiness after
idempotent reservation recovery and before provider/source admission. This
preflight grants no allocation authority. Ready work still needs current route
permission and exclusive source admission; the final mutation rechecks revision
and task readiness. The tests also cover real reviewed Git dependency integration,
unchanged lost-response readback, and a revision changed during route resolution.

[Verification](verification.json) binds 20 corrected product outcomes and 24
complementary scheduler/API outcomes to unchanged 637-file source manifests. Both
runs used the race detector, with zero failures/skips. These are controlled Linux
tests, not new installed, Docker, Windows or model-quality acceptance.

[Installed attempt 3](attempt3.json) preserves the failed exact `9de6c800` run.
A and B completed naturally with captured reports; both containers and exact
execution leases were absent before the probe. The request/public/private revision
was 14 throughout. The exact response was `Project source is busy. Wait before
reserving this task.` Primary HEAD, task operations and Create/Start counts stayed
unchanged. This is source-admission contention, not evidence that dependency
validation ran. The source lock holder was not recorded, so no particular UI poll
or observer is blamed. No inference is made about an earlier attempt whose body
was not retained.

The driver stopped naturally without retry. No A integration, retained B conflict,
replacement B, join, or whole-build Docker check was reached. Independent closure
confirmed zero containers, volumes and fixture processes, all 1273 package members,
image/engine identity, 19 helper inputs and the clean original revision. Private
fixture outputs and all failed evidence remain retained; no live resources remain.

The correction awaits a new matching installed attempt. Existing ready-task lock
contention remains unchanged; this is a bounded refusal-order correction rather
than a source-lock redesign.

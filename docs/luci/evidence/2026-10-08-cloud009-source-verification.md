# Interrupted-launch recovery source verification

The integrated source adds explicit recovery for an uncertain new Docker task
launch, preserves its context/output, and prevents delayed old requests from
starting duplicate work. Recovery uses current authority and retained proof;
ordinary abandon/retry cannot erase an unresolved attempt. Cached Docker-storage
refresh now leaves Settings navigation usable without launching unrelated
readiness probes. Native selector defects LOCAL-004/006 are separately reported
verified on Windows.

[Machine-readable evidence](2026-10-08-cloud009-source-verification.json) preserves
an initial complete Go race run: 2,939 passes, 3 failures, 29 skips, with 1,882 inputs
unchanged. These identified an incomplete Docker fixture, scheduler-fixture
synchronization and missing route-audit registration. Corrected full API/cmd
packages pass 974 outcomes/9 explicit skips; both route audits pass. Only the
listed Go files changed after the initial run; the quota-message change has two
additional passing regressions. Failed evidence remains failed evidence.

The full frontend passes 2,418 tests, TypeScript and production build. Backend 77
race outcomes include actual Docker fence mechanics, with matching final source
hashes. The pin log/manifest pair is explicitly identified; older similarly named
manifests are not substituted. Windows API cross-compilation passes but was not
native execution. Native module/packaging reports remain separately scoped.

These are source and individual-mechanism results. A clean matching package must
still pass installed controller-crash/delayed-request/lost-ack recovery and the
sustained mixed four-project run. Live-model quality and native app/workstation
acceptance are not established by the deterministic providers or Linux tests.

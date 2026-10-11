# Preview cancellation — management admission

`POST /v1/product/projects/{projectId}/releases/{releaseId}/cancel` now requires current Read, Edit and Deploy authority. Execute and Secrets are not required. Cancellation receives the same suspended-service cleanup exemption as Stop; that exemption grants no team or project permissions.

The two production changes are preserved in [management.patch](retained/management.patch). Only `team_authorization.go`, `team_service.go`, and their two affected test files belong to this implementation scope. No commit or staging action was performed by its owner.

| Selected run | Exit | Passing outcomes | Failing outcomes | Scope |
| --- | ---: | ---: | ---: | --- |
| [Original production with new regressions](retained/before/result.json) | 1 | 6 | 5 | Two top-level tests and nine subtests |
| [Fixed production, same selection](retained/fixed/result.json) | 0 | 11 | 0 | Two top-level tests and nine subtests |

The original failures show authorized cancellation rejected as an unavailable route, including both active and suspended service. Outcome totals include the two parent test results; they are not 11 separate top-level tests. No case was skipped. The exact stdout/stderr and invocation are retained under `retained/before/` and `retained/fixed/`.

The cases cover Deploy without Execute, missing Read/Edit/Deploy, unsupported methods, suspended cleanup, team and project Deploy revocation with the same login and expected release revision, restored authority, cross-project refusal, and continued refusal of new release creation under suspension. The worker is the controlled management fixture. Product cancellation, Docker settlement and runtime behavior are outside this test result.

The exact command, from `control-plane`, was:

```sh
source /workspace/.luci-tools/activate.sh
GOMAXPROCS=2 go test -p 1 -count=1 -timeout=2m -json ./internal/management -run '^TestPreviewCancellation(TeamRouteCapabilities|SuspendedServiceRechecksCurrentGrants)$'
```

[run-selected.py](retained/run-selected.py) preserves the wrapper. Each run binds **86 selected source inputs** before and after, and each input set remained unchanged during its run. These bindings cover management source/tests, module declarations and activation inputs; they are **not the full transitive compiled closure**. This 11-outcome result remains separate from later backend and integrated compilation evidence.

[original/](retained/original/) preserves all four starting files. [before-sources/](retained/before-sources/) preserves the original production files plus the tested new regressions; [fixed-sources/](retained/fixed-sources/) preserves all four tested final files. [summary.json](retained/summary.json) records their final hashes and the explicit validation limits.

The separate [source review resolution](source-review-resolution.json) records the narrow independent review and exact inspected backend hashes. Three findings were resolved: matching cancellation to the latest published immutable identity, rechecking fresh Deploy authority after the mutex wait independently of context cancellation, and returning HTTP422 when cancellation wins the final ready receipt. Direct cases also cover source-guard availability and sealed publishers after bounded shutdown. No outstanding source finding remained in that review scope; its statement is not an additional test or integrated runtime claim.

All **31 original evidence files** are retained byte-for-byte beneath `retained/`, including their original manifest and checksums. [original-file-map.json](original-file-map.json) maps them to published paths. [publication-verification.json](publication-verification.json), [manifest.json](manifest.json) and `SHA256SUMS` bind this complete capsule. No new test, build or runtime operation was performed for publication.

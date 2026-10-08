# Native lifecycle correction evidence

These receipts preserve source checks for CLOUD-006, CLOUD-007 and
LOCAL-001/002/003. The tested source was an uncommitted correction snapshot. No
future commit is assigned, and an earlier full-package pass is not attributed to
later changes. Original artifact SHA-256 values and recorded source hashes travel
with these receipts; raw logs and private runtime data remain excluded.

| Receipt | Scope |
| --- | --- |
| [CLOUD-006](cloud006.json) | Real HTTP harness catalogue capability projection; full API package: 663 passing test events and five opt-in/platform skips, 39.474 seconds. All 573 recorded inputs remained stable during that run. This preceded the later disk and native-fixture corrections. |
| [CLOUD-007](cloud007.json) | Driver-aware pending-create disk estimate: two image copies for VFS, one for supported overlay drivers, refusal for unknown drivers/overflow, and durable pending reservations. Seventy race-test events passed in 6.898 seconds. Five selected source hashes retained. |
| [LOCAL-001/003 API](local001-local003-api.json) | Fifteen affected race-test events passed in 13.347 seconds. All 336 module/production/API-test inputs were stable. A dependency's product-only test file changed during the run; this command did not compile that file. The held-Create scenario budget changed, while production deadline contracts remain checked. |
| [LOCAL-001/002 product](local001-local002-product.json) | Portable paths, checked worktree lookup, and fixture isolation: 67 focused race-test events passed (one intentional subprocess-entry skip), 48.803 seconds. Full product package: 739 passing events, two skips, 137.107 seconds; all 578 recorded Go/module hashes stable. |
| [Native failures](native-reported-failures.json) | The original Windows batch failed four of 15 top-level tests. Its fixture changed the isolated checkout's HEAD. The subsequent fresh logging overlay identified the portable-path defect; it was a diagnostic, not a passing acceptance run. |

[Curation checks](curation.json) independently recount the retained test events and
compare the final product input manifest, API compilation-input superset and
selected catalogue/disk inputs with source bytes. These are timestamped byte
observations, not a future commit binding.

LOCAL-001 preserves traversal validation and converts the internal relative path
to portable slashes before invoking it. LOCAL-002 checks lookup errors and requires
registered fixture ownership before writes/Git, including failure cases that run
inside a disposable parent repository. LOCAL-003 fixes an unrealistically short
test scenario budget. These descriptions identify the corrections under test;
native Windows verification remains a separate gate.

The original native batch began at
`9898cf5d4a5302e431c8e42f5b3612c0a95e7061` and advanced unexpectedly to
`8f4deb8a4d7300a875b4e18ff44cf5e222fca9b1`. The reported unchanged Go blobs do not
make that later checkout pristine. Preserve both original receipts and the
contaminated isolated checkout. The source defect must not be hidden by resetting
the evidence. This cloud curation did not independently inspect the native raw
logs or recompute the reported diagnostic-overlay digest.

## Reproduce affected source checks

Use an isolated checkout containing the corrections and the repository's supported
Go version. These commands are recorded checks, not proof that a later checkout
has already passed them. No tests/builds/Docker actions were repeated during
curation.

```sh
go -C control-plane test -p 2 ./internal/api -count=1 -json
go -C control-plane test -race ./internal/api -run 'TestExecution|TestQueued(Dispatch|Admission)|TestSwarmTask' -count=1 -json
go -C control-plane test -race -p 2 ./internal/api -run 'TestSwarmTask|TestSwarmCheckout|TestProjectSwarm(TwoReviewed|PrepareSecond)|TestExecutionPendingCreate|TestTheCreateBudget|TestCreateFinishesAfter' -count=1 -json
go -C control-plane test -race -json ./internal/product -run 'Test(SwarmFixture|SwarmPath|SwarmReaperExit|ProjectSwarmHTTP|ProjectSwarmWorkspace|JourneyGit|NativePreviewFixture|GitCommit)' -count=1
go -C control-plane test -json ./internal/product -count=1
```

Installed four-project execution, real-provider output quality, physical Windows
backing-volume admission and native Windows recovery/resource acceptance remain
open. A source pass or package integrity check does not close those gates.

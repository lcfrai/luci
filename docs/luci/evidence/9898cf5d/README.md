# Task-worker lifecycle source evidence

These receipts describe checks associated with
`9898cf5d4a5302e431c8e42f5b3612c0a95e7061`. They do **not** establish a finished
local product. Its installed four-project attempt stopped before worker allocation
because the harness catalogue omitted task capabilities (CLOUD-006). Windows
backing-volume admission and native Windows acceptance also remain open.
Installed/browser receipts are maintained separately by their test owners.

| Receipt | Result and source scope |
| --- | --- |
| [Frontend](frontend.json) | 2,375 tests in 143 files, typecheck and production web build passed. All 473 recorded inputs match the commit. The preceding one-test contract mismatch is retained. |
| [Targeted Go checks](targeted-go.json) | Launch-fence/API/harness race checks: 143 passing test events; all 572 retained inputs match. Product lifecycle checks preceded the final overview change; their nine selected hashes match. Final overview/read-only checks passed in 22.070 seconds; three selected hashes match. |
| [Full Go baseline](go-baseline.json) | 2,640 passing and 25 skipped test events; 32 passing and two skipped packages. This ran **before** the final launch-fence and overview corrections. Twelve recorded input hashes differ, and the two final launch-observer/test files are absent from its manifest; this entire run is not bound to the final commit. |
| [Short controlled run](controlled-short.json) | 15-second requested workload, 21.269-second package result: four projects, peak three controlled workers, seven starts, six successes and one deliberate failure. Synthetic admission telemetry is distinct from real child processes. |
| [Package](package.json) | Clean-revision Linux amd64 package: 968 extracted members matched; 347 licence components, none missing. Integrity is separate from runtime correctness. |
| [Source export](source-export.json) | 1,480 source files; archive SHA-256 independently rechecked. No generated binaries or private runtime state are included in these tracked receipts. |

The separately reported full API package result of 45.160 seconds has only an
agent-reported tool transcript. There is no retained raw log or source manifest
for that run; the later race manifest is not substituted for it.

[Source binding](source-binding.json) records independent comparisons against Git
blobs, not the moving working tree. Checks that ran before the commit retain that
history. Selected-file hashes do not establish complete compilation provenance.
The short controlled run has a reported clean revision but no independent
compilation manifest. It neither repeats the [earlier sustained test](../e81b5860/README.md)
nor proves live-model quality, Docker enforcement, Windows performance or market
leadership.

Original ignored artifacts are referenced by repository-relative name and SHA-256.
Raw logs, private paths, fixture identities and captured contexts are omitted.
No test, build or Docker workflow was rerun while curating these receipts.

## Historical source commands — do not run affected fixtures at this revision

Native testing subsequently found **LOCAL-002**: this revision's unchecked
worktree error can make a lifecycle fixture write and commit into the test
checkout. Do not execute its swarm/product lifecycle fixtures from an ordinary
checkout. Use the corrected revision identified by the fix handoff. The commands
below document historical coverage; they are not authorization to reproduce the
known unsafe helper. LOCAL-001 also records four failed native lifecycle tests,
so these Linux receipts must not be described as native acceptance.

Use the repository's supported Go and Node versions and installed dependencies.
Run in an isolated checkout at the revision above. The following commands exercise
the recorded checks; running the full Go command now would be a **new** result,
not a reproduction of the historical baseline bytes.

```sh
python3 docs/luci/evidence/9898cf5d/verify-source.py
npm --prefix desktop test
npm --prefix desktop run typecheck
npm --prefix desktop run build
go test -p 2 -json ./control-plane/... ./sandboxd/...
go -C control-plane test -race -p 2 ./internal/api ./internal/harness -run 'TestExecution|TestSwarm|TestQueued|TestProjectSwarm|TestProjectRun|TestRestart|TestTask' -count=1 -json
go -C control-plane test -race ./internal/product -run 'TestProjectSwarm|TestSwarm|TestProjectRun|TestProductRegisteredReads' -count=1
go -C control-plane test -race ./internal/product -run 'TestSwarmTaskPermissions|TestSwarmResume|TestSwarmQueueRemainsAvailable' -count=1
go -C control-plane test -race ./internal/product -run 'TestProjectOverview|TestProductRegisteredReads' -count=1
```

The controlled fixture is explicitly opt-in. This Linux shell example writes its
private report to a caller-chosen location and starts controlled child processes;
it does not use a live model or Docker:

```sh
LUCI_SWARM_MULTI_PROJECT=1 LUCI_SWARM_MULTI_PROJECT_SECONDS=15 LUCI_SWARM_MULTI_PROJECT_REPORT=/tmp/luci-controlled-report.json go -C control-plane test ./internal/api -run '^TestProjectSwarmFourProjectControlledCapacity$' -count=1 -v
```

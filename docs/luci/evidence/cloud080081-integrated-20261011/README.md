# CLOUD-080 / CLOUD-081 integrated source checks

This capsule records the combined preview-admission and trusted runtime-installer candidate based on clean `2416135f89049efc21bcc647102c892b3152c692`. The immutable commit containing this capsule identifies the final source; `final-linux-source-bindings.json` binds the final changed product/test inputs. The earlier `final-source-bindings.json` is retained unchanged and predates the Linux wire correction. Linux source and controlled Docker SDK fixtures are verified here. No actual Docker build, Windows installer workload, live provider or latency pass is claimed.

## Final integrated checks

- `integrated/race-final-linux-wire-2`: **304 passing outcomes, 89 top-level tests**, zero failures/skips, 18.397 seconds; all 879 selected inputs unchanged. This final combined race batch includes the pinned-SDK Linux wire regression, current bundled source and cold typed demand, and supersedes the earlier overlapping 291-result batch for this final source.
- `integrated/linux-wire-before`: the unchanged regression reproduces both runtime and preview request rejection against the pinned Linux build API contract. Docker v28 rejects ImageBuildOptions.SecurityOpt on Linux (that field supports Windows credentials specs). Both production build option constructors now omit it; the serving container still retains no-new-privileges and capability drops. CPU/memory/swap/ulimit settings are unchanged and asserted through the actual pinned SDK wire request. No real Docker daemon was invoked by this regression.
- `integrated/race-final-linux-wire`: 303 outcomes pass and one older option assertion fails because it expected the invalid field. The assertion now requires nil SecurityOpt; the failure is retained. `linux-wire` includes exact before/after source and the pinned Docker router source. All other resource/ownership assertions remain. Earlier all-package compilation predates this narrow option/test change; final affected packages compile and execute in the final race batch.

- `integrated/race-1`: **291 passing outcomes, 86 top-level tests**, zero failures/skips, 81.488 seconds, 877 selected repository/workspace input bindings unchanged. This includes controlled API/ledger, Docker SDK, product, platform and current authority checks.
- `integrated/compile-2`: all control-plane packages compile with `-run '^$'`; **zero behavioral tests**. The first all-package compile failed because offline mode could not find the already-locked `gopkg.in/evanphx/json-patch.v4@v4.12.0`. `module-repair` records its checksum-verified download; no dependency declaration/lockfile changed. The second compile passes in 38.021 seconds.
- `integrated/cold-demand-1`: one top-level test plus eight subtests passes, preserving cold typed demand, worker-image requirements for pending worker allocations and existing headroom checks.
- `integrated/actual-bundle-1`: the actual current bundled Dockerfile, entrypoint and vendored sandbox sources pass read-only preparation with a controlled engine. No pull/build/tag occurs. This new test was added after the race batch; production files are unchanged.
- `integrated/managed-route-1`: the actual invited-member/backing-setup HTTP regression passes, including denial of installer cancel/reconcile. No daemon action occurs.
- The separate [UI capsule](../cloud081-runtime-install-ui-20261011/UI-VALIDATION.md) records 31 affected passes and full TypeScript success. These do not establish visual/native interaction or performance acceptance.

The 877/878/879 manifests are explicitly selected repository Go/module/JSON/HTML plus external workspace bindings, not a claim to hash every compiler/toolchain dependency. Exact commands, underlying exits, raw JSONL, errors and before/after selected inputs are retained. Compiling all packages is distinct from running their complete suites. Earlier staged batches overlap and are not additional unique coverage.

## Retained earlier evidence

The compressed archives contain original per-invocation frozen sources, overlays, commands and raw receipts; adjacent member manifests bind each uncompressed file. Overlay absolute paths describe the original execution workspace. To reconstruct an older overlay elsewhere, extract its frozen tree and translate only those root paths; do not silently substitute current files.

- Coordinator attempts 1 and 2 retain the missing combined `Server.previewRuntime` compile failure. Attempt 2 inadvertently continued after its three-way merge assertion failed; it is an additional failed wrapper invocation, not independent product evidence. Attempt 3 retains two pre-allocation/no-ledger fixture/protocol failures. Attempt 4 passes after exact JobID lookup and no-ledger handling were corrected. Final integrated race evidence supersedes these for the current source.
- Preview shared-admission compile attempts preserve the exhausted temporary storage and staged fixture/import problems; its corrected controlled batch passes.
- The authority before/after archives retain the deterministic cancelled-waiter false-success counterexample and correction.
- [Backend terminal evidence](../cloud080-backend-20261011/README.md) retains six false-terminal cases and the strict final-marker correction. [Product label evidence](../cloud080-label-followup-20261011/README.md) retains the original HTTP422 versus expected201 and its correction. Their counts overlap the integrated batch.

## Reproduce

Use pinned Go 1.26.3, existing module locks, `GOMAXPROCS=2`, and a private roomy temporary directory. From the repository root:

```sh
go test -race -p 1 -count=1 -timeout=4m -run 'TestClassicBuild|TestPreview|TestRuntimeInstall|TestRuntimeImage|TestColdTypedExecution|TestDeployment' ./control-plane/internal/api ./control-plane/internal/backend/dockerbe ./control-plane/internal/product ./control-plane/internal/platform ./control-plane/internal/management
go test -p 1 -count=1 -run TestColdTypedExecutionDemandRetainsCapacityAndPendingWorkerRequirements ./control-plane/internal/api
go test -p 1 -count=1 -run TestInvitedMemberImmediatelyCreatesPrivateProjectWithoutBackingSetup ./control-plane/internal/product
```

Windows may need per-package time budgets and `-race` requires a supported C toolchain. Do not interpret a cross-compile or controlled SDK fixture as native Docker acceptance. Follow the checkpoint's isolated runtime procedure; never bypass the existing host/backing-volume or authority gates.

## Remaining functional limits

The runtime installer retains the existing 32 GiB build allowance plus 8 GiB reserve. Classic Docker build RUN containers receive enforced CPU and memory/swap limits; daemon downloads and image bookkeeping share a reservation but are not independently cgroup-capped. Docker lacks atomic tag compare-and-swap against unrelated privileged writers; an observed differing tag is refused and Luci installer jobs serialize through the shared engine slot. Existing images are not overwritten as a shortcut.

Preview Dockerfiles require locally available immutable base images and the supported classic-builder syntax. Missing-base provisioning for arbitrary project images remains open. An uncertain build keeps its reservation; neither empty inventory nor cancellation proves remote daemon settlement. Exact successful/settled publication can be reconciled without Build/ImageTag replay, but every ambiguous interrupted build is not yet automatically recoverable. Actual bounded install, resource measurements, approved build-to-preview, Windows, live-provider quality and CLOUD-032 responsiveness remain open.

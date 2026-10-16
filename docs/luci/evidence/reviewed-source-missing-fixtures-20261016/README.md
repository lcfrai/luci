# Reviewed-source missing source fixtures

Source: coherent reviewed-source/assembler branch `2501f9f89` (includes LOCAL112–114 and the two corrected selective checks). Before this batch, cgroup charge was 33,057,214,464 of 34,359,738,368 bytes and host available memory was 2,436,132,864 bytes. No Docker, native preview, model, account or cloud action ran.

The exact serial command in `control-plane/` was:

```sh
GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=512MiB GOGC=20 GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache GOPATH=/dev/shm/luci-gopath GOTMPDIR=/dev/shm/luci-gotmp TMPDIR=/dev/shm timeout 240s /dev/shm/luci-toolchain/go/bin/go test -p 1 ./internal/api ./internal/backend/dockerbe ./internal/previewruntime ./internal/product -run '^(TestReviewedSourceUsesOneServingLeaseAndRetiresAcrossReopen|TestReviewedSourceLostCreateRetainsUnknownLeaseWithoutReplay|TestReviewedSourceCancellationBeforeStartPreservesExactCleanup|TestReviewedSourceRefusesEnvironmentDockerfileAndStaleAuthority|TestReviewedSourceWaitingAdmissionHasNoPhantomOwnedAllocation|TestReviewedSourceFreezesWholeSnapshotAndServesOnlyStaticBytes|TestReviewedSourceRejectsUnsafeOrOversizedSnapshotBeforeCreate|TestReviewedSourceStrictOwnershipBindsCommandImageAndOrdinaryLimits|TestSourceIdentityIsAdditiveAndCannotChangeKindSnapshotOrAcquireBuild|TestReviewedSourceReleaseRequiresExplicitKindAndExactCommit|TestReviewedSourceReleaseUsesReviewedSanitizedCommitWithoutDockerfile|TestReviewedSourceReleaseRefusesPrivateEnvironmentBeforeReservation|TestReviewedSourceReadinessRequiresExactBytesAndFullSnapshotReceipt|TestReviewedSourceJoinCASRejectsPublicAndRuntimeKindChanges)$' -count=1 -parallel 1
```

Result: exit 1. `internal/backend/dockerbe` passed (0.023s), `internal/previewruntime` passed (0.007s), and `internal/product` passed (0.191s). `internal/api` failed only `TestReviewedSourceRefusesEnvironmentDockerfileAndStaleAuthority` (environment, dockerfile, source, kind subcases) and `TestReviewedSourceWaitingAdmissionHasNoPhantomOwnedAllocation`. Both call the older `previewRetained(t, a, 0)` helper after pre-reservation refusal; it reports `{Status:unavailable executionReservationView:<nil>}` because no ledger or lock exists. The recently corrected LOCAL114 test instead proves exact ledger/lock absence and preserves the non-numeric unavailable observation. The remaining API assertions require the same fixture treatment by their dedicated owner; this does not prove a retained lease. Other selected API tests passed.

The independent inert server command was:

```sh
timeout 15s node --max-old-space-size=128 internal/backend/dockerbe/reviewed_source_preview_server.test.mjs
```

Result: exit 0, `PASS 20 inert static-server cases (fake HTTP; zero sockets/children/Docker)`. Previously passing environment, LOCAL114, coordinator handler and assembler fixtures were not rerun.

## Owner fixture correction and selective verification

The owner fixture-only revision `08e816ed81380e6ed970fe8440d1a8df5469ac97` was merged with this branch's `9744d033f` receipt at `0e67fe30fee0c33f826c15da8b82ee5628d1f77f`. It proves exact ledger and lock absence before and after pre-reservation refusal, while retaining the truthful unavailable, nil-numeric observation. No production code changed in this correction.

The exact serial command in `control-plane/` was:

```sh
GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=512MiB GOGC=20 GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache GOPATH=/dev/shm/luci-gopath GOTMPDIR=/dev/shm/luci-gotmp TMPDIR=/dev/shm timeout 120s /dev/shm/luci-toolchain/go/bin/go test -p 1 ./internal/api -run '^(TestReviewedSourceRefusesEnvironmentDockerfileAndStaleAuthority|TestReviewedSourceWaitingAdmissionHasNoPhantomOwnedAllocation)$' -count=1 -parallel 1 -v
```

Result: exit 0. `TestReviewedSourceRefusesEnvironmentDockerfileAndStaleAuthority` passed all four subcases; `TestReviewedSourceWaitingAdmissionHasNoPhantomOwnedAllocation` passed. Package result: `ok github.com/lcfrai/luci/control-plane/internal/api 0.043s`. These are source fixtures only; no Docker, native preview or runtime acceptance is claimed. No other tests were rerun for this join.

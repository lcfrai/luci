# Automatic repair budget source check — 2026-10-16

Reviewed backend input: `b2d596d815933249c5df29b6187ed849fec984a6`.
Merged onto the isolated PDF pilot branch at `51d2cddc23f342637b655ea1373dea732d06c075`.
All changed Go files were formatted once with Go 1.25.0 `gofmt`; formatting changed `cmd/api/e2b_config_test.go` and `internal/product/orchestrator_policy.go`. `git diff --check` passed.

The source test ran once from `control-plane` with `GOWORK=off`, `GOMAXPROCS=1`, `GOMEMLIMIT=512MiB`, `GOGC=20`, existing task caches under `/dev/shm/luci-*`, `go test -p 1 -parallel 1 -count=1`, and a 360-second whole-command timeout. Initial cgroup charge/max are in [precheck.txt](precheck.txt). The exact command selector was:

```
^(TestAutomaticRepairLimitPolicyHTTPPreservesZeroAndRejectsInvalid|TestAutomaticRepairLimitContractCaptureAndPolicyStaleness|TestAutomaticRepairLimitLegacyReadDoesNotRewriteDigests|TestAutomaticRepairBudgetExactOrdinalAndNoRefund|TestCoordinatorAutomaticRepairBudgetZeroAndOneActualJourney|TestCoordinatorWaitsForCandidateThenBindsReservedReviewerBeforeDispatch|TestCloudControllerActualEffectCallbackRechecksRepairBudgetAndPreservesCleanup|TestSwarmCheckpointActualHTTPRechecksUnsettledAndRevokedAuthorityBeforeCopy|TestProjectRunRevalidatesSourceAfterRegistryWriteBeforeBackendAllocation|TestQueuedAdmissionRefusesRevokedActorBeforeCapacityOrAllocation)$
```

Packages: `./internal/product ./internal/api ./cmd/api`. Exit 0; all three packages passed. Raw output: [go-test.log](go-test.log), SHA-256 `08f07239a9329bd2b359bffc47501c4c77003b020cfd7209a48db99460bbc86c`.

Unchanged desktop tree `4e1b739bd7acae8a44422626f820fbbe56fc7568` retained its previous 164-test and TypeScript evidence; no desktop rerun. This is source-only evidence, with no local runtime or provider execution.

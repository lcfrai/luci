# LOCAL-096 source check, 16 October 2026

Source commits: `24cf11ffa` with the new continuation/review path and `a26931f17` with the first-effect contract preflight, on `codex/local096-continuation-contract`, based on `798aaba3fd6f85229beb6e815defc8121dee5b99`.

New coordinator continuations require a named approved execution contract. Before the new row is retained, a read-only preflight checks its actor, selection, route and current approval. The ordinary run creation gate still verifies its full route, captured Design, source, controls, actor and approval before allocating work. Automatic candidate integration refuses a coordinator row or run without that contract. An accepted amendment uses only a current actor-owned exact approval; the saved route never supplies approval, and pending source commit/reconciliation does not supersede old work. Legacy rows remain readable for recovery.

The desktop Continue flow now retains the exact selected route, task set, check/delivery delegation and request keys before effects. It prepares a contract, requires explicit review/approval, then sends the exact reference on the existing continuation route. Lost responses require readback; no automatic resubmission occurs.

Checks, run serially with no Docker, model, provider or cloud effects:

```sh
cd control-plane
GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=512MiB GOGC=20 GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache TMPDIR=/dev/shm GOTMPDIR=/dev/shm /dev/shm/luci-toolchain/go/bin/go test -p 1 ./internal/product -run '^(TestCoordinatorContinuationRequiresApprovedContractBeforeNewWork|TestCoordinatorContinuationRunsTypedPreparationQueueAndExactlyOnceAdmission|TestCoordinatorFirstCloudActivationQueuesExactReviewedTasksWithoutLocalFallback|TestAcceptedAmendmentWaitsForFreshExecutionContract)$' -count=1 -timeout=120s
# exit 0; go.log SHA-256 f4dbac0186dc2d05a176b2608378473c49f6d9f3cfa1c531f1fd1438644637ed

cd ../desktop
TMPDIR=/dev/shm NODE_OPTIONS=--max-old-space-size=768 npm test -- src/components/orchestrator/CoordinatorExecution.test.tsx src/components/develop/ExecutionContractReview.test.tsx --maxWorkers=1 --no-file-parallelism
# exit 0; 80/80; ui.log SHA-256 b49510ca80b403451aaf8c56081eddbcb38a15dbd9da797476affff264cd999b

TMPDIR=/dev/shm NODE_OPTIONS=--max-old-space-size=768 npm run typecheck
# exit 0; typecheck.log SHA-256 07c98bedf7c154a3ea2dc186fb6846a1b1fef080176372e7088f389ff509cd4d
```

These are source checks. The joined desktop, API, approved-contract continuation, independent review and integration have not been exercised in an installed local run. An accepted amendment without a new current approved contract remains pending for human review; it does not automatically inherit the old route's approval.

# LOCAL-096 source check, 16 October 2026

Source commits: `24cf11ffa` with the new continuation/review path, `a26931f17` with the first-effect contract preflight, and `abc39ef0c` with the configured-model correction, on `codex/local096-continuation-contract`, based on `798aaba3fd6f85229beb6e815defc8121dee5b99`.

New coordinator continuations require a named approved execution contract. Before the new row is retained, a read-only preflight checks its actor, selection, route and current approval. The ordinary run creation gate still verifies its full route, captured Design, source, controls, actor and approval before allocating work. Automatic candidate integration refuses a coordinator row or run without that contract. An accepted amendment uses only a current actor-owned exact approval; the saved route never supplies approval, and pending source commit/reconciliation does not supersede old work. Legacy rows remain readable for recovery.

The desktop Continue flow now retains the exact selected route, task set, check/delivery delegation and request keys before effects. It prepares a contract, requires explicit review/approval, then sends the exact reference on the existing continuation route. Lost responses require readback; no automatic resubmission occurs.

Checks, run serially with no Docker, model, provider or cloud effects:

```sh
cd control-plane
GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=512MiB GOGC=20 GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache TMPDIR=/dev/shm GOTMPDIR=/dev/shm /dev/shm/luci-toolchain/go/bin/go test -p 1 ./internal/product -run '^(TestCoordinatorContinuationRequiresApprovedContractBeforeNewWork|TestCoordinatorContinuationRunsTypedPreparationQueueAndExactlyOnceAdmission|TestCoordinatorContinuationAcceptsConfiguredModelFromApprovedConnection|TestCoordinatorFirstCloudActivationQueuesExactReviewedTasksWithoutLocalFallback|TestAcceptedAmendmentWaitsForFreshExecutionContract)$' -count=1 -timeout=120s
# exit 0; go.log SHA-256 a72a3330bf510a7481aab8f8b3fb75c8be9f6ce2c291eb76df633038978d0dd4

cd ../desktop
TMPDIR=/dev/shm NODE_OPTIONS=--max-old-space-size=768 npm test -- src/components/orchestrator/CoordinatorExecution.test.tsx src/components/develop/ExecutionContractReview.test.tsx --maxWorkers=1 --no-file-parallelism
# exit 0; 80/80; ui.log SHA-256 b49510ca80b403451aaf8c56081eddbcb38a15dbd9da797476affff264cd999b

TMPDIR=/dev/shm NODE_OPTIONS=--max-old-space-size=768 npm run typecheck
# exit 0; typecheck.log SHA-256 07c98bedf7c154a3ea2dc186fb6846a1b1fef080176372e7088f389ff509cd4d
```

These are source checks. The joined desktop, API, approved-contract continuation, independent review and integration have not been exercised in an installed local run. An accepted amendment without a new current approved contract remains pending for human review; it does not automatically inherit the old route's approval.

## LOCAL-099 accepted amendment reconciliation successor

The accepted-event background path now reads back its exact pending context commit before considering new work. A confirmed journal entry is not treated as a perpetual reservation. An ambiguous commit remains pending and neither repeats the Git write nor starts a replacement. Once the source is clean, an actor-private read projects the precise changed task selection and a waiting or review-required reason. The user must prepare and approve a new current contract; the approved route then resumes under its normal authority and freshness gates. Task ordering differences between the sorted request and approved plan no longer reject the same exact unique selection.

Focused source fixture covers pending to confirmed readback, ambiguous retained pending without replay, refusal before approval, actual prepare and approval handlers, replacement run capture, candidate handoff, addressed independent reviewer finding, passed review, reviewed integration, and combined check reservation. The two older accepted-event fixtures now explicitly approve the changed scope before testing restart and stop uncertainty.

Retained checks from the current successor, run serially without Docker, model, provider, or cloud effects:

```sh
cd control-plane
GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=384MiB GOGC=10 GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache TMPDIR=/dev/shm GOTMPDIR=/dev/shm /dev/shm/luci-toolchain/go/bin/go test -p 1 ./internal/product -run 'Test(AcceptedAmendment|CoordinatorAccepted)' -count=1
# exit 0; ok github.com/lcfrai/luci/control-plane/internal/product 8.012s

cd ../desktop
TMPDIR=/dev/shm NODE_OPTIONS=--max-old-space-size=768 npm test -- --run src/components/orchestrator/CoordinatorExecution.test.tsx src/api/coordinatorContinuations.test.ts
# exit 0; 2 files, 131 tests
TMPDIR=/dev/shm NODE_OPTIONS=--max-old-space-size=768 npm run typecheck
# exit 0
```

These checks establish source behavior only. Installed Windows execution and actual independent agent review/check outcomes remain open.

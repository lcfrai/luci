# LOCAL110 and reviewed-source coordinator source checks

Source heads: `67029c2c2` (LOCAL110 only), `34e3f4e94` (kind-aware coordinator helper), `c712ba827` (desktop preview selector/client), `36662822b` (coherent merge of remote main `707676f7d`). No Docker, model, cloud, preview release or image was started by these checks.

## Passing commands

All desktop commands ran in `desktop/` with a single Vitest worker or a 768 MiB Node heap. The task-owned `/dev/shm/luci-go-cache` was cleaned with `go clean -cache` before TypeScript after the previous exit-137; shared cgroup charge decreased from 33,759,596,544 to 32,333,160,448 bytes. No source or retained receipt was removed.

| Source | Exact command | Result |
| --- | --- | --- |
| `67029c2c2` | `TMPDIR=/dev/shm NODE_OPTIONS=--max-old-space-size=384 npm test -- --maxWorkers=1 src/components/orchestrator/CoordinatorExecution.test.tsx` | exit 0; 1 file, 77 tests |
| `67029c2c2` | `NODE_OPTIONS=--max-old-space-size=768 ./node_modules/.bin/tsc --noEmit --pretty false` | exit 0 |
| `34e3f4e94` | `GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=512MiB GOGC=20 GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache GOPATH=/dev/shm/luci-gopath GOTMPDIR=/dev/shm/luci-gotmp TMPDIR=/dev/shm /dev/shm/luci-toolchain/go/bin/go test -p 1 ./internal/product -run 'TestCoordinatorPreviewKindCannotInheritLegacyApproval' -count=1` | exit 0 |
| `c712ba827` | `TMPDIR=/dev/shm NODE_OPTIONS=--max-old-space-size=384 npm test -- --maxWorkers=1 src/api/coordinatorContinuations.test.ts src/api/coordinatorPolicy.test.ts src/components/orchestrator/CoordinatorPreviewSetup.test.tsx src/components/orchestrator/CoordinatorExecution.test.tsx` | exit 0; 4 files, 166 tests after correcting two changed-copy selectors; the preceding attempt exited 1 with exactly those 2 selector failures and 163 passes |
| `c712ba827` | `NODE_OPTIONS=--max-old-space-size=768 ./node_modules/.bin/tsc --noEmit --pretty false` | exit 0 |
| `c712ba827` | `GOWORK=off GOMAXPROCS=1 GOMEMLIMIT=512MiB GOGC=20 GOCACHE=/dev/shm/luci-go-cache GOMODCACHE=/dev/shm/luci-gomodcache GOPATH=/dev/shm/luci-gopath GOTMPDIR=/dev/shm/luci-gotmp TMPDIR=/dev/shm /dev/shm/luci-toolchain/go/bin/go test -p 1 ./internal/product -run 'TestCoordinatorPreviewKindCannotInheritLegacyApproval|TestCoordinatorPreviewPolicyIsExplicitPreservedAndCurrentlyAuthorized|TestCoordinatorSavedPreviewPlanCannotBeSubstitutedOrDropped' -count=1` | exit 0 |

The initial Go command without `GOMODCACHE` failed before compile because the default `/home/agent/go/pkg/mod` could not be created on the full overlay. The explicit task-owned cache path above completed.

## Existing broader fixture incompatibility

`go test -p 1 ./internal/product -run 'TestCoordinatorPreview|TestCoordinatorSavedPreviewPlan' -count=1` against the 15fb continuation-contract source reached the older preview journey fixture and failed: its changed run/policy request retained an approval for different contract bytes (`POST .../orchestrator/continuations` returned `revision_conflict` 409). A temporary local fixture reapproval reached the next missing independent-review obligation (`controlled native source was not reviewable`); that temporary edit was reverted and is **not** in the source heads. The old journey fixtures need a full current approved-contract and reviewer/candidate setup before they can exercise the new path. The three focused Go checks above passed; the broader failure is not an installed/runtime claim or evidence that the new kind reached a release.

The release owner still has to call `admitCoordinatorPreviewKind` from the new kind-aware endpoint and expose immutable release kind for coordinator readback. The source candidate keeps the old helper as an empty-kind wrapper and refuses cross-kind policy/plan substitution. No actual reviewed-source release has been observed.

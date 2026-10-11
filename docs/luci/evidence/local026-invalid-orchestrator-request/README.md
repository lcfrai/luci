# LOCAL-026 — invalid conversation requests return HTTP 400

A validly shaped `POST /v1/product/projects/{id}/orchestrator/coordination/intents` referring to a nonexistent proposal returned HTTP 500 because `orchestratorProblem` omitted the existing `ErrOrchestratorInvalid` sentinel. The correction maps that sentinel, including wrapped errors, to HTTP 400 with `invalid_orchestrator_request`. Revision conflicts remain 409; unexpected storage failures remain 500 with private details suppressed.

The actual registered HTTP regression asserts that the refused request changes neither project files nor private state and allocates no preview, tunnel or job. The identical fixture fails before the two-line mapping and passes afterward. Original: two top-level failures, two invalid-error subcase failures, two passing control subcases. Corrected: **2 top-level + 4 subtests passed**, no failure/skip. Counts overlap; they are not eight unique passing tests. Both invocations retained all 861 effective repository Go/module input hashes unchanged.

## Exact scope and reproduction

Product-package sources were restored through explicit Go overlays to committed `544ae842c1db4cbdf3ef1ae8055289db9115f4c5`, excluding unrelated pending archive/preview source. Both runs add the exact same `orchestrator_problem_test.go`; the fixed run changes only the captured error-mapping lines. Frozen original/fixed source, fixture, patch, overlay bindings, exact commands, raw JSONL/stderr, statuses and input manifests are included. Other dependency packages/module files were the current captured workspace inputs. This is not a clean historical whole-checkout build, a full compiled-dependency provenance claim or a Windows/runtime/provider test. External module/workspace originals were not copied into this capsule.

The declared overlay paths are the original execution paths. Their baseline source bytes are retrievable from the pinned Git commit; excluded unrelated source is the literal `package product\n` empty compilation unit. `head-overlay-bindings.json` identifies every replacement; the corrected whole-file source and fixture are included. The compressed before/after manifests have decoded hashes and lengths in `original-bindings.json`.

On the corrected source with the documented Go environment, the affected test command is:

```sh
cd control-plane
GOMAXPROCS=2 go test -p 1 -count=1 -timeout=90s -json ./internal/product \
  -run '^(TestCoordinationMissingTurnReturnsBadRequestWithoutMutation|TestOrchestratorProblemKeepsConflictAndStorageFailureDistinct)$'
```

A prior unrelated guest build exhausted the cloud root filesystem. These test invocations used the root-approved test-only `GOCACHE=/dev/shm/luci-go-cache-20261011-recovery`, with root free-space floor 512 MiB, effective cgroup remaining-memory floor 4 GiB and cache limit 2 GiB, sampled each second. Both completed without a guard stop. This setup adjustment changes no product admission predicate; its samples are not app performance evidence. No application startup/configuration change is required. Windows retest: submit the exact nonexistent-turn request with current trusted project authority and confirm 400/nonmutation, retaining other recovery and full-product gates.

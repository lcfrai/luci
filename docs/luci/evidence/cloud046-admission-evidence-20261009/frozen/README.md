# CLOUD046: actual final-admission evidence

The backend retains the actual CPU or memory refusal operands and the sampled/decision timestamps on the exact session, atomically with its existing canonical `StartFailure`. No snapshot is reread to manufacture evidence. Existing admission predicates, resource policies, public fixed reasons and retry/release behavior are unchanged.

The new `startAdmissionEvidence` field is private in registry storage. Only the existing single-session GET can expose it, under the scheduler's host administrator/local host boundary; team contexts, ordinary members and disabled actors receive no operands. Lists, shared session DTOs, mutation responses and run/queue projections remain unchanged. Both existing `StartFailure` reset sites clear the private field for a new generation. Invalid optional operands are discarded without preventing the canonical failure from being persisted.

`metric` selects `host_cpu_utilization_percent`, `host_available_memory` or `execution_total_memory`. Numeric fields are always present within evidence, so zero is distinguishable from absence. The unused numeric pair is zero. CPU percentages are finite and within 0–100; the observation must exceed the recorded maximum. Memory uses exact unsigned integer bytes and the observation must be below the recorded requirement. The timestamp difference must be within the original five-second freshness bound. Historical records are validated against their decision time, never against current capacity.

## Evidence

- `before/`: unchanged production code plus the new behavioral regression. All three CPU/host-memory/execution-memory cases failed at “actual final admission operands were not retained,” after asserting final refusal, canonical reason and zero backend Create calls. The regression uses existing APIs and a JSON map; this was not a missing-type compilation failure.
- `validation-1/`: 33 top-level tests, 98 named test/subtest results and 85 leaf cases passed; zero failures/skips. The three actual-dispatch regression cases now pass, including exact final sample/time and file-store reopen. Other checks cover malformed/nonfinite operands, measured zero, current authority/demotion/disabled actors, team redaction, list/mutation exclusion, lifecycle clearing and existing admission/start-failure behavior.
- `receipt.json`: affected source SHA256 bindings, summaries and artifact hashes.
- Each stage retains raw `go-test.jsonl`, `go-test.stderr`, the exact command/exit status in `execution.json`, raw `go-list.jsonstream`, and source hashes. `declared-compile-source-inputs.json` selects the actual compile/embedding fields for each listed package variant (2,334 before and 2,435 final files); the broader `compiled-inputs.json` additionally binds declared dependency test metadata. External `go.work` and `go.work.sum` are included. No captured input changed during validation.
- `validation-1/source/` retains copies of all ten affected Go files. Independent read-only review by `candidate_review` identified no actionable privacy, lifecycle or operand defect.

## Reproduction

From `/workspace/luci/control-plane`, activate the retained toolchain and run only the bounded affected test selection:

```bash
source /workspace/.luci-tools/activate.sh
GOMAXPROCS=2 go test -p 1 -count=1 -timeout=180s -json ./internal/api ./internal/registry ./internal/product -run '^(TestAdmissionEvidence.*|TestStartAdmissionEvidence.*|TestSessionStartFailure.*|TestQueuedStartFailure.*|TestExecutionAdmission.*|TestQueuedDispatchRechecksFreshAuthorityAfterDurableClaim|TestQueuedAdmissionRefusesRevokedActorBeforeCapacityOrAllocation|TestExecutionReservationViewDoesNotWidenSchedulerAuthority|TestManualTaskStartFailureIsObservedWithoutQueueOrReconcile|TestSwarmSchedulerSeparatesFailedStartFromFailedWorkerAndNormalExit|TestSwarmFailedQueueRetainsDiagnosisWhileCleanupRetries)$'
```

Pinned Go was 1.26.3. The retained caches were used; retired default caches were not recreated. No installed runtime, performance acceptance, whole API suite, original fixture mutation, external service call or commit was performed. The original attempt6 CPU percentage remains unavailable; this records future refusals only. Product UI presentation is not implemented here; authorized session GET and retained private evidence are available.

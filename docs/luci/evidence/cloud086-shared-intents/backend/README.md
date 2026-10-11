# CLOUD-086 shared-intent source checkpoint

This is a partial, controlled Linux source checkpoint. CLOUD-086 remains open: it does not establish real two-person Luci negotiation, upstream provider execution, microVMs, Windows behavior, checks/review or actual deployment.

The backend automatically captures successful proposals with authenticated personal origin, immutable base/proposal identity, affected task/requirement scope and causal events. Existing apply operations reconcile structurally disjoint changes while preserving original model input and proposal bytes. Overlap requires an explicit current authorized resolution; an actor can supersede only their own intent. Structural compatibility is not semantic consensus.

Current authority, current-context CAS and exact existing apply receipts govern writes and replay. Uncertain canonical writes cannot be rebased/reexecuted. Pending and accepted origins remain available through conversation rotation. Copied/corrupt authoritative records fail closed; missing completion receipts recover all retained pending origins before application, including opposition. Whole-context acceptance hashes stay unknown where only the canonical target receipt is proven. Reads never dispatch, settle resources or repair state.

## Evidence

- `original/result.json`: identical final HTTP fixture against two preserved original production files fails at the absent coordination GET route (404), 1 test failure, no timeout. Compilation succeeded. All 828 covered repository Go/module inputs remained unchanged across that invocation.
- `fixture/coordination_http_test.go`: exact same test body used for original/fixed comparison, SHA256 `50bf8c124797bafbd3fb1c0576d91556837540e5d815c1dd0ab4ac03c106a1a9`.
- `validation-binding.json`: hashes/binding to the joint C085/C086 fixed receipt. Its regular run passed 40 top-level tests and 42 subtests, with 0 failures/0 skips; these are hierarchical outcomes, not 82 independent workloads. Raw joint logs, source-before/after hashes and retained earlier failures live in [C085 backend evidence](../../cloud085-coordinator/backend/README.md).
- `final-source.json`: the seven C086 source/test files for this checkpoint. Shared registration, completion, apply, recovery and rotation hooks are part of the joint C085 source/evidence, not separately claimed here.
- `original/source.before.json.gz` and `source.after.json.gz` preserve the full losslessly compressed repository input manifests. They exclude external module/Go SDK inputs; no exact executed-binary provenance claim is made.

Original invocation/overlay paths describe the actual retained cloud workspace. For reproduction elsewhere, map the two overlay keys to the local checkout and values to `original/source`, retain the exact fixture, and use the recorded command. Other source remains the matching integrated checkpoint; this counterexample is not a clean complete historical-tree build.

## Open lifecycle work

The accepted-event consumer must carry affected work into C085's current-authority typed continuation without blanket restarts or replay, then through independent checks/review and policy-permitted actual deployment. That full team delivery loop is unproved.

This checkpoint bounds the private coordination ledger at 64 retained intents / 8 MiB. It conservatively refuses more capture at capacity. Settled-intent archiving/readback and dependency/event-consumer retention are still missing, so sustained use can hit this limit; retaining every origin forever is not the completed design. This is a known open limitation, not a release-ready lifecycle claim.

No live provider, browser, Docker allocation or production fixture was used by these source regressions. Earlier failed installed acceptance and uncertain charges retain their original scope.

# CLOUD-017 source verification

The manual Start action now validates the acknowledged child, reads the current
run revision and reconciles that existing worker once. It uses the returned
status instead of treating a read of the dispatch record as a live observation.
If the observation fails, the acknowledged child remains inspectable and the
current batch stops. No reservation or session start is replayed.

[Verification receipt](verification.json): **90 tests passed in five files**,
zero failed/skipped/todo, and TypeScript passed on 2026-10-08 from
14:28:49 to 14:29:31 UTC. The [432-input manifest](source-inputs.json) was identical
before and after. These hashes include the uncommitted correction on base
`29be4018f951ba2b3f33ae63067e4b7257da0c7a`; that base revision alone does not contain
the fix. The receipt includes exact commands, timings, exit codes and hashes of
the original result, logs and manifests. Only the checkout prefix is normalized
to `<repo>` in the public commands.

The tests cover dispatching until reconciliation, fresh revisions across two
independent starts, failed/stale observation, HTTP 409, lost or mismatched 201,
retained session access and late responses after scope or authority changes.
The old fixture incorrectly changed the operation directly to running on Create;
the corrected fixture changes it only through reconciliation.

The [original installed attempt](../../../../scripts/acceptance/task-continuation/evidence/29be4018-attempt1.json)
remains failed. Its helper waited for a running operation without production
observation, leaving the acknowledged worker gated behind a scripted-provider
prerequisite until timeout. Independently, the UI contradicted its successful
start acknowledgement with an unconfirmed-start label. The linked artifact owns
that failure evidence; it is not duplicated or converted into a pass here.

This receipt establishes a controlled source contract, not installed, native
Windows, Docker, live-model or performance acceptance. It is not another full
frontend-suite run or build. The matching installed continuation must still
observe the app's automatic reconciliation and the actual task status without a
manual fallback. CLOUD-018 background completion and report observation remains
a separate change.

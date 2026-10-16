# Develop independent integration progress — source candidate

Base: reviewed main `14ad76892447cea7a7311a8b334d59415c027322`.
Scope: one production candidate-loop correction and one new regression source.
The frozen PDF branch remains at `6aab39a772b32a2d36e0c3d8a290426819dad51a`.

## Observed gap and ownership

In `internal/product/coordinator_integration.go`, a complete first candidate
without its independent review receipt returned from the whole candidate scan.
A later task with its own exact passed review therefore could not integrate.
The main checkout could remain blocked by task order even though scoped work
was ready. The early scope handoff reserved only this product function and
`coordinator_integration_progress_test.go`; API native/census/auth, shared
resource ledgers and the PDF files are untouched.

The new branch continues the scan only when that candidate is otherwise
reviewable but lacks its independent receipt. Source-inspection errors,
unreviewable source, invalid digests, current policy/actor checks, the final
integration handler, exact target binding and unknown integration recovery
retain their existing behavior. Each coordinator turn still performs at most
one integration. This does not approve the waiting candidate or run checks on
a partially integrated run.

## New regression source, execution held

`TestCoordinatorPendingReviewDoesNotStrandLaterReviewedIntegration` creates a
fresh approved four-task contract and normal continuation. Two stopped coding
candidates retain their handoffs and an attributed addressed message. Only the
second initially has an independent reviewer, delivered exact patch, finding
and resolved obligation. The fixture asserts:

- A foreign actor changes no durable run state.
- The second candidate integrates through the actual handler despite the
  first pending review; the first file is absent from the target.
- The same observation cannot replay integration, and partial completion
  cannot reserve or claim combined checks.
- After the first candidate receives its own freshly bound review, both
  changes reach the normal check handler and exact checked-source receipt.

The deferred fixture uses existing temporary Git worktrees and the bounded
Node check fixture to inspect both actual integrated files. Upstream workers
are controlled test outcomes. It is not provider, Docker, swarm performance
or Windows acceptance. The existing bounded repair implementation is unchanged;
this candidate does not close the separately recorded failed-combined-check
repair gap or change stale-review handling after target movement.

No formatter, compiler, test, capacity query, model, native tool, SDK,
container or acquisition was run for this source candidate. Only source reads,
edits and Git whitespace inspection were used. The focused future selector is
`^TestCoordinatorPendingReviewDoesNotStrandLaterReviewedIntegration$` in
`./internal/product`; execution requires a later explicit source-check release.

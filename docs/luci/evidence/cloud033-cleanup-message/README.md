# CLOUD-033: truthful check cleanup copy

The actual installed dependency run on product `4df94ab9` completed its check and confirmed exact cleanup, but retained the command text “exact container cleanup is pending.” ProjectChecks renders that command message verbatim. This capsule records a source correction and controlled regression evidence; it is not an installed correction or Windows acceptance witness.

The backend updates only known completed-command cleanup copy after exact acknowledged-container absence and before the new outcome digest is created. Unknown output, admission refusal, cancellation, authority and resource proofs keep their meaning. Already-finished historical receipts are not rewritten or rehashed. The component maps only the four exact stale strings for a cleanup-confirmed command with matching recorded passed/failed exit, stage and reason. Matching terminal run text additionally requires resourcesHeld=false. The API result object, output and stored digest remain unchanged.

Final corrected Go race checks: 9 top-level tests and 17 subtests passed across the new and six affected test groups, zero failures/skips. The final original-code regression retains the four expected behavioral failures (and its parent failure), with controls passing. Initial fixture projection error is preserved separately. The component's original code fails four new legacy display cases; six uncertainty controls pass. The corrected entire ProjectChecks file passes 61 tests; final TypeScript passes. Counts from repeated/focused rounds overlap and are not added.

`before/` and `after/` freeze the changed code and unchanged final regression. `attempt1-source/` and `go-evidence/` preserve the initial wrong projection comparison. `go-evidence-final/` contains final original/corrected/affected raw JSONL, stderr, commands, cache size and source bindings. `frontend-evidence/` preserves original/corrected raw reports, full copied source bindings, the first missing-support-file typecheck error and final success. `installed-source-finding.json` binds the exact original installed receipt without changing it.

For reproduction, apply `candidate.patch` to an isolated checkout matching the base source, activate the supported toolchains, and run from `control-plane`:

```
GOMAXPROCS=2 go test -race -p=1 -count=1 -timeout=8m ./internal/api -run '^TestProjectCheckHTTP(CleanupConfirmationReplacesPendingMessage|FinishedLegacyCleanupMessagePreservesDigest|CleanupConfirmationPreservesOtherDiagnoses|AccountsSharePendingCapacityBeforeCreate|CleanupFailureKeepsOutputAcrossReopenWithoutReplay|FinishedReceiptSurvivesFinalPublicationBoundary|FinishedObservationPreservesLaterCommandOutcome|UnacknowledgedCreateRequiresReviewedFenceAndFreshEditor|RechecksRevokedAuthorityBeforeCreate)$'
```

Run `npm exec -- vitest run src/components/journey/ProjectChecks.test.tsx --maxWorkers=1` and `npm run typecheck` from `desktop`. Do not apply patches to an active personal checkout. The archived runner paths describe this cloud workspace; their raw commands and overlay mappings are exact historical evidence, not a portable preconfigured Windows launcher. Go supports omitting `-race` on a host lacking its required compiler; that would be a different validation scope.

The complete product, sustained workload, recovery and Windows gates remain governed by the main worklist. This local copy fix does not close them.

# Live preview cancellation — CLOUD-080 / CLOUD-082

The preview screen now exposes **Cancel build** while its Start request is still running. Cancellation saves intent for the exact live release under current Edit/Deploy authority, then signals only that operation. It does not acquire the build lifecycle lock, invoke Docker, replay work or release uncertain capacity. Ordinary Stop remains the control for an acknowledged serving allocation.

The UI distinguishes requested, confirmed and unconfirmed cancellation. An uncertain transport result requires a subsequent explicit Refresh before another action. Project/authority changes suppress stale private responses. Controller shutdown signals owned previews, joins publication for one five-second budget and then seals late publishers; restarting cannot recreate ownership of an old operation.

## Evidence and limits

| Scope | Final recorded result | Receipt |
| --- | --- | --- |
| Product authority, exact identity, revision merging and shutdown | 87 race outcomes / 36 top-level tests; original cancellation/Close and later grant-revocation failures preserved | [Product](product/README.md) |
| Reopen before allocation or after builder settlement | Same four cases: two original failures/two controls; all four corrected cases and their parent pass with race detection | [Restart follow-up](product-reopen/README.md) |
| Managed route authorization and suspended-team cleanup access | 11 outcomes / 2 top-level tests; original route failures preserved | [Management](management/README.md) |
| Registered HTTP → actual API lifecycle and durable ledger | One race test; unknown exact charge survives reopen, independent release unaffected, no replay | [API integration](api/README.md) |
| Frontend contract and intervention controls | 66 affected tests, TypeScript success; 458 stable desktop inputs | [Frontend](frontend/README.md) |
| Styled desktop and phone interaction | Held Start, keyboard Cancel, exact single POST, project change, retained uncertainty and explicit Refresh at 1440/390 | [Browser attempts](frontend/README.md) |

These are separate controlled scopes, not a summed full-suite count. The 87-outcome product run and API bridge precede the narrow restart-observation follow-up; that follow-up changes only inactive cancellation projection and has its own original/fixed evidence. It leaves durable history and any charge unchanged, exposes unknown rather than indefinitely cancelling, and does not invent a new cancellation handle. Each capsule preserves exact source bindings, commands, failures, outputs and checksums. The browser uses the actual styled component and HTTP client with a controlled server, not an installed controller or Docker. Its final cleanup records eight absent observed child identities and two Chromium PID-1 zombies, not zero PIDs.

Actual installed cancellation, cold pulls, unknown Docker-build settlement, native Windows execution and performance remain open. The earlier matching `80462898` warm multistage build/serve/ordinary-Stop pass does not verify these new cancellation bytes. No original failed fixture or charged reservation was edited.

## Contract and retest

`POST /v1/product/projects/{project}/releases/{release}/cancel` takes the current positive `expectedRevision`. HTTP 202 records a new intent; HTTP 200 acknowledges the same originally submitted revision while its exact operation remains live. Stale, completed, reopened or foreign operations refuse with 409; current authority is rechecked after waiting for the product lock. Public receipts expose only revision and cancellation capability/intent, without private runtime identity.

Use the [product Windows cases](product/WINDOWS-FOCUSED-CASES.md) and the current checkpoint. Retain the independent runtime-install, provider-quality, responsiveness, native lifecycle and four-project acceptance gates. Setup variables and credential requirements are unchanged.

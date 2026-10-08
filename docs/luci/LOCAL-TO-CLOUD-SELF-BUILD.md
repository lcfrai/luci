# Use Luci to build its cloud capabilities

This is the next phase after the owner's local Windows acceptance and supplied
provider/cloud access. It is not a prerequisite for the local handoff. The owner
reviews the running local product first; source tests and controlled providers
do not stand in for that acceptance.

## Preserve one execution contract

Keep the local coordinator authoritative for reviewed task inputs, dependency
receipts, capacity reservations and integration decisions. A remote worker is
another execution location for the same owned task attempt, not a second task
system. Transport carries immutable context and the exact source base; it never
silently reuses a different provider account, model, credential generation or
project identity. Remote loss is an unconfirmed outcome until reconciled.

Remote results must include the task/attempt identity, base and candidate commit,
changed-file inventory, independent check evidence and observed resource use.
Review the exact candidate against the current target before integration. A
remote agent's claim, task checkbox or successful process exit does not release
dependencies or accept the resulting application. Keep credentials outside Git,
captured public context, model messages and downloaded review artifacts.

## Prepare the self-build in the app

1. Import the exact accepted Luci revision as a local project. Keep the checkout
   being improved separate from the installed Luci instance running the work.
2. Use Define and Design to review the cloud execution contract, ownership,
   disconnected states, resource/cost acknowledgement and recovery behavior.
   Record unresolved decisions before implementation.
3. Add the following tasks to the existing implementation plan, with dependency
   IDs, intended files and observable acceptance. Review the actual current
   source before narrowing file scopes; the paths below identify areas, not
   permission to replace unreviewed code.
4. Prepare a team on the explicitly selected provider connection. Start ready
   tasks through Luci; inspect actual sessions, captured task context and output.
   Pause new starts when an amendment changes the agreement. Review and integrate
   each candidate, then run checks on the combined source.
5. Build a separate candidate package, test reconnect/recovery against owned
   resources and compare it with the running stable installation. Keep rollback
   and source/data backups. Do not overwrite the working installation mid-run.

## Initial dependency-aware backlog

| ID | Work and intended areas | Depends on | Required evidence |
| --- | --- | --- | --- |
| cloud-contract | Define immutable worker transport and ownership/lease contract; `control-plane/internal/backend`, `docs/luci` | — | Versioned request/result schemas; foreign project/attempt, stale lease and duplicate dispatch refusals; no private configuration in public context. |
| cloud-worker | Implement remote allocation and reconciliation using existing backend boundaries; `control-plane/internal/backend`, `control-plane/internal/api` | cloud-contract | One child per reserved attempt across disconnect/restart; no automatic replay of unknown allocation; stop addresses the exact owned child. |
| cloud-context | Transfer exact reviewed context and source/candidate artifacts; `control-plane/internal/product`, `control-plane/internal/api` | cloud-contract | Source and every captured artifact match digests; cross-team reads refused; untrusted output cannot mutate host Git configuration or main source before review. |
| cloud-controls | Extend existing location, task progress and intervention controls; `desktop/src/components`, `desktop/src/api` | cloud-worker, cloud-context | Clear local/remote location and observed status; desktop/phone keyboard flow; pending local drafts survive disconnect; no false success after a lost response. |
| cloud-recovery | Implement resource observations, bounded cleanup and recovery; `control-plane/internal/backend`, `control-plane/internal/product` | cloud-worker, cloud-context | Owned-only inspection/stop; expired connection and partial upload recovery; retained output; clean cancellation without losing the source or data needed for retry. |
| cloud-acceptance | Run the representative real-agent app journey on owned remote resources, then redeploy an amendment | cloud-controls, cloud-recovery | Real application behavior, auth/roles/data durability, integration checks, measured time/usage, failure recovery and verified cleanup; actual provider quote and explicit cost acknowledgement before provisioning. |

The broad directory scopes deliberately serialize potentially overlapping work.
Refine them against the reviewed source when preparing the actual tasks. Do not
claim safe parallelism from a guessed non-overlap or increase capacity past the
host/provider limits. Merchant billing and public distribution have their own
later gates; cloud execution does not silently activate either.

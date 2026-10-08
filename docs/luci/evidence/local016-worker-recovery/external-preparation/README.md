# LOCAL-016: fresh acknowledged-worker recovery preparation

This external preparation exercises ordinary Stop for an acknowledged Docker task worker. It does not diagnose the retained lease in the original sustained failure. That original fixture has not been opened, changed, restarted, or reused by this work.

The scenario follows CLOUD-004's exact settlement and release rules. CLOUD-009's permanent-fence path is a separate protocol: a `launchGuard:1`, `backend-settled` lease is ineligible, and this scenario requires the installed recovery preview to refuse it with `recovery_launch_unconfirmed`. It creates no fence and never rewrites the ledger, registry, or task history.

## Concrete installed scenario

1. Create a fresh team and two approved synthetic projects through the installed API. Configure the shared machine scheduler using the owner's unprefixed endpoint, with one worker slot and unchanged headroom. Queue A and B while dispatch is paused.
2. Start A, run one real native Codex command through a scripted local Responses provider, and record its committed synthetic output. Hold the provider's final response so the native worker remains active. Pause dispatch before collecting the exact child, operation, container, original queue actor and 1 GiB/1 CPU settled lease. B must remain queued and uncharged.
3. Restart the owned controllers with their normal shutdown/startup path. Require the same worker container and PID, exact frozen context, recorded output, transcript prefixes and one command dispatch. This does not repeat the sustained fuzz workload or reproduce its helper failure.
4. Read the fence-ineligible preview. Using the same non-admin member, revoke project Edit and Execute separately, retaining Read, and require Stop to return 403 without changing Docker or the lease. Read B's known run without a B project grant and require 403. Submit stale revision, foreign child and foreign operation selections to task abandonment and require 409 with no effects. Session Stop itself has no revision field; the adapter does not invent one.
5. Exercise unavailable inspection with the existing proxy. Eight bounded, one-shot gates discard real 200 replies for the exact named worker Inspect while a Stop request runs. Background observations share this window. Require at least one and fewer than eight consumed gates, a 503 Stop response, no Docker mutation and the identical held lease. Cancel all unused gates before retry. Gate exhaustion fails the attempt. No absent or successful daemon response is fabricated.
6. Restore current Edit and Execute, issue ordinary `DELETE /v1/sessions/{childId}` without purge, and separately require exact immutable-container absence, removal of only A's private lease, and zero or omitted public queue counters. A 200 response alone is insufficient. Retain the actual operation status and exit reason; unknown outcomes are not converted to success or forcibly abandoned.
7. Resume dispatch. B must execute once, exit naturally with native status 0, reach completed queue state, and release its exact lease. A must not create, start, or dispatch again. Its output, Git state, context, native transcript prefixes and retained session remain available.

The installed adapter is API-only. App-control/browser acceptance remains separate. The scripted provider verifies lifecycle and isolation mechanics; it establishes no model reasoning, Windows, throughput, sustained-load, deployment, or original-failure diagnosis.

## Runtime selection and bounds

`installed-worker-recovery.mjs` imports without side effects. An explicit `--window-granted` flag is required before setup. Product and image pins are selected per invocation; `4df94ab9dad0e9fedd28a873bbadc56720e1402e` and image `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98` are preparation references, not unchangeable runtime requirements. Clean package provenance, binary/web/archive members, image identity and image source compatibility with the selected product commit are checked before allocation.

The fresh fixture uses two full-image Creates and two Starts, one worker at a time, 1 GiB memory, one CPU and 256 PIDs each. Every Create requires six GiB free on the verified Docker backing filesystem, and a one-second monitor retains one GiB running reserve. The scenario is bounded to ten minutes and cleanup phases to less than three minutes. No image pulls/builds, browsers, compilers, cloud resources, model calls, or original-fixture inputs are supported. Shared proxy configuration requires a positive fence-kind parser bound; the Create guard rejects all fence labels and the final receipt requires zero fences.

Example after the root grants a separate exclusive runtime window and selects a fresh package:

```sh
source /workspace/.luci-tools/activate.sh
node /workspace/.luci-tools/local016-worker-recovery/installed-worker-recovery.mjs \
  --window-granted \
  --expected-source '<exact selected product commit>' \
  --expected-image-id 'sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98' \
  --binary '<installed package>/luci' --web '<installed package>/web' \
  --provenance '<matching package provenance.json>' \
  --image-provenance '<verified image receipt.json>' \
  --report '<fresh evidence directory>/result.json' \
  --fixture-root '<fresh short private parent>' \
  --observer-instance-pointer '<fresh private observer pointer>'
```

The observer pointer contains only the new team's exact instance-file path. Use the existing resource observer and external timeout wrapper when admitting the actual run. The adapter preserves its fresh fixture on success and failure; it contains generated fixture credentials and must remain private. Report publication requires allowlisting. Failed-fixture Docker cleanup is exact-ID only after owned controllers are stopped and proxy requests are drained; unexpected leftover workers fail acceptance.

## Validation and source bindings

No installed scenario has run. The external bounded Node tests passed: 22 scenario contract tests, two real local Unix-socket transport tests for the actual adapter inspection function, and three inert-import/generated-command checks. Node syntax checking also passed. The transport fixture initially failed because its VM-created RegExp had another realm; passing the host RegExp constructor fixed that test setup. The unchanged failure log is retained beside the passing log. None of these controlled tests establishes product behavior.

`worker-recovery-scenario-test.log`, `inspect-window-test.log`, and `adapter-static-test.log` contain the raw results. Each batch used `timeout 30s node --test --test-concurrency=1`; no batches overlapped. `validation.json` records commands and limitations. `source-binding.json` binds the executed helper files. `reused-source-binding.json` binds verbatim repository runtime helpers, and the `*.inc` files preserve the adapter assembly inputs. `SHA256SUMS` covers this preparation except itself.

Sources consulted include `control-plane/internal/api/swarm_fenced_recovery.go`, `swarm_missing_recovery.go`, `sessions.go`, `execution_leases.go`, `execution_task_cleanup.go`, `swarm_worktree_observer.go`; `control-plane/internal/product/project_swarm.go`, `project_swarm_observation.go`, `project_swarm_scheduler.go`; and `control-plane/internal/management/team_authorization.go`, `team_proxy.go`, `conversations.go`. Existing repository acceptance helpers supply the proxy, startup/deadline cleanup, native terminal probe, context validation, and synthetic native command/provider mechanics. This copies no external source or dependency; the repository's existing MIT license applies.

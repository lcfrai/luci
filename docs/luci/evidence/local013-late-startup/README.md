# LOCAL-013 late startup and mutation fencing

The exact `6002b2ee` DAG startup sequence reproduces the reported race in a
controlled VM: after cancellation and completed controller cleanup, a delayed
proxy factory returns, then the old code writes configuration and calls spawn.
The corrected sequence performs neither action, closes the late handle once,
and preserves the failed proxy-drain phase. All process, file-write, provider and
Docker factory effects in this reproduction are injected; no real service,
browser or Docker workload ran.

| Same zero-postcancel-spawn assertion | Result |
| --- | --- |
| Exact original startup block plus original `startService` | Expected failure, exit 1; one spawn callback and one configuration-write callback after cancellation |
| Exact corrected startup block plus corrected `startService` | Pass, exit 0; zero spawn/configuration callbacks; one late proxy close |

The [extraction record](orchestration-extraction.json) preserves the verbatim
source slices, line ranges, slice hashes and whole-driver hashes. The original
driver hash is `e22fabe57952ad03c763bc188121b3886c64ea0941cf2ebdc795523f360e91f3`.
The [comparison receipt](regression-comparison.json), [original observation](original-regression.log),
[expected assertion failure](original-regression.stderr) and [corrected observation](corrected-regression.log)
remain separate. This is a source-level reproduction of a specific race; it is
not evidence that this race caused installed attempts 4 or 5 to fail.

## Correction and controlled checks

The actual driver invokes the shared
[`startAcceptanceServices` / `startAcceptanceBrowser` orchestration](../../../../scripts/acceptance/launch-recovery/acceptance-startup.mjs).
Acquisition is registered before factory invocation. Every startup await has an
activity fence before subsequent effects, and the actual detached-service spawn
has its own fence. A handle returned after cancellation is retained and closed
once through a bounded waiter. A pending or failed factory cannot count as an
absent proxy. Confirmed browser close settles acquired child handles; pending
child factories retain uncertainty and close their later results separately.
Exact-owned browser fallback remains reachable even when launch or close never
returns. Only completed bounded cleanup phases grant controller/drain authority;
a late callback cannot repair an expired phase.

The proxy's optional synchronous `beforeStart` callback runs after durable intent
and immediately before forwarding setup. It refuses new Start after workflow
failure/deadline. Exact Stop/Delete, original ownership/budget checks, and draining
of already-forwarded requests remain available. An asynchronous Start callback is
refused because awaiting it would introduce another continuation boundary.
An intent entry remains an intent, never proof of forwarding or ACK.

The DAG additionally rejects an executed helper directory outside the declared
helper checkout before importing runtime helpers. This prevents an external
staged driver from claiming a different clean checkout's revision. API return,
source seeding and terminal attach effects also recheck workflow activity.

The final controlled run passed **51 tests**: 15 startup/location cases, seven
lifecycle/privacy cases and 29 synthetic proxy cases. It includes deferred
provider/proxy/browser/context/page factories, both port awaits, a never-returning
factory, a never-returning close, failure during handle adoption, exact source
spawn/location boundaries, and deadline Start refusal with cleanup still usable.
Its [source-bound receipt](controlled-result.json) and [output](controlled-node.log)
are retained. After merging the separately prepared CLOUD-025 diagnostic delta,
the 15 affected startup/location tests passed again with the
[merged driver hashes](merged-result.json); these are repeated cases, not an
additional count. DAG syntax and exact patch whitespace/application checks passed.

The [patch manifest](manifest.json) lists eight exact base/result file hashes and
the patch digest. The [merge record](dag-merge.json) documents applying unique
startup edit blocks onto the CLOUD-025 driver; its unrelated import, helper list
and stale-review diagnostics were preserved. The earlier
[bounded-acceptance capsule](../local013-bounded-acceptance/README.md) is historical
and does not establish that the later race was fixed at that earlier source.

## Reproduction

From the repository root after applying this helper patch:

```sh
node --test --test-concurrency=1 scripts/acceptance/launch-recovery/acceptance-startup.test.mjs scripts/acceptance/launch-recovery/acceptance-lifecycle.test.mjs scripts/acceptance/launch-recovery/fenced-docker-proxy.test.mjs
node docs/luci/evidence/local013-late-startup/exact-source-regression.mjs original
node docs/luci/evidence/local013-late-startup/exact-source-regression.mjs corrected
```

The `original` command intentionally exits 1 on the zero-spawn assertion. The
`corrected` command must exit 0. The published reproduction changes only imports
from external staging paths to repository-relative helpers; that adapter is
recorded in [publication.json](publication.json).

The installed C013 workflow, sustained workload, actual Docker transport and
native Windows behavior remain unverified by these tests. Cleanup still retains
ambiguous resources; no ACK, drain, resource peak, or cause of a historical driver
disappearance is inferred from these controlled results.

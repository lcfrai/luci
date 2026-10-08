# CLOUD-013 installed same-project DAG witness — prepared only

This fixture has not passed installed acceptance. It uses one verified installed
Luci package, one private project, one Chromium browser, the actual Docker/native
Codex path, and an explicitly scripted local provider. It makes no model-quality,
parallel throughput, Windows or deployment claim.

The fixed maximum is four workers: A, initial B, explicitly authorized B retry,
and join, plus one separately authorized whole-build Docker check after all four
worker attempts finish and the three current tasks are integrated. The separate
check uses the same 1 GiB / 1 CPU / 256 PID limits; total Create/Start caps are five. A and B have no dependency edge but overlap `shared.txt`, so they are
not eligible for concurrent writes. One worker is allowed globally, with exactly
1 GiB RAM, 1 CPU and 256 PIDs, 6 GiB before each Create and a 1 GiB running reserve.
The complete scenario has a predeclared fifteen-minute deadline. No image pulls,
cloud allocation, artificial final-reply hold, success-path Stop or hidden task
retry is allowed.

The witness requires a corrected package containing CLOUD-011 task-context V2
and CLOUD-015 truthful integration notices, plus the current CLOUD-020 shared
check ledger/hard-limit and CLOUD-021 dependency-refusal corrections. Do not run it against known-unfixed
919 merely to repeat that defect.

The witness requires:

1. Browser authorization of A, B and join; direct negative reserve probes prove
   join has no workspace/session before reviewed prerequisite integration.
2. A and B finish naturally from the same original base. Both native commands
   verify their frozen inputs, readonly context, distinct outputs and clean Git
   commit, then write an attributed worker report into their exact private output
   mount. Their exit alone does not release join.
3. Read B's complete diff, then integrate A through the browser. Reusing B's old
   digest with the **current** run revision must fail without an integration
   checkout, source mutation or new process. This is separate from a Git conflict.
4. Renew B's review and integrate: the real add/add conflict must be retained in
   an isolated integration checkout, with main unchanged and join unallocated.
   The visible notice and screenshot must agree with that retained conflict.
5. Explicit browser retirement retains B's old source and conflict. A separate
   queue consent starts B2 from A's integrated base. It preserves alpha, adds beta,
   checks and commits; only reviewed B2 integration releases join.
6. Join receives the immutable V2 dependency packet with the accepted A and B2
   operations, commits, source-context hashes and captured worker reports; retired
   B1 is excluded. Worker-reported checks remain separate from recorded project
   checks. Join runs from that combined commit, checks alpha 17 and beta 25, emits the
   exact sum-42 artifact, exits naturally and integrates only after browser review.
   A final independent fixture check is bound to exact clean main HEAD.
7. In **Verify this build**, select the exact `AC-dag` criterion and explicitly
   consent to Docker execution of `node` with arguments `["verify.mjs","final"]`.
   The actual product check must record exit 0, exact expected output, unchanged
   clean source and the saved requirement/approval revisions. **Use these results
   for this build** joins that receipt explicitly. The final run is
   `awaiting-release`; preview, deployment and overall completion remain unverified.

Old B source, conflict files, all task contexts, receipts and Git history remain
in the private fixture even after success. The public report includes bounded
controlled-source conflict/diff evidence and hashes; the private fixture contains
test credentials and must not be committed or shared. Docker cleanup acts only on
exact acknowledged fixture IDs, after owned controllers stop and proxy requests
drain. Any ambiguous mutation prevents destructive cleanup. Positive runs need no
container stop/delete by the fixture because normal reaping removes every worker.
The check uses the product's protocol-1 Docker SDK path with `AutoRemove=false`.
Normal product cleanup removes its verified immutable ID after natural exit; the
proxy forwards that real operation and never fabricates its result.

The bundle validator accepts only the explicit legacy V1 eight-file schema or V2
nine-file schema (with `.luci/task-handoffs.json`); this new-task witness requires
V2. Unknown files, versions, scope identities, dependency commits and digests fail
closed. The raw exported-file hash is checked separately from the typed packet
digest. Legacy coverage is a pure validator test, not a live legacy execution claim.

The shared [`worker-create-guard.mjs`](../launch-recovery/worker-create-guard.mjs)
retains the exact legacy default V1 shape. Explicit `taskContextVersion: 2`
requires the exact child-bound writable report mount. Do not replace accepted
helper bytes during an active runtime witness.

The proxy, strict worker wire-shape guard and terminal observer are imported from
the current checkout and copied/hashed into each evidence directory. Use the
corrected CAP-prefixed/sorted guard; do not substitute a stale helper. Package
members and image/source lineage are checked against supplied immutable receipts.

The product binary/web remain bound to the clean package supplied by `--expected-source`. `--expected-helper-source` separately pins the clean acceptance-helper checkout and defaults to the product revision. A helper-only correction may use a distinct explicitly supplied helper revision; both revisions and all executed helper hashes are recorded. The sandbox source comparison still uses the product revision. Dirty or unexpected helper source is refused.

The context check requires the actual public `reviewedContextDelivery` receipt, the private `reviewedContext` reference, and the frozen manifest bytes to agree on version/digest. It also binds both session identities and their approved source/context fields to the exact task operation; a missing or legacy delivery does not pass.

A negative dependency probe records its bounded response error/code/message, requested and observed revisions, and source/allocation snapshots before checking the exact dependency refusal. Stale revisions, busy source locks and other HTTP 409 responses fail; the driver never automatically retries the POST. Attempt 1 on `c4400f11` stopped after two natural worker exits because its post-exit refusal wording did not match. That older helper did not retain the response body or requested revision, so its reason remains unavailable; it did not exercise conflict recovery, integration or join.

The driver also stops on a terminal `failed` queue entry belonging to the exact explicit queue authorization and previously unseen child. It saves bounded public queue, session and current operation diagnostics before failing, including `startFailure` when available. An older failed B1 cannot be selected after B2 receives its own authorization. Unknown outcomes keep the existing bounded observation behavior; the diagnostic never grants a retry or creates a worker. Attempt 2 on `3b8054e6` completed A but B1 was refused before sandbox allocation; its generic old record cannot establish the particular access/capacity condition. Both earlier failures remain separate.

## Separate SDK check admission and evidence limits

Check traffic remains disabled in the shared proxy unless explicitly enabled.
This driver selects `checkTrafficProtocol: 'sdk-ledger-v1'` with one check Create
and Start. Its name is `luci-check-<checkexec-allocation-id>`; all twelve protocol
labels bind engine, instance, allocation, check, command index, project, scope,
command/source digests and image. The guard independently matches those labels
to the persisted project-check row, authenticated actor, fully integrated clean
source and the exact `creating` lease in the fixture's private
`state/control-plane/execution/leases.json`. Scope is the canonical conversations
root digest. Missing capabilities, stale resource observations or ledger/identity
mismatches fail before Create.

The SDK request pins the verified image ID and exact `node verify.mjs final`
argv, 1 GiB memory and combined RAM+swap (no additional swap), one CPU, 256 PIDs,
read-only root, init, no added capabilities and no-new-privileges. Only the
fixture-owned primary checkout is writable; worker context/report authority,
extra mounts, ports and host namespaces are refused. `AutoRemove=false` leaves
cleanup observable. The check is never counted as a fifth agent. The pre-consent
capacity guard observes no live reservation; before Create it validates the
single exact check reservation and counts its own demand once. These acceptance
cross-checks accompany the product's shared admission path; controlled guard
checks alone do not establish installed production behavior.

Only a real, fully drained Create `201` with its full immutable ID establishes an
ACK. For that ID, SDK mode permits inspect, non-following stdout/stderr logs,
one Start and removal with `v=1` (optional `force=1` for the product's cancellation
path). Named check lifecycle requests, CLI attach/wait and upgrades are refused.
The legacy CLI protocol remains a separate default for other acceptance helpers;
its request format is not used here. Successful recorded checks must confirm
cleanup and release their reservation before results can be joined. Failed
fixture cleanup still requires stopped owned controllers, conclusively drained
proxy requests, a real ACK and matching immutable container ownership/config.
An uncertain mutation never authorizes name-based or ambiguous cleanup.

The wrapper explicitly enables `--include-checks` on the independently frozen
Linux observer. It selects the separately labelled check only after exact
instance/image/protocol/engine/allocation validation, and bounds running usage by
fresh immutable-ID statistics and current PID/start/cgroup identity. Each row
distinguishes worker/check and retains the configured combined RAM+swap cap.
The observer summary distinguishes measured usage, identity-only, and unobserved
checks; missing or stale observations produce explicit issues. A brief Node check
can finish between one-second samples, so enabling attribution does not establish
a measured check peak. Admission/hard-limit proof and sampled usage remain
separate, and cgroup memory is never added to process RSS. This preparation has
controlled fixtures only; the next actual workflow needs fresh observer evidence.

## Durable deadline and shutdown evidence (LOCAL-013)

The scenario deadline is 900 seconds. It rejects the workflow wait as well as
requesting owned controller shutdown; API/browser/Create mutation boundaries
refuse late continuations after failure. Failure screenshots have their own
three-second bound. Before any blocking cleanup, a durable
`result.json.incomplete.json` checkpoint records `passed: false`, stage, deadlines,
owned process identities and unresolved Docker drain. The bounded, mode-0600
journal records sanitized stage/signal/fatal locations, mutation-forward intent,
real Create ACKs and cleanup phases. Intent is not ACK; response-end is not proof
of client receipt. Headers, bodies, credentials and process environments are not
journaled, and journal failure refuses subsequent mutations.

Cleanup attempts are independent: controllers 30 seconds, proxy 125 seconds,
browser 12 seconds and provider 3 seconds, inside a 180-second overall cleanup
budget. Browser close gets five seconds, then the same exact-profile/PID-start
fallback runs even if Playwright launch never returned a handle. Unsettled
transport remains explicitly uncertain. The final failure receipt is attempted
before ending only this driver's lifetime; the cleanup watchdog can leave an
incomplete failure record when cleanup cannot settle. It does not force removal
of ambiguous Docker resources. Tests use never-resolving promises and synthetic
transport; they do not establish an infinite-hang defect in the proxy's existing
bounded gates.

The wrapper's total driver budget is 1200 seconds (120 seconds preparation
allowance plus the scenario/cleanup budgets). After that total expires it allows
only five seconds for SIGTERM, then at most five seconds to reap its own forced
exit; it does not add another three-minute cleanup window. Observer settlement
has separate 30-second wait, five-second terminate and five-second reap bounds.
The wrapper records its own PID/start/group/session, sent signals and child exit
results in a durable lifecycle journal. These safeguards do not make SIGKILL or
an external supervisor loss observable inside a killed process.

A later LOCAL-013 audit found a separate late-startup race: a pending proxy
factory could resolve after cancellation and completed controller cleanup, then
spawn Luci. The corrected driver uses tracked provider/proxy/browser factories,
checks workflow activity after each startup await and at the actual spawn,
settles late handles within a bound, and never treats a pending factory as absent.
Only completed cleanup phases grant stopped/drained authority. An optional
synchronous proxy Start guard is checked immediately before forwarding; exact
Stop/Delete and existing request draining remain available. The executed driver
must also reside inside the declared helper checkout. See the
[exact-source negative and corrected startup regression](../../../docs/luci/evidence/local013-late-startup/README.md).
This correction does not establish the cause of any interrupted installed run.

Attempt 4 on `4e7d5fe4` retained the four worker exits and all current task
integrations, but the final legacy check recorded Create EOF/exit 125 and no
complete driver/observer receipt. Its disappearance remains unclassified. The
source-confirmed LOCAL-013 deadline/cleanup gap and daemon-normalized MemorySwap
are not established causes of that run. Earlier frozen helpers and failed
artifacts retain their original scope; the new SDK/observer path requires a fresh
installed witness. See the [controlled lifecycle and observer evidence](../../../docs/luci/evidence/local013-bounded-acceptance/README.md).

## Preparation checks

From this directory:

```sh
node --test --test-concurrency=1 ../task-context-delivery.test.mjs helper.test.mjs context-contract.test.mjs dependency-probe.test.mjs queued-failure.test.mjs project-check-guard.test.mjs
python3 -m unittest run_with_observer_test.py
node --check dag.mjs
node --test --test-concurrency=1 ../launch-recovery/worker-create-guard.test.mjs ../launch-recovery/fenced-docker-proxy.test.mjs ../launch-recovery/acceptance-lifecycle.test.mjs ../launch-recovery/acceptance-startup.test.mjs
python3 ../four-project-queue/sample_linux_owned_test.py
```

These checks start no Docker, Go compiler, Luci or provider. Oracle tests run Node
only in test-created scratch directories. They do not establish installed success.

## Installed execution (dedicated Linux acceptance daemon only)

```sh
LUCI_ACCEPTANCE_REPO=/ABS/LUCI-CHECKOUT python3 run-with-observer.py \
  --observation-dir /ABS/FRESH-observer --fixture-base /SHORT/ROOMY/LOCAL/DIRECTORY -- \
  --binary /ABS/VERIFIED-PACKAGE/luci --web /ABS/VERIFIED-PACKAGE/web \
  --provenance /ABS/package-provenance.json --expected-source EXACT40HEX \
  --image VERIFIED_TAG --expected-image-id sha256:EXACT64HEX \
  --image-provenance /ABS/runtime-receipt.json --report /ABS/FRESH/result.json
```

Use an exclusive fixture-owned Linux Docker daemon, never the owner’s personal/shared Docker Desktop environment. Coordinate this window with other acceptance work. The zero-container/volume entry requirement is not an instruction to remove existing resources.

The observer is the independently frozen Linux sampler. The wrapper has one
1200-second driver budget: a 120-second preparation allowance plus 900 seconds for
the scenario and 180 seconds for owned cleanup. Observation permits 15 further
seconds of settlement.
Observer-self-cost reporting remains separate; missing
or partial coverage remains explicit. Preserve every failed attempt and its source
hashes. Do not run during another performance/Docker/Go-compiler window.

Current UI binding: secondary checkpoint/retirement actions first open the exact task card’s `More task actions` disclosure. These staged drivers target the matching corrected package; the original accepted V1 benchmark helpers are unchanged.

## Windows manual acceptance (separate, not established here)

Do not run this Linux-only driver or any of its cleanup on a normal shared Docker Desktop. In the installed Windows app, create a new disposable project and use a one-worker ceiling with an already configured provider. Settings must report usable local storage and resource capacity. Keep existing applications and Docker resources outside the fixture.

1. Approve A and B with no dependency edge but an overlapping `shared.txt` write scope, plus a join depending on both. Run A, let it stop, then run B from the same original base before integrating A. Check that no concurrent writer was admitted for the overlap, and that join stays waiting after process exit.
2. Review B, integrate A, and verify the old B review cannot be reused. Refresh B’s review and attempt the deliberate add/add conflict. Inspect the retained conflict and unchanged primary; the notice must not say integration succeeded or dependencies were released.
3. Use **More task actions → Retain changes and retry** for the conflicted attempt. Check its old source/conflict remain available. Explicitly authorize a fresh B attempt on A's accepted base; review and integrate its result. Do not silently restart or alter the original attempt.
4. Only after A and the current B are integrated, authorize join. Inspect its frozen dependency context for the current accepted attempts and separately attributed reports, with the retired B excluded. Review and integrate join, then run the real project checks through the app. Preserve IDs, exact revisions, screenshots, failure details and the final source/check evidence without exposing credentials.

This manual checklist is not equivalent to the Linux scripted witness, its exact output oracles or independent resource observer. Windows execution and live-model quality remain separate gates; failures retain their attempts and require an explicit next decision. No ledger repair, global Docker cleanup or hidden additional worker is authorized.

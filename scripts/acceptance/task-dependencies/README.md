# CLOUD-013 installed same-project DAG witness — prepared only

This fixture has not passed installed acceptance. It uses one verified installed
Luci package, one private project, one Chromium browser, the actual Docker/native
Codex path, and an explicitly scripted local provider. It makes no model-quality,
parallel throughput, Windows or deployment claim.

The fixed maximum is four workers: A, initial B, explicitly authorized B retry,
and join. A and B have no dependency edge but overlap `shared.txt`, so they are
not eligible for concurrent writes. One worker is allowed globally, with exactly
1 GiB RAM, 1 CPU and 256 PIDs, 6 GiB before each Create and a 1 GiB running reserve.
The complete scenario has a predeclared fifteen-minute deadline. No image pulls,
cloud allocation, artificial final-reply hold, success-path Stop or hidden task
retry is allowed.

The witness requires a corrected package containing CLOUD-011 task-context V2
and CLOUD-015 truthful integration notices; do not run it against known-unfixed
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
   A final independent fixture check is bound to exact clean main HEAD. This is
   not fabricated as a Luci product check receipt: the run remains awaiting checks.

Old B source, conflict files, all task contexts, receipts and Git history remain
in the private fixture even after success. The public report includes bounded
controlled-source conflict/diff evidence and hashes; the private fixture contains
test credentials and must not be committed or shared. Docker cleanup acts only on
exact acknowledged fixture IDs, after owned controllers stop and proxy requests
drain. Any ambiguous mutation prevents destructive cleanup. Positive runs need no
container stop/delete by the fixture because normal reaping removes every worker.

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

## Preparation checks

From this directory:

```sh
node --test --test-concurrency=1 ../task-context-delivery.test.mjs helper.test.mjs context-contract.test.mjs
python3 -m unittest run_with_observer_test.py
node --check dag.mjs
node --test --test-concurrency=1 ../launch-recovery/worker-create-guard.test.mjs
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

The observer is the frozen independent Linux sampler. Its eighteen-minute window
includes bounded owned cleanup and separate observer-self-cost reporting; missing
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

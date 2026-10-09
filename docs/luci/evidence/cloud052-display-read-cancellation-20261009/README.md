# CLOUD-052 — canceled display reads stop obsolete work

This is controlled Linux source evidence for a backend cancellation correction. It is not installed, Windows, browser, live-provider or responsiveness acceptance. CLOUD-032 latency remains independently open; no measured gain is claimed.

The original product is `67a5b7c75ca6354d43085f5bb81ec53ee4291f34`. Documentation-only commits advanced HEAD during the work; selected production bytes and exact counterexamples are bound separately. The manifests here cover selected changed production/test inputs, not every Go compiler, workspace, module or transitive source input.

## Correction

The Home overview now carries the request context through a single globally bounded cache-fill gate and between bounded context/run/question reads. A canceled waiter leaves the gate and releases its shared source admission without waiting for another project's fill. An already-started bounded operation is allowed to finish; cancellation then prevents subsequent reads, cache publication and response work. The 64-entry cache, one-second TTL, stamp validation, compact per-project summaries, private authority/path checks, indexed run limits and fresh queue snapshot semantics remain intact.

The scheduler display GET now passes its request context into the read-only Resources callback and existing reservation observation. That reader retains its 100 ms timeout, exact shared ledger validation and unknown-on-invalid behavior. The cached resource snapshot supplier is skipped after cancellation; it is not a new host probe. No change was made to worker/admission lifecycle contexts, capacity limits, policy mutation ownership or reviewed recovery.

The existing store/scheduler mutexes and synchronous bounded file/JSON reads are not forcibly preempted. This does not claim cancellation of every filesystem syscall or an atomic transaction with transport teardown.

## Original failures and matching correction

The actual registered overview route reproduces two failures: two canceled requests remain stuck behind an unrelated fill, later populate cache and serialize obsolete responses; cancellation during the run-store stage also publishes. The final exact tests use a generous two-second completion bound while the fixture deliberately holds another fill indefinitely. They fail against four unchanged original overview production files supplied by Go overlay, then pass against corrected production. The overlay adapts one unrelated old scheduler callback fixture solely to compile against the new context-accepting Resources signature.

Earlier 200 ms-bound versions and all original failures are retained. The final two tests overlap the affected overview race batch's 18 top-level + two subtest passes; counts are not additive. That affected batch used the earlier test wait bound but the same final production bytes. The initial non-race focused pass predates only the final cache-eviction checkpoint move and is not presented as the final broad check. The first shell omitted pinned tool activation and ran no Go tests; its failure is retained separately.

Scheduler details and final exact matching source/old-overlay results are recorded under `scheduler/`. The route tests preserve a real held execution-ledger lock and charged allocation, verify request-context propagation/no obsolete response and subsequent authorized readback. Policy-response cancellation must leave the already-committed revision/policy intact; live reconciliation succeeds and stale replay remains rejected. Early narrow timing assertions and their receipts are preserved as superseded test versions, not app-performance gates.

## Reproduction

Use the repository's pinned Go toolchain/workspace and run from `control-plane` with `GOMAXPROCS=2` and `-p 1`:

```text
go test -race -p 1 -count=1 -timeout=5m -run '^(TestProjectOverview|TestOverview)' ./internal/product
```

The exact final scheduler commands and source-overlay maps are in its summary. Overlay maps contain the original execution workspace paths; relocate keys to your checkout and values to the bound original-source copies when reproducing elsewhere. These helpers are source fixture tests; they do not require or authorize running an installed app against existing user state or Docker workloads.

All receipts are preserved byte-for-byte unless an explicit publication record says otherwise. `manifest.json` and `SHA256SUMS` bind published members. A final aggregate full-suite, installed package or native Windows pass is not asserted by this capsule.

Final scoped results (overlapping repetitions are not added):

| Batch | Result | Exact scope |
| --- | --- | --- |
| Overview final same-test old/current pair | Old 2 failures; current 2 passes, race | Generous2s cancellation bound, unchanged paired regression |
| Overview affected race | 18 top-level +2 subtests passed | Final production; earlier200ms test wait bound |
| Scheduler final same-test old/current pair | Old 2 top-level +2 subtests fail; current all4 pass, race | Synchronous exact request-context attachment; paired source identical |
| Scheduler affected race | 9 top-level +16 subtests passed | Earlier test timing/handshake version; cancellation cases overlap final pair |
| Scheduler policy race | 2 tests passed | Settled mutation retained, stale replay rejected; shared policy read controls |

The current scheduler test differs from the matched final pair only by the explicit `scheduler/comment-only.patch` correcting a coverage comment. Its executable assertions are unchanged. `final-repository-bindings.json` binds the current selected source bytes including that comment and the final overview2s regression. The broader compiler closure remains a separate parent verification obligation.

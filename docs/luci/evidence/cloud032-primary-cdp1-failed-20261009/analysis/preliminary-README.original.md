# Primary CDP diagnostic 1 — incomplete capture, not acceptance

This diagnostic ran with packaged product **2da1a821b8456c20a858fb86ef87649ea917d9de**, clean checkout guard **e2f426bb50267a7cceac1f53eb1e00a0a4cc278e**, and separately frozen runtime helper copies originally accepted at **2da**. These are distinct identities. The diagnostic candidate changes measurement code and is not the unmodified acceptance collector or a production-source change.

The original driver failed with **“Primary profiling incomplete; preserve original actions and diagnostics.”** It completed all **400 primary actions** (100 switches, 200 inputs, 100 draft resets), then failed while collecting the profile: the timeline stream exceeded its declared **32 MiB** retained-byte cap, and Chrome reported **traceDataLoss=true**. The retained timeline is incomplete. CPU output was retained, but its existence and successful protocol detach do not establish complete diagnostics or causal attribution.

Recorded instrumented p95 switch/input values are **406.196951 / 87.063964 ms**. Switch p99 was **509.931130 ms**; reset p95 was **384.949992 ms**. These values include profiling and marker overhead and cannot replace the original acceptance run, establish a speedup, or be corrected by subtracting measured costs. Every original row and numeric diagnostic span is retained in `result.projection.json.gz`; no rows are filtered. `quantiles.recomputed.json` exactly matches all three original nearest-rank groups. `recompute-quantiles.py` can check those public rows offline. No additional 32-action followup was run (`trace-after=no`).

## Original capture limits and failure

The original limits remain explicit: 150-second recording allowance, 5 ms CPU sampling interval, 16 MiB `recordUntilFull` trace buffer, 32 MiB retained timeline, 12 MiB serialized CPU profile, 256 read chunks of at most 256 KiB, 9-second internal finish and 10-second outer finish, original 210-second workflow and 450-second wrapper. The capture timer did not expire. Recorded trace buffer peak was 0.992548; the profile finished with zero pending protocol commands and a detached transport. Those observations do not cancel the byte-cap failure or the data-loss report.

The wrapper records exit **1**, no external-monitor timeout, and 156.193237 seconds elapsed (02:27:46–02:30:22 UTC). All five startup handles closed, original cleanup recorded no errors, and the idle fixture admitted zero Create/Start, tasks or provider calls. The failed private fixture was retained. The original incomplete checkpoint, final failure projection, stdout/stderr and journal remain separate. No attempt was repaired or relabeled successful.

## Private artifact bindings and limited analysis

Raw browser timeline and CPU data are intentionally excluded because arbitrary trace arguments and source URLs may contain private information. The original profiler recorded:

- Partial timeline: 33,554,432 bytes, SHA-256 `87881413924fa94f254a8c56d78827ba56da3bdb3eaa6f29f7bbb9be6b8d08b0`.
- CPU profile: 1,097,185 bytes, SHA-256 `265646ccaf4ef0742369958494d4c6656369283e8e5988eb331e0a9d0def3fe3`.

Publication checked byte lengths and metadata consistency but did not reread or rehash these large private payloads during the separate sustained run. Their hashes remain original-profiler observations. `profiling-status.original.json` is the exact retained metadata.

`analysis/failed-prefix-analysis.original.json` was computed separately before that sustained window. It describes 140,324 complete events from the truncated trace prefix and 453 frame markers covering primary ordinals 1–151. It does **not** establish complete coverage for all 400 actions. CPU special-frame weights are whole-capture observations; `(program)` means unknown/native work and must not be assigned to application CPU or a React component. Marker/category summaries and incomplete profile data cannot establish causality or explain the acceptance tail conclusively. The original calculation was an inline Python tool command; it has no original source-file execution hash. `analysis/reproduce-failed-prefix.py` is a subsequently reconstructed helper that refuses nonmatching raw inputs. It was not executed during publication or the separate sustained run, and its source hash is not attributed to the original inline execution. Future output equality remains unverified.

## Executed sources, tooling and preparation history

`sources/accepted/` contains exact compressed accepted candidate files, verified against the frozen candidate. All 17 local-source hashes and seven frozen runtime helper copies are identified in the binding. The external dependency map also includes 73 installed Playwright/playwright-core tooling hashes; they are runtime tooling, not product source, and were not rehashed during publication. The task command remains the frozen pre-CLOUD-041 version. The changed diagnostic driver adds CDP CPU/timeline capture and primary markers; Playwright tracing was disabled. Instrumented measurements stay separate from unchanged acceptance thresholds.

The exact launcher, patch and preparation sources are retained under `preparation/`. Their README and preparation receipts were written before execution and still say “not executed”; that is historical preparation text, superseded for run status by this run's immutable invocation/wrapper/final receipt. Earlier discarded candidate directories are not treated as executed source. Eleven fake-transport profiler checks and six pure socket-path checks are historical helper-preparation evidence, not an installed or performance pass. No tests were rerun for publication.

## Independent closure scope

The read-only closure at 02:34:53 UTC independently observes the recorded **driver PID/start identity absent** and the recorded **controller PID absent**. Browser PID/start identities were not captured, so it establishes neither browser-zombie absence nor whole-machine process absence. Three other process environments were unreadable. Docker containers/volumes and parsed execution ledgers were empty. The receipt records 238 private fixture files and original inputs unchanged, with no signal or mutation. The allowlisted closure projection omits private inventory rows and is bound to the original private receipt. The driver's reported browser/app cleanup remains separate evidence.

## Publication and reproduction

`publication.json` binds originals, projections, exact source copies and selected allowlisted fields. Actual fixture-parent/retained-fixture locations are replaced by explicit role placeholders in invocation, logs and source binding representations; raw result, trace, CPU and fixture inventories stay private. The 400-row projection preserves numerical fields and diagnostics while omitting project IDs and unrestricted browser/network payloads. Known-root checks are not a universal secret-scan claim.

Reproduction requires the recorded matching package/image, frozen candidate/tooling, original limits, a fresh private output/state directory and the exclusive disposable Docker contract. Do not reuse the retained failed fixture or run this exclusive harness on personal shared workloads. Preparation is not permission to alter a user's machine. This capsule was assembled outside the checkout with lightweight metadata/source/row processing only; no browser, application, worker, compiler, test suite or new CPU/trace analysis ran during publication.

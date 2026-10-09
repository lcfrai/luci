# Separate primary-action diagnostics

This publication contains two distinct instrumented runs. Their results are not combined into a performance or acceptance pass. All 400 original action rows and numeric diagnostics from each run are retained in its own child capsule. Both used packaged product `2da1a821b8456c20a858fb86ef87649ea917d9de`, with separate exact helper/candidate identities and no worker, provider or external-model execution.

| Run | Original outcome | Instrumented switch/input p95 | CPU timing validity |
| --- | --- | --- | --- |
| `primary-markers-1` | Driver exit 0; selected marker capture complete, no trace data loss | 349.842204 / 78.436254 ms | 1,288 negative deltas; all time-weighted attribution withheld |
| `public-api-spans-1` | Driver exit 1; public logger coverage failed at its 100-problem cap; CDP marker capture separately complete | 363.614553 / 64.931624 ms | 1,297 negative deltas; all time-weighted attribution withheld |

No action, duration or threshold was removed and no observer cost was subtracted. These runs added instrumentation and do not replace the unchanged 300 ms switch / 100 ms input p95 acceptance requirements, prove a causal improvement, or establish four-project sustained performance. Their switch p95 values also exceed the reference. Neither run measures physical input-to-display latency.

The marker run's preliminary CPU-weighted analysis is preserved as explicitly superseded, alongside its corrected source and projection. Equal sample counts and nearly matching signed totals do not make negative timing deltas valid duration weights. The logger run's corrected analysis also refuses weighted CPU, frame/bundle and per-action attribution. Negative delta causes remain unestablished; no deltas were repaired or discarded.

The failed logger run retains its 137 allowlisted callbacks, all 400 action records and 100 unrecognized-message problem codes. Original unrecognized message names were not retained. Later static-browser smoke demonstrates a possible background-callback mechanism, not the exact cause of those historical 100 messages, and is intentionally excluded from these original-run capsules.

Each child README provides the exact product, clean checkout guard, frozen candidate and runtime/tooling bindings, original bounds, reproduction instructions, raw artifact hashes and its own independent closure scope. Both captured only the recorded driver identity and controller PID independently; neither independently captured browser identities. Three unrelated process environments were unreadable. Empty Docker inventory is not a whole-machine or zero-zombie observation. The failed logger fixture remains retained; the marker fixture was removed and its exact parent observed empty.

Raw report/profile payloads, unknown message strings, private parameters and inventories are excluded. Public projections retain every measured action, safe numerical timing evidence and the declared exact source copies. Known-root scans are bounded rather than universal privacy guarantees. The child manifests preserve exact original hashes and distinguish projected metadata from original bytes.

The earlier failed CDP1 capsule is unchanged at `/workspace/.luci-tools/publish-primary-cdp1/capsule`: 60 files, publication SHA-256 `453f2067d3c3f0108a992a0812686bbfe8bf18b011d9266fd29e2f8188756c33`. Its 32 MiB cap failure/data loss, original preliminary analysis and later CPU rejection remain separate evidence. Sustained attempt 5 is also untouched. Neither is silently replaced by these diagnostics.

This combined folder is a byte-exact publication of two already frozen child capsules. It was assembled outside the repository. No application/browser, compiler, test suite, model or new performance workload was launched for publication; only bounded read-only analysis of retained files and checksum verification ran. All local product acceptance gates remain open at their previously recorded scopes.

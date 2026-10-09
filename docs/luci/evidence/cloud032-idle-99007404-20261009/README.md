# CLOUD-032: installed idle diagnostic at 99007404

**Responsiveness gate remains failed.** Product and helper checkout were `99007404a5ddd8592dee376ea00d99c6180d26fe`. The fresh installed diagnostic completed in 173.278 seconds with exit 0 and all 400 primary actions successful. Completion does not mean its unchanged timing targets passed.

| Primary cohort | Count | Median | p95 | p99 | Target |
| --- | ---: | ---: | ---: | ---: | --- |
| Project switch | 100 | 242.779 ms | **561.359 ms** | 756.904 ms | p95 ≤ 300 ms; failed |
| Input | 200 | 49.222 ms | **81.122 ms** | 313.269 ms | p95 ≤ 100 ms; passed |
| Draft reset | 100 | 283.084 ms | 617.274 ms | 701.327 ms | Reported separately; no declared p95 target |

The original switching/input p99 requirement remains strictly below 1000 ms. The separately instrumented eight-round trace follow-up contains another 32 actions and does not replace any primary rows. Its switch p95 is 272.269 ms and input p95 247.796 ms; these different, smaller cohorts cannot upgrade the primary result.

## What the captured stages support

Eighteen primary switches exceed 300 ms. Their slow stages vary. The 824.996 ms switch includes 540.775 ms in the `All projects` command; a 756.904 ms switch includes 533.336 ms in `Open project`; other slow switches include 328.498 and 343.368 ms in expected-context verification. Across all 100 switches the individual stage p95 values are 122.359 ms, 128.168 ms and 98.925 ms respectively; these p95 values must not be added.

`analysis.json` preserves all 18 slow cases with their original Node subspans, browser pointer/driver-expected-DOM/frame references and temporally overlapping resource categories. Browser intervals use only the same document's clock. For example, some slow cases have more than 300 ms from the last browser pointer to the driver-triggered expected-DOM witness; others spend most time in the earlier automation command. Expected-DOM is sampled by driver evaluation after a locator wait: this is **not autonomous DOM-ready, React-commit or physical-display timing**, and can include driver/transport delays. Overlapping browser resource rows do not identify an exact project's request or a causal backend function.

The separate follow-up trace has 40 click calls. Each records exactly one click attempt, one visible/enabled/stable witness, and one click completion. There is no retry evidence in those captured calls. That trace does not cover the original 18 slow primary switches and cannot prove why they were slow. No CPU profile weighting, instrumentation subtraction, filtered-out actions or latency improvement is claimed.

The controlled CLOUD-047 layout correction remains separately verified. This installed response-time result does not establish that the earlier layout movement caused the failed timing tails, or that the layout correction improved timing.

## Binding, reproduction and scope

- `timings.projection.json.gz` retains all **432 original action rows** with their numeric Node/browser diagnostics, omitting only each interaction's project ID. All **3052 original browser latency rows** are retained with document clock references. The original private result's SHA256 and byte length are bound; this is not a reconstruction of every private result field.
- `analysis.json`, `analyze.py` and `trace.projection.json` retain the numerical derivation and separately bounded trace observations. The trace projection omits selectors, parameter values and body content; original archive bytes remain private and hash-bound.
- `source-bindings.json` binds original result/invocation/wrapper/provenance/journal/trace bytes, **17 accepted helper inputs**, and 17 current frozen template inputs. Gzip helper members decode to their exact accepted bytes. Original collector/action semantics and 100/200/100 counts are unchanged.
- The external extraction and launcher are retained under `sources/`. Metadata path substitutions are labelled, preserve original hashes, and do not imply direct local verification of privately retained archive/image bytes. Full-package verification is the retained driver's observation.
- The exact run used zero task results, zero provider calls, zero external models, hard zero-Create/Start guards and fresh local state. Docker readiness/inventory reads are part of setup. This is neither a live-provider run nor sustained four-project execution, Windows acceptance, or a RAM/CPU efficiency result. No independent Linux resource sampler ran in this diagnostic.
- `recompute.py` verifies every numerical cohort and each slow-case subspan using only published projection data. Running it does not launch Luci, Docker, a browser or a provider.

The source checkout was clean at launch. During the later independent closure check root had already prepared documentation/publication changes; the captured status rows retain that distinction. The selected helper bytes remained identical to the accepted snapshot; no new whole-checkout cleanliness claim is made for publication time.

## Closure

The original driver reports all five startup handles closed, controller/proxy settlement, empty cleanup errors and removal of its disposable fixture. Independent read-only observation confirms the **exact recorded driver and controller PID/start identities are absent**, empty Docker container/volume inventory, unchanged engine and image identity, and unchanged original artifact hashes.

There was no independent browser PID/start census or Linux observer for this idle run. Browser/zombie absence and whole-machine closure are **not established** by empty Docker inventory or absent controller. Existing unrelated or unassigned zombies are not removed or attributed. The initial read-only closure helper's image-object argument mistake is retained and explained; its corrected run performed only reads. No old failed fixture or execution ledger was edited.

# CLOUD-052 scheduler display cancellation

This is selected Linux source/controlled HTTP evidence, not a Windows, socket-disconnect, installed Docker, live-model or application-latency result. The actual Product/API registrar, shared reservation reader and filesystem lock are used; cached host capacity and execution backends are controlled. No host sampler, task worker, browser, provider or Docker process is launched.

The original scheduler display callback discarded the HTTP context. A pre-cancelled GET still called the cached snapshot supplier, waited for the independent reservation timeout and serialized capacity. Cancellation inside the cached supplier likewise continued to the shared reservation read. The initial exact original regression and all historical test revisions/results are retained.

The correction changes only `SwarmSchedulerRuntime.Resources` to accept the request context, passes it from the scheduler GET, checks cancellation around the scheduler read and cached snapshot, and passes it into the existing bounded reservation observation. The existing 100 ms child observation budget and unknown-on-invalid semantics are unchanged. Admission, mutation, execution and sampling contexts are unchanged. The scheduler's short shared mutex is not made preemptible; cancellation is checked immediately after acquiring it. Synchronous filesystem reads are not claimed to be interruptible.

Final regression scope:

- A pre-cancelled registered scheduler GET does not call the cached snapshot supplier or serialize a response.
- Cancellation inside the cached supplier stops subsequent display work. The exact reservation lock remains held. Retained names/modes/timestamps/bytes do not change, and a subsequent live read returns the original one allocation / 1024 MiB / 1000 mCPU charge.
- A test context cancels synchronously when its Done channel is attached at reservation-observation entry, after the cached supplier arms it. This proves exact request-context propagation without requiring the test goroutine to be scheduled within the production 100 ms budget. It does not claim an already-blocked real lock waiter; existing reservation tests cover a real held-lock timeout and a pre-cancelled read; they do not establish asynchronous cancellation after a reader is already blocked.
- A separate product test proves a committed policy mutation remains committed if cancellation suppresses its response. Fresh GET recovers its exact revision/policy, and replay with the consumed revision remains a conflict.
- Selected existing reservation authority, unavailable/unsafe/read-only, shared peer demand and storage-projection checks remain covered.

`before/` and `after/` retain the first counterexample and affected race suite, whose initial tests used a 50 ms timing assertion. `revised-before/` and `revised-after/` retain its removal with a 2-second settlement wait, before the context handshake was made synchronous. These are historical overlapping runs. `final-before/` and `final-after/` contain the final byte-identical regression source and matching old/current focused results. Elapsed observations are retained but are not acceptance thresholds or UI performance evidence.

The final old run uses a Go overlay with exactly the frozen original scheduler HTTP/type/API sources and the original mechanical reservation test call. Other dependency sources come from the current working tree. This is a selected-source counterexample, not an assertion that the entire checkout was reset to the original revision. The final corrected run uses the current production sources under the race detector. Parent/root owns the final complete compiler-source binding and integrated verification.

All commands use retained Go 1.26.3, GOMAXPROCS=2 and `-p 1`. Commands, raw JSONL/stderr, exit codes, frozen source and bounded summaries accompany this note. No test failures or earlier receipts were overwritten.

The matched test source contains an overbroad comment about the pre-existing helper cancellation coverage. The precise observed scope is the paragraph above. Its test behavior and retained original bytes are unchanged; `comment-only.patch` corrects it after the matched pair; `final-current/` and `final-source-bindings.json` bind that exact sole documentation delta. Executable assertions are unchanged; no additional runtime result is claimed.

Final matched focused result: old selected sources fail 2 top-level tests and 2 subtests; corrected source passes all 2 top-level tests and 2 subtests under race. The earlier affected API race run passed 9 top-level tests plus 16 subtests; its repeated cancellation cases overlap the final pair. The separate product race run passed 2 tests. All failed and intermediate runs remain retained.

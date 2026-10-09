# d20 autonomous-readiness attempt 2: retained analysis

Product `d20ebd292f3f1de04d6726bab0998c210987bcb5`, clean helper `edd0cb121ff6847f681ab6bf41509a497f043d41`, frozen external observer v2. Wrapper exit 0 in 178.91 seconds. All 432 original actions succeeded. The primary 100 project switches have p95 **522.381 ms, failing 300 ms**; the 200 primary inputs have p95 **110.087 ms, failing 100 ms**. No time is subtracted. This diagnostic subset is not full sustained or release acceptance, and standalone differences from earlier runs are not a demonstrated improvement.

The v2 stage gate restores coverage missing in frozen v1: **100 Home ready and 100 Define ready**, plus one closed row, with 100 complete primary switch joins. There are zero censored, missing, ambiguous, overflow or late-callback stages; all generations are closed. All 201 observer rows, 102 windows, 432 original action rows and their original indices are retained. The separate 32-action trace cohort has no autonomous observer by design.

## Supported stage findings

Define's exact selected-project, saved-value, connected and native-enabled structural predicate was observed before the driver's expected-DOM mark in all 100 primary switches. The same-browser-clock signed lag has p50 31.3 ms, p95 62.1 ms, p99 220.6 ms and max 341.5 ms. This is an observed predicate, not first possible readiness, continuous readiness, actionability, paint or pure transport overhead.

| Original index | Full switch | Specific retained observation |
| --- | ---: | --- |
| 32 | 654.285 ms | Define predicate observed 341.5 ms before driver DOM mark; Node verification interval 393.903 ms. |
| 232 | 443.496 ms | Define predicate observed 220.6 ms before driver DOM mark; Node verification interval 239.757 ms. |
| 156 | 879.571 ms | Home predicate observed 579.7 ms before trusted Open click; the destination context panel is not mounted during this interval. |
| 104 | 651.945 ms | Open to observed Define 386.8 ms, but a 222.7 ms observer sampling gap prevents establishing when readiness first became true. |

Home observed-to-Open has p50 83.8 ms and p95 151.7 ms. Define's sample gaps exceed 50 ms in 3 stages and 100 ms in 2; Home gaps exceed 50 ms in 2 stages. These descriptive cutoffs are not acceptance criteria. The aggregate grouping by project and ten-round block is in `actual/analysis.json`; all original rows remain available, including resets and trace actions.

Node subspan p95 values are All-projects command 124.476 ms, heading wait 25.845 ms, Open command 275.457 ms, verification 103.865 ms, and two-frame command 42.605 ms. They include transport, scheduling, app work and instrumentation; percentiles cannot be summed. Neither the large verification gaps nor the large pre-click gaps justify a product optimization without narrower observations.

`stage-markers.json` retains a bounded allowlisted projection for indices 32, 104, 156 and 232. In index 32 the specification and orchestrator route-class ResourceTiming responses end before the late driver DOM mark. Route classes cannot independently identify the exact project request, and EventTiming omits short events. No primary Playwright actionability trace or long-task collector exists. The separate trace cohort cannot explain individual primary actions.

## Source-grounded boundary

The inspected immutable d20 AppShell mounts the lens and context panel as siblings. Definition does not await the orchestrator read. Inspected shell/orchestrator CSS declares static grid/flex widths, not a timed horizontal animation. Mounting may cause layout displacement; a horizontal-transition explanation or serial backend barrier is not established. The index 156 interval precedes destination mounting.

The existing driver verifies through `textbox.waitFor`, a saved-value evaluation, and `inputValue`. Index 32/232 delay lies within their combined Node verification interval. A concrete next diagnostic is timestamps at these existing await boundaries and around the existing Open command, preserving every action, RPC, wait, quota, full span and threshold. This is a proposed diagnostic, not a product correction or established cause.

## Provenance, privacy and closure

`actual/original-binding.json` binds the private original result SHA `fcfa0cca877aac152ee449645caa068d26641f317e31979b63cdb000656d0ad3` and all six accepted helpers to the sealed v2 sources. The current repository is not used as run identity. `analyzer-binding.json` records the only analyzer change from v1: the expected frozen-helper directory. Its projection/join/quantile algorithm is unchanged; six original offline synthetic tests passed in the v1 capsule and were not rerun or recounted here. `test_analyze.py` is retained for reproduction, not as a new pass claim.

The projections omit private names, values, credentials, raw URLs, free-form errors and private fixture paths. Project IDs map to declared owner-order ordinals. The private original result, journal, trace and provenance are retained in place, not copied into this capsule. Scripts contain external evidence locations, not private fixture paths.

`independent-closure.json` records exact journal-bound driver PID 2394348 and controller PID 2394554 absent, empty Docker containers/volumes, matching engine/image identity, and unchanged bounded original artifacts. It does not claim a browser PID census, whole-machine absence, historical zombie attribution, private-fixture inspection or cleanup mutation. Browser/fixture removal and resource counters remain explicitly driver assertions.

`old-archive-check.json` independently captured the d20 archive hash and original provenance before the parent replaced the legacy release directory. The archive was 69,055,452 bytes, SHA `5e4e0a2a380268f0d17cbbf2cf7cfb41f5aa64abc8f454864fdb7eb0bdb6400c`, matching provenance; the original file metadata and provenance were stable. Do not reinterpret a later archive at the old pathname as the measured package.

The prior Rust relocation moved root-disk bytes into tmpfs, consuming memory capacity. It is background environment context, not a whole-host capacity gain. No matched experiment isolates that change. Frozen v1's 100 Define control-cap censors and the v2 controlled fixture's initial precondition failure remain preserved in their respective prior capsules; this result does not overwrite them.

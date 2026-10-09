# CLOUD-054 installed terminal attempt11 at 29931afc — failed settlement, retained reservation

Actual product/package and four Git utility helpers: **`29931afc5b31ae9b9bd7b8973a7e3fdb840609a3`**. The frozen external **CLOUD-069 v11 warm** launcher/scenario and outer wrapper drove a real interactive shell through the installed API, Docker runtime and Chromium SessionPane. No model provider was invoked. Runtime image **`sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8`** was carried from its `5f4a966f` build; all 106 captured runtime inputs match the selected immutable product source. The matching package receipt reports 962 verified members. Publication binds existing receipts without repeating the build or workflow.

The first shell printed READY, a textual exit lookalike and FIRST. Exactly one typed native `exit status 0` followed FIRST; the same socket closed with **code 1000, wasClean=true, socket error=false**. Captured ordering is FIRST 70 → typed Exit 71 → clean Close 72. Completed assertions establish the ended footer, no reconnect overlay, suppressed further stdin and no replacement socket during the bounded guard. Fresh-status/Restart readiness subsequently passed; the final passive snapshot shows exited/Restart and the original single browser socket.

Typed-exit receipt is at **15:24:37.694Z on 2026-10-09**, with clean close at **15:24:37.697Z** and a roughly **2.4 ms** interval on the same browser monotonic clock. The held running poll was released at **15:24:40.588Z**. The `initial-natural-settlement` milestone is **15:24:41.572Z**, after the helper awaited Restart presence. Readiness therefore passed within **984 ms after release**, or **3.878 s after typed-exit receipt**. These are milestone upper bounds; no exact button-appearance or first fresh-status response timestamp was retained.

The attempt failed during the **35-second `initial-natural-settlement` observation loop**, with the actual driver error **`spawnSync docker ETIMEDOUT`**. The failure phase at **15:25:16.579Z** preserves a 26-byte message hash matching that exact driver message. The scenario's safe projection labels it `kind: other` and `code: null`; those original fields remain unchanged. This was not the earlier attempt's unknown-state assertion.

[The recorded lifecycle operands](settlement-observations.json) establish:

| Observation time (UTC) | Original allocation | Retained charge |
| --- | --- | --- |
| 15:24:41.798 | `exited` | 1 lease |
| 15:25:02.166 | `removing` | 1 lease |
| 15:25:05.378 | absent | 1 lease |

The following detailed samples preserve exact allocation identity, ledger hash and root-free-byte observations at their own timestamps. Those free-byte samples are not an atomic failure-time operand. The timeout does not establish a new capacity-admission refusal or a permanent leak. Recorded API and browser-forwarded **Restart counts are both zero**. No Restart response, replacement allocation/socket, second exit 7, warm-restart acceptance or cold-recovery acceptance was reached. Wrapper exit is **1**, `passed: false`, over **85.63230749499053 seconds**.

The original container was absent by later observation, while [the original reservation](closure/retained-lease-projection.json) remained held:

| Field | Retained value |
| --- | --- |
| Lease | `manual-4c302090` |
| Session / sandbox | `s-0cf36a75` |
| Project | `p-eccf393f` |
| Ledger state | `backend-settled` |
| Charge | **1,024 MiB and 1,000 mCPU** |

[The immutable product source contract](source-contract.json) shows that a later eligible worker admission invokes bounded reconciliation that can remove a same-scope, settled, exited worker's lease when the required absence proof is available. A charge held at this closure alone does not establish a permanent leak. No subsequent admission or release was observed or performed; a separate proposed idle-cleanup change is outside this source and witness.

The cleanup phase named `supported-session-stop` completed with the same retained row and no containers; there is **no recorded session DELETE request**. Its label does not establish product Stop or charge release. The final source/inventory phase failed with **“Retained charge — 1 !== 0”**, and the driver has no successful `final` receipt. `matchesDriverFinal: false` remains literal. No ledger edit, hand release, fixture repair or state mutation was performed for publication.

[Independent closure](closure/independent-closure.json), SHA-256 **`0b37cbc99eead1c620aaeb04920bbf9b2da3148fe47a1bc1b3caa47758f904c0`**, records zero selected containers and volumes, one lease and zero checks/fences. It records **zero known live processes and two PID 1 Chromium zombies**: `2482355/30554254` and `2482358/30554254` (PID/start ticks). Launcher, controller and main browser are absent. This covers known identities and selected inventory; repeated unreadable-environment observations are not distinct process counts or whole-machine closure. The original fixture remains retained and unaltered.

Private ledger bytes are excluded. The **498-byte SHA-256 `560daf59fa75755f943608d04ec4fc76db228365c9ae0debfbaf5d726f018953`** matches the captured lease projection and independent closure. Session-registry and shell-history files are bound by hash/length only in `private-state-source-bindings.json`, with no content export. These publication-time hashes do not invent a historical output file or observation time. Existing browser output/marker/wire observations and both screenshot source hashes are bound in `output-observation-bindings.json`.

The private browser-error sink reports zero rows/bytes and successful settlement. Controller metadata reports one projected relay closure 1000 with hashes and lengths; raw private controller stderr and transport-error bytes were not reread or exported. Both screenshots were visually reviewed and show disposable shell markers and terminal state without visible credentials. They support displayed state, not a successful Restart mutation.

All **35 phase records**, final/incomplete results, wrapper/identity samples, both screenshots, six accepted helper sources, outer wrapper, package/runtime provenance, independent closure and scoped projections are retained. The result's broad `scope` describes the intended scenario; the reached stages above define this failed witness. Earlier failures remain separate and unchanged.

`original-bindings.json` separates exact captured bytes from exact fixture-root substitution using `[PRIVATE_FIXTURE_ATTEMPT_11]`. Private state/configuration/credentials, raw diagnostic sinks, fixture-path pointer and launcher HOME are excluded. Only the three authorized state/history files were read for bounded hashes. `publication.json` hashes all members except itself and `SHA256SUMS`; `SHA256SUMS` also binds the manifest. Publication performed no runtime, browser, Docker, API, test, repository edit or private-state mutation. Linux functional positives do not establish model quality, performance, Windows behavior, warm/cold recovery or full-product acceptance.

# CLOUD-077 warm terminal observer budget

Status: the external helper correction has passed its scoped controlled validation. No app, browser, compiler or Docker workload was started. The original warm attempt and its retained allocation/lease remain untouched; installed verification is still open. The parent owns any later runtime admission. Historical prepared-only notes are preserved under `preparation/` and in the unchanged test README.

The frozen source copied into `before/` is `/workspace/.luci-tools/cloud073-teardown-route-v2/warm`. `original-inputs.json` binds its three exact files and selected product retry/client inputs. `fixed/` changes only the warm launch/scenario helper; `outer-run.py` is byte-identical. No repository product change is proposed.

## Confirmed helper defect and proposed correction

The old CDP observer and native observer stopped retaining after eight sockets, the Playwright observer failed above eight, and the private error recorder rejected ordinals above eight. The latter also retained only 32 errors. After any permitted failed handshakes, scenario assertions still treated raw socket array positions 0 and 1 as the two successful terminal streams. Raising only one socket cap therefore could not correct the helper.

Product `desktop/src/api/attachRegistry.ts` specifies delays 250, 500, 1,000, then 2,000 ms and a 60,000 ms failed-ladder budget. The budget is checked after failure before scheduling, so a continuously refused ladder can start 34 attempts, earliest at 0, 250, 750, 1,750, 3,750, …, 59,750 and 61,750 ms. Network/ticket waits can delay those starts. A successful open resets the ladder; 34 is **not** a universal count bound for repeated successful reconnects. This scenario instead requires exactly one successful stream in each of two declared generations and rejects additional attempts after a successful stream until the one explicit Restart boundary.

The fixed helper derives and cross-checks these limits in its launch and scenario modules:

| Retained observation | Bound |
| --- | ---: |
| Attempts per declared generation | 34 |
| Two-generation native/CDP/Playwright sockets | 68 |
| Browser mutations: up to 68 tickets + login + one Restart | 70 |
| Error records per channel/socket | 8, unchanged |
| Two channels × 68 sockets × 8 errors | 1,088 |
| Raw error text | 2,048 bytes, unchanged |
| Encoded private row | 6 × 2,048 + 1,024 = 13,312 bytes |
| Private error-file capacity | 1,088 × 13,312 = 14,483,456 bytes |

The row allowance covers worst-case JSON escaping plus bounded metadata. The file capacity is an enforced upper limit, not a preallocation or expected run size. Private raw errors remain private; public projections retain only ordinal/length/hash. Overflow remains failure. Existing frame/output/phase/receipt bounds are not increased.

The passive native monitor permits the second ladder only when the launcher admits the exact single Restart POST after fresh admission. Merely intending to click does not arm it. The first successful stream must already have typed Exit 0, a clean normal close and settled observations; all preceding failed handshakes must be settled. Before that request, any new attempt after the first successful stream fails, including the interval after typed Exit. Any attempt after the second successful stream and any third successful stream also fail.

The actual scenario uses `terminalGenerationStreams` to select opened streams by their observed ordinal, while requiring unopened attempts to carry no server frames, stdin, resize or other sent frames. It still requires same session/project, two distinct current execution generations, one Restart, retained terminal DOM, ordered output markers, typed Exit 0/7 and clean normal closes. The after-Exit observation intervals must show no additional attempts. The source correction does not fabricate the original handshake's HTTP body or backend refusal reason.

## Unchanged acceptance boundaries

`preserved-boundaries.json` records a source-byte comparison of the original held-list gate, bounded second-poll release and exact natural-settlement observer. The outer supervisor is unchanged. Initial attachment wait remains 15 seconds, post-ACK replacement wait 6 seconds, Restart-response allowance 150 seconds, and workflow limit 8 minutes. Product HTTP reads remain 10 seconds and the held old response remains bounded to 8 seconds.

A slow Create may force the old-response gate to release before Restart acknowledgment. In that case the installed after-ACK ordering is unexercised: `warmRestartPassed` may become true but overall `passed` remains false and the required ordering gate remains listed as incomplete. This preparation neither changes that acceptance result nor attributes slow Create to a particular backend/storage cause.

## Controlled verification and exact run scopes

The shared test body uses exact exported helper functions with controlled native WebSocket, CDP and Playwright events. It includes the original two-stream positive control and a counterexample with first Exit 0, explicit Restart, ten refused handshakes, then replacement Exit 7. Separate shared cases target native/CDP caps, private ordinal nine and more than 32 errors. Fixed guards cover the full 34+34 envelope, actual scenario stream selection, excess attempts/streams, post-Exit reconnect, invalid Restart boundaries, unopened payload/input, per-socket errors and encoded byte bounds.

These tests are helper protocol evidence, not an installed runtime replay, product timer test, real Create result or latency result. Exact commands, raw TAP, exit codes and source/contract hashes are under `validation/`. Node v24.19.0 ran serially with `--test-concurrency=1` in two parent-granted source-only windows.

- Original six-case shared body: **1 passed / 5 failed**, exit 1. The unchanged two-stream positive control passed; all five cap counterexamples failed.
- Initial fixed batch: **22 passed / 4 failed** out of 26, exit 1. All six shared cases passed. Four fixed-only negative cases correctly rejected invalid Restart boundaries, but their test regex expected a different message. Exact initial test source and failure output are retained.
- Corrected affected cases only: **4 passed / 0 failed**, exit 0. The sole test change matches `Initial normal exit0 required before Restart` exactly for abnormal/unclean close, nonzero first Exit and prior stream error. No helper source or shared-body test changed.

This gives **26 unique final passing cases assembled from the documented runs**, not one clean combined 26-case rerun. There were no skips or cancellations. The six-case shared body SHA-256 is `6338e4266b50b8f2d44087995a49837df6575ce3d10e0b5f7aea08665bd412b1`. All 14 helper/test/contract inputs were unchanged during each run; only the documented fixed-only test expectation differs between the two windows. Parsing occurred through imports of the actual helper modules; no separate compiler or syntax-only pass is claimed.

An initial command attempted `--version` at a nonexistent presumed Node path and failed before starting any test. Its receipt is retained. `command -v node` identified the existing inherited binary; no dependency install or source change was needed.

Independent read-only review found no concrete blocker in the exact final launch/scenario bytes, recorded in `review.json`. That reviewer ran no tests or runtime. The helper source pins remain launch `0ebd521c6160a19848e6e549b890478af1ed83d7e2230399862879c2d000a66f` and scenario `31981be982085541d1f79c1af3c52a6e480b939efc9d4020abb577905e98b2a6`. No original failure, retained allocation or incomplete installed-ordering gate is cleared by this source result.

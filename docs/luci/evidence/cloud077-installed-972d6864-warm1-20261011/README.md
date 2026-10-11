# CLOUD-077 installed warm terminal: 972d6864 attempt 1

**Overall acceptance remains false**, with launcher exit 1. The fresh installed terminal lifecycle succeeded: two streams opened on their first attempt, the first ended naturally with Exit 0, one same-session UI Restart produced a distinct execution generation, and the replacement ended naturally with Exit 7. Both streams closed normally with code 1000. The required old-session-list response after Restart acknowledgement was not exercised.

The second held old response was released at monotonic **71,995.726071 ms**, before the Restart acknowledgement at **93,132.538242 ms**: a **21,136.812171 ms** gap. The helper records `slow-or-late-restart-within-longer-create-budget`. The original 8-second old-response gate, 10-second client read, and all other helper deadlines remain unchanged. No specific backend/storage cause is established by this observation. `warmRestartPassed=true` does not override `passed=false` or the incomplete `installed-old-response-after-restart-acknowledgement` gate. The wrapper's `failure=null` means it recorded no separate supervisory failure, not that the scenario passed.

Product and imported repository helper source are exactly `972d6864a30b742b12545c8e12f4ca3ae2faa884`. The installed package has 965 matching archive/extracted members. Image `sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8` retains its original `5f4a966f` provenance; all 106 captured runtime inputs match the product commit. No image rebuild is claimed.

Exact C077 helper pins:

- launch: `0ebd521c6160a19848e6e549b890478af1ed83d7e2230399862879c2d000a66f`
- scenario: `31981be982085541d1f79c1af3c52a6e480b939efc9d4020abb577905e98b2a6`
- outer supervisor: `e7eddf9d3eb3edaca5930bc50f3eb7079836ba22cc158d5de3aa6e51dea31a49`

The initial allocation and charge disappeared naturally before Restart; its settlement observer recorded 52 polls over 27,538.742757 ms. Final cleanup phases all completed, with no pending mutations or cleanup errors. Independent read-only closure observed the exact launcher, controller and primary browser identities absent. Two independently bound browser identities remain zombies under PID 1: 2680124 and 2680129, both with start tick 45981574. The selected daemon had no containers or volumes, matched the recorded engine and image, and the fresh private ledger had zero leases, checks, fences and check fences. This is scoped identity/inventory evidence, not whole-machine closure. The original unreadable-process observations remain explicit in the raw receipts. The older warm786 failed fixture and retained charge were untouched.

`run/driver/result.json` preserves the complete failed result, including scenario, phases, protocol observations, final state and cleanup. All 55 individual phase receipts, initial checkpoint, wrapper receipt, identity journal, package/image provenance, six accepted helper sources and outer supervisor are retrievable here. `original-bindings.json` records exact original and public hashes; only the exact private fixture-root string is substituted. The original result SHA-256 is `0d01c6ba4b5b2ed21adcfb91088f84785c980ad9cb6932953466ec923ba5e4e5`.

Private controller/error logs, launcher stdout/stderr, fixture pointer, screenshots and fixture/auth/config contents are excluded and hash-bound. The capsule preserves exact outcome fields; no result is promoted to pass. `original-immutability.json` verifies every original run artifact before and after assembly; the closure separately binds all 31 private fixture files unchanged. Assembly ran no app, browser, Docker mutation, compiler or test. No Linux responsiveness, native Windows, live-model or complete-product result follows from this terminal run.

Verify the retrievable capsule with `sha256sum -c SHA256SUMS` from this directory.

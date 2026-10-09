# Installed 786 cold attempt1 — terminal scenario true, overall failed

The actual cold-reopen terminal scenario passed; the driver and wrapper failed overall with launcher exit1. The first retained failure is origin `callback`, stage `product-settlement`, phase `cleanup`, source label `workflow-closing`, observed at **2026-10-09T17:13:25.514Z**, then snapshotted at **17:13:29.736Z**. Main-catch failure/failed-stage fields are absent, `cleanup=[]`, pending mutations are empty, and all six named cleanup phases completed. Those fields do not override the preserved first callback failure or the overall false result.

The public writer metadata reports durable private retention of **660 bytes**, SHA-256 `e4f790dce7b419c1b97276d5d5c252583cf810b484946ca3ee77d059fb252738`. The publisher did not read or export that private error record. **The failed callback's request method and path were not retained.** One separate closing read was recorded as `GET`, class `session-list`, starting at17:13:25.516Z and acknowledged aborted at17:13:25.712Z. This row does not identify the earlier failed callback. A later C073 correction remains separate from this original attempt.

Product/package and four Git helper inputs are pinned to **`786652704fa8bebd1ba1af41cfa1d414d7def100`**. Accepted cold launcher SHA-256 is `7abcaeebed205bc83344497498a27fa84c83dc1dfc78367a667b190fe61c160b`; accepted scenario SHA-256 is `7af700a6e90262629e1ea5687b883430b4acfd966ec41e305319c963ba3bcf5e`. The six accepted source files, exact outer runner and original source-before/after records are retained. The package receipt reports964 matching extracted members. Portable archive SHA-256 is `265b41aa1eb18334035e08b9dd0e500787f31a008988ebe5c4ca559c9a77b4aa`; package binary/index identities remain in original provenance. The publication does not rehash or rebuild binary/archive/image payloads.

Runtime image is `sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8`, originally built at5f4a966f. The existing parity receipt verifies all106 captured image inputs against immutable786 and is preserved verbatim. Source parity is separate from this run's selected-daemon identity and observed functional behavior.

| Actual observation | Retained evidence |
| --- | --- |
| First native shell exit0; clean browser Close1000 | phase011; output/Exit/close sequences76/78/79 |
| Original exact allocation and reservation naturally absent before Restart | phase022;55 polls over30.369985223seconds from settlement-loop start |
| Cold reopened exited record omits old execution fingerprint | phases023–026; ended screenshot |
| Authentic new-generation running poll before acknowledgement; zero replacement sockets then | phase032 |
| Successful acknowledgement precedes replacement attachment | phase034 at17:13:06.643Z; socket created17:13:06.678Z in phase039 |
| Replacement native shell exit7; clean browser Close1000 | phase042; sequences77/78/79 |
| Replacement exact allocation and reservation naturally absent before cleanup | phase044 at17:13:25.461Z |
| One browser Restart; no API Restart, Stop or DELETE assistance | result action records; pending mutations empty |

The held successful response and early running poll are **not identical parsed session JSON**. This is a cold acknowledgement-ordering observation, not an identical-DTO proof or the separate old-response-before-Restart/delivery-after-acknowledgement scenario. The installed Restart response was200; non-200 diagnostic behavior was not exercised. Both browser closes report clean1000. CDP has no decoded Close frames, so raw Close-frame direction/count is not established. Exit7 is a tested nonzero terminal outcome, not successful useful work. Settlement-loop durations do not measure typed-exit-to-release latency or responsiveness acceptance.

## C074 disposal observation

The monitor was disposed once and fulfilled; protocol failures are empty. The before/after/final observations are copied separately and exactly. The final snapshot at **17:13:29.734Z** retains the same settled monitor/transport observation as the immediate after snapshot.

| Boundary | UTC timestamp | Monotonic ms |
| --- | --- | ---: |
| Outer disposal observation entry |17:13:25.481Z|129157.705074|
| Outer operation entered |17:13:25.485Z|129160.591366|
| Monitor first-call entry |17:13:25.485Z|129161.082506|
| Transport detach invoked |17:13:25.488Z|129163.876860|
| Detach returned its promise |17:13:25.493Z|129169.049450|
| Underlying detach fulfilled |17:13:25.521Z|129196.464680|
| Inner wait fulfilled |17:13:25.521Z|129196.517981|
| Monitor fulfilled |17:13:25.521Z|129196.529108|
| Outer wait fulfilled |17:13:25.521Z|129196.540545|

Monitor first-call-to-settlement is **35.446602ms**; outer operation-entered-to-wait-settlement is35.949179ms; detach-invocation-to-underlying-settlement is32.587820ms. These are retained diagnostic intervals, not a calibrated benchmark. This run does **not** reproduce or explain the earlier b94 transport-observer disposal timeout, prove it fixed, or causally connect disposal to the callback failure.

## Closure and publication scope

Independent closure SHA-256 **`cc3f7bdf34899e54fc556070655d5a17189a40abb3cbfc745ba63d0f847c9134`** records three exact known identities absent and two exact-start Chromium zombies parented to PID1. The selected daemon has no containers/volumes; all four ledger arrays are empty. All31 private fixture files remain unchanged. Historical known-allocation entries are not surviving resources. This is known-identity/selected-daemon closure, not a whole-machine census or an upgrade to overall acceptance. Unreadable environment observations remain explicit in original receipts.

Original result SHA-256: `f14784401c03a0b26376c0455516806a083363115ecdbd0bb5caebfa009f9bf2`. Original wrapper SHA-256: `b3b971629e50e02aceb5c708d9236ee5cfe3f6cc242e3229d0120418af794a95`. Original and public hashes record each exact private-fixture-root substitution. All46 phase receipts, safe result/wrapper metadata, accepted source and two visually reviewed disposable fixture screenshots are retained. Screenshots support manual presentation review, not OCR or authority claims.

Raw private errors, controller/transport logs, launcher output, fixture/auth/config contents and large package/image payloads are excluded. Private sinks appear only through existing writer length/hash metadata. The pointer file was read solely to replace the exact private root; no fixture contents were opened. The finite credential scan supplements the fixed file/field scope and is not exhaustive detection. The manifest binds public bytes; immutability checks cover selected published originals. Existing independent closure supplies private fixture immutability, without a new census.

Run `sha256sum -c SHA256SUMS` to verify the public capsule. `assemble.py` requires retained originals and an unused external destination; it performs only bounded metadata/source reads and publication. The accepted original launch command is retained for provenance and must not be replayed against this consumed output or failed fixture. Any new run needs a separate coordinated window, fresh fixture and separately bound helper. No browser, Docker operation, tests, source edits or admission changes occurred during this publication. Windows/native, model/provider quality, full recovery, sustained resources and responsiveness remain outside this run's scope.

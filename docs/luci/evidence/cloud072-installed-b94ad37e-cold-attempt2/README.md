# Installed b94 cold attempt2 — C072 diagnosis retained, overall failed

The cold browser-reload scenario passed; the driver and wrapper failed overall (launcher exit 1). C072 retained the first failure after cleanup: origin `callback`, stage `product-settlement`, phase `cleanup`, source label `workflow-closing`, observed at `2026-10-09T16:48:43.843Z` and snapshotted at `16:48:49.500Z`. The public record reports durable private retention of 1,119 bytes, SHA256 `ae4ade04b15237ab5cc2bacaf6e6e668164e4abd60f91a126161edf874eb6f49`. The publisher did not read or export that private record. This verifies C072 diagnostic retention for this attempt; it does not establish the cause of the earlier C071 cold-attempt1 failure.

Cleanup independently records `transport-observer-dispose-timeout`, while all six named cleanup phases report completed. That timeout remains unexplained and is not claimed fixed or causally linked to the callback failure. The callback's request method and path were not retained; this receipt does not establish an actual GET. Two PID 1 Chromium zombies are observed residue, not a demonstrated failure cause. The conservative overall failure is preserved. Original receipts and private fixtures remain unchanged.

The bound product and four Git helpers are from immutable source `b94ad37e53181e360ac8910ee980570de622728c`; this is not a claim about the current checkout. Runtime image is `sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8`. C072 launcher SHA256 is `7789d0f5e89fdd643cf433dc8786f44fba16a3e24884c12aad738ab4935b75cc`; unchanged scenario SHA256 is `e96058d031f8d46b9ab4c4fcb4e14c485bacc939044bf1592a8ec24497a7d7f7`. The six accepted helpers, outer runner, and unchanged source-before/after bindings are retained. All 106 captured runtime inputs match immutable b94. The retained package verification reports 963 matching members; publication does not reread or rebuild the binary, archive, or image.

| Observed result | Evidence |
|---|---|
| Native shell exit 0, then browser-reported clean Close1000 | phase011; output/Exit/close sequence 76/77/78 |
| Original exact allocation and manual reservation naturally absent before Restart | phase022; 52 polls, 34.041147739 seconds |
| Cold reopened ended row omits execution fingerprint | phases023–026; ended screenshot |
| New-generation running poll reaches cold view before Restart acknowledgement; no replacement socket yet | phase032 |
| Acknowledgement precedes replacement attachment | phase034 at 16:48:23.345Z; socket creation phase039 at 16:48:23.367Z |
| Native shell exit 7, then browser-reported clean Close1000 | phase042; sequence 78/79/80 |
| Replacement exact allocation and reservation naturally absent before cleanup | phase044 at 16:48:40.963Z |
| Exactly one browser Restart; no API Restart, Stop or DELETE assistance | result actions/mutations; pending mutations empty |

The held response says `starting` and the early poll says `running`; `identicalParsedSessionJSON=false`. This does not demonstrate an identical-DTO race. The installed Restart response is **200 only**; C071's non-200 diagnostic branch was not exercised. Browser close events report clean 1000; CDP contains no decoded Close frames, so direction/count of raw Close frames is not established. The 34.041147739-second observation belongs to this attempt's first retirement; the earlier 16.009196-second result belongs to separate warm attempt2. These controlled receipt observations are not performance acceptance.

Independent read-only closure, SHA256 `5b842fd44feeca33f2cdd4f04be018d8b857c9a2a8951056d45874fb85c6c7a8`, records three tracked identities absent, two exact-start Chromium zombies parented to PID 1, empty selected-daemon container/volume inventory, empty four-array ledger, and 31 private fixture files unchanged. Historical `knownAllocations` entries are not surviving resources. Closure covers the recorded identities and selected daemon; it does not certify the whole machine or overall cleanup acceptance.

This capsule exports safe structured receipts, accepted source, and two visually reviewed disposable fixture screenshots. The exact private fixture root is replaced consistently; original and public hashes record every substitution. Raw controller/transport logs, the private first-error record, fixture contents, auth files and launcher output are excluded. Private sinks are represented only by existing writer hashes/lengths. The publication manifest and checksums bind the exported files; original immutability checks cover selected published originals. No runtime, Docker mutation, tests, or product edits were performed for this audit.

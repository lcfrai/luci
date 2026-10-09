# Installed b94 cold terminal evidence — overall failed, scenario passed

The original cold browser-reload scenario passed, but the driver and wrapper failed overall (launcher exit 1). The receipt does not retain a final failure or failed stage. All recorded cleanup phases completed and cleanup errors are empty. The pinned final expression also requires the hidden `failure` value to be empty, so a failure occurred outside the retained main-catch projection. Its provenance is unknown. Two PID 1 Chromium zombies are observed residue; the available evidence does not identify them as the cause. CLOUD-072 tracks the missing late-failure diagnostic. Original receipts are unchanged.

Product source is `b94ad37e53181e360ac8910ee980570de622728c`, runtime image `76043c5d…71acec8`. The six accepted helpers are retained; four Git helpers match that immutable product commit, and launch/scenario hashes match the run and the unchanged outer source-before/after receipts. All 106 runtime inputs match b94. The retained package verification reports 963 matching members. Publication does not reread or rebuild the binary/archive/image.

| Observed result | Evidence |
|---|---|
| Native shell exit 0, then browser-reported clean Close1000 | phase011; output/Exit/close sequence 76/77/78 |
| Original exact allocation and manual reservation naturally absent before Restart | phase022; 44 polls, 32.332788356 seconds |
| Cold reopened ended row omits execution fingerprint | phases023–026; ended screenshot |
| New-generation running poll reaches cold view before Restart acknowledgement; no replacement socket yet | phase032 |
| Acknowledgement precedes replacement attachment | phase034 at 16:29:51.206Z; socket creation phase039 at 16:29:51.224Z |
| Native shell exit 7, then browser-reported clean Close1000 | phase042; sequence 77/78/79 |
| Replacement exact allocation and reservation naturally absent | phase044, before cleanup |
| Exactly one browser Restart; no API Restart, Stop or DELETE assistance | result actions/mutations; pending mutations empty |

The held response says `starting` and the early poll says `running`; `identicalParsedSessionJSON=false`. This does not demonstrate an identical-DTO race. The installed Restart response is **200 only**: it does not exercise C071's non-200 diagnostic branch. Browser close events report clean 1000; CDP contains no decoded Close frames, so direction/count of raw Close frames is not established.

Independent read-only closure records three tracked identities absent, two exact-start Chromium zombies parented to PID 1, empty selected-daemon container/volume inventory, empty four-array ledger, and 31 private fixture files unchanged. Historical `knownAllocations` entries are not surviving resources. This is not whole-machine process certification or automatic-cleanup acceptance. The 16.009196-second first retirement belongs to separate warm attempt2, not this cold attempt.

This capsule exports safe structured receipts, accepted source, and two visually reviewed disposable fixture screenshots. The exact private fixture root is replaced consistently; original and public hashes record every substitution. Raw controller/transport logs, fixture contents, auth files and launcher output are excluded. Private sinks are represented only by existing writer hashes/lengths. The source manifest and checksums bind the publication; original immutability checks cover selected published originals. No runtime, Docker mutation, tests, or product edits were performed for this audit.

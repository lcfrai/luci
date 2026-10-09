# V2 switch-specific source correspondence

No actionable product patch is supported by the switch-specific layout/style stacks.

The analysis joins the projection clock to the eight original Home/Define `clickAt` → `observedAt` pairs. All **111 selected events** are completely inside those 16 windows: **41 Layout** and **70 UpdateLayoutTree**. Of these, **16 have stacks** (8 of each type); the remaining **95 are unattributed**. Home accounts for 24 events and Define for 87.

Each Define window contains one attributed style/layout pair, taking 1.350–2.674 ms and 0.903–2.227 ms respectively. Home has no attributed layout/style stacks. Maximum switch-specific Layout is **7.243 ms**; maximum style update is **9.046 ms**, without a stack. The overall 14.6 ms FocusScope event belongs to draft reset and is excluded.

The exact asset `web/assets/index-oRjgUib4.js` has SHA-256 `526b6f92ddac31da933f04978f13f5bd5a3e2e5312422a04d89b8c53051c9c3e`, matching every selected attributed frame. All 16 leading frames are at **9:107406**, the emitted `removeChild` operation in `Tu`. It corresponds structurally to `commitDeletionEffectsOnFiber` in `desktop/node_modules/react-dom/cjs/react-dom-client.production.js` (function line 10201; removal line 10283), SHA-256 `8a8147c1cebdb19d82fd11bdb76391be9b20d00edf67cc8e8b30f44e850ce693`. The 18 observed positions, retained in the JSON, remain in React commit/deletion/scheduled-work paths. **No app component caller is identified.**

The clock offset is 310353563604 µs, with 16 anchors and approximately 81.000 µs spread. Per-window counts, exact positions, input hashes, and selection details are in `source-correspondence.json`.

This is static emitted-code correspondence, not source-map attribution or verification of full dependency/build provenance. No source maps were generated or consumed; no asset was rebuilt. No tests, builds, browser execution, or source edits were performed for this report. These diagnostic observations support no acceptance or performance-improvement claim. Duration sums in the JSON are inclusive wall times, not exclusive CPU or an end-to-end latency decomposition.

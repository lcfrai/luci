# CLOUD-085 primary Luci pane: source checkpoint

The existing project-scoped Luci conversation is the default main pane. Its composer, retained history, current context, questions and coordinator settings remain in one existing component. Discover, Design, Develop and Deploy remain tools in the phase navigation. The top bar always offers Luci and identifies the current project; choosing a different project preserves the existing draft guards. This changes presentation and navigation, not backend authority or the conversation engine.

The same-project phase editor stays mounted and inert when returning to Luci, preserving current edits; it is deferred until first opened and its retention is cleared when project/source changes. A new regression caught an unwanted hidden remount after leaving and returning to a project. Its failed original AppShell source and exact test binding are retained. Explicit existing-session navigation is a separate prerequisite checkpoint `0664275c0c39f27fe583e6aac55045f425bff685`, with exact identity checks before any discard. The main surface store checkpoint is `f68fbfb2f704377ced872421462ad016c1ae009a`.

Validation is overlapping, not additive:

- Initial three-file run: 157 passes, two fixture failures. The failed assertions queried a Radix-hidden background as accessible and assumed jsdom exposed `inert` as a property. Both original failures remain.
- Corrected initial shell cases: seven passes. Expanded actual Orchestrator and shell run: 142 passes (133 + 9).
- New hidden-editor-return regression: one original failure with nine filtered cases. The test is byte-identical in final validation; the original AppShell is captured separately.
- Final AppShell and LuciWorkspace run after that correction and the session-navigation checkpoint: 29 passes, zero failures across two files.
- Full desktop TypeScript: exit 0. All 468 captured repository TS/TSX/CSS/package/config inputs were unchanged across final shell validation and TypeScript. This includes the additive team-intent API and other current frontend sources.

Actual Orchestrator component checks cover primary composer focus without sending, message continuity when tools open/close, explicit project draft recovery and read-denial hiding. Shell composition tests use controlled phase/conversation children and the real store/navigation/dialog components to cover current-project event routing, draft guards, four project choices, compact drawer Escape/focus, breakpoint changes, and hidden editor lifecycle. They do not substitute for browser rendering or backend tests. Read-only independent review found no additional concrete regression in this scope.

The command list, source overlays, cohort counts and limits are in `receipt.json`. Logs, original failure source, frozen final owned sources and full input manifests are retained. `manifest.json` hashes every capsule member except itself and `SHA256SUMS`; the latter includes the manifest. No private credentials, user project files or runtime fixture data are included.

No real browser geometry, latency, Windows/native application, live provider, cloud VM, full autonomous delivery or cross-project merged history is verified here. Current no-project guidance requires a project choice; it does not invent a global conversation. No full frontend suite was run for this checkpoint, and CSS/layout behavior still needs real browser acceptance. Existing tool components may retain their read-only observation work while mounted and hidden.

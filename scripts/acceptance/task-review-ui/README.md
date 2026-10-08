# Task review UI witness

This bounded Linux Chromium check renders Luci's real Home, task team, result,
checkpoint and captured-context components against explicitly controlled callbacks.
It checks keyboard focus, mobile layout, recorded review navigation and report
presentation. It does **not** run an installed Luci package, contact a control
plane, start Docker or agents, or establish native Windows or model performance.

From an existing development checkout with the desktop dependencies installed:

```bash
node scripts/acceptance/task-review-ui/source-browser.mjs \
  artifacts/quality-swarm/task-review-ui-new-run
```

Run as an ordinary Linux user. Chromium defaults to `/usr/bin/chromium`; set
`LUCI_CHROMIUM_EXECUTABLE` for another installed executable. The temporary browser
profile is created under `~/.cache/luci-ui-acceptance`, or
`LUCI_BROWSER_STATE_ROOT`, and removed afterwards. The helper closes its own Vite
server and browser. A pre-existing `result.json` is refused to preserve evidence.

The helper checks:

- Four-project Home summaries at 1440 and 390 pixels, with an explicit expansion
  for earlier review work and exact saved-build navigation.
- Worker-reported checks separately from recorded evidence; unsaved answer
  protection and one explicit answer callback.
- Checkpoint consent with no agent start, and focus on the exact successor task
  after successful reservation or a lost response followed by readback.
- Visible earlier-attempt question counts, explicit disclosure, and the exact
  retained report. Opening this view performs no capture or answer mutation.
- V2 captured definition/plan display, keyboard focus trapping, automated axe
  checks and horizontal overflow at both viewport sizes.

`fixture.tsx` is test data and replaces the relevant API methods in memory. Its
permission provider and queue are controlled fixtures. External browser requests
are blocked. These safeguards do not make the fixture a backend integration test.

The fresh output directory contains screenshots and a machine-readable
`result.json` with checks, callback counts, source hashes before/after, timestamps,
cleanup and limitations. Failed assertions preserve their receipt and screenshot.
Geometry and accessibility checks are functional evidence, not an application
performance benchmark or a complete accessibility certification.

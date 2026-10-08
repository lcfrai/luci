# Controlled Develop overview witness

Runs actual AppShell, Develop, project run, agent task, background queue, task result, integration review and Project understanding components at 1440×1080 and 390×844. Four project states and API callbacks are controlled fixtures. ProjectPlan and ProjectChecks leaves below the run are omitted. This is a source UI check, not installed-package, native Windows, backend, Docker or live-model acceptance.

Prerequisites: Linux, an ordinary user (Chromium's sandbox stays enabled), Node 24, Git, desktop dependencies from `npm ci --prefix desktop`, and Chromium at `/usr/bin/chromium` (or `LUCI_CHROMIUM_EXECUTABLE`). Do not run alongside a resource/timing acceptance window. The helper blocks requests outside its own loopback Vite server and never starts agents, enqueues tasks, integrates changes or saves answers.

From the checkout root:

```bash
node scripts/acceptance/develop-overview-ui/source-browser.mjs artifacts/quality-swarm/local010/my-browser-run
```

Choose a fresh output directory; an existing receipt is refused. The helper writes screenshots, source/helper/fixture SHA-256 identities, scoped API reads, geometry, axe results, focus observations and cleanup to `result.json`. Chromium, Vite and the private profile are closed on success or failure. `LUCI_BROWSER_STATE_ROOT` optionally selects the parent of that private profile. No global process cleanup is used.

Checks include exact four-project rail navigation; early review navigation without mutation; accessible task/history/queue disclosures; pending task and reported-question visibility; exact report and integration-review reads; required integration consent; ready controls; uncertainty; a controlled positive pre-launch diagnostic with no report/review-success controls; keyboard trapping and settled focus restoration; zero horizontal overflow; and axe WCAG 2 A/AA and 2.1 AA rules. Checking zero violations is not full accessibility certification. Linux focus results do not resolve the separate Windows LOCAL-008 timing report.

On the phone, open Project understanding occupies the lower pane. The receipt records the raw first-card coordinate separately from the visible early Review tasks button and the focused task heading after using it. It does not claim the first task is initially visible beneath that pane.

## Exact baseline

`baseline/source-browser.mjs.txt` and `baseline/fixture.tsx.txt` preserve the original geometry-only helper and fixture bytes. `baseline/manifest.json` records their hashes. The recorded desktop source matched commit `3b8054e69340bce37afe3aec17189a33c2fb2991` exactly. To reproduce in a separate disposable checkout at that revision, copy the two originals into `scripts/acceptance/develop-overview-ui/source-browser.mjs` and `fixture.tsx`, install its desktop dependencies, then invoke the helper as above with a fresh output directory. Do not overwrite the final helper in a working checkout. Baseline and final helpers have intentionally different assertions; the initial task geometry uses the same controlled alpha state and open Project understanding.

Published source-check evidence and failure lineage: [LOCAL-010 UI evidence](../../../docs/luci/evidence/local010-ui/README.md). The earlier task-review-polish helper is separately archived under [task-review-polish](../task-review-polish/README.md); its receipts are not rewritten by this witness.

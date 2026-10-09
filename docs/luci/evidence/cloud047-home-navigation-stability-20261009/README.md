# CLOUD-047: stable Home project destinations

Status: corrected and verified with controlled Linux source components; matching installed/native acceptance is pending. CLOUD-032 response-time acceptance remains open and unchanged.

Home's project-name buttons originally lived in independently populated cards below an asynchronously populated capacity panel. The retained actual AppShell fixture measures those same accessible `Open project <name>` destinations moving by up to **428.015625 px at 1440 px** and **1504.546875 px at 390 px** as delayed capacity, summary and warning responses arrive. These are controlled layout measurements, not evidence that layout movement caused the failed installed response-time percentiles.

The correction puts the primary project destinations in a compact, labelled navigation grid before all asynchronous capacity and project details. Each destination uses Luci's pinned shadcn Button and the current project catalogue. The same `selectProject` action retains the normal unsaved-draft decision. Individual cards keep their goals, counts, warnings, exact review actions, questions, queue controls and context disclosures below. Their names are headings; the existing footer still opens the project. No stale cache, delayed navigation, summary-derived execution permission, hidden warning or fixed-height content clipping was added.

## Evidence and limits

- `before/`: original unchanged product files, 18 captured geometry records, original helper and fixture, screenshots, and failed stability assertions. The original three source hashes match the retained before copies byte for byte.
- `final-browser/`: 37 geometry/behavior records, **zero primary-target movement at both widths**, with exact final source/helper/fixture hashes. Includes delayed individual replies, capacity error/retry, visible unavailable/paused warnings, source-busy retained context with unavailable review actions, removed private summaries after denied reads, keyboard order, Enter navigation to gamma's exact saved purpose, and a 135-character unbroken project name at 390 px. The late long-name case covers the final CSS-only wrap correction.
- Scoped axe WCAG 2A/AA + 2.1AA scans of the overview region have zero violations at both widths. This is not an entire application or native accessibility audit. The raw reports retain passing and incomplete rule counts.
- `validation/`: **65 passing tests in three files**, zero failures, and TypeScript exit 0. The 426 captured repository inputs were stable across those commands. Only the heading `overflow-wrap` CSS rule changed afterward; `final-source.json` and the final browser source binding identify that exact follow-up. The TypeScript inputs are unchanged. Dependency installation bytes were not exhaustively rehashed for this scoped run.
- `retained/`: initial test helper failure (52 passes, one unsupported Chai matcher) remains separate from the corrected 65-test batch. The original failure used `toHaveAttribute`; the correction uses `getAttribute`. `after-extended/` retains the first extended browser helper's wrong expectation that stale review actions were disabled. The actual existing safe contract removes them; the corrected helper asserts removal and retained explanatory text. Neither helper error is credited as a product defect.
- The earlier intermediate passing browser runs remain private under `/workspace/.luci-tools/cloud032-home-stability/`; the final published run covers their scenarios plus the capacity, accessibility and long-name follow-ups. Counts overlap and are not added together.

Browser fixtures use real AppShell, ProjectsHome, ProjectOverviewBoard, HostCapacity and navigation/state code with controlled read-only provider/permission/API responses. They are neither a static screenshot nor a real service/agent/provider workflow. No runtime Docker allocation, model call, installed timing or RAM/CPU claim follows from these results. The original installed response targets and all failed timings remain unchanged.

## Reproduction

Use a disposable Linux source checkout at `/workspace/luci` with the pinned desktop dependencies installed, Node 24.19.0, and Chromium at `/usr/bin/chromium`. Run as a non-root user. The retained helper and fixture explicitly target that source directory; do not reinterpret their hashes as a Windows helper. The helper starts a loopback-only Vite server on a free port, blocks other browser origins, and uses disposable browser state.

```sh
cd /workspace/luci/desktop
npm ci
npm run test -- --maxWorkers=1 src/components/projects/ProjectOverviewBoard.test.tsx src/components/projects/HostCapacity.test.tsx src/store/projectNavigation.test.ts
npm run typecheck
cd /workspace/luci
node docs/luci/evidence/cloud047-home-navigation-stability-20261009/final-browser/browser.mjs /tmp/luci-cloud047-fresh-output
```

`before/browser.mjs` reproduces the original geometry counterexample only against the retained original product source (or its immutable baseline). It is not an instruction to overwrite a working checkout. The final helper additionally requires the new primary navigation landmark, so the original before helper is preserved separately.

Browser and Vite close calls completed in every captured run. The read-only closure receipt found no matching live helper/profile identity and no remaining fixture profile directories. It also explicitly records three unreadable live process environments and unassigned zombies. No per-browser PID/start snapshot was retained before these launches; this is **not complete browser/zombie or whole-machine closure proof**. No process was killed or assigned ownership on inference alone.

Root publishes the source and capsule together; no source revision is inferred from a test count. Windows should verify stable four-project hit targets during delayed real reads, keyboard navigation, long names and the unsaved-draft guard on the exact integrated candidate, alongside the separately open live-provider, recovery and sustained resource gates.

# CLOUD-048 retained refusal recovery guidance

A retained pre-Create failure now keeps its previous diagnostic visible and offers **Go to queue review**. This moves focus to the existing **Queue reviewed tasks** button when enabled, or to the labelled **Background queue review** region when authorization is currently unavailable. It starts nothing and opens no consent. A new attempt still requires the existing reviewed-task consent and explicit **Authorize queued tasks** action.

Only `ProjectSwarm.tsx`, `SwarmRunQueue.tsx` and `ProjectSwarmQueue.test.tsx` change. The applied bytes exactly match prepared variant 2. Source baseline is `b81d6454778ddedb6c3102b202c1e2fa6f458a0a` (documentation checkpoint following product/helper `99007404a5ddd8592dee376ea00d99c6180d26fe`). Historical failure, operation/session identity, admission, permission and retry authority remain unchanged.

Validation is preserved in [capsule/validation/summary.json](capsule/validation/summary.json):

- The same new regression file against original components: four existing cases passed; all four new cases failed because the recovery navigation was absent.
- Applied candidate: 75 tests passed across the four affected component suites, with empty stderr. Cases cover preserved diagnosis, no implicit enqueue/reserve/create or consent, current-revision explicit authorization, already queued and Execute-disabled fallbacks, focus and stale project guidance.
- Styled Chromium: 10 checks passed at 1440×1080 and 390×844. Actual expanded queues were 3,202–7,797 pixels tall. The enabled native control was focused and visible within a one CSS pixel rounding tolerance; disabled cases focused the region with its top visible. Navigation caused no mutation or consent; keyboard activation opened the existing consent and one explicit authorization sent one queue POST. Project switching removed the guidance. Served source, fixture and runner hashes remained unchanged. See [browser evidence](capsule/browser/README.md).
- The TypeScript invocation exited 2 on `DefinitionWorkspace.test.tsx(363,58)`, where another active owner used `fireEvent.toggle`. That unrelated failure and the post-run observation of the owner's correction are retained. This capsule does **not** claim a TypeScript pass; root owns the final combined check after source freeze.

The first browser attempt failed before mounting the components because Chromium's temporary Unix socket path was too long. Its original runner and raw failure remain here. A correction confined to the external runner selected an owned persistent profile and a short socket location; the second attempt passed. Chromium and Vite closed, the owned profile was removed, and no owned browser processes or port listener remained. Browser stdout and stderr were originally merged and are preserved as such; component and TypeScript logs retain separate streams.

`capsule/before`, `capsule/after`, and the two prepared variants preserve exact historical source and preparation bytes. Their original README/manifest statements describe preparation-time status. The current validation is recorded separately. Commands, durations, exits, source hashes, raw failures and successful output are retained; no failed result was replaced.

These checks use actual source components and styles with controlled API transport. They do not establish installed backend behavior, native platforms, Docker execution, model quality, full AppShell behavior, sustained completion or latency. No runtime fixture was mutated. No automatic retry or admission relaxation was added.

[publication-manifest.json](publication-manifest.json) binds every copied capsule member to its exact external original; `SHA256SUMS` covers every publication member except itself. Publication excludes caches, browser profiles and private runtime fixtures. No commit or push was made by this task.

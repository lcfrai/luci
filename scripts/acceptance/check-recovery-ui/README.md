# Controlled check recovery UI witness

This directory publishes the **exact successful browser helper and fixture**, unchanged from the executed witness. Publication runs no browser or tests. The recorded [evidence](../../../docs/luci/evidence/cloud020-check-recovery-ui/README.md) includes the original failed attempts, source manifests and final screenshots.

| File | SHA-256 |
| --- | --- |
| `source-browser.mjs` | `f3f3cf648159801858cc81704dd4f38bb471d262bb200b312009edd2d4044c78` |
| `fixture.tsx` | `dfff76c5eb378031607881a9942bca90548b3cecb067eb54983466243e11347e` |

The fixture mounts actual AppShell, Develop, ProjectWork, ProjectChecks, CheckRecoveryDialog, WorkflowDialog and the pinned shadcn dialog. Only the unrelated ProjectPlan leaf is omitted. Account/team/setup providers and API responses are controlled fixtures; access/draft hooks and navigation are actual source. The synthetic recovery POST log records a component API callback, not HTTP transport or Docker cleanup.

The successful witness completed 34 checkpoints and produced 17 screenshots at 1440×1080 and 390×844. It covers retained failed/unknown output, exact consent, one recovery callback, lost-response readback without replay, read-refusal privacy boundaries, draft retention, scoped navigation, keyboard focus/scrolling and pending dismissal guards. The actual five-second observation timer is checked for four-read bounds, no overlap, controlled hidden-view abort, six-record fairness and natural stop. These are source/fixture observations, not native Windows, installed package, backend, Docker, model, cloud or performance acceptance. Controlled visibility is not an OS background-window test.

## Reproduce with explicit paths

Use Linux, an ordinary user, Node compatible with the repository, the existing locked desktop dependencies, and sandbox-capable Chromium. Install dependencies separately through the repository's normal setup; this helper never installs them. Coordinate an exclusive browser slot and use a fresh output directory for every attempt. Do not run during timing/resource acceptance.

The unchanged helper retains its original external default manifest path. **Set the supported manifest and repository environment variables explicitly**; no script rewrite is required. From the checkout root:

```sh
luci_repo="$(pwd)"
install -d -m 700 /tmp/lc20
LUCI_CLOUD020_BROWSER_RELEASE=1 \
LUCI_CHECK_UI_REPO="$luci_repo" \
LUCI_CLOUD020_MANIFEST="$luci_repo/docs/luci/evidence/cloud020-check-recovery-ui/final-source-manifest.json" \
LUCI_CHROMIUM_EXECUTABLE=/usr/bin/chromium \
LUCI_BROWSER_STATE_ROOT=/tmp/lc20 \
node "$luci_repo/scripts/acceptance/check-recovery-ui/source-browser.mjs" \
  "$luci_repo/artifacts/quality-swarm/check-recovery-ui/reproduction-new"
```

Choose another short, private state directory if `/tmp/lc20` belongs to someone else. Keep the path short: the first retained attempt failed on Chromium's UNIX socket length limit. The runner refuses a pre-existing output directory and refuses source files that differ from the 12-file final manifest. A refusal on changed code is not an old receipt's pass for that code; preserve it and prepare an explicitly identified new witness if needed.

The virtual entry remains `scripts/acceptance/cloud020-check-ui/fixture.tsx`; the Vite plugin loads the adjacent published fixture there so its original relative imports resolve. No virtual fixture is written into the checkout. Vite binds loopback, Chromium's sandbox remains enabled, external browser requests are blocked, and command/agent starts are forbidden fixture callbacks.

Keep the complete output directory. `result.json` records application/helper/fixture hashes, actual browser version, scoped fixture calls, geometry, Axe results, focus observations and cleanup. Failed attempts preserve a failed-stage screenshot where a page exists. The helper closes only its own Chromium and Vite server and removes its own temporary profile. It does not stop unrelated processes.

The recorded successful run's 439 application input hashes and final source identity are retained in [browser attempt 4](../../../docs/luci/evidence/cloud020-check-recovery-ui/browser-attempt-4/result.json). Fresh execution is required for a new outcome; publication and hash checks alone do not rerun that witness.

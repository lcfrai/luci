# LOCAL-015 — retain start uncertainty across authority changes

The original component at `6002b2eef3151ffd02963aa34cf32c71fc9b2755` discarded an in-flight check start response when its display-authority generation changed. This correctly suppressed stale private content, but also discarded the start-outcome barrier. A later empty history response could precede the mutation's settlement, leaving another start eligible without an appropriate subsequent read.

The correction tracks each start attempt independently of display-authority generations within the mounted project/account scope. A usable current-authority receipt or a successful explicit history request **begun after settlement** retires that attempt. Lost or discarded responses retain generic uncertainty without exposing their stale private contents. Dispatch consumes consent; another start needs reconciliation when required and fresh consent. Cancellation/recovery authority and backend admission are unchanged.

## Evidence and limits

- `unfixed.json`: the six new, unchanged regression cases against the original component produced **4 failures and 2 passes**; 45 existing cases were intentionally unselected. Both Edit-churn outcomes failed, as did an early explicit history response and Read revoke/restore. Project/account isolation passed.
- `fixed.json`: **69 passes, 0 failures, 0 skipped/unselected** across ProjectChecks (51), CheckRecoveryDialog (11), and product API contracts (7). This is an affected source batch, not the whole frontend suite. `typecheck.log` and `exits.json` record full TypeScript exit 0.
- `browser-attempt-3/result.json`: **6 focused controlled Chromium checkpoints passed** at 1440×1080 and 390×844, covering both late outcomes, exact subsequent explicit history, fresh keyboard consent, and Alpha-to-Beta isolation. Actual ProjectChecks, useProjectAccess, project draft hooks, shadcn controls and their normal journey stylesheet are loaded. Account/team providers and product API callbacks are controlled. These are not HTTP/backend/Docker/Windows execution receipts.
- The final browser pass records 428 projected source/config inputs before and after, unchanged helper/fixture hashes, zero page errors, scoped Axe WCAG2A/AA+2.1AA violations and horizontal overflow checks, and closed owned Chromium/Vite with its private profile removed. It is not whole-app accessibility or responsiveness certification.
- Component input manifests were reconstructed after completion from the isolated projection and preserved originals; the browser fingerprints are actual pre/post observations. `unfixed-source-reconstruction.json` explicitly retains that difference. Original production and test files are preserved under `original/`.
- `browser-attempt-1` failed before scenario checkpoints because the helper expected a visible button inside a deliberately collapsed disclosure; its staging-only Vite allow list also missed the exact symlinked font dependency path. The corrected helper opens the disclosure and permits the resolved dependency directory. No product edit was needed.
- `browser-attempt-2` passed the same six functional checkpoints but omitted journey.css normally supplied by ProjectWork. Screenshot inspection caught this fixture omission. Attempt 3 adds that stylesheet and repeats the six checks. Counts overlap; they are not 12 independent scenarios.

This closes the controlled reproduction only. It does **not** demonstrate duplicate backend allocation, native Windows correction, provider work, or completion of local product acceptance. Earlier 6002 full-suite and 34-check browser evidence retains its original revision and scope.

## Reproduce against the matching source revision

Use the repository's supported Node runtime and locked dependencies. The tests use no Docker or provider credentials:

```sh
cd desktop
npm exec -- vitest run src/components/journey/ProjectChecks.test.tsx src/components/journey/CheckRecoveryDialog.test.tsx src/api/product.test.ts --maxWorkers=1
npm run typecheck
```

For the original failure, use an isolated copy of the original production source plus the final ProjectChecks.test.tsx and filter with `-t LOCAL-015`. Do not replace production files in a running/shared checkout. The same six cases are in the passing batch.

The exact final helper and fixture are published in `browser-attempt-3/`. On Linux as an ordinary user with sandbox-enabled Chromium, run from the matching repository root in an exclusive browser slot, using a **new** output directory:

```sh
LUCI_LOCAL015_BROWSER_RELEASE=1 \
LUCI_CHECK_UI_REPO="$PWD" \
LUCI_LOCAL015_INPUTS="$PWD/docs/luci/evidence/local015-check-start-uncertainty/fixed-source.json" \
LUCI_CHROMIUM_EXECUTABLE=/usr/bin/chromium \
node docs/luci/evidence/local015-check-start-uncertainty/browser-attempt-3/source-browser.mjs /tmp/luci-local015-fresh-evidence
```

The helper refuses input hash mismatches, reused output directories, root execution, and non-Linux platforms. Its account permissions/API responses are explicit fixture controls; no service or worker is created. Reproducing on changed application source needs a newly reviewed source manifest, not relabelling this receipt.

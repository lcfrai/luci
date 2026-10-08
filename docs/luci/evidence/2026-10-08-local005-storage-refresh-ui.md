# LOCAL005: storage refresh leaves Settings responsive

Storage refresh now reads only the cached storage endpoint. It no longer starts
a comprehensive readiness check or restarts runtime preparation. Unchanged
observations stay within the storage panel. A saved confirmation or changed
storage receipt marks storage readiness **unknown** locally and points to the
explicit **Check readiness** action; a confirmed location alone never marks the
workspace ready. Unsaved projects-folder edits are retained.

Close, Back, Settings sections and onboarding continuation remain usable while
the storage read is pending. Existing identity/request guards still discard
late results. Runtime preparation rejects a readiness response invalidated by a
new storage receipt, including the interval before React renders the new props,
so an older response cannot trigger automatic preparation.

## Source regression

The failing-before check mounted the actual Settings and storage components.
An unchanged cached refresh caused an unintended comprehensive check and
disabled Close, Back and section controls. In onboarding, it repeated the
initial readiness request and disabled Continue. Those outcomes are retained in
the [before-fix report](local005-storage-refresh-ui/before-fix-confirmed-vitest.json)
and [log](local005-storage-refresh-ui/before-fix-confirmed.log), with the
[original product source hashes](local005-storage-refresh-ui/before-fix-source.json).

The corrected source passed **66 focused tests across seven files**, with no
failed, skipped, pending or todo tests. TypeScript passed. Source hashes matched
before and after verification. This is a targeted check, not another complete
frontend baseline.

Earlier attempts remain in the bundle: the initial portal fixture needed the
explicit **On this computer** selection; the corrected before-fix run confirmed
both product failures. Subsequent passing test runs exposed test-only role-query
type errors and a retry-connection lambda returning the new readiness acceptance
boolean. Both were corrected before the final passing TypeScript run.

## Browser witness and reproduction

Linux Chromium rendered the actual Settings, storage and runtime-preparation
components at 1440×1080 and 390×844 with controlled identity and HTTP-shaped
callbacks. It held a storage GET, observed only local loading, used the actual
Appearance and Back controls, and completed the old response after leaving
storage without updating the new view. It also checked confirmation, a later
measurement, and the separate explicit readiness action. Portal refresh kept
the readiness request count at one and left continuation/setup choice usable.

Both widths passed geometry checks and the tested axe rules with zero findings.
The final phone screenshot was visually inspected. The unrelated dependency
installer panel was omitted; no tool execution was simulated as successful.

- [Portable helper and instructions](../../../scripts/acceptance/storage-refresh-ui/README.md)
- [Summary JSON](2026-10-08-local005-storage-refresh-ui.json)
- [Final checks](local005-storage-refresh-ui/verified/result.json),
  [individual test outcomes](local005-storage-refresh-ui/verified/vitest.json),
  [test log](local005-storage-refresh-ui/verified/tests.log), and
  [TypeScript log](local005-storage-refresh-ui/verified/typecheck.log)
- [Browser receipt and source hashes](local005-storage-refresh-ui/browser-verified/result.json)
- [Phone during pending storage refresh](local005-storage-refresh-ui/browser-verified/settings-storage-pending-390.png)
  and [desktop](local005-storage-refresh-ui/browser-verified/settings-storage-pending-1440.png)
- [Original and curated artifact hashes](local005-storage-refresh-ui/artifact-manifest.json)

This verifies the frontend call chain and loading/navigation behavior. It makes
no real management request and runs no Git, Docker, AWS/Azure CLI, worker, model
or cloud operation. It is **not** a native Windows process-start trace or an
installed-package retest. The native report that triggered LOCAL005 and the
separately successful native storage confirmation remain distinct evidence; the
packaged navigation fix still needs its own native witness.

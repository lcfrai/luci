# CLOUD-033: confirmed check cleanup copy

9 October 2026 (Brisbane). **Integrated source correction; controlled tests passed. Updated installed and Windows acceptance remain open.**

The installed dependency run on product `4df94ab9dad0e9fedd28a873bbadc56720e1402e` / helper `d6949ac18e4f72fc2dc286627dc5bddc4067e36d` recorded a passed command with `cleanupConfirmed=true` and no held capacity, but its command text still said cleanup was pending. The existing component rendered that text verbatim. [Original receipt binding and retained check](cloud033-cleanup-message/installed-source-finding.json).

The backend now updates the completed-command cleanup message only after exact acknowledged-container absence and before calculating the new outcome digest. It leaves admission, cancellation, missing-output, authority and capacity rules intact. Historical finished receipt bytes and digests remain unchanged. The component corrects only the four exact historical messages when command status, integer exit, cleanup flag, stage and reason agree; run-level display additionally requires a terminal run with `resourcesHeld=false` and the same proven command. This display change does not mutate stored/API records or evidence eligibility.

Validation used Go 1.26.3, `-race`, `GOMAXPROCS=2`, `-p=1`, plus Node 24.19.0 and one Vitest worker:

- Final corrected Go: **9 top-level tests + 17 subtests passed**, zero failures/skips, across disjoint new and affected groups. The unchanged final regression reproduces four original faulty transitions; its parent failure is not another scenario.
- Original component: four expected legacy-message failures, six passing uncertainty controls, 51 unselected tests. Corrected entire component file: **61 passed**, zero failed/skipped. Final TypeScript check passed.
- Retained initial test-infrastructure failures: the first Go historical comparison mixed a private stored row with its HTTP evidence projection; the correction compares the same projection. The first external-tree typecheck omitted unchanged `e2e/defect6.mjs` support; after copying that original support directory it passed. Neither failure is hidden or credited as a product regression.
- Final source captures bind **917 repository Go/module/testdata/embedded-asset inputs**, with two external workspace files separately bound, and **425 component-tree / 429 typecheck-tree inputs**. External dependency-cache bytes are not independently attested. [Final summary](cloud033-cleanup-message/summary.json), [raw reports and reproduction commands](cloud033-cleanup-message/README.md).

The frozen publication has 69 manifest members (plus the manifest itself), all verified at integration. `artifact-manifest.json` SHA-256: `3cefddb860b1091e0baeb12159bf37abca139f4a85c6b5c9a041e9b010d84068`. Tests ran against unchanged source on base `9121b5bab09df5827efa291026ed842253256842` with the recorded external overlays. These five integrated files exactly match the tested frozen copies:

| File | SHA-256 |
| --- | --- |
| `control-plane/internal/api/check_execution.go` | `8e63303295c66179943c88d29c4c66f10596fbc3de3c8cac601a38b20cb2cf3f` |
| `control-plane/internal/api/check_observation.go` | `04bc6a02192d0fc244a668a11fd14417fbd69cf1d580deefce5e7c406d9b3870` |
| `control-plane/internal/api/project_check_cleanup_message_test.go` | `a7dede32a4892df9123d9b0394ddf95e3fd1a35792a5bcafc9e0a5e3c0e233cc` |
| `desktop/src/components/journey/ProjectChecks.tsx` | `efd6677ccf54a4bd1e4d802d4845e6575a964bd335907b0513baa79d7ae7bcb5` |
| `desktop/src/components/journey/ProjectChecks.test.tsx` | `b9405b3738238da9c8b251708f4df6b98c95f9c7cc88448d8a3673a4e29a78b0` |

The simultaneous `uiSlice.ts` / draft-registration work and check-recovery helper changes are independent. Their combined frontend integration is not established by these earlier copied-tree receipts; the parent integration run covers that composition. No additional Docker, browser, Windows, provider, or sustained-performance acceptance is claimed here. The dedicated Go cache was 341,572,775 bytes at completion and remains separately recorded as task-owned recomputable environment data.

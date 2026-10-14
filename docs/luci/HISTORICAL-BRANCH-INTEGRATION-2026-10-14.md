# Historical branch integration — 14 October 2026

Input main: `cfd311127bc534d2c32db031fb80d59ede6c4e32`. Eleven ordinary two-parent merges preserve all **22 historical remote heads** in the supplied inventory. Eight were already ancestors; feat/env, feat/hub and feat/phase-7-deploy are additionally covered by their salvage descendants. The independently advancing Azure head belongs to the root integration and is excluded here. Later root getting-started docs and setup worker changes join separately.

The current PRODUCT-CONTRACT.md and setup amendment control the product. Historical plans retain their original wording and dates; they do not reactivate KPMG/Panacea infrastructure, override current authority or prove current acceptance. Original source and attribution remain reachable in exact merge parents. No proprietary platform or UNLICENSED canvas code was imported into the active tree. MIT, NOTICE and third-party grants remain unchanged; corporate authorship and an UNLICENSED package declaration do not themselves establish redistribution rights.

The [exact branch/path index](HISTORICAL-BRANCH-DISPOSITIONS-2026-10-14.json) enumerates every incoming changed path (both sides of renames), exact heads, integration commits, conflict paths and modern replacements. History-only source has an explicit replacement/provenance disposition rather than an implicit successful-feature claim. Historical licence allow/exception policies and secret-fixture allowlists remain in history; current distribution licence collection is retained.

| Head | Exact incoming pin | Integration merge | Conflicts | Modern replacement |
| --- | --- | --- | ---: | --- |
| `codex/luci-subscription-e2e` | `802be34d7fa8a7e44fc922583f90a98847a52df8` | `4ebd923006b5bc5b624d27da00b5da7354e27412` | 7 | desktop/src/assets/brand + current product docs/eval drivers |
| `codex/luci-portable-diagnostic` | `efa15a833bf68dec452fc643af2b874582460b8a` | `c34fa5342e91da40ba486359a11af3dc4a435285` | 2 | current .github/workflows/luci-ci.yml + scripts/portable-launcher-eval.mjs |
| `feat/phase-6-gateway` | `b85e9edadaafff0e2a16e52240fabb276388023b` | `65b290b6a49e54dcee768908fe902fbca24c6fad` | 0 | internal/management identity, users, policy, runtime + docs/luci/API.md |
| `feat/work-layer` | `c6047f2984fe37187d95720bfebb142c264f5fc9` | `9b43685f120501c0be3c40daf81c5cdb2ab314b4` | 0 | internal/product/project_runs.go, project_runs_store.go, coordinator_delivery*.go + desktop ProjectWork |
| `salvage/hub-2026-09-28` | `2ffe76a660497c100a5e8785b2c279a21cda17c2` | `279f61b77c6f178c65bb94fec96f8b38bce8b3f8` | 3 | internal/product/context_history.go + internal/management/team_service.go, team_authorization.go |
| `feat/team-mode` | `e3b3e854324e99bd0b3bc41d7787d95f0688a050` | `d5939f270729edb034791d9fa4aaa80e8d6b8226` | 0 | internal/management/teams.go, team_authorization.go, team_service.go; product/coordinator_* + context_history.go |
| `feat/phase-3-git` | `3676625a7b8c04b0f6f6fe8f7f3225e4a0873e81` | `e413e5e3935d9129ebb206523b2f13c70b36d87c` | 2 | internal/product/project_branches.go, files_git.go, files_git_review.go; internal/project/gitboundary.go; internal/worktree |
| `feat/phase-4-discover` | `ca737d9204518add364ebf092ccf08b09ffa4536` | `441c19ce2f5698deb522d73c9fd9ff92d57db95f` | 2 | internal/product/specification.go, journey_context.go, reviewed_context.go, implementation_context.go; desktop definition/DesignWorkspace |
| `feat/phase-5-canvas` | `22675f392b6999c7c1cf07973867195e1f1d7770` | `e064237bfe8a1316331aaf0a88ff4e0f03ee6418` | 9 | desktop/src/components/journey/DesignEditor.tsx Excalidraw; DesignWorkspace; upstream harness/sandbox APIs |
| `salvage/env-2026-09-28` | `55b7d546396ddcb6c2dbc2b771865fbacf1cf2c4` | `0c425d27e4cc81a66b769e3dcd89776c183b7181` | 15 | internal/backend/dockerbe and e2bbe; internal/platform; scripts/azure-embed-host |
| `salvage/phase-7-2026-09-28` | `ffa1e4a8cb0df96b0e76189c0e976b5ee1c5caf0` | `8a6ae331fdf4e61e245bf4ed6a023a8ac13d5302` | 6 | internal/product/coordinator_delivery_deployment.go, cloud_maintenance.go; internal/management/deployment_authority.go |

## Distinct retained work

The historical checkpoint below is `ee307c4347c9f3fc1c2b47eb2dcf9d8715bc7f99`. Its outstanding favicon/native/Rail application was subsequently completed in source by the [bounded Fourfold follow-on](FOURFOLD-APPLICATION-2026-10-14.md). SetupPanel remains owned by the setup worker; its reusable mark is available from `LuciMark.tsx`.

The approved Fourfold source, derivatives, PNGs, recipe, manifests, ZIP and approval/provenance receipts are retained under desktop/src/assets/brand. The canonical SVG is 3,801 bytes with SHA-256 `e69afb839bc6384841c53ebcc1a72953ae1c18b1a1bf672f989d38c42ca9e155`. DESIGN.md and IDENTITY.md describe the approved static mark and current four concurrent activities. No motion or visual redesign is claimed. The remaining application work is concrete: desktop/public/luci.svg still contains the earlier L; desktop/index.html references it; configured native icons in desktop/src-tauri/icons have not been regenerated. Rail.tsx and SetupPanel.tsx remain text-only brand placements. Applying the approved mark and checking native/favicon results is a separate outstanding integration task.

Current native and Docker evaluation drivers gain exact saved-context-to-Git-commit byte assertions while preserving newer reviewed-source admission and failure reporting. The native restart driver recognizes the current Discover navigation and retained historical Define installations. Older approval helpers are preserved in history because they bypass newer required screen/handoff details.

The independent Windows PowerShell startup probe is adapted to the fresh package from the exact clean workflow commit/version, records binary/launcher/package identities and uses a private profile with Windows system paths. Manual CI option `portable_probe` requires `native_windows_only` and runs after normal current-package acceptance. It never starts the app, relaxes normal gates or replays fixed old RC claims. The receipt explicitly says earlier CI steps may warm the host; it cannot establish cold application startup.

## Bounded verification

All 22 historical ancestry assertions passed; all 11 integrations are ordinary merge commits. Three changed scripts passed Node syntax checks; all seven existing native failure-reporting tests passed. Four stale-source/dirty-source/wrong-version/invalid-identity cases refused before probe fixture creation or PowerShell execution. The exact native context helper passed a disposable Git fixture: matching bytes pass, an uncommitted context amendment fails, and its explicit new commit passes. Workflow YAML parsed and current-package probe wiring was checked. Imported text had no private-key/AWS/GitHub/OpenAI-secret-shaped matches in the bounded scan.

The active Go, sandboxd, Terraform, desktop package/native configuration, LICENSE/NOTICE and third-party trees match input main exactly. No broad build, valid-package PowerShell run, native/Docker/UI/model acceptance or GitHub workflow was executed for this history-only consolidation. Root owns coherent final-main checks, Azure/setup integration, publication and the another-machine package handoff.

# CLOUD-075 prepared authority-read coordination

Preparation and validation used an external source overlay only. This worker did not edit tracked repository product files, no application/browser runtime was launched, and no production build or installed-binary verification was performed. The source base is product b94ad37e, copied from a later documentation-only repository HEAD captured in `baseline-git.txt`. The prepared patch has four files: the provider, a small provider-owned pending-read helper, ten new regressions, and three compatibility adjustments to existing provider assertions.

## Observed behavior

Two same-project permission-hook consumers mounted while their shared server response remains pending issue two GETs before the change; the fixed implementation issues one. Source review also finds three consumers mounted on the normal Define route: DefinitionWorkspace, OrchestratorPanel, and the closed TopBar SessionPicker through useCreateSession. This latter three-consumer path is a source observation, not a measured application request count.

The same-body race regression exposed a separate display-authority issue: resolve a previously unpublished grant promise, synchronously dispatch a current workspace-access denial before its promise continuation publishes, then restore membership while the fresh project-grant read remains unresolved. Before the change, the delayed grant becomes a prior-readable projection (`read:true`, all writes false) after recovery; the fixed implementation remains at `noPermissions` until its current read completes. This is a narrowly demonstrated stale display-grant publication race. It does not demonstrate a backend authorization bypass or disclosure of project data.

This finding does not explain CLOUD-032 switching latency. No latency metric was changed, no latency claim is made, and the existing end-to-end threshold remains unresolved.

## Prepared behavior and boundaries

Pending reads belong to one provider instance and are keyed by API route, team-scope generation, team ID, project ID, membership revision and internal invalidation generation. Settled values are removed immediately, so a later subscriber always makes a fresh authority read. Each consumer keeps its own read projection and write gate. Releasing one subscriber leaves other readers intact; releasing the last subscriber aborts its request. Provider replacement, workspace selection, accepted new membership observations and every current authoritative denial invalidate all older pending reads. A `current()` guard also prevents a settled but not yet published response from escaping that invalidation. Blocked workspaces start no new project-permission GETs.

Previously admitted read projections and mounted dirty editors remain retained while membership or project permissions refresh. Writes stay disabled during project revalidation and transient failures. Confirmed 401/403/404 project-read failures continue to remove the read projection. Account, project, revision, provider-instance and StrictMode lifecycle boundaries are covered.

## Validation

All commands used the existing toolchain through `/workspace/.luci-tools/activate.sh`, with one Vitest worker. Exact raw output, JSON reports and exit codes are under `logs/`; `results.json` binds counts and the same-body test SHA-256.

- Before: `node node_modules/vitest/vitest.mjs run src/components/teams/TeamWorkspaceCoalescing.test.tsx --maxWorkers=1 --reporter=default --reporter=json --outputFile=.../logs/before.json` — 8 failing / 2 passing of 10, exit 1.
- Fixed same-body regression: identical test file SHA-256 `2b916bb06468c2ef3b796264d92e06c37177c0df00926699880a2787d7fbdb32` — 10/10 passing, exit 0.
- Initial fixed existing affected tests: Provider, Authority, PortalAuthority, Control — 64/66 passing with 2 exact-argument assertion failures and 1 unhandled rejection. These raw failures are retained. The two assertions expected the old two-argument GET and now require the cancellation signal. The revoked-membership test's deferred rejection was unused because the fixed hook no longer launches permission reads while blocked; its assertion now checks that no such read starts. Actual pre-denial pending and late-response behavior is covered in the unchanged new regression body.
- Corrected existing Provider suite only: 28/28 passing, exit 0. The other three affected suites passed 38/38 in the initial run and were not repeated. Thus the final affected coverage is 76 unique passing tests across the new ten-case suite and four existing suites; this is assembled from these documented runs, not claimed as one clean combined run.
- Full external desktop `node node_modules/typescript/bin/tsc --noEmit` — exit 0.

`dependency-bindings.json` captures the reused lockfile hash and installed package metadata for the selected React, Zustand, Vitest, Vite, jsdom, Testing Library, Radix and TypeScript dependencies; each captured installed version matches the lockfile. There was no package install or claim of a newly reproduced dependency environment. The external copies symlink the already-installed node_modules for reuse; runner-generated ignored caches may use that existing dependency directory.

`changed-source-bindings.json` binds each original/final changed file. `source-inputs.sha256.json` binds the copied tracked desktop inputs. `prepared.patch` is reviewable and applies to the repository source base; originals and final sources are also retained separately. Repository status before/after is recorded to distinguish concurrent parent documentation changes from this external work.

## Integration handoff

After these checks joined, the parent applied the exact four-file `prepared.patch` to the repository and verified every final changed hash plus the other captured desktop source inputs. The parent reported a successful read-only `git apply --check` immediately before applying. This worker's concurrent later check ran against the already-integrated tree and therefore exited 1; `logs/patch-check.*` retain that inapplicable-after-apply attempt and are not a product or validation failure. The parent's exact integrated parity artifact is retained as `integrated-parity.json`. No final production source changes were made after the reviewed and tested overlay.

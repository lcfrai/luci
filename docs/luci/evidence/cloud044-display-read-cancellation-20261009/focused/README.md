# Abandoned project display GET cancellation

This correction cancels outstanding display reads when the user leaves a project, loses Read access or supersedes an orchestrator refresh. It retains the existing generation and scope fences for responses from transports that ignore abort. Reopening a project always gets a fresh controller and current saved context; this introduces no project-data cache.

Only `definitionApi.get` and `orchestratorApi.get` accept the new optional signal. Definition initial/reload reads and `OrchestratorPanel.refresh` supply it. Writes, paid requests, jobs, their ACK/uncertainty tracking, post-write readback and the connection catalogue retain their existing lifecycle. Edit-only changes do not abandon a readable draft. When Read is restored during an outstanding definition save, the fresh baseline waits for that save; unsaved answers remain recoverable explicitly. StrictMode's abandoned initial effect cannot cancel the succeeding effect's controller.

Seven repository files were changed, listed and hash-bound in `result.json`; no documentation, other product sources, backend or runtime helpers were edited by this task. The original external candidate was applied after its base files matched. Only one test syntax correction followed: `.at(-1)` became ordinary indexed access to match the configured TypeScript library. Product code matched the prepared candidate throughout validation.

## Evidence

- The same selected new regressions against the original production implementation failed in **4 cases**; 139 cases were filtered out by name, not executed. These failures establish missing API signal forwarding and absent component display signals, not an installed leak or measured speed cost. Original production files and new test bytes are retained.
- **183/183 affected tests across five files passed**, including 20 newly prepared transport/component cases and the existing affected API/Definition/Orchestrator scenarios. All **722 captured inputs** remained unchanged during that run and its initial typecheck.
- The first TypeScript run returned **exit 2** because a newly authored test used `Array.at`, outside this project's configured library. Its exact diagnostics are preserved. After the test-only indexed-access correction, the single changed post-send-readback case passed and **TypeScript returned 0**, with the corrected 722 inputs stable. That one follow-up overlaps the 183; it is not a 184th distinct test or a rerun of all affected tests on the final assertion syntax.
- The 722-input capture includes all **429 repository frontend/config inputs** plus **293 dependency inputs** listed by the TypeScript compiler. Dependency metadata hashes are retained; their full bytes and all runtime node_modules JavaScript are not published here. The lock and compiler/Vitest configuration are in the repository input set. The full stability capture applies to corrected validation; the original counterexample has the selected changed-file binding verified before the run.

Raw JSON reporters, commands, exit codes, stdout/stderr, original files, prepared candidate bytes, final files and input snapshots are all retrievable. `source.patch` contains the final seven-file delta. The enclosing repository also had parent-owned helper/document changes; this capsule makes no whole-checkout-clean or independently committed-product claim.

No browser performance run, installed app, live provider, Docker, backend workload or native Windows check was executed. A signaled/aborted browser GET does not prove that every server computation was preempted. There is **no p95, throughput or RAM improvement claim**. Correlate abandoned request counts and resource timings with a matched full-app workload before making one.

## Reproduce after integration

Using the supported Node version from the repository's desktop directory:

```sh
node node_modules/vitest/vitest.mjs run --maxWorkers=1 \
  src/api/displayReads.test.ts src/api/orchestrator.test.ts src/api/product.test.ts \
  src/components/definition/DefinitionWorkspace.test.tsx \
  src/components/orchestrator/OrchestratorPanel.test.tsx
node node_modules/typescript/bin/tsc --noEmit
```

The exact recorded commands and filtered follow-up are under `validation/`. Do not run controlled verification concurrently with an exclusive sustained/performance witness.

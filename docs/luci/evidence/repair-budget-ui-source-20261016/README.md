# Automatic-repair-budget desktop source checks

Source: reviewed desktop revision `5c391d11143ff2282beaca0744de89f4482f9df2` joined with `068720c511ce9df080d4e5d0e4782b4925970af9` at `d15a1f0a637df10c7915a0e67185a46f6b69609b`. Exact tested `desktop` tree: `4e1b739bd7acae8a44422626f820fbbe56fc7568`. No backend repair-budget source, runtime, provider or Docker action was part of this check.

Before the test, cgroup memory usage/max was `33,117,229,056 / 34,359,738,368` bytes and host `MemAvailable` was `2,396,016,640` bytes. The first CLI attempt with `--minWorkers=1` exited 1 before running any test because Vitest 5.0.1 does not recognize that option; the effective run used `--maxWorkers=1`.

From `desktop/`:

```sh
timeout 180s env NODE_OPTIONS=--max-old-space-size=512 npm test -- src/api/coordinatorPolicy.test.ts src/api/orchestrationContracts.test.ts src/api/swarmSupervision.test.ts src/components/develop/ExecutionContractReview.test.tsx src/components/develop/SwarmReviewPanel.test.tsx src/components/orchestrator/CoordinatorExecution.test.tsx --pool=threads --maxWorkers=1
```

Exit 0: 6 files, 164 tests passed in 14.94 seconds. No unchanged suites were run.

Before typecheck, cgroup memory usage/max was `33,131,540,480 / 34,359,738,368` bytes and host `MemAvailable` was `1,937,448,960` bytes. From `desktop/`:

```sh
timeout 180s env NODE_OPTIONS=--max-old-space-size=768 npm run typecheck
```

Exit 0: `tsc --noEmit`. This is source and type evidence only. Real coordinator policy/contract/run behavior and constrained-host acceptance remain open; a later backend join may reuse this receipt only while the exact desktop tree is unchanged.

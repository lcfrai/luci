# Integrated C076 v2 + C078 frontend validation

Run only after the root integration/grant. This uses the actual dirty checkout, installed Node24.19.0/shared dependencies, unchanged test configuration and all desktop Vitest tests at one worker, then full `tsc --noEmit`. No installation, compiler target change, filtering or skipped failing test is introduced. Budgets: Vitest900seconds, TypeScript180seconds; each timeout gets10seconds TERM plus10seconds KILL/join. Logs are bounded at32MiB per stream. A source/dependency change, missing reporter, nonzero exit, timeout or log bound fails the receipt. Tests can execute ordinary controlled fixtures; this is not installed/native/browser/Docker/provider acceptance.

```sh
. /workspace/.luci-tools/activate.sh
python3 /workspace/.luci-tools/frontend-integrated-c076v2-c078/run.py --execute-granted --output /workspace/.luci-tools/frontend-integrated-c076v2-c078/runs/attempt-001
```

The output must be unused. The source manifests cover all desktop source/e2e/scripts/config/locks, the actual external Go/docs/task-plan/Dockerfile glob read by `seams.test.ts`, and literal raw imports including golden JSON/native driver. Dirty and untracked inputs are included. Root HEAD and working status are recorded as metadata, never claimed as the exact tested clean commit. Changes to unrelated evidence/docs outside effective input sets are permitted but remain visible in Git metadata. Unexpected `.env*` files refuse capture without reading them. Child processes receive only basic path/locale/home/temp environment plus `CI=1`, `NO_COLOR=1`; no environment values or credentials are dumped. Selected dependency/executable hashes and installed lock are bound, without claiming a full transitive installed-module byte census. Original JSON reporter and logs remain exact private run evidence pending publication review.

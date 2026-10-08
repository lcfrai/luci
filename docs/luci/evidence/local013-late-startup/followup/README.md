# LOCAL-013 DAG continuation follow-up

The merged DAG still had two cancellation gaps: response waiters created before
a failing click could reject unobserved during cleanup, and an in-flight provider
request could resume after cancellation and issue another tool response. This
follow-up changes only `dag.mjs` and adds a focused test file. CLOUD-025 probe code,
shared startup/proxy helpers, cleanup ownership requirements and budgets remain
unchanged.

All three response waiters now attach a rejection observer immediately; callers
still receive their awaited failures. Provider body completion, tool selection
and response writing now check workflow health before proceeding.

The same [exact-wiring tests](../../../../../scripts/acceptance/task-dependencies/dag-lifecycle.test.mjs)
ran against the preserved original DAG and the corrected source:

| Source | Result |
| --- | --- |
| Original merged DAG `8ac2a44a…` | Expected exit 1: six regression failures, two controls passed |
| Corrected DAG | Exit 0: eight passed, zero failed/skipped |

The three orphan-waiter cases execute the actual extracted `action`, `readReview`
and check-consent waiter code in short sequential Node processes with strict
unhandled-rejection behavior. Each preserves the click failure, then lets the
abandoned response reject while simulated cleanup remains active. The original
process exits; the corrected process reaches cleanup settlement.

The provider cases execute the actual HTTP handler, tool builder and response
writer with injected requests/responses. Cancellation while body reading or
bundle observation is pending produces a dispatch in the original source and
none in the corrected source. A direct response-writer case covers an already
built tool response. Healthy dispatch and propagation of an awaited response
failure remain covered by the two passing controls.

The [application/hash record](apply-and-validation.json), [before output](before.log),
[after output](after.log), [run receipt](verification.json),
[preserved original driver](dag-before.mjs.txt) and
[narrow driver patch](dag-boundary-followup.patch) bind these results. Previous
capsules and their hashes were not rewritten. Driver syntax and diff whitespace
checks passed.

From the repository root:

```sh
node --test --test-concurrency=1 scripts/acceptance/task-dependencies/dag-lifecycle.test.mjs
LUCI_DAG_LIFECYCLE_TEST_SOURCE=docs/luci/evidence/local013-late-startup/followup/dag-before.mjs.txt node --test --test-concurrency=1 scripts/acceptance/task-dependencies/dag-lifecycle.test.mjs
```

The second command intentionally fails six regression assertions. These tests
start no browser, Docker container or Luci service. Installed acceptance and the
causes of prior interrupted attempts remain unestablished.

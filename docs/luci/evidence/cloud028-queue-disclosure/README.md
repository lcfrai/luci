# CLOUD-028: sustained queue controls use the actual disclosure

The sustained driver tried to cancel a queued task inside a closed native disclosure, then used an exact label selector that included the priority select's option text. Its next project navigation mounted another closed disclosure. The helper now opens **Queue status and controls** through its visible summary before both actions and locates priority through the scoped, exact accessible combobox name. The product component and all sustained workload, mutation, ownership, allocation, deadline and latency assertions are unchanged.

The changed production acceptance files are `scripts/acceptance/sustained-four-project/sustained.mjs` and its new captured import `queue-controls.mjs`. The helper checks workflow liveness after each await, does not force hidden clicks and leaves an already-open disclosure open. The new import is included in accepted-harness capture and final source rehashing.

## Controlled evidence

One Chromium execution, [attempt1/result.json](attempt1/result.json), completed in 8.309 seconds with six checkpoints and six screenshots:

- At each of 1440 and 390 pixels, the exact original `queueControls` and `action` functions timed out at the hidden Cancel button, with zero writes. Both expected failures and their original source are retained.
- At each width, the exact corrected functions produced one cancel POST for `p-a/q-cancel-a` with revision 2, then one priority POST for `p-b/q-build-b` with revision 2 and `high`. Both actual component mounts began closed. Another run's entries remained filtered out.
- At each width, reopening the disclosure twice left it open without extra writes. Chromium found zero matches for the original exact priority label and one match for the exact accessible combobox.

The fixture mounts the complete actual `SwarmRunQueue`, including its real button/dialog imports, effects and `swarmQueueApi` ownership/receipt validation. Only its HTTP transport and supplied run/queue data are controlled. Actual component files, dependencies and configuration—421 files—matched product `d6eaa540960fb11c6d425104f3da136bf9217712` before and after. The prepared helper checkout was `81f2ba5b194a81a4f2ad167e466793f59162b387`; the corrected driver was external staging during the browser run and the identical bytes were subsequently applied.

This is component/selector evidence, not installed AppShell navigation, backend allocation, Docker, model, sustained performance or whole-product acceptance. The driver's private run/effect readers are controlled in this regression. No allocation claim follows from those fixtures. Both owned Chromium and Vite closed normally. The fixture's unexercised emergency watchdog exits directly; this receipt does not establish cleanup on that failure path. The read-only child review is recorded in [review-scope.json](review-scope.json).

The two applied JavaScript files also passed syntax checks; exact commands and outputs are in [source-syntax-checks.json](source-syntax-checks.json). No broader unchanged suites were rerun. Original failures and executed bytes were not rewritten after the review.

## Reproduce the controlled component regression

Use an exclusive, root-released browser slot with installed `desktop/node_modules`, the existing Chromium and the exact source hashes in `prepared/preparation.json`. The preserved fixture uses the original `/workspace/luci` layout. Copy the capsule to a **fresh** sibling under `/workspace/.luci-tools`; this preserves its relative imports without rewriting helper bytes. The runner refuses an existing result path.

```sh
set -eu
C028_PREPARED=/workspace/.luci-tools/cloud028-component-reproduction
test ! -e "$C028_PREPARED"
cp -R /workspace/luci/docs/luci/evidence/cloud028-queue-disclosure/prepared "$C028_PREPARED"
LUCI_ACCEPTANCE_REPO=/workspace/luci \
  LUCI_CLOUD028_RELEASE=controlled-browser \
  LUCI_CLOUD028_OUT="$C028_PREPARED/attempt1" \
  node "$C028_PREPARED/browser/source-browser.mjs"
```

The release environment value is an explicit operator gate, not proof that another exclusive run has finished. This helper creates no Docker/provider/cloud resources. It retains the two expected old failures separately from unexpected harness failures.

## Sustained witness remains pending

This correction does not establish a successful sustained witness. Its fixed limits remain preparation 120 seconds, scenario 1800 seconds, cleanup 180 seconds, driver total 2100 seconds and observer 2115 seconds. Five Creates/Starts maximum execute serially: four initial 150-second reported fuzz workloads and one short amendment. Worker admission remains 1 GiB/1 CPU/256 PIDs, zero preexisting containers/volumes, 6 GiB pre-create free disk and 1 GiB running reserve. Source-context version 2 and an explicit clean helper revision are required for the d6 product.

The interaction gates remain at least 100 switches/200 inputs in each unloaded phase and 200 switches/400 inputs while loaded; switch p95 ≤300 ms, input p95 ≤100 ms and p99 <1000 ms, including failures, with 15-second action deadlines. Passive event storage remains bounded to 200,000 delivered entries/32 MiB/2000 windows/128 documents, with 10,000 entries per document. No thresholds, polling or resource checks were relaxed.

The separate installed DAG/check attempt and its memory-pressure finding are not resolved by this helper change. An installed sustained run requires its own fresh root admission and final reviewed product/helper/image pins.

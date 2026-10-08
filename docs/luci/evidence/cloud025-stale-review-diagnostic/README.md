# CLOUD-025 stale-review diagnostic

The installed `6002b2ee` attempt 5 failed at the stale-review reason assertion after two actual workers and alpha integration. Its API action recorded HTTP 409, but the response body and after-state were not retained. The original reason remains **unclassified**. This correction repairs helper evidence; it does not establish an integration product defect or complete the DAG acceptance journey.

[Original failure extraction](original-failure-summary.json) binds the unchanged installed report by SHA256. The [full failed installed receipt](../cloud013-installed-6002b2ee-failed/README.md) remains separate. Its cleanup recorded zero containers, volumes, owned service processes and owned browser processes.

The helper now persists `stale-review-probe.json` before sending the one POST, upon observing its bounded response, and after independent state reads. It retains only scrubbed `code` (128 characters), `message` (2048) and `error` (2048), flags truncation, and records request/body failures without inventing a response reason. Each state snapshot retains Create/Start/Destroy counts, primary HEAD, SHA256 of Git porcelain status, private/public run revisions, and integration-checkout existence. Unavailable reads are explicit; independent reads continue. Private revision differences are observations, not an added acceptance rule.

The original HTTP 409, `/review/i` reason, unchanged effects/HEAD/public revision and absent integration-checkout checks remain mandatory. Truncated fields and unavailable after-state also fail; porcelain-status change now fails. A diagnostic is always classified `unclassified-response`; only the caller's separate successful receipt marks the gate passed. There is no automatic mutation retry. The GET after the attempt is bounded to five seconds; the existing POST and Git bounds remain in force.

The 15 focused tests passed, with zero failures/skips/todos. DAG and helper syntax checks also passed. [Raw output](checks/diagnostic1-2.stdout) and the [command/source receipt](checks/diagnostic1.json) preserve the checked bytes. Tests extract the exact staged stale-review block and existing API/body parser, supply local fakes, and verify on-disk evidence before failures escape. They cover expected refusal, unexpected busy/revision 409, wrong status, bounded-field truncation, truncated JSON, over-bound body, effect/HEAD/status/checkout/revision changes, unavailable after-state, lost response without replay, and scrubbing. All tests use synthetic inputs; none identifies attempt 5's unknown reason. No Docker, browser, product service, external provider or network was run for these checks.

Reproduce the frozen source checks from repository root:

```sh
node --check docs/luci/evidence/cloud025-stale-review-diagnostic/source/dag.mjs
node --check docs/luci/evidence/cloud025-stale-review-diagnostic/source/stale-review-probe.mjs
node --test --test-concurrency=1 docs/luci/evidence/cloud025-stale-review-diagnostic/source/stale-review-probe.test.mjs
```

After merging the correction, the focused current-source command is:

```sh
node --test --test-concurrency=1 scripts/acceptance/task-dependencies/stale-review-probe.test.mjs
```

[Source manifest](source-manifest.json) binds the baseline, three staged files and [narrow patch](stale-review-diagnostic.patch). The patch changes the import, helper capture list and stale-review block, plus the new module/test. Merge it with concurrent startup-lifecycle edits; do not overwrite the whole DAG from the snapshot. [Publication manifest](publication-manifest.json) hashes every evidence file except itself.

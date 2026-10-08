# LOCAL-013 sustained helper correction

This is controlled evidence for the affected `sustained-four-project` acceptance
helper. The seven-file candidate was prepared outside the checkout; this evidence
publication changes only this new folder. The installed 30-minute sustained
workflow has **not** run or passed as part of this task. No actual latency,
resource peak, native Windows or real-model quality result is supplied here.

The [result](result.json), [candidate verification](verification.json),
[input manifest](input-manifest.json) and [seven-file patch](candidate.patch)
bind the reviewable helper bytes. The patch requires the separately owned shared
startup/proxy correction. Its final dependency bytes are included under `inputs/`
and identified by SHA-256 below.

| Check | Result | Scope |
| --- | --- | --- |
| First final combined Node batch | 71 passed, 1 failed of 72 | Retained fixture timeout failure; [raw output](controlled-js-final.tap) |
| Corrected fixture batch | 72 passed, 0 failed | [Raw output](controlled-js-final-corrected.tap) |
| Additional actual browser-wiring fixture | 73 passed, 0 failed | Full selected controlled batch; [raw output](controlled-js-ready.tap) |
| Final helper source/location follow-up | 29 passed, 0 failed | 13 lifecycle plus 16 source-binding cases; [raw output](helper-binding-final.tap) |
| Python wrapper | 28 passed, 0 failed/skipped | Mocked subprocess handles and local/in-process fixtures; [source-bound receipt](python-verification.json) |
| Shared dependency owner suite | 51 passed, 0 failed | 15 startup/location, 7 lifecycle/privacy and 29 synthetic proxy cases; [separate result](shared-dependency-verification/result.json) and [output](shared-dependency-verification/node.log) |

These batches overlap and must not be summed. The actual-browser collector was
explicitly excluded by `--test-skip-pattern="actual browser collector"`; this Node
runner did not count that excluded case as selected or skipped. The browser-wiring
case uses controlled handles and promises, without starting a browser. Workload
helper tests inspect generated command contracts and fixtures without compiling
or executing the declared native workload. The wrapper tests mock `Popen`; its
self-cost tests execute tiny temporary scripts in the unittest interpreter.

The first combined failure used a fixture-only outer cleanup timer of 15 ms,
against 5 ms close and 5 ms settle timers. Scheduling delay let the outer fixture
expire before its identity-checked fallback assertion. Only that fixture outer
cap changed to 200 ms. The production 12-second browser phase and every workload,
latency, buffer and resource bound stayed unchanged. The original failed output
remains intact. A later source-location guard added one focused validation case;
the 29-test follow-up overlaps the preceding 73-test batch.

The Python result was observed in the tool transcript with exact wrapper/test
hashes. No standalone raw Python log was saved; that absence is explicit in its
receipt, and no rerun was made solely to manufacture an output file. Earlier
controlled logs and stage verification records are retained alongside the final
ones. Input snapshots were captured after validation; they are not a claimed
complete pre-run/after-run stability manifest for every earlier batch. Historical stage receipts retain the hashes and staging filenames recorded then; some earlier candidate files were subsequently replaced. The final `input-manifest.json` and `artifact-manifest.json` identify the bytes actually retained here.

## Behavior and fixed budgets

The scenario remains **1800 seconds (30 minutes)**. Preparation has **120 seconds**,
and failure diagnostics plus cleanup share **180 seconds**, giving a **2100-second
total driver budget**. The observer budget is 2115 seconds. The wrapper allows
5 seconds after terminating an expired driver and 5 seconds to reap its forced
exit; it grants no second 180-second cleanup period. Observer settlement is
bounded at 30/5/5 seconds and the identity watcher join at 2 seconds.

A durable private incomplete checkpoint records the failing stage, process
identities and unresolved mutation/drain status before screenshots or cleanup
awaits. The bounded shared journal records sanitized stage and resource events.
Independent cleanup attempts cover controllers (30 seconds), proxy (125), browser
(12) and provider (3), within the single overall cleanup deadline. A stuck or
throwing close does not skip later independent phases. Missing browser handles
still reach exact profile/PID-creation-identity fallback.

The actual startup orchestration registers provider, proxy and browser factories
before invoking them, checks workflow state after awaits, and closes late handles
once. A pending proxy cannot count as never started or drained. The controlled
sustained startup case cancels while a proxy factory is pending, completes cleanup,
then returns the handle: it observes zero late configuration writes or controller
spawns, one close, and retained drain/mutation uncertainty. The actual browser
wiring case cancels during timing setup and proves no later route or page creation.
API/socket continuations, service readiness, browser routes, Create and Start
admission are fenced. Intentional service restart remains allowed while healthy.

Worker removal still requires completed controller and drain phases, conclusive
transport state, immutable creation ID, name, instance/session, image and owned
worktree checks. A late result from a timed-out phase cannot grant deletion
authority. Unsettled startup state retains the failed fixture. These fixtures do
not prove that a real Docker workload was drained or removed.

`--expected-source` pins the clean packaged product. `--expected-helper-source`
separately pins the clean helper checkout and defaults to the product revision.
Both revisions and cleanliness are recorded; the helper binding is rechecked at
final verification. The executed driver directory must canonically belong to
that declared checkout. An external staged driver cannot borrow another clean
checkout's HEAD. Sandbox image source comparison remains bound to the packaged
product revision.

## Exact review and dependency hashes

| Artifact | SHA-256 |
| --- | --- |
| Seven-file candidate patch | `0f50ee2da5eed2f70b1cc14c8e77fe38f71a02467191c2d0f965ad92e19b85f3` |
| Shared `acceptance-startup.mjs` | `dd1c60474019edaae223a6298397f5386d4e7504a82b4928a1a906e906e06680` |
| Shared `fenced-docker-proxy.mjs` | `48e97d6d5c915cf80c99f0c79a8fb16261eb219616f878fa1ce103ee2c59303d` |
| Shared `durable-acceptance-journal.mjs` | `b62b3335168e13715ed0839704831f93d224b4f0714d1fbacb7b5bdb596525a1` |
| Unchanged `acceptance-lifecycle.mjs` | `87cc128366cdc5d8cdde3338e0e795ebcdd76533a34703ce9708be22a531c54e` |

The shared owner moved the synchronous Start guard immediately after durable
intent and before request forwarding; its callback contract and startup module
bytes stayed unchanged. The separate 51-test result covers those final shared
bytes. Sustained controlled fixtures mock proxy handles, so their result is not
relabeled as execution of a real final proxy.

## Reproduction

[commands.json](commands.json) preserves the actual validation commands, working
directories and external module override. The final focused command was:

```sh
cd /workspace/.luci-tools/sustained-lifecycle-staging
LUCI_ACCEPTANCE_REPO=/workspace/luci \
LUCI_ACCEPTANCE_STARTUP_TEST_MODULE=/workspace/.luci-tools/late-startup-lifecycle/staged/scripts/acceptance/launch-recovery/acceptance-startup.mjs \
node --test --test-isolation=none --test-reporter=tap \
  scripts/acceptance/sustained-four-project/lifecycle.test.mjs \
  scripts/acceptance/sustained-four-project/source-binding.test.mjs
```

The final candidate and controlled-test inputs are retained under `inputs/`.
After source integration, the helper [README](inputs/scripts/acceptance/sustained-four-project/README.md)
documents normal checkout commands and the separately authorized installed
workload. No reproduction command was rerun during this evidence publication.

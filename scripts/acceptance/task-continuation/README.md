# CLOUD-012 installed continuation acceptance — prepared, not yet run

This source prepares one installed Linux/Docker witness against a **matching clean package** containing CLOUD-011 reports, CLOUD-012 checkpoints and CLOUD-015 truthful integration notices and CLOUD-017 automatic post-start status observation and CLOUD-018 automatic task completion/report observation. It uses the packaged native Codex CLI, real Docker workers and real Git, with a deterministic local provider. Its scripted output establishes workflow mechanics only. It cannot establish live-model quality, Windows execution, concurrent throughput, cloud deployment or completion of the whole product.

Exactly two serial worker starts are authorized. A selected dependent task is deliberately never authorized or started. Each worker is restricted to 1 GiB, 1 CPU and 256 PIDs. The proxy requires zero existing containers/volumes at entry, at least 6 GiB free root storage before each Create, and preserves a hard 1 GiB running reserve. The scenario has a 15-minute deadline; the wrapper allows bounded owned cleanup. No image pulls or release allocation are allowed. Docker fault gates are not used; the only injected fault is a browser reservation response lost **after** a real product HTTP 201.

## Required integration before runtime use

The shared [`worker-create-guard.mjs`](../launch-recovery/worker-create-guard.mjs) supports the explicitly opted-in V2 task-output mount. Default V1 behavior retains the legacy mount shape. Do not replace the shared helper while another accepted run is using its bytes. This driver passes numeric `taskContextVersion: 2`; unknown versions or extra/missing/writable protected mounts remain refused. The guard has its own controlled tests in `../launch-recovery/worker-create-guard.test.mjs`.

The driver verifies the package archive, every installed member, binary/web entry hashes, clean source commit, sandbox image and image-source provenance. It records the exact reused proxy/guard/terminal/observer bytes. Supply an image lineage receipt with the same contract used by the existing DAG/four-project helpers; no new image is built here.

The product binary/web remain bound to the clean package supplied by `--expected-source`. `--expected-helper-source` separately pins the clean acceptance-helper checkout and defaults to the product revision. A helper-only correction may use a distinct explicitly supplied helper revision; both revisions and all executed helper hashes are recorded. The sandbox source comparison still uses the product revision. Dirty or unexpected helper source is refused.

The context check requires the actual public `reviewedContextDelivery` receipt, the private `reviewedContext` reference, and the frozen manifest bytes to agree on version/digest. It also binds both session identities and their approved source/context fields to the exact task operation; a missing or legacy delivery does not pass.

## Workflow and assertions

1. API fixture setup creates one owner, one local scripted provider connection and approved context. The browser prepares a one-worker team and explicitly starts the first task. After each acknowledged manual start, the helper requires the app’s automatic production reconciliation and visible **Recorded as running** state for the exact reserved child before native context inspection and scripted provider release. It captures exactly one real post-start observation response. There is no manual **Check team status** fallback to hide a CLOUD-017 regression; a `dispatching` flag alone is never treated as running. These setup actions are labelled separately from the app interactions under test.
2. Worker one commits `committed.mjs`, leaves `work.mjs` dirty and `note.txt` untracked, runs its exact partial oracle and writes a structured handoff with one blocking question. Its CLI must exit naturally with code zero; there is no success-path DELETE/Stop request. Before any manual status action, GET-only polling must find the exact operation `review-required`, its attributed captured handoff, and the current run in Home attention within 60 seconds. The already-open task card must also update to **Awaiting review**, show enabled review/report controls, and display the question warning within that deadline, before any navigation, reload or click. Its scope must still identify the exact child/workspace. Only then does the browser open Home and require **Review reported task questions** for that build. Separate backend, in-place UI and Home timestamps/screenshots are retained. No manual reconciliation or report collection can manufacture this CLOUD-018 witness.
3. The browser opens all complete checkpoint before/after text, then leaves without consent. It separately reviews the captured report and saves an attributed answer. No new task starts and no integration is inferred from either event.
4. Unauthenticated checkpoint POST is refused with 401. An old review digest is refused with 409 even using the current run revision after the answer. Private operation history, worktree inventory, Docker effects, old source and primary source remain unchanged. This is **not** a managed-grant-revocation acceptance claim.
5. The browser opens and checks every complete refreshed file, then consents once. A page-scoped transport intercept forwards the exact request to the real product, drains its real 201 response, and aborts only the browser reply. Browser state must read back the reservation without automatically replaying the POST or starting a session. A separately labelled, explicit identical-key request returns 200 and the same operation/child/worktree/seed without recopying.
6. The new attempt is distinct, reserved and ready, with a clean seed commit containing all three retained files. The original committed/dirty/untracked state and primary checkout remain unchanged. A stopped-and-restarted controller must preserve that reservation and not allocate its child. Only a subsequent browser `Continue reserved task` starts worker two.
7. Worker two reads its frozen V2 manifest and task packet. `previousAttempt` must be the exact old report/source/context and editor answer, separate from empty integrated dependencies and recorded checks. It verifies the initial seed and every retained byte, finishes the result and commits. It also exits naturally. The same automatic observation check now requires **Task changes to review** on Home, with an exact captured successor report, no invented independent checks and no integration. The answered prior question is retained separately. Both checks run while queue dispatch is paused and without queued work.
8. The dependent task is refused before allocation at each incomplete stage, including after successor exit. Final browser review explicitly integrates the successor. A separate host oracle checks exact resulting files on the accepted primary commit. That oracle is labelled harness evidence, **not** a product-joined verification record. The run is not declared complete because its dependent task remains unstarted.

The original checkout, successor seed, reports/answer and frozen contexts remain in the private fixture for inspection even after a passing run. They contain test credentials and must not be published as a workspace. Cleanup waits for all owned controller processes and proxy requests to drain, then handles only acknowledged, exact, owned immutable container IDs. Any forced cleanup, surviving resource, unexpected third launch, lost proxy observations or unsupported source shape fails the witness. There is no global prune or automatic attempt retry.

## Pure preparation checks

From this directory:

```sh
node --test --test-concurrency=1 ../task-context-delivery.test.mjs helper.test.mjs context-contract.test.mjs checkpoint-contract.test.mjs manual-start.test.mjs automatic-completion.test.mjs
python3 -m unittest run_with_observer_test.py
node --check continuation.mjs
```

These tests run deterministic Node output oracles in fresh scratch directories and mock wrapper subprocesses. They never invoke Git, Docker, a browser, Luci or a compiler. They do not prove that the installed workflow works.

## Deferred installed invocation

Run only in a dedicated Linux acceptance environment with an exclusive, fixture-owned Docker daemon and a matching package plus shared V2 guard. Coordinate the runtime window; do not use a personal/shared Docker daemon. Use fresh evidence paths and a short, roomy fixture base. No command below has been run for this staged helper.

```sh
export LUCI_ACCEPTANCE_REPO=/ABS/LUCI-CHECKOUT
python3 run-with-observer.py \
  --observation-dir /ABS/FRESH-EVIDENCE/continuation-ATTEMPT/observer \
  --fixture-base /SHORT/ROOMY/LOCAL/DIRECTORY \
  -- \
  --binary /ABS/INSTALLED/luci \
  --web /ABS/INSTALLED/web \
  --provenance /ABS/CLEAN-PACKAGE-PROVENANCE.json \
  --expected-source FULL_MATCHING_COMMIT \
  --expected-helper-source FULL_MATCHING_HELPER_COMMIT \
  --image TRUSTED_EXISTING_IMAGE \
  --expected-image-id sha256:EXACT_IMAGE_HASH \
  --image-provenance /ABS/VERIFIED-IMAGE-PROVENANCE.json \
  --report /ABS/FRESH-EVIDENCE/continuation-ATTEMPT/result/result.json
```

The result and independent observer are necessary for an installed claim. Failures stay in their original evidence directories; a corrected attempt uses a new directory and records its new helper/package lineage.

Current UI binding: secondary checkpoint/retirement actions first open the exact task card’s `More task actions` disclosure. These staged drivers target the matching corrected package; the original accepted V1 benchmark helpers are unchanged.

## Windows manual acceptance (separate, not established here)

Do not run this Linux driver, its exclusive-daemon entry checks, or any fixture cleanup against the owner's normal Docker Desktop environment. Use the installed app's ordinary controls in a new disposable project, with the configured provider and a one-worker ceiling. Confirm Windows Docker storage and resource readiness through Settings before starting. Existing projects, containers and volumes stay outside this procedure.

1. Create and approve a task with one dependent task. Use the three retained-file examples in `continuation-fixture.mjs`: one committed partial file, one dirty tracked file, and one untracked note. Ask the first task to produce a handoff with the final-sum question and stop. Confirm its actual changes and stopped state in the app; do not infer completion from a report. Without pressing Check team status, require automatic Awaiting review, a captured task report and the exact build’s attention on Home. After each explicit Start, also require the app’s automatic status observation; record any unconfirmed state without repeating the start.
2. Open Task result and questions, record the answer, then open **More task actions → Continue from retained changes**. Expand and read every before/after file, verify task/source identity, and consent. Confirm a new distinct reserved workspace exists, the old changes remain, the primary is unchanged and the dependent task has not started.
3. Close and reopen the app normally before choosing **Continue reserved task**. Confirm the same seed/attempt survived and no agent started merely because the app reopened. Start explicitly, inspect that the successor received the separately attributed prior report and answer, and verify it preserves the retained files before finishing.
4. Review and integrate the successor's exact changes. Run the project checks through the app and inspect their recorded result. Keep the original attempt available for inspection. Record the package revision, provider/permission route, operation/child IDs, screenshots and observed failures without publishing credentials or private context.

This manual procedure does not reproduce the automated lost-HTTP-reply fault, strict no-allocation negative probes or complete independent observer. Those checks remain unverified on Windows until separately exercised. A failure should retain the existing attempts and evidence; this checklist does not authorize hidden retries, manual ledger edits or Docker-wide cleanup.

## Retained first installed failure

The first attempt on product package `29be4018` created one exact worker but issued no fixture task command. The manual-start adapter waited for a durable operation to change from `dispatching` to `running` without using the existing explicit reconciliation action. Its provider prerequisite deadline expired, and owned cleanup drained to zero containers/volumes while retaining the failed evidence. This was a helper orchestration failure, not a successful product acceptance result. The corrected helper requires CLOUD-017 automatic start observation plus CLOUD-018 automatic completion/report/Home observation and verifies immutable child/operation identity; a new matching product package and fresh evidence are required. The helper does not itself fix the product clarity issue and makes no pass claim for the failed package.

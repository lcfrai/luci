# Prepared sustained mixed-work acceptance

This runner has not passed installed acceptance. Its source and controlled checks are prepared while the integrated candidate is verified. Do not treat helper tests or short standalone workload smoke checks as an installed workflow result.

The runner uses one actual installed Luci service, four private fixture projects, one Chromium browser, one explicitly scripted local Responses provider and at most five real Docker/native Codex workers, serially. The pre-authored workloads implement an escaped HTML renderer, a role-scoped HTTP handler, CSV event aggregation and a dependency-graph library. Each initial task runs a named expected-red test, implements `engine.go`, builds an actual binary, passes ordinary tests and compiled-output checks, then runs 150 seconds of varied-input fuzz testing. A fifth short task handles a reviewed visible-total amendment. It is not a live-model quality benchmark or a complete application deployment test.

The check also requires browser queue consent, one canceled entry without allocation, priority editing, durable unsent drafts, an active-command service restart without replay, automatic native exit/capacity release, a next project's start before prior human integration, unchanged old reviewed context after amendment, and explicit browser review/integration. Every successful worker must naturally exit zero; there is no provider final-reply hold, success-path session stop or hidden allocation retry.

## Measurements and bounds

- A 60-second quiet app-open baseline and a separate 60-second quiet app-open settle phase. Interaction calibrations are separate and are never called quiet idle.
- At least 100 complete project switches and 200 input events in each unloaded calibration, and 200 switches/400 input events while initial workers execute. Each switch includes the actual All projects action and opening the saved Define context. Each input ends at the expected textarea value plus two animation-frame opportunities. Automation, transport and rendering-scheduling costs are included; this is not physical keyboard/display latency.
- Definition edits deliberately remain dirty after text returns to baseline. A real Design → Discard edits and continue → Define action clears each test edit and is measured separately as `draft-reset`. No injected product state or backend save clears drafts.
- Fifteen-second host-side action deadlines. Failed and timed-out actions remain in p50/p95/p99 calculations. Predeclared candidate targets are p95 input ≤100 ms, p95 full switch ≤300 ms, and p99 <1 second. Exceeding them preserves the functional result and reports acceptance failure; thresholds are not adjusted after seeing the run.
- Passive Chromium Event Timing, when supported, is recorded with its 16 ms reporting threshold and rounded durations. Per-round snapshots drain this fixture's observation buffer outside measured action intervals. Monotonic delivered-entry sequences reject gaps, duplicates and drops; the ledger is capped at200,000events/32MiB/2,000windows/128documents and each document buffer stays at10,000events. These checks cover delivered observer entries, not all physical input or events below16ms. Snapshots precede every full document navigation. Missing support stays explicit. The primary timings do not depend on this API. Agent terminal protocol is observed through an authenticated socket; the browser latency samples cover context views, not sustained terminal painting.
- Whole exact-owned process RSS/CPU/I/O and separately verified worker cgroup measurements come from the frozen Linux observer. RSS includes shared-page duplication; worker cgroup memory must not be added to process RSS. Shared Docker/kernel overhead is unassigned. The wrapper records the observer interpreter's own `RUSAGE_SELF`; Docker CLI/daemon overhead is excluded from that self-cost measurement.
- The 30-minute scenario deadline, five Create/Start maximum, one worker, 1 GiB/1 CPU/256 PID worker limit, 6 GiB pre-create disk guard and 1 GiB running reserve are fixed. No image pull or cloud allocation occurs.
- Per-worker scratch is bounded to 512 MiB, including a separate 32 MiB corpus cap, with one-second sampling. Uniform `GOFLAGS=-p=1 -trimpath` avoids duplicate trim-path build profiles; toolchain/modules are offline. These bounds were chosen before any installed run after a real cold API smoke exceeded 384 MiB total scratch. Fuzz command deadline is 330 seconds, including instrumentation; initial task command budget is nine minutes and amendment command budget is 90 seconds. The overall scenario deadline still applies. Before execution, standalone offline smoke took584s for four2s-fuzz workloads; the uniform-trimpath B recheck saved about66s. Replacing the four2s windows with150s estimates1110s before quiet phases, amendment, UI and lifecycle/review overhead. Root therefore approved30minutes before execution, with a33-minute observer window; this is not an after-the-fact extension.
- Actual fuzz command wall time includes cold instrumentation. Restart is asserted during that active command and may occur during instrumentation rather than the mutation loop. Reported fuzz elapsed seconds are rounded. Only reported active fuzz elapsed seconds satisfy the four ×150-second gate; corpus execution counts are fixture-specific, not product/model throughput. Ordinary test names and real exit codes are recorded. Cache peaks are sampled, not exact instantaneous maxima.
- A settled native command failure writes bounded command logs/crashing corpus into a private owned checkout diagnostic, copied to the fresh report before exact container teardown. If the browser/controller fails while the native command is still active, the receipt explicitly records the unavailable failure bundle; teardown may interrupt that command and its temporary corpus is not claimed retained. Failed fixtures and partial measurements are retained. Unknown Docker mutation/drain state prevents destructive cleanup rather than being relabeled as success.

## Reproduction

Requires the root-coordinated exclusive Docker/performance window, zero existing containers/volumes, a clean source-bound extracted package and verified runtime image. Do not run concurrently with other worker tests or compilers. The wrapper refuses existing report/observer paths.

From the checkout, with task tool activation already applied:

```sh
LUCI_ACCEPTANCE_REPO=/workspace/luci python3 /workspace/luci/scripts/acceptance/sustained-four-project/run-with-observer.py \
  --observation-dir /workspace/luci/artifacts/quality-swarm/sustained-mixed/NEW-observer \
  --fixture-base /workspace/.s -- \
  --binary /ABS/VERIFIED-PACKAGE/luci --web /ABS/VERIFIED-PACKAGE/web --task-context-version 2 \
  --provenance /ABS/package-provenance.json --expected-source EXACT40HEX \
  --image EXACT_VERIFIED_TAG --expected-image-id sha256:EXACT64HEX \
  --image-provenance /ABS/runtime-receipt.json \
  --report /workspace/luci/artifacts/quality-swarm/sustained-mixed/NEW/result.json
```

The placeholders are mandatory inputs, not a claim that a candidate is ready. The runner verifies installed package members against the archive and sandbox source hashes against the package commit; moving checkout source does not substitute for that lineage. All executed helpers are captured and rehashed at the end.

Controlled helper checks, with no browser, Docker or compiler (the separate actual-browser collector case is explicitly excluded):

```sh
node --test --test-skip-pattern="actual browser collector" /workspace/luci/scripts/acceptance/sustained-four-project/{event-drain,task-context,helper,workload,latency-diagnostics}.test.mjs /workspace/luci/scripts/acceptance/launch-recovery/worker-create-guard.test.mjs
LUCI_ACCEPTANCE_REPO=/workspace/luci python3 /workspace/luci/scripts/acceptance/sustained-four-project/run_with_observer_test.py
```

After a real run, preserve `result.json`, accepted helper snapshots, package/image receipts, screenshots, `samples.jsonl`, observer metadata/summary and the separate observer-self-cost receipt. The driver's timestamped `phases` identify actual quiet baseline, workload, amendment and settle intervals. The frozen sampler's generic “post-start” phase text does not itself identify quiet idle; join it to this driver's explicit intervals and report missing/partial samples honestly. Historical failure receipts must remain separate.

Live-provider result quality, simultaneous heavy workers, native Windows/WebView latency and other-application responsiveness remain separate release gates.

For pre-C011 packages explicitly select `--task-context-version 1`; the default remains1 and retains the original strict mount/file contract. New task packages require version2 and the reviewed shared V2 create guard. Version is trusted fixture configuration, never inferred from a worker request. V2 requires exactly nine context files, the exact per-child writable report mount, matching manifest/binding digests and an empty direct-dependency packet bound to this independent task attempt. The scripted workload intentionally does not manufacture a worker handoff report; its absence stays explicit and cannot count as successful handoff quality. Actual report and dependency behavior is tested by the separate DAG/checkpoint witnesses.

Additive [latency diagnostics](LATENCY-DIAGNOSTICS.md) retain Node action/verification/frame subspans and bounded browser-clock input/DOM/frame/resource observations. They preserve the original full end-to-end action sequence and100/300ms p95 targets. The added bookkeeping has measurement overhead; the earlier919 baseline remains an uninstrumented-primary comparison with explicit limits, not an isolated product regression estimate. The diagnostics do not measure physical paint or prove React persistence completion.

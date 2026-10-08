# CLOUD-032: prepared latency diagnosis

Prepared externally; no browser, app, Docker, compiler or test execution has occurred for this diagnostic. Original sustained attempt3 remains failed. Product source is unchanged.

The preserved result is `artifacts/quality-swarm/sustained-4df94ab9-helper-5d306b6d-attempt3/result/result.json`, SHA256 `582b60536d93aac1d31b73c675fe46683c5c71042c92e8e38ac28a301b2c40ab`. Product is `4df94ab9dad0e9fedd28a873bbadc56720e1402e`; original helper is `5d306b6dae3cc9d0a4d9cbf95f8142c37958964f`. `original/` preserves its16 captured helper files. `analysis.json` is reproduced by `python3 analyze.py`; it contains only numeric spans, source identities and scope statements, not response bodies or credentials. Seventeen relevant product files are copied from the product commit into `product-source/`; all matched current source at preparation.

| Original phase | Switches | Switch p95 | Inputs | Input p95 |
| --- | ---: | ---: | ---: | ---: |
| Idle | 100 | 523.941ms | 200 | 120.045ms |
| Partial loaded | 74 | 492.327ms | 148 | 82.602ms |

The fixed targets remain switch p95≤300ms, input p95≤100ms and p99<1000ms. The loaded phase did not finish and did not meet its200-switch/400-input minimum. These are Node automation command→expected DOM→two-rAF measurements, including transport and scheduling; they are not physical-input/display timings. All696 retained actions succeeded functionally. No observer cost is subtracted.

For idle switches, separate p95 distributions are action415.868ms, verification126.749ms and final frame RPC37.990ms. The nested All-projects click is206.198ms and Open-project click122.194ms. For inputs they are action73.561ms, verification12.319ms and final frame RPC37.945ms. These quantiles are not additive. Browser input→expected-DOM p95 is43.2ms; last-event references are temporal associations rather than explicit causal IDs.

The browser diagnostic collector reports409.4ms cumulative callback self-time across13 documents, excluding IPC, serialization, clock reads and the separate EventTiming observer. The full Linux observer interpreter consumed80.199CPU seconds over440.749wall seconds (~0.182core), peak RSS19,267,584bytes. That cost excludes subprocesses/daemon work and is not attributable solely to its cgroup reader. No matched no-observer replay exists.

## Source-supported candidates and limits

`AppShell.tsx:153` selects Home versus a project-keyed lens. Each Home mount starts up to four overview reads (`ProjectOverviewBoard.tsx:66`) and a scheduler read (`HostCapacity.tsx:50`). Each completed summary separately updates React state. At `ProjectOverviewBoard.tsx:172`, loading cards grow into variable-height goal, activity, paused-status and saved-run content. The two-column grid has content-sized rows (`project-overview.css:6`), so first-row growth can move lower project buttons. Capacity changes from a loading paragraph to a summary above the heading (`HostCapacity.tsx:102`). These are plausible actionability/layout candidates, not measured causes. The overview CSS has no entrance/position animation. The rail button has a color transition. Draft dialogs have120ms opacity animation; no retained evidence establishes that it delayed the next click.

`DefinitionWorkspace.tsx:117` reads the specification on mount, and Orchestrator reads its project view. Input changes trigger `useProjectDraft.ts` synchronous localStorage/JSON work in a layout effect, unsaved-draft registration, and persisted Zustand writes. Their durations and React render/commit work were not separately measured. The recorded network classes do not cover every request, aborted work or backend admission span.

There is **no original Playwright actionability trace, HAR, target geometry history or per-retry log** in the retained artifact inventory. The original slowest switch includes an Open-project await473.74ms, but that span alone does not identify a stability retry, rendering, backend work or transport delay. Product optimization is not yet justified as a demonstrated latency fix.

The current rail already has a distinct direct path: `ProjectRow.tsx:40` calls `selectProject(project.id)`. `uiSlice.ts:44` restores that project's saved lens and `uiSlice.ts:208` keeps the unsaved-draft guard. It does not mount Home, the four overview cards or HostCapacity. Define/Orchestrator project reads still change. Its latency is untested; it neither replaces nor clears the failed two-click Home journey.

## Run static control first, only in the granted runtime window

The control imports the **exact original** `interaction-metrics.mjs` and `latency-diagnostics.mjs`. It executes100 switches,200 inputs and100 draft resets after the same four-project warmup and60s quiet interval. Ordinary native buttons, textarea and discard dialog are a deliberately minimal DOM fixture, not a fake product acceptance UI. It has no app service, provider, Docker or external requests. One Chromium,1440×1080, existing Playwright and `/usr/bin/chromium`;180s workflow plus20s cleanup watchdog. Per-operation cleanup plus marker/PID/creation-time fallback is explicit; an unsettled resource causes failure, not a cleanup claim. A blocked synchronous kernel operation can delay a JavaScript watchdog.

Use a fresh, nonexistent report parent:

```sh
node /workspace/.luci-tools/cloud032-latency-diagnostic/static/control.mjs \
  --repo /workspace/luci \
  --report /workspace/.luci-tools/cloud032-latency-diagnostic/runs/static-1/result.json \
  --trace-after yes
```

Primary measurements use only the original passive collectors. `--trace-after yes` adds **eight separately labelled rounds after the primary measurements**, with action logs and input boxes but no DOM snapshots, screenshots or source capture. This trace has different instrumentation and cannot explain an unrecorded original action retroactively. No trace is recorded during sign-in in the actual arm. The installed Playwright starts HAR capture only with DOM snapshots enabled; this option disables them. Inspect any trace before publication. The static arm reports full raw interactions, separate summaries, source copies/hashes, Node self-cost and owned cleanup. `completed` means diagnostic completion, not product acceptance.

Both proposed primary arms omit the whole-process Linux sampler and retain the same original browser collectors. Their comparison isolates more of the automation floor versus actual application work, but it is not a matched replay of the old observer load. New timing targets are reported as measured, with no subtraction, relaxed threshold or removed wait.

## Actual installed idle arm, only if needed after static inspection

`prepare-idle.py` does not run the app. It generates `actual/sustained.mjs` using15 exact guarded changes to the preserved driver. `idle-extraction.patch` and `idle-extraction.json` bind every changed block. The actual original interaction/verification/frame helpers remain byte-identical. This avoids an independently maintained replacement of the product setup and interaction sequence.

The generated driver retains package archive/member verification, explicit clean product/dependency checkout identities, fresh private auth/state/project fixtures, four exact source contexts, paused queues, original unsent-draft setup, warmup,60s quiet,100 switches/200 inputs/100 resets and owned cleanup. It stops before dispatch is enabled. Guards reject every Docker Create and Start **before forwarding**, and all zero-effect assertions remain mandatory. The existing proxy requires positive structural budget values; unconditional denial is the authority limit. Cleanup runs controllers→browser→provider→proxy so a slow read drain cannot consume the browser cleanup window. A scripted local provider remains part of the original setup but must receive zero calls. No old auth/private fixture is reused.

The actual arm needs Docker **read-only** readiness/image/inventory access and the trusted preexisting image, but authorizes no containers or model work. It preserves the six-GiB root admission and one-GiB running reserve. Preparation60s+workflow210s+cleanup120s gives390s; together with the static200s budget, the planned pair is at most590s under cooperative operation. No deadline extension on failure. Optional trace adds32 actions after the primary400; total bound432 is explicit.

The driver imports reusable lifecycle/proxy helpers from the declared clean repository. Each is captured and hash-checked separately from the old interaction code. At initial preparation all seven matched attempt3. Root has authorized an upcoming opt-in proxy diagnostic update: after that clean helper pin is finalized, regenerate **a fresh prepared copy** with `--allow-current-proxy`, inspect `current-proxy.patch` and any new imports, and bind its exact helper identity. Any other changed dependency refuses preparation. Do not silently replace this frozen preparation or claim a newer helper is the original5d helper. The currentC031 task-command fix is not exercised by this idle subset; its copied task module remains the original unreachable source. Do not use this derivative for full sustained acceptance.

After a fresh source binding is reviewed, substitute the exact clean dependency commit below (never a moving branch name):

```sh
LUCI_ACCEPTANCE_REPO=/workspace/luci node /workspace/.luci-tools/cloud032-latency-diagnostic/actual/sustained.mjs \
  --task-context-version 2 \
  --binary /tmp/luci-retained-tools/package-0.5.0-4df94ab9/luci \
  --web /tmp/luci-retained-tools/package-0.5.0-4df94ab9/web \
  --provenance /workspace/luci/artifacts/quality-swarm/candidate-4df94ab9/package/package-provenance.json \
  --expected-source 4df94ab9dad0e9fedd28a873bbadc56720e1402e \
  --expected-helper-source EXACT_CLEAN_DEPENDENCY_COMMIT \
  --image luci/sandbox:verified-candidate-20261008t140025z-a4a2e82b \
  --expected-image-id sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98 \
  --image-provenance /workspace/.luci-tools/sandbox-candidate-20261008T140025Z-a4a2e82b/receipt.json \
  --fixture-root /workspace/.c32 \
  --report /workspace/.luci-tools/cloud032-latency-diagnostic/runs/actual-idle-1/result.json \
  --trace-after yes
```

Do not run the copied `run-with-observer.py`; its original sustained scope/budget is preserved as source evidence, not as this diagnostic's runner. No product source change, acceptance rerun or sidebar latency result is included in this preparation.

# Frozen installed idle diagnostic: preparation, launch, and closure

Prepared 2026-10-09 by read-only inspection. This note is the only artifact created for this task. No runner, generated diagnostic copy, package, fixture, process, browser, Docker command, compiler, or test was created or executed. The runtime window remains closed. The commands below are a future procedure for the root agent after packaging and explicit scheduling.

## Current evidence and unresolved inputs

The inspected checkout HEAD was `d52f7fe3d3981342b46391c34ad6517f100ba1be`. All nine local files in the previous diagnostic's binding, and all seven live repository dependencies, matched `/workspace/.luci-tools/cloud032-idle-d348f012/actual/external-source-binding.json` byte-for-byte. The checkout contained root-owned audit edits/untracked files at inspection, so it was **not** eligible for the driver's clean-helper admission check. Do not remove those files to make admission pass. Root must finish its work and provide the final clean product/helper identities.

The new package path, package-provenance path, and exact packaged source commit have not been supplied. They must come from root's completed combined-source packaging receipt. Do not substitute d52, a short hash, or current HEAD for the package identity without checking the actual provenance. The driver separately checks product and helper commits; they may legitimately differ, but both identities must be explicit and clean.

Exact previous successful diagnostic launch metadata:

- `/workspace/.luci-tools/cloud032-idle-d348f012/invocation.json`
- `/workspace/.luci-tools/cloud032-idle-d348f012/wrapper-result.json`
- `/workspace/.luci-tools/cloud032-idle-d348f012/runs/actual-idle-after-1/result.json`
- `/workspace/.luci-tools/cloud032-idle-d348f012/independent-closure.json`

The previous invocation ran `node .../actual/sustained.mjs` directly. Its preserved `run-with-observer.py` is source evidence, **not** the idle launcher. Do not run that wrapper: it changes the scope and introduces the full Linux sampler. No dedicated source for the previous lightweight logging wrapper was found in the inspected directory; its invocation and wrapper-result receipts do not themselves constitute executable wrapper source.

## Freeze the workload and collector

| Identity | Required SHA-256 |
| --- | --- |
| Generated idle driver `actual/sustained.mjs` | `d7fb7586e5b4a2639da9abb6c2e1c6a1296d2161ea7cf2ec393a8f1d3ab1b016` |
| `actual/interaction-metrics.mjs` | `ec0bc9d06fda6636cbba6975e05b9c66979ca189b59aa38c373b37a318b79502` |
| `actual/latency-diagnostics.mjs` | `a2d787493f0eeaeb95570c7f211ee69c649ff1b7247feec171f2118ac3a42c96` |
| Preserved full driver `original/sustained.mjs` | `6d9e4e1a45c778c5d2066738f3b34930a477d6cf8bcadeec10dab5c975792d32` |
| `prepare-idle.py` | `f3562a2440076553693ddd4b285884b052026d2dabcc26ef7436fa42cbf3c519` |
| `idle-extraction.patch` | `dbdb75f33e94de9b7f20b85dc832669498ea4b7cdc7a60ee0dd554d7a0bf71fa` |

The fixed primary sequence is four fresh saved project contexts and paused queues, original unsent-draft isolation exercise, original four-project warmup, 60 seconds quiet, then **100 project switches + 200 inputs + 100 separately reported draft resets = 400 primary actions**. Switches retain All projects → overview-heading wait → Open project → exact loaded `The idea` value → two animation frames. Each input pair remains x then Backspace, with expected DOM verification and two frames. Reset uses the existing Design/discard/Define journey. No direct-rail substitution, removed wait, injected product state, or alternate input collector is permitted for this comparison.

Targets remain project-switch p95 ≤ 300 ms, input p95 ≤ 100 ms, and each p99 strictly < 1000 ms, using the original nearest-rank collector. Do not drop slow/failed actions, subtract sampler/transport/browser time, add a React profiler to the primary sequence, or change thresholds.

The previous d348 invocation used `--trace-after yes`. Preserve that option for the same declared follow-up scope: eight additional rounds, 32 actions, **after** the primary 400; no screenshots, DOM snapshots, or source capture in the action trace. Those 32 rows have phase `idle-actionability-trace-followup`, not `idle-interactive-calibration`. The completed receipt therefore normally has 432 total interaction rows, while primary acceptance remains exactly 400. `--trace-after no` is supported but changes the follow-up scope and must be explicitly recorded; it must never remove primary actions. Raw traces remain private pending inspection.

The driver itself bounds preparation at 60 seconds, workflow at 210 seconds, cleanup at 120 seconds, total at 390 seconds. There is no deadline extension on failure. A blocked synchronous operation can delay a JavaScript timer, so the scheduled root execution monitor remains responsible for detecting an unsettled process. Do not use a shell-wide kill or shortened external timeout that bypasses owned cleanup.

## What changes, and what stays fixed

- **Packaged product:** new installed archive/members, binary/web hashes, provenance path, and exact `--expected-source` supplied by root. This is the behavior being measured. The installed binary and sibling `web/` must come from one verified package; no loose new web directory over an old binary.
- **Dependency checkout:** separately supplied exact clean `--expected-helper-source` matching `/workspace/luci` HEAD throughout the run. The generated driver imports lifecycle/proxy/startup helpers and Playwright from this checkout. Its dependency source hashes remain checked separately from its product identity.
- **External binding:** fresh `executionDirectory`, and consequently binding-file hash, change when prepared in a new external directory. The preserved template identity, extraction changes, nine local source hashes, and seven dependency hashes should match the prior binding for the currently inspected dependency set. Changing the path-bound JSON is expected; changing collector/driver bytes is not.
- **Fixture and evidence:** use fresh paths. The driver creates fresh private home/auth/state/projects/docker directories under a short fixture parent. It refuses a nonempty report directory and retains failed private fixtures for diagnosis. Never reuse the prior fixture, report, or authenticated browser state.
- **Fixed collector:** the generated driver and two timing collectors above remain exact. The embedded `diagnosticPreparedFrom` points to the historical full-driver origin (product `4df94ab9…`, helper `5d306b6d…`); it is not the new product/helper identity and must not be rewritten.
- **Trusted image:** prior image reference `luci/sandbox:verified-candidate-20261008t140025z-a4a2e82b`, ID `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`, provenance `/workspace/.luci-tools/sandbox-candidate-20261008T140025Z-a4a2e82b/receipt.json`. Reuse only if root confirms this remains the intended existing image and its source fence matches the new package commit. No pull or image rebuild is part of this procedure.

Current seven dependency hashes match the d348 binding. The one intentionally allowed difference from the historical full-driver template remains `scripts/acceptance/launch-recovery/fenced-docker-proxy.mjs`: template `48e97d6d5c915cf80c99f0c79a8fb16261eb219616f878fa1ce103ee2c59303d`, accepted current `7315bcd8ecd230bf33de8eb4d06d900e4fdc380b52ed901875b12bf536672dae`. `--allow-current-proxy` acknowledges only that previously reviewed difference. The generator refuses any other changed dependency; if future work changes one, stop and review it rather than updating expected hashes silently.

The driver hard-refuses every Docker Create/Start before forwarding, keeps queues paused, requires zero provider/auxiliary/model calls and zero task execution, and checks zero existing containers/volumes at admission. Positive proxy structural limits (`maxCreates=1`, `maxStarts=1`) are not authority to create/start: unconditional guards remain the controlling limit.

## Future preparation commands — not executed here

Root supplies these shell variables from its final package/commit receipt. They are deliberately not inferred from moving HEAD:

```bash
set -e
source /workspace/.luci-tools/activate.sh
: "${luci_product_commit:?Exact clean packaged 40-hex source commit required}"
: "${luci_helper_commit:?Exact clean helper 40-hex source commit required}"
: "${luci_package_root:?Fresh installed package directory required}"
: "${luci_package_provenance:?Matching package-provenance.json required}"
: "${luci_idle_dir:?Fresh nonexistent external diagnostic directory required}"

test "$(git -C /workspace/luci rev-parse HEAD)" = "$luci_helper_commit"
test -z "$(git -C /workspace/luci status --porcelain)"
test ! -e "$luci_idle_dir"
mkdir -m 700 "$luci_idle_dir"
cp -a /workspace/.luci-tools/cloud032-idle-d348f012/original "$luci_idle_dir/original"
cp /workspace/.luci-tools/cloud032-idle-d348f012/prepare-idle.py "$luci_idle_dir/prepare-idle.py"
python3 -B "$luci_idle_dir/prepare-idle.py" --allow-current-proxy
```

This uses the exact preserved generator; it does not launch the application. `REPO` in that generator is explicitly `/workspace/luci`; do not relocate the dependency checkout silently. Use a new directory for each preparation so previous runs and bindings remain unchanged. Confirm the emitted source hash is the frozen d7fb hash and review the location-specific binding and current-proxy patch.

Verify all generated local source and dependency identities against the frozen prior binding before launch:

```bash
python3 -B - "$luci_idle_dir" <<'PY'
from pathlib import Path
import hashlib, json, sys
root = Path(sys.argv[1]).resolve()
prior_root = Path('/workspace/.luci-tools/cloud032-idle-d348f012')
prior = json.loads((prior_root/'actual/external-source-binding.json').read_text())
current = json.loads((root/'actual/external-source-binding.json').read_text())
assert current['executionDirectory'] == str(root/'actual')
for field in ['templateSourceSHA256', 'localSources', 'repositoryDependencies',
              'dependencyDifferences', 'extractionChanges']:
    assert current[field] == prior[field], ('Unexpected binding change', field)
for group, base in [('localSources', root/'actual'),
                    ('repositoryDependencies', Path('/workspace/luci'))]:
    for name, expected in current[group].items():
        assert hashlib.sha256((base/name).read_bytes()).hexdigest() == expected, name
for name in ['prepare-idle.py', 'idle-extraction.patch']:
    assert (root/name).read_bytes() == (prior_root/name).read_bytes(), name
print(json.dumps({'preparedOnly': True,
                  'driverSHA256': current['localSources']['sustained.mjs'],
                  'interactionSHA256': current['localSources']['interaction-metrics.mjs'],
                  'bindingSHA256': hashlib.sha256((root/'actual/external-source-binding.json').read_bytes()).hexdigest(),
                  'allFrozenLocalSourcesAndDependenciesMatch': True}))
PY

python3 -B /workspace/luci/scripts/acceptance/four-project-queue/verify-package.py \
  "$luci_package_root" "$luci_package_provenance" "$luci_product_commit"
```

The last command verifies the already installed package against the original archive; it does not build/install or run it. Root's packaging workflow must have completed safe extraction beforehand. The diagnostic repeats archive/member verification, product/helper clean-source checks, image/source checks, and binary/web checks internally. Do not bypass any refusal.

Before runtime, save a fresh invocation JSON with the exact argument array below, working directory `/workspace/luci`, `LUCI_ACCEPTANCE_REPO`, product/helper full hashes, driver/collector/binding hashes, the prior fixedScope values, and the final package provenance/hash. The old invocation JSON is a schema/example, not a current receipt. Record this new preparation as prepared-only until launched. Do not put bootstrap tokens, passwords, full environment values, or browser storage in that record.

## Future launch — only in root's granted idle window

Finish and stop all competing compiler/test/build/audit jobs before admission. The current parent's desktop suite/typecheck is not a permitted concurrent measurement load. Root should record the controlled runtime window separately, with no whole-process Linux sampler.

Ensure the report parent below is nonexistent (or empty); keep stdout/stderr/invocation outside it. The same short fixture parent `/workspace/.c32` is appropriate; the driver generates a fresh private child.

```bash
set -e
source /workspace/.luci-tools/activate.sh
cd /workspace/luci
test "$(git rev-parse HEAD)" = "$luci_helper_commit"
test -z "$(git status --porcelain)"
test ! -e "$luci_idle_dir/runs/actual-idle-1"

if LUCI_ACCEPTANCE_REPO=/workspace/luci node "$luci_idle_dir/actual/sustained.mjs" \
  --task-context-version 2 \
  --binary "$luci_package_root/luci" \
  --web "$luci_package_root/web" \
  --provenance "$luci_package_provenance" \
  --expected-source "$luci_product_commit" \
  --expected-helper-source "$luci_helper_commit" \
  --image luci/sandbox:verified-candidate-20261008t140025z-a4a2e82b \
  --expected-image-id sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98 \
  --image-provenance /workspace/.luci-tools/sandbox-candidate-20261008T140025Z-a4a2e82b/receipt.json \
  --fixture-root /workspace/.c32 \
  --report "$luci_idle_dir/runs/actual-idle-1/result.json" \
  --trace-after yes \
  > "$luci_idle_dir/driver.stdout.log" 2> "$luci_idle_dir/driver.stderr.log"; then
  luci_idle_exit=0
else
  luci_idle_exit=$?
fi
```

Run the launch command through root's normal nonblocking command/session tool, allowing short output waits and progress communication while it runs. Preserve the actual driver exit status as `luci_idle_exit` in the wrapper result along with start/end timestamps and elapsed time, complete the closure record, then return `exit "$luci_idle_exit"` from the launch wrapper. The explicit conditional records nonzero exits even under `set -e`. Do not transform failure to success with `|| true`. Do not change parent process environment `HOME`: the driver gives the installed app its own private home internally.

Chromium remains headless, `/usr/bin/chromium`, 1440×1080, service workers blocked, and external browser origins refused. It imports the current installed Playwright from `/workspace/luci/desktop/node_modules/playwright/index.mjs`; record browser version from the receipt and dependency versions as additional execution identity, without changing the collector. Do not attach an extra browser or Vite instance.

## Closure and interpretation

Wait for the driver's final result and process exit before scheduling other runtime work. Normal cleanup is controllers → browser → provider → proxy, with bounded owned-process cleanup and transport drain. The driver checks service/browser identity using PID plus process creation time and private fixture markers; a PID count alone is not equivalent. A forced termination or unsettled operation remains a failed diagnostic even if later resources disappear.

Inspect (without printing credentials or raw private fixtures):

- Final `result.json`, `result.json.incomplete.json` if present, and `diagnostic-journal.jsonl`. The driver writes a checkpoint before cleanup even on success, so checkpoint **presence alone** does not prove incomplete final closure; inspect the final receipt and process result.
- `originalReceiptComplete=true`, `idleDiagnosticCompleted=true`, zero browser errors, empty `cleanup`, `workflowOperationUnsettled=false`, and successful cleanup phases.
- `controllersStopped`, `dockerRequestsDrained`, and `dockerMutationStateCertain` all true; all startup resource states closed without close timeouts.
- `runtimeCleanup` zero containers, volumes, owned service processes, owned browser processes, with `protectedImagePackageAndHelpersUnchanged=true`; `fixtureRemoved=true` on success. If a fixture is retained, treat it as private failed evidence and never reuse it.
- `providerCalls=0`, `auxiliaryCalls=0`, `actualTaskCount=0`, no worker/task children, no successful Docker Create/Start effects, and no proxy failure/drop. The proxy still performs required image/readiness/inventory reads.
- `sourceCommit` equals the declared product commit and `sourceDirty=false`; `harnessCheckout` equals `finalHarnessCheckout`, matching the exact helper commit and clean state. Verify captured accepted-harness hashes against the preparation binding.
- `idle-interactive-calibration` contains exactly 100 successful project-switch, 200 successful input, and 100 successful draft-reset rows. The optional 32 trace rows stay separate. Evaluate p95/p99 from the fixed primary phase, preserving failures and all samples.

Root should record a separate read-only daemon/source closure after the driver finishes:

```bash
source /workspace/.luci-tools/activate.sh
docker ps -aq --no-trunc
docker volume ls -q
docker image inspect luci/sandbox:verified-candidate-20261008t140025z-a4a2e82b --format '{{.Id}}'
git -C /workspace/luci rev-parse HEAD
git -C /workspace/luci status --porcelain
```

Capture these outcomes with timestamp in `independent-closure.json`, retaining the explicit scope limitation: daemon/source inventory does not independently prove process-specific closure; that claim also requires the driver's owned-process receipt/journal. If a process is unsettled, use only the proven run-owned identities and creation times for subsequent investigation/cleanup. Do not run `pkill`, kill unrelated PIDs, delete failed fixture trees, or prune Docker globally.

Finally preserve exact invocation, wrapper exit/times, generated binding, accepted-harness copies, package/image provenance, raw result, log hashes, and independent closure. Keep the raw action trace/private failure data outside public evidence until reviewed. Compare against d348's original 400-action result without rewriting it. A final `passed:true` means the diagnostic and closure completed; only `targetComparison` plus the primary data establish whether the unchanged latency targets passed. This idle subset never establishes full sustained, model-quality, cloud, or release acceptance.

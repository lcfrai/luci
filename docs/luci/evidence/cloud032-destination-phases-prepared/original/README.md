# CLOUD-032 destination phases — frozen preparation

This prepares one source-bound full100 diagnostic from the exact completed Home diagnostic. Product577, helper648, four saved contexts, 100 switches/200 inputs/100 resets, every original await/action, saved-value assertion, two-frame completion, deadline, quota and 300/100 ms references remain. It is not an optimization, polling measurement, gain claim or acceptance run. Existing plain577 switch p95 497.324950 ms remains failed; Home full100 300.444424/73.000268 ms remains diagnostic.

Only this ignored external directory is changed. `source-basis.json`, `actual/external-source-binding.json`, `overlay-binding.json`, and `dependency-bindings.json` identify the exact inputs. The old full100 and Home directories, original private result, package, source checkout and dependency libraries remain untouched. `helper.patch` and `destination-overlay.patch` show incremental changes; `overlay.patch` reconstructs all eight overlay files against immutable577 frontend inputs. `operator.patch` records launcher/build changes.

The original `waitFor → evaluate → inputValue → two frames` sequence stays intact. Six synchronous Node markers surround its three awaits; the original outer span still includes them. The existing evaluate RPC now records entry, first connected/value sample, sample count and the original expected-value witness. It retains the immediate connected/value predicate, rAF retries, ten-second inner deadline, fifteen-second action deadline, expected-DOM marker and final inputValue equality. There are no extra measured RPCs, waits, sleeps or selectors.

The gated product probe binds each measured Home→Discover navigation to an action ordinal and target project in memory. A read token captures that target and actual read generation before the unchanged API await. Acceptance is recorded only after the original alive/permission/abort/generation guard. The first layout effect with the exact accepted specification object, revisions, purpose, current generation, connected textarea ref and matching DOM value records the commit. No state or storage writes are added. IDs, text, tokens, response objects and element references never enter snapshots; completed witnesses release their private bindings. Lens reset reads are ignored after completion. Entry and negative samples are allowed before response acceptance or commit; the probe does not assume readiness.

Destination rows have numeric sequence/action/read/kind/at fields and optional connected/match/sample-count flags. Kinds are1 Home navigation,2 target selection,3 read begin,4 guarded response accepted,5 first matching layout commit,6 evaluate entry,7 first sample and8 original value witness. There are exactly800 rows for100 complete actions, with a2048-row total cap,100-action/read caps and2048-sample cap per action. Snapshot drains rows but retains counters. Missing/overlapping/stale/foreign/overflowed observations invalidate completeness; diagnostic failures do not escape into product behavior. The original Home probe is byte-identical and still requires1200 rows/84 raw completed-start retirements. Only numeric, bounded data should later be projected for publication; private raw reports remain private.

A matching DOM-value layout commit is not a CSS-visibility, physical-paint or response-transfer completion claim. New browser-clock spans separate accepted response→commit→evaluate entry/witness; Node-clock spans locate each await. Never subtract across clocks, add overlapping spans, subtract instrumentation overhead or rename automation delay as polling. Exact Playwright1.63.0 library entry/bundle hashes are now bound, but no selector-attempt instrumentation is added. Any continued selector/RPC interval still includes resolution, transport and scheduling.

`tests/contract-test.mjs` contains27 controlled contracts using the pinned TypeScript transpiler and NodeVM with fake DOM/clock/RAF. They cover pre-read/pre-commit entry, exact and adversarial identity/value/generation/ref matching, private-field exclusion, bounds/gaps/counters, original immediate/rAF/timeout oracle, full original mock action/await order, unchanged inputValue assertion and two-frame completion. These controls cannot establish actual React/browser timing. The wrapper freezes every attempted input and retains stdout/stderr/result, including failures. A successful receipt tied to this exact preparation is required before build or launch.

Controlled checks use one Node child,512 MiB heap,20-second wall limit plus5-second TERM/5-second KILL waits. They start no app, browser, compiler process or package, write no dependencies, and do not require the6 GiB runtime floor:

```bash
PATH=/workspace/.luci-tools/bin:$PATH python3 /workspace/.luci-tools/cloud032-destination-phases/run-contract-check.py \
  --output /workspace/.luci-tools/cloud032-destination-phases/contract-check
```

After a separately admitted build window with at least6 GiB root free space, build only the diagnostic frontend:

```bash
PATH=/workspace/.luci-tools/bin:$PATH python3 /workspace/.luci-tools/cloud032-destination-phases/build-web.py \
  --contract-receipt /workspace/.luci-tools/cloud032-destination-phases/contract-check/result.json
```

This freezes tracked577 frontend inputs plus the overlay, links unchanged existing node_modules, runs TypeScript noEmit then Vite sequentially, and creates a new web-member binding. Each has a180-second bound,3 GiB Node heap andGOMAXPROCS2. Earlier equivalent source+web occupied approximately41.1 MB (18.1 MB source/23.0 MB web); previous TS/Vite times were22.1/3.85 seconds, not a forecast guarantee. Expect additional bounded logs/receipts. No backend/package/image build, dependency install/download, model call or container is needed. New full100 runtime outputs previously occupied about8 MB; retaining failed fixtures can require more. The selected dependency hashes are not a complete archived toolchain/browser environment.

Only after successful build and a separately admitted exclusive Linux runtime window, retaining the unchanged6 GiB root floor and empty-container/volume gate, launch:

```bash
source /workspace/.luci-tools/activate.sh
python3 /workspace/.luci-tools/cloud032-destination-phases/launch.py \
  --contract-receipt /workspace/.luci-tools/cloud032-destination-phases/contract-check/result.json \
  --source 577f7a0fdb039b7f875901a6a567d62fdfee88c8 \
  --helper-source 64829123a5799858b57616b711c84c61b945871a \
  --package /workspace/.luci-tools/package-577f7a0f/installed \
  --provenance /workspace/.luci-tools/package-577f7a0f/verified/integrity.json \
  --image sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8 \
  --expected-image-id sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8 \
  --image-provenance /workspace/.luci-tools/cloud032-long-frame-29931afc/image-provenance.json \
  --output /workspace/.luci-tools/cloud032-destination-phases/run-001 \
  --fixture-root /workspace/.c32-destination-577-a1 \
  --trace-after no
```

Launch reuses the old installed binary and one owned browser with existing lifecycle helpers and zero Create/Start guards; preparation/workflow/cleanup60/210/120 seconds, outer450/30/10 seconds,60-second quiet interval, Node24.19.0/3 GiB heap andGOMAXPROCS2 stay fixed. It preserves failed evidence and records observed browser identities, not an exhaustive lifetime census or zero-zombie claim. Independent closure remains a separate required observation. Build/run have not been executed by preparation. A later runtime must report complete raw outcomes and numeric projection without rewriting old evidence or treating capture success as performance acceptance.

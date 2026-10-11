# Eight-round Home phase diagnostic — staged, not executed

Purpose: locate a measurable synchronous Home/navigation/persistence cost or a delay between the real Home handler and the AppShell layout-effect commit. No product optimization is included. The retained 972d 400.974902 ms switch-p95 failure remains unchanged; this eight-round instrumented run cannot satisfy or relax the 300 ms acceptance threshold.

The binary is the unmodified installed package at product commit `577f7a0fdb039b7f875901a6a567d62fdfee88c8`. `build-web.py` extracts tracked frontend inputs from that immutable commit into this external directory and applies the six-file numeric probe overlay. No mutable backend, docs, scripts or frontend source enters the build. Shared installed dependencies are reused and selected dependency bytes/lockfile are bound before and after. The installed package and its web stay unchanged; the separately built web is explicitly declared diagnostic and all its members are checked before and after the browser run.

The seven reused lifecycle/Docker helpers are frozen copies of `64829123a5799858b57616b711c84c61b945871a`; the ten local helpers derive from the retained canonical idle driver and the exact adaptation is `driver.patch`. All 17 files are hashed. The working checkout HEAD/status is recorded for context and is not required clean: unrelated API edits are not inputs. The original Docker image provenance/source checks, exclusive empty containers/volumes, 6 GiB root floor, immutable image ID, hard zero Create/Start forwarding guards, 60-second quiet phase, controller/browser/proxy settlement, and private failed-fixture retention remain.

Run only from the root agent's exclusive runtime window. These commands have NOT been executed. The initial static check only parsed Python and the two modified JavaScript files. The typecheck and Vite build below must pass before the run. Both have individual 180-second wall bounds, a 3 GiB Node heap limit and GOMAXPROCS=2. Build attempt directories must be fresh; do not erase a failed attempt to replay it. The driver preserves preparation/workflow/cleanup bounds of 60/210/120 seconds and its outer exact-PID watchdog of 450 + 30 + 10 seconds. No worker, image pull or installer is authorized by these helpers.

```bash
source /workspace/.luci-tools/activate.sh
python3 /workspace/.luci-tools/cloud032-home-phase-9536e417/build-web.py
python3 /workspace/.luci-tools/cloud032-home-phase-9536e417/launch.py \
  --source 577f7a0fdb039b7f875901a6a567d62fdfee88c8 \
  --helper-source 64829123a5799858b57616b711c84c61b945871a \
  --package /workspace/.luci-tools/package-577f7a0f/installed \
  --provenance /workspace/.luci-tools/package-577f7a0f/verified/integrity.json \
  --image sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8 \
  --expected-image-id sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8 \
  --image-provenance /workspace/.luci-tools/cloud032-long-frame-29931afc/image-provenance.json \
  --output /workspace/.luci-tools/cloud032-home-phase-9536e417/run-001 \
  --fixture-root /workspace/.c32-home-phase-a1 \
  --trace-after no
python3 /workspace/.luci-tools/cloud032-home-phase-9536e417/analyze.py \
  /workspace/.luci-tools/cloud032-home-phase-9536e417/run-001/driver/result.json \
  /workspace/.luci-tools/cloud032-home-phase-9536e417/run-001/numeric-projection.json
```

The driver arms probes once, after original saved-context/draft/warm preparation and the 60-second idle period. It runs eight original rounds: 8 switches, 16 inputs and 8 discard/reset actions. Each original action retains its Node spans, saved-value DOM assertion and two animation-frame completion; the added data drains through the existing between-round observation RPC. There is no generic trace, new polling loop or RPC inside a measured action. The probe is in the real Rail/AppShell/store path. All added browser rows contain numbers only; no project identifiers, storage keys, values, draft contents, request text or credentials. Original raw driver evidence remains private and may contain fixture details. `analyze.py` emits an allowlisted projection only.

Numeric kinds: 1 handler duration; 2 handler start → Home layout effect; 3 navigation-state calculation; 4 complete scoped-navigation write; 5 navigation encode/sort/JSON; 6 existing scope scan/parse/eviction; 7 navigation setItem; 8 cockpit JSON-storage serialization/write; 9 cockpit partialize; 10 localStorage key count; 11 other navigation-scope count; 12 AppShell render entry → Home layout effect. Kinds 8/9 may occur more than once in the handler-to-commit interval; counts must match and remain explicit. Home ordinal 1–8 joins switch ordinal 1–8. Missing commits, gaps, overflow or operation errors invalidate complete capture. Pending/active state and partial windows remain in failure evidence.

Interpret direct spans: if the measured handler or scan/write spans themselves consume substantial time, inspect that exact work next and propose a behavior-preserving change with draft/cross-scope persistence regressions. If they are small and the All projects command remains long, these spans do not establish a storage cause; retained driver and browser observations should determine the next narrower diagnostic. Render-entry-to-layout-effect includes React scheduling/commit and is not isolated render CPU time. Nested durations overlap and must not be summed. Do not subtract instrumentation or driver overhead from results, or subtract values across Node/browser clocks. Eight-round quantiles remain descriptive only. `passed` means complete capture and cleanup, not benchmark acceptance.

Rollback is stopping only this run's exact owned processes through the retained lifecycle and keeping failed private fixtures/evidence. No tracked checkout files, old helpers, packages, fixtures or ledgers are modified. The external overlay need not be installed or copied into the repository. Root still owns the independent post-run closure and any publication review.

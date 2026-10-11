# Full original idle sample on packaged577 — prepared, not executed

The completed eight-round probe measured Home handler ≤2.1 ms, handler→Home commit ≤9.7 ms, scoped navigation write ≤1 ms, scope scan ≤0.2 ms and cockpit serialization/write ≤0.2 ms. It supplies no justification for a storage optimization. One switch took337.12 ms and seven took184.81–255.67 ms; that sample cannot decide the100-switch p95 gate. No automation-floor or causal bottleneck claim follows. Exact projection hashes and measured maxima are retained in `preceding-diagnostic-finding.json`.

Next decisive measurement: all100 original switches,200 inputs and100 draft resets on the original packaged577 binary AND packaged web. No frontend build or Home overlay is used. No first-round exclusion, additional warmup, generic trace, metric subtraction or threshold change. Exact same four saved-context/draft/paused-queue setup,60-second idle, original command spans, expected saved-value verification and two animation frames. Original thresholds: switch p95≤300 ms, input p95≤100 ms, each p99<1000 ms. Original raw972d failure is preserved separately.

Root-only execution during the exclusive runtime window:

```bash
source /workspace/.luci-tools/activate.sh
python3 /workspace/.luci-tools/cloud032-idle-577f7a0f-plain/launch.py \
  --source 577f7a0fdb039b7f875901a6a567d62fdfee88c8 \
  --helper-source 64829123a5799858b57616b711c84c61b945871a \
  --package /workspace/.luci-tools/package-577f7a0f/installed \
  --provenance /workspace/.luci-tools/package-577f7a0f/verified/integrity.json \
  --image sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8 \
  --expected-image-id sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8 \
  --image-provenance /workspace/.luci-tools/cloud032-long-frame-29931afc/image-provenance.json \
  --output /workspace/.luci-tools/cloud032-idle-577f7a0f-plain/run-001 \
  --fixture-root /workspace/.c32-idle-577-a1 \
  --trace-after no
```

The original idle driver and exact adaptations are bound by `actual/external-source-binding.json`, `driver.patch` and `prepared-files.json`. Seventeen local helper copies are bound; seven lifecycle dependencies are additionally stored in original directory layout and verified against immutable648 Git objects. The unchanged package binary, archive, extracted members, packaged index and106 image runtime source inputs retain original checks. Mutable checkout HEAD/status is recorded separately and need not be clean; mutable API/frontend sources are not executed. No installs, builds or runtime occurred during preparation.

Admission and bounds remain: non-root Linux, Node24.19.0, GOMAXPROCS=2,3 GiB Node heap, exclusive empty containers/volumes, exact existing image/engine,6 GiB root floor; hard zero Create/Start forwarding guards; preparation/workflow/cleanup60/210/120 seconds, outer driver watchdog450 + TERM30 + KILL10 seconds. Only the owned driver is signaled by the wrapper. Original resource cleanup remains controller→browser→provider→proxy with exact fixture ownership. A durable failure checkpoint precedes cleanup. All400 raw interaction rows and browser/Node spans stay in the private receipt; no old fixtures, ledgers, packages or failed evidence are modified. Failed or partial fixture evidence remains private.

A small additional receipt uses the existing fixture-owned process discovery only after browser startup and immediately before cleanup, outside measured actions. `owned-browser-identities.json` retains numeric PID/start-tick/process-group identities and final exact absent/reused/zombie/present observations. It explicitly is not an exhaustive browser lifetime census. The existing active-process cleanup check excludes zombies; this new receipt does not claim zero zombies or change cleanup authority. Root owns independent closure.

As in the trusted original helper, `passed`/exit0 mean complete capture and cleanup. Idle performance is separately determined by `targetComparison` and `idlePerformanceTargetsMet`; a completed capture with a failed target remains a failed performance gate. No full sustained-worker, live-provider or Windows acceptance claim follows, even if all idle targets pass.

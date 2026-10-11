# Full100 Home numeric diagnostic — prepared, not run

This repeats the approved numeric Home observation across all100 original switches/200 inputs/100 resets. It reuses the exact previously built577 diagnostic web byte-for-byte through the declared `web` directory alias. No new frontend/product code or web build. Original300 ms switch-p95,100 ms input-p95 and strict<1000 ms p99 references,60-second quiet period, saved-value assertions, two-frame completion, per-action Node/browser rows and all existing guards/cleanup remain. Diagnostic results cannot constitute plain packaged-web acceptance or justify subtracting probe overhead.

The small probe's existing16-entry start map increments its generic `dropped` counter when retiring completed old entries. This full100 capture therefore requires exactly84 retirements, while independently requiring all100 complete phase sets, contiguous sequence numbers, successful operations, no active/pending Home and matched persistence counts. Every snapshot must satisfy `dropped=max(0,homes−16)`; raw counters are preserved. Any additional overflow/overlap/gap refuses complete capture. The tiny source-bound counter test passed four cases:17th completed Home retains all12 rows while retiring one map entry; full100 retains1200 rows and84 retirements; missing rows/extra drop refuse validation; an actual overlapping unfinished Home exceeds the allowed counter. It transpiled the exact original probe module in Node only; no browser or web build ran.

The plain helper's browser identity receipts are copied outside measured actions: observed startup/pre-cleanup PID/start identities and exact final absent/reused/zombie/present classifications. This is not an exhaustive lifetime census and does not claim zero zombies.

Root-only command, after exclusive runtime admission:

```bash
source /workspace/.luci-tools/activate.sh
python3 /workspace/.luci-tools/cloud032-home-phase-577-full100/launch.py \
  --source 577f7a0fdb039b7f875901a6a567d62fdfee88c8 \
  --helper-source 64829123a5799858b57616b711c84c61b945871a \
  --package /workspace/.luci-tools/package-577f7a0f/installed \
  --provenance /workspace/.luci-tools/package-577f7a0f/verified/integrity.json \
  --image sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8 \
  --expected-image-id sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8 \
  --image-provenance /workspace/.luci-tools/cloud032-long-frame-29931afc/image-provenance.json \
  --output /workspace/.luci-tools/cloud032-home-phase-577-full100/run-001 \
  --fixture-root /workspace/.c32-home-577-full100-a1 \
  --trace-after no
```

The launcher binds all prepared inputs, complete existing diagnostic web members, immutable original577 package binary/archive/members, immutable648 frozen helper copies and existing image/source provenance. Mutable current API source is recorded separately and not an input. Node24.19.0,3 GiB heap,GOMAXPROCS2, exclusive empty Docker containers/volumes and6 GiB root floor remain. Hard zero Create/Start forwarding guards and external-origin blocking remain unchanged. Preparation/workflow/cleanup60/210/120 seconds and outer450+30+10 seconds remain. Durable failure receipt and exact ownership cleanup retain failed evidence; old fixtures/packages/ledgers/source are untouched. Root owns launch and independent closure. `passed` means complete diagnostic capture/cleanup, not a performance gate pass.

# Prepared matching installed idle diagnostic — not executed

Preparation only. No app, browser, Docker command, test, compiler, or runtime workload was launched while producing this directory. `prepare-idle.py` only reads/copies/hashes source and writes the extracted external helper.

`original/` freezes the current 17 selected repository helper files. The generator follows the earlier `cloud032-idle-2da1a821` extraction, but uses the current driver and current dependencies (including receipt-transport). It replaces the full-work tail after the original idle phase, without rewriting `interaction-metrics.mjs` or `latency-diagnostics.mjs`. Both files are byte-identical to the 2da baseline. The original selectors and project action protocol are unchanged; the product now renders that same primary Open-project destination in the stable navigation grid.

The current saved-context/setup prefix is retained, including protecting the cancellation entry through the current queue UI. This extra setup was absent from the oldest driver. It is before the quiet/measurement window and is not claimed to be a byte-identical whole-driver replay.

## Fixed scope

- Fresh package, private state/auth and four saved project contexts; all queues paused.
- Same draft isolation and warm-up, then 60-second quiet phase.
- Exactly 100 switches, 200 input actions and 100 resets. No discarded primary rows or timing subtraction.
- Targets unchanged: switch p95 <= 300 ms, input p95 <= 100 ms, p99 < 1000 ms.
- Optional eight later trace rounds remain separate (32 extra actions). Default matches the prior idle run's `--trace-after yes`; raw trace is private.
- Hard unconditional zero-Create and zero-Start guards before proxy forwarding; `setPaused(false)` is forbidden. Positive structural proxy maxima do not authorize requests.
- No external models, worker execution, accepted results, independent Linux sampler, sustained or Windows claim.
- Preparation 60 s, workflow 210 s, cleanup 120 s. Outer wait 450 s, then exact-driver TERM grace 30 s and final exact-driver kill wait 10 s. Any outer timeout is a failed run and does not certify service/browser cleanup. Original durable journal/bounded startup/cleanup remain in the driver.

The driver's `passed` and exit 0 mean the diagnostic completed with its scope/cleanup checks, not that responsiveness targets passed. Inspect `targetComparison` and all original `interactions`/spans. The wrapper makes no aggregate resource or whole-machine cleanup claim.

## Root-only launch, after matching commit/package are ready

Use the task runtime activation and an exclusive Linux Docker window. Never use this harness against Dylan's shared Windows Docker.

```sh
source /workspace/.luci-tools/activate.sh
python /workspace/.luci-tools/cloud032-idle-integrated-candidate-20261009/launch.py \
  --source EXACT_NEW_PRODUCT_COMMIT \
  --helper-source EXACT_NEW_HELPER_COMMIT \
  --package /tmp/luci-retained-tools/EXACT_MATCHING_EXTRACTED_PACKAGE \
  --provenance /workspace/luci/artifacts/quality-swarm/EXACT_CANDIDATE/package/package-provenance.json \
  --image luci/sandbox:verified-candidate-20261008t140025z-a4a2e82b \
  --expected-image-id sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98 \
  --image-provenance /workspace/.luci-tools/sandbox-candidate-20261008T140025Z-a4a2e82b/receipt.json \
  --output /workspace/luci/artifacts/quality-swarm/FRESH_CANDIDATE_IDLE \
  --fixture-root /workspace/.c32 \
  --trace-after yes
```

The launcher's preflight verifies clean exact source, every frozen repository input against its committed Git bytes, external generated bindings, matching package provenance/binary/web, exact image, empty Docker containers/volumes, and unchanged six-GiB root floor. The driver subsequently hashes the entire archive and every extracted package member. Missing capacity remains a refusal.

No source revision is silently substituted. If a selected helper changed after capture, preparation must be explicitly regenerated/reviewed before launch. The launcher does not edit the repository or the failed fixtures/ledgers.

Keep raw result, node/browser timing spans, accepted-harness copies, stdout/stderr, source/provenance, durable journal, wrapper identity/timeout fields and independent final closure. The raw fixture may contain credentials and source context; publication must be a separately hash-bound allowlisted copy. Known controller identities can be independently read from the journal. Browser/zombie closure needs its own observed identities; no such future observations are invented here.

No syntax command or controlled/runtime test was executed for this prepared follow-up at root's request. The exact textual extraction assertions and source-binding construction completed. Root may check syntax before its runtime window if appropriate.

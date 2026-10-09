# Installed worker recovery attempt 3 — ordinary Stop and exact release passed

The installed Linux API scenario passed with product **`d348f0127a920aab05b54a4192baa8ed3800fe54`**, clean helper **`d52f7fe3d3981342b46391c34ad6517f100ba1be`**, native Codex and a **scripted local provider**. Wrapper, driver and observer exited **0**. There were **zero external model calls**, browser runs or compiler runs. This verifies the specified worker lifecycle mechanics, without establishing model reasoning, app-control acceptance, Windows behavior, sustained performance or full product quality.

The driver ran **2026-10-09 00:04:23.871–00:06:08.413 UTC**. Wrapper elapsed time was **125.881 seconds**. [The actual launcher's sanitized source](actual-launcher.sanitized.py), [invocation](invocation.sanitized.json) and [exit capture](invocation-exit.sanitized.json) bind the selected package, helper, image, fresh output and launcher exit. Sanitized installation locations are placeholders, not executable replay instructions. Original byte counts and SHA-256 bindings are retained in [publication.json](publication.json).

All **13** archived helper modules match the pinned Git blobs and driver manifest; wrapper source bindings before and after execution are identical. The external wrapper, sampler and observer-cost launcher match their retained launch-manifest hashes. That manifest's `runtimeInvoked: false` describes its earlier preparation; this attempt's invocation and actual receipts establish the subsequent run.

The driver verified **2,714 matching installed package members**. Binary SHA-256 is `94e59ff60a5ee6a156f55302f0e55000c232a81a90e4d6fa3fd4872947930db3`; archive SHA-256 is `01512720ca8190b2737fd521b2db39e29329529b211fd830b0ac0a891073d7fd`. Image is `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`, with its own [sanitized provenance](image-provenance.sanitized.json). The matching earlier [package integrity receipt](../recovery-d348f012-attempt1/package/package-integrity.json) records 347 licence components and zero missing licences. This capsule preserves local verification evidence; it does not upload the package or establish independent remote retrieval.

## Observed recovery behavior

Two approved synthetic projects shared one worker slot. Each real worker requested **1,024 MiB / 1,000 mCPU / 256 PIDs**; Docker recorded exactly **two Creates, two Starts and zero fence Creates**. Admission recorded **6,582,775,808** and **6,578,233,344** free root-filesystem bytes before those Creates, exceeding the six-GiB requirement. The one-GiB running reserve and original time/resource bounds remained active.

1. A executed one native command, committed `build-a-result.txt`, and remained running with its final provider response held. Its settled lease charged 1,024 MiB and 1,000 mCPU; B remained queued without a worker charge.
2. Normal controller shutdown and restart preserved A's exact container and PID, immutable context, output, transcript prefixes, clean Git state and single command dispatch. The settled worker was correctly refused by the separate permanent-fence recovery preview with `recovery_launch_unconfirmed`.
3. Revoking Edit and Execute separately from the same member produced **403** Stop responses while retaining the exact lease and Docker state. Reading the foreign project produced **403**. Three stale/foreign abandonment selections produced **409**, without changed claims, retained state or Docker mutations. Ordinary session Stop has no request-revision field; the helper did not invent one.
4. One of eight bounded exact-Inspect gates discarded a real daemon response. Stop returned **503** with the exact lease retained. The gate budget was not exhausted; unused gates were cancelled before retry.
5. The supported ordinary session Stop then removed A's exact container and lease and cleared its queue resource counters. A retained **exit reason `killed`**, operation **`review-required`**, and queue state **`completed`**. That queue state is scheduling completion, not a successful natural agent exit or accepted implementation. No permanent fence was created and A never restarted or redispatched.
6. Dispatch resumed and B executed once. Its native exit-status frame was **0**, session exit reason was **`process exited`**, queue reached **`completed`**, and its exact lease disappeared. B's output was not integrated automatically; its saved operation also remained **`review-required`**.

The ten [phase receipts](result/phase-01.json) and complete [sanitized driver result](result/result.sanitized.json.gz) preserve the detailed assertions and API/Docker observations. A's two terminal attachments observed no exit-status frame; its killed outcome comes from the supported Stop and retained session state. B's exit frame was observed at **00:05:39.337 UTC**, container/session absence at **00:06:03.935**, and completed queue/absent lease at **00:06:06.423**. Socket closures were abnormal (`1006`); they are not used as evidence of natural completion.

## Retained work and independent closure

Both controlled outputs remained in their clean task checkouts, absent from the primary projects:

| Task | Task commit | Output SHA-256 | Result |
| --- | --- | --- | --- |
| A | `f1f8833dac0d361e756c06e67bb027b741b6fcf4` | `d010792f3cb603f1bfe74359ea707d8ac4e858a02bdf42cc85c22bb929a0c822` | Ordinary Stop; killed; review required |
| B | `47aa97af4403ef1f31869ac79a563ac8cd5bca0a` | `f009d9ebd4e4c22374f16f092ef4a8df764866db69c6be9cb977657389390624` | Natural native exit 0; review required |

At **00:11:59.708 UTC**, the [independent read-only closure](independent-scoped-closure.json) found all **seven** recorded PID/start identities no longer live, no fixture-owned process, **zero Docker containers or volumes**, and the exact parsed version-1 execution ledger with **zero leases and zero check fences**. Both queue rows had zero memory/CPU charge. The check performed no signals, stops, removals, manual ledger repair or application mutation. All controller stop receipts had exit **0**, `forced: false`; driver cleanup contained no unexpected-worker or force-removal entry, and proxy requests were fully drained.

Three other process environments were unreadable. The seven recorded PID/start identities were checked separately; the environment scan's absence observation covers accessible process environments rather than every process on the machine.

The independent inspection also matched every current captured context file to the actual native command's recorded hash/length, verified the manifest digest against its saved session, and matched primary canonical exports to the capture. It checked clean primary/task Git heads, exact changed files and output hashes, and found the recorded native observations in each retained transcript. It preserved all **342** fixture files byte-for-byte and left all original evidence inputs unchanged. Raw transcripts, session credentials, private paths and raw ledger bytes are excluded; hashes and allowlisted results remain retrievable. The complete private closure receipt is bound by SHA-256 `a72ea3bdca5a97315e817c1bcae123173bdf968ba2f3d4fe577ff231d9dfc8e8`.

## Observation and publication limits

The observer retained **124 samples over 124.841 seconds**, covering both worker container IDs and controller generations. Its initial six `exact-owned-instance-not-yet-provided` observations remain explicit. Check-specific counters are `not-observed`, because this was a worker scenario. There was no pre-run idle baseline; sampled RSS/cgroup observations do not establish sustained memory requirements, UI latency, other-app responsiveness or causal observer overhead. The observer self-cost receipt measures its own interpreter, excluding spawned Docker clients and daemon work.

[Publication validation](publication-validation.json) and [member bindings](publication.json) document exact source matching, deterministic gzip, original/decoded hashes and recursive sanitization. Every decoded member and JSON string was scanned against exact private fixture roots and known retained credential values, including the full `fixtureRetained.path` and wrapper `privateFixtureParent` fields. Known installation paths are replaced with placeholders. Original inputs and the fixture inventory were rechecked unchanged after publication reads. Ephemeral credentials no longer stored cannot be re-read; this is not a universal secret-detection claim.

The capsule was prepared with read-only Docker/process/filesystem inspection and documentation writes only. No additional runtime, browser, compiler, test or model workload was launched to prepare it. It closes this exact installed API recovery witness; app-control acceptance and broader release gates remain separate.

# CLOUD-079 — Project draft recovery triage

P2 validation-only fixture correction; no proven product defect.

The interrupted integrated frontend report observed sketch recovery restoring `x:1` instead of the expected `x:99`. Original report, log and source-binding hashes are preserved in `original-failure.json`; all remain unchanged. The affected test, DesignEditor, useProjectDraft and DraftRecovery matched that run's source bytes at triage start.

The mock exposes its change callback during render but hydrates in a passive effect. The test previously waited for placeholder DOM only. A premature synthetic edit could become the clean baseline and the later hydration could become the retained edit. This is a source-supported ordering risk, **not a deterministically reproduced cause**: the unchanged isolated file passed 22/22.

`draft-recovery-hydration.patch` changes only the test. It waits for explicit adapter hydration on each mount and asserts the exact retained edited scene before unmount, while keeping explicit restore and no-autosave requirements. Fixed isolated file: 22/22 passed. No product defect was established and product code was not changed.

Both runs used activated Node v24.19.0, one worker, no file parallelism and a 120-second deadline. The external Vite plugin loaded the recorded before/fixed test bytes under the original test module path. `loaded-test-source.json` binds the actual loader bytes. All desktop source and root source/config/lock inputs were hashed before/after each run and stable. Installed-lock and selected dependency identity matched between runs. See `runs/{before,fixed}/result.json` and `report.json`.

Run commands were `source /workspace/.luci-tools/activate.sh && python3 /workspace/.luci-tools/draft-recovery-triage-20261011/run.py before` and the corresponding `fixed` command. Output directories are exclusive; this runner will not overwrite either evidence run.

Root owns repository integration. No fullsuite rerun or fullsuite pass is claimed. Original integrated outcome remains 2938 passed / 2 failed. Root's integrated typecheck and wider product gates remain separate. No application, browser or Docker runtime, private fixture, persistent state or ledger was touched.

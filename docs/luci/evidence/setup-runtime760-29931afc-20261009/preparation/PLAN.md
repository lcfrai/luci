# Retained760 bootstrap preparation — no operations executed

Root owns execution, actual Docker/tag state, exclusive windows and settings save. This stage performs no app/runtime/container/real-storage action. Required currentGo was actually restored separately: workspace is a real directory; TMP is a compatibility alias. Go operation receipt remains separate.

## Space calculation

Original Go restoration final observation: workspace7,352,782,848B, TMP2,260,738,048B. A later root observation8KiB lower is separate. `forecast.json` uses original numbers and logical bytes only; metadata allocation/receipts/concurrent growth require fresh observations after every operation.

1. Optional Rust620,087,163B + PowerShell192,479,038B + historical425 archive1,013,359,941B moved TMP→workspace: predicted root5,526,856,706/TMP4,086,664,190B.
2. Unchanged exact760 export admission remains3,905,250,841B on EACH filesystem. Retaining gzip adds a HARD1,288,490,188B output bound on workspace, so this preparation requires workspace≥5,193,741,029B and TMP≥3,905,250,841B. Exceeding raw/compressed bounds FAILS; no truncation can become a pass. The output cap is not a predicted measured size.
3. At that compressed cap, after export root≈4,238,366,518B. Return optional Rust/Pwsh/425 to TMP: root≈6,064,292,660/TMP2,260,738,048B, still below exact runtimeCreate6,736,759,858B.
4. ONLY AFTER exact760 archive verification and publication, old333 becomes historical. Preserve its1,013,367,768B and original receipts/aliases in TMP: final predicted root7,077,660,428/TMP1,247,370,280B. This restores the runtime rootfloor with≈341MB predicted margin at the full output cap, but must be measured. At actual~1.014GB output, margin is larger. No new Create is allowed from arithmetic alone.

No export can share a runtime window. These temporary placements deliberately lower root below the runtimeCreate floor. Keep each admission guard unchanged; do not start warm/cold workloads until tools are returned, old333 is safely historical and fresh actual capacity passes. The monitor uses its existing1,342,177,280B cancellation reserve during export, not a continuous3.905GB floor. Daemon temporary placement remains unknown; no override is assumed.

## Reviewed sequence and commands

Before all operations, root confirms no task compiler/tool/archive writer and no active Docker request/container; preserve all existing failed fixtures/charges. The two known historical caches have already been cleaned; do not count their gain twice. Current299 and5f packages, all fixtures/evidence, modules, current760 image and original333 remain protected. Every step writes original before/after receipts; failure stops the sequence with partial topology retained.

1. Reuse existing reviewed optional-tool mover and its source-bound live-owner watchdog:

```sh
python3 -B /workspace/.luci-tools/run-cloud054-tool-relocation.py --tool rust-1.99.0 --direction to-root
python3 -B /workspace/.luci-tools/run-cloud054-tool-relocation.py --tool powershell-7.5.5 --direction to-root
```

These exact tools retain their historical manifest checks, mode/ownership/mtime/byte verification, process-reference refusal and explicit accepted unreadable PID/start limitations. Fresh operation receipts are mandatory; do not reuse old forward receipts for the later return.

2. Return only the exact425 archive from TMP to its two original hardlink paths, preserving the TMP alias:

```sh
python3 -B /workspace/.luci-tools/runtime760-bootstrap-v1/run_place.py --window-granted --mode 425-to-root --output /workspace/.luci-tools/runtime760-placement-425-to-root-UNIQUE
```

The helper uses the same checked copy/hash/fsync/alias primitives as prior425 placement. It refuses different bytes, unexpected targets, insufficient destination capacity and occupied stage paths. Its180s internal limit is cooperative between chunks. The wrapper independently waits300s then bounds exact known child settlement to20s; synchronous OS calls are not preemptible. It distinguishes zombies/non-live from PID absence. Root must stop and inspect preserved partial state after any interruption. It sends no signals and makes no Docker requests.

3. Remeasure both filesystems and freeze source/helpers. In the exclusive Docker window, root verifies the intended engine and exact760 configuration. If `luci/sandbox:dev` already denotes exact760, retain it. If absent, root may add that tag to the exact immutable image; if it denotes a different image, refuse and review, never silently overwrite. The archive helper itself never tags, loads, builds, starts, prunes or removes images/containers.

4. Stage the new archive through the bounded root watchdog. Substitute only actual reviewed source/engine/evidence values, not a moving branch or guessed pass. The reviewed runtime receipt is bound as context at its original scope; the helper never upgrades a failed receipt.

```sh
python3 -B /workspace/.luci-tools/runtime760-bootstrap-v1/run_archive.py --window-granted --expected-source EXACT40HEX --expected-engine EXACT_ENGINE_ID stage --reviewed-runtime-receipt /absolute/original/receipt.json --reviewed-runtime-sha256 EXACT64HEX
```

The wrapper provides the private child record before traffic and pins archive helper, budget and startup template. It bounds the operation leader wait to900s and exact known leader/client settlement to at most40s additional waits. Monitor600s/TERM/KILL and source-bound private client identity are separate. Receipt writes cannot bypass settlement; unreadable/missing identity, forced closure or incomplete state remains failed/uncertain. Synchronous OS I/O, daemon drain and descendant census remain explicit limits. No output or pin publication occurs merely because the timer ended.

Raw archive bytes, compressed cap, gzip footer/CRC, complete image config/rootfs/diffIDs, exact entrypoint/sandboxd bytes/mode/ownership,106 runtime source inputs and immutable candidate image are verified. The original build receipt remains unchanged. The workspace keeps original333 through a rollback hardlink plus old setup files. An incomplete export retains partial gzip, diagnostic logs, original pins and durable failure receipt; root may later retire only the exact incomplete output after settlement review.

5. Review generated `new-install.sh`, `new-start.md`, provenance and exact archive receipt. The full installer starts from root's exported saved field, including its trailing newline; the new concise startup template preserves actual constraints/commands and removes obsolete/truncated coordination prose. The complete original saved fields are hash-bound and retained in the stage. No network/secret/environment settings are altered. Root owns configuration-draft saving after actual verification.

6. Publish filesystem pins (not platform configuration/snapshot) only from status`verified-staged-not-published`:

```sh
python3 -B /workspace/.luci-tools/runtime760-bootstrap-v1/run_archive.py --window-granted --expected-source EXACT40HEX --expected-engine EXACT_ENGINE_ID publish /workspace/.luci-tools/runtime-archive-760-EXACT/receipt.json
```

Each archive/file replacement is atomic; the four-file transaction is journaled, not atomic. Original old/new files remain available. An interrupted publication fails closed; do not run the installer until exact final pins agree. Required760 canonical archive is a real workspace regular file. `dockerLoadVerified=false` remains until a separate actual load/restoration check. Source/runtime/package identity are distinct.

7. Return optional tools using each NEW forward receipt's final file and existing wrapper:

```sh
python3 -B /workspace/.luci-tools/run-cloud054-tool-relocation.py --tool powershell-7.5.5 --direction return-to-tmp --forward /absolute/new-forward-operation/FINAL-finished.json
python3 -B /workspace/.luci-tools/run-cloud054-tool-relocation.py --tool rust-1.99.0 --direction return-to-tmp --forward /absolute/new-forward-operation/FINAL-finished.json
python3 -B /workspace/.luci-tools/runtime760-bootstrap-v1/run_place.py --window-granted --mode 425-to-tmp --output /workspace/.luci-tools/runtime760-placement-425-to-tmp-UNIQUE
```

Each return requires measured destination room and byte/metadata parity; do not force it if another task consumed TMP.

8. Move only historical333 to TMP, keeping both historical paths as aliases; the required canonical760 archive is excluded. The helper requires the published760 receipt's exact SHA, status, newarchive checksum and newsetup pins before touching the now-historical333 pair:

```sh
python3 -B /workspace/.luci-tools/runtime760-bootstrap-v1/run_place.py --window-granted --mode 333-to-tmp --published760 /workspace/.luci-tools/runtime-archive-760-EXACT/receipt.json --published760-sha256 EXACT64HEX --output /workspace/.luci-tools/runtime760-placement-333-to-tmp-UNIQUE
```

Original333 bytes and historical receipts remain unchanged; inode/hardlink topology changes, TMP historical snapshot retention is not guaranteed. This is not original fixture or lease release, and never authorizes deleting failed evidence.

9. Root verifies actual final floors, real workspaceGo and760 archive/pins, optional compatibility paths, unchanged protected files and exact idle/cleanup state. Only then can it save full configuration fields and separately test a supported runtime restore/start. Existing-image scope is not a fresh-daemon load or fresh snapshot proof.

## Controls and preparation scope

Eight tiny archive/placement tests and five fake watchdog tests pass: both filesystem/export-output boundary refusals; compressed overflow leaves only allowed bytes; signal-error preservation; bounded settlement waits; final-space dependency;425 roundtrip preservingbytes/aliases/protectedfile; wrongbytes refusal;333 refuses absent verified760 publication. They use disposable tiny files/fake signals, not Docker or real archives. First attempt7pass/1fixtureerror is preserved: fixture accidentally used realTMP free space, so the3.9GBguard correctly refused before its tiny copy. Corrected fixture explicitly substitutes20GiB; production guards unchanged.

The initial archive draft and source review are retained: outerwatchdog added for inherited monitor join/synchronous verification bounds, and settlement errors now preserve primary failure. No implementation/runtime/benchmark acceptance follows from preparing these helpers.

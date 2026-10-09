# Prepared C054 runtime space operations — NOT EXECUTED

These are narrowly scoped external operations, not Luci product features. **No Docker operation, archive/tool hashing, copy, move, deletion, cache reclamation or process scan has been executed by these scripts.** Preparation read existing small receipts and wrote these external files. Twelve tiny mock controls pass; neither archive restoration nor actual relocation/retirement has been validated.

## Important current blocker

The current inactivity policy **refuses any unreadable or changed live-process observation**. This host previously had unreadable foreign processes, so either operation may refuse before mutation. This is deliberate conservatism, not a claim those processes use the tools or Docker. Do not bypass the condition silently. Root can choose another space step, or request a separately reviewed scoped policy based on exact identities and explicitly retained uncertainty. A readable process scan also cannot prove that no remote client uses Docker; a fresh, live root-owned exclusive-coordination receipt is required in addition.

Both entrypoints require an explicit operation flag; importing modules does nothing. No script runs automatically. Final invocations require root review and the actual new-image/coordination pins. Never execute these examples with placeholder values.

## Exact image retirement

`retire_old_image.py` accepts only these removal targets, in order:

* `luci/sandbox:dev`
* `luci/sandbox:verified-candidate-20261008t140025z-a4a2e82b`

Both must resolve immediately beforehand to `sha256:333effcf40151e2411fa036b4581113be2af7f8d441bf9566f1cce6e7f208e98`. Every removal is `docker --host unix:///var/run/docker.sock image rm --no-prune TAG`, with no force/prune or image-ID removal. A fresh private HOME/DOCKER_CONFIG and allowlisted locale/PATH environment prevent inherited Docker contexts/TLS settings from redirecting it. All containers must be absent, including stopped ones. Full image/tag inventory must preserve the trusted base `sha256:9e0fd9f731408c4405c4b9e97308bf807c8088cf521a138c0b1921667a831351`, the explicitly supplied new candidate, and every unrelated image/tag. Only the old image may disappear.

Before removal the script streams a fresh SHA256 of the preserved 1,013,367,768-byte archive and checks exact SHA `7caf7aab012cd4a5bf2ace5c8a2e12e617b4afbaef20f6b3db64586a24e9fcf3`. It pins the existing historical full layer/file receipt and the newer full compressed-byte/CRC/manifest/config inspection. This is an immutable-content proof chain; the script does not repeat decompression or claim fresh `docker load`. The archive remains untouched. Each mutation has a durable pending receipt; a timeout or failure remains uncertain/partial, with no replay or rollback. A second-tag failure does not erase the first removal.

The candidate receipt is an explicit bounded descriptor with `status: "passed"`, `candidateImage: "sha256:..."`, `baseImage` equal to the trusted base, and `sourceStable: true`, and must be SHA-pinned. Root must produce it from its actual new candidate evidence; this script does not create or infer it. If the real build receipt has another schema, prepare a source-bound descriptor or review a narrow schema adapter. Do not label an unbuilt candidate passed.

Example (not executed):

```sh
python3 retire_old_image.py --execute-retirement \
  --new-image sha256:ACTUAL_NEW_64_HEX_ID \
  --candidate-receipt /ABS/ACTUAL_CANDIDATE_DESCRIPTOR.json \
  --candidate-receipt-sha256 ACTUAL_DESCRIPTOR_SHA \
  --quiescence /ABS/FRESH_COORDINATION.json \
  --quiescence-sha256 ACTUAL_COORDINATION_SHA \
  --receipt /workspace/.luci-tools/cloud054-runtime-operations/FRESH_RETIREMENT_RECEIPT_DIR
```

## Reversible tool relocation

`relocate_tools.py` accepts only Rust1.99.0 (620,087,163 regular bytes) and PowerShell7.5.5 (192,479,038 regular bytes). Existing pinned manifests supply every path, byte hash, mode, owner/group and mtime. These exact trees historically have no symlinks or repeated/external hardlinks; any newly observed symlink, special file or hardlink is refused rather than normalized.

`--direction to-root` copies `/tmp/luci-retained-tools/TOOL` into the fresh private staging root here (`retained-tools/TOOL`). It verifies both trees, fsyncs copied files/directories, checks inactivity again, and atomically exchanges the exact `/tmp` directory with an exact compatibility symlink using Linux `renameat2(RENAME_EXCHANGE)`. Only after verification and another inactivity check does it remove its exact duplicate original. Existing `/workspace/.luci-tools/TOOL` compatibility links remain unchanged, pointing through the exact `/tmp` path.

`--direction return-to-tmp` requires the SHA-pinned successful forward receipt. It copies back into an exclusive new `/tmp` sibling, verifies both trees, atomically exchanges the copy with the exact expected compatibility link, and removes only that exchanged symlink plus the verified root duplicate. Unknown destinations/scratch paths refuse. Interrupted copies, exchanged duplicates or partial deletion remain recorded; there is **no automatic rollback or rerun**. Review the last immutable numbered receipt before any recovery. The return direction expects a fully completed forward operation, not a partially completed one.

Destination available space must cover pinned logical size plus1GiB. This is a copy guard, not the image-export budget, installed6GiB floor, effective-memory admission or whole-machine resource acceptance. Root must retain those independent gates. Inodes/ctime/atime are not preserved across filesystems. All compilers/tests/tool users must stay quiescent through copy, exchange and duplicate removal.

Example (not executed):

```sh
python3 relocate_tools.py --execute-relocation --tool rust-1.99.0 --direction to-root \
  --quiescence /ABS/FRESH_TOOLS_COORDINATION.json --quiescence-sha256 ACTUAL_SHA \
  --receipt /workspace/.luci-tools/cloud054-runtime-operations/FRESH_FORWARD_RECEIPT_DIR
# Later, after export and a new explicit coordination receipt:
python3 relocate_tools.py --execute-relocation --tool rust-1.99.0 --direction return-to-tmp \
  --forward-receipt /ABS/FORWARD/NUMBER-finished.json --forward-receipt-sha256 ACTUAL_SHA \
  --quiescence /ABS/FRESH_TOOLS_COORDINATION.json --quiescence-sha256 ACTUAL_SHA \
  --receipt /workspace/.luci-tools/cloud054-runtime-operations/FRESH_RETURN_RECEIPT_DIR
```

## Coordination and receipts

A coordination receipt must be generated by a real, still-live root orchestration wrapper after workers have stopped, with `version:1`, exact `scope` (`cloud054-exclusive-docker` or `cloud054-exclusive-tools`), `exclusive:true`, `activeOperations:[]`, current kernel `bootID`, current `monotonicNS`, and that wrapper's `ownerPID`/`ownerStartTicks`. It must be no older than30seconds at each critical boundary; slow hashing/copy can cause an honest refusal with partial copy retained. This file is an operator coordination assertion, not a fabricated daemon lease or proof of universal inactivity. No valid coordination receipt is created during preparation.

All operation receipts require a new direct child of this owned/private directory. Numbered JSON records are exclusively created0600 and fsynced, including pending intent before mutation and failure/partial state afterward. Successful and failed final records include available source-before/after hashes. The two entrypoints share `operation_common.py`; pin all three sources and review them together. There is no generic deletion, cache cleaning, Docker prune, fixture/ledger mutation, image load or rollback command.

Synchronous file operations, subprocess capture and filesystem deletion are not hard-preemptible by Python's checks. A root-owned independent watchdog remains necessary for a hard wall-time envelope; it must preserve uncertain daemon/move state rather than automatically retry or erase partial receipts. Docker command output is checked after return (4MiB stdout/64KiB stderr), not a streaming memory bound. The source scripts are preparation artifacts, not unattended production utilities.

## Inert verification

`python3 -B -m unittest -v test_operations` exercises the exact two-tag command sequence using fake Docker, archive mismatch, extra/foreign tags, protected candidate identity, partial second-removal uncertainty, unrelated inventory drift, fixed tool/direction boundaries, explicit operation consent and active/unknown process refusal. No real Docker command, process enumeration, tool/archive read, copy/exchange or removal occurs. These mocks do not establish actual filesystem or daemon operation safety. Initial12 and final12 outcomes overlap; the follow-up only adds failure-source binding. All original logs/source are retained.

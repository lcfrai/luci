# Prepared scoped tool relocation v2 — no operation executed

Only the Rust/Pwsh relocation is included. Original v1 scripts/evidence remain untouched. The exact copy, manifest, fsync, atomic exchange, verified duplicate retirement and return paths remain those described in `../cloud054-runtime-operations/README.md`; v2 staging is `/workspace/.luci-tools/cloud054-runtime-operations-v2/retained-tools/TOOL`. Image retirement policy is NOT changed by this bundle.

Root explicitly approved scoped uncertainty: readable executable/cwd/argument/maps/open-file references to either tool always refuse, even when a later observation is unreadable. Unknown records retain PID/start ticks only when stat identity is readable and stable before/after the limited observations. Only exact pairs listed in the current root coordination receipt may remain unknown. New, reused, missing or unstable identities refuse. No unrelated process is signalled; no environ file is read. This is scoped uncertainty accepted by root, NOT global inactivity or a claim unreadable processes cannot use the tool.

## Exact coordination JSON

A still-live root orchestration wrapper must create and SHA-pin a fresh private JSON file with these fields. Fill real values; do not execute a placeholder receipt. The wrapper remains alive until the relocation finishes. `/proc/PID/stat` start ticks are field22 (index19 after the closing `)`); owner state Z/X refuses. Maximum explicitly declared validity is600 seconds; every critical boundary rechecks time, owner and script hashes.

```json
{
  "version": 2,
  "scope": "cloud054-exclusive-tools",
  "exclusive": true,
  "activeOperations": [],
  "bootID": "ACTUAL /proc/sys/kernel/random/boot_id",
  "monotonicNS": "INTEGER time.monotonic_ns() at issuance",
  "expiresMonotonicNS": "INTEGER issued + at most 600000000000",
  "ownerPID": "INTEGER still-live wrapper PID",
  "ownerStartTicks": "ACTUAL decimal start-tick STRING",
  "operationSources": {
    "operation_common.py": "ff414e7be81a069bfb7a08eab0a73f826e4dc731596049324812dbfc70a1aa39",
    "relocate_tools.py": "a86951497592b707ff2b157cf4c6e193180f18eedf5af42e09f98994242a3f78"
  },
  "acceptedUnknownProcesses": [
    {
      "pid": "INTEGER explicitly reviewed unreadable PID",
      "startTicks": "ACTUAL stable decimal start-tick STRING"
    }
  ]
}
```

`acceptedUnknownProcesses` must always be present (empty if no exceptions), has at most32 unique pairs and is not a wildcard. Permission-limited identities from old receipts must be checked by root against current PID/start before being accepted. Known busy observations override accepted unknown pairs. The coordination receipt asserts root has stopped competing task tool use and will keep it stopped; it is not a kernel lease.

## Forward command (root executes only after its own preparation)

```sh
cd /workspace/.luci-tools/cloud054-runtime-operations-v2
python3 -B relocate_tools.py --execute-relocation --tool rust-1.99.0 --direction to-root \
  --quiescence /ABS/FRESH_COORDINATION.json --quiescence-sha256 ACTUAL_JSON_SHA256 \
  --receipt /workspace/.luci-tools/cloud054-runtime-operations-v2/FRESH_FORWARD_RECEIPT_DIR
```

For PowerShell substitute exactly `powershell-7.5.5`. Rust source is `/tmp/luci-retained-tools/rust-1.99.0`; PowerShell source is `/tmp/luci-retained-tools/powershell-7.5.5`. Existing `/workspace/.luci-tools/TOOL` links remain unchanged. Only the exact `/tmp` target changes atomically to a v2 staging symlink after successful byte/mode/owner/mtime/no-link-surprise comparison. No unknown destination or old scratch directory is overwritten. Destination free space must exceed pinned bytes+1GiB; independent export/installed/memory gates remain root's responsibility.

## Return command

Create a new coordination receipt under the same source hashes, then pin the successful forward `NNN-finished.json` and pass it explicitly:

```sh
python3 -B relocate_tools.py --execute-relocation --tool rust-1.99.0 --direction return-to-tmp \
  --forward-receipt /ABS/FORWARD/NNN-finished.json --forward-receipt-sha256 ACTUAL_FORWARD_SHA256 \
  --quiescence /ABS/FRESH_RETURN_COORDINATION.json --quiescence-sha256 ACTUAL_JSON_SHA256 \
  --receipt /workspace/.luci-tools/cloud054-runtime-operations-v2/FRESH_RETURN_RECEIPT_DIR
```

Return verifies the root copy, copies/flushes/verifies a fresh `/tmp` sibling, atomically exchanges it with the exact expected compatibility link, then retires only the verified duplicate. A failed/partial operation does not authorize a blind rerun or return: inspect the last numbered durable receipt and preserve partial paths. Every mutation intention is journalled; failures retain available source-before/after hashes. Inodes/ctime/atime change across filesystems. A parent watchdog is still required for a hard deadline on synchronous filesystem I/O; no script here supervises or signals unrelated processes.

## Validation

Three pure mock control groups passed, including exact accepted identity versus new/reused/missing/unstable unknowns; a known executable/argument match followed by unreadable or disappearing maps still refuses; explicit expiry≤600seconds, source hashes and live owner identity. Original partial-observation counterexample failed one group (two passed), then the identical final test source passed all three. Initial earlier3-pass controls lacked that scanner case and overlap; they are retained separately. No real `/proc` scan, Docker call, tool/archive hash, copy, exchange or deletion was executed. The old12 mock controls belong to v1 and are not claimed as new v2 coverage.

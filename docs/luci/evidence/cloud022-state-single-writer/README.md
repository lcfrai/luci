# CLOUD-022: one writer per state directory

A second Luci API or desktop instance using the same `LUCI_STATE_DIR` now refuses
startup before instance identity, stores, provisioning or backend construction,
even when it selects a different `LUCI_ADDR`. Separate private state directories
remain independent. Setup and acceptance fixtures must give simultaneous isolated
instances distinct state roots; changing only their addresses is insufficient.
An ordinary restart reuses the same root after the previous process exits.

The startup lease now applies to unmanaged Windows/macOS as well as Linux and
managed workers. The persistent `.luci-state.lock` is never deleted: OS ownership
ends when its handle closes or the process dies. A refused contender does not
choose alternate state or overwrite the existing lease record. Linux retains the
existing `flock` protocol used by offline host recovery. Windows uses handle-based
path/ACL checks, rejects junctions and hard-linked lock files, and takes an exclusive
`LockFileEx` lock. Safe inherited user/SYSTEM/Administrators ACLs are retained;
existing directory ACLs are not silently rewritten.

[Verification](verification.json) records the integrated Linux race run:
**219 passed outcomes, 0 failures and 3 skips**, across `internal/statelease`
(17 passed) and `cmd/api` (202 passed, 3 skipped). Both packages completed; stderr
was empty and no race diagnostic was observed. Counts include subtests and helper
entry points. The three skips are existing Windows-junction scanner cases that
cannot execute on Linux, with their exact reasons retained in the receipt.

The executed lease cases include contention between real processes, different
API addresses sharing one root, independent roots, graceful exit, process kill,
reopening the same lock identity, retained existing bytes, negative path/mode cases,
and two-way exclusion with a raw offline `flock` holder. The worker startup fixture
supplies its execution-root setting so an unrelated configuration error cannot
masquerade as lease refusal.

[Raw integrated output](linux-actual-source-race.jsonl),
[stderr](linux-actual-source-race.stderr) and the
[reported command](linux-command.json) are retained. The
[post-run source manifest](source-postrun.json) hashes 716 inputs and confirms all
11 CLOUD-022 applied-file actions match the
[applied candidate](source-apply.json), including removal of `lease_linux.go` in
favor of `lease_unix.go`. The manifest was captured **after** execution; no full
pre-run manifest exists for this invocation. The tested working tree includes
uncommitted CLOUD-020/CLOUD-022 changes, so its base HEAD alone is not the tested
source identity. Earlier candidate logs and provenance are retained separately.

[Crosscompile evidence](crosscompile.json) records successful **statelease test
binary compilation only** for Windows amd64 and macOS arm64. The integrated lease
sources match that candidate. Native Windows ACL, junction, lock/restart behavior
and native macOS execution remain **pending**. Crosscompilation does not establish
native acceptance; the Windows suite also has explicit capability skips for file
symlinks and administrator-owner assignment when the current token lacks those
permissions.

This evidence does not establish installed desktop/package acceptance, real
Docker workloads, provider/model quality, cloud deployment or release readiness.
No broad tests were rerun while collecting these receipts, and collection made no
production source or user-state changes. Crosscompiled binaries remain external;
their sizes and hashes are recorded instead of committing generated executables.

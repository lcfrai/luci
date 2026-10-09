# Older425 runtime archive placement — prepared, not executed

Root requested a byte-preserving relocation of only the historical42597a/c593 compressed archive, after the new package build settles and only if actual root headroom still requires it. This is not elimination of duplicate physical bytes: two old names are hardlinks to one1,013,359,941-byte inode. Expected SHA256 `c593f37eaa3795358aa1b76dd355828ddaadfcf503b9a5e3a1aea6976c947c91` belongs to old image `sha256:42597a985b20f2aee31a629a320980f02180bde307b6ffdd3ec60b257a82af5b`.

Exact source paths:

- `/workspace/.luci-tools/runtime-archive-7e1b1f608cfa/verified-luci-sandbox-dev.tar.gz`
- `/workspace/.luci-tools/runtime-archive-1f845cfd2b45/old-luci-sandbox-dev.tar.gz`

Exact new byte location: `/tmp/luci-retained-tools/historical-runtime-42597a-c593f37e.tar.gz`. Both old paths become absolute compatibility symlinks only after complete copy/hash verification. Original archive bytes, mode0600, uid/gid and mtime are retained; original inode/hardlink topology changes. **TMP retention across environment snapshots is not guaranteed.** No Docker/image operation occurs. The333eff/7caf bootstrap pair, active760 image, all fixture/state/ledger/evidence contents and the old receipt bytes are not mutation targets. Protected333 validation checks metadata parity only, not a fresh333 hash pass. A new760 snapshot/archive remains separate and unperformed here.

The helper requires root's coordinated no-launch/no-writer window, ordinary same-inode/nlink2/owned0600 exact-size sources, matching both historical receipt hashes, and fresh payload SHA. Initial TMP free must cover the entire new file plus500MiB minimum and64MiB working guard; the guard is checked throughout copying and before each alias replacement. Actual free space must be remeasured after the current build, not inferred from earlier observations. Final receipt reports observed root/TMP deltas without crediting a future admission pass.

Each source replacement has a durable intent. If a write/fsync/replace fails, no automatic deletion or unrecorded rollback occurs: original hardlink(s), verified target, partial copy and journal remain for exact reconciliation. The list of completed replacements may lag an interrupted post-replace fsync; inspect the last intent and actual exact aliases. Never call a timeout or incomplete receipt successful. Reversing the move requires root headroom, a separately verified target→root copy and reconstruction of the two hardlinks; metadata receipts cannot restore the original inode itself.

Root command, only after review and current-build settlement:

```sh
timeout --signal=TERM --kill-after=5s 200s python3 -B \
  /workspace/.luci-tools/relocate-historical425-v1/relocate.py \
  --window-granted \
  --output /workspace/.luci-tools/historical425-relocation-operation-20261009-root1
```

The180-second internal bound is cooperative between I/O calls; outer200s/5s bounds only this new helper if a syscall stalls. Original package/runtime processes are not signalled. Helper SHA256 `5a5335c6d980c4744fbc2c134cef282d4c2c594a771eb5ca170e63d72d8c7f18`. No operation has been run by the preparer.

Six tiny disposable controls pass: successful exact placement, mismatched receipt hash, non-hardlinked pair, insufficient TMP guard, interrupted post-replace fsync after first alias and after second alias. Both injected failure cases retain accessible original bytes and fail durably; neither is claimed successful. Original test-script syntax failure is retained and never ran. Independent source review found no blocker at this scope; it did not hash/copy the real1GB archive or execute tests. Runtime/provider/Windows/performance/lease-release acceptance is unrelated.

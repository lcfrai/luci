# Prepared retained-archive placement

Prepared only; the placement script has not run. Scope is exactly four generated archives in the two preserved release directories named by the parent. No repository source, manifest, old receipt, Docker state/configuration or other archive is changed.

| Original preserved release | Archive | Bytes |
| --- | --- | ---: |
| 0.5.0-29931afc-preserved-before-b94ad37e | luci-cloud-linux-amd64.tar.gz | 34,651,655 |
| 0.5.0-29931afc-preserved-before-b94ad37e | luci-linux-amd64.tar.gz | 69,117,092 |
| 0.5.0-b94ad37e-preserved-before-78665270 | luci-cloud-linux-amd64.tar.gz | 34,672,996 |
| 0.5.0-b94ad37e-preserved-before-78665270 | luci-linux-amd64.tar.gz | 69,160,779 |

Total: **207,602,522 bytes**. The initial target-free-space requirement is **627,032,922 bytes**: all archive bytes plus **400 MiB (419,430,400 bytes)**. Preparation read only the two manifests and exact filesystem metadata; it did not hash archive bodies or execute a placement/dry run/test.

The fresh destination is `/tmp/luci-retained-tools/release-archives-299-b94`, with `29931afc/` and `b94ad37e/` subdirectories. Original archive paths become absolute symlinks to their verified corresponding copies. The archive payloads remain present and accessible at the original paths; retained target files are not deleted by the script.

Execution performs these steps:

1. Require explicit `--apply`, the reviewed script SHA-256, ordinary owning user, private real directories, fresh absent target, different source/target filesystems, and the free-space reserve. Open only the two pinned manifests and four expected regular, owned, mode0600, single-link archive files with no-follow checks and retain their identities. Directory/source advisory locks serialize cooperating placement commands; the parent must keep these historical archive paths free of concurrent writers.
2. Match both manifests' complete pinned bytes, source commits, archive sizes and SHA-256 values. Copy all four archives into new exclusive files. Check each original's hash while copying, fsync, then independently read/hash the copy. Recheck source and target identities and the remaining-byte reserve. **Every copy must pass before any original archive path changes.**
3. Create a unique temporary symlink beside each original, recheck pinned identities, and atomically replace that one original path. Verify its exact symlink target and resolved verified inode, fsync the directory, and record the placement before releasing the old original file descriptor. No manifest or old receipt is opened for writing.
4. Verify unchanged manifest bytes, record observed free space, and write a unique result under `operations/<UTC>-<UUID>/` beside this prepared script. Journal/results are mode0600, parent directories mode0700. No archive extraction, shell subprocess, network request, Docker operation or additional cleanup occurs.

Each path replacement is atomic; the four-path operation is not transactional. Failure before replacement leaves all originals regular. Failure during replacement can leave a mix of verified symlinks and regular originals; all copied payloads remain and the journal identifies each completed placement and any temporary symlink. There is no automatic rollback or deletion. The target must be absent for any invocation; a partial attempt is inspected before deciding a recovery action. Copy/check loops have a cooperative 180-second bound; blocking filesystem calls are not preempted.

Root executes after reviewing the exact source and replacing `SCRIPT_SHA256` with the supplied pin:

```sh
python3 /workspace/.luci-tools/release-archive-placement-299-b94/placement.py \
  --apply --expected-script-sha256 SCRIPT_SHA256
```

Require both exit 0 and `result.json.passed=true`: these mean four verified placements and unchanged manifests were recorded. Non-receipt descriptor-close failures are recorded before serializing the result. Receipt-file/directory finalization failure is reported by stdout and a nonzero exit and may prevent a trustworthy final receipt; do not accept a receipt alone after such an exit. Other failures retain a failure receipt where receipt setup succeeded. The script reports actual before/after free space; it does not promise an exact filesystem-space delta or a subsequent build/admission result.

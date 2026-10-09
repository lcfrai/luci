# Proposed retirement of six reproducible extracted packages

This is a read-only proposal. No package, archive, fixture, ledger, cache or Docker object has been removed or changed.

The six exact generated trees are `package-0.5.0-0dcafb14`, `-2da1a821`, `-67a5b7c7`, `-99007404`, `-d20ebd29` and `-edd0cb12`, all under `/tmp/luci-retained-tools`. Every retained portable archive was fully hashed against its original package-provenance receipt. All 8,362 extracted regular files match their archive members by size and SHA256, and all file modes match. There are no symlinks, hardlinked regular files or nonregular entries in these trees. All entries belong to uid1000. Exact directory/file ownership, mode, timestamps, inode and path metadata are retained in per-package manifests.

Their total file block allocation is **859,254,784 bytes (about0.80GiB)**, versus838,893,329 logical bytes. TMP directory entries report zero additional allocated blocks. This is an estimate of generated copies that could be retired, not a guaranteed available-RAM increase or successful admission. A mutation receipt must measure actual tmpfs and effective-memory changes afterward without dropping caches or weakening admission.

Keep b94ad37e,29931afc and5f4a966f packages; all archives and their original aliases; all source, evidence, fixture directories and ledgers; runtime760 and every Docker object; and all module/model/tool sources. Only the six frozen extracted trees are proposed. Historical `packageDir` receipt paths must remain unchanged. If retired, those old copies are explicitly unavailable until restored from their bound original archive; do not redirect historical identities to a newer package.

The bounded usage scan recorded all6,637 listed identities, including6,630 literal zombies. Seven initially live identities had readable fields checked for the exact target paths; no target reference was found. Exe/cwd/maps/fd remained unreadable for203/start1521,254/start1534 and260700/start2331192. Their readable cmdlines were inspected without environment reads. The inspector's own disappearing fd3 is a separate observation race. This establishes no universal idle or future-launch guarantee. Any retirement must recheck current references and preserve these uncertainties.

`plan.json` is the exact proposed allowlist and binds every metadata manifest. `verification-v2/verification.json` records the full archive/member verification. The first verifier refusal remains in `verification.json`: it incorrectly expected archive directory modes to equal extracted modes, while the original Python tar data filter omits directory-mode restoration. The corrected verifier still requires every file mode/content and retains actual directory metadata for restoration. The initial4096-identity capped usage scan also remains; the corrected scan records all identities.

Root must approve the exact plan before any retirement, then revalidate unchanged archive/member metadata, exact roots and current usage; record durable intent; unlink only the pinned ordinary singly-linked files and empty directories; and retain precise partial-failure receipts. No generic recursive deletion, following symlinks, global cache drop or Docker operation is proposed. New live references, changed files or unexpected entries must refuse mutation.

`restore.py` is prepared and syntax-checked, but has not performed restoration or been run against disposable controls. It verifies the plan/manifest/archive hashes, refuses an existing destination, exclusively creates a new package tree, verifies every written member and reapplies recorded ownership assumptions, modes and modification times. It leaves a partial destination with a failure receipt on errors. Original inode/ctime/topology cannot be recreated; access times are observer-sensitive. Its300-second cooperative bound must be combined with a root-owned outer watchdog before execution.

Example restoration after a separately approved retirement, using the frozen plan hash:

```sh
python3 -B /workspace/.luci-tools/old-extracted-packages-retirement-plan/restore.py \
  --plan /workspace/.luci-tools/old-extracted-packages-retirement-plan/plan.json \
  --plan-sha256 c7e298a50984471954939b0d0f3f01e7f96a2e881cfb0252fcfaec5cf61287cd \
  --package 0dcafb14 \
  --receipt /workspace/.luci-tools/old-extracted-packages-retirement-plan/restore-0dcafb14-result.json
```

This preparation performed no app, build, provider, Docker or Windows workload. The plan does not retroactively change any acceptance result.

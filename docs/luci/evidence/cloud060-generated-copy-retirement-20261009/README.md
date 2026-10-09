# C060 generated-copy retirement: completed maintenance only

Root's **2026-10-09** operation records successful retirement of three exact duplicate generated `luci`/`web` payloads at product `5f4a966f6d70264c92ee2176dde912e1b43fc3c7`. [The final journal](operation/028-final.json) is separate from the frozen preparation manifest, whose `preparationOnly`/`realRetirementExecuted: false` and README describe its earlier preparation stage. No original receipt has been edited to erase that chronology.

## Exact operation and scope

The three roots are `cloud060-package-reuse-5f4a966f`, `cloud060-package-reuse-v2-5f4a966f`, and `cloud060-package-reuse-v3-5f4a966f` under `/workspace/.luci-tools`. Each selects **425 files**: the service binary and 424 web files. [The complete proof](preparation/proof.json) retains every selected reference/archive/copy hash and size, generated-copy file/directory metadata, and nonpayload sibling metadata. It is a record of root's pre-operation verification; this publication did not repeat binary/archive hashing or read private application state.

The final script is `b50fdde1a24745da33b72c970bf0cc606065b095994a17c710acbbc3acfa39a4`; proof is `cbab4651da95badd8f04ea40153587ae92790d3b2ac831a8a197de2b0903bce4`. All **28 actual journal files** are retained, including intent, byte-verification checkpoint, six link intentions/completions, the all-links-before-unlink checkpoint, six unlink intentions/completions, and final status. Compatibility links now point to the retained exact `/tmp/luci-retained-tools/package-0.5.0-5f4a966f/{luci,web}` bytes. Physical duplicate inode identity is not preserved; reconstruction can preserve bytes and recorded metadata, not recover the original inode.

Only those six payload paths were eligible. Receipts, cache/home/tmp siblings, helpers, fixture state/ledgers/evidence, the reference/archive and **v4** were excluded. The script rechecks the build receipts' bytes and immediate nonpayload sibling metadata; this is not recursive content verification of all sibling trees. The final preservation statements are run-owner assertions supported by the narrow source/journal/proof, not a universal private-tree or v4 byte census. No Docker/image/cache mutation, application acceptance or product reservation release is credited to this storage operation.

## Storage values and remaining gate

| Observation | Before, bytes | After, bytes | Difference, bytes |
| --- | ---: | ---: | ---: |
| `/workspace` available | 6,360,612,864 | 6,609,715,200 | +249,102,336 |
| `/tmp` available | 2,387,849,216 | 2,386,538,496 | -1,310,720 |

The **260,227,072 candidate allocated bytes** and **257,436,648 logical duplicate bytes** are different measurements from the observed available-space change. Directory/filesystem activity and concurrent accounting prevent substituting one for another or claiming all measured change is uniquely attributable. The final root value remains below root's stated approximately **6.736 GB Create floor**. No new readiness/admission decision, workload outcome, latency/RAM improvement or acceptance pass follows. These are historical values, not promised current headroom.

## Preserved failure and validation evidence

The original `else424` syntax error is retained and did not execute. Initial seven controls passed, after which independent source review found two gaps: untracked partial link transitions and loss of a known busy reference after a later permission failure. Three added controls fail against the original source and pass against the corrected source; the corrected existing seven also pass, giving **10 distinct controlled cases**, not a cumulative count of repeated runs. [Original review](preparation/source-review-v1.json) and [final review](preparation/source-review-final.json) retain their exact source bindings. These small disposable fixtures are separate from the actual root operation.

The script's 180-second deadline is cooperative between I/O operations; the documented outer timeout handles stalled I/O. The journal confirms root coordination and accepts three exact unreadable PID/start identities. The process check covers readable cmdline/exe/cwd only, with no environment/fd/maps census or global/future-launch guarantee. Controlled success does not erase these limits. Failure after unlink begins must remain partial/uncertain; no implicit replay or original-inode restoration is supported.

The capsule contains only prepared sources, inert-control receipts, full public metadata proof, prior read-only review, actual journals and publication material. No binaries, archive bytes, fixture state or private logs are included. Standard task paths and PID/start identities are intentionally retained. [Original bindings](original-bindings.json) and [checksums](SHA256SUMS) bind the exact copies. The earlier 293-file package/terminal overlay remains separate and unchanged. This supplement is maintenance evidence, not `LOCAL TEST HANDOFF READY`.

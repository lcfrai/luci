# Source-only inspector/controller candidate

No acquisition, archive execution/inspection, compiler/tests, native payload import,
Docker or model operation has run. This candidate needs exact root review and one
separate effect release per phase. It does not close the SBOM or release an image.
The accepted dedicated lifecycle remains `0b8b33c6` unchanged.

The implementation is `windows_inspector_controller.py` plus `inspect_inputs.py`;
`test_inspect_inputs.py` supplies inert fake-input/network/ownership regressions.
No new dependency is required. Root can arrange the one coherent source/inert check
batch; actual Windows JOB_LIST/readback/handle behavior remains later acceptance.

## Fixed input and batch identity

The exact manifest remains SHA256
`f839f2d1edaf0fbc1f039d20fcc4abd83b04accba2d0846e369174a0f2bad8b2`;
the exact inventory lock remains
`91f2739b1ccfb8c5277e41ebbcaa45ba7ba16f1018abdf0122fc39c1ec0d0423`.
Scoped `.gitattributes` now preserves those existing bytes across platforms.
No URL, package version, binary size/hash or lifecycle field changed.

The only accepted batches are fixed projections of that same immutable manifest:

| Batch | Inputs | Exact compressed bytes |
| --- | ---: | ---: |
| `wheels` | 2 | 10,733,989 |
| `missing-notice` | 1 | 40,888 |
| `base-layer` | 1 | 28,238,443 |
| `primary` | 4 | 39,013,320 |
| `debian-controls` | 174 | 144,257,468 |

There is no generic URL/argv/member selection and no source-archive batch. Each
subset is explicitly named; a failed primary batch never silently becomes a
successful wheel subset. Separate wheel/notice phases allow useful review without
weakening the unresolved base transport. HTTP401/302, duplicate/missing length,
content/transfer encoding, extra/truncated body or foreign digest refuses. There is
no proxy handler, ambient credentials, cookies, redirect, registry token or retry.
A registry challenge/redirect remains an explicit blocker for base-layer/primary.

## Separate proposed effects, each still unreleased

Use only the pinned Python executable:
`C:/Users/DylanMackie/.cache/codex-runtimes/codex-primary-runtime/dependencies/python/python.exe`,
SHA256 `b847bb9f4e48c19f585c8bf7d8b355f3f8c6d2125717ece9c9d7e16fb9f09de8`.
The controller path is this checkout's absolute
`E:/Luci-local-acceptance/checkouts/discover-documents-2017/images/document-extractor/windows_inspector_controller.py`.
Source/worker/controller/test hashes are frozen in the external source receipt.

1. First proposed release is **acquisition only**, `--phase acquire --batch wheels
   --output E:/Luci-local-acceptance/artifacts/extractor-wheels-acquire-review-20261016`.
   Controller/worker accept no `--input` in this phase. Every exact response streams
   once to an exclusive partial file; only exact size/hash plus body EOF publishes
   the final owned file. This does not parse wheel members or import a package.
2. Root reviews that phase's durable result and **positive complete Job/PDH closure**.
   Only then may it release `--phase inspect --batch wheels --input
   E:/Luci-local-acceptance/artifacts/extractor-wheels-acquire-review-20261016
   --output E:/Luci-local-acceptance/artifacts/extractor-wheels-inspect-review-20261016`.
   This phase has no network calls. Prior worker-result bytes must match the retired
   controller receipt, and all input identities are reverified while immutable read
   handles remain held. Changed/failed/uncertain acquisition is never adopted.
3. `missing-notice`, `base-layer` and `debian-controls` require separately reviewed
   exact fresh directories and phase releases. No automatic follow-up/acquisition,
   install, parser, image load/build, signature-key installation or runtime follows.

Launch Python with `-I -S -B`, a closed/unused parent stdin and no credential-bearing
environment or command text. The controller never reads stdin. Its data child has
NUL stdin and only owned stdout/stderr handles. No secret enters argv, stdin or
receipt; server/member text cannot become command/error authority. The child gets
only validated SystemRoot/WINDIR and its own TEMP/TMP. No HOME/PATH/proxy/SSL override,
Docker config, user account or model credentials are inherited.

## Resource and ownership proof

Root performs fresh positive identity/no-concurrent-heavy-work preflight and pins
the exact source/Python/primitives/output before release. Built-in admission uses
fresh >=8GiB physical and commit, CPU<50%, and >=2GiB free output volume. Child Job
limits stay 2GiB aggregate/1.5GiB process/4 processes with kill-on-close, no breakaway.
The existing pinned API Job structs/readback/inventory/PDH and normal OwnedJob
resume semantics are reused; the historical API/auth run entrypoints are not called.

The suspended child is created with **JOB_LIST plus explicit HANDLE_LIST**, so it is
assigned at process creation. There is no create-then-assign fallback or uncontained
Python-loader execution. Unsupported Windows attributes refuse. Attribute value
arrays stay alive until list destruction; Job handle is never inherited. Suspension,
exact membership and hard limits are read back before resume. This follows the
[documented Windows attributes](https://learn.microsoft.com/en-us/windows/win32/api/processthreadsapi/nf-processthreadsapi-updateprocthreadattribute).

Whole420s includes preflight, work<=300s and cleanup reserve120s. Native-memory/PDH
monitoring has a separate no-I/O deadline watcher, independent of blocked observers.
4GiB reserve, CPU unknown >5s, failure or deadline requests exact Job termination
**before** result reads, receipt formatting/publication, reader settlement or joins.
Cleanup stops both watchdogs before closing handles; every CloseHandle ACK is
checked, terminal process + empty Job are separate required facts, and PDH worker,
query and counter handles must be positively closed. Unknown ownership/cleanup or
receipt publication never permits an inspection phase. Root's external420s guard
remains necessary for a stuck controller/OS filesystem call; such an overrun is
uncertainty, not a promised successful cleanup or automatic replacement.

Bounded actual streaming writes enforce compressed data<=256MiB, result<=2MiB and
stdout/stderr<=64KiB each. No archive payload member is extracted to a host path.
Trusted metadata receipts may retain bounded licence/control bytes as base64; those
bytes are never evaluated. Expansion<=1GiB, members<=100,000, member<=128MiB,
metadata<=1MiB and total retained member/metadata rows<2MiB are checked incrementally.
ZIP central records are bounded before ZipFile allocation. A single gzip/xz/bzip2
frame is accepted; extra frames/bytes, unsupported tar dialect/PAX/ZIP64/sparse/device
features, traversal, duplicates and ambiguous local/central extents refuse.

Windows denies input write/delete sharing for the complete hash+parse lifetime.
Tar links resolve only against immutable inventory, with confined paths/depth and
explicit missing targets; no host link is followed. ELF x86_64 LE dynamic tables,
wheel RECORD hashes and release metadata, Debian control identity, OCI uncompressed
diff_id and the complete88-package base inventory are data checks. None imports a
wheel, invokes ldd, initializes PDFium, executes a package script or runs a document.
Native compiled feature/version attestations and authenticated transaction/security
closure remain explicitly unverified. Unsupported evidence is a refusal or unknown,
never an inferred pass. Result-budget exhaustion can truthfully refuse a large
batch; no rows are silently omitted to manufacture completeness.

## Deferred checks and next owner

The inert suite covers exact enumerated batches; compressed extra/truncated frames;
tar checksums/traversal/devices/whiteouts/links; actual expansion/retention brakes;
ZIP RECORD and local/central mismatches/collisions/trailing bytes; hostile ELF extents;
ar/control/JSON framing; fake HTTP body hashes/lengths/redirect/challenge/proxy cases;
prior uncertain cleanup/changed receipts; and emergency-stop-before-I/O ordering.
It has not executed locally. Windows atomic process assignment, real HTTP/archive
behavior and actual binary membership require their own exact admission later.

Signature-chain work remains limited to established trusted material; this candidate
does not import/install keys or turn HTTPS hashes into authenticated source closure.
The dedicated builder containment adapter is the next independent source proposal.
Normal Codex sign-in and real Develop preparation belong to the separate assigned
worker; this image-source batch provides no evidence that Develop works end to end.

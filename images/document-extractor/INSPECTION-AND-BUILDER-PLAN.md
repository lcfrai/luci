# Exact remaining inputs and contained inspection proposal

This is a source/metadata checkpoint. Acquisition, archive inspection, image build,
parser/native imports and product runtime are **not authorized**. Lifecycle source
remains `0b8b33c6bf8dcbb1e80b6a23655794eeae17cc6a` unchanged.

## Notice and relation work completed

`notice-review.json` records 34 resolved source notice items and one exact remaining
notice. It retains complete bytes under `notices/debian`, with per-source copyright,
source, attribution, modification and library-replacement requirements. Twenty-two
freeform notices have concrete retention decisions, rather than manual-review
placeholders. Source-wide mixed terms remain intact; `NOASSERTION` is retained for
the final binary licence conclusion. This is not fulfilled distribution compliance.

Twelve formerly missing notices are tied to exact release-version changelogs and
immutable Debian maintainer/LTS commits. Libxcrypt and libxcb generate copyright
files; their retained notice is the byte concatenation explicitly named in the
exact packaging rules. No packaging program ran. Installed-byte comparison remains
outstanding. Libarchive `3.6.2-1+deb12u5` was absent from the authoritative copyright,
maintainer and LTS metadata checked; older `+deb12u4` is not substituted. Its exact
40,888-byte patch archive is now a finite, separately gated inspection input.

All 464 selected dependency edges in the retained metadata satisfy Debian epoch,
upstream, revision, tilde and numeric version comparisons for the 174 selected
packages. `review_selected_relations.ps1` is a data-only comparator; receipt
`selected-relations-09156044.json` is outside the repo in the metadata evidence
directory. It does not prove authenticated binary controls, virtual-provider
versioning, Breaks/Conflicts/Replaces, maintainer scripts or an installable transaction.

The installed read-only verifier is `C:/Program Files/Git/usr/bin/gpgv.exe`, version
2.4.9, SHA256 `f4d13204d77fdf63c02b0e6742230f83a833128c28f7b715709c2c63a96c427b`.
No existing trusted Debian archive/uploader verification keyring was established
in the inspected runtime; none was imported, written or modified. Therefore no
signature authentication is asserted. A release needs the exact signed archive
InRelease -> Sources/Packages hashes -> selected files, plus uploader/source
authentication against trusted verification material. Fresh HTTPS fingerprint or
key pages alone do not establish that trust. Source descriptor hashes already
retained do not substitute for this chain.

## Finite next acquisition and data-only inspection

`acquisition-inspection-manifest.json` gives exact URLs, lengths and SHA256 for:

- Two wheels, the selected immutable OCI layer and missing libarchive patch metadata:
  **4 inputs / 39,013,320 bytes**. The layer is compressed OCI gzip, not the different
  debuerreotype rootfs xz archive. Existing retained inline config/manifest metadata
  need no new download.
- The optional subsequent exact Debian control batch: **174 files / 144,257,468
  bytes**, with no resolver or version substitution. Inspect all complete control
  relations and actual member/licence hashes before an install/build release.
- A separate corresponding-source review set: **282 exact archives / 2,031,106,636
  bytes**. It exceeds the binary batch cap and is not included in its authorization
  or disk budget. Retaining source URLs is not an offer already fulfilled.

Concrete proposed helper seam, to implement/review before any effect:
`extractor-input-inspector.py --manifest <exact-hash-pinned-json> --batch primary
--output <fresh-absolute-owned-directory>`. It accepts only the enumerated four
input IDs; the Debian batch is a separate explicit `--batch debian-controls`, and
there is no generic URL/argv/archive option or implicit source batch. Every file
is streamed once to exclusive `.partial` files; reject non-200/range/trailing data,
Content-Encoding and redirect, enforce actual received bytes and incremental hash,
then atomically publish only exact length/hash matches. An incomplete attempt is
retained as failed evidence, never resumed automatically or passed to Docker.

Registry detail is an explicit remaining transport prerequisite: the manifest
refuses redirects. If the pinned registry blob requires a CDN redirect, stop and
review the exact redirect policy/host binding before acquiring it. An anonymous
`auth.docker.io/token` request would be solely for service `registry.docker.io`,
scope `repository:library/debian:pull`; its token stays in memory, only sent to the
exact registry host, never read from Docker config or stored in logs/receipts.
No saved credential/proxy/environment/default endpoint is consulted.

The controller reuses the inert reviewed Windows `OwnedJob` and `PdhMonitor`
primitives, without calling their build main:

| Primitive | SHA256 |
| --- | --- |
| `windows-api-job-2g-pdh-recovery.py` | `c050bd16622be034a4ef8db050a1554c0ee492a4519ff0b581c647af14f5fd6d` |
| `windows-job-inventory.py` | `52cf3bd64fc39e8c986cf80eb7022b70e58b59f6e2ad3db443d8ab924f4da844` |
| `windows-pdh-cpu-format-recovery.py` | `2bc56957918bc577b206b4aede979aab867c929a3f48799fd7be477f564d2b49` |

The pins identify reusable primitives, not an executed or already-written inspector.
Before runnable release pin its Python executable, helper, manifest, final source
revision and fresh literal output. Verify host reserve >=8GiB physical and commit,
CPU<50%, no other heavy workload; Job 2GiB aggregate/1.5GiB per process/4 children.
One absolute 420s deadline covers transfer+inspection <=300s and reserved cleanup
120s, including all clients and PDH. Requests <=30s and serial; their sublimits do
not extend the whole deadline. Stop at 4GiB reserve or unknown monitoring.

The inspector treats bytes as data. It never imports a wheel, executes package
scripts, calls `ldd`, initializes PDFium or opens a user document. Bound cumulative
actual expansion to1GiB, members to100,000, each member to128MiB, metadata to1MiB
and receipt to2MiB; hash/discard payloads incrementally. Reject duplicate/colliding
paths, traversal, device entries and ambiguous framing. ZIP headers/RECORD hashes
and ELF dynamic tables are inspected statically. Base-layer links may only resolve
against the confined immutable member inventory; never dereference host paths.
The exact wheel member/compiled-feature evidence can still remain unknown after
static inspection; missing build attestation is not converted to a success label.

Disk enforcement for this trusted data-only writer is by actual streaming counters:
compressed retained files <=256MiB, no member extraction onto disk, receipt<=2MiB,
exclusive workspace marker and path confinement, no child that can write elsewhere.
In-memory/decompression allocations are additionally subject to the Windows Job.
Do not advertise that byte-count strategy as containment for arbitrary build scripts.
On failure close exact responses/readers, terminate only the owned Job, verify it
empty and PDH worker/query closed. Preserve history/input hashes, report residual
exact PIDs/handles and uncertainty; no blanket cleanup or automatic replay.

## Specific unsupported builder primitive

The existing builder cannot satisfy the proposed complete build bounds unchanged:

- At the frozen source base `2017e2bd`, `dockerbe/runtime_image.go:578–580` uses
  classic `ImageBuildOptions` with RUN-container memory/swap/CPU and `nproc=256`.
  It has no hard disk/cache allocation quota, no `pids.max=64` bound covering all
  daemon work, and explicitly states that pull/bookkeeping share an allowance
  rather than a separate cgroup. The existing path also chooses network `default`
  and its runtime-image-specific context/identity; it is not this offline recipe.
- `runtime_image.go:27,538–544` is a30-minute runtime-image budget, shortened by a
  caller deadline; a new420s controller alone would not create missing quotas.
- `resource_disk_windows.go:28–50` observes matching Desktop daemon free space
  through WSL; that is capacity observation, not an enforced disk allocation.
- Windows `OwnedJob` contains its Windows client and children. Docker daemon
  build workers/cache are not descendants in that Job. Killing the client or
  watching free disk does not prove their retirement or enforce a2GiB cache cap.

Thus **no runnable Windows image-builder candidate is released**. The precise
missing primitive is an independently owned build-worker+cache resource domain
with enforceable hard disk quota and aggregate cgroup CPU/memory/swap/PID limits,
plus positive exact worker/cache retirement. A new isolated Linux builder VM or a
proved dedicated builder/storage quota adapter could supply it; either needs its
own exact source/resource/identity review. No daemon mode change, privileged nested
Docker/BuildKit, socket mount, unreviewed builder image, cloud query or held producer
transport is proposed as an implicit fallback.

Once supplied, freeze the closed inputs/recipe and require network none, no pull,
1CPU,2GiB memory=swap,64PIDs,128MiB tmpfs,1GiB expanded inputs,2GiB output/cache,
1MiB logs, compute<=300s plus cleanup120s within420s. Keep normal8/4GiB reserves,
serial admission and retained uncertainty. Actual image/runtime/OCR/model-backed
Discover acceptance remains required and has not run.

# Extractor image source and acceptance plan

This is an unexecuted, source-only proposal for root review. Acquisition, build,
runtime and distribution are **not authorized**. `closure.status` deliberately
refuses the recipe. `dependency-lock.json` is a metadata inventory with explicit
unresolved entries, **not a closed SBOM** or an installed-image observation.

## Exact inputs prepared

- Existing Debian index `3783cc01769c7b2b1b83a5c5ad96c815348e28ed7da68e2e3687004faa906251`
  resolves to amd64 manifest `f3034a6ec3c1205360777c4aae76234998866ad18806ae62b63a3f84ccad782b`.
  The official artifacts commit `8f962b15d7884a90e17876a9303cbac909d119aa`
  publishes the same manifest and its complete 88-package base inventory. Only
  manifest/config metadata was read; no layer or rootfs archive was acquired.
- 174 concrete Debian packages have exact versions (including epochs), filenames,
  byte counts and SHA256 pins, covering the base and declared native/Python/font
  dependencies. Total declared binary bytes:144,257,468. The selected alternatives
  prefer existing base/root packages; `mawk` provides virtual `awk`. Ten metadata
  candidates are excluded, including the unnecessary `usrmerge` alternative.
  This still needs authenticated Debian control/relation verification.
- Python3.11 / Tesseract5.3.0-2 / eng and osd1:4.1.0-2 / DejaVu2.37-6 /
  Noto core20201225-1 / Noto CJK1:20220127+repack1-1 are pinned in that set.
  The two existing wheel hashes remain unchanged in `wheel-requirements.txt`.
- 127 Debian source descriptors have metadata digests and source-archive hashes.
  Available exact copyright texts are identified by URL/digest. Thirteen copyright
  metadata texts were unavailable; twenty-two notices have freeform declarations
  requiring review. Those cases remain explicit rather than guessed SPDX claims.
- Exact upstream helper/Pillow commits, twenty declared Pillow native components,
  twenty-nine upstream notices and PDFium8076 dependency revisions are recorded.
  Upstream declarations are not proof of the native members in the selected wheels.
  Default non-V8/non-XFA PDFium, no forms/JS initialization, remains mandatory.

Primary base provenance:
[official image definition](https://github.com/docker-library/official-images/blob/c7e58ea932df828de3150ca59c2bdf6e23e93717/library/debian),
[amd64 inventory](https://github.com/debuerreotype/docker-debian-artifacts/blob/8f962b15d7884a90e17876a9303cbac909d119aa/bookworm/slim/rootfs.manifest),
[amd64 OCI metadata](https://github.com/debuerreotype/docker-debian-artifacts/blob/8f962b15d7884a90e17876a9303cbac909d119aa/bookworm/slim/oci/index.json).
Per-component primary links/hashes are retained in the lock.

## Closure before an effect release

1. Verify the Debian InRelease/Packages and signed source-descriptor chain against
   the exact recorded pins. Check amd64 Depends, Pre-Depends, Provides, Breaks,
   Conflicts and version relations from authenticated controls. No resolver may
   silently substitute versions or add packages. Resolve missing notices or obtain
   exact notices through separately authorized, bounded archive inspection.
2. Root separately reviews any acquisition plan. Proposed binary context cap is
   256MiB (174 debs, two wheels, source/lock/notices); compressed base layer is
   exactly28,238,443bytes with its recorded digest. No package or layer is downloaded
   by this source work. Each future transfer needs exact URL, expected bytes/hash,
   bounded response/time and an owned temporary destination; incomplete data stays
   incomplete. Corresponding-source retention is a separate finite reviewed set.
3. Before build, inspect the exact archives in a separately contained job: bound
   total expanded inputs to1GiB, files to100,000, paths to confined regular members,
   reject traversal/special entries and bound decoded metadata. Close native ELF
   dependency/member hashes, trained-data hashes, wheel feature flags and notices.
   Never infer them from source tags alone. Materialize reviewed `SHA256SUMS` and
   exact `verifiedInstalledFiles`; generate SPDX/CycloneDX with honest licence
   conclusions and retained corresponding-source/notice obligations. Run the exact
   vulnerability review. Missing facts keep the lock unresolved.
4. Only a separately reviewed closed lock plus root effect authorization can
   change `closure.status` to `closed-reviewed`. Freeze source, lock, checksum
   list, notices and recipe hashes together. A status file alone is not approval.

## One bounded offline build proposal

Root must select and review the actual Linux amd64 Docker host, endpoint, builder
identity, controller helper and resource enforcement **before** this is runnable.
No suitable contained builder has been established by this source checkpoint.
Windows client Job limits alone do not contain daemon/build-worker allocation.

Proposed one-attempt limits:1CPU,2GiB memory=swap,64PIDs,128MiB temporary filesystem,
1GiB expanded inputs,2GiB output/cache disk allowance,1MiB logs,300s build compute
plus120s retirement reserve within one absolute420s deadline. Host admission keeps
normal fresh8GiB physical/commit and CPU<50%;4GiB/unknown brake; serial heavy work.
If a selected builder cannot prove all limits, refuse rather than change engine,
builder mode or resource semantics. No daemon/service fallback or automatic retry.

The exact build uses `Dockerfile.review`, the pinned amd64 base already verified in
the authorized offline cache, local hashed debs/wheels only, build network none,
pull disabled and no package resolver/index/update. It unpacks/configures only the
closed set and installs only hash-locked wheels with no deps/index/source builds.
It preserves distribution and wheel notices, installs only eng/osd data, verifies
the complete installed package/member set and fixed Python/PDFium/Pillow versions,
then removes only its own `/inputs` and build-source directories inside the image.
No project data, model, account credentials or host home/socket enter the image.

An outer whole-attempt supervisor must stop the positively owned build on deadline,
pressure or unknown capacity and retire its exact job/client/builder objects. Track
actual daemon workers, not just the CLI PID. Failed/uncertain cleanup retains its
claim and prevents another attempt. No prune/reset/blanket kill or held transport.
Freeze image ID/platform manifest, final actual SBOM/notices, bounded output,
resource timeline and positive cleanup receipt. Reproducibility means an exact
input/package/member/config comparison; byte-identical OCI output is not asserted
until a separately authorized second build proves it.

## Actual product acceptance after image release

Use the dedicated normal Discover upload/extract routes and its actual selected
immutable image. One extraction at a time; fixed512MiB memory=swap,1CPU,32PIDs,
128MiB tmpfs, networknone, nonroot, one exact private original RO mount,120s compute
and60s cleanup within180s. Preserve originals, extraction revisions and receipts.
Normal runtime admission and positive owned cleanup are still mandatory.

Required real cases: native digital PDF; scanned and mixed pages; rotated/EXIF;
Unicode including fallback fonts; correction/source-location verification;
encrypted/corrupt/unsupported and page/pixel/expanded-output limits. Inspect actual
retained text/OCR segments, page geometry and attributed uncertainty. No idle,
prose, exit alone or fixture count proves extraction quality or completion.

Exercise cancellation/host pressure while the actual parser is working, reload and
explicit restart, changed/revoked current authority, capture loss and positive exact
retirement. Emergency exact-CID stop is independent of reader/publisher blockers;
unknown reader/cleanup/publication keeps capacity. Inert lost-ACK regressions are
separate mechanics evidence, not real Docker lifetime acceptance.

Then use actual source-linked questions through existing Luci conversation and
explicit specification context acceptance. That model-backed step needs its own
authorized native connection/runtime admission; no borrowed credentials, API key,
remote OCR or implicit provider fallback. Actual upload -> extraction -> correction
-> question -> accepted context E2E remains required and unproved.

No local compiler, parser, test, Docker, image build or model ran for this checkpoint.
Deferred source checks: the two Python standard-library test files, the review
verifier fail-closed cases, Dockerfile/lock consistency and meaningful image checks
only after their respective exact gates are released.

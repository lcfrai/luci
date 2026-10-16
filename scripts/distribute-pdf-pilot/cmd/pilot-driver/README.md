# Fixed PDF pilot driver — source only

This command is isolated under the existing `luci.local/distribute-pdf-pilot`
module. It is not imported by product code, included in an accepted image, or
permitted to run. Its source accepts only `--role render|read`, one lowercase
32-hex attempt, one lowercase 64-hex allocation, and one lowercase 64-hex
original input SHA-256 in fixed order. It cannot select a tool path, argument,
URL, output directory, font or model. The later owner must bind the controller's
exact allocation/actor/approved source/image/tool receipts and inspect the
selected container before Start; this program cannot authorize itself.

## Fixed operations and primary page-count source

Renderer starts the existing source pilot with `/opt/luci/luci-pdf-pilot
--font /input/NotoSansCJK-Regular.ttc --out-dir
/out/pilot-<AttemptID>`. The input face-0 font must be exactly 19,484,784
bytes with SHA-256
`b76b0433203017ca80401b2ee0dd69350349871c4b19d504c34dbdd80541690a`.
That is an **expected original-byte value in source**, not accepted package or
original ownership proof. The output directory is created empty under the
container's capped `/out` tmpfs, matching the writer's `ownedStage` check.

Reader executes these exact absolute tools serially under `LC_ALL=C`:

1. `/usr/bin/pdfinfo /input/pilot.pdf` — require exactly one bounded `Pages: 1` field before proceeding.
2. `/usr/bin/pdftotext -enc UTF-8 -nopgbrk /input/pilot.pdf /out/pilot-<AttemptID>/pilot.txt`.
3. `/usr/bin/pdftoppm -f 1 -l 1 -singlefile -r 72 -png /input/pilot.pdf /out/pilot-<AttemptID>/page`.

The selected package's own primary documentation is Debian Trixie
[`poppler-utils` `pdfinfo(1)`](https://manpages.debian.org/trixie/poppler-utils/pdfinfo.1.en.html):
the proposed pilot relies on its total page-count field, not on first-page
`pdftoppm` output or the writer's asserted one-page structure. The manual and
the exact packaged `pdfinfo` binary/recursive libraries/notice must be matched
to the later selected immutable `poppler-utils` archive before any execution.
The existing candidate reader inventory in
`debian-reader-source-v1.json` lists `pdftotext` and `pdftoppm` but **not**
`pdfinfo`. Its candidate `poppler-utils` version is
`25.03.0-5+deb13u4`; that version and the prior metadata/GPG/readelf
observations are unaccepted trust
evidence and cannot silently supply this third tool. No host-installed reader
is an eligible substitute.

## Bounds and ownership still required of the caller

Linux input opens refuse symlinks, FIFO/device files, hard links, changes and
oversized originals before reading; other platforms refuse. The driver waits
for one exact writer child or three serial reader children and retains no raw
tool output in its accepted frame. Its 28-second context leaves two seconds
for child-pipe reaping within the owner's 30-second role deadline. A blocked
kernel/native operation or stdout still requires the independent exact-CID
watchdog; the source driver alone cannot prove positive retirement.

Frames contain role `R` (PDF) or `D` (text then PNG), allocation/input digests,
a typed page-count byte (0 for renderer, 1 only after reader `pdfinfo`),
fixed ordered records with big-endian lengths and SHA-256, and one exact end
marker. Reader text is <=1 MiB and must be exactly `日本語` plus one terminal
line break; PNG is <=16 MiB, valid 595×842 PNG. Renderer PDF is <=16 MiB.
Docker stdout multiplex framing and the final EOF require a separate bounded
API parser. On any failure/cancel/partial stream the caller retains its exact
owned lease and partial API stage for readback, without replaying Create/Start
or treating prose/stdout as a tool call.

The later reviewed image must supply exact renderer, driver, `pdfinfo`,
`pdftotext`, `pdftoppm`, font and package/grant receipts, a numeric nonroot UID,
and enforce 32 MiB `/out` plus 8 MiB `/tmp` tmpfs under a 512 MiB cgroup.
Input binds must be read-only and positively readable by that UID on the
selected Docker engine. None of these image, package, ABI, UID/ACL or stage
proofs exists yet. The API-owned host stage remains byte-bounded application
capture with a 48 MiB disk admission charge, not an OS filesystem quota.

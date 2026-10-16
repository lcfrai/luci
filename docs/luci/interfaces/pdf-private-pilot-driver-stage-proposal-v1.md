# PDF pilot driver and bounded stage: source proposal

**State:** source plan only, superseding the writable host-bind output portion
of [the prior seam](pdf-private-pilot-stage-sdk-seam-v1.md). No driver source,
binary, image, selected font/reader package, production adapter, shared ledger
insertion, or execution grant exists. The pure chronology correction at
`01c51891d84681d093ec07d2ace4f2d671b67b22` remains unwired and untested.
No acquisition, build, native parser/reader, Docker or render is authorized by
this proposal. Prior evidence receipts are unchanged.

## The disk-bound correction

The existing 48 MiB `StageDiskBytes` is a host admission/reservation amount,
**not** a filesystem quota on a Docker bind. A renderer with a writable host
directory could fill more than 48 MiB before the API inspected `pilot.pdf`.
Neither Docker bind mounts nor the source-only 16 MiB final-PDF check provide
a hard cumulative write limit. Do not enable that configuration.

The smallest supported direction uses the existing selected-client **pre-Start
stdout attach** pattern in
`control-plane/internal/backend/dockerbe/document_extractor_result.go:AttachDocumentResult`.
The driver writes inside its own container only: `ReadonlyRootfs=true`,
`HostConfig.Tmpfs` has `/out` at **32 MiB** and `/tmp` at **8 MiB**, each with
reviewed `uid`, `gid`, `mode=0700`, `nosuid,nodev,noexec` where the packaged
binaries permit it. `HostConfig.Memory=MemorySwap=512 MiB` covers the child
and its tmpfs use; CPU 1, PIDs 32, network and IPC none, no log driver output,
no other write mounts. The adapter must verify the daemon's actual accepted
tmpfs/limits at Create and the effective cgroup/UID/write behavior on the
target selected engine before effects. Unsupported mount options or limits
refuse. A temp filesystem is destroyed at container stop, so attach must be
established and owned **before** Start, and complete framed output must be
captured while the driver is still alive. No late CopyFromContainer path.

The API alone writes captured bytes to its private host stage under
`ExecutionStateDir/pdf-pilots/<AttemptID>`; there is **no container-writable
host bind**. The host 48 MiB reservation covers at most one PDF (16 MiB), one
text result (1 MiB), one PNG (16 MiB) and bounded receipts/partial files. API
stream parsing caps each object and their aggregate **before** writing,
creates exact names with `O_EXCL` beneath retained `os.Root` handles, rechecks
ordinary-file identity and same private parent, syncs data and receipt, and
publishes a digest before the next effect. The 64 MiB host-capture allowance
still applies. The host stage has no per-directory OS quota: API-only bounded
writes and fresh disk admission are the proposed containment, and a selected
platform unable to prove private root/ownership and durable bounded capture
must refuse. A write error or extra Docker frame retains a partial owned stage
and uncertain lease for readback; it cannot become a successful result.

## One authored driver, two fixed roles

Smallest dedicated sources for the next review:

| Owner | Proposed file | Responsibility |
| --- | --- | --- |
| Pilot source | `scripts/distribute-pdf-pilot/cmd/pilot-driver/main.go` | Standard-library driver with a fixed `render` or `read` role and no shell, URLs, arbitrary argv or provider credential input. Its binary/source/notice need independent release receipts. |
| Pilot source | `scripts/distribute-pdf-pilot/cmd/pilot-driver/frame.go` | One versioned, bounded stdout frame format with role, exact allocation/input digest, lengths and SHA-256 values; no stderr/tool output in the accepted stream. |
| Selected SDK | `control-plane/internal/backend/dockerbe/pdf_pilot.go` | Exact selected-client image/config/limit/identity checks, Create, pre-Start attach, Start, Inspect and retirement. It may adapt the supported local transport logic above; remote/opaque/TLS fallback is not presumed. |
| Private API | `control-plane/internal/api/pdf_pilot_private_stage.go` | Rooted API-only stage, bounded streamed capture, durable digest/identity receipts and positive stage cleanup after exact retirement. |

Shared `execution_leases.go`, admission, runtime-install/Product and projections
remain the separate bridge reviewer's files. The dedicated
`backend.PDFPilotSpec` and private lease would need a reviewed additive
revision: `OutputStagePath` becomes **API capture state only**, never an SDK
writable bind. `RendererFileIdentity` records the API-created retained PDF,
while the frame and selected container identity bind its source. There is no
implicit conversion of existing host-bind source state into runtime approval.

Before either Create, the private current-source resolver must return one
immutable, approved declaration with the following exact fields; a request
value or preliminary inventory cannot fill a missing field:

| Declaration | Existing source constraint | Release state |
| --- | --- | --- |
| Actor/project/scope, approval revision/digest, engine/instance, attempt and both allocation IDs | `pdfPilotRequest`, `PDFPilotAllocationID`, current actor/source recheck under final mutation | Shape only; no authenticated pilot approval/source resolver. |
| Original face-0 font bytes and original grant | `main.go` expects exactly 19,484,784 bytes and SHA-256 `b76b0433203017ca80401b2ee0dd69350349871c4b19d504c34dbdd80541690a`; mount read-only at the one fixed path | Expected source value only. Wrapper-to-font, notice and original-file owner proofs are missing. |
| Renderer binary and its Go toolchain/import graph | Fixed `scripts/distribute-pdf-pilot` source and selected seven-module candidate inventory | No accepted built binary, module graph closure or binary receipt. |
| Authored driver | Proposed `cmd/pilot-driver` source, exact driver binary digest and source/grant receipt to be added to the immutable command/config digest | No driver source or binary exists yet. |
| Standard reader tools | Exact `pdftotext` and `pdftoppm` argv below; require selected executable hashes, recursive library/archive/notice receipts and one-page capability | No selected Poppler binary/package/image closure. Host-installed tools are not eligible. |
| Selected image and filesystem profile | Exact image digest/ABI/source/package receipt; fixed UID/GID, tmpfs options and readable font/PDF binds | No accepted image or target-engine UID/bind/ACL proof. |

The accepted declaration must be copied unchanged into both role identities,
stage receipts and command digests. A new selected image, binary, font,
permission profile or approved source requires a new attempt; never substitute
current mutable inventory into an existing allocation.

`render` runs the receipted existing Go pilot binary with exactly
`--font /input/NotoSansCJK-Regular.ttc --out-dir
/out/pilot-<AttemptID>`. The driver creates the empty `pilot-<AttemptID>`
directory under `/out` first, because `scripts/distribute-pdf-pilot/main.go`
requires that basename and emptiness. It waits for the one writer child to
exit zero, checks only `pilot.pdf` exists, opens it without following links,
reads at most 16 MiB+1, hashes it and emits one exact renderer frame before
the container exits. Fixed nonzero failure codes carry no raw parser output;
EOF, partial frame, overrun, nonzero child exit and uncertain SDK exit are not
evidence. Cancellation or deadline closes the owned attach and triggers the
independent exact-CID watchdog; it does not execute a replacement role.

The proposed binary stream begins with `LUCI-PDF-PILOT/1\n`, then a fixed
role byte, 32 raw allocation-digest bytes and 32 raw input-digest bytes
(original font for renderer, retained PDF for reader).
Renderer has exactly one `pdf` record; reader has exactly two ordered records,
`text` then `png`. Each record has its fixed one-byte type, big-endian uint32
length and 32 raw SHA-256 bytes followed by exactly that many payload bytes;
`LUCI-PDF-PILOT-END/1\n` closes the stream. The decoder rejects any other
record count/order, bytes after the end marker or stderr/other Docker streams.
It checks the declared length against the per-type bound and remaining
aggregate bound before allocating or writing, then hashes bytes as they are
streamed to the API-owned stage. Docker's 8-byte stdout multiplex headers,
including split headers/payloads, count toward a fixed raw maximum of
**34 MiB + 16 KiB**. A declared digest is an integrity check, not source
authority; exact owned container and approved input identities supply that.

`read` starts only after renderer output is durably captured **and** its CID
is positively retired. The exact retained PDF is the reader's single
read-only input bind at `/input/pilot.pdf`, with source file identity/digest
checked before Create and again before Start. The reader driver runs these
two already specified Poppler argv, one after the other and without a shell:

1. `pdftotext -enc UTF-8 -nopgbrk /input/pilot.pdf /out/pilot-<AttemptID>/pilot.txt`
2. `pdftoppm -f 1 -l 1 -singlefile -r 72 -png /input/pilot.pdf /out/pilot-<AttemptID>/page`

It waits for each exact child and refuses a nonzero exit, extra output files,
text over 1 MiB, PNG over 16 MiB, invalid UTF-8 or a PDF digest change. The
frame contains the exact PDF input digest and bounded text/PNG bytes. The API
accepts only the fixed original Unicode line `日本語` (apart from the documented
terminal line break), one 595×842 PNG and an independently checked one-page
PDF structure; partial extraction or replacement glyphs fail. Both tools and
their transitive libraries, original grants, driver binary and image remain
**unselected**. These argv specify behavior, not a claim that the reader can
run today.

The driver must keep both subprocesses in the one owned container/cgroup,
reap them and make its own cancellation terminate them. No detached process,
second reader container or borrowed document-extractor image. The Docker
watchdog independently retires the exact container and all its descendants if
the driver, attach, SDK read or API publication blocks. The pilot's 120-second
attempt envelope, 40/40-second role phases, <=30-second child run limits and
40-second cleanup reserve remain unchanged; a frame read cannot extend them.

## Nonblocking retirement and exact recovery

The effect lane takes the existing durable mutation lock only to publish each
transition, then releases it **before** native Create/Start/Attach/Inspect,
frame reading, disk sync or stage publication. The API stage has its own short
per-file guard but never holds `executionAdmission.mu`, `leases.lock` or the
cross-process mutation lock during synchronous native I/O. The existing
`distribute-work.lock` may serialize admitted workbook/PDF work, but exact-CID
retirement must not need that permit. A watchdog owns an independent selected
SDK call lane and bounded context; it is started with the durable allocation
claim and receives a known CID immediately. A blocked capture/publisher must
not precede its retirement. If the engine cannot confirm retirement, the full
512 MiB lease and partial stage remain unknown across restart; timeouts do
not erase them. A positive late retirement is historical cleanup only.

The framed attach needs its own finite handshake, transport authentication
and bounded raw Docker multiplex parser. The present document extractor
path supports only a selected local socket/pipe transport and has a distinct
small document-result protocol. Reuse its supported transport and ownership
mechanism by source review, **not** its protocol, image or credentials. The
new receiver must reject extra streams, stderr payload, duplicate/trailing
frames, wrong role/allocation/input digest, excess length or incomplete EOF.
Do not log raw PDF, text, PNG, font or command output. A lost Create or Start
ACK permits exact owned readback/retirement, never a second Create/Start.

## Finite proof before any effect

The next source checkpoint should contain the driver and inert split/chunk,
overrun, malformed frame, cancel, child-failure and two-role serialization
fixtures; private stage original-file/partial-write/parent-swap tests; and
selected-SDK fake attach/limit/CID/watchdog tests. Review the actual selected
Go binary and Poppler package graph/licences, image digest and ABI, fixed
numeric UID/GID and tmpfs option support, read-only font/PDF bind access on
the target Docker Desktop filesystem, selected transport headers, and API
private stage ownership. Missing proof is an explicit refusal. No current
source or retained preliminary GPG/index/readelf observation fills these
release fields.

# PDF pilot: dedicated host stage and selected SDK slice

**Source plan only, 16 October 2026.** This follows the reviewed receiver proposal
`99615db783f71a60a73b0361e4342888fbc0b78c` and the bounded private frame
candidate at `0256409500711787428e297d1cb044a32505918e`. The latter retains
at most the role's exact PDF or text and PNG payloads in memory; it proves
framing and hashes, not an exit, durable file, current authority, semantic
reader result or retired container. This plan does not enable a product route
or authorize an image, package, native tool, compiler or Docker operation.

## One caller and one existing ownership chain

The only proposed caller is an unregistered, effect-tagged
`(*api.API).runPrivatePDFPilot(ctx, request, executor)` in
`internal/api/pdf_pilot_private_run.go`. It accepts the existing
`pdfPilotRequest` and a purpose-specific `backend.PrivatePDFPilotExecutor`;
it accepts no user path, command, URL, Dockerfile or credentials. The caller
obtains the root through `a.execution.prepareExecutionDirectory()`
(`execution_leases.go`), uses the existing `distribute-work.lock` order before
the shared ledger mutation, and asks the separately reviewed shared bridge to
resolve current actor, approved source and resource capacity inside final
admission. `pdfPilotCurrentBinding` is a shape check only. Missing shared
authority/admission hooks stop before `Prepare` or `Create`; the dedicated
caller cannot invent a second lease or scheduler.

The current document path demonstrates the relevant pattern:
`document_extraction_runtime.go:219–332` journals one Create, attaches before
Start, rechecks admission, records exact exit and capture, then retires;
`document_extractor_result.go:129–235` binds a local selected SDK client and
pre-Start owned hijack. The PDF role follows that order with the existing
`pdfPilotLease`'s two allocation IDs and one 512 MiB charge. It does not reuse
the document image, command, framing, limits or automatic recovery decision.

## Dedicated files and typed joins

| File | Exact addition |
| --- | --- |
| `internal/backend/pdf_pilot.go` | Add `AttachPDFPilotFrame(context.Context, PDFPilotIdentity, string) (PDFPilotFrameStream, error)` to the dedicated executor. `PDFPilotFrameStream` is a single-use `io.ReadCloser` of **raw Docker multiplex bytes**, with idempotent close. Clarify that `PDFPilotSpec.OutputStagePath` identifies API-owned capture state and is never an output bind; reader `RendererPDFPath` alone is the exact read-only host input after renderer settlement. Do not change legacy executors. |
| `internal/backend/dockerbe/pdf_pilot.go` | Dedicated selected `*dockerbe.Backend` adapter for fixed Prepare/Create/Start/Inspect/Retire/fence, exact two command vectors from `pilot-driver/main.go`, image and inspect equality, numeric nonroot UID/GID, fixed labels, no network/IPC, read-only root, 512 MiB memory=swap, 1 CPU, 32 PIDs, `/out` 32 MiB and `/tmp` 8 MiB tmpfs. No generic command or source-build fallback. |
| `internal/backend/dockerbe/pdf_pilot_attach.go` | One local selected-client pre-Start attach per exact CID/identity, using the `document_extractor_result.go` owned connection/header/deadline pattern. Copy owner-declared headers; require supported socket transport; retain the hijack until consumed or closed. An opaque/remote/TLS transport refuses. |
| `internal/backend/dockerbe/pdf_pilot_original_windows.go`, `_other.go` | Windows held original file **and all ancestor** handles, one-link/regular/non-reparse identity and read-only sharing from before Create through positive role retirement; unsupported targets refuse. Check the original object and the selected bind mapping at Prepare, inspect, immediately before Start, and after exit. The reader original is the API-held renderer stage file. See the unresolved bind proof below. |
| `internal/api/pdf_pilot_private_stage_windows.go`, `_other.go` | From only the trusted execution-state root and retained attempt ID, create a private `pdf-pilots/<attempt>` directory and role files with exclusive no-replace, held parent/file handles, ordinary/single-link/ACL/identity checks; other targets refuse until separately proven. Never take a path from the request. |
| `internal/api/pdf_pilot_private_run.go` | One renderer then one reader state machine, exact durable stage receipt, current authority/fence before each first Start, positive exact CID exit/retirement, immutable old attempt readback and independent emergency stop. It calls reviewed shared ledger/admission helpers; it owns no shared schema or capacity arithmetic. |

The shared `execution_leases.go`, its `PDFPilots` record and accounting,
admission at every existing worker/check/preview/image/document/assembly join,
runtime installation, Product and projections remain the bridge reviewer's
files. This slice cannot compile into an active executor or run until those
exact helpers and selected immutable package/image profile are approved.

The dedicated API seam is concrete: `openPDFPilotStage(root string, lease
pdfPilotLease) (*pdfPilotStage, error)` owns rooted handles;
`(*pdfPilotStage).Capture(ctx context.Context, lease pdfPilotLease, candidate
pdfPilotFrameCandidate) (pdfPilotStageReceipt, error)` returns only exact
role/attempt/allocation/CID/input, length/hash and held file identity after
bounded write and flush; `(*pdfPilotStage).ReaderOriginal(ctx, receipt)`
returns only the still-held renderer file and its digest for the fixed reader
spec. The receipt is private evidence to be persisted through the reviewed
ledger mutation, not a standalone acceptance store. No stage mutex is held
across selected SDK calls or blocking attach reads; emergency retirement is
independent of stage I/O.

## Host stage: same PDF, durable before reader

`openPDFPilotStage(root, lease)` first verifies the exact persisted lease and
creates an exclusive rooted stage. Its typed result is a retained owner, not a
caller path. `Capture(role, pdfPilotFrameCandidate)` requires the copied
scope/attempt/allocation/CID/input from the live lease and writes only the
candidate's `renderer.pdf`, or ordered `reader.txt` and `reader.png`, from their
existing bounded slices. No raw stream or second payload buffer is retained.
It checks each byte count and digest again, writes at most 16+1+16 MiB of
payload across the attempt, and charges the separate 48 MiB stage reservation.
The role receipt binds the exact actor/scope/attempt/allocation/input/CID,
original file identity, length and SHA-256. Partial writes, a preexisting name,
failed flush/receipt publication or lost ACK retain the exact stage and lease
as **unknown**; no truncate, adopt-by-name, cleanup-by-time or replay.

The renderer file handle remains owned and its digest/bytes/file identity are
persisted before the reader can be prepared. The reader's
`RendererPDFPath` is derived only from that held file, and its input digest is
the persisted renderer output digest. The adapter must prove the read-only
bind resolves to that same original throughout `pdfinfo`, `pdftotext` and
`pdftoppm` reopens. No tempfs-after-exit copy, arbitrary host path or second
render is permitted. Only after both exact role retirements and durable
readback may cleanup remove its own stage and release the one charge. The
existing `pdfPilotFrameCandidate` does **not** itself pass text/PNG semantics;
the fixed reader checks and independent source/glyph verification remain
separate gates.

The target Windows filesystem durability protocol is still **unproven**:
exclusive handle creation, file flush, directory-entry persistence and a
retained stage receipt must all be demonstrated under the selected private
root. The 48 MiB charge is accounting, not a filesystem quota. API-only writes
and bounded candidates cap this attempt's own output; fresh whole-volume
capacity and a positive owned cleanup observation remain required. A source
fixture alone cannot claim crash-durable bytes.

## Two once-only roles and independent stop

The shared ledger reserves deterministic renderer and reader allocation IDs
up front. Before each Create, the dedicated caller verifies selected engine,
actor/source approval, original handles, host headroom and the exact package
binding. It journals that role's `creating` transition **before** the SDK
effect. An uncertain Create ACK retains that allocation; only exact
selected-engine CID/labels/config/fence readback or retirement is permitted.
Neither a new Create nor a late attach is recovery. A confirmed CID is
persisted before Start and immediately arms a separately owned exact-CID
watchdog. Pre-Start attach must succeed and be claimed once; otherwise Start
is never sent. Fresh current authority, input guard, selected inspect and
launch-fence verification precede the one journaled Start.

The attach owner closes its own connection on cancellation/deadline, without
pretending a context interrupts a blocked `Read`. The decoder starts no
goroutine. If a separate caller capture task remains blocked, its owned
stream-close/settlement is tracked and the lifecycle lock/lease stay held;
the watchdog's independently selected client/context retires the exact CID
without taking the capture, stage, publisher or ledger mutex. Follow the
`document_image_assembly_watchdog.go` exact-reference lane, not a shared
engine reset or unrelated process kill. Missing positive exit zero/non-OOM,
complete frame/EOF, stage receipt or positive absence makes the result
unavailable. It cannot authorize the reader. After renderer capture and
positive retirement, reader Create/attach/Start repeats with its separate
allocation; both stay within the original 40/40 phases and 120-second whole
deadline including reserved cleanup. A late positive fact is historical
cleanup, never retroactive pilot acceptance.

## Required original and package proof still absent

`document_image_assembly_sources_windows.go:93–176` shows an existing
`CreateFile`/`FILE_SHARE_READ` original-and-ancestor handle pattern that denies
ordinary host writes, deletes and renames while held. It is **not** a PDF
guard, nor proof that Docker Desktop translated a Windows path into that
same immutable guest bind across child tool reopens. The new guard must show
both: held original identity/bytes/ACL and selected engine mount-to-original
identity for each role, including the stage PDF. The Linux driver currently
rehashes on entry/exit and opens ordinary files with no-follow; those checks
cannot cover an intervening namespace swap. If the target OS/engine cannot
establish the mapping and held-name guarantee, refuse before Create/Start;
periodic path hashes do not substitute.

There is no accepted pilot image ID, packaged `pilot-driver` or writer binary,
Poppler `pdfinfo`/`pdftotext`/`pdftoppm` and recursive dependency/licence
closure, font wrapper-to-exact-original grant, target numeric UID/ACL mapping,
verified selected attach headers, runtime bind proof or approved Product
source resolver. Existing host tools, document extractor image, provisional
index outputs and source fixtures cannot populate those fields. The next
reviewable implementation can compile guarded dedicated types/fakes, but
the first actual Create remains blocked until each original/package/engine
fact is independently closed and current shared authority is wired.

The first dedicated fake regressions should prove exclusive stage creation
and retained partial identity, parent/file replacement refusal, one exact
Create and no replay after lost ACK, attach failure before Start, blocked
capture versus watchdog retirement, changed approval before first Start,
renderer capture with nonzero exit refusal, renderer retirement before reader
Create, and unchanged input file across the reader's three fixed reopens.
These are source checks, not a substitute for selected Windows bind/ACL,
durability or package proof.

# PDF pilot: stage-first private owner addendum

**Source plan only, 16 October 2026.** This narrows the reviewed
[host-stage/SDK plan](pdf-private-pilot-host-stage-sdk-slice-v2.md) to one
dedicated API stage owner. It does not implement an SDK adapter, attach,
executor, shared lease/admission/store change, original-input grant or public
route. The bounded frame decoder at `7423194df7ce1788b02fb39be847bf9cab06ec5c`
produces only an in-memory capture candidate. No compiler, parser, test,
native API or Docker call has verified this addendum.

## Exact source boundary

| Dedicated file | Proposed responsibility |
| --- | --- |
| `control-plane/internal/api/pdf_pilot_private_stage.go` | Common unexported stage owner, immutable claim/candidate/receipt types, finite counters, role order, pending-operation states and settlement rules. No `os.OpenFile` path fallback. |
| `control-plane/internal/api/pdf_pilot_private_stage_windows.go` | Windows rooted original handles, exclusive creation, bounded file I/O, identity/ACL observations, writer seal and held read-only file handoff. Native calls belong here; no ledger mutex is held through them. |
| `control-plane/internal/api/pdf_pilot_private_stage_other.go` | Explicit unsupported result on other platforms. A Linux host path implementation is not inferred from the Linux child driver. |
| `control-plane/internal/api/pdf_pilot_private_stage_test.go` | Inert common state/limit/late-result fixtures using an injected stage operations seam. No filesystem or SDK effect. |
| `control-plane/internal/api/pdf_pilot_private_stage_windows_test.go` | Deferred exact Windows original-handle/ACL/name-swap/flush/reopen/close fixtures. Their source is not Windows execution proof. |

The constructor is **not** `open(root string, attempt string)`. Proposed
`openPDFPilotStage(ctx context.Context, owner *executionAdmission, claim
pdfPilotStageClaim) (*pdfPilotStageOwner, error)` accepts an unexported claim
minted only by a reviewer-owned, final shared-ledger read of the exact
`PDFPilots` lease. The claim copies actor/scope/attempt/role/allocation,
selected engine/CID, original input digest, expected stage root identity and
ledger generation. The constructor itself obtains the trusted root through
`owner.prepareExecutionDirectory()` (`execution_leases.go:98–136`), opens
and holds its native root and every ancestor, checks the current lease and
root identity, and refuses a path or claim supplied by HTTP, desktop or
driver output. That shared claim helper does **not** exist yet; until it is
reviewed, the stage cannot open. No second authority store is proposed.

Common owner methods are `CreateRole(ctx, claim) (pdfPilotStageCreate, error)`,
`WriteCandidate(ctx, claim, pdfPilotFrameCandidate) (pdfPilotStageWrite,
error)`, `SealRole(ctx, claim) (pdfPilotStageSeal, error)`,
`Readback(ctx, claim) (pdfPilotStageReadback, error)`, and
`CloseRole(ctx, claim) (closed bool, err error)`. Each method checks the
same copied claim against the current held owner and role state. Typed
results distinguish `acknowledged`, `create-unknown`, `write-unknown`,
`seal-unknown`, `readback-unknown` and `closed-confirmed`; an error or
cancelled call never implies absence. A late ACK is attributed only to its
original operation/claim; no second Create, writer reopen or reader role is
issued because the first response was lost. The three permitted payload
names are exactly `renderer.pdf`, `reader.txt`, `reader.png` in one
`pdf-pilots/<AttemptID>` directory, with exclusive no-replace creation.
The private `pdf-pilots` parent and attempt directory are two further
once-only Create intents. These
states must be persisted through the reviewer-owned existing lease before
the next effect; this stage file is not an independent journal.

## Finite owner limits

All limits count **attempt lifetime**, including partial and failed calls;
an unknown result never refunds a slot. The owner refuses before the
operation whose upper bound would cross a limit.

| Limit | Proposed cap |
| --- | ---: |
| Native original-handle opens by stage plus the future original-input guard, including root/ancestors/data files | 96 over attempt lifetime, with at most 64 ancestor components; no more than 96 simultaneously |
| Pending or completed Create intents | 5 exact names: purpose directory, attempt directory and three role files; one Create per name |
| Read-only reopen intents after sealing | One per file; a lost ACK remains unknown, not retried |
| Payload written | `renderer.pdf` ≤16 MiB, `reader.txt` ≤1 MiB, `reader.png` ≤16 MiB; ≤33 MiB across attempt |
| Readback of retained payloads | At most two complete bounded passes per file and ≤66 MiB of API readback across the attempt; no path-based third pass |
| Private receipt/metadata written | ≤64 KiB across attempt; payload plus metadata <48 MiB charged stage reservation |
| Stage-owned physical I/O | ≤128 MiB read+write including known readbacks; ≤4096 native read/write calls, each requested chunk ≤64 KiB, counting failures/EOF/no progress; ≤8192 total native stage calls including create/open/stat/flush/close failures |
| Capture decoder | Existing raw Docker bound 34 MiB+16 KiB and ≤65536 underlying calls, independent of stage accounting |

`WriteCandidate` consumes the decoder's existing exact payload slices; it
does not copy the full raw stream or allocate a second payload buffer. It
reserves the requested upper bound **before** each Windows write/read and
reconciles only a valid returned short count. Unknown/error/no-progress
retains the full reservation and stops that operation. The 48 MiB stage
charge is shared admission accounting, **not** an OS-enforced per-directory
quota. Fresh volume space and the retained 512 MiB whole attempt charge
are separate admission obligations.

The 128 MiB/call counters must be durably retained by the reviewed shared
lease before any crash-restart readback. Without that insertion, they are
bounded only for the current held owner; reopening the API can inspect
historical status and retire a proven exact CID, but cannot restart stage
readback, Create or reader admission. This makes the aggregate bound
truthful rather than resetting an in-memory count on each process restart.

## Same file: write, flush, protect, read-only hold

The Windows owner first holds the trusted execution root and every parent
using ordinary non-reparse handles and original volume/file-index/attribute
facts. It creates the attempt directory and each file with `CREATE_NEW`,
checks the returned handle rather than a later path alone, requires a
single-link ordinary disk file, exact parent identity and private ACL, and
holds the original handles throughout the operation. A captured record is
written at bounded offsets to the original writer handle and hashed from
those exact bytes. `FlushFileBuffers` and a verified length/hash/file
identity precede any stage receipt or reader eligibility. A failed flush
leaves the stage unknown and charged.

The **same file** must then become a continuously write-excluded, held
read-only renderer original before the reader is prepared. `DuplicateHandle`
is insufficient: it retains the original writer access/sharing. A
permissive `ReOpenFile` is also insufficient: it does not establish
continuous exclusion of writes across close/reopen. A possible native seal
sequence is to apply and verify a protected ACL that denies new write,
append, delete and ACL-change access **while the writer is still open**;
flush; close the writer with an explicit `closed == true` result; reopen the
same file for read only with `FILE_SHARE_READ` (no write/delete sharing);
compare the original volume/file index, parent identities, security
descriptor, length and hash; and hold that read-only handle through the
reader's positive retirement. Any failed or ambiguous ACL change, Close,
reopen, identity check or intervening namespace change leaves
`seal-unknown` and forbids the reader. No duplicate handle or name-only
substitution can promote it.

**That ACL/owner-rights transition is not yet proved on the selected
Windows identity and filesystem.** In particular, the file owner may have
implicit security-descriptor powers and a Docker Desktop bind uses a host
path rather than this handle. The reviewed Windows source must demonstrate
continuous exclusion over the close/reopen gap and the selected engine's
bind-to-original relation. If either is unavailable, `SealRole` refuses;
the stage-first implementation may retain bounded bytes but cannot start
the reader. The existing assembly original-handle pattern
(`document_image_assembly_sources_windows.go:93–176`) is a starting pattern,
not this proof. The Linux child's no-follow/re-hash checks do not close a
host namespace swap between its `pdfinfo`, `pdftotext` and `pdftoppm` opens.

## Retirement and restart are separate facts

The stage owner holds no mutex while calling native file I/O, an SDK method
or a publication callback. A small in-memory state lock may select a
once-only intent and publish its return, but is released before the call.
The run owner has a separate exact-CID retirement latch armed immediately
after a known Create ACK; its independent selected-client stop/absence lane
does not take the stage lock, wait for a blocked writer/readback or delete
an unrelated process. `retired == true` only after exact positive SDK
retirement **and** exact absence readback. A false/unknown return retains
the CID, stage, handle ownership and full charge; cancellation, timeout or
`CloseRole` failure cannot set the latch. If a late ACK arrives after the
watchdog fired, it belongs to that original intent and needs exact
retirement; it never starts a replacement role.

Current-process held file bytes plus successful `FlushFileBuffers` are one
fact. **Crash-restart recovery** additionally needs a durable ledger claim,
root/file identity, ACL/namespace proof, persistently charged counters,
exact original input grant and selected CID fence. Process death releases
the held Windows handles; a matching pathname/hash cannot recreate their
continuous lifetime or retroactively authorize a reader. Until that
recovery contract is reviewed, reopen is read-only status plus exact owned
retirement, with output unavailable. The missing immutable pilot image,
writer/Poppler package and licences, numeric UID/ACL relation and
Windows-to-Docker original mapping remain explicit first-effect blockers.

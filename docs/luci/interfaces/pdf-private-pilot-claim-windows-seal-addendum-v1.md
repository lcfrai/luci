# PDF pilot: shared claim and Windows writer-seal prerequisites

**Source plan only, 16 October 2026.** This makes the previously proposed
[stage-first owner](pdf-private-pilot-stage-first-addendum-v1.md) contingent on
specific existing admission state and a demonstrable Windows file seal. No
claim helper, stage, ACL transition, SDK call, reader or product route is
implemented by this addendum. No compiler, tests or native call was run for it.

## Existing owner and the smallest shared insertion

`internal/api/execution_leases.go` has `executionAdmission{api,mu,path,scope}`.
`newExecutionAdmission` derives `scope` from the configured conversation root
and `path` from `ExecutionStateDir` or its private `.execution/leases.json`.
`prepareExecutionDirectory` validates/creates the private root but returns
**only a path**. `g.mutate(ctx,fn)` holds `g.mu`, the exact `leases.lock` and
the bounded Version 1 `executionLeaseLedger` through validation and atomic
replacement; it never supplies a held stage-root handle. The ledger currently
has **no `PDFPilots` field**. Its 2 MiB JSON, 4096 lease and reservation-view
checks cannot count a proposed PDF pilot until changed. Existing
`pdfPilotRequest`, `pdfPilotCurrentBinding`, `pdfPilotLease` and
`validPDFPilotLease` in `pdf_pilot_private.go` validate shapes/chronology;
none resolves current Product/source approval or grants an allocation.

| Owner | Exact proposed file and addition |
| --- | --- |
| Shared bridge reviewer | `internal/api/execution_leases.go`: additive `PDFPilots []pdfPilotLease` with JSON field `pdfPilots,omitempty` in Version 1; unique scope/attempt/allocation validation, aggregate count/2 MiB limit, live 512 MiB + 48 MiB stage reservation view, and every existing worker/check/preview/document/image/assembly admission sum. Old records remain readable. |
| Shared bridge reviewer | `internal/api/pdf_pilot_private.go`: additive bounded stage operation state on `pdfPilotLease`, checked by `validPDFPilotLease`; no stage path, payload or handle serialized. |
| Shared bridge reviewer | New `internal/api/pdf_pilot_private_stage_admission.go`: unexported `pdfPilotStageSelection`, `pdfPilotRootIdentity`, `pdfPilotStageClaim`, `pdfPilotStagePermit` and the three mutation methods below. No independent store or issuer. |
| Shared bridge reviewer | Its separate final reservation/first-Start Product and selected-source resolver: under the established lock order, read **current** actor, approved source, grant, selected engine and capacity. `validPDFPilotAdmissionBinding` alone is insufficient. This resolver is absent today; claim/effects fail closed until it exists. |
| Dedicated PDF owner, later and only after review | `pdf_pilot_private_stage.go`, `_windows.go`, `_other.go` and focused tests from the stage-first addendum. These consume a shared claim but never mutate the shared ledger directly or edit the reviewer's shared files. |

The shared helper's proposed signatures are
`(*executionAdmission).beginPDFPilotStage(ctx, selection pdfPilotStageSelection, observed pdfPilotRootIdentity) (pdfPilotStageClaim,error)`,
`claimPDFPilotStageOp(ctx, claim, kind, upperBound) (pdfPilotStagePermit,error)`
and `settlePDFPilotStageOp(ctx, permit, observation) error`. `selection` is only
the exact persisted scope/attempt/role/allocation key, **not authority**.
`observed` is volume/file identity obtained from a held Windows handle opened
by the dedicated owner from `filepath.Dir(g.path)` after
`g.prepareExecutionDirectory`; a request, desktop, driver frame or arbitrary
string cannot supply the root. The owner must also hold/check every ancestor
against the rooted original, not merely pass back a path or hash.

`begin` calls `g.mutate`, locates exactly one valid live `PDFPilots` row,
checks its actor/scope/approval/source/engine/role/allocation and the
authoritative current resolver, and CAS-persists the observed root identity
and a monotonically increasing stage generation. It refuses an already
pending or unknown different root. The returned unexported claim copies the
ledger generation, root identity and exact lease/request binding. The stage
constructor accepts only `owner *executionAdmission` plus that claim and
requires its own held root identity to match. It must not reopen a mutable
request path as authority. The root observation itself has no right to create
a directory or start a child before this claim is durable.

`claim...Op` uses another `g.mutate` to recheck exact claim generation,
retained lease, current actor/source/engine authority, whole deadline, fixed
file/role order and budget. It records a unique once-only operation intent and
debits its **full** upper-bound bytes/calls/handles before any native effect.
The permit names exactly one Create, write, flush, ACL transition, seal,
readback or Close and its original claim/generation. An uncertain call remains
pending/charged. `settle...Op` records only the returned observation for that
permit; a late ACK cannot mint a new permit, reset a counter or rewrite a
different generation. No ledger mutex or `leases.lock` spans native file I/O,
SDK, capture or publication. The later run owner must provide the independent
exact-CID watchdog and positive retirement path described in the earlier
source plan; this claim helper does not implement either.

The durable stage field needs fixed operation IDs/states, root/file original
identities and returned lengths/digests, at most 96 owned-handle opens, five
Create names, three read-only reopen intents, 128 MiB requested physical I/O,
4096 requested read/write calls, 8192 total native calls and the separate
payload/readback limits from the stage-first addendum. It retains counters
across API restart with one current operation and monotonic sequence/counters,
not an 8192-entry log that could exceed the existing 2 MiB ledger. A settled
operation advances the sequence without resetting its debits; only the
documented five Create names and three reopen names have per-name terminal
states. The validator rejects counter rollback, reused operation IDs and
an impossible file/role transition. It **never** treats a serialized identity as a held
handle. A missing field, unreadable ledger, stale claim, lost Create/Close
return or process restart permits status and exact owned-CID retirement only;
no reissued stage or reader effect. The still-missing original grant and
selected engine/source resolver must be wired by the reviewer before any claim can be
used; the PDF owner may not synthesize them from `pdfPilotRequest`.

## Windows same-file seal: required proof, not an implemented recipe

Primary Windows documentation at pinned MicrosoftDocs source says
`CreateFileW` access/share modes must be mutually compatible with existing
opens, and `ReOpenFile`'s new sharing must also be compatible with the
original handle's access. A writer opened with write access therefore cannot
simultaneously obtain a read-only `FILE_SHARE_READ` handle that excludes
writes merely by `ReOpenFile`. `DuplicateHandle` preserves the existing
writer capability. The source of these rules is
[CreateFileW](https://raw.githubusercontent.com/MicrosoftDocs/sdk-api/656e9656bfaef34dd29cd7ee9e1146aec17e924c/sdk-api-src/content/fileapi/nf-fileapi-createfilew.md) and
[ReOpenFile](https://raw.githubusercontent.com/MicrosoftDocs/sdk-api/656e9656bfaef34dd29cd7ee9e1146aec17e924c/sdk-api-src/content/winbase/nf-winbase-reopenfile.md).

For one original file, the required current-process sequence is: hold the
private root and ancestor originals; `CreateFileW(CREATE_NEW)` once with a
restricted initial DACL; write bounded bytes to that original writer;
`FlushFileBuffers`; verify original file index/volume, one-link ordinary
attributes, parent identities, length and SHA-256; apply a protected DACL
**on the original handle** and verify it; close the writer with a positive
return; `CreateFileW(OPEN_EXISTING, GENERIC_READ, FILE_SHARE_READ)` through
the held parent; compare original identity/security/length/hash again; hold
this read-only handle through the reader's positive retirement. Every
transition has its own persisted once-only permit. Failed flush, ACL write,
security readback, writer Close, exclusive read open or identity comparison
is `seal-unknown`; there is no path-based retry or reader Start.

An ACL written by the same API process is **not yet a seal**. MicrosoftDocs
[SetSecurityInfo](https://raw.githubusercontent.com/MicrosoftDocs/sdk-api/656e9656bfaef34dd29cd7ee9e1146aec17e924c/sdk-api-src/content/aclapi/nf-aclapi-setsecurityinfo.md)
notes that a NULL DACL grants full access. MicrosoftDocs
[Owner of a New Object](https://raw.githubusercontent.com/MicrosoftDocs/win32/ca8274decead49403d70ceeed4af8d4ca74eaf11/desktop-src/SecAuthZ/owner-of-a-new-object.md)
states that an object owner implicitly has `WRITE_DAC` and can change its
DACL. The normal creator-owner could undo its own attempted write denial
during the close/reopen gap. Before implementation, the selected identity
must prove a protected non-NULL DACL and a concrete owner/OWNER RIGHTS or
distinct trusted owner arrangement that prevents this API identity and every
other admitted writer from regaining write, append, delete, `WRITE_DAC` or
`WRITE_OWNER`. Required token privileges, filesystem semantics and cleanup
rights must be pinned and verified; no assumed deny ACE overrides implicit
owner rights. If that model cannot be established, the seal and reader are
unavailable. A read-only Docker bind is also **not** proof that the guest
reopens this held original; the selected Windows-to-Docker bind identity
and each fixed `pdfinfo`/`pdftotext`/`pdftoppm` reopen remain a separate
first-effect prerequisite. Current-process held handles do not prove
crash-restart continuity or durable directory entries.

Fixed proposed native/security budget, included in the persisted 8192-call
attempt cap: at most 96 opened stage/input handles and 64 ancestors; three
file DACL writes, six DACL/owner readbacks total; at most 16 KiB per security
descriptor, 64 ACEs per descriptor and 64 KiB total security bytes read or
written over the attempt. Oversized descriptors, unsupported ACEs, NULL or
inherited broad-write DACLs refuse before changing an original. Each
requested read/write is at most 64 KiB and debited before call. These caps
are proposed implementation limits, **not evidence** that Windows or Docker
currently satisfies the owner/bind contract.

## Required inert negative fixtures before any effect release

The reviewer's shared-ledger fixtures must cover absent PDF row, conflicting
scope/allocation, stale approval/source/engine, root identity changed after
claim, generation CAS failure, late Create ACK, failed/unknown Close, restart
with only path/hash, full pending-counter debit and no second operation.
Dedicated Windows-source fixtures must cover a swapped ancestor or same-name
different file ID, hard link/reparse, NULL/inherited DACL, creator-owner still
holding `WRITE_DAC`, descriptor >16 KiB or >64 ACEs, failed Set/flush/close,
write-compatible reopen, duplicate retained writer, and guest bind target
different from the held original. The last two are refusals, not simulated
proof that the selected Docker mapping works. A blocked stage call must not
delay the exact-CID watchdog. No reader may be admitted from an unknown seal
or from a restarted process's name/digest alone.

The selected immutable pilot image, tool/font/package closures, numeric
UID/ACL mapping, original source grant, selected bind proof and crash-safe
stage recovery are still unavailable. This addendum supplies the reviewer
with exact source ownership and claim inputs; it grants no runtime effect.

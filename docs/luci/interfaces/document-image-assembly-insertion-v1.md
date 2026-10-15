# Document-image assembly: exact additive shared seam

Source implementation only; no route/autostart/acquisition/compiler/runtime grant.
Existing inspector73550716 is frozen; LOCAL097 byte correction40faa3c2 preserves its
manifest/lock pins. Accepted document extraction lifecycle0b8b33c6 is unchanged.

Dedicated source owns `backend/document_image_assembly.go`,
`dockerbe/document_image_assembly*`, `api/document_image_assembly*` and the static
helper in `images/document-extractor/assembler`. Shared insertion remains cloud/root
owned and must not run until separately reviewed/wired.

## Exact cloud fields/calls

Under the existing shared execution ledger, add only:

```go
ImageAssemblies []documentImageAssemblyLease `json:"imageAssemblies,omitempty"`
ImageAssemblyFences []documentImageAssemblyFenceRecord `json:"imageAssemblyFences,omitempty"`
```

Call `validateDocumentImageAssemblyRecords(values,fences)` at every complete-ledger
read/write validation, retaining the existing aggregate byte/history bounds.
Call `addDocumentImageAssemblyCapacity(snapshot,values,skipScope,skipID,&demand)`
from **every** worker/check/preview/image/document/assembly admission path under
the same existing lock. Call `addDocumentImageAssemblyReservationView` in existing
reservation projections. Expired/unknown/cleaning/fencing leases remain charged;
only positively finished receipts release them. Existing actor/team limits still
apply; this assembler must additionally hold the serial heavy-work slot.

Each lease fixes backend `DocumentImageAssemblyResources()` =2048MiB/1000mCPU,
BaseImageID, exact full immutable identity,300s compute+120s cleanup=420s whole,
daemon create-disk estimate, **2GiB host capture disk**, and the selected Windows
launch allowance (normal2GiB where required). Observed exact CID/kind covers its
container demand only; host capture/launch reservations remain independent.
No clock expiry or unidentified container can zero those claims.

The exact private typed runtime hooks are declared in
`api/document_image_assembly_types.go` as `documentImageAssemblyHooks`: Authorize,
Admit, Lock, Transition, HoldFence, RecordFence, Finish, Headroom, CaptureSink,
Publish. They adapt existing current installation authority/ledger/CAS/lifecycle
lock/host observation/private receipt primitives. They are not user callbacks,
another store/controller or a public API. Nil hooks leave this optional feature
unsupported. No API Deps/New/Product field/route is allocated by this checkpoint.

`Admit` returns the exact durable lease after current-grant and fresh8GiB physical/
commit CPU<50/capture-volume/disk checks. A repeated admission can only revalidate
the same reserved/created identity/resources/deadlines and must never allocate a
replacement. `Transition` journals exact before/after states before each RPC.
`Finish` accepts only proven absence, reader settlement, durable full capture or
explicit failed/interrupted outcome; never a caller success label or expiry.

## Docker shared recognizers, later dedicated checkpoint

The dedicated adapter will expose the pure same-package recognizer
`inertDocumentImageAssemblyFenceLabels(container.InspectResponse) bool`; include
it only in the existing inert-fence shape validation after its source is reviewed.
The resource observer's managed-kind projection must recognize only the exact
`luci.document-image-assembly=true` marker as kind`document-image-assembly`, with
the exact dedicated identity and supported observed limits. It must not classify
agent/check/parser/foreign markers as this purpose, or turn observation into a
start/adoption/retirement grant. Those shared lines are not edited by local.

## Lifecycle and resource facts

The separate executor methods are Prepare/Create/pre-Start Attach/Start/Inspect/
Retire/Fence/Verify; each requires the exact backend identity. UID0, reviewed cap
allowlist, readonly root,2GiB /work tmpfs+128MiB /tmp, memory=total-swap2GiB,1CPU,
64PIDs, networknone and no SYS_ADMIN/host mounts/proc fallback are fixed profile.
The helper/profile/source/input/command digests are immutable. A lost Create/Start
ACK fences; later inspection cannot substitute for a real ACK. Attachment never
recovers late. Emergency retirement of known CID precedes blocked capture/receipt
I/O; unknown lease/reader cleanup remains held. OCI capture is streamed into an
owned bounded host artifact, not accumulated in a2GiB byte slice. Image import/
registration is a distinct future effect; no helper can call ImageBuild/Pull/Load.

Current source declarations are not compilation or recipe compatibility evidence.
The closed manifest constructor cannot use the unresolved existing SBOM lock or
invent a completed source/notice/authentication claim. Current authority/source
revalidation still runs before both Create and Start. No shared insertion, default
configuration or new runtime is authorized by this document.

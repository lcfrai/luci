# Private document assembler shared adapter

Source-only continuation from exact main798aaba3 plus reviewed dedicated
LOCAL100/103 successors. No public route, API Deps/New/Product field, startup
registration, image acquisition/build/import or model/parser execution is added.

## Concrete entrypoint and hooks

Add only an unregistered same-package method:

```go
func (a *API) executeDocumentImageAssembly(
    ctx context.Context, executor backend.DocumentImageAssemblyExecutor,
    req documentImageAssemblyRequest,
) (documentImageAssemblyOutcome, error)
```

The explicit executor is the reviewed pinned constructor, never a backend fallback
or a dependency selected from environment. Request/source validation remains a
mandatory server-owned function. Authorize reuses current
`a.authorizeRuntimeInstaller`, not ordinary worker/check permissions, and checks
the actor digest, installation scope, immutable request identity and current
source/grant again before Create/Start. Managed users cannot inherit a saved
authority snapshot. No existing installer path calls this method.

Dedicated `document_image_assembly_shared*.go` implements all existing typed
hooks using the same `executionAdmission.mutate` complete ledger and cross-process
lock: exact reservation, forward-only CAS transitions, durable fence intent,
actual acknowledged verified fence publication, and positive completion. Lock
reuses `lockChildLifecycle` with a purpose-prefixed full-identity digest. No global
capacity lock spans Docker calls. Lost/expired/unknown claims remain present;
reserved replay does not allocate another attempt, and allocated replay refuses.

The private capture writer lives under the existing confined execution state
directory, in an exact identity-named directory. Exclusive creation, ordinary
single-name file validation, private permissions, pre-write 2GiB limit, bounded
chunked hashing and fsync/close are required. Publish retains the actual complete
capture hash/size and bounded outcome in private state before ordinary retirement.
Finish compares that durable exact receipt and requires reader/watchdog settlement
plus a fresh exact absent-container or verified real-ACK fence witness; caller
labels alone cannot release resources or register an image. Cancellation/pressure
keeps the reviewed independent emergency stop lane ahead of reader/publication.

## Shared accounting and recognizers

Add ImageAssemblies/ImageAssemblyFences to executionLeaseLedger; validate all
records, shared allocation identity collisions, combined4096 lease/fence limits
and existing2MiB full-ledger read/write bound. Add assembly memory/CPU/create-disk,
host-launch and2GiB host capture reservations to worker, check, preview, runtime
image and document admissions, plus the assembler's own reservation. Expiry and
unknown ownership never remove charge. Both cached and read-only reservation
projections include documents and assemblies; the latter presently omits documents
at798 and needs its narrow projection insertion as part of this join.

Fresh capture-volume capacity must refer to the exact private artifact directory,
not assume the snapshot's `state` role names the same volume when ExecutionStateDir
is overridden. Reuse `resourcebudget.ObserveDisk` on that confined directory and
require fresh measured availability for aggregate retained assembly captures plus
the configured reserve. This is read-only native capacity observation, not another
command runner; an unavailable observation refuses admission. All existing
admission families retain this aggregate capture constraint. The assembler also
holds a serial assembly slot and refuses competing active/unconfirmed execution
at its own admission; existing starts must not crowd its serial heavy-work slot.

Docker gets exactly the inert assembly fence recognizer in inertFenceContainer,
and the assembly managed-kind projection. The latter requires the exact dedicated
label shape/identity and cannot erase demand for an agent/check/parser/foreign
marker. Observation confers no start/adoption/stop authority.

## Source files and deferred checks

Allocated existing files: execution_leases.go, execution_check_leases.go,
execution_preview_leases.go, execution_runtime_image_leases.go,
document_extraction_reservation.go, launch_fence.go, resource_observer.go, and the
necessary narrow execution_reservation_view.go projection. Dedicated shared
adapter/store and test files remain under document_image_assembly*. No coordinator,
management, distribution, auth or shared boot files are changed.

Meaningful inert sources will cover cross-class resource pressure, expired and
foreign-scope capture claims, exact current operator authority, CAS/replay,
durable fence ACK, hostile/mismatched retained records, byte/count bounds,
capture write/publication failures, positive absence versus caller labels,
read-only projections and exact Docker fence/marker purpose. Root reviews the
coherent source before one cloud compiler/inert batch. All real Windows handles,
container/source lifetime, dependency closure, native recipe and OCR acceptance
remain outstanding; source fixtures are not that proof.

## Implemented source checkpoint details

The dedicated shared adapter now implements all ten typed hooks and the explicit
private Execute method. It resolves current installer authority at every admission
boundary (including after the shared lock wait), retains source validation,
refuses allocated replay, and uses the existing lifecycle lock for both execution
and explicit retire/fence-only recovery. New inert fence admission uses the existing
`backend.MaxLaunchFences` across all four purposes. The complete ledger preserves
the historical4096 combined fence read/settlement bound for valid legacy mixed
histories; quota changes cannot render old ownership/cleanup unreadable.

Capture files are exclusively created with a pre-write2GiB cap and fsync/close.
A bounded private receipt retains a separately verified exact exit0/non-OOM fact;
normal retirement cannot substitute an unsupported caller success label. Finish
compares that exact durable outcome, verifies actual file bytes and a fresh exact
absence or the acknowledged fence, then commits the finished lease/fence in one
existing-ledger mutation. No import/registration/publish route is present. A
private deterministic disk-probe seam supports inert tests; production defaults
only to the existing native ObserveDisk on the confined artifact volume.

Nine new regression functions cover complete bounds/collisions and both
reservation projections (including the Documents omission), actual shared serial
refusal at every worker/check/preview/image/document admission, expired/foreign
capture claims, source/current-authority CAS, complete private capture/exit versus
unknown absence, pre-write cap/mutation/exclusive ownership, failed publication,
exact fence ACK/replay/current proof, and both shared Docker recognizers. These
sources have not been executed. Also run the prior LOCAL100 standalone module,
LOCAL103 watchdog and affected worker/check/preview/image/document accounting and
fence/recovery regression suites in the root-reviewed coherent cloud batch.

LOCAL108 recovery preserves the shared lease's previously published capture as
`RetainedCapture` (digest/size history), while `Capture` stays empty and state
explicitly says capture unavailable. The private receipt retains whether the
past capture had a verified exit; a missing receipt makes that fact unknown,
without inventing available/current bytes. Explicit recovery can settle exact
resources through a fresh real-ACK fence even when artifact bytes are missing.
It never reattaches, accepts the old capture as a new assembly result, or erases
the historical lease capture. Dedicated private-hook regressions model process
loss after successful capture publication with present/missing artifact or
receipt; these sources remain unexecuted.

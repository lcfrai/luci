# Contained Discover extraction insertion contract

This checkpoint is source implementation only. No image, dependency acquisition,
runtime launch, real PDF/OCR or Docker lifetime acceptance is authorized or proven.
The fixed image inventory remains unreleased and incomplete.

Cloud owns these small existing-file insertions; the dedicated implementation
owns the referenced types/helpers. Do not use CheckExecutor as the dependency.

- API `Deps.DocumentExtractor backend.DocumentExtractionExecutor` is optional.
  API initialization calls
  `deps.Product.SetDiscoveryExtractionRuntime(a.documentExtractionRuntime())`.
  The selected local Docker owner constructs
  `dockerbe.NewDocumentExtractor(selectedBackend, exactImageID,
  dockerbe.DocumentAttachConfiguration{Headers: exactSelectedHeaders})` only from
  trusted configuration. An explicit nonnil empty map is the normal SDK profile.
  `DocumentImageID() string` returns that immutable configured ID without lookup.
  No environment engine fallback, image pull/build or automatic enablement.
- Product `Server.discoveryExtractionRuntime DiscoveryExtractionRuntime` and
  `SetDiscoveryExtractionRuntime(value DiscoveryExtractionRuntime)` assigning
  `s.discoveryExtractionRuntime = prepareDiscoveryExtractionRuntime(value)`.
  The dedicated initializer creates only an owned cancel/job registry, no poller.
- Existing `registerDiscoveryDocuments` POST `/{documentId}/extractions` handler
  becomes `s.startDiscoveryExtraction`; PUT `/{documentId}/extractions/{attemptId}`
  becomes `s.cancelDiscoveryExtraction`. Paths and body keys are unchanged. Text
  and Markdown delegate to unchanged legacy handlers. Native profile stays off
  when the optional runtime is not configured. Current UI remains text-only until
  the integration publishes an actual configured native capability.
- `DiscoveryDocumentAttempt.ExecutionProfile string` with JSON key
  `executionProfile,omitempty`. Replace the unconditional legacy lifetime check
  in stored-document validation with
  `validDiscoveryAttemptProfile(attempt.ExecutionProfile, started, expires)`.
  Empty profile retains legacy positive lifetime <=120s; the exact server-owned
  `pdfium-tesseract-v1` profile requires exactly180s. Unknown profiles are invalid.
  Public expiry/interrupted/cancelled labels never clear private Busy ownership.
- Shared host ledger fields are
  `Documents []documentExtractionLease` (`documents,omitempty`) and
  `DocumentFences []documentExtractionFenceRecord` (`documentFences,omitempty`).
  Include both in existing bounded complete-ledger validation and joint4096
  history/permanent-fence limits using
  `validateDocumentExtractionRecords(documents, fences) error`.
  Every worker/check/preview/image admission calls
  `addDocumentCapacity(snapshot resourcebudget.Snapshot,
  documents []documentExtractionLease, skipScope, skipID string,
  demand *executionCapacityDemand) error` with empty skips. The dedicated document
  admission skips only its exact scope/allocation. Existing reservation snapshots
  call `addDocumentReservationView(view *executionReservationView,
  documents []documentExtractionLease)`.
- Docker shared `inertFenceContainer` permits `inertDocumentFenceLabels(row)`
  alongside its existing purpose recognizers, retaining the full inert config
  checks. Shared `containerResourceKind` returns `document-extraction` for
  documentLabel, preserving strict inert-marker handling and ownership.

The typed runtime callbacks are CaptureActor, AuthorizeControl, Busy, Execute,
Read and Recover, declared in `product/discovery_documents_runtime.go`. Execute
takes a service-derived request plus private durable publisher. Recover receives
only a validated private receipt and explicit retire action; it cannot resume
compute or attach late. Read has no reconciliation, allocation or repair effects.
Private outcomes, engine identity, original paths and raw capture never leave
the private companion store. Public evidence still uses existing document CAS,
history, source links and specification approval.

One absolute180s attempt includes compute<=120s and cleanup60s. Windows start
requires fresh8GiB physical/commit and CPU<50%; running unknown pressure or4GiB
reserve breach retires only the exact parser. Existing shared policy also applies.
Ordinary already-exited success keeps framed capture durable before retirement.
Emergency retirement of an exact ACKed CID precedes reader/publication blockers;
its lease and lifecycle lock stay held until reader and durable state settle.
Evidence additionally
requires exact exit0/non-OOM, settled reader, verified retirement and unchanged
document CAS before the original whole deadline. Unknown Create needs an explicit
durable fence intent and a real marker ACK; readback never substitutes for ACK.
Expired or uncertain leases retain all capacity and the serial slot. A reader
that does not settle retains its exact OS lifecycle lock until reader closure or
process death; recovery cannot race it. Recovery has a separate bounded60s cleanup
window, never a new compute window or automatic replacement.

Before Execute, private/public claim publication failure retires only the exact
positively unallocated stage. A lost public save ACK preserves that exact attempt
as interrupted with a closed private receipt. Unknown or possibly allocated
ownership is retained for explicit recovery; rollback does not dispatch a parser.

Deferred checks: backend/dockerbe focused document stream/fence tests and existing
check/fence/resource-observer tests; API document capacity/lifecycle/recovery tests
with shared ledger insertions; product contained-result/native profile/private
intake tests and existing document/text/CAS/source-link tests. Fixture tests are
inert source checks, not real PDF/OCR/chat/Docker/E2E acceptance. Real native/scanned/
mixed/rotated PDF, corrupt/encrypted/oversize, cancellation/restart/host pressure,
revocation and actual positive resource retirement remain required later.

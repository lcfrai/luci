# CLOUD-076 controller-owned allocation preparation

The product source base is `786652704fa8bebd1ba1af41cfa1d414d7def100`; validation uses the root checkout plus external overlays, with actual invocation HEAD/status retained. Documentation-only HEAD advances are not an immutable-checkout claim. No repository, installed fixture or Docker state is modified by this preparation.

`attachReady` is an optional display hint. A known current starting/running/idle allocation with a recorded endpoint projects true. A negative hint is emitted only while the same API instance owns preparation of the exact session and execution generation. Registration precedes the first public row write for ordinary allocation and Restart; deferred cleanup matches both generation and unique registration, including errors and caller cancellation. It carries no token, endpoint, resource authority, backend probe or persisted migration.

Unknown identity, terminal status and untracked empty endpoints omit the field. In particular, a controller reopened after a successful backend start but before the endpoint write does not suppress the existing Backend.Get-based attach recovery. The earlier pure recorded-endpoint predicate is retained in sibling `backend`, marked superseded, because it could suppress that recovery. Its existing receipts are not evidence for the narrowed implementation.

All nine production projection calls use the owning API wrapper: Create/list/detail/rename/Restart/Stop, Codex sign-in response, and both existing project-run session responses. Utility sign-in's allocation remains untracked and preserves prior behavior. Team/workspace routing continues through the same API instance and authorization paths.

The original-compatible real HTTP Restart regression has exactly the same function body as the first proposal. It independently holds backend Create and health, checks a distinct generation while creation is pending, then checks true at that same generation while GET and Restart ACK still say starting. Additional controlled tests cover omission/recovery, per-API and generation isolation, late/idempotent cleanup, failed creation, and canceled callers. Existing generation, Restart, attach origin/host, backend verdict and starting attach tests remain selected regressions.

Commands use Go 1.26.3 race instrumentation, GOMAXPROCS=2, package parallelism1, shared retained caches, readonly modules and storage floors. Invocation logs and complete effective Go/native/module/embed hashes are retained before and after each run. No executed-binary hash is claimed unless sampled from the actual process. No Linux fake-backend result establishes installed Docker/browser or Windows behavior or the requested warm post-ACK ordering.

## Controlled outcomes

Original production: 4 recovery/transport guard passes and 1 expected missing-hint failure. Superseded pure predicate: 1 expected recovery failure. Final implementation: 38 focused passes and 40 related passes, no skips. Per-run raw events, invocation metadata and complete source manifests are retained. The standalone recovery test and HTTP Restart test run unchanged across their before/final comparisons.

The superseded directory preserves its original source/receipts and explicit do-not-integrate note. Only `tree/` and `CLOUD-076-backend.patch` at this capsule root contain the final implementation. Relative overlay paths in receipts refer to the original external staging locations and are provenance, not a promise that copied commands are relocatable without updating overlays.

# 9 October 2026 Brisbane — check recovery attempt 3 failed after dialog opened

The [separate capsule](check-recovery-d348-d52-attempt3/README.md) preserves installed product `d348f0127a920aab05b54a4192baa8ed3800fe54`, helper `d52f7fe3d3981342b46391c34ad6517f100ba1be`, the actual invocation, all 11 matching executed helpers and zero external model calls.

The first check passed with confirmed cleanup. The second reached unknown/Create-unconfirmed with the expected held 1,024 MiB / 1,000 mCPU charge. Its recovery dialog opened; the original source-bound screenshot shows revision 7. The helper then failed the exact HTTP review `deepEqual` at line 175. Neither expected nor actual HTTP body was retained, so their precise difference is unknown.

A separate comparison finds different messages between an earlier revision-6 public check DTO and the later revision-7 persisted record. Revision, timestamp and stale-reason wording also differ; the latter may reflect DTO-derived evidence fields. These are distinct observation scopes, not captured HTTP operands or a pure persisted-row mutation trace. CLOUD-038 source diagnosis concerns observer-driven message/revision churn; the failed assertion is preserved without weakening it.

Controller/proxy/browser teardown and all startup handles completed, while the never-started check container and charged `creating` lease remained. Later exact-ID removal without force preserved all 110 recorded private file bytes and the lease. No product recovery, lease release or automatic container/lease cleanup is certified. The capsule retains complete report/journal/observer evidence and sanitized projections without raw state/ledger contents; publication ran no additional runtime or tests.

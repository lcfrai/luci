# 9 October 2026 Brisbane — check recovery attempt 2 failed

The [separate evidence capsule](check-recovery-d348-fdde-attempt2/README.md) records actual installed product `d348f0127a920aab05b54a4192baa8ed3800fe54` with helper `fdde0470025e612c631fb97128c6f0a8df8e99b1`, the captured invocation and zero external model calls. All 11 executed helper copies match the pinned Git revision.

The first check passed with confirmed cleanup and public reservation count 1→0. The second reached unknown/Create-unconfirmed, passed the corrected inherited-image inspection, and retained the expected 1,024 MiB / 1,000 mCPU charge. Controller/proxy/browser teardown and the repeated-stop path completed. These are scoped actual CLOUD-036 observations, not recovery acceptance.

The attempt failed before opening the recovery drawer at the helper's immediate button-count assertion, before waiting for check-history readiness. The actual count was not exported; no zero-count assertion is justified. CLOUD-037 addresses that helper readiness mechanism separately.

The original final report retained the created, never-started container and charged `creating` lease. Later exact-ID removal without force left all 110 recorded private fixture file bytes and the lease unchanged. No product recovery, lease release or automatic container/lease cleanup pass is claimed. The capsule contains full original-bound report/journal/observer evidence and a sanitized independent closure; no additional runtime or tests were executed for publication.

# 9 October 2026 Brisbane — worker recovery attempt 2 failed after task output

The [separate evidence capsule](worker-recovery-d348-c322-attempt2/README.md) records installed product `d348f0127a920aab05b54a4192baa8ed3800fe54` with clean helper `c322398ce5645236866cfc7b1bad13cd94bdcc99`, an actual captured invocation, 2,714 matching package members and zero external model calls. All 13 executed helper copies match that helper commit.

One worker was created and started through native Codex and a scripted local provider. One `build-a` result and 7,650 terminal stdout bytes were recorded. The helper then failed with `ReferenceError: canonicalNames is not defined` in `retained()` line 314. No recovery phase ran; no terminal exit was observed. The task output does not establish natural completion or product recovery.

Controller/proxy/provider teardown completed. Exact failed-fixture cleanup removed the unexpectedly remaining worker; a later independent read-only closure observed no owned processes, containers or volumes. One `backend-settled` queue lease still holds 1,024 MiB / 1,000 mCPU, with ledger SHA-256 `23cc36d21e99b4e5e22ca566727134a1919392a582c8002035b9522621eabc3b`. No ledger repair or lease-release pass is claimed.

The capsule preserves original bindings, compressed report/journal/observer data, captured sources and a scoped closure without raw ledger contents. Every published JSON string and decoded text member was checked against exact private roots and available retained credentials. Original reports/fixtures were unchanged; publication preparation ran no additional runtime or tests.

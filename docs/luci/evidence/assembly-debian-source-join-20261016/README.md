# Debian-domain source join and capacity refusal — 2026-10-16

Exact source input `1d76353ed4802e2e69bf9158e96c3423ec27df5a` was merged onto the held PDF checkpoint `0abf44ddbe1b5337d71a7c962b6f86342faeefde`. Joined commit before this receipt: `c29e4a9898ac7633d084feda4f41cd5feea9e79b`; tree `9817067edaba3abbd7a8e3389f801769487ab4da`. `git diff --check` passed. The two exact metadata fixtures retained `.gitattributes -text`, with working-tree SHA-256 `13bd77f40356ef37ae01f3baf05172a035423815b33e9a292a7736ec699150a5` and `64583d9f02277a0f311d1f6776850471b1720f18ea22f24bb93381c6c00b5205`. The former is CRLF and the latter mixed EOL as recorded by `git ls-files --eol`.

The requested one-pass API source command would use the existing Go 1.25.0 toolchain/cache, `GOWORK=off`, `GOMAXPROCS=1`, `GOMEMLIMIT=512MiB`, `GOGC=20`, `-p 1`, `-parallel 1`, `-count=1`, and only:

```
go test -p 1 -parallel 1 -count=1 -run '^TestAssemblyDebian(PolicyNormalizerPinsSourcesAndKeepsAbsentExpiry|KeySummaryBoundIdentityRevocationAndTime|SignatureCoreBindsExactObjectsRolesAndExtras|HistoricalBindingAndExpiryDoNotGrantCurrentEligibility|CanonicalSummaryReadsRejectUnknownTrailingAndOversize)$' ./internal/api
```

**No compiler or test was started.** Fresh cgroup readings were `33,680,797,696 / 34,359,738,368` bytes: **678,940,672 bytes** available. Host MemAvailable was **1,618,046,976 bytes**. The cgroup headroom is below the existing 512 MiB Go heap target plus a 512 MiB process/linker margin; an API test-package compile has unbounded additional toolchain/link overhead relative to that remaining space. There is no exit status, test log or pass to claim. The prior repair-budget/catalog/Linux/UI receipts are unchanged and were not rerun.

The independent held PDF work in this join only refines [the first-run bridge](../../../../scripts/distribute-pdf-pilot/FIRST-RUN-PLAN.md): exact API charge signature and durable attempt transitions, selected SDK/Docker ownership, and a finite pinned Poppler reader candidate with direct ELF dependencies. It does not enable the PDF route or perform acquisition/build/render/runtime effects.

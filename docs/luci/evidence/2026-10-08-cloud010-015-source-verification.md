# Swarm context and continuation source verification

The full source snapshots did **not** pass completely. The Go race run recorded
3,023 passing outcomes, 29 named skips and one failure in
`TestLargeStdinFrameIsAccepted`. The full frontend recorded 2,575 passes and
three failures; TypeScript and the production web build passed. These original
results remain failed evidence.

The [machine-readable summary](2026-10-08-cloud010-015-source-verification.json)
and [retrievable reports, test outcomes and input hashes](cloud010-015-source/README.md)
bind each run to its exact working tree. The integrated baseline had ancestry
`23e25c21788fa47607a6dca98479d4d3b51351f0` **with uncommitted changes**; that commit
alone does not contain or identify the tested implementation. All 2,003 inputs,
HEAD and working-tree status stayed unchanged during the full Go run. The full
frontend's 433 scoped inputs also stayed unchanged. A later publication must
identify its own source/package; this receipt does not certify a future commit.

| Observation | Result and scope |
| --- | --- |
| Complete Go race run | 3,023 pass, 29 named skips, one terminal failure; 32 packages passed, one failed and two had package-level skip events. 1,048.567 seconds, Go 1.26.3 Linux/amd64, GOMAXPROCS=2, `-p 1`. |
| Complete configured frontend | 2,575 pass, three fail, zero skipped, 152 actual files. TypeScript/build pass. This is component/source validation, not installed interaction or performance. |
| Corrected checkpoint boundary | 40 product and eight actual HTTP/API outcomes pass. Real Git/private state covers distinct unstarted seed, source locks, retained output, reopen/idempotency, fresh authority, live-fence guard and separately starting the successor. Backend states were controlled. |
| Later affected backend | 126 pass, zero failures/skips across two packages; 669 control-plane Go/module inputs unchanged. Covers retained handoff output through transcript cleanup, context compatibility and compact human answers with their original question/impact. |
| Later affected frontend | 169 pass, zero failures/skips, nine actual files; TypeScript/build pass, 436 inputs unchanged. Ten paths were requested but one nonexistent extra path matched nothing; it is not counted as coverage. The unchanged real orchestrator test file had 32 passes in the full baseline. |
| Controlled source browser | 22 checkpoints pass at 1440/390 px in Chromium 151.0.7922.173, zero reported axe violations/overflow. The original keyboard-only focus diagnostic failed with BODY focused; its after-fix repeat focuses Continue reserved task, with zero starts. These use controlled callbacks, not persistent backend changes. |
| CLOUD-016 terminal follow-up | 32 affected race outcomes pass, zero failures/skips, 88 inputs unchanged. Bounded-byte coalescing corrects the fragment-count disconnect; real local PTY/WebSocket and subscriber bounds were checked. Build a matching new sandbox image before installed acceptance. The original full Go run remains failed. |

The [failure lineage](cloud010-015-source/failure-lineage.json) preserves the
route-fixture 404s, test pointer compile error, real checkpoint source-lock
registration defect and reused Go JSON destination in the earlier focused runs.
The checkpoint correction removes the conflicting outer read lock for that
exact POST; read requests remain protected and held locks still block allocation.
The full baseline subsequently passed the corrected checkpoint/orchestrator cases.

Frontend lineage preserves a stale strict signature matcher, a missing V2 type
mirror and a refused-recovery test whose expected alert was absent. The latter's
unchanged isolated rerun passed, which does not prove a flake or its root cause.
Its corrected test waits for enabled consent and the exact request before
resolving the refusal; production recovery behavior was unchanged by that fix.
A separate controlled source-browser check found focus on BODY after checkpoint
preparation; the task-scoped focus correction has component/typecheck/build
coverage. The repeated keyboard-only diagnostic and final controlled source-browser
checks now pass; the original BODY-focus failure remains retrievable.

Totals from overlapping full and focused runs must not be added as unique tests;
Go outcomes include named subtests. Published skip entries include platform-only,
opt-in live-Docker/provider/soak and subprocess-helper cases. They remain unverified
by the ordinary Linux invocation. Narrow follow-up hash maps establish their own
scope, not an unchanged entire checkout since the full run.

Matching installed checkpoint, dependency join, real Git-conflict and sustained
workflows remain separate gates, as do native Windows workload/recovery and
live-provider usefulness. Frontend/Go checks overlapped on this development host;
no throughput, responsiveness or market-leadership claim follows. No raw test
DOM, credentials, private fixture state or authentication logs are published.

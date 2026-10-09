# Ordered check recovery with a charged workspace witness

One new controlled Go regression passed with `-race -p 1`, `GOMAXPROCS=2`, Go 1.26.3.
The test body took 0.96 seconds; the command took 18.486 seconds. One top-level
pass, zero failures/skips, no forced timeout. No app, browser, Docker or provider
was launched. The existing original CLOUD-038 test was not rerun.

The test uses real production HTTP, local store, lifecycle locks, observation and
recovery with controlled backend replies. It captures the original unknown-Create
review before the first observer, adds a separately HTTP-created and barrier-held
workspace charge, observes/reopens, checks exact history bytes and charge
preservation, verifies current-authority refusal, then recovers only the target.
The witness completes once after its one barrier release and leaves target fence
proof unchanged. No ledger rows are seeded or repaired. This is controlled
concurrent-charge evidence, not the still-open actual Docker witness or installed
first-observer ordering.

The 995 selected inputs (all tracked repository Go/module sources plus external
workspace, test and overlay) are unchanged before/after. This is **not** full
compiled-input or installed-module/standard-library provenance. Actual HEAD stayed
8c7bf16aa035b27dbea185b565052495eb60777f, clean before/after; tracked Go/module
inputs are unchanged from product edd0.

`run-1` retains the original unformatted draft, exact formatted/tested overlay,
raw JSONL, empty stderr, command and source snapshots. No compilation or fixture
failure occurred. The repository-relative candidate changes only the three
leading comments to remove obsolete unrun status; the tested code is identical.
Root owns integration and any later checks. Original plan/capsule remains intact.

# CLOUD046 — retained CPU and memory admission operands

The focused backend validation passes **85 leaf cases across 33 top-level tests** (98 named test/subtest results), with zero failures or skips. The original regression fails all three CPU, host-memory and execution-memory cases because the numerical operands were missing after a final admission refusal. Those original failures and all passing raw results are preserved byte-for-byte.

The change retains the actual sampled value, refusal threshold and sample/decision timestamps on the exact session, atomically with the unchanged canonical `StartFailure`. It does not resample, relax admission, start a replacement or alter recovery/release authority. Only the existing single-session GET can return the optional evidence under the scheduler's host-operator/administrator boundary; members, disabled actors and all team contexts receive none. Shared DTOs, lists, mutation responses and run/queue projections do not expose it. Both existing new-generation reset sites clear the retained operands with the failure.

This is operator acceptance evidence through the optional diagnostic API. UI rendering is not part of this batch. The original installed attempt 6 CPU percentage remains unavailable: these operands are recorded only for future refusals. No installed-app, real-Docker, external-model, Windows or performance validation is claimed.

## Frozen records

The complete 35-member external capsule is copied without byte changes into [`frozen/`](frozen/). Its own [README](frozen/README.md) gives the implementation contract and reproduction command; its [receipt](frozen/receipt.json) retains SHA256 `e521f42597c54724e6fc8eb53f5eafa66c607285fbdd15500159524b99a75871`.

- [Before regression](frozen/before/go-test.jsonl), [command/exit status](frozen/before/execution.json), [test source](frozen/before/session_admission_evidence_test.go) and [summary](frozen/before/summary.json): three meaningful runtime assertion failures after final refusal, canonical-reason and zero-Create checks. No missing-type compilation failure.
- [Final raw results](frozen/validation-1/go-test.jsonl), [command/exit status](frozen/validation-1/execution.json) and [summary](frozen/validation-1/summary.json): exact final-sample/reopen behavior, finite/bounded operands, measured zero, current administrator/member/disabled/team authority, list/mutation exclusion, generation clearing and affected admission/lifecycle regressions.
- [Before declared compile inputs](frozen/before/declared-compile-source-inputs.json) and [final declared compile inputs](frozen/validation-1/declared-compile-source-inputs.json): 2,334 and 2,435 source/embedding/configuration files, respectively. Both bind the external `go.work` and `go.work.sum`. Raw `go-list.jsonstream` and broader `compiled-inputs.json` metadata are preserved for each stage. No captured input changed during final validation.
- [Before source copies](frozen/before/source/) and [all ten final affected source copies](frozen/validation-1/source/). The independent read-only source review identified no actionable privacy, lifecycle or operand defect.

Go 1.26.3 ran with `GOMAXPROCS=2`, `-p 1` and the frozen targeted test selection. The installed retained toolchain/caches were used; no full API suite or original failed acceptance fixture was run or changed. Publication performs no additional tests or product source edits. Source files contain deliberate synthetic privacy-test markers, not credentials or private fixture data.

## Integrity

[`publication-manifest.json`](publication-manifest.json) binds this README and all 35 unchanged frozen members. [`SHA256SUMS`](SHA256SUMS) additionally binds the publication manifest. The checksum file excludes itself.

From this directory:

```bash
sha256sum -c SHA256SUMS
```

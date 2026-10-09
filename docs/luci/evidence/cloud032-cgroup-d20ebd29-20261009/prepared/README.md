# External counter-only adjunct — prepared, not executed

This is an optional Linux observation around the **unchanged** sealed idle launcher and driver. It reads only the selected visible cgroup's `cpu.stat` every 100 ms, and fixed cgroup/namespace/clock metadata before and after. It does not discover processes, sample schedstat, contact a browser, profile CPU, change clock origins, alter admission or send signals to the application. No real observer run was performed during preparation.

## Exact invocation shape

Run this only after root approves the matching package/run and allocates an exclusive measurement window. `unchanged_idle_command` below is the **existing exact launcher argv** already selected by root; the adjunct does not rewrite it, its environment, source/image identity, counts or thresholds.

```python
from pathlib import Path
import json
import subprocess
import sys
import tempfile

adjunct = Path('/workspace/.luci-tools/cloud032-cgroup-counter-adjunct')
sys.path.insert(0, str(adjunct))
from supervisor import ManagedObserver

# A newly allocated owned 0700 parent; never reuse a previous report/receipt.
private_run = Path(tempfile.mkdtemp(prefix='c052-idle-cpu-', dir='/workspace/.luci-tools'))
private_run.chmod(0o700)
managed = ManagedObserver(
    ['--output', str(private_run / 'cpu.jsonl'),
     '--cgroup-dir', '/sys/fs/cgroup',
     '--duration-seconds', '399'],
    private_run / 'supervision',
    wall_seconds=400, term_grace=1, kill_grace=1,
)
try:
    with managed:
        completed = subprocess.run(unchanged_idle_command, check=False)
finally:
    # A returned snapshot is not automatically a valid/complete observation.
    # Preserve both it and the original launcher's independent result.
    observation_receipt = managed.snapshot()
```

The snippet is instructions, not an executed run. Choosing a 399-second collector ceiling leaves margin before the independent 400-second supervisor deadline; normal owner completion stops it earlier. It never caps or terminates the app/launcher. If the original launcher continues beyond the observation window, the uncovered period remains unobserved.

`ManagedObserver.start()` verifies fresh output, a new private receipt parent and the two fixed source files, pins a **Linux pidfd to its own direct child**, then releases a one-byte bootstrap gate. It refuses unsupported pidfd operation; it has no numeric-PID signal fallback. Return means the child is pinned and its execution gate released: **it is not a live collector-header readiness acknowledgment**. The final validator separately checks that the serialized header's PID matches that child. Bind actual header/first/last sample timestamps to the unchanged run's phases before claiming phase coverage. There is no app launch inside `start()`.

The watchdog detects its wall deadline with at most a 50 ms polling interval, then allows one second after TERM and one second after KILL. Only the pinned observer can receive those signals. Public `stop()` waits at most three seconds and returns uncertainty if settlement/publication is still unconfirmed. Forced termination, deadline, source drift, nonzero exit, unreadable/missing/invalid footer or reused/foreign PID acknowledgment cannot become a successful observation. No kernel guarantee of terminating an uninterruptible process is claimed; such a case remains `settlement_unknown`.

The parent's pre-spawn filesystem operations and final receipt fsync are cooperative. A filesystem hang can therefore leave final serialization unconfirmed even though the caller's stop wait is bounded. Do not infer durable closure from exit code alone. Observer stdout/stderr are discarded by this minimal supervisor: core exception text is unavailable, so only its exit/receipt failure is classified, without inventing a root cause. The unchanged launcher's own output/error capture is unaffected.

## Limits and interpretation

- Maximum collector duration: 400 seconds. Maximum report: **4,000 total JSONL rows including header/footer**, **2 MiB**. Both caps are upper bounds; the sample/footer reserves can stop capture earlier. The file is new, private (0600), append-only and refuses existing/symlink outputs or nonprivate parents. Final summary capacity is reserved before sampling; no counter read is silently discarded when the report becomes full.
- No catch-up bursts. Late samples, missed future deadlines, read durations/overruns, unavailable or malformed observations, missing fields, counter resets/shape changes and source/metadata uncertainty are explicit. Unknown observations are never filled with zero. Cgroup directory reads use a held descriptor; before/after path identity, quota, cpuset and namespace comparisons guard stable-scope interpretation.
- Monotonic and wall read brackets permit only **coarse phase association**. There is no new driver/browser clock patch. A positive valid throttle delta shows some task in the observed shared cgroup was throttled in that interval. It does not assign the delay to a specific action, thread or owned process, and hidden ancestors remain unobserved.
- No timing or instrumentation cost is subtracted. Original 300/100 ms p95 and strictly-below-1,000 ms p99 targets stay unchanged. Report bounded collector CPU through footer construction and watchdog-thread CPU through validation separately; both exclude their final serialization/fsync and other declared work. They are not a measured total adjunct overhead or a speedup claim.
- A complete JSONL serialization may still have unavailable readings, unstable metadata or insufficient coverage. `observationComplete` in the supervisor means source-stable, exact-child, normal-exit serialization only; inspect `stopReason`, gaps, reset/unknown counts, metadata, timestamps and deadline/force status separately. Original app success/failure remains independent.

## Controlled preparation evidence

`evidence/observer-fixtures-attempt1/` records **20 passing** pure parser/fake-clock/temporary-file tests with the observer/test hashes stable before and after. They cover cadence and no catch-up, duration/row/byte bounds, footer reserves, missing/denied/malformed/duplicate/negative/oversize counters, positive deltas, reset/gap handling, wall-clock jumps, metadata changes, private fresh output, symlink/nonregular refusal and partial-write failure.

The supervisor's final `supervisor-fixture.stderr.log` records **31 passing** fake-process/pidfd/clock and tiny file fixtures. Its earlier 26- and 27-case passes overlap this final run. The intermediate 31-case run's one fixture failure is retained: the environment umask made a purported 0755 test directory 0700, so the corrected fixture explicitly sets 0755. No product/observer defect is inferred from that fixture setup error. Earlier logs are retained unchanged.

Independent cross-review is `/workspace/.luci-tools/cloud032-cgroup-counter-adjunct-review/final-cross-review.json`. These checks exercised no real observer child, live cgroup workload, app, browser, Docker or benchmark. They do not establish live pidfd/cgroup/clock behavior, actual observer overhead, application performance or Windows acceptance. The final manifest binds the four source/test files, README and retained evidence. Root must capture source/package/helper identities again around any later real run and preserve that new result separately.

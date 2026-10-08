# Installed task cleanup and turnover at 8f78804f

**The controlled installed workflow passed; the full product goal remains open.**
Four real Docker/native Codex tasks completed naturally, handed the shared slot
to the next project, and passed the existing browser review/integration checks.
The four output files exactly match the earlier baseline by SHA-256. Provider
responses were scripted, so this verifies execution and coordination mechanics,
not model reasoning or production application quality.

The [sanitized receipt](2026-10-08-8f78804f-installed-swarm.json) binds the run to
clean source `8f78804ff318ab744226f139df5998cbd6e0d0ff`, **1,048 matching installed
package members**, all six executed helper hashes, and the same verified runtime
image used for the baseline:
`sha256:42597a985b20f2aee31a629a320980f02180bde307b6ffdd3ec60b257a82af5b`.
Its retained raw result SHA-256 is
`f5395edf99e56addc238d05052bc846b318164ced6bf6639830b70c857ee7a10`.
Later source edits were not executed by this installed package.

## What passed

- Explicit browser authorization of five queued tasks across four reviewed
  projects, plus Ask-mode refusal without silent conversion to Bypass or an
  allocation. Executed tasks deliberately saved Bypass.
- Four native status-0 exits, four Create/Start pairs, and three next-project
  slot transfers before the previous result was integrated. The order remained
  A, B, D, C; the extra waiting A task was cancelled before allocation.
- Exact read-only captured context and independent workspaces for every task.
  Each worker changed its assigned result file and committed the expected bytes.
- A real service restart preserved the first live child, container and context,
  without another allocation, tool execution or provider retry.
- Browser automation opened each diff, confirmed review, integrated that exact
  commit, and checked primary output bytes and unchanged canonical context.
  This exercises review controls, not independent human assessment of generated
  production code.

There were **zero session DELETE requests**, zero browser errors and **1,961
transport observations with none dropped**. The four cleanup requests targeted
immutable container IDs. All containers enforced 1 GiB RAM, 1 CPU, 256 processes
and `no-new-privileges`; no trust keystrokes were injected.

The 6 GiB pre-create and 1 GiB running disk guards remained active. Minimum
driver-observed root free space was 2,132,017,152 bytes. Final checks found zero
owned containers, volumes, service processes or browser processes. The private
fixture was removed, protected package/image bytes were unchanged, and the
wrapper and independent observer exited successfully.

## Measured turnover

The new cleanup path retains exact task launch proof until the stopped container
is removed. Eligible tasks are collected in the same reaper sweep that records
their exit, using an immutable container ID and non-force removal. The regular
30-second sweep interval is unchanged.

These are actual observations from the two installed runs, in seconds:

| Task | Baseline: first completion poll → cleanup 204 | New: first completion poll → cleanup 204 | New: native exit frame → cleanup 204 |
| --- | ---: | ---: | ---: |
| A | 60.855 | 31.782 | 26.682 |
| B | 54.109 | 14.233 | 9.106 |
| D | 55.541 | 22.259 | 16.947 |
| C | 67.967 | 22.276 | 17.157 |

The completion-poll interval includes process completion, sweep waiting and
removal. It is not a measurement of Docker removal alone. In the new run, lease
disappearance was bounded between samples 0.143–0.998 seconds apart, with no
unavailable samples. Exact lease publication and next-admission instants were
not instrumented. The next Docker create request followed cleanup's 204 response
by 9.001, 3.485 and 3.446 seconds; those intervals also include scheduling and
checkout preparation.

The [timing receipt](2026-10-08-8f78804f-swarm-turnover.json) retains UTC endpoints,
native exit observations, bounded lease observations, creation/start durations,
and the comparison limits. Baseline native-exit timestamps and lease-release
observations are explicitly **unavailable**; stdout timestamps were not used as
substitutes. Proxy Create-request times are used for the new comparison; the
earlier baseline document retains its immediately following fixture guard times.

Total workflow time was **276.837 seconds**, versus **428.478 seconds** in the
[preserved baseline](2026-10-08-2819ce9e-installed-swarm.md). These totals include
setup, browser actions, VFS copies, restart, review and cleanup. This is one run
per package, not a statistical speedup estimate or an isolated cleanup benchmark.
The baseline overlapped a bounded 35.878-second compiler check; this run had no
compiler overlap. Sweep phase, host/cache state and browser timing also vary.
The added helper changes only record timing; workload commands, output checks,
provider behavior and resource guards remained unchanged.

## Resources, scope and reproduction

The independent observer retained **294 samples over 295.83 seconds**, including
all four exact-owned containers and final cleanup. See the separate
[resource companion](2026-10-08-8f78804f-installed-swarm-resources.md). This is an
approximately five-minute serial workflow, not the sustained ten-minute or
parallel full-container acceptance gate. No idle baseline, other-application
responsiveness or user input latency was measured.

Account/team/provider setup, approved context and initial runs were API fixture
actions. The screenshot's remaining Develop stage for A is expected because its
extra selected task was cancelled; the other projects await verification. No
full application check or deployment was requested. Native Windows, live-model
quality, installation, paid-service behavior and cloud release remain separate
gates. Earlier failed attempts and baseline evidence remain unchanged.

Reproduce using the [installed runtime procedure](../../../scripts/acceptance/four-project-queue/RUNTIME-WORKFLOW.md)
and [spawn-time resource wrapper](../../../scripts/acceptance/four-project-queue/RESOURCE-MEASUREMENT.md),
with the matching clean package/image provenance and fresh report directories.
Executed driver SHA-256:
`b9619765aa841fa606a18a64bad04e3939833add71f2588ba804d4a077759cf2`.
The receipt publishes hashes and allowlisted outcomes; private credentials,
provider payloads and runtime state are excluded.

The [desktop](8f78804f/installed-swarm-desktop.png),
[phone](8f78804f/installed-swarm-phone.png) and
[running-project](8f78804f/installed-swarm-running.png) screenshots supplement
the checked DOM/API outcomes. They do not establish complete mobile interaction
coverage or finished visual polish.

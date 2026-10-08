# Swarm capacity and responsiveness acceptance

The local release must remain usable while at least four projects have ongoing
work. Queued projects count as ongoing work, but a queue must never be reported
as four concurrently executing agents. Record the actual running count, each
worker's resolved limits, current account/provider policy, and the reason for
any waiting project. The target workstation has approximately 31.2 GiB usable
RAM; measurements from the cloud development machine do not certify it.

## Controlled acceptance

`control-plane/internal/api/project_swarm_capacity_test.go` is an opt-in,
repeatable integration test. It uses one real API, product service, private
registry and scheduler across four reviewed projects. Its backend starts actual
local child processes that read their assigned immutable contexts and perform
bounded CPU, memory, file and output work. Admission uses explicitly synthetic
capacity observations; host/process observations are collected separately using
the platform sampler. Requested resource limits are not enforced by this test
backend and must not be confused with measured memory use.

The workflow exercises batch consent and lost-response recovery, cancellation
before allocation, a shared three-worker budget with one worker per project,
fair access for the fourth project, reopening the API/product services and
private ledgers with live workers, unchanged captured context, a controlled
worker failure and production reaper observation. This reopen happens within
the test process; it does not simulate an OS-process crash or a host reboot.
Exactly one deliberate exit-2 failure and six successful task results are
required; unexpected worker failures fail acceptance. Dependent joins remain unallocated until their prerequisites have
actually been integrated. Process completion does not establish correctness or
authorize integration.

Run a short mechanics check first, then a ten-minute workload once the source is
stable. Use a new report path each time; retain failures and earlier measurements.

```sh
cd control-plane
LUCI_SWARM_MULTI_PROJECT=1 \
LUCI_SWARM_MULTI_PROJECT_SECONDS=600 \
LUCI_SWARM_MULTI_PROJECT_REPORT=/absolute/path/four-project-acceptance.json \
go test ./internal/api -run '^TestProjectSwarmFourProjectControlledCapacity$' -count=1 -timeout=35m -v
```

For this cloud checkout, source `/workspace/.luci-tools/activate.sh` first and set
both `TMPDIR` and `GOTMPDIR` to a short external path such as `/workspace/.t`
(create it first). A long temporary path exceeds Unix-domain socket limits in
related integration checks. Recheck both volumes' free space before compiling
or running. On Windows, set the same three `LUCI_SWARM_...` environment variables in
PowerShell before running the Go command. The duration accepts 1–1800 seconds;
ordinary test suites skip this opt-in workload.

Reports identify the fixture assumptions, requested resource limits, process
IDs, creation identities and lifetimes, observed peak concurrency, exact exit
codes and terminal events, task output provenance, per-task workspace/context
bytes, CPU time, and timestamped host/process samples before, during and after
the controlled workload.
Unavailable OS measurements remain unavailable. A passing controlled run does
not certify Docker enforcement, real model quality, native desktop interaction,
deployment, or sustained behavior on the target Windows computer.

The first ten-minute controlled trial is retained as a failed run. A short
reproduction exposed a fixture mistake: its inherited 50 ms unit-test readiness
budget allowed only 100 ms for actual executable startup. The process fixture
now uses the production 60-second readiness budget and records startup, exit
and registry diagnostics; it fails immediately if the intended live count drops.
Admission inputs are also explicit: a synthetic 16 GiB execution pool retains
25% headroom while fitting three requested 4 GiB workers. A 12 GiB pool correctly
admits only two. The corrected ten-minute run passes against source
`e81b586077f8af97e5e59472e34eafd161f16b8a`: 628.11 seconds elapsed, 602 samples,
three live workers in all 600 workload samples, six successful task results and
one deliberate exit-2 failure. Fair admission, retained captured context through
service reopen and amendment, and cancellation before allocation passed.

The sampled simultaneous RSS sum for the test control plane and its controlled
workers peaked at 158,851,072 bytes (151.49 MiB); shared pages can be counted
more than once. This excludes browsers, Docker and models. Each worker actually
touched 16 MiB; requested 4 GiB limits were synthetic scheduling inputs, not
enforced memory consumption. The host also performed unrelated build work.
These observations do not change production limits, prove whole-application
memory use or establish native workstation capacity. The earlier failed run
remains a failure. The installed Docker/browser mixed workload is a separate
gate, and successful process exit still requires result review and checks.

## Terminal memory and recovery

The desktop retains at most eight terminals with no mounted consumers, each for
at most five seconds. Mounted consumers are never evicted by that cache. Each
terminal admits at most 1 MiB of combined pending/parser payload and 1024 pending
chunks, with one asynchronous xterm write outstanding. These are application
payload bounds, not a claim about total WebView memory or browser network queues.

When output exceeds those bounds, the view disconnects with a visible message;
the worker is not stopped. Manual retry cancels any incomplete terminal escape
sequence and resets the display before receiving recent replay. The replay is
limited history, not a complete transcript.

Sandbox subscribers retain at most 32 chunks of up to 32 KiB each. A full queue
closes that subscription with an explicit overflow signal instead of silently
continuing after missing bytes. Sandboxd sends WebSocket close code 1013, the
control plane forwards it before cancelling its relay, and the desktop requires
manual retry. Binary frame kinds are unchanged. The full behavior requires
matching desktop, control-plane and sandboxd versions: an old sandbox image can
still silently drop upstream output, and an old desktop may automatically retry.
Keep the old image pin for reproducibility and verify the rebuilt candidate's
source/image identity before claiming end-to-end acceptance.

The October 8 browser fixture rendered actual ColumnsLayout, xterm and the
registry with 50 synthetic sessions. The same rapid-scroll sequence retained
32 terminals/sockets before this change and at most 11 afterward: three mounted
plus eight cached. After unmounting and the grace period, none remained. A second
fixture verified visible overload, no automatic retry, successful manual replay
and cleanup. These are browser retention/recovery observations with synthetic
sockets, not whole-process memory or model benchmarks. Evidence is in ignored
`artifacts/quality-swarm/navigation-review/` and
`artifacts/quality-swarm/terminal-limits/`, including source hashes.

## Native Windows release procedure

Start a fresh, isolated acceptance workspace from the checkout root:

```powershell
.\scripts\start-local-acceptance.ps1 -AcceptanceRoot 'D:\LuciAcceptance'
```

Choose an existing drive with sufficient space. The launcher requires at least
40 GiB free by default, creates new GUID-owned state/projects/artifacts, records
a native baseline, and uses API port 18730 and frontend port 14200 by default.
`-PrepareOnly` records the baseline without starting Luci; another invocation
creates another fresh run. It does not reuse the user's existing Luci state.
The launcher's native Windows behavior and WSL engine/disk compatibility remain
unverified in this Linux cloud session.

1. Record the Windows build, usable physical RAM, CPU count, Luci build, sandbox
   image digest, Docker/WSL configuration, account/provider limits and normal
   browser/editor workload. Capture an idle baseline before starting agents.
2. Open four distinct projects with realistic source/history sizes and different
   outputs: an interface, an API, a data/task processor, and a tested library.
   Review their contexts and task graphs. Queue independent work in all four;
   record actual agent concurrency and any admission restriction.
3. Run for at least 30 minutes, including builds, tests, substantial terminal
   output, a quiet/waiting agent, project/context amendments, and agent failure.
   Switch repeatedly through Define, Design, Develop and Files, restore unsent
   drafts, inspect another project's progress, stop an agent, and restart Luci.
   Verify frozen-context provenance, exact child recovery and no duplicated work.
4. Sample host physical available RAM, commit separately from physical RAM,
   CPU, disk capacity, and owned process memory. Record Docker/WSL observations
   separately; do not add VM memory and its guest/container memory together.
   Process page-fault counts include soft faults and are not hard-paging rates.
5. Measure input and project-switch latency, long UI stalls, terminal retention
   and memory after work stops. Also type and navigate in another ordinary
   Windows application: Luci frame timing alone cannot establish whole-computer
   responsiveness. Keep the normal browser/editor workload present throughout.
6. Verify the applications' actual outcomes, tests, safe integration and recovery.
   Retain timings, traces, failures and intervention notes. Compare serial and
   parallel task groupings at useful work sizes; do not select only favorable
   trials or substitute throughput for correct results.

Choose the worker pool from measured headroom. An initial target is approximately
8 GiB of available physical memory on this workstation, with no sustained paging
or repeated long interaction freezes. Candidate interaction targets are p95
input response below 100 ms, p95 project switch below 300 ms, and p99 below one
second. These targets require native measurements and calibration against the
recorded idle baseline; no native performance pass is asserted here. A queue
that preserves headroom and explains waiting is preferable to an unsupported
promise that all projects can run an unrestricted number of agents.

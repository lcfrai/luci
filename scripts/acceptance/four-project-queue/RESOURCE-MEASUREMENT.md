# Resource measurement for installed acceptance

The Linux observer reads `/proc` and the local Docker Unix API. It never creates or stops containers, changes application state, or reads process command lines/environments. Run it beside the installed workflow driver, including failed attempts. Its accepted source/hash and timestamped raw observations are retained in the chosen fresh output directory.

## Linux commands and ownership

Requirements: Linux, Python 3, permission to read the particular owned processes and the existing local Docker socket. Access that is missing stays unavailable. No extra Python packages are needed.

Prefer the [tracked workflow wrapper](run-with-observer.py), which starts observation directly at driver spawn and publishes the exact newly created team instance path without waiting for an operator message. The driver runs the actual Docker acceptance workload and its owned cleanup; only the observer is read-only. Supply the verified package/image arguments described in the [workflow README](README.md):

```sh
python3 scripts/acceptance/four-project-queue/run-with-observer.py \
  --observation-dir "$fresh_resource_report_dir" --fixture-base "$short_roomy_directory" -- \
  --binary "$installed_binary" --web "$installed_web" \
  --provenance "$package_provenance" --expected-source "$source_commit" \
  --image "$existing_image_tag" --expected-image-id "$verified_image_id" \
  --image-provenance "$image_provenance" --report "$fresh_workflow_report"
```

The wrapper supplies `--fixture-root`; do not pass another one. Both report locations must be fresh. It keeps the sampler active through the driver's exit and the settle window. A wrapper/sampler error is a failed acceptance run.

For integration into another controlled runner, use the observer directly as follows. The workflow wrapper should start the driver and then this observer directly, before waiting for browser/API readiness. Capture the child PID from the spawn call; do not search all `node` processes. Read its creation identity from `/proc/<pid>/stat` field 22, accounting for the parenthesized process name:

```sh
python3 -c 'import pathlib,sys; s=pathlib.Path("/proc",sys.argv[1],"stat").read_text(); print(s[s.rfind(")")+2:].split()[19])' "$driver_pid"
```

Start observation immediately, using that PID and returned start tick value:

```sh
python3 scripts/acceptance/four-project-queue/sample-linux-owned.py \
  --driver-pid "$driver_pid" --driver-start-ticks "$driver_start_ticks" \
  --instance-path-file "$private_instance_pointer" \
  --image-id "$verified_image_id" \
  --output "$fresh_resource_report_dir" --seconds 900 --settle-seconds 15
```

`--output` must not already exist. `--seconds` sets the requested observation window (1–3600 seconds); an in-flight sample can finish after that window. The observer also stops after the exact driver exits plus its settle interval. Stop the observer itself with Ctrl+C if needed; it never kills the driver or children. Partial JSONL remains available if interrupted, but a final summary is not guaranteed in that case.

`--instance-path-file` names a fresh private text file. Once the **same run's** team exists, the wrapper writes the absolute path to its exact `runtime/state/instance/instance-id` into that pointer file. Use a unique fixture parent so discovery cannot select another run. Before then, the observer captures processes and marks Docker ownership unavailable. Once resolved, the instance identity is fixed for this observation; if the actual instance changes, start a new observation. Alternatively, use `--instance-file` when the exact file already exists. Never substitute a wildcard matching every user's projects. The pointer contains a private path and does not need to be published.

Descendants are followed by PID **and start ticks**, including already-discovered descendants after reparenting. A reused PID does not inherit ownership. Very short processes and children reparented before discovery can be missed. Docker observations require the exact instance label, `luci.managed=true`, and the verified sandbox image ID; this fixture observer measures those worker containers. It does not claim to inventory previews, different-image application services, the Docker daemon, or arbitrary host processes. The product's broader capacity sampler accounts for other active daemon workloads separately.

After both driver and observer have ended:

```sh
python3 scripts/acceptance/four-project-queue/summarize-linux-owned.py \
  "$resource_report_dir" --workflow-report "$workflow_report_json"
```

The summarizer only reads existing files and writes `analysis.json`; it does not contact Docker. It binds the observations to the workflow's success/failure and source commit. Raw files contain PID/container/sandbox identities and potentially local disk roles, so inspect them before sharing. Use an explicit field allowlist when publishing; do not upload the fixture's state, tokens, environment, raw authentication logs or unreviewed error stacks. The [published partial receipt](../../../docs/luci/evidence/2026-10-08-e81b5860-partial-owned-resources.json) is an example with stable worker aliases and numerical timeline data.

## Interpretation

- Process RSS is separated into test driver, browser, control-plane and other observed descendants. Summed RSS double-counts shared pages; it is not unique physical memory. Peaks for different roles happen at different times and must not be added.
- Process CPU and I/O counter deltas cover only observed intervals. Permission failures remain missing. Short-lived children can contribute work that is never observed.
- Docker cgroup usage, working-set estimate, hard limits and CPU are separate. Do not add cgroup memory to container process RSS or infer daemon/kernel/VM overhead from it. Zero active containers at one sample is not evidence that none ran between samples.
- Host available memory, Linux commit accounting, CPU and filesystem space include unrelated activity. `Committed_AS` is not Windows private-memory commit. Host availability does not attribute consumption to Luci.
- The observer targets one-second intervals, but scheduler/API delays can create gaps; inspect actual intervals. Sampling starts after driver spawn. Post-exit settlement is not idle Luci, and neither interval proves input latency or other-app responsiveness.
- Controlled commands prove execution mechanics only. Record live-model quality, heavier builds, native Windows behavior and whole-computer responsiveness separately.

## Native Windows sampler: short native check reported

`sample-windows-owned.ps1` and its C# companion require Windows PowerShell 5.1 or PowerShell 7 on Windows. The helper reads exact process handles, native physical/commit/CPU/disk counters and optional bounded local Docker named-pipe statistics. It does not launch or stop applications, restart Docker, change WSL settings, pull images, move disks or invoke full container inspection. It writes only a fresh capture folder with a protected current-user ACL. An existing output folder is refused.

Use the [isolated launcher](../../../docs/luci/LOCAL-WINDOWS-ACCEPTANCE.md), leaving the normal browser/editor workload present. Read its private baseline manifest, then capture the exact API listener's identity:

```powershell
$run = Get-Content -LiteralPath '<run>\artifacts\windows-baseline.json' -Raw | ConvertFrom-Json
$apiPort = [int]($run.apiAddress.Split(':')[-1])
$owners = @(Get-NetTCPConnection -LocalAddress 127.0.0.1 -LocalPort $apiPort -State Listen |
    Select-Object -ExpandProperty OwningProcess -Unique)
if ($owners.Count -ne 1) { throw 'The isolated API listener is not uniquely identified.' }
$apiProcess = Get-Process -Id $owners[0]
$selected = @([pscustomobject]@{ processId = $apiProcess.Id; role = 'control-plane' })
```

This initially selects only the API tree. Before a whole-app capture, add the exact desktop root or **dedicated acceptance browser** root to `$selected`, identified from its own launch or Task Manager tree. For example, after replacing the example PID: `$selected += [pscustomobject]@{ processId = 12345; role = 'browser' }`. Supported roles are `control-plane`, `browser`, `desktop`, `test-driver` and `other-owned`. Do not select every matching browser/WebView process or attribute an ordinary shared browser to one Luci tab. Unspecified processes remain unassigned.

Create the private scope manifest. The sampler checks every root's creation FILETIME before starting and on subsequent samples, so a recycled PID does not inherit ownership:

```powershell
$roots = @($selected | ForEach-Object {
    $process = Get-Process -Id $_.processId
    [ordered]@{
        processId = $process.Id
        creationFileTimeUtc = $process.StartTime.ToUniversalTime().ToFileTimeUtc().ToString(
            [Globalization.CultureInfo]::InvariantCulture)
        role = $_.role
    }
})
$scope = [ordered]@{
    version = 1
    roots = $roots
    disks = @(
        @{ role = 'projects'; path = $run.projectsDir },
        @{ role = 'state'; path = $run.stateDir }
    )
    docker = $null
}
```

For worker observations, set `docker` using the **exact current run's** instance identifier, verified image digest and local Docker pipe. The instance file is the selected workspace's `runtime\state\instance\instance-id`; select its exact known workspace path, not a wildcard across other teams/users. For an already verified `$exactInstanceFile` and `$verifiedImageDigest`:

```powershell
$scope.docker = @{
    instance = (Get-Content -LiteralPath $exactInstanceFile -Raw).Trim()
    imageId = $verifiedImageDigest
    pipeName = 'docker_engine'
}
```

Use the actual local endpoint's pipe name if Docker Desktop uses `dockerDesktopLinuxEngine` instead. Remote engines are not supported by this helper. When Docker is stopped, inaccessible or not configured, host/process observations continue and Docker fields explicitly remain unavailable. The helper never starts Docker. Container IDs/image/labels are checked from its list endpoint; only statistics are then queried. Guest PIDs and configured CPU quotas are deliberately not read because full inspection contains environment/mount data. The cgroup memory limit reported by statistics is separate from observed usage and may reflect an unbounded container's VM limit.

Include the **verified physical Windows volume containing Docker's active VHD** as a separate disk role when known, for example `$scope.disks += @{role='docker-backing'; path='C:\'}` only if that location was actually confirmed. Up to four local paths are supported. Free space inside the Linux guest does not prove free space for growth of a dynamic VHD on Windows; this observer does not discover or move that VHD. Never assume its default location after the user relocates Docker data.

Write a new manifest and run a bounded capture from the checkout root:

```powershell
$manifest = Join-Path $run.artifactsDir ('resource-scope-' + [guid]::NewGuid().ToString('N') + '.json')
$scope | ConvertTo-Json -Depth 8 | Set-Content -LiteralPath $manifest -Encoding UTF8
$capture = Join-Path $run.artifactsDir ('resource-capture-' + [guid]::NewGuid().ToString('N'))
.\scripts\acceptance\four-project-queue\sample-windows-owned.ps1 `
    -ManifestPath $manifest -OutputDirectory $capture -DurationSeconds 1800 -IntervalSeconds 1
```

A single explicitly pinned root can instead use `-RootProcessId`, `-RootCreationFileTimeUtc`, `-RootRole` and `-DiskPath`; array lengths must match. The manifest is clearer for multiple roles. PID identity remains fixed; to add or replace a root, start a new capture. The sampler runs for the requested duration even if all roots exit, retaining a host-only settle interval. Ctrl+C interrupts only the observer's normal loop; it never sends termination requests to other processes or Docker. An interrupted file is partial evidence.

The private output contains `metadata.json`, `samples.jsonl` and `summary.json`. It records timestamps and actual sample duration. Native OS reads can overrun the requested interval; Docker reads have a shared per-sample budget (600 ms by default, adjustable from 100–2000 ms) and a 2 MiB response bound. Topology is bounded to 32,768 system entries and 1,024 owned descendants. Missing/incomplete working sets do not produce a complete summed-memory claim. CPU deltas use pinned identities; process I/O counters include cached/device I/O and are not equivalent to physical disk throughput. Host hard-paging rates are not collected. No process names, command lines, environment values, private disk paths or credentials are written; raw output still contains PID/container identities and should be sanitized before publishing.

Nine controlled checks execute the real PowerShell selection/serialization code with substituted OS/Docker readers and compile/exercise the native HTTP decoder. They cover PID reuse, historical-parent rejection, reparenting, missing data, scope rejection, exact image identity, freshness, deadlines and private-field exclusion. They do **not** call Windows kernel APIs or connect to Docker. Reproduce them without a workload:

```powershell
.\scripts\acceptance\four-project-queue\test-windows-owned.ps1
```

The [controlled-check receipt](../../../docs/luci/evidence/2026-10-08-windows-owned-observer-controlled.json) records Linux PowerShell validation and helper hashes. A separate [native tester report](../../../docs/luci/evidence/2026-10-08-windows-owned-observer-native-reported.md) records nine controlled checks passing on Windows and an 8.017-second, eight-sample native capture of an owned PowerShell root and exact descendant. Host/process/E: disk metrics and protected current-user output ACL creation were reported successful. The cloud session has not inspected those native artifacts or their hashes. Docker was not part of that capture; Docker named-pipe compatibility, Luci process attribution and sustained workload acceptance remain unverified.

For release acceptance, record at least 60 seconds of idle Luci before agents, the [four-project workload](../../../docs/luci/SWARM-CAPACITY-ACCEPTANCE.md), and a settle interval. Mark those phase times separately. Measure UI input/project-switch latency and ordinary editor/browser responsiveness; resource counters alone do not establish them. Keep Docker/WSL VM resident memory separate from its guest containers, and do not assume every `vmmemWSL` process belongs to Luci when other distributions are active. Preserve failed task/recovery cases and verify actual results and integration.

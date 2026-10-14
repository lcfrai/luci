# Local execution on smaller computers

Luci's application, project context and saved conversations remain usable while
new Docker work waits for capacity. This does not promise that every model or
project can run within a small memory allocation. Model-server RAM and GPU memory
are separate from the coding-agent container and must be included in the measured
whole-computer budget.

## Supported resource settings

New local Docker services request **1024 MiB and one CPU per agent**. The machine
menu reports the effective agent allocation. **Admin** can impose lower account
ceilings. Operators can set `LUCI_SESSION_MEMORY_MIB` and
`LUCI_SESSION_CPU_MILLIS` before starting the service to support larger tasks;
the actual allocation is the lower of an explicit service ceiling and current
account sizing. Without an explicit service override, previously saved larger
Admin sizes remain effective; a new account starts at 1024 MiB / one CPU.
Existing containers keep their original limits. Remote VM defaults and provider
templates are unchanged.

In **All projects → Expand capacity and settings → Scheduling settings**, new
workspaces use **Responsive local work**. Its RAM reserve is the larger of:

- The explicit floor, initially 512 MiB.
- The percentage target, initially 25%, capped at 1024 MiB.

The CPU margin remains 25%. The cap does not limit a worker's declared memory or
erase pending allocations. With a 1 GiB worker and no pending allocations, the
initial RAM prerequisite is 2 GiB available on a 4, 8, 16 or 32 GiB host; Docker
must separately have room for all observed and pending workload limits, that
worker and its own reserve. This is an admission calculation, not a claim that a
4 GiB Windows computer can always run Docker and a local model comfortably.

This reserve belongs to the controller's shared machine admission for workers,
checks, previews and runtime preparation. Cloud task microVM admission uses its
separate provider limits, exact template sizing, retained VM usage and spending
authority; the local RAM profile does not resize those VMs or relax those limits.

Existing saved policies keep their uncapped percentage reserve and custom floors.
They change only after the administrator selects a profile and saves its review.
The RAM target cap and floor are editable. The UI reports the effective host and
Docker reserves and actual new agent/build limits; unavailable observations remain
unavailable. There is no hidden fallback to an unbounded container or paid model.

## One-time runtime installation

Runtime image preparation has an independent default **1024 MiB / one CPU** RUN
container budget. It does not inherit a large worker allocation. An operator can
set `LUCI_RUNTIME_BUILD_MEMORY_MIB` and `LUCI_RUNTIME_BUILD_CPU_MILLIS` before
starting the service; the signed-in administrator's check ceilings still apply.
The Go image build uses one compiler worker and a 512 MiB GC target; npm install
uses a 512 MiB JavaScript heap. These are workload controls inside the real Docker
memory/CPU limits, not replacements for them. Docker downloads and daemon overhead
are outside the RUN container limit and still affect whole-host availability.

Install the matching runtime through Settings/onboarding. The separate conservative
40 GiB runtime-storage prerequisite is unchanged. A missing image, unknown inventory,
stale host measurement, CPU contention or insufficient disk/RAM stops a new start.
Saved context stays available. Queue reasons distinguish host RAM from Docker
capacity and include the actual operation, pending demand and reserve.

If an agent exceeds its hard limit, retain and review its failed session/output.
Increase the supported limits only if the computer has capacity, then explicitly
restart. Unknown creation/build outcomes retain ownership and capacity across
restart; cancellation does not release resources until exact cleanup is confirmed.
Changing the profile does not delete containers, stop other applications or alter
Windows, WSL, Docker Desktop limits or the pagefile.

## Portable production source setup

Use a complete published portable package and its launcher when available. The
following builds a local production source artifact; it is not a native installer
or evidence of packaged acceptance. Build before opening a development watcher or
loading a local model. Use an owned artifact directory outside the checkout.

```powershell
# Run from the chosen immutable source checkout. Node, npm, Go, Git and Docker
# must already be installed. Use private state/projects paths you own.
$artifact = 'E:\LuciArtifacts\local-production'
New-Item -ItemType Directory -Path $artifact -ErrorAction Stop
Remove-Item Env:VITE_LUCI_API -ErrorAction SilentlyContinue
$env:NODE_OPTIONS = '--max-old-space-size=1024'
Push-Location desktop
npm ci
npm run build -- --outDir "$artifact\web"
if ($LASTEXITCODE -ne 0) { throw 'Frontend build failed; do not launch.' }
Pop-Location
$env:GOMAXPROCS = '1'
$env:GOMEMLIMIT = '512MiB'
Push-Location control-plane
go build -p 1 -o "$artifact\luci-api.exe" ./cmd/api
if ($LASTEXITCODE -ne 0) { throw 'Service build failed; do not launch.' }
Pop-Location
```

Set `LUCI_WEB_DIR` to that `web` directory and `LUCI_RUNTIME_DIR` to the same
immutable source root, alongside your private `LUCI_STATE_DIR` and
`LUCI_PROJECTS_DIR`. Start the API on a free loopback `LUCI_ADDR` and open its
URL. With `VITE_LUCI_API` unset at production build time, the frontend uses its
own page origin automatically; no test injection or fixed development port is
needed. Keep the MIT licence and dependency notices with any distributed bundle.
Use the standard packaged launcher for managed lifecycle and first-run setup,
or the documented service configuration for a source-managed service.

Resource boundary tests use controlled capacity snapshots. Real upstream model,
Docker, cancellation/restart, preview and whole-computer responsiveness acceptance
must still be performed on the actual target machine; a source build does not
establish those passes.

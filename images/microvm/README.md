# Trusted task guest boundary (CLOUD-084/088)

This is the contract for the `control-plane/cmd/microvm-bootstrap` helper,
not a published E2B template or an installed cloud backend. The controlling
[product contract](../../docs/luci/PRODUCT-CONTRACT.md) still requires actual
provider/KVM, durable host-loss recovery, upstream execution and per-task VM
teardown evidence. The product exposes an explicit configured microVM route,
but no working provider/template has passed live verification.

A candidate OCI guest filesystem is now at `images/microvm/Dockerfile`. From
repository root, `docker build -f images/microvm/Dockerfile -t
luci/microvm-guest:candidate .` builds the fixed Linux bootstrap, collector,
sandboxd and pinned upstream CLI versions, with the root-owned input/context
paths required below. Record the resulting immutable image digest and all
binary/CLI versions before any provider template publication. This local image
build is not an E2B template or microVM execution result. The Embed operator
must import/build it under the selected template ID, record the exact provider
`buildId` and signed source inventory, verify envd and the root/cgroup/network
preconditions on a real guest, and keep that template exclusive to the build
while Luci allocates tasks. The pinned Create API selects a template ID and
cannot atomically pin a build ID; a concurrent template rebuild is therefore
an unresolved operational risk. Luci refuses a non-ready selected build but
cannot infer guest bytes from the status response alone.

The explicit `control-plane/microvm-template.go` operator tool can request one
private Embed template/build ID, then start that build from an OCI registry
image pinned by `@sha256:<64 lowercase hex>`. From `control-plane`:

```sh
go run ./microvm-template.go -mode create -config /private/template-create.json
go run ./microvm-template.go -mode created -config /private/template-create.json
go run ./microvm-template.go -mode reconcile -config /private/template-create.json
go run ./microvm-template.go -mode start -config /private/template-build.json
go run ./microvm-template.go -mode status -config /private/template-build.json
```

The creation JSON contains `endpoint`, `apiKeyFile`, optional `rootCaFile`, a
unique `luci-...-<32 lowercase random hex>` `name`, `cpuCount`, `memoryMB`, and
`journalPath`. The tool refuses a name already visible in complete provider
inventory before Create. The separate
build JSON uses the returned `templateId` and `buildId`, immutable `fromImage`,
the absolute `createJournalPath` from the private Create request, and a
**different** `journalPath`. Before the one build POST, Luci reopens the
acknowledged Create intent and response and requires exact matching template
and build IDs. A missing, changed or unacknowledged Create never grants build
authority. Keep the key/config and
the journal's directory private and outside Git. The tool requires protected
HTTPS management, checkpoints each intent before its **single** provider POST,
and refuses another POST for that journal even after a lost response. `created`
reads only the acknowledged template/build IDs; `reconcile` uses one bounded
read-only provider inventory to settle a unique private name, exact CPU/memory
and sole build after a lost Create response. Missing, paginated or ambiguous
inventory remains unknown and never causes another Create. `status` reads only
the same build ID. On any uncertain response retain the journal; never delete
it to guess at a retry. The image must already exist in
a registry reachable by Embed. This tool neither proves final guest bytes, KVM
isolation nor model execution. Independently bind
the source inventory, OCI digest and resulting ready provider build before
using `template.buildId` and `template.buildDigest` in worker configuration.

The helper reuses the existing sandbox image's Git, unprivileged `dev` identity,
`entrypoint.sh`, `sandboxd` and upstream coding engines. It does not implement an
agent engine. Build the Linux helper from the same reviewed source/module lock
as the controller:

```sh
cd control-plane
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o ../artifacts/luci-microvm-bootstrap ./cmd/microvm-bootstrap
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o ../artifacts/luci-microvm-collect ./cmd/microvm-collect
```

That command alone does not assemble or verify a provider template. A template
publication must bind the exact executable, existing sandbox image digest,
upstream CLI versions, envd version and third-party licence inventory. Include
this command's compiled Go dependencies in the inventory when distributing it;
the ordinary API/sandboxd inventory alone is insufficient before API wiring.

## Required trusted template

- Linux with the pinned Embed envd protocol, an independently verified private
  authenticated management/guest route, and explicit model egress policy. The
  controller now sends an operator-approved exact-domain `allowOut` list over
  `denyOut: ["0.0.0.0/0"]` and refuses an unlisted model endpoint before VM
  allocation. The pinned API documents allow-list precedence. This source
  behavior does not establish a working model connection, other IPv4 egress
  refusal on a real guest, or any IPv6 policy.
- Unified cgroup v2 mounted at `/sys/fs/cgroup` from the real root, with a
  root-owned, undelegated `/sys/fs/cgroup/luci-tasks` parent. The root collector
  and envd must run outside every task group. No task process may write cgroup
  controls or move itself/descendants out of its group. Host/container cgroup
  namespaces alone do not establish this template contract.
- Existing sandbox tools at their fixed absolute paths, `dev` UID/GID1001 and
  the unchanged privilege-dropping entrypoint. Root must retain the ability to
  make the one read-only context bind mount before dropping capabilities.
- `/luci-transfer` and `/luci-work`: ordinary root-owned directories, neither
  group- nor world-writable. `/luci-work/attempt` must not already exist.
- `/luci-transfer/input.tar`: precreated ordinary root-owned single-link file,
  mode0600. The pinned envd upload endpoint preserves an existing mode; its
  default mode for a new file is not a private-context boundary.
- `/home/dev/.luci-reviewed-context`: precreated empty ordinary root-owned
  directory, neither group- nor world-writable. Every ancestor, including
  `/home/dev`, stays root-owned and non-writable after the trusted entrypoint
  drops privilege. The upstream process gets a separate writable
  `/home/dev/.luci-agent-home` as `HOME`. The helper checks this hierarchy
  before mounting context read-only/no-exec/no-suid/no-dev. A symlink or
  writable ancestor is not an equivalent boundary.

Do not run a generic image entrypoint as a task before the controller's single
authenticated Start. The controller supplies the exact operation/project/task/
attempt and reviewed source/context/archive digests as fixed arguments. Its
private launch configuration supplies the upstream command, argument array,
attach token and permitted model connection. Context files are data; they
cannot choose commands, credentials or deployment authority.

## Preparation and uncertainty

The helper reserves a fresh private directory, durably records the intended
input, bounds and verifies its archive, then materializes only the listed
regular files. It preserves user AGENTS.md/CLAUDE.md bytes. Altered bindings,
extra bytes, cancellation and existing destinations cannot grant launch. An
opaque prepared handle rechecks the exact directory and receipt before later
permission or launch changes.

Source becomes writable by UID1001. Reviewed context remains root-owned under
a root-owned parent after entrypoint UID bootstrap and is exposed through the read-only bind mount. A new Git
repository contains the exact input as a **synthetic guest baseline**: original
host Git configuration, hooks, credentials and history are not imported. Its
recorded commit is not the original host commit. Return/review wiring must use
the retained original source digest and transfer binding, not relabel this
synthetic commit as host history.

A durable one-shot launch reservation precedes execution of the existing
entrypoint. A crash or failed exec after it is unknown; do not invoke bootstrap
again in that VM. A failed read-only remount stops before execution but can
leave the first mount present. Retain the failure and reconcile/retire that
exact VM; this is not a cleanup success. A process PID acknowledgement proves
only envd fork, not prepared input, sandboxd readiness, model execution or task
success.

## Exact task output capture

Bootstrap records root-protected `launch-kernel.json` with the acknowledged
process PID, VM/request/launch/input identities, boot ID and task-cgroup inode,
then moves itself into the fresh `luci-tasks/<operation>` group before exec.
The controller must authenticate and durably retain this receipt. It observes
the existing sandboxd typed Exit frame independently; stdout or collector exit
cannot declare upstream success. Native completion alone never releases a VM.

The fixed root collector takes only `--identity-base64` containing that exact
prior JSON identity. It reloads protected original input and launch markers,
checks the kernel boot/inode and that it runs outside the task, requests freeze,
and waits for actual `cgroup.events` frozen=1. Cancellation/uncertainty leaves
the group frozen and the VM charged; it never supplies an unfreeze action.
The collector executes no task Git configuration, hooks or commands.

It writes one exclusive capture intent and `/luci-transfer/output.tar`, then
publishes `/luci-transfer/output.json` only after archive verification, durable
writes and another exact freeze observation. Partial files remain diagnostic
uncertainty and cannot trigger a second capture. The archive contains full
bounded source under `output/source/`, plus hash-bound `output/source-manifest.json`
and `output/changes.json`. Original transferred paths remain represented when
changed/deleted, even if the task edits ignore files. Fixed exclusions apply
only to new dependency/cache/Git/credential paths; exclusions are enumerated.
Links, special files, unowned/cross-device files and exceeded bounds refuse
capture. Current limits are4094 source files,63MiB source plus1MiB metadata,
16384 visited entries and1024 directories, with the underlying portable archive
path limits. Unsupported larger repositories require an explicit future
transfer policy; no truncated-success claim is made.

The controller-side `ReceiveOutput` still validates returned data; complete
collector invocation, result import/review, product scheduler/backend wiring,
template build and live VM lifecycle remain required integration work. Missing cloud credentials or KVM block live
operations only; source and controlled validation continue independently.

## Focused checks

```sh
cd control-plane
go test -race -count=1 ./internal/microvmguest ./internal/backend/e2bbe
```

The guest-local package is Linux-only, including its directory-fsync and ownership
contract. Controller-side archive/transport fixtures in `e2bbe` remain separate
from Linux root permission and real Git checks. `TestProtectedGuestContextCannotBeReplacedByUpstreamUser`
explicitly skips without root; a skipped test is not an ownership pass. A
controlled root test in a Docker container can verify Linux ownership/Git
behavior, but cannot establish a real microVM, mount capability, upstream
provider execution, template provenance or cloud resource settlement.

Controlled source evidence is in [the output-capture capsule](../../docs/luci/evidence/cloud084-guest-capture/README.md). Kernel observations there are fixtures. Root freezer/mount/UID permissions, executable template provenance, live model execution and real per-VM teardown have **not** passed.

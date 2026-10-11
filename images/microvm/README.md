# Trusted task guest boundary (CLOUD-084/088)

This is the contract for the new `control-plane/cmd/microvm-bootstrap` helper,
not a published E2B template or an enabled cloud backend. The controlling
[product contract](../../docs/luci/PRODUCT-CONTRACT.md) still requires actual
provider/KVM, durable host-loss recovery, upstream execution and per-task VM
teardown evidence. The current product does not offer a working microVM choice.

The helper reuses the existing sandbox image's Git, unprivileged `dev` identity,
`entrypoint.sh`, `sandboxd` and upstream coding engines. It does not implement an
agent engine. Build the Linux helper from the same reviewed source/module lock
as the controller:

```sh
cd control-plane
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -o ../artifacts/luci-microvm-bootstrap ./cmd/microvm-bootstrap
```

That command alone does not assemble or verify a provider template. A template
publication must bind the exact executable, existing sandbox image digest,
upstream CLI versions, envd version and third-party licence inventory. Include
this command's compiled Go dependencies in the inventory when distributing it;
the ordinary API/sandboxd inventory alone is insufficient before API wiring.

## Required trusted template

- Linux with the pinned Embed envd protocol, an independently verified private
  authenticated management/guest route, and explicit model egress policy. The
  current allocation boundary denies outbound IPv4; it does not establish a
  working model connection or IPv6 policy.
- Existing sandbox tools at their fixed absolute paths, `dev` UID/GID1001 and
  the unchanged privilege-dropping entrypoint. Root must retain the ability to
  make the one read-only context bind mount before dropping capabilities.
- `/luci-transfer` and `/luci-work`: ordinary root-owned directories, neither
  group- nor world-writable. `/luci-work/attempt` must not already exist.
- `/luci-transfer/input.tar`: precreated ordinary root-owned single-link file,
  mode0600. The pinned envd upload endpoint preserves an existing mode; its
  default mode for a new file is not a private-context boundary.
- `/home/dev/.luci-reviewed-context`: precreated empty ordinary root-owned
  directory, neither group- nor world-writable. The helper bind-mounts reviewed
  context here read-only/no-exec/no-suid/no-dev so existing harness prompts keep
  their established path. A symlink beneath writable `/home/dev` is not an
  equivalent boundary.

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
a root-owned parent and is exposed through the read-only bind mount. A new Git
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

The separate `e2bbe.ReceiveOutput` validates a returned archive as data. It does
not produce an archive, establish that writers stopped, or integrate changes.
The exact terminal observation, quiesced output producer, original-baseline
review/import, scheduler/backend wiring, template build and live VM lifecycle
remain required integration work. Missing cloud credentials or KVM block live
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

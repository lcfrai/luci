# Private Azure E2B Embed host package

This module is the **offline deployable infrastructure definition** for the
audited Azure design at `6bf391dc`. It is not a live Azure receipt. It creates
one dedicated runtime VMSS host model, a separately identified controller VM
and protected managed disk, private VNet/DNS/load balancer, a Key Vault private
endpoint, distinct managed identities and exact per-secret read grants. The
runtime subnet NSG admits only the selected controller to private TLS port 443
and the Azure fabric TCP health probe; it denies all other inbound traffic.
The host service adds a default-deny nftables table. E2B's host-network
ports 3000–3002 and unauthenticated 5008 are never published by this module.
The guest/task isolation boundary is E2B Firecracker: **one newly owned guest
microVM per Luci task**, not one process/container per task on the VMSS host.

## Immutable image/package build inputs

Use an isolated Ubuntu 24.04 **image build root**, not a shared user host. It
must contain the pinned Docker Engine and Compose plugin, Python 3, nftables,
iproute2, systemd and an Envoy executable whose original release digest and
licence have been independently recorded. Build the existing Luci Go commands
from one clean, immutable Luci commit. Supply a clean upstream E2B runtime
checkout at `cf536c284434fdd2f09bbab35ec807a5dacc87e4`, including its
`embed/compose` tree and Apache-2.0 licence. `image-digests.json` must map
**every** upstream Compose service name to a complete `image@sha256:<64 hex>`
reference. The package builder refuses a missing service, floating image,
source change, symlink or incomplete manifest.

```
cd control-plane
go build -o /private-build/azure-embed-host-bootstrap ./cmd/azure-embed-host-bootstrap
go build -o /private-build/azure-gateway-config ./cmd/azure-gateway-config
cd ..
python3 scripts/azure-embed-host/package.py build \
  --upstream /private-build/e2b-runtime \
  --luci /private-build/luci-checkout --luci-revision "$EXACT_LUCI_COMMIT" \
  --image-digests /private-build/image-digests.json \
  --bootstrap /private-build/azure-embed-host-bootstrap \
  --gateway /private-build/azure-gateway-config \
  --envoy /private-build/envoy --envoy-license /private-build/ENVOY-LICENSE \
  --output /private-build/azure-embed-host.tar
python3 scripts/azure-embed-host/package.py verify \
  --archive /private-build/azure-embed-host.tar --sha256 "$PACKAGE_SHA256"
python3 scripts/azure-embed-host/package.py install-image \
  --archive /private-build/azure-embed-host.tar --sha256 "$PACKAGE_SHA256" \
  --root /private-build/image-root
```

The `build` command prints `PACKAGE_SHA256`; bind that exact value to
`runtime_package_sha256` and publish the image as one immutable Azure Compute
Gallery image version. Bind its full ARM version ID to
`runtime_image_version_id`. The controller image is **separate** and has its
own immutable `controller_image_version_id`. The image root installation
places the archive, checked manifest, binaries, Compose tree and three systemd
units, but does not enable units or contact Azure/Docker. An actual image build
and original-image/licence verification remain deployment prerequisites.

The module's VMSS custom data contains only a nonsecret deployment selection:
scope/generation, source package digest, private addresses, managed-identity
client IDs and **Key Vault secret names/versions, never values**. Cloud-init
mounts a dedicated `/run/luci_embed` tmpfs and enables the installed host
service. The host service rechecks its image archive and every installed file,
requires a real KVM device and one selected private NIC, reads exact Key Vault
versions through its managed identity, then validates the pinned Compose,
private Envoy TLS/routes and nftables rules before effecting anything. It
journals firewall, Compose and gateway starts **before** each effect. A lost
acknowledgment refuses automatic replay. After a reboot it may restage the
same selected secret versions and gateway if the already-owned Compose
services are observed running; it does not call Compose `up` again.

Only gateway certificate/CA versions may hot-rotate under a new generation
using `service.py rotate` and `previous_generation`. Runtime team/admin/seed
key changes require a drained, fenced replacement host because existing
guests and Compose containers may still hold the old values. A rotation with
an uncertain gateway restart retains both generations in the host journal;
the service refuses replay. VMSS uses **Manual** upgrade mode so a Terraform
model change cannot replace a host with live guests. The module emits
`runtime_host_config_sha256` for comparison. `scripts/azure-embed-host/deliver.py`
is the protected delivery path on the currently fenced controller. Its private
selection binds the current mounted ext4 UUID/generation receipt, exact
disk/controller/VMSS instance, selected nonsecret host config and digest. It
verifies current Azure readback and saves an fsynced `run_command_pending`
record on the authoritative disk **before** calling the selected VMSS
instance's Azure guest-agent RunCommand. The controller identity's custom
role permits only instance read and RunCommand on this VMSS. The host command
calls `service.py deliver`, which refuses changed deployment/credential
boundaries, journals gateway rotation before the effect, and persists the new
config only after rotation settles. Only an exact returned host receipt marks
delivery observed. A missing acknowledgment or output retains the action and
operation identity for read-only reconciliation; repeating the command is
refused. Terraform's manual model update and a successful ARM POST alone do
not prove the running host rotated. The existing private gateway generation
readback remains mandatory before new task effects.

## Boundary and remaining run gates

`host_secret_refs` and `host_secret_ids` are exact, role-scoped **references**;
`controller_secret_ids` belong to a different identity. The selected vault
must use RBAC and its private endpoint. No PEM, API key, access token or SAS
belongs in Terraform variables, state, cloud-init or logs. The protected
controller must read the gateway generation receipt and verify current ARM
disk/instance ownership before fresh E2B effects. This module creates and
attaches a controller disk but does **not** format/mount/fence/restore the
authoritative Luci journal on it. That required automation remains the next
source implementation gap, not an operator attestation substitute.

Subscription, region, SKU/nested-KVM capability, quota, private DNS/TLS/Key
Vault setup, actual image and guest template, budget, controller disk fencing,
task execution, 12-task/4-project/4-concurrent distinct guests, provider
usage/final invoice and per-VM retirement all need later authorized Azure
evidence. No `terraform plan/apply`, Azure API or E2B call was used to validate
this offline module. `terraform init -backend=false`, `terraform fmt -check`
and `terraform validate` check source/schema only.

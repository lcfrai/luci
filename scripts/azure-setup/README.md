# Azure Develop setup bundle

This archive is supplied by an installed Luci package at
`GET /v1/setup/cloud/azure/bundle`. Compare its `X-Luci-Archive-SHA256` header
with the `bundleSHA256` from `POST /v1/setup/cloud/azure/plan`. The archive
manifest binds every member to the exact Luci source commit. This is for
**Develop microVM execution**, not the older deployment-target cloud VM.
Run the commands on Linux or WSL2 with Python 3, Go, Terraform, Azure CLI and
isolated Ubuntu image build roots. The Windows app supplies and verifies the
bundle; it does not run Linux image construction in PowerShell.

After `init-selection`, run `configure-guided --bundle . --selection
/private/selection.json` for ordinary account, durable-state, host-size, public
SSH-key and Key Vault questions. This reuses the same account validator as the
flag-based command; no Terraform or operator JSON needs to be authored for
those selections. `status` reads the retained local stage journals, including
uncertain operations, without querying Azure or claiming a connected workspace.

For an existing generalized managed image containing the exact prepared root,
`gallery-plan --bundle . --selection /private/selection.json --role runtime
--image-definition-id <exact-existing-definition-ARM-ID> --version 1.0.1
--managed-image-id <exact-generalized-managed-image-ARM-ID>` saves a source and
package-bound publication plan. Repeat for `--role controller`. Review its
region, single replica, source, costs and `planDigest`, then use
`gallery-publish --role runtime --plan-digest <exact-digest> --acknowledge-costs`
with the same bundle/selection. It establishes bounded version absence before
one journaled ARM write and requires exact source/identity readback. An unknown
write must use `gallery-reconcile`, which performs only GET, never another PUT.
The source managed image must actually contain the prepared root; tags and a
gallery receipt do not prove filesystem contents, KVM or successful tasks.
Building/generalizing/uploading a VHD from the prepared root remains a distinct
prerequisite. No live publication has been exercised by these offline checks.

1. Extract into a private directory with `mkdir -m 700 luci-azure-setup` and
   `tar -xf luci-azure-setup.tar -C luci-azure-setup`.
2. Run the exact `init-selection` command returned by the plan API. It takes
   the name, region, monthly budget and maximum concurrent VMs and creates
   an owner-only file with Luci ownership IDs, generation, disk UUID, private
   DNS and service paths. An optional `--private-network-cidr` derives
   runtime/controller subnets from an unused private `/16`; confirm it does
   not overlap an existing network. Run `plan` to see account-specific missing
   inputs. Use `configure-account --bundle . --selection
   /private/selection.json --subscription-id ... --resource-group ...
   --state-resource-group ... --state-storage-account ... --state-container ...
   --zone ... --runtime-size ... --controller-size ...
   --ssh-public-key-file /absolute/path/to/public-key.pub` to save common
   account/host choices without editing Terraform JSON. The command refuses
   changes once image preparation or provisioning has a retained record. An
   **existing** Azure Blob container for
   Terraform state is required. It must be separate from the Luci controller
   disk and the off-disk writer-fence Blob. Authenticate with your own Azure
   sign-in; no token or Key Vault secret value belongs in this JSON.
3. On a private setup host, `generate-secrets` creates a private CA, gateway
   server and controller client certificates and independent API material for
   the selected scope/generation. Select an existing private Key Vault using
   `configure-account --key-vault-id ... --key-vault-name ...`, then run
   `publish-secrets --acknowledge-credential-publication` and `bind-secrets`.
   Publication journals each exact secret before a write. An uncertain write
   is never repeated: `reconcile-secrets` reads exact versions and either
   settles one unique matching version or keeps the operation unknown. Keep
   `azure-credentials/` owner-private for controlled rotation; no secret value
   enters Terraform state or the plan API. These commands require Azure CLI
   authentication and explicit authority; the offline plan itself does not.
4. Supply immutable source-bound host/controller packages, exact Azure Compute
   Gallery image version IDs and a selected signed E2B guest template. Obtain
   the exact Luci commit named in `manifest.json` from
   `https://github.com/lcfrai/luci.git` using your existing source access.
   The included `azure-embed-host/package.py` and
   `azure-controller-disk/package.py` build and verify image payloads from
   that clean commit and E2B runtime commit
   `cf536c284434fdd2f09bbab35ec807a5dacc87e4`. Use
   `init-image-inputs --bundle . --selection /private/image-inputs.json`
   with `--luci-source`, `--e2b-source`, `--image-digests`, `--envoy-binary`,
   `--envoy-license`, `--runtime-image-root`, `--controller-image-root` and
   `--operator-template` to save those selected paths without editing an
   operator JSON of local paths. The `prepare-images`
   action compiles the three pinned Luci Linux commands, verifies both
   packages and installs them into **distinct user-selected isolated image
   roots**. It requires an owner-only JSON with exactly `luciSource`,
   `e2bSource`, `imageDigests`, `envoyBinary`, `envoyLicense`,
   `runtimeImageRoot`, `controllerImageRoot`, `operatorTemplate`, plus the
   explicit `--acknowledge-image-roots` flag. Python PyYAML, Go and the pinned
   Envoy binary/licence are local build prerequisites. The generated package
   SHA values must be copied into the private selection. The operator
   template contains exact Key Vault **references**, never secret values.
   The image roots still need to be published as immutable Azure Compute
   Gallery versions by the account owner; `prepare-images` does not upload,
   provision or start anything.
5. Run `python3 azure-setup/installer.py plan --bundle . --selection
   /private/selection.json` from this extracted directory. `plan` is
   offline and reports each missing input. Then run only its exact `prepare`
   command after the prerequisites are configured. `prepare` performs
   authenticated Azure read-only Terraform planning; it does not provision.
6. Review the provider plan, actual account/region pricing, limits and the
   exact resource inventory. Run the exact `apply` command with its two
   acknowledgments only when the existing authority and budget permit it.
   A lost `apply` response leaves `apply_pending`; inspect and run the
   read-only `reconcile` command against the separately durable Terraform
   state before any further operation. Never replay `apply`. Reconciliation
   requires exact refreshed ARM identities and still reports
   `infrastructure_observed_pending_private_handoff`; it does not activate
   the controller, publish a gateway or prove private connectivity. The
   `bootstrap` command performs one journaled private controller
   initialization after readback; an unknown response remains pending.

The installed API returns this same source and archive identity at
`POST /v1/setup/cloud/azure/plan`. Its request is
`{name,region,monthlyBudgetUsd,maxConcurrentVMs}`. The returned JSON is an
offline setup proposal; saving the first-run BYO choice is separate, and
neither call creates Azure resources. The installer writes owner-only
`azure-setup-state.json` and `azure-image-prep-state.json` before effects.
Do not delete either pending record to retry an unknown provider effect.
Gallery image versions, Key Vault secret/certificate versions, a durable Blob
backend, suitable KVM VM sizes, SSH keys and a signed E2B guest template are
genuine account-specific selections. The installer does not invent them or
silently create billable resources. Guided account discovery, gallery
publication and verified workspace linking remain implementation gaps; a
generated selection and package alone are not a working cloud connection.
Teardown must first settle all Luci-owned task VM attempts and reservations,
verify a durable controller backup and then review provider resource/charge
state. The protected controller disk has `prevent_destroy`; this installer
does not erase it or infer a final Azure bill from a stopped VM.

The authority store, returned task artifacts, owned microVM retirement and
final provider billing are distinct. An installed archive, generated plan or
Terraform output does not establish live Azure, KVM, guest isolation, a
completed task, released Luci reservations or a final provider invoice.

Controller template staging uses `prepare-operator --guest-image IMAGE@sha256:...
--cpu-count N --memory-mib N --task-max-seconds N
--task-estimated-cost-micros N --model-domain api.example.com`, with the same
`--bundle` and `--selection` arguments. It generates the deferred operator and
signed source selection after exact Key Vault credential publication. Use the
generated operator-template path for image preparation. Its package hashes are
bound automatically; `bind-images` consumes both exact observed gallery receipts.
After infrastructure readback and bootstrap, `template-step --mode create` and
`--mode start` require `--acknowledge-template-publication`; `--mode reconcile`
and `--mode status` read retained provider/journal state. These operations run
only on the protected controller using its managed identity. The setup host
never accesses the runtime gateway or receives private credential material.
Then `finalize-operator` signs the exact current controller/template receipts;
`install-operator --finalization-digest SHA256` installs that verified result.
An unknown installation requires `reconcile-operator` with the same digest.
Do not delete pending records. Account management can start while setup is
deferred, but task execution remains refused until finalization. The signature
on a chosen OCI digest does not establish guest execution or Azure readiness.

`configure-guided` now discovers the signed-in subscription and private VNets in
the selected region. Choose an existing endpoint subnet and listener to prove
your existing route. The command proposes two unused /24 task/controller
subnets only after authenticated address-space and DNS inventory. It stops for
review of a new separately owned foundation, or verifies reuse of existing
private state/Key Vault/DNS resources. No public IP, VPN, Bastion or peering is
created. An existing private route and endpoint subnet remain prerequisites.

The exact foundation proposal lists private StorageV2 state/version retention,
private container, RBAC-only private Key Vault with purge protection, the two
private endpoints and necessary DNS/RBAC resources. Existing linked DNS zones
stay externally owned; only the new endpoints' reviewed zone groups reference
them. Review the actual account quote, retained resources and access authority.
Run `foundation-step --plan-digest SHA256 --acknowledge-costs` for one operation
at a time. A pending operation permits only `foundation-reconcile --plan-digest
SHA256` (GET readback), even if a read returns absent. Never erase its journal.
After all exact properties are observed, `foundation-bind` requires private DNS
and authenticated Blob/Key Vault data-plane reads before selecting the remote
Terraform backend. Then rerun `configure-guided` for account-available sizes,
zone and an SSH public key. Dsv5 is a documented nested-virtualization family;
the offered sizes are choices for a quote, not KVM or workload acceptance. See
[Azure Dsv5](https://learn.microsoft.com/en-us/azure/virtual-machines/sizes/general-purpose/dsv5-series)
and [private DNS integration](https://learn.microsoft.com/en-us/azure/private-link/private-endpoint-dns-integration).
Foundation journals precede remote state and preserve separate ownership.
Existing private infrastructure is never silently imported or overwritten.

Normal guided controller images contain the installed service/binaries and
licences, with no active account/generation selection. The public CA, deferred
operator references and loopback service environment are staged by the existing
authenticated initialization channel only after exact controller creation.
They are not private credential values. Unknown staging/mount results retain
the off-disk bootstrap journal and cannot repeat the write/format branch.

Private VM-source image preparation is available through `builder-source`,
`builder-plan` and `builder-step`. `builder-source` archives the exact clean
Luci and pinned E2B runtime checkouts plus a selected Envoy binary/original
licence; it performs no compiler work on the setup PC. `builder-plan` selects
an exact `Canonical:ubuntu-24_04-lts:server:VERSION`, available builder size,
new gallery name/version and admission lifetime, and prints all temporary and
retained resources/permissions for price review. Never use `latest` as a base
image version. Original source trees/commit bytes and licence inventory are
verified on the builder before building its role package.

For the reviewed plan digest, `builder-step --stage STAGE --role ROLE
--plan-digest SHA256 --acknowledge-costs` advances one explicit operation.
Stages are staging, allocate, grant, install, deprovision, deallocate,
generalize, capture and retire. Run the runtime role first, then the controller;
the second allocation is refused until the first builder's observed retirement.
Use `--reconcile` for an unknown response; it reads exact retained state without
repeating create/upload/install/generalize/capture/delete. The temporary system
identity has Blob Data Reader at the separate owned staging container only.
Complete-byte download verification is required; metadata/tags are insufficient.
Each VM-source capture retains its gallery version separately from temporary
VM, private OS disk, NIC and grant cleanup. `--stage bind` binds both observed
packages/images without downloading binaries to the setup PC;
`--stage retire-staging` removes only exact owned staging bytes and an empty
owned container. Compute, disk, staging, gallery and final invoice outcomes
remain separate. Missing egress/private reachability or an incomplete operation
stays unresolved; no public connectivity is added. These adapters have offline
fixture evidence only. `--stage abort --role ROLE --acknowledge-costs` records an
explicit stop request for an incomplete builder. An unknown create/install/capture
blocks deletion until its exact readback resolves; no intent is discarded.
Subsequent abort steps observe deallocation, revoke only the exact temporary
grant and delete only the owned VM and positively bound unattached OS disk/NIC.
Run abort for both roles (including a never-allocated role), then retire-staging.
Published gallery artifacts and other foundation/runtime resources remain retained;
abort receipts do not establish a zero bill. New production/image binding is refused
after abort. Normal single-command progression remains in development.


Normal private image production also builds the exact Luci microVM guest source
on the existing runtime builder. Its base image digests, actual harness version
probes, complete OCI manifest/config/layers and original grants are retained with
an immutable source receipt. The runtime image carries those nonsecret bytes in
a read-only Distribution registry bound only to `127.0.0.1:5055`; no Azure registry,
public port, private registry credential or additional managed service is created.
The exact upstream Distribution 3.0.0 executable archive and original licence are
checksum-bound, and the runtime service cannot upload, delete, proxy or rebuild.
Upstream runtime service images are actually preloaded before capture. The Azure
stack disables the independent automatic `e2bdev/base` build/ready services.

`prepare-operator` normally omits `--guest-image`: it derives the observed guest
digest from the completed image producer and refuses source/reference substitution.
The explicit verified existing-registry path remains available. Startup and reboot
verify all retained source bytes and the owned loopback listener. Protected template
create/start inspect the actual runtime source before provider mutation. Source
restoration from the exact gallery image does not restore tasks, template metadata,
projects, artifacts or active journals; their existing independent durable recovery
and predecessor fence still apply. No actual Azure, KVM, guest task, native registry
or billing outcome is established by the offline tests.

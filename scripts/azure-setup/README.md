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

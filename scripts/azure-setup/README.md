# Azure Develop setup bundle

This archive is supplied by an installed Luci package at
`GET /v1/setup/cloud/azure/bundle`. Compare its `X-Luci-Archive-SHA256` header
with the `bundleSHA256` from `POST /v1/setup/cloud/azure/plan`. The archive
manifest binds every member to the exact Luci source commit. This is for
**Develop microVM execution**, not the older deployment-target cloud VM.
Run the commands on Linux or WSL2 with Python 3, Go, Terraform, Azure CLI and
isolated Ubuntu image build roots. The Windows app supplies and verifies the
bundle; it does not run Linux image construction in PowerShell.

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
3. Supply immutable source-bound host/controller packages, exact Azure Compute
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
4. Run `python3 azure-setup/installer.py plan --bundle . --selection
   /private/selection.json` from this extracted directory. `plan` is
   offline and reports each missing input. Then run only its exact `prepare`
   command after the prerequisites are configured. `prepare` performs
   authenticated Azure read-only Terraform planning; it does not provision.
5. Review the provider plan, actual account/region pricing, limits and the
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

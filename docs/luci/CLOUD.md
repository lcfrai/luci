# Existing management and application cloud rollout

## Current status — scope audit passed, implementation resumed (12 October 2026)

The originating local thread explicitly lifted the scope gate for contract revision `b3ecb0114dba6b6e0dfcfedbf1d3a3d177794707`. **Aligned implementation, delegation and verification are authorized to resume now; no further scope confirmation is needed.** This is scope alignment, not product acceptance or platform-goal activation. Existing provider credentials, cloud cost/authority policies and all unfinished release gates remain applicable. See [the audit-lift record](SCOPE-AUDIT-LIFT-2026-10-12.md). Any preserved paused-state wording below describes the prior gate, not a current stop instruction.


[PRODUCT-CONTRACT.md](PRODUCT-CONTRACT.md) controls scope; its audit gate was lifted at `b3ecb011`. **This page documents existing VM/Docker management, SSH workspace compatibility and application deployment. It does not establish live cloud Develop microVM acceptance.** CLOUD-084 has a configured E2B task path under implementation; CLOUD-088 must allocate each cloud task subagent its own microVM with elastic scheduling and observed teardown. A remote Docker host, these VM presets and a Coolify application are not those capabilities.

The default Develop path is local Docker. The existing remote workspace stores projects and runs the retained container compatibility path on the selected SSH host. The desktop stays on the person's computer, and management stays behind verified SSH. Coolify handles public application deployment and HTTPS. Deployment location and coding execution location are separate. Per-person Luci and team coordination must lead through actual policy-permitted deployment; this infrastructure guide does not reduce that journey to manual setup.

Local Luci has no licence or platform fee and creates no cloud-host bill. Paid model usage is separate. The existing **Small** preset is for the management/application Docker host described here; increase capacity after measuring its workload. It is not a selected microVM host, KVM/nested-virtualization guarantee, swarm concurrency ceiling or all-in cloud quote. MicroVM execution requires separately verified provider capabilities, persistence/recovery and resource/cost policy in [CLOUD-MICROVM-EXECUTION.md](CLOUD-MICROVM-EXECUTION.md).

| Provider | Small, selected initially | Standard, explicit upgrade |
| --- | --- | --- |
| AWS Lightsail | `medium_3_0`: 2 vCPUs, 4 GB RAM, 80 GB SSD | `large_3_0`: 2 vCPUs, 8 GB RAM, 160 GB SSD |
| Azure | `Standard_B2als_v2`: 2 vCPUs, 4 GiB RAM, 64 GiB Standard SSD | `Standard_B2as_v2`: 2 vCPUs, 8 GiB RAM, 64 GiB Standard SSD |

The optional **Full data stack** preset provides four cores so Docker can reserve
the data services alongside one application and one agent: AWS `xlarge_3_0`
has 16 GB RAM; Azure `Standard_B4als_v2` has 8 GiB. Supabase installation is
optional and remains a separate resource choice. Restoring stages another stack;
stop existing data services first when the host cannot reserve both. Luci checks
actual Docker capacity before installation or restore. Provider availability and
pricing still require the selected account and region.

[Coolify lists a 2-core, 2 GB minimum](https://coolify.io/docs/start-with-self-hosted#_3-minimum-hardware-requirements), but that excludes Luci's agents, builds and deployed apps. Keeping 4 GB as the starting preset is a practical headroom choice, not a claim that every application fits. Start with one active build or agent session, monitor memory and disk, and avoid concurrent heavy work on Small. The cheaper 2 GB Lightsail tier is not the preset for this combined workload.

[AWS's public pricing page](https://aws.amazon.com/lightsail/pricing/) lists the IPv4 4 GB bundle at US$24/month as checked on 2 October 2026. Luci still obtains its bundle quote from the selected region and account; missing, zero, invalid or inactive prices remain unverified. A verified bundle price is not an all-in bill. Azure pricing and subscription availability remain unverified until reviewed with the provider; VM, managed disk and public address are separate charges. The chosen [Azure Basv2 sizes are x86-64, with 4/8 GiB memory](https://learn.microsoft.com/en-us/azure/virtual-machines/sizes/general-purpose/basv2-series); they replace the [retiring Bv1 family](https://learn.microsoft.com/en-us/azure/virtual-machines/sizes/general-purpose/bv1-series). No preset is advertised as the globally cheapest regional offer, and unavailable sizes are not silently replaced with larger machines.

The entered budget gates a known machine quote; it is not a provider billing cap. Domain registration, model usage, backups, traffic beyond the bundle allowance and tax can add cost. Larger configurations and cloud creation require explicit choices; local mode stays available without creating resources.

1. Install the provider CLI if the readiness card links to it. Use **Sign in**; credentials remain in its credential cache.
2. Generate a Luci SSH key or choose an existing public key and private-key path.
3. Choose AWS Lightsail or an Azure VM, region, domain and budget. Generate the plan and inspect its resources and cost assumptions. Missing prices remain visibly unverified.
4. Accept ongoing billing and any incomplete quote separately. Create resources. Luci refuses existing stack/resource-group names and records the operation before issuing creation.
5. Refresh provider progress until the public IP is available. Installation can continue after the provider reports completion.
6. Inspect SSH, compare the host fingerprint with a trusted source such as the cloud console, and connect. Luci starts a localhost-only management tunnel and closes it on exit.
7. For the retained SSH workspace/management capability, explicitly accept installation of the bundled matching service. Luci transfers it over SSH, verifies the archive, installs its system service and checks readiness. Activate only after that succeeds. Discover, Design, files, Git, existing container sessions and previews then use projects on that host. This remains remote Docker, not microVM Develop. Existing local projects are not automatically uploaded.
8. The existing compatibility path installs the container agent runtime and connects a model account in the selected instance. Provider sign-in stays on the desktop; model credentials are not implicitly copied. Its containers consume host resources in addition to Coolify/apps; they do not satisfy CLOUD-084/088. A preview through its separate SSH bridge is not public deployment.
9. To publish publicly, complete the deployment service's first-account setup in the protected management view. Create an API token and connect it in Luci; select the server if more than one exists. Add the domain's A record pointing to the public IP and check DNS. In Deploy, select the target, reachable Git repository, exact commit, domain and health path. Coolify builds the app and provisions HTTPS; Luci verifies health before recording readiness. A local-only Git repository cannot be fetched by Coolify until it is published somewhere the server can access.
10. To return to local development, select the local workspace. Its projects stay on this computer; the cloud workspace's projects stay on the server. A disconnected cloud workspace shows an error instead of silently using local files. Restart reconnects using the previously accepted SSH host key; verify any legitimate host-key replacement explicitly.
11. To remove the estate, export needed project files and application data, then type the resource name in **Delete resources**. Luci rechecks ownership before deletion. Confirm completion in the provider console; deletion requests do not themselves prove billing stopped. Closing Luci, stopping the SSH tunnel or returning to local mode does not remove cloud resources.

Use a machine with at least two cores and enough memory for Coolify plus application builds. VM disk, public address, traffic, backups, tax and model usage can add costs. A single host avoids mandatory cluster/database/load-balancer overhead but needs backups for valuable data.

AWS Lightsail instances continue to accrue charges while stopped until they are deleted; see [AWS billing guidance](https://docs.aws.amazon.com/en_en/lightsail/latest/userguide/amazon-lightsail-frequently-asked-questions-faq-billing-and-account-management.html). Azure VMs in the stopped, allocated state still incur compute charges. Deallocation ends compute charges, but disks and other retained resources can still cost money; see [Azure state and billing guidance](https://learn.microsoft.com/en-us/azure/virtual-machines/states-billing). Luci's delete action is the guided cleanup path. Check the provider account afterwards for retained billable resources.

Real account-based acceptance for both providers is required before claiming production rollout has been verified. It must include runtime installation, activation, terminal traffic, interactive previews, reconnect, public DNS/TLS deployment and cleanup. Local adapter and HTTP/WebSocket fixture tests do not replace those outcomes. Ubuntu CI has verified real Docker shell traffic, private project/transcript mounts, host-user mapping, a committed preview returning HTTP 200, and owned-container stop. The transcript check launched Claude without a key, prompt or model inference; paid agent execution remains a separate acceptance gate. Docker is now available on the development host. Installed whole-Docker and data-platform receipts are recorded in [the full-product checkpoint](FULL-PRODUCT.md); they do not prove live provider provisioning.

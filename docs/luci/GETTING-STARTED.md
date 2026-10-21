# Start using Luci

Luci brings your conversation, project decisions, design, coding work and reviews together. Start with one project and a small, useful change. You can return to Discover, Design, Develop, Deploy or Distribute as the project changes.

## Open the Windows portable app

1. Download the candidate's `luci-windows-amd64.zip`, `SHA256SUMS` and `release-manifest.json` from the [repository's releases](https://github.com/lcfrai/luci/releases). Use the exact candidate named in its testing notes. Private releases require repository access.
2. Compare the ZIP's SHA-256 hash with its entry in `SHA256SUMS`. In PowerShell, run `Get-FileHash -Algorithm SHA256 -LiteralPath .\luci-windows-amd64.zip` from the download folder.
3. Extract the **whole ZIP** into a new folder. Keep its files together, then open `Start-Luci.cmd`. Luci opens in your browser; leave the launch terminal open while you use it.
4. Complete account setup and choose where you want Luci to run.

You do not need an existing Luci or lcfr.ai account. In a fresh workspace, the normal launcher provides the one-time setup handoff for creating the first owner; choose and enter your own credentials in Luci. If the workspace already has accounts, use the local launcher's account flow, an invitation or recovery. Keep existing workspace data intact; a plain local browser address does not grant owner access.

You do not need Go, Node.js or Python to run the portable app. Local project work needs Git, and coding sessions need a running Docker engine. The complete release package includes the coding runtime. Setup verifies and imports it into Docker, so ordinary installation does not compile the coding tools on your computer. Keep the bundled runtime and licence files with the app. Setup identifies missing prerequisites. A model running on your computer is optional: you can instead connect a supported model account or API.

## Choose your environment

**Local** keeps Luci and your projects on this computer, with coding agents in isolated Docker environments. Choose a supported harness and model connection, then create a project, clone a repository or open existing work. Begin by telling Luci what you want to make.

**Bring your own cloud** uses your own infrastructure account. Azure is the first target. The guided setup prepares the commands and required inputs for your selected environment. Review its permissions, resources and cost acknowledgment before you run an installation command. A prepared plan is not a connected environment; connection checks must succeed before cloud work can run. Live Azure validation remains pending until a test environment is supplied. AWS support is deferred.

**Luci Cloud** is the planned hosted service. It is not available yet. Choose Local to use this candidate, or prepare your own Azure environment; selecting the future hosted option does not create an account, subscription or runtime.

## Set up your own Azure workspace

In Luci, choose **Bring your own cloud → Azure**, enter your environment name,
region, reviewed monthly budget and VM concurrency limit, then prepare the setup
plan. Download the installer and use the commands shown only after Luci verifies
its SHA-256 against that exact plan. This is the Develop microVM setup, separate
from adding an ordinary deployment-target VM.

Run those commands in **Linux or WSL2**, with Python 3, Git, OpenSSL, Terraform
and Azure CLI available. Sign in to the intended Azure tenant/subscription with
`az login`; user sign-in can require MFA. The selected account needs permission
to deploy the reviewed resources and assign their scoped roles. Azure CLI login
does not provide a private network route or model-provider access. The normal
guided flow uses packaged source and temporary private Azure builders; local Go,
a source checkout and a local Ubuntu image-build environment are not required.

The plan's commands verify and extract `luci-azure-setup.tar`, create private
directories, and run `init-selection` with your selected `--name`, `--region`,
`--monthly-budget-usd` and `--max-concurrent-vms`. Keep those generated values
and the private selection directory. The guided command is:

```bash
python3 luci-azure-setup/azure-setup/installer.py continue --bundle luci-azure-setup --selection luci-azure-private/selection.json
```

Have an Azure subscription, an existing privately reachable VNet and management
source, an SSH public key, and the required resource/state ownership ready. The
guide selects or creates its separately reviewed private state/Key Vault
foundation, then asks about image builders, capacity, exact resource/cost plans,
task limits and allowed model-provider DNS names. Review each approval before
effects; do not supply model or account secrets in command-line examples or
public project files. Keep generated private credentials and receipts private.

Both the setup computer and the Luci computer need working private access and
DNS to the selected environment, for example through an existing VPN/private
route. The guide does not create that access for you. Check region/zone stock,
both regional and VM-family vCPU quotas, and the selected sizes' nested
virtualization support. The default Dsv5 family supports nested virtualization
and requires Accelerated Networking, but that is not proof that the installed
host and guest work. Public model/service endpoints also need confirmed outbound
connectivity, or appropriately configured private endpoints; the internal
gateway alone is not internet egress. Azure's newer private-subnet defaults make
this explicit. A size listing, successful sign-in or prepared plan is not a
connectivity/capacity test. See Microsoft's [quota guidance](https://learn.microsoft.com/en-us/azure/virtual-machines/quotas),
[Dsv5 specifications](https://learn.microsoft.com/en-us/azure/virtual-machines/sizes/general-purpose/dsv5-series)
and [outbound-access guidance](https://learn.microsoft.com/en-us/azure/virtual-network/ip-services/default-outbound-access).

Budgets, task estimates and deadlines do not cap Azure billing. Review compute,
builders, disks, snapshots, gallery, Blob, Key Vault and networking/egress costs;
retained resources can still charge after compute stops. On interruption, use
the same `continue` command and preserve its private files. `status` with the
same `--bundle` and `--selection` arguments reads local receipts; it does not
query Azure or prove connectivity. Follow the bundled `azure-setup/README.md`
for exact retirement and uncertain-operation recovery.

Finally, import the produced connection files through Luci's protected
connection flow and let its private reachability/authentication checks finish.
Configure a supported harness/model connection before coding. This candidate's
source and controlled-fixture checks do not establish a live Azure deployment,
recovery or billing result; live Azure validation remains pending.

## Build and review your first change

Describe the outcome and answer Luci's useful questions. Refine the requirements and design, then start a coding task with your chosen harness. Review its changes and verification results before accepting them. Publishing uses the project's deployment target and authority policy; a completed task is not automatically an approved deployment.

You can request changes in the conversation and switch between projects while work runs. Resource and scheduling settings limit concurrent workers and reserve capacity for your computer. If capacity is temporarily unavailable, keep working on context and reviews while the task waits; do not disable admission guards to force another worker to start.

## Stop, update and recover

Press **Enter in the portable launch terminal** to stop Luci when finished, then close the terminal. Your project files and Luci state live outside the extracted application folder. Before an update, stop the old launcher and back up both. Extract the new package into a separate folder, then launch it; do not overwrite a running installation.

If another service occupies the default port, run `.\Start-Luci.ps1 -Port 8731` from the extracted package folder in PowerShell, using a free port. A package-identity warning means a different package is already using that workspace: close its launcher before trying the new package. It does not mean your projects need resetting.

For build details, package integrity, licences and the limits of each candidate's testing, see the [release guide](RELEASE.md) and the notes attached to your exact download.

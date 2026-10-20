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

## Build and review your first change

Describe the outcome and answer Luci's useful questions. Refine the requirements and design, then start a coding task with your chosen harness. Review its changes and verification results before accepting them. Publishing uses the project's deployment target and authority policy; a completed task is not automatically an approved deployment.

You can request changes in the conversation and switch between projects while work runs. Resource and scheduling settings limit concurrent workers and reserve capacity for your computer. If capacity is temporarily unavailable, keep working on context and reviews while the task waits; do not disable admission guards to force another worker to start.

## Stop, update and recover

Press **Enter in the portable launch terminal** to stop Luci when finished, then close the terminal. Your project files and Luci state live outside the extracted application folder. Before an update, stop the old launcher and back up both. Extract the new package into a separate folder, then launch it; do not overwrite a running installation.

If another service occupies the default port, run `.\Start-Luci.ps1 -Port 8731` from the extracted package folder in PowerShell, using a free port. A package-identity warning means a different package is already using that workspace: close its launcher before trying the new package. It does not mean your projects need resetting.

For build details, package integrity, licences and the limits of each candidate's testing, see the [release guide](RELEASE.md) and the notes attached to your exact download.

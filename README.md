# Luci

Luci is lcfr.ai's open-source workspace for defining, designing, developing and deploying software. Start on your own machine, then use your own AWS or Azure account when you need a server. Your projects stay as ordinary files and Git repositories.

## Use Luci

Install the Windows desktop package or extract a portable package and use its launcher. First setup creates your Super Admin account from the private bootstrap code. In Settings, check Git and Docker, install the bundled runtime, choose your Git identity and connect Claude Code, Codex or an explicit OpenCode model. A local OpenAI-compatible model can avoid inference charges when your hardware and model suit the task.

Create or clone a project. Define its purpose and acceptance criteria, design the screens, and save a task plan. Develop passes that saved context into an agent working in the project. Inspect its changes and run checks against the recorded source; task completion is separate from passing checks. Commit and publish through the same workspace, start a Docker preview, set private runtime values such as a database URL in Deploy, then deploy a reachable Git commit through your connected cloud target. The app retains release history, supports stopping and rebuilding a historical release, and exports/restores project backups.

The Admin screen manages users, workspace processes and audit events on this machine or the selected cloud backing, using that control plane's account and permissions. Super Admin controls cloud/host execution policy, concurrent sessions/checks and CPU/memory limits. Each additional user has separate project, state and provider profiles. Host commands are disabled by default; Docker supplies the supported code execution boundary. Your desktop can select a verified AWS/Azure backing without copying local credentials or silently falling back when the server is unavailable.

See the [operator runbook](docs/RUNBOOK.md), [provider setup](docs/luci/PROVIDERS.md) and [release guide](docs/luci/RELEASE.md) for prerequisites and verified limits.

## Structure

- `desktop/` — React/Tauri interface, Excalidraw and terminal cockpit
- `control-plane/` — local setup, project journey and session API
- `sandboxd/` — isolated agent runtime
- `docs/luci/` — current plan, API contract and reuse decisions

## Run from source

Install Git, Go 1.26+, Node 22+ and Docker. Docker is needed for agent sessions and container previews; Define and Design work without it. Run `scripts/dev.ps1` on Windows or `scripts/dev.sh` on Linux/macOS, then follow setup in the application. Native desktop builds also need Rust and the [Tauri prerequisites](https://v2.tauri.app/start/prerequisites/).

## Building and testing

```bash
go build ./sandboxd/...
go build ./control-plane/...
```

## Testing

```bash
go test ./sandboxd/...
go test ./control-plane/...
```

The desktop uses `npm ci`, `npm run typecheck`, `npm test` and `npm run build` from `desktop/`.

## Cloud setup

Luci prepares a single-machine deployment using AWS Lightsail or an Azure VM, shows the resources and cost information, and guides you through sign-in, provisioning, DNS and connection. Cloud resources belong to you and continue billing until you remove them. Preview the plan and accept its costs before creation. Luci keeps its control API on loopback; remote administration uses SSH.

Local use has no Luci licence or platform subscription fee, and Define and Design need no cloud resources or model requests. Model subscriptions/API usage and any third-party services have their own costs. Keep development local when it meets your needs. If you need cloud execution or public hosting, start with the smallest viable single host in your chosen region, allowing memory for the deployment service, builds and running apps. Add capacity only when the workload requires it. A smaller advertised VM price is not useful if builds cannot finish.

Review the provider's current quote and currency, plus disk, public addresses, traffic, backups, domain, tax and model costs. An incomplete quote remains unverified; the budget field does not cap the provider's bill. See [cloud setup and cleanup](docs/luci/CLOUD.md) before creating resources.

Coolify supplies the optional application deployment service. Its Git deployment needs a repository the server can access. A local Dockerfile alone does not upload a project's files.

## Contributing

Read [the current plan](docs/luci/PLAN.md), [API contract](docs/luci/API.md), and [reuse decisions](docs/luci/REUSE.md). `AGENTS.md` describes contributor boundaries. Historical Panacea planning documents are retained as provenance and superseded by the Luci plan.

Luci's own code uses the [MIT licence](LICENSE). Third-party components keep their original licences and notices, including dependencies licensed under Apache-2.0. Publication still requires confirming rights to retained code and history; current package and distribution status is described in [the release guide](docs/luci/RELEASE.md).

# Luci

Luci is lcfr.ai's open-source workspace for defining, designing, developing and deploying software. Start on your own machine, then use your own AWS or Azure account when you need a server. Your projects stay as ordinary files and Git repositories.

## Use Luci

Install the Windows desktop package or extract a portable package and use its launcher. The [Docker distribution](distribution/docker/README.md) runs the entire app and its private execution engine in containers; its launchers need Docker, without host Go, Node.js or Python. Create your local account; the first account administers the installation. If an account already exists, choose **Create account** for a separate account or **Forgot sign-in details** to recover the original owner and saved work.

The onboarding portal guides you through **On this computer**, **AWS**, **Azure** or **Connect to an existing workspace**, checks the actual prerequisites and prepares the agent runtime. It distinguishes installed tools from a running engine and a connected provider account. Choose your Git identity and connect Claude Code, Codex or an explicit OpenCode model. A local OpenAI-compatible model can avoid inference charges when your hardware and model suit the task.

Create or clone a project from Projects home. Define its purpose and acceptance criteria, design the screens, and save a task plan. Develop passes that saved context into an agent working in the project. Inspect its changes and run checks against the recorded source; task completion is separate from passing checks. Commit and publish through the same workspace, start a Docker preview, set private runtime values such as a database URL in Deploy, then deploy a reachable Git commit through your connected cloud target. The app retains release history, supports stopping and rebuilding a historical release, and exports/restores project backups. Optional Supabase data services share Luci's project navigation and appearance rather than requiring a separate administration portal.

The project overview carries the agreed idea, current stage, active build and next decision across these views. Amendments remain drafts until reviewed; existing runs retain their original inputs. Team workspaces let authorized contributors build that context together, with separate read, edit, execution and secret permissions. Git commits are bound to the displayed review: a teammate's later changes require a fresh review, while your commit message stays intact.

For Codex subscription access, complete official device sign-in from Settings, then **Save Codex sign-in**. New implementation sessions use that connection, with separate private histories and no fallback to API billing. API credits remain a separate option. A saved sign-in is not proof of a successful model request.

Static HTML/CSS/JavaScript sites can use **GitHub Pages** in Deploy without buying a server or domain: commit the site under `docs/`, publish the reviewed commit to a public repository and acknowledge public hosting. Luci verifies the deployed commit and actual HTTPS content. Later pushes to that default branch republish `docs/`. Pages does not run server code, databases or private runtime configuration; use the connected cloud target for those applications.

The Admin screen manages users, workspace processes and audit events on this machine or the selected cloud backing, using that control plane's account and permissions. Super Admin controls cloud/host execution policy, concurrent sessions/checks and CPU/memory limits. Each additional user has separate project, state and provider profiles. Host commands are disabled by default; Docker supplies the supported code execution boundary. Your desktop can select a verified AWS/Azure backing without copying local credentials or silently falling back when the server is unavailable.

See the [operator runbook](docs/RUNBOOK.md), [provider setup](docs/luci/PROVIDERS.md) and [release guide](docs/luci/RELEASE.md) for prerequisites and verified limits.

The [UAT guide](docs/UAT.md) walks through onboarding and the complete project journey. [DESIGN.md](DESIGN.md) describes the implemented visual and interaction system. Current incomplete acceptance gates are recorded in the [full product checkpoint](docs/luci/FULL-PRODUCT.md).

## Structure

The latest engineering handoff and continuation instructions are in
[the production checkpoint](docs/luci/HANDOFF-2026-10-06-PRODUCTION.md).
[The product-quality goal](docs/luci/GOAL.md) remains open: managed-team billing,
live model quality, public hosting and supported native distribution need their
own acceptance evidence before a production release.

- `desktop/` — React/Tauri interface, Excalidraw and terminal cockpit
- `control-plane/` — local setup, project journey and session API
- `sandboxd/` — isolated agent runtime
- `docs/luci/` — current plan, API contract and reuse decisions

## Run from source

Install Git, Go 1.26+, Node 24.19.0 and Docker. Docker is needed for agent sessions and container previews; Define and Design work without it. Run `scripts/dev.ps1` on Windows or `scripts/dev.sh` on Linux/macOS, then follow setup in the application. Native desktop builds also need Rust and the [Tauri prerequisites](https://v2.tauri.app/start/prerequisites/).

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

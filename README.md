# Luci

Luci is lcfr.ai's open-source workspace for defining, designing, developing and deploying software. Start on your own machine, then use your own AWS or Azure account when you need a server. Your projects stay as ordinary files and Git repositories.

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

Coolify supplies the optional application deployment service. Its Git deployment needs a repository the server can access. A local Dockerfile alone does not upload a project's files.

## Contributing

Read [the current plan](docs/luci/PLAN.md), [API contract](docs/luci/API.md), and [reuse decisions](docs/luci/REUSE.md). `AGENTS.md` describes contributor boundaries. Historical Panacea planning documents are retained as provenance and superseded by the Luci plan.

New Luci code is intended for Apache-2.0 distribution. Third-party components keep their original licences. Publication requires confirming the rights to retained code and history; the old private repository is not a public download yet.

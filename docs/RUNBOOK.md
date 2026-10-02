# Luci operator runbook

This is the current local and cloud-workspace runbook. The [Phase 1 runbook](verification/phase-1-operator-runbook.md) is historical evidence for the earlier cluster experiment; its Azure/Kata prerequisites are not prerequisites for Luci's default experience.

## Start and check the application

For packaged candidates, follow [RELEASE.md](luci/RELEASE.md), extract the complete archive and use its launcher, or install the Windows desktop candidate. From source, install the prerequisites in the project README and run `scripts/dev.ps1` or `scripts/dev.sh` from the repository root.

First startup creates an owner-only bootstrap file in `StateDir/control-plane/bootstrap-token`. The packaged launcher or native application can populate that first-run code; otherwise enter it in the application. Choose the initial Super Admin username and password. Subsequent starts use ordinary sign-in. API automation obtains a bearer token from `/v1/auth/login`; do not put tokens in URLs or commit them. `GET /v1/control-plane/status` is the public readiness endpoint; all workspace APIs require sign-in.

The control service defaults to `127.0.0.1:8730`. Open the application and choose a project folder. Setup checks Git, Docker and the sandbox image separately. Define and Design remain available while Docker is unavailable. A running Docker engine and installed agent runtime are required for real sessions and container previews. Configure a real Git identity and the person's chosen model access in Settings.

`GET /v1/setup/status` reports the selected workspace and known setup state. **Check readiness** explicitly probes dependencies; a previously successful runtime installation is not proof that Docker is running now. Health at `/v1/healthz` describes the service; it is not proof that an agent, application or cloud deployment works.

## Configuration and state

| Setting | Purpose |
| --- | --- |
| `LUCI_ADDR` | Loopback control listener, default `127.0.0.1:8730` |
| `LUCI_BACKEND` | `docker` is the supported default; see the retained Kata limitation below |
| `LUCI_STATE_DIR` | Overrides the base directory for private state and runtime subdirectories |
| `LUCI_PROJECTS_DIR` | Initial project root; Settings persists the configured root |
| `LUCI_WEB_DIR` | Built web assets, normally the packaged `web/` directory |
| `LUCI_RUNTIME_DIR` | Packaged `runtime/` sources used to build the Docker agent image |
| `LUCI_CLOUD_BUNDLE` | Matching Linux cloud archive, normally `cloud/luci-cloud-linux-amd64.tar.gz` beside the executable |
| `LUCI_SANDBOX_IMAGE` | Agent image, default `luci/sandbox:dev` |
| `LUCI_DOCKER_HOST` | Optional Docker endpoint override |
| `LUCI_GIT_NAME`, `LUCI_GIT_EMAIL` | Optional explicit identity overrides; never substitute invented authors |
| `LUCI_CA_BUNDLE` | Optional trusted CA file for networks that require one; mounted read-only at `/etc/luci/ca-bundle.pem` and passed with `NODE_EXTRA_CA_CERTS` |
| `LUCI_SESSION_CPU_MILLIS`, `LUCI_SESSION_MEMORY_MIB` | Resource limits for agent containers |
| `LUCI_LOG_LEVEL` | `debug`, `info`, `warn` or `error` |

Keep the control service on loopback. Cloud workspaces use SSH forwarding rather than a publicly exposed control port. Custom `LUCI_ALLOWED_ORIGINS` replaces the default origin allowlist; configure it only when changing the actual client origin. Do not disable origin protection or certificate verification to work around setup problems.

Without `LUCI_STATE_DIR`, state uses `%LOCALAPPDATA%\Luci\state` on Windows, `$XDG_STATE_HOME/luci` when set, or `~/.local/state/luci`. Services use subdirectories beneath that base. Private state must remain outside the projects root. It includes credentials, accepted host keys, connection selections and release records; back it up securely with project files before upgrades or moving machines. Do not commit that directory or include it in a support bundle. A package upgrade does not back it up.

## Projects and previews

Create a project or clone an accessible repository. Save a versioned brief, design and task plan, then use Develop to send those project artifacts into the chosen agent. Run the project checks and inspect their commands, source digest, exit codes and output before accepting changes. A source change during checks invalidates the outcome. Files/Git and publication use that same project; public repository creation is an explicit selection, with private as the default. A project backup includes Git history, brief/design/plan and regular source files while omitting known credential paths and runtime state. Restore always creates a new project and refuses to overwrite an existing folder. Before creating a preview, commit all changes, including the current `LUCI.md` and Dockerfile. Preview builds use the committed snapshot and record that commit and brief revision. A build failure or unsuccessful health response must remain a failed release.

Use **Stop preview** to remove the recorded container after its ownership labels have been checked. Stopping preserves release history. An unresponsive URL is not evidence that Docker removed the container; inspect Docker if the stop reports an error. The optional cloud-workspace bridge gives each remote preview a separate local browser origin and carries HTTP and WebSocket traffic through SSH. It does not make the preview public.

## Cloud workspace and public deployment

Follow [CLOUD.md](luci/CLOUD.md). Provider login and resource creation run on the desktop through the person's AWS or Azure CLI. Review the planned resources and quote, accept ongoing costs, verify the server's SSH host fingerprint, and connect.

To develop remotely, explicitly install the bundled matching Luci runtime and sign in or bootstrap its separate Luci backing account, and activate that workspace after its readiness check succeeds. The bootstrap code is read only from the installer-owned remote state path over the accepted SSH connection. Remote bearer credentials remain private on the desktop and are replaced when forwarding; the local account token is never sent to the backing. Projects, model settings, Git operations, sessions and runtime installation then apply to the selected server. A switch does not copy local files or credentials. Returning to local mode preserves remote data and does not stop cloud billing.

Public release uses a connected Coolify server, a reachable Git repository, an exact commit and a domain pointing at the server. Readiness requires both the expected provider deployment and a real HTTPS health response. Complete Coolify first-account/token setup explicitly; a generated infrastructure template is not a deployed application.

On a remotely installed workspace, the service is `luci.service`; use the selected machine's service manager to inspect its status and bounded recent logs. The current runtime is at `/opt/luci/current`. Treat host logs and local private state as sensitive when collecting diagnostics. Preserve the accepted SSH host key; do not bypass strict host checking after an unexpected key change.

## Failure recovery and verification limits

- If Docker is missing or stopped, start or repair the user's Docker installation through its normal supported controls, then repeat the readiness check. A test fixture is not a substitute for a real engine.
- If the sandbox image is missing, use **Install runtime** with Docker working. A failed build reports failure; confirm the recorded image exists before attempting a session.
- If an account login expires, start a fresh login. Provider CLI credentials remain in the provider CLI cache. A saved model API key is separate from a subscription login.
- If SSH disconnects, reconnect the selected workspace. The app must show the remote failure instead of falling back to similarly named local projects.
- If provisioning or deployment is uncertain, refresh the existing recorded operation and inspect the provider before retrying. Do not create duplicate resources based on a timeout.
- Before deleting cloud resources, export needed project/application data and confirm the owned resource name. Check the provider console for actual removal; closing Luci or a tunnel does not end billing.

The retained `kata` backend is an earlier in-cluster integration and currently refuses the host-mounted worktree specification. Setting `LUCI_BACKEND=kata` does not make local projects cloud-ready. The supported cloud-workspace path runs the Docker backend on an explicitly selected machine; no Kubernetes cluster is required.

The 0.4 build has automated management, project and provider tests, a compiled multi-user service eval and a real browser journey. These outcome results cover local credentials, isolated users, revocation, checks, Git commits and backup/restore. New packages and the Docker image must still pass their exact-commit CI checks. Real provider account/model quality, clean-host installer behavior, live AWS/Azure and domain/TLS journeys require the corresponding external environment. See [the release gates](luci/RELEASE.md); historical test results do not close those gates for a new version.

## Manage the backing

Admins can create member accounts, inspect bounded audit events and stop an owned member workspace process. An enabled member's next request restarts its process with durable state; disable its account to prevent restart. Role, password and disable changes revoke sessions, including active terminal sockets. The initial owner's inline workspace ends with the service.

Super Admin sets allowed agent harnesses, concurrency and per-session/check resources. Effective resources are limited by the operator's configured ceiling. These are individual process limits; size the host for the actual combined workload. Cloud budget policy is an admission limit for an operation, not a billing cap.

Private Git deployment needs a read-only GitHub deploy key. Use the integrated deployment-key action with a repository administrator account or an existing Coolify private-key UUID. If a response is lost, refresh the recorded operation before retrying. Cloud Stop waits for provider confirmation and cannot stop a newer release. Rebuilding a historical release restores code at its original commit; it does not reverse database migrations or restore application data.

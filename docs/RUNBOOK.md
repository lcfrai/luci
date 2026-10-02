# Luci operator runbook

This is the current local and cloud-workspace runbook. The [Phase 1 runbook](verification/phase-1-operator-runbook.md) is historical evidence for the earlier cluster experiment; its Azure/Kata prerequisites are not prerequisites for Luci's default experience.

## Start and check the application

For packaged candidates, follow [RELEASE.md](luci/RELEASE.md), extract the complete archive and use its launcher, or install the Windows desktop candidate. From source, install the prerequisites in the project README and run `scripts/dev.ps1` or `scripts/dev.sh` from the repository root.

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

Create a project or clone an accessible repository. Save a versioned brief and design, then use Develop and Files/Git to work in that same project. Before creating a preview, commit all changes, including the current `LUCI.md` and Dockerfile. Preview builds use the committed snapshot and record that commit and brief revision. A build failure or unsuccessful health response must remain a failed release.

Use **Stop preview** to remove the recorded container after its ownership labels have been checked. Stopping preserves release history. An unresponsive URL is not evidence that Docker removed the container; inspect Docker if the stop reports an error. The optional cloud-workspace bridge gives each remote preview a separate local browser origin and carries HTTP and WebSocket traffic through SSH. It does not make the preview public.

## Cloud workspace and public deployment

Follow [CLOUD.md](luci/CLOUD.md). Provider login and resource creation run on the desktop through the person's AWS or Azure CLI. Review the planned resources and quote, accept ongoing costs, verify the server's SSH host fingerprint, and connect.

To develop remotely, explicitly install the bundled matching Luci runtime and activate that workspace after its readiness check succeeds. Projects, model settings, Git operations, sessions and runtime installation then apply to the selected server. A switch does not copy local files or credentials. Returning to local mode preserves remote data and does not stop cloud billing.

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

The current build has local automated tests and candidate packaging. A real Docker agent/preview journey, clean-host installer journey, both cloud-provider journeys, real DNS/TLS and cleanup must still be evidenced at the release commit. See [the release gates](luci/RELEASE.md); historical test results do not close those gates for a new version.

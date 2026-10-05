# Luci in Docker

This distribution runs the actual Luci interface and connection service in a
container, with a private Docker engine for agents, checks, applications and data
services. Your projects and account data live in named volumes. It does not mount
your host Docker socket, GitHub account, cloud account or Codex credentials.

Install and open Docker Desktop with Linux containers, or start a Linux Docker
engine with the official Compose plugin. A 64-bit AMD/Intel host is supported;
Apple Silicon uses Docker's amd64 emulation. Start with at least 4 GB available
for lightweight work. The full data stack needs additional memory and CPU alongside
the coding agent and application; Luci checks those capacities before starting it.

On Windows, double-click `Start-Luci.cmd` or run `Start-Luci.ps1`. On macOS/Linux,
run `./start-luci.sh`. The first
launch builds the controller image from the included release bundle and downloads
its pinned tools. Go, Node.js and Python are not required on your host. The launcher
opens the first-account page using a private token without printing that token.
Choose your own username and password, choose this computer, AWS, Azure or an
existing workspace, and connect your coding agent through its official sign-in.
There is no default account or automatic paid resource creation.

The app opens at <http://127.0.0.1:8730/>. Verified application previews use the
separate loopback gateway on port 8731. Both ports remain local to your computer.
On Windows, `./Start-Luci.ps1 -Port 8740 -PreviewPort 8741` selects different ports.
On macOS/Linux, `LUCI_PORT=8740 LUCI_PREVIEW_PORT=8741 ./start-luci.sh` does the same.
Keep the two ports different. `-NoBrowser` or `--no-browser` starts the services
without opening a browser; run the normal launcher to obtain the first-account link.

## Operating the installation

Run these commands from this distribution directory:

```sh
docker compose ps
docker compose logs --tail 100 engine controller
docker compose restart controller
docker compose stop
docker compose up --detach --wait
```

Stop and restart retain the workspace, inner images, running application definitions
and databases. `docker compose down` removes the service containers while preserving
the named volumes. Do not add `--volumes` unless you intend to delete the installation's
accounts, projects, databases and engine data. Back up project and application data
through Luci before removing an installation. Different Compose project names create
separate installations; use the same name when reopening yours.

The engine container is privileged because it runs Docker inside Docker. The
controller runs as non-root user 1001:1001, is not privileged, has a read-only root filesystem and connects only
to the private engine socket. The engine does not expose a TCP Docker API. Its
workspace volume uses the same `/luci-workspace` path as the controller so nested
agent and application mounts refer to the correct private files.

Before starting its daemon, the private engine initializes a new empty workspace
and any missing `home`, `state`, `projects` and `tmp` directories for user 1001:1001,
with mode 0700. Its socket directory is root:1001 with mode 0750; the daemon socket
is available only to that group. The controller and coding agent use the same
private workspace identity. No workspace directory is made writable by everyone.

Existing workspace directories and their contents are never recursively reowned,
rewritten or deleted at startup. A retained populated workspace must already have
the expected private ownership and permissions. Linked or incompatible retained
paths stop startup before initialization, preserving the files. Export projects
and application data from the original installation and restore them into a fresh
installation, or have an administrator inspect the named volume ownership; do not
delete a retained volume to clear an ownership error.

Git, Docker CLI/Compose, GitHub CLI, AWS CLI and Azure CLI are included. Their
presence is distinct from signing in, connecting a server or approving its costs.
The onboarding dependency panel describes the computer running the connection
service; inside this package it is a Linux container. Windows installation controls
do not install software on the host from this container.

## Maintainer build and provenance

Build the normal clean Linux portable release first, then prepare a fresh Docker
distribution from its manifest-verified archive:

```sh
python scripts/prepare-docker-release.py 0.4.13 --archive
cd artifacts/docker/0.4.13
docker compose build engine controller
docker compose up --detach --wait
```

The preparation helper accepts only the matching release's Linux bundle, refuses
links and unexpected top-level/private files, verifies the archive hash and every
extracted file, and emits a file-hash manifest. The Docker build context contains
only the Dockerfile, bounded startup scripts and that attested bundle. Do not
build this Dockerfile with the repository root as the context.
The optional archive is written beside the prepared directory, with a SHA-256
receipt printed for release attachment. Refusing existing output protects earlier
release evidence; `--output artifacts/docker/<fresh-name>` selects a fresh folder.

Images are pinned by digest. GitHub, AWS and Azure CLI assets are pinned by version
and SHA-256. The official inputs are [Docker's Debian installation documentation](https://docs.docker.com/engine/install/debian/),
[Docker Compose plugin documentation](https://docs.docker.com/compose/install/linux/),
[GitHub CLI releases](https://github.com/cli/cli/releases),
[AWS CLI installation documentation](https://docs.aws.amazon.com/cli/latest/userguide/getting-started-install.html)
and [Azure CLI's Linux package instructions](https://learn.microsoft.com/en-us/cli/azure/install-azure-cli-linux).
Update pins deliberately and preserve upstream licences. Luci's own MIT licence,
notices and portable dependency licences are included under `/opt/luci`; installed
CLI packages retain their own upstream licence files.

Packaging and health checks establish process readiness. Full release acceptance
still requires a real Codex subscription journey, an implemented and verified
application, and a working deployed site with evidence from all four phases.

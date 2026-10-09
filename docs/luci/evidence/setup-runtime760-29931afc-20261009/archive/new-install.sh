#!/usr/bin/env bash
set -euo pipefail
umask 077
# Full installation is the default. These narrow modes avoid rebuilding the app
# while another task is editing or compiling it.
case "${1:---full}" in
  --full) LUCI_SETUP_MODE=full ;;
  --tools-only) LUCI_SETUP_MODE=tools ;;
  --restore-runtime) LUCI_SETUP_MODE=runtime ;;
  *) printf '%s\n' 'Usage: install.sh [--full|--tools-only|--restore-runtime]' >&2; exit 2 ;;
esac
cd /workspace/luci
test -f AGENTS.md
test -f desktop/package-lock.json
test "$(uname -sm)" = 'Linux x86_64'
node -e 'const [a,b,c]=process.versions.node.split(".").map(Number); if (!((a===22 && (b>22 || (b===22 && c>=2))) || (a===24 && b>=15) || a>24)) throw Error("Luci requires Node >=22.22.2 (22 line) or >=24.15")'
command -v python >/dev/null
command -v git >/dev/null
command -v chromium >/dev/null
mkdir -p /workspace/.luci-tools /workspace/.cache
# The toolchain and verified archives are retained regular files in /workspace.
# Temporary caches can disappear on restore and are recreated by activation.
LUCI_GO_DIR=/workspace/.luci-tools/go1.26.3
LUCI_GO_ARCHIVE=/workspace/.luci-tools/go1.26.3.linux-amd64.tar.gz
LUCI_GO_SHA256=2b2cfc7148493da5e73981bffbf3353af381d5f93e789c82c79aff64962eb556
if [[ "$("$LUCI_GO_DIR/bin/go" version 2>/dev/null || true)" != 'go version go1.26.3 linux/amd64' ]]; then
  if [[ ! -f "$LUCI_GO_ARCHIVE" ]] || ! printf '%s  %s\n' "$LUCI_GO_SHA256" "$LUCI_GO_ARCHIVE" | sha256sum --check --status; then
    LUCI_GO_DOWNLOAD=$(mktemp /workspace/.luci-tools/.go-download.XXXXXX)
    trap 'rm -f "${LUCI_GO_DOWNLOAD:-}"' EXIT
    curl --fail --location --silent --show-error --retry 2 --proto '=https' --tlsv1.2 \
      https://dl.google.com/go/go1.26.3.linux-amd64.tar.gz -o "$LUCI_GO_DOWNLOAD"
    printf '%s  %s\n' "$LUCI_GO_SHA256" "$LUCI_GO_DOWNLOAD" | sha256sum --check
    # Replace a stale or dangling managed archive link without following it.
    mv -T "$LUCI_GO_DOWNLOAD" "$LUCI_GO_ARCHIVE"
    trap - EXIT
  fi
  printf '%s  %s\n' "$LUCI_GO_SHA256" "$LUCI_GO_ARCHIVE" | sha256sum --check
  LUCI_GO_STAGE=$(mktemp -d /workspace/.luci-tools/.go-install.XXXXXX)
  tar --no-same-owner -xzf "$LUCI_GO_ARCHIVE" -C "$LUCI_GO_STAGE" --strip-components=1
  test "$("$LUCI_GO_STAGE/bin/go" version)" = 'go version go1.26.3 linux/amd64'
  # Keep a mismatched managed installation for diagnosis instead of deleting it.
  if [[ -e "$LUCI_GO_DIR" || -L "$LUCI_GO_DIR" ]]; then
    mv "$LUCI_GO_DIR" "$LUCI_GO_STAGE.previous"
  fi
  mv "$LUCI_GO_STAGE" "$LUCI_GO_DIR"
fi
# Use the repository's module graph in a local workspace file. Go may append
# workspace checksums even in readonly module mode; keep those outside Git.
sed 's|\./control-plane|/workspace/luci/control-plane|; s|\./sandboxd|/workspace/luci/sandboxd|' go.work > /workspace/.luci-tools/go.work
cp go.work.sum /workspace/.luci-tools/go.work.sum
mkdir -p /workspace/.luci-tools/bin
cat > /workspace/.luci-tools/bin/docker <<'LUCI_DOCKER_ADAPTER'
#!/usr/bin/env python3
"""Cloud build adapter: inherit the provided proxy and its public CA safely.

Only `docker build` is adapted. Runtime containers retain all original arguments.
The tracked sandbox Dockerfile is read afresh, then a temporary external copy
mounts the existing CA only for npm's build step. No credentials are persisted.
"""
import os
import hashlib
from pathlib import Path
import socket
import sys
from urllib.parse import urlparse

REAL_DOCKER = "/usr/local/bin/docker"
args = sys.argv[1:]
if not args or args[0] != "build":
    os.execv(REAL_DOCKER, [REAL_DOCKER, *args])

environment = os.environ.copy()
extra = []
proxy_hosts = set()
for upper, lower in [("HTTP_PROXY", "http_proxy"), ("HTTPS_PROXY", "https_proxy"), ("NO_PROXY", "no_proxy")]:
    value = environment.get(upper) or environment.get(lower)
    if value:
        environment[upper] = environment[lower] = value
        extra += ["--build-arg", upper, "--build-arg", lower]
        if upper != "NO_PROXY":
            hostname = urlparse(value).hostname
            if hostname:
                proxy_hosts.add(hostname)
if proxy_hosts:
    if not any(arg == "--network" or arg.startswith("--network=") for arg in args):
        extra += ["--network", "host"]
    for hostname in sorted(proxy_hosts):
        address = socket.getaddrinfo(hostname, None, socket.AF_INET, socket.SOCK_STREAM)[0][4][0]
        extra += ["--add-host", f"{hostname}:{address}"]

file_index = None
inline_file = False
dockerfile = None
for index, arg in enumerate(args):
    if arg in ("-f", "--file") and index + 1 < len(args):
        file_index, dockerfile, inline_file = index + 1, Path(args[index + 1]), False
    elif arg.startswith("--file="):
        file_index, dockerfile, inline_file = index, Path(arg.split("=", 1)[1]), True

if dockerfile and dockerfile.is_file() and dockerfile.as_posix().endswith("images/sandbox/Dockerfile"):
    source = dockerfile.read_text()
    marker = "RUN npm install -g "
    cert = environment.get("NODE_EXTRA_CA_CERTS") or environment.get("CODEX_PROXY_CERT")
    if cert and Path(cert).is_file() and marker in source:
        replacement = ("RUN --mount=type=secret,id=luci_proxy_ca,target=/run/secrets/luci_proxy_ca,required=true "
                       "NODE_EXTRA_CA_CERTS=/run/secrets/luci_proxy_ca npm install -g ")
        configured = source.replace(marker, replacement).replace(
            "chmod +x /usr/local/bin/entrypoint.sh", "chmod 0755 /usr/local/bin/entrypoint.sh")
        directory = Path("/workspace/.luci-tools/dockerfiles")
        directory.mkdir(exist_ok=True)
        generated = directory / (hashlib.sha256(configured.encode()).hexdigest() + ".Dockerfile")
        if not generated.exists():
            generated.write_text(configured)
        elif generated.read_text() != configured:
            raise SystemExit("Generated Dockerfile checksum mismatch")
        args[file_index] = ("--file=" if inline_file else "") + str(generated)
        extra += ["--secret", f"id=luci_proxy_ca,src={cert}"]
# Preserve Docker's signal and exit-status behavior, including build cancellation.
os.execve(REAL_DOCKER, [REAL_DOCKER, "build", *extra, *args[1:]], environment)
LUCI_DOCKER_ADAPTER
chmod 700 /workspace/.luci-tools/bin/docker
cat > /workspace/.luci-tools/activate.sh <<'EOF'
# Source this in every new shell. /workspace contains retained dependencies;
# /tmp contains disposable caches and is recreated after each restore.
export PATH="/workspace/.luci-tools/bin:/workspace/.luci-tools/go1.26.3/bin:$PATH"
export GOCACHE=/tmp/luci-go-build-cache
export GOMODCACHE=/tmp/luci-retained-tools/go-mod
export GOPATH=/tmp/luci-gopath
export GOMAXPROCS=4
# Short external temporary paths preserve native Unix socket limits in Go fixtures.
export TMPDIR=/workspace/.t
export GOTMPDIR=/workspace/.t
export GOFLAGS=-mod=readonly
export GOTOOLCHAIN=local
export GOWORK=/workspace/.luci-tools/go.work
export npm_config_cache=/tmp/luci-npm-cache-recovery
export PLAYWRIGHT_BROWSERS_PATH=/tmp/luci-playwright-cache
export DOCKER_CONFIG=/workspace/.luci-tools/docker
(umask 077; mkdir -p "$TMPDIR" "$GOTMPDIR" "$GOCACHE" "$GOMODCACHE" "$GOPATH" "$npm_config_cache" "$PLAYWRIGHT_BROWSERS_PATH" "$DOCKER_CONFIG")
# A historical generated development output may have been relocated for space.
# Its dangling, known-owned link must not prevent scripts/dev.sh recreating it.
if [ -L /workspace/luci/artifacts/dev ] && [ ! -e /workspace/luci/artifacts/dev ]; then
  case "$(readlink /workspace/luci/artifacts/dev)" in
    /tmp/luci-retained-artifacts/dev-binary) rm /workspace/luci/artifacts/dev ;;
  esac
fi
if [ ! -x /workspace/.luci-tools/go1.26.3/bin/go ]; then
  printf '%s\n' 'Pinned Go is missing. Run bash /workspace/.luci-tools/install.sh --tools-only.' >&2
  return 1 2>/dev/null || exit 1
fi
EOF
source /workspace/.luci-tools/activate.sh
mkdir -p "$DOCKER_CONFIG" artifacts/onboarding
cat > /workspace/.luci-tools/use-system-chromium.mjs <<'EOF'
// Use the image's Chromium with Playwright's supported executablePath option.
// Its version can differ from Playwright's pinned browser. Tests stay intact.
import { chromium } from '/workspace/luci/desktop/node_modules/playwright/index.mjs';
const launch = chromium.launch;
chromium.launch = function (options = {}) {
  return launch.call(this, { executablePath: '/usr/bin/chromium', ...options });
};
EOF
go version
if [[ "$LUCI_SETUP_MODE" == tools ]]; then
  printf '%s\n' 'Pinned tools, local workspace and disposable cache directories are ready; application builds and Docker restore were not run.'
  exit 0
fi
if [[ "$LUCI_SETUP_MODE" == full ]]; then
go build ./control-plane/... ./sandboxd/...
CGO_ENABLED=0 go build -trimpath -o artifacts/onboarding/luci ./control-plane/cmd/api
cd /workspace/luci/desktop
npm ci --no-fund --no-audit
npm run build
NODE_OPTIONS="${NODE_OPTIONS:+$NODE_OPTIONS }--import=/workspace/.luci-tools/use-system-chromium.mjs" node --input-type=module <<'JS'
import { chromium } from 'playwright';
const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  await page.setContent('<title>Luci browser ready</title><p>Ready</p>');
  if (await page.title() !== 'Luci browser ready') throw Error('Browser readiness failed');
  console.log(`Playwright browser ready: ${browser.version()}`);
} finally { await browser.close(); }
JS
fi
cd /workspace/luci
# The privileged daemon is supplied by the environment, not launched by this
# unprivileged helper. A failed probe is an environment service prerequisite.
if ! docker info >/dev/null 2>&1; then
  printf '%s\n' 'Docker is unavailable. Restore/start the environment-provided daemon and verify its socket before retrying --restore-runtime. Do not change daemon storage or disable verification.' >&2
  exit 1
fi
LUCI_RUNTIME_ARCHIVE=/workspace/.luci-tools/luci-sandbox-dev.tar.gz
LUCI_RUNTIME_SHA256=637d1d02c701be5f4327bb76dcb58335e1665b27945e3f1cb08bb825c5fd96fd
LUCI_RUNTIME_IMAGE=sha256:76043c5d0f30207c05343e1dbc0f450658eb52726590c8c47c714c67371acec8
# A /tmp symlink is not a retained recovery artifact. Never depend on it when
# preparing a snapshot; the verified archive must live in /workspace itself.
if [[ ! -f "$LUCI_RUNTIME_ARCHIVE" || -L "$LUCI_RUNTIME_ARCHIVE" ]]; then
  printf '%s\n' 'The retained sandbox recovery archive is missing or temporary. Restore the verified /workspace archive before publication. If both image and archive are unavailable, rebuild from the reviewed repository Dockerfile with enough disk; never substitute an unverified image.' >&2
  exit 1
fi
printf '%s  %s\n' "$LUCI_RUNTIME_SHA256" "$LUCI_RUNTIME_ARCHIVE" | sha256sum --check
if ! docker image inspect luci/sandbox:dev >/dev/null 2>&1; then
  docker load --input "$LUCI_RUNTIME_ARCHIVE"
fi
LUCI_ACTUAL_IMAGE=$(docker image inspect luci/sandbox:dev --format '{{.Id}}')
if [[ "$LUCI_ACTUAL_IMAGE" != "$LUCI_RUNTIME_IMAGE" ]]; then
  printf '%s\n' 'The existing sandbox tag differs from the retained verified image. Review intentional source/image changes and update provenance; this setup does not overwrite it or weaken identity checks.' >&2
  exit 1
fi
printf 'Prepared Luci sandbox %s\n' "$LUCI_ACTUAL_IMAGE"


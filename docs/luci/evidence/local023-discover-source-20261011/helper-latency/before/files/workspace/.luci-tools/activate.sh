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

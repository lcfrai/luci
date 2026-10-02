#!/usr/bin/env bash
# Build portable review artifacts. This script never creates cloud resources or
# publishes a release. Run npm ci in desktop before invoking it.
set -euo pipefail

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
version=${1:-0.3.0}
selected=${2:-all}
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9.-]+)?$ ]]; then
  printf '%s\n' 'Use a release version such as 0.3.0 or 0.3.0-rc.1.' >&2
  exit 2
fi
case "$selected" in all|linux-amd64|darwin-arm64) ;; *) printf '%s\n' 'Target must be all, linux-amd64 or darwin-arm64.' >&2; exit 2 ;; esac
for executable in go npm python3; do
  command -v "$executable" >/dev/null || { printf 'Missing build tool: %s\n' "$executable" >&2; exit 1; }
done
[[ -d "$root/desktop/node_modules" ]] || { printf '%s\n' 'Run npm ci in desktop first.' >&2; exit 1; }
[[ -f "$root/sandboxd/vendor/modules.txt" ]] || { printf '%s\n' 'The vendored sandbox runtime sources are required.' >&2; exit 1; }
output="$root/artifacts/releases/$version"
mkdir -p "$output"
stage=$(mktemp -d "$output/.luci-build-XXXXXX")
cleanup() {
  # Only remove this invocation's generated staging directory inside output.
  case "$stage" in "$output"/.luci-build-*) rm -rf -- "$stage" ;; esac
}
trap cleanup EXIT

(cd "$root/desktop" && npm run build)
build_package() {
  local goos=$1 goarch=$2 package=$3
  mkdir -p "$package/runtime/sandboxd" "$package/runtime/images" "$package/docs"
  (cd "$root" && GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o "$package/luci" ./control-plane/cmd/api)
  cp -R "$root/desktop/dist" "$package/web"
  for source in cmd internal vendor go.mod go.sum; do
    cp -R "$root/sandboxd/$source" "$package/runtime/sandboxd/$source"
  done
  cp -R "$root/images/sandbox" "$package/runtime/images/sandbox"
  cp "$root/LICENSE" "$root/NOTICE" "$root/README.md" "$package/"
  cp "$root/docs/luci/RELEASE.md" "$root/docs/luci/REUSE.md" "$package/docs/"
  cp "$root/scripts/launch-unix.sh" "$package/start-luci.sh"
  chmod 755 "$package/luci" "$package/start-luci.sh"
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 python3 "$root/scripts/collect-licenses.py" "$root" "$package/licenses"
}

# Snapshot the Linux runtime before adding cloud/ to any desktop package.
# This makes the transferred runtime self-contained without nesting itself.
cloud_package="$stage/luci-cloud-linux-amd64"
build_package linux amd64 "$cloud_package"
python3 "$root/scripts/package.py" "$cloud_package" linux
for target in linux-amd64 darwin-arm64; do
  [[ "$selected" == all || "$selected" == "$target" ]] || continue
  goos=${target%-*}
  goarch=${target#*-}
  package="$stage/luci-$target"
  if [[ "$target" == linux-amd64 ]]; then
    cp -R "$cloud_package" "$package"
  else
    build_package "$goos" "$goarch" "$package"
  fi
  mkdir -p "$package/cloud"
  cp "$cloud_package.tar.gz" "$package/cloud/luci-cloud-linux-amd64.tar.gz"
  python3 "$root/scripts/package.py" "$package" "$goos"
  mv -f -- "$package.tar.gz" "$output/luci-$target.tar.gz"
done
python3 - "$output" <<'PY'
import hashlib, pathlib, sys
output = pathlib.Path(sys.argv[1])
archives = sorted(p for p in output.iterdir() if p.is_file() and (p.name.endswith('.tar.gz') or p.suffix == '.zip'))
lines = [hashlib.file_digest(p.open('rb'), 'sha256').hexdigest() + '  ' + p.name for p in archives]
(output / 'SHA256SUMS').write_text('\n'.join(lines) + '\n', encoding='utf-8')
PY
printf 'Reviewable Luci packages: %s\n' "$output"

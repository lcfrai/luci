#!/usr/bin/env bash
# Build portable review artifacts. This script never creates cloud resources or
# publishes a release. Run npm ci in desktop before invoking it.
set -euo pipefail

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
version=${1:-0.4.6}
selected=${2:-all}
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9.-]+)?$ ]]; then
  printf '%s\n' 'Use a release version such as 0.4.6 or 0.4.6-rc.1.' >&2
  exit 2
fi
case "$selected" in all|linux-amd64|darwin-arm64) ;; *) printf '%s\n' 'Target must be all, linux-amd64 or darwin-arm64.' >&2; exit 2 ;; esac
for executable in go npm python3; do
  command -v "$executable" >/dev/null || { printf 'Missing build tool: %s\n' "$executable" >&2; exit 1; }
done
[[ -d "$root/desktop/node_modules" ]] || { printf '%s\n' 'Run npm ci in desktop first.' >&2; exit 1; }
[[ -f "$root/sandboxd/vendor/modules.txt" ]] || { printf '%s\n' 'The vendored sandbox runtime sources are required.' >&2; exit 1; }
# A source export can lack Git metadata. Do not accidentally attribute it to
# an enclosing unrelated checkout. Capture state before generated build files.
source_commit=
source_dirty=
if [[ -e "$root/.git" ]]; then
  command -v git >/dev/null || { printf '%s\n' 'Git is required to identify this source checkout.' >&2; exit 1; }
  source_commit=$(git -C "$root" rev-parse --verify HEAD)
  [[ "$source_commit" =~ ^([0-9a-f]{40}|[0-9a-f]{64})$ ]] || { printf '%s\n' 'Git returned an invalid source commit.' >&2; exit 1; }
  source_status=$(git -C "$root" status --porcelain=v1 --untracked-files=all)
  source_dirty=false
  [[ -z "$source_status" ]] || source_dirty=true
fi
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
cp "$cloud_package.tar.gz" "$output/luci-cloud-linux-amd64.tar.gz"
built_targets=()
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
  built_targets+=("$target")
done
python3 - "$output" "$version" "$source_commit" "$source_dirty" "$stage" "${built_targets[@]}" <<'PY'
import datetime, hashlib, json, pathlib, sys
output, version, commit, dirty, stage, *targets = sys.argv[1:]
output, stage = pathlib.Path(output), pathlib.Path(stage)
# Only this invocation's artifacts belong to this manifest, even when another
# platform's older archives still exist in the output directory.
archives = [output / 'luci-cloud-linux-amd64.tar.gz'] + [output / f'luci-{target}.tar.gz' for target in targets]
artifacts = []
for archive in sorted(archives):
    with archive.open('rb') as stream:
        digest = hashlib.file_digest(stream, 'sha256').hexdigest()
    artifacts.append({'name': archive.name, 'bytes': archive.stat().st_size, 'sha256': digest})
components = []
for target in targets:
    inventory = json.loads((stage / f'luci-{target}' / 'licenses/manifest.json').read_text(encoding='utf-8'))
    components.append({'target': f'luci-{target}', 'count': len(inventory['components']), 'missing': len(inventory['missingLicenses'])})
lines = [item['sha256'] + '  ' + item['name'] for item in artifacts]
(output / 'SHA256SUMS').write_text('\n'.join(lines) + '\n', encoding='utf-8')
manifest = {'version': version, 'builtAt': datetime.datetime.now(datetime.timezone.utc).isoformat(),
            'sourceCommit': commit or None, 'sourceDirty': dirty == 'true' if dirty else None,
            'artifacts': artifacts, 'portableLicenseComponents': components,
            'nativeLicenseComponents': 0, 'nativeRequested': False}
(output / 'release-manifest.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
PY
printf 'Reviewable Luci packages: %s\n' "$output"

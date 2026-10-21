#!/usr/bin/env bash
# Build portable review artifacts. This script never creates cloud resources or
# publishes a release. Run npm ci in desktop before invoking it.
set -euo pipefail
export GOMAXPROCS=1 GOMEMLIMIT=512MiB GOFLAGS='-mod=readonly -p=1' NODE_OPTIONS='--max-old-space-size=1024'
unset VITE_LUCI_API

root=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")/.." && pwd)
version=${1:-0.5.0}
selected=${2:-all}
runtime_payload=${3:-${LUCI_RUNTIME_PAYLOAD_DIR:-}}
if (( $# > 3 )); then shift 3; else set --; fi
e2b_source=
document_profile_dir=
document_profile_sha256=
document_tool_domain_sha256=
document_image_dir=
document_image_manifest_sha256=
allow_incomplete_document_runtime=false
while (( $# )); do
  case "$1" in
    --e2b-source)
      [[ -z "$e2b_source" && $# -ge 2 && -n "$2" ]] || { printf '%s\n' 'Supply --e2b-source once with the absolute pinned E2B checkout path.' >&2; exit 2; }
      e2b_source=$2
      shift 2 ;;
    --document-profile-dir|--document-profile-sha256|--document-tool-domain-sha256|--document-image-dir|--document-image-manifest-sha256)
      [[ $# -ge 2 && -n "$2" ]] || { printf 'Supply a value for %s.\n' "$1" >&2; exit 2; }
      case "$1" in
        --document-profile-dir) [[ -z "$document_profile_dir" ]] || exit 2; document_profile_dir=$2 ;;
        --document-profile-sha256) [[ -z "$document_profile_sha256" ]] || exit 2; document_profile_sha256=$2 ;;
        --document-tool-domain-sha256) [[ -z "$document_tool_domain_sha256" ]] || exit 2; document_tool_domain_sha256=$2 ;;
        --document-image-dir) [[ -z "$document_image_dir" ]] || exit 2; document_image_dir=$2 ;;
        --document-image-manifest-sha256) [[ -z "$document_image_manifest_sha256" ]] || exit 2; document_image_manifest_sha256=$2 ;;
      esac
      shift 2 ;;
    --allow-incomplete-document-runtime-for-development)
      [[ "$allow_incomplete_document_runtime" == false ]] || exit 2
      allow_incomplete_document_runtime=true
      shift ;;
    *) printf 'Unknown release argument: %s\n' "$1" >&2; exit 2 ;;
  esac
done
[[ -n "$runtime_payload" ]] || { printf '%s\n' 'Supply the verified producer runtime payload directory as argument 3 or LUCI_RUNTIME_PAYLOAD_DIR.' >&2; exit 2; }
[[ "$e2b_source" = /* && -d "$e2b_source" && ! -L "$e2b_source" ]] || {
  printf '%s\n' 'Supply --e2b-source with an existing absolute, ordinary pinned E2B checkout; this is a maintainer build input.' >&2; exit 2;
}
has_document_proof=false
if [[ -n "$document_profile_dir$document_profile_sha256$document_tool_domain_sha256" ]]; then
  [[ "$document_profile_dir" = /* && -d "$document_profile_dir" && ! -L "$document_profile_dir" && "$document_profile_sha256" =~ ^[0-9a-f]{64}$ && "$document_tool_domain_sha256" =~ ^[0-9a-f]{64}$ ]] || {
    printf '%s\n' 'Document proof packaging requires an absolute profile directory and both independently selected lowercase SHA256 pins.' >&2; exit 2;
  }
  has_document_proof=true
fi
has_document_image=false
if [[ -n "$document_image_dir$document_image_manifest_sha256" ]]; then
  [[ "$document_image_dir" = /* && -d "$document_image_dir" && ! -L "$document_image_dir" && "$document_image_manifest_sha256" =~ ^[0-9a-f]{64}$ ]] || {
    printf '%s\n' 'Document runtime packaging requires an absolute verified prebuilt directory and its independently selected manifest SHA256.' >&2; exit 2;
  }
  has_document_image=true
fi
[[ "$has_document_image" == true || "$allow_incomplete_document_runtime" == true ]] || {
  printf '%s\n' 'A full release requires --document-image-dir and --document-image-manifest-sha256 for its verified document runtime companion. Only an explicitly incomplete development build may omit them.' >&2; exit 2;
}
if [[ ! "$version" =~ ^[0-9]+\.[0-9]+\.[0-9]+([.-][A-Za-z0-9.-]+)?$ ]]; then
  printf '%s\n' 'Use a release version such as 0.5.0 or 0.5.0-rc.1.' >&2
  exit 2
fi
case "$selected" in all|linux-amd64|darwin-arm64) ;; *) printf '%s\n' 'Target must be all, linux-amd64 or darwin-arm64.' >&2; exit 2 ;; esac
for executable in go npm python3; do
  command -v "$executable" >/dev/null || { printf 'Missing build tool: %s\n' "$executable" >&2; exit 1; }
done
document_tool="$root/scripts/document_runtime_package.py"
document_license_args=()
if [[ "$has_document_proof" == true ]]; then
  python3 "$document_tool" verify-source --profile-dir "$document_profile_dir" --profile-sha256 "$document_profile_sha256" --tool-domain-sha256 "$document_tool_domain_sha256" >/dev/null
fi
if [[ "$has_document_image" == true ]]; then
  python3 "$document_tool" verify-image-source --image-dir "$document_image_dir" --image-manifest-sha256 "$document_image_manifest_sha256" >/dev/null
  document_license_args=(--document-image-dir "$document_image_dir" --document-image-manifest-sha256 "$document_image_manifest_sha256")
fi
python3 "$root/scripts/release-version.py" check "$version"
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
[[ "$source_dirty" == false ]] || { printf '%s\n' 'Runtime distribution requires the full clean source checkout.' >&2; exit 1; }
python3 "$root/scripts/runtime-oci/payload.py" verify --payload "$runtime_payload"
output="$root/artifacts/releases/$version"
mkdir -p "$output"
stage=$(mktemp -d "$output/.luci-build-XXXXXX")
cleanup() {
  # Only remove this invocation's generated staging directory inside output.
  case "$stage" in "$output"/.luci-build-*) rm -rf -- "$stage" ;; esac
}
trap cleanup EXIT

azure_setup="$stage/azure-setup"
mkdir -p "$azure_setup"
if [[ "$source_dirty" == false ]]; then
  azure_sha=$(python3 "$root/scripts/azure-setup/package.py" build --source "$root" --revision "$source_commit" --e2b-source "$e2b_source" --output "$azure_setup/azure-setup.tar")
  python3 "$root/scripts/azure-setup/package.py" verify --archive "$azure_setup/azure-setup.tar" --sha256 "$azure_sha" >/dev/null
  printf '%s\n' "$azure_sha" > "$azure_setup/azure-setup.sha256"
fi
(cd "$root/desktop" && npm run build)
guides="$stage/package-guides"
python3 "$root/scripts/stage-release-docs.py" "$root" "$guides" --source-commit="$source_commit" --source-dirty="${source_dirty:-unknown}"
build_package() {
  local goos=$1 goarch=$2 package=$3
  mkdir -p "$package/runtime/sandboxd" "$package/runtime/images" "$package/docs/luci"
  (cd "$root" && GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags '-s -w' -o "$package/luci" ./control-plane/cmd/api)
  cp -R "$root/desktop/dist" "$package/web"
  for source in cmd internal vendor go.mod go.sum; do
    cp -R "$root/sandboxd/$source" "$package/runtime/sandboxd/$source"
  done
  cp -R "$root/images/sandbox" "$package/runtime/images/sandbox"
  if [[ "$source_dirty" == false ]]; then cp "$azure_setup/azure-setup.tar" "$azure_setup/azure-setup.sha256" "$package/runtime/"; fi
  cp -R "$guides/." "$package/"
  cp "$root/scripts/launch-unix.sh" "$package/start-luci.sh"
  chmod 755 "$package/luci" "$package/start-luci.sh"
  if [[ "$has_document_proof" == true ]]; then
    python3 "$document_tool" stage --profile-dir "$document_profile_dir" --package "$package" --profile-sha256 "$document_profile_sha256" --tool-domain-sha256 "$document_tool_domain_sha256" >/dev/null
  fi
  if [[ "$has_document_image" == true ]]; then
    python3 "$document_tool" stage-image --image-dir "$document_image_dir" --package "$package" --image-manifest-sha256 "$document_image_manifest_sha256" >/dev/null
  fi
  GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 python3 "$root/scripts/collect-licenses.py" "$root" "$package/licenses" "${document_license_args[@]}"
  python3 "$root/scripts/runtime-oci/payload.py" stage --payload "$runtime_payload" --repo "$root" --revision "$source_commit" --package "$package"
}

# Snapshot the Linux runtime before adding cloud/ to any desktop package.
# This makes the transferred runtime self-contained without nesting itself.
cloud_package="$stage/luci-cloud-linux-amd64"
linux_base="$stage/luci-linux-runtime-base"
build_package linux amd64 "$linux_base"
python3 "$root/scripts/runtime-oci/payload.py" core-from-package --package "$linux_base" --output "$cloud_package"
verify_document_package() {
  if [[ "$has_document_proof" == true ]]; then
    python3 "$document_tool" verify --package "$1" --profile-sha256 "$document_profile_sha256" --tool-domain-sha256 "$document_tool_domain_sha256" >/dev/null
  fi
  if [[ "$has_document_image" == true ]]; then
    python3 "$document_tool" verify-image --package "$1" --image-manifest-sha256 "$document_image_manifest_sha256" >/dev/null
  fi
}
verify_document_package "$cloud_package"
python3 "$root/scripts/package.py" "$cloud_package" linux
cp "$cloud_package.tar.gz" "$output/luci-cloud-linux-amd64.tar.gz"
built_targets=()
for target in linux-amd64 darwin-arm64; do
  [[ "$selected" == all || "$selected" == "$target" ]] || continue
  goos=${target%-*}
  goarch=${target#*-}
  package="$stage/luci-$target"
  if [[ "$target" == linux-amd64 ]]; then
    cp -R "$linux_base" "$package"
  else
    build_package "$goos" "$goarch" "$package"
  fi
  mkdir -p "$package/cloud"
  cp "$cloud_package.tar.gz" "$package/cloud/luci-cloud-linux-amd64.tar.gz"
  verify_document_package "$package"
  python3 "$root/scripts/package.py" "$package" "$goos"
  mv -f -- "$package.tar.gz" "$output/luci-$target.tar.gz"
  built_targets+=("$target")
done
# A clean manifest cannot describe source that changed while packages built.
if [[ "$source_dirty" == false ]]; then
  [[ "$(git -C "$root" rev-parse --verify HEAD)" == "$source_commit" && -z "$(git -C "$root" status --porcelain=v1 --untracked-files=all)" ]] || {
    printf '%s\n' 'Source changed during packaging; no clean release manifest was written.' >&2
    exit 1
  }
fi
PYTHONPATH="$root/scripts/runtime-oci:$root/scripts" python3 - "$output" "$version" "$source_commit" "$source_dirty" "$stage" "$allow_incomplete_document_runtime" "$document_profile_sha256" "$document_tool_domain_sha256" "$document_image_manifest_sha256" "${built_targets[@]}" <<'PY'
import datetime, hashlib, json, pathlib, sys
output, version, commit, dirty, stage, incomplete, profile_pin, tool_pin, image_pin, *targets = sys.argv[1:]
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
import payload
manifest['runtimeDistribution'] = payload.distribution_record(stage / 'luci-linux-runtime-base')
import document_runtime_package
manifest['documentInputProof'] = {'profileSHA256': profile_pin, 'toolDomainSHA256': tool_pin} if profile_pin else None
document_image = document_runtime_package.verify_image_package(stage / 'luci-linux-runtime-base', image_pin or None)
if document_image is not None:
    document_image.pop('files')
manifest['documentRuntime'] = {'status': 'packaged-prebuilt' if document_image is not None else 'omitted-development',
                               'incompleteDevelopment': incomplete == 'true', 'companion': document_image}
(output / 'release-manifest.json').write_text(json.dumps(manifest, indent=2) + '\n', encoding='utf-8')
PY
printf 'Reviewable Luci packages: %s\n' "$output"

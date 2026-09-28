#!/usr/bin/env bash
# pipelines/scripts/install-tool.sh — install pinned tools, each verified by SHA-256 (task 28).
#
#   bash pipelines/scripts/install-tool.sh <tool> [<tool>…]
#
# Every tool is downloaded from its release URL over https only, its SHA-256 compared with the
# pin below, and only then unpacked into PANACEA_APP_TOOLS_DIR (default
# $HOME/.panacea-app/tools): executables in bin/, Helm plugins in helm-plugins/, tflint rulesets
# in tflint-plugins/. Nothing is piped into a shell and no pin names a moving version, because
# Panacea's run-time installs drifted (survey §5, :474-476). A download whose hash differs is
# deleted and the script exits 1 before anything is installed.
#
# In an Azure Pipelines job (TF_BUILD=True) it also prints the logging commands that put bin/ on
# PATH and set HELM_PLUGINS and TFLINT_PLUGIN_DIR for the later steps. On a workstation it
# prints the directory to add to PATH. CI and laptop run this same script and the same pins.
#
# The pins: name, platform (linux-amd64 is the hosted agent; windows-amd64 is the build laptop;
# "any" is a Go module built from source), version, kind, member, sha256, url.
#   kind bin           the download is the executable
#        zip tgz txz   an archive; member is the executable's path inside it
#        helm-plugin   a Helm plugin archive, unpacked whole into helm-plugins/<member>/
#        tflint-plugin a tflint ruleset zip; member is the executable inside it
#        gomod         a module zip from proxy.golang.org, verified, unpacked, and built with
#                      the Go on PATH (GOTOOLCHAIN=local, -mod=readonly: every dependency is
#                      checked against the module's own go.sum); member is the package to build
# Where the project publishes a checksum file, the pin is that file's value; where it does not
# (shellcheck, the two Go modules), the pin is the hash of a download made on 2026-09-25.
# Versions were chosen on 2026-09-25: the newest release, except govulncheck v1.1.4, the
# newest golang.org/x/vuln whose go directive (1.22) builds on CI's Go 1.23; v1.2.0 and later
# need Go 1.25. OpenTofu is 1.12.6, chosen on 2026-09-28 as the newest stable release.
#
# OD-32, applied 2026-09-28. This pin is closed; the bootstrap lock file is not (infra/terraform/README.md). The IaC engine is OpenTofu (MPL-2.0), never
# HashiCorp Terraform, which has been BUSL-1.1 since August 2023 and this product ships to
# customers to run on their own clouds. The two `tofu` rows replace the `terraform` rows that
# stood here. Both pins are the values published in
# https://github.com/opentofu/opentofu/releases/download/v1.12.6/tofu_1.12.6_SHA256SUMS, and
# both archives were downloaded and re-hashed on 2026-09-28: the sums matched and the member
# is `tofu` / `tofu.exe` at the archive root, beside CHANGELOG.md, LICENSE and README.md.
# The archive's LICENSE is MPL-2.0. Do not restore a `terraform` row: infra/scripts/validate.sh
# refuses TOFU_BIN=terraform by design. See infra/terraform/README.md, "The engine is OpenTofu".

# name                 platform      version kind          member                                               sha256                                                           url
read -r -d '' PINS <<'PINS' || true
tofu                   linux-amd64   1.12.6  zip           tofu                                                 5dc43da4f750f33873dc25e94587128709e819e544b7be9016b255316153c3a8 https://github.com/opentofu/opentofu/releases/download/v1.12.6/tofu_1.12.6_linux_amd64.zip
tofu                   windows-amd64 1.12.6  zip           tofu.exe                                             0d1421721cf9ec24b41b698a9620dda218d47fa7e76ac3dc15cdbc13bd79b0bb https://github.com/opentofu/opentofu/releases/download/v1.12.6/tofu_1.12.6_windows_amd64.zip
helm                   linux-amd64   4.3.0   tgz           linux-amd64/helm                                     86584a54def73570558f66f5111cc53dfed56689637ae32c1201205d494f54fb https://get.helm.sh/helm-v4.3.0-linux-amd64.tar.gz
helm                   windows-amd64 4.3.0   zip           windows-amd64/helm.exe                               304ea163cce4d9ad14e189c01846c6a34de9cfdfe48536ae54b2e8ba7884e67c https://get.helm.sh/helm-v4.3.0-windows-amd64.zip
helm-unittest          linux-amd64   1.1.2   helm-plugin   unittest                                             de03d6b7ba7683282064702d0b36fbbe812ab2a22691fa3a8b2ce57c2682609c https://github.com/helm-unittest/helm-unittest/releases/download/v1.1.2/helm-unittest-linux-amd64-1.1.2.tgz
helm-unittest          windows-amd64 1.1.2   helm-plugin   unittest                                             c41cb6d399d80183e1d77a70239236c345b83bd1c785a11ab5e84e5d76e3d79d https://github.com/helm-unittest/helm-unittest/releases/download/v1.1.2/helm-unittest-windows-amd64-1.1.2.tgz
kubeconform            linux-amd64   0.8.0   tgz           kubeconform                                          9bc2bffbf71f261128533edaf912153948b7ff238f9a531ae6d34466ec287883 https://github.com/yannh/kubeconform/releases/download/v0.8.0/kubeconform-linux-amd64.tar.gz
kubeconform            windows-amd64 0.8.0   zip           kubeconform.exe                                      e3f56102bcf4f50b034a567e2482a1c5330799983ddd655952310211aef73d93 https://github.com/yannh/kubeconform/releases/download/v0.8.0/kubeconform-windows-amd64.zip
kubelogin              linux-amd64   0.2.20  zip           bin/linux_amd64/kubelogin                            2e92450a929dd2aec4da818b40dd4fda97aaeae2d6cd9c1074cc1371210f4ab9 https://github.com/Azure/kubelogin/releases/download/v0.2.20/kubelogin-linux-amd64.zip
kubelogin              windows-amd64 0.2.20  zip           bin/windows_amd64/kubelogin.exe                      e26d5ce8a48e9b6ac53fd93cf45a1eec050f859ccca3654beb4887decd8dee33 https://github.com/Azure/kubelogin/releases/download/v0.2.20/kubelogin-win-amd64.zip
gitleaks               linux-amd64   8.30.1  tgz           gitleaks                                             551f6fc83ea457d62a0d98237cbad105af8d557003051f41f3e7ca7b3f2470eb https://github.com/gitleaks/gitleaks/releases/download/v8.30.1/gitleaks_8.30.1_linux_x64.tar.gz
gitleaks               windows-amd64 8.30.1  zip           gitleaks.exe                                         d29144deff3a68aa93ced33dddf84b7fdc26070add4aa0f4513094c8332afc4e https://github.com/gitleaks/gitleaks/releases/download/v8.30.1/gitleaks_8.30.1_windows_x64.zip
govulncheck            any           v1.1.4  gomod         cmd/govulncheck                                      115ff76fba8f73b27106eb2e59e3f30696f4f7faaeed55471b5b65c3994b503d https://proxy.golang.org/golang.org/x/vuln/@v/v1.1.4.zip
shellcheck             linux-amd64   0.11.0  txz           shellcheck-v0.11.0/shellcheck                        8c3be12b05d5c177a04c29e3c78ce89ac86f1595681cab149b65b97c4e227198 https://github.com/koalaman/shellcheck/releases/download/v0.11.0/shellcheck-v0.11.0.linux.x86_64.tar.xz
shellcheck             windows-amd64 0.11.0  zip           shellcheck.exe                                       8a4e35ab0b331c85d73567b12f2a444df187f483e5079ceffa6bda1faa2e740e https://github.com/koalaman/shellcheck/releases/download/v0.11.0/shellcheck-v0.11.0.zip
tflint                 linux-amd64   0.64.0  zip           tflint                                               cca9d13e2e1d7a2c627af60ff899a3c9b74212899416aeb96ec764d2ef954537 https://github.com/terraform-linters/tflint/releases/download/v0.64.0/tflint_linux_amd64.zip
tflint                 windows-amd64 0.64.0  zip           tflint.exe                                           fb42fb859d844b156a8ea9d3363078c4d8b85ca78782e60876b08c9b8e59f303 https://github.com/terraform-linters/tflint/releases/download/v0.64.0/tflint_windows_amd64.zip
tflint-ruleset-azurerm linux-amd64   0.32.0  tflint-plugin tflint-ruleset-azurerm                               f1150b2dea7aef001ad07002671c75b5495f4521ddf3538ededf00896f7b0aaa https://github.com/terraform-linters/tflint-ruleset-azurerm/releases/download/v0.32.0/tflint-ruleset-azurerm_linux_amd64.zip
tflint-ruleset-azurerm windows-amd64 0.32.0  tflint-plugin tflint-ruleset-azurerm.exe                           98025d2ae156aef33272a8520d7425da5f4a70f5e04aac8a7ba999766f0be3d1 https://github.com/terraform-linters/tflint-ruleset-azurerm/releases/download/v0.32.0/tflint-ruleset-azurerm_windows_amd64.zip
trivy                  linux-amd64   0.74.0  tgz           trivy                                                2ae6fe3ee734b7fdf11335663e18c75ea12dccc76062f09f164a3b0f8be4371a https://github.com/aquasecurity/trivy/releases/download/v0.74.0/trivy_0.74.0_Linux-64bit.tar.gz
trivy                  windows-amd64 0.74.0  zip           trivy.exe                                            94c40e0696e4b907a74b7b2e1438d5d72ebaca83115817407f568a002d520842 https://github.com/aquasecurity/trivy/releases/download/v0.74.0/trivy_0.74.0_windows-64bit.zip
syft                   linux-amd64   1.52.0  tgz           syft                                                 caeedb81fb0491615f1ebd1761e4145d41ee86dd2cc7bf80669f9f5ad9d6133d https://github.com/anchore/syft/releases/download/v1.52.0/syft_1.52.0_linux_amd64.tar.gz
syft                   windows-amd64 1.52.0  zip           syft.exe                                             de787a374cf961c56fd7b206b2e183295abba32d0af3cb44d9aef2357ca9eda2 https://github.com/anchore/syft/releases/download/v1.52.0/syft_1.52.0_windows_amd64.zip
go-licenses            any           v2.0.1  gomod         .                                                    caa4e1930299b96430f3b3fc98296b008cb38e0909caaaebdee270595c30b9ec https://proxy.golang.org/github.com/google/go-licenses/v2/@v/v2.0.1.zip
cargo-deny             linux-amd64   0.20.2  tgz           cargo-deny-0.20.2-x86_64-unknown-linux-musl/cargo-deny 9f12ed4c49936e09b48bf862b595cde2fe64fcbd9d74dfacac6131ca824c8d5f https://github.com/EmbarkStudios/cargo-deny/releases/download/0.20.2/cargo-deny-0.20.2-x86_64-unknown-linux-musl.tar.gz
cargo-deny             windows-amd64 0.20.2  tgz           cargo-deny-0.20.2-x86_64-pc-windows-msvc/cargo-deny.exe 975a22143262fd27476d19ee00c7af67978426e40e1dee94eed6bbade1cf87dc https://github.com/EmbarkStudios/cargo-deny/releases/download/0.20.2/cargo-deny-0.20.2-x86_64-pc-windows-msvc.tar.gz
yq                     linux-amd64   4.53.6  bin           yq_linux_amd64                                       c5f056448f973ae7d39b5401949648a78f2dc1947d6a8eb65be60d5c504b9385 https://github.com/mikefarah/yq/releases/download/v4.53.6/yq_linux_amd64
yq                     windows-amd64 4.53.6  bin           yq_windows_amd64.exe                                 ece3dd8bb50d39f93610506273ea262feb91e5c486bbddbb10abf91b2a6c0f14 https://github.com/mikefarah/yq/releases/download/v4.53.6/yq_windows_amd64.exe
PINS

TOOLS_DIR="${PANACEA_APP_TOOLS_DIR:-$HOME/.panacea-app/tools}"
PROTO='=https'     # curl --proto and --proto-redir: no http, no file, no downgrade on redirect
CURL=()
TMPS=()

die() { echo "install-tool: $*" >&2; exit 1; }

platform() {
  local os arch
  case "$(uname -s)" in
    Linux) os=linux ;;
    MINGW*|MSYS*|CYGWIN*) os=windows ;;
    Darwin) os=darwin ;;
    *) os=unknown ;;
  esac
  case "$(uname -m)" in
    x86_64|amd64) arch=amd64 ;;
    aarch64|arm64) arch=arm64 ;;
    *) arch=unknown ;;
  esac
  printf '%s-%s' "$os" "$arch"
}

# setup_curl: the one download command. Windows' curl uses Schannel, whose revocation check
# fails behind the KPMG proxy; --ssl-no-revoke skips only that check, never the certificate.
setup_curl() {
  CURL=(curl -fsSL --retry 3 --retry-delay 2 --proto "$PROTO" --proto-redir "$PROTO")
  case "$(curl -V 2>/dev/null)" in *Schannel*) CURL+=(--ssl-no-revoke) ;; esac
}

sha256_of() {
  local out
  if command -v sha256sum > /dev/null; then out="$(sha256sum "$1")" || return 1
  else out="$(shasum -a 256 "$1")" || return 1; fi
  printf '%s' "${out%% *}"
}

# fetch <tool> <url> <sha256> <file>: download, then refuse (and delete) a file whose hash differs.
fetch() {
  local got
  "${CURL[@]}" -o "$4" "$2" || { rm -f "$4"; die "$1: download failed: $2"; }
  got="$(sha256_of "$4")" || die "$1: cannot hash $4"
  if [ "$got" != "$3" ]; then
    rm -f "$4"
    die "$1: SHA-256 mismatch for $2: expected $3, got $got"
  fi
}

# publish <kind>: tell the pipeline (or the person) where the tool is.
publish() {
  if [ "${TF_BUILD:-}" = True ]; then
    echo "##vso[task.prependpath]$TOOLS_DIR/bin"
    case "$1" in
      helm-plugin)   echo "##vso[task.setvariable variable=HELM_PLUGINS]$TOOLS_DIR/helm-plugins" ;;
      tflint-plugin) echo "##vso[task.setvariable variable=TFLINT_PLUGIN_DIR]$TOOLS_DIR/tflint-plugins" ;;
    esac
  else
    case "$1" in
      helm-plugin)   echo "install-tool: export HELM_PLUGINS=$TOOLS_DIR/helm-plugins" ;;
      tflint-plugin) echo "install-tool: export TFLINT_PLUGIN_DIR=$TOOLS_DIR/tflint-plugins" ;;
      *)             echo "install-tool: put $TOOLS_DIR/bin first on PATH" ;;
    esac
  fi
}

# place <file> <dest>: install one executable, atomically.
place() {
  mkdir -p "${2%/*}" && cp "$1" "$2.new" && chmod 0755 "$2.new" && mv -f "$2.new" "$2"
}

# build_module <tool> <version> <package> <url> <tmp> <dest>: build a verified module zip.
build_module() {
  local mod="${4#https://proxy.golang.org/}"
  mod="${mod%/@v/*}"
  command -v go > /dev/null || { echo "install-tool: $1 needs Go on PATH" >&2; return 1; }
  unzip -q "$5/download" -d "$5/x" || return 1
  ( cd "$5/x/$mod@$2" &&
    GOWORK=off GOFLAGS=-mod=readonly GOTOOLCHAIN=local CGO_ENABLED=0 \
      go build -trimpath -o "$6" "./$3" )
}

# install_one <tool>: the pin for this platform (or "any"), downloaded, verified, unpacked.
# A stamp records the hash and version installed, so a re-run with the same pin downloads
# nothing, and a changed pin is always downloaded and verified again.
install_one() {
  local tool="$1" plat n p version kind member sha url found="" exe="" dest stamp line tmp
  plat="$(platform)"
  while read -r n p version kind member sha url; do
    case "$n" in ''|\#*) continue ;; esac
    if [ "$n" = "$tool" ] && { [ "$p" = "$plat" ] || [ "$p" = any ]; }; then found=1; break; fi
  done <<< "$PINS"
  [ -n "$found" ] || die "no pin for $tool on $plat"
  [[ "$sha" =~ ^[0-9a-f]{64}$ ]] || die "$tool: the pin has no SHA-256"
  case "$plat" in windows-*) exe=.exe ;; esac
  case "$kind" in
    helm-plugin)   dest="$TOOLS_DIR/helm-plugins/$member" ;;
    tflint-plugin) dest="$TOOLS_DIR/tflint-plugins/github.com/terraform-linters/$tool/$version/$tool$exe" ;;
    *)             dest="$TOOLS_DIR/bin/$tool$exe" ;;
  esac
  stamp="$TOOLS_DIR/stamps/$tool"
  if [ -e "$dest" ] && [ -f "$stamp" ] && IFS= read -r line < "$stamp" &&
     [ "$line" = "$sha $version" ]; then
    echo "install-tool: $tool $version already installed ($dest)"
    publish "$kind"
    return 0
  fi
  mkdir -p "$TOOLS_DIR/bin" "$TOOLS_DIR/stamps" || die "cannot create $TOOLS_DIR"
  tmp="$(mktemp -d "$TOOLS_DIR/.tmp.XXXXXX")" || die "mktemp failed"
  TMPS+=("$tmp")
  rm -f "$stamp"
  fetch "$tool" "$url" "$sha" "$tmp/download"
  mkdir -p "$tmp/x" || die "mktemp failed"
  case "$kind" in
    bin)           place "$tmp/download" "$dest" ;;
    zip)           unzip -q -o "$tmp/download" "$member" -d "$tmp/x" && place "$tmp/x/$member" "$dest" ;;
    tgz)           tar -xzf "$tmp/download" -C "$tmp/x" "$member" && place "$tmp/x/$member" "$dest" ;;
    txz)           tar -xJf "$tmp/download" -C "$tmp/x" "$member" && place "$tmp/x/$member" "$dest" ;;
    helm-plugin)   rm -rf "$dest" && mkdir -p "$dest" && tar -xzf "$tmp/download" -C "$dest" ;;
    tflint-plugin) unzip -q -o "$tmp/download" "$member" -d "$tmp/x" && place "$tmp/x/$member" "$dest" ;;
    gomod)         build_module "$tool" "$version" "$member" "$url" "$tmp" "$dest" ;;
    *)             false ;;
  esac || die "$tool: unpacking a $kind failed"
  printf '%s %s\n' "$sha" "$version" > "$stamp" || die "cannot write $stamp"
  rm -rf "$tmp"
  echo "install-tool: $tool $version -> $dest"
  publish "$kind"
}

main() {
  [ "$#" -gt 0 ] || die "usage: install-tool.sh <tool> [<tool>…]"
  trap 'rm -rf "${TMPS[@]}"' EXIT
  setup_curl
  local t
  for t in "$@"; do install_one "$t"; done
}

# Sourced (install-tool_test.sh), it defines the functions and the pins and runs nothing.
if [ "${BASH_SOURCE[0]}" = "$0" ]; then
  set -euo pipefail
  main "$@"
fi

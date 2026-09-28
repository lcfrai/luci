#!/usr/bin/env bash
# infra/scripts/validate.sh — the ONLY OpenTofu commands this phase may run (the one exception
# is task 07's by-hand `tofu graph` of the foundation stack: config only, review m8).
#
#   infra/scripts/validate.sh                 every dir under modules/ and stacks/
#   infra/scripts/validate.sh <dir> [<dir>…]  just these, relative to the repo root
#
# For each directory, in order, stopping that directory at the first failure:
#   1. tofu fmt -check -recursive -diff
#   2. stacks:  tofu init -backend=false -input=false -lockfile=readonly
#      modules: tofu init -backend=false -input=false
#   3. tofu validate -no-color
#   4. if the directory has a *.tftest.hcl, in tests/ or at its root (tofu test runs both,
#      plan re-review b04-2 M2):
#        modules: tofu test -no-color
#        stacks:  tofu test -no-color -var-file=<env>.tfvars, once per <env>.tfvars in the
#                 stack, so every committed tfvars file is proven to plan against mocks. The
#                 test files supply only what tfvars may not hold (ids, as TF_VAR_* would).
#                 A file-level `variables` block in a .tftest.hcl OVERRIDES -var-file
#                 (Terraform 1.15.8 probe, review I1 — every "Terraform 1.15.8" observation in
#                 this file predates OD-32 and was made on the old engine; the behaviour is
#                 expected to carry to OpenTofu but has NOT been re-probed there), so
#                 guards.sh G10 forbids a file-level
#                 key that any <env>.tfvars in the stack sets, and a one-line file-level block
#                 outright (fmt leaves one on one line, b04-2 I1); a run that needs an
#                 environment-specific value sets it in that run's own `variables` block.
#                 Run-level values override -var-file too (b04 probe, Terraform 1.15.8: a
#                 run pinning "dev" saw "dev" under a prod -var-file; a run with no pin and
#                 no file-level key saw "prod"), so the proof rests on each stack's
#                 `committed_tfvars_plan` run: `command = plan`, and no run-level key that a
#                 tfvars sets, so the dev pass plans dev.tfvars and the prod pass plans
#                 prod.tfvars, each untouched. That run cannot tell which pass it is in, so
#                 G10, not the run, is what kills a file-level override.
#
# A directory is a stack if its path has a stacks/ component, and a module otherwise.
#
# Output, one line per directory:   validate: OK   <dir>
#                                   validate: FAIL <dir> (<step>)
# Last line:                        validate: <n> ok, <m> failed
#                                   validate: nothing to validate yet     (no dirs exist)
# Exit status: 0 iff m == 0. A failed step's own output goes to stderr, indented.
#
# It never runs plan, apply, import, state or console, never passes -backend-config, and
# unsets ARM_SUBSCRIPTION_ID, ARM_CLIENT_ID, ARM_CLIENT_SECRET, ARM_USE_OIDC and
# ARM_OIDC_TOKEN for its own process tree, so a developer's shell credentials cannot reach a
# provider even if a test forgot a mock (guards.sh G3 is the first line; this is the second).
# It unsets every other ARM_* variable too, the Azure SDK's AZURE_* credential variables, the
# pipeline token variables, and TF_CLI_ARGS / TF_CLI_ARGS_* (which could add -backend-config).
# It also exports AZURE_CONFIG_DIR as a fresh empty directory (mktemp -d, removed by an EXIT
# trap), so no `az login` context is reachable either: azurerm's Azure CLI authentication
# finds no account, and a missed mock fails instead of reading the operator's subscription —
# who runs Panacea day to day, so that context is likely Panacea's (review I3).
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
cd "$HERE/../.."   # the repo root: <dir> arguments are relative to it

# --- the credential scrub, for this process tree -------------------------------------------------
for v in $(compgen -e); do
  case "$v" in
    ARM_*|TF_CLI_ARGS|TF_CLI_ARGS_*) unset "$v" ;;
  esac
done
unset AZURE_CLIENT_ID AZURE_CLIENT_SECRET AZURE_TENANT_ID AZURE_SUBSCRIPTION_ID \
  AZURE_FEDERATED_TOKEN_FILE AZURE_CLIENT_CERTIFICATE_PATH AZURE_CLIENT_CERTIFICATE_PASSWORD \
  AZURE_USERNAME AZURE_PASSWORD SYSTEM_ACCESSTOKEN \
  ACTIONS_ID_TOKEN_REQUEST_TOKEN ACTIONS_ID_TOKEN_REQUEST_URL

WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT
AZURE_CONFIG_DIR="$(mktemp -d "$WORK/azure.XXXXXX")"
if command -v cygpath > /dev/null 2>&1; then
  AZURE_CONFIG_DIR="$(cygpath -w "$AZURE_CONFIG_DIR")"   # a native tofu.exe reads it
fi
export AZURE_CONFIG_DIR
# OpenTofu reads the TF_* environment variables for compatibility (training knowledge,
# UNVERIFIED: no tofu binary on the build host to probe).
export TF_IN_AUTOMATION=1 TF_INPUT=0
LOG="$WORK/step.log"

# --- the engine (OD-32) --------------------------------------------------------------------------
# The engine is OpenTofu (MPL-2.0). HashiCorp Terraform is BUSL-1.1 and this product ships to
# customers' own clouds, so the terraform binary may not be used here — see
# infra/terraform/README.md, "The engine is OpenTofu, not Terraform".
# TOFU_BIN exists so a pinned install can be named by path. It may not name terraform.
TOFU="${TOFU_BIN:-tofu}"
case "$(basename "$TOFU")" in
  terraform|terraform.exe)
    echo "validate: refusing TOFU_BIN=$TOFU: terraform is BUSL-1.1 and OD-32 forbids it" >&2
    exit 2 ;;
esac
if ! command -v "$TOFU" > /dev/null 2>&1; then
  echo "validate: '$TOFU' not found. OD-32 made OpenTofu the engine; install it (pin it in" >&2
  echo "          pipelines/scripts/install-tool.sh and ask for it in the tools template), or" >&2
  echo "          set TOFU_BIN to its path. Do not substitute terraform." >&2
  exit 2
fi

# --- one directory -----------------------------------------------------------------------------
STEP=""
step() {   # step <name> <tofu args…>: run one step in $DIR, output to $LOG
  STEP="$1"; shift
  "$TOFU" -chdir="$DIR" "$@" > "$LOG" 2>&1
}
has_tests() {
  compgen -G "$DIR/tests/*.tftest.hcl" > /dev/null || compgen -G "$DIR/*.tftest.hcl" > /dev/null
}
validate_dir() {
  local kind=module v found=0
  case "/$DIR/" in */stacks/*) kind=stack ;; esac
  if [ ! -d "$DIR" ]; then STEP="no such directory"; : > "$LOG"; return 1; fi
  step fmt fmt -check -recursive -diff || return 1
  if [ "$kind" = stack ]; then
    step init init -backend=false -input=false -lockfile=readonly -no-color || return 1
  else
    step init init -backend=false -input=false -no-color || return 1
  fi
  step validate validate -no-color || return 1
  has_tests || return 0
  if [ "$kind" = module ]; then
    step test test -no-color || return 1
    return 0
  fi
  for v in "$DIR"/*.tfvars; do
    [ -f "$v" ] || continue
    found=1
    step "test ${v##*/}" test -no-color -var-file="${v##*/}" || return 1
  done
  [ "$found" = 1 ] || step test test -no-color || return 1
}

# --- the directories -----------------------------------------------------------------------------
DIRS=()
if [ "$#" -gt 0 ]; then
  for d in "$@"; do DIRS+=("${d%/}"); done
else
  for d in infra/terraform/modules/*/ infra/terraform/stacks/*/; do
    [ -d "$d" ] && DIRS+=("${d%/}")
  done
fi
if [ "${#DIRS[@]}" -eq 0 ]; then
  echo "validate: nothing to validate yet"
  exit 0
fi

ok=0 failed=0
for DIR in "${DIRS[@]}"; do
  if validate_dir; then
    echo "validate: OK   $DIR"
    ok=$((ok + 1))
  else
    echo "validate: FAIL $DIR ($STEP)"
    sed 's/^/    | /' "$LOG" >&2
    failed=$((failed + 1))
  fi
done
echo "validate: $ok ok, $failed failed"
[ "$failed" -eq 0 ]

#!/usr/bin/env bash
# infra/scripts/guards_test.sh — one throwaway fixture tree per guards.sh rule.
#
# Each case builds a conforming tree under $(mktemp -d), breaks exactly one rule, copies
# guards.sh in, runs it with INFRA_ROOT pointed at the fixture, and asserts on the exit status
# and the named line. Every case is a mutant guards.sh must kill (task 01's table).
#
#   bash infra/scripts/guards_test.sh                 all fifteen cases
#   bash infra/scripts/guards_test.sh <case>…         just these (names as listed at the end)
#
# Last line:  guards_test: <n> cases, <p> passed        exit 0 iff p == n
# A failed case prints its first failed assertion and guards.sh's output, indented.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GUARDS="$HERE/guards.sh"
WORK="$(mktemp -d)"
trap 'rm -rf "$WORK"' EXIT

TF="infra/terraform"   # every fixture path below is relative to the fixture root
T=""                   # the current fixture root
FAILED=""              # the current case's first failed assertion, "" while it holds
CASES=0
PASSED=0

# put <relative path> — write stdin to a file in the current fixture.
put() {
  mkdir -p "$T/${1%/*}"
  cat > "$T/$1"
}

# new_tree <case> — a copy of a tree every guard passes: one stack (x) and one module (net).
# The template is built once and copied, git index included: process start-up is slow on
# Windows, and fourteen fresh builds took most of three minutes.
new_tree() {
  [ -d "$WORK/template" ] || build_template
  T="$WORK/$1"
  cp -R "$WORK/template" "$T"
}

build_template() {
  T="$WORK/template"
  mkdir -p "$T/infra/scripts"
  put "$TF/modules/net/versions.tf" <<'EOF'
terraform {
  required_version = ">= 1.9"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.81"
    }
  }
}
EOF
  put "$TF/modules/net/main.tf" <<'EOF'
resource "azurerm_resource_group" "this" {
  name     = var.name
  location = var.location
  tags     = var.tags
}
EOF
  put "$TF/modules/net/tests/net.tftest.hcl" <<'EOF'
mock_provider "azurerm" {}

run "plans" {
  command = plan
}
EOF
  put "$TF/stacks/x/versions.tf" <<'EOF'
terraform {
  required_version = ">= 1.9"
  required_providers {
    azurerm = {
      source  = "hashicorp/azurerm"
      version = "~> 4.81"
    }
    azuread = {
      source  = "hashicorp/azuread"
      version = "~> 3.9"
    }
    azapi = {
      source  = "azure/azapi"
      version = "~> 2.12"
    }
  }
}
EOF
  put "$TF/stacks/x/backend.tf" <<'EOF'
terraform {
  backend "azurerm" {}
}
EOF
  put "$TF/stacks/x/providers.tf" <<'EOF'
provider "azurerm" {
  features {}
  storage_use_azuread             = true
  resource_provider_registrations = "none"
}

provider "azuread" {}

provider "azapi" {}
EOF
  put "$TF/stacks/x/main.tf" <<'EOF'
locals {
  tags = merge(var.tags, {
    product     = "panacea-app"
    environment = var.environment
    stack       = "x"
    managed-by  = "opentofu"
  })
}

module "net" {
  source   = "../../modules/net"
  name     = "rg-panacea-app-${var.environment}"
  location = var.location
  tags     = local.tags
}
EOF
  put "$TF/stacks/x/dev.tfvars" <<'EOF'
environment = "dev"
location    = "australiaeast"
EOF
  put "$TF/stacks/x/prod.tfvars" <<'EOF'
environment = "prod"
location    = "australiaeast"
api_private = true
EOF
  put "$TF/stacks/x/.terraform.lock.hcl" <<'EOF'
provider "registry.terraform.io/azure/azapi" {
  version = "2.12.0"
}

provider "registry.terraform.io/hashicorp/azuread" {
  version = "3.9.0"
}

provider "registry.terraform.io/hashicorp/azurerm" {
  version = "4.81.0"
}
EOF
  put "$TF/stacks/x/tests/x.tftest.hcl" <<'EOF'
mock_provider "azurerm" {}
mock_provider "azuread" {}
mock_provider "azapi" {}

variables {
  state_writer_object_ids = ["x"]
}

run "committed_tfvars_plan" {
  command = plan
}

run "names_follow_the_convention" {
  command = plan

  variables {
    environment = "dev"
    api_private = false
  }
}
EOF
  put "infra/terraform/README.md" <<'EOF'
# infra/terraform (fixture)
EOF
  # Not Terraform: task 30 keeps custom role definitions as JSON under infra/rbac/, so only
  # the two Terraform JSON forms (*.tf.json, *.tfvars.json) may fail G1, not every *.json.
  put "infra/rbac/plan-reader.json" <<'EOF'
{"Name": "panacea-app-plan-reader", "IsCustom": true}
EOF
  git init -q "$T" >/dev/null 2>&1
  git -C "$T" config core.autocrlf false
  track "$TF/stacks/x/.terraform.lock.hcl"
}

# track <relative path>… — git add in the fixture repo (index only; no commit needed).
track() {
  git -C "$T" add -- "$@"
}

# guards — copy guards.sh into the fixture and run it there. Output: $T/out.txt; status: $RC.
# A case that sets STUB to a directory puts it first on guards.sh's PATH (stub commands).
RC=""
STUB=""
guards() {
  if [ ! -f "$GUARDS" ]; then
    RC="missing"
    OUT=()
    : > "$T/out.txt"
    note "guards.sh does not exist at $GUARDS"
    return
  fi
  cp "$GUARDS" "$T/infra/scripts/guards.sh"
  PATH="${STUB:+$STUB:}$PATH" INFRA_ROOT="$T/infra" bash "$T/infra/scripts/guards.sh" > "$T/out.txt" 2>&1
  RC=$?
  mapfile -t OUT < "$T/out.txt"
}

# note <why> — record the case's first failed assertion.
note() { [ -n "$FAILED" ] || FAILED="$1"; }

# The matchers use bash, not grep: a process costs about 300 ms on the build laptop.
OUT=()
has_exact()    { local l; for l in "${OUT[@]}"; do [ "$l" = "$1" ] && return 0; done; return 1; }
has_contains() { local l; for l in "${OUT[@]}"; do case "$l" in *"$1"*) return 0 ;; esac; done; return 1; }

expect_pass() {        # exit 0 and the full summary line
  [ "$RC" = "0" ] || note "expected exit 0, got $RC"
  has_exact "guards: 10 of 10 passed" || note "no 'guards: 10 of 10 passed' line"
}
expect_fail() {        # exit non-zero
  [ "$RC" != "0" ] && [ "$RC" != "missing" ] || note "expected a non-zero exit from guards.sh, got $RC"
}
expect_only() {        # expect_only <n>: guard G<n> is the one failing guard
  expect_fail
  has_exact "guards: 9 of 10 passed" || note "expected 'guards: 9 of 10 passed' (only G$1 failing)"
}
expect_line() {        # expect_line <fixed string>: some output line contains it
  has_contains "$1" || note "no line containing '$1'"
}
expect_no_line() {     # expect_no_line <fixed string>
  if has_contains "$1"; then note "unexpected line containing '$1'"; fi
}
expect_ok() {          # expect_ok <n>: the exact line "guard G<n>: ok"
  has_exact "guard G$1: ok" || note "expected 'guard G$1: ok'"
}
expect_scan_failed() { # expect_scan_failed <line>: exit 2, that exact line, and no rule reported
  [ "$RC" = "2" ] || note "expected exit 2 from a failed scan, got $RC"
  has_exact "$1" || note "no line '$1'"
  expect_no_line "guard G"
  expect_no_line " of 10 passed"
}

# case <name> — run the function case_<name> and tally it.
case_run() {
  CASES=$((CASES + 1))
  FAILED=""
  STUB=""
  "case_$1"
  if [ -z "$FAILED" ]; then
    PASSED=$((PASSED + 1))
    echo "ok   $1"
  else
    echo "FAIL $1: $FAILED"
    [ -s "$T/out.txt" ] && sed 's/^/       | /' "$T/out.txt"
  fi
}

# ---------------------------------------------------------------------------------------------
# The cases. Each names the mutant it kills (task 01, "Tests to write first").

case_clean_tree_passes() {           # kills: a guard that fails on everything
  new_tree clean
  guards
  expect_pass
  for n in 1 2 3 4 5 6 7 8 9 10; do expect_ok "$n"; done
}

case_g1_terraform_tfvars_fails() {   # kills: G1 deleted, or matching only *.auto.tfvars
  new_tree g1a
  put "$TF/stacks/x/terraform.tfvars" <<< 'environment = "dev"'
  guards
  expect_only 1
  expect_line "guard G1: FAIL $TF/stacks/x/terraform.tfvars:"
  # …and the other auto-loaded name, so a G1 matching only terraform.tfvars dies too.
  rm -f "$T/$TF/stacks/x/terraform.tfvars"
  put "$TF/stacks/x/dev.auto.tfvars" <<< 'environment = "dev"'
  guards
  expect_only 1
  expect_line "guard G1: FAIL $TF/stacks/x/dev.auto.tfvars:"
  # JSON configuration is refused outright: G3, G5, G9 and G10 read HCL only (review M3).
  # kills: G1 without the JSON rule, or one that refuses only *.tf.json
  rm -f "$T/$TF/stacks/x/dev.auto.tfvars"
  put "$TF/stacks/x/extra.tf.json" <<< '{"locals": {"a": 1}}'
  put "$TF/stacks/x/prod.tfvars.json" <<< '{"environment": "prod"}'
  guards
  expect_only 1
  expect_line "guard G1: FAIL $TF/stacks/x/extra.tf.json: JSON configuration"
  expect_line "guard G1: FAIL $TF/stacks/x/prod.tfvars.json: JSON configuration"
}

case_g1_tracked_override_fails() {   # kills: no override check; *_override.tf only; untracked fails
  new_tree g1b
  put "$TF/stacks/x/backend_override.tf" <<< 'terraform { backend "local" {} }'
  put "$TF/stacks/y/override.tf" <<< 'terraform { backend "local" {} }'
  put "$TF/stacks/y/override.tf.json" <<< '{"terraform": {"backend": {"local": {}}}}'
  guards                             # on disk, untracked: runbook §2 needs exactly this
  expect_ok 1
  track "$TF/stacks/x/backend_override.tf" "$TF/stacks/y/override.tf" "$TF/stacks/y/override.tf.json"
  guards
  expect_fail
  expect_line "guard G1: FAIL $TF/stacks/x/backend_override.tf:"
  expect_line "guard G1: FAIL $TF/stacks/y/override.tf:"
  expect_line "guard G1: FAIL $TF/stacks/y/override.tf.json:"
  # Outside a git work tree tracking cannot be read, so G1 and G7 fail closed (review M2).
  # kills: G1 or G7 passing when git ls-files fails
  rm -rf "$T/.git"
  if git -C "$T" rev-parse --git-dir > /dev/null 2>&1; then
    note "fixture: $T is still inside a git work tree after removing .git"
  fi
  guards
  expect_fail
  expect_line "guard G1: FAIL infra: not inside a git work tree"
  expect_line "guard G7: FAIL $TF/stacks/x/.terraform.lock.hcl: not inside a git work tree"
}

case_g2_guid_in_tfvars_fails() {     # kills: G2 matching only lowercase hex
  new_tree g2
  put "$TF/stacks/x/dev.tfvars" <<'EOF'
environment = "dev"
location    = "australiaeast"
tenant_id   = "00000000-0000-0000-0000-000000000000"
EOF
  guards
  expect_only 2
  expect_line "guard G2: FAIL $TF/stacks/x/dev.tfvars:"
  # The all-zero id has no hex letter, so it cannot kill a lowercase-only G2. This one can.
  put "$TF/stacks/x/dev.tfvars" <<'EOF'
environment = "dev"
location    = "australiaeast"
tenant_id   = "7A3F9C2E-1B4D-4E8A-9C6B-2D5E8F1A3B7C"
EOF
  guards
  expect_only 2
  expect_line "guard G2: FAIL $TF/stacks/x/dev.tfvars:"
}

case_g3_test_without_azuread_mock_fails() {   # kills: G3 checking for "any" mock, not "every"
  new_tree g3
  put "$TF/stacks/x/tests/x.tftest.hcl" <<'EOF'
mock_provider "azurerm" {}
mock_provider "azapi" {}

run "committed_tfvars_plan" {
  command = plan
}
EOF
  guards
  expect_only 3
  expect_line "guard G3: FAIL $TF/stacks/x/tests/x.tftest.hcl: no mock_provider \"azuread\""
  # `tofu test` runs a root-level file too, and it needs the same mocks (review I2).
  # kills: G3 reading only tests/ (a root-level file checked against stacks/, which requires
  # nothing)
  put "$TF/stacks/x/tests/x.tftest.hcl" < "$WORK/template/$TF/stacks/x/tests/x.tftest.hcl"
  put "$TF/stacks/x/x.tftest.hcl" <<'EOF'
mock_provider "azurerm" {}
mock_provider "azapi" {}

run "committed_tfvars_plan" {
  command = plan
}
EOF
  guards
  expect_only 3
  expect_line "guard G3: FAIL $TF/stacks/x/x.tftest.hcl: no mock_provider \"azuread\""
  expect_no_line "guard G3: FAIL $TF/stacks/x/tests/"
}

case_g4_helm_provider_fails() {      # kills: G4 that only greps required_providers
  new_tree g4
  printf '\nprovider "helm" {}\n' >> "$T/$TF/stacks/x/providers.tf"
  guards
  expect_only 4
  expect_line "guard G4: FAIL $TF/stacks/x/providers.tf:"
  # …and the source form, so a G4 that only greps provider blocks dies too.
  new_tree g4b
  put "$TF/modules/net/versions.tf" <<'EOF'
terraform {
  required_providers {
    azurerm = {
      source = "hashicorp/azurerm"
    }
    helm = {
      source = "hashicorp/helm"
    }
  }
}
EOF
  guards
  expect_fail
  expect_line "guard G4: FAIL $TF/modules/net/versions.tf:"
}

case_g5_backend_with_body_fails() {  # kills: G5 that only checks the block exists
  new_tree g5
  put "$TF/stacks/x/backend.tf" <<'EOF'
terraform {
  backend "azurerm" {
    storage_account_name = "x"
  }
}
EOF
  guards
  expect_only 5
  expect_line "guard G5: FAIL $TF/stacks/x/backend.tf:"
  # No backend.tf at all: `tofu init -backend-config=<file>` on a configuration with no
  # backend block exits 0 with a warning and uses local state (review I3), so missing fails.
  # kills: G5 that checks only the backend.tf files that exist
  rm -f "$T/$TF/stacks/x/backend.tf"
  guards
  expect_only 5
  expect_line "guard G5: FAIL $TF/stacks/x/backend.tf: missing"
}

case_g6_local_exec_fails() {         # kills: G6 deleted
  new_tree g6
  put "$TF/stacks/x/hook.tf" <<'EOF'
resource "terraform_data" "hook" {
  provisioner "local-exec" {
    command = "echo hi"
  }
}
EOF
  guards
  expect_only 6
  expect_line "guard G6: FAIL $TF/stacks/x/hook.tf:"
}

case_g7_untracked_lock_fails() {     # kills: G7 that tests file existence instead of tracking
  new_tree g7
  git -C "$T" rm -q --cached -- "$TF/stacks/x/.terraform.lock.hcl"
  [ -f "$T/$TF/stacks/x/.terraform.lock.hcl" ] || note "fixture: the lock file should still exist"
  guards
  expect_only 7
  expect_line "guard G7: FAIL $TF/stacks/x/.terraform.lock.hcl:"
}

case_g8_client_secret_fails() {      # kills: G8 that matches only administrator_password
  new_tree g8
  put "$TF/stacks/x/app.tf" <<'EOF'
resource "azuread_application_password" "this" {
  application_id = var.application_id
}

locals {
  sp = {
    client_id     = var.client_id
    client_secret = var.x
  }
}
EOF
  guards
  expect_only 8
  expect_line "guard G8: FAIL $TF/stacks/x/app.tf:"
}

case_g9_external_source_fails() {    # kills: G9 deleted, or checking stacks only
  new_tree g9a
  put "$TF/modules/net/versions.tf" <<'EOF'
terraform {
  required_providers {
    azurerm = {
      source = "hashicorp/azurerm"
    }
    external = {
      source = "hashicorp/external"
    }
  }
}
EOF
  put "$TF/modules/net/tests/net.tftest.hcl" <<'EOF'
mock_provider "azurerm" {}
mock_provider "external" {}

run "plans" {
  command = plan
}
EOF
  guards
  expect_only 9
  expect_line "guard G9: FAIL $TF/modules/net/versions.tf:"
  # An entry with no source is inferred as hashicorp/<name>, so it is checked as that.
  # kills: G9 that skips an entry without a source (predicted survivor P1)
  put "$TF/modules/net/versions.tf" <<'EOF'
terraform {
  required_providers {
    azurerm = {
      source = "hashicorp/azurerm"
    }
    external = {
      version = "~> 2.3"
    }
  }
}
EOF
  guards
  expect_only 9
  expect_line "guard G9: FAIL $TF/modules/net/versions.tf: provider \"external\" has source hashicorp/external"
  # fmt leaves a one-argument block on one line (fmt -check exits 0, Terraform 1.15.8 probe).
  # kills: a required_providers reader that sees only the spread-out layout (predicted P2)
  put "$TF/modules/net/versions.tf" <<'EOF'
terraform {
  required_providers { external = { source = "hashicorp/external" } }
}
EOF
  guards
  expect_only 9
  expect_line "guard G9: FAIL $TF/modules/net/versions.tf: provider \"external\" has source hashicorp/external"
}

case_g9_lockfile_provider_fails() {  # kills: G9 that reads only required_providers
  new_tree g9b
  printf '\nprovider "registry.terraform.io/hashicorp/external" {\n  version = "2.3.5"\n}\n' \
    >> "$T/$TF/stacks/x/.terraform.lock.hcl"
  track "$TF/stacks/x/.terraform.lock.hcl"
  guards
  expect_only 9
  expect_line "guard G9: FAIL $TF/stacks/x/.terraform.lock.hcl:"
}

# The file-level block below is exactly what `tofu fmt` (1.15.8) leaves: it aligns the
# `=` of consecutive attributes, so the short key sits under the long one with 13 spaces.
# kills: G10 deleted; G10 reading only dev.tfvars; a single-space key pattern; G10 flagging
# run-level variables; G10 reading only tests/ (a root-level *.tftest.hcl runs too, b04-2 M2)
case_g10_tftest_file_level_tfvars_key_fails() {
  new_tree g10a
  put "$TF/stacks/x/tests/x.tftest.hcl" <<'EOF'
mock_provider "azurerm" {}
mock_provider "azuread" {}
mock_provider "azapi" {}

variables {
  state_writer_object_ids = ["x"]
  api_private             = false
}

run "committed_tfvars_plan" {
  command = plan
}
EOF
  guards
  expect_only 10
  expect_line "guard G10: FAIL $TF/stacks/x/tests/x.tftest.hcl: file-level variables key api_private"
  expect_no_line "state_writer_object_ids"
  # The same key inside a run's own variables block is the fix, and must pass.
  put "$TF/stacks/x/tests/x.tftest.hcl" <<'EOF'
mock_provider "azurerm" {}
mock_provider "azuread" {}
mock_provider "azapi" {}

variables {
  state_writer_object_ids = ["x"]
}

run "committed_tfvars_plan" {
  command = plan
}

run "private_api" {
  command = plan

  variables {
    state_writer_object_ids = ["y"]
    api_private             = false
  }
}
EOF
  guards
  expect_pass
  # A root-level test file is run by `tofu test` as well, so G10 reads it too.
  put "$TF/stacks/x/x.tftest.hcl" <<'EOF'
mock_provider "azurerm" {}
mock_provider "azuread" {}
mock_provider "azapi" {}

variables {
  state_writer_object_ids = ["x"]
  api_private             = false
}

run "committed_tfvars_plan" {
  command = plan
}
EOF
  guards
  expect_only 10
  expect_line "guard G10: FAIL $TF/stacks/x/x.tftest.hcl: file-level variables key api_private"
}

# `tofu fmt` leaves a one-argument block on one line and `fmt -check` exits 0 (b04-2 I1,
# probe p1), so the key sits on the opener line. G10 refuses the layout outright.
# kills: G10 that reads only the lines after `variables {`
case_g10_one_line_file_level_variables_fails() {
  new_tree g10b
  put "$TF/stacks/x/tests/x.tftest.hcl" <<'EOF'
mock_provider "azurerm" {}
mock_provider "azuread" {}
mock_provider "azapi" {}

variables { api_private = false }

run "committed_tfvars_plan" {
  command = plan
}
EOF
  guards
  expect_only 10
  expect_line "guard G10: FAIL $TF/stacks/x/tests/x.tftest.hcl: one-line file-level variables block"
  # Any one-line file-level block, even of a key no tfvars sets: multi-line is the only layout.
  sed -i 's/^variables { api_private = false }$/variables { state_writer_object_ids = ["x"] }/' \
    "$T/$TF/stacks/x/tests/x.tftest.hcl"
  guards
  expect_only 10
  expect_line "guard G10: FAIL $TF/stacks/x/tests/x.tftest.hcl: one-line file-level variables block"
  # A one-line block inside a run is a run-level block, and is fine.
  put "$TF/stacks/x/tests/x.tftest.hcl" <<'EOF'
mock_provider "azurerm" {}
mock_provider "azuread" {}
mock_provider "azapi" {}

run "committed_tfvars_plan" {
  command = plan
}

run "dev_names" {
  variables { environment = "dev" }
}
EOF
  guards
  expect_pass
}

# A read that fails must not be taken for "nothing found" (review I1): with an awk that failed,
# seven rules printed ok having checked nothing. The tree breaks G2, so a fail-open run shows.
# stub <name> <status> [<real>] — a command that exits <status>. Without <real> it prints to
# stderr only; with <real> it first runs <real> with its arguments, so its output is complete
# and only the status says it failed.
stub() {
  mkdir -p "$STUB"
  if [ -n "${3:-}" ]; then
    printf '#!/bin/sh\n"%s" "$@"\nexit %s\n' "$3" "$2" > "$STUB/$1"
  else
    printf '#!/bin/sh\necho "%s: stub failure" >&2\nexit %s\n' "$1" "$2" > "$STUB/$1"
  fi
  chmod +x "$STUB/$1"
}
# kills: awk's exit status unread (a scanner failure reported as ok); the inventory's unread
case_scanner_failure_fails_closed() {
  new_tree scan
  put "$TF/stacks/x/dev.tfvars" <<'EOF'
environment = "dev"
location    = "australiaeast"
tenant_id   = "7A3F9C2E-1B4D-4E8A-9C6B-2D5E8F1A3B7C"
EOF
  guards                             # control: with the real awk, the GUID fails G2
  expect_only 2
  STUB="$T/stub"
  stub awk 2
  guards
  expect_scan_failed "guards: scanner failed (awk exit 2)"
  rm -f "$STUB/awk"
  stub find 1
  guards
  expect_scan_failed "guards: inventory failed (find | sort exit 1)"
  rm -f "$STUB/find"
  # Output that looks complete, with a status that says the read failed, is still a failure.
  # kills: sort's status ignored (only find's read); awk's failure ignored whenever awk
  # printed something (fix round 1's predicted survivors S1 and S2)
  stub sort 1 "$(command -v sort)"
  guards
  expect_scan_failed "guards: inventory failed (find | sort exit 1)"
  rm -f "$STUB/sort"
  stub awk 2 "$(command -v awk)"
  guards
  expect_scan_failed "guards: scanner failed (awk exit 2)"
}

# ---------------------------------------------------------------------------------------------

ALL=(
  clean_tree_passes
  g1_terraform_tfvars_fails
  g1_tracked_override_fails
  g2_guid_in_tfvars_fails
  g3_test_without_azuread_mock_fails
  g4_helm_provider_fails
  g5_backend_with_body_fails
  g6_local_exec_fails
  g7_untracked_lock_fails
  g8_client_secret_fails
  g9_external_source_fails
  g9_lockfile_provider_fails
  g10_tftest_file_level_tfvars_key_fails
  g10_one_line_file_level_variables_fails
  scanner_failure_fails_closed
)
[ "$#" -gt 0 ] && ALL=("$@")
for c in "${ALL[@]}"; do
  if declare -F "case_$c" > /dev/null; then
    case_run "$c"
  else
    CASES=$((CASES + 1))
    echo "FAIL $c: no such case"
  fi
done

echo "guards_test: $CASES cases, $PASSED passed"
[ "$PASSED" -eq "$CASES" ]

#!/usr/bin/env bash
# pipelines/scripts/matrix_test.sh — matrix.sh on trees built here (task 28).
#
#   bash pipelines/scripts/matrix_test.sh
#
# Output: one line per case, then   matrix_test: <k> of <n> cases passed   exit 0 iff k == n
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
M="${PANACEA_APP_MATRIX:-$HERE/matrix.sh}"
WORK="$(mktemp -d)" || { echo "matrix_test: mktemp failed"; exit 2; }
trap 'rm -rf "$WORK"' EXIT
V='##vso[task.setvariable variable='

pass=0 n=0
check() { # check <name> <kind> <tree> <want legs> <want count>
  local out want
  n=$((n + 1))
  out="$(bash "$M" "$2" "$3" 2>&1)"
  want="${V}legs;isOutput=true]$4"$'\n'"${V}count;isOutput=true]$5"
  if [ "$out" = "$want" ]; then echo "ok   $1"; pass=$((pass + 1))
  else echo "FAIL $1"; printf '    got:  %s\n' "$out"; printf '    want: %s\n' "$want"; fi
}

mkdir -p "$WORK/empty"
check no_directories terraform "$WORK/empty" '{}' 0
check no_charts charts "$WORK/empty" '{}' 0

mkdir -p "$WORK/tf/infra/terraform/modules/aks" "$WORK/tf/infra/terraform/modules/ci-pool" \
  "$WORK/tf/infra/terraform/stacks/foundation"
touch "$WORK/tf/infra/terraform/README.md"
check modules_and_stacks terraform "$WORK/tf" \
  '{"modules_aks":{"dir":"infra/terraform/modules/aks"},"modules_ci_pool":{"dir":"infra/terraform/modules/ci-pool"},"stacks_foundation":{"dir":"infra/terraform/stacks/foundation"}}' 3

mkdir -p "$WORK/ch/deploy/charts/hub" "$WORK/ch/deploy/charts/not-a-chart"
touch "$WORK/ch/deploy/charts/hub/Chart.yaml"
check only_dirs_with_chart_yaml charts "$WORK/ch" '{"hub":{"dir":"deploy/charts/hub"}}' 1

n=$((n + 1))
if bash "$M" nonsense "$WORK/empty" > /dev/null 2>&1; then echo "FAIL unknown_kind_refused"
else echo "ok   unknown_kind_refused"; pass=$((pass + 1)); fi

echo "matrix_test: $pass of $n cases passed"
[ "$pass" -eq "$n" ]

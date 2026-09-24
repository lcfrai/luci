#!/usr/bin/env bash
# pipelines/scripts/matrix.sh — the job matrix for a per-directory job (task 28), as the two
# output variables an Azure Pipelines matrix reads:
#
#   bash pipelines/scripts/matrix.sh terraform [<root>]   every infra/terraform/{modules,stacks}/*/
#   bash pipelines/scripts/matrix.sh charts    [<root>]   every deploy/charts/*/ with a Chart.yaml
#
# prints   ##vso[task.setvariable variable=legs;isOutput=true]{"<leg>":{"dir":"<dir>"},…}
#          ##vso[task.setvariable variable=count;isOutput=true]<n>
# A leg is the directory below the base with every character outside [A-Za-z0-9_] made "_"
# (modules/aks -> modules_aks), because a matrix leg name allows no other. The consuming job skips
# on count 0, so a tree with no directory yet (true until the first module or chart lands) runs nothing.
set -uo pipefail
case "${1:-}" in
  terraform) BASE=infra/terraform; PATTERNS="infra/terraform/modules/*/ infra/terraform/stacks/*/" ;;
  charts) BASE=deploy/charts; PATTERNS="deploy/charts/*/" ;;
  *) echo "usage: matrix.sh terraform|charts [<root>]" >&2; exit 2 ;;
esac
cd "${2:-.}" 2> /dev/null || { echo "matrix: ${2:-.} is not a directory" >&2; exit 2; }
shopt -s nullglob
# shellcheck disable=SC2206  # the patterns are expanded here, after the cd, on purpose
DIRS=($PATTERNS)
legs="" n=0
for d in "${DIRS[@]}"; do
  d="${d%/}"
  [ "$1" = charts ] && [ ! -f "$d/Chart.yaml" ] && continue
  leg="${d#"$BASE"/}"
  leg="${leg//[^A-Za-z0-9_]/_}"
  legs+="${legs:+,}\"$leg\":{\"dir\":\"$d\"}"
  n=$((n + 1))
done
echo "##vso[task.setvariable variable=legs;isOutput=true]{$legs}"
echo "##vso[task.setvariable variable=count;isOutput=true]$n"

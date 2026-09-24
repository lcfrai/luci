#!/usr/bin/env bash
# pipelines/scripts/chart-licences_test.sh — the nine cases of the Helm half of the OD-11
# licence gate (task 28, review m6: the part's eight, and a Chart.yaml with dependencies but no
# Chart.lock, added by the implementer). Each case is a fixture tree under
# testdata/chart-licences/<case>/ that chart-licences.sh is pointed at; charts.yml runs this
# before the real check, so a weakened script fails here first.
#
#   bash pipelines/scripts/chart-licences_test.sh
#
# Output: one line per case, "ok   <case>" or "FAIL <case>: <why>", then
#         chart-licences_test: <k> of 9 cases passed          exit 0 iff k == 9
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA="$HERE/testdata/chart-licences"
CHECK="${PANACEA_APP_CHART_LICENCES:-$HERE/chart-licences.sh}"

# case | exit status | a line the output must contain
CASES=(
  "unrecorded|1|chart-licences: FAIL deploy/charts/hub/Chart.lock: postgresql 16.4.1 (https://charts.example.invalid/bitnami): no row in licences/charts.yaml"
  "version_bump|1|chart-licences: FAIL deploy/charts/hub/Chart.lock: postgresql 16.4.1 (https://charts.example.invalid/bitnami): licences/charts.yaml records version 16.4.0, not 16.4.1"
  "licence_not_allowed|1|chart-licences: FAIL deploy/charts/hub/Chart.lock: postgresql 16.4.1 (https://charts.example.invalid/bitnami): licence GPL-3.0-only is not in licences/allow.yaml and has no exception"
  "denied_name|1|chart-licences: FAIL deploy/charts/canvas/Chart.lock: tldraw 1.0.0 (https://charts.example.invalid/tl): tldraw is in denied_packages"
  "no_source|1|chart-licences: FAIL deploy/charts/hub/Chart.lock: postgresql 16.4.1 (https://charts.example.invalid/bitnami): the row has no source"
  "all_allowed|0|chart-licences: 3 chart dependencies, all recorded and allowed"
  "no_chart_lock|0|chart-licences: 0 chart dependencies"
  "stale_row|1|chart-licences: FAIL licences/charts.yaml: redis 20.1.0 (https://charts.example.invalid/bitnami): stale, matches no Chart.lock entry"
  "chart_yaml_without_lock|1|chart-licences: FAIL deploy/charts/hub/Chart.yaml: declares dependencies but has no Chart.lock"
)

pass=0
for row in "${CASES[@]}"; do
  IFS='|' read -r name want_status want_line <<< "$row"
  out="$(bash "$CHECK" "$DATA/$name" 2>&1)"
  status=$?
  if [ "$status" != "$want_status" ]; then
    echo "FAIL $name: exit $status, want $want_status"
  else
    case $'\n'"$out"$'\n' in
      *$'\n'"$want_line"$'\n'*) echo "ok   $name"; pass=$((pass + 1)); continue ;;
      *) echo "FAIL $name: no line \"$want_line\"" ;;
    esac
  fi
  while IFS= read -r l; do printf '    %s\n' "$l"; done <<< "$out"
done
echo "chart-licences_test: $pass of ${#CASES[@]} cases passed"
[ "$pass" -eq "${#CASES[@]}" ]

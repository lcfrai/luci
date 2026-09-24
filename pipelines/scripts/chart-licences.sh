#!/usr/bin/env bash
# pipelines/scripts/chart-licences.sh — the Helm half of the OD-11 licence gate (task 28,
# review m6). A chart's Chart.yaml has no standard licence field, so the licence of each chart
# dependency is recorded by a person in licences/charts.yaml, and this script makes that record
# mandatory and machine-checked.
#
#   bash pipelines/scripts/chart-licences.sh [<root>]      <root> defaults to the repository root
#
# It reads <root>/deploy/charts/*/Chart.lock, <root>/deploy/charts/*/Chart.yaml,
# <root>/licences/charts.yaml, allow.yaml and exceptions.yaml, and nothing outside <root>. No
# network call. It fails, naming the chart and the entry, when a Chart.lock entry has no row
# (same name and repository), when the row's version differs from the lock's, when the row has
# no source or no licence, when the name is in denied_packages, or when the licence is neither in
# allow.yaml nor named, for this chart, in exceptions.yaml (ecosystem helm). A row that matches no
# Chart.lock entry is stale and fails too, so the file cannot drift into a list nobody uses. A
# Chart.yaml that declares dependencies with no Chart.lock beside it fails as well: without the
# lock, `helm dependency build` would resolve versions nobody recorded (task 28 departure).
#
# It does not prove a recorded licence is right: that is the reviewer's read of the source URL.
#
# Output: one "chart-licences: FAIL <file>: <name> <version> (<repository>): <why>" per problem,
# then  chart-licences: <n> chart dependencies, all recorded and allowed      exit 0
#   or  chart-licences: 0 chart dependencies                                   exit 0
#   or  chart-licences: <n> chart dependencies, <k> problems                   exit 1
# A file that cannot be read is exit 2.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=pipelines/scripts/lib/allow.sh
source "$HERE/lib/allow.sh"

ROOT="${1:-$(git -C "$HERE" rev-parse --show-toplevel)}" || exit 2
cd "$ROOT" 2> /dev/null || { echo "chart-licences: $ROOT is not a directory"; exit 2; }
command -v yq > /dev/null ||
  { echo "chart-licences: yq is not on PATH (bash pipelines/scripts/install-tool.sh yq)"; exit 2; }
load_allow . || {
  while IFS= read -r l; do echo "chart-licences: FAIL $l"; done <<< "$ALLOW_ERR"
  exit 1
}

problems=0
fail() { echo "chart-licences: FAIL $*"; problems=$((problems + 1)); }

shopt -s nullglob
LOCKS=(deploy/charts/*/Chart.lock)
CHARTS=(deploy/charts/*/Chart.yaml)

for c in "${CHARTS[@]}"; do
  [ -f "${c%/Chart.yaml}/Chart.lock" ] && continue
  n="$(yq '.dependencies | length' "$c")" || { echo "chart-licences: cannot read $c"; exit 2; }
  [ "${n%$'\r'}" = 0 ] || fail "$c: declares dependencies but has no Chart.lock"
done

LOCKROWS=()
if [ "${#LOCKS[@]}" -gt 0 ]; then
  out="$(yq '.dependencies[] | [filename, .name, .repository, .version] | map(. // "") | join("|")' \
    "${LOCKS[@]}")" || { echo "chart-licences: cannot read a Chart.lock"; exit 2; }
  while IFS= read -r l; do l="${l%$'\r'}"; [ -n "$l" ] && LOCKROWS+=("$l"); done <<< "$out"
fi

[ -f licences/charts.yaml ] || { echo "chart-licences: FAIL licences/charts.yaml is missing"; exit 1; }
out="$(yq '.dependencies[] | [.name, .repository, .version, .licence, .source] | map(. // "") | join("|")' \
  licences/charts.yaml)" || { echo "chart-licences: cannot read licences/charts.yaml"; exit 2; }
ROWS=()
while IFS= read -r l; do l="${l%$'\r'}"; [ -n "$l" ] && ROWS+=("$l"); done <<< "$out"

# Each lock entry against the rows: the same name, repository and version is a match; the same
# name and repository at another version is a bump nobody re-read the licence for.
USED=()
for i in "${!ROWS[@]}"; do USED[i]=0; done
for e in "${LOCKROWS[@]}"; do
  IFS='|' read -r file name repo ver <<< "$e"
  what="$file: $name $ver ($repo)"
  hit="" near=""
  for i in "${!ROWS[@]}"; do
    IFS='|' read -r rn rr rv _ <<< "${ROWS[i]}"
    [ "$rn" = "$name" ] && [ "$rr" = "$repo" ] || continue
    if [ "$rv" = "$ver" ]; then hit="$i"; break; fi
    [ -n "$near" ] || near="$i"
  done
  if [ -z "$hit" ]; then
    if [ -n "$near" ]; then
      IFS='|' read -r _ _ rv _ <<< "${ROWS[near]}"
      USED[near]=1
      fail "$what: licences/charts.yaml records version $rv, not $ver"
    else
      fail "$what: no row in licences/charts.yaml"
    fi
    continue
  fi
  USED[hit]=1
  IFS='|' read -r _ _ _ lic src <<< "${ROWS[hit]}"
  [ -n "$src" ] || fail "$what: the row has no source"
  if denied "$name"; then
    fail "$what: $name is in denied_packages"
  elif [ -z "$lic" ]; then
    fail "$what: the row has no licence"
  elif ! licence_ok helm "$name" "$lic"; then
    fail "$what: licence $lic is not in licences/allow.yaml and has no exception"
  fi
done

for i in "${!ROWS[@]}"; do
  [ "${USED[i]}" = 1 ] && continue
  IFS='|' read -r rn rr rv _ <<< "${ROWS[i]}"
  fail "licences/charts.yaml: $rn $rv ($rr): stale, matches no Chart.lock entry"
done

n="${#LOCKROWS[@]}"
if [ "$problems" -gt 0 ]; then
  echo "chart-licences: $n chart dependencies, $problems problems"
  exit 1
fi
if [ "$n" -eq 0 ]; then
  echo "chart-licences: 0 chart dependencies"
else
  echo "chart-licences: $n chart dependencies, all recorded and allowed"
fi

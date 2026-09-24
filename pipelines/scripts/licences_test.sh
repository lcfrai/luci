#!/usr/bin/env bash
# pipelines/scripts/licences_test.sh — licences.sh's npm comparator against two fixture trees
# (task 28). Go and Cargo need their report tools and the network, so they are proven by the real
# run in licences.yml, not here; the rules they are judged by are lib/allow_test.sh's.
#
#   bash pipelines/scripts/licences_test.sh
#
# Output: one line per check, then   licences_test: <k> of <n> checks passed   exit 0 iff k == n
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA="$HERE/testdata/licences"
CHECK="${PANACEA_APP_LICENCES:-$HERE/licences.sh}"

# Each fixture tree has only its own exceptions.yaml and lock; allow.yaml is the real one's
# shape, copied in here so a fixture cannot drift from it.
WORK="$(mktemp -d)" || { echo "licences_test: mktemp failed"; exit 2; }
trap 'rm -rf "$WORK"' EXIT

# case | exit status | a line the output must contain
CHECKS=(
  "npm_ok|0|licences: npm: 5 dependencies, all allowed"
  "npm_bad|1|licences: FAIL npm gpl-prod@1.0.0: licence GPL-3.0-only is not in licences/allow.yaml and has no exception"
  "npm_bad|1|licences: FAIL npm @tldraw/tldraw@3.0.0: in denied_packages"
  "npm_bad|1|licences: FAIL npm nolicence@0.1.0: no licence recorded"
  "npm_bad|1|licences: npm: 4 dependencies, 3 problems"
)

declare -A OUT=() STATUS=()
pass=0
for row in "${CHECKS[@]}"; do
  IFS='|' read -r c want_status want_line <<< "$row"
  if [ -z "${STATUS[$c]:-}" ]; then
    if ! { mkdir -p "$WORK/$c" && cp -R "$DATA/$c/." "$WORK/$c/" &&
           cp "$HERE/../../licences/allow.yaml" "$WORK/$c/licences/"; }; then
      echo "licences_test: cannot build $c"; exit 2
    fi
    OUT[$c]="$(bash "$CHECK" npm "$WORK/$c" 2>&1)"
    STATUS[$c]=$?
  fi
  if [ "${STATUS[$c]}" != "$want_status" ]; then
    echo "FAIL $c: exit ${STATUS[$c]}, want $want_status"
  else
    case $'\n'"${OUT[$c]}"$'\n' in
      *$'\n'"$want_line"$'\n'*) echo "ok   $c: $want_line"; pass=$((pass + 1)); continue ;;
      *) echo "FAIL $c: no line \"$want_line\"" ;;
    esac
  fi
  while IFS= read -r l; do printf '    %s\n' "$l"; done <<< "${OUT[$c]}"
done
echo "licences_test: $pass of ${#CHECKS[@]} checks passed"
[ "$pass" -eq "${#CHECKS[@]}" ]

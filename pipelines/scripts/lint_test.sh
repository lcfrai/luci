#!/usr/bin/env bash
# pipelines/scripts/lint_test.sh — runs lint.sh against one fixture tree per case (task 28).
#
#   bash pipelines/scripts/lint_test.sh          every case under testdata/lint/
#   bash pipelines/scripts/lint_test.sh <case>…  just these
#
# A case is testdata/lint/<case>/. Its tree is built in a temporary directory: _base/ first,
# then the case named in <case>/.extends (if any), then <case>/ itself, each copied over the
# last; then every path listed in <case>/.remove is deleted. <case>/.expect holds two lines: the
# exit status lint.sh must return, and a line its output must contain, verbatim.
#
# Output: one line per case, "ok   <case>" or "FAIL <case>: <why>", then
#         lint_test: <k> of <n> cases passed          exit 0 iff k == n
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA="$HERE/testdata/lint"
LINT="${PANACEA_APP_LINT:-$HERE/lint.sh}"

WORK="$(mktemp -d)" || { echo "lint_test: mktemp failed"; exit 2; }
trap 'rm -rf "$WORK"' EXIT

# overlay <case> <dest>: copy a case's files over <dest>; the harness's dot-files are removed.
# One cp per layer: a process costs about 300 ms on the build laptop.
overlay() {
  [ -d "$DATA/$1" ] || { echo "no such case: $1"; return 1; }
  cp -R "$DATA/$1/." "$2/" && rm -f "$2/.expect" "$2/.extends" "$2/.remove"
}

# show <text>: the output of a failed case, indented, with no extra process.
show() { local l; while IFS= read -r l; do printf '    %s\n' "$l"; done <<< "$1"; }

run_case() {
  local c="$1" tree="$WORK/$1" want_status="" want_line="" ext="" out status rel
  [ -f "$DATA/$c/.expect" ] || { echo "FAIL $c: no .expect"; return 1; }
  { IFS= read -r want_status; IFS= read -r want_line; } < "$DATA/$c/.expect"
  mkdir -p "$tree"
  overlay _base "$tree" || { echo "FAIL $c: _base did not copy"; return 1; }
  if [ -f "$DATA/$c/.extends" ]; then
    IFS= read -r ext < "$DATA/$c/.extends"
    overlay "$ext" "$tree" || { echo "FAIL $c: .extends $ext did not copy"; return 1; }
  fi
  overlay "$c" "$tree" || { echo "FAIL $c: did not copy"; return 1; }
  if [ -f "$DATA/$c/.remove" ]; then
    while IFS= read -r rel; do [ -n "$rel" ] && rm -f "$tree/$rel"; done < "$DATA/$c/.remove"
  fi
  out="$(bash "$LINT" "$tree" 2>&1)"
  status=$?
  if [ "$status" != "$want_status" ]; then
    echo "FAIL $c: exit $status, want $want_status"; show "$out"; return 1
  fi
  case $'\n'"$out"$'\n' in
    *$'\n'*"$want_line"*$'\n'*) ;;
    *) echo "FAIL $c: no line containing \"$want_line\""; show "$out"; return 1 ;;
  esac
  echo "ok   $c"
}

if [ "$#" -gt 0 ]; then
  CASES=("$@")
else
  CASES=()
  for d in "$DATA"/*/; do
    d="${d%/}"; d="${d##*/}"
    [ "$d" = _base ] || CASES+=("$d")
  done
fi

pass=0
for c in "${CASES[@]}"; do
  run_case "$c" && pass=$((pass + 1))
done
echo "lint_test: $pass of ${#CASES[@]} cases passed"
[ "$pass" -eq "${#CASES[@]}" ]

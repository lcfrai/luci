#!/usr/bin/env bash
# pipelines/scripts/licences_test.sh — licences.sh's three reports against fixture trees (task 28;
# Go and Cargo added in fix round 1, review I2). Go and Cargo run against stubs of go,
# go-licenses, cargo and cargo-deny (testdata/licences/stubs/, first on PATH), so no case needs
# the real tools or the network. The stub go-licenses prints each module's licences.csv, and fails
# as the real one does on a workspace module it was not told to --ignore; the stub cargo prints the
# deny.toml licences.sh wrote, one key per line after its section. The rules every licence is
# judged by are lib/allow_test.sh's; the real graphs are licences.yml's go, npm and cargo jobs.
# A case's ecosystem is its name up to the first "_".
#
#   bash pipelines/scripts/licences_test.sh
#
# Output: one line per check, then   licences_test: <k> of <n> checks passed   exit 0 iff k == n
set -uo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DATA="$HERE/testdata/licences"
CHECK="${PANACEA_APP_LICENCES:-$HERE/licences.sh}"
REAL_ALLOW="$HERE/../../licences/allow.yaml"

# Each fixture tree has only its own exceptions.yaml (and lock, modules or stub status);
# allow.yaml is the real one's, copied in here so a fixture cannot drift from it.
WORK="$(mktemp -d)" || { echo "licences_test: mktemp failed"; exit 2; }
trap 'rm -rf "$WORK"' EXIT

# The stubs, executable and first on PATH (git records them 100644, so they are copied).
if ! { mkdir -p "$WORK/bin" && cp "$DATA/stubs/"* "$WORK/bin/" && chmod +x "$WORK/bin/"*; }; then
  echo "licences_test: cannot install the stubs"; exit 2
fi
STUBPATH="$WORK/bin:$PATH"

# deny.toml's allow list must be allow.yaml's, in its order: '"MIT", "Apache-2.0", …'.
ALLOWED=""
while IFS= read -r a; do
  a="${a%$'\r'}"
  [ -n "$a" ] && ALLOWED+="${ALLOWED:+, }\"$a\""
done < <(yq '.allowed[]' "$REAL_ALLOW")
[ -n "$ALLOWED" ] || { echo "licences_test: cannot read $REAL_ALLOW"; exit 2; }

# case | exit status | a line the output must contain ("!" first: text no line may contain).
# @ALLOWED@ is allow.yaml's list, as above.
CHECKS=(
  "npm_ok|0|licences: npm: 5 dependencies, all allowed"
  "npm_bad|1|licences: FAIL npm gpl-prod@1.0.0: licence GPL-3.0-only is not in licences/allow.yaml and has no exception"
  "npm_bad|1|licences: FAIL npm @tldraw/tldraw@3.0.0: in denied_packages"
  "npm_bad|1|licences: FAIL npm nolicence@0.1.0: no licence recorded"
  "npm_bad|1|licences: npm: 4 dependencies, 3 problems"
  "npm_empty|2|licences: npm: desktop/package-lock.json has no production package: a report that reads nothing is a failure, not a clean graph"
  "go_ok|0|licences: go: 3 dependencies, all allowed"
  "go_bad|1|licences: FAIL go example.com/gpl: licence GPL-3.0-only is not in licences/allow.yaml and has no exception"
  "go_bad|1|licences: go: 3 dependencies, 1 problems"
  "go_empty|2|licences: go: go-licenses reported no package for the 2 modules in go.work: a report that reads nothing is a failure, not a clean graph"
  "cargo_ok|0|deny.toml: [graph] exclude-dev = true"
  "cargo_ok|0|deny.toml: [licenses] allow = [@ALLOWED@]"
  "cargo_ok|0|deny.toml: [licenses] { crate = \"unicode-ident\", allow = [\"MIT\", \"Apache-2.0\", \"Unicode-3.0\"] },"
  "cargo_ok|0|deny.toml: [licenses] { crate = \"target-lexicon\", allow = [\"Apache-2.0 WITH LLVM-exception\"] },"
  "cargo_ok|0|!npm-only"
  "cargo_ok|0|deny.toml: [licenses.private] ignore = true"
  "cargo_ok|0|licences: cargo: cargo deny check licenses passed"
  "cargo_bad|1|licences: FAIL cargo desktop/src-tauri: cargo deny check licenses exit 1 (its report is above)"
  "cargo_bad|1|licences: cargo: cargo deny check licenses failed"
)

declare -A OUT=() STATUS=()
pass=0
for row in "${CHECKS[@]}"; do
  IFS='|' read -r c want_status want_line <<< "$row"
  want_line="${want_line//@ALLOWED@/$ALLOWED}"
  if [ -z "${STATUS[$c]:-}" ]; then
    if ! { mkdir -p "$WORK/$c" && cp -R "$DATA/$c/." "$WORK/$c/" &&
           cp "$REAL_ALLOW" "$WORK/$c/licences/"; }; then
      echo "licences_test: cannot build $c"; exit 2
    fi
    OUT[$c]="$(PATH="$STUBPATH" bash "$CHECK" "${c%%_*}" "$WORK/$c" 2>&1)"
    STATUS[$c]=$?
  fi
  if [ "${STATUS[$c]}" != "$want_status" ]; then
    echo "FAIL $c: exit ${STATUS[$c]}, want $want_status"
  else
    case "$want_line" in
      '!'*)
        case "${OUT[$c]}" in
          *"${want_line#!}"*) echo "FAIL $c: a line contains \"${want_line#!}\"" ;;
          *) echo "ok   $c: no \"${want_line#!}\""; pass=$((pass + 1)); continue ;;
        esac ;;
      *)
        case $'\n'"${OUT[$c]}"$'\n' in
          *$'\n'"$want_line"$'\n'*) echo "ok   $c: $want_line"; pass=$((pass + 1)); continue ;;
          *) echo "FAIL $c: no line \"$want_line\"" ;;
        esac ;;
    esac
  fi
  while IFS= read -r l; do printf '    %s\n' "$l"; done <<< "${OUT[$c]}"
done
echo "licences_test: $pass of ${#CHECKS[@]} checks passed"
[ "$pass" -eq "${#CHECKS[@]}" ]

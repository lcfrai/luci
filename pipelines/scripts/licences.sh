#!/usr/bin/env bash
# pipelines/scripts/licences.sh — the linked-graph half of the OD-11 licence gate, applied
# repo-wide (task 28). Each graph is compared with licences/allow.yaml and exceptions.yaml.
#
#   bash pipelines/scripts/licences.sh go    [<root>]  go-licenses report, every module in go.work
#   bash pipelines/scripts/licences.sh npm   [<root>]  the desktop's production npm graph, read from
#                                                      desktop/package-lock.json: every package not
#                                                      marked dev or devOptional (what npm ci ships)
#   bash pipelines/scripts/licences.sh cargo [<root>]  cargo deny check licenses on desktop/src-tauri,
#                                                      with a deny.toml written from allow.yaml
#
# Go and npm licences are judged by lib/allow.sh (SPDX AND/OR/WITH; an exception names the
# ecosystem, the package and its exact licence text). Cargo is judged by cargo-deny itself; an
# exception with ecosystem cargo allows each licence id its licence text names, for that crate.
# denied_packages is checked by name for Go and npm (tldraw is an npm package); Cargo's graph is
# not checked by name.
#
# Output: one "licences: FAIL <eco> <package>: <why>" per problem, then
#         licences: <eco>: <n> dependencies, all allowed           exit 0
#         licences: <eco>: <n> dependencies, <k> problems          exit 1
# A tool that is missing or a report that fails is exit 2.
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=pipelines/scripts/lib/allow.sh
source "$HERE/lib/allow.sh"

ECO="${1:-}"
ROOT="${2:-$(git -C "$HERE" rev-parse --show-toplevel)}" || exit 2
cd "$ROOT" 2> /dev/null || { echo "licences: $ROOT is not a directory"; exit 2; }
need() { command -v "$1" > /dev/null || { echo "licences: $1 is not on PATH (bash pipelines/scripts/install-tool.sh $1)"; exit 2; }; }
need yq
load_allow . || {
  while IFS= read -r l; do echo "licences: FAIL $l"; done <<< "$ALLOW_ERR"
  exit 1
}

problems=0 n=0
fail() { echo "licences: FAIL $*"; problems=$((problems + 1)); }
judge() { # judge <eco> <name> <licence> [<version, shown after the name>]
  local who="$1 $2${4:-}"
  n=$((n + 1))
  if denied "$2"; then fail "$who: in denied_packages"
  elif [ -z "$3" ]; then fail "$who: no licence recorded"
  elif ! licence_ok "$1" "$2" "$3"; then fail "$who: licence $3 is not in licences/allow.yaml and has no exception"
  fi
}

go_report() {
  need go; need go-licenses
  # install-tool.sh builds go-licenses -trimpath, so the binary records no GOROOT and reads every
  # standard-library package as one with no module info (seen on 2026-09-25); name it.
  GOROOT="$(go env GOROOT)" || exit 2
  export GOROOT
  local mods m own out err ignore=()
  mods="$(awk '/^use[ \t]*\(/ {u = 1; next} u && /^\)/ {u = 0} u {gsub(/[ \t\r]/, ""); if ($0 != "") print}
               /^use[ \t]+[^( \t]/ {print $2}' go.work)" || exit 2
  [ -n "$mods" ] || { echo "licences: go.work names no module"; exit 2; }
  for m in $mods; do
    own="$(awk '$1 == "module" {print $2; exit}' "${m#./}/go.mod")" || exit 2
    ignore+=(--ignore "${own%$'\r'}")   # our own modules, whichever of them a module imports
  done
  err="$(mktemp)" || exit 2
  for m in $mods; do
    m="${m#./}"
    out="$(cd "$m" && go-licenses report ./... "${ignore[@]}" 2> "$err")" || {
      echo "licences: go $m: go-licenses report failed"; cat "$err"; rm -f "$err"; exit 2
    }
    while IFS=, read -r pkg _ lic; do
      [ -n "$pkg" ] && judge go "$pkg" "${lic%$'\r'}"
    done <<< "$out"
  done
  rm -f "$err"
}

npm_report() {
  local lock=desktop/package-lock.json out key ver lic
  [ -f "$lock" ] || { echo "licences: $lock is missing"; exit 2; }
  out="$(yq -p json -o yaml '.packages | to_entries | .[] | select(.key != "")
      | select((.value.dev // false) == false and (.value.devOptional // false) == false)
      | [.key, (.value.version // ""), (.value.license // "")] | join("|")' "$lock")" ||
    { echo "licences: cannot read $lock"; exit 2; }
  while IFS='|' read -r key ver lic; do
    [ -n "$key" ] && judge npm "${key##*node_modules/}" "${lic%$'\r'}" "@$ver"
  done <<< "$out"
}

cargo_report() {
  need cargo; need cargo-deny
  local tmp a x eco name lic e ids sep st
  tmp="$(mktemp -d)" || exit 2
  {
    echo "# Written by pipelines/scripts/licences.sh from licences/allow.yaml. Not committed."
    printf '[graph]\nexclude-dev = true\n\n[licenses]\nallow = ['
    sep=""; for a in "${ALLOWED[@]}"; do printf '%s"%s"' "$sep" "$a"; sep=", "; done
    printf ']\nconfidence-threshold = 0.93\nexceptions = [\n'
    for x in "${EXC[@]}"; do
      IFS=$'\t' read -r eco name lic <<< "$x"
      [ "$eco" = cargo ] || continue
      e="${lic//(/ }"; e="${e//)/ }"; e="${e// AND /|}"; e="${e// OR /|}"
      ids="" sep=""
      while IFS= read -r -d '|' a; do
        a="${a#"${a%%[![:space:]]*}"}"; a="${a%"${a##*[![:space:]]}"}"
        [ -n "$a" ] && { ids+="$sep\"$a\""; sep=", "; }
      done <<< "$e|"
      printf '  { crate = "%s", allow = [%s] },\n' "$name" "$ids"
    done
    printf ']\n\n[licenses.private]\nignore = true\n'
  } > "$tmp/deny.toml"
  cargo deny --manifest-path desktop/src-tauri/Cargo.toml --config "$tmp/deny.toml" check licenses
  st=$?
  rm -rf "$tmp"
  [ "$st" -eq 0 ] || fail "cargo desktop/src-tauri: cargo deny check licenses exit $st (its report is above)"
}

case "$ECO" in
  go) go_report ;;
  npm) npm_report ;;
  cargo) cargo_report ;;
  *) echo "usage: licences.sh go|npm|cargo [<root>]"; exit 2 ;;
esac
if [ "$ECO" = cargo ]; then
  if [ "$problems" -eq 0 ]; then echo "licences: cargo: cargo deny check licenses passed"; exit 0; fi
  echo "licences: cargo: cargo deny check licenses failed"
  exit 1
fi
if [ "$problems" -gt 0 ]; then
  echo "licences: $ECO: $n dependencies, $problems problems"
  exit 1
fi
echo "licences: $ECO: $n dependencies, all allowed"

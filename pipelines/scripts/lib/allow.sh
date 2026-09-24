# shellcheck shell=bash
# pipelines/scripts/lib/allow.sh — the OD-11 allowlist, read once (task 28). Sourced by
# chart-licences.sh (Helm) and licences.sh (Go, npm, Cargo); it needs yq on PATH.
#
#   load_allow <root>           reads <root>/licences/allow.yaml and exceptions.yaml into
#                               ALLOWED (SPDX ids), DENIED (name globs) and EXC ("eco<TAB>name<TAB>licence").
#                               Fails closed: no allowed list, an unreadable file, or an exception
#                               without all five fields (name, ecosystem, licence, why, approved)
#                               returns 1 with the reason(s), one per line, in ALLOW_ERR.
#   spdx_ok <expression>        0 iff the SPDX expression is satisfied by ALLOWED: AND needs every
#                               side, OR any side, parentheses group, "<id> WITH <exception>" counts
#                               as <id> (an exception only adds permissions). Anything that does
#                               not parse is not satisfied.
#   licence_ok <eco> <name> <expression>
#                               spdx_ok, or an exception that names exactly this ecosystem, name
#                               and licence text (a relicensed package fails again).
#   denied <name>               0 iff <name> matches a denied_packages glob (tldraw, @tldraw/*).

ALLOWED=() DENIED=() EXC=() ALLOW_ERR=""

load_allow() {
  local out line eco name lic why ok
  ALLOWED=() DENIED=() EXC=() ALLOW_ERR=""
  [ -f "$1/licences/allow.yaml" ] || { ALLOW_ERR="licences/allow.yaml is missing"; return 1; }
  out="$(yq '(.allowed[] | "A " + .), (.denied_packages[] | "D " + .)' "$1/licences/allow.yaml")" ||
    { ALLOW_ERR="licences/allow.yaml does not parse"; return 1; }
  while IFS= read -r line; do
    line="${line%$'\r'}"
    case "$line" in
      "A "?*) ALLOWED+=("${line#A }") ;;
      "D "?*) DENIED+=("${line#D }") ;;
    esac
  done <<< "$out"
  [ "${#ALLOWED[@]}" -gt 0 ] || { ALLOW_ERR="licences/allow.yaml lists no allowed licence"; return 1; }
  [ -f "$1/licences/exceptions.yaml" ] || { ALLOW_ERR="licences/exceptions.yaml is missing"; return 1; }
  out="$(yq '.exceptions[] | [.ecosystem, .name, .licence, .why, .approved] | map(. // "") | join("|")' \
    "$1/licences/exceptions.yaml")" || { ALLOW_ERR="licences/exceptions.yaml does not parse"; return 1; }
  ok=0
  # "|", not a tab: tab is IFS white space, so an empty field would collapse into the next.
  while IFS='|' read -r eco name lic why line; do
    line="${line%$'\r'}"
    [ -n "$eco$name$lic$why$line" ] || continue
    if [ -z "$eco" ] || [ -z "$name" ] || [ -z "$lic" ] || [ -z "$why" ] || [ -z "$line" ]; then
      [ -z "$ALLOW_ERR" ] || ALLOW_ERR+=$'\n'
      ALLOW_ERR+="licences/exceptions.yaml: the exception for '$name' needs name, ecosystem, licence, why and approved"
      ok=1
      continue
    fi
    EXC+=("$eco"$'\t'"$name"$'\t'"$lic")
  done <<< "$out"
  return "$ok"
}

allowed_id() {
  local a
  for a in "${ALLOWED[@]}"; do [ "$a" = "$1" ] && return 0; done
  return 1
}

# The expression parser: recursive descent over _T (tokens) from _I; _BAD marks a parse error.
_T=() _I=0 _BAD=0
_or()  { local r=1; _and && r=0; while [ "${_T[_I]:-}" = OR ]; do _I=$((_I + 1)); _and && r=0; done; return "$r"; }
_and() { local r=0; _atom || r=1; while [ "${_T[_I]:-}" = AND ]; do _I=$((_I + 1)); _atom || r=1; done; return "$r"; }
_atom() {
  local t="${_T[_I]:-}" r=0
  case "$t" in
    '(')
      _I=$((_I + 1)); _or || r=1
      [ "${_T[_I]:-}" = ')' ] || { _BAD=1; return 1; }
      _I=$((_I + 1)); return "$r" ;;
    ''|')'|AND|OR|WITH) _BAD=1; return 1 ;;
  esac
  _I=$((_I + 1))
  if [ "${_T[_I]:-}" = WITH ]; then
    [ -n "${_T[_I + 1]:-}" ] || { _BAD=1; return 1; }
    _I=$((_I + 2))
  fi
  allowed_id "$t"
}

spdx_ok() {
  local e="${1//(/ ( }"
  e="${e//)/ ) }"
  read -r -a _T <<< "$e"
  _I=0 _BAD=0
  [ "${#_T[@]}" -gt 0 ] || return 1
  _or || return 1
  [ "$_BAD" -eq 0 ] && [ "$_I" -eq "${#_T[@]}" ]
}

licence_ok() {
  local x
  spdx_ok "$3" && return 0
  for x in "${EXC[@]}"; do [ "$x" = "$1"$'\t'"$2"$'\t'"$3" ] && return 0; done
  return 1
}

denied() {
  local g
  # shellcheck disable=SC2053  # the right side is a glob on purpose
  for g in "${DENIED[@]}"; do [[ "$1" == $g ]] && return 0; done
  return 1
}

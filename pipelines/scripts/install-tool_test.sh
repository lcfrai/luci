#!/usr/bin/env bash
# pipelines/scripts/install-tool_test.sh — install-tool.sh's download-and-verify path, offline
# (task 28). It sources install-tool.sh, replaces the pin table with rows that point at local
# file:// fixtures built here, and allows the file protocol; nothing touches the network.
#
#   bash pipelines/scripts/install-tool_test.sh
#
# Output: one line per case, then   install-tool_test: <k> of <n> cases passed   exit 0 iff k == n
set -uo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPT="${PANACEA_APP_INSTALL_TOOL:-$HERE/install-tool.sh}"
WORK="$(mktemp -d)" || { echo "install-tool_test: mktemp failed"; exit 2; }
trap 'rm -rf "$WORK"' EXIT

# Fixtures: a bare executable, and a .tar.gz holding one under a directory.
mkdir -p "$WORK/src/pkg"
printf '#!/bin/sh\necho fixture-tool\n' > "$WORK/src/fixture-tool"
cp "$WORK/src/fixture-tool" "$WORK/src/pkg/fixture-tool"
tar -czf "$WORK/src/fixture.tar.gz" -C "$WORK/src" pkg/fixture-tool
sum() { local o; o="$(sha256sum "$1")" && printf '%s' "${o%% *}"; }
BIN_SHA="$(sum "$WORK/src/fixture-tool")"
TGZ_SHA="$(sum "$WORK/src/fixture.tar.gz")"
BAD_SHA="0000000000000000000000000000000000000000000000000000000000000000"
URL_BIN="file://$WORK/src/fixture-tool"
URL_TGZ="file://$WORK/src/fixture.tar.gz"

# run <pins> <tool> [<proto>]: install <tool> in a fresh shell that sourced the script, with <pins> as
# the table (@PLAT@ becomes this platform) and curl --proto <proto> (default =file; "default"
# keeps the script's own), into $WORK/tools. Prints the output; returns the status.
run() {
  PANACEA_APP_TOOLS_DIR="$WORK/tools" bash -c '
    set -euo pipefail
    source "$1" || exit 99
    PINS="${2//@PLAT@/$(platform)}"
    [ "$4" = default ] || PROTO="$4"
    setup_curl
    install_one "$3"' _ "$SCRIPT" "$1" "$2" "${3:-=file}" 2>&1
}
exe=""; case "$(uname -s)" in MINGW*|MSYS*|CYGWIN*) exe=.exe ;; esac

pass=0 n=0
check() { # check <name> <want status> <want substring> <status> <output> [<extra test>]
  n=$((n + 1))
  if [ "$4" != "$2" ]; then echo "FAIL $1: exit $4, want $2"; printf '    %s\n' "$5"; return; fi
  case "$5" in *"$3"*) ;; *) echo "FAIL $1: no \"$3\" in output"; printf '    %s\n' "$5"; return ;; esac
  if [ -n "${6:-}" ] && ! eval "$6"; then echo "FAIL $1: $6"; return; fi
  echo "ok   $1"; pass=$((pass + 1))
}

T="$WORK/tools/bin"
out="$(run "fixture-tool @PLAT@ 1.0.0 bin fixture-tool $BIN_SHA $URL_BIN" fixture-tool)"
check bin_good 0 "install-tool: fixture-tool 1.0.0" $? "$out" '[ -x "$T/fixture-tool$exe" ] && cmp -s "$T/fixture-tool$exe" "$WORK/src/fixture-tool"'

out="$(run "bad-tool @PLAT@ 1.0.0 bin bad-tool $BAD_SHA $URL_BIN" bad-tool)"
check bad_sha_refused 1 "SHA-256 mismatch" $? "$out" '[ ! -e "$T/bad-tool$exe" ]'

# The stamp from bin_good must not stand in for a pin whose hash has since changed.
out="$(run "fixture-tool @PLAT@ 1.0.1 bin fixture-tool $BAD_SHA $URL_BIN" fixture-tool)"
check changed_pin_is_verified 1 "SHA-256 mismatch" $? "$out"

out="$(run "tgz-tool @PLAT@ 2.0.0 tgz pkg/fixture-tool $TGZ_SHA $URL_TGZ" tgz-tool)"
check tgz_member_installed 0 "install-tool: tgz-tool 2.0.0" $? "$out" 'cmp -s "$T/tgz-tool$exe" "$WORK/src/fixture-tool"'

out="$(run "fixture-tool @PLAT@ 1.0.0 bin fixture-tool $BIN_SHA $URL_BIN" nope)"
check no_pin_refused 1 "no pin for nope on" $? "$out"

out="$(run "short-tool @PLAT@ 1.0.0 bin short-tool ${BIN_SHA:1} $URL_BIN" short-tool)"
check short_sha_refused 1 "short-tool: the pin has no SHA-256" $? "$out" '[ ! -e "$T/short-tool$exe" ]'

# The script's own protocol rule: https only, so a file:// (or http://) URL is never fetched.
out="$(run "proto-tool @PLAT@ 1.0.0 bin proto-tool $BIN_SHA $URL_BIN" proto-tool default)"
check https_only 1 "download failed" $? "$out" '[ ! -e "$T/proto-tool$exe" ]'

echo "install-tool_test: $pass of $n cases passed"
[ "$pass" -eq "$n" ]
